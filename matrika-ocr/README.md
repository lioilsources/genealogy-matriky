# matrika-ocr

Go nástroj (jen standardní knihovna), který posílá naskenované stránky matrik
do OCR modelu (Qwen, OpenAI-kompatibilní `/v1/chat/completions` na Sparku) a
sbírá výstup do **JSONL** (řádek na sken). Dva režimy:

- **`structured`** (výchozí) — z tištěné hlavičky dané **schéma sloupců**, model
  vrací ručně psané záznamy jako JSON řádky; nástroj je normalizuje, ověří
  **lintem** a uloží do `ocr-out/<typ>/<kniha>.jsonl`.
- **`transcribe`** — prostý přepis celé stránky do pole `text`.
- **`report`** — jen přepočítá report nad hotovým JSONL.

Navrženo tak, aby backend (1 GPU / 1 request) nepřetěžovalo: sekvenčně
(`--concurrency 1`), pauza mezi requesty, retry s backoffem, resume.

## Předpoklady

- **Přeložit na Macu** (Spark nemá Go): `cd matrika-ocr && go build -o matrika-ocr .`
- Strukturovaný režim potřebuje ve složce knihy **`meta.json`** (typ N/O/Z).
  Vytvoří ho downloader při stažení, nebo dodatečně:
  `../ebadatelna-dl --meta-only --in "../Kladno-ev 01 [8386]"`.
- **`schemas/<typ>.json`** — šablony sloupců (`narozeni`, `oddani`, `umrti`).
  Nástroj vybere schéma podle `meta.typ`; kombinované knihy dostanou superset.

## Použití

```sh
# strukturovaná extrakce celé knihy (schéma dle meta.typ) → ocr-out/oddani/…jsonl
make ocr IN="../Kladno-ev 01 [8386]"

# jen rozsah stran 4–6 (ať to nejede na všech)
make ocr IN="../Kladno-ev 01 [8386]" START=4 END=6

# rychlý test na 2 strany
make test IN="../Kladno-ev 01 [8386]"

# přepočítat report nad hotovým JSONL
make report OUT="ocr-out/oddani/Kladno-ev 01 [8386].jsonl"

# prostý přepis (bez struktury)
make ocr MODE=transcribe IN="../Kladno-ev 01 [8386]" OUT=prepis.jsonl
```

Přerušení `Ctrl+C` dokončí rozdělaný request a skončí čistě; opětovné spuštění
se stejným výstupem **naváže** (resume přeskočí `ok:true`).

## Výstup (structured)

`ocr-out/<typ>/<kniha>.jsonl`, řádek = 1 sken:
```json
{"file":"0006.jpg","typ":"oddani","folio":"4","rok":"1901","ok":true,"rows_count":3,
 "rows":[{"cislo":"1","zenich_jmeno_stav_rodice":"Slavík František…","nevesta_datum_narozeni":"30.9.1878", "...":"..."}],
 "lint":{"ok":true,"empty_cells":2,"issues":[]},
 "raw":"…surová odpověď modelu (--keep-raw)…",
 "model":"ocr","prompt_tokens":16512,"completion_tokens":734,"duration_ms":58200,"attempts":1,"ts":"…"}
```
Vedle JSONL vznikne `…report.json` a `…report.txt`: počet záznamů, **návaznost
čísel záznamů** (mezery/duplicity napříč knihou), vyplněnost sloupců, stránky
s upozorněními.

Pole `typ` (u kombinovaných navíc `record_type` na řádku) říká, jaké hrany
záznam dává při stavbě stromu (narození = dítě↔rodiče, oddaní = manžel↔manželka,
úmrtí = koncové datum).

## Lint (deterministický)

Per stránka: počet řádků, prázdné buňky, řádky bez čísla, čísla mimo pořadí,
„řádek vypadá jako hlavička", a typová pravidla (narození → otec+matka+dítě;
oddaní → oba snoubenci; úmrtí → zemřelý+datum). Cíl: upozornit, kde ověřit
ručně — OCR ručně psané češtiny (kurent) není 100%.

## Důležité flagy

| flag | default | význam |
|------|---------|--------|
| `--in` | — | složka se skeny (structured potřebuje i `meta.json`) |
| `--mode` | `structured` | `structured` / `transcribe` / `report` |
| `--out` | auto | structured: `ocr-out/<typ>/<kniha>.jsonl`; jinak `out.jsonl` |
| `--schema` | dle meta | override cesty ke schématu |
| `--start` / `--end` | 1 / 0 | rozsah stran (1-based, včetně) |
| `--limit` | 0 | max N stran (test) |
| `--keep-raw` | true | ukládat surovou odpověď modelu |
| `--base-url` | `http://192.168.88.66:8080` | OCR gateway (LAN, bez CF timeoutu) |
| `--model` | `ocr` | název modelu (pre-flight ověří, že je Qwen) |
| `--delay` / `--timeout` / `--retries` | 1500ms / 180s / 3 | tempo a odolnost |
| `--max-side` | 0 | volitelný downscale delší strany (0 = vypnuto) |
| `--force` | false | obejít pre-flight kontrolu ne-Qwen modelu |

## Poznámky

- Schémata `narozeni`/`umrti` jsou zatím **drafty** (`"status":"draft-k-potvrzeni"`)
  podle standardního formuláře — doladit labely proti reálné knize daného typu;
  `oddani` je ověřené z knihy 8386.
- Do `oddani` přibyl sloupec **`nevesta_jmeno_stav_rodice`** (jméno, stav a
  rodiče nevěsty) — bez něj nejde nevěsta automaticky spárovat s křestním
  záznamem při stavbě stromu. Knihy vytěžené starším schématem znovu vytěž
  (resume přeskakuje `ok:true`, takže starý JSONL smaž nebo změň `--out`).
- **TIFF**: stdlib Go umí jen JPEG/PNG; TIFF převeď předem na JPEG.
- Model vrací volný text (u structured JSON objekt); nástroj z něj vytáhne první
  vyvážený `{…}` (umí i ```` ```json ```` fence), při chybě zkusí content-retry
  a v nouzi uloží `raw` + lint chybu.
