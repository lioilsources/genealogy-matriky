# genealogy — strom a analytika nad OCR výstupy matrik

Go nástroj (stdlib + `modernc.org/sqlite`), který z JSONL výstupů
[`matrika-ocr`](../matrika-ocr/) staví **osoby, rodinné vazby a interaktivní
rodokmen** s webovým UI ([`web/`](../web/)). Čtyři subpříkazy = pipeline:

```
ingest  →  extract  →  match  →  serve
JSONL      zmínky       osoby     API + web UI
→ SQLite   + události   + vazby
```

## Použití

```sh
make test                                   # unit + integrační testy (fixture vesnice Testov)
make all JSONL="../matrika-ocr/ocr-out/*/*.jsonl" BOOKS=".."   # celý pipeline
make serve                                  # http://localhost:8090 (API + web UI z ../web/dist)
```

Nebo přímo:

```sh
./genealogy ingest  --db data/genealogy.db --books-root .. "ocr-out/oddani/Kladno-ev 01 [8386].jsonl"
./genealogy extract --db data/genealogy.db
./genealogy match   --db data/genealogy.db --auto 0.90 --flag 0.72 --suggest 0.50
./genealogy serve   --db data/genealogy.db --addr :8090 --web ../web/dist
```

## Datový model (SQLite, vrstvy)

```
Vrstva 0  books / scans / records        surové OCR řádky s proveniencí (kniha, sken, folio, řádek)
          cell_corrections               ruční opravy OCR buněk (užívatelovy, přežijí vše)
Vrstva 1  events / mentions              extrahované události a zmínky o osobách (rebuild: extract)
Vrstva 2  constraints                    must_link / cannot_link — JEDINÝ uživatelský vstup matcheru
Vrstva 3  persons / relations            osoby (clustery zmínek) a vazby (rebuild: match)
          match_candidates               návrhy „možná táž osoba" pro UI
```

Zásady:

- **Provenience všude** — každá zmínka nese knihu + sken + folio + řádek, v UI
  je jeden klik od původního skenu.
- **Opravy přežijí re-run.** `extract` i `match` svou vrstvu přestaví, ale
  `cell_corrections` a `constraints` jen čtou. Merge v UI = `must_link`,
  split = `cannot_link`; další `match` je respektuje.
- **Stabilní id zmínek i osob.** Zmínky se upsertují podle
  `(record_id, role, ordinal)`; osoby dědí id z minulého běhu podle největšího
  překryvu zmínek → URL a odkazy přežijí přepočet.
- **Determinismus:** stejná data + stejné prahy → bitově shodný výstup
  (testováno v `TestMatchDeterminism`).

## Extrakce (`extract`)

- Datumy: `30.9.1878`, `dne 30. září 1878`, měsíce cs/de/lat, přesnost
  day/month/year; věk (`72 let`, `6 měsíců`) → odhad roku narození.
- Kombinované buňky (`zenich_jmeno_stav_rodice`, `otec_jmeno_stav`,
  `matka_jmeno_rodice`) se rozpadnou na **více zmínek**: hlavní osoba + otec +
  matka (kotvy `syn/dcera`, `roz.`, lexikon povolání, genitiv → 1. pád).
- Normalizace jmen: fold diakritiky, varianty cs/de/lat
  (`seed/name_variants.csv`: Jan/Johann/Joannes…), ženské `-ová` → kmen,
  genitivy (`syn Josefa Slavíka` → Josef Slavík).

## Matching (`match`)

1. **Blocking**: prefix příjmení + pohlaví + dekáda věrohodného roku narození.
2. **Skóre dvojice**: podobnost jmen (kanonické varianty, jinak Jaro-Winkler)
   + datum narození (DOB nevěsty ↔ křest, věk zemřelého) + místo +
   **cross-check rodičů** (rodiče ženicha na oddacím ↔ otec+matka na křtu) −
   tvrdé zákazy (nesouhlas pohlaví, dvakrát narozen, událost po smrti).
3. **Clustering**: union-find; `must_link` vždy, kandidáti dle skóre
   (`--auto` jisté, `--flag` s příznakem nejistoty, `--suggest` jen návrh
   do UI), `cannot_link` blokuje.
4. Vazby: křest → rodič→dítě (+ prarodiče), oddavky → manželé + rodiče,
   úmrtí → koncové datum. Confidence hrany = min confidence osob.

## API (`serve`)

```
GET   /api/search?q=&surname=&place=&year_from=&year_to=
GET   /api/persons/{id}                        detail + zmínky + návrhy shod
GET   /api/persons/{id}/neighborhood?depth=2   podgraf pro strom (uzly+hrany+vrstvy)
GET   /api/records/{id}                        OCR buňky + opravy + odkaz na sken
GET   /api/scans/{id}/image?maxw=1600          náhled; /full = originál (deep-zoom)
PATCH /api/records/{id}/cells                  oprava buňky → re-extract knihy
POST  /api/persons/{a}/merge/{b}               merge → must_link + re-match
POST  /api/persons/{id}/split                  split → cannot_link + re-match
POST  /api/match/run                           přepočet
GET   /api/analytics/{kind}                    surnames|lifespan|marriage-age|
                                               seasonality|migration|family-size|timeline
GET   /api/stats
```

## Testy

`testdata/` obsahuje syntetickou vesnici **Testov** (3 knihy: N/O/Z, provázaná
rodina Slavík–Dvořák). Integrační testy pokrývají celý pipeline: počty po
ingestu, extrakci rolí, správné sloučení (ženich↔křest přes rodiče, nevěsta↔křest
přes DOB), determinismus, persistenci constraints a oprav buněk, stabilitu
person id.

## Statický export a GitHub Pages (`export`)

```sh
./genealogy export --db data/genealogy.db --out ../web/public/data
```

vygeneruje JSON snapshot (persons/graph/details/analytics/stats), který web UI
umí číst **bez backendu** — build s `VITE_STATIC=true` počítá vyhledávání, BFS
okolí i rodový strom v prohlížeči. Workflow `.github/workflows/pages.yml`
nasazuje statickou verzi na **GitHub Pages** při pushi do main (v repu zapni
Settings → Pages → Source: *GitHub Actions*).

Omezení statické verze: jen ke čtení (merge/split/opravy vyžadují lokální
`serve`), skeny se otevírají odkazem přímo v ebadatelně. **Pozor:** commitnutý
snapshot v `web/public/data/` je na Pages veřejný — exportuj jen data, která
chceš zveřejnit.

## Rodové zobrazení

`GET /api/tree?surname=Vořechovský` vrátí strom celého rodu: všechny osoby
nesoucí příjmení (vč. rozených a historických pravopisů — **Worechowsky,
Worechowski i ženská Vořechovská se normalizují na totéž**) plus jejich přímé
okolí (manželé, rodiče, děti). V UI je vstup „rod (příjmení)" — výchozí
**Vořechovský** — a nositelé jména jsou zvýrazněni, okolí ztlumené.

Normalizace pravopisu: w→v, adjektivní tvary -ská/-ské/-ského/-ski → -ský
(s ochranou proti jménům jako Růžička), ženské -ová/-ové → kmen.

## Poznámky

- Schéma `oddani` má nově sloupec `nevesta_jmeno_stav_rodice` — knihy
  vytěžené starším schématem (bez jména nevěsty) je potřeba **znovu vytěžit**
  (`matrika-ocr` resume přeskakuje hotové stránky, takže starý JSONL smaž
  nebo zvol jiný `--out`).
- LLM normalizační pass (Qwen) je připraven v datovém modelu (`llm_cache`),
  ale zatím se nepoužívá — rule-based parser pokrývá běžné zápisy.
