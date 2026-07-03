package main

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Meta je per-kniha metadata (meta.json) získaná z /pages/MatrikaPage/matrikaId/{id}.
type Meta struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Typ          string             `json:"typ"` // narozeni|oddani|umrti|kombinovana|unknown
	RecordRanges map[string]*string `json:"record_ranges"`
	District     string             `json:"district,omitempty"`
	Provenance   string             `json:"provenance,omitempty"`
	Sheets       int                `json:"sheets,omitempty"`
	Scans        int                `json:"scans,omitempty"`
	Localities   []string           `json:"localities,omitempty"`
	Note         string             `json:"note,omitempty"`
	Source       string             `json:"source"`
	SchemaRef    string             `json:"schema_ref,omitempty"`
}

var (
	reMetaH1   = regexp.MustCompile(`tableMatrikaBasicInfoHeader1"[^>]*>\s*<div>([^<]+)</div>`)
	reMetaH2   = regexp.MustCompile(`tableMatrikaBasicInfoHeader2"[^>]*>\s*<div[^>]*>(?:<span>)?([^<]+)`)
	reYearSpan = regexp.MustCompile(`^\d{4}\s*-\s*\d{4}$`)
	reProvLink = regexp.MustCompile(`puvodceLink"[^>]*>\s*<span>([^<]+)`)
	reLocality = regexp.MustCompile(`obecCastLabel">([^<]+)`)
	// hodnota "Obecného popisu": za labelem následuje další description-span s textem
	reNote = regexp.MustCompile(`(?:General description|Obecný popis|Allgemeine Beschreibung)[^<]*</span>[\s\S]*?tableMatrikaDescriptionHeader1"[^>]*>\s*<span>([\s\S]*?)</span>`)

	// vícejazyčné labely (web může vracet cs/en/de)
	districtLabels = []string{"District", "Okres", "Bezirk"}
	sheetsLabels   = []string{"Number of sheets", "Počet listů", "Anzahl der Blätter"}
	typKeys        = []string{"narozeni", "oddani", "umrti"} // pořadí řádků N/O/Z na stránce
)

// metaSourceURL vrátí URL stránky s metadaty knihy.
func metaSourceURL(id string) string {
	return baseURL + "/pages/MatrikaPage/matrikaId/" + id
}

// doMeta stáhne MatrikaPage, naparsuje a zapíše meta.json do dir.
func doMeta(client *http.Client, cfg config, dir, name string, scans int) error {
	htmlStr, err := fetchText(client, metaSourceURL(cfg.id))
	if err != nil {
		return err
	}
	m := parseMeta(htmlStr, cfg.id, name, scans)
	wrote, err := writeMeta(dir, m, cfg.forceMeta)
	if err != nil {
		return err
	}
	if wrote {
		loc := ""
		if len(m.Localities) > 0 {
			loc = fmt.Sprintf(", %d lokalit", len(m.Localities))
		}
		fmt.Printf("meta.json: typ=%s, %d skenů%s → %s\n", m.Typ, m.Scans, loc, filepath.Join(dir, "meta.json"))
	} else {
		fmt.Printf("meta.json už existuje (ponechávám; -force-meta pro přepis)\n")
	}
	return nil
}

// parseMeta naparsuje HTML MatrikaPage do Meta. name a scans jsou z prohlížeče.
func parseMeta(htmlStr, id, name string, scans int) *Meta {
	m := &Meta{
		ID:           id,
		Name:         name,
		Scans:        scans,
		Source:       metaSourceURL(id),
		RecordRanges: map[string]*string{"narozeni": nil, "oddani": nil, "umrti": nil},
	}

	// řádky label -> hodnoty ze základní info tabulky
	type row struct {
		label  string
		values []string
	}
	var rows []row
	for _, seg := range strings.Split(htmlStr, "tableMatrikaBasicInfoRow")[1:] {
		lab := ""
		if mm := reMetaH1.FindStringSubmatch(seg); mm != nil {
			lab = cleanCell(mm[1])
		}
		var vals []string
		for _, vm := range reMetaH2.FindAllStringSubmatch(seg, -1) {
			vals = append(vals, cleanCell(vm[1]))
		}
		rows = append(rows, row{lab, vals})
	}

	// N/O/Z = první tři řádky se ≥2 hodnotami (register + index), v pořadí N,O,Z
	var multi []row
	for _, r := range rows {
		if len(r.values) >= 2 {
			multi = append(multi, r)
		}
	}
	for i, key := range typKeys {
		if i < len(multi) && reYearSpan.MatchString(multi[i].values[0]) {
			v := multi[i].values[0]
			m.RecordRanges[key] = &v
		}
	}
	m.Typ = detectTyp(m.RecordRanges)
	m.SchemaRef = "schemas/" + m.Typ + ".json"

	// bonus pole (vícejazyčné labely)
	for _, r := range rows {
		if len(r.values) == 0 {
			continue
		}
		switch {
		case matchLabel(r.label, districtLabels):
			m.District = r.values[0]
		case matchLabel(r.label, sheetsLabels):
			if n, err := strconv.Atoi(strings.TrimSpace(r.values[0])); err == nil {
				m.Sheets = n
			}
		}
	}

	// provenance (z puvodceLink), lokality, poznámka — přes třídy (jazykově nezávislé)
	if mm := reProvLink.FindStringSubmatch(htmlStr); mm != nil {
		m.Provenance = cleanCell(mm[1])
	}
	for _, lm := range reLocality.FindAllStringSubmatch(htmlStr, -1) {
		if loc := cleanCell(lm[1]); loc != "" {
			m.Localities = append(m.Localities, loc)
		}
	}
	if mm := reNote.FindStringSubmatch(htmlStr); mm != nil {
		m.Note = cleanCell(mm[1])
	}
	return m
}

// detectTyp určí typ podle počtu vyplněných rozsahů N/O/Z.
func detectTyp(ranges map[string]*string) string {
	var present []string
	for _, key := range typKeys {
		if ranges[key] != nil {
			present = append(present, key)
		}
	}
	switch len(present) {
	case 0:
		return "unknown"
	case 1:
		return present[0]
	default:
		return "kombinovana"
	}
}

func matchLabel(label string, want []string) bool {
	for _, w := range want {
		if strings.EqualFold(label, w) {
			return true
		}
	}
	return false
}

// cleanCell strhne tagy, unescapuje entity, srazí whitespace a ořízne pomlčky ("---").
func cleanCell(s string) string {
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ") // &nbsp;
	s = strings.Join(strings.Fields(s), " ") // srazí i nbsp (U+00A0)
	s = strings.Trim(s, " -")
	return s
}

// writeMeta zapíše meta.json do adresáře knihy (nepřepisuje, pokud force=false a existuje).
func writeMeta(dir string, m *Meta, force bool) (bool, error) {
	path := filepath.Join(dir, "meta.json")
	if !force {
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
			return false, nil // existuje, nepřepisuji
		}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// idFromFolder vytáhne ID z názvu složky "Nazev [12345]".
func idFromFolder(path string) string {
	base := filepath.Base(strings.TrimRight(path, string(os.PathSeparator)))
	if m := regexp.MustCompile(`\[(\d+)\]`).FindStringSubmatch(base); m != nil {
		return m[1]
	}
	return ""
}
