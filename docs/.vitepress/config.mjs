import { defineConfig } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";

// https://vitepress.dev/reference/site-config
export default withMermaid(
  defineConfig({
    title: "Lynq",
    description: "RecordOps Platform - Database-Driven Kubernetes Automation",
    base: "/",
    srcDir: ".",
    ignoreDeadLinks: false,
    appearance: "force-dark", // Force dark mode only

    themeConfig: {
      // https://vitepress.dev/reference/default-theme-config
      logo: "/logo.png",

      nav: [
        { text: "Home", link: "/" },
        { text: "About", link: "/about-lynq" },
        { text: "Quick Start", link: "/quickstart" },
        { text: "Documentation", link: "/architecture" },
        { text: "API", link: "/api" },
      ],

      sidebar: [
        {
          text: "Getting Started",
          collapsed: false,
          items: [
            { text: "About Lynq", link: "/about-lynq" },
            { text: "Installation", link: "/installation" },
            { text: "Quick Start", link: "/quickstart" },
          ],
        },
        {
          text: "Core Concepts",
          collapsed: false,
          items: [
            { text: "RecordOps", link: "/recordops" },
            { text: "How It Works", link: "/how-it-works" },
            { text: "Architecture", link: "/architecture" },
            { text: "API Reference", link: "/api" },
            { text: "Configuration", link: "/configuration" },
            { text: "Datasources", link: "/datasource" },
            {
              text: "Templates",
              collapsed: false,
              items: [
                { text: "Overview", link: "/templates" },
                { text: "🛠️ Form Builder", link: "/template-builder" },
              ],
            },
            {
              text: "Dependencies",
              collapsed: false,
              items: [
                { text: "Overview", link: "/dependencies" },
                { text: "🔍 Visualizer", link: "/dependency-visualizer" },
              ],
            },
            {
              text: "Policies",
              collapsed: false,
              items: [
                { text: "Overview", link: "/policies" },
                { text: "Examples", link: "/policies-examples" },
                { text: "Field-Level Ignore", link: "/field-ignore" },
              ],
            },
          ],
        },
        {
          text: "Advanced Use Cases",
          collapsed: false,
          items: [
            { text: "Overview", link: "/advanced-use-cases" },
            { text: "Custom Domains", link: "/use-case-custom-domains" },
            { text: "Multi-Tier Stack", link: "/use-case-multi-tier" },
            { text: "Blue-Green Deployments", link: "/use-case-blue-green" },
            {
              text: "Database-per-Tenant",
              link: "/use-case-database-per-tenant",
            },
            { text: "Feature Flags", link: "/use-case-feature-flags" },
          ],
        },
        {
          text: "Operations",
          collapsed: false,
          items: [
            {
              text: "Monitoring & Observability",
              link: "/monitoring",
            },
            { text: "Prometheus Queries", link: "/prometheus-queries" },
            { text: "Performance Tuning", link: "/performance" },
            { text: "Security", link: "/security" },
            { text: "Troubleshooting", link: "/troubleshooting" },
            { text: "Alert Runbooks", link: "/alert-runbooks" },
          ],
        },
        {
          text: "Integrations",
          collapsed: false,
          items: [
            {
              text: "Crossplane (Recommended)",
              link: "/integration-crossplane",
            },
            {
              text: "External DNS (Recommended)",
              link: "/integration-external-dns",
            },
            {
              text: "Flux",
              link: "/integration-flux",
            },
            {
              text: "Argo CD",
              link: "/integration-argocd",
            },
          ],
        },
        {
          text: "Development",
          collapsed: false,
          items: [
            {
              text: "Local Development",
              link: "/local-development-minikube",
            },
            { text: "Development Guide", link: "/development" },
            {
              text: "Contributing",
              link: "/contributing-datasource",
            },
            { text: "Roadmap", link: "/roadmap" },
          ],
        },
        {
          text: "Glossary",
          link: "/glossary",
        },
      ],

      search: {
        provider: "local",
      },

      footer: {
        message:
          '<p style="margin-bottom: 12px">Released under the Apache 2.0 License.<br />Built with ❤️ using Kubebuilder, Controller-Runtime, and VitePress.</p>',
        copyright: "Copyright © 2025 Lynq",
      },

      editLink: {
        pattern: "https://github.com/k8s-lynq/lynq/edit/main/docs/:path",
        text: "Edit this page on GitHub",
      },

      lastUpdated: {
        text: "Updated at",
        formatOptions: {
          dateStyle: "full",
          timeStyle: "medium",
        },
      },
    },

    markdown: {
      theme: "github-dark",
      lineNumbers: true,
    },

    mermaid: {
      // Mermaid configuration options
    },

    mermaidPlugin: {
      class: "mermaid my-class", // set additional css classes for parent container
    },

    vue: {
      template: {
        compilerOptions: {
          isCustomElement: () => false,
        },
      },
    },

    vite: {
      optimizeDeps: {
        exclude: [],
      },
    },

    head: [
      // Standard favicon
      ["link", { rel: "icon", type: "image/x-icon", href: "/favicon.ico" }],
      [
        "link",
        { rel: "shortcut icon", type: "image/x-icon", href: "/favicon.ico" },
      ],

      // PNG favicons for different sizes
      [
        "link",
        {
          rel: "icon",
          type: "image/png",
          sizes: "16x16",
          href: "/favicon-16x16.png",
        },
      ],
      [
        "link",
        {
          rel: "icon",
          type: "image/png",
          sizes: "32x32",
          href: "/favicon-32x32.png",
        },
      ],

      // Apple Touch Icon
      [
        "link",
        {
          rel: "apple-touch-icon",
          sizes: "180x180",
          href: "/apple-touch-icon.png",
        },
      ],

      // Android Chrome icons
      [
        "link",
        {
          rel: "icon",
          type: "image/png",
          sizes: "192x192",
          href: "/android-chrome-192x192.png",
        },
      ],
      [
        "link",
        {
          rel: "icon",
          type: "image/png",
          sizes: "512x512",
          href: "/android-chrome-512x512.png",
        },
      ],

      // Web App Manifest
      ["link", { rel: "manifest", href: "/site.webmanifest" }],

      // Theme color for mobile browsers
      ["meta", { name: "theme-color", content: "#1a1a1a" }],

      // Basic SEO
      [
        "meta",
        {
          name: "description",
          content:
            "Lynq is a RecordOps platform that automates database-driven Kubernetes resource provisioning. Transform database records into production-ready infrastructure automatically.",
        },
      ],
      [
        "meta",
        {
          name: "keywords",
          content:
            "kubernetes, operator, automation, database-driven, k8s, lynq, multi-tenancy, resource provisioning, template engine, recordops, record operations, data-driven infrastructure",
        },
      ],
      ["meta", { name: "author", content: "Lynq Contributors" }],

      // OpenGraph (Facebook, LinkedIn, etc.)
      ["meta", { property: "og:type", content: "website" }],
      ["meta", { property: "og:site_name", content: "Lynq" }],
      [
        "meta",
        {
          property: "og:title",
          content: "Lynq - RecordOps Platform for Kubernetes",
        },
      ],
      [
        "meta",
        {
          property: "og:description",
          content:
            "RecordOps platform for Kubernetes. Transform database records into production-ready infrastructure. Real-time, data-driven automation without CI/CD delays.",
        },
      ],
      ["meta", { property: "og:url", content: "https://lynq.sh" }],
      [
        "meta",
        { property: "og:image", content: "https://lynq.sh/og-image.png" },
      ],
      ["meta", { property: "og:image:width", content: "1200" }],
      ["meta", { property: "og:image:height", content: "630" }],
      ["meta", { property: "og:image:alt", content: "Lynq Logo" }],
      ["meta", { property: "og:locale", content: "en_US" }],

      // Twitter Card
      ["meta", { name: "twitter:card", content: "summary_large_image" }],
      [
        "meta",
        {
          name: "twitter:title",
          content: "Lynq - RecordOps Platform for Kubernetes",
        },
      ],
      [
        "meta",
        {
          name: "twitter:description",
          content:
            "RecordOps platform for Kubernetes. Transform database records into production-ready infrastructure. Real-time, data-driven automation without CI/CD delays.",
        },
      ],
      [
        "meta",
        { name: "twitter:image", content: "https://lynq.sh/og-image.png" },
      ],
      ["meta", { name: "twitter:image:alt", content: "Lynq Logo" }],

      // Google site verification
      [
        "meta",
        {
          name: "google-site-verification",
          content: "g7LPr3Wcm6hCm-Lm8iP5KVl11KvPv6Chxpjh3oNKHPw",
        },
      ],
    ],
  })
);
