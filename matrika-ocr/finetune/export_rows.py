#!/usr/bin/env python3
# Fáze C: řádkové ořezy. Matriky jsou předtištěné tabulky — detekuj vodorovné
# linky (OpenCV), vyřež pásy záznamů z půlstran a spáruj s rows[] z JSONL.
# Bez ruční anotace boxů: páruje se POUZE když počet detekovaných pásů
# == rows_count záznamu (jinak stránka přeskočena) — filtr si přesnost hlídá sám.
#
# Použití:  python3 export_rows.py [--max-side 1536] [--debug 8]
import argparse
import glob
import json
import os

import cv2
import numpy as np
from PIL import Image

from common import (OCR_OUT, TYP_LABEL, book_dir_for_jsonl, compact_json,
                    file_sha256, folio_num, iter_records, load_config,
                    load_schema, resize_max_side)


def build_row_prompt(schema, side):
    """Prompt pro přepis JEDNOHO vyříznutého záznamu (řádku) půlstrany."""
    strana = "LEVÁ" if side == "left" else "PRAVÁ"
    b = [f"Toto je výřez JEDNOHO záznamu (řádku) z matriky typu „{TYP_LABEL[schema['typ']]}\", "
         f"{strana} polovina dvojstrany. "
         "Přepiš ručně psaný obsah záznamu do JSON objektu s těmito klíči:\n"]
    for c in schema["columns"]:
        b.append("- %s: %s\n" % (c["key"], c["label"]))
    b.append("Každou hodnotu vrať jako STRUČNÝ prostý řetězec. Pole, která ve výřezu "
             "nevidíš, vrať jako \"\". Nevymýšlej data. "
             "Vrať POUZE validní JSON objekt — nic dalšího, bez komentářů a bez markdown fence.")
    return "".join(b)


def page_bbox(gray):
    """Bounding box světlé stránky (odřízne černé pozadí skeneru/hřbet)."""
    bright = (gray > 110).astype(np.uint8)
    bright = cv2.morphologyEx(bright, cv2.MORPH_OPEN, np.ones((15, 15), np.uint8))
    contours, _ = cv2.findContours(bright, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    if not contours:
        return 0, 0, gray.shape[1], gray.shape[0]
    x, y, w, h = cv2.boundingRect(max(contours, key=cv2.contourArea))
    return x, y, w, h


def detect_row_bands(bgr, min_h_frac=0.08, min_ink=0.004, max_ink=0.25):
    """Vrátí [(y0,y1)] pásů záznamů mezi vodorovnými linkami tabulky (souřadnice
    v původní půlce). Robustnost: pracuje jen uvnitř světlé stránky; separátory
    z řádkové projekce pokrytí (přežije zakřivení u hřbetu); pás musí protínat
    svislé linky tabulky (vyřadí titul nad hlavičkou) a mít rozumný inkoust."""
    gray_full = cv2.cvtColor(bgr, cv2.COLOR_BGR2GRAY)
    px, py, pw, ph = page_bbox(gray_full)
    gray = gray_full[py:py + ph, px:px + pw]
    h, w = gray.shape
    thr = cv2.adaptiveThreshold(gray, 255, cv2.ADAPTIVE_THRESH_MEAN_C,
                                cv2.THRESH_BINARY_INV, 51, 15)
    horiz = cv2.morphologyEx(thr, cv2.MORPH_OPEN,
                             cv2.getStructuringElement(cv2.MORPH_RECT, (max(20, w // 40), 1)))
    vert = cv2.morphologyEx(thr, cv2.MORPH_OPEN,
                            cv2.getStructuringElement(cv2.MORPH_RECT, (1, max(20, h // 40))))

    # separátory: řádky, kde vodorovné linky pokrývají > 30 % šířky stránky
    coverage = horiz.mean(axis=1) / 255.0
    ys = np.where(coverage > 0.30)[0]
    if len(ys) < 2:
        return [], []
    merged = [int(ys[0])]                       # slouč pásma linek (zakřivení ⇒ víc řádků)
    for y in ys[1:]:
        if y - merged[-1] > 20:
            merged.append(int(y))
    # virtuální spodní separátor: tabulka občas končí bez spodní linky (ořez skenu)
    if h - 8 - merged[-1] > min_h_frac * h:
        merged.append(h - 8)

    ink = cv2.subtract(thr, cv2.add(horiz, vert))   # rukopis = práh minus mřížka
    bands = []
    for y0, y1 in zip(merged, merged[1:]):
        if (y1 - y0) < min_h_frac * h:
            continue                            # hlavička / úzké pásy
        seg = ink[y0 + 6:y1 - 6, int(0.03 * w):int(0.97 * w)]
        density = (seg > 0).mean() if seg.size else 0
        if not (min_ink <= density <= max_ink):
            continue                            # prázdný pás / černé pozadí
        vseg = vert[y0 + 6:y1 - 6, :]           # záznam protínají svislé linky sloupců
        vcols = (vseg > 0).mean(axis=0)         # podíl výšky pásu pokrytý linkou, per x
        if (vcols > 0.5).sum() < 2:
            continue                            # titul/okraj bez mřížky
        bands.append((py + y0, py + y1))        # zpět do souřadnic půlky
    return bands, [py + y for y in merged]


def pick_uniform_window(bands, n):
    """Sloty formuláře mají stejnou výšku — z >n pásů vyber n po sobě jdoucích
    s nejmenším rozptylem výšek (typicky ustřihne hlavičku/titul na kraji)."""
    if len(bands) == n:
        return bands
    if len(bands) < n:
        return None
    best, best_cv = None, None
    for i in range(len(bands) - n + 1):
        win = bands[i:i + n]
        hs = np.array([y1 - y0 for y0, y1 in win], dtype=float)
        cv_ = hs.std() / hs.mean()
        if best_cv is None or cv_ < best_cv:
            best, best_cv = win, cv_
    return best if best_cv is not None and best_cv < 0.35 else None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=os.path.join(os.path.dirname(__file__), "dataset"))
    ap.add_argument("--max-side", type=int, default=1536)
    ap.add_argument("--debug", type=int, default=8, help="počet debug overlayů s detekcí")
    ap.add_argument("--quality", type=int, default=90)
    args = ap.parse_args()

    cfg = load_config()
    exclude = set(cfg["exclude_jsonl"])
    val_books = set(cfg["val_books"])
    val_tail = cfg["val_folio_tail"]

    img_dir = os.path.join(args.out, "images")
    dbg_dir = os.path.join(args.out, "debug_rows")
    os.makedirs(img_dir, exist_ok=True)
    os.makedirs(dbg_dir, exist_ok=True)
    os.makedirs(os.path.join(args.out, "splits"), exist_ok=True)

    samples, dbg_left = [], args.debug
    stats = {}  # book -> [pages, matched, rows]

    for jsonl_path in sorted(glob.glob(os.path.join(OCR_OUT, "*", "*.jsonl"))):
        if os.path.basename(jsonl_path) in exclude:
            continue
        book_dir, book = book_dir_for_jsonl(jsonl_path)
        if not os.path.isdir(book_dir):
            continue
        recs = list(iter_records(jsonl_path))
        folio_cut = None
        if book in val_tail:
            folios = sorted({f for f in (folio_num(r.get("folio")) for r in recs) if f is not None})
            if folios:
                folio_cut = folios[max(0, len(folios) - val_tail[book])]

        st = stats.setdefault(book, [0, 0, 0])
        for rec in recs:
            typ = rec["typ"]
            half_cfg = cfg["typy"].get(typ)
            img_path = os.path.join(book_dir, rec["file"])
            if not half_cfg or not os.path.exists(img_path) or not rec.get("rows"):
                continue
            schema = load_schema(typ)
            fnum = folio_num(rec.get("folio"))
            split = "val" if (book in val_books or (folio_cut is not None and fnum is not None and fnum >= folio_cut)) else "train"
            src = cv2.imread(img_path)
            if src is None:
                continue
            H, W = src.shape[:2]
            scan = os.path.splitext(rec["file"])[0]
            h_sha = file_sha256(img_path)

            for side in ("left", "right"):
                st[0] += 1
                x0, x1 = (0, int(W * 0.52)) if side == "left" else (int(W * 0.48), W)
                half = src[:, x0:x1]
                bands, seps = detect_row_bands(half)

                if dbg_left > 0:                # overlay detekce pro vizuální QA
                    ov = half.copy()
                    for y in seps:
                        cv2.line(ov, (0, y), (ov.shape[1], y), (255, 0, 0), 3)
                    for y0, y1 in bands:
                        cv2.rectangle(ov, (10, y0 + 4), (ov.shape[1] - 10, y1 - 4), (0, 0, 255), 6)
                    small = cv2.resize(ov, (ov.shape[1] // 3, ov.shape[0] // 3))
                    cv2.imwrite(os.path.join(
                        dbg_dir, f"{book}_{scan}_{side[0].upper()}_n{len(bands)}_gt{len(rec['rows'])}.jpg".replace(" ", "_")), small)
                    dbg_left -= 1

                bands = pick_uniform_window(bands, len(rec["rows"]))
                if bands is None:
                    continue                    # nesouhlas počtu → stránku vynech
                st[1] += 1

                visible = set(half_cfg[side])
                prompt = build_row_prompt(schema, side)
                for ri, ((y0, y1), row) in enumerate(zip(bands, rec["rows"])):
                    pad = 10
                    crop = half[max(0, y0 - pad):min(half.shape[0], y1 + pad), :]
                    pil = Image.fromarray(cv2.cvtColor(crop, cv2.COLOR_BGR2RGB))
                    pil = resize_max_side(pil, args.max_side)
                    sid = f"{book}_{scan}_{side[0].upper()}_r{ri + 1}".replace(" ", "_")
                    out_img = os.path.join(img_dir, sid + ".jpg")
                    pil.save(out_img, quality=args.quality)
                    target = {c["key"]: (row.get(c["key"], "") if c["key"] in visible else "")
                              for c in schema["columns"]}
                    samples.append((split, {
                        "id": sid, "kind": "row",
                        "image": os.path.relpath(out_img, args.out),
                        "prompt": prompt,
                        "target": compact_json(target),
                        "meta": {"book": book, "scan": rec["file"], "side": side, "typ": typ,
                                 "row_index": ri, "rok": rec.get("rok", ""), "src_sha256": h_sha},
                    }))
                    st[2] += 1

    counts = {}
    for split in ("train", "val"):
        rows = [s for sp, s in samples if sp == split]
        counts[split] = len(rows)
        with open(os.path.join(args.out, f"rows_{split}.jsonl"), "w", encoding="utf-8") as f:
            for s in rows:
                f.write(json.dumps(s, ensure_ascii=False) + "\n")
        with open(os.path.join(args.out, "splits", f"rows_{split}.txt"), "w", encoding="utf-8") as f:
            for s in rows:
                f.write(f"{s['id']}\t{s['meta']['src_sha256']}\n")

    print("export_rows — úspěšnost párování (půlstrany):")
    for book, (pages, matched, rows) in stats.items():
        pct = 100.0 * matched / pages if pages else 0
        print(f"  {book}: {matched}/{pages} půlstran spárováno ({pct:.0f} %), {rows} řádkových vzorků")
    print(f"→ rows_train.jsonl={counts['train']}, rows_val.jsonl={counts['val']}, debug_rows/")


if __name__ == "__main__":
    main()
