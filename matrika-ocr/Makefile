# Parametry (přepiš na řádce: make ocr IN="../Kladno-ev 01 [8386]" START=1 END=20)
# POZOR: žádné inline komentáře za hodnotami — přidaly by koncové mezery.
# IN: složka se skeny | MODE: structured|transcribe|report
# START/END: rozsah stran (1-based, včetně) | LIMIT: max N (0 = vše; pro test)
IN       ?= ../Kladno-ev 01 [8386]
MODE     ?= structured
START    ?= 1
END      ?= 0
LIMIT    ?= 0

# OUT prázdné = auto: structured → ocr-out/<typ>/<kniha>.jsonl (dle meta.json).
# SCHEMA prázdné = dle meta.typ.
OUT      ?=
SCHEMA   ?=
OUTFLAG    := $(if $(strip $(OUT)),--out "$(strip $(OUT))",)
SCHEMAFLAG := $(if $(strip $(SCHEMA)),--schema "$(strip $(SCHEMA))",)

BASEURL  ?= http://192.168.88.66:8080
MODEL    ?= ocr
DELAY    ?= 1500ms
TIMEOUT  ?= 180s
RETRIES  ?= 3
MAXTOK   ?= 2048
MAXSIDE  ?= 0
# FORCE=1 obejde pre-flight kontrolu modelu (ne-Qwen)
FORCE    ?=
FORCEFLAG := $(if $(strip $(FORCE)),--force,)

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

# TESTSTART: od které strany testovat (přeskoč obálku/titul; u 8386 data ~str. 6)
TESTSTART ?= 6
test: build ## Rychlý test: 2 strany od TESTSTART (výchozí 6 = reálná data, ne obálka)
	./$(BIN) --in "$(IN)" --mode $(MODE) $(OUTFLAG) $(SCHEMAFLAG) --start $(TESTSTART) --limit 2 \
		--base-url $(BASEURL) --model $(MODEL) --delay $(DELAY) --timeout $(TIMEOUT) \
		--retries $(RETRIES) $(FORCEFLAG)

report: build ## Přepočítat report nad existujícím JSONL (zadej OUT=cesta.jsonl)
	@test -n "$(OUT)" || { echo "chyba: zadej OUT=ocr-out/<typ>/<kniha>.jsonl"; exit 2; }
	./$(BIN) --mode report --out "$(OUT)"

clean: ## Smazat binárku
	rm -f $(BIN)
