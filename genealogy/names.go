package main

import (
	_ "embed"
	"encoding/csv"
	"regexp"
	"strings"
)

//go:embed seed/name_variants.csv
var nameVariantsCSV string

// variantToCanonical: fold(varianta) → kanonické křestní jméno (jan/johann/joannes → jan).
// Obsahuje i časté genitivy (josefa → josef), takže "syn Josefa Slavíka" projde.
var variantToCanonical = map[string]string{}

// canonicalGivenSex: kanonické křestní → pohlaví (m/f)
var canonicalGivenSex = map[string]string{}

// ženská kanonická jména (pro sex inference); zbytek ze seedu je mužský
var femaleCanonical = map[string]bool{
	"marie": true, "anna": true, "katerina": true, "barbora": true, "alzbeta": true,
	"frantiska": true, "josefa": true, "terezie": true, "antonie": true, "veronika": true,
	"ruzena": true, "magdalena": true, "dorota": true, "ludmila": true, "zofie": true,
	"karolina": true, "kristyna": true, "apolena": true, "eleonora": true, "johana": true,
}

func init() {
	r := csv.NewReader(strings.NewReader(nameVariantsCSV))
	rows, _ := r.ReadAll()
	for i, row := range rows {
		if i == 0 || len(row) < 3 || row[2] != "given" {
			continue
		}
		can, variant := row[0], row[1]
		variantToCanonical[variant] = can
		variantToCanonical[can] = can
	}
	for v := range variantToCanonical {
		can := variantToCanonical[v]
		if femaleCanonical[can] {
			canonicalGivenSex[can] = "f"
		} else {
			canonicalGivenSex[can] = "m"
		}
	}
}

// foldASCII převede české/německé znaky na ASCII (á→a, ř→r, ß→ss) a lowercase.
func foldASCII(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'á', 'ä', 'à', 'â':
			b.WriteRune('a')
		case 'č', 'ç':
			b.WriteRune('c')
		case 'ď':
			b.WriteRune('d')
		case 'é', 'ě', 'ë', 'è':
			b.WriteRune('e')
		case 'í', 'ï', 'ì':
			b.WriteRune('i')
		case 'ň':
			b.WriteRune('n')
		case 'ó', 'ö', 'ô':
			b.WriteRune('o')
		case 'ř':
			b.WriteRune('r')
		case 'š':
			b.WriteRune('s')
		case 'ť':
			b.WriteRune('t')
		case 'ú', 'ů', 'ü', 'ù':
			b.WriteRune('u')
		case 'ý', 'ÿ':
			b.WriteRune('y')
		case 'ž':
			b.WriteRune('z')
		case 'ß':
			b.WriteString("ss")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// maleGenitive: 2. pád mužských jmen → kanonické ("syn Josefa" → josef).
// Držené mimo CSV, protože kolidují se ženskými jmény (Josefa, Petra, Pavla…) —
// použijí se jen v genitivním kontextu u muže.
var maleGenitive = map[string]string{
	"josefa": "josef", "jana": "jan", "vaclava": "vaclav", "frantiska": "frantisek",
	"antonina": "antonin", "karla": "karel", "jakuba": "jakub", "mateje": "matej",
	"tomase": "tomas", "jiriho": "jiri", "petra": "petr", "pavla": "pavel",
	"martina": "martin", "vojtecha": "vojtech", "ondreje": "ondrej", "vita": "vit",
	"vavrince": "vavrinec", "stepana": "stepan", "ignace": "ignac", "aloise": "alois",
	"rudolfa": "rudolf", "emanuela": "emanuel", "bohumila": "bohumil",
	"jindricha": "jindrich", "bedricha": "bedrich", "vilema": "vilem", "ludvika": "ludvik",
	"oldricha": "oldrich",
}

// displayGiven: kanonické jméno → hezký nominativ s diakritikou (pro display_name).
var displayGiven = map[string]string{
	"jan": "Jan", "josef": "Josef", "frantisek": "František", "vaclav": "Václav",
	"antonin": "Antonín", "karel": "Karel", "jakub": "Jakub", "matej": "Matěj",
	"tomas": "Tomáš", "jiri": "Jiří", "petr": "Petr", "pavel": "Pavel",
	"martin": "Martin", "vojtech": "Vojtěch", "ondrej": "Ondřej", "vit": "Vít",
	"vavrinec": "Vavřinec", "stepan": "Štěpán", "ignac": "Ignác", "alois": "Alois",
	"rudolf": "Rudolf", "emanuel": "Emanuel", "bohumil": "Bohumil",
	"jindrich": "Jindřich", "bedrich": "Bedřich", "vilem": "Vilém", "ludvik": "Ludvík",
	"oldrich": "Oldřich",
	"marie": "Marie", "anna": "Anna", "katerina": "Kateřina", "barbora": "Barbora",
	"alzbeta": "Alžběta", "frantiska": "Františka", "josefa": "Josefa",
	"terezie": "Terezie", "antonie": "Antonie", "veronika": "Veronika",
	"ruzena": "Růžena", "magdalena": "Magdalena", "dorota": "Dorota",
	"ludmila": "Ludmila", "zofie": "Žofie", "karolina": "Karolína",
	"kristyna": "Kristýna", "apolena": "Apolena",
}

// nominativeGiven vrátí zobrazitelný 1. pád křestního jména ("Josefa"(gen) → "Josef").
func nominativeGiven(raw string, genitive bool, sex string) string {
	can := givenNormCtx(raw, genitive, sex)
	if d, ok := displayGiven[can]; ok {
		return d
	}
	return raw
}

// nominativeSurname převede genitivní tvar příjmení na 1. pád se zachováním
// diakritiky ("Slavíka"→"Slavík", "Dvořákové"→"Dvořáková", "Svobody"→"Svoboda").
func nominativeSurname(raw string, genitive bool) string {
	if !genitive {
		return raw
	}
	r := []rune(raw)
	n := len(r)
	switch {
	case n > 3 && string(r[n-3:]) == "ové":
		return string(r[:n-3]) + "ová"
	case n > 3 && string(r[n-3:]) == "ého":
		return string(r[:n-3]) + "ý"
	case n > 3 && string(r[n-3:]) == "eho": // historický zápis bez diakritiky
		return string(r[:n-3]) + "y"
	case n > 4 && r[n-1] == 'a':
		return string(r[:n-1])
	case n > 4 && r[n-1] == 'y':
		return string(r[:n-1]) + "a"
	}
	return raw
}

// feminizeSurname přechýlí mužské příjmení pro zobrazení u žen, které zdědily
// příjmení z jiné zmínky (matka/dcera po otci): Dvořák→Dvořáková, Černý→Černá.
func feminizeSurname(s string) string {
	f := foldASCII(s)
	if s == "" || strings.HasSuffix(f, "ova") || strings.HasSuffix(f, "a") && strings.HasSuffix(s, "á") {
		return s
	}
	r := []rune(s)
	n := len(r)
	switch {
	case n > 2 && r[n-1] == 'ý':
		return string(r[:n-1]) + "á"
	case n > 3 && (strings.HasSuffix(f, "sky") || strings.HasSuffix(f, "cky")):
		return string(r[:n-1]) + "á" // Worechowsky → Worechowská
	case n > 2 && r[n-1] == 'a':
		return string(r[:n-1]) + "ová"
	default:
		return s + "ová"
	}
}

// givenNorm vrátí kanonizované křestní jméno (fold + varianty; historický
// pravopis w→v se zkouší až po slovníku — Wenzel je varianta, ne pravopis).
func givenNorm(given string) string {
	f := foldASCII(strings.TrimSpace(given))
	if can, ok := variantToCanonical[f]; ok {
		return can
	}
	f = oldOrthography(f)
	if can, ok := variantToCanonical[f]; ok {
		return can
	}
	return f
}

// givenNormCtx kanonizuje se znalostí kontextu: v genitivu u muže zkusí
// nejdřív mužské genitivy (Josefa→josef), jinak běžnou tabulku.
func givenNormCtx(given string, genitive bool, sex string) string {
	f := foldASCII(strings.TrimSpace(given))
	for _, form := range []string{f, oldOrthography(f)} {
		if genitive && sex == "m" {
			if can, ok := maleGenitive[form]; ok {
				return can
			}
		}
		if can, ok := variantToCanonical[form]; ok {
			return can
		}
	}
	return oldOrthography(f)
}

// oldOrthography převádí historický pravopis matrik: w→v (Worechowsky→Vorechovsky),
// koncové -ii→-i. Používá se jen na jména a místa, ne na volný text.
func oldOrthography(f string) string {
	f = strings.ReplaceAll(f, "w", "v")
	return f
}

// adjSurnameSuffix sjednotí tvary adjektivních příjmení na mužský nominativ:
// Vořechovská/-ské/-ského/-ski → vorechovsky (analogicky -cký). Aby se
// nerozbila jména jako Růžička/Matouška, musí kmen před -sk/-ck končit
// souhláskou (vorechov-ská ano, ruzi-čka ne).
func adjSurnameSuffix(f string) string {
	for _, grp := range []string{"sk", "ck"} {
		for _, suf := range []string{grp + "eho", grp + "emu", grp + "a", grp + "e", grp + "i", grp + "ym", grp + "ou"} {
			if !strings.HasSuffix(f, suf) {
				continue
			}
			stem := f[:len(f)-len(suf)]
			if len(stem) >= 4 && !strings.ContainsRune("aeiouy", rune(stem[len(stem)-1])) {
				return stem + grp + "y"
			}
		}
	}
	return f
}

// surnameNorm normalizuje příjmení: fold, historický pravopis (w→v), sjednocení
// adjektivních tvarů (-ská/-ski→-sky), strip ženských přípon -ová/-ové a
// (v genitivním kontextu "syn Josefa Slavíka") koncového -a/-y.
func surnameNorm(surname string, genitive bool) string {
	f := oldOrthography(foldASCII(strings.TrimSpace(surname)))
	f = adjSurnameSuffix(f)
	switch {
	case strings.HasSuffix(f, "ova"):
		f = strings.TrimSuffix(f, "ova")
	case strings.HasSuffix(f, "ove"):
		f = strings.TrimSuffix(f, "ove")
	case strings.HasSuffix(f, "sky") || strings.HasSuffix(f, "cky"):
		// adjektivní příjmení už je v kanonickém tvaru — genitiv neřezat
	case genitive && strings.HasSuffix(f, "a") && len(f) > 4:
		f = strings.TrimSuffix(f, "a")
	case genitive && strings.HasSuffix(f, "y") && len(f) > 4:
		f = strings.TrimSuffix(f, "y")
	}
	return f
}

// sexFromGiven odhadne pohlaví ze křestního jména (varianta/kanonické/-a).
func sexFromGiven(given string) string {
	can := givenNorm(given)
	if s, ok := canonicalGivenSex[can]; ok {
		return s
	}
	if strings.HasSuffix(can, "a") || strings.HasSuffix(can, "ie") {
		return "f"
	}
	return ""
}

// placeNorm normalizuje místní jméno přes pádové tvary: fold + odtržení jedné
// koncové samohlásky ("Testov" / "z Testova" / "v Testově" → "testov").
func placeNorm(s string) string {
	f := oldOrthography(foldASCII(strings.TrimSpace(s)))
	if len(f) > 3 && strings.ContainsRune("aeiouy", rune(f[len(f)-1])) {
		f = f[:len(f)-1]
	}
	return f
}

// lexikon povolání/stavu — pro rozpoznání segmentů, které nejsou jméno
var occupationWords = []string{
	"rolnik", "domkar", "chalupnik", "sedlak", "havir", "hornik", "delnik",
	"mistr", "obuvnik", "krejci", "kovar", "truhlar", "zednik", "ucitel",
	"svec", "mlynar", "reznik", "pekar", "tkadlec", "tesar", "nadenik",
	"hostinsky", "obchodnik", "kupec", "baracnik", "vymenkar", "celedin",
	"devecka", "tovarys", "hutnik", "slevac", "strojnik", "zamecnik",
	"kocijas", "slouha", "ovcak", "hajny", "soused", "mestan", "gruntovnik",
	"zahradnik", "familiant", "podruh", "vojin", "vojak", "cetnik", "listonos",
	"zeleznicni", "topic", "strojvedouci", "sladek", "bednar", "kolar", "sklenar",
}

var maritalWords = map[string]string{
	"svobodny": "svobodny", "svobodna": "svobodna",
	"vdovec": "vdovec", "vdova": "vdova", "ovdovela": "vdova", "ovdovely": "vdovec",
	"rozvedeny": "rozvedeny", "rozvedena": "rozvedena",
	"manzelsky": "manzelsky", "nemanzelsky": "nemanzelsky",
}

var (
	reMaiden  = regexp.MustCompile(`(?i)\broz(?:ené|ene|ená|ena|\.)\s+([\p{Lu}][\p{L}]+)`)
	reHouseNo = regexp.MustCompile(`(?:č|c)\.?\s*(?:p|d)?\.?\s*(\d+)|\bčp\.?\s*(\d+)`)
	// "v Kladně", "z Hnidous", "ze Kročehlav", "na Vinařicích"
	rePlace = regexp.MustCompile(`\b(?:v|ve|z|ze|na)\s+([\p{Lu}][\p{L}]+(?:\s+[\p{Lu}][\p{L}]+)?)`)
	// dvě velká slova za sebou = kandidát na jméno
	reNamePair = regexp.MustCompile(`([\p{Lu}][\p{L}]+)\s+([\p{Lu}][\p{L}]+)`)
	reSonOf    = regexp.MustCompile(`(?i)\b(syn|dcera)\b`)
	reListAnd  = regexp.MustCompile(`\s+a\s+([\p{Lu}][\p{L}]+\s+[\p{Lu}][\p{L}]+)`)
)

// parsedPerson je jedna osoba vytěžená z buňky.
type parsedPerson struct {
	Given, Surname  string
	GivenNorm       string
	SurnameNorm     string
	MaidenNorm      string
	Sex             string
	Occupation      string
	MaritalStatus   string
	Place           string
	Raw             string
}

// splitName rozdělí "Slavík František" / "František Slavík" na (křestní, příjmení).
// Heuristika: token, který je ve slovníku křestních jmen, je křestní. Když
// nerozhodne slovník: sloupce matrik píší "Příjmení Křestní", ale genitivní
// úseky ("syn Václava Dvořáka") mají přirozené pořadí "Křestní Příjmení".
func splitName(a, b string, genitive bool, sex string) (given, surname string) {
	known := func(s string) bool {
		f := foldASCII(s)
		if _, ok := variantToCanonical[f]; ok {
			return true
		}
		if genitive && sex != "f" {
			_, ok := maleGenitive[f]
			return ok
		}
		return false
	}
	aGiven, bGiven := known(a), known(b)
	switch {
	case aGiven && !bGiven:
		return a, b
	case bGiven && !aGiven:
		return b, a
	case genitive:
		return a, b
	default:
		return b, a
	}
}

// parsePersonCell vytěží z kombinované buňky ("Slavík František, mistr obuvnický
// v Kladně, syn Josefa Slavíka, rolníka z Hnidous, a Marie roz. Dvořákové")
// hlavní osobu a její rodiče. Vrací (osoba, otec, matka); otec/matka mohou být nil.
func parsePersonCell(cell string) (p parsedPerson, father, mother *parsedPerson) {
	p.Raw = cell
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return
	}

	// rozdělení na část o osobě a část o rodičích (kotva syn/dcera)
	principal, parents := cell, ""
	if loc := reSonOf.FindStringIndex(cell); loc != nil {
		principal, parents = cell[:loc[0]], cell[loc[1]:]
		if strings.EqualFold(cell[loc[0]:loc[1]], "dcera") {
			p.Sex = "f"
		} else {
			p.Sex = "m"
		}
	}

	fillPerson(&p, principal, false)

	if parents != "" {
		// rodiče odděluje " a " — "Josefa Slavíka, rolníka z Hnidous, a Marie roz. Dvořákové"
		fatherPart, motherPart := parents, ""
		if i := findMotherSplit(parents); i >= 0 {
			fatherPart, motherPart = parents[:i], parents[i+3:]
		}
		if f := parseParent(fatherPart, "m"); f != nil {
			father = f
			if p.Surname == "" && p.Sex != "f" {
				p.Surname = f.Surname
				p.SurnameNorm = f.SurnameNorm
			}
		}
		if motherPart != "" {
			if m := parseParent(motherPart, "f"); m != nil {
				mother = m
			}
		}
	}
	return
}

// findMotherSplit najde " a " oddělující otce a matku (poslední výskyt, za kterým
// následuje velké písmeno — jméno matky).
func findMotherSplit(s string) int {
	for i := len(s) - 3; i >= 0; i-- {
		if s[i] != ' ' || s[i+1] != 'a' || s[i+2] != ' ' {
			continue
		}
		rest := strings.TrimSpace(s[i+3:])
		if rest != "" && strings.ToUpper(rest[:1]) == rest[:1] {
			return i
		}
	}
	return -1
}

// parseParent vytěží rodiče z genitivního úseku ("Josefa Slavíka, rolníka z Hnidous").
func parseParent(s, sex string) *parsedPerson {
	s = strings.Trim(strings.TrimSpace(s), ",. ")
	if s == "" {
		return nil
	}
	pp := &parsedPerson{Raw: s, Sex: sex}
	fillPerson(pp, s, true)
	if pp.Given == "" && pp.Surname == "" {
		return nil
	}
	if pp.Sex == "" {
		pp.Sex = sex
	}
	return pp
}

// fillPerson naplní jméno, povolání, stav, místo a rozenou z textového úseku.
// genitive=true pro úseky za "syn/dcera" (jména jsou ve 2. pádě).
func fillPerson(p *parsedPerson, s string, genitive bool) {
	if p.Raw == "" {
		p.Raw = s
	}
	// rozená (dřív, než se chytne jako jméno)
	if m := reMaiden.FindStringSubmatch(s); m != nil {
		p.MaidenNorm = surnameNorm(m[1], genitive)
		s = strings.Replace(s, m[0], "", 1)
		if p.Sex == "" {
			p.Sex = "f"
		}
	}

	segments := strings.Split(s, ",")
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		fseg := foldASCII(seg)
		for _, occ := range occupationWords {
			if strings.Contains(fseg, occ) && p.Occupation == "" {
				p.Occupation = seg
			}
		}
		for w, status := range maritalWords {
			if strings.Contains(fseg, w) && p.MaritalStatus == "" {
				p.MaritalStatus = status
			}
		}
		if m := rePlace.FindStringSubmatch(seg); m != nil && p.Place == "" {
			p.Place = m[1]
		}
		// jméno: první dvojice velkých slov, která není za předložkou místa
		if p.Given == "" && p.Surname == "" {
			if nm := reNamePair.FindStringSubmatch(seg); nm != nil && !isPlaceMatch(seg, nm[0]) {
				g, sn := splitName(nm[1], nm[2], genitive, p.Sex)
				p.Given, p.Surname = g, sn
			} else if p.Given == "" {
				// jednoslovné jméno (dítě: "František"; matka: "Marie")
				words := strings.Fields(seg)
				if len(words) >= 1 && isCapitalized(words[0]) && variantToCanonical[foldASCII(words[0])] != "" {
					p.Given = words[0]
				}
			}
		}
	}

	p.GivenNorm = givenNormCtx(p.Given, genitive, p.Sex)
	if p.Surname != "" {
		p.SurnameNorm = surnameNorm(p.Surname, genitive)
	}
	// pro zobrazení převést genitiv na 1. pád ("Jana Slavíka" → "Jan Slavík")
	if genitive {
		p.Given = nominativeGiven(p.Given, true, p.Sex)
		p.Surname = nominativeSurname(p.Surname, true)
	}
	if p.Sex == "" {
		p.Sex = sexFromGiven(p.Given)
	}
	if p.Sex == "" && strings.HasSuffix(foldASCII(p.Surname), "ova") {
		p.Sex = "f"
	}
}

// isPlaceMatch zjistí, jestli nalezená dvojice slov je ve skutečnosti za
// místní předložkou ("z Dolních Přítočen").
func isPlaceMatch(seg, pair string) bool {
	i := strings.Index(seg, pair)
	if i < 2 {
		return false
	}
	before := strings.Fields(seg[:i])
	if len(before) == 0 {
		return false
	}
	last := foldASCII(before[len(before)-1])
	return last == "v" || last == "ve" || last == "z" || last == "ze" || last == "na"
}

func isCapitalized(w string) bool {
	return w != "" && strings.ToUpper(w[:1]) == w[:1] && strings.ToLower(w[:1]) != w[:1]
}

// splitPeopleList rozdělí buňku s více osobami (svědkové, kmotři) na jednotlivé
// úseky. Odděluje středník, u čárky jen když za ní následuje další jméno-pár.
func splitPeopleList(cell string) []string {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return nil
	}
	if strings.Contains(cell, ";") {
		var out []string
		for _, part := range strings.Split(cell, ";") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	// "Jan Novák, rolník z Kladna a Josef Dvořák, havíř z Buštěhradu"
	if m := reListAnd.FindStringSubmatchIndex(cell); m != nil {
		return []string{strings.TrimSpace(cell[:m[0]]), strings.TrimSpace(cell[m[2]:])}
	}
	return []string{cell}
}
