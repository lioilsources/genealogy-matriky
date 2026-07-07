# Fine-tuning pipeline: Mac (export/eval) + DGX Spark (trénink/serving).
# Kroky podrobně: how-to.md. Parametry lze přepsat: make eval-base URL=http://spark:8000/v1

VENV      := .venv
PY        := $(VENV)/bin/python

# --- Spark / síť (přepiš dle svého prostředí) ---
SPARK     ?= ol1n@spark            # ssh cíl DGX Spark
DEST      ?= ~/matriky-ft          # pracovní adresář na Sparku
URL       ?= http://spark:8000/v1  # vLLM OpenAI-kompatibilní endpoint
BASE      ?= Qwen/Qwen2.5-VL-7B-Instruct
TUNED     ?= qwen-matriky
OUT       ?= out/qwen-matriky-lora

# --- exporty ---
MAX_SIDE_PAGES ?= 1280
MAX_SIDE_ROWS  ?= 1536
LIMIT          ?= 0                # eval: 0 = celá val sada

.PHONY: help venv pages rows dataset qa push dryrun train serve eval-base eval-tuned clean

help: ## Zobrazit tuto nápovědu
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

venv: ## [Mac] Vytvořit venv a nainstalovat závislosti (pillow, opencv, numpy)
	python3 -m venv $(VENV)
	$(VENV)/bin/pip install -q -r requirements.txt

pages: ## [Mac] Fáze A: export půlstran z ocr-out → dataset/pages_{train,val}.jsonl
	$(PY) export_pages.py --max-side $(MAX_SIDE_PAGES)

rows: ## [Mac] Fáze C: export řádkových ořezů → dataset/rows_{train,val}.jsonl
	$(PY) export_rows.py --max-side $(MAX_SIDE_ROWS)

dataset: pages rows ## [Mac] Kompletní export datasetu (pages + rows)

qa: ## [Mac] Otevřít kontaktní archy a debug detekce k vizuální kontrole
	open dataset/contact_sheets dataset/debug_rows

push: ## [Mac] Nahrát dataset + trénovací skript na Spark (SPARK=, DEST=)
	ssh $(SPARK) "mkdir -p $(DEST)"
	rsync -a --info=progress2 dataset train_unsloth.py requirements.txt $(SPARK):$(DEST)/

dryrun: ## [Spark] Ověřovací trénink 20 kroků (loss musí klesat)
	$(MAKE) _flush-cache
	python3 train_unsloth.py --data dataset --max-steps 20 --output out/dryrun

train: ## [Spark] Plný QLoRA trénink + merge bf16 checkpointu pro vLLM
	$(MAKE) _flush-cache
	python3 train_unsloth.py --data dataset --epochs 2 --merge --output $(OUT)

serve: ## [Spark] Servírovat vyladěný model přes vLLM (OpenAI API)
	vllm serve $(OUT)-merged --served-model-name $(TUNED)

eval-base: ## [Mac] Baseline eval základního modelu (URL=, BASE=, LIMIT=)
	$(PY) eval_cer.py --url $(URL) --model $(BASE) --limit $(LIMIT) --out report_base.json

eval-tuned: ## [Mac] Eval vyladěného modelu — musí porazit report_base.json
	$(PY) eval_cer.py --url $(URL) --model $(TUNED) --limit $(LIMIT) --out report_tuned.json

_flush-cache: # Spark playbook: uvolnit UMA buffer cache mezi běhy
	-sync && sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches'

clean: ## Smazat vygenerovaný dataset a reporty (zdrojová GT v ocr-out zůstávají)
	rm -rf dataset report_base.json report_tuned.json
