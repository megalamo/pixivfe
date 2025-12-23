# syntax=docker/dockerfile:1

# -- Builder stage
# Compiles the Go application to produce a binary for the release stage.
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.25.3-alpine3.22 AS builder

# ARGs provided by buildx for cross-compilation
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Install git for VCS information
RUN apk add --no-cache git~=2.49

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -v -trimpath -buildvcs=true -o pixivfe .

# Create minimal passwd and group entries for non-privileged user
RUN echo "pixivfe:x:10001:10001::/:/" >> /etc/passwd && \
    echo "pixivfe:x:10001:" >> /etc/group

# -- Release stage
# Creates a minimal image from scratch for the application.
FROM scratch AS release

# Copy the binary. The parent /app directory will be created as root.
COPY --from=builder /app/pixivfe /app/pixivfe
# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Copy user and group definitions
COPY --from=builder /etc/passwd /etc/group /etc/
# Copy the pre-made /app/data directory and set its ownership.
COPY --from=builder --chown=10001:10001 /app/data /app/data

WORKDIR /app

EXPOSE 8282

USER pixivfe

ENTRYPOINT ["/app/pixivfe"]
