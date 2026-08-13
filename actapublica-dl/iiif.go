package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"net/http"
	"strings"
)

// maxTile je strop IIPServeru na rozměr výstupu jedné dlaždice (ověřeno:
// full/full vrací jen 2000x1533 podvzorkované; region <=2000x2000 vrací 1:1).
const maxTile = 2000

type imgInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// imageInfo zjistí nativní rozměry obrázku přes IIIF info.json.
func imageInfo(client *http.Client, jp2Path string) (imgInfo, error) {
	u := iipsrvURL + "?IIIF=" + escapeIIIFPath(jp2Path) + "/info.json"
	b, err := fetchBytesRetry(client, u, 3)
	if err != nil {
		return imgInfo{}, err
	}
	var info imgInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return imgInfo{}, fmt.Errorf("info.json: %w", err)
	}
	if info.Width == 0 || info.Height == 0 {
		return imgInfo{}, fmt.Errorf("info.json: chybí width/height v odpovědi")
	}
	return info, nil
}

// fetchRegion stáhne jednu dlaždici v nativním rozlišení.
func fetchRegion(client *http.Client, jp2Path string, x, y, w, h, retries int) ([]byte, error) {
	u := fmt.Sprintf("%s?IIIF=%s/%d,%d,%d,%d/full/0/default.jpg", iipsrvURL, escapeIIIFPath(jp2Path), x, y, w, h)
	return fetchBytesRetry(client, u, retries)
}

// escapeIIIFPath escapuje jen mezery — lomítka musí zůstat syrová, iipsrv
// očekává absolutní cestu v souborovém systému jako hodnotu IIIF= parametru,
// ne standardní IIIF identifikátor (viz README, sekce "Co je ověřeno").
func escapeIIIFPath(p string) string {
	return strings.ReplaceAll(p, " ", "%20")
}

// stitch stáhne celý obrázek po dlaždicích ≤2000×2000 a sešije do jednoho JPEGu.
func stitch(client *http.Client, jp2Path string, width, height, retries, quality int) ([]byte, error) {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y0 := 0; y0 < height; y0 += maxTile {
		h := maxTile
		if y0+h > height {
			h = height - y0
		}
		for x0 := 0; x0 < width; x0 += maxTile {
			w := maxTile
			if x0+w > width {
				w = width - x0
			}
			data, err := fetchRegion(client, jp2Path, x0, y0, w, h, retries)
			if err != nil {
				return nil, fmt.Errorf("dlaždice (%d,%d,%d,%d): %w", x0, y0, w, h, err)
			}
			tile, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("dlaždice (%d,%d,%d,%d): dekódování: %w", x0, y0, w, h, err)
			}
			draw.Draw(dst, image.Rect(x0, y0, x0+w, y0+h), tile, tile.Bounds().Min, draw.Src)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
