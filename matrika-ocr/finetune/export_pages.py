#!/usr/bin/env python3
# Fáze A: JSONL ground truth (ocr-out) → trénovací dataset stránek pro Qwen-VL.
# Každá dvojstrana → 2 vzorky (levá/pravá půlka, jako produkční --split lr):
#   {image, prompt (1:1 s matrika-ocr + L/R hint), target JSON (sloupce půlky)}.
# Split train/val po knihách/foliových blocích (žádný náhodný per-page leakage).
#
# Použití:  python3 export_pages.py [--max-side 1280] [--out dataset] [--sheets 12]
import argparse
import glob
import json
import os
import sys

from PIL import Image, ImageDraw

from common import (OCR_OUT, book_dir_for_jsonl, build_structured_prompt,
                    compact_json, crop_half, file_sha256, find_font, folio_num,
                    half_target, iter_records, load_config, load_schema,
                    resize_max_side)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=os.path.join(os.path.dirname(__file__), "dataset"))
    ap.add_argument("--max-side", type=int, default=1280, help="delší strana půlky po resize")
    ap.add_argument("--sheets", type=int, default=12, help="počet vzorků na kontaktní archy (0=vypnout)")
    ap.add_argument("--quality", type=int, default=90)
    args = ap.parse_args()

    cfg = load_config()
    exclude = set(cfg["exclude_jsonl"])
    val_books = set(cfg["val_books"])
    val_tail = cfg["val_folio_tail"]

    img_dir = os.path.join(args.out, "images")
    os.makedirs(img_dir, exist_ok=True)
    os.makedirs(os.path.join(args.out, "splits"), exist_ok=True)

    samples = []          # (split, sample_dict)
    seen_hashes = {}      # dedup duplicitních skenů téže strany (case 0076/0077)
    stats = {"jsonl": 0, "records": 0, "skipped_dup": 0, "skipped_noimg": 0, "samples": 0}

    for jsonl_path in sorted(glob.glob(os.path.join(OCR_OUT, "*", "*.jsonl"))):
        if os.path.basename(jsonl_path) in exclude:
            print(f"  – přeskočeno (generované): {os.path.basename(jsonl_path)}")
            continue
        book_dir, book = book_dir_for_jsonl(jsonl_path)
        if not os.path.isdir(book_dir):
            print(f"  ! kniha nenalezena: {book_dir}", file=sys.stderr)
            continue
        stats["jsonl"] += 1

        # foliová hranice pro val (posledních N folií knihy)
        recs = list(iter_records(jsonl_path))
        folio_cut = None
        if book in val_tail:
            folios = sorted({f for f in (folio_num(r.get("folio")) for r in recs) if f is not None})
            if folios:
                folio_cut = folios[max(0, len(folios) - val_tail[book])]

        for rec in recs:
            typ = rec["typ"]
            half_cfg = cfg["typy"].get(typ)
            if not half_cfg:
                print(f"  ! {book}/{rec['file']}: typ {typ} nemá half_map — přeskočeno", file=sys.stderr)
                continue
            img_path = os.path.join(book_dir, rec["file"])
            if not os.path.exists(img_path):
                stats["skipped_noimg"] += 1
                continue
            h = file_sha256(img_path)
            if h in seen_hashes:
                print(f"  – duplicitní sken: {book}/{rec['file']} == {seen_hashes[h]}")
                stats["skipped_dup"] += 1
                continue
            seen_hashes[h] = f"{book}/{rec['file']}"
            stats["records"] += 1

            schema = load_schema(typ)
            prompt = build_structured_prompt(schema)
            fnum = folio_num(rec.get("folio"))
            split = "val" if (book in val_books or (folio_cut is not None and fnum is not None and fnum >= folio_cut)) else "train"

            src = Image.open(img_path).convert("RGB")
            scan = os.path.splitext(rec["file"])[0]
            for side in ("left", "right"):
                half = resize_max_side(crop_half(src, side), args.max_side)
                sid = f"{book}_{scan}_{side[0].upper()}".replace(" ", "_")
                out_img = os.path.join(img_dir, sid + ".jpg")
                half.save(out_img, quality=args.quality)
                target = half_target(rec, schema, half_cfg, side)
                samples.append((split, {
                    "id": sid,
                    "kind": "page_half",
                    "image": os.path.relpath(out_img, args.out),
                    "prompt": prompt + half_cfg["hint_" + side],
                    "target": compact_json(target),
                    "meta": {"book": book, "scan": rec["file"], "side": side, "typ": typ,
                             "folio": rec.get("folio", ""), "rok": rec.get("rok", ""),
                             "n_rows": len(rec.get("rows", [])), "src_sha256": h},
                }))
                stats["samples"] += 1

    # zápis train/val JSONL + manifesty splitů
    counts = {}
    for split in ("train", "val"):
        rows = [s for sp, s in samples if sp == split]
        counts[split] = len(rows)
        with open(os.path.join(args.out, f"pages_{split}.jsonl"), "w", encoding="utf-8") as f:
            for s in rows:
                f.write(json.dumps(s, ensure_ascii=False) + "\n")
        with open(os.path.join(args.out, "splits", f"pages_{split}.txt"), "w", encoding="utf-8") as f:
            for s in rows:
                f.write(f"{s['id']}\t{s['meta']['src_sha256']}\n")

    if args.sheets:
        contact_sheets(args.out, [s for _, s in samples], args.sheets)

    print(f"\nexport_pages: {stats['jsonl']} knih, {stats['records']} dvojstran → "
          f"{stats['samples']} vzorků (train={counts['train']}, val={counts['val']}); "
          f"dedup={stats['skipped_dup']}, bez obrázku={stats['skipped_noimg']}")
    print(f"→ {args.out}/pages_train.jsonl, pages_val.jsonl, images/, contact_sheets/")


def contact_sheets(out_dir, samples, n):
    """Kontaktní archy: půlka skenu + vyrenderovaný target vedle — vizuální QA."""
    sheet_dir = os.path.join(out_dir, "contact_sheets")
    os.makedirs(sheet_dir, exist_ok=True)
    font = find_font(16)
    # vzorky rozprostřít napříč knihami/stranami
    step = max(1, len(samples) // n)
    picks = samples[::step][:n]
    for i, s in enumerate(picks):
        img = Image.open(os.path.join(out_dir, s["image"]))
        img.thumbnail((900, 1100))
        target = json.loads(s["target"])
        lines = [f"[{s['id']}]  rows={len(target['rows'])}  folio={target['folio']!r} rok={target['rok']!r}", ""]
        for ri, row in enumerate(target["rows"]):
            lines.append(f"— řádek {ri + 1} —")
            for k, v in row.items():
                if v:
                    lines.extend(wrap(f"{k}: {v}", 60))
            lines.append("")
        canvas = Image.new("RGB", (img.width + 760, max(img.height, 40 + 19 * len(lines))), "white")
        canvas.paste(img, (0, 0))
        d = ImageDraw.Draw(canvas)
        y = 16
        for ln in lines[:120]:
            d.text((img.width + 16, y), ln, fill="black", font=font)
            y += 19
        canvas.save(os.path.join(sheet_dir, f"sheet_{i:02d}_{s['id']}.jpg"), quality=85)


def wrap(text, width):
    out = []
    while len(text) > width:
        cut = text.rfind(" ", 0, width)
        cut = cut if cut > width // 2 else width
        out.append(text[:cut])
        text = text[cut:].lstrip()
    out.append(text)
    return out


if __name__ == "__main__":
    main()
