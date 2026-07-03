// Command ebadatelna-downloader stahuje naskenované matriky ze čtenárny
// Státního oblastního archivu v Praze (https://ebadatelna.soapraha.cz)
// do adresářové struktury "Nazev [ID]/0001.png".
//
// Web běží na Apache Wicket a je stavový: obrázek NENÍ dán číslem strany
// v cestě URL, ale stavem session + render-counterem v query stringu.
// Naivní stažení `/d/{id}/{N}?1--scanImage` proto vrací pořád tutéž stranu.
// Správné řešení: v jedné trvalé session (cookie jar) načíst HTML strany
// `/d/{id}/{N}`, z něj vyparsovat aktuální `#scanImage` src (s platným
// counterem) a stáhnout přesně tu URL.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	baseURL   = "https://ebadatelna.soapraha.cz"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

var (
	// <img id="scanImage" ... src="./1?6--scanImage">
	reScanImage = regexp.MustCompile(`id="scanImage"[^>]*\bsrc="([^"]+)"`)
	// <div class="dataPageTitle"> ... <h1>Kladno-ev 01</h1>
	reTitle = regexp.MustCompile(`dataPageTitle"[\s\S]*?<h1>([^<]+)</h1>`)
	// každá strana = jeden <option ...> v přepínači stran
	reOption = regexp.MustCompile(`<option\b`)
	// nepovolené znaky v názvu složky
	reUnsafe = regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f]`)
)

type config struct {
	id        string
	out       string
	in        string
	pages     int
	start     int
	end       int
	delay     time.Duration
	retries   int
	metaOnly  bool
	forceMeta bool
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.id, "id", "", "ID knihy (matrikaId), např. 8386 (povinné, nebo odvozeno z -in)")
	flag.StringVar(&cfg.out, "out", ".", "kořenový výstupní adresář")
	flag.StringVar(&cfg.in, "in", "", "existující složka knihy (pro -meta-only); ID se vezme z názvu [ID]")
	flag.IntVar(&cfg.pages, "pages", 0, "počet stran; 0 = auto-detekce z HTML")
	flag.IntVar(&cfg.start, "start", 1, "první strana ke stažení")
	flag.IntVar(&cfg.end, "end", 0, "poslední strana ke stažení; 0 = do konce")
	flag.DurationVar(&cfg.delay, "delay", 500*time.Millisecond, "pauza mezi stranami")
	flag.IntVar(&cfg.retries, "retries", 3, "počet opakování na stranu")
	flag.BoolVar(&cfg.metaOnly, "meta-only", false, "jen zapsat meta.json (bez stahování obrázků)")
	flag.BoolVar(&cfg.forceMeta, "force-meta", false, "přepsat existující meta.json")
	flag.Parse()
	cfg.in = strings.TrimSpace(cfg.in)

	if cfg.id == "" && cfg.in != "" {
		cfg.id = idFromFolder(cfg.in)
	}
	if cfg.id == "" {
		fmt.Fprintln(os.Stderr, "chyba: -id je povinné (např. -id 8386), nebo použij -in se složkou obsahující [ID]")
		flag.Usage()
		os.Exit(2)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "chyba: %v\n", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Jar:     jar,
		Timeout: 120 * time.Second,
	}

	// 1) Načíst HTML první strany → název knihy + počet stran + navázat session.
	firstHTML, err := fetchText(client, fmt.Sprintf("%s/d/%s/1", baseURL, cfg.id))
	if err != nil {
		return fmt.Errorf("načtení knihy %s: %w", cfg.id, err)
	}

	name := "kniha"
	if m := reTitle.FindStringSubmatch(firstHTML); m != nil {
		name = strings.TrimSpace(m[1])
	}
	total := len(reOption.FindAllStringIndex(firstHTML, -1))
	if cfg.pages > 0 {
		total = cfg.pages
	}
	if total <= 0 && !cfg.metaOnly {
		return fmt.Errorf("nepodařilo se zjistit počet stran (použij -pages)")
	}

	// Výstupní adresář: existující složka (-in) nebo odvozený z názvu.
	dir := cfg.in
	if dir == "" {
		dir = filepath.Join(cfg.out, sanitize(name)+" ["+cfg.id+"]")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// meta.json — z /pages/MatrikaPage/matrikaId/{id} (typ, datace, lokality…).
	if err := doMeta(client, cfg, dir, name, total); err != nil {
		fmt.Fprintf(os.Stderr, "VAROVÁNÍ: meta.json se nepodařilo vytvořit: %v\n", err)
	}
	if cfg.metaOnly {
		return nil
	}

	start := cfg.start
	if start < 1 {
		start = 1
	}
	end := cfg.end
	if end == 0 || end > total {
		end = total
	}

	fmt.Printf("Kniha: %q (ID %s) — %d stran, stahuji %d..%d do %q\n",
		name, cfg.id, total, start, end, dir)

	hashes := map[string]int{} // sha -> první strana s tímto obsahem (detekce duplicit)
	saved := 0
	for n := start; n <= end; n++ {
		// a) resume — přeskočit už stažené
		if existing := findExisting(dir, n); existing != "" {
			fmt.Printf("[%d/%d] přeskočeno (existuje %s)\n", n, total, filepath.Base(existing))
			continue
		}

		data, ext, err := downloadPage(client, cfg, n)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%d/%d] CHYBA: %v\n", n, total, err)
			continue
		}

		fname := fmt.Sprintf("%04d.%s", n, ext)
		if err := os.WriteFile(filepath.Join(dir, fname), data, 0o644); err != nil {
			return err
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(data))
		if prev, dup := hashes[sum]; dup {
			fmt.Fprintf(os.Stderr, "[%d/%d] VAROVÁNÍ: %s je BYTE-IDENTICKÝ se stranou %d!\n", n, total, fname, prev)
		} else {
			hashes[sum] = n
		}
		saved++
		fmt.Printf("[%d/%d] uloženo %s (%.1f MB)\n", n, total, fname, float64(len(data))/(1<<20))

		if cfg.delay > 0 && n < end {
			time.Sleep(cfg.delay)
		}
	}

	fmt.Printf("Hotovo: %d nových stran, %d unikátních obrazů.\n", saved, len(hashes))
	if saved > 1 && len(hashes) < saved {
		fmt.Fprintf(os.Stderr, "VAROVÁNÍ: nalezeny duplicitní obrazy (%d unikátních z %d stažených) — "+
			"to je příznak chyby „identické stránky\". Zkontroluj výstup.\n", len(hashes), saved)
	}
	return nil
}

// downloadPage stáhne jednu stranu: nejdřív HTML (pro čerstvý counter), pak obrázek.
func downloadPage(client *http.Client, cfg config, n int) (data []byte, ext string, err error) {
	pageURL := fmt.Sprintf("%s/d/%s/%d", baseURL, cfg.id, n)
	for attempt := 1; attempt <= cfg.retries; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt) * time.Second) // backoff
		}
		var html string
		html, err = fetchText(client, pageURL)
		if err != nil {
			continue
		}
		m := reScanImage.FindStringSubmatch(html)
		if m == nil {
			err = fmt.Errorf("v HTML nenalezen #scanImage")
			continue
		}
		var imgURL *url.URL
		imgURL, err = resolveURL(pageURL, m[1])
		if err != nil {
			continue
		}

		data, err = fetchBytes(client, imgURL.String())
		if err != nil {
			continue
		}
		ext = imageExt(data)
		if ext == "" {
			err = fmt.Errorf("odpověď není obrázek (%d B, možná přihlášení/chyba)", len(data))
			data = nil
			continue
		}
		return data, ext, nil
	}
	return nil, "", err
}

// imageExt vrátí "png"/"jpg" podle magic bytes, jinak "" (není obrázek).
func imageExt(b []byte) string {
	switch {
	case len(b) >= 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G':
		return "png"
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "jpg"
	default:
		return ""
	}
}

// findExisting vrátí cestu k už staženému neprázdnému souboru strany n, jinak "".
func findExisting(dir string, n int) string {
	for _, ext := range []string{"png", "jpg"} {
		p := filepath.Join(dir, fmt.Sprintf("%04d.%s", n, ext))
		if fi, err := os.Stat(p); err == nil && fi.Size() > 0 {
			return p
		}
	}
	return ""
}

func resolveURL(base, ref string) (*url.URL, error) {
	b, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	r, err := url.Parse(ref)
	if err != nil {
		return nil, err
	}
	return b.ResolveReference(r), nil
}

func fetchText(client *http.Client, u string) (string, error) {
	b, err := fetchBytes(client, u)
	return string(b), err
}

func fetchBytes(client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("HTTP %d pro %s", resp.StatusCode, u)
	}
	return io.ReadAll(resp.Body)
}

func sanitize(name string) string {
	s := reUnsafe.ReplaceAllString(name, "-")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ".") // Windows nemá rád tečku na konci
	if s == "" {
		s = "kniha"
	}
	return s
}
