# Lynq Operator: Database-Driven Kubernetes Automation

Welcome to the Lynq Operator interactive tutorial! 🚀

## What is Lynq?

**Lynq** is a Kubernetes operator that automatically provisions infrastructure based on your database rows. Think of it as a bridge between your business data and Kubernetes resources.

```
Database Row ──► Lynq Operator ──► Kubernetes Resources
   (MySQL)                         (Deployments, Services, etc.)
```

## What You'll Learn

In this hands-on scenario, you will:

1. ✅ Install **Lynq Operator** on a Kubernetes cluster
2. ✅ Set up a **MySQL database** with sample tenant data
3. ✅ Create a **LynqHub** to connect to your database
4. ✅ Define a **LynqForm** template for resource provisioning
5. ✅ Watch **automatic provisioning** as database rows change

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Lynq Architecture                     │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────┐    ┌──────────┐    ┌─────────────────┐   │
│  │  MySQL   │───►│ LynqHub  │───►│   LynqNode CRs  │   │
│  │ Database │    │(Sync DB) │    │  (Per active    │   │
│  └──────────┘    └──────────┘    │   row)          │   │
│                                   └────────┬────────┘   │
│                                            │            │
│  ┌──────────┐                              ▼            │
│  │ LynqForm │──────────────────►  K8s Resources        │
│  │(Template)│                    (Deployments,         │
│  └──────────┘                     Services, etc.)      │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## Use Cases

- **Multi-Tenant SaaS**: Provision isolated infrastructure per customer
- **Development Environments**: Create dev/staging environments from config
- **Feature Flags**: Enable/disable infrastructure based on feature toggles
- **Blue-Green Deployments**: Manage deployment strategies via database

Let's get started! Click **Start** to begin.
