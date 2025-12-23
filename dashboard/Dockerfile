# syntax=docker/dockerfile:1
# =============================================================================
# Lynq Dashboard - Production Dockerfile
# Multi-arch support (amd64/arm64), optimized builds, security hardened
# =============================================================================

# Build arguments
ARG NODE_VERSION=20
ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.20

# -----------------------------------------------------------------------------
# Stage 1: Build UI (React/Vite)
# -----------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS ui-builder

WORKDIR /app/ui

# Install dependencies with cache mount
COPY ui/package.json ui/package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

# Copy source and build
COPY ui/ ./
RUN npm run build

# -----------------------------------------------------------------------------
# Stage 2: Build BFF (Go)
# -----------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS bff-builder

# Build arguments for cross-compilation
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /app/bff

# Install build dependencies
RUN apk add --no-cache ca-certificates tzdata

# Download dependencies with cache mount
COPY bff/go.mod bff/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# Copy source
COPY bff/ ./

# Build with optimizations for target platform
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -trimpath \
    -o /app/bff-server \
    ./cmd/server

# -----------------------------------------------------------------------------
# Stage 3: Runtime (Minimal & Secure)
# -----------------------------------------------------------------------------
FROM alpine:${ALPINE_VERSION}

# Labels for container metadata
LABEL org.opencontainers.image.title="Lynq Dashboard" \
      org.opencontainers.image.description="Lynq Operator Dashboard - UI and BFF" \
      org.opencontainers.image.vendor="k8s-lynq" \
      org.opencontainers.image.source="https://github.com/k8s-lynq/lynq"

# Install minimal runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    tini \
    && rm -rf /var/cache/apk/* /tmp/*

# Create non-root user
RUN addgroup -g 65532 -S lynq && \
    adduser -u 65532 -S -G lynq -s /sbin/nologin lynq

WORKDIR /app

# Copy artifacts with explicit ownership
COPY --from=bff-builder --chown=lynq:lynq /app/bff-server ./bff-server
COPY --from=ui-builder --chown=lynq:lynq /app/ui/dist ./public

# Set proper permissions (world-readable/executable for flexibility with --user flag)
# - Binary: executable by all (needed when running with different UID)
# - Directories: readable + executable (needed for traversal)
# - Files: readable by all
RUN chmod 555 /app/bff-server && \
    find /app/public -type d -exec chmod 555 {} \; && \
    find /app/public -type f -exec chmod 444 {} \;

# Switch to non-root user
USER 65532:65532

# Environment
ENV STATIC_DIR=/app/public

# Expose port
EXPOSE 8080

# Health check using built-in wget
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/healthz || exit 1

# Use tini for proper signal handling (PID 1)
ENTRYPOINT ["/sbin/tini", "--"]

# Run server in cluster mode
CMD ["/app/bff-server", "-mode", "cluster", "-addr", ":8080"]
