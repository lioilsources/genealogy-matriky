package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// jsonlRecord odpovídá StructuredRecord z matrika-ocr (jen pole, která ingest potřebuje).
type jsonlRecord struct {
	File      string              `json:"file"`
	Typ       string              `json:"typ"`
	Folio     string              `json:"folio"`
	Rok       string              `json:"rok"`
	OK        bool                `json:"ok"`
	RowsCount int                 `json:"rows_count"`
	Rows      []map[string]string `json:"rows"`
	Lint      json.RawMessage     `json:"lint"`
}

// bookMeta je podmnožina meta.json z downloaderu.
type bookMeta struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Typ        string   `json:"typ"`
	District   string   `json:"district"`
	Localities []string `json:"localities"`
}

var reBookID = regexp.MustCompile(`\[(\d+)\]`)

func cmdIngest(args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	dbPath := fs.String("db", "data/genealogy.db", "cesta k SQLite databázi")
	booksRoot := fs.String("books-root", "..", "kořen se složkami knih (Nazev [ID]) kvůli meta.json a skenům")
	fs.Parse(args)
	if fs.NArg() == 0 {
		return fmt.Errorf("zadej aspoň jeden JSONL soubor (ocr-out/<typ>/<kniha>.jsonl)")
	}

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		return err
	}
	db, err := openDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	for _, pattern := range fs.Args() {
		paths, err := filepath.Glob(pattern)
		if err != nil || len(paths) == 0 {
			paths = []string{pattern}
		}
		for _, p := range paths {
			if err := ingestFile(db, p, *booksRoot); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
		}
	}
	return nil
}

// ingestFile nahraje jeden JSONL (jedna kniha) do books/scans/records.
// Opakovaný běh knihu přepíše (records/scans dané knihy smaže a založí znovu);
// odvozené vrstvy se přestaví následným extract/match.
func ingestFile(db *sql.DB, jsonlPath, booksRoot string) error {
	bookName := strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")
	bookID := ""
	if m := reBookID.FindStringSubmatch(bookName); m != nil {
		bookID = m[1]
	}
	if bookID == "" {
		bookID = bookName // fallback: bez [ID] v názvu použij celé jméno
	}

	// meta.json + složka skenů (nepovinné — bez nich jen nebude náhled skenu)
	scansDir := filepath.Join(booksRoot, bookName)
	meta := bookMeta{ID: bookID, Name: bookName}
	metaJSON := ""
	if b, err := os.ReadFile(filepath.Join(scansDir, "meta.json")); err == nil {
		metaJSON = string(b)
		json.Unmarshal(b, &meta)
	} else {
		scansDir = ""
	}

	f, err := os.Open(jsonlPath)
	if err != nil {
		return err
	}
	defer f.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// re-ingest: smaž odvozený obsah knihy (records → mentions/events padnou při extractu)
	if _, err := tx.Exec(`DELETE FROM records WHERE scan_id IN (SELECT id FROM scans WHERE book_id=?)`, bookID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM scans WHERE book_id=?`, bookID); err != nil {
		return err
	}

	locJSON, _ := json.Marshal(meta.Localities)
	if _, err := tx.Exec(`INSERT INTO books(id,name,typ,district,localities_json,meta_json,scans_dir)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, typ=excluded.typ,
			district=excluded.district, localities_json=excluded.localities_json,
			meta_json=excluded.meta_json, scans_dir=excluded.scans_dir`,
		bookID, meta.Name, orUnknown(meta.Typ), meta.District, string(locJSON), metaJSON, scansDir); err != nil {
		return err
	}

	scanStmt, _ := tx.Prepare(`INSERT INTO scans(book_id,file,folio,rok,ok,lint_json) VALUES(?,?,?,?,?,?)`)
	recStmt, _ := tx.Prepare(`INSERT INTO records(scan_id,row_idx,record_type,cislo,cells_json) VALUES(?,?,?,?,?)`)
	defer scanStmt.Close()
	defer recStmt.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	nScans, nRows, line := 0, 0, 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var jr jsonlRecord
		if err := json.Unmarshal([]byte(raw), &jr); err != nil {
			return fmt.Errorf("řádek %d: %w", line, err)
		}
		res, err := scanStmt.Exec(bookID, jr.File, jr.Folio, jr.Rok, boolInt(jr.OK), string(jr.Lint))
		if err != nil {
			return fmt.Errorf("řádek %d (%s): %w", line, jr.File, err)
		}
		scanID, _ := res.LastInsertId()
		nScans++
		for i, row := range jr.Rows {
			recType := jr.Typ
			if rt := row["record_type"]; rt != "" { // kombinované knihy
				recType = rt
			}
			cells, _ := json.Marshal(row)
			if _, err := recStmt.Exec(scanID, i, recType, row["cislo"], string(cells)); err != nil {
				return fmt.Errorf("řádek %d (%s) row %d: %w", line, jr.File, i, err)
			}
			nRows++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Printf("ingest %s [%s]: %d skenů, %d záznamů\n", meta.Name, bookID, nScans, nRows)
	return nil
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
