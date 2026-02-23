---
title: "In the Age of AI Agents, How Should Infrastructure Change?"
date: 2026-02-23
author: Tim Kang
github: selenehyun
description: The era of AI Agents autonomously managing infrastructure is coming. But do we have an answer to the question, 'When AI decides, who executes?'
tags:
  - Agentic AI
  - AI Infrastructure
  - Infrastructure as Data
sidebar: false
editLink: false
prev: false
next: false
---

# In the Age of AI Agents, How Should Infrastructure Change?

<BlogPostMeta />

2026 is supposedly the year AI Agents start managing infrastructure autonomously.

[Gartner](https://www.gartner.com/en/newsroom/press-releases/2025-08-26-gartner-predicts-40-percent-of-enterprise-apps-will-feature-task-specific-ai-agents-by-2026-up-from-less-than-5-percent-in-2025) predicts that 40% of enterprise applications will embed AI Agents by 2026, and [Deloitte](https://www.deloitte.com/us/en/insights/topics/technology-management/tech-trends/2026/agentic-ai-strategy.html) forecasts that Agentic AI will account for 10-15% of IT spending. Terms like "AI SRE" and "autonomous infrastructure" are everywhere in conference keynotes.

But one question keeps nagging at me.

**When AI decides, who executes?**

When an AI Agent determines "we need to provision 3 GPU instances for Tenant A," how does that decision become actual infrastructure? Does the Agent call the Kubernetes API directly? Commit to Git? Something else?

I've been building [Lynq](/), a Kubernetes operator that provisions resources based on database records. Recently I connected it with an AI Agent, and it opened up some interesting possibilities. This post shares what I learned.

## Limitations of Current Approaches

Two approaches come to mind for AI Agents managing infrastructure.

### 1. Agent Calls APIs Directly

The obvious approach. Give the AI Agent permission to call Kubernetes APIs and let it create resources as needed. Anthropic's MCP (Model Context Protocol) has made this even easier.

But security researchers are raising red flags:

> "APIs will become the most valuable and vulnerable element of digital infrastructure. As AI agents begin exchanging data and performing actions independently, API traffic will surge beyond human oversight, exposing new pathways for exploitation."
>
> [SecurityWeek: Cyber Insights 2026](https://www.securityweek.com/cyber-insights-2026-api-security/)

The 2025 Huntress report shows NHI (Non-Human Identity) compromise as the fastest-growing attack vector. API keys hardcoded in config files, credentials left in Git repositories. One compromised Agent credential exposes everything that Agent can do.

The bigger problem is **Shadow MCP**. MCP servers deployed without IT approval are becoming new attack surfaces. Direct API calls from AI Agents amplify these risks.

### 2. Use GitOps

The AI commits to a Git repository, and ArgoCD or Flux syncs the changes to the cluster. Change history stays in Git, so auditing is possible.

But GitOps struggles with **dynamic infrastructure**:

> "GitOps assumes that infrastructure is defined in Git ahead of time, but ephemeral environments are dynamic by nature. You don't want to push a new commit just to create a preview stack."
>
> [Firefly: Infrastructure Orchestration Guide](https://www.firefly.ai/academy/beyond-provisioning-a-2025-guide-to-infrastructure-orchestration-with-iac-and-gitops)

When AI Agents make real-time decisions, creating a Git commit, opening a PR, and merging for every change is unrealistic. Fine with 10 tenants. With 1,000 tenants being dynamically created and deleted, the Git repository becomes a graveyard of commit history.

And GitOps's **eventual consistency** clashes with real-time needs. What if the AI decides resources are needed "right now," but has to wait for the Git sync cycle?

## Connecting AI Agent + MCP + Lynq

I tried connecting an AI Agent to Lynq.

The scenario: an admin requests "expand resources for Tenant X" via chat. The AI Agent analyzes the request and approves or rejects it. If approved, the Agent INSERTs a new tenant record into MySQL via MCP.

Lynq picks it up from there.

```
[Admin] --chat--> [AI Agent] --MCP--> [MySQL] <--sync-- [Lynq] --> [K8s Resources]
```

Lynq polls MySQL periodically. When it finds new records, it creates Kubernetes resources according to predefined templates. The Agent records its "intent" in the database. Lynq handles the actual infrastructure creation.

You might ask, "Why put a database in the middle? Just another layer."

**Lynq isn't just a DB-to-K8s mirroring tool.** I designed it to validate and control before reflecting database records to Kubernetes:

- Is this change intentional? (Is another Operator already managing this resource?)
- Are too many changes happening at once?
- How should conflicts be handled?

Lynq's policy system creates a "gateway" between DB records and K8s resources. You can reflect everything unconditionally, or block based on conditions and wait for admin intervention. All change attempts are recorded as events.

## What This Architecture Enables

Running this setup revealed why the database layer matters for AI-driven infrastructure.

### 1. Separation of Concerns: AI is What, Lynq is How

The AI Agent doesn't need to know Kubernetes. It focuses on business logic: "Does this tenant need a GPU?", "Is the resource quota appropriate?"

Lynq handles the Kubernetes complexity. Deployment readiness probes, Service port mappings, all defined once in templates.

This keeps AI Agent development and infrastructure template management independent. Changing Agent logic doesn't touch Kubernetes manifests. Changing templates doesn't touch the Agent.

### 2. Auditability: Decisions Leave a Trail

Every INSERT becomes an audit log.

- When was the tenant added
- What values was it created with
- Who (or which Agent) made the decision

With direct API calls, tracking this history is surprisingly hard. With a database in the middle, every Agent "decision" is a record.

When something breaks, instead of wondering "What did the AI do?", you can just query the database.

### 3. Policy-Based Control: A Gateway, Not Just Mirroring

"If you're just reflecting DB changes to K8s, isn't that just adding another layer?"

When Agents call the Kubernetes API directly, bad decisions hit the cluster immediately. Accidentally deleting a production Deployment. Rolling out all Pods with a wrong image. It happens.

I built Lynq with **policy-based control** between DB records and K8s resources:

- **Conflict Detection** (`conflictPolicy`): If another Operator or person already manages a resource, Lynq alerts the admin instead of blindly overwriting. You choose whether to force it or back off.
- **Gradual Rollout** (`maxSkew`): Prevents template changes from hitting hundreds of tenants at once. Apply 10 at a time, stop if problems arise.
- **Data Protection** (`deletionPolicy`): Even if a DB record is accidentally deleted, critical resources like PVCs can be protected from automatic deletion.

Every change attempt (successful, failed, or blocked) becomes a Kubernetes event. You can always trace what happened. (Monitoring still has gaps to fill. But the structure for tracking is there.)

## Anticipated Objections

A few objections might come to mind.

### "Why not just call the K8s API directly?"

More direct, sure. Sometimes direct is better.

But [Gartner](https://www.gartner.com/en/newsroom/press-releases/2025-06-25-gartner-predicts-over-40-percent-of-agentic-ai-projects-will-be-canceled-by-end-of-2027) predicts over 40% of Agentic AI projects will be canceled by 2027, with integration problems as a major reason. Direct API calls are fast, but permission management, audit trails, and failure isolation get complicated.

The Kubernetes API is general-purpose. Giving an AI Agent `kubectl`-level permissions means it can do *everything*. A database schema limits what the Agent can do. Hmm, whether that tradeoff makes sense depends on your situation.

### "Isn't GitOps enough?"

For declarative infrastructure, GitOps works well. Define desired state in Git, sync continuously. Many scenarios are covered.

But tenants created and deleted in real-time, resources scaling on demand—Git commits for every change creates too much overhead.

[Firefly's guide](https://www.firefly.ai/academy/beyond-provisioning-a-2025-guide-to-infrastructure-orchestration-with-iac-and-gitops) notes the same: GitOps's "eventual consistency" can clash with dynamic environments.

That said, GitOps and data-driven approaches aren't mutually exclusive. I version-control templates (LynqForm) in Git, and manage dynamic data (tenant lists) in a database. Structure in code, data in the database.

### "Won't the database become a bottleneck?"

Hmm, for real-time AI inference needing millisecond responses? Yes, probably.

Lynq is built for **provisioning**, not **real-time response**. Whether creating a tenant takes 5 seconds or 30 seconds doesn't matter. The 30-second polling interval works fine for this use case.

For real-time inference routing or autoscaling, tools like KEDA or Knative make more sense. Lynq's problem space is "declaratively managing infrastructure foundations based on data."

## Open Questions

Some things crystallized through this. Others didn't.

**Separating decision and execution** worked well here. The AI Agent deciding what, Lynq handling how. Easier to manage, easier to debug. But would it work for every scenario? Hmm, probably not.

**Database as middle layer** served as a "contract" between AI intent and infrastructure state. Schema as interface, records as contracts. Well, message queues or event streams might fit better for some use cases.

**Safety mechanisms** have to live somewhere when AI manages infrastructure. Lynq's policy system gave me a natural place to put them. Other architectures would need different solutions.

## Closing

I'm not claiming Lynq is *the* answer for AI infrastructure. But connecting it with an AI Agent showed me one way to answer "When AI decides, who executes?"

In the age of AI Agents, some kind of "execution layer" seems necessary. Agents decide, something executes. Whether that something is direct API calls, Git commits, or database + operator... well, depends on the context.

If you're thinking about similar problems or have tried different approaches, I'd like to hear about it on [GitHub Discussions](https://github.com/k8s-lynq/lynq/discussions).

<BlogPostFooter />
