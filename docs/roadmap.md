---
description: "Lynq feature roadmap with completed milestones (v1.0, v1.1) and planned features for v1.2 and v1.3."
---

# Roadmap

Future plans and feature roadmap for Lynq.



## v1.0 ✅

::: info Status
Released
:::

### Features
- ✅ MySQL datasource support
- ✅ Template-based resource generation
- ✅ Server-Side Apply (SSA)
- ✅ Dependency management with DAG
- ✅ Policy-based lifecycle (Creation/Deletion/Conflict)
- ✅ Patch strategies (apply/merge/replace)
- ✅ Fast reconciliation (30s requeue)
- ✅ Smart watch predicates
- ✅ Multi-template support
- ✅ Webhook validation
- ✅ Prometheus metrics
- ✅ Comprehensive documentation

### Performance
- ✅ Event-driven architecture
- ✅ Optimized reconciliation
- ✅ Label-based namespace tracking
- ✅ Efficient database querying

## v1.1 (Current) ✅

::: info Focus
Cross-namespace support and operational improvements
:::

### New Features

- ✅ **Helm Chart Distribution**
  - Helm chart published via GitHub Releases
  - Public repo: https://k8s-lynq.github.io/lynq
  - Customizable values and upgrade path with `helm upgrade`

- ✅ **Cross-Namespace Resource Provisioning**
  - Support creating node resources in different namespaces using `targetNamespace` field
  - Uses label-based tracking (`lynq.sh/node`, `lynq.sh/node-namespace`) for cross-namespace resources
  - Automatic detection: same-namespace uses ownerReferences, cross-namespace uses labels
  - Dual watch system: `Owns()` for same-namespace + `Watches()` with label selectors for cross-namespace
  - Enables multi-namespace node isolation and organizational boundaries

- ✅ **Orphan Resource Cleanup**
  - Automatic detection and cleanup of resources removed from templates
  - Status-based tracking with `appliedResources` field
  - Respects DeletionPolicy (Delete/Retain)
  - Orphan labels for retained resources for easy identification

## v1.2

::: info Focus
Additional datasources and enhanced observability
:::

### New Features

- [ ] **PostgreSQL Datasource**
  - Full PostgreSQL support
  - Connection pooling
  - SSL/TLS support
  - Query optimization

- [ ] **Enhanced Metrics Dashboard**
  - Pre-built Grafana dashboards
  - Comprehensive AlertManager rules
  - Multi-node metrics visualization
  - Performance analytics

### Improvements
- [ ] Improved error messages
- [ ] Performance optimizations
- [ ] Extended template functions
- [ ] Better documentation examples

## v1.3

::: info Focus
Scalability and advanced multi-tenancy features
:::

### New Features

- [ ] **Node Sharding for Large-Scale Deployments**
  - Horizontal sharding of node workloads across multiple operator instances
  - Shard key-based node distribution
  - Load balancing across shards
  - Shard rebalancing and migration support
  - Use cases:
    - Supporting 10,000+ nodes per cluster
    - Isolating node failures to specific shards
    - Reducing controller resource consumption
    - Enabling independent scaling of operator replicas

- [ ] **Advanced Multi-Tenancy Isolation**
  - Node priority and resource quotas
  - Per-node rate limiting
  - Node lifecycle hooks
  - Custom node tagging and filtering

### Breaking Changes

::: danger Breaking Changes in v1.3.0
The following deprecated features will be **removed** in v1.3.0:
:::

- [ ] **Removal of Deprecated `.hostOrUrl` and `.host` Variables**
  - **Deprecated since**: v1.1.11
  - **Removed in**: v1.3.0
  - **Migration required**: Use `extraValueMappings` + `toHost()` template function
  - **Impact**: LynqHub configurations using `valueMappings.hostOrUrl` will fail validation
  - **Migration guide**:
    ```yaml
    # Before (deprecated - will fail in v1.3.0)
    valueMappings:
      uid: node_id
      hostOrUrl: domain_url  # ❌ Removed
      activate: is_active

    # After (v1.3.0+)
    valueMappings:
      uid: node_id
      activate: is_active
    extraValueMappings:
      nodeUrl: domain_url    # ✅ Use extraValueMappings

    # In templates: {{ .nodeUrl | toHost }}
    ```
  - **Rationale**: Lynq is a general database-driven automation platform; removing the hardcoded host/URL requirement provides flexibility for diverse use cases beyond web-hosting scenarios

### Improvements
- [ ] Enhanced reconciliation performance for large node counts
- [ ] Improved status reporting and aggregation
- [ ] Optimized database query batching
- [ ] Better scaling metrics and recommendations

## Contributing to Roadmap

Want to influence the roadmap?

1. **Open a Discussion**: Share your use case
2. **Vote on Features**: Upvote existing requests
3. **Submit PRs**: Implement features yourself
4. **Join Community**: Participate in discussions

## Stability Commitments

### API Stability
- v1 API: Stable, no breaking changes
- Future versions: Migration guides provided
- Deprecation policy: 6 months notice

### Backwards Compatibility
- Database schema changes: Automatic migration
- Template syntax: Backwards compatible
- Metrics: No breaking changes without notice

## Getting Involved

- 💬 Discussions: https://github.com/k8s-lynq/lynq/discussions
- 🐛 Issues: https://github.com/k8s-lynq/lynq/issues
- 📧 Email: rationlunas@gmail.com
- 🔔 Release notifications: Watch repository

## See Also

- [Contributing Guide](https://github.com/k8s-lynq/lynq/blob/main/CONTRIBUTING.md)
- [Development Guide](development.md)
- [GitHub Discussions](https://github.com/k8s-lynq/lynq/discussions)
