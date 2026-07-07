# Konvence přepisu matrik (teacher = Claude, žák = Qwen-VL)

Jednotné konvence pro VŠECHNY dávky OCR. Model se učí přesně to, co teacher píše —
nekonzistence = šum v datasetu. Při nové dávce přepisů tento soubor přiložit k promptu
nebo dodržet zpaměti.

## Obecné
- **Pravopis jmen zachovat, jak je psán** v prameni (Worzechowsky ≠ opravovat na
  Vořechovský; Kaškova/Kašíková jak stojí). Interpretaci dělá až ETL/matcher.
- **Nejistota**: nejisté čtení označit `(?)` hned za nejistým slovem — `Kutík(?)`.
- **Nečitelné**: `[nečitelné]`. Chybějící/prázdná buňka: prázdný řetězec `""`.
- **Nevymýšlet**: co na skenu není, nepsat. Radši prázdno než konfabulace.
- **Pořadí záznamů** shora dolů; prázdné řádky formuláře přeskočit.
- Víc hodnot v jedné buňce (svědci, kmotři): oddělit `"; "`.
- Škrtnuté zachytit jako text s poznámkou v `poznamenani` (`škrtnuto: …`).

## Data (datumy)
- Formát ve strukturovaných buňkách: `D. měsíc RRRR` slovně — `27. červen 1840`,
  `14., 21., 28. duben 1901` (více ohlášek čárkami).
- Roky bez interpretace kalendáře; latinské měsíce v prose ponechat latinsky
  (`7bris` → `7bris [září]` jen je-li jistota).

## Zkratky a znaky
- `+` před jménem = zemřelý (ponechat: `+ Františka Váni`).
- Obecné zkratky ponechat (`manž.`, `kř.`, `okr.`, `č.`, `roz.`); rozepisovat jen
  jednoznačné ligatury/značky domů (`N°`, `Häus` → `č.`).
- Ditto značky (`detto`, `"`,  `—//—`) **rozepsat plnou hodnotou** z řádku výše.
- Číslo domu psát `č.7` (bez mezery); okres/panství do závorky jen je-li v prameni.

## Jazyky a písmo
- **Latina** (≤ ~1784): přepis latinsky, beze změn pádů; česká jména nechat v dobové
  grafice (Worzechowsky). Humanistika i kurent dle originálu.
- **Němčina** (~1784–1860 úřední záhlaví): přepis německy, ß/ae/oe zachovat.
- **Čeština**: dobový pravopis zachovat (w→v NEnormalizovat: `Wes Pchera`).

## Strukturovaný režim (typy narozeni/oddani/umrti)
- Klíče a tvar přesně dle `matrika-ocr/schemas/<typ>.json`; hodnoty stručné prosté
  řetězce; výstup `{"folio":"","rok":"","rows":[…]}` bez markdown.
- `folio`/`rok` z tištěné/psané hlavičky strany (ne z datumů záznamů).
- Prázdné strany/desky/indexy: `{"folio":"","rok":"","rows":[]}` — DŮLEŽITÉ,
  tyto negativní vzorky učí model nehalucinovat.

## Transcribe režim (staré knihy bez tabulky, próza)
- Plný přepis po řádcích, zachovat řádkování originálu; nadpisy vsí/roků na
  samostatném řádku tak, jak stojí.
