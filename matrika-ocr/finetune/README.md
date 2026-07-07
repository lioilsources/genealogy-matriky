# finetune/ — destilace Claude-OCR do Qwen2.5-VL-7B (DGX Spark)

Cíl: naučit self-hosted **Qwen2.5-VL-7B** číst matriky (kurent/latina/čeština) na
úrovni použitelné pro hromadné OCR, s Claudem jen jako teacherem/verifikátorem.

```
Claude (teacher) ─OCR→ ocr-out/*.jsonl ─export→ dataset/ ─Unsloth QLoRA→ adapter
      ▲ aktivní učení (špatné strany zpět teacherovi)          │ merge
      └──────────── eval_cer.py (CER/field) ◀── vLLM serve ◀───┘
```

> **Krok-za-krokem průvodce: [how-to.md](how-to.md).** Všechny kroky mají make cíle
> (`make help`); níže je referenční popis.

## Soubory
| Soubor | Účel |
|---|---|
| `Makefile` | make cíle pro celý workflow (venv/dataset/qa/push/train/serve/eval) |
| `how-to.md` | podrobný průvodce krok za krokem vč. troubleshootingu |
| `half_map.json` | mapa sloupec→půlka dvojstrany per typ/kniha, exclude, val split |
| `common.py` | prompt builder (1:1 s `schema.go`), půlení (1:1 se `split.go`), utility |
| `export_pages.py` | fáze A: JSONL GT → vzorky půlstran `{image, prompt, target}` |
| `export_rows.py` | fáze C: OpenCV detekce linek → řádkové ořezy (páruje jen při shodě počtu) |
| `train_unsloth.py` | QLoRA trénink (SPOUŠTĚT NA SPARKU) |
| `eval_cer.py` | CER / field-exact / JSON-validity proti vLLM endpointu (base vs. tuned) |
| `../..​/PREPIS-KONVENCE.md` | konvence teachera — číst před každou dávkou přepisů |

## 1) Export datasetu (na Macu)
```bash
cd matrika-ocr/finetune
make venv && make dataset      # → dataset/{pages,rows}_{train,val}.jsonl + QA výstupy
make qa                        # otevřít kontaktní archy + debug detekce
```
**QA:** projít `dataset/contact_sheets/` (target vedle obrázku) a `dataset/debug_rows/`
(detekce pásů). Splity jsou po knihách/foliích (`splits/*.txt`, žádný page-level leakage).

Formát vzorku (JSONL řádek):
```json
{"id":"…","kind":"page_half|row","image":"images/….jpg",
 "prompt":"…(identický s produkčním matrika-ocr + L/R hint)…",
 "target":"{\"folio\":…}","meta":{"book":…,"side":…,"typ":…}}
```

## 2) Trénink na Sparku (Unsloth QLoRA)
```bash
make push                    # [Mac] rsync dataset + skript na Spark
make dryrun                  # [Spark] 20 kroků — loss musí klesat
make train                   # [Spark] plný běh + merge bf16 pro vLLM
```
(Unsloth playbook container, viz https://build.nvidia.com/spark; make cíle samy
flushují UMA buffer cache mezi běhy.)
7B + QLoRA ≈ 20–30 GB z 128 GB UMA; ~1–2k vzorků × 2 epochy = řádově hodiny.
Loss se počítá jen na assistant tokenech; ViT zmrazený.

## 3) Nasazení + eval
```bash
make serve                                   # [Spark] vLLM s merged modelem
make eval-base URL=http://spark:8000/v1     # [Mac] baseline → report_base.json
make eval-tuned URL=http://spark:8000/v1    # [Mac] tuned → report_tuned.json
```
Nasadit jen pokud tuned **porazí base** na val CER/field-exact. Pak v `matrika-ocr`
přepnout URL/model a jet hromadné OCR žákem.

## 4) Aktivní učení (další kola)
1. Žák OCRuje neoznačené knihy → lint + nízké self-consistency vybere špatné strany.
2. Ty přepíše Claude (dle `PREPIS-KONVENCE.md`, `--keep-raw`) → přibudou do `ocr-out/`.
3. Re-export, re-train, re-eval. Cíl mixu: kurent a latina mají největší přínos
   (base Qwen je tam nejslabší) — viz plán vrstev v kořenovém plánu projektu.

## Stav datasetu (v0, 2026-07-07)
- Zdroj GT: Kladno-ev 01 [8386] (oddaní 1901–1929, ČJ kurent+latinka, 102 dvojstran),
  Pchery 12 [11341] (naroz[…] 1840, ČJ kurent, 2 dvojstrany). Generované JSONL vyřazeny.
- `pages_*`: 208 vzorků (train 180 / val 28), půlstrany 1280 px, target = sloupce půlky.
- `rows_*`: 441 vzorků (train 385 / val 56), pásy záznamů 1536 px; párování jen při
  shodě počtu pásů s GT (Kladno-ev 76 % půlstran; Pchery 12 layout 1840 zatím slabý).
- Chybí (fáze B): latina/němčina/starý kurent, negativní vzorky (prázdné strany,
  desky, indexy), více knih/ér — doplní teacher dávkami à ~50 skenů.

## Known limits / TODO
- `split.go` má L/R hinty natvrdo pro oddaní — při rozšíření o další typy přesunout
  hinty do `schemas/*.json`, ať drží prompt parity s exportem (tady už jsou per typ
  v `half_map.json`).
- Pchery 12 (éra 1840) má jemnější mřížku → doladit detekci (`min_h_frac`, prahy)
  než se přidají další staré knihy.
- Dataset NEpublikovat veřejně bez souhlasu SOA Praha (skeny ebadatelna = osobní
  badatelské užití).
