-- Schéma genealogické DB. Vrstvy: raw (books/scans/records) → mentions/events
-- → persons/relations. Vše pod uživatelskými tabulkami (cell_corrections,
-- constraints) je odvozené a lze je kdykoli přestavět příkazy extract/match.

PRAGMA journal_mode = WAL;

-- Vrstva 0: provenience (immutable po ingestu)
CREATE TABLE IF NOT EXISTS books (
    id             TEXT PRIMARY KEY,          -- id z ebadatelny, např. "8386"
    name           TEXT NOT NULL,
    typ            TEXT NOT NULL,             -- narozeni|oddani|umrti|kombinovana|unknown
    district       TEXT,
    localities_json TEXT,
    meta_json      TEXT,                      -- celý meta.json pro dohledání
    scans_dir      TEXT                       -- absolutní cesta ke složce se skeny
);

CREATE TABLE IF NOT EXISTS scans (
    id        INTEGER PRIMARY KEY,
    book_id   TEXT NOT NULL REFERENCES books(id),
    file      TEXT NOT NULL,                  -- "0006.jpg"
    folio     TEXT,
    rok       TEXT,
    ok        INTEGER NOT NULL,
    lint_json TEXT,
    UNIQUE (book_id, file)
);

CREATE TABLE IF NOT EXISTS records (
    id          INTEGER PRIMARY KEY,
    scan_id     INTEGER NOT NULL REFERENCES scans(id),
    row_idx     INTEGER NOT NULL,             -- pořadí řádku na skenu (0-based)
    record_type TEXT NOT NULL,                -- narozeni|oddani|umrti
    cislo       TEXT,
    cells_json  TEXT NOT NULL,                -- surový OCR řádek beze změny
    UNIQUE (scan_id, row_idx)
);
CREATE INDEX IF NOT EXISTS idx_records_scan ON records(scan_id);

-- Uživatelské opravy OCR buněk; extract čte opravenou hodnotu místo raw.
CREATE TABLE IF NOT EXISTS cell_corrections (
    id              INTEGER PRIMARY KEY,
    record_id       INTEGER NOT NULL REFERENCES records(id),
    cell_key        TEXT NOT NULL,
    corrected_value TEXT NOT NULL,
    note            TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (record_id, cell_key)
);

-- Vrstva 1: výstup extrakce (rebuild příkazem extract)
CREATE TABLE IF NOT EXISTS events (
    id             INTEGER PRIMARY KEY,
    record_id      INTEGER NOT NULL UNIQUE REFERENCES records(id),
    type           TEXT NOT NULL,             -- birth|marriage|death
    date_text      TEXT,
    date           TEXT,                      -- ISO YYYY-MM-DD (dle přesnosti zkráceno)
    date_precision TEXT,                      -- day|month|year|none
    year           INTEGER,
    place_text     TEXT,
    house_no       TEXT
);
CREATE INDEX IF NOT EXISTS idx_events_year ON events(year);

CREATE TABLE IF NOT EXISTS mentions (
    id           INTEGER PRIMARY KEY,
    record_id    INTEGER NOT NULL REFERENCES records(id),
    role         TEXT NOT NULL,               -- dite|otec|matka|kmotr|babka|zenich|nevesta|
                                              -- zenich_otec|zenich_matka|nevesta_otec|nevesta_matka|
                                              -- svedek|zemrely|zemrely_otec|zemrely_matka
    ordinal      INTEGER NOT NULL DEFAULT 0,  -- pro víc svědků/kmotrů z jedné buňky
    raw_text     TEXT NOT NULL,
    given_name   TEXT,
    surname      TEXT,
    given_norm   TEXT,                        -- kanonizovaná varianta bez diakritiky
    surname_norm TEXT,                        -- bez diakritiky, bez -ova
    maiden_norm  TEXT,                        -- rozená (roz.) — normalizovaná
    sex          TEXT,                        -- m|f|''
    occupation   TEXT,
    marital_status TEXT,
    place_text   TEXT,
    birth_year   INTEGER,                     -- explicitní DOB (nevěsta na oddacím) / odvozeno z věku
    age_text     TEXT,
    extra_json   TEXT,
    UNIQUE (record_id, role, ordinal)
);
CREATE INDEX IF NOT EXISTS idx_mentions_record ON mentions(record_id);
CREATE INDEX IF NOT EXISTS idx_mentions_surname ON mentions(surname_norm);

CREATE TABLE IF NOT EXISTS name_variants (
    canonical TEXT NOT NULL,
    variant   TEXT NOT NULL,
    kind      TEXT NOT NULL,                  -- given|surname
    PRIMARY KEY (variant, kind)
);

CREATE TABLE IF NOT EXISTS llm_cache (
    cell_hash      TEXT PRIMARY KEY,          -- sha256(buňka) + verze promptu
    prompt_version INTEGER NOT NULL,
    response_json  TEXT NOT NULL
);

-- Vrstva 2: JEDINÝ uživatelský vstup matcheru
CREATE TABLE IF NOT EXISTS constraints (
    id         INTEGER PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('must_link','cannot_link')),
    mention_a  INTEGER NOT NULL REFERENCES mentions(id),
    mention_b  INTEGER NOT NULL REFERENCES mentions(id),
    note       TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Vrstva 3: výstup matcheru (rebuild při každém běhu, person id stabilní)
CREATE TABLE IF NOT EXISTS match_runs (
    id          INTEGER PRIMARY KEY,
    started_at  TEXT NOT NULL,
    params_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS match_candidates (
    run_id        INTEGER NOT NULL REFERENCES match_runs(id),
    mention_a     INTEGER NOT NULL,
    mention_b     INTEGER NOT NULL,
    score         REAL NOT NULL,
    features_json TEXT,
    accepted      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_candidates_run ON match_candidates(run_id);

CREATE TABLE IF NOT EXISTS persons (
    id             INTEGER PRIMARY KEY,       -- stabilní napříč běhy (dědí se dle překryvu mentions)
    display_name   TEXT NOT NULL,
    sex            TEXT,
    birth_year_est INTEGER,
    death_year_est INTEGER,
    confidence     REAL NOT NULL DEFAULT 1.0, -- min párové skóre uvnitř clusteru
    first_seen_run INTEGER
);

CREATE TABLE IF NOT EXISTS person_mentions (
    person_id  INTEGER NOT NULL REFERENCES persons(id),
    mention_id INTEGER NOT NULL UNIQUE REFERENCES mentions(id),
    confidence REAL NOT NULL DEFAULT 1.0,
    source     TEXT NOT NULL DEFAULT 'auto'   -- auto|constraint
);
CREATE INDEX IF NOT EXISTS idx_pm_person ON person_mentions(person_id);

CREATE TABLE IF NOT EXISTS relations (
    id            INTEGER PRIMARY KEY,
    type          TEXT NOT NULL,              -- parent_child|spouse
    person_a      INTEGER NOT NULL REFERENCES persons(id),  -- parent / manžel A
    person_b      INTEGER NOT NULL REFERENCES persons(id),  -- child / manžel B
    confidence    REAL NOT NULL DEFAULT 1.0,
    evidence_json TEXT,                       -- id záznamů, ze kterých hrana plyne
    source        TEXT NOT NULL DEFAULT 'auto'
);
CREATE INDEX IF NOT EXISTS idx_relations_a ON relations(person_a);
CREATE INDEX IF NOT EXISTS idx_relations_b ON relations(person_b);
