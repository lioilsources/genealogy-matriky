package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// LintResult je výsledek per-stránka kontroly.
type LintResult struct {
	OK         bool     `json:"ok"`
	EmptyCells int      `json:"empty_cells"`
	Issues     []string `json:"issues"`
}

// requiredByType vrací klíče, které by měly být na řádku vyplněné (typová pravidla).
func requiredByType(typ string) []string {
	switch typ {
	case "narozeni":
		return []string{"dite_jmeno", "otec_jmeno_stav", "matka_jmeno_rodice"}
	case "oddani":
		return []string{"zenich_jmeno_stav_rodice", "nevesta_datum_narozeni"}
	case "umrti":
		return []string{"zemrely_jmeno_stav", "datum_umrti"}
	default:
		return nil // kombinovaná: příliš heterogenní
	}
}

// lintPage doplní rec.Lint deterministickými kontrolami.
func lintPage(schema *Schema, rec *StructuredRecord) {
	L := &rec.Lint
	if rec.RowsCount == 0 {
		L.Issues = append(L.Issues, "stránka bez záznamů (0 řádků)")
	}

	labelSet := map[string]bool{}
	for _, c := range schema.Columns {
		labelSet[strings.ToLower(c.Label)] = true
	}
	reqKeys := requiredByType(schema.Typ)

	missing := map[string]int{}
	prevCislo := -1
	noCislo, headerRows := 0, 0

	for _, row := range rec.Rows {
		empty, total, matchLabels := 0, 0, 0
		for _, c := range schema.Columns {
			v := strings.TrimSpace(row[c.Key])
			total++
			if v == "" {
				empty++
			} else if labelSet[strings.ToLower(v)] {
				matchLabels++
			}
		}
		L.EmptyCells += empty
		if total > 0 && matchLabels >= 3 && matchLabels*2 >= total {
			headerRows++
		}

		cs := strings.TrimSpace(row["cislo"])
		if cs == "" {
			noCislo++
		} else if n, ok := leadingInt(cs); ok {
			if prevCislo >= 0 && n <= prevCislo {
				L.Issues = append(L.Issues, fmt.Sprintf("čísla záznamů nejdou vzestupně (%d po %d)", n, prevCislo))
			}
			prevCislo = n
		}
		for _, k := range reqKeys {
			if strings.TrimSpace(row[k]) == "" {
				missing[k]++
			}
		}
	}

	if noCislo > 0 {
		L.Issues = append(L.Issues, fmt.Sprintf("%d řádků bez čísla", noCislo))
	}
	if headerRows > 0 {
		L.Issues = append(L.Issues, fmt.Sprintf("%d řádků vypadá jako hlavička (ne data)", headerRows))
	}
	for _, k := range sortedKeys(missing) {
		L.Issues = append(L.Issues, fmt.Sprintf("%d řádků bez povinného %q", missing[k], k))
	}
	L.OK = len(L.Issues) == 0
}

// --- Book-level report ---

type bookReport struct {
	JSONL       string            `json:"jsonl"`
	Pages       int               `json:"pages"`
	PagesOK     int               `json:"pages_ok"`
	PagesFailed int               `json:"pages_failed"`
	TotalRows   int               `json:"total_rows"`
	CisloMin    int               `json:"cislo_min"`
	CisloMax    int               `json:"cislo_max"`
	CisloGaps   []int             `json:"cislo_gaps"`
	CisloDups   []int             `json:"cislo_dups"`
	Fill        map[string]string `json:"fill"`
	PageIssues  []string          `json:"page_issues"`
}

// runReport přečte JSONL a vytvoří report.json + report.txt vedle něj.
func runReport(jsonlPath string) error {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return fmt.Errorf("otevření %s: %w", jsonlPath, err)
	}
	defer f.Close()

	rep := bookReport{JSONL: jsonlPath, CisloMin: -1, Fill: map[string]string{}}
	fillCount := map[string]int{}
	fillTotal := 0
	cisloSeen := map[int]int{}
	var cislos []int

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r StructuredRecord
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		rep.Pages++
		if !r.OK {
			rep.PagesFailed++
			rep.PageIssues = append(rep.PageIssues, r.File+": "+r.Error)
			continue
		}
		rep.PagesOK++
		rep.TotalRows += len(r.Rows)
		if !r.Lint.OK {
			rep.PageIssues = append(rep.PageIssues, r.File+": "+strings.Join(r.Lint.Issues, "; "))
		}
		for _, row := range r.Rows {
			fillTotal++
			for k, v := range row {
				if strings.TrimSpace(v) != "" {
					fillCount[k]++
				}
			}
			if n, ok := leadingInt(row["cislo"]); ok {
				cisloSeen[n]++
				cislos = append(cislos, n)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	// návaznost čísel záznamů
	if len(cislos) > 0 {
		sort.Ints(cislos)
		rep.CisloMin, rep.CisloMax = cislos[0], cislos[len(cislos)-1]
		for n := rep.CisloMin; n <= rep.CisloMax; n++ {
			if cisloSeen[n] == 0 {
				rep.CisloGaps = append(rep.CisloGaps, n)
			} else if cisloSeen[n] > 1 {
				rep.CisloDups = append(rep.CisloDups, n)
			}
		}
	}

	// vyplněnost sloupců
	for _, k := range sortedKeys(fillCount) {
		pct := 0
		if fillTotal > 0 {
			pct = fillCount[k] * 100 / fillTotal
		}
		rep.Fill[k] = fmt.Sprintf("%d/%d (%d%%)", fillCount[k], fillTotal, pct)
	}

	return writeReport(jsonlPath, rep, fillCount, fillTotal)
}

func writeReport(jsonlPath string, rep bookReport, fillCount map[string]int, fillTotal int) error {
	base := strings.TrimSuffix(jsonlPath, filepath.Ext(jsonlPath))
	// report.json
	jb, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(base+".report.json", append(jb, '\n'), 0o644); err != nil {
		return err
	}

	// report.txt (čitelný souhrn)
	var b strings.Builder
	fmt.Fprintf(&b, "Report: %s\n", filepath.Base(jsonlPath))
	fmt.Fprintf(&b, "Stránky: %d (ok %d, chyb %d)\n", rep.Pages, rep.PagesOK, rep.PagesFailed)
	fmt.Fprintf(&b, "Záznamů celkem: %d\n", rep.TotalRows)
	if rep.CisloMin >= 0 {
		fmt.Fprintf(&b, "Čísla záznamů: %d–%d, mezer %d, duplicit %d\n",
			rep.CisloMin, rep.CisloMax, len(rep.CisloGaps), len(rep.CisloDups))
		if len(rep.CisloGaps) > 0 {
			fmt.Fprintf(&b, "  chybějící čísla: %s\n", intsPreview(rep.CisloGaps, 40))
		}
		if len(rep.CisloDups) > 0 {
			fmt.Fprintf(&b, "  duplicitní čísla: %s\n", intsPreview(rep.CisloDups, 40))
		}
	}
	b.WriteString("Vyplněnost sloupců:\n")
	for _, k := range sortedKeys(fillCount) {
		pct := 0
		if fillTotal > 0 {
			pct = fillCount[k] * 100 / fillTotal
		}
		fmt.Fprintf(&b, "  %-28s %d/%d (%d%%)\n", k, fillCount[k], fillTotal, pct)
	}
	if len(rep.PageIssues) > 0 {
		fmt.Fprintf(&b, "Stránky s upozorněními (%d):\n", len(rep.PageIssues))
		for _, s := range rep.PageIssues {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}
	out := b.String()
	if err := os.WriteFile(base+".report.txt", []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, "\n"+out)
	fmt.Fprintf(os.Stderr, "Report uložen: %s(.json/.txt)\n", base+".report")
	return nil
}

// --- pomocné ---

func leadingInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(s[:i])
	return n, err == nil
}

func sortedKeys(m map[string]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func intsPreview(xs []int, max int) string {
	if len(xs) > max {
		parts := make([]string, max)
		for i := 0; i < max; i++ {
			parts[i] = strconv.Itoa(xs[i])
		}
		return strings.Join(parts, ", ") + fmt.Sprintf(", … (+%d)", len(xs)-max)
	}
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ", ")
}
