# RESUME — Max OCR knihy 8386 (Kladno-ev 01, oddaní)

## Stav
- Hotovo: skeny **0006–0084** (79 stran, ~216 sňatků), roky **1901–1926** (kompletní).
- Výstup: `ocr-out/oddani/Kladno-ev 01 [8386].jsonl` (append, resume dle `file`).
- CSV/strom/Pages se přegenerují z JSONL na konci.
- **Pokračovat od skenu `0085.jpg`** (folio 79, rok 1927).
- Kniha jde do ~skenu **0108** (rok 1929).

## Postup (pro Claude/Max v novém okně)
1. `Read` sken(y) z `../Kladno-ev 01 [8386]/00NN.jpg`.
2. Přepsat ručně psané záznamy do symetrického schématu (viz `schemas/oddani.json`).
3. Append přes Python heredoc (jeden JSON objekt = jeden sken) do JSONL:
   pole záznamu: file, typ="oddani", folio, rok, ok=true, rows_count, lint, ts, rows[].
   klíče řádku: cislo, jmeno_oddavajiciho, misto_oddavek, datum_ohlasek, datum_oddavek,
   zenich_jmeno_stav_rodice, zenich_misto_prebyvani, zenich_nabozenstvi,
   zenich_misto_narozeni, zenich_datum_narozeni, zenich_stav,
   nevesta_jmeno_stav_rodice, nevesta_misto_prebyvani, nevesta_nabozenstvi,
   nevesta_misto_narozeni, nevesta_datum_narozeni, nevesta_stav, svedkove, poznamenani.
   (Osvědčené helpery `rec()`/`r()` s 19 poz. argumenty — viz dřívější dávky v transcriptu.)
4. Po dávce: `./matrika-ocr --mode relint --in "../Kladno-ev 01 [8386]" --out "ocr-out/oddani/Kladno-ev 01 [8386].jsonl"`.

## Na konci (úkol od uživatele)
- Vygenerovat **CSV** (plochý, 1 řádek = 1 sňatek, vč. typ/folio/rok) a uložit do
  složky knihy `Kladno-ev 01 [8386]/oddani.csv`.

## Specifika knihy
- Čísla záznamů se **restartují každý rok** → duplicity/nevzestupnost čísel v reportu = OK.
- Oddávající většinou **Ladislav Funda, čbr. ev. farář**, místo **Husova kaple, Kladno**;
  náboženství **českobratr. evang.** Občas zástup (Kubát in subsidium) nebo ohlášky i v Rakovníku.
- Sken = dvojstrana (vlevo ženich, vpravo nevěsta); ~1–3 záznamy/stranu.
- Vyskytují se: vložené soudní dokumenty (prohlášení za mrtvého, **rozvody/rozluky** —
  dopsané i o 10+ let později, viz č.2/1923 Košař, č.8/1924 Ryba) → zapsat do `poznamenani`,
  prázdné/přenesené řádky vynechat.
- Kvalita: jména osob + hlavní obce spolehlivé; drobná rodiště/čísla domů mají místy nejistotu.
