package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// StructuredRecord je jeden řádek JSONL ve strukturovaném režimu (řádek = 1 sken).
type StructuredRecord struct {
	File             string              `json:"file"`
	Typ              string              `json:"typ"`
	Folio            string              `json:"folio,omitempty"`
	Rok              string              `json:"rok,omitempty"`
	OK               bool                `json:"ok"`
	RowsCount        int                 `json:"rows_count"`
	Rows             []map[string]string `json:"rows"`
	Lint             LintResult          `json:"lint"`
	Raw              string              `json:"raw,omitempty"`
	Model            string              `json:"model,omitempty"`
	PromptTokens     int                 `json:"prompt_tokens,omitempty"`
	CompletionTokens int                 `json:"completion_tokens,omitempty"`
	DurationMs       int64               `json:"duration_ms,omitempty"`
	Attempts         int                 `json:"attempts"`
	Error            string              `json:"error,omitempty"`
	TS               string              `json:"ts"`
}

const strictSuffix = "\n\nDŮLEŽITÉ: Vrať POUZE validní JSON objekt " +
	"{\"folio\":\"\",\"rok\":\"\",\"rows\":[{…}]} — nic jiného, žádný text ani markdown."

// structuredExtract zpracuje jeden sken (celý, nebo rozpůlený při --split lr).
func structuredExtract(client *http.Client, opt options, schema *Schema, prompt, path string) StructuredRecord {
	rel := relKey(opt.in, path)
	if opt.split == "lr" {
		if rec, ok := splitExtract(client, opt, schema, prompt, path, rel); ok {
			return rec
		}
		// crop selhal → spadni na celý sken
	}
	dataURL, err := imageDataURL(path, opt.maxSide)
	if err != nil {
		return failStructured(newRec(rel, schema), "načtení obrázku: "+err.Error())
	}
	return extractOne(client, opt, schema, prompt, dataURL, rel)
}

// newRec vytvoří prázdný StructuredRecord.
func newRec(rel string, schema *Schema) StructuredRecord {
	return StructuredRecord{
		File: rel, Typ: schema.Typ,
		Rows: []map[string]string{}, Lint: LintResult{OK: true, Issues: []string{}},
		TS: time.Now().UTC().Format(time.RFC3339),
	}
}

// extractOne pošle jeden obrázek (dataURL) do modelu a vrátí strukturovaný záznam.
func extractOne(client *http.Client, opt options, schema *Schema, prompt, dataURL, rel string) StructuredRecord {
	rec := newRec(rel, schema)
	backoff := []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second}
	var lastErr error
	strict := ""
	for attempt := 1; attempt <= opt.retries; attempt++ {
		rec.Attempts = attempt
		body, _ := json.Marshal(buildRequest(opt, prompt+strict, dataURL))
		start := time.Now()
		text, pt, ct, retryable, err := doRequest(client, opt, body)
		if err != nil {
			lastErr = err
			if !retryable || attempt == opt.retries {
				break
			}
			wait := backoff[min(attempt-1, len(backoff)-1)]
			fmt.Fprintf(os.Stderr, "    %s: pokus %d selhal (%v), čekám %s…\n", rec.File, attempt, err, wait)
			time.Sleep(wait)
			continue
		}

		// máme text → zkus vytáhnout a naparsovat JSON
		jsonStr, ok := extractJSON(text)
		var parsed struct {
			Folio string                   `json:"folio"`
			Rok   string                   `json:"rok"`
			Rows  []map[string]interface{} `json:"rows"`
		}
		if ok {
			err = json.Unmarshal([]byte(jsonStr), &parsed)
		} else {
			err = fmt.Errorf("v odpovědi nenalezen JSON")
		}
		if err != nil {
			lastErr = fmt.Errorf("parsování JSON: %w", err)
			if opt.keepRaw {
				rec.Raw = text
			}
			if attempt == opt.retries {
				break
			}
			strict = strictSuffix // příště tvrdší výzva (content-retry)
			fmt.Fprintf(os.Stderr, "    %s: odpověď nebyla JSON, zkouším znovu (content-retry)…\n", rec.File)
			continue
		}

		// úspěch
		rec.OK = true
		rec.Folio = strings.TrimSpace(parsed.Folio)
		rec.Rok = strings.TrimSpace(parsed.Rok)
		rec.Rows = normalizeRows(schema, parsed.Rows, &rec.Lint)
		rec.RowsCount = len(rec.Rows)
		rec.Model = opt.model
		rec.PromptTokens = pt
		rec.CompletionTokens = ct
		rec.DurationMs = time.Since(start).Milliseconds()
		if opt.keepRaw {
			rec.Raw = text
		}
		lintPage(schema, &rec)
		return rec
	}

	msg := "neznámá chyba"
	if lastErr != nil {
		msg = lastErr.Error()
	}
	return failStructured(rec, msg)
}

func failStructured(rec StructuredRecord, msg string) StructuredRecord {
	rec.OK = false
	rec.Error = msg
	rec.Lint.OK = false
	rec.Lint.Issues = append(rec.Lint.Issues, "extrakce selhala: "+msg)
	rec.TS = time.Now().UTC().Format(time.RFC3339)
	return rec
}

// normalizeRows nechá jen klíče ze schématu, doplní chybějící "", neznámé klíče
// zaznamená do lintu. Hodnoty (i čísla/null) převede na řetězec.
func normalizeRows(schema *Schema, raw []map[string]interface{}, lint *LintResult) []map[string]string {
	// Qwen občas vkládá mezery do názvů klíčů ("ne vest a _ datum") → porovnávej
	// klíče bez mezer a malými písmeny.
	canon := map[string]string{}
	for _, c := range schema.Columns {
		canon[normKey(c.Key)] = c.Key
	}
	unknown := map[string]bool{}
	out := make([]map[string]string, 0, len(raw))
	for _, r := range raw {
		row := map[string]string{}
		for _, c := range schema.Columns {
			row[c.Key] = ""
		}
		nonEmpty := false
		for k, v := range r {
			if sk, ok := canon[normKey(k)]; ok {
				if s := anyToString(v); s != "" || row[sk] == "" {
					row[sk] = s
					if s != "" {
						nonEmpty = true
					}
				}
			} else {
				unknown[k] = true
			}
		}
		if nonEmpty { // zahoď zcela prázdné řádky ({} od modelu)
			out = append(out, row)
		}
	}
	for k := range unknown {
		lint.Issues = append(lint.Issues, "neznámý klíč od modelu: "+strings.TrimSpace(k))
	}
	return out
}

// normKey normalizuje klíč pro porovnání: odstraní veškeré mezery a malá písmena.
func normKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), ""))
}

func anyToString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		// celá čísla bez desetinné tečky
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	case bool:
		if t {
			return "ano"
		}
		return "ne"
	case []interface{}:
		// Qwen občas vrátí hodnotu jako pole → spoj do jednoho řetězce
		parts := make([]string, 0, len(t))
		for _, e := range t {
			if s := anyToString(e); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "; ")
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// extractJSON vytáhne první vyvážený JSON objekt z textu (odstraní markdown fence).
func extractJSON(text string) (string, bool) {
	s := strings.TrimSpace(text)
	// odstraň ```json … ``` fence, pokud jsou
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		s = strings.TrimPrefix(s, "json")
		s = strings.TrimPrefix(s, "JSON")
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

// structuredSummary sestaví shrnutí pro stderr.
func structuredSummary(rec StructuredRecord) string {
	if !rec.OK {
		return "CHYBA: " + rec.Error
	}
	warn := ""
	if !rec.Lint.OK {
		warn = fmt.Sprintf(", ⚠ %d lint", len(rec.Lint.Issues))
	}
	return fmt.Sprintf("ok (%.1fs, %d řádků, %d prázdných buněk%s)",
		float64(rec.DurationMs)/1000, rec.RowsCount, rec.Lint.EmptyCells, warn)
}
