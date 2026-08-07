# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:alpine AS builder
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags='-w -s -extldflags "-static"' -a \
    -o /go/bin/rclonarr ./cmd/main.go

FROM alpine:3.21 AS release

ARG TARGETARCH
ARG PROTON_DRIVE_VERSION=0.7.0

RUN apk add --no-cache ca-certificates mongodb-tools postgresql-client tzdata libsecret dbus-x11 curl \
    && case "$TARGETARCH" in \
         amd64) ARCH=x64 ;; \
         arm64) ARCH=arm64 ;; \
         *) echo "unsupported arch: $TARGETARCH" >&2; exit 1 ;; \
       esac \
    && curl -fsSL "https://proton.me/download/drive/cli/${PROTON_DRIVE_VERSION}/linux-${ARCH}-musl/proton-drive" \
         -o /usr/local/bin/proton-drive \
    && chmod +x /usr/local/bin/proton-drive \
    && mkdir -p /data \
    && chown nobody:nobody /data

COPY --from=builder /go/bin/rclonarr /usr/local/bin/rclonarr
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENV HOME=/data \
    APP_PROTON_DRIVE_BIN=/usr/local/bin/proton-drive \
    APP_PROTON_DRIVE_DBUS=false \
    APP_REMOTE_PREFIX=/my-files/homelab-backups

USER nobody
WORKDIR /data

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
