# ╔══════════════════════════════════════════╗
# ║  Yunque Agent — Production Dockerfile    ║
# ╚══════════════════════════════════════════╝

# ── Stage 1: Build Go agent ──
# NOTE: Build context must be the PARENT directory containing both yunque-agent/
# and ledger/. Run: docker build -f yunque-agent/Dockerfile -t yunque-agent .
# from the parent directory, or use docker compose which handles this.
#
# SECURITY: for reproducible/audited builds, pin by digest instead of tag, e.g.
#   FROM golang:1.26.2-alpine@sha256:<digest>
# Refresh digests with `docker buildx imagetools inspect golang:1.26.2-alpine`.
FROM golang:1.26.2-alpine AS builder
RUN apk add --no-cache git ca-certificates nodejs npm
WORKDIR /src

# Copy ledger dependency first (referenced by go.mod replace directive)
COPY ledger/ /src/ledger/

# Copy dependency manifests first for better layer caching.
COPY yunque-agent/go.mod yunque-agent/go.sum /src/yunque-agent/
COPY yunque-agent/apps/web/package.json yunque-agent/apps/web/package-lock.json /src/yunque-agent/apps/web/
# The web app depends on the local TypeScript SDK via
# `file:../../packages/yunque-client`, so npm needs the package present before
# `npm ci`.
COPY yunque-agent/packages/yunque-client/ /src/yunque-agent/packages/yunque-client/
WORKDIR /src/yunque-agent
RUN go mod download
WORKDIR /src/yunque-agent/apps/web
RUN npm ci --prefer-offline --no-audit --no-fund

# Build the static WebUI before compiling Go so go:embed includes the real UI.
COPY yunque-agent/apps/web/ /src/yunque-agent/apps/web/
RUN npm run build

WORKDIR /src/yunque-agent
COPY yunque-agent/ /src/yunque-agent/
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X yunque-agent/internal/version.Version=${VERSION} -X yunque-agent/internal/version.GitCommit=${GIT_COMMIT} -X yunque-agent/internal/version.BuildDate=${BUILD_DATE}" \
    -o /agent ./cmd/agent

# ── Stage 2: Runtime (WebUI is embedded in binary, no Node.js needed) ──
# SECURITY: pin to a digest for production builds (see note above).
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
