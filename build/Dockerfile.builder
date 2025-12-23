# syntax=docker/dockerfile:1

# -- Dockerfile.build
# Provides dependencies for CI jobs.
FROM docker.io/library/golang:1.25.3-alpine3.22

ENV ORAS_VERSION=1.2.3

RUN apk add --no-cache \
  git~=2.49 \
  cosign~=2.4 \
  syft~=1.31.0-r2 \
  jq~=1.8 \
  glab~=1.58 \
  curl~=8.14 \
  bash~=5.2 \
  xz~=5.8 \
  zip~=3.0

# Install ORAS from GitHub releases
RUN curl -sL "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_amd64.tar.gz" | tar -xz -C /usr/local/bin/ oras

SHELL ["/bin/bash", "-c"]
