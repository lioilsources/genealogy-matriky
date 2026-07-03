# Parametry (lze přepsat na příkazové řádce: make download ID=6367)
ID      ?= 8386
OUT     ?= .
IN      ?=
PAGES   ?= 0
START   ?= 1
END     ?= 0
DELAY   ?= 500ms
RETRIES ?= 3

BIN := ebadatelna-dl

.PHONY: build run download meta clean help

help: ## Zobrazit tuto nápovědu
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Přeložit utilitu
	go build -o $(BIN) .

run: build ## Stáhnout knihu (viz parametry ID, OUT, PAGES, START, END, DELAY)
	./$(BIN) -id $(ID) -out $(OUT) -pages $(PAGES) -start $(START) -end $(END) \
		-delay $(DELAY) -retries $(RETRIES)

download: run ## Alias pro `run`

meta: build ## Doplnit/přegenerovat meta.json k už stažené knize (IN="Nazev [ID]"), bez obrázků
	./$(BIN) -meta-only $(if $(IN),-in "$(IN)",-id $(ID) -out $(OUT)) $(if $(FORCE),-force-meta,)

clean: ## Smazat přeložený binárník
	rm -f $(BIN)
