---
description: "Advanced Lynq patterns: custom domains, blue-green deployments, multi-tier stacks, database-per-node, and feature flags."
---

# Advanced Use Cases

::: info Multi-Node Examples
As the most common use case for Lynq, this guide presents various patterns using **Multi-Node** (SaaS application with multiple customers/nodes) as examples. These patterns can be adapted for any database-driven infrastructure automation scenario.
:::

## Overview

Lynq's flexible architecture enables powerful automation patterns beyond basic resource provisioning. This guide helps you understand which pattern fits your requirements.

## Available Patterns

### 1. Custom Domain Provisioning

**Use when:** Each node needs their own custom domain with automatic DNS and SSL.

**Key features:**
- Automatic DNS record creation via External DNS
- Domain verification workflows
- Let's Encrypt SSL certificates
- CNAME delegation support

**Best for:** SaaS platforms where customers bring their own domains.

**[Read full guide →](/use-case-custom-domains)**

---

### 2. Multi-Tier Application Stack

**Use when:** Your application has multiple services (web, API, workers, data) that need coordinated deployment.

**Key features:**
- Separate templates per tier
- Independent scaling per tier
- Different policies per tier
- Coordinated lifecycle management

**Best for:** Complex applications with multiple service tiers.

**[Read full guide →](/use-case-multi-tier)**

---

### 3. Blue-Green Deployments

**Use when:** You need zero-downtime deployments with instant rollback capability.

**Key features:**
- Two complete environments (blue and green)
- Traffic switch via Service selector
- Test new versions before going live
- Instant rollback by switching back

**Best for:** Production systems requiring zero-downtime updates.

**[Read full guide →](/use-case-blue-green)**

---

### 4. Database-per-Node

**Use when:** Each node needs complete data isolation with dedicated database instances.

**Key features:**
- Automatic RDS/Cloud SQL provisioning via Crossplane
- Connection secret management
- Plan-based database sizing
- Retention policies for data safety

**Best for:** Compliance-heavy industries requiring complete data isolation.

**[Read full guide →](/use-case-database-per-tenant)**

---

### 5. Dynamic Feature Flags

**Use when:** You want to enable/disable features per node without redeployment.

**Key features:**
- Application-level flags via environment variables
- Infrastructure-level flags via database views
- A/B testing support
- Plan-based feature gating

**Best for:** SaaS platforms with multiple subscription tiers or gradual feature rollouts.

**[Read full guide →](/use-case-feature-flags)**

## Pattern Selection Guide

### By Deployment Strategy

| Requirement | Recommended Pattern |
|-------------|---------------------|
| Zero-downtime updates | [Blue-Green Deployments](/use-case-blue-green) |
| Multiple service tiers | [Multi-Tier Stack](/use-case-multi-tier) |

### By Infrastructure Needs

| Requirement | Recommended Pattern |
|-------------|---------------------|
| Custom domains per node | [Custom Domain Provisioning](/use-case-custom-domains) |
| Dedicated databases | [Database-per-Node](/use-case-database-per-tenant) |
| Optional expensive features | [Feature Flags](/use-case-feature-flags) (Pattern 2) |

### By Business Model

| Business Model | Recommended Patterns |
|----------------|----------------------|
| SaaS with subscription tiers | [Feature Flags](/use-case-feature-flags) + [Custom Domains](/use-case-custom-domains) |
| Enterprise B2B | [Database-per-Node](/use-case-database-per-tenant) + [Multi-Tier](/use-case-multi-tier) |
| High-traffic consumer app | [Blue-Green Deployments](/use-case-blue-green) + [Feature Flags](/use-case-feature-flags) |

## Combining Patterns

Many use cases benefit from combining multiple patterns:

### SaaS Platform (Recommended Stack)
1. **Multi-Tier Stack** - Separate web, API, worker, data tiers
2. **Custom Domains** - Each customer gets their own domain
3. **Feature Flags** - Different features per subscription plan
4. **Blue-Green Deployments** - Safe deployment of new versions

### Enterprise Platform
1. **Database-per-Node** - Complete data isolation
2. **Multi-Tier Stack** - Complex application architecture
3. **Blue-Green** - Zero-downtime updates for mission-critical systems

### Startup/Growth Stage
1. **Feature Flags** - Rapid iteration and A/B testing
2. **Custom Domains** - Professional branding for customers
3. **Blue-Green Deployments** - Safe scaling as you grow

## Implementation Best Practices

### 1. Start Simple
Begin with a single pattern that addresses your most critical need, then add more as requirements grow.

### 2. Use Database Views
For complex filtering logic, create database views rather than trying to implement logic in templates.

```sql
-- Example: Filter nodes by plan and feature
CREATE OR REPLACE VIEW enterprise_with_ai AS
SELECT * FROM nodes
WHERE plan_type = 'enterprise'
  AND feature_ai_enabled = TRUE
  AND is_active = TRUE;
```

### 3. Leverage Cross-Namespace Resources
Use `targetNamespace` to organize resources across namespaces for better isolation.

```yaml
deployments:
  - id: app
    nameTemplate: "{{ .uid }}-app"
    targetNamespace: "node-{{ .uid }}"  # Creates in node's namespace
```

### 4. Set Appropriate Policies
Choose policies based on resource type:

- **Databases**: `creationPolicy: Once`, `deletionPolicy: Retain`
- **Configurations**: `creationPolicy: WhenNeeded`, `deletionPolicy: Delete`
- **Temporary resources**: `creationPolicy: WhenNeeded`, `deletionPolicy: Delete`

### 5. Monitor Everything
Set up comprehensive monitoring for:
- Resource provisioning status
- Feature usage per node
- Deployment progression
- Cost per node

## Common Pitfalls

### ❌ Don't: Use YAML-level conditionals
Templates are for **values**, not for conditional YAML structure.

```yaml
# ❌ This doesn't work
{{- if .featureEnabled }}
deployments:
  - id: feature-deployment
{{- end }}
```

### ✅ Do: Use database views and separate templates
```sql
-- Database view
CREATE VIEW nodes_with_feature AS
SELECT * FROM nodes WHERE feature_enabled = TRUE;
```

```yaml
# Separate LynqHub and Template
apiVersion: operator.lynq.sh/v1
kind: LynqHub
metadata:
  name: feature-enabled-nodes
spec:
  source:
    mysql:
      table: nodes_with_feature  # Use the view
```

### ❌ Don't: Store complex logic in LynqHub
The hub should only read and map data, not transform it.

### ✅ Do: Use database views for complex queries
Move JOIN operations and complex filtering to database views.

## Getting Help

- **Documentation Issues**: [Report on GitHub](https://github.com/k8s-lynq/lynq/issues)
- **Architecture Questions**: Review [Architecture Guide](/architecture)
- **Template Help**: See [Templates Guide](/templates)
- **Policy Questions**: Check [Policies Documentation](/policies)

## Next Steps

1. Review the pattern that best matches your needs
2. Study the full guide for that pattern
3. Adapt the example to your requirements
4. Start with a single node for testing
5. Gradually roll out to more nodes

## Contributing

Have a use case not covered here? We'd love to hear about it!

- **Open an Issue**: [GitHub Issues](https://github.com/k8s-lynq/lynq/issues)
- **Share Your Story**: Contribute a use case guide
- **Join Discussions**: Share your experience with the community
