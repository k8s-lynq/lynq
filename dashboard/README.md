# Lynq Dashboard

React-based dashboard for visualizing and monitoring the Lynq Operator's Hub/Form/Node pipeline.

## Overview

The Lynq Dashboard provides:
- **Overview**: KPI cards showing Hub/Form/Node counts and status
- **Topology View**: Visual representation of Hub → Form → Node relationships (coming soon)
- **Resource Pages**: Detailed views for Hubs, Forms, and Nodes

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│   React UI      │────▶│   Go BFF        │────▶│   Kubernetes    │
│   (Vite)        │     │   (chi router)  │     │   API Server    │
│                 │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
      :5173                   :8080
```

### Components

- **UI** (`/ui`): React 18 + TypeScript + Vite + shadcn/ui
- **BFF** (`/bff`): Go backend-for-frontend with Kubernetes client

## Prerequisites

- Node.js 20+ (LTS)
- Go 1.25+
- Access to a Kubernetes cluster with Lynq CRDs installed
- `kubectl` configured with appropriate context

## Quick Start

### Local Development

1. **Install dependencies:**
   ```bash
   make install
   ```

2. **Start the BFF server (in terminal 1):**
   ```bash
   make dev-bff
   ```

3. **Start the UI dev server (in terminal 2):**
   ```bash
   make dev-ui
   ```

4. **Open the dashboard:**
   Navigate to http://localhost:5173

### Docker Compose

```bash
make docker-up
```

## Configuration

### BFF Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `:8080` | HTTP server address |
| `-mode` | `local` | Application mode: `local` or `cluster` |
| `-kubeconfig` | (see below) | Path to kubeconfig (local mode only) |
| `-context` | (current) | Kubernetes context to use (local mode only) |

**Kubeconfig resolution order:**
1. `-kubeconfig` flag
2. `KUBECONFIG` environment variable
3. `~/.kube/config` (default)

### Environment Variables

| Variable | Description |
|----------|-------------|
| `KUBECONFIG` | Path to kubeconfig file (local mode) |
| `STATIC_DIR` | Static files directory (default: `./public`) |

## API Endpoints

### Core Resources

```
GET  /api/v1/hubs              # List all hubs
GET  /api/v1/hubs/{name}       # Get hub detail
GET  /api/v1/hubs/{name}/nodes # Get nodes for hub

GET  /api/v1/forms             # List all forms
GET  /api/v1/forms/{name}      # Get form detail

GET  /api/v1/nodes             # List all nodes
GET  /api/v1/nodes/{name}      # Get node detail
```

### Topology

```
GET  /api/v1/topology          # Get topology graph data
```

### Context (Local Mode)

```
GET  /api/v1/contexts          # List kubeconfig contexts
POST /api/v1/contexts/switch   # Switch active context
```

### Health

```
GET  /healthz                  # Health check
GET  /readyz                   # Readiness check
```

## Project Structure

```
dashboard/
├── ui/                        # React Frontend
│   ├── src/
│   │   ├── components/        # shadcn/ui components
│   │   ├── contexts/          # React contexts (Hub, Form, Node)
│   │   ├── hooks/             # Custom hooks (usePolling)
│   │   ├── pages/             # Page components
│   │   ├── lib/               # API client, utilities
│   │   └── types/             # TypeScript types
│   └── package.json
│
├── bff/                       # Go BFF Server
│   ├── cmd/server/            # Entry point
│   └── internal/
│       ├── api/               # HTTP handlers
│       └── kube/              # Kubernetes client
│
├── Makefile                   # Development commands
└── docker-compose.yaml        # Local development
```

## Technology Stack

### Frontend
- React 18 + TypeScript
- Vite (bundler)
- shadcn/ui (Radix + Tailwind)
- TanStack Query (data fetching)
- React Router (routing)
- Lucide React (icons)

### Backend
- Go 1.25
- chi (HTTP router)
- client-go (Kubernetes client)

## Development

### Available Commands

```bash
make help              # Show all commands
make dev-ui            # Run UI dev server
make dev-bff           # Run BFF dev server
make build             # Build both UI and BFF
make lint              # Lint all code
make test              # Run all tests
make clean             # Clean build artifacts
make docker-build      # Build production Docker image
make docker-build-multi # Build multi-arch image (amd64/arm64)
make docker-push       # Push image to registry
```

### Local Development Setup

**Terminal 1 - BFF Server:**
```bash
# Install Go dependencies
make install-bff

# Run BFF with default kubeconfig (~/.kube/config)
make dev-bff

# Or specify a context
cd bff && go run ./cmd/server -mode local -context my-cluster
```

**Terminal 2 - UI Dev Server:**
```bash
# Install Node dependencies
make install-ui

# Run Vite dev server (http://localhost:5173)
make dev-ui
```

The UI dev server proxies `/api` requests to the BFF at `localhost:8080`.

## Docker Production Build

### Build Image

```bash
# Build for current architecture
make docker-build

# Build multi-arch image (amd64/arm64) and push
make docker-build-multi

# Custom tag
IMAGE_TAG=v1.0.0 make docker-build
```

### Run in Cluster Mode

When deployed in Kubernetes, the container uses in-cluster authentication:

```bash
docker run --rm -p 8080:8080 ghcr.io/k8s-lynq/lynq-dashboard:latest
```

### Run in Local Mode

For local testing with kubeconfig:

```bash
# Basic (uses current user permissions)
docker run --rm -p 8080:8080 \
    --user $(id -u):$(id -g) \
    -v ~/.kube/config:/app/.kube/config:ro \
    -e KUBECONFIG=/app/.kube/config \
    ghcr.io/k8s-lynq/lynq-dashboard:latest \
    /app/bff-server -mode local -addr :8080

# With specific context
docker run --rm -p 8080:8080 \
    --user $(id -u):$(id -g) \
    -v ~/.kube/config:/app/.kube/config:ro \
    -e KUBECONFIG=/app/.kube/config \
    ghcr.io/k8s-lynq/lynq-dashboard:latest \
    /app/bff-server -mode local -addr :8080 -context my-cluster
```

Then open http://localhost:8080

### Image Details

| Property | Value |
|----------|-------|
| Base Image | Alpine 3.20 |
| Size | ~85MB |
| User | 65532 (non-root) |
| Architectures | amd64, arm64 |
| Health Check | `/healthz` |

### Adding shadcn/ui Components

Components are manually implemented following shadcn/ui patterns. To add new components:

1. Create component in `ui/src/components/ui/`
2. Use `cn()` utility from `lib/utils`
3. Follow existing component patterns

## Roadmap

- [x] **Phase 1**: Basic dashboard with Hub/Form/Node listing
- [x] **Phase 2**: Canvas-based topology visualization
- [x] **Phase 3**: Detailed resource views with events
- [x] **Phase 4**: Production Docker image
- [ ] **Phase 5**: Helm chart for cluster deployment

## License

Apache License 2.0
