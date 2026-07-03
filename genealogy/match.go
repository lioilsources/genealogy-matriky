package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"
)

// matchParams jsou prahy matcheru; ukládají se do match_runs.params_json.
type matchParams struct {
	Auto    float64 `json:"auto"`    // ≥ → automatické spojení
	Flag    float64 `json:"flag"`    // ≥ → spojení s příznakem nízké jistoty
	Suggest float64 `json:"suggest"` // ≥ → jen návrh v UI
}

func cmdMatch(args []string) error {
	fs := flag.NewFlagSet("match", flag.ExitOnError)
	dbPath := fs.String("db", "data/genealogy.db", "cesta k SQLite databázi")
	auto := fs.Float64("auto", 0.90, "práh automatického spojení")
	flagT := fs.Float64("flag", 0.72, "práh spojení s příznakem nízké jistoty")
	suggest := fs.Float64("suggest", 0.50, "práh návrhu (jen do match_candidates)")
	fs.Parse(args)

	db, err := openDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return runMatch(db, matchParams{Auto: *auto, Flag: *flagT, Suggest: *suggest})
}

// mMention je zmínka nahraná do paměti se vším kontextem pro skórování.
type mMention struct {
	ID          int64
	RecordID    int64
	Role        string
	GivenRaw    string
	SurnameRaw  string
	GivenNorm   string
	SurnameNorm string
	MaidenNorm  string
	Sex         string
	PlaceFold   string
	BirthYear   int // explicitní (DOB nevěsty, věk zemřelého, rok křtu dítěte)
	EventYear   int
	EventType   string
	YearMin     int // věrohodný rozsah roku narození
	YearMax     int
	// jména rodičů z téhož záznamu (cross-check)
	FatherGiven, FatherSurname string
	MotherGiven, MotherSurname string
}

// principal říká, jestli role popisuje hlavní osobu záznamu (dvě hlavní role
// téhož typu = dvě různé události → nemohou být tatáž osoba u narození/úmrtí).
func isPrimaryOnce(role string) bool { return role == "dite" || role == "zemrely" }

// runMatch přestaví persons/person_mentions/relations pod platnými constraints.
// Deterministický: stejná data + stejné prahy → identický výstup.
func runMatch(db *sql.DB, p matchParams) error {
	ms, err := loadMentions(db)
	if err != nil {
		return err
	}
	byID := map[int64]*mMention{}
	for i := range ms {
		byID[ms[i].ID] = &ms[i]
	}

	// constraints
	type pair struct{ a, b int64 }
	var mustLinks, cannotLinks []pair
	crows, err := db.Query(`SELECT kind, mention_a, mention_b FROM constraints ORDER BY id`)
	if err != nil {
		return err
	}
	for crows.Next() {
		var kind string
		var a, b int64
		crows.Scan(&kind, &a, &b)
		if byID[a] == nil || byID[b] == nil {
			continue
		}
		if kind == "must_link" {
			mustLinks = append(mustLinks, pair{a, b})
		} else {
			cannotLinks = append(cannotLinks, pair{a, b})
		}
	}
	crows.Close()

	// blocking → kandidátní páry → skóre
	type scored struct {
		a, b     int64
		score    float64
		features map[string]float64
	}
	var cands []scored
	blocks := map[string][]*mMention{}
	for i := range ms {
		m := &ms[i]
		if m.SurnameNorm == "" || m.GivenNorm == "" {
			continue
		}
		for _, key := range blockKeys(m) {
			blocks[key] = append(blocks[key], m)
		}
	}
	seen := map[[2]int64]bool{}
	var blockKeysSorted []string
	for k := range blocks {
		blockKeysSorted = append(blockKeysSorted, k)
	}
	sort.Strings(blockKeysSorted)
	for _, key := range blockKeysSorted {
		grp := blocks[key]
		for i := 0; i < len(grp); i++ {
			for j := i + 1; j < len(grp); j++ {
				a, b := grp[i], grp[j]
				if a.ID > b.ID {
					a, b = b, a
				}
				k := [2]int64{a.ID, b.ID}
				if seen[k] {
					continue
				}
				seen[k] = true
				score, feats := scorePair(a, b)
				if score >= p.Suggest {
					cands = append(cands, scored{a.ID, b.ID, score, feats})
				}
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		if cands[i].a != cands[j].a {
			return cands[i].a < cands[j].a
		}
		return cands[i].b < cands[j].b
	})

	// union-find: seed must_link, pak kandidáti dle skóre
	uf := newUnionFind()
	for _, ml := range mustLinks {
		uf.union(ml.a, ml.b)
	}
	violatesCannot := func(x, y int64) bool {
		rx, ry := uf.find(x), uf.find(y)
		for _, cl := range cannotLinks {
			ca, cb := uf.find(cl.a), uf.find(cl.b)
			if (ca == rx && cb == ry) || (ca == ry && cb == rx) {
				return true
			}
		}
		return false
	}
	pairScore := map[[2]int64]float64{}
	accepted := map[[2]int64]bool{}
	for _, c := range cands {
		if c.score < p.Flag {
			continue
		}
		if uf.find(c.a) == uf.find(c.b) {
			accepted[[2]int64{c.a, c.b}] = true
			pairScore[[2]int64{c.a, c.b}] = c.score
			continue
		}
		if violatesCannot(c.a, c.b) {
			continue
		}
		uf.union(c.a, c.b)
		accepted[[2]int64{c.a, c.b}] = true
		pairScore[[2]int64{c.a, c.b}] = c.score
	}

	// zaokrouhlení skóre (žádný float šum v DB a API)
	for k, v := range pairScore {
		pairScore[k] = float64(int(v*1000+0.5)) / 1000
	}

	// clustery (deterministické pořadí dle nejmenšího mention id)
	clusters := map[int64][]int64{}
	for i := range ms {
		root := uf.find(ms[i].ID)
		clusters[root] = append(clusters[root], ms[i].ID)
	}
	var roots []int64
	for r := range clusters {
		sort.Slice(clusters[r], func(i, j int) bool { return clusters[r][i] < clusters[r][j] })
		roots = append(roots, r)
	}
	sort.Slice(roots, func(i, j int) bool { return clusters[roots[i]][0] < clusters[roots[j]][0] })

	// confidence clusteru = min přijaté párové skóre uvnitř
	clusterConf := func(members []int64) float64 {
		conf := 1.0
		for i := 0; i < len(members); i++ {
			for j := i + 1; j < len(members); j++ {
				k := [2]int64{members[i], members[j]}
				if accepted[k] {
					if s := pairScore[k]; s < conf {
						conf = s
					}
				}
			}
		}
		return conf
	}

	// stabilita person id: zdědit id z minulého běhu dle největšího překryvu
	prev := map[int64]int64{} // mention → person
	prows, err := db.Query(`SELECT mention_id, person_id FROM person_mentions`)
	if err != nil {
		return err
	}
	for prows.Next() {
		var mid, pid int64
		prows.Scan(&mid, &pid)
		prev[mid] = pid
	}
	prows.Close()
	var maxPersonID int64
	db.QueryRow(`SELECT COALESCE(MAX(id),0) FROM persons`).Scan(&maxPersonID)

	type clusterOut struct {
		root    int64
		members []int64
		pid     int64
		conf    float64
	}
	var outs []clusterOut
	for _, r := range roots {
		outs = append(outs, clusterOut{root: r, members: clusters[r], conf: clusterConf(clusters[r])})
	}
	// větší clustery si vybírají zděděné id první
	order := make([]int, len(outs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := outs[order[i]], outs[order[j]]
		if len(a.members) != len(b.members) {
			return len(a.members) > len(b.members)
		}
		return a.members[0] < b.members[0]
	})
	taken := map[int64]bool{}
	for _, idx := range order {
		counts := map[int64]int{}
		for _, mid := range outs[idx].members {
			if pid, ok := prev[mid]; ok {
				counts[pid]++
			}
		}
		best, bestN := int64(0), 0
		var pids []int64
		for pid := range counts {
			pids = append(pids, pid)
		}
		sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
		for _, pid := range pids {
			if !taken[pid] && counts[pid] > bestN {
				best, bestN = pid, counts[pid]
			}
		}
		if best != 0 {
			outs[idx].pid = best
			taken[best] = true
		} else {
			maxPersonID++
			outs[idx].pid = maxPersonID
		}
	}

	// zápis
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	paramsJSON, _ := json.Marshal(p)
	res, err := tx.Exec(`INSERT INTO match_runs(started_at, params_json) VALUES(?,?)`,
		time.Now().UTC().Format(time.RFC3339), string(paramsJSON))
	if err != nil {
		return err
	}
	runID, _ := res.LastInsertId()

	for _, stmt := range []string{`DELETE FROM relations`, `DELETE FROM person_mentions`, `DELETE FROM persons`} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	mentionPerson := map[int64]int64{}
	personConf := map[int64]float64{}
	for _, c := range outs {
		name, sex, by, dy := personSummary(c.members, byID)
		if _, err := tx.Exec(`INSERT INTO persons(id,display_name,sex,birth_year_est,death_year_est,confidence,first_seen_run)
			VALUES(?,?,?,?,?,?,?)`, c.pid, name, sex, nullYear(by), nullYear(dy), c.conf, runID); err != nil {
			return err
		}
		personConf[c.pid] = c.conf
		for _, mid := range c.members {
			src := "auto"
			for _, ml := range mustLinks {
				if ml.a == mid || ml.b == mid {
					src = "constraint"
				}
			}
			if _, err := tx.Exec(`INSERT INTO person_mentions(person_id,mention_id,confidence,source) VALUES(?,?,?,?)`,
				c.pid, mid, c.conf, src); err != nil {
				return err
			}
			mentionPerson[mid] = c.pid
		}
	}

	if err := writeRelations(tx, ms, mentionPerson, personConf); err != nil {
		return err
	}

	// kandidáti pro UI (pásmo suggest..flag, mezi různými osobami)
	if _, err := tx.Exec(`DELETE FROM match_candidates`); err != nil {
		return err
	}
	nSuggest := 0
	for _, c := range cands {
		acc := boolInt(accepted[[2]int64{c.a, c.b}])
		if acc == 0 && mentionPerson[c.a] == mentionPerson[c.b] {
			continue // spojeni tranzitivně
		}
		fj, _ := json.Marshal(c.features)
		if _, err := tx.Exec(`INSERT INTO match_candidates(run_id,mention_a,mention_b,score,features_json,accepted)
			VALUES(?,?,?,?,?,?)`, runID, c.a, c.b, c.score, string(fj), acc); err != nil {
			return err
		}
		if acc == 0 {
			nSuggest++
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	nMulti := 0
	for _, c := range outs {
		if len(c.members) > 1 {
			nMulti++
		}
	}
	fmt.Printf("match: %d zmínek → %d osob (%d sloučených clusterů), %d návrhů k posouzení\n",
		len(ms), len(outs), nMulti, nSuggest)
	return nil
}

// loadMentions načte zmínky s kontextem záznamu/události a jmény rodičů.
func loadMentions(db *sql.DB) ([]mMention, error) {
	rows, err := db.Query(`SELECT m.id, m.record_id, m.role, COALESCE(m.given_name,''),
		COALESCE(m.surname,''), m.given_norm, m.surname_norm,
		m.maiden_norm, m.sex, COALESCE(m.place_text,''), COALESCE(m.birth_year,0),
		COALESCE(e.year,0), COALESCE(e.type,'')
		FROM mentions m
		LEFT JOIN events e ON e.record_id = m.record_id
		ORDER BY m.id`)
	if err != nil {
		return nil, err
	}
	var ms []mMention
	for rows.Next() {
		var m mMention
		var place string
		if err := rows.Scan(&m.ID, &m.RecordID, &m.Role, &m.GivenRaw, &m.SurnameRaw,
			&m.GivenNorm, &m.SurnameNorm,
			&m.MaidenNorm, &m.Sex, &place, &m.BirthYear, &m.EventYear, &m.EventType); err != nil {
			return nil, err
		}
		m.PlaceFold = placeNorm(place)
		ms = append(ms, m)
	}
	rows.Close()

	// jména rodičů z téhož záznamu: dite → otec/matka; zenich → zenich_otec/… atd.
	type key struct {
		rec  int64
		role string
	}
	names := map[key][2]string{} // role → (given_norm, surname_norm)
	for i := range ms {
		names[key{ms[i].RecordID, ms[i].Role}] = [2]string{ms[i].GivenNorm, ms[i].SurnameNorm}
	}
	for i := range ms {
		m := &ms[i]
		var fRole, mRole string
		switch m.Role {
		case "dite":
			fRole, mRole = "otec", "matka"
		case "zenich", "nevesta", "zemrely", "otec", "matka":
			fRole, mRole = m.Role+"_otec", m.Role+"_matka"
		default:
			continue
		}
		if v, ok := names[key{m.RecordID, fRole}]; ok {
			m.FatherGiven, m.FatherSurname = v[0], v[1]
		}
		if v, ok := names[key{m.RecordID, mRole}]; ok {
			m.MotherGiven, m.MotherSurname = v[0], v[1]
		}
	}

	// věrohodný rozsah roku narození
	for i := range ms {
		m := &ms[i]
		switch {
		case m.BirthYear > 0:
			m.YearMin, m.YearMax = m.BirthYear-2, m.BirthYear+2
		case m.EventYear > 0:
			switch m.Role {
			case "dite":
				m.YearMin, m.YearMax = m.EventYear-1, m.EventYear
			case "zenich", "nevesta":
				m.YearMin, m.YearMax = m.EventYear-60, m.EventYear-15
			case "zemrely":
				m.YearMin, m.YearMax = m.EventYear-105, m.EventYear
			default: // rodiče, kmotři, svědci, babka
				m.YearMin, m.YearMax = m.EventYear-75, m.EventYear-15
			}
		default:
			m.YearMin, m.YearMax = 0, 3000
		}
	}
	return ms, nil
}

// blockKeys vrátí blokovací klíče zmínky: prefix příjmení + pohlaví + dekádové
// buckety věrohodného rozsahu narození (s přesahem ±1 bucket).
func blockKeys(m *mMention) []string {
	pfx := m.SurnameNorm
	if len(pfx) > 4 {
		pfx = pfx[:4]
	}
	lo, hi := m.YearMin/10-1, m.YearMax/10+1
	if hi-lo > 30 { // neznámý rozsah — neblokovat po dekádách, jeden široký bucket
		return []string{pfx + "/" + m.Sex + "/*"}
	}
	var keys []string
	for b := lo; b <= hi; b++ {
		keys = append(keys, fmt.Sprintf("%s/%s/%d", pfx, m.Sex, b))
	}
	keys = append(keys, pfx+"/"+m.Sex+"/*") // most na zmínky bez odhadu roku
	return keys
}

// scorePair spočítá skóre dvojice zmínek (0..1) a příznaky pro debug.
func scorePair(a, b *mMention) (float64, map[string]float64) {
	// tvrdé zákazy
	if a.RecordID == b.RecordID {
		return 0, nil
	}
	if a.Sex != "" && b.Sex != "" && a.Sex != b.Sex {
		return 0, nil
	}
	if isPrimaryOnce(a.Role) && isPrimaryOnce(b.Role) && a.Role == b.Role {
		return 0, nil // dvakrát narozený / dvakrát zemřelý
	}
	// mrtvý nemůže figurovat na pozdější události
	if a.Role == "zemrely" && b.EventYear > a.EventYear && a.EventYear > 0 {
		return 0, nil
	}
	if b.Role == "zemrely" && a.EventYear > b.EventYear && b.EventYear > 0 {
		return 0, nil
	}
	// roky narození se musí protínat
	if a.YearMax < b.YearMin || b.YearMax < a.YearMin {
		return 0, nil
	}

	feats := map[string]float64{}
	given := nameSim(a.GivenNorm, b.GivenNorm)
	surname := nameSim(a.SurnameNorm, b.SurnameNorm)
	// žena: příjmení po manželovi vs. rodné — zkus i maiden
	if a.Sex == "f" || b.Sex == "f" {
		if s := nameSim(orStr(a.MaidenNorm, a.SurnameNorm), orStr(b.MaidenNorm, b.SurnameNorm)); s > surname {
			surname = s
			feats["maiden_used"] = 1
		}
	}
	feats["given"] = given
	feats["surname"] = surname
	if given < 0.70 || surname < 0.75 {
		return 0, feats
	}
	score := 0.35*given + 0.35*surname

	// datum narození
	if a.BirthYear > 0 && b.BirthYear > 0 {
		d := a.BirthYear - b.BirthYear
		if d < 0 {
			d = -d
		}
		switch {
		case d == 0:
			score += 0.15
			feats["birth_year_exact"] = 1
		case d <= 2:
			score += 0.08
			feats["birth_year_close"] = 1
		default:
			score -= 0.20
			feats["birth_year_conflict"] = 1
		}
	}

	// místo
	if a.PlaceFold != "" && a.PlaceFold == b.PlaceFold {
		score += 0.08
		feats["place"] = 1
	}

	// cross-check rodičů (nejsilnější signál matrik)
	pc := parentSim(a, b)
	if pc > 0 {
		score += pc
		feats["parents"] = pc
	}

	if score > 1 {
		score = 1
	}
	if score < 0 {
		score = 0
	}
	return score, feats
}

// parentSim porovná jména rodičů obou zmínek: oba rodiče sedí → 0.15, jeden → 0.08.
func parentSim(a, b *mMention) float64 {
	fBoth := a.FatherGiven != "" && b.FatherGiven != ""
	mBoth := a.MotherGiven != "" && b.MotherGiven != ""
	fMatch := fBoth && nameSim(a.FatherGiven, b.FatherGiven) >= 0.85 &&
		(a.FatherSurname == "" || b.FatherSurname == "" || nameSim(a.FatherSurname, b.FatherSurname) >= 0.85)
	mMatch := mBoth && nameSim(a.MotherGiven, b.MotherGiven) >= 0.85
	switch {
	case fMatch && mMatch:
		return 0.15
	case fMatch || mMatch:
		return 0.08
	case fBoth && !fMatch && mBoth && !mMatch:
		return -0.15 // oba rodiče známí a nesedí — silný protidůkaz
	}
	return 0
}

// nameSim: 1.0 při shodě (kanonické varianty už řeší givenNorm), jinak Jaro-Winkler.
func nameSim(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	return jaroWinkler(a, b)
}

func orStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// personSummary vybere zobrazované jméno (nejčastější, preference hlavních rolí),
// pohlaví a odhad roků narození/úmrtí clusteru.
func personSummary(members []int64, byID map[int64]*mMention) (name, sex string, birthYear, deathYear int) {
	votes := map[string]int{}
	sexVotes := map[string]int{}
	for _, mid := range members {
		m := byID[mid]
		n := strings.TrimSpace(m.GivenRaw + " " + m.SurnameRaw)
		if n == "" {
			n = strings.TrimSpace(m.GivenNorm + " " + m.SurnameNorm)
		}
		w := 1
		switch m.Role {
		case "dite", "zenich", "nevesta", "zemrely":
			w = 3
		case "otec", "matka":
			w = 2
		}
		if n != "" {
			votes[n] += w
		}
		if m.Sex != "" {
			sexVotes[m.Sex]++
		}
		if m.Role == "dite" && m.EventType == "birth" && m.EventYear > 0 {
			birthYear = m.EventYear
		}
		if birthYear == 0 && m.BirthYear > 0 {
			birthYear = m.BirthYear
		}
		if m.Role == "zemrely" && m.EventYear > 0 {
			deathYear = m.EventYear
		}
	}
	best := 0
	var keys []string
	for k := range votes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if votes[k] > best {
			best, name = votes[k], k
		}
	}
	if name == "" {
		name = "(nečitelné jméno)"
	}
	if sexVotes["m"] > sexVotes["f"] {
		sex = "m"
	} else if sexVotes["f"] > 0 {
		sex = "f"
	}
	return
}

// writeRelations odvodí hrany z rolí v záznamech: rodič→dítě a manželé.
func writeRelations(tx *sql.Tx, ms []mMention, mentionPerson map[int64]int64, personConf map[int64]float64) error {
	// role → osoba per záznam
	type key struct {
		rec  int64
		role string
	}
	roleP := map[key]int64{}
	recIDs := map[int64]bool{}
	for i := range ms {
		roleP[key{ms[i].RecordID, ms[i].Role}] = mentionPerson[ms[i].ID]
		recIDs[ms[i].RecordID] = true
	}
	type edge struct {
		typ  string
		a, b int64
	}
	edges := map[edge][]int64{} // → evidence record ids
	addEdge := func(typ string, a, b, rec int64) {
		if a == 0 || b == 0 || a == b {
			return
		}
		if typ == "spouse" && a > b {
			a, b = b, a
		}
		e := edge{typ, a, b}
		edges[e] = append(edges[e], rec)
	}
	var recs []int64
	for r := range recIDs {
		recs = append(recs, r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i] < recs[j] })
	for _, rec := range recs {
		p := func(role string) int64 { return roleP[key{rec, role}] }
		addEdge("parent_child", p("otec"), p("dite"), rec)
		addEdge("parent_child", p("matka"), p("dite"), rec)
		addEdge("spouse", p("otec"), p("matka"), rec)
		addEdge("spouse", p("zenich"), p("nevesta"), rec)
		for _, pr := range []string{"otec", "matka", "zenich", "nevesta", "zemrely"} {
			addEdge("parent_child", p(pr+"_otec"), p(pr), rec)
			addEdge("parent_child", p(pr+"_matka"), p(pr), rec)
			addEdge("spouse", p(pr+"_otec"), p(pr+"_matka"), rec)
		}
	}
	var keys []edge
	for e := range edges {
		keys = append(keys, e)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.typ != b.typ {
			return a.typ < b.typ
		}
		if a.a != b.a {
			return a.a < b.a
		}
		return a.b < b.b
	})
	for _, e := range keys {
		conf := personConf[e.a]
		if c := personConf[e.b]; c < conf {
			conf = c
		}
		ev, _ := json.Marshal(map[string][]int64{"records": edges[e]})
		if _, err := tx.Exec(`INSERT INTO relations(type,person_a,person_b,confidence,evidence_json,source)
			VALUES(?,?,?,?,?,'auto')`, e.typ, e.a, e.b, conf, string(ev)); err != nil {
			return err
		}
	}
	return nil
}

// ---- union-find ----

type unionFind struct{ parent map[int64]int64 }

func newUnionFind() *unionFind { return &unionFind{parent: map[int64]int64{}} }

func (u *unionFind) find(x int64) int64 {
	p, ok := u.parent[x]
	if !ok || p == x {
		return x
	}
	r := u.find(p)
	u.parent[x] = r
	return r
}

func (u *unionFind) union(a, b int64) {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	if ra > rb { // deterministicky: menší id vyhrává jako root
		ra, rb = rb, ra
	}
	u.parent[rb] = ra
}

// ---- Jaro-Winkler ----

func jaroWinkler(s1, s2 string) float64 {
	j := jaro(s1, s2)
	if j < 0.7 {
		return j
	}
	prefix := 0
	for i := 0; i < len(s1) && i < len(s2) && i < 4; i++ {
		if s1[i] != s2[i] {
			break
		}
		prefix++
	}
	return j + float64(prefix)*0.1*(1-j)
}

func jaro(s1, s2 string) float64 {
	if s1 == s2 {
		return 1
	}
	l1, l2 := len(s1), len(s2)
	if l1 == 0 || l2 == 0 {
		return 0
	}
	window := max(l1, l2)/2 - 1
	if window < 0 {
		window = 0
	}
	m1 := make([]bool, l1)
	m2 := make([]bool, l2)
	matches := 0
	for i := 0; i < l1; i++ {
		lo, hi := max(0, i-window), min(l2-1, i+window)
		for j := lo; j <= hi; j++ {
			if !m2[j] && s1[i] == s2[j] {
				m1[i], m2[j] = true, true
				matches++
				break
			}
		}
	}
	if matches == 0 {
		return 0
	}
	transpositions := 0
	k := 0
	for i := 0; i < l1; i++ {
		if !m1[i] {
			continue
		}
		for !m2[k] {
			k++
		}
		if s1[i] != s2[k] {
			transpositions++
		}
		k++
	}
	m := float64(matches)
	return (m/float64(l1) + m/float64(l2) + (m-float64(transpositions)/2)/m) / 3
}
