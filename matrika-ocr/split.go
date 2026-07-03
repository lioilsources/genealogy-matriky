package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
)

// splitExtract rozpůlí dvojstranu na levou/pravou půlku, každou pošle zvlášť a spojí.
// Vrací (rec, true) při úspěchu cropu; (—, false) když obrázek nejde rozdělit
// (volající pak spadne na zpracování celého skenu).
func splitExtract(client *http.Client, opt options, schema *Schema, prompt, path, rel string) (StructuredRecord, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return StructuredRecord{}, false
	}
	left, lok := cropHalf(raw, "left")
	right, rok := cropHalf(raw, "right")
	if !lok || !rok {
		return StructuredRecord{}, false
	}
	lURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(left)
	rURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(right)

	const leftHint = "\n\n(Toto je LEVÁ polovina rozevřené dvojstrany — číslo, oddávající, " +
		"místo a data oddavek, a jména/stav/rodiče. Pole, která na této polovině nevidíš, nech prázdná.)"
	const rightHint = "\n\n(Toto je PRAVÁ polovina rozevřené dvojstrany — místo zrození, datum " +
		"narození, stav (svobodný/ovdovělý/rozvedený), svědkové a poznamenání. " +
		"Pole, která na této polovině nevidíš, nech prázdná.)"

	lrec := extractOne(client, opt, schema, prompt+leftHint, lURL, rel)
	rrec := extractOne(client, opt, schema, prompt+rightHint, rURL, rel)
	return mergeLR(opt, rel, schema, lrec, rrec), true
}

// cropHalf ořízne levou/pravou polovinu skenu (s mírným překryvem přes hřbet) a vrátí JPEG.
func cropHalf(raw []byte, side string) ([]byte, bool) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, false
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	var x0, x1 int
	if side == "left" {
		x0, x1 = 0, int(float64(w)*0.52) // levá půlka + malý překryv přes hřbet
	} else {
		x0, x1 = int(float64(w)*0.48), w // pravá půlka + malý překryv
	}
	dst := image.NewRGBA(image.Rect(0, 0, x1-x0, h))
	for y := 0; y < h; y++ {
		for x := x0; x < x1; x++ {
			dst.Set(x-x0, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 90}); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// mergeLR spojí řádky levé a pravé půlky podle indexu (neprázdné pole vyhrává, levá má přednost).
func mergeLR(opt options, rel string, schema *Schema, l, r StructuredRecord) StructuredRecord {
	rec := newRec(rel, schema)
	rec.Folio = firstNonEmpty(l.Folio, r.Folio)
	rec.Rok = firstNonEmpty(l.Rok, r.Rok)
	rec.Model = firstNonEmpty(l.Model, r.Model)
	rec.PromptTokens = l.PromptTokens + r.PromptTokens
	rec.CompletionTokens = l.CompletionTokens + r.CompletionTokens
	rec.DurationMs = l.DurationMs + r.DurationMs
	rec.Attempts = l.Attempts + r.Attempts

	if !l.OK && !r.OK {
		return failStructured(rec, "obě poloviny selhaly: "+l.Error+" | "+r.Error)
	}
	rec.OK = true

	n := len(l.Rows)
	if len(r.Rows) > n {
		n = len(r.Rows)
	}
	for i := 0; i < n; i++ {
		row := map[string]string{}
		for _, c := range schema.Columns {
			row[c.Key] = ""
		}
		if i < len(l.Rows) {
			for k, v := range l.Rows[i] {
				if v != "" {
					row[k] = v
				}
			}
		}
		if i < len(r.Rows) {
			for k, v := range r.Rows[i] {
				if v != "" && row[k] == "" {
					row[k] = v
				}
			}
		}
		rec.Rows = append(rec.Rows, row)
	}
	rec.RowsCount = len(rec.Rows)

	if opt.keepRaw {
		rec.Raw = "=== LEVÁ ===\n" + l.Raw + "\n=== PRAVÁ ===\n" + r.Raw
	}
	if len(l.Rows) != len(r.Rows) {
		rec.Lint.Issues = append(rec.Lint.Issues,
			fmt.Sprintf("split: nesouhlasí počet řádků (L=%d, P=%d) — možný posun párování", len(l.Rows), len(r.Rows)))
	}
	lintPage(schema, &rec)
	return rec
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
