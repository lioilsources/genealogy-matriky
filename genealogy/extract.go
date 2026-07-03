package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"
)

// mentionRow je zmínka o osobě připravená k zápisu.
type mentionRow struct {
	Role       string
	Ordinal    int
	Raw        string
	Given      string
	Surname    string
	GivenNorm  string
	SurnameNorm string
	MaidenNorm string
	Sex        string
	Occupation string
	Marital    string
	Place      string
	BirthYear  int
	AgeText    string
	Extra      map[string]string
}

func cmdExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	dbPath := fs.String("db", "data/genealogy.db", "cesta k SQLite databázi")
	book := fs.String("book", "", "jen jedna kniha (id); default všechny")
	fs.Parse(args)

	db, err := openDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return runExtract(db, *book)
}

// runExtract přestaví events+mentions z records (+cell_corrections).
// Mentions se upsertují podle (record_id, role, ordinal), takže jejich id
// přežijí opakovaný běh a constraints na ně navázané zůstávají platné.
func runExtract(db *sql.DB, bookID string) error {
	where, params := "", []any{}
	if bookID != "" {
		where = ` WHERE s.book_id = ?`
		params = append(params, bookID)
	}
	rows, err := db.Query(`SELECT r.id, r.record_type, r.cells_json FROM records r
		JOIN scans s ON s.id = r.scan_id`+where+` ORDER BY r.id`, params...)
	if err != nil {
		return err
	}
	type rec struct {
		id    int64
		typ   string
		cells map[string]string
	}
	var recs []rec
	for rows.Next() {
		var r rec
		var cellsJSON string
		if err := rows.Scan(&r.id, &r.typ, &cellsJSON); err != nil {
			return err
		}
		json.Unmarshal([]byte(cellsJSON), &r.cells)
		recs = append(recs, r)
	}
	rows.Close()

	// opravy buněk mají přednost před raw OCR hodnotou
	corr := map[int64]map[string]string{}
	crows, err := db.Query(`SELECT record_id, cell_key, corrected_value FROM cell_corrections`)
	if err != nil {
		return err
	}
	for crows.Next() {
		var id int64
		var k, v string
		crows.Scan(&id, &k, &v)
		if corr[id] == nil {
			corr[id] = map[string]string{}
		}
		corr[id][k] = v
	}
	crows.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stats := map[string]int{}
	var unparsedDates int
	for _, r := range recs {
		for k, v := range corr[r.id] {
			r.cells[k] = v
		}
		ev, mentions := extractRecord(r.typ, r.cells)
		if ev.Type != "" {
			if ev.Date.Precision == "none" {
				unparsedDates++
			}
			if err := upsertEvent(tx, r.id, ev); err != nil {
				return fmt.Errorf("record %d: %w", r.id, err)
			}
		}
		if err := upsertMentions(tx, r.id, mentions); err != nil {
			return fmt.Errorf("record %d: %w", r.id, err)
		}
		for _, m := range mentions {
			stats[m.Role]++
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	var roles []string
	for role := range stats {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	fmt.Printf("extract: %d záznamů\n", len(recs))
	for _, role := range roles {
		fmt.Printf("  %-16s %d\n", role, stats[role])
	}
	if unparsedDates > 0 {
		fmt.Printf("  ! nezparsovaných dat událostí: %d\n", unparsedDates)
	}
	return nil
}

// eventRow je událost (narození/svatba/úmrtí) záznamu.
type eventRow struct {
	Type      string
	DateText  string
	Date      parsedDate
	PlaceText string
	HouseNo   string
}

// extractRecord převede OCR řádek na událost + zmínky o osobách dle typu záznamu.
func extractRecord(typ string, cells map[string]string) (eventRow, []mentionRow) {
	switch typ {
	case "narozeni":
		return extractBirth(cells)
	case "oddani":
		return extractMarriage(cells)
	case "umrti":
		return extractDeath(cells)
	}
	return eventRow{}, nil
}

func extractBirth(c map[string]string) (eventRow, []mentionRow) {
	ev := eventRow{Type: "birth", DateText: firstNonEmpty(c["datum_narozeni"], c["datum_krtu"])}
	ev.Date = parseDate(ev.DateText)
	ev.PlaceText, ev.HouseNo = splitPlaceHouse(c["misto_dum"])

	var out []mentionRow

	// otec + jeho rodiče
	var fatherSurname, fatherSurnameNorm string
	if s := c["otec_jmeno_stav"]; strings.TrimSpace(s) != "" {
		p, gf, gm := parsePersonCell(s)
		p.Sex = "m"
		if p.Place == "" {
			p.Place, _ = splitPlaceHouse(c["otec_bydliste"])
		}
		fatherSurname, fatherSurnameNorm = p.Surname, p.SurnameNorm
		out = append(out, toMention("otec", 0, p))
		out = appendParents(out, "otec", gf, gm)
	}
	// matka + její rodiče
	if s := c["matka_jmeno_rodice"]; strings.TrimSpace(s) != "" {
		p, gf, gm := parsePersonCell(s)
		p.Sex = "f"
		if p.Place == "" {
			p.Place, _ = splitPlaceHouse(c["matka_bydliste"])
		}
		// příjmení matky: provdaná = po otci dítěte (přechýleně)
		if p.Surname == "" && fatherSurname != "" {
			p.Surname, p.SurnameNorm = feminizeSurname(fatherSurname), fatherSurnameNorm
		}
		out = append(out, toMention("matka", 0, p))
		out = appendParents(out, "matka", gf, gm)
	}
	// dítě — jméno křestní, příjmení po otci
	if s := c["dite_jmeno"]; strings.TrimSpace(s) != "" {
		p, _, _ := parsePersonCell(s)
		if p.Given == "" {
			p.Given = strings.TrimSpace(s)
			p.GivenNorm = givenNorm(p.Given)
		}
		switch foldASCII(c["dite_pohlavi"]) {
		case "chlapec", "muz", "m":
			p.Sex = "m"
		case "devce", "divka", "zena", "f", "z":
			p.Sex = "f"
		}
		if p.Surname == "" && fatherSurname != "" {
			p.Surname, p.SurnameNorm = fatherSurname, fatherSurnameNorm
			if p.Sex == "f" {
				p.Surname = feminizeSurname(p.Surname)
			}
		}
		if st, ok := maritalWords[foldASCII(strings.TrimSpace(c["dite_manzelske"]))]; ok {
			p.MaritalStatus = st
		}
		if p.Place == "" {
			p.Place = ev.PlaceText
		}
		p.Raw = s
		m := toMention("dite", 0, p)
		m.BirthYear = ev.Date.Year
		out = append(out, m)
	}
	// kmotři a porodní bába
	for i, part := range splitPeopleList(c["kmotri"]) {
		p, _, _ := parsePersonCell(part)
		out = append(out, toMention("kmotr", i, p))
	}
	if s := strings.TrimSpace(c["babka"]); s != "" {
		p, _, _ := parsePersonCell(s)
		p.Sex = "f"
		out = append(out, toMention("babka", 0, p))
	}
	return ev, out
}

func extractMarriage(c map[string]string) (eventRow, []mentionRow) {
	ev := eventRow{Type: "marriage", DateText: firstNonEmpty(c["datum_oddavek"], c["datum_ohlasek"])}
	ev.Date = parseDate(ev.DateText)
	ev.PlaceText, ev.HouseNo = splitPlaceHouse(c["misto_oddavek"])

	var out []mentionRow
	if s := c["zenich_jmeno_stav_rodice"]; strings.TrimSpace(s) != "" {
		p, gf, gm := parsePersonCell(s)
		p.Sex = "m"
		if p.Place == "" {
			p.Place, _ = splitPlaceHouse(c["zenich_misto_prebyvani"])
		}
		out = append(out, toMention("zenich", 0, p))
		out = appendParents(out, "zenich", gf, gm)
	}
	// nevěsta: schéma knihy 8386 nemá sloupec se jménem nevěsty; podporujeme
	// nevesta_jmeno_stav_rodice / nevesta_jmeno, jinak vznikne zmínka beze jména
	// (nese aspoň DOB a místo narození pro cross-check s křtem).
	brideCell := firstNonEmpty(c["nevesta_jmeno_stav_rodice"], c["nevesta_jmeno"])
	if strings.TrimSpace(brideCell) != "" || strings.TrimSpace(c["nevesta_datum_narozeni"]) != "" {
		p, gf, gm := parsePersonCell(brideCell)
		p.Sex = "f"
		if p.Place == "" {
			p.Place, _ = splitPlaceHouse(c["nevesta_misto_zrozeni"])
		}
		if st, ok := maritalWords[foldASCII(strings.TrimSpace(c["nevesta_stav"]))]; ok && p.MaritalStatus == "" {
			p.MaritalStatus = st
		}
		m := toMention("nevesta", 0, p)
		if d := parseDate(c["nevesta_datum_narozeni"]); d.Year > 0 {
			m.BirthYear = d.Year
			m.Extra = map[string]string{"birth_date": d.ISO, "birth_date_precision": d.Precision}
		}
		if m.Raw == "" {
			m.Raw = strings.TrimSpace(strings.Join([]string{c["nevesta_misto_zrozeni"], c["nevesta_datum_narozeni"], c["nevesta_stav"]}, " | "))
		}
		out = append(out, m)
		out = appendParents(out, "nevesta", gf, gm)
	}
	for i, part := range splitPeopleList(c["svedkove"]) {
		p, _, _ := parsePersonCell(part)
		out = append(out, toMention("svedek", i, p))
	}
	return ev, out
}

func extractDeath(c map[string]string) (eventRow, []mentionRow) {
	ev := eventRow{Type: "death", DateText: firstNonEmpty(c["datum_umrti"], c["datum_pohrbu"])}
	ev.Date = parseDate(ev.DateText)
	ev.PlaceText, ev.HouseNo = splitPlaceHouse(c["misto_dum"])

	var out []mentionRow
	if s := c["zemrely_jmeno_stav"]; strings.TrimSpace(s) != "" {
		p, gf, gm := parsePersonCell(s)
		switch foldASCII(strings.TrimSpace(c["pohlavi"])) {
		case "muz", "m":
			p.Sex = "m"
		case "zena", "f", "z":
			p.Sex = "f"
		}
		if p.Place == "" {
			p.Place = ev.PlaceText
		}
		m := toMention("zemrely", 0, p)
		m.AgeText = strings.TrimSpace(c["vek"])
		if age, ok := parseAgeYears(m.AgeText); ok && ev.Date.Year > 0 {
			m.BirthYear = ev.Date.Year - age
		}
		out = append(out, m)
		out = appendParents(out, "zemrely", gf, gm)
	}
	return ev, out
}

func appendParents(out []mentionRow, prefix string, f, m *parsedPerson) []mentionRow {
	if f != nil {
		out = append(out, toMention(prefix+"_otec", 0, *f))
	}
	if m != nil {
		out = append(out, toMention(prefix+"_matka", 0, *m))
	}
	return out
}

func toMention(role string, ordinal int, p parsedPerson) mentionRow {
	return mentionRow{
		Role: role, Ordinal: ordinal, Raw: p.Raw,
		Given: p.Given, Surname: p.Surname,
		GivenNorm: p.GivenNorm, SurnameNorm: p.SurnameNorm, MaidenNorm: p.MaidenNorm,
		Sex: p.Sex, Occupation: p.Occupation, Marital: p.MaritalStatus, Place: p.Place,
	}
}

// splitPlaceHouse rozdělí "Kročehlavy č. 123" na (místo, číslo domu).
func splitPlaceHouse(s string) (place, house string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if m := reHouseNo.FindStringSubmatch(s); m != nil {
		house = firstNonEmpty(m[1], m[2])
		s = strings.TrimSpace(strings.Split(s, m[0])[0])
	}
	return strings.Trim(s, " ,."), house
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func upsertEvent(tx *sql.Tx, recordID int64, ev eventRow) error {
	iso := ev.Date.ISO
	_, err := tx.Exec(`INSERT INTO events(record_id,type,date_text,date,date_precision,year,place_text,house_no)
		VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, date_text=excluded.date_text,
			date=excluded.date, date_precision=excluded.date_precision, year=excluded.year,
			place_text=excluded.place_text, house_no=excluded.house_no`,
		recordID, ev.Type, ev.DateText, iso, ev.Date.Precision, nullYear(ev.Date.Year), ev.PlaceText, ev.HouseNo)
	return err
}

// upsertMentions upsertuje zmínky podle (record_id, role, ordinal) a smaže ty,
// které v novém běhu extraktu už nevznikly (vč. závislých constraints/person_mentions).
func upsertMentions(tx *sql.Tx, recordID int64, mentions []mentionRow) error {
	keep := map[string]bool{}
	for _, m := range mentions {
		keep[fmt.Sprintf("%s/%d", m.Role, m.Ordinal)] = true
		extra := ""
		if len(m.Extra) > 0 {
			b, _ := json.Marshal(m.Extra)
			extra = string(b)
		}
		if _, err := tx.Exec(`INSERT INTO mentions(record_id,role,ordinal,raw_text,given_name,surname,
			given_norm,surname_norm,maiden_norm,sex,occupation,marital_status,place_text,birth_year,age_text,extra_json)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(record_id,role,ordinal) DO UPDATE SET raw_text=excluded.raw_text,
				given_name=excluded.given_name, surname=excluded.surname,
				given_norm=excluded.given_norm, surname_norm=excluded.surname_norm,
				maiden_norm=excluded.maiden_norm, sex=excluded.sex, occupation=excluded.occupation,
				marital_status=excluded.marital_status, place_text=excluded.place_text,
				birth_year=excluded.birth_year, age_text=excluded.age_text, extra_json=excluded.extra_json`,
			recordID, m.Role, m.Ordinal, m.Raw, m.Given, m.Surname,
			m.GivenNorm, m.SurnameNorm, m.MaidenNorm, m.Sex, m.Occupation, m.Marital,
			m.Place, nullYear(m.BirthYear), m.AgeText, extra); err != nil {
			return err
		}
	}
	// smazání zaniklých zmínek záznamu
	rows, err := tx.Query(`SELECT id, role, ordinal FROM mentions WHERE record_id=?`, recordID)
	if err != nil {
		return err
	}
	var stale []int64
	for rows.Next() {
		var id int64
		var role string
		var ord int
		rows.Scan(&id, &role, &ord)
		if !keep[fmt.Sprintf("%s/%d", role, ord)] {
			stale = append(stale, id)
		}
	}
	rows.Close()
	for _, id := range stale {
		if _, err := tx.Exec(`DELETE FROM constraints WHERE mention_a=? OR mention_b=?`, id, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM person_mentions WHERE mention_id=?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM mentions WHERE id=?`, id); err != nil {
			return err
		}
	}
	return nil
}

func nullYear(y int) any {
	if y == 0 {
		return nil
	}
	return y
}
