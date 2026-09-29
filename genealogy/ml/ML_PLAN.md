# ML_PLAN.md — Naučit se ML na vlastním problému: entity resolution v matrikách

> **Instrukce pro Claude Code:** Tento soubor je zadání i studijní plán. Student je senior backend
> inženýr (Go, Python, 20+ let), který **nikdy nedělal ML jako disciplínu**, ale provozuje vlastní
> inference stack na DGX Spark a už má za sebou QLoRA fine-tune Qwen2.5-VL (`matrika-ocr/finetune/`).
> Cíl: naučit se **klasické supervised ML od základů, poctivě, na reálných datech** — a jako vedlejší
> produkt nahradit ručně psaný matcher v `genealogy/match.go` naučeným modelem, který je
> měřitelně lepší. Vysvětluj česky, kód anglicky. Každý koncept uč **ve chvíli, kdy je potřeba**, ne
> dopředu. Teorii drž na úrovni „proč to funguje a kdy to selže", ne odvozování. Kde to jde,
> mapuj na to, co student zná („cross-validation je jako spouštět integrační testy na jiném
> fixture než na tom, na kterém jsi ladil").

## Proč zrovna tento problém

`genealogy match` dnes dělá **record linkage**: blocking → kandidátní páry zmínek →
ručně psané skóre z features (jména, roky, místo, rodiče) → prahy `--auto 0.90 --flag 0.72
--suggest 0.50` → clustering → osoby. Je to deterministické a funguje, ale:

- prahy a váhy jsou odhadnuté rukou, nikdo neví, jestli jsou dobré;
- neexistuje **měřitelná** precision/recall — jen pocit z UI;
- uživatelské `must_link` / `cannot_link` constraints jsou **hotové labely**, které se dnes
  používají jen jako override, ne jako trénovací data.

To je učebnicová supervised klasifikace: `pár zmínek → P(táž osoba)`. Má vlastní data, vlastní
labely, jasnou metriku a existující baseline, kterou je třeba porazit. Přesně to, na čem se ML
učí nejlíp — a přesně doména „data intelligence / knowledge graph" firem, kam se student hlásí.

## Co student umí a co ne

- **Umí:** Python, NumPy (Avast), SQLite, Go pipeline v tomto repu, Docker, Spark (GPU), vLLM,
  QLoRA trénink „podle návodu" (Unsloth), CER eval.
- **Neumí / nikdy nedělal vědomě:** train/val/test split a proč, overfitting, cross-validation,
  precision/recall/F1 a jejich trade-off, feature engineering, logistickou regresi, rozhodovací
  stromy / gradient boosting, kalibraci pravděpodobností, class imbalance, learning curves, jak
  poznat data leakage, jak srovnat model s baseline poctivě.
- **Riziko:** sáhne po LLM („nechám to posoudit Qwen") místo po malém modelu. Pravidlo plánu:
  **LLM až v poslední fázi jako feature/teacher, ne jako klasifikátor.**

## Realita dat (zjištěno při zakládání, 2026-09-29)

Plán vznikl dřív, než se data spočítala. Co platí doopravdy:

- **Labely dnes neexistují.** Lokální `data/genealogy.db` není; DB postavená z
  `matrika-ocr/ocr-out` (321 záznamů, 2 413 zmínek) má `constraints` = 0.
- **`match_candidates.accepted` NENÍ rozhodnutí z UI** — je to rozhodnutí matcheru samotného
  (skóre ≥ `--flag`). Použít ho jako label = učit model napodobit baseline (kruh). Rozhodnutí
  z UI (merge/split/reject) se ukládají výhradně do `constraints`.
- **Testov nemá ground-truth soubor** — pravda je jen implicitně v asserts `pipeline_test.go`
  (8 záznamů, desítky párů). Dobré na sanity check, ne na trénink.
- `--dump-pairs` na reálných datech: **5 454 párů** z blockingu, 1 565 se skóre > 0,
  zbytek zabitý vetem (`name_gate` 2 575, `years_disjoint` 758, `same_record` 475,
  `primary_twice` 81). Dvě dominantní knihy: Pchery 52 [11476] a Kladno-ev 01 [8386].

Důsledek: **lekce 1 je hlavně o tom, odkud labely vzít**, a nevyhnutelně skončí ručním
labelováním (přes UI → `constraints`). Jak labelovat, aby vzorek nebyl zkreslený (nejen
páry, které matcher už spojil!), je jádro lekce.

## Struktura

```
genealogy-matriky/
└── genealogy/
    ├── dump.go                     # HOTOVO: --dump-pairs (páry z blockingu + surové features)
    └── ml/                         # Python
        ├── ML_PLAN.md              # tento soubor
        ├── PROGRESS.md             # Claude Code aktualizuje po každé lekci
        ├── NOTES/                  # jedna markdown poznámka na koncept (student píše vlastními slovy)
        ├── pyproject.toml          # pandas, scikit-learn, lightgbm, matplotlib (+ extras)
        ├── data/                   # exporty z SQLite (gitignore), nikdy ruční úpravy
        ├── export_pairs.py         # 1: kandidátní páry + features + label z DB
        ├── baseline.py             # 2: reprodukce ručního skóre v Pythonu, metriky
        ├── train.py                # 3+: trénink, CV, uložení modelu + reportu
        ├── evaluate.py             # metriky, PR křivka, confusion, chyby k prohlédnutí
        ├── explain.py              # feature importance, SHAP (pozdější lekce)
        ├── export_model.py         # model → JSON/ONNX pro Go
        └── notebooks/              # exploratorní, nikdy zdroj pravdy
```

Python skripty píše **student** v rámci lekcí — proto zatím neexistují.

Go strana (`match.go`) dostane tři malé změny: (a) `--dump-pairs` vypíše kandidátní páry
s feature mapou jako JSONL **(hotovo)**, (b) `--scorer=rules|model` s načtením modelu
(lekce 11), (c) `--eval` proti labelům (lekce 11). Zbytek pipeline (blocking, clustering,
constraints) se **nemění** — učíme se ML, nepřepisujeme systém.

### Formát `pairs.jsonl`

Jeden řádek = jeden pár `(a, b)`, `a.mention < b.mention`, **všechny** páry z blockingu
(i se skóre 0 — jinak by model nikdy neviděl snadné negativy).

```jsonc
{
  "a": {"mention": 291, "record": 38, "book": "8386", "scan": "0009.jpg", "role": "zenich",
        "sex": "m", "name": "Josef Adlt", "event_type": "marriage", "event_year": 1902},
  "b": { /* totéž */ },
  "rule_score": 0.0,            // přesně scorePair() z match.go
  "rule_veto": "same_record",   // same_record|sex|primary_twice|dead_before_event|years_disjoint|name_gate
  "rule_feats": { /* debug features ručního skóre, jak jsou v match_candidates */ },
  "f": {                        // surové features, chybějící = klíč chybí (pandas NaN)
    "given_sim": 0.0, "surname_sim": 1.0, "surname_sim_best": 1.0, "maiden_used": 0,
    "given_exact": 0, "surname_exact": 1, "sex_known_both": 1,
    "birth_year_diff": 0, "event_year_diff": 0, "year_overlap": 45, "year_range_width": 60,
    "same_place": 0, "father_given_sim": 1.0, "father_surname_sim": 1.0,
    "mother_given_sim": 1.0, "parent_rule": 0.0, "same_event_type": 1
  }
}
```

`book`/`record` slouží pro group split (lekce 2), `scan`/`name` pro error analysis proti
skenům (lekce 4). `TestDumpPairs` hlídá, že `rule_score` == `scorePair` 1:1.

## Lekce

Každá lekce = jedna session. Formát: krátký výklad → student píše kód sám (Claude Code
review, ne diktát) → výsledek jako číslo do `PROGRESS.md` → student napíše 5–10 řádků do
`NOTES/<koncept>.md` vlastními slovy (test, že to pochopil).

### Fáze A — data a poctivá baseline (lekce 1–4)

| # | Lekce | Co se udělá | Co se student naučí | Stav |
|---|-------|-------------|---------------------|------|
| 1 | Labely | Export všech párů, které kdy prošly blockingem, s features a **labelem** ze tří zdrojů: `constraints` (must/cannot = 1/0), ~~`match_candidates` přijaté/odmítnuté v UI~~ (viz *Realita dat* — rozhodnutí z UI jsou v `constraints`), a fixture `testdata/Testov` (ground truth). Spočítat kolik labelů je, poměr tříd, kolik párů je bez labelu. | Co je supervised learning, kde se berou labely, **class imbalance** (většina párů je „ne"), proč nelze labelovat jen pozitivní | 🟡 infra hotová |
| 2 | Split | Rozdělit na train/val/test — **po knihách nebo po rodinách, ne náhodně po párech** (jinak stejná osoba leakuje mezi train a test). Zafixovat seed a uložit split do souboru. | Train/val/test, **data leakage**, proč split musí respektovat strukturu dat (group split), reprodukovatelnost | ⬜ |
| 3 | Metriky | Reprodukovat současné ruční skóre v Pythonu z feature mapy (1:1 s `match.go`), spočítat precision, recall, F1 na test setu při prazích 0.90/0.72/0.50. **Tohle je baseline, kterou musíme porazit.** Nakreslit PR křivku. | Precision vs. recall a proč se nedají maximalizovat obě, F1, PR křivka vs. práh, proč accuracy u nevyvážených tříd lže | ⬜ |
| 4 | Chyby | Vypsat 30 nejhorších chyb baseline (false positives s vysokým skóre, false negatives s nízkým) a prohlédnout je proti skenům. Zapsat, **jaké informace ruční skóre nevidí**. | Error analysis jako hlavní nástroj ML inženýra; hypotézy pro features | ⬜ |

### Fáze B — první model (lekce 5–8)

| # | Lekce | Co se udělá | Co se student naučí | Stav |
|---|-------|-------------|---------------------|------|
| 5 | Logistická regrese | Natrénovat `LogisticRegression` na stejných features jako ruční skóre. Srovnat F1 s baseline. Podívat se na koeficienty — **to jsou naučené váhy místo odhadnutých**. | Lineární model, sigmoid, loss, regularizace (C), škálování features, interpretace koeficientů | ⬜ |
| 6 | Cross-validation | Místo jednoho val splitu k-fold (group k-fold po knihách). Learning curve: F1 vs. počet trénovacích párů. | Variance odhadu, overfitting vs. underfitting, kolik dat je „dost", proč se test set otevírá až na konci | ⬜ |
| 7 | Feature engineering | Přidat features z lekce 4: Jaro-Winkler/edit distance jmen místo binární shody, `name_variants` tabulka (Jan/Johann/Joannes), věková konzistence rodič–dítě, shoda rodičů jako samostatný signál, vzdálenost obcí, četnost příjmení v knize (vzácné příjmení = silnější důkaz). Měřit každou feature zvlášť (ablation). | Feature engineering, ablation study, proč doménová znalost > model, one-hot vs. numeric | ⬜ |
| 8 | Gradient boosting | `LightGBM` na stejných features. Ladění pár hyperparametrů přes CV (ne grid search naslepo — vysvětlit, co který dělá). Srovnat s LR a baseline. | Rozhodovací stromy, boosting, proč GBDT vyhrává na tabulkových datech, hyperparametry a jejich vliv, early stopping | ⬜ |

### Fáze C — z modelu produkt (lekce 9–12)

| # | Lekce | Co se udělá | Co se student naučí | Stav |
|---|-------|-------------|---------------------|------|
| 9 | Kalibrace a prahy | Zjistit, jestli P=0.9 z modelu znamená 90 % shod (reliability diagram). Kalibrovat (Platt/isotonic). Odvodit nové `--auto/--flag/--suggest` z PR křivky podle **zvoleného** trade-offu (auto = precision ≥ 0.99, suggest = recall ≥ 0.95). | Kalibrace, proč skóre ≠ pravděpodobnost, volba prahu je produktové rozhodnutí | ⬜ |
| 10 | Vysvětlitelnost | SHAP na vzorku párů; v UI u `match_candidates` ukázat 3 hlavní důvody návrhu. | Feature importance vs. SHAP, proč to u lidského review záleží | ⬜ |
| 11 | Nasazení do Go | Export modelu: LR jako koeficienty v JSON (Go spočítá sám), GBDT přes ONNX + `onnxruntime` nebo jako serializované stromy v JSON s malým Go evaluátorem. `--scorer=model`. Integrační test: stejná data → stejné skóre jako v Pythonu (tolerance 1e-6). | Model serving bez Pythonu, drift mezi train a inference kódem, verzování modelu vedle kódu | ⬜ |
| 12 | Aktivní učení | Model vybírá páry, kde si je nejméně jistý (P≈0.5), ty jdou do UI k rozhodnutí; každé rozhodnutí = nový label; retrain skript. Změřit, kolik labelů zvedne F1 o 1 bod. | Active learning, human-in-the-loop, data flywheel — a proč je tohle to, co firmy jako 1touch prodávají | ⬜ |

### Fáze D — volitelné rozšíření (po fázi C)

| # | Lekce | Co se udělá | Co se student naučí | Stav |
|---|-------|-------------|---------------------|------|
| 13 | LLM jako feature, ne soudce | Pro těžké páry (P 0.3–0.7) nechat lokální Qwen posoudit s kontextem záznamů; jeho verdikt je **další feature** do GBDT, ne finální slovo. Změřit, jestli to pomůže víc než stojí (latence, cena). | Kdy LLM, kdy klasický model, ensembling, cost/benefit | ⬜ |
| 14 | Embeddingy jmen | Charakterové embeddingy jmen (trénované na `name_variants`) místo edit distance; malá síť v PyTorch. První „vlastní neuronka" s vlastním tréninkovým cyklem. | Embeddingy, PyTorch training loop, loss, optimizer, batch — ručně, ne přes Unsloth | ⬜ |
| 15 | Zpět k OCR fine-tune | Vzít existující `finetune/` a projít ho **s ML očima**: je val split poctivý (jiné knihy)? Je CER správná metrika, nebo field-exact? Jaká je learning curve — pomohlo by 2× víc stran? Overfitting adapteru? | Přenos naučeného na deep learning, eval design | ⬜ |

## Pravidla hry

1. **Test set se otevírá jednou**, na konci fáze B a znovu na konci fáze C. Všechno ladění na val/CV.
2. **Baseline je posvátná.** Každý model se srovnává s ručním skóre z lekce 3 na stejném splitu, stejné metrice. Bez toho číslo nic neznamená.
3. **Každý experiment má řádek v `PROGRESS.md`:** datum, změna, val F1, test F1 (jen když je povoleno), poznámka. Neúspěchy se zapisují taky.
4. **Žádný notebook není zdroj pravdy.** Co se ukázalo jako dobré, přejde do `train.py`.
5. **Konceptové poznámky píše student**, ne Claude Code. Claude Code je opraví, pokud jsou špatně.
6. **Žádné LLM do lekce 13.**

## Workflow pro Claude Code

1. Na začátku session přečti `PROGRESS.md` a poslední `NOTES/*.md`; navaž.
2. Nová lekce: 10 minut výkladu s jedním konkrétním příkladem z těchto dat, pak student píše kód. Ty děláš review a pokládáš otázky („proč random split leakuje?", „co se stane s F1, když zdvojnásobíš práh?").
3. Po lekci: student doplní `PROGRESS.md` a napíše `NOTES/<koncept>.md`; ty ho zkontroluješ a opravíš chybné intuice.
4. Spaced repetition: na začátku každé lekce 2 otázky z předchozích (např. „jaký je rozdíl mezi precision a recall a kterou chceme u `--auto`?").
5. Kdykoli student sáhne po LLM před lekcí 13, zastav ho a zeptej se, co konkrétně malý model neumí.

## Setup

```bash
cd genealogy/ml
python3 -m venv .venv && source .venv/bin/activate
pip install pandas scikit-learn matplotlib          # lekce 1–7
brew install libomp && pip install lightgbm         # lekce 8 (macOS: LightGBM bez libomp nenaběhne)
pip install shap onnx onnxmltools skl2onnx jupyter  # lekce 10–11 (vše ověřeno na Pythonu 3.14)

# DB (pokud neexistuje) + export párů:
cd .. && make all            # ingest+extract+match z ../matrika-ocr/ocr-out
make dump-pairs            # = go run . match --db data/genealogy.db --dump-pairs ml/data/pairs.jsonl
```

Spark není pro fáze A–C potřeba — vše běží na CPU v sekundách. GPU přijde na řadu ve fázi D.

## Co z toho bude v CV a na pohovoru

Po fázi C: *„Nahradil jsem ručně psaný record-linkage scorer naučeným GBDT modelem trénovaným na
uživatelských constraints; F1 z X na Y na held-out knihách, kalibrované pravděpodobnosti, SHAP
vysvětlení v UI, model nasazený v Go bez Python runtime, aktivní učení pro sběr labelů."* Každé
slovo v té větě bude student umět obhájit, protože si každé odpracoval.
