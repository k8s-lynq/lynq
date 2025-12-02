````markdown
# Lynq: Adopting Existing Kubernetes Resources

Welcome to the Lynq Operator advanced tutorial! 🚀

## The Challenge

You have **existing Kubernetes resources** (ConfigMaps, Deployments, etc.) that were manually created or managed by another tool. Now you want to:

- **Sync configuration data** from a central database
- **Automate updates** when database values change
- **Keep existing resources** running without disruption

**Lynq can take over management of existing resources using the `Force` conflict policy!**

## What You'll Learn

In this hands-on scenario, you will:

1. ✅ Create **existing ConfigMaps** (simulating pre-existing infrastructure)
2. ✅ Set up a **MySQL database** with configuration data
3. ✅ Install **Lynq Operator** to manage these resources
4. ✅ Use `conflictPolicy: Force` to **adopt existing resources**
5. ✅ Watch **automatic sync** as database values change

## Architecture: Before and After

### Before Lynq

```
┌─────────────────────────────────────────────────────────┐
│              Manual Resource Management                  │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  Manual kubectl apply                                    │
│        │                                                 │
│        ▼                                                 │
│  ┌──────────────────┐                                   │
│  │  app-a-config    │  (manually maintained)            │
│  │  app-b-config    │  (out of sync risk)               │
│  │  app-c-config    │  (no audit trail)                 │
│  └──────────────────┘                                   │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### After Lynq

```
┌─────────────────────────────────────────────────────────┐
│              Database-Driven Management                  │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────┐    ┌──────────┐    ┌─────────────────┐   │
│  │  MySQL   │───►│ LynqHub  │───►│   LynqNode CRs  │   │
│  │ Database │    │(Sync 30s)│    │  (Per app)      │   │
│  │  (SSOT)  │    └──────────┘    └────────┬────────┘   │
│  └──────────┘                              │            │
│                                            ▼            │
│  ┌──────────┐                    ┌─────────────────┐   │
│  │ LynqForm │──────Force───────►│  app-a-config   │   │
│  │(Template)│    (adopt)         │  app-b-config   │   │
│  └──────────┘                    │  app-c-config   │   │
│                                   └─────────────────┘   │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## Key Feature: ConflictPolicy

| Policy | Behavior |
|--------|----------|
| `Stuck` (default) | Fail if resource exists with different owner |
| `Force` | **Take ownership** of existing resources via SSA |

The `Force` policy enables Lynq to **adopt and manage existing resources** without deleting and recreating them!

Let's get started! Click **Start** to begin.

````
