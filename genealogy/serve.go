package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type server struct {
	db  *sql.DB
	web string
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dbPath := fs.String("db", "data/genealogy.db", "cesta k SQLite databázi")
	addr := fs.String("addr", ":8090", "adresa HTTP serveru")
	web := fs.String("web", "web/dist", "složka s buildem web UI (prázdná = jen API)")
	fs.Parse(args)

	db, err := openDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	s := &server{db: db, web: *web}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/persons/{id}", s.handlePerson)
	mux.HandleFunc("GET /api/persons/{id}/neighborhood", s.handleNeighborhood)
	mux.HandleFunc("GET /api/tree", s.handleSurnameTree)
	mux.HandleFunc("GET /api/records/{id}", s.handleRecord)
	mux.HandleFunc("GET /api/scans/{id}/image", s.handleScanImage)
	mux.HandleFunc("GET /api/scans/{id}/full", s.handleScanFull)
	mux.HandleFunc("PATCH /api/records/{id}/cells", s.handleCellPatch)
	mux.HandleFunc("POST /api/constraints", s.handleConstraintCreate)
	mux.HandleFunc("DELETE /api/constraints/{id}", s.handleConstraintDelete)
	mux.HandleFunc("POST /api/persons/{a}/merge/{b}", s.handleMerge)
	mux.HandleFunc("POST /api/persons/{id}/split", s.handleSplit)
	mux.HandleFunc("POST /api/match/run", s.handleMatchRun)
	mux.HandleFunc("GET /api/analytics/{kind}", s.handleAnalytics)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("/", s.handleStatic)

	fmt.Printf("genealogy serve na %s (web: %s)\n", *addr, *web)
	return http.ListenAndServe(*addr, cors(mux))
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// handleStatic servíruje SPA build (fallback na index.html pro klientské routy).
func (s *server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if s.web == "" {
		httpErr(w, 404, fmt.Errorf("web UI není nasazené (spusť s --web web/dist)"))
		return
	}
	path := filepath.Join(s.web, filepath.Clean("/"+r.URL.Path))
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.web, "index.html"))
}

// ---- vyhledávání a osoby ----

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	where := []string{"1=1"}
	var params []any
	// fulltext přes normalizovaná jména zmínek (funguje bez diakritiky i s ní)
	for _, tok := range strings.Fields(foldASCII(q.Get("q"))) {
		where = append(where, `EXISTS (SELECT 1 FROM person_mentions pm JOIN mentions m ON m.id=pm.mention_id
			WHERE pm.person_id=p.id AND (m.given_norm LIKE ? OR m.surname_norm LIKE ?))`)
		params = append(params, tok+"%", tok+"%")
	}
	if v := strings.TrimSpace(q.Get("surname")); v != "" {
		where = append(where, `EXISTS (SELECT 1 FROM person_mentions pm JOIN mentions m ON m.id=pm.mention_id
			WHERE pm.person_id=p.id AND m.surname_norm LIKE ?)`)
		params = append(params, foldASCII(v)+"%")
	}
	if v := strings.TrimSpace(q.Get("place")); v != "" {
		where = append(where, `EXISTS (SELECT 1 FROM person_mentions pm JOIN mentions m ON m.id=pm.mention_id
			WHERE pm.person_id=p.id AND m.place_text LIKE ?)`)
		params = append(params, "%"+v+"%")
	}
	if v := q.Get("year_from"); v != "" {
		where = append(where, `COALESCE(p.birth_year_est, p.death_year_est, 9999) >= ?`)
		params = append(params, v)
	}
	if v := q.Get("year_to"); v != "" {
		where = append(where, `COALESCE(p.birth_year_est, p.death_year_est, 0) <= ?`)
		params = append(params, v)
	}
	rows, err := s.db.Query(`SELECT p.id, p.display_name, p.sex, COALESCE(p.birth_year_est,0),
		COALESCE(p.death_year_est,0), p.confidence,
		(SELECT COUNT(*) FROM person_mentions pm WHERE pm.person_id=p.id)
		FROM persons p WHERE `+strings.Join(where, " AND ")+`
		ORDER BY p.display_name LIMIT 200`, params...)
	if err != nil {
		httpErr(w, 500, err)
		return
	}
	defer rows.Close()
	type hit struct {
		ID          int64   `json:"id"`
		Name        string  `json:"name"`
		Sex         string  `json:"sex"`
		BirthYear   int     `json:"birth_year,omitempty"`
		DeathYear   int     `json:"death_year,omitempty"`
		Confidence  float64 `json:"confidence"`
		MentionCount int    `json:"mention_count"`
	}
	out := []hit{}
	for rows.Next() {
		var h hit
		rows.Scan(&h.ID, &h.Name, &h.Sex, &h.BirthYear, &h.DeathYear, &h.Confidence, &h.MentionCount)
		out = append(out, h)
	}
	writeJSON(w, out)
}

// mentionDetail je zmínka s plnou proveniencí (kniha/sken/folio/řádek).
type mentionDetail struct {
	ID         int64             `json:"id"`
	Role       string            `json:"role"`
	Raw        string            `json:"raw"`
	Given      string            `json:"given"`
	Surname    string            `json:"surname"`
	Place      string            `json:"place,omitempty"`
	Occupation string            `json:"occupation,omitempty"`
	BirthYear  int               `json:"birth_year,omitempty"`
	AgeText    string            `json:"age_text,omitempty"`
	RecordID   int64             `json:"record_id"`
	RowIdx     int               `json:"row_idx"`
	Cislo      string            `json:"cislo,omitempty"`
	ScanID     int64             `json:"scan_id"`
	ScanFile   string            `json:"scan_file"`
	Folio      string            `json:"folio,omitempty"`
	BookID     string            `json:"book_id"`
	BookName   string            `json:"book_name"`
	EventType  string            `json:"event_type,omitempty"`
	EventYear  int               `json:"event_year,omitempty"`
	EventDate  string            `json:"event_date,omitempty"`
	Confidence float64           `json:"confidence"`
	Cells      map[string]string `json:"cells,omitempty"`
}

func (s *server) queryMentions(whereSQL string, params ...any) ([]mentionDetail, error) {
	rows, err := s.db.Query(`SELECT m.id, m.role, m.raw_text, COALESCE(m.given_name,''), COALESCE(m.surname,''),
		COALESCE(m.place_text,''), COALESCE(m.occupation,''), COALESCE(m.birth_year,0), COALESCE(m.age_text,''),
		r.id, r.row_idx, COALESCE(r.cislo,''), sc.id, sc.file, COALESCE(sc.folio,''), b.id, b.name,
		COALESCE(e.type,''), COALESCE(e.year,0), COALESCE(e.date,''), COALESCE(pm.confidence, 1.0)
		FROM mentions m
		JOIN records r ON r.id = m.record_id
		JOIN scans sc ON sc.id = r.scan_id
		JOIN books b ON b.id = sc.book_id
		LEFT JOIN events e ON e.record_id = r.id
		LEFT JOIN person_mentions pm ON pm.mention_id = m.id
		`+whereSQL, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mentionDetail
	for rows.Next() {
		var m mentionDetail
		if err := rows.Scan(&m.ID, &m.Role, &m.Raw, &m.Given, &m.Surname, &m.Place, &m.Occupation,
			&m.BirthYear, &m.AgeText, &m.RecordID, &m.RowIdx, &m.Cislo, &m.ScanID, &m.ScanFile,
			&m.Folio, &m.BookID, &m.BookName, &m.EventType, &m.EventYear, &m.EventDate, &m.Confidence); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *server) handlePerson(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var p struct {
		ID         int64           `json:"id"`
		Name       string          `json:"name"`
		Sex        string          `json:"sex"`
		BirthYear  int             `json:"birth_year,omitempty"`
		DeathYear  int             `json:"death_year,omitempty"`
		Confidence float64         `json:"confidence"`
		Mentions   []mentionDetail `json:"mentions"`
		Candidates []any           `json:"candidates"`
	}
	err := s.db.QueryRow(`SELECT id, display_name, sex, COALESCE(birth_year_est,0),
		COALESCE(death_year_est,0), confidence FROM persons WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.Sex, &p.BirthYear, &p.DeathYear, &p.Confidence)
	if err == sql.ErrNoRows {
		httpErr(w, 404, fmt.Errorf("osoba %d neexistuje", id))
		return
	} else if err != nil {
		httpErr(w, 500, err)
		return
	}
	p.Mentions, err = s.queryMentions(`JOIN person_mentions pm2 ON pm2.mention_id = m.id AND pm2.person_id = ?
		ORDER BY COALESCE(e.year, 0), m.id`, id)
	if err != nil {
		httpErr(w, 500, err)
		return
	}

	// návrhy "možná táž osoba" (pásmo suggest..flag z posledního běhu)
	p.Candidates = []any{}
	crows, err := s.db.Query(`SELECT mc.mention_a, mc.mention_b, mc.score,
		pma.person_id, pmb.person_id
		FROM match_candidates mc
		JOIN person_mentions pma ON pma.mention_id = mc.mention_a
		JOIN person_mentions pmb ON pmb.mention_id = mc.mention_b
		WHERE mc.accepted = 0 AND (pma.person_id = ? OR pmb.person_id = ?)
		ORDER BY mc.score DESC LIMIT 20`, id, id)
	if err == nil {
		for crows.Next() {
			var ma, mb int64
			var score float64
			var pa, pb int64
			crows.Scan(&ma, &mb, &score, &pa, &pb)
			other := pa
			if pa == id {
				other = pb
			}
			var otherName string
			s.db.QueryRow(`SELECT display_name FROM persons WHERE id=?`, other).Scan(&otherName)
			p.Candidates = append(p.Candidates, map[string]any{
				"person_id": other, "person_name": otherName,
				"mention_a": ma, "mention_b": mb, "score": score,
			})
		}
		crows.Close()
	}
	writeJSON(w, p)
}

// grel je hrana grafu v paměti (sdílené pro neighborhood i rodový strom).
type grel struct {
	ID   int64
	Type string
	A, B int64
	Conf float64
	Evid string
}

// loadRelations načte všechny hrany a adjacency mapu.
func (s *server) loadRelations() ([]grel, map[int64][]grel, error) {
	var rels []grel
	rows, err := s.db.Query(`SELECT id, type, person_a, person_b, confidence, COALESCE(evidence_json,'') FROM relations`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	adj := map[int64][]grel{}
	for rows.Next() {
		var e grel
		rows.Scan(&e.ID, &e.Type, &e.A, &e.B, &e.Conf, &e.Evid)
		rels = append(rels, e)
		adj[e.A] = append(adj[e.A], e)
		adj[e.B] = append(adj[e.B], e)
	}
	return rels, adj, nil
}

// writeGraph serializuje podgraf (osoby z dist + hrany mezi nimi) pro Cytoscape.
// focus označuje uzly, kvůli kterým graf vznikl (rodové jméno) — UI ostatní ztlumí.
func (s *server) writeGraph(w http.ResponseWriter, root int64, dist map[int64]int, focus map[int64]bool, rels []grel) {
	// vrstvy hrany = typy událostí jejích evidenčních záznamů
	evidenceLayers := func(evid string) []string {
		var v struct {
			Records []int64 `json:"records"`
		}
		json.Unmarshal([]byte(evid), &v)
		set := map[string]bool{}
		for _, rid := range v.Records {
			var t string
			if err := s.db.QueryRow(`SELECT type FROM events WHERE record_id=?`, rid).Scan(&t); err == nil {
				set[t] = true
			}
		}
		out := []string{}
		for _, t := range []string{"birth", "marriage", "death"} {
			if set[t] {
				out = append(out, t)
			}
		}
		return out
	}

	type node struct {
		ID         int64   `json:"id"`
		Name       string  `json:"name"`
		Sex        string  `json:"sex"`
		BirthYear  int     `json:"birth_year,omitempty"`
		DeathYear  int     `json:"death_year,omitempty"`
		Confidence float64 `json:"confidence"`
		Depth      int     `json:"depth"`
		HasDeath   bool    `json:"has_death"`
		Focus      bool    `json:"focus"`
	}
	nodes := []node{}
	for pid, d := range dist {
		var n node
		n.ID, n.Depth = pid, d
		err := s.db.QueryRow(`SELECT display_name, sex, COALESCE(birth_year_est,0), COALESCE(death_year_est,0), confidence
			FROM persons WHERE id=?`, pid).Scan(&n.Name, &n.Sex, &n.BirthYear, &n.DeathYear, &n.Confidence)
		if err != nil {
			continue
		}
		n.HasDeath = n.DeathYear > 0
		n.Focus = focus == nil || focus[pid]
		nodes = append(nodes, n)
	}
	type edgeOut struct {
		ID         int64    `json:"id"`
		Type       string   `json:"type"`
		Source     int64    `json:"source"`
		Target     int64    `json:"target"`
		Confidence float64  `json:"confidence"`
		Layers     []string `json:"layers"`
	}
	edgesOut := []edgeOut{}
	for _, e := range rels {
		if _, okA := dist[e.A]; !okA {
			continue
		}
		if _, okB := dist[e.B]; !okB {
			continue
		}
		edgesOut = append(edgesOut, edgeOut{e.ID, e.Type, e.A, e.B, e.Conf, evidenceLayers(e.Evid)})
	}
	writeJSON(w, map[string]any{"root": root, "nodes": nodes, "edges": edgesOut})
}

// handleNeighborhood vrátí podgraf kolem osoby do dané hloubky pro Cytoscape.
func (s *server) handleNeighborhood(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth"))
	if depth <= 0 || depth > 6 {
		depth = 2
	}
	rels, adj, err := s.loadRelations()
	if err != nil {
		httpErr(w, 500, err)
		return
	}
	// BFS
	dist := map[int64]int{id: 0}
	queue := []int64{id}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if dist[cur] >= depth {
			continue
		}
		for _, e := range adj[cur] {
			next := e.A
			if next == cur {
				next = e.B
			}
			if _, ok := dist[next]; !ok {
				dist[next] = dist[cur] + 1
				queue = append(queue, next)
			}
		}
	}
	s.writeGraph(w, id, dist, nil, rels)
}

// handleSurnameTree vrátí strom celého rodu: všechny osoby s daným příjmením
// (vč. rozených, historických pravopisů Worechowsky→Vořechovský a ženských
// tvarů -ská) + jejich přímé okolí do vzdálenosti hops (default 1).
func (s *server) handleSurnameTree(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("surname"))
	if q == "" {
		httpErr(w, 400, fmt.Errorf("chybí parametr surname"))
		return
	}
	hops, _ := strconv.Atoi(r.URL.Query().Get("hops"))
	if hops <= 0 || hops > 3 {
		hops = 1
	}
	norm := surnameNorm(q, false)

	rows, err := s.db.Query(`SELECT DISTINCT pm.person_id FROM mentions m
		JOIN person_mentions pm ON pm.mention_id = m.id
		WHERE m.surname_norm = ? OR m.maiden_norm = ?`, norm, norm)
	if err != nil {
		httpErr(w, 500, err)
		return
	}
	focus := map[int64]bool{}
	dist := map[int64]int{}
	var queue []int64
	for rows.Next() {
		var pid int64
		rows.Scan(&pid)
		focus[pid] = true
		dist[pid] = 0
		queue = append(queue, pid)
	}
	rows.Close()

	rels, adj, err := s.loadRelations()
	if err != nil {
		httpErr(w, 500, err)
		return
	}
	// BFS od všech nositelů jména najednou (přivěsí manžele/rodiče/děti)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if dist[cur] >= hops {
			continue
		}
		for _, e := range adj[cur] {
			next := e.A
			if next == cur {
				next = e.B
			}
			if _, ok := dist[next]; !ok {
				dist[next] = dist[cur] + 1
				queue = append(queue, next)
			}
		}
	}
	var root int64 = -1 // rodový strom nemá jednu kořenovou osobu
	s.writeGraph(w, root, dist, focus, rels)
}

// ---- záznamy a skeny ----

func (s *server) handleRecord(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var out struct {
		ID       int64             `json:"id"`
		Type     string            `json:"type"`
		Cislo    string            `json:"cislo"`
		RowIdx   int               `json:"row_idx"`
		Cells    map[string]string `json:"cells"`
		Corrections map[string]string `json:"corrections"`
		ScanID   int64             `json:"scan_id"`
		ScanFile string            `json:"scan_file"`
		Folio    string            `json:"folio"`
		BookID   string            `json:"book_id"`
		BookName string            `json:"book_name"`
	}
	var cellsJSON string
	err := s.db.QueryRow(`SELECT r.id, r.record_type, COALESCE(r.cislo,''), r.row_idx, r.cells_json,
		sc.id, sc.file, COALESCE(sc.folio,''), b.id, b.name
		FROM records r JOIN scans sc ON sc.id=r.scan_id JOIN books b ON b.id=sc.book_id
		WHERE r.id=?`, id).Scan(&out.ID, &out.Type, &out.Cislo, &out.RowIdx, &cellsJSON,
		&out.ScanID, &out.ScanFile, &out.Folio, &out.BookID, &out.BookName)
	if err == sql.ErrNoRows {
		httpErr(w, 404, fmt.Errorf("záznam %d neexistuje", id))
		return
	} else if err != nil {
		httpErr(w, 500, err)
		return
	}
	json.Unmarshal([]byte(cellsJSON), &out.Cells)
	out.Corrections = map[string]string{}
	rows, _ := s.db.Query(`SELECT cell_key, corrected_value FROM cell_corrections WHERE record_id=?`, id)
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		out.Corrections[k] = v
	}
	rows.Close()
	writeJSON(w, out)
}

func (s *server) scanPath(id int64) (string, error) {
	var dir, file string
	err := s.db.QueryRow(`SELECT b.scans_dir, sc.file FROM scans sc JOIN books b ON b.id=sc.book_id
		WHERE sc.id=?`, id).Scan(&dir, &file)
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", fmt.Errorf("kniha nemá dostupnou složku skenů (ingest bez --books-root)")
	}
	return filepath.Join(dir, file), nil
}

func (s *server) handleScanFull(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	path, err := s.scanPath(id)
	if err != nil {
		httpErr(w, 404, err)
		return
	}
	http.ServeFile(w, r, path)
}

// handleScanImage vrátí zmenšený náhled (nearest-neighbor, JPEG) — rychlé pro seznamy.
func (s *server) handleScanImage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	maxw, _ := strconv.Atoi(r.URL.Query().Get("maxw"))
	if maxw <= 0 || maxw > 4000 {
		maxw = 1600
	}
	path, err := s.scanPath(id)
	if err != nil {
		httpErr(w, 404, err)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		httpErr(w, 404, err)
		return
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		httpErr(w, 500, err)
		return
	}
	b := img.Bounds()
	if b.Dx() > maxw {
		img = downscale(img, maxw)
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	jpeg.Encode(w, img, &jpeg.Options{Quality: 82})
}

// downscale zmenší obrázek na šířku maxw (nearest-neighbor — náhled, ne archiv).
func downscale(src image.Image, maxw int) image.Image {
	b := src.Bounds()
	ratio := float64(maxw) / float64(b.Dx())
	h := int(float64(b.Dy()) * ratio)
	dst := image.NewRGBA(image.Rect(0, 0, maxw, h))
	for y := 0; y < h; y++ {
		sy := b.Min.Y + int(float64(y)/ratio)
		for x := 0; x < maxw; x++ {
			sx := b.Min.X + int(float64(x)/ratio)
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

// ---- mutace: opravy, constraints, merge/split ----

func (s *server) handleCellPatch(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var body struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
		httpErr(w, 400, fmt.Errorf("očekávám {key, value}"))
		return
	}
	var bookID string
	err := s.db.QueryRow(`SELECT b.id FROM records r JOIN scans sc ON sc.id=r.scan_id
		JOIN books b ON b.id=sc.book_id WHERE r.id=?`, id).Scan(&bookID)
	if err != nil {
		httpErr(w, 404, fmt.Errorf("záznam %d neexistuje", id))
		return
	}
	_, err = s.db.Exec(`INSERT INTO cell_corrections(record_id,cell_key,corrected_value,note)
		VALUES(?,?,?,?) ON CONFLICT(record_id,cell_key) DO UPDATE SET
		corrected_value=excluded.corrected_value, note=excluded.note`, id, body.Key, body.Value, body.Note)
	if err != nil {
		httpErr(w, 500, err)
		return
	}
	// oprava buňky → přestavět extrakci knihy (mentions se upsertují, id drží)
	if err := runExtract(s.db, bookID); err != nil {
		httpErr(w, 500, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "reextracted_book": bookID})
}

func (s *server) handleConstraintCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind     string `json:"kind"`
		MentionA int64  `json:"mention_a"`
		MentionB int64  `json:"mention_b"`
		Note     string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		(body.Kind != "must_link" && body.Kind != "cannot_link") || body.MentionA == 0 || body.MentionB == 0 {
		httpErr(w, 400, fmt.Errorf("očekávám {kind: must_link|cannot_link, mention_a, mention_b}"))
		return
	}
	res, err := s.db.Exec(`INSERT INTO constraints(kind,mention_a,mention_b,note) VALUES(?,?,?,?)`,
		body.Kind, body.MentionA, body.MentionB, body.Note)
	if err != nil {
		httpErr(w, 500, err)
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (s *server) handleConstraintDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if _, err := s.db.Exec(`DELETE FROM constraints WHERE id=?`, id); err != nil {
		httpErr(w, 500, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleMerge spojí dvě osoby: must_link mezi reprezentativními zmínkami + re-match.
func (s *server) handleMerge(w http.ResponseWriter, r *http.Request) {
	a, _ := strconv.ParseInt(r.PathValue("a"), 10, 64)
	b, _ := strconv.ParseInt(r.PathValue("b"), 10, 64)
	var ma, mb int64
	if err := s.db.QueryRow(`SELECT MIN(mention_id) FROM person_mentions WHERE person_id=?`, a).Scan(&ma); err != nil || ma == 0 {
		httpErr(w, 404, fmt.Errorf("osoba %d nemá zmínky", a))
		return
	}
	if err := s.db.QueryRow(`SELECT MIN(mention_id) FROM person_mentions WHERE person_id=?`, b).Scan(&mb); err != nil || mb == 0 {
		httpErr(w, 404, fmt.Errorf("osoba %d nemá zmínky", b))
		return
	}
	// merge ruší cannot_linky, které by spojení blokovaly
	s.db.Exec(`DELETE FROM constraints WHERE kind='cannot_link' AND
		mention_a IN (SELECT mention_id FROM person_mentions WHERE person_id IN (?,?)) AND
		mention_b IN (SELECT mention_id FROM person_mentions WHERE person_id IN (?,?))`, a, b, a, b)
	if _, err := s.db.Exec(`INSERT INTO constraints(kind,mention_a,mention_b,note)
		VALUES('must_link',?,?,'merge z UI')`, ma, mb); err != nil {
		httpErr(w, 500, err)
		return
	}
	if err := runMatch(s.db, lastParams(s.db)); err != nil {
		httpErr(w, 500, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleSplit oddělí vybrané zmínky od zbytku osoby: cannot_link + zrušení
// must_linků uvnitř osoby + re-match.
func (s *server) handleSplit(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var body struct {
		MentionIDs []int64 `json:"mention_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.MentionIDs) == 0 {
		httpErr(w, 400, fmt.Errorf("očekávám {mention_ids: [...]}"))
		return
	}
	split := map[int64]bool{}
	for _, m := range body.MentionIDs {
		split[m] = true
	}
	rows, err := s.db.Query(`SELECT mention_id FROM person_mentions WHERE person_id=?`, id)
	if err != nil {
		httpErr(w, 500, err)
		return
	}
	var keep, out []int64
	for rows.Next() {
		var m int64
		rows.Scan(&m)
		if split[m] {
			out = append(out, m)
		} else {
			keep = append(keep, m)
		}
	}
	rows.Close()
	if len(out) == 0 || len(keep) == 0 {
		httpErr(w, 400, fmt.Errorf("split musí nechat na obou stranách aspoň jednu zmínku"))
		return
	}
	// zruš must_linky přes hranici splitu
	for _, o := range out {
		for _, k := range keep {
			s.db.Exec(`DELETE FROM constraints WHERE kind='must_link' AND
				((mention_a=? AND mention_b=?) OR (mention_a=? AND mention_b=?))`, o, k, k, o)
		}
	}
	if _, err := s.db.Exec(`INSERT INTO constraints(kind,mention_a,mention_b,note)
		VALUES('cannot_link',?,?,'split z UI')`, out[0], keep[0]); err != nil {
		httpErr(w, 500, err)
		return
	}
	if err := runMatch(s.db, lastParams(s.db)); err != nil {
		httpErr(w, 500, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *server) handleMatchRun(w http.ResponseWriter, r *http.Request) {
	if err := runMatch(s.db, lastParams(s.db)); err != nil {
		httpErr(w, 500, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// lastParams vrátí prahy posledního běhu matcheru (nebo defaulty).
func lastParams(db *sql.DB) matchParams {
	p := matchParams{Auto: 0.90, Flag: 0.72, Suggest: 0.50}
	var js string
	if err := db.QueryRow(`SELECT params_json FROM match_runs ORDER BY id DESC LIMIT 1`).Scan(&js); err == nil {
		json.Unmarshal([]byte(js), &p)
	}
	return p
}

func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]any{}
	for name, q := range map[string]string{
		"books":    `SELECT COUNT(*) FROM books`,
		"scans":    `SELECT COUNT(*) FROM scans`,
		"records":  `SELECT COUNT(*) FROM records`,
		"mentions": `SELECT COUNT(*) FROM mentions`,
		"persons":  `SELECT COUNT(*) FROM persons`,
		"relations": `SELECT COUNT(*) FROM relations`,
	} {
		var n int
		s.db.QueryRow(q).Scan(&n)
		stats[name] = n
	}
	writeJSON(w, stats)
}
