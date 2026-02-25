import { defineConfig, createContentLoader } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";
import { writeFileSync } from "fs";
import path from "path";

// https://vitepress.dev/reference/site-config
export default withMermaid(
  defineConfig({
    title: "Lynq",
    description: "Infrastructure as Data for Kubernetes - A RecordOps Platform",
    base: "/",
    srcDir: ".",
    ignoreDeadLinks: false,
    appearance: "force-dark", // Force dark mode only

    themeConfig: {
      // https://vitepress.dev/reference/default-theme-config
      logo: "/logo.png",

      nav: [
        { text: "Docs", link: "/quickstart" },
        { text: "Blog", link: "/blog/" },
        {
          text: "Links",
          items: [
            {
              text: "GitHub",
              link: "https://github.com/k8s-lynq/lynq",
            },
            {
              text: "RSS Feed",
              link: "/feed.xml",
            },
          ],
        },
      ],

      sidebar: [
        {
          text: "Getting Started",
          collapsed: false,
          items: [
            { text: "About Lynq", link: "/about-lynq" },
            { text: "Installation", link: "/installation" },
            { text: "Quick Start", link: "/quickstart" },
            { text: "Dashboard", link: "/dashboard" },
          ],
        },
        {
          text: "Core Concepts",
          collapsed: false,
          items: [
            { text: "Infrastructure as Data", link: "/recordops" },
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
            "Lynq is a RecordOps platform that implements Infrastructure as Data for Kubernetes. Transform database records into production-ready infrastructure automatically. No YAML, no CI/CD delays—just data.",
        },
      ],
      [
        "meta",
        {
          name: "keywords",
          content:
            "kubernetes, operator, automation, database-driven, k8s, lynq, multi-tenancy, resource provisioning, recordops, infrastructure as data, infrastructure as code alternative, data-driven infrastructure",
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
          content: "Lynq - Infrastructure as Data for Kubernetes",
        },
      ],
      [
        "meta",
        {
          property: "og:description",
          content:
            "A RecordOps platform that implements Infrastructure as Data for Kubernetes. Turn database records into infrastructure. No YAML files, no CI/CD delays—just data.",
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
          content: "Lynq - Infrastructure as Data for Kubernetes",
        },
      ],
      [
        "meta",
        {
          name: "twitter:description",
          content:
            "A RecordOps platform that implements Infrastructure as Data for Kubernetes. Turn database records into infrastructure. No YAML, no CI/CD delays—just data.",
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

      // RSS feed link
      [
        "link",
        {
          rel: "alternate",
          type: "application/rss+xml",
          title: "Lynq Blog RSS Feed",
          href: "https://lynq.sh/feed.xml",
        },
      ],
    ],

    async buildEnd(siteConfig) {
      const hostname = "https://lynq.sh";
      const postsPerPage = 10;
      const posts = await createContentLoader("blog/*.md").load();

      const sortedPosts = posts
        .filter((post) => post.url !== "/blog/")
        .sort(
          (a, b) =>
            new Date(b.frontmatter.date) - new Date(a.frontmatter.date)
        );

      const totalPages = Math.ceil(sortedPosts.length / postsPerPage);
      const getFeedUrl = (page) =>
        page === 1 ? `${hostname}/feed.xml` : `${hostname}/feed-page-${page}.xml`;
      const getFeedFilename = (page) =>
        page === 1 ? "feed.xml" : `feed-page-${page}.xml`;

      for (let page = 1; page <= totalPages; page++) {
        const startIdx = (page - 1) * postsPerPage;
        const pagePosts = sortedPosts.slice(startIdx, startIdx + postsPerPage);

        const feedItems = pagePosts
          .map((post) => {
            const title = escapeXml(post.frontmatter.title || "Untitled");
            const description = escapeXml(post.frontmatter.description || "");
            const author = escapeXml(post.frontmatter.author || "Lynq Team");
            const link = `${hostname}${post.url}`;
            const pubDate = new Date(post.frontmatter.date).toUTCString();

            return `    <item>
      <title>${title}</title>
      <link>${link}</link>
      <guid>${link}</guid>
      <pubDate>${pubDate}</pubDate>
      <description>${description}</description>
      <author>${author}</author>
    </item>`;
          })
          .join("\n");

        // RFC 5005 pagination links
        const atomLinks = [
          `    <atom:link href="${getFeedUrl(page)}" rel="self" type="application/rss+xml"/>`,
        ];

        if (totalPages > 1) {
          atomLinks.push(
            `    <atom:link href="${getFeedUrl(1)}" rel="first" type="application/rss+xml"/>`
          );
          if (page > 1) {
            atomLinks.push(
              `    <atom:link href="${getFeedUrl(page - 1)}" rel="previous" type="application/rss+xml"/>`
            );
          }
          if (page < totalPages) {
            atomLinks.push(
              `    <atom:link href="${getFeedUrl(page + 1)}" rel="next" type="application/rss+xml"/>`
            );
          }
          atomLinks.push(
            `    <atom:link href="${getFeedUrl(totalPages)}" rel="last" type="application/rss+xml"/>`
          );
        }

        const rss = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>Lynq Blog${totalPages > 1 ? ` (Page ${page}/${totalPages})` : ""}</title>
    <link>${hostname}/blog/</link>
    <description>Insights and lessons learned from building Lynq</description>
    <language>en</language>
${atomLinks.join("\n")}
${feedItems}
  </channel>
</rss>`;

        writeFileSync(path.join(siteConfig.outDir, getFeedFilename(page)), rss);
      }
    },
  })
);

function escapeXml(str) {
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}
