package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// bookDetail je obsah stránky detailu jedné knihy: jp2 cesty (pořadí skenů)
// + metadata. Pole jsou parsovaná ze dvou ověřených částí stránky (živé
// měření na obec_id=2787):
//   - <div id="matrika-header"> — číslo knihy, původce, hlavní i indexové
//     rozsahy N/O/Z (čistý markup, žádné vnořené tabulky)
//   - technická tabulka (#pill_data_extra) — vazba/jazyk/poznámka/svazek/
//     typ původce a seznam obcí s okresem (odkazy na hledani_obec?obec_id=)
type bookDetail struct {
	DetailID       string
	BookNo         string
	Name           string // = Původce, použito i jako název složky
	Typ            string
	RecordRanges   map[string]*string // narozeni/oddani/umrti/rejstrik -> "1785-1949" | nil
	District       string
	Provenance     string
	ProvenanceType string
	ObecID         string // obec_id první lokality (fallback, když uživatel nezadal -obec)
	Volume         string
	Binding        string
	Language       string
	ProvenanceNote string
	Localities     []string
	Note           string
	JP2Paths       []string // pořadí = pořadí skenů (z CreateSeadragon, NE z čísel v názvu)
}

func detailURL(id string) string {
	return fmt.Sprintf("%s/actapublica/matrika/detail/%s", baseURL, id)
}

var (
	// jp2 cesty jsou v Deepzoom=<path>.jp2.dzi uvnitř JS pole CreateSeadragon(...),
	// s escapovanými lomítky (\/).
	reDeepzoom = regexp.MustCompile(`Deepzoom=([^"']+\.jp2)\.dzi`)

	// <span class="small font-italic">LABEL</span><br><span class="font-weight-bolder">VALUE</span>
	reHeaderField = func(label string) *regexp.Regexp {
		return regexp.MustCompile(`(?is)small font-italic">\s*` + regexp.QuoteMeta(label) +
			`\s*</span>\s*<br>\s*<span class="font-weight-bolder">(.*?)</span>`)
	}
	// "LABEL:" <span class="d-inline-block" ...>VALUE</span> — hlavní i indexové N/O/Z v hlavičce.
	reHeaderRange = func(label string) *regexp.Regexp {
		return regexp.MustCompile(`(?is)` + regexp.QuoteMeta(label) + `:\s*<span class="d-inline-block"[^>]*>\s*(.*?)\s*</span>`)
	}
	// jednoduché řádky technické tabulky: <td class="table-item-label">LABEL</td><td class="table-item-value...">VALUE</td>
	reTableField = func(label string) *regexp.Regexp {
		return regexp.MustCompile(`(?is)<td class="table-item-label">\s*` + regexp.QuoteMeta(label) +
			`\s*</td>\s*<td class="table-item-value[^"]*">(.*?)</td>`)
	}
	// Typ původce je vedlejší hodnota (<small><em>) v řádku Původce/Typ původce.
	reProvenanceType = regexp.MustCompile(`(?is)Typ původce.*?</td>\s*<td class="table-item-value[^"]*">.*?<br>\s*<small><em>(.*?)</em></small>`)
	// obce a jiné lokality: <a href=".../hledani_obec?obec_id=N" ...>Název (aliasy)[, obec: X], okres: Y</a>
	reLocalityLink = regexp.MustCompile(`(?is)hledani_obec\?obec_id=(\d+)"[^>]*>(.*?)</a>`)
	reOkresSuffix  = regexp.MustCompile(`(?i)okres:\s*(.+)$`)
)

// fetchDetail stáhne stránku detailu (http.Client sám následuje 302) a vrátí
// jp2 cesty + metadata.
func fetchDetail(client *http.Client, id string) (*bookDetail, error) {
	htmlStr, err := fetchText(client, detailURL(id))
	if err != nil {
		return nil, fmt.Errorf("detail %s: %w", id, err)
	}
	d := &bookDetail{
		DetailID:     id,
		RecordRanges: map[string]*string{"narozeni": nil, "oddani": nil, "umrti": nil, "rejstrik": nil},
	}
	for _, m := range reDeepzoom.FindAllStringSubmatch(htmlStr, -1) {
		d.JP2Paths = append(d.JP2Paths, strings.ReplaceAll(m[1], `\/`, "/"))
	}
	if len(d.JP2Paths) == 0 {
		return nil, fmt.Errorf("detail %s: v HTML nenalezen žádný Deepzoom .jp2 odkaz (CreateSeadragon) — "+
			"stránka možná změnila strukturu, zkontroluj ručně %s", id, detailURL(id))
	}
	parseDetailMeta(htmlStr, d)
	d.Typ = detectTyp(d.RecordRanges)
	return d, nil
}

func parseDetailMeta(htmlStr string, d *bookDetail) {
	d.BookNo = headerField(htmlStr, "Číslo knihy")
	d.Provenance = headerField(htmlStr, "Původce")
	d.Name = d.Provenance

	d.RecordRanges["narozeni"] = optionalRange(headerRange(htmlStr, "Narození"))
	d.RecordRanges["oddani"] = optionalRange(headerRange(htmlStr, "Oddaní"))
	d.RecordRanges["umrti"] = optionalRange(headerRange(htmlStr, "Zemřelí"))
	idxNarozeni := headerRange(htmlStr, "Index narození")
	idxOddani := headerRange(htmlStr, "Index oddaní")
	idxZemreli := headerRange(htmlStr, "Index zemřelí")
	if r := optionalRange(idxNarozeni); r != nil {
		d.RecordRanges["rejstrik"] = r
	} else if r := optionalRange(idxOddani); r != nil {
		d.RecordRanges["rejstrik"] = r
	} else if r := optionalRange(idxZemreli); r != nil {
		d.RecordRanges["rejstrik"] = r
	}

	d.Volume = tableField(htmlStr, "Číslo svazku")
	d.Binding = tableField(htmlStr, "Vazba")
	d.Language = tableField(htmlStr, "Jazyk")
	d.Note = tableField(htmlStr, "Poznámka")
	d.ProvenanceNote = tableField(htmlStr, "Poznámka o původci")
	if mm := reProvenanceType.FindStringSubmatch(htmlStr); mm != nil {
		d.ProvenanceType = cleanText(mm[1])
	}

	for _, lm := range reLocalityLink.FindAllStringSubmatch(htmlStr, -1) {
		if d.ObecID == "" {
			d.ObecID = lm[1]
		}
		text := cleanText(lm[2])
		name := text
		if om := reOkresSuffix.FindStringSubmatch(text); om != nil {
			if d.District == "" {
				d.District = strings.TrimSpace(om[1])
			}
			name = strings.TrimRight(text[:len(text)-len(om[0])], ", ")
		}
		d.Localities = append(d.Localities, name)
	}
}

func headerField(htmlStr, label string) string {
	if mm := reHeaderField(label).FindStringSubmatch(htmlStr); mm != nil {
		return cleanText(mm[1])
	}
	return ""
}

func headerRange(htmlStr, label string) string {
	if mm := reHeaderRange(label).FindStringSubmatch(htmlStr); mm != nil {
		return cleanText(mm[1])
	}
	return ""
}

func tableField(htmlStr, label string) string {
	if mm := reTableField(label).FindStringSubmatch(htmlStr); mm != nil {
		return cleanText(mm[1])
	}
	return ""
}

// optionalRange vrátí nil pro prázdnou hodnotu nebo "-" (žádný rozsah), jinak
// ukazatel na normalizovaný rozsah "YYYY-YYYY".
func optionalRange(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return nil
	}
	v := strings.Join(strings.Fields(s), " ")
	return &v
}

// detectTyp určí typ podle vyplněných rozsahů — stejné pravidlo jako
// detectTyp v root meta.go (počet vyplněných N/O/Z), rozšířené o "rejstrik":
// když N/O/Z nejsou vyplněné vůbec, ale je vyplněný index-rozsah, typ je
// "rejstrik" místo "unknown" (rejstříky patří do transcribe režimu OCR).
func detectTyp(ranges map[string]*string) string {
	var present []string
	for _, key := range []string{"narozeni", "oddani", "umrti"} {
		if ranges[key] != nil {
			present = append(present, key)
		}
	}
	switch len(present) {
	case 0:
		if ranges["rejstrik"] != nil {
			return "rejstrik"
		}
		return "unknown"
	case 1:
		return present[0]
	default:
		return "kombinovana"
	}
}

// --- Meta.json (nadmnožina root Meta — viz meta.go) ---

type scanFileMeta struct {
	Index     int    `json:"index"`
	JP2       string `json:"jp2"`
	IIIF      string `json:"iiif"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	ViewerURL string `json:"viewer_url,omitempty"`
}

type Meta struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Typ          string             `json:"typ"`
	RecordRanges map[string]*string `json:"record_ranges"`
	District     string             `json:"district,omitempty"`
	Provenance   string             `json:"provenance,omitempty"`
	Sheets       int                `json:"sheets,omitempty"`
	Scans        int                `json:"scans,omitempty"`
	Localities   []string           `json:"localities,omitempty"`
	Note         string             `json:"note,omitempty"`
	Source       string             `json:"source"`
	SchemaRef    string             `json:"schema_ref,omitempty"`

	Archive        string         `json:"archive"`
	ObecID         string         `json:"obec_id,omitempty"`
	BookNo         string         `json:"book_no,omitempty"`
	Volume         string         `json:"volume,omitempty"`
	ProvenanceType string         `json:"provenance_type,omitempty"`
	Binding        string         `json:"binding,omitempty"`
	Language       string         `json:"language,omitempty"`
	ProvenanceNote string         `json:"provenance_note,omitempty"`
	ScanFiles      []scanFileMeta `json:"scan_files,omitempty"`
}

// bookDisplayName je "<Původce> <číslo knihy>" — použito pro Meta.Name i pro
// bookFolderName (jen s přidaným "[<detail id>]").
func bookDisplayName(d *bookDetail) string {
	name := d.Name
	if name == "" {
		name = "kniha"
	}
	parts := []string{name}
	if d.BookNo != "" {
		parts = append(parts, d.BookNo)
	}
	return strings.Join(parts, " ")
}

func bookFolderName(d *bookDetail) string {
	return sanitize(bookDisplayName(d)) + " [" + d.DetailID + "]"
}

func buildMeta(d *bookDetail, scans []scanFileMeta, obecID string) *Meta {
	if obecID == "" {
		obecID = d.ObecID
	}
	return &Meta{
		ID:   d.DetailID,
		Name: bookDisplayName(d),
		Typ:  d.Typ,
		RecordRanges: map[string]*string{
			"narozeni": d.RecordRanges["narozeni"],
			"oddani":   d.RecordRanges["oddani"],
			"umrti":    d.RecordRanges["umrti"],
		},
		District:       d.District,
		Provenance:     d.Provenance,
		Scans:          len(scans),
		Localities:     d.Localities,
		Note:           d.Note,
		Source:         detailURL(d.DetailID),
		SchemaRef:      "schemas/" + d.Typ + ".json",
		Archive:        "mza-actapublica",
		ObecID:         obecID,
		BookNo:         d.BookNo,
		Volume:         d.Volume,
		ProvenanceType: d.ProvenanceType,
		Binding:        d.Binding,
		Language:       d.Language,
		ProvenanceNote: d.ProvenanceNote,
		ScanFiles:      scans,
	}
}

func writeMetaJSON(dir string, m *Meta) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), append(b, '\n'), 0o644)
}

// loadExistingScanCache načte rozměry skenů z už existujícího meta.json
// (resume — ať se info.json nedotazuje znovu na to, co už jednou zjistilo).
func loadExistingScanCache(dir string) map[string]scanFileMeta {
	cache := map[string]scanFileMeta{}
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return cache
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return cache
	}
	for _, sf := range m.ScanFiles {
		if sf.Width > 0 && sf.Height > 0 {
			cache[sf.JP2] = sf
		}
	}
	return cache
}
