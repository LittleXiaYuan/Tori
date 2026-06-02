#!/usr/bin/env python3
"""Distill a (fine-tuned) embedding model into a tiny static Model2Vec model.

Model2Vec turns a transformer encoder into a static token->vector table, so
inference becomes tokenize + lookup + mean-pool: ~tens of MB, CPU microseconds,
no neural forward pass at runtime. Slight quality drop vs the full encoder, but
ideal for an offline, low-footprint, embeddable default.

Run on a machine with the deps (GPU optional for distillation):

    pip install model2vec sentence-transformers
    python distill.py --model ./yunque-embed --out ./yunque-embed-static --dims 256

Output: a Model2Vec model dir. It can be served via the model2vec runtime, or
its vectors/tokenizer exported for a pure-Go loader (Yunque roadmap).
"""
import argparse


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", default="BAAI/bge-base-zh-v1.5",
                    help="source model (your fine-tuned dir or an HF base)")
    ap.add_argument("--out", default="./yunque-embed-static")
    ap.add_argument("--dims", type=int, default=256,
                    help="PCA output dims (Matryoshka-style; 256/512 typical)")
    args = ap.parse_args()

    from model2vec.distill import distill

    m = distill(model_name=args.model, pca_dims=args.dims)
    m.save_pretrained(args.out)
    print(f"Saved static (Model2Vec) embedding to {args.out} (dims={args.dims})")
    print("Sanity check:")
    print("    from model2vec import StaticModel")
    print(f"    v = StaticModel.from_pretrained('{args.out}').encode(['你好'])")


if __name__ == "__main__":
    main()
