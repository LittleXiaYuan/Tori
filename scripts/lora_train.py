#!/usr/bin/env python3
"""
Production-grade LoRA fine-tuning script for Yunque Agent self-evolution.

Called by the Go LoRATrainer via subprocess:
    python3 lora_train.py --json-args '{"base_model":"...","data_path":"...","output_dir":"...",...}'

Outputs a JSON result to stdout. Logs go to stderr.

Requirements:
    pip install torch transformers peft datasets accelerate bitsandbytes
"""

import argparse
import datetime
import json
import logging
import os
import sys
import time
from pathlib import Path
from typing import Any

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    stream=sys.stderr,
)
log = logging.getLogger("lora_train")

REQUIRED_FIELDS = ["base_model", "data_path", "output_dir", "adapter_name"]

DEFAULT_TARGET_MODULES_MAP = {
    "llama": ["q_proj", "v_proj", "k_proj", "o_proj"],
    "qwen": ["q_proj", "v_proj", "k_proj", "o_proj"],
    "mistral": ["q_proj", "v_proj", "k_proj", "o_proj"],
    "chatglm": ["query_key_value"],
    "bloom": ["query_key_value"],
    "gpt_neox": ["query_key_value"],
    "phi": ["q_proj", "v_proj", "k_proj", "dense"],
}

RESPONSE_SEPARATOR = "### Response:\n"


def parse_args():
    parser = argparse.ArgumentParser(description="LoRA fine-tuning for Yunque Agent")
    parser.add_argument("--json-args", required=True, help="JSON training arguments from Go caller")
    return parser.parse_args()


def validate_args(args: dict) -> list[str]:
    """Validate training arguments. Returns list of error messages (empty if valid)."""
    errors = []

    missing = [k for k in REQUIRED_FIELDS if k not in args or not args[k]]
    if missing:
        errors.append(f"Missing required fields: {missing}")

    if "data_path" in args and args["data_path"]:
        if not Path(args["data_path"]).exists():
            errors.append(f"data_path does not exist: {args['data_path']}")

    if "output_dir" in args and args["output_dir"]:
        output = Path(args["output_dir"]).resolve()
        adapter = args.get("adapter_name", "")
        if adapter:
            final_path = (output / adapter).resolve()
            if not str(final_path).startswith(str(output)):
                errors.append(f"adapter_name causes path traversal: {adapter}")

    num_epochs = args.get("num_epochs", 3)
    if not isinstance(num_epochs, int) or num_epochs < 1:
        errors.append(f"num_epochs must be a positive integer, got: {num_epochs}")

    lora_rank = args.get("lora_rank", 16)
    if not isinstance(lora_rank, int) or lora_rank < 1:
        errors.append(f"lora_rank must be a positive integer, got: {lora_rank}")

    lr = args.get("learning_rate", 2e-4)
    if not isinstance(lr, (int, float)) or lr <= 0:
        errors.append(f"learning_rate must be positive, got: {lr}")

    max_seq = args.get("max_seq_length", 2048)
    if not isinstance(max_seq, int) or max_seq < 64:
        errors.append(f"max_seq_length must be >= 64, got: {max_seq}")

    return errors


def load_training_data(data_path: str) -> tuple[list[dict], dict]:
    """
    Load JSONL training data. Returns (samples, stats).
    Stats include loaded, skipped_json, skipped_schema counts.
    """
    stats = {"total_lines": 0, "loaded": 0, "skipped_empty": 0, "skipped_json": 0, "skipped_schema": 0}
    samples = []

    with open(data_path, "r", encoding="utf-8") as f:
        for line in f:
            stats["total_lines"] += 1
            line = line.strip()
            if not line:
                stats["skipped_empty"] += 1
                continue

            try:
                record = json.loads(line)
            except json.JSONDecodeError:
                stats["skipped_json"] += 1
                continue

            if "instruction" in record and "output" in record:
                samples.append({
                    "instruction": record["instruction"],
                    "input": record.get("input", ""),
                    "output": record["output"],
                })
                stats["loaded"] += 1
            elif "trajectory" in record:
                for step in record.get("trajectory", []):
                    if step.get("step_type") == "decide" and step.get("decision"):
                        samples.append({
                            "instruction": "You are an agentic decision maker. Given the context, decide the best action.",
                            "input": step.get("content", ""),
                            "output": json.dumps({
                                "decision": step["decision"],
                                "reason": step.get("reason", ""),
                                "confidence": step.get("confidence", 0.5),
                            }, ensure_ascii=False),
                        })
                        stats["loaded"] += 1
            else:
                stats["skipped_schema"] += 1

    return samples, stats


def format_prompt(sample: dict) -> str:
    instruction = sample["instruction"]
    inp = sample.get("input", "")
    output = sample["output"]

    if inp:
        return f"### Instruction:\n{instruction}\n\n### Input:\n{inp}\n\n{RESPONSE_SEPARATOR}{output}"
    return f"### Instruction:\n{instruction}\n\n{RESPONSE_SEPARATOR}{output}"


def build_labels_with_response_mask(input_ids, texts: list[str], tokenizer) -> list:
    """
    Build labels that mask everything before '### Response:\\n' with -100,
    so loss is only computed on the model's response portion.
    """
    import torch

    labels = input_ids.clone()
    sep_tokens = tokenizer.encode(RESPONSE_SEPARATOR, add_special_tokens=False)
    sep_len = len(sep_tokens)

    for i in range(labels.size(0)):
        seq = input_ids[i].tolist()
        mask_end = 0
        for j in range(len(seq) - sep_len + 1):
            if seq[j:j + sep_len] == sep_tokens:
                mask_end = j + sep_len
                break

        if mask_end > 0:
            labels[i, :mask_end] = -100

        pad_token_id = tokenizer.pad_token_id
        if pad_token_id is not None:
            labels[i][input_ids[i] == pad_token_id] = -100

    return labels


def infer_target_modules(model_name: str, explicit_modules: list[str] | None = None) -> list[str]:
    """Infer LoRA target modules from model name, or use explicit list if provided."""
    if explicit_modules:
        return explicit_modules

    name_lower = model_name.lower()
    for key, modules in DEFAULT_TARGET_MODULES_MAP.items():
        if key in name_lower:
            log.info("Auto-detected target_modules for '%s': %s", key, modules)
            return modules

    default = ["q_proj", "v_proj"]
    log.warning("Could not infer target_modules for '%s', using default: %s", model_name, default)
    return default


def verify_target_modules(model, target_modules: list[str]) -> list[str]:
    """Verify that target modules exist in the model. Returns list of missing modules."""
    all_names = {name.split(".")[-1] for name, _ in model.named_modules()}
    missing = [m for m in target_modules if m not in all_names]
    return missing


def save_training_metadata(
    output_path: str,
    args: dict,
    data_stats: dict,
    train_result: dict,
    trainable_params: int,
    total_params: int,
    duration_sec: float,
):
    """Write training_metadata.json alongside the adapter for auditability."""
    _resume_from = args.get("resume_from", "")
    metadata = {
        "base_model": args["base_model"],
        "adapter_name": args["adapter_name"],
        "lora_rank": args.get("lora_rank", 16),
        "lora_alpha": args.get("lora_rank", 16) * 2,
        "learning_rate": args.get("learning_rate", 2e-4),
        "num_epochs": args.get("num_epochs", 3),
        "max_seq_length": args.get("max_seq_length", 2048),
        "seed": args.get("seed", 42),
        "target_modules": args.get("_resolved_target_modules", []),
        "trust_remote_code": args.get("trust_remote_code", False),
        "trainable_params": trainable_params,
        "total_params": total_params,
        "trainable_ratio": f"{100 * trainable_params / max(total_params, 1):.4f}%",
        "data_stats": data_stats,
        "final_loss": train_result.get("final_loss"),
        "duration_seconds": round(duration_sec, 2),
        "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "data_path": args["data_path"],
        "output_dir": args["output_dir"],
        "resume_from": _resume_from or None,
        "incremental": bool(_resume_from),
    }

    meta_path = os.path.join(output_path, "training_metadata.json")
    with open(meta_path, "w", encoding="utf-8") as f:
        json.dump(metadata, f, indent=2, ensure_ascii=False)
    log.info("Metadata saved to %s", meta_path)


def train(args: dict) -> dict:
    validation_errors = validate_args(args)
    if validation_errors:
        return {"success": False, "error": f"Validation failed: {'; '.join(validation_errors)}"}

    try:
        import torch
        from transformers import (
            AutoModelForCausalLM,
            AutoTokenizer,
            TrainingArguments,
            Trainer,
            DataCollatorForLanguageModeling,
        )
        from peft import LoraConfig, get_peft_model, TaskType
        from datasets import Dataset
    except ImportError as e:
        return {
            "success": False,
            "error": f"Missing dependency: {e}. Run: pip install torch transformers peft datasets accelerate",
        }

    base_model = args["base_model"]
    data_path = args["data_path"]
    output_dir = args["output_dir"]
    adapter_name = args["adapter_name"]
    num_epochs = args.get("num_epochs", 3)
    lora_rank = args.get("lora_rank", 16)
    learning_rate = args.get("learning_rate", 2e-4)
    max_seq_length = args.get("max_seq_length", 2048)
    seed = args.get("seed", 42)
    trust_remote_code = args.get("trust_remote_code", False)
    explicit_modules = args.get("target_modules")
    resume_from = args.get("resume_from", "")

    log.info("Loading training data from %s", data_path)
    samples, data_stats = load_training_data(data_path)
    if not samples:
        return {
            "success": False,
            "error": "No valid training samples found",
            "data_stats": data_stats,
        }
    log.info(
        "Data loaded: %d samples, %d skipped (json=%d, schema=%d, empty=%d)",
        data_stats["loaded"], data_stats["skipped_json"] + data_stats["skipped_schema"] + data_stats["skipped_empty"],
        data_stats["skipped_json"], data_stats["skipped_schema"], data_stats["skipped_empty"],
    )

    log.info("Loading tokenizer for %s (trust_remote_code=%s)", base_model, trust_remote_code)
    tokenizer = AutoTokenizer.from_pretrained(base_model, trust_remote_code=trust_remote_code)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    log.info("Tokenizing %d samples (max_seq_length=%d, dynamic padding)", len(samples), max_seq_length)
    texts = [format_prompt(s) for s in samples]
    encodings = tokenizer(
        texts,
        truncation=True,
        max_length=max_seq_length,
        padding=True,
        return_tensors="pt",
    )

    labels = build_labels_with_response_mask(encodings["input_ids"], texts, tokenizer)

    dataset = Dataset.from_dict({
        "input_ids": encodings["input_ids"],
        "attention_mask": encodings["attention_mask"],
        "labels": labels,
    })

    log.info("Loading base model %s", base_model)
    model = AutoModelForCausalLM.from_pretrained(
        base_model,
        torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32,
        device_map="auto" if torch.cuda.is_available() else None,
        trust_remote_code=trust_remote_code,
    )

    # Incremental training: resume from previous adapter if available
    if resume_from and Path(resume_from).exists():
        from peft import PeftModel
        log.info("Incremental training: loading previous adapter from %s", resume_from)
        try:
            model = PeftModel.from_pretrained(model, resume_from)
            model = model.merge_and_unload()
            log.info("Previous adapter merged into base model for continued training")
        except Exception as e:
            log.warning("Failed to load previous adapter, training from scratch: %s", e)
    elif resume_from:
        log.warning("resume_from path does not exist: %s, training from scratch", resume_from)

    target_modules = infer_target_modules(base_model, explicit_modules)
    missing = verify_target_modules(model, target_modules)
    if missing:
        return {
            "success": False,
            "error": f"target_modules not found in model: {missing}. "
                     f"Available leaf modules: {sorted({n.split('.')[-1] for n, _ in model.named_modules() if n})}",
        }

    args["_resolved_target_modules"] = target_modules

    lora_config = LoraConfig(
        task_type=TaskType.CAUSAL_LM,
        r=lora_rank,
        lora_alpha=lora_rank * 2,
        lora_dropout=0.05,
        target_modules=target_modules,
        bias="none",
    )
    model = get_peft_model(model, lora_config)
    trainable_params = sum(p.numel() for p in model.parameters() if p.requires_grad)
    total_params = sum(p.numel() for p in model.parameters())
    log.info(
        "LoRA applied: trainable %d / %d (%.4f%%)",
        trainable_params, total_params, 100 * trainable_params / total_params,
    )

    adapter_output = os.path.join(output_dir, adapter_name)
    final_path = Path(adapter_output).resolve()
    allowed_root = Path(output_dir).resolve()
    if not str(final_path).startswith(str(allowed_root)):
        return {"success": False, "error": f"Path traversal detected: {adapter_name}"}
    os.makedirs(adapter_output, exist_ok=True)

    training_args = TrainingArguments(
        output_dir=adapter_output,
        num_train_epochs=num_epochs,
        per_device_train_batch_size=4 if torch.cuda.is_available() else 1,
        gradient_accumulation_steps=4,
        learning_rate=learning_rate,
        fp16=torch.cuda.is_available(),
        logging_steps=10,
        save_strategy="epoch",
        save_total_limit=2,
        warmup_ratio=0.1,
        weight_decay=0.01,
        report_to="none",
        remove_unused_columns=False,
        seed=seed,
        data_seed=seed,
    )

    data_collator = DataCollatorForLanguageModeling(tokenizer=tokenizer, mlm=False)

    trainer = Trainer(
        model=model,
        args=training_args,
        train_dataset=dataset,
        data_collator=data_collator,
    )

    log.info(
        "Starting training: epochs=%d, rank=%d, lr=%s, seed=%d, target_modules=%s",
        num_epochs, lora_rank, learning_rate, seed, target_modules,
    )
    start = time.time()
    train_result = trainer.train()
    duration = time.time() - start

    final_loss = train_result.training_loss
    log.info("Training complete: loss=%.4f, duration=%.1fs", final_loss, duration)

    model.save_pretrained(adapter_output)
    tokenizer.save_pretrained(adapter_output)
    log.info("Adapter saved to %s", adapter_output)

    result = {
        "adapter_path": adapter_output,
        "final_loss": final_loss,
        "samples": len(samples),
        "epochs": num_epochs,
        "success": True,
        "base_model": base_model,
        "adapter_name": adapter_name,
        "lora_rank": lora_rank,
        "learning_rate": learning_rate,
        "max_seq_length": max_seq_length,
        "duration_seconds": round(duration, 2),
        "trainable_params": trainable_params,
        "total_params": total_params,
        "seed": seed,
        "target_modules": target_modules,
        "data_stats": data_stats,
    }

    save_training_metadata(adapter_output, args, data_stats, result, trainable_params, total_params, duration)

    return result


def main():
    cli_args = parse_args()
    try:
        args = json.loads(cli_args.json_args)
    except json.JSONDecodeError as e:
        result = {"success": False, "error": f"Invalid JSON args: {e}"}
        print(json.dumps(result))
        sys.exit(1)

    log.info("Training args: %s", json.dumps(args, indent=2))
    result = train(args)
    print(json.dumps(result))

    if not result.get("success"):
        sys.exit(1)


if __name__ == "__main__":
    main()
