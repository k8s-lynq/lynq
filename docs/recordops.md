---
description: "Understand RecordOps and Infrastructure as Data — the paradigm where database records become infrastructure specifications. Compare IaD vs IaC and learn when each approach works best."
---

# RecordOps: Infrastructure as Data

If you've ever built a multi-tenant SaaS platform, you've probably felt this pain: your customer data lives in your database, but their infrastructure is managed somewhere else (YAML files in Git, Terraform state, manual kubectl commands). Every time you onboard a new customer, you're coordinating between multiple systems that don't naturally talk to each other.

**RecordOps** changes this. It's a new paradigm called **Infrastructure as Data** where database records become the source of truth for infrastructure. When you insert a row, infrastructure provisions. When you update a field, resources reconfigure. When you delete a record, everything cleans up.

[[toc]]

## What is Infrastructure as Data?

You're familiar with Infrastructure as Code (IaC): using Terraform or Pulumi to define infrastructure through code. Infrastructure as Data (IaD) is different: instead of writing code to describe infrastructure, you write data to describe state.

The term was coined by Michael DeHaan (creator of Ansible) in 2013:

> "Infrastructure is best modeled not as code, nor in a GUI, but as a text-based, middle-ground, data-driven policy."

::: tip The Paradigm Shift
- **Infrastructure as Code**: Write code (HCL, TypeScript) → Run apply → Infrastructure changes
- **Infrastructure as Data**: Write data (YAML, SQL, database rows) → Infrastructure follows automatically
:::

Infrastructure as Data is the foundation of tools like Ansible (playbooks), Crossplane (Kubernetes CRs), and RecordOps (database records).

It makes sense when:
- Your infrastructure maps directly to your data model (one customer = one stack)
- You're provisioning the same pattern repeatedly
- Your application already knows everything needed to provision infrastructure

## What is RecordOps?

**RecordOps** (Record Operations) is the operational pattern that implements Infrastructure as Data. It treats database records as infrastructure specifications.

::: tip Core Principle
RecordOps is how you practice Infrastructure as Data in production. Every database record defines a running stack in your cluster.
:::

The pattern is straightforward:

```
INSERT a row     →  Infrastructure provisions
UPDATE a column  →  Resources reconfigure
DELETE a record  →  Everything cleans up
```

This is Infrastructure as Data in action. Your database becomes your infrastructure control plane.

## Lynq: A RecordOps Platform

**Lynq** is an open-source Kubernetes operator that implements the RecordOps pattern. It's a practical implementation of Infrastructure as Data for cloud-native environments.

Lynq watches your database and continuously syncs infrastructure state to match your data. It's infrastructure management that's as simple as managing database records.

## The Problem This Solves

Let me show you a typical customer onboarding flow:

::: code-group

```text [Infrastructure as Code (Traditional)]
1. Customer signs up → INSERT into customers table
2. Write Terraform/YAML manifests
3. Commit to Git and wait for PR approval
4. Wait for CI/CD pipeline (hope it doesn't fail)
5. Monitor until everything is up
6. Update customer record with endpoint URL

⏱️  Time: 15-45 minutes
🔄  Manual steps: 6
❌  Context switches: Multiple systems
```

```text [Infrastructure as Data (RecordOps)]
1. Customer signs up
2. INSERT INTO customers (id, domain, plan, active) VALUES (...)
3. Infrastructure provisions automatically

⏱️  Time: 30 seconds
🔄  Manual steps: 1
✅  One system: Your database
```

:::

::: info The Key Insight
With Infrastructure as Data, your application's data model IS your infrastructure model. No duplication. No coordination. Just data.
:::

## Why Infrastructure as Data?

### Your Database Already Has the Answers

Think about what information you need to provision infrastructure:
- Customer ID
- Domain name
- Plan/tier
- Region
- Resource limits
- Feature flags

All of this is already in your database. Infrastructure as Code duplicates this in YAML files or Terraform variables. Infrastructure as Data just reads it directly.

::: tip Question Worth Asking
If your infrastructure maps to your data, why not let your data drive your infrastructure?
:::

### Operations Become Data Changes

With Infrastructure as Data, operational tasks are just database operations:

::: code-group

```sql [Scale a Customer]
UPDATE customers
SET replicas = 10
WHERE id = 'acme-corp';
```

```sql [Enable a Feature]
INSERT INTO feature_flags (customer_id, feature, enabled)
VALUES ('acme-corp', 'ai-assistant', true);
```

```sql [Rollback Deployment]
UPDATE deployments
SET active_version = 'blue'
WHERE customer_id = 'acme-corp';
```

:::

No new tooling. No context switching. Just SQL, operations you already know.

### Testing Becomes Natural

Infrastructure as Code cloning: Export state → Modify variables → Run apply → Debug conflicts → Maybe it works

Infrastructure as Data cloning: Clone database rows

```sql
-- Clone production to staging
INSERT INTO customers
SELECT * FROM customers WHERE environment = 'prod';

UPDATE customers
SET environment = 'staging',
    domain = CONCAT(domain, '.staging')
WHERE id IN (...);
```

::: tip
30 seconds later, perfect staging environment. Every service, every configuration, every dependency recreated automatically because the data was copied.
:::

## How Infrastructure as Data Compares

### Infrastructure as Data vs Infrastructure as Code

| Aspect | Infrastructure as Code | Infrastructure as Data |
|--------|----------------------|----------------------|
| **Definition** | Code describes infrastructure | Data describes state |
| **Language** | HCL, TypeScript, Python | YAML (Ansible), Kubernetes CR (Crossplane), SQL (RecordOps) |
| **Execution** | Manual plan/apply | Automatic sync/reconciliation |
| **State Storage** | Separate state files | Declarative specs (playbooks, CRs, database) |
| **Examples** | Terraform, Pulumi | Ansible, Crossplane, RecordOps |
| **Best For** | Cloud resources, cluster setup | Declarative automation, data-driven apps |

::: info They Complement Each Other
- **IaC**: Provision your Kubernetes cluster, cloud resources
- **IaD**: Declarative automation (Ansible for config, Crossplane for cloud, RecordOps for tenants)
:::

### Infrastructure as Data vs GitOps

GitOps is great for cluster-level infrastructure. Your operators, CRDs, system services should absolutely be in Git with proper review processes.

But for per-customer stacks? Git becomes tedious. You're creating YAML files for each customer, managing merge conflicts, waiting for CI/CD. Infrastructure as Data makes this simple: one database row per customer.

::: info They Work Well Together
- **GitOps**: Cluster-level configuration (changes infrequently, requires review)
- **Infrastructure as Data**: Customer-level resources (changes frequently, follows your data)
:::

## RecordOps with Lynq: Implementation Details

Lynq implements Infrastructure as Data through three components:

**1. LynqHub** - Database watcher that syncs records every 30 seconds (configurable)

**2. LynqForm** - Infrastructure template. Defines what each database record creates

**3. LynqNode** - One per active record, managing all resources for that customer/tenant/project

### A Concrete Example

Your database schema defines your infrastructure API:

```sql
CREATE TABLE tenants (
  tenant_id VARCHAR(50) PRIMARY KEY,
  domain VARCHAR(255) NOT NULL,
  plan VARCHAR(20),
  active BOOLEAN DEFAULT TRUE,
  replicas INT DEFAULT 2
);
```

This is Infrastructure as Data. Columns become infrastructure parameters.

You define a LynqForm template once: "For each active tenant, create namespace, deployment (with `replicas` replicas), service, and ingress (pointing to `domain`)."

Now infrastructure follows your data:

```sql
INSERT INTO tenants VALUES
  ('acme-corp', 'acme.example.com', 'enterprise', true, 5);
```

::: tip What Happens Automatically
Lynq detects the new row within 30 seconds and provisions:
- **Namespace**: `acme-corp`
- **Deployment**: 5 replicas
- **Service**: `acme-corp-app`
- **Ingress**: Routes `acme.example.com` to the service

This is Infrastructure as Data in practice. Data defines infrastructure, Lynq provisions it.
:::

## Common Patterns in Infrastructure as Data

::: details Pattern 1: Feature Flags as Infrastructure Parameters
Infrastructure as Data makes feature flags literal infrastructure parameters:

```sql
CREATE TABLE feature_flags (
  tenant_id VARCHAR(50),
  feature VARCHAR(50),
  enabled BOOLEAN
);

-- Feature flag becomes infrastructure
INSERT INTO feature_flags VALUES ('acme-corp', 'ai-assistant', true);
```

Your template has conditional logic: if `ai-assistant` flag is enabled, deploy AI service. Otherwise, skip it. Infrastructure automatically adapts to your data.
:::

::: details Pattern 2: Blue-Green Deployments as a Column
With Infrastructure as Data, deployment strategy is just a column:

```sql
CREATE TABLE deployments (
  tenant_id VARCHAR(50),
  active_version VARCHAR(10) -- 'blue' or 'green'
);

-- Change the data, change the infrastructure
UPDATE deployments SET active_version = 'green' WHERE tenant_id = 'acme-corp';
```

Service selector updates automatically. Traffic switches in seconds. Roll back by changing the column back to 'blue'.
:::

::: details Pattern 3: Ephemeral Environments with TTL
Infrastructure as Data with lifecycle management:

```sql
CREATE TABLE environments (
  id VARCHAR(50),
  domain VARCHAR(255),
  ttl TIMESTAMP
);

INSERT INTO environments VALUES
  ('demo-acme', 'demo-acme.example.com', NOW() + INTERVAL 7 DAY);

-- Database trigger cleans up expired data
CREATE TRIGGER cleanup_expired
AFTER INSERT OR UPDATE ON environments
BEGIN
  DELETE FROM environments WHERE ttl < NOW();
END;
```

When the data is deleted, infrastructure cleans up automatically. Perfect for demo environments or PR previews.
:::

## Practical Benefits of Infrastructure as Data

### For Development

Before: Multiple tools (Git, Terraform, kubectl), multiple contexts, manual coordination

Now: One tool (SQL), one context (your database), automatic provisioning

Test changes by inserting test records. Same skills you use every day (writing queries, managing transactions) work for infrastructure too.

### For Operations

Before: Infrastructure state scattered (Git, Terraform state, Vault, etc.)

Now: Infrastructure state in one place (your database)

- Audit logs are database logs
- Backups include infrastructure configuration
- No state file reconciliation
- Automatic drift correction (Lynq continuously syncs database to cluster)

Rollbacks: Restore database → Infrastructure recreates automatically

### For Business

Before: Customer onboarding takes 15-45 minutes of manual coordination

Now: Customer onboarding is a database transaction (30 seconds)

Feature rollouts are database toggles. The gap between what your application knows and what your infrastructure provides disappears.

## When Infrastructure as Data Makes Sense

::: tip ✅ Perfect For
- **Multi-tenant SaaS platforms** - Each customer/tenant needs isolated infrastructure
- **Data-driven applications** - Infrastructure follows your data model
- **Frequent provisioning** - Multiple times per day
- **Repeated patterns** - Same stack per customer/project
:::

::: warning ❌ Better to Use Infrastructure as Code If
- You rarely provision infrastructure (once a month or less)
- Infrastructure requires manual approval for every change
- You need deep cloud provider integrations (use Terraform)
- Infrastructure doesn't map to database records
:::

::: info 🤝 Best: Combine Both
Use Infrastructure as Code for your cluster and cloud resources. Use Infrastructure as Data for customer/tenant infrastructure. They complement each other perfectly.
:::

## Considerations for Infrastructure as Data

::: warning Your Database Becomes Infrastructure Control Plane
With Infrastructure as Data, your database controls infrastructure. This means:

- **Database availability is critical** - Though existing infrastructure keeps running if DB goes down
- **Schema migrations affect infrastructure** - Test carefully in staging
- **Database permissions = Infrastructure permissions** - Be thoughtful about access control
:::

::: danger Security Model Shift
SQL injection becomes infrastructure injection. User input that manipulates queries could trigger unwanted infrastructure changes.

Always use parameterized queries. Validate all inputs. Protect database credentials; they control your cluster.
:::

::: info Sync Interval Trade-offs
Lynq syncs every 30 seconds by default. Small delay between data change and infrastructure provisioning. For most cases, this is fine. Tune as needed for your latency requirements.
:::

## Getting Started with Infrastructure as Data

Ready to try Infrastructure as Data with Lynq?

**1. Design your data schema as infrastructure API**

What database rows should represent infrastructure? Customers? Projects? Environments?

```sql
ALTER TABLE customers
  ADD COLUMN replicas INT DEFAULT 2,
  ADD COLUMN region VARCHAR(20) DEFAULT 'us-east-1',
  ADD COLUMN active BOOLEAN DEFAULT TRUE;
```

Your schema IS your infrastructure API.

**2. Install Lynq and connect to your database**

```yaml
apiVersion: operator.lynq.sh/v1
kind: LynqHub
spec:
  source:
    type: mysql
    mysql:
      host: mysql.default.svc
      database: myapp
      table: customers
    syncInterval: 30s
  valueMappings:
    uid: customer_id
    activate: active
  extraValueMappings:
    replicas: replicas
    plan: plan
```

**3. Define your infrastructure template**

What should each record create? Namespace? Deployment? Services? Define once in a LynqForm.

**4. Test with data**

```sql
INSERT INTO customers VALUES ('test', 'test.example.com', 'pro', true, 2);
```

Watch infrastructure provision. If something's wrong, delete the row and try again. It's just data.

## Closing Thoughts

Infrastructure as Data isn't about replacing every tool you use. It's about recognizing when your infrastructure naturally maps to your data and eliminating the artificial gap between them.

**Infrastructure as Code** is powerful for cloud resources and cluster setup. **Infrastructure as Data** is powerful for per-customer stacks and data-driven applications.

**RecordOps** is how you practice Infrastructure as Data. **Lynq** is an open-source platform that implements it for Kubernetes.

If your infrastructure follows your data model, maybe it's time to let your data drive your infrastructure directly.

## Learn More

- [How Lynq Works](./how-it-works.md) - RecordOps architecture
- [Quick Start](./quickstart.md) - Try Infrastructure as Data in 5 minutes
- [Architecture](./architecture.md) - Lynq system design
- [Use Cases](./advanced-use-cases.md) - Real-world patterns

---

Questions? Open an issue on [GitHub](https://github.com/k8s-lynq/lynq/issues) or start a [discussion](https://github.com/k8s-lynq/lynq/discussions).
