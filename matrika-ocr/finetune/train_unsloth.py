#!/usr/bin/env python3
# Unsloth QLoRA fine-tuning Qwen2.5-VL-7B na matriční OCR dataset.
# SPOUŠTÍ SE NA DGX SPARK (ne na Macu) — viz README.md, sekce „Trénink na Sparku".
#
#   python3 train_unsloth.py --data dataset --epochs 2 --output out/qwen-matriky-lora
#   python3 train_unsloth.py --data dataset --max-steps 20 --output out/dryrun   # dry-run
#
# Poznámky:
#  - loss se počítá JEN na assistant tokenech (train_on_responses_only řeší collator)
#  - vision encoder zůstává zmrazený (stabilita, málo dat); ladí se jazykové vrstvy
#  - mezi běhy na Sparku: sync && sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches'
import argparse
import json
import os


def load_samples(data_dir, files):
    out = []
    for fn in files:
        p = os.path.join(data_dir, fn)
        if not os.path.exists(p):
            print(f"  – {fn} chybí, přeskočeno")
            continue
        with open(p, encoding="utf-8") as f:
            out += [json.loads(l) for l in f if l.strip()]
    return out


def to_conversation(sample, data_dir):
    """Náš JSONL → Unsloth/HF messages formát s PIL obrázkem."""
    from PIL import Image
    img = Image.open(os.path.join(data_dir, sample["image"])).convert("RGB")
    return {"messages": [
        {"role": "user", "content": [
            {"type": "image", "image": img},
            {"type": "text", "text": sample["prompt"]},
        ]},
        {"role": "assistant", "content": [
            {"type": "text", "text": sample["target"]},
        ]},
    ]}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--data", default="dataset")
    ap.add_argument("--files", nargs="*", default=["pages_train.jsonl", "rows_train.jsonl"])
    ap.add_argument("--base", default="unsloth/Qwen2.5-VL-7B-Instruct")
    ap.add_argument("--output", default="out/qwen-matriky-lora")
    ap.add_argument("--epochs", type=float, default=2)
    ap.add_argument("--max-steps", type=int, default=0, help=">0 = dry-run na N kroků")
    ap.add_argument("--batch", type=int, default=2)
    ap.add_argument("--grad-accum", type=int, default=4)
    ap.add_argument("--lr", type=float, default=2e-4)
    ap.add_argument("--rank", type=int, default=16)
    ap.add_argument("--merge", action="store_true", help="po tréninku uložit merged bf16 model pro vLLM")
    args = ap.parse_args()

    # Unsloth importovat PŘED transformers (patchuje kernely)
    from unsloth import FastVisionModel
    from unsloth.trainer import UnslothVisionDataCollator
    from trl import SFTConfig, SFTTrainer

    model, tokenizer = FastVisionModel.from_pretrained(
        args.base,
        load_in_4bit=True,          # QLoRA — 7B se vejde bohatě do 128GB UMA
        use_gradient_checkpointing="unsloth",
    )
    model = FastVisionModel.get_peft_model(
        model,
        finetune_vision_layers=False,     # ViT zmrazený — málo dat, stabilita
        finetune_language_layers=True,
        finetune_attention_modules=True,
        finetune_mlp_modules=True,
        r=args.rank, lora_alpha=args.rank, lora_dropout=0.0, bias="none",
        random_state=3407,
    )

    samples = load_samples(args.data, args.files)
    print(f"trénovacích vzorků: {len(samples)} (stránky+řádky)")
    dataset = [to_conversation(s, args.data) for s in samples]

    FastVisionModel.for_training(model)
    trainer = SFTTrainer(
        model=model,
        tokenizer=tokenizer,
        data_collator=UnslothVisionDataCollator(model, tokenizer),  # povinné pro VLM
        train_dataset=dataset,
        args=SFTConfig(
            per_device_train_batch_size=args.batch,
            gradient_accumulation_steps=args.grad_accum,
            num_train_epochs=args.epochs,
            max_steps=args.max_steps if args.max_steps > 0 else -1,
            learning_rate=args.lr,
            warmup_ratio=0.05,
            lr_scheduler_type="cosine",
            optim="adamw_8bit",
            weight_decay=0.01,
            logging_steps=5,
            save_strategy="epoch",
            output_dir=args.output,
            bf16=True,
            report_to="none",
            seed=3407,
            # VLM specifika (viz Unsloth vision docs):
            remove_unused_columns=False,
            dataset_text_field="",
            dataset_kwargs={"skip_prepare_dataset": True},
            max_seq_length=8192,
        ),
    )
    trainer.train()

    model.save_pretrained(args.output)            # LoRA adapter
    tokenizer.save_pretrained(args.output)
    print(f"→ adapter uložen: {args.output}")
    if args.merge:
        merged = args.output.rstrip("/") + "-merged"
        model.save_pretrained_merged(merged, tokenizer)   # bf16 pro vLLM serve
        print(f"→ merged model pro vLLM: {merged}")


if __name__ == "__main__":
    main()
