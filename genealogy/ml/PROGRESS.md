# PROGRESS

Stav studia podle [ML_PLAN.md](ML_PLAN.md). Claude Code čte na začátku každé session.

## Kde jsme

**Lekce 1 (Labely) — připravena, nezačata.** Infrastruktura hotová, kód lekce píše student.

## Log

| Datum | Lekce | Změna / experiment | val F1 | test F1 | Poznámka |
|-------|-------|--------------------|--------|---------|----------|
| 2026-09-29 | 0 | Setup: `ml/` adresář, `match --dump-pairs`, venv | — | — | 5 454 párů z blockingu, **0 labelů** (viz ML_PLAN *Realita dat*). LightGBM potřebuje `brew install libomp`. |

## Lekce 1 — zadání pro studenta

**Cíl:** `export_pairs.py` → `data/labeled.parquet` (nebo CSV) + krátký výpis statistik.

1. `make dump-pairs` → `data/pairs.jsonl`. Načti do pandas (`pd.json_normalize`).
2. Připoj label z `constraints` (`must_link`=1, `cannot_link`=0). Pozor: constraint je
   neuspořádaný pár, dump má vždy `a < b`.
3. Spočítej: kolik párů má label, poměr tříd, kolik constraints **v dumpu vůbec není**
   (neprošly blockingem — co to znamená pro recall celého systému?).
4. Testov: z asserts v `pipeline_test.go` sepiš ground truth (pár rolí → 1/0) jako malý
   soubor. Kolik labelů to dá?
5. Rozhodni, jak nalabelovat víc. Než začneš klikat v UI, odpověz:
   - Když budu labelovat jen páry, které UI nabízí (skóre 0.50–0.72), co se model
     nikdy nenaučí?
   - Kolik negativů na jeden pozitiv je v dumpu? Co by udělal model, který vždy řekne „ne"?
   - Proč `match_candidates.accepted` nesmí být label?

**Poznámka do NOTES:** `supervised-learning.md` a `class-imbalance.md`.

**Otázky na začátek lekce 2** (spaced repetition): (1) Proč nestačí labelovat jen pozitivní
páry? (2) Co je label a proč `rule_score` label není?
