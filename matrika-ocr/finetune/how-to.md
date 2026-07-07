# How-to: fine-tuning Qwen2.5-VL-7B na matriky, krok za krokem

Kompletní průvodce od OCR ground truth po nasazený vyladěný model. Rozdělení rolí:
**Mac** = příprava dat + eval (řídí se odsud), **DGX Spark** = trénink + serving.
Rychlý přehled cílů: `make help`. Co je co: `README.md`. Konvence přepisu:
`../../PREPIS-KONVENCE.md`.

```
[Mac] ocr-out/*.jsonl ──make dataset──▶ dataset/ ──make push──▶ [Spark]
[Spark] make dryrun → make train → make serve (vLLM)
[Mac] make eval-base / eval-tuned → porovnat → nasadit do matrika-ocr
```

---

## Krok 0 — předpoklady

**Mac:**
- Python 3.10+ (`python3 --version`)
- stažené knihy matrik v rootu repa (`Kniha [ID]/*.jpg`) a ground truth
  v `matrika-ocr/ocr-out/*/*.jsonl` (vyrábí `matrika-ocr` s modelem Claude)

**DGX Spark:**
- ssh přístup (`ssh ol1n@spark`), Docker s NVIDIA runtime
- NVIDIA DGX Spark playbooky: https://build.nvidia.com/spark
  (potřebuješ „Unsloth" playbook pro trénink a „vLLM" pro serving)

**Jednou nastav proměnné** (nebo je předávej na příkazové řádce):
```bash
# v Makefile uprav SPARK (ssh cíl), DEST, URL — nebo:
make push SPARK=ol1n@192.168.1.42 DEST='~/matriky-ft'
```

---

## Krok 1 — [Mac] příprava prostředí

```bash
cd matrika-ocr/finetune
make venv
```
Vytvoří `.venv` s pillow/opencv/numpy. Kdykoli jde smazat a vyrobit znovu.

---

## Krok 2 — [Mac] export datasetu

```bash
make dataset          # = make pages + make rows
```

Co se stane:
- **pages** (`export_pages.py`): každá přepsaná dvojstrana z `ocr-out` se rozpůlí
  na L/P (stejné poměry jako produkční `--split lr`), zmenší na 1280 px a dostane
  target = JSON se sloupci té půlky (mapování `half_map.json`). Prompt je **totožný
  s produkčním matrika-ocr** — train musí == inference.
- **rows** (`export_rows.py`): OpenCV najde vodorovné linky tištěné tabulky,
  vyřeže pásy jednotlivých záznamů a spáruje je s řádky GT — **jen když počet
  pásů přesně sedí** na počet řádků v GT (jinak se stránka zahodí; tím se
  párování hlídá samo, bez ruční anotace).

Očekávaný výstup (v0):
```
export_pages: 2 knih, 104 dvojstran → 208 vzorků (train=180, val=28)
export_rows — Kladno-ev 01: ~76 % půlstran spárováno, ~440 řádkových vzorků
```

Struktura `dataset/`:
```
images/            půlstrany + řádkové ořezy (jpg)
pages_train.jsonl  pages_val.jsonl   rows_train.jsonl   rows_val.jsonl
splits/            manifesty splitů (id + sha256 zdrojového skenu)
contact_sheets/    QA: obrázek + target vedle sebe
debug_rows/        QA: vizualizace detekce linek (modré=linky, červené=pásy)
```

---

## Krok 3 — [Mac] vizuální kontrola (NEPŘESKAKOVAT)

```bash
make qa    # otevře contact_sheets/ a debug_rows/
```

Kontroluj:
1. **Kontaktní archy**: sedí přepis vpravo na rukopis vlevo? (zvlášť folio, rok, jména)
2. **Debug detekce**: červené rámečky = přesně záznamy (žádná hlavička/titul navíc)?
3. Náhodně 2–3 řádkové ořezy z `images/*_r*.jpg` proti jejich targetu v JSONL —
   posunuté párování (off-by-one) by trénink otrávilo.

Když něco nesedí → oprav `half_map.json` (mapa sloupců, hinty) nebo prahy detekce
v `export_rows.py` a `make dataset` znovu.

---

## Krok 4 — [Mac→Spark] přenos

```bash
make push                      # rsync dataset/ + train_unsloth.py na Spark
```

---

## Krok 5 — [Spark] prostředí Unsloth

Podle NVIDIA playbooku (https://build.nvidia.com/spark, sekce Unsloth) spusť
kontejner s Unsloth pro ARM64/GB10, např.:
```bash
ssh ol1n@spark
cd ~/matriky-ft
# dle playbooku (image se může lišit verzí):
docker run --gpus all -it --rm -v $PWD:/work -w /work nvcr.io/nvidia/pytorch:latest bash
pip install unsloth trl pillow          # uvnitř kontejneru, pokud image nemá
```
> Mezi trénovacími běhy vždy: `sync && sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches'`
> (128GB UMA sdílí CPU+GPU; zaplněná buffer cache = OOM). Make cíle to dělají samy.

---

## Krok 6 — [Spark] dry-run (ověření, že se model učí)

```bash
make dryrun        # 20 kroků, ~minuty
```

Kontroluj: **loss v logu klesá** (z ~1.5+ znatelně dolů) a běh nespadl na OOM.
- OOM → sniž `--batch` na 1, zvyš `--grad-accum` na 8 (`train_unsloth.py`).
- Loss neklesá → zkontroluj dataset (krok 3), případně zvyš `--max-steps 50`.

---

## Krok 7 — [Spark] plný trénink + merge

```bash
make train         # 2 epochy nad ~650 vzorky + merge bf16 → out/qwen-matriky-lora-merged
```
Řádově hodiny. Výstupy:
- `out/qwen-matriky-lora/` — LoRA adapter (malý, verzuj/zálohuj)
- `out/qwen-matriky-lora-merged/` — bf16 model pro vLLM

---

## Krok 8 — [Spark] serving

```bash
make serve         # vllm serve out/...-merged --served-model-name qwen-matriky
```
Endpoint: `http://spark:8000/v1` (OpenAI-kompatibilní).

---

## Krok 9 — [Mac] eval: base vs. tuned

Nejdřív baseline (dokud vLLM servíruje ZÁKLADNÍ model), pak tuned:
```bash
make eval-base URL=http://spark:8000/v1          # → report_base.json
make eval-tuned URL=http://spark:8000/v1         # → report_tuned.json
```
(Rychlá sonda: `LIMIT=20`.) Metriky v reportu:
- `json_valid_%` — kolik odpovědí je parsovatelný JSON
- `rowcount_match_%` — souhlasí počet záznamů na stránce
- `CER_neprazdnych` — character error rate neprázdných polí (menší = lepší)
- `field_exact_%` — přesná shoda polí

**Nasazuj jen když tuned > base** (hlavně CER a field_exact). Pak v `matrika-ocr`
přepni endpoint/model na `qwen-matriky` a pusť hromadné OCR žákem.

---

## Krok 10 — další kola (aktivní učení)

1. Žák (tuned Qwen) OCRuje neoznačené knihy → `matrika-ocr --mode structured`.
2. Lint + podezřelé stránky (nevalidní JSON, prázdná povinná pole) → přepíše
   **Claude** dle `PREPIS-KONVENCE.md` (s `--keep-raw`) → nové GT v `ocr-out/`.
3. `make dataset && make push` → `make train` → `make eval-tuned` → porovnej.
4. Prioritně přidávej **kurent a latinu** (staré knihy Pchery 01–07) — tam je
   base model nejslabší a přínos největší.

---

## Troubleshooting

| Problém | Řešení |
|---|---|
| `make pages` — „kniha nenalezena" | složka `Kniha [ID]/` musí být v rootu repa, název = název JSONL |
| málo spárovaných řádků (`make rows`) | koukni do `debug_rows/`; uprav prahy v `detect_row_bands()` (min_h_frac, coverage 0.30) |
| OOM na Sparku | flush cache (dělá make sám), `--batch 1 --grad-accum 8`, zavřít ostatní kontejnery |
| eval: samé JSON chyby u base | normální — base model fence/kecá; tuned se učí čistý JSON z targetů |
| vLLM nevidí merged model | cesta musí být NA Sparku; `--served-model-name` = to, co dáváš do `--model` |
| nové knihy jiného typu/layoutu | doplň typ/knihu do `half_map.json` (sloupce L/P + hinty) a ověř kontaktními archy |
