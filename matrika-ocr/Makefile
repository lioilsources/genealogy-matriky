# Parametry (přepiš na řádce: make ocr IN="../Kladno-ev 01 [8386]" START=1 END=20)
IN       ?= ../Kladno-ev 01 [8386]   # adresář matriky/složka se skeny
MODE     ?= structured               # structured | transcribe | report
START    ?= 1                        # první stránka rozsahu (1-based)
END      ?= 0                        # poslední stránka včetně (0 = do konce)
LIMIT    ?= 0                        # zpracuj max N (0 = bez limitu; pro rychlý test)

# OUT prázdné = auto: structured → ocr-out/<typ>/<kniha>.jsonl (dle meta.json).
OUT      ?=
SCHEMA   ?=                          # override schématu (jinak dle meta.typ)
OUTFLAG    := $(if $(OUT),--out "$(OUT)",)
SCHEMAFLAG := $(if $(SCHEMA),--schema "$(SCHEMA)",)

BASEURL  ?= http://192.168.88.66:8080
MODEL    ?= ocr
DELAY    ?= 1500ms
TIMEOUT  ?= 180s
RETRIES  ?= 3
MAXTOK   ?= 2048
MAXSIDE  ?= 0
FORCE    ?=                           # FORCE=1 obejde kontrolu modelu (ne-Qwen)
FORCEFLAG := $(if $(FORCE),--force,)

BIN := matrika-ocr

.PHONY: build ocr test report clean help

help: ## Zobrazit nápovědu
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

build: ## Přeložit nástroj
	go build -o $(BIN) .

ocr: build ## OCR na IN (MODE, rozsah START..END); structured → ocr-out/<typ>/<kniha>.jsonl
	./$(BIN) --in "$(IN)" --mode $(MODE) $(OUTFLAG) $(SCHEMAFLAG) \
		--start $(START) --end $(END) --limit $(LIMIT) \
		--base-url $(BASEURL) --model $(MODEL) --delay $(DELAY) --timeout $(TIMEOUT) \
		--retries $(RETRIES) --max-tokens $(MAXTOK) --max-side $(MAXSIDE) $(FORCEFLAG)

test: build ## Rychlý test: první 2 stránky z IN
	./$(BIN) --in "$(IN)" --mode $(MODE) $(OUTFLAG) $(SCHEMAFLAG) --limit 2 \
		--base-url $(BASEURL) --model $(MODEL) $(FORCEFLAG)

report: build ## Přepočítat report nad existujícím JSONL (zadej OUT=cesta.jsonl)
	@test -n "$(OUT)" || { echo "chyba: zadej OUT=ocr-out/<typ>/<kniha>.jsonl"; exit 2; }
	./$(BIN) --mode report --out "$(OUT)"

clean: ## Smazat binárku
	rm -f $(BIN)
