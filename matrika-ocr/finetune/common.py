# Sdílené utility exportů: schémata, prompt builder (1:1 port z Go), půlení
# skenů (1:1 s split.go), konfigurace half_map.json, cesty repa.
import hashlib
import json
import os
import re

FINETUNE_DIR = os.path.dirname(os.path.abspath(__file__))
MATRIKA_OCR_DIR = os.path.dirname(FINETUNE_DIR)
REPO_ROOT = os.path.dirname(MATRIKA_OCR_DIR)
OCR_OUT = os.path.join(MATRIKA_OCR_DIR, "ocr-out")
SCHEMAS = os.path.join(MATRIKA_OCR_DIR, "schemas")

TYP_LABEL = {
    "narozeni": "kniha narozených",
    "oddani": "kniha oddaných",
    "umrti": "kniha zemřelých",
    "kombinovana": "kombinovaná matrika (narození/oddaní/úmrtí)",
}


def load_config():
    with open(os.path.join(FINETUNE_DIR, "half_map.json"), encoding="utf-8") as f:
        return json.load(f)


def load_schema(typ):
    with open(os.path.join(SCHEMAS, typ + ".json"), encoding="utf-8") as f:
        return json.load(f)


def build_structured_prompt(schema):
    """1:1 port buildStructuredPrompt ze schema.go — train musí == inference."""
    b = []
    b.append("Toto je rozevřená dvojstrana matriky typu „%s\". " % TYP_LABEL[schema["typ"]])
    b.append("Přepiš POUZE ručně psané záznamy, NE předtištěnou hlavičku a NE prázdné řádky. ")
    b.append("Z horní hlavičky vypiš folio a rok. ")
    b.append("Každý záznam (řádek) vrať jako JSON objekt s těmito klíči:\n")
    for c in schema["columns"]:
        b.append("- %s: %s\n" % (c["key"], c["label"]))
    b.append("Každou hodnotu vrať jako STRUČNÝ prostý řetězec (ne pole, ne vnořený objekt). ")
    b.append("Prázdnou buňku vrať jako \"\". Zachovej pořadí záznamů shora dolů. ")
    b.append("Neopakuj řádky a nevymýšlej data. ")
    b.append("Vrať POUZE validní JSON tvaru: {\"folio\":\"\",\"rok\":\"\",\"rows\":[{…}]} — nic dalšího, bez komentářů a bez markdown fence.")
    return "".join(b)


def crop_half(img, side):
    """Půlení dvojstrany s překryvem přes hřbet — stejné poměry jako split.go."""
    w, h = img.size
    if side == "left":
        box = (0, 0, int(w * 0.52), h)
    else:
        box = (int(w * 0.48), 0, w, h)
    return img.crop(box)


def resize_max_side(img, max_side):
    w, h = img.size
    m = max(w, h)
    if m <= max_side:
        return img
    scale = max_side / m
    return img.resize((max(1, round(w * scale)), max(1, round(h * scale))))


def book_dir_for_jsonl(jsonl_path):
    """ocr-out/<typ>/<Kniha [ID]>.jsonl → {repo}/<Kniha [ID]>/"""
    base = os.path.splitext(os.path.basename(jsonl_path))[0]
    return os.path.join(REPO_ROOT, base), base


def iter_records(jsonl_path):
    with open(jsonl_path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            rec = json.loads(line)
            if rec.get("ok"):
                yield rec


def half_target(rec, schema, half_cfg, side):
    """Cílový JSON pro půlku: plná sada klíčů, sloupce druhé půlky prázdné.
    Řádky se zachovávají všechny (i kdyby na půlce byly prázdné) kvůli párování
    indexem při merge — stejně jako mergeLR ve split.go."""
    visible = set(half_cfg[side])
    rows = []
    for row in rec.get("rows", []):
        out = {}
        for c in schema["columns"]:
            k = c["key"]
            out[k] = row.get(k, "") if k in visible else ""
        rows.append(out)
    folio = rec.get("folio", "") if half_cfg.get("folio") in (side, "both") else ""
    rok = rec.get("rok", "") if half_cfg.get("rok") in (side, "both") else ""
    return {"folio": folio, "rok": rok, "rows": rows}


def compact_json(obj):
    return json.dumps(obj, ensure_ascii=False, separators=(",", ":"))


def file_sha256(path, limit=1 << 22):
    """Hash prvních 4 MB (skeny ~1 MB, stačí; rychlé)."""
    h = hashlib.sha256()
    with open(path, "rb") as f:
        h.update(f.read(limit))
    return h.hexdigest()


def folio_num(folio):
    m = re.search(r"\d+", str(folio or ""))
    return int(m.group()) if m else None


def find_font(size=22):
    """TTF s diakritikou pro kontaktní archy (macOS/Linux kandidáti)."""
    from PIL import ImageFont
    candidates = [
        "/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
        "/System/Library/Fonts/Supplemental/Arial.ttf",
        "/System/Library/Fonts/Helvetica.ttc",
        "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
    ]
    for p in candidates:
        if os.path.exists(p):
            try:
                return ImageFont.truetype(p, size)
            except OSError:
                continue
    return ImageFont.load_default()
