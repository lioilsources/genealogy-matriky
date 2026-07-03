// Command matrika-ocr posílá naskenované stránky matrik dávkově do OCR modelu
// (Qwen, OpenAI-kompatibilní /v1/chat/completions na Sparku) a sbírá přepsaný
// text do jednoho JSONL souboru (řádek na stránku).
//
// Cíleno na to, aby backend (1 GPU / 1 request) nepřetěžoval: striktně
// sekvenčně (concurrency=1), pauza mezi requesty, retry s backoffem, resume.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const defaultPrompt = "Přepiš veškerý text z tohoto obrázku přesně tak, jak je. " +
	"Zachovej řádkování a pořadí. Vrať pouze přepsaný text, bez komentářů. Neopakuj řádky."

type options struct {
	in          string
	out         string
	mode        string // transcribe | structured | report
	schemaPath  string
	keepRaw     bool
	baseURL     string
	model       string
	apiKey      string
	concurrency int
	delay       time.Duration
	timeout     time.Duration
	retries     int
	maxTokens   int
	prompt      string
	promptFile  string
	exts        string
	resume      bool
	start       int
	end         int
	limit       int
	force       bool
	maxSide     int
}

// Record je jeden řádek výstupního JSONL.
type Record struct {
	File             string `json:"file"`
	OK               bool   `json:"ok"`
	Text             string `json:"text,omitempty"`
	Model            string `json:"model,omitempty"`
	Chars            int    `json:"chars,omitempty"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	DurationMs       int64  `json:"duration_ms,omitempty"`
	Attempts         int    `json:"attempts"`
	Error            string `json:"error,omitempty"`
	TS               string `json:"ts"`
}

func main() {
	opt := parseFlags()

	if err := run(opt); err != nil {
		fmt.Fprintf(os.Stderr, "chyba: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	var opt options
	fs := flag.NewFlagSet("matrika-ocr", flag.ExitOnError)
	fs.StringVar(&opt.in, "in", "", "vstupní adresář se skeny (rekurzivně) [povinné]")
	fs.StringVar(&opt.out, "out", "out.jsonl", "výstupní JSONL soubor (structured: auto ocr-out/<typ>/<kniha>.jsonl)")
	fs.StringVar(&opt.mode, "mode", "transcribe", "režim: transcribe | structured | report")
	fs.StringVar(&opt.schemaPath, "schema", "", "cesta ke schématu (structured; jinak dle meta.typ)")
	fs.BoolVar(&opt.keepRaw, "keep-raw", true, "ukládat surovou odpověď modelu (structured)")
	fs.StringVar(&opt.baseURL, "base-url", "http://192.168.88.66:8080", "base URL OCR gateway")
	fs.StringVar(&opt.model, "model", "ocr", "název modelu")
	fs.StringVar(&opt.apiKey, "api-key", "", "volitelný Bearer token (když gateway vyžaduje)")
	fs.IntVar(&opt.concurrency, "concurrency", 1, "počet souběžných requestů (backend zvládá 1)")
	fs.DurationVar(&opt.delay, "delay", 1500*time.Millisecond, "pauza mezi requesty")
	fs.DurationVar(&opt.timeout, "timeout", 180*time.Second, "HTTP timeout na request")
	fs.IntVar(&opt.retries, "retries", 3, "max pokusů na stránku (backoff 5/15/45 s)")
	fs.IntVar(&opt.maxTokens, "max-tokens", 2048, "max_tokens odpovědi")
	fs.StringVar(&opt.prompt, "prompt", "", "OCR prompt (prázdné = výchozí)")
	fs.StringVar(&opt.promptFile, "prompt-file", "", "soubor s promptem (přebíjí --prompt)")
	fs.StringVar(&opt.exts, "exts", "jpg,jpeg,png", "přípony obrázků (čárkou)")
	fs.BoolVar(&opt.resume, "resume", true, "přeskočit stránky už úspěšně zpracované v --out")
	fs.IntVar(&opt.start, "start", 1, "první stránka rozsahu (1-based index v seřazeném seznamu)")
	fs.IntVar(&opt.end, "end", 0, "poslední stránka rozsahu včetně (0 = do konce)")
	fs.IntVar(&opt.limit, "limit", 0, "zpracuj max N stránek (0 = bez limitu; pro test)")
	fs.BoolVar(&opt.force, "force", false, "ignoruj varování, že aktivní model není Qwen")
	fs.IntVar(&opt.maxSide, "max-side", 0, "downscale delší stranu na N px (0 = vypnuto)")
	_ = fs.Parse(os.Args[1:])

	if opt.mode != "report" && opt.in == "" {
		fmt.Fprintln(os.Stderr, "chyba: --in je povinné")
		fs.Usage()
		os.Exit(2)
	}
	return opt
}

func run(opt options) error {
	if opt.mode == "report" {
		return runReport(opt.out)
	}

	// Structured: načti meta (typ) + schéma, odvoď výstupní cestu, postav prompt.
	var schema *Schema
	prompt, err := resolvePrompt(opt)
	if err != nil {
		return err
	}
	if opt.mode == "structured" {
		meta, err := loadBookMeta(opt.in)
		if err != nil {
			return fmt.Errorf("nenačteno meta.json v %q — spusť downloader `--meta-only`: %w", opt.in, err)
		}
		schema, err = resolveSchema(opt.schemaPath, meta.Typ)
		if err != nil {
			return err
		}
		if opt.out == "out.jsonl" { // default → auto cesta ocr-out/<typ>/<kniha>.jsonl
			book := filepath.Base(strings.TrimRight(opt.in, string(os.PathSeparator)))
			opt.out = filepath.Join("ocr-out", meta.Typ, book+".jsonl")
		}
		if !customPromptSet(opt) {
			prompt = buildStructuredPrompt(schema)
		}
		fmt.Fprintf(os.Stderr, "Režim structured: typ=%s, schéma=%d sloupců, výstup %q.\n",
			schema.Typ, len(schema.Columns), opt.out)
	} else if opt.mode != "transcribe" {
		return fmt.Errorf("neznámý --mode %q (transcribe|structured|report)", opt.mode)
	}

	files, err := listImages(opt.in, opt.exts)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("v %q nenalezeny žádné obrázky (%s)", opt.in, opt.exts)
	}

	// Rozsah stránek (1-based, včetně end) na seřazeném seznamu — před resume/limit.
	all := len(files)
	lo := opt.start
	if lo < 1 {
		lo = 1
	}
	hi := opt.end
	if hi == 0 || hi > all {
		hi = all
	}
	if lo > hi {
		return fmt.Errorf("prázdný rozsah: --start %d > --end %d (celkem %d)", opt.start, hi, all)
	}
	files = files[lo-1 : hi]
	fmt.Fprintf(os.Stderr, "Rozsah stránek %d..%d z %d.\n", lo, hi, all)

	// Resume: posbírej už hotové (ok:true) a přeskoč je.
	done := map[string]bool{}
	if opt.resume {
		done, err = loadDone(opt.out)
		if err != nil {
			return fmt.Errorf("čtení %s pro resume: %w", opt.out, err)
		}
	}

	var todo []string
	for _, f := range files {
		rel := relKey(opt.in, f)
		if done[rel] {
			continue
		}
		todo = append(todo, f)
	}
	if opt.limit > 0 && len(todo) > opt.limit {
		todo = todo[:opt.limit]
	}

	fmt.Fprintf(os.Stderr, "Nalezeno %d obrázků, hotovo %d, ke zpracování %d.\n",
		len(files), len(done), len(todo))
	if len(todo) == 0 {
		fmt.Fprintln(os.Stderr, "Není co dělat.")
		return nil
	}

	client := &http.Client{Timeout: opt.timeout}

	// Pre-flight: ověř, že aktivní model je Qwen (ne omylem TrOCR).
	preflight(client, opt)

	// Výstup v append módu (resume přidává za existující); zajisti adresář.
	if dir := filepath.Dir(opt.out); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	outF, err := os.OpenFile(opt.out, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer outF.Close()
	var writeMu sync.Mutex

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	total := len(todo)
	var counter, okCount, failCount int64
	started := time.Now()

	// process vrací JSON řádek, ok a shrnutí pro stderr (dle režimu).
	process := func(f string) (line []byte, ok bool, summary string) {
		if opt.mode == "structured" {
			rec := structuredExtract(client, opt, schema, prompt, f)
			line, _ = json.Marshal(rec)
			return line, rec.OK, structuredSummary(rec)
		}
		rec := processFile(client, opt, prompt, f)
		line, _ = json.Marshal(rec)
		if rec.OK {
			return line, true, fmt.Sprintf("ok (%.1fs, %d znaků)", float64(rec.DurationMs)/1000, rec.Chars)
		}
		return line, false, "CHYBA: " + rec.Error
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for f := range jobs {
			if ctx.Err() != nil {
				return // SIGINT: doběhne rozdělané, nové nezačíná
			}
			line, ok, summary := process(f)
			n := atomic.AddInt64(&counter, 1)
			if ok {
				atomic.AddInt64(&okCount, 1)
			} else {
				atomic.AddInt64(&failCount, 1)
			}

			writeMu.Lock()
			outF.Write(append(line, '\n'))
			outF.Sync() // durabilita: progres přežije pád
			writeMu.Unlock()

			fmt.Fprintf(os.Stderr, "[%d/%d] %s … %s\n", n, total, relKey(opt.in, f), summary)

			if opt.delay > 0 && ctx.Err() == nil {
				select {
				case <-time.After(opt.delay):
				case <-ctx.Done():
				}
			}
		}
	}

	conc := opt.concurrency
	if conc < 1 {
		conc = 1
	}
	for i := 0; i < conc; i++ {
		wg.Add(1)
		go worker()
	}

feed:
	for _, f := range todo {
		select {
		case jobs <- f:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	fmt.Fprintf(os.Stderr, "\nHotovo: %d ok, %d chyb, za %s.\n",
		atomic.LoadInt64(&okCount), atomic.LoadInt64(&failCount),
		time.Since(started).Round(time.Second))
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "(přerušeno signálem — znovu spusť pro navázání, resume doplní zbytek)")
	}

	if opt.mode == "structured" {
		if err := runReport(opt.out); err != nil {
			fmt.Fprintf(os.Stderr, "VAROVÁNÍ: report se nepovedl: %v\n", err)
		}
	}
	return nil
}

// processFile stáhne přepis jednoho obrázku, s retry/backoffem.
func processFile(client *http.Client, opt options, prompt, path string) Record {
	rel := relKey(opt.in, path)
	rec := Record{File: rel, TS: time.Now().UTC().Format(time.RFC3339)}

	dataURL, err := imageDataURL(path, opt.maxSide)
	if err != nil {
		rec.Attempts = 0
		rec.Error = "načtení obrázku: " + err.Error()
		return rec
	}

	body, err := json.Marshal(buildRequest(opt, prompt, dataURL))
	if err != nil {
		rec.Error = "sestavení requestu: " + err.Error()
		return rec
	}

	backoff := []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second}
	var lastErr error
	for attempt := 1; attempt <= opt.retries; attempt++ {
		rec.Attempts = attempt
		start := time.Now()
		text, pt, ct, retryable, err := doRequest(client, opt, body)
		if err == nil {
			rec.OK = true
			rec.Text = text
			rec.Model = opt.model
			rec.Chars = len([]rune(text))
			rec.PromptTokens = pt
			rec.CompletionTokens = ct
			rec.DurationMs = time.Since(start).Milliseconds()
			rec.Error = ""
			rec.TS = time.Now().UTC().Format(time.RFC3339)
			return rec
		}
		lastErr = err
		if !retryable || attempt == opt.retries {
			break
		}
		wait := backoff[min(attempt-1, len(backoff)-1)]
		fmt.Fprintf(os.Stderr, "    %s: pokus %d selhal (%v), čekám %s…\n", rel, attempt, err, wait)
		time.Sleep(wait)
	}
	rec.OK = false
	if lastErr != nil {
		rec.Error = lastErr.Error()
	} else {
		rec.Error = "neznámá chyba"
	}
	rec.TS = time.Now().UTC().Format(time.RFC3339)
	return rec
}

// doRequest provede jeden POST; vrací text, tokeny a zda je chyba opakovatelná.
func doRequest(client *http.Client, opt options, body []byte) (text string, pt, ct int, retryable bool, err error) {
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(opt.baseURL, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if opt.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+opt.apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, 0, true, err // timeout / conn error → opakovatelné
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))

	if resp.StatusCode != http.StatusOK {
		retry := resp.StatusCode == 429 || resp.StatusCode >= 500
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return "", 0, 0, retry, fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}

	var cr chatResp
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return "", 0, 0, false, fmt.Errorf("neplatná odpověď: %w", err)
	}
	if cr.Error != nil && cr.Error.Message != "" {
		return "", 0, 0, false, fmt.Errorf("API error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", 0, 0, true, errors.New("odpověď bez choices")
	}
	return cr.Choices[0].Message.Content, cr.Usage.PromptTokens, cr.Usage.CompletionTokens, false, nil
}

// --- HTTP typy ---

type chatReq struct {
	Model            string    `json:"model"`
	Temperature      float64   `json:"temperature"`
	MaxTokens        int       `json:"max_tokens"`
	FrequencyPenalty float64   `json:"frequency_penalty"`
	PresencePenalty  float64   `json:"presence_penalty"`
	Messages         []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content []part `json:"content"`
}

type part struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func buildRequest(opt options, prompt, dataURL string) chatReq {
	return chatReq{
		Model:            opt.model,
		Temperature:      0.1,
		MaxTokens:        opt.maxTokens,
		FrequencyPenalty: 0.6,
		PresencePenalty:  0.4,
		Messages: []message{{
			Role: "user",
			Content: []part{
				{Type: "text", Text: prompt},
				{Type: "image_url", ImageURL: &imageURL{URL: dataURL}},
			},
		}},
	}
}

// --- Pre-flight ---

func preflight(client *http.Client, opt options) {
	req, _ := http.NewRequest(http.MethodGet, strings.TrimRight(opt.baseURL, "/")+"/v1/models", nil)
	if opt.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+opt.apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pre-flight: nelze ověřit model (%v), pokračuji (retry to podchytí).\n", err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var mr struct {
		Data []struct {
			ID   string `json:"id"`
			Root string `json:"root"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &mr); err != nil || len(mr.Data) == 0 {
		fmt.Fprintf(os.Stderr, "pre-flight: nečekaná odpověď /v1/models, pokračuji.\n")
		return
	}
	// najdi model podle id, jinak vezmi první
	root, id := mr.Data[0].Root, mr.Data[0].ID
	for _, m := range mr.Data {
		if m.ID == opt.model {
			root, id = m.Root, m.ID
			break
		}
	}
	if strings.Contains(strings.ToLower(root+id), "qwen") {
		fmt.Fprintf(os.Stderr, "pre-flight: aktivní model %q (root %q) — OK.\n", id, root)
		return
	}
	msg := fmt.Sprintf("pre-flight: aktivní model %q (root %q) NEvypadá jako Qwen", id, root)
	if opt.force {
		fmt.Fprintf(os.Stderr, "%s — pokračuji (--force).\n", msg)
		return
	}
	fmt.Fprintf(os.Stderr, "%s.\nZkontroluj, že na Sparku běží OCR (Qwen), nebo přidej --force.\n", msg)
	os.Exit(3)
}

// --- Obrázek → data URL ---

func imageDataURL(path string, maxSide int) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mime := sniffMIME(raw)

	if maxSide > 0 {
		if scaled, smime, ok := downscaleIfNeeded(raw, maxSide); ok {
			raw, mime = scaled, smime
		}
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func sniffMIME(b []byte) string {
	if len(b) >= 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G' {
		return "image/png"
	}
	return "image/jpeg"
}

// downscaleIfNeeded zmenší delší stranu na maxSide (bilineárně) a vrátí JPEG.
// Když je obrázek menší nebo ho nelze dekódovat, vrátí ok=false (pošle se originál).
func downscaleIfNeeded(raw []byte, maxSide int) ([]byte, string, bool) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", false
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	longest := w
	if h > w {
		longest = h
	}
	if longest <= maxSide {
		return nil, "", false
	}
	scale := float64(maxSide) / float64(longest)
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	if nw < 1 || nh < 1 {
		return nil, "", false
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := float64(y) / scale
		for x := 0; x < nw; x++ {
			sx := float64(x) / scale
			dst.Set(x, y, src.At(b.Min.X+int(sx), b.Min.Y+int(sy)))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 90}); err != nil {
		return nil, "", false
	}
	return buf.Bytes(), "image/jpeg", true
}

// --- Pomocné ---

// customPromptSet vrátí true, když uživatel zadal vlastní prompt (--prompt/-file).
func customPromptSet(opt options) bool {
	return strings.TrimSpace(opt.prompt) != "" || opt.promptFile != ""
}

func resolvePrompt(opt options) (string, error) {
	if opt.promptFile != "" {
		b, err := os.ReadFile(opt.promptFile)
		if err != nil {
			return "", fmt.Errorf("čtení prompt-file: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	if strings.TrimSpace(opt.prompt) != "" {
		return opt.prompt, nil
	}
	return defaultPrompt, nil
}

func listImages(root, exts string) ([]string, error) {
	want := map[string]bool{}
	for _, e := range strings.Split(exts, ",") {
		e = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(e, ".")))
		if e != "" {
			want[e] = true
		}
	}
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(p), "."))
		if want[ext] {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out) // deterministicky, resumovatelně
	return out, nil
}

// loadDone načte z existujícího JSONL soubory s ok:true.
func loadDone(out string) (map[string]bool, error) {
	done := map[string]bool{}
	f, err := os.Open(out)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return done, nil
		}
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20) // dlouhé řádky (přepis textu)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r struct {
			File string `json:"file"`
			OK   bool   `json:"ok"`
		}
		if json.Unmarshal(line, &r) == nil && r.OK && r.File != "" {
			done[r.File] = true
		}
	}
	return done, sc.Err()
}

// relKey vrací cestu souboru relativně k root, s '/' oddělovači (stabilní klíč).
func relKey(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	return filepath.ToSlash(rel)
}
