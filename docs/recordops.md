# RecordOps: Infrastructure That Follows Your Data

If you've ever built a multi-tenant SaaS platform, you've probably felt this pain: your customer data lives in your database, but their infrastructure is managed somewhere else—YAML files in Git, Terraform state, manual kubectl commands. Every time you onboard a new customer, you're coordinating between multiple systems that don't naturally talk to each other.

RecordOps is a different approach. Your database records become the source of truth for infrastructure. When you insert a row, infrastructure provisions. When you update a field, resources reconfigure. When you delete a record, everything cleans up. It's infrastructure management that's as simple as managing data.

[[toc]]

## What is RecordOps?

RecordOps (Record Operations) is an operational pattern where database records define infrastructure state. Instead of maintaining YAML files or writing Terraform code, you define infrastructure parameters as columns in your database tables.

The pattern is straightforward:

```
INSERT a row     →  Infrastructure provisions
UPDATE a column  →  Resources reconfigure
DELETE a record  →  Everything cleans up
```

Every active database row represents a running stack in your cluster. That's it.

## The Problem This Solves

Let me show you a typical customer onboarding flow:

**Traditional approach:**
1. Customer signs up, gets inserted into your `customers` table
2. You write YAML manifests for their namespace, deployment, service, ingress
3. Commit to Git and wait for approval
4. CI/CD pipeline runs (if it doesn't fail)
5. Monitor until everything is up
6. Update the customer record with their endpoint URL

This works, but it feels disconnected. Your application already knows about the customer (it's in your database), but now you need to manually coordinate with your infrastructure system.

**RecordOps approach:**
1. Customer signs up
2. `INSERT INTO customers (id, domain, plan, active) VALUES (...)`
3. Infrastructure provisions automatically within 30 seconds

The database transaction is all you need. Your application's data model and your infrastructure model are the same thing.

## Why This Resonates

### Your Database Already Has the Answers

Think about it: your database already contains all the information needed to provision infrastructure. Customer ID, domain, plan, region, feature flags—it's all there. You're just duplicating this information in YAML files or Terraform variables.

What if infrastructure could just read from the same place your application does?

### Operations Are Just Data Changes

When you think about common operational tasks, they're really just data changes:

**Scaling a customer:**
```sql
UPDATE customers SET replicas = 10 WHERE id = 'acme-corp';
```

**Enabling a feature:**
```sql
INSERT INTO feature_flags (customer_id, feature, enabled)
VALUES ('acme-corp', 'ai-assistant', true);
```

**Rolling back a deployment:**
```sql
UPDATE deployments SET active_version = 'blue' WHERE customer_id = 'acme-corp';
```

These are operations you already know how to do. They're part of your normal workflow. Why should provisioning infrastructure require learning a completely different toolchain?

### Testing Becomes Natural

With traditional infrastructure, cloning an environment is a project. You need to export state, modify variables, coordinate across systems.

With RecordOps, cloning an environment is just cloning database rows:

```sql
-- Clone production to staging
INSERT INTO customers
SELECT * FROM customers WHERE environment = 'prod';

UPDATE customers
SET environment = 'staging',
    domain = CONCAT(domain, '.staging')
WHERE id IN (...);
```

30 seconds later, you have a perfect copy of production running in staging. Every service, every configuration, every dependency—recreated automatically because the data was copied.

## How It Compares to Other Approaches

### vs GitOps

GitOps is great for cluster-level infrastructure. Your operators, CRDs, system services—these should absolutely be in Git with proper review processes.

But for per-customer stacks? Git becomes tedious. You're creating YAML files for each customer, managing merge conflicts, waiting for CI/CD. Meanwhile, your application already knows about these customers through the database.

RecordOps and GitOps work well together:
- **GitOps**: Cluster-level configuration (changes infrequently, requires review)
- **RecordOps**: Customer-level resources (changes frequently, follows your data)

### vs Traditional IaC

Terraform and Pulumi are excellent for cloud infrastructure. If you're provisioning AWS resources or managing your Kubernetes cluster itself, use them.

But if you're provisioning the same pattern repeatedly (one stack per customer, one environment per project), you might not need infrastructure-as-code. You might just need infrastructure-as-data.

Instead of writing code to describe infrastructure, you're adding rows to describe state. It's a different mental model that maps naturally to applications built around a database.

## RecordOps with Lynq

Lynq implements this pattern for Kubernetes. Here's how it works:

**1. LynqHub** - Connects to your database and syncs records periodically (default: every 30 seconds)

**2. LynqForm** - Defines the infrastructure template. What should each record create? A deployment? Services? Ingresses?

**3. LynqNode** - Represents each active record. One LynqNode per row, managing all resources for that customer/tenant/project.

### A Concrete Example

Let's say you have a `tenants` table:

```sql
CREATE TABLE tenants (
  tenant_id VARCHAR(50) PRIMARY KEY,
  domain VARCHAR(255) NOT NULL,
  plan VARCHAR(20),
  active BOOLEAN DEFAULT TRUE,
  replicas INT DEFAULT 2
);
```

You point Lynq to this table and define a template that says: "For each active tenant, create a namespace, deployment (with `replicas` replicas), service, and ingress (pointing to `domain`)."

Now when you insert:

```sql
INSERT INTO tenants VALUES
  ('acme-corp', 'acme.example.com', 'enterprise', true, 5);
```

Lynq detects the new row within 30 seconds and provisions:
- Namespace: `acme-corp`
- Deployment: 5 replicas
- Service: `acme-corp-app`
- Ingress: Routes `acme.example.com` to the service

All automatically. No manual steps.

## Common Patterns

### Feature Flags Control Infrastructure

Instead of deploying optional features for everyone, make them data-driven:

```sql
CREATE TABLE feature_flags (
  tenant_id VARCHAR(50),
  feature VARCHAR(50),
  enabled BOOLEAN
);

-- Enable AI assistant for specific customer
INSERT INTO feature_flags VALUES ('acme-corp', 'ai-assistant', true);
```

Your LynqForm template includes conditional logic: if the feature flag exists and is enabled, deploy the AI service. If disabled or absent, skip it.

Suddenly your infrastructure adapts to your feature flags automatically.

### Blue-Green Deployments as a Column

```sql
CREATE TABLE deployments (
  tenant_id VARCHAR(50),
  active_version VARCHAR(10) -- 'blue' or 'green'
);

-- Switch traffic
UPDATE deployments SET active_version = 'green' WHERE tenant_id = 'acme-corp';
```

Your service selector updates to point to the green deployment. Traffic switches within seconds. Roll back by changing the column back to 'blue'.

### Ephemeral Environments with TTL

```sql
-- Create temporary environment
INSERT INTO environments (id, domain, ttl)
VALUES ('demo-acme', 'demo-acme.example.com', NOW() + INTERVAL 7 DAY);

-- Add a database trigger to clean up expired environments
CREATE TRIGGER cleanup_expired
AFTER INSERT OR UPDATE ON environments
BEGIN
  DELETE FROM environments WHERE ttl < NOW();
END;
```

Environments provision on insert and automatically clean up after their TTL. Perfect for demo environments or PR previews.

## Practical Benefits

### For Development

Before, adding a customer meant coordinating across multiple systems. Now it's just a database operation. The same skills you use every day—writing queries, managing transactions—work for infrastructure too.

You can test changes by inserting test records. No need to learn Terraform or maintain separate YAML files.

### For Operations

Your infrastructure state is in the same place as your application state. Audit logs are database logs. Backups include infrastructure configuration. There's no need to reconcile state files or worry about drift—Lynq continuously syncs your database to the cluster.

Rollbacks are straightforward: restore your database, and infrastructure recreates automatically.

### For the Business

Customer onboarding is faster because there's less coordination. Feature rollouts are simpler because they're just database toggles. The gap between what your application knows and what your infrastructure provides shrinks to almost nothing.

## When RecordOps Makes Sense

This pattern works well when:

- You're building a multi-tenant platform where each customer/project needs isolated infrastructure
- You provision infrastructure frequently (multiple times per day)
- Your infrastructure follows your data model closely
- You want less coordination between application logic and infrastructure

It's probably not the right fit if:

- You rarely provision new infrastructure (once a month or less)
- Your infrastructure requires manual approval for every change
- You need deep integration with cloud provider services beyond Kubernetes

And honestly, you can mix approaches. Use GitOps for cluster-level config, RecordOps for per-tenant stacks, and manual processes for critical infrastructure changes. They complement each other.

## Things to Consider

### Your Database Becomes More Critical

With RecordOps, your database isn't just storing application data—it's controlling infrastructure. This means:

- Database availability matters more. If the database is down, you can't provision new infrastructure (though existing infrastructure keeps running)
- Schema migrations affect infrastructure. Test them carefully in staging
- Database permissions become infrastructure permissions. Be thoughtful about who can write to these tables

### Security Model Changes

SQL injection vulnerabilities become infrastructure vulnerabilities. If user input can manipulate your queries, they could potentially trigger unwanted infrastructure changes. Validate inputs carefully and use parameterized queries.

Database credentials need strong protection—they control your cluster.

### You Need to Think About Sync Delays

Lynq syncs your database every 30 seconds by default. That means there's a small delay between when you insert a row and when infrastructure provisions. For most cases, this is fine. But if you need instant provisioning, you'll need to tune the sync interval or reconsider the approach.

## Getting Started

If this resonates with you, here's how to try it:

**1. Identify your infrastructure records**

What database rows should represent infrastructure? Customers? Projects? Deployments? Feature environments?

**2. Add infrastructure columns**

```sql
ALTER TABLE customers
  ADD COLUMN replicas INT DEFAULT 2,
  ADD COLUMN region VARCHAR(20) DEFAULT 'us-east-1',
  ADD COLUMN active BOOLEAN DEFAULT TRUE;
```

**3. Install Lynq and point it at your database**

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

**4. Define your template**

What should each customer get? A namespace? Deployment? Services? Define it once, and it applies to every row.

**5. Test with a single record**

```sql
INSERT INTO customers VALUES ('test', 'test.example.com', 'pro', true, 2);
```

Watch it provision. If something's wrong, delete the row and try again. It's just data.

## Closing Thoughts

RecordOps isn't revolutionary—it's actually pretty obvious once you see it. If your infrastructure maps to your data, why not let your data drive your infrastructure?

This pattern won't replace every infrastructure tool you use. But for the specific problem of provisioning repeated patterns (per-customer stacks, per-project environments), it might make your life simpler.

I built Lynq because I kept solving the same problem: syncing my application's database state with infrastructure state. Eventually I realized they could be the same thing.

If that sounds familiar, maybe RecordOps is worth exploring.

## Learn More

- [How Lynq Works](./how-it-works.md) - Technical architecture
- [Quick Start](./quickstart.md) - Try it in 5 minutes
- [Architecture](./architecture.md) - System design
- [Use Cases](./advanced-use-cases.md) - Real-world patterns

---

Questions? Open an issue on [GitHub](https://github.com/k8s-lynq/lynq/issues) or start a [discussion](https://github.com/k8s-lynq/lynq/discussions).
