#!/usr/bin/env python3
"""Fine-tune an open embedding base (default bge-base-zh) on Yunque data.

Input: train.jsonl / eval.jsonl with {"anchor": ..., "positive": ...} per line
(produced by `go run ./cmd/embed-data-export`). Training uses
MultipleNegativesRankingLoss, which draws negatives in-batch, so only positive
pairs are required.

Run on a GPU machine (NOT inside the Go repo runtime):

    pip install -r requirements.txt   # + torch for your CUDA
    python finetune.py --train train.jsonl --eval eval.jsonl \
        --base BAAI/bge-base-zh-v1.5 --out ./yunque-embed --epochs 2 --batch 64

Output: a SentenceTransformer model dir (servable via TEI / sentence-transformers,
or further shrunk with distill.py).
"""
import argparse

from datasets import load_dataset
from sentence_transformers import (
    SentenceTransformer,
    SentenceTransformerTrainer,
    SentenceTransformerTrainingArguments,
    losses,
)


def pick_precision() -> dict:
    """Choose bf16/fp16 based on the available accelerator.

    Supports NVIDIA CUDA and Huawei Ascend NPU (昇腾, via CANN + torch_npu).
    Ascend 910B/910C (e.g. Atlas 800I A2/A3) handle BF16 well.
    """
    import torch

    try:
        import torch_npu  # noqa: F401  Ascend/CANN PyTorch adapter
        if hasattr(torch, "npu") and torch.npu.is_available():
            print("accelerator: Ascend NPU (torch_npu), using bf16")
            return {"bf16": True}
    except Exception:
        pass

    if torch.cuda.is_available():
        major = torch.cuda.get_device_capability()[0]
        if major >= 8:  # Ampere+ supports bf16
            print("accelerator: CUDA (bf16)")
            return {"bf16": True}
        print("accelerator: CUDA (fp16)")
        return {"fp16": True}

    print("accelerator: CPU (no mixed precision)")
    return {}


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--train", default="train.jsonl")
    ap.add_argument("--eval", default="eval.jsonl")
    ap.add_argument("--base", default="BAAI/bge-base-zh-v1.5",
                    help="base model (bge-small-zh-v1.5 / bge-base-zh-v1.5 / bge-m3)")
    ap.add_argument("--out", default="./yunque-embed")
    ap.add_argument("--epochs", type=float, default=2.0)
    ap.add_argument("--batch", type=int, default=64)
    ap.add_argument("--lr", type=float, default=2e-5)
    args = ap.parse_args()

    model = SentenceTransformer(args.base)

    data_files = {"train": args.train}
    try:
        with open(args.eval):
            data_files["eval"] = args.eval
    except OSError:
        pass
    ds = load_dataset("json", data_files=data_files)
    # MultipleNegativesRankingLoss expects exactly (anchor, positive) columns.
    keep = ["anchor", "positive"]
    ds = ds.remove_columns([c for c in ds["train"].column_names if c not in keep])

    loss = losses.MultipleNegativesRankingLoss(model)

    targs = SentenceTransformerTrainingArguments(
        output_dir=args.out,
        num_train_epochs=args.epochs,
        per_device_train_batch_size=args.batch,
        learning_rate=args.lr,
        warmup_ratio=0.1,
        logging_steps=50,
        save_strategy="epoch",
        report_to=[],
        **pick_precision(),
    )

    trainer = SentenceTransformerTrainer(
        model=model,
        args=targs,
        train_dataset=ds["train"],
        loss=loss,
    )
    trainer.train()

    model.save_pretrained(args.out)
    print(f"Saved fine-tuned embedding model to {args.out}")
    print("Serve it via TEI/sentence-transformers, point Yunque EMBED_BASE_URL at it,")
    print("or shrink it with: python distill.py --model", args.out)


if __name__ == "__main__":
    main()
