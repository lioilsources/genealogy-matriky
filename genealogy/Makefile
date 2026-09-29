BIN     := genealogy
DB      ?= data/genealogy.db
JSONL   ?= ../matrika-ocr/ocr-out/*/*.jsonl
BOOKS   ?= ..
ADDR    ?= :8090
WEB     ?= ../web/dist

.PHONY: build ingest extract match serve test all dump-pairs

build:
	go build -o $(BIN) .

test:
	go test ./...

ingest: build
	./$(BIN) ingest --db $(DB) --books-root "$(BOOKS)" $(JSONL)

extract: build
	./$(BIN) extract --db $(DB)

match: build
	./$(BIN) match --db $(DB)

serve: build
	./$(BIN) serve --db $(DB) --addr $(ADDR) --web $(WEB)

# celý pipeline nad novými OCR výstupy
all: ingest extract match

# páry z blockingu + features pro ML (ml/ML_PLAN.md, lekce 1)
dump-pairs: build
	./$(BIN) match --db $(DB) --dump-pairs ml/data/pairs.jsonl
