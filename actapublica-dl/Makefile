# Parametry (lze přepsat na řádce: make download OBEC=2787 nebo make download ID=5226)
OBEC    ?=
ID      ?=
OUT     ?= .
START   ?= 1
END     ?= 0
PAGES   ?= 0
DELAY   ?= 500ms
RETRIES ?= 3
QUALITY ?= 92
DERIVE  ?= none

TARGET := $(if $(OBEC),-obec $(OBEC),)$(if $(ID), -id $(ID),)

BIN := actapublica-dl

.PHONY: build list download meta clean help

help: ## Zobrazit tuto nápovědu
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Přeložit utilitu
	go build -o $(BIN) .

list: build ## Vypsat knihy obce (OBEC=2787), nestahovat
	@test -n "$(OBEC)" || { echo "chyba: zadej OBEC=<obec_id>"; exit 2; }
	./$(BIN) -obec $(OBEC) -list

download: build ## Stáhnout knihu (ID=) nebo celou obec (OBEC=); PAGES pro rychlý test
	@test -n "$(TARGET)" || { echo "chyba: zadej OBEC=<obec_id> nebo ID=<detail id>"; exit 2; }
	./$(BIN) $(TARGET) -out "$(OUT)" -start $(START) -end $(END) -pages $(PAGES) \
		-delay $(DELAY) -retries $(RETRIES) -quality $(QUALITY) -derive $(DERIVE)

meta: build ## Jen meta.json (bez obrázků), ID= nebo OBEC=
	@test -n "$(TARGET)" || { echo "chyba: zadej OBEC=<obec_id> nebo ID=<detail id>"; exit 2; }
	./$(BIN) $(TARGET) -out "$(OUT)" -meta-only

clean: ## Smazat přeložený binárník
	rm -f $(BIN)
