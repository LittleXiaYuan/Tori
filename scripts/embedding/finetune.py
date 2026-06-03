#!/usr/bin/env python3
"""Fine-tune an open embedding base (default bge-base-zh) on Yunque data.

Input: train.jsonl / eval.jsonl with {"anchor": ..., "positive": ...} per line
(produced by `go run ./cmd/embed-data-export`). Training uses
MultipleNegativesRankingLoss (in-batch negatives), optionally wrapped in
Matryoshka loss and fed mined hard negatives.

Run on a GPU / Ascend NPU machine (NOT inside the Go repo runtime):

    pip install -r requirements.txt   # + torch for your CUDA / Ascend
    python finetune.py --train train.jsonl --eval eval.jsonl \
        --base BAAI/bge-base-zh-v1.5 --out ./yunque-embed \
        --epochs 2 --batch 64 --hard-negatives 4

Key flags:
    --matryoshka-dims 768,512,256,128   # MRL: one model, truncatable vectors
                                        # (empty string disables MRL)
    --hard-negatives N                  # mine N hard negatives per pair (0=off)
    --eval eval.jsonl                   # enables real Recall@k / MRR auto-eval

Output: a SentenceTransformer model dir (servable via serve.py / TEI, or
further shrunk with distill.py). After training it prints a Recall@k/MRR table
at each Matryoshka dim so you can pick the end-side vs cloud-side dim.
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


def parse_dims(spec: str) -> list[int]:
    """'768,512,256,128' -> [768, 512, 256, 128]; '' -> []."""
    dims = []
    for part in spec.split(","):
        part = part.strip()
        if part:
            dims.append(int(part))
    return dims


def mine_negatives(train_ds, model, num_negatives: int, batch_size: int, use_faiss: bool):
    """Mine hard negatives into (anchor, positive, negative_1..N) n-tuples.

    Hard negatives are the most similar *wrong* documents; training against them
    teaches the model to separate look-alike memories (fixes the "unrelated floor
    too high" problem). Falls back to the original (anchor, positive) pairs if the
    installed sentence-transformers lacks/changes the mining API.
    """
    try:
        from sentence_transformers.util import mine_hard_negatives
    except Exception as exc:  # pragma: no cover - depends on ST version
        print(f"[hard-neg] mine_hard_negatives unavailable ({exc}); using in-batch negatives only")
        return train_ds

    # Clamp the candidate rank window to the corpus size so small datasets don't
    # raise "selected index k out of range"; large corpora keep the 10..60 window.
    n = len(train_ds)
    range_max = min(60, max(num_negatives + 1, n - 1))
    range_min = min(10, max(0, range_max - num_negatives - 1))
    try:
        mined = mine_hard_negatives(
            train_ds,
            model,
            anchor_column_name="anchor",
            positive_column_name="positive",
            num_negatives=num_negatives,
            range_min=range_min,  # skip the very top (likely near-duplicates of positive)
            range_max=range_max,  # clamped to corpus size
            max_score=0.95,       # drop candidates too similar to be true negatives
            sampling_strategy="top",
            batch_size=batch_size,
            use_faiss=use_faiss,
            output_format="n-tuple",
            verbose=True,
        )
        print(f"[hard-neg] mined dataset columns: {mined.column_names}")
        return mined
    except Exception as exc:  # pragma: no cover - signature drift across versions
        print(f"[hard-neg] mining failed ({exc}); falling back to in-batch negatives")
        return train_ds


def build_ir_evaluator(eval_ds, dim: int):
    """Build an InformationRetrievalEvaluator: anchor=query, positives=corpus.

    Each anchor must retrieve its own stored positive out of *all* positives.
    Evaluated at a truncated `dim` so Matryoshka quality/size tradeoff is visible.
    """
    from sentence_transformers.evaluation import InformationRetrievalEvaluator

    queries, corpus, relevant, pos2cid = {}, {}, {}, {}
    for i, row in enumerate(eval_ds):
        anchor, positive = row["anchor"], row["positive"]
        qid = f"q{i}"
        queries[qid] = anchor
        if positive not in pos2cid:
            pos2cid[positive] = f"d{len(pos2cid)}"
            corpus[pos2cid[positive]] = positive
        relevant[qid] = {pos2cid[positive]}

    return InformationRetrievalEvaluator(
        queries=queries,
        corpus=corpus,
        relevant_docs=relevant,
        truncate_dim=dim,
        name=f"yq-dim{dim}",
        show_progress_bar=False,
        accuracy_at_k=[1, 3, 5, 10],
        precision_recall_at_k=[1, 5, 10],
        mrr_at_k=[10],
        map_at_k=[10],
        write_csv=False,
    )


def _metric(results: dict, dim: int, needle: str):
    """Pull a metric out of the evaluator result dict, robust to key naming."""
    for key, val in results.items():
        if needle in key:
            return val
    return float("nan")


def run_auto_eval(model, eval_ds, dims: list[int]) -> None:
    print("\n=== auto-eval (Recall@k / MRR, held-out) ===")
    print(f"queries: {len(eval_ds)}")
    print(f"{'dim':>5} | {'R@1':>6} {'R@5':>6} {'R@10':>6} | {'MRR@10':>7}")
    print("-" * 44)
    for dim in dims:
        ev = build_ir_evaluator(eval_ds, dim)
        res = ev(model)
        r1 = _metric(res, dim, "recall@1")
        r5 = _metric(res, dim, "recall@5")
        r10 = _metric(res, dim, "recall@10")
        mrr = _metric(res, dim, "mrr@10")
        print(f"{dim:>5} | {r1:>6.3f} {r5:>6.3f} {r10:>6.3f} | {mrr:>7.3f}")
    print("-" * 44)
    print("(higher = better; small gap between dims => Matryoshka truncation is safe)")


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
    ap.add_argument("--matryoshka-dims", default="768,512,256,128",
                    help="comma dims for Matryoshka (empty disables MRL)")
    ap.add_argument("--hard-negatives", type=int, default=0,
                    help="mine N hard negatives per pair (0 = in-batch only)")
    ap.add_argument("--use-faiss", action="store_true",
                    help="use FAISS for faster hard-negative mining")
    args = ap.parse_args()

    model = SentenceTransformer(args.base)
    dims = parse_dims(args.matryoshka_dims)

    data_files = {"train": args.train}
    has_eval = False
    try:
        with open(args.eval):
            data_files["eval"] = args.eval
            has_eval = True
    except OSError:
        print(f"[eval] {args.eval} not found; auto-eval will be skipped")
    ds = load_dataset("json", data_files=data_files)

    # Start from clean (anchor, positive); mining adds negative columns.
    keep = ["anchor", "positive"]
    train_ds = ds["train"].remove_columns(
        [c for c in ds["train"].column_names if c not in keep]
    )

    if args.hard_negatives > 0:
        print(f"[hard-neg] mining {args.hard_negatives} hard negative(s) per pair ...")
        train_ds = mine_negatives(
            train_ds, model, args.hard_negatives, args.batch, args.use_faiss
        )

    # Base loss = in-batch (+ mined) negatives; optionally wrapped in Matryoshka.
    base_loss = losses.MultipleNegativesRankingLoss(model)
    if dims:
        print(f"[mrl] Matryoshka enabled at dims {dims}")
        loss = losses.MatryoshkaLoss(model, base_loss, matryoshka_dims=dims)
    else:
        loss = base_loss

    evaluator = None
    if has_eval:
        # During-training eval at the largest dim (cheap signal); full multi-dim
        # table is printed at the end.
        evaluator = build_ir_evaluator(ds["eval"], dims[0] if dims else model.get_sentence_embedding_dimension())

    targs = SentenceTransformerTrainingArguments(
        output_dir=args.out,
        num_train_epochs=args.epochs,
        per_device_train_batch_size=args.batch,
        learning_rate=args.lr,
        warmup_ratio=0.1,
        logging_steps=50,
        save_strategy="epoch",
        eval_strategy="epoch" if has_eval else "no",
        report_to=[],
        **pick_precision(),
    )

    trainer = SentenceTransformerTrainer(
        model=model,
        args=targs,
        train_dataset=train_ds,
        evaluator=evaluator,
        loss=loss,
    )
    trainer.train()

    model.save_pretrained(args.out)
    print(f"\nSaved fine-tuned embedding model to {args.out}")

    if has_eval:
        eval_dims = dims if dims else [model.get_sentence_embedding_dimension()]
        run_auto_eval(model, ds["eval"], eval_dims)

    print("\nServe it via serve.py / TEI, point Yunque EMBED_BASE_URL at it,")
    print("or shrink it with: python distill.py --model", args.out)


if __name__ == "__main__":
    main()
