// Command actapublica-dl stahuje naskenované matriky z Acta Publica
// (https://www.mza.cz/actapublica/), digitálního archivu MZA Brno, do adresářové
// struktury "Nazev cislo [detail-id]/0001.jpg" — stejný kontrakt jako
// ebadatelna-dl (root main.go/meta.go), ale jiný archiv/stack (PHP +
// OpenSeadragon + IIPImage/IIIF místo Apache Wicket), proto samostatná
// utilita s vlastním go.mod.
//
// Obrázky nejde stáhnout přímo (FIF/JTL/CVT/obj vrací 403, get_image
// přesměruje na přihlášení) — jediná průchozí cesta je IIIF Image API
// přes iipsrv.fcgi, který navíc výstup stropuje na 2000px v každém
// rozměru. Nativní rozlišení se proto skládá z dlaždic ≤2000×2000 (viz
// iiif.go).
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	baseURL   = "https://www.mza.cz"
	iipsrvURL = baseURL + "/iipsrv/iipsrv.fcgi"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

type config struct {
	obec     string
	id       string
	list     bool
	out      string
	start    int
	end      int
	pages    int
	delay    time.Duration
	retries  int
	quality  int
	metaOnly bool
	derive   string
}

func main() {
	cfg := parseFlags()
	client := &http.Client{Timeout: 180 * time.Second}
	if err := run(client, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "chyba: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.obec, "obec", "", "obec_id — vylistuje/stáhne všechny knihy obce")
	flag.StringVar(&cfg.id, "id", "", "detail id jedné knihy (kombinuj s -obec pro stažení jen té jedné z výpisu)")
	flag.BoolVar(&cfg.list, "list", false, "jen vypsat knihy (vyžaduje -obec), nestahovat")
	flag.StringVar(&cfg.out, "out", ".", "kořenový výstupní adresář")
	flag.IntVar(&cfg.start, "start", 1, "první sken ke stažení")
	flag.IntVar(&cfg.end, "end", 0, "poslední sken ke stažení; 0 = do konce")
	flag.IntVar(&cfg.pages, "pages", 0, "kolik skenů stáhnout od -start; 0 = celá kniha")
	flag.DurationVar(&cfg.delay, "delay", 500*time.Millisecond, "pauza mezi skeny")
	flag.IntVar(&cfg.retries, "retries", 3, "počet opakování na dlaždici/požadavek")
	flag.IntVar(&cfg.quality, "quality", 92, "JPEG kvalita sešitého snímku")
	flag.BoolVar(&cfg.metaOnly, "meta-only", false, "jen zapsat meta.json (bez stahování obrázků)")
	flag.StringVar(&cfg.derive, "derive", "none", "OCR odvozeniny: none|halves")
	flag.Parse()
	return cfg
}

func run(client *http.Client, cfg config) error {
	if cfg.obec == "" && cfg.id == "" {
		return fmt.Errorf("zadej -obec <obec_id> nebo -id <detail id>")
	}
	if cfg.derive != "none" && cfg.derive != "halves" {
		return fmt.Errorf("-derive musí být 'none' nebo 'halves', ne %q", cfg.derive)
	}

	var ids []string
	if cfg.obec != "" {
		rows, meta, err := listBooks(client, cfg.obec)
		if err != nil {
			return fmt.Errorf("hledání obec_id=%s: %w", cfg.obec, err)
		}
		printSearchResults(cfg.obec, rows, meta)
		if cfg.list {
			return nil
		}
		if cfg.id != "" {
			ok := false
			for _, r := range rows {
				if r.DetailID == cfg.id {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("detail id=%s nebyl mezi výsledky obec_id=%s (zkus -list)", cfg.id, cfg.obec)
			}
			ids = []string{cfg.id}
		} else {
			for _, r := range rows {
				ids = append(ids, r.DetailID)
			}
		}
	} else {
		if cfg.list {
			return fmt.Errorf("-list vyžaduje -obec")
		}
		ids = []string{cfg.id}
	}

	var firstErr error
	for i, id := range ids {
		fmt.Printf("=== kniha %d/%d (detail %s) ===\n", i+1, len(ids), id)
		if err := downloadBook(client, cfg, id, cfg.obec); err != nil {
			fmt.Fprintf(os.Stderr, "kniha %s: CHYBA: %v\n", id, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
	}
	return firstErr
}
