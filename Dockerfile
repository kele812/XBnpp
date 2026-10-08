# syntax=docker/dockerfile:1.7

# Build stage
FROM --platform=$BUILDPLATFORM golang:1.26.8-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=1.0.2
ARG BUILD_TIME=unknown

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -p 2 -ldflags "-s -w \
    -X main.version=$VERSION \
    -X main.buildTime=$BUILD_TIME" \
    -tags "with_quic with_utls with_wireguard with_clash_api" \
    -o XBnpp ./cmd/XBnpp

# Runtime stage — sing-box & xray-core are embedded as Go libraries
FROM alpine:3.20
LABEL org.opencontainers.image.title="XBnpp" \
      org.opencontainers.image.source="https://github.com/kele812/XBnpp" \
      org.opencontainers.image.licenses="MPL-2.0"

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /build/XBnpp /usr/local/bin/XBnpp

RUN mkdir -p /etc/XBnpp

WORKDIR /etc/XBnpp

# Config can be provided via file mount OR environment variables.
# Env var mode (no config file needed):
#   docker run -d --network=host \
#     -e apiHost=https://panel.example.com \
#     -e apiKey=YOUR_TOKEN \
#     -e nodeID=1 \
#     ghcr.io/kele812/xbnpp:latest
#
# Supported env vars:
#   apiHost  / API_HOST    → panel URL
#   apiKey   / API_KEY     → server token
#   nodeID   / NODE_ID     → node ID
#   nodeType / NODE_TYPE   → node type (optional)
#   kernel   / KERNEL_TYPE -> auto (default), singbox or xray
#   domain   / DOMAIN      → TLS domain (enables auto_tls)
#   certFile / CERT_FILE   → TLS cert path
#   keyFile  / KEY_FILE    → TLS key path
#   logLevel / LOG_LEVEL   → log level

ENTRYPOINT ["XBnpp"]
CMD ["-c", "/etc/XBnpp/config.yml"]
