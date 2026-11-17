# Matriky: stahovač + OCR extrakce + rodokmen

<!-- Max 80 řádků. Co se nevejde, jde do docs/ a sem jeden odkaz. Veřejné repo: žádné LAN IP, domovské cesty, ID. -->

## Účel
Nástroje pro matriky ze SOA Praha (ebadatelna): stažení skenů, OCR do JSONL,
SQLite databáze osob a vazeb a webový rodokmen (lokálně i read-only na GitHub Pages).

## Build a test
```
make build                          # root: ./ebadatelna-dl
cd matrika-ocr && make build        # OCR nástroj (gateway: BASEURL=)
cd genealogy && make build && make test   # go test ./...
cd web && npm ci && npm run build   # tsc -b && vite build
```
Pipeline: `ebadatelna-dl` → `matrika-ocr` → `genealogy ingest/extract/match` → `genealogy serve` + web.

## Mapa
- `main.go`, `meta.go`, `Makefile` — stahovač `ebadatelna-dl` (Go modul v rootu).
- `matrika-ocr/` — OCR přes gateway, schémata sloupců (`schemas/`), lint; vlastní Go modul.
- `matrika-ocr/finetune/` — Python: destilace Claude-OCR do Qwen2.5-VL (průvodce `how-to.md`).
- `matrika-ocr/ocr-out/` — ground truth JSONL (ignorováno, vybrané soubory force-add).
- `genealogy/` — Go: ingest/extract/match/serve/export do SQLite; testy `*_test.go`.
- `web/` — React + Vite; statická verze přes `VITE_STATIC`, data v `web/public/data/`.
- `.github/workflows/pages.yml` — deploy statického webu na GitHub Pages.
- `PREPIS-KONVENCE.md` — konvence přepisu matrik (dodržovat u každé dávky OCR).
- `RODOKMEN-VORECHOVSKY.md` — badatelský přehled rodu.

## Invarianty
- Stažené skeny, `meta.json`, OCR výstupy a databáze se neverzují (viz `.gitignore`).
- Přepisy dodržují `PREPIS-KONVENCE.md`: nekonzistence = šum v datasetu.

## Kam dál
README.md, `matrika-ocr/README.md`, `genealogy/README.md`, `matrika-ocr/finetune/how-to.md`.
