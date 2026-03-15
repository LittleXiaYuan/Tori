# ╔══════════════════════════════════════════╗
# ║  Yunque Agent — Production Dockerfile    ║
# ╚══════════════════════════════════════════╝

# ── Stage 1: Build Go agent ──
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X yunque-agent/internal/version.Version=${VERSION} -X yunque-agent/internal/version.GitCommit=${GIT_COMMIT} -X yunque-agent/internal/version.BuildDate=${BUILD_DATE}" \
    -o /agent ./cmd/agent

# ── Stage 2: Runtime (WebUI is embedded in binary, no Node.js needed) ──
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata curl python3
RUN adduser -D -u 1000 agent
WORKDIR /app

COPY --from=builder /agent .

# Pre-create data directories (including Phase F-I additions)
RUN mkdir -p data/memory/daily data/plugins data/persona data/sessions \
    data/knowledge data/cron data/i18n \
    data/iterate data/audit data/skills data/clawhub_cache \
    && chown -R agent:agent /app

USER agent
VOLUME ["/app/data"]

ENV AGENT_ADDR=:9090
ENV OPEN_BROWSER=false
EXPOSE 9090

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:9090/healthz || exit 1

ENTRYPOINT ["./agent"]
