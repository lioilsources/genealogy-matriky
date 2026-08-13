# Matriky: stahovač + OCR extrakce + rodokmen

Nástroje pro práci s matrikami z digitálních čítáren dvou archivů — od stažení
skenů až po interaktivní rodokmen:

- **`ebadatelna-dl`** (tento adresář) — stahuje snímky knihy ze čtenárny
  Státního oblastního archivu v Praze
  ([ebadatelna.soapraha.cz](https://ebadatelna.soapraha.cz)) v plném rozlišení
  a zapisuje `meta.json` (typ knihy, datace, lokality…).
- **[`actapublica-dl/`](actapublica-dl/)** — totéž pro Acta Publica
  ([actapublica.eu](https://actapublica.eu)), digitální archiv MZA Brno —
  jiný stack (PHP + OpenSeadragon + IIIF/IIPImage), proto samostatná utilita
  se stejným výstupním kontraktem (`meta.json`, `Nazev [ID]/0001.jpg`).
- **[`matrika-ocr/`](matrika-ocr/)** — posílá skeny do OCR modelu (Qwen) a dělá
  strukturovanou extrakci do JSONL podle schématu sloupců + lint.
- **[`genealogy/`](genealogy/)** — z JSONL staví SQLite databázi osob a vazeb
  (extrakce zmínek, automatické propojování osob s mírou jistoty) a servíruje
  API + webové UI.
- **[`web/`](web/)** — React aplikace: interaktivní strom (vrstvy
  narození/svatby/úmrtí, filtry, proklik na sken matriky, merge/split oprav)
  a analytika rodinných vazeb.

Pipeline: `ebadatelna-dl` / `actapublica-dl` → `matrika-ocr` →
`genealogy ingest/extract/match` → `genealogy serve` + web.

Read-only verze webu se dá nasadit na **GitHub Pages**: `genealogy export`
vygeneruje JSON snapshot do `web/public/data/` a workflow
`.github/workflows/pages.yml` ho s buildem webu publikuje (viz
[`genealogy/README.md`](genealogy/README.md)).

Data (stažené skeny, `meta.json`, OCR výstupy, databáze) nejsou verzována —
viz `.gitignore`.

---

## ebadatelna-downloader

Pro danou knihu stáhne všechny snímky v plném rozlišení do složky
`Nazev [ID]/0001.png` (řeší chybu, kdy jiný stahovač vrací identické stránky).

## Použití

```sh
make build                      # přeloží ./ebadatelna-dl
make download ID=8386           # stáhne celou knihu 8386 (Kladno-ev 01) + meta.json
make download ID=8386 PAGES=3   # jen první 3 strany (rychlý test)
make download ID=6367 OUT=data  # jiná kniha do adresáře data/

# doplnit meta.json k už stažené knize (bez stahování obrázků)
make meta IN="Kladno-ev 01 [8386]"
```

Nebo přímo:

```sh
./ebadatelna-dl -id 8386 -out . -delay 500ms
```

### Parametry

| Makefile | flag       | význam                                    | default |
|----------|------------|-------------------------------------------|---------|
| `ID`     | `-id`      | ID knihy (matrikaId), **povinné**         | 8386    |
| `OUT`    | `-out`     | kořenový výstupní adresář                  | `.`     |
| `PAGES`  | `-pages`   | počet stran; `0` = auto-detekce z HTML     | 0       |
| `START`  | `-start`   | první strana                              | 1       |
| `END`    | `-end`     | poslední strana; `0` = do konce           | 0       |
| `DELAY`  | `-delay`   | pauza mezi stranami (slušnost k serveru)  | 500ms   |
| `RETRIES`| `-retries` | počet opakování na stranu                 | 3       |
| `IN`     | `-in`      | složka knihy pro `-meta-only` (ID z `[ID]`) | —     |
|          | `-meta-only` | jen zapsat `meta.json`, bez obrázků     | false   |
|          | `-force-meta`| přepsat existující `meta.json`          | false   |

Kde vzít `ID`: ve výsledcích hledání odkaz na knihu vede na
`/pages/MatrikaPage/matrikaId/{ID}`; totéž `ID` je i v URL prohlížeče
`/d/{ID}/{strana}`.

## meta.json (typ knihy pro OCR)

Ke každé knize se zapíše `meta.json` s metadaty z
`/pages/MatrikaPage/matrikaId/{ID}`: **typ** (`narozeni`/`oddani`/`umrti`/
`kombinovana` — pozná se podle vyplněných rozsahů N/O/Z), název, datace, okres,
původce, počet listů/skenů, lokality, poznámka a `schema_ref`. Slouží jako vstup
pro `matrika-ocr` (vybere podle `typ` správné schéma sloupců). Kombinované starší
knihy (N+O+Z v jedné) se rozpoznají a označí `typ=kombinovana`.

## Vlastnosti

- **Správné, různé snímky** — hlavní důvod vzniku. Web běží na Apache Wicket a
  je stavový: obrázek se neřídí číslem strany v URL, ale stavem session a
  render-counterem. Naivní stahování `/d/{id}/{N}?1--scanImage` proto vrací pro
  každé `N` **tentýž** obrázek. Utilita místo toho v jedné trvalé session načte
  HTML strany, vyparsuje aktuální `#scanImage` odkaz (s platným counterem) a
  stáhne přesně ten.
- **Resume** — už stažené neprázdné soubory přeskočí.
- **Detekce formátu** podle magic bytes (PNG/JPEG); když server vrátí HTML
  (např. vyžaduje přihlášení), soubor se neuloží a nahlásí se chyba.
- **Pojistka proti duplicitám** — počítá SHA-256 každého snímku a na konci
  varuje, pokud jsou snímky identické (příznak selhání).

## Poznámky

- Kniha 8386 je veřejná a stahuje se bez přihlášení. U novějších knih může web
  vyžadovat přihlášení — přihlašování tato utilita neřeší.
- Stahuje se v pořadí strana po straně (2 requesty na stranu: HTML + obrázek),
  což je nutné pro získání správného obrázku.

---

## actapublica-dl (MZA Brno)

Stahovač pro **Acta Publica** ([actapublica.eu](https://actapublica.eu)) —
digitální archiv Moravského zemského archivu v Brně. Úplně jiný stack než
`ebadatelna-dl` (PHP + OpenSeadragon + IIPImage/IIIF místo Apache Wicket),
proto samostatný adresář s vlastním `go.mod`, ale stejný výstupní kontrakt
(`Nazev [ID]/0001.jpg` + `meta.json`), který konzumují `matrika-ocr` a
`genealogy` beze změny.

### Použití

```sh
cd actapublica-dl
make build                     # přeloží ./actapublica-dl
make list OBEC=2787            # jen vypsat knihy obce (obec_id), nestahovat
make download ID=5226 PAGES=2  # rychlý test: první 2 skeny jedné knihy
make download OBEC=2787        # stáhnout VŠECHNY knihy obce (může být desítky GB)
make meta ID=5226               # jen meta.json, bez obrázků
```

Nebo přímo:

```sh
./actapublica-dl -obec 2787 -out . -delay 500ms
```

### Parametry

| Makefile  | flag         | význam                                          | default |
|-----------|--------------|--------------------------------------------------|---------|
| `OBEC`    | `-obec`      | obec_id — vylistuje/stáhne všechny knihy obce   | —       |
| `ID`      | `-id`        | detail id jedné knihy (jde kombinovat s `-obec`) | —      |
|           | `-list`      | jen vypsat knihy (vyžaduje `-obec`), nestahovat | false   |
| `OUT`     | `-out`       | kořenový výstupní adresář                        | `.`     |
| `START`/`END` | `-start`/`-end` | rozsah skenů v knize                       | 1 / 0   |
| `PAGES`   | `-pages`     | kolik skenů stáhnout od `-start`; 0 = celá kniha | 0      |
| `DELAY`   | `-delay`     | pauza mezi skeny (slušnost k serveru)            | 500ms   |
| `RETRIES` | `-retries`   | počet opakování na dlaždici/požadavek            | 3       |
| `QUALITY` | `-quality`   | JPEG kvalita sešitého snímku                     | 92      |
|           | `-meta-only` | jen zapsat `meta.json`, bez obrázků              | false   |
| `DERIVE`  | `-derive`    | `none`\|`halves` — OCR odvozeniny (L/R půlky ≤2576px pro `matrika-ocr`) | `none` |

Kde vzít `obec_id`: v URL výsledků hledání na Acta Publica
(`?typ=obec&obec_id=<ID>`). Detail id knihy je z výpisu (`make list`) nebo
z URL `/matrika/detail/<ID>`.

### Jak to funguje

- **Obrázky nejde stáhnout přímo** — `FIF=`/`JTL=`/`CVT=`/`obj=` na iipsrv
  vrací 403, `matrika/get_image/<id>/jpg` přesměruje na přihlášení. Jediná
  průchozí cesta je **IIIF Image API** (`iipsrv.fcgi?IIIF=…`).
- **Výstup IIPServeru je zastropovaný na 2000 px** v každém rozměru — `full/full`
  proto vrátí jen podvzorkovaný náhled. Nativní rozlišení jde získat jen po
  regionech ≤2000×2000 (ty se vrací 1:1), typicky mřížka 4×3 = 12 requestů na
  stranu, které `iiif.go` sešije přes `image/draw` do jednoho JPEGu.
- **Pořadí skenů** se bere z pole `CreateSeadragon(...)` na stránce detailu
  knihy, ne z číslování souborů (`…00050`, `…00055`, …) — to není aritmetické.
- **`-derive halves`** navazuje na budoucí OCR (`matrika-ocr`): rozřízne
  sešitou dvojstranu na levou/pravou půlku stejnou geometrií jako
  `matrika-ocr/split.go` (`cropHalf`), zmenší na ≤2576 px (strop Sonnetu 5) a
  uloží do `ocr/0001-L.jpg` / `-R.jpg`.
- **`meta.json`** je nadmnožina kontraktu z `ebadatelna-dl` (`meta.go`) — navíc
  `archive`, `obec_id`, `book_no`, `volume`, `provenance_type`, `binding`,
  `language`, `provenance_note`, `scan_files[]` (index/jp2/iiif/rozměry/viewer
  URL). Popisuje vždy CELOU knihu (i skeny mimo právě stahovaný rozsah), takže
  konzumenti znají plný inventář dřív, než se stáhnou všechny obrázky. Nová
  hodnota `typ: "rejstrik"` pro knihy s jen abecedním rejstříkem (žádný
  strukturovaný N/O/Z rozsah) — patří do `transcribe` režimu OCR, ne do
  strukturované extrakce.

### Co je (a co není) ověřené

Endpointy, IIIF chování a regex na jp2 cesty (`Deepzoom=([^"]+\.jp2)\.dzi`)
jsou z živého měření na `obec_id=2787` (Sudoměřice, okres Hodonín — farnost
Strážnice, 21 knih, ~4060 skenů). **Mapování sloupců výsledků hledání**
(`search.go`) a **scraping popisných polí stránky detailu** (`detail.go`,
label/value heuristika) jsou odvozené/best-effort — přesná struktura HTML
nebyla ověřena z prostředí, kde se to psalo (přímý HTTP přístup na
`actapublica.eu` je odsud blokovaný Cloudflare bot-ochranou). Obě místa
degradují bezpečně (prázdná pole / `typ: "unknown"`, nikdy pád), a `bookRow.Cells`
nese syrové buňky řádku pro rychlou opravu mapování po prvním `make list`.
Než spustíš plné stažení, ověř výstup `make list OBEC=<id>` a
`make download ID=<id> PAGES=2` proti očekávání.

**Plné stažení nedělat v efemérním kontejneru** — desítky GB a data jsou
v `.gitignore`. Doporučený lokální běh:
`./actapublica-dl -obec 2787 -out . -delay 500ms` (řádově 10 000+ HTTP
requestů na obec, počítej s hodinami).
