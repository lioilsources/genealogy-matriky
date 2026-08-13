package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// metaProbeDelay je pauza mezi lehkými info.json dotazy při stavbě
// scan_files (odděleně od uživatelského -delay, který platí mezi
// staženými STRÁNKAMI — viz downloadBook).
const metaProbeDelay = 150 * time.Millisecond

// downloadBook stáhne jednu knihu: nejdřív meta.json pro CELOU knihu
// (lehké info.json dotazy na všechny skeny, i mimo -start/-end/-pages —
// meta.json popisuje knihu, ne jen stahovaný rozsah), pak pixely pro
// rozsah start..end.
func downloadBook(client *http.Client, cfg config, id, obecID string) error {
	d, err := fetchDetail(client, id)
	if err != nil {
		return err
	}
	dir := filepath.Join(cfg.out, bookFolderName(d))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	totalAll := len(d.JP2Paths)
	rangeTotal := totalAll
	if cfg.pages > 0 && cfg.pages < rangeTotal {
		rangeTotal = cfg.pages
	}
	start := cfg.start
	if start < 1 {
		start = 1
	}
	end := cfg.end
	if end == 0 || end > rangeTotal {
		end = rangeTotal
	}

	fmt.Printf("Kniha %q (detail %s) — %d skenů celkem, typ=%s, stahuji %d..%d do %q\n",
		d.Name, id, totalAll, d.Typ, start, end, dir)

	scans := buildScanMeta(client, dir, id, d.JP2Paths)
	m := buildMeta(d, scans, obecID)
	if err := writeMetaJSON(dir, m); err != nil {
		fmt.Fprintf(os.Stderr, "VAROVÁNÍ: meta.json se nepodařilo zapsat: %v\n", err)
	} else {
		fmt.Printf("meta.json: typ=%s, %d skenů → %s\n", m.Typ, len(m.ScanFiles), filepath.Join(dir, "meta.json"))
	}

	if cfg.metaOnly {
		return nil
	}

	hashes := map[string]int{}
	saved := 0
	for n := start; n <= end; n++ {
		sf := scans[n-1]
		if sf.Width == 0 || sf.Height == 0 {
			fmt.Fprintf(os.Stderr, "[%d/%d] přeskakuji: neznámé rozměry (info.json selhalo)\n", n, end)
			continue
		}

		fname := fmt.Sprintf("%04d.jpg", n)
		fpath := filepath.Join(dir, fname)
		if fi, err := os.Stat(fpath); err == nil && fi.Size() > 0 {
			fmt.Printf("[%d/%d] přeskočeno (existuje %s)\n", n, end, fname)
			continue
		}

		data, err := stitch(client, sf.JP2, sf.Width, sf.Height, cfg.retries, cfg.quality)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%d/%d] CHYBA: %v\n", n, end, err)
			continue
		}
		if err := os.WriteFile(fpath, data, 0o644); err != nil {
			return err
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(data))
		if prev, dup := hashes[sum]; dup {
			fmt.Fprintf(os.Stderr, "[%d/%d] VAROVÁNÍ: %s je BYTE-IDENTICKÝ se skenem %d!\n", n, end, fname, prev)
		} else {
			hashes[sum] = n
		}

		if cfg.derive == "halves" {
			if err := deriveHalves(dir, n, data, cfg.quality); err != nil {
				fmt.Fprintf(os.Stderr, "[%d/%d] VAROVÁNÍ: -derive halves selhalo: %v\n", n, end, err)
			}
		}

		saved++
		fmt.Printf("[%d/%d] uloženo %s (%dx%d, %.1f MB)\n", n, end, fname, sf.Width, sf.Height, float64(len(data))/(1<<20))

		if cfg.delay > 0 && n < end {
			time.Sleep(cfg.delay)
		}
	}

	fmt.Printf("Hotovo: %d nových skenů, %d unikátních obrazů.\n", saved, len(hashes))
	return nil
}

// buildScanMeta sestaví scan_files pro celou knihu. Rozměry už jednou
// zapsané v existujícím meta.json (dir) se znovu nedotazují (resume).
func buildScanMeta(client *http.Client, dir, id string, jp2Paths []string) []scanFileMeta {
	cache := loadExistingScanCache(dir)
	scans := make([]scanFileMeta, len(jp2Paths))
	for i, jp2 := range jp2Paths {
		n := i + 1
		sf := scanFileMeta{Index: n, JP2: jp2, IIIF: iipsrvURL, ViewerURL: viewerURL(id, jp2)}
		if cached, ok := cache[jp2]; ok {
			sf.Width, sf.Height = cached.Width, cached.Height
		} else if info, err := imageInfo(client, jp2); err == nil {
			sf.Width, sf.Height = info.Width, info.Height
			time.Sleep(metaProbeDelay)
		} else {
			fmt.Fprintf(os.Stderr, "[meta %d/%d] CHYBA info.json: %v\n", n, len(jp2Paths), err)
		}
		scans[i] = sf
	}
	return scans
}

func viewerURL(detailID, jp2Path string) string {
	return fmt.Sprintf("%s/actapublica/matrika/detail/%s?image=%s", baseURL, detailID, jp2Path)
}

var reUnsafe = regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f]`)

func sanitize(name string) string {
	s := reUnsafe.ReplaceAllString(name, "-")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ".")
	if s == "" {
		s = "kniha"
	}
	return s
}

// --- HTTP pomocníci (stejný vzor jako root main.go: spoofnutý UA, lineární backoff) ---

func fetchText(client *http.Client, u string) (string, error) {
	b, err := fetchBytes(client, u)
	return string(b), err
}

func fetchBytes(client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("HTTP %d pro %s", resp.StatusCode, u)
	}
	return io.ReadAll(resp.Body)
}

func fetchBytesRetry(client *http.Client, u string, retries int) ([]byte, error) {
	if retries < 1 {
		retries = 1
	}
	var err error
	for attempt := 1; attempt <= retries; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		var b []byte
		if b, err = fetchBytes(client, u); err == nil {
			return b, nil
		}
	}
	return nil, err
}
