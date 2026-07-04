# RESUME — Max OCR knihy 8386 (Kladno-ev 01, oddaní) — ✅ HOTOVO

## Stav: DOKONČENO
- Přepsáno **celé**: skeny 0006–0107 = **102 stran, 293 sňatků**, roky **1901–1929**.
- Kniha končí č.24/1929 (folio 99, sken 0107); 0108 = zadní deska.
- Výstup: `ocr-out/oddani/Kladno-ev 01 [8386].jsonl`.
- CSV: `../Kladno-ev 01 [8386]/oddani.csv`. Strom+Pages: přegenerováno z JSONL.

## Vořechovští v této knize
- **č.5/1929: Vořechovský Karel** (kotlář, nar. 5.12.1903 Pchery č.4), syn **Karla
  Vořechovského** (horník, Pchery č.4) a **Marie roz. Bílkové**; sňatek 13.4.1929
  s Františkou Boženou Škarbanovou. Svědek **František Vořechovský** (Pchery č.82).
- Jiní Vořechovští v knize 8386 nejsou. Rod je z **Pcher** → další stopy hledat
  v knihách Pchery 01–12 (zatím nepřepsané) a v knihách N/Z Kladno-ev.

## Další knihy k přepisu (nepřepsané, jen skeny)
- Kladno-ev 02–04, Kladno-oú 13, Pchery 01–12.

## Postup (osvědčený)
1. `Read` sken(y) z `../<kniha>/00NN.jpg`.
2. Přepsat do schématu (viz `schemas/<typ>.json`), Python heredoc s helpery `rec()`/`r()`.
3. Po dávce `./matrika-ocr --mode relint --in "../<kniha>" --out <jsonl>`.
4. Na konci: CSV + `genealogy` pipeline (ingest→extract→match→export) + commit + push (Pages).
