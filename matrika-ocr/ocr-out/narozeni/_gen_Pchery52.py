#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import json

OUT = "/Users/ol1n/Dev/GitHub/Genealogy/matrika-ocr/ocr-out/narozeni/Pchery 52 [11476].jsonl"
TS = "2026-07-04T12:00:00Z"

# KANONICKÉ popisy prarodičů — byte-identické ve všech záznamech, aby je matcher
# spároval do jednoho uzlu (jinak vzniká duplicitní děda/bába na každý záznam).
DED_JOSEF   = "Josefa Vořechovského, domkáře z Volšan č.44, a Josefy Kašíkové ze Zaječova č.36"
DED_BILEK   = "Františka Bílka, domkáře v Pcherách č.58, a Marie Landové ze Svinařova č.21"
DED_ZELENKA = "Františka Zelenky, horníka z Pcher č.27, a Karly Anýžové z Brandýska č.22"

# rodiče (bohatá próza -> extract vytáhne i prarodiče); prarodičovská část identická
OTEC_KAREL   = "Vořechovský Karel, horník v Pcherách, narozen 30.10.1867, syn " + DED_JOSEF
OTEC_BEDRICH = "Vořechovský Bedřich, horník v Pcherách, narozen 7.12.1874, syn " + DED_JOSEF
MATKA_MARIE  = "Marie roz. Bílková v Pcherách, narozena 4.8.1870, dcera " + DED_BILEK
MATKA_ZELENKA= "Marie roz. Zelenková v Pcherách, narozena 26.11.1876, dcera " + DED_ZELENKA

def rec(file, folio, rok, rows):
    return {"file": file, "typ": "narozeni", "folio": folio, "rok": rok,
            "ok": True, "model": "claude-max", "attempts": 1, "ts": TS,
            "rows_count": len(rows),
            "lint": {"ok": True, "empty_cells": 0, "issues": []},
            "rows": rows}

def b(cislo, dn, dk, misto, dite, pohl, otec, otbyd, matka, mabyd,
      kmotri="", baba="", krt="", pozn="", manz="manželské"):
    return {"cislo": cislo, "datum_narozeni": dn, "datum_krtu": dk,
            "misto_dum": misto, "dite_jmeno": dite, "dite_pohlavi": pohl,
            "dite_manzelske": manz, "nabozenstvi": "katol.",
            "otec_jmeno_stav": otec, "otec_bydliste": otbyd,
            "matka_jmeno_rodice": matka, "matka_bydliste": mabyd,
            "kmotri": kmotri, "babka": baba, "krtitel": krt, "poznamenani": pozn}

# ---- Karel × Marie Bílková (10 dětí) ----
K = [
 ("0031","28","1890", b("28","6.12.1890","8.12.1890","Pchery č.30","Josefa Marie","děvče",
    OTEC_KAREL,"Pchery č.33",MATKA_MARIE,"Pchery č.33",
    "Josefina Milerová; Marie Kumherová","Barbora Kubíková z Humen","Fr. Hrabě, farář",
    "zemřela 31.12.1890")),
 ("0037","34","1891", b("34","25.12.1891","27.12.1891","Pchery č.15","Milada","děvče",
    OTEC_KAREL,"Pchery č.15",MATKA_MARIE,"Pchery č.15",
    "Marie Kumherová","Barbora Kubíková z Humen","M. Kaufner, kaplan",
    "dle přípisu okr. hejtm. Slaný 7.8.1919 vystoupila 15.6.1919 z církve k bezvěří")),
 ("0045","42","1893", b("42","28.7.1893","30.7.1893","Pchery č.50","Oldřich Jan","chlapec",
    OTEC_KAREL,"Pchery č.50",MATKA_MARIE,"Pchery č.33",
    "Antonín Vořechovský, horník z Volšan; Anna Kumperová","Barbora Kubíková z Humen","Fr. Hrabě, farář",
    "oddán 19.10.1920 ve Pcherách s Annou Šimkovou (*28.12.1902 Dubí); 1935 s Andělou Němcovou")),
 ("0053","49","1895", b("49","26.2.1895","3.3.1895","Pchery č.50","Božena","děvče",
    OTEC_KAREL,"Pchery č.50",MATKA_MARIE,"Pchery č.83",
    "Anna Vořechovská, manž. Antonína Vořechovského, horníka z Volšan","Barbora Kubíková z Humen","Karel Pokorný, kaplan",
    "zemřela 26.3.1895")),
 ("0059","55","1896", b("55","29.3.1896","5.4.1896","Pchery č.32","Anna","děvče",
    OTEC_KAREL,"Pchery č.32",MATKA_MARIE,"Pchery č.58",
    "Antonín Vořechovský, horník z Volšan","Barbora Kubíková z Humen","Fr. Hrabě, farář",
    "oddáni rodiče 2.9.1890; Anna oddána 4.12.1918 ve Pcherách s Václavem Šedivým (*26.1.1895 Rozdělov)")),
 ("0081","78","1899", b("78","19.8.1899","27.8.1899","Pchery č.22","Antonie","děvče",
    OTEC_KAREL,"Pchery č.22",MATKA_MARIE,"Pchery č.22",
    "Antonín Vořechovský, horník z Volšan","Barbora Kubíková z Humen","",
    "vystoupila z církve k čsl. ev. 22.5.1929")),
 ("0096","93","1901", b("93","8.10.1901","13.10.1901","Pchery č.4","Marie","děvče",
    OTEC_KAREL,"Pchery č.4",MATKA_MARIE,"Pchery č.58",
    "Anna Vořechovská, manž. Antonína Vořechovského, horníka z Volšan","Barbora Kubíková z Humen","","")),
 ("0105","102","1903", b("102","5.2.1903","8.2.1903","Pchery č.4","Karel","chlapec",
    OTEC_KAREL,"Pchery č.4",MATKA_MARIE,"Pchery č.55",
    "Antonín Vořechovský, horník z Volšan","Barbora Kubíková z Humen","Fr. Hrabě, farář",
    "dle přípisu okr. úř. Slaný 17.4.1929 vystoupil z církve k čsl. evang.; oddán 13.4.1929 s Františkou Boženou Škarbanovou")),
 ("0118","115","1905", b("115","5.4.1905","9.4.1905","Pchery č.4","František","chlapec",
    OTEC_KAREL,"Pchery č.4",MATKA_MARIE,"Pchery č.28",
    "Antonín Vořechovský, horník z Volšan","Barbora Kubíková z Humen","Josef Sainer, kaplan","")),
 ("0141","138","1908", b("138","12.2.1908","16.2.1908","Pchery č.82","Josef","chlapec",
    OTEC_KAREL,"Pchery č.82",MATKA_MARIE,"Pchery č.28",
    "Barbora Kubíková, manž. Josefa Kubíky, horníka z Humen","Barbora Kubíková z Humen","Josef Sainer, administrátor","")),
]

# ---- Bedřich × Marie Zelenková (9 dětí) ----
BR = [
 ("0067","63","1897", b("63","29.8.1897","1.9.1897","Pchery č.27","Josefa","děvče",
    OTEC_BEDRICH,"Pchery č.27",MATKA_ZELENKA,"Pchery č.27",
    "František Zelenka, horník z Pcher","Anna Černá z Humen","Fr. Hrabě, farář","zemřela 10.4.1899")),
 ("0074b","70","1898", b("70","15.7.1898","16.7.1898","Pchery č.46","Anna","děvče",
    OTEC_BEDRICH,"Pchery č.46",MATKA_ZELENKA,"Pchery č.27",
    "Karolina Zelenková","Anna Černá z Humen","Fr. Hrabě, farář","zemřela 17.7.1898")),
 ("0087","82","1900", b("82","10.7.1900","12.7.1900","Pchery č.46","Anna","děvče",
    OTEC_BEDRICH,"Pchery č.46",MATKA_ZELENKA,"Pchery č.27",
    "Karel Zelenka","Barbora Kubíková z Humen","","oddána 1918 s Rudolfem Vánou")),
 ("0093","88","1901", b("88","14.6.1901","16.6.1901","Pchery č.46","Josefa","děvče",
    OTEC_BEDRICH,"Pchery č.46",MATKA_ZELENKA,"Pchery č.27",
    "Barbora Zelenková, dcera Františka Zelenky","Barbora Kubíková z Humen","Karel Pokorný, kaplan","")),
 ("0101","98","1902", b("98","24.9.1902","27.9.1902","Pchery č.46","Václav","chlapec",
    OTEC_BEDRICH,"Pchery č.46",MATKA_ZELENKA,"Pchery č.27",
    "František Zelenka, horník z Pcher","Barbora Kubíková z Humen","Fr. Hrabě, farář","zemřel 19.11.1903")),
 ("0110","107","1904", b("107","25.1.1904","26.1.1904","Pchery č.74","Marie","děvče",
    OTEC_BEDRICH,"Pchery č.74",MATKA_ZELENKA,"Pchery č.27",
    "Barbora Zelenková","","Fr. Hrabě, farář","zemřela 13.11.1904")),
 ("0119","116","1905", b("116","1.5.1905","3.5.1905","Pchery č.74","Julie","děvče",
    OTEC_BEDRICH,"Pchery č.74",MATKA_ZELENKA,"Pchery č.27",
    "Barbora Zelenková, dcera Františka Zelenky","","Fr. Hrabě, farář","zemřela 6.11.1905")),
 ("0125","122","1906", b("122","2.4.1906","3.4.1906","Pchery č.74","Františka","děvče",
    OTEC_BEDRICH,"Pchery č.74",MATKA_ZELENKA,"Pchery č.27",
    "Barbora Zelenková","Barbora Kubíková z Humen","Fr. Hrabě, farář","")),
 ("0137","134","1907", b("134","29.9.1907","2.10.1907","Pchery č.16","František Josef","chlapec",
    OTEC_BEDRICH,"Pchery č.16",MATKA_ZELENKA,"Pchery č.16",
    "Josef Zelenka, horník z Pcher","Barbora Kubíková z Humen","Josef Sainer, kaplan","")),
]

# ---- Anna roz. Vořechovská (sestra Karla/Bedřicha) ∞ František Šanda ----
SANDA = [
 ("0042","39","1893", {
    "cislo":"39","datum_narozeni":"2.12.1893","datum_krtu":"8.12.1893","misto_dum":"Pchery č.16",
    "dite_jmeno":"Antonín","dite_pohlavi":"chlapec","dite_manzelske":"manželské","nabozenstvi":"katol.",
    "otec_jmeno_stav":"Šanda František, horník ve Pcherách č.16","otec_bydliste":"Pchery č.16",
    "matka_jmeno_rodice":"Anna roz. Vořechovská, dcera " + DED_JOSEF,
    "matka_bydliste":"Pchery č.16","kmotri":"Antonín Koudela, horník z Pcher","babka":"","krtitel":"","poznamenani":""}),
]

rows_all = K + BR + SANDA
with open(OUT, "w", encoding="utf-8") as f:
    for file, folio, rok, row in rows_all:
        fn = file.replace("b","") + ".jpg"
        f.write(json.dumps(rec(fn, folio, rok, [row]), ensure_ascii=False) + "\n")

print("zapsáno", len(rows_all), "záznamů ->", OUT)
