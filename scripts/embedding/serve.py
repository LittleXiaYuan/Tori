#!/usr/bin/env python3
"""OpenAI 兼容的最小嵌入服务（仅依赖 sentence-transformers，零 Web 框架）。

启动:
    python serve.py --model ./yunque-embed-v1 --port 8080
    # 昇腾/CUDA:  --device npu  或  --device cuda

云雀接入 (.env):
    EMBED_BASE_URL=http://127.0.0.1:8080
    EMBED_MODEL=yunque-embed-v1
    EMBED_DIMS=768

接口:  POST /v1/embeddings   {"input": "文本" | ["文本", ...], "model": "..."}
       GET  /health
"""
import argparse
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from sentence_transformers import SentenceTransformer


def build_handler(model, model_name):
    class Handler(BaseHTTPRequestHandler):
        def _send(self, code, obj):
            body = json.dumps(obj, ensure_ascii=False).encode("utf-8")
            self.send_response(code)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            if self.path.rstrip("/") in ("", "/health"):
                self._send(200, {"status": "ok", "model": model_name})
            else:
                self._send(404, {"error": "not found"})

        def do_POST(self):
            if self.path.rstrip("/") not in ("/v1/embeddings", "/embeddings"):
                self._send(404, {"error": "not found"})
                return
            length = int(self.headers.get("Content-Length", "0"))
            try:
                req = json.loads(self.rfile.read(length) or b"{}")
            except Exception as exc:
                self._send(400, {"error": f"bad json: {exc}"})
                return
            inp = req.get("input", [])
            if isinstance(inp, str):
                inp = [inp]
            if not inp:
                self._send(400, {"error": "empty input"})
                return
            vecs = model.encode(inp, normalize_embeddings=True)
            data = [
                {"object": "embedding", "index": i, "embedding": v.tolist()}
                for i, v in enumerate(vecs)
            ]
            self._send(200, {
                "object": "list",
                "data": data,
                "model": req.get("model", model_name),
                "usage": {"prompt_tokens": 0, "total_tokens": 0},
            })

        def log_message(self, *_):
            pass

    return Handler


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", required=True, help="模型目录或名称")
    ap.add_argument("--host", default="127.0.0.1")
    ap.add_argument("--port", type=int, default=8080)
    ap.add_argument("--device", default="cpu", help="cpu | cuda | npu")
    ap.add_argument("--max-seq", type=int, default=512)
    args = ap.parse_args()

    print(f"loading model: {args.model} (device={args.device}) ...", flush=True)
    model = SentenceTransformer(args.model, device=args.device)
    model.max_seq_length = args.max_seq
    name = "yunque-embed-v1"
    server = ThreadingHTTPServer((args.host, args.port), build_handler(model, name))
    print(f"listening on http://{args.host}:{args.port}  (POST /v1/embeddings, dim={model.get_embedding_dimension()})", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        server.shutdown()


if __name__ == "__main__":
    main()
