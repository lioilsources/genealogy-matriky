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
// Mapování buněk (BookNo/Provenance/Ranges/Localities/ScanCount) je ODVOZENÉ
// z pořadí polí v návrhu, ne ověřené na živém HTML (síť je z tohoto prostředí
// blokovaná Cloudflare — viz README). Cells obsahuje syrové buňky řádku v
// pořadí z tabulky, takže mapování jde po prvním `make list` rychle opravit.
type bookRow struct {
	DetailID   string
	BookNo     string
	Provenance string
	Ranges     string
	Localities string
	ScanCount  int
	Cells      []string
}

type searchMeta struct {
	Header string // hlavička stránky (obec, okres…), pokud se ji podaří najít
	Total  int    // "celkem: N" z patičky; 0 = nenalezeno
}

var (
	// <tr ... onclick="window.location='…/matrika/detail/12345'" ...> … </tr>
	reSearchRow = regexp.MustCompile(`(?is)<tr\b[^>]*\bonclick="window\.location='([^']*?/matrika/detail/(\d+))'"[^>]*>(.*?)</tr>`)
	reCell      = regexp.MustCompile(`(?is)<t[dh]\b[^>]*>(.*?)</t[dh]>`)
	reCelkem    = regexp.MustCompile(`(?i)celkem[:\s]+(\d+)`)
	reTag       = regexp.MustCompile(`<[^>]+>`)
	reH1        = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
)

func searchURL(obecID string, page int) string {
	return fmt.Sprintf("%s/actapublica/matrika/hledani?typ=obec&obec_id=%s&page=%d", baseURL, obecID, page)
}

// listBooks projde všechny stránky hledání pro obec_id a vrátí všechny
// nalezené knihy (20 řádků/stránka, končí na první prázdné stránce).
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
	if mm := reH1.FindStringSubmatch(htmlStr); mm != nil {
		m.Header = cleanText(mm[1])
	}
	return m
}

func parseSearchRows(htmlStr string) []bookRow {
	var rows []bookRow
	for _, m := range reSearchRow.FindAllStringSubmatch(htmlStr, -1) {
		detailID := m[2]
		var cells []string
		for _, cm := range reCell.FindAllStringSubmatch(m[3], -1) {
			cells = append(cells, cleanText(cm[1]))
		}
		row := bookRow{DetailID: detailID, Cells: cells}
		if len(cells) > 0 {
			row.BookNo = cells[0]
		}
		if len(cells) > 1 {
			row.Provenance = cells[1]
		}
		if len(cells) > 2 {
			row.Ranges = cells[2]
		}
		if len(cells) > 3 {
			row.Localities = cells[3]
		}
		if n := len(cells); n > 0 {
			if sc, err := strconv.Atoi(strings.TrimSpace(cells[n-1])); err == nil {
				row.ScanCount = sc
			}
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
		fmt.Printf("  [%s] %-10s %-20s %-15s %-20s skenů=%d\n",
			r.DetailID, r.BookNo, r.Provenance, r.Ranges, r.Localities, r.ScanCount)
		totalScans += r.ScanCount
	}
	fmt.Printf("celkem skenů (dle výpisu): %d\n", totalScans)
}

// cleanText strhne tagy, unescapuje HTML entity a srazí whitespace.
func cleanText(s string) string {
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.Trim(s, " -")
}
