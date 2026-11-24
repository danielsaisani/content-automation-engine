# ---- Stage 1: Builder ----
FROM golang:1.26.11-bookworm AS builder

WORKDIR /build

COPY incubator/go.mod incubator/go.sum ./

RUN go mod download

COPY incubator/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -o bin/content-automation-engine ./cmd/engine

# ---- Minimal runner: scraping only ----
# Build with `docker build --target minimal .` for a tiny image that can scrape stories into
# MongoDB but cannot generate or post videos (no ffmpeg, no browser). Keep PIPELINE_ENABLED off.
FROM scratch AS minimal

# Copy certificate authority from build step
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /build/bin/content-automation-engine /content-automation-engine

EXPOSE 8000

CMD ["/content-automation-engine"]

# ---- Full runner (default): the whole pipeline ----
# ffmpeg renders the videos, Chromium is driven by the TikTok poster to sign its requests,
# and the DejaVu fonts are what the captions are drawn with.
FROM debian:bookworm-slim AS full

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg chromium fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 engine

COPY --from=builder /build/bin/content-automation-engine /usr/local/bin/content-automation-engine

USER engine
WORKDIR /home/engine

EXPOSE 8000

CMD ["content-automation-engine"]
