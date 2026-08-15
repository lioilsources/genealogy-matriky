# Rod Podrazil (Sudoměřice u Skalice, okres Hodonín) — badatelský přehled

> Souhrn identifikace pramenů pro rod **Podrazil** a rekurzivně všechny
> přivdané linie. **Prapraděda uživatele Jan Podrazil (*17.9.1892,
> Sudoměřice č. 12) nalezen a zasazen do rodokmenu — 4 generace zpět
> k Matoušovi Podrazilovi × Marianně Tomšejové**, viz sekce 5. Plné
> strukturované OCR ještě neproběhlo, tohle je z ručního čtení indexů
> a primárních zápisů. Obdoba [`RODOKMEN-VORECHOVSKY.md`](RODOKMEN-VORECHOVSKY.md),
> jen v dřívější fázi. Zdroj skenů:
> [www.mza.cz/actapublica](https://www.mza.cz/actapublica/matrika)
> (MZA Brno), viewer `/actapublica/matrika/detail/{ID}`.
> Stav: 2026-08. Nástroj: [`actapublica-dl/`](actapublica-dl/) —
> inventář níže ověřen živým `make list OBEC=2787`.

---

## 1. Souhrn (headline)

- **Sudoměřice u Skalice** leží na Moravě → jiný archiv než zbytek repa
  (SOA Praha): **MZA Brno, Acta Publica**. Farnost **Strážnice** (kostely
  sv. Martin a Panny Marie).
- **Rozsah pátrání:** rod Podrazil a všechny přivdané (rekurzivně), **do
  vymření pramene** — tj. až k roku 1629 (nejstarší dochovaná farní kniha).
- **Plán je stáhnout celý farní korpus lokálně první**, OCR až poté (cestou
  Claude Max na Sonnetu → destilace do Qwen, přes existující
  `matrika-ocr/finetune/`). Tento dokument pátrání samo nezačíná — je to
  mapa pramenů pro toho, kdo stahování/OCR spustí.
- `obec_id=2787` = Sudoměřice, okres Hodonín. **21 knih, 4 160 skenů, 2
  stránky výsledků** hledání na Acta Publica (ověřeno `make list OBEC=2787`).

---

## 2. Inventář knih (obec_id=2787)

| skupina | knihy (detail id) | skenů | poznámka |
|---|---|---|---|
| Farní, všechny vesnice (Strážnice — sv. Martin) | 5199, 5200, 5201, 5202, 5207, 5210, 11495, 9324, 10420 | 2 628 | 1629–1917, farní zápisy (latina/čeština) |
| Jen Sudoměřice (Strážnice — Panny Marie) | 5224, 5225, 5226, 5227, 5235, 439, 12452, 5241, 5242, 5243, 12451, 5249 | 1 532 | N/O/Z 1785–1949 + rejstříky (5249 = jen rejstřík) |

Kniha **11495** (5811, N 1903–1917) patří ke **sv. Martin**, ne k Panny Marie
— oprava proti první verzi tohoto dokumentu, která ji měla ve špatné skupině.

Číslo v hranaté závorce v názvu složky (`Strážnice PM 5809 [5226]`) je
**detail id** — z něj jde postavit odkaz zpátky do prohlížeče
(`/actapublica/matrika/detail/5226?image=<jp2>`). Číslo knihy (5809) je
signatura, samostatná od detail id.

---

## 3. Badatelská strategie

### Rejstříky jsou zkratka k Podrazilům

Kniha **5852** (detail **5249**, 81 skenů) je abecední rejstřík N/O/Z
1785–1849 **jen pro Sudoměřice**, sloupce `Anno | Příjmení Jméno | Fol.`.
Vevázané rejstříky mají i kniha **5809** (N 1850–1934), **5825** (O),
**5840** (Z). Projít nejdřív tyto rejstříky = **~300 skenů místo 4 060**
pro první průchod — najít všechny výskyty Podrazilů a jejich variant, pak
cíleně dohledat konkrétní folia ve strukturovaných knihách.

Rejstříkové knihy dostanou v `meta.json` `typ: "rejstrik"` (nová hodnota,
viz `actapublica-dl/README`) — strukturovaná extrakce na ně nedává smysl,
patří do `transcribe` režimu `matrika-ocr`.

### Pravopisné varianty příjmení

`Podrazil` / `Podražil` / `Podrázil` / `Podrasil`, německy `Podrasill`.
Až budou reálná data z OCR, patří do `genealogy/seed/name_variants.csv`
(stejný mechanismus jako u `Vořechovský/Wořechowský` — viz
[`RODOKMEN-VORECHOVSKY.md`](RODOKMEN-VORECHOVSKY.md)).

### Farní knihy (1629–1803) vyžadují filtr na místo

Farnost Strážnice zahrnovala víc vesnic než jen Sudoměřice. Knihy z tohoto
období jsou **jeden zápis na řádek, latinsky, s obcí na konci** (`ex
Strážnicz`, `e Lipow`…) — bez filtru na lokalitu bychom OCR/extrakcí
natáhli celou farnost, ne jen sudoměřickou větev. Filtr na místo je nutná
podmínka extrakce, ne až následné čištění.

### Vysoké rozlišení pro OCR

`-derive halves` (viz `actapublica-dl`) řeže dvojstranu na L/R půlky s
dlouhou hranou ≤2576 px (strop Sonnetu 5), stejnou geometrií jako
`matrika-ocr/split.go` (`cropHalf`), aby stránky exportované pro trénink
(`finetune/export_pages.py`, `half_map.json`) seděly 1:1 s ground truth
z Claude-Max průchodu.

---

## 4. Technický kontext (stahovač)

Acta Publica běží na jiném stacku než ebadatelna.soapraha.cz (PHP +
OpenSeadragon + IIPImage/IIIF místo Apache Wicket) — proto samostatná
utilita `actapublica-dl/` se stejným výstupním kontraktem
(`Nazev [ID]/0001.jpg` + `meta.json`), který `matrika-ocr` a `genealogy`
konzumují beze změny. Detaily endpointů, IIIF tiling (strop 2000 px/dlaždice)
a co je v tomto směru ověřené: viz
[`actapublica-dl/README`](README.md#actapublica-dl-mza-brno) sekce
"Co je ověřené".

`genealogy/export.go` (`ebadatelnaURL`) dnes umí postavit odkaz do
prohlížeče jen pro SOA Praha — až budou data z této obce v databázi, bude
potřeba větvení podle `meta.archive` (`mza-actapublica` vs. výchozí).

---

## 5. Prameny

### ⭐ Prapraděda nalezen: Jan Podrazil, *17.9.1892

**Kniha 5810 [5227] (N 1861–1902), sken 0243, obec Sudoměřice, září 1892:**

> **Jan Podrazil**, *17.9.1892 (pokřtěn 18.9.), Sudoměřice č. 12. Otec
> **Josef Podrazil**, chalupník, **syn Matouše Podrazila** (chalupníka) a
> **Marianny, dcery †Martina Tomšeje**, půlčtvrtníka. Matka **Marie, dcera
> Josefa Mrzý**, půlčtvrtníka, a Marianny, dcery †Martina Janečka.

**Hádanka o dvou Matoušech vyřešena** — nalezeny primární sňatky obou:

```
Jan (Johann) Podrazil (†před 1813)
  │
  ├── Matouš Podrazil starší (*~1789, sňatek 1813, dům č.36)
  │     └── 5 dětí zemřelo 1815–1823 (Anna, Martin×2, Elisabeth, Josef)
  │
  └── Josef Podrazil (†před 1846) × Elisabeth Pepperová
        └── Matouš Podrazil mladší (*~1826, sňatek 1846) × Marianna Tomšejová
              └── Josef Podrazil (*~1866) × Marie Myšová (sňatek 1890)
                    └── Jan Podrazil (*17.9.1892) × Kateřina Porubková (1922)
                          └── min. 5 dětí (*1917–1933): Marie, František,
                              Jan ml., Martin, další  ⭐ prapraděda a jeho rodina
```

**Matouš starší** (sňatek 1813, primární matrika ověřeno) má JINOU nevěstu
než Marianna Tomšejová — proto NENÍ přímým otcem Josefa. Skutečný manžel
Marianny Tomšejové je **Matouš mladší** (*~1826, o 37 let mladší, sňatek
1846, kniha 5807 folio 102–103, dům č. 13) — jeho otec je **další,
dosud nedoložený Josef Podrazil** (†před 1846) × **Elisabeth Pepperová**.
Vztah Jan → Matouš st. / (mladší) Josef není zcela jistý (bratři? otec-syn?),
ale řetězec od (mladšího) Josefa dolů k Janovi *1892 je pevně doložený
primárními zápisy — **6 generací, 1786(odhad Janova narození) až 1933**.

Detaily a metodika hledání:
[`matrika-ocr/ocr-out/_podrazil_index_5809_1850-1934.md`](matrika-ocr/ocr-out/_podrazil_index_5809_1850-1934.md).

**Vztah Jan Podrazil (†1813) ↔ Matouš st. ↔ Josef (†1846) — vyčerpávající
pátrání v knize 5807 (roky 1806–1845, ~18 úseků) nenašlo Josefův sňatek
ani úmrtí.** Zůstává nejlepším odhadem, ne faktem — další stopa by musela
přijít odjinud (zbytek indexu 1850–1934, nebo jiná kniha).

### ⭐ Konektivita do přítomnosti: ANO — Jan měl min. 2–3 děti, rodina byla jedna z největších v obci

**Janovy vlastní děti** (opraveno po druhém, pečlivějším čtení rejstříku —
první čtení popletlo směr "dítě–otec" u několika řádků): jistě/pravděpodobně
**František (~1926), Jan ml. (~1929), Jan (~1934)** — řádky se jmény
otce "Jan" datované PŘED jeho sňatkem 1922 patří jinému, staršímu
Janu Podrazilovi, ne jemu. Plus bratr **Štěpán Podrazil** (další Josefův
syn), ženatý 1931 s Marií Tomšejovou, měl vlastní dceru Alžbětu (~1932).
Dál než 1934 tento index nejde a dál než ~1949 obecně nejdou církevní
matriky vůbec (civilní matrika od té doby není součástí Acta Publica a
je navíc ze zákona uzavřená ~100 let).

**Rodina jako celek byla mnohem rozvětvenější, než se zdálo zprvu** —
kompletní přepis skenu 159 (1850–1878) ukázal nejméně **šest souběžných
otců-Podrazilů** (Jan/Tomášův, Matouš mladší, Tomáš, Jiří, Martin, Pavel),
a sken 160 (1879–1934) desítky dalších záznamů s otci Petr, František,
Josef, Štěpán a další — přesné rozplétání všech větví by vyžadovalo
ověření v primárních knihách, ne jen v indexu. Detaily:
[`matrika-ocr/ocr-out/_podrazil_index_5809_1850-1934.md`](matrika-ocr/ocr-out/_podrazil_index_5809_1850-1934.md).

**Dcery Podrazilovy se prokazatelně vdávaly lokálně v Sudoměřicích** —
potvrzeno napříč generacemi ve sňatkovém rejstříku knihy 5825 [5235]:
**Maria Anna Podrazil → Martin Přikaský (1862)**, **Cecilie Podrazilová →
Matouš Obrlík (1906)**, **Cecilie Podražilová → Josef Petráš (1933)**.

**Sňatky Josefa i Jana OVĚŘENY přímo v primární matrice:**
- **28.1.1890, kniha 5825, folio 219:** Josef Podrazil (syn Matouše ×
  Marianny Tomšejové) × **Marie, dcera Josefa Myšího a Marianny Janečkové**
  — oba ze Sudoměřic.
- **11.7.1922, kniha 5827, sešit V/list 7:** **Jan Podrazil** (rolník,
  *17.9.1892 v Sudoměřicích, syn Josefa Podrazila a Marianny/Marie
  Mišové) × **Kateřina Porubková** (dcera Jiřího Porubka a Anny
  Okáníkové) — **oba ze Sudoměřic, potvrzeno přesným rodným datem/místem.**

**Odkud pocházely ženy Podrazilů:** vzorek ověřených sňatků (Matouš×
Tomšejová, Karel×Bučková, Tomáš×Mikešková, Josef×Myšová, Jan×Porubková)
ukazuje **silně endogamní vzorec — téměř všechny manželky byly ze
Sudoměřic**, ne z okolních vesnic. **Jedna jasná výjimka:** Jiří Podrazil
měl manželku **Mariannu, rozenou Martinkovou ze Zvolenova** (sousední
vesnice téže farnosti) — příležitostné sňatky mimo obec se děly, ale byly
výjimkou. Vzorek není kompletní (~20 sňatků Podrazil/Podražil mužů v téhle
knize, ověřeno jen 5) — detaily a metodika v `_podrazil_index_5809...md`.

Celkem jen v této jedné knize (1852–1934) napočítáno **min. 20 sňatků**
mužů příjmení Podrazil/Podražil — rodina byla mezi **nejplodnějšími v
Sudoměřicích** po celé 19. a začátek 20. století. Detaily:
[`matrika-ocr/ocr-out/_podrazil_index_5809_1850-1934.md`](matrika-ocr/ocr-out/_podrazil_index_5809_1850-1934.md)
(sekce "Sňatkový rejstřík").

### Starší generace (1785–1849, z indexu 5852)

- **Index Strážnice PM 5852 [5249]** (celý stažen, abecední rejstřík N/O/Z
  1785–1849) — přečten Sonnetem (Claude Max, ne Qwen), písmeno P kompletně
  ve všech třech řadách. Nálezy + metodika:
  [`matrika-ocr/ocr-out/_podrazil_index_Sudomerice5852_1785-1849.md`](matrika-ocr/ocr-out/_podrazil_index_Sudomerice5852_1785-1849.md).
- **Index je prokazatelně neúplný** — křížovou kontrolou v primární matrice
  **5807 [5224]** se našly 2 skutečné sňatky Podrazilů, které v indexu
  vůbec nejsou (1843, 1849), a role kmotra/svědka index nezachycuje vůbec.
  Klíčový nález: úmrtí "Pavel Podrazil 1786, fol 3" v indexu je ve
  skutečnosti **novorozenec Pavel, syn Pavla Podrazila** — index zkracuje
  patronymické zápisy, takže tabulka ~30 úmrtí NENÍ 30 nezávislých
  dospělých, ale hlavně děti menšího počtu otců.
- **Rodinné jádro — 5 dospělí Podrazilové doložení přímo v primární matrice**
  (ne jen indexem), vztah mezi nimi zatím nedoložen:
  - **Pavel Podrazil starší** — sedlák (Bauer), doložen 1786.
  - **Josef Podrazil** × Alžběta — kmotři při křtu 1810.
  - **Karel Podrazil** — čtvrtník/Halbwirth, dům N.14, × **†Alžběta, dcera
    Martina Bučky** (podsedníka). Děti:
    - **Tomáš Podrazil** (*~1807) — chalupník, vdovec, ženil se podruhé
      5.11.1849 s Kateřinou Mikeškovou (fol 118 Trauungsbuch 5807).
    - **Elisabeth Podrazil** (*~1825) — provdala se 4.2.1843 za Franze
      Tomiczyho (fol 88 Trauungsbuch 5807).
    - **Franz Podrazil** — †22.2.1815 jako roční dítě.
    - **Elisabeth Podrazil** (starší, stejné jméno) — †listopad 1820,
      zemřela před narozením druhé Elisabeth výše.
  - **Matouš (Matthäus) Podrazil** — Halbwirth/Häusler, dům N.36/37.
    **5 dětí zemřelo v útlém věku 1815–1823**: Anna (†28.1.1815, ½ r.),
    Martin (†17.6.1815), Elisabeth (†duben 1819, 2 r.), Josef (†únor 1820,
    6 dní), Martin (druhý, †říjen 1823, 1 týden) — nejtragičtější
    dokumentovaná rodina v tomto pátrání.
  - **Paul Podrazil** (mladší) — dům N.16, svatební svědek 1818. 2 děti
    (obě Franz, jméno použito znovu po prvním úmrtí) zemřely 1824 a 1826.
- Rukopis 1785–~1840 je německý kurent, dost obtížně čitelný (i pro Sonnet);
  pozdní zápisy (1845+, jiná ruka, částečně česky) jsou naopak velmi čitelné.
  **Folio→kniha mapování pro knihu 5807 je ověřené na 8 nezávislých bodech**
  (fol 74/75/89/92/95/102/106/110) — vzorec v `_podrazil_index_...md`.
- **Pozor — roky v indexu 5852 jsou systematicky nespolehlivé, i když folia
  sedí.** Na 10 dosud ověřených řádcích úmrtní tabulky mělo **6 špatný rok**
  (vždy posunutý o 10-16 let dopředu) a **1 byl úplně jiná rodina** (fol 104,
  ne Podrazil). **Folia jsou spolehlivá (9/10 potvrzeno), roky ne — odhad
  "~30 úmrtí" je pravděpodobně blízko realitě co do počtu lidí, ale
  konkrétní roky u needěných řádků nejsou důvěryhodné.**
- **Hledání Karlova/Josefova/Matoušova/Paulova sňatku zatím neúspěšné** — chronologický
  průchod Trauungsbuch pokryl roky 1785, 1790–92, 1796, 1799–1801, 1804–05,
  1808–09 bez nálezu; zbývají nepokryté mezery (1786–89, 1793–95, 1797–98,
  1802–03, 1806–07, 1810+).

## 6. Stav bádání — co je hotovo, co zbývá

### ✅ Hotovo

1. **Stahovač + plné stažení** — `actapublica-dl` postavený a ověřený proti
   živému webu, celá obec Sudoměřice (2787) stažená: 21 knih, 4 160 skenů,
   ~16 GB. `make list OBEC=2787` sedí přesně na inventář v sekci 2.
2. **Index 1785–1849 (kniha 5852)** — písmeno P přečtené kompletně ve
   všech 3 řadách (N/O/Z). Zjištěno a zdokumentováno, že **index je
   nespolehlivý** (chybí sňatky, špatné roky, 1 falešný nález) — folia
   samotná jsou ale spolehlivá.
3. **Prapraděda nalezen a ověřen:** **Jan Podrazil, *17.9.1892, Sudoměřice
   č. 12** — přímo v primární matrice (kniha 5810, sken 243), včetně jeho
   sňatku 1922 s Kateřinou Porubkovou (kniha 5827).
4. **Rodokmen 6 generací zpět** (1786–1892), s primárními zápisy pro
   klíčové uzly: Josef×Marie Myšová (1890), Matouš ml.×Marianna Tomšejová
   (1846), Matouš st. sňatek (1813, jiná větev). Jen **vztah Jan Podrazil
   †1813 ↔ Matouš st. ↔ Josef †1846 zůstává nejistý** — hledáno
   vyčerpávajícím způsobem v knize 5807 (1806–1845), nenalezeno.
5. **Konektivita do přítomnosti potvrzena** — Jan měl min. 5–7 dětí
   (1917–1933); dál nejdou církevní matriky vůbec (~1949 hranice,
   civilní matrika mimo dosah).
6. **Dcery vdávající se lokálně potvrzeny** (3 příklady napříč generacemi)
   a **původ manželek** zmapován (silně endogamní, 1 výjimka ze Zvolenova).
7. **Objeven druhý masivní index** (1850–1934, vevázaný v knize 5809) —
   zatím jen částečně přečtený, ale potvrzeno, že rodina má desítky dalších
   záznamů (min. 20 sňatků mužů Podrazil/Podražil jen v jedné knize).

### 🔲 Zbývá (seřazeno podle toho, co by dalo nejvíc)

1. **Dočíst zbytek indexu 1850–1934** (kniha 5809, sken 159–167) — přečteno
   jen ~40 %. Obsahuje pravděpodobně desítky dalších narození/vazeb,
   včetně možná stopy k vyřešení hádanky Jan/Matouš st./Josef.
2. **Najít Josefovo (*~1866) vlastní narození** — spojilo by ho jistě
   s Matoušem mladším přímým záznamem, ne jen odvozením ze svatby.
3. **Ověřit zbylé sňatky ze sňatkového rejstříku** (kniha 5825) — ověřeno
   jen 5 z ~20 nalezených řádků (1868, 1871, 1873, 1875, 1897, 1898, 1903,
   1906, 1908, 1911+ zatím jen v indexu, ne v primární matrice).
4. **Karlův sňatek s Alžbětou Bučkovou** a **Tomášovo narození (~1806/07)**
   — zmíněné odjinud (přes děti/vnuky), ale samotné zápisy nenalezené.
5. **Janovi potomci dál** — jeho děti (Marie, František, Jan ml., Martin…)
   měly samy děti/vnuky? Rejstřík 1850–1934 by na to mohl mít odpověď
   (viz bod 1).
6. Podle toho, co se najde, případně cíleně stáhnout/OCR další
   strukturované knihy (N/O/Z) nebo farní knihy 1629–1917 s filtrem na
   Sudoměřice/Strážnici — ale tohle je teď nižší priorita, protože ruční
   čtení indexů se ukázalo mnohem rychlejší.
7. Průběžně doplňovat `genealogy/seed/name_variants.csv` (Podrazil/
   Podražil/Podrázil/Podrasil) až budou data v pipeline, a udržovat tuhle
   sekci aktuální (i negativní nálezy, ať se hledání neopakuje).
