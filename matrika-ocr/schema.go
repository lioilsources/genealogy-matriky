package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Column je jeden sloupec tištěné hlavičky matriky.
type Column struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Group    string `json:"group"`
	Required bool   `json:"required"`
}

// Schema popisuje strukturu jednoho typu matriky (sloupce hlavičky).
type Schema struct {
	Typ      string   `json:"typ"`
	Status   string   `json:"status"`
	Unit     string   `json:"unit"`
	PageMeta []string `json:"page_meta"`
	Columns  []Column `json:"columns"`
}

// BookMeta je podmnožina meta.json, kterou OCR potřebuje.
type BookMeta struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Typ       string `json:"typ"`
	SchemaRef string `json:"schema_ref"`
}

// loadBookMeta načte meta.json z adresáře knihy.
func loadBookMeta(dir string) (*BookMeta, error) {
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil, err
	}
	var m BookMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.Typ == "" {
		return nil, fmt.Errorf("meta.json nemá pole \"typ\"")
	}
	return &m, nil
}

// resolveSchema vrátí schéma: buď z explicitní cesty, nebo dle typu z meta.
// Pro typ "kombinovana" sestaví superset ze tří typových šablon.
func resolveSchema(path, typ string) (*Schema, error) {
	if path != "" {
		return loadSchemaFile(path)
	}
	if typ == "kombinovana" {
		return combinedSchema()
	}
	if typ == "unknown" || typ == "" {
		return nil, fmt.Errorf("typ knihy je neznámý — zadej schéma přes --schema")
	}
	f, err := findSchemaFile(typ + ".json")
	if err != nil {
		return nil, err
	}
	return loadSchemaFile(f)
}

func loadSchemaFile(path string) (*Schema, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("čtení schématu %s: %w", path, err)
	}
	var s Schema
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("neplatné schéma %s: %w", path, err)
	}
	if len(s.Columns) == 0 {
		return nil, fmt.Errorf("schéma %s nemá žádné sloupce", path)
	}
	return &s, nil
}

// findSchemaFile hledá schemas/<name> vedle binárky a v CWD.
func findSchemaFile(name string) (string, error) {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}
	for _, d := range dirs {
		p := filepath.Join(d, "schemas", name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("schéma schemas/%s nenalezeno (hledáno v %s)", name, strings.Join(dirs, ", "))
}

// combinedSchema sestaví superset schéma pro kombinovanou knihu:
// record_type + sjednocení sloupců N/O/Z (dedup dle key, pořadí N,O,Z).
func combinedSchema() (*Schema, error) {
	out := &Schema{
		Typ:      "kombinovana",
		Unit:     "dvojstrana",
		PageMeta: []string{"folio", "rok"},
		Columns: []Column{
			{Key: "record_type", Label: "Typ záznamu (narozeni/oddani/umrti)", Group: "", Required: true},
		},
	}
	seen := map[string]bool{"record_type": true}
	for _, typ := range []string{"narozeni", "oddani", "umrti"} {
		f, err := findSchemaFile(typ + ".json")
		if err != nil {
			return nil, err
		}
		s, err := loadSchemaFile(f)
		if err != nil {
			return nil, err
		}
		for _, c := range s.Columns {
			if seen[c.Key] {
				continue
			}
			seen[c.Key] = true
			if c.Group == "" {
				c.Group = typ
			}
			out.Columns = append(out.Columns, c)
		}
	}
	return out, nil
}

// buildStructuredPrompt sestaví prompt pro strukturovanou extrakci ze schématu.
func buildStructuredPrompt(s *Schema) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Toto je rozevřená dvojstrana matriky typu „%s\". ", typLabel(s.Typ))
	b.WriteString("Přepiš POUZE ručně psané záznamy, NE předtištěnou hlavičku a NE prázdné řádky. ")
	b.WriteString("Z horní hlavičky vypiš folio a rok. ")
	b.WriteString("Každý záznam (řádek) vrať jako JSON objekt s těmito klíči:\n")
	for _, c := range s.Columns {
		fmt.Fprintf(&b, "- %s: %s\n", c.Key, c.Label)
	}
	b.WriteString("Každou hodnotu vrať jako STRUČNÝ prostý řetězec (ne pole, ne vnořený objekt). ")
	b.WriteString("Prázdnou buňku vrať jako \"\". Zachovej pořadí záznamů shora dolů. ")
	b.WriteString("Neopakuj řádky a nevymýšlej data. ")
	b.WriteString("Vrať POUZE validní JSON tvaru: {\"folio\":\"\",\"rok\":\"\",\"rows\":[{…}]} — nic dalšího, bez komentářů a bez markdown fence.")
	return b.String()
}

func typLabel(typ string) string {
	switch typ {
	case "narozeni":
		return "kniha narozených"
	case "oddani":
		return "kniha oddaných"
	case "umrti":
		return "kniha zemřelých"
	case "kombinovana":
		return "kombinovaná matrika (narození/oddaní/úmrtí)"
	default:
		return typ
	}
}
