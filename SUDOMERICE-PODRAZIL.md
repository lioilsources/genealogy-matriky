# Rod Podrazil (Sudoměřice u Skalice, okres Hodonín) — badatelský přehled

> Souhrn identifikace pramenů pro rod **Podrazil** a rekurzivně všechny
> přivdané linie. Zatím **bez genealogických dat** — obsahuje inventář knih,
> badatelskou strategii a roadmap; samotné stahování/OCR ještě neproběhlo.
> Obdoba [`RODOKMEN-VORECHOVSKY.md`](RODOKMEN-VORECHOVSKY.md), jen v dřívější
> fázi. Zdroj skenů: [www.mza.cz/actapublica](https://www.mza.cz/actapublica/matrika)
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

- **Index Strážnice PM 5852 [5249]** (celý stažen, abecední rejstřík N/O/Z
  1785–1849) — přečten Sonnetem (Claude Max, ne Qwen), písmeno P **kompletně
  ve všech třech řadách** (narození/oddaní/zemřelí). Nálezy + metodika:
  [`matrika-ocr/ocr-out/_podrazil_index_Sudomerice5852_1785-1849.md`](matrika-ocr/ocr-out/_podrazil_index_Sudomerice5852_1785-1849.md).
  **Odhad počtu osob: ~30 zápisů úmrtí 1786–1849** (po očištění duplicit
  ze čtení cca 28–30) — vzhledem k opakujícím se křestním jménům (Josef,
  Elisabeth, Franz, Martin) jde spíš o **jádro cca 5–10 dospělých +
  desítky dětských/kojeneckých úmrtí**, typické pro dobu, ne 30 nezávislých
  dospělých větví.
  **Nesrovnalost k prověření: v Geburts-Buch (narození) ani Trauungs-Buch
  (oddaní) není za stejné období 1785–1848 ani jeden Podrazil** — obě řady
  jsou kompletně přečtené, ne jen částečně. To může znamenat chybu ve čtení,
  jiný zápis příjmení u křtů/sňatků, nebo že se rodina do Sudoměřic
  přistěhovala už jako dospělá. Nejde brát počet úmrtí jako definitivní,
  dokud se to neověří v primární matrice (viz Další kroky).
- Rukopis 1785–~1840 je německý kurent, dost obtížně čitelný (i pro Sonnet) —
  jistota zápisů je u starších let nižší, u pozdních (1845+, jiná ruka) vysoká.
  Folio čísla z indexu **nejsou ještě ověřená proti skutečné matrice** (pracovní
  hypotéza: kniha 5807 [5224]) — to je další krok, ne hotová věc.

## 6. Další kroky

1. ~~`cd actapublica-dl && make build && make list OBEC=2787` — ověřit inventář~~
   — hotovo, inventář v sekci 2 je z živého běhu (21 knih, 4 160 skenů sedí).
2. ~~Stáhnout rejstříkovou knihu 5249~~ — hotovo, celá obec 2787 stažena
   (viz commit historie), včetně 5249.
3. ~~OCR rejstříku 5249 → seznam folií s Podrazily~~ — hotovo, všechny 3
   řady (N/O/Z) písmeno P kompletně přečtené, viz
   `_podrazil_index_Sudomerice5852_1785-1849.md`. ~30 úmrtí, 0 křtů/sňatků
   (nesrovnalost k ověření).
4. Dočíst zbytek indexu (Geburts sken 0031, Trauungs sken 0050 celé) — pak
   ověřit folio→kniha mapování a přečíst samotné zápisy v 5807 [5224] na
   nalezených foliích (rodiče, čísla domů — mnohem bohatší než index).
5. Podle toho cíleně stáhnout/OCR další strukturované knihy (N/O/Z), pak
   farní knihy 1629–1917 s filtrem na Sudoměřice/Strážnici.
6. Průběžně doplňovat `genealogy/seed/name_variants.csv` a Prameny výše
   (i negativní nálezy, ať se OCR/hledání neopakuje).
