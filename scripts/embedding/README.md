# 云雀嵌入模型训练（务实版）

把开源底座（默认 `bge-base-zh`）在云雀记忆数据上微调成「云雀特化嵌入」，可选蒸馏成
几十 MB 的静态模型。设计见 [`docs/spec/yunque-embedding-plan.md`](../../docs/spec/yunque-embedding-plan.md)。

> 这些脚本运行在 **GPU 机器**上（Python），不是云雀 Go 运行时的一部分。

## 端到端流程

```
① 导出数据 (Go, 仓库内, 无需 GPU)
   go run ./cmd/embed-data-export -daily data/memory/daily \
       -llm-base <OpenAI兼容地址>/v1 -llm-key <key> -llm-model <模型> \
       -out train.jsonl -eval eval.jsonl
   # 产出 (anchor=改写问句, positive=记忆事实) 训练对

② 微调 (GPU/NPU 机)
   pip install -r requirements.txt          # 另装匹配 CUDA/CANN 的 torch
   python finetune.py --train train.jsonl --eval eval.jsonl \
       --base BAAI/bge-base-zh-v1.5 --out ./yunque-embed \
       --epochs 2 --batch 64 --hard-negatives 4
   # --hard-negatives N        : 挖 N 个难负例(像但不对的记忆)→ 把无关项压下去
   # --matryoshka-dims a,b,c   : 一次训出可截断向量(默认 768,512,256,128;端侧截 256/云端 768)
   # 训练后自动用 eval.jsonl 做 held-out，打印各维度 Recall@k / MRR(取代假满分)

③ (可选) 蒸馏成静态小模型 (~几十MB)
   python distill.py --model ./yunque-embed --out ./yunque-embed-static --dims 256

④ 服务 + 接入云雀
   # 用 TEI / sentence-transformers / Ollama 起一个 OpenAI 兼容 /embeddings 服务，
   # 然后在云雀 .env：
   EMBED_BASE_URL=http://<embed服务地址>
   EMBED_MODEL=yunque-embed
   EMBED_DIMS=768            # bge-base=768；蒸馏版按 --dims
   # EMBED_API_KEY=...       # 若该服务需要鉴权

⑤ 评测 (Go, 仓库内)
   go run ./cmd/recall-eval -key <DEFAULT_API_KEY>
   # 对比：底座原版 vs 云雀微调版 vs 关键词基线 的命中率
```

## 硬件与昇腾(Ascend)说明

微调嵌入底座（bge-base ~110M、bge-m3 ~560M）算力需求很低，**昇腾 800I A2/A3
（910B/910C 级）绰绰有余**，可上大 batch、几分钟到几十分钟级。

- 昇腾不是 CUDA：装 **CANN + `torch_npu`**（不要用 CUDA 版 torch）。
  参考 https://gitee.com/ascend/pytorch ，`torch`/`torch_npu`/CANN 三者版本要对齐。
- `finetune.py` 会**自动检测 NPU**（`import torch_npu`）并用 **bf16**（910 支持好）；
  检测不到则回退 CUDA / CPU。
- 一般无需改代码；若遇到个别算子不支持，设 `ASCEND_RT_VISIBLE_DEVICES` 选卡，
  或把该步放 CPU。
- Model2Vec 蒸馏（`distill.py`）是轻量操作，CPU/单卡即可。

### 在容器里跑（昇腾）

提供了 `Dockerfile.ascend` + `run_ascend.sh`：

```bash
cd scripts/embedding

# 1) 构建镜像（基础镜像到 hiascend.com/developer/ascendhub 选与你 CANN/驱动匹配的 tag）
docker build -f Dockerfile.ascend --build-arg BASE_IMAGE=<昇腾pytorch镜像> -t yunque-embed-ascend .

# 2) 把 train.jsonl / eval.jsonl（cmd/embed-data-export 产出）放到当前目录

# 3) 跑微调（默认 0 号卡；多卡 ASCEND_DEVICE=0,1）
chmod +x run_ascend.sh
ASCEND_DEVICE=0 ./run_ascend.sh --train train.jsonl --eval eval.jsonl \
    --base BAAI/bge-base-zh-v1.5 --out ./yunque-embed
```

要点：
- 容器要能看到 NPU：脚本用 `--device /dev/davinci*` + 挂载 `driver`/`dcmi`/`npu-smi`（最通用）。
  若装了 **Ascend Docker Runtime**，可改用 `docker run --runtime=ascend -e ASCEND_VISIBLE_DEVICES=0`。
- 进容器后 `npu-smi info` 应能看到卡；看不到就是设备/驱动没挂进去。
- 产物 `./yunque-embed` 落在挂载的当前目录，宿主机可直接拿到。

## 数据来源

`cmd/embed-data-export` 当前从 `data/memory/daily/*.md` 抽取事实。后续可扩展：
ledger Recall 的命中/未命中日志、会话历史、`DataCollector` 采样 → 更真实的
(anchor, positive) 对（用户真实问句 ↔ 被召回的记忆）。

## 选型与体积（现实区间）

| 底座 | 维度 | 体积(约) | 场景 |
|---|---|---|---|
| bge-small-zh-v1.5 | 512 | ~130MB / int8 ~33MB | 端侧/桌面 |
| bge-base-zh-v1.5 | 768 | ~400MB | 均衡（推荐起步） |
| bge-m3 | 1024 | ~1–2GB | 服务端/多语 |
| 蒸馏静态(Model2Vec) | 可选(256/512) | ~几十MB | 离线/可内置 |

## 非目标

- 不从零训练（=重造 BGE）；不追极小到不可用；不对标/吊打 SOTA。
- 求稳：先用底座出基线 → 微调特化 → 视部署再瘦身。
