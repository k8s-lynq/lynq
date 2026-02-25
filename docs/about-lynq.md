---
description: "Learn about Lynq, a RecordOps platform that links your data sources to Kubernetes resources. Discover its origin, core features, and what you can build with it."
---

# About Lynq

Hey there! 👋 Let me tell you the story behind Lynq.

## The Name

**Lynq** is pronounced just like "link" - you know, like a hyperlink. Simple, right?

The project was originally called **"TenantOperator"** - yeah, not the catchiest name, I know. I started building it because I had a specific problem: I needed to automatically provision full Kubernetes workloads in real-time whenever new tenants showed up in the database. Not sure how common that use case is, but it was absolutely critical for what I was building.

## How It Evolved

Here's the thing - as I kept working on it, I realized this wasn't just about tenants anymore. What I was really building was a **data-driven automation platform** for Kubernetes. It could do so much more than just tenant management!

So I decided to rename it to something that better captures what it does: **Lynq** - because it links your data sources to your Kubernetes resources. Plus, it sounds way better than "TenantOperator", right?

This eventually evolved into what I now call **RecordOps**—an operational pattern that implements **Infrastructure as Data**. Think GitOps, but with your database instead of Git. When you insert a row, infrastructure provisions. When you update a field, resources reconfigure. When you delete a record, everything cleans up.

It's a shift from Infrastructure as Code to Infrastructure as Data. Instead of writing code to describe infrastructure, you write data to describe state.

The big rename happened in **v1.1.9**, and yes, it introduced breaking changes. But here's the thing - the project was only about two weeks old at that point, and I was literally the only person using it 😂 So no one got hurt in the process!

Going forward though, I promise to be more careful. Any breaking changes will be properly documented in `MIGRATION.md` or `BREAKING_CHANGES.md` files with clear upgrade paths.

Today, Lynq can connect to various data sources (starting with MySQL, PostgreSQL coming soon), and it's packed with policies and flexibility to handle most real-world scenarios I've encountered in production environments.

## What You Can Build With It

I've collected some of my favorite use cases that show what Lynq can really do:

- [**Blue-Green Deployments**](./use-case-blue-green.md): Zero-downtime deployments controlled by database flags
- [**Custom Domains**](./use-case-custom-domains.md): Give each node its own domain, automatically
- [**Database-per-Tenant**](./use-case-database-per-tenant.md): Provision isolated databases on the fly
- [**Feature Flags**](./use-case-feature-flags.md): Toggle features through database records
- [**Multi-Tier Stacks**](./use-case-multi-tier.md): Complex architectures, automatically provisioned

Check out the full [Advanced Use Cases](./advanced-use-cases.md) guide for more inspiration.

:::tip Pro Tip
Honestly? Pair Lynq with [Crossplane](./integration-crossplane.md), and you can manage pretty much any repeatable infrastructure in a data-driven way. It's been a game-changer for me.
:::

## What Makes It Special

### 🎯 Infrastructure as Data
Lynq is a RecordOps platform that implements **Infrastructure as Data**—a paradigm where database records become infrastructure specifications. Instead of Infrastructure as Code (writing Terraform/YAML), you have Infrastructure as Data (writing SQL/database rows).

Your database schema IS your infrastructure API. Change your data, infrastructure follows automatically. No CI/CD pipelines. No coordination between systems.

[Learn more about Infrastructure as Data →](./recordops.md)

### 🔧 Flexible Templates
You get the full power of Go templates plus Sprig functions. I've also added some custom helpers that I found myself needing over and over (like proper host extraction from URLs, SHA1 hashing for resource names, etc.).

### 📊 Smart Policies
This one took me a while to get right. You can control exactly how resources are created, updated, and deleted. Want something created once and never touched again? Done. Need to force-take ownership of a conflicting resource? You got it.

### 🔍 Observable by Default
I'm a big believer in observability. Every reconciliation, every error, every state change - it's all tracked. Prometheus metrics, detailed events, structured logs. Because when things go wrong at 2 AM, you'll thank me.

## Getting Started

Ready to try it? Here's where to go:

1. [**Installation**](./installation.md): Get it running with Helm in 5 minutes
2. [**Quick Start**](./quickstart.md): Deploy your first automated node
3. [**Architecture**](./architecture.md): Understand how it works under the hood
4. [**API Reference**](./api.md): Deep dive into the CRDs

## Community & Support

I'm actively maintaining this project, and I'd love to hear from you:

- **GitHub**: [https://github.com/k8s-lynq/lynq](https://github.com/k8s-lynq/lynq)
- **Report Issues**: [GitHub Issues](https://github.com/k8s-lynq/lynq/issues)
- **Contribute**: [CONTRIBUTING.md](https://github.com/k8s-lynq/lynq/blob/main/CONTRIBUTING.md)

### Help Me Make It Better

I'm constantly thinking about better ways to visualize how Lynq works. I know some of the concepts might feel unfamiliar at first - database-driven Kubernetes automation isn't exactly mainstream (yet!). So I'm always brainstorming new diagrams, interactive tools, and visualizations to make things clearer.

Got an idea for a visualization that would help you understand something better? Maybe a diagram showing the reconciliation flow? An interactive template builder? Anything! Please open a [GitHub Issue](https://github.com/k8s-lynq/lynq/issues) and share your thoughts. I'd genuinely love to hear what would make the learning curve smoother for you.

:::tip Work in Progress
I recently renamed everything from "Tenant" terminology to the new Lynq naming (Hub, Form, Node). There might still be some old "Tenant" references lurking in examples or comments that I missed.

If you spot any, please open an issue! I'll fix it right away.
:::

## License

Lynq is released under the Apache License 2.0. Free to use, free to modify, free to distribute.

---

⭐ **If Lynq helps you, I'd really appreciate a [star on GitHub](https://github.com/k8s-lynq/lynq)!**
