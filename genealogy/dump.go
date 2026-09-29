package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// dumpSide je kontext jedné zmínky páru — pro group split (book/record)
// a pro error analysis proti skenům (scan, jména tak, jak je přečetlo OCR).
type dumpSide struct {
	Mention   int64  `json:"mention"`
	Record    int64  `json:"record"`
	Book      string `json:"book"`
	Scan      string `json:"scan"`
	Role      string `json:"role"`
	Sex       string `json:"sex,omitempty"`
	Name      string `json:"name"`
	EventType string `json:"event_type,omitempty"`
	EventYear int    `json:"event_year,omitempty"`
	BirthYear int    `json:"birth_year,omitempty"`
}

// dumpRow je jeden řádek pairs.jsonl. `f` jsou surové features bez vah
// (chybějící hodnota = klíč chybí, v pandas NaN); `rule_*` je výstup
// současného ručního scoreru pro reprodukci baseline 1:1.
type dumpRow struct {
	A         dumpSide           `json:"a"`
	B         dumpSide           `json:"b"`
	RuleScore float64            `json:"rule_score"`
	RuleVeto  string             `json:"rule_veto,omitempty"`
	RuleFeats map[string]float64 `json:"rule_feats,omitempty"`
	F         map[string]float64 `json:"f"`
}

// runDumpPairs vypíše VŠECHNY páry, které prošly blockingem (i ty se skóre 0),
// jako JSONL. Labely sem nepatří — ty připojuje ml/export_pairs.py z DB.
func runDumpPairs(db *sql.DB, path string) error {
	ms, err := loadMentions(db)
	if err != nil {
		return err
	}
	ctx, err := loadRecordContext(db)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)

	n, nPos := 0, 0
	for _, pr := range candidatePairs(ms) {
		a, b := pr[0], pr[1]
		score, rf := scorePair(a, b)
		row := dumpRow{
			A:         dumpSideOf(a, ctx),
			B:         dumpSideOf(b, ctx),
			RuleScore: score,
			RuleVeto:  hardVeto(a, b),
			RuleFeats: rf,
			F:         pairFeatures(a, b),
		}
		if row.RuleVeto == "" && (row.F["given_sim"] < 0.70 || row.F["surname_sim_best"] < 0.75) {
			row.RuleVeto = "name_gate"
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
		n++
		if score > 0 {
			nPos++
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Printf("dump-pairs: %d zmínek → %d párů z blockingu (%d se skóre > 0) → %s\n",
		len(ms), n, nPos, path)
	return nil
}

type recordCtx struct{ book, scan string }

func loadRecordContext(db *sql.DB) (map[int64]recordCtx, error) {
	rows, err := db.Query(`SELECT r.id, s.book_id, s.file FROM records r JOIN scans s ON s.id = r.scan_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]recordCtx{}
	for rows.Next() {
		var id int64
		var c recordCtx
		if err := rows.Scan(&id, &c.book, &c.scan); err != nil {
			return nil, err
		}
		out[id] = c
	}
	return out, rows.Err()
}

func dumpSideOf(m *mMention, ctx map[int64]recordCtx) dumpSide {
	c := ctx[m.RecordID]
	name := strings.TrimSpace(m.GivenRaw + " " + m.SurnameRaw)
	if name == "" {
		name = strings.TrimSpace(m.GivenNorm + " " + m.SurnameNorm)
	}
	return dumpSide{
		Mention: m.ID, Record: m.RecordID, Book: c.book, Scan: c.scan,
		Role: m.Role, Sex: m.Sex, Name: name,
		EventType: m.EventType, EventYear: m.EventYear, BirthYear: m.BirthYear,
	}
}

// pairFeatures: surové, neváhované signály páru. Záměrně spojité (Jaro-Winkler,
// rozdíly let) a s explicitní chybějící hodnotou — co z nich udělat je úkol ML.
func pairFeatures(a, b *mMention) map[string]float64 {
	f := map[string]float64{}
	put := func(k string, v float64) { f[k] = v }
	bit := func(k string, v bool) {
		if v {
			f[k] = 1
		} else {
			f[k] = 0
		}
	}

	// jména
	put("given_sim", nameSim(a.GivenNorm, b.GivenNorm))
	put("surname_sim", nameSim(a.SurnameNorm, b.SurnameNorm))
	best, maiden := surnameSim(a, b)
	put("surname_sim_best", best)
	bit("maiden_used", maiden)
	bit("given_exact", a.GivenNorm != "" && a.GivenNorm == b.GivenNorm)
	bit("surname_exact", a.SurnameNorm != "" && a.SurnameNorm == b.SurnameNorm)

	// pohlaví
	bit("sex_known_both", a.Sex != "" && b.Sex != "")

	// roky
	if a.BirthYear > 0 && b.BirthYear > 0 {
		put("birth_year_diff", float64(absInt(a.BirthYear-b.BirthYear)))
	}
	if a.EventYear > 0 && b.EventYear > 0 {
		put("event_year_diff", float64(absInt(a.EventYear-b.EventYear)))
	}
	if a.YearMax-a.YearMin < 1000 && b.YearMax-b.YearMin < 1000 {
		// průnik věrohodných rozsahů narození v letech (záporný = mezera)
		put("year_overlap", float64(min(a.YearMax, b.YearMax)-max(a.YearMin, b.YearMin)))
		put("year_range_width", float64(max(a.YearMax-a.YearMin, b.YearMax-b.YearMin)))
	}

	// místo
	if a.PlaceFold != "" && b.PlaceFold != "" {
		bit("same_place", a.PlaceFold == b.PlaceFold)
	}

	// rodiče (jen když jsou známí na obou stranách)
	if a.FatherGiven != "" && b.FatherGiven != "" {
		put("father_given_sim", nameSim(a.FatherGiven, b.FatherGiven))
		if a.FatherSurname != "" && b.FatherSurname != "" {
			put("father_surname_sim", nameSim(a.FatherSurname, b.FatherSurname))
		}
	}
	if a.MotherGiven != "" && b.MotherGiven != "" {
		put("mother_given_sim", nameSim(a.MotherGiven, b.MotherGiven))
	}
	put("parent_rule", parentSim(a, b))

	// struktura
	bit("same_event_type", a.EventType != "" && a.EventType == b.EventType)
	return f
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
