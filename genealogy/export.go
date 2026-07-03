package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// cmdExport vygeneruje statický read-only snapshot dat pro nasazení bez
// backendu (GitHub Pages apod.): sadu JSON souborů, které statický režim
// web UI čte místo /api. Mutace (merge/split/opravy) ve statické verzi nejsou;
// skeny se odkazují na ebadatelnu (viewer /d/{id}/{strana}).
func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	dbPath := fs.String("db", "data/genealogy.db", "cesta k SQLite databázi")
	out := fs.String("out", "../web/public/data", "výstupní složka pro JSON snapshot")
	fs.Parse(args)

	db, err := openDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}

	// persons.json — index pro vyhledávání a filtry (jména normalizovaná)
	type personIdx struct {
		ID           int64    `json:"id"`
		Name         string   `json:"name"`
		Sex          string   `json:"sex"`
		BirthYear    int      `json:"birth_year,omitempty"`
		DeathYear    int      `json:"death_year,omitempty"`
		Confidence   float64  `json:"confidence"`
		MentionCount int      `json:"mention_count"`
		SurnameNorms []string `json:"surname_norms"`
		GivenNorms   []string `json:"given_norms"`
		Places       []string `json:"places,omitempty"`
	}
	var persons []personIdx
	rows, err := db.Query(`SELECT p.id, p.display_name, p.sex, COALESCE(p.birth_year_est,0),
		COALESCE(p.death_year_est,0), p.confidence,
		(SELECT COUNT(*) FROM person_mentions pm WHERE pm.person_id=p.id)
		FROM persons p ORDER BY p.id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p personIdx
		if err := rows.Scan(&p.ID, &p.Name, &p.Sex, &p.BirthYear, &p.DeathYear, &p.Confidence, &p.MentionCount); err != nil {
			return err
		}
		persons = append(persons, p)
	}
	rows.Close()
	for i := range persons {
		p := &persons[i]
		nrows, err := db.Query(`SELECT DISTINCT m.surname_norm, m.maiden_norm, m.given_norm, COALESCE(m.place_text,'')
			FROM mentions m JOIN person_mentions pm ON pm.mention_id=m.id WHERE pm.person_id=?`, p.ID)
		if err != nil {
			return err
		}
		sn, gn, pl := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for nrows.Next() {
			var s, mdn, g, place string
			nrows.Scan(&s, &mdn, &g, &place)
			if s != "" {
				sn[s] = true
			}
			if mdn != "" {
				sn[mdn] = true
			}
			if g != "" {
				gn[g] = true
			}
			if place != "" {
				pl[placeNorm(place)] = true
			}
		}
		nrows.Close()
		p.SurnameNorms = sortedSet(sn)
		p.GivenNorms = sortedSet(gn)
		p.Places = sortedSet(pl)
	}
	if err := writeJSONFile(filepath.Join(*out, "persons.json"), persons); err != nil {
		return err
	}

	// graph.json — všechny hrany s vrstvami (klient dělá BFS sám)
	type edgeOut struct {
		ID         int64    `json:"id"`
		Type       string   `json:"type"`
		Source     int64    `json:"source"`
		Target     int64    `json:"target"`
		Confidence float64  `json:"confidence"`
		Layers     []string `json:"layers"`
	}
	rels, _, err := loadRelationsDB(db)
	if err != nil {
		return err
	}
	edges := []edgeOut{}
	for _, e := range rels {
		edges = append(edges, edgeOut{e.ID, e.Type, e.A, e.B, e.Conf, edgeLayers(db, e.Evid)})
	}
	if err := writeJSONFile(filepath.Join(*out, "graph.json"), map[string]any{"edges": edges}); err != nil {
		return err
	}

	// details.json — detail osoby (zmínky s proveniencí + odkaz na ebadatelnu)
	type candidateOut struct {
		PersonID   int64   `json:"person_id"`
		PersonName string  `json:"person_name"`
		Score      float64 `json:"score"`
	}
	type detailOut struct {
		ID         int64           `json:"id"`
		Name       string          `json:"name"`
		Sex        string          `json:"sex"`
		BirthYear  int             `json:"birth_year,omitempty"`
		DeathYear  int             `json:"death_year,omitempty"`
		Confidence float64         `json:"confidence"`
		Mentions   []mentionDetail `json:"mentions"`
		Candidates []candidateOut  `json:"candidates"`
	}
	details := map[string]detailOut{}
	for _, p := range persons {
		d := detailOut{ID: p.ID, Name: p.Name, Sex: p.Sex, BirthYear: p.BirthYear,
			DeathYear: p.DeathYear, Confidence: p.Confidence, Candidates: []candidateOut{}}
		ms, err := queryMentions(db, `JOIN person_mentions pm2 ON pm2.mention_id = m.id AND pm2.person_id = ?
			ORDER BY COALESCE(e.year, 0), m.id`, p.ID)
		if err != nil {
			return err
		}
		for i := range ms {
			ms[i].ScanURL = ebadatelnaURL(ms[i].BookID, ms[i].ScanFile)
		}
		d.Mentions = ms

		// pozor: SetMaxOpenConns(1) — nejdřív kurzor dočíst a zavřít,
		// jména dotahovat až pak (vnořený dotaz by se zablokoval)
		crows, err := db.Query(`SELECT DISTINCT
			CASE WHEN pma.person_id = ? THEN pmb.person_id ELSE pma.person_id END, mc.score
			FROM match_candidates mc
			JOIN person_mentions pma ON pma.mention_id = mc.mention_a
			JOIN person_mentions pmb ON pmb.mention_id = mc.mention_b
			WHERE mc.accepted = 0 AND (pma.person_id = ? OR pmb.person_id = ?)
			ORDER BY mc.score DESC LIMIT 20`, p.ID, p.ID, p.ID)
		if err != nil {
			return err
		}
		for crows.Next() {
			var c candidateOut
			crows.Scan(&c.PersonID, &c.Score)
			if c.PersonID != p.ID {
				d.Candidates = append(d.Candidates, c)
			}
		}
		crows.Close()
		for i := range d.Candidates {
			db.QueryRow(`SELECT display_name FROM persons WHERE id=?`, d.Candidates[i].PersonID).
				Scan(&d.Candidates[i].PersonName)
		}
		details[strconv.FormatInt(p.ID, 10)] = d
	}
	if err := writeJSONFile(filepath.Join(*out, "details.json"), details); err != nil {
		return err
	}

	// analytics.json — všechny grafy najednou
	analytics := map[string]any{}
	for kind, q := range analyticsQueries {
		data, err := queryRows(db, q)
		if err != nil {
			return fmt.Errorf("analytika %s: %w", kind, err)
		}
		analytics[kind] = data
	}
	if err := writeJSONFile(filepath.Join(*out, "analytics.json"), analytics); err != nil {
		return err
	}

	// stats.json
	stats := map[string]any{"static": true}
	for name, q := range map[string]string{
		"books": `SELECT COUNT(*) FROM books`, "scans": `SELECT COUNT(*) FROM scans`,
		"records": `SELECT COUNT(*) FROM records`, "mentions": `SELECT COUNT(*) FROM mentions`,
		"persons": `SELECT COUNT(*) FROM persons`, "relations": `SELECT COUNT(*) FROM relations`,
	} {
		var n int
		db.QueryRow(q).Scan(&n)
		stats[name] = n
	}
	if err := writeJSONFile(filepath.Join(*out, "stats.json"), stats); err != nil {
		return err
	}

	fmt.Printf("export: %d osob, %d hran → %s\n", len(persons), len(edges), *out)
	return nil
}

// ebadatelnaURL sestaví odkaz do prohlížeče ebadatelny na konkrétní sken
// ("0006.png" → strana 6). Prázdné, když nejde určit.
func ebadatelnaURL(bookID, scanFile string) string {
	base := strings.TrimSuffix(scanFile, filepath.Ext(scanFile))
	n, err := strconv.Atoi(strings.TrimLeft(base, "0"))
	if err != nil || bookID == "" {
		return ""
	}
	if _, err := strconv.Atoi(bookID); err != nil {
		return "" // testovací/lokální knihy bez číselného id
	}
	return fmt.Sprintf("https://ebadatelna.soapraha.cz/d/%s/%d", bookID, n)
}

func writeJSONFile(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// deterministické pořadí exportu
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// loadRelationsDB je varianta loadRelations nad *sql.DB (bez serveru).
func loadRelationsDB(db *sql.DB) ([]grel, map[int64][]grel, error) {
	s := &server{db: db}
	return s.loadRelations()
}
