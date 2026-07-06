#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Přidá generaci NAD Josefem přes křestní záznam samotného Josefa
# (dítě = Josef, rodiče Matěj Vořechovský × Marie roz. Kyliešová).
# Josef-dítě má stejné jméno+místo (Volšany) jako existující uzel Josef (id1)
# z grandparent-prózy v Pchery 52 → sloučí se, žádný duplikát Karla nevzniká.
# Kašíkova (matčina) linie se do grafu nedává: dítě "Josefa Kašíková" by se
# nesloučilo s provdanou "Josefa Vořechovská" (id2) → jen zdokumentováno v paměti.
import json
OUT = "/Users/ol1n/Dev/GitHub/Genealogy/matrika-ocr/ocr-out/narozeni/Pchery 38 [11367].jsonl"
TS = "2026-07-06T12:00:00Z"

row = {
  "cislo": "", "datum_narozeni": "", "datum_krtu": "", "misto_dum": "Volšany č.7",
  "dite_jmeno": "Josef", "dite_pohlavi": "chlapec", "dite_manzelske": "manželské",
  "nabozenstvi": "katol.",
  "otec_jmeno_stav": "Vořechovský Matěj, havíř z Volšan č.7, syn Matěje Vořechovského, sedláka z Volšan č.7, a Roziny Nové z Hnidous č.4",
  "otec_bydliste": "Volšany č.7",
  "matka_jmeno_rodice": "Marie Vořechovská roz. Kyliešová ze Studňovsi č.17",
  "matka_bydliste": "Volšany č.7",
  "kmotri": "", "babka": "", "krtitel": "",
  "poznamenani": "Josef, otec Karla st. a Bedřicha; jeho rodiče doloženi ze záznamů dětí v Pchery 38 (Olšany)"
}
rec = {"file": "0011.jpg", "typ": "narozeni", "folio": "", "rok": "",
       "ok": True, "model": "claude-max", "attempts": 1, "ts": TS,
       "rows_count": 1, "lint": {"ok": True, "empty_cells": 0, "issues": []},
       "rows": [row]}
with open(OUT, "w", encoding="utf-8") as f:
    f.write(json.dumps(rec, ensure_ascii=False) + "\n")
print("zapsáno 1 záznam ->", OUT)
