# ---- Build stage ----
FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/music-tagger ./cmd/server

# ---- Runtime stage ----
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        libchromaprint-tools \
        ffmpeg \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/music-tagger /usr/local/bin/music-tagger

ENV MUSIC_DIR=/music
ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/music-tagger"]
