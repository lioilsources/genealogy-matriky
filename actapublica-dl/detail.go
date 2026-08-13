package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// bookDetail je obsah stránky detailu jedné knihy: jp2 cesty (pořadí skenů)
// + metadata.
type bookDetail struct {
	DetailID       string
	BookNo         string
	Name           string // krátký název fondu/farnosti (z <title>)
	Typ            string
	RecordRanges   map[string]*string // narozeni/oddani/umrti/rejstrik -> "1785-1949" | nil
	District       string
	Provenance     string
	ProvenanceType string
	Volume         string
	Binding        string
	Language       string
	ProvenanceNote string
	Sheets         int
	Localities     []string
	Note           string
	JP2Paths       []string // pořadí = pořadí skenů (z CreateSeadragon, NE z čísel v názvu)
}

func detailURL(id string) string {
	return fmt.Sprintf("%s/actapublica/matrika/detail/%s", baseURL, id)
}

var (
	// jp2 cesty jsou v Deepzoom=<path>.jp2.dzi uvnitř JS pole CreateSeadragon(...),
	// s escapovanými lomítky (\/) — ověřeno v návrhu (viz README, sekce "Co je ověřeno").
	reDeepzoom = regexp.MustCompile(`Deepzoom=([^"']+\.jp2)\.dzi`)
	reTitleTag = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reYearSpan = regexp.MustCompile(`\d{4}\s*[-–]\s*\d{4}`)
)

// fetchDetail stáhne stránku detailu (http.Client sám následuje 302 —
// "nutné následovat 302" z návrhu platí pro nástroje jako curl -I, ne pro
// http.Client s výchozím nastavením) a vrátí jp2 cesty + metadata.
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
	d.Name = extractTitle(htmlStr)
	parseDetailMeta(htmlStr, d)
	d.Typ = detectTyp(d.RecordRanges)
	return d, nil
}

func extractTitle(htmlStr string) string {
	m := reTitleTag.FindStringSubmatch(htmlStr)
	if m == nil {
		return ""
	}
	t := cleanText(m[1])
	for _, sep := range []string{" | ", " — ", " – ", " - "} {
		if i := strings.Index(t, sep); i > 0 {
			t = t[:i]
		}
	}
	return strings.TrimSpace(t)
}

// parseDetailMeta doplní popisná metadata z volného textu stránky.
//
// POZOR: přesná struktura stránky Acta Publica nebyla v této relaci ověřena
// naživo (síť je odsud blokovaná Cloudflare, browser extension nepřipojen).
// Jde o obecný label/value scraper s vícejazyčnými/pravopisnými aliasy —
// degraduje na prázdná pole (typ=unknown), nikdy na chybu. Po prvním
// skutečném běhu (`make list`/`make download`) je potřeba zkontrolovat, jestli
// labely níže odpovídají realitě, a případně je doplnit/opravit.
func parseDetailMeta(htmlStr string, d *bookDetail) {
	d.District = findLabel(htmlStr, "Okres", "District")
	d.Provenance = findLabel(htmlStr, "Původce", "Provenance", "Fond")
	d.ProvenanceType = findLabel(htmlStr, "Druh původce", "Typ původce")
	d.Volume = findLabel(htmlStr, "Svazek", "Volume")
	d.Binding = findLabel(htmlStr, "Vazba", "Binding")
	d.Language = findLabel(htmlStr, "Jazyk", "Language")
	d.ProvenanceNote = findLabel(htmlStr, "Poznámka k původci")
	d.BookNo = findLabel(htmlStr, "Signatura", "Číslo knihy", "Sign.")
	d.Note = findLabel(htmlStr, "Obecný popis", "Poznámka", "General description")
	if s := findLabel(htmlStr, "Počet listů", "Number of sheets"); s != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			d.Sheets = n
		}
	}
	if loc := findLabel(htmlStr, "Lokality", "Obce", "Localities"); loc != "" {
		for _, l := range strings.Split(loc, ",") {
			if l = strings.TrimSpace(l); l != "" {
				d.Localities = append(d.Localities, l)
			}
		}
	}

	d.RecordRanges["narozeni"] = findYearRangeNear(htmlStr, "Narození", "Narozeni", "Křest", "Baptism", "Birth")
	d.RecordRanges["oddani"] = findYearRangeNear(htmlStr, "Oddaní", "Oddani", "Sňatky", "Marriage")
	d.RecordRanges["umrti"] = findYearRangeNear(htmlStr, "Úmrtí", "Umrti", "Zemřelí", "Death")
	d.RecordRanges["rejstrik"] = findYearRangeNear(htmlStr, "Rejstřík", "Rejstrik", "Index")
}

// detectTyp určí typ podle vyplněných rozsahů — stejné pravidlo jako
// detectTyp v root meta.go (počet vyplněných N/O/Z), rozšířené o "rejstrik":
// když N/O/Z nejsou vyplněné vůbec, ale je vyplněný index-rozsah, typ je
// "rejstrik" místo "unknown" (viz README — rejstříky patří do transcribe
// režimu OCR, ne do strukturované extrakce).
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

// findLabel hledá v HTML jeden z labelů následovaný hodnotou v nejbližším
// dalším tagu (toleruje libovolný počet uzavíracích tagů mezi labelem a
// hodnotou). Vrací první nalezenou neprázdnou shodu.
func findLabel(htmlStr string, labels ...string) string {
	for _, label := range labels {
		re := regexp.MustCompile(`(?is)` + regexp.QuoteMeta(label) + `\s*:?\s*(?:</[a-zA-Z0-9]+>\s*)*<[^>]+>\s*([^<]{1,300}?)\s*<`)
		if m := re.FindStringSubmatch(htmlStr); m != nil {
			if v := cleanText(m[1]); v != "" {
				return v
			}
		}
	}
	return ""
}

// findYearRangeNear hledá letopočtový rozsah ("1785-1949") v okolí výskytu
// některého z labelů — proximity heuristika, odolnější vůči neznámému
// značkování než přesná pozice tagu.
func findYearRangeNear(htmlStr string, labels ...string) *string {
	lower := strings.ToLower(htmlStr)
	for _, label := range labels {
		idx := strings.Index(lower, strings.ToLower(label))
		if idx < 0 {
			continue
		}
		end := idx + 400
		if end > len(htmlStr) {
			end = len(htmlStr)
		}
		if m := reYearSpan.FindString(cleanText(htmlStr[idx:end])); m != "" {
			v := strings.Join(strings.Fields(m), "")
			v = strings.ReplaceAll(v, "–", "-")
			return &v
		}
	}
	return nil
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

// bookDisplayName je "<název> <číslo knihy>" — použito pro Meta.Name i pro
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
		Sheets:         d.Sheets,
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
