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

# Must use TARGETPLATFORM so each multi-arch manifest slice gets a native base + CLI.
FROM --platform=$TARGETPLATFORM alpine:3.21 AS release

# Do NOT give TARGETARCH a default — Buildx sets it per platform; a default of amd64
# can bake the x86_64 proton-drive into the arm64 image (Rosetta ld-musl-x86_64 errors).
ARG TARGETARCH
ARG TARGETPLATFORM
# Pin + checksums from https://proton.me/download/drive/cli/index.html (musl builds)
ARG PROTON_DRIVE_VERSION=0.7.0
ARG PROTON_DRIVE_SHA512_AMD64=fb0e9bb12e18ff3f9c07b18be76e209b7aeedcae23f0e6953d8334ba6516fb6264575f5c1f42021673b7562f0b751d476d4b92c01bffb5e02167f7d1f35889cb
ARG PROTON_DRIVE_SHA512_ARM64=cd00d47e932fdb9f36e1d5db20d7016997ffa2f050a65df845563f4509c94f2c53d51a0f60fa6864a362ee6504d2730634abe84814b2aa86e6d2621cdac43680

# libstdc++ / libgcc are required by the Proton Drive CLI (C++ runtime).
RUN apk add --no-cache \
        ca-certificates \
        mongodb-tools \
        postgresql-client \
        tzdata \
        libsecret \
        gnome-keyring \
        dbus-x11 \
        curl \
        libstdc++ \
        libgcc \
        file \
    && echo "release stage TARGETPLATFORM=${TARGETPLATFORM} TARGETARCH=${TARGETARCH}" \
    && test -n "${TARGETARCH}" \
    && case "${TARGETARCH}" in \
         amd64|x86_64) ARCH=x64; SHA="${PROTON_DRIVE_SHA512_AMD64}"; FILE_RE='x86-64|x86_64|Intel 80386' ;; \
         arm64|aarch64) ARCH=arm64; SHA="${PROTON_DRIVE_SHA512_ARM64}"; FILE_RE='ARM aarch64|ARM64|aarch64' ;; \
         *) echo "unsupported arch: ${TARGETARCH}" >&2; exit 1 ;; \
       esac \
    && curl -fsSL "https://proton.me/download/drive/cli/${PROTON_DRIVE_VERSION}/linux-${ARCH}-musl/proton-drive" \
         -o /usr/local/bin/proton-drive \
    && echo "${SHA}  /usr/local/bin/proton-drive" | sha512sum -c - \
    && chmod +x /usr/local/bin/proton-drive \
    && file /usr/local/bin/proton-drive | tee /tmp/proton-drive.file \
    && grep -Eq "${FILE_RE}" /tmp/proton-drive.file \
    && ldd /usr/local/bin/proton-drive | tee /tmp/proton-drive.ldd \
    && ! grep -q 'not found' /tmp/proton-drive.ldd \
    && mkdir -p /data \
    && chown nobody:nobody /data

COPY --from=builder /go/bin/rclonarr /usr/local/bin/rclonarr
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENV HOME=/data \
    PATH="/usr/local/bin:${PATH}" \
    APP_PROTON_DRIVE_BIN=/usr/local/bin/proton-drive \
    APP_PROTON_DRIVE_DBUS=false \
    APP_REMOTE_PREFIX=/my-files/homelab-backups

USER nobody
WORKDIR /data

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
