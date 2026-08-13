# Rod Podrazil (Sudoměřice u Skalice, okres Hodonín) — badatelský přehled

> Souhrn identifikace pramenů pro rod **Podrazil** a rekurzivně všechny
> přivdané linie. Zatím **bez genealogických dat** — obsahuje inventář knih,
> badatelskou strategii a roadmap; samotné stahování/OCR ještě neproběhlo.
> Obdoba [`RODOKMEN-VORECHOVSKY.md`](RODOKMEN-VORECHOVSKY.md), jen v dřívější
> fázi. Zdroj skenů: [actapublica.eu](https://actapublica.eu) (MZA Brno),
> viewer `/actapublica/matrika/detail/{ID}`.
> Stav: 2026-08. Nástroj: [`actapublica-dl/`](actapublica-dl/).

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
- `obec_id=2787` = Sudoměřice. **21 knih, ~4 060 skenů, 2 stránky výsledků**
  hledání na Acta Publica.

---

## 2. Inventář knih (obec_id=2787)

| skupina | knihy (detail id) | skenů | poznámka |
|---|---|---|---|
| Jen Sudoměřice (Strážnice — Panny Marie) | 5224, 5225, 5226, 5227, 11495, 5235, 439, 12452, 5241, 5242, 5243, 12451, 5249 | 1 530 | N/O/Z 1785–1949 + rejstříky |
| Farní, všechny vesnice (Strážnice — sv. Martin) | 5199, 5200, 5201, 5202, 5207, 5210, 9324, 10420 | 2 530 | 1629–1803, latinská próza |

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
a co je v tomto směru ověřené vs. odvozené: viz
[`actapublica-dl/README`](README.md#actapublica-dl-mza-brno) sekce
"Co je (a co není) ověřené".

`genealogy/export.go` (`ebadatelnaURL`) dnes umí postavit odkaz do
prohlížeče jen pro SOA Praha — až budou data z této obce v databázi, bude
potřeba větvení podle `meta.archive` (`mza-actapublica` vs. výchozí).

---

## 5. Prameny

Zatím žádné — stahování ani OCR této obce ještě neproběhlo. Až proběhne,
sem patří (stejně jako v `RODOKMEN-VORECHOVSKY.md`) i **negativní nálezy**
(prohledané knihy/rozsahy bez výskytu rodu), aby se OCR/hledání neopakovalo.

## 6. Další kroky

1. `cd actapublica-dl && make build && make list OBEC=2787` — ověřit inventář
   (21 knih, ~4 060 skenů) proti tabulce v sekci 2.
2. Stáhnout rejstříkové knihy (5249, a `5809`/`5825`/`5840` jde-li je stáhnout
   samostatně) — cca 300 skenů, rychlý první průchod.
3. OCR rejstříků v `transcribe` režimu → seznam folií s výskytem Podrazilů
   (a variant z bodu 3).
4. Podle folií cíleně stáhnout/OCR strukturované knihy (N/O/Z), pak farní
   knihy 1629–1803 s filtrem na Sudoměřice/Strážnici.
5. Průběžně doplňovat `genealogy/seed/name_variants.csv` a tuto sekci 5
   (prameny + negativní nálezy).
