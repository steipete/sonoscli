# syntax=docker/dockerfile:1.27.0

ARG GO_VERSION=1.26.8

FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sonos ./cmd/sonos

FROM python:3.14-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl ffmpeg tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && python -m pip install --no-cache-dir --upgrade pip \
    && python -m pip install --no-cache-dir yt-dlp \
    && useradd --create-home --home-dir /data --uid 10001 sonos
ENV HOME=/data \
    XDG_CONFIG_HOME=/data/config \
    XDG_CACHE_HOME=/data/cache \
    XDG_STATE_HOME=/data/state
VOLUME ["/data"]
WORKDIR /data
COPY --from=build /out/sonos /usr/local/bin/sonos
USER sonos
ENTRYPOINT ["sonos"]
CMD ["--help"]
