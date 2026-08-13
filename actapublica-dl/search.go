package main

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// bookRow je jeden řádek výsledků hledání (jedna kniha matriky).
//
// Sloupce tabulky (ověřeno na živém HTML, obec_id=2787): Číslo knihy |
// Původce (+ typ původce) | Narození od-do (+ index) | Oddaní od-do (+ index)
// | Zemřelí od-do (+ index) | Obce (tlačítko s popoverem) | Počet snímků |
// odkaz "Zobrazit".
type bookRow struct {
	DetailID       string
	BookNo         string
	Provenance     string
	ProvenanceType string
	Narozeni       string
	Oddani         string
	Zemreli        string
	Localities     []string
	ScanCount      int
}

type searchMeta struct {
	Header string // hlavička hledání ("Sudoměřice (...), obec: ..., okres: ...")
	Total  int    // "celkem: N" z patičky; 0 = nenalezeno
}

var (
	reTbody     = regexp.MustCompile(`(?is)<tbody>(.*?)</tbody>`)
	reRow       = regexp.MustCompile(`(?is)<tr>(.*?)</tr>`)
	reCell      = regexp.MustCompile(`(?is)<td\b[^>]*>(.*?)</td>`)
	reRowDetail = regexp.MustCompile(`matrika/detail/(\d+)`)
	reCelkem    = regexp.MustCompile(`(?i)celkem[:\s]+(\d+)`)
	reTag       = regexp.MustCompile(`<[^>]+>`)
	reDummyTag  = regexp.MustCompile(`(?is)<input\b[^>]*\bid="dummy"[^>]*>`)
	reValueAttr = regexp.MustCompile(`(?is)\bvalue="([^"]*)"`)
	reStrong    = regexp.MustCompile(`(?is)<strong>(.*?)</strong>`)
)

func searchURL(obecID string, page int) string {
	return fmt.Sprintf("%s/actapublica/matrika/hledani?typ=obec&obec_id=%s&page=%d", baseURL, obecID, page)
}

// listBooks projde všechny stránky hledání pro obec_id a vrátí všechny
// nalezené knihy (20 řádků/stránka, končí na první prázdné stránce nebo po
// dosažení "celkem").
func listBooks(client *http.Client, obecID string) ([]bookRow, searchMeta, error) {
	var all []bookRow
	var meta searchMeta
	for page := 1; ; page++ {
		htmlStr, err := fetchText(client, searchURL(obecID, page))
		if err != nil {
			return all, meta, fmt.Errorf("stránka %d: %w", page, err)
		}
		if page == 1 {
			meta = parseSearchMeta(htmlStr)
		}
		rows := parseSearchRows(htmlStr)
		if len(rows) == 0 {
			break
		}
		all = append(all, rows...)
		if meta.Total > 0 && len(all) >= meta.Total {
			break
		}
	}
	return all, meta, nil
}

func parseSearchMeta(htmlStr string) searchMeta {
	var m searchMeta
	if mm := reCelkem.FindStringSubmatch(htmlStr); mm != nil {
		m.Total, _ = strconv.Atoi(mm[1])
	}
	// hlavička hledání: <input ... id="dummy" value="Sudoměřice (...), obec: ..., okres: ..." disabled>
	if tag := reDummyTag.FindString(htmlStr); tag != "" {
		if mm := reValueAttr.FindStringSubmatch(tag); mm != nil {
			m.Header = html.UnescapeString(mm[1])
		}
	}
	return m
}

func parseSearchRows(htmlStr string) []bookRow {
	tb := reTbody.FindStringSubmatch(htmlStr)
	if tb == nil {
		return nil
	}
	var rows []bookRow
	for _, rm := range reRow.FindAllStringSubmatch(tb[1], -1) {
		rowHTML := rm[1]
		dm := reRowDetail.FindStringSubmatch(rowHTML)
		if dm == nil {
			continue // hlavička / řádek bez odkazu na detail
		}
		cells := reCell.FindAllStringSubmatch(rowHTML, -1)
		row := bookRow{DetailID: dm[1]}
		get := func(i int) string {
			if i < len(cells) {
				return cells[i][1]
			}
			return ""
		}
		row.BookNo = cleanText(get(0))
		row.Provenance, row.ProvenanceType = splitMainSub(get(1))
		row.Narozeni, _ = splitMainSub(get(2))
		row.Oddani, _ = splitMainSub(get(3))
		row.Zemreli, _ = splitMainSub(get(4))
		for _, sm := range reStrong.FindAllStringSubmatch(get(5), -1) {
			if loc := cleanText(sm[1]); loc != "" {
				row.Localities = append(row.Localities, loc)
			}
		}
		if sc, err := strconv.Atoi(strings.TrimSpace(cleanText(get(6)))); err == nil {
			row.ScanCount = sc
		}
		rows = append(rows, row)
	}
	return rows
}

func printSearchResults(obecID string, rows []bookRow, meta searchMeta) {
	if meta.Header != "" {
		fmt.Println(meta.Header)
	}
	fmt.Printf("obec_id=%s: %d knih", obecID, len(rows))
	if meta.Total > 0 {
		fmt.Printf(" (server hlásí celkem: %d)", meta.Total)
	}
	fmt.Println()
	totalScans := 0
	for _, r := range rows {
		fmt.Printf("  [%s] %-8s %-28s N=%-14s O=%-14s Z=%-14s skenů=%d\n",
			r.DetailID, r.BookNo, r.Provenance, orDash(r.Narozeni), orDash(r.Oddani), orDash(r.Zemreli), r.ScanCount)
		totalScans += r.ScanCount
	}
	fmt.Printf("celkem skenů (dle výpisu): %d\n", totalScans)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// splitMainSub rozdělí buňku tvaru "HLAVNÍ<br><small><em>VEDLEJŠÍ</em></small>"
// (typicky Původce/typ původce nebo rozsah/index rozsah) na hlavní a vedlejší
// hodnotu. "-" (žádná hodnota) se vrací jako prázdný řetězec.
func splitMainSub(cellHTML string) (main, sub string) {
	parts := regexp.MustCompile(`(?is)<br\s*/?>`).Split(cellHTML, 2)
	main = cleanText(parts[0])
	if main == "-" {
		main = ""
	}
	if len(parts) > 1 {
		if sm := regexp.MustCompile(`(?is)<small><em>(.*?)</em>`).FindStringSubmatch(parts[1]); sm != nil {
			sub = cleanText(sm[1])
			if sub == "-" {
				sub = ""
			}
		}
	}
	return main, sub
}

// cleanText strhne tagy, unescapuje HTML entity a srazí whitespace.
func cleanText(s string) string {
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.Trim(s, " -")
}
