package main

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// openDB otevře (a případně založí) SQLite databázi a aplikuje schéma.
// Schéma je psané jako CREATE IF NOT EXISTS, takže je bezpečné ho pouštět
// při každém startu.
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// modernc/sqlite je in-process; víc spojení znamená jen zámky navíc
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
