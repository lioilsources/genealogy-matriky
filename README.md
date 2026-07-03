# Matriky: stahovač + OCR extrakce + rodokmen

Nástroje pro práci s matrikami ze čtenárny Státního oblastního archivu v Praze
([ebadatelna.soapraha.cz](https://ebadatelna.soapraha.cz)) — od stažení skenů
až po interaktivní rodokmen:

- **`ebadatelna-dl`** (tento adresář) — stahuje snímky knihy v plném rozlišení
  a zapisuje `meta.json` (typ knihy, datace, lokality…).
- **[`matrika-ocr/`](matrika-ocr/)** — posílá skeny do OCR modelu (Qwen) a dělá
  strukturovanou extrakci do JSONL podle schématu sloupců + lint.
- **[`genealogy/`](genealogy/)** — z JSONL staví SQLite databázi osob a vazeb
  (extrakce zmínek, automatické propojování osob s mírou jistoty) a servíruje
  API + webové UI.
- **[`web/`](web/)** — React aplikace: interaktivní strom (vrstvy
  narození/svatby/úmrtí, filtry, proklik na sken matriky, merge/split oprav)
  a analytika rodinných vazeb.

Pipeline: `ebadatelna-dl` → `matrika-ocr` → `genealogy ingest/extract/match`
→ `genealogy serve` + web.

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
