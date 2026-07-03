package main

import (
	"fmt"
	"net/http"
)

// handleAnalytics vrací data pro grafy. Vše se počítá v SQL nad extrahovanou
// vrstvou (events/mentions/persons) — po opravách a re-matchi je hned aktuální.
func (s *server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	switch kind {
	case "surnames":
		// četnost příjmení (podle osob, ne zmínek — jinak vyhrávají svědci)
		s.rowsJSON(w, `SELECT m.surname_norm AS name, COUNT(DISTINCT pm.person_id) AS value
			FROM mentions m JOIN person_mentions pm ON pm.mention_id = m.id
			WHERE m.surname_norm <> ''
			GROUP BY m.surname_norm ORDER BY value DESC, name LIMIT 30`)
	case "lifespan":
		// histogram délky života (osoby se známým narozením i úmrtím)
		s.rowsJSON(w, `SELECT (death_year_est - birth_year_est)/10*10 AS name, COUNT(*) AS value
			FROM persons
			WHERE birth_year_est IS NOT NULL AND death_year_est IS NOT NULL
			  AND death_year_est >= birth_year_est AND death_year_est - birth_year_est < 110
			GROUP BY name ORDER BY name`)
	case "marriage-age":
		// průměrný věk při sňatku po dekádách (jen osoby se známým rokem narození)
		s.rowsJSON(w, `SELECT e.year/10*10 AS name,
			ROUND(AVG(CASE WHEN m.role='zenich' THEN e.year - p.birth_year_est END),1) AS zenich,
			ROUND(AVG(CASE WHEN m.role='nevesta' THEN e.year - p.birth_year_est END),1) AS nevesta
			FROM mentions m
			JOIN events e ON e.record_id = m.record_id AND e.type='marriage' AND e.year IS NOT NULL
			JOIN person_mentions pm ON pm.mention_id = m.id
			JOIN persons p ON p.id = pm.person_id AND p.birth_year_est IS NOT NULL
			WHERE m.role IN ('zenich','nevesta') AND e.year - p.birth_year_est BETWEEN 14 AND 80
			GROUP BY name ORDER BY name`)
	case "seasonality":
		// sezónnost: počty událostí po měsících a typech
		s.rowsJSON(w, `SELECT CAST(substr(date, 6, 2) AS INTEGER) AS name,
			SUM(CASE WHEN type='birth' THEN 1 ELSE 0 END) AS birth,
			SUM(CASE WHEN type='marriage' THEN 1 ELSE 0 END) AS marriage,
			SUM(CASE WHEN type='death' THEN 1 ELSE 0 END) AS death
			FROM events WHERE date_precision IN ('day','month')
			GROUP BY name ORDER BY name`)
	case "migration":
		// páry (místo narození nevěsty/ženicha → místo oddavek) = pohyb mezi obcemi
		s.rowsJSON(w, `SELECT m.place_text AS name, e.place_text AS target, COUNT(*) AS value
			FROM mentions m
			JOIN events e ON e.record_id = m.record_id AND e.type='marriage'
			WHERE m.role IN ('zenich','nevesta') AND m.place_text <> '' AND e.place_text <> ''
			  AND m.place_text <> e.place_text
			GROUP BY name, target ORDER BY value DESC LIMIT 40`)
	case "family-size":
		// počet dětí na pár rodičů (hrany parent_child sdílené otcem+matkou)
		s.rowsJSON(w, `WITH kids AS (
			SELECT ra.person_a AS otec, rb.person_a AS matka, ra.person_b AS dite
			FROM relations ra
			JOIN relations rb ON rb.person_b = ra.person_b AND rb.type='parent_child' AND rb.person_a <> ra.person_a
			JOIN persons pa ON pa.id = ra.person_a AND pa.sex='m'
			JOIN persons pb ON pb.id = rb.person_a AND pb.sex='f'
			WHERE ra.type='parent_child')
			SELECT n AS name, COUNT(*) AS value FROM (
				SELECT COUNT(DISTINCT dite) AS n FROM kids GROUP BY otec, matka)
			GROUP BY n ORDER BY n`)
	case "timeline":
		// počty událostí po letech a typech (přehledový graf)
		s.rowsJSON(w, `SELECT year AS name,
			SUM(CASE WHEN type='birth' THEN 1 ELSE 0 END) AS birth,
			SUM(CASE WHEN type='marriage' THEN 1 ELSE 0 END) AS marriage,
			SUM(CASE WHEN type='death' THEN 1 ELSE 0 END) AS death
			FROM events WHERE year IS NOT NULL GROUP BY year ORDER BY year`)
	default:
		httpErr(w, 404, fmt.Errorf("neznámá analytika %q", kind))
	}
}

// rowsJSON provede dotaz a vrátí pole objektů {sloupec: hodnota}.
func (s *server) rowsJSON(w http.ResponseWriter, query string, params ...any) {
	rows, err := s.db.Query(query, params...)
	if err != nil {
		httpErr(w, 500, err)
		return
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			httpErr(w, 500, err)
			return
		}
		m := map[string]any{}
		for i, c := range cols {
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	writeJSON(w, out)
}
