package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

// setupPipeline provede ingest+extract+match nad testovací vesnicí Testov.
func setupPipeline(t *testing.T) *sql.DB {
	t.Helper()
	db, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, book := range []string{"Testov N [9001]", "Testov O [9002]", "Testov Z [9003]"} {
		if err := ingestFile(db, filepath.Join("testdata", book+".jsonl"), filepath.Join("testdata", "books")); err != nil {
			t.Fatalf("ingest %s: %v", book, err)
		}
	}
	if err := runExtract(db, ""); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if err := runMatch(db, matchParams{Auto: 0.90, Flag: 0.72, Suggest: 0.50}); err != nil {
		t.Fatalf("match: %v", err)
	}
	return db
}

func count(t *testing.T, db *sql.DB, query string, params ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, params...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestPipelineIngest(t *testing.T) {
	db := setupPipeline(t)
	if n := count(t, db, `SELECT COUNT(*) FROM books`); n != 3 {
		t.Errorf("books = %d, chci 3", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM scans`); n != 4 {
		t.Errorf("scans = %d, chci 4", n)
	}
	// rows_count: 2+1 narozeni, 1 oddani, 2 umrti = 6
	if n := count(t, db, `SELECT COUNT(*) FROM records`); n != 6 {
		t.Errorf("records = %d, chci 6", n)
	}
}

func TestPipelineExtract(t *testing.T) {
	db := setupPipeline(t)
	if n := count(t, db, `SELECT COUNT(*) FROM events WHERE type='birth' AND date_precision='day'`); n != 3 {
		t.Errorf("birth events s denní přesností = %d, chci 3", n)
	}
	// oddací záznam: zenich + jeho rodiče + nevesta + její rodiče + 2 svědci = 8
	if n := count(t, db, `SELECT COUNT(*) FROM mentions m JOIN records r ON r.id=m.record_id
		WHERE r.record_type='oddani'`); n != 8 {
		t.Errorf("mentions z oddacího záznamu = %d, chci 8", n)
	}
	// nevěsta má explicitní rok narození z DOB
	var by int
	db.QueryRow(`SELECT birth_year FROM mentions WHERE role='nevesta'`).Scan(&by)
	if by != 1878 {
		t.Errorf("nevesta birth_year = %d, chci 1878", by)
	}
	// zemřelý 72 let v 1903 → narozen ~1831
	db.QueryRow(`SELECT birth_year FROM mentions WHERE role='zemrely' AND surname_norm='slavik'`).Scan(&by)
	if by != 1831 {
		t.Errorf("zemrely birth_year = %d, chci 1831", by)
	}
}

// clusterRoles vrátí role zmínek osoby, ke které patří zmínka dané role+příjmení.
func clusterRoles(t *testing.T, db *sql.DB, role, surname string) map[string]bool {
	t.Helper()
	rows, err := db.Query(`SELECT m2.role FROM mentions m
		JOIN person_mentions pm ON pm.mention_id = m.id
		JOIN person_mentions pm2 ON pm2.person_id = pm.person_id
		JOIN mentions m2 ON m2.id = pm2.mention_id
		WHERE m.role=? AND m.surname_norm=?`, role, surname)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	roles := map[string]bool{}
	for rows.Next() {
		var r string
		rows.Scan(&r)
		roles[r] = true
	}
	return roles
}

func TestPipelineMatch(t *testing.T) {
	db := setupPipeline(t)

	// ženich František Slavík (oddani 1901) = dítě František (narozeni 1876):
	// jména + cross-check rodičů (Josef Slavík + Marie roz. Dvořáková)
	roles := clusterRoles(t, db, "zenich", "slavik")
	if !roles["dite"] {
		t.Errorf("ženich Slavík se nespojil s křestním záznamem; role v clusteru: %v", roles)
	}

	// nevěsta Anna Dvořáková = dítě Anna (DOB 30.9.1878 přesně + rodiče Václav+Kateřina)
	roles = clusterRoles(t, db, "nevesta", "dvorak")
	if !roles["dite"] {
		t.Errorf("nevěsta Dvořáková se nespojila s křestním záznamem; role: %v", roles)
	}

	// otec Josef Slavík (křest 1876) = ženichův otec (1901) = zemřelý (1903)
	roles = clusterRoles(t, db, "zemrely", "slavik")
	if !roles["otec"] || !roles["zenich_otec"] {
		t.Errorf("Josef Slavík (zemřelý) se nespojil s rolemi otce; role: %v", roles)
	}

	// osoba Františka má hranu spouse na osobu Anny
	var frantisek, anna int64
	db.QueryRow(`SELECT pm.person_id FROM mentions m JOIN person_mentions pm ON pm.mention_id=m.id
		WHERE m.role='zenich'`).Scan(&frantisek)
	db.QueryRow(`SELECT pm.person_id FROM mentions m JOIN person_mentions pm ON pm.mention_id=m.id
		WHERE m.role='nevesta'`).Scan(&anna)
	if n := count(t, db, `SELECT COUNT(*) FROM relations WHERE type='spouse' AND
		((person_a=? AND person_b=?) OR (person_a=? AND person_b=?))`, frantisek, anna, anna, frantisek); n == 0 {
		t.Errorf("chybí spouse hrana František(%d)—Anna(%d)", frantisek, anna)
	}

	// dva různí zemřelí se nesloučili
	if n := count(t, db, `SELECT COUNT(DISTINCT pm.person_id) FROM mentions m
		JOIN person_mentions pm ON pm.mention_id=m.id WHERE m.role='zemrely'`); n != 2 {
		t.Errorf("zemřelí clusterů = %d, chci 2", n)
	}
}

// dumpMatch sesbírá obsah persons/person_mentions/relations pro porovnání běhů.
func dumpMatch(t *testing.T, db *sql.DB) string {
	t.Helper()
	out := ""
	for _, q := range []string{
		`SELECT id, display_name, sex, COALESCE(birth_year_est,0), COALESCE(death_year_est,0), ROUND(confidence,4) FROM persons ORDER BY id`,
		`SELECT person_id, mention_id, source FROM person_mentions ORDER BY mention_id`,
		`SELECT type, person_a, person_b, ROUND(confidence,4) FROM relations ORDER BY type, person_a, person_b`,
	} {
		rows, err := db.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			rows.Scan(ptrs...)
			out += fmt.Sprintln(vals...)
		}
		rows.Close()
	}
	return out
}

func TestMatchDeterminism(t *testing.T) {
	db := setupPipeline(t)
	first := dumpMatch(t, db)
	if err := runMatch(db, matchParams{Auto: 0.90, Flag: 0.72, Suggest: 0.50}); err != nil {
		t.Fatal(err)
	}
	second := dumpMatch(t, db)
	if first != second {
		t.Errorf("match není deterministický:\n--- 1. běh ---\n%s--- 2. běh ---\n%s", first, second)
	}
}

func TestConstraintsSurviveRematch(t *testing.T) {
	db := setupPipeline(t)

	// split: ženichova zmínka nesmí být tatáž osoba jako křestní dítě
	var zenichM, diteM int64
	db.QueryRow(`SELECT id FROM mentions WHERE role='zenich'`).Scan(&zenichM)
	db.QueryRow(`SELECT m.id FROM mentions m JOIN records r ON r.id=m.record_id
		WHERE m.role='dite' AND m.surname_norm='slavik'`).Scan(&diteM)
	if zenichM == 0 || diteM == 0 {
		t.Fatal("nenašel jsem zmínky pro constraint test")
	}
	if _, err := db.Exec(`INSERT INTO constraints(kind,mention_a,mention_b) VALUES('cannot_link',?,?)`, zenichM, diteM); err != nil {
		t.Fatal(err)
	}
	if err := runMatch(db, matchParams{Auto: 0.90, Flag: 0.72, Suggest: 0.50}); err != nil {
		t.Fatal(err)
	}
	var pa, pb int64
	db.QueryRow(`SELECT person_id FROM person_mentions WHERE mention_id=?`, zenichM).Scan(&pa)
	db.QueryRow(`SELECT person_id FROM person_mentions WHERE mention_id=?`, diteM).Scan(&pb)
	if pa == pb {
		t.Errorf("cannot_link ignorován: obě zmínky u osoby %d", pa)
	}

	// must_link: spoj dva zemřelé (nesmysl, ale ověřuje mechaniku) a zkontroluj persistenci
	var z1, z2 int64
	db.QueryRow(`SELECT MIN(id) FROM mentions WHERE role='zemrely'`).Scan(&z1)
	db.QueryRow(`SELECT MAX(id) FROM mentions WHERE role='zemrely'`).Scan(&z2)
	if _, err := db.Exec(`INSERT INTO constraints(kind,mention_a,mention_b) VALUES('must_link',?,?)`, z1, z2); err != nil {
		t.Fatal(err)
	}
	if err := runMatch(db, matchParams{Auto: 0.90, Flag: 0.72, Suggest: 0.50}); err != nil {
		t.Fatal(err)
	}
	db.QueryRow(`SELECT person_id FROM person_mentions WHERE mention_id=?`, z1).Scan(&pa)
	db.QueryRow(`SELECT person_id FROM person_mentions WHERE mention_id=?`, z2).Scan(&pb)
	if pa != pb {
		t.Errorf("must_link ignorován: osoby %d a %d", pa, pb)
	}
}

func TestPersonIDStability(t *testing.T) {
	db := setupPipeline(t)
	var before int64
	db.QueryRow(`SELECT pm.person_id FROM mentions m JOIN person_mentions pm ON pm.mention_id=m.id
		WHERE m.role='zenich'`).Scan(&before)
	if err := runMatch(db, matchParams{Auto: 0.90, Flag: 0.72, Suggest: 0.50}); err != nil {
		t.Fatal(err)
	}
	var after int64
	db.QueryRow(`SELECT pm.person_id FROM mentions m JOIN person_mentions pm ON pm.mention_id=m.id
		WHERE m.role='zenich'`).Scan(&after)
	if before != after {
		t.Errorf("person id se změnilo mezi běhy: %d → %d", before, after)
	}
}

func TestCellCorrectionSurvivesExtract(t *testing.T) {
	db := setupPipeline(t)
	var recID int64
	db.QueryRow(`SELECT record_id FROM mentions WHERE role='zemrely' AND surname_norm='benes'`).Scan(&recID)
	if recID == 0 {
		t.Fatal("nenašel jsem záznam Beneše")
	}
	// oprava OCR: špatně přečtené příjmení
	if _, err := db.Exec(`INSERT INTO cell_corrections(record_id,cell_key,corrected_value)
		VALUES(?,?,?)`, recID, "zemrely_jmeno_stav", "Beneš Vít, nádeník v Testově, syn Tomáše Beneše"); err != nil {
		t.Fatal(err)
	}
	if err := runExtract(db, ""); err != nil {
		t.Fatal(err)
	}
	// nová zmínka otce zemřelého vznikla z opravené buňky
	if n := count(t, db, `SELECT COUNT(*) FROM mentions WHERE record_id=? AND role='zemrely_otec'`, recID); n != 1 {
		t.Errorf("oprava buňky se nepromítla do extrakce")
	}
	// a přežije další re-run extractu
	if err := runExtract(db, ""); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM mentions WHERE record_id=? AND role='zemrely_otec'`, recID); n != 1 {
		t.Errorf("oprava buňky nepřežila re-extract")
	}
}
