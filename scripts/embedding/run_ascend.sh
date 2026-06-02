#!/usr/bin/env bash
# 在昇腾(Ascend)NPU 容器里跑嵌入微调。
#
# 用法：
#   # 1) 构建镜像（按你的 CANN 版本指定基础镜像）
#   docker build -f Dockerfile.ascend \
#     --build-arg BASE_IMAGE=<你的昇腾pytorch镜像> -t yunque-embed-ascend .
#
#   # 2) 准备数据：把 train.jsonl / eval.jsonl 放到当前目录（由 cmd/embed-data-export 生成）
#
#   # 3) 跑微调（默认用 0 号卡；多卡用 ASCEND_DEVICE=0,1,...）
#   ./run_ascend.sh --train train.jsonl --eval eval.jsonl --base BAAI/bge-base-zh-v1.5 --out ./yunque-embed
#
# 说明：本脚本用手动挂载设备方式（最通用）。若已装 Ascend Docker Runtime，
# 可改用  docker run --runtime=ascend -e ASCEND_VISIBLE_DEVICES=$ASCEND_DEVICE ...  省去 --device。
set -euo pipefail

IMAGE="${IMAGE:-yunque-embed-ascend}"
ASCEND_DEVICE="${ASCEND_DEVICE:-0}"

# 为每张卡拼出 --device /dev/davinciN
DEV_ARGS=()
IFS=',' read -ra CARDS <<< "$ASCEND_DEVICE"
for c in "${CARDS[@]}"; do
  DEV_ARGS+=(--device "/dev/davinci${c}")
done

docker run -it --rm \
  "${DEV_ARGS[@]}" \
  --device /dev/davinci_manager \
  --device /dev/devmm_svm \
  --device /dev/hisi_hdc \
  -v /usr/local/Ascend/driver:/usr/local/Ascend/driver \
  -v /usr/local/dcmi:/usr/local/dcmi \
  -v /usr/local/bin/npu-smi:/usr/local/bin/npu-smi \
  -v "$PWD":/workspace -w /workspace \
  "$IMAGE" \
  python finetune.py "$@"
