package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
)

// deriveMaxSide je strop vysokého rozlišení Sonnetu 5 pro OCR vstup (viz
// matrika-ocr/finetune) — dlouhá hrana půlky se na něj zmenší, pokud je větší.
const deriveMaxSide = 2576

// deriveHalves rozřízne sešitou dvojstranu na L/R půlky se stejnou geometrií
// jako cropHalf v matrika-ocr/split.go (52%/48% s překryvem přes hřbet), aby
// stránky exportované pro trénink (finetune/export_pages.py, half_map.json)
// seděly 1:1 s ground truth z Claude-Max průchodu. Uloží do <dir>/ocr/.
func deriveHalves(dir string, n int, raw []byte, quality int) error {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	ocrDir := filepath.Join(dir, "ocr")
	if err := os.MkdirAll(ocrDir, 0o755); err != nil {
		return err
	}
	for _, side := range []struct{ name, suffix string }{{"left", "L"}, {"right", "R"}} {
		half := downscaleToMax(cropHalf(src, side.name), deriveMaxSide)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, half, &jpeg.Options{Quality: quality}); err != nil {
			return err
		}
		name := fmt.Sprintf("%04d-%s.jpg", n, side.suffix)
		if err := os.WriteFile(filepath.Join(ocrDir, name), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func cropHalf(src image.Image, side string) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	var x0, x1 int
	if side == "left" {
		x0, x1 = 0, int(float64(w)*0.52)
	} else {
		x0, x1 = int(float64(w)*0.48), w
	}
	dst := image.NewRGBA(image.Rect(0, 0, x1-x0, h))
	draw.Draw(dst, dst.Bounds(), src, image.Pt(b.Min.X+x0, b.Min.Y), draw.Src)
	return dst
}

// downscaleToMax zmenší delší stranu na maxSide (nejbližší soused — stejný
// přístup jako downscaleIfNeeded v matrika-ocr/main.go). Menší obrázky vrací
// beze změny.
func downscaleToMax(src image.Image, maxSide int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	longest := w
	if h > w {
		longest = h
	}
	if longest <= maxSide {
		return src
	}
	scale := float64(maxSide) / float64(longest)
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + int(float64(y)/scale)
		for x := 0; x < nw; x++ {
			sx := b.Min.X + int(float64(x)/scale)
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
