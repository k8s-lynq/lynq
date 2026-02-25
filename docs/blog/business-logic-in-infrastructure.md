---
title: "Is It Okay to Delegate Business Logic to Infrastructure?"
date: 2026-02-25
author: Tim Kang
github: selenehyun
description: Thoughts on delegating business logic to the infrastructure layer, through the lens of Uber embedding Rate Limiting into their service mesh
tags:
  - Infrastructure
  - Platform Engineering
sidebar: false
editLink: false
prev: false
next: false
---

# Is It Okay to Delegate Business Logic to Infrastructure?

<BlogPostMeta />

"Is Rate Limiting business logic, or infrastructure?"

Ask this to most developers and they pause for a second. You can almost see the "Well, isn't it both?" on their face.

Right. It's both. If request limits change based on a customer's billing plan, that's business logic. But inspecting and blocking tens of millions of requests per second in real time? That's infrastructure.

And yet Uber combined the two into a single layer. **Inside the service mesh.**

## When Every Team Did Their Own Thing

In Uber's early microservices environment, Rate Limiting was each team's responsibility. Some teams built custom middleware. Others used Redis-based counters. Some just hardcoded it into their business logic.

Once the number of services hit the thousands, the predictable problems showed up.

Configurations varied across services. Operational overhead scaled linearly with the number of services. Smaller services sometimes had no rate limiting at all. Managing hundreds of Redis clusters was considered, but that would have been its own operational nightmare.

This isn't unique to Uber. When you leave cross-cutting concerns to individual services in a distributed system, consistency degrades as scale grows. Someone's on the latest version, someone else is on a version from a year ago, and someone doesn't even know it exists.

## Uber's Choice: Embedding It in the Service Mesh

[Uber moved Rate Limiting into the service mesh layer.](https://www.uber.com/en-IN/blog/ubers-rate-limiting-system/) The resulting Global Rate Limiting (GRL) system handles 80 million requests per second across more than 1,100 services.

The architecture has three tiers. A local client in each service's sidecar for immediate decisions, zone-level aggregators for metric collection, and regional/global controllers for computing the overall drop ratio.

The core design philosophy was achieving both **low latency on the hot path** and **global consistency** at the same time. Instead of per-service token buckets, the system performs probabilistic dropping based on aggregated load data. It accepts a 2-3 second policy propagation delay in exchange for operational simplicity and system-wide consistency.

The results were impressive. One critical service saw up to 90% improvement in P99.5 latency, and a 15x traffic spike (22K to 367K RPS) was handled without any service outage.

But the interesting part isn't the performance numbers. **Rate limit policies are still defined by each service team.** The service mesh only handles enforcement. "Which service gets what limit" is a business decision. "How to enforce that limit" is infrastructure's job.

## A Similar Pattern: Access Control in the Service Mesh

Uber's Rate Limiting isn't the only example. Istio's AuthorizationPolicy follows the same pattern.

"Can Service A call Service B's `/api/orders`?" is both an architectural decision and a business policy. Traditionally, each service handled authorization through its own middleware. Istio moved this into the service mesh layer. No code changes needed. Declarative YAML. Consistent across the board. Every service gets mTLS-based identity verification, and inter-service communication policies are managed centrally.

The pattern is the same. **Who can access what** is defined by the business/architecture team. **Actually enforcing that access control** is handled by infrastructure. The service mesh does the plumbing. It doesn't replace business logic.

## Another Pattern: Transactional Outbox and Debezium

Here's one more interesting case. Event publishing.

Publishing events to Kafka in a microservices architecture is business logic. "When an order completes, publish an order completion event" is a business requirement. Traditionally, the application code calls the Kafka producer directly.

But this approach has a chronic problem. What if the DB transaction succeeds but the Kafka publish fails? Or the other way around? Events get lost or duplicated, and data consistency across services breaks down.

[Netflix](https://netflixtechblog.com/dblog-a-generic-change-data-capture-framework-69351fb9099b) built their own CDC framework called DBLog to capture real-time data changes across dozens of microservices. [Uber](https://www.uber.com/blog/dbevents-ingestion-framework/) built DBEvents, a CDC system that streams change logs from MySQL and Cassandra to Kafka. [Zalando](https://engineering.zalando.com/posts/2022/02/transactional-outbox-with-aws-lambda-and-dynamodb.html) adopted the Transactional Outbox pattern in their order system to separate synchronous data changes from asynchronous event publishing.

What they all converged on was pushing the act of event publishing itself into infrastructure. The **Transactional Outbox pattern**.

```
[Application] --DB write--> [Outbox Table] --CDC (Debezium, etc.)--> [Kafka]
```

The application processes business logic and writes events to the outbox table within the same transaction. CDC infrastructure (Debezium, or each company's in-house solution) captures the DB change log and publishes it to Kafka.

The pattern here is the same. **"What events to publish" is decided by the application (writing to the outbox table), while "how to reliably deliver them" is handled by infrastructure (CDC).** The application doesn't even need to know Kafka exists. Just write to the database.

The benefits are dramatic. The atomicity of the DB transaction guarantees the reliability of event publishing. All the complex logic around Kafka failure handling, retries, and deduplication disappears from the application code. Infrastructure guarantees at-least-once delivery.

## Separating Definition from Execution

A common pattern emerges across all three cases. **Separating the "definition" and "execution" of business rules.**

| Domain | Business Defines | Infrastructure Executes |
|--------|-----------------|------------------------|
| Rate Limiting | Request limits per plan | Real-time request inspection and blocking |
| Access Control | Inter-service communication policies | mTLS authentication and authorization enforcement |
| Event Publishing | Which events to publish (outbox writes) | Reliable delivery (CDC to Kafka) |

"Definition" belongs to the business. "Execution" is something infrastructure can do better. Consistently, fast, reliably.

If this separation can be done cleanly, delegating execution to infrastructure is a natural choice. Having each service individually implement the "execution" part is actually the less efficient approach.

## But Historically, This Has Failed Before

Honestly, there's an uncomfortable precedent here. **ESB (Enterprise Service Bus).**

In the SOA era of the 2000s, ESB served as the central hub for inter-service communication. Initially it just handled message routing and protocol translation. But over time, business logic crept in, one piece at a time. Data validation, business rules, conditional routing, orchestration.

[Martin Fowler and James Lewis, when introducing the microservices architecture in 2014](https://martinfowler.com/articles/microservices.html), called out the ESB problem like this:

> "Smart endpoints and dumb pipes."

The principle that services should be smart and pipes (infrastructure) should be dumb. The result of putting business logic into ESB is well documented. A massive single point of failure known as the "God Bus." Untestable business logic written in BPEL or XSLT. Every change funneled through the ESB team, creating an organizational bottleneck.

And this pattern keeps repeating. Start putting business logic in the API Gateway, and a few years later you've got thousands of lines of Lua plugins that nobody understands. Different generation, same mistake. You add business logic to infrastructure because it's convenient, one piece at a time, and before you know it the API Gateway has become the new ESB.

So isn't what I'm talking about here just the same mistake all over again?

## Same Mistake, or a Different Call?

Well, for now at least, I think it's different. There is a key distinction.

The reason ESB failed was that the **"definition" of business logic was also placed into infrastructure.** Routing rules, data transformation rules, orchestration logic were all trapped inside ESB configuration files or proprietary DSLs. When the business team wanted to change a rule, they had to file a ticket with the ESB team, and the ESB team modified configurations without understanding the business context.

In contrast, with Uber's GRL or Istio's AuthorizationPolicy, **ownership of the definition stays with the business/service team.** Each service team at Uber defines their own Rate Limit policies. Istio's AuthorizationPolicy is declared in YAML by the service team. Infrastructure just takes that definition and executes it.

Here's the difference:

| | ESB (Failed) | Modern Patterns (Working) |
|---|-------------|--------------------------|
| Definition | Infra team via ESB config | Service/business teams, declaratively |
| Execution | Infrastructure (ESB) | Infrastructure (service mesh, operators) |
| Who changes it | Infra team (bottleneck) | Service teams (autonomous) |
| How it's changed | Proprietary DSL, GUI | Standard YAML, Git-manageable |

**"Delegating execution to infrastructure" and "delegating definition to infrastructure" are completely different things.** The lesson from ESB isn't "don't delegate anything to infrastructure." It's "don't hand over ownership of the definition to infrastructure."

## Cases Where It Still Fails

Of course, separating definition from execution doesn't solve everything. There are cases where delegating execution to infrastructure still fails.

**When the infra team doesn't understand the business context.** At one company, Istio AuthorizationPolicy was set to "deny by default." A new service launch required communication with 12 services, and the platform team missed one. Launch was delayed by 3 days. The platform team's response: "We weren't informed about that dependency."

**When the boundary gets blurry.** Rate limiting starts as a simple numeric cap, then expands to different policies per customer segment, different limits by time of day, per-endpoint exceptions. At some point you start wondering whether it even belongs in infrastructure anymore. As Charity Majors (co-founder of Honeycomb) put it:

> "The most dangerous thing in software is a layer that silently makes decisions about data it doesn't understand."

**When the pool of people who can change the infrastructure shrinks.** People who understand both Kubernetes and the business domain are rare. Once business execution logic lives in infrastructure, the number of people who can modify it drops sharply. This is a very real risk.

## Decision Criteria

So ultimately this isn't a question of "should we or shouldn't we" but rather "is this a more manageable approach for our situation?"

Here are the decision criteria I've distilled from my experience:

**When it makes sense to delegate to infrastructure:**
- It's a cross-cutting concern spanning multiple services
- "What" and "How" can be cleanly separated
- Consistency matters more than per-service customization
- Moving it to infrastructure actually reduces operational complexity

**When it's better to keep it in each service:**
- The execution method itself is core to the business logic (e.g., three-stage payment verification)
- Services need subtly different behaviors
- The infra team can't realistically understand the domain
- Rules change fast enough that they don't fit the infrastructure deployment cycle

## My Experience with Lynq

Building Lynq Operator, I found myself at this exact boundary.

Per-tenant Kubernetes resource provisioning is business logic. "Create 2 Deployments and 1 Service for this tenant" is a business requirement. But I chose to delegate this execution to a Kubernetes Operator.

The database holds business data about which tenants are active, and LynqForm templates declaratively define what resources to create for active tenants. Lynq reads both and turns them into actual Kubernetes resources.

Same pattern as Uber's GRL. Business defines, infrastructure executes.

The benefits were clear. SaaS developers just need to insert a record into the database. They don't need to know Kubernetes. Safety mechanisms like Conflict Policy and Deletion Policy are applied uniformly at the infrastructure layer. Every tenant gets resources provisioned the exact same way, so there's no drift from manual work.

That said, I don't think this is the right call for every situation. If you're managing 10 tenants, a single script is probably more reasonable than building an operator.

## Wrapping Up

Is it okay to delegate business logic to infrastructure?

My answer is <b><u>conditional Yes</u></b>.

If the definition and execution of business rules can be separated, if ownership of the definition stays with the business team, and if consistency and operational efficiency matter more in your situation, then delegating execution to infrastructure is a reasonable choice.

But as the history of ESB teaches, **boundaries blur over time.** The temptation of "let's just add this one more thing..." repeats, and one day you find that infrastructure has become a "God Layer" holding everything the business cares about.

In the end, maybe the real question isn't about infrastructure at all. It's about whether you're putting business logic into infrastructure, or just putting the runtime engine there. And deciding which one it is? That's not something a framework or a best-practices doc can tell you. That's a call that people have to make. People who understand both the business and the infrastructure, who can look at the boundary and say "this is where we draw the line, and here's why."

One thing is clear though: **the decade-old principle that "infrastructure should be dumb" is too outdated to follow blindly.** Service meshes, CDC, Kubernetes Operators. These aren't "smart pipes." They're "reliable executors" that carry out business intent safely and consistently. Where you draw that line is ultimately your call.

<BlogPostFooter />
