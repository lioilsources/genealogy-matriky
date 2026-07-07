#!/usr/bin/env python3
# Eval harness: pošle val vzorky na OpenAI-kompatibilní endpoint (vLLM na Sparku)
# a spočítá CER, field-level přesnost a JSON-validity — base vs. tuned model.
# Bez fine-tune baseline: python3 eval_cer.py --url http://spark:8000/v1 --model Qwen/Qwen2.5-VL-7B-Instruct
# Po fine-tune:           python3 eval_cer.py --url http://spark:8000/v1 --model qwen-matriky --out report_tuned.json
import argparse
import base64
import json
import os
import re
import sys
import urllib.request


def levenshtein(a, b):
    if a == b:
        return 0
    if not a:
        return len(b)
    if not b:
        return len(a)
    prev = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        cur = [i]
        for j, cb in enumerate(b, 1):
            cur.append(min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + (ca != cb)))
        prev = cur
    return prev[-1]


def cer(gt, pred):
    return levenshtein(gt, pred) / max(1, len(gt))


def parse_model_json(text):
    """Tolerantní parse: strip fence, vezmi první {...} blok."""
    text = text.strip()
    text = re.sub(r"^```(json)?|```$", "", text, flags=re.M).strip()
    start = text.find("{")
    if start < 0:
        return None
    depth = 0
    for i, ch in enumerate(text[start:], start):
        if ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0:
                try:
                    return json.loads(text[start:i + 1])
                except json.JSONDecodeError:
                    return None
    return None


def call_model(url, model, api_key, prompt, image_path, timeout=300):
    with open(image_path, "rb") as f:
        b64 = base64.standard_b64encode(f.read()).decode()
    payload = {
        "model": model,
        "temperature": 0,
        "max_tokens": 4096,
        "messages": [{"role": "user", "content": [
            {"type": "image_url", "image_url": {"url": "data:image/jpeg;base64," + b64}},
            {"type": "text", "text": prompt},
        ]}],
    }
    req = urllib.request.Request(
        url.rstrip("/") + "/chat/completions",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json",
                 "Authorization": "Bearer " + (api_key or "none")})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        out = json.load(resp)
    return out["choices"][0]["message"]["content"]


def rows_of(target_obj):
    """Sjednocení tvarů: page target má {folio,rok,rows[]}, row target je rovnou objekt."""
    if isinstance(target_obj, dict) and "rows" in target_obj:
        return target_obj.get("rows") or []
    return [target_obj] if isinstance(target_obj, dict) else []


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", required=True, help="OpenAI-kompatibilní base URL (vLLM)")
    ap.add_argument("--model", required=True)
    ap.add_argument("--api-key", default=os.environ.get("OPENAI_API_KEY", ""))
    ap.add_argument("--data", default=os.path.join(os.path.dirname(__file__), "dataset"))
    ap.add_argument("--files", nargs="*", default=["pages_val.jsonl", "rows_val.jsonl"])
    ap.add_argument("--limit", type=int, default=0, help="omezit počet vzorků (rychlá sonda)")
    ap.add_argument("--out", default="report_base.json")
    args = ap.parse_args()

    per_book = {}
    total = {"n": 0, "json_valid": 0, "rowcount_match": 0, "cer_sum": 0.0, "cer_n": 0,
             "field_match": 0, "field_total": 0}
    details = []

    samples = []
    for fn in args.files:
        p = os.path.join(args.data, fn)
        if not os.path.exists(p):
            continue
        with open(p, encoding="utf-8") as f:
            samples += [json.loads(l) for l in f if l.strip()]
    if args.limit:
        samples = samples[:args.limit]

    for i, s in enumerate(samples):
        img = os.path.join(args.data, s["image"])
        try:
            reply = call_model(args.url, args.model, args.api_key, s["prompt"], img)
        except Exception as e:  # endpoint down / timeout — reportuj a pokračuj
            print(f"  ! {s['id']}: {e}", file=sys.stderr)
            reply = ""
        gt = json.loads(s["target"])
        pred = parse_model_json(reply)
        book = s["meta"]["book"]
        agg = per_book.setdefault(book, dict(total))
        for a in (total, agg):
            a["n"] += 1
        drec = {"id": s["id"], "json_valid": pred is not None}
        if pred is not None:
            for a in (total, agg):
                a["json_valid"] += 1
            gt_rows, pr_rows = rows_of(gt), rows_of(pred)
            if len(gt_rows) == len(pr_rows):
                for a in (total, agg):
                    a["rowcount_match"] += 1
            cers = []
            for gr, pr in zip(gt_rows, pr_rows):
                if not isinstance(pr, dict):
                    continue
                for k, gv in gr.items():
                    pv = str(pr.get(k, "") or "")
                    for a in (total, agg):
                        a["field_total"] += 1
                        a["field_match"] += (gv.strip() == pv.strip())
                    if gv.strip():
                        c = cer(gv.strip(), pv.strip())
                        cers.append(c)
                        for a in (total, agg):
                            a["cer_sum"] += c
                            a["cer_n"] += 1
            drec["cer_mean"] = round(sum(cers) / len(cers), 4) if cers else None
        details.append(drec)
        if (i + 1) % 10 == 0:
            print(f"  {i + 1}/{len(samples)} …")

    def fmt(a):
        return {
            "vzorky": a["n"],
            "json_valid_%": round(100 * a["json_valid"] / max(1, a["n"]), 1),
            "rowcount_match_%": round(100 * a["rowcount_match"] / max(1, a["n"]), 1),
            "CER_neprazdnych": round(a["cer_sum"] / max(1, a["cer_n"]), 4),
            "field_exact_%": round(100 * a["field_match"] / max(1, a["field_total"]), 1),
        }

    report = {"model": args.model, "celkem": fmt(total),
              "po_knihach": {b: fmt(a) for b, a in per_book.items()},
              "detaily": details}
    with open(args.out, "w", encoding="utf-8") as f:
        json.dump(report, f, ensure_ascii=False, indent=2)
    print(json.dumps({"model": args.model, "celkem": report["celkem"],
                      "po_knihach": report["po_knihach"]}, ensure_ascii=False, indent=2))
    print(f"→ {args.out}")


if __name__ == "__main__":
    main()
