package main

import (
	"fmt"
	"os"
)

const usage = `genealogy — stavba rodokmenu nad výstupy matrika-ocr

Použití:
  genealogy ingest  --db data/genealogy.db [--books-root ..] soubor.jsonl [...]
  genealogy extract --db data/genealogy.db [--book ID]
  genealogy match   --db data/genealogy.db [--auto 0.90] [--flag 0.72] [--suggest 0.50]
  genealogy serve   --db data/genealogy.db [--addr :8090] [--web web/dist]

Pipeline: ingest (JSONL → raw záznamy) → extract (záznamy → osoby-zmínky a
události) → match (zmínky → osoby a vazby) → serve (API + web UI).
Extract i match jsou idempotentní: přestaví svou vrstvu, uživatelské opravy
(cell_corrections, constraints) zůstávají.`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "ingest":
		err = cmdIngest(os.Args[2:])
	case "extract":
		err = cmdExtract(os.Args[2:])
	case "match":
		err = cmdMatch(os.Args[2:])
	case "serve":
		err = cmdServe(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Println(usage)
	default:
		fmt.Fprintf(os.Stderr, "neznámý příkaz %q\n\n%s\n", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "chyba:", err)
		os.Exit(1)
	}
}
