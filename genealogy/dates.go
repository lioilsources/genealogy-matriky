package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// parsedDate je výsledek parsování datumové buňky.
type parsedDate struct {
	ISO       string // YYYY-MM-DD / YYYY-MM / YYYY (zkráceno dle přesnosti)
	Precision string // day|month|year|none
	Year      int
}

// měsíce česky, německy a latinsky (matriky střídají jazyky dle období)
var monthNames = map[string]int{
	"leden": 1, "ledna": 1, "unor": 2, "unora": 2, "brezen": 3, "brezna": 3,
	"duben": 4, "dubna": 4, "kveten": 5, "kvetna": 5, "cerven": 6, "cervna": 6,
	"cervenec": 7, "cervence": 7, "srpen": 8, "srpna": 8, "zari": 9,
	"rijen": 10, "rijna": 10, "listopad": 11, "listopadu": 11, "prosinec": 12, "prosince": 12,
	"januar": 1, "janner": 1, "februar": 2, "marz": 3, "april": 4, "mai": 5,
	"juni": 6, "juli": 7, "august": 8, "september": 9, "oktober": 10, "november": 11, "dezember": 12,
	"januarius": 1, "februarius": 2, "martius": 3, "aprilis": 4, "maius": 5, "maji": 5,
	"junius": 6, "julius": 7, "augustus": 8, "septembris": 9, "octobris": 10,
	"novembris": 11, "decembris": 12, "januarii": 1, "februarii": 2, "martii": 3,
}

var (
	reNumericDate = regexp.MustCompile(`(\d{1,2})\s*\.?\s*/?\s*(\d{1,2})\s*\.?\s+?(\d{4})`)
	reNumericTight = regexp.MustCompile(`(\d{1,2})\.(\d{1,2})\.(\d{4})`)
	reWordDate    = regexp.MustCompile(`(\d{1,2})\.?\s+([\p{L}]+)\s+(\d{4})`)
	reMonthYear   = regexp.MustCompile(`([\p{L}]+)\s+(\d{4})`)
	reYear        = regexp.MustCompile(`(1[5-9]\d\d|20\d\d)`)
)

// parseDate zkusí z volného textu buňky vytáhnout datum. Vrací nejlepší
// dosažitelnou přesnost; když nenajde nic, Precision je "none".
func parseDate(s string) parsedDate {
	s = strings.TrimSpace(s)
	if s == "" {
		return parsedDate{Precision: "none"}
	}

	if m := reNumericTight.FindStringSubmatch(s); m != nil {
		return dayDate(m[3], m[2], m[1])
	}
	if m := reNumericDate.FindStringSubmatch(s); m != nil {
		return dayDate(m[3], m[2], m[1])
	}
	if m := reWordDate.FindStringSubmatch(s); m != nil {
		if mo, ok := monthNames[foldASCII(strings.ToLower(m[2]))]; ok {
			return dayDate(m[3], strconv.Itoa(mo), m[1])
		}
	}
	if m := reMonthYear.FindStringSubmatch(s); m != nil {
		if mo, ok := monthNames[foldASCII(strings.ToLower(m[1]))]; ok {
			y, _ := strconv.Atoi(m[2])
			return parsedDate{ISO: fmt.Sprintf("%04d-%02d", y, mo), Precision: "month", Year: y}
		}
	}
	if m := reYear.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		return parsedDate{ISO: fmt.Sprintf("%04d", y), Precision: "year", Year: y}
	}
	return parsedDate{Precision: "none"}
}

func dayDate(ys, ms, ds string) parsedDate {
	y, _ := strconv.Atoi(ys)
	mo, _ := strconv.Atoi(ms)
	d, _ := strconv.Atoi(ds)
	if mo < 1 || mo > 12 || d < 1 || d > 31 {
		return parsedDate{ISO: fmt.Sprintf("%04d", y), Precision: "year", Year: y}
	}
	return parsedDate{ISO: fmt.Sprintf("%04d-%02d-%02d", y, mo, d), Precision: "day", Year: y}
}

var reAgeYears = regexp.MustCompile(`(\d{1,3})\s*(let|roku|roků|rok|r\.|Jahre?)`)
var reAgeSub = regexp.MustCompile(`(měsíc|mesic|týd|tyd|dn[iíů]|den|hodin)`)

// parseAgeYears vytáhne věk v letech ("72 let", "3/4 roku" → 0, "6 měsíců" → 0).
// Vrací (roky, ok).
func parseAgeYears(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if m := reAgeYears.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, true
	}
	if reAgeSub.MatchString(foldASCII(strings.ToLower(s))) {
		return 0, true // kojenec/batole — méně než rok
	}
	// holé číslo bereme jako roky (častý zápis ve sloupci Stáří)
	if n, err := strconv.Atoi(s); err == nil && n < 120 {
		return n, true
	}
	return 0, false
}
