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

**Poprvé známe jméno Matoušovy manželky: Marianna Tomšejová.**

```
[Jan Podrazil, †před 1813]
  → Matouš Podrazil st. (*~1789, sňatek 1813, dům č.36)
    → [5 dětí zemřelo 1815–1823] + Matouš Podrazil ml.(?) × Marianna Tomšejová
      → Josef Podrazil (*~1866) × Marie (dcera Josefa Mrzý a Marianny Janečkové)
        → Jan Podrazil, *17.9.1892, Sudoměřice č. 12  ⭐ prapraděda
```

⚠️ **Otevřená hádanka:** sňatek Matouše st. (1813, primární matrika
ověřeno) má JINOU nevěstu než Marianna Tomšejová, a věkem (Matouš st.
*1789 by měl Josefovi v roce 1866 přes 77 let) **nemůže být jeho otcem**.
Musí jít o **druhého, mladšího Matouše** (syn/vnuk Matouše st.?) — jeho
sňatek s Mariannou Tomšejovou zatím nenalezen (hledáno v rejstříku
1850–1905 pod P i T bez úspěchu — buď je starší než 1850, nebo
nezaznamenaný). Nebrat prozatím řetězec Matouš st. → Josef jako jistý,
jen jako pracovní hypotézu.

Detaily a metodika hledání (prošlo se přes rok 1891 v Sudoměřicích i
Petrově naprázdno, než uživatel dal přesnou lokaci sken 243/řádek 2):
[`matrika-ocr/ocr-out/_podrazil_index_5809_1850-1934.md`](matrika-ocr/ocr-out/_podrazil_index_5809_1850-1934.md).

**Další krok:** najít Matouše ml. × Tomšejová sňatek (možná v knize
5813/5814, O 1786–1849) — to spojí Josefa jistě s předky, ne jen
odhadem. Taky najít Josefovo vlastní narození.

### ⭐ Konektivita do přítomnosti: ANO — Jan měl min. 5 dětí, rodina byla jedna z největších v obci

**Janovi vlastní děti** (z pokračování stejného rejstříku 1850–1934):
Marie, František, Jan (ml.), Martin a další — min. 5–7 dětí narozených
~1917–1933. Plus bratr **Štěpán Podrazil** (další Josefův syn), ženatý
1931 s Marií Tomšejovou. Dál než 1934 tento index nejde a dál než ~1949
obecně nejdou církevní matriky vůbec (civilní matrika od té doby není
součástí Acta Publica a je navíc ze zákona uzavřená ~100 let).

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

## 6. Další kroky

1. ~~`cd actapublica-dl && make build && make list OBEC=2787` — ověřit inventář~~
   — hotovo (21 knih, 4 160 skenů sedí).
2. ~~Stáhnout rejstříkovou knihu 5249~~ — hotovo, celá obec 2787 stažena.
3. ~~OCR rejstříku 5249 → seznam folií s Podrazily~~ — hotovo, všechny 3
   řady písmeno P kompletně přečtené.
4. ~~Ověřit folio→kniha mapování a přečíst pár zápisů v primární matrice~~ —
   hotovo, mapování kalibrované, 2 sňatky + kmotr/svědek role nalezené,
   klíčový úmrtní zápis (fol 3/1786) přečten a reinterpretován.
5. **Najít Tomášovo narození (~1806/07)** a **Karlův/Josefův sňatek**
   (asi 1795–1810, mimo pokrytí indexu) v Geburtsbuch/Trauungsbuch 5807 —
   viz "Další kroky" v `_podrazil_index_...md` pro konkrétní skeny.
6. Přečíst zbylá jistá folia z úmrtní tabulky přímo v matrice (kalibrace
   mapování to teď umožňuje rychle) — u každého ověřit dítě/dospělý,
   rodiče, číslo domu.
7. Podle toho cíleně stáhnout/OCR další strukturované knihy (N/O/Z), pak
   farní knihy 1629–1917 s filtrem na Sudoměřice/Strážnici.
8. Průběžně doplňovat `genealogy/seed/name_variants.csv` a Prameny výše
   (i negativní nálezy, ať se OCR/hledání neopakuje).
