package main

import "testing"

func TestParseDate(t *testing.T) {
	cases := []struct {
		in        string
		iso       string
		precision string
		year      int
	}{
		{"30.9.1878", "1878-09-30", "day", 1878},
		{"30. 9. 1878", "1878-09-30", "day", 1878},
		{"dne 30. září 1878", "1878-09-30", "day", 1878},
		{"5. ledna 1901", "1901-01-05", "day", 1901},
		{"12. Jänner 1855", "1855-01-12", "day", 1855},
		{"září 1878", "1878-09", "month", 1878},
		{"1878", "1878", "year", 1878},
		{"", "", "none", 0},
		{"nečitelné", "", "none", 0},
	}
	for _, c := range cases {
		got := parseDate(c.in)
		if got.ISO != c.iso || got.Precision != c.precision || got.Year != c.year {
			t.Errorf("parseDate(%q) = %+v, chci iso=%q precision=%q year=%d", c.in, got, c.iso, c.precision, c.year)
		}
	}
}

func TestParseAgeYears(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"72 let", 72, true},
		{"1 rok", 1, true},
		{"6 měsíců", 0, true},
		{"14 dní", 0, true},
		{"45", 45, true},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseAgeYears(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseAgeYears(%q) = (%d,%v), chci (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestFoldASCII(t *testing.T) {
	if got := foldASCII("Šťastný Vojtěch"); got != "stastny vojtech" {
		t.Errorf("foldASCII = %q", got)
	}
}

func TestGivenNorm(t *testing.T) {
	for _, c := range [][2]string{
		{"Johann", "jan"}, {"Jan", "jan"}, {"Joannes", "jan"},
		{"Wenzel", "vaclav"}, {"Maria", "marie"},
	} {
		if got := givenNorm(c[0]); got != c[1] {
			t.Errorf("givenNorm(%q) = %q, chci %q", c[0], got, c[1])
		}
	}
	// genitivní kontext: "syn Josefa" je Josef, ale samostatná Josefa je žena
	if got := givenNormCtx("Josefa", true, "m"); got != "josef" {
		t.Errorf("givenNormCtx(Josefa, gen, m) = %q, chci josef", got)
	}
	if got := givenNormCtx("Josefa", false, "f"); got != "josefa" {
		t.Errorf("givenNormCtx(Josefa, nom, f) = %q, chci josefa", got)
	}
}

func TestSurnameNorm(t *testing.T) {
	cases := []struct {
		in       string
		genitive bool
		want     string
	}{
		{"Nováková", false, "novak"},
		{"Novákové", true, "novak"},
		{"Slavíka", true, "slavik"},
		{"Slavík", false, "slavik"},
		{"Svoboda", false, "svoboda"},
	}
	for _, c := range cases {
		if got := surnameNorm(c.in, c.genitive); got != c.want {
			t.Errorf("surnameNorm(%q,%v) = %q, chci %q", c.in, c.genitive, got, c.want)
		}
	}
}

func TestParsePersonCellGroom(t *testing.T) {
	cell := "Slavík František, mistr obuvnický v Kladně, syn Josefa Slavíka, rolníka z Hnidous, a Marie roz. Dvořákové"
	p, f, m := parsePersonCell(cell)
	if p.Given != "František" || p.Surname != "Slavík" {
		t.Errorf("hlavní osoba: %+v", p)
	}
	if p.GivenNorm != "frantisek" || p.SurnameNorm != "slavik" {
		t.Errorf("normalizace: %q %q", p.GivenNorm, p.SurnameNorm)
	}
	if p.Sex != "m" {
		t.Errorf("pohlaví: %q", p.Sex)
	}
	if p.Occupation == "" {
		t.Errorf("povolání nezachyceno")
	}
	if f == nil || f.GivenNorm != "josef" || f.SurnameNorm != "slavik" {
		t.Errorf("otec: %+v", f)
	}
	if m == nil || m.GivenNorm != "marie" || m.MaidenNorm != "dvorak" {
		t.Errorf("matka: %+v", m)
	}
}

func TestParsePersonCellDaughter(t *testing.T) {
	cell := "Anna, dcera Václava Dvořáka, domkáře z Buštěhradu, a Kateřiny rozené Novákové"
	p, f, m := parsePersonCell(cell)
	if p.GivenNorm != "anna" || p.Sex != "f" {
		t.Errorf("hlavní osoba: %+v", p)
	}
	if f == nil || f.GivenNorm != "vaclav" || f.SurnameNorm != "dvorak" {
		t.Errorf("otec: %+v", f)
	}
	if m == nil || m.GivenNorm != "katerina" || m.MaidenNorm != "novak" {
		t.Errorf("matka: %+v", m)
	}
}

func TestSplitPeopleList(t *testing.T) {
	got := splitPeopleList("Jan Novák, rolník z Kladna a Josef Dvořák, havíř z Buštěhradu")
	if len(got) != 2 || got[0] != "Jan Novák, rolník z Kladna" || got[1] != "Josef Dvořák, havíř z Buštěhradu" {
		t.Errorf("splitPeopleList = %#v", got)
	}
	got = splitPeopleList("Jan Novák; Josef Dvořák")
	if len(got) != 2 {
		t.Errorf("středník: %#v", got)
	}
}

func TestSplitPlaceHouse(t *testing.T) {
	place, house := splitPlaceHouse("Kročehlavy č. 123")
	if place != "Kročehlavy" || house != "123" {
		t.Errorf("splitPlaceHouse = %q, %q", place, house)
	}
}

func TestJaroWinkler(t *testing.T) {
	if s := jaroWinkler("slavik", "slavik"); s != 1 {
		t.Errorf("identita: %f", s)
	}
	if s := jaroWinkler("slavik", "slavyk"); s < 0.9 {
		t.Errorf("slavik/slavyk: %f", s)
	}
	if s := jaroWinkler("slavik", "dvorak"); s > 0.7 {
		t.Errorf("rozdílná: %f", s)
	}
}
