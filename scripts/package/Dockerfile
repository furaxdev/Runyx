# Minimal Docker packaging for Runyx. Build from the repo root:
#   docker build -f scripts/package/Dockerfile -t runyx .
#
# Not built or tested in this session's sandbox (no Docker daemon
# available) — see docs/BLOCKED.md and docs/MANUAL_TESTS.md.
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/runyx ./cmd/runyx

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates tesseract-ocr tesseract-ocr-fra ffmpeg git \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/runyx /usr/local/bin/runyx
EXPOSE 11535
ENTRYPOINT ["runyx", "serve"]
