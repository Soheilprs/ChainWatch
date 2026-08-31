FROM golang:1.27.0-alpine3.23@sha256:3747dcba41c8b0db3211fda4db61638b980e17ac5bb3c94460a975a9cfe19395 AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
ARG GOPROXY=https://proxy.golang.org,direct
RUN GOPROXY=${GOPROXY} go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" \
    -o /out/chainwatch ./cmd/chainwatch

FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce

RUN apk add --no-cache ca-certificates \
    && addgroup -S chainwatch \
    && adduser -S -D -H -u 10001 -G chainwatch chainwatch

COPY --from=builder /out/chainwatch /usr/local/bin/chainwatch

USER chainwatch

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/chainwatch"]
