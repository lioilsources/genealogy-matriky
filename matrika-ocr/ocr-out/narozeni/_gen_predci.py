#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Přidá do stromu HLUBŠÍ generace + manželčiny (přivdané) větve:
#  - Kašíkova linie (rodiče Josefy Kašíkové): Václav Kašík × Josefa Svejkovská
#  - Matěj st. (Josef→otec Matěj→jeho otec Matěj st.) jako DÍTĚ Václava × Anny Podratzké
#    → přidá Václava, jeho otce Jakuba (Křivčice) a Anniny rodiče (Jakub Podratzký).
# Jména psána surname-first, ať parser dá rodné příjmení jako vlastní uzel.
import json
OUT = "/Users/ol1n/Dev/GitHub/Genealogy/matrika-ocr/ocr-out/narozeni/Pchery_predci.jsonl"
TS = "2026-07-06T12:00:00Z"

def rec(file, row):
    return {"file": file, "typ": "narozeni", "folio": "", "rok": "",
            "ok": True, "model": "claude-max", "attempts": 1, "ts": TS,
            "rows_count": 1, "lint": {"ok": True, "empty_cells": 0, "issues": []},
            "rows": [row]}

def b(dite, pohl, misto, otec, matka, dn="", pozn=""):
    return {"cislo": "", "datum_narozeni": dn, "datum_krtu": "", "misto_dum": misto,
            "dite_jmeno": dite, "dite_pohlavi": pohl, "dite_manzelske": "manželské",
            "nabozenstvi": "katol.", "otec_jmeno_stav": otec, "otec_bydliste": misto,
            "matka_jmeno_rodice": matka, "matka_bydliste": misto,
            "kmotri": "", "babka": "", "krtitel": "", "poznamenani": pozn}

rows = [
  # Kašíkova linie — Josefa Kašíková (babička, ∞ Josef) jako dcera Kašík × Svejkovská
  ("kasik.jpg", b("Josefa", "děvče", "Volešná č.25",
     "Kašík Václav, havíř z Volešné č.25",
     "Svejkovská Josefa ze Zaječova č.36",
     pozn="Josefa Kašíková, ∞ Josef Vořechovský; rodiče doloženi z Pchery 52/38")),
  # Matěj st. jako dítě Václava × Anny Podratzké → přidá Václava, Jakuba (Křivčice) a Podratzké
  ("matej_st.jpg", b("Matěj", "chlapec", "Volšany č.7",
     "Vořechovský Václav, chalupník z Volšan č.7, syn Jakuba Vořechovského z Křivčic",
     "Podratzká Anna, dcera Jakuba Podratzkého, sedláka z Olšan",
     dn="1775",
     pozn="Matěj st., sedlák Olšany č.7, †<1827; syn Václava (∞1763) a Anny Podratzké; datum ~orient.")),
]
with open(OUT, "w", encoding="utf-8") as f:
    for file, row in rows:
        f.write(json.dumps(rec(file, row), ensure_ascii=False) + "\n")
print("zapsáno", len(rows), "záznamů ->", OUT)
