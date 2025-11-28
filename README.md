<div align="center">

<img src="docs/public/logo.png" alt="Kubernetes Lynq Operator" width="200">

# Lynq Operator

### Kubernetes-Native Database-Driven Automation

**Automate resource lifecycle from database to production**

[![Go Report Card](https://goreportcard.com/badge/github.com/k8s-lynq/lynq)](https://goreportcard.com/report/github.com/k8s-lynq/lynq)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/lynq)](https://artifacthub.io/packages/search?repo=lynq)
[![Build Status](https://github.com/k8s-lynq/lynq/actions/workflows/build-push.yml/badge.svg)](https://github.com/k8s-lynq/lynq/actions/workflows/build-push.yml)
[![Container Image](https://img.shields.io/badge/container-ghcr.io-blue)](https://github.com/k8s-lynq/lynq/pkgs/container/lynq)
[![Go Version](https://img.shields.io/github/go-mod/go-version/k8s-lynq/lynq)](go.mod)

[Documentation](https://lynq.sh/) • [Quick Start](https://lynq.sh/quickstart) • [Architecture](https://lynq.sh/architecture) • [Contributing](#-contributing)

[![Try Interactive Demo](https://img.shields.io/badge/🎮_Try_Demo-Killercoda-10b981?style=for-the-badge)](https://killercoda.com/lynq-operator/course/killercoda/lynq-quickstart)

</div>

---

## 📖 Overview

**Lynq Operator** is a Kubernetes operator that automates database-driven infrastructure provisioning. It reads data from external datasources and dynamically creates, updates, and manages Kubernetes resources using declarative templates.

### 💡 Core Concept

> [!IMPORTANT]
> **One database row = One complete set of Kubernetes resources**
>
> Each row in your database automatically provisions and manages a complete stack of Kubernetes resources (Deployments, Services, Ingresses, ConfigMaps, and more) using declarative templates.

### ✨ Key Features

- 🗄️ **Database-driven automation** - Sync resources from MySQL, PostgreSQL, and more
- 📝 **Go templates + Sprig functions** - Powerful template engine with 200+ built-in functions
- 🔄 **Server-Side Apply** - Kubernetes-native resource management
- 📊 **DAG-based dependencies** - Control resource creation order with dependency graphs
- ⚙️ **Lifecycle policies** - Fine-grained control over creation, deletion, and conflicts
- 🚀 **Production-ready** - Battle-tested with comprehensive monitoring and observability

### 📚 Learn More

- **[Complete Documentation](https://lynq.sh/)** - Full guides, tutorials, and API reference
- **[Architecture Guide](https://lynq.sh/architecture)** - Deep dive into design and internals

---

## 🚀 Quick Start

Get started with Lynq Operator in 5 minutes:

- 🎯 **[Quick Start Guide](https://lynq.sh/quickstart)** - Step-by-step tutorial with working examples
- 📦 **[Installation Guide](https://lynq.sh/installation)** - Helm, Kustomize, and manual installation options
- ⚙️ **[Configuration Examples](https://lynq.sh/quickstart#configuration)** - LynqHub and LynqForm setup

---

## 📚 Documentation

Complete documentation is available at **[lynq.sh](https://lynq.sh/)**

### Getting Started
- [Quick Start](https://lynq.sh/quickstart) - Get up and running in minutes
- [Installation](https://lynq.sh/installation) - Multiple installation methods

### Core Concepts
- [Architecture](https://lynq.sh/architecture) - System design and components
- [API Reference](https://lynq.sh/api) - CRD specifications
- [Templates](https://lynq.sh/templates) - Template syntax and functions

### Operations
- [DataSource Setup](https://lynq.sh/datasource) - Configure MySQL, PostgreSQL, and more
- [Monitoring](https://lynq.sh/monitoring) - Metrics and observability
- [Troubleshooting](https://lynq.sh/troubleshooting) - Common issues and solutions

### Integrations
- [Crossplane](https://lynq.sh/integration-crossplane) - Infrastructure as Code
- [ExternalDNS](https://lynq.sh/integration-external-dns) - Dynamic DNS management
- [Flux](https://lynq.sh/integration-flux) - GitOps workflows
- [Argo CD](https://lynq.sh/integration-argocd) - Declarative GitOps

---

## 🛠️ Development

### Getting Started

```bash
# Clone the repository
git clone https://github.com/k8s-lynq/lynq.git
cd lynq

# Install CRDs
make install

# Run locally
make run

# Run tests
make test
```

### Resources

- 📖 **[Development Guide](https://lynq.sh/development)** - Building, testing, and contributing
- 🔧 **[Local Development Setup](https://lynq.sh/development#local-setup)** - Environment configuration
- 🧪 **[Testing Guide](https://lynq.sh/development#testing)** - Unit, integration, and e2e tests

---

## 🤝 Contributing

We welcome contributions! Whether you're fixing bugs, adding features, improving documentation, or sharing use cases - all contributions are valued.

### How to Contribute

- 🐛 **[Report Issues](https://github.com/k8s-lynq/lynq/issues/new)** - Bug reports and feature requests
- 📋 **[Contributing Guidelines](CONTRIBUTING.md)** - Code standards, commit conventions, and PR process
- 🌟 **[Add a Datasource](https://lynq.sh/contributing-datasource)** - Pluggable adapter pattern guide
- 💬 **[Join Discussions](https://github.com/k8s-lynq/lynq/discussions)** - Ask questions and share ideas

---

## 📊 Project Status

**Production-ready** • Kubernetes v1.28-v1.33 validated • Apache 2.0 License

- ✅ **Stable** - Used in production environments
- 🗺️ **[Roadmap](docs/roadmap.md)** - Feature plans and versioning
- 🔄 **Active Development** - Regular updates and improvements

---

## 📝 License

Licensed under the [Apache License 2.0](LICENSE).

Copyright 2025 Lynq Operator Authors.

---

## 📬 Community & Support

### Get Help

- 🐛 **[Report Issues](https://github.com/k8s-lynq/lynq/issues)** - Bug reports and feature requests
- 💬 **[Discussions](https://github.com/k8s-lynq/lynq/discussions)** - Ask questions and share ideas
- 📖 **[Documentation](https://lynq.sh/)** - Comprehensive guides and tutorials

### Stay Updated

- ⭐ **Star this repository** to show your support
- 👁️ **Watch** for updates and releases
- 🔔 **Follow** our releases for the latest features

---

## ⭐ Star History

<a href="https://www.star-history.com/#k8s-lynq/lynq&type=timeline&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=k8s-lynq/lynq&type=timeline&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=k8s-lynq/lynq&type=timeline&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=k8s-lynq/lynq&type=timeline&legend=top-left" />
 </picture>
</a>

---

<div align="center">

**Made with ❤️ by the Lynq community**

[⬆ Back to Top](#lynq-operator)

</div>
