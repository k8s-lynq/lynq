---
description: "Advanced Lynq patterns: custom domains, blue-green deployments, multi-tier stacks, database-per-node, feature flags, preview environments, and sandbox environments."
---

# Advanced Use Cases

Seven production-proven patterns. Each has a full guide with working YAML.

## Which Pattern Do I Need?

| My requirement | Pattern |
|----------------|---------|
| Per-PR isolated environments, automatic cleanup | [Preview Environments](#preview-environments) |
| Developer/test sandboxes per account | [Sandbox Environments](#sandbox-environments) |
| Zero-downtime deploys, one-SQL-statement rollback | [Blue-Green Deployments](#blue-green-deployments) |
| Customers bring their own domain + TLS | [Custom Domain Provisioning](#custom-domain-provisioning) |
| Web, API, worker, data tiers independently scaled | [Multi-Tier Application Stack](#multi-tier-application-stack) |
| Complete data isolation per node (compliance) | [Database-per-Node](#database-per-node) |
| Enable/disable features per plan without redeployment | [Dynamic Feature Flags](#dynamic-feature-flags) |

## Preview Environments

**Per-PR environments provisioned from CI with a single SQL INSERT. Deleting the row cleans up every resource.**

CI inserts one row per open PR — Lynq provisions a namespace, Deployment, Service, and TLS Ingress within ~60 seconds. When the PR closes, CI runs `DELETE`. No orphan environments, no stale namespaces.

**[Full guide →](use-case-preview-environments.md)**

## Sandbox Environments

**Isolated API sandbox environments per developer account — provisioned automatically, updated on plan change, torn down on account close.**

Each account row maps to one sandbox: isolated namespace, API server, mock webhook sink, and per-account config. Plan upgrades reconcile resource limits automatically.

**[Full guide →](use-case-sandbox-environments.md)**

## Blue-Green Deployments

**Zero-downtime deploys controlled by a single database column. Roll back with one SQL UPDATE.**

An `active_color` column determines which environment is live. Lynq keeps both running; the Service selector follows the column value.

**[Full guide →](use-case-blue-green.md)**

## Custom Domain Provisioning

**Each node gets its own domain with automatic DNS and TLS — driven by a URL column in your database.**

A `custom_domain` column triggers ExternalDNS record creation and cert-manager certificate provisioning. The domain goes live without any manual configuration.

**[Full guide →](use-case-custom-domains.md)**

## Multi-Tier Application Stack

**Separate templates for web, API, worker, and data tiers — each independently scaled and lifecycled.**

Multiple LynqForms reference the same hub. Each tier gets its own LynqNode and can be updated, scaled, or replaced without touching the others.

**[Full guide →](use-case-multi-tier.md)**

## Database-per-Node

**A dedicated cloud database per node, provisioned via Crossplane. Required for compliance isolation.**

Each node row triggers an RDS/Cloud SQL instance (or isolated schema/database on a shared instance). Credentials are written to Kubernetes Secrets automatically.

**[Full guide →](use-case-database-per-tenant.md)**

## Dynamic Feature Flags

**Enable or disable features per node via database columns — no redeployment needed.**

A boolean column in your database maps to a Kubernetes resource. Set the column to `false`; Lynq removes the resource on the next sync. Works for infrastructure features (dedicated queues, extra replicas) and application flags (env vars).

**[Full guide →](use-case-feature-flags.md)**

## Combining Patterns

| Scenario | Recommended combination |
|----------|------------------------|
| SaaS platform | Multi-Tier + Custom Domains + Feature Flags + Blue-Green |
| Enterprise B2B | Database-per-Node + Multi-Tier + Blue-Green |
| Developer platform | Sandbox Environments + Feature Flags |
| Startup / early growth | Feature Flags + Custom Domains + Blue-Green |

## See Also

- [Templates](templates.md) — All template variables and functions.
- [Policies](policies.md) — DeletionPolicy for safe teardown, CreationPolicy for one-time resources.
- [Dependencies](dependencies.md) — Ordered provisioning for multi-resource patterns.
- [Multi-Tenant Isolation](multi-tenant-isolation.md) — Namespace-per-node and NetworkPolicy.
