---
title: "Why I Built Lynq: From Internal Tool to Open Source 👶"
date: 2025-12-17
author: Tim Kang
github: selenehyun
description: The journey of building Lynq - from solving internal provisioning challenges to discovering a new paradigm called Infrastructure as Data.
tags:
  - Lynq
  - Open Source
  - Infrastructure as Data
  - RecordOps
sidebar: false
editLink: false
prev: false
next: false
---

# Why I Built Lynq: From Internal Tool to Open Source 👶

<BlogPostMeta />

What started as a solution to my own Terraform scaling pains became an open source project exploring a different approach to infrastructure management.

This post shares the journey from building an internal "Tenant Operator" to discovering what I now call Infrastructure as Data, including a thought-provoking conversation with Ansible creator Michael DeHaan about the complexity trap in modern infrastructure.

## The Beginning: Hitting Terraform's Limits

In 2023, I was running a SaaS platform and handling per-customer Kubernetes resource provisioning. Deployment, Service, Ingress, ConfigMap... each customer needed almost identical resource structures, with only the customer ID and a few config values being different.

At first, I solved it with Terraform. I abstracted everything cleanly into modules and managed per-customer resources. My customers were business owners, so they could wait a bit for provisioning, and it worked fine for a while.

But as the customer count grew, problems started to show. The time to sync Terraform state and provision resources kept increasing proportionally with the number of customers. Eventually, I'd run `terraform apply`, go do something else, and come back much later to review the plan and apply it.

```bash
# This flow repeated every time
terraform plan -out=tfplan  # ... wait 10 minutes
# (go do something else, come back)
terraform apply tfplan      # ... wait again
```

It was stable and not bad. But isn't reducing these small, repetitive tasks what engineers do best?

"The customer data is already in the database. Why do I have to sync it manually every time?"

That question was the beginning of Lynq.

## The Birth of Tenant Operator

I decided to build a Kubernetes Operator. I named it "Tenant Operator" because the goal at the time was to solve multi-tenant SaaS customer provisioning.

The first version was simple:

1. Periodically query the MySQL database for active customers
2. Create Kubernetes resources for each customer using predefined templates
3. Automatically clean up resources for deactivated customers

```yaml
# Early TenantRegistry (now LynqHub)
apiVersion: kubernetes-tenants.org/v1
kind: TenantRegistry
spec:
  source:
    mysql:
      query: "SELECT id, domain, plan FROM customers WHERE active = 1"
```

After a few weeks of development, the first version was complete, and over 200 customer resources started syncing automatically. When a new customer signed up, all resources were created within seconds. When a contract expired, I just set `is_active` to `false` in the database and resources were cleaned up automatically. No more waiting for `terraform apply`.

## "Could This Be Used Elsewhere?"

While using Tenant Operator, I realized this wasn't just a tool for tenant provisioning. The core pattern was simple: **"Automatically provision Kubernetes resources based on data from somewhere"**

That's when I decided to open source it. This seemed like a problem others were facing too, not just me.

## Refactoring for General Use

After deciding to open source, the first thing I did was make the interface more generic. We were using MySQL, so Tenant Operator only supported MySQL, but I redesigned the datasource interface to accommodate popular databases like PostgreSQL.

The domain-specific terms like "tenant" and "customer" were replaced with more abstract concepts:

| Before | After |
|--------|-------|
| TenantRegistry | LynqHub |
| TenantTemplate | LynqForm |
| Tenant | LynqNode |
| customer_id | uid |
| domain | (generalized via extraValueMappings) |

I started collecting use cases. Beyond the big picture, small ideas came to mind:

- Syncing DB data to ConfigMaps or Secrets
- Using DB as a simple job queue, where inserting data automatically creates Job resources
- Deploying per-node workloads in edge computing based on device lists in a central DB
- Auto-generating individual lab environments in learning platforms based on student registration

Nothing revolutionary, but things I'd thought "this would be nice to integrate with K8s" while working.

The name "Tenant" no longer fit. This tool could link all kinds of entities, not just tenants.

So I chose the name "Lynq," a variation of "link," meaning connecting external data with Kubernetes.

## Infrastructure as Data: A New Paradigm

While developing Lynq, I realized I was using a different approach from traditional Infrastructure as Code (IaC).

**Infrastructure as Code** defines infrastructure as code. Terraform, Pulumi, CDK are prime examples. But IaC has limitations:

- Hard to respond to dynamic requirements (modify code every time a new customer comes?)
- Hard to reflect external system state in real-time
- Inefficient for managing large-scale similar resources

Lynq's approach is different. Define the "structure" of infrastructure in templates, but let data determine "what" to provision.

```
IaC: Code → Infrastructure
IaD: Data + Template → Infrastructure
```

This way, only templates containing repetitive structures need Git version control. The actual repetitive data comes from a dynamically managed database (single source of truth).

I decided to call this approach **Infrastructure as Data (IaD)**, or from an operations perspective, **RecordOps**. Each database record becomes the unit of infrastructure operations.

(Someone might argue that DB schemas and records can also be managed as code, so this is just a subset or variation of IaC. That's a fair point, and I'm still thinking about it.)

## A Conversation with Michael DeHaan

While organizing the Infrastructure as Data concept, I thought of Michael DeHaan, creator of Ansible. I remembered his 2013 writing about modeling infrastructure as "text-based, middle-ground, data-driven policy," so I sent him an email asking for his thoughts on RecordOps.

He replied, but it wasn't what I expected. He'd been away from this space for almost 10 years and was skeptical about the modern infrastructure ecosystem as a whole.

> "YAML becoming a monster is true. There's hardly anything checking it, and it's hard to remember different YAML dialects across tools."

> "Most developers these days spend their time on deployment and editing YAML. Almost all of them would rather be coding, but they don't seem to realize the trap they've walked into."

> "Early 2000s 3-tier architecture was fine. ELBs, load balancers, a caching layer was enough... now there are too many moving parts."

Then he asked a sharp question: **"Is this just adding more complexity to an already complicated ecosystem?"**

Honestly, I'd thought about this too, but I'd been too caught up in my work to properly reflect on it. He said complexity itself has become an "economy." Complexity allows selling "solutions" to that complexity. Places that need maybe 2 IT people have whole teams just running cloud ops, and it's still down a lot.

I agree that Kubernetes is overkill for most companies. But Kubernetes also enables managing large-scale, self-healing infrastructure with minimal staff. At least in my case. Just 2 people have been running over 1,000 containers for more than 5 years, rarely having our sleep interrupted. Without it, I wouldn't have even thought about providing 200+ tenants independently.

Through the conversation, Lynq's goal became clear. Help manage large-scale repetitive infrastructure with minimal complexity. Simpler than before, if possible. YAML being terrible is undeniable, but everyone in the Kubernetes ecosystem is using it somehow anyway.

## Baking in Operational Experience

I baked insights from operating Tenant Operator into Lynq.

Data-driven sync creates various situations. When you need to manually modify already-synced resources. When you need to force sync. When resources created once shouldn't be recreated. Engineers needed to control the full lifecycle of resources to their liking. Monitoring, logging, events. Basic features, but tricky to implement systematically.

The most recent feature I built was [maxSkew](/blog/maxskew-implementation-lessons). It prevents cluster meltdown when a single template change gets applied to hundreds of already-provisioned tenants all at once.

<RolloutAnimation />

Building these safety features one by one brought it to production-ready level.

## Lynq's Vision

Lynq 1.x focused on using MySQL as a data source. There's plenty of room to expand.

### Short-term Goals (v1.x ~ v2.0)

- **Multiple data source support**: PostgreSQL, MongoDB, REST API, GraphQL
- **Advanced rollout strategies**: Canary, Blue-Green deployment support
- **Enhanced observability**: Detailed metrics and tracing

### Long-term Vision

- **Cross-cluster sync**: Consistent provisioning across multiple Kubernetes clusters
- **Bidirectional sync**: Reflect Kubernetes state back to external systems

Ultimately, I want to simplify infrastructure deployment and operations that have become overly complex. So people can say "With Lynq, I don't worry about deployment" or "Lynq handles zero-downtime automatically."

## Closing

Lynq started to solve my own problem. While building it, I found the scope of application was wider than expected, so I decided to share it.

HR systems, CRM, project management tools, IoT platforms... these systems have data that can be connected to infrastructure. Automating that connection is what Lynq does.

If you're facing similar problems, give [Lynq](https://github.com/k8s-lynq/lynq) a try. If you have feedback or ideas, let's talk on GitHub Issues or Discussions.

---

*Lynq is an open source project. Check out the code and contribute on [GitHub](https://github.com/k8s-lynq/lynq).*

<BlogPostFooter />
