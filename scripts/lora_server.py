#!/usr/bin/env python3
"""
LoRA Training Server for Yunque Agent.

A standalone HTTP service that accepts training jobs via REST API,
runs them asynchronously, and reports status. Matches the remote API
protocol defined in lora_trainer.go.

Usage:
    pip install fastapi uvicorn
    python lora_server.py [--host 0.0.0.0] [--port 8100] [--api-key sk-xxx]

API:
    POST /v1/train           → Submit training job
    GET  /v1/train/{job_id}  → Get job status
    GET  /v1/jobs            → List all jobs
    DELETE /v1/train/{job_id} → Cancel job
    GET  /health             → Health check
"""

import argparse
import json
import logging
import os
import subprocess
import sys
import threading
import time
import uuid
from dataclasses import dataclass, field, asdict
from enum import Enum
from pathlib import Path
from typing import Optional

try:
    from fastapi import FastAPI, HTTPException, Header, BackgroundTasks
    from fastapi.responses import JSONResponse
    from pydantic import BaseModel
    import uvicorn
except ImportError:
    print("Missing dependencies. Run: pip install fastapi uvicorn", file=sys.stderr)
    sys.exit(1)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
log = logging.getLogger("lora_server")


class JobStatus(str, Enum):
    QUEUED = "queued"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


@dataclass
class TrainJobRecord:
    job_id: str
    status: JobStatus = JobStatus.QUEUED
    base_model: str = ""
    data_path: str = ""
    adapter_name: str = ""
    output_dir: str = ""
    config: dict = field(default_factory=dict)

    adapter_path: str = ""
    final_loss: float = 0.0
    samples: int = 0
    epochs: int = 0
    duration_sec: float = 0.0
    error: str = ""

    created_at: float = field(default_factory=time.time)
    started_at: float = 0.0
    finished_at: float = 0.0


class TrainConfig(BaseModel):
    lora_rank: int = 16
    num_epochs: int = 3
    learning_rate: float = 2e-4
    max_seq_length: int = 2048
    seed: int = 42
    target_modules: Optional[list[str]] = None
    trust_remote_code: bool = False


class TrainRequest(BaseModel):
    base_model: str
    data_path: str
    adapter_name: str
    output_dir: str
    config: TrainConfig = TrainConfig()


class JobStore:
    """Thread-safe in-memory job store."""

    def __init__(self):
        self._jobs: dict[str, TrainJobRecord] = {}
        self._procs: dict[str, subprocess.Popen] = {}
        self._lock = threading.Lock()

    def create(self, req: TrainRequest) -> TrainJobRecord:
        job = TrainJobRecord(
            job_id=f"train-{uuid.uuid4().hex[:12]}",
            base_model=req.base_model,
            data_path=req.data_path,
            adapter_name=req.adapter_name,
            output_dir=req.output_dir,
            config=req.config.model_dump(),
        )
        with self._lock:
            self._jobs[job.job_id] = job
        return job

    def get(self, job_id: str) -> Optional[TrainJobRecord]:
        with self._lock:
            return self._jobs.get(job_id)

    def update(self, job_id: str, **kwargs):
        with self._lock:
            job = self._jobs.get(job_id)
            if job:
                for k, v in kwargs.items():
                    setattr(job, k, v)

    def set_proc(self, job_id: str, proc: subprocess.Popen):
        with self._lock:
            self._procs[job_id] = proc

    def kill_proc(self, job_id: str):
        with self._lock:
            proc = self._procs.pop(job_id, None)
        if proc and proc.poll() is None:
            proc.kill()

    def clear_proc(self, job_id: str):
        with self._lock:
            self._procs.pop(job_id, None)

    def list_all(self) -> list[dict]:
        with self._lock:
            return [asdict(j) for j in self._jobs.values()]


store = JobStore()
app = FastAPI(title="Yunque LoRA Training Server", version="1.0.0")

EXPECTED_API_KEY: Optional[str] = None
TRAIN_SCRIPT: str = ""
MAX_CONCURRENT: int = 2
_running_count = 0
_running_lock = threading.Lock()


def verify_auth(authorization: Optional[str] = Header(None)):
    if not EXPECTED_API_KEY:
        return
    if not authorization or not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="Missing or invalid Authorization header")
    token = authorization[len("Bearer "):]
    if token != EXPECTED_API_KEY:
        raise HTTPException(status_code=403, detail="Invalid API key")


@app.get("/health")
def health():
    return {"status": "ok", "running_jobs": _running_count}


@app.post("/v1/train", status_code=202)
def submit_train(req: TrainRequest, background_tasks: BackgroundTasks, authorization: Optional[str] = Header(None)):
    verify_auth(authorization)

    if not req.base_model or not req.data_path or not req.adapter_name:
        raise HTTPException(status_code=400, detail="Missing required fields")

    global _running_count
    with _running_lock:
        if _running_count >= MAX_CONCURRENT:
            raise HTTPException(status_code=429, detail=f"Max concurrent jobs ({MAX_CONCURRENT}) reached")

    job = store.create(req)
    log.info("Job submitted: %s (model=%s, adapter=%s)", job.job_id, req.base_model, req.adapter_name)

    background_tasks.add_task(run_training, job.job_id)
    return {"job_id": job.job_id}


@app.get("/v1/train/{job_id}")
def get_status(job_id: str, authorization: Optional[str] = Header(None)):
    verify_auth(authorization)

    job = store.get(job_id)
    if not job:
        raise HTTPException(status_code=404, detail=f"Job {job_id} not found")

    return {
        "status": job.status.value,
        "adapter_path": job.adapter_path,
        "final_loss": job.final_loss,
        "samples": job.samples,
        "epochs": job.epochs,
        "duration_sec": job.duration_sec,
        "error": job.error,
    }


@app.get("/v1/jobs")
def list_jobs(authorization: Optional[str] = Header(None)):
    verify_auth(authorization)
    return {"jobs": store.list_all()}


@app.delete("/v1/train/{job_id}")
def cancel_job(job_id: str, authorization: Optional[str] = Header(None)):
    verify_auth(authorization)

    job = store.get(job_id)
    if not job:
        raise HTTPException(status_code=404, detail=f"Job {job_id} not found")

    if job.status in (JobStatus.QUEUED, JobStatus.RUNNING):
        store.update(job_id, status=JobStatus.CANCELLED, error="cancelled by user")
        store.kill_proc(job_id)
        return {"status": "cancelled"}

    return {"status": job.status.value, "message": "job already finished"}


def run_training(job_id: str):
    global _running_count

    with _running_lock:
        _running_count += 1

    try:
        _run_training_inner(job_id)
    finally:
        with _running_lock:
            _running_count -= 1


def _run_training_inner(job_id: str):
    job = store.get(job_id)
    if not job or job.status == JobStatus.CANCELLED:
        return

    store.update(job_id, status=JobStatus.RUNNING, started_at=time.time())
    log.info("Training started: %s", job_id)

    train_args = {
        "base_model": job.base_model,
        "data_path": job.data_path,
        "output_dir": job.output_dir,
        "adapter_name": job.adapter_name,
        **job.config,
    }

    try:
        proc = subprocess.Popen(
            ["python3", TRAIN_SCRIPT, "--json-args", json.dumps(train_args)],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        store.set_proc(job_id, proc)
        try:
            stdout, stderr = proc.communicate(timeout=7200)
        except subprocess.TimeoutExpired:
            proc.kill()
            stdout, stderr = proc.communicate()
            store.clear_proc(job_id)
            store.update(
                job_id,
                status=JobStatus.FAILED,
                error="Training timed out after 2 hours",
                finished_at=time.time(),
                duration_sec=7200,
            )
            log.error("Training timed out: %s", job_id)
            return
        finally:
            store.clear_proc(job_id)

        result_code = proc.returncode

        if result_code != 0:
            error_msg = stderr.strip()[-500:] if stderr else "unknown error"
            store.update(
                job_id,
                status=JobStatus.FAILED,
                error=f"Training script failed (exit {result_code}): {error_msg}",
                finished_at=time.time(),
                duration_sec=time.time() - job.started_at,
            )
            log.error("Training failed: %s — %s", job_id, error_msg[:200])
            return

        try:
            output = json.loads(stdout)
        except json.JSONDecodeError:
            store.update(
                job_id,
                status=JobStatus.FAILED,
                error="Failed to parse training output",
                finished_at=time.time(),
                duration_sec=time.time() - job.started_at,
            )
            return

        if not output.get("success"):
            store.update(
                job_id,
                status=JobStatus.FAILED,
                error=output.get("error", "unknown"),
                finished_at=time.time(),
                duration_sec=time.time() - job.started_at,
            )
            return

        store.update(
            job_id,
            status=JobStatus.COMPLETED,
            adapter_path=output.get("adapter_path", ""),
            final_loss=output.get("final_loss", 0.0),
            samples=output.get("samples", 0),
            epochs=output.get("epochs", 0),
            duration_sec=output.get("duration_seconds", time.time() - job.started_at),
            finished_at=time.time(),
        )
        log.info(
            "Training complete: %s (loss=%.4f, samples=%d, duration=%.1fs)",
            job_id, output.get("final_loss", 0), output.get("samples", 0),
            output.get("duration_seconds", 0),
        )

    except Exception as e:
        store.update(
            job_id,
            status=JobStatus.FAILED,
            error=str(e),
            finished_at=time.time(),
            duration_sec=time.time() - (job.started_at or time.time()),
        )
        log.error("Training error: %s — %s", job_id, e)


def main():
    global EXPECTED_API_KEY, TRAIN_SCRIPT, MAX_CONCURRENT

    parser = argparse.ArgumentParser(description="Yunque LoRA Training Server")
    parser.add_argument("--host", default="0.0.0.0", help="Bind host")
    parser.add_argument("--port", type=int, default=8100, help="Bind port")
    parser.add_argument("--api-key", default=os.getenv("LORA_SERVER_API_KEY", ""), help="API key for authentication")
    parser.add_argument("--train-script", default="", help="Path to lora_train.py")
    parser.add_argument("--max-concurrent", type=int, default=2, help="Max concurrent training jobs")
    args = parser.parse_args()

    EXPECTED_API_KEY = args.api_key or None

    if args.train_script:
        TRAIN_SCRIPT = args.train_script
    else:
        TRAIN_SCRIPT = str(Path(__file__).parent / "lora_train.py")

    if not Path(TRAIN_SCRIPT).exists():
        log.error("Training script not found: %s", TRAIN_SCRIPT)
        sys.exit(1)

    MAX_CONCURRENT = args.max_concurrent

    log.info("Starting LoRA Training Server on %s:%d", args.host, args.port)
    log.info("  Train script: %s", TRAIN_SCRIPT)
    log.info("  Max concurrent: %d", MAX_CONCURRENT)
    log.info("  Auth: %s", "enabled" if EXPECTED_API_KEY else "disabled")

    uvicorn.run(app, host=args.host, port=args.port, log_level="info")


if __name__ == "__main__":
    main()
