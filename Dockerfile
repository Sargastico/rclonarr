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

RUN apk add --no-cache ca-certificates mongodb-tools tzdata

COPY --from=builder /go/bin/rclonarr /usr/local/bin/rclonarr

USER nobody

ENTRYPOINT ["/usr/local/bin/rclonarr"]
