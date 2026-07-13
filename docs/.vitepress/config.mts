import { defineConfig } from "vitepress";
import packageJSON from "../../package.json";

// The docs site is served at the domain root: it is deployed to Vercel via
// vercel.json (the linked .vercel project auto-deploys on push). The base path is
// therefore the VitePress default "/". Do not add a second deploy target.
//
// Custom domain: veclite.dev (deployed to Vercel, auto-deploys on push to main).
const SITE_URL = "https://veclite.dev";
const OG_IMAGE = "/og-image.png";
const VERSION = packageJSON.version;

const ogTitle = "VecLite — Local Vector, Keyword, and Multimodal Search";
const ogDescription =
  "Embed vector, BM25, hybrid, and multimodal search in a Go application, or use VecLite through its CLI, HTTP, and MCP surfaces. Portable local persistence, no separate database service.";

const jsonLd = JSON.stringify({
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "VecLite",
  applicationCategory: "DeveloperApplication",
  operatingSystem: "Cross-platform",
  description: ogDescription,
  url: SITE_URL,
  downloadUrl: "https://github.com/abdul-hamid-achik/veclite/releases",
  softwareVersion: VERSION,
  license: "https://github.com/abdul-hamid-achik/veclite/blob/main/LICENSE",
  offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
  author: { "@type": "Person", name: "Abdul Hamid Achik" },
  sourceOrganization: {
    "@type": "Organization",
    name: "VecLite",
    url: SITE_URL,
  },
});

export default defineConfig({
  title: "VecLite",
  titleTemplate: "Local Search Engine",
  description: ogDescription,
  cleanUrls: true,
  lastUpdated: true,

  transformHead({ pageData }) {
    const relativePath = pageData.relativePath || "index.md";
    const pagePath = relativePath === "index.md"
      ? "/"
      : `/${relativePath.replace(/(?:index)?\.md$/, "").replace(/\/$/, "")}`;
    const canonical = `${SITE_URL}${pagePath}`;
    const isHome = pagePath === "/";
    const title = isHome ? ogTitle : `${pageData.title} — VecLite`;
    const description = pageData.description || pageData.frontmatter.description || ogDescription;

    return [
      ["link", { rel: "canonical", href: canonical }],
      ["meta", { property: "og:title", content: title }],
      ["meta", { property: "og:description", content: description }],
      ["meta", { property: "og:url", content: canonical }],
      ["meta", { name: "twitter:title", content: title }],
      ["meta", { name: "twitter:description", content: description }],
    ];
  },

  sitemap: {
    hostname: SITE_URL,
  },

  head: [
    // Favicons
    ["link", { rel: "icon", type: "image/svg+xml", href: "/favicon.svg" }],
    ["link", { rel: "icon", type: "image/png", sizes: "32x32", href: "/favicon-32x32.png" }],
    ["link", { rel: "icon", type: "image/png", sizes: "16x16", href: "/favicon-16x16.png" }],
    ["link", { rel: "apple-touch-icon", href: "/apple-touch-icon.png" }],

    // Theme + viewport
    ["meta", { name: "theme-color", content: "#12120f" }],
    ["meta", { name: "color-scheme", content: "dark light" }],

    // Keywords
    [
      "meta",
      {
        name: "keywords",
        content:
          "vector database, Go, golang, embeddable, HNSW, BM25, hybrid search, RAG, semantic search, vector search, named vector spaces, multimodal, local-first, single-file, AI agent memory",
      },
    ],

    // Open Graph
    ["meta", { property: "og:type", content: "website" }],
    ["meta", { property: "og:site_name", content: "VecLite" }],
    ["meta", { property: "og:image", content: `${SITE_URL}${OG_IMAGE}` }],
    ["meta", { property: "og:image:width", content: "1200" }],
    ["meta", { property: "og:image:height", content: "630" }],
    ["meta", { property: "og:locale", content: "en_US" }],

    // Twitter Card
    ["meta", { name: "twitter:card", content: "summary_large_image" }],
    ["meta", { name: "twitter:image", content: `${SITE_URL}${OG_IMAGE}` }],

    // Structured data
    ["script", { type: "application/ld+json" }, jsonLd],
  ],

  themeConfig: {
    logo: "/logo.svg",
    nav: [
      {
        text: "Get Started",
        link: "/guide/getting-started",
        activeMatch: "^/guide/(getting-started|interfaces|using-veclite)",
      },
      {
        text: "Guides",
        items: [
          { text: "Search & Ranking", link: "/guide/search" },
          { text: "Named Vector Spaces", link: "/guide/named-vector-spaces" },
          { text: "Agent Memory", link: "/guide/agent-memory" },
          { text: "Embedding Strategy", link: "/embeddings" },
          { text: "Durability & WAL", link: "/guide/durability" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "CLI", link: "/reference/cli" },
          { text: "HTTP API", link: "/reference/http-api" },
          { text: "Go HTTP Client", link: "/guide/go-client" },
          { text: "Go API", link: "https://pkg.go.dev/github.com/abdul-hamid-achik/veclite" },
        ],
      },
      { text: "Examples", link: "https://github.com/abdul-hamid-achik/veclite/tree/main/examples" },
      {
        text: `v${VERSION}`,
        items: [
          { text: "Compatibility", link: "/project-status" },
          { text: "Releases", link: "https://github.com/abdul-hamid-achik/veclite/releases" },
        ],
      },
    ],
    sidebar: [
      {
        text: "Start Here",
        items: [
          { text: "Getting Started", link: "/guide/getting-started" },
          { text: "Choose an Interface", link: "/guide/interfaces" },
          { text: "Core Concepts", link: "/guide/using-veclite" },
        ],
      },
      {
        text: "Search",
        items: [
          { text: "Search & Ranking", link: "/guide/search" },
          { text: "Named Vector Spaces", link: "/guide/named-vector-spaces" },
          { text: "Embedding Strategy", link: "/embeddings" },
        ],
      },
      {
        text: "Build",
        items: [
          { text: "Agent Memory", link: "/guide/agent-memory" },
          { text: "Go HTTP Client", link: "/guide/go-client" },
        ],
      },
      {
        text: "Operate",
        items: [
          { text: "Durability & WAL", link: "/guide/durability" },
          { text: "Compatibility", link: "/project-status" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "CLI Commands", link: "/reference/cli" },
          { text: "HTTP API", link: "/reference/http-api" },
          { text: "Go API", link: "https://pkg.go.dev/github.com/abdul-hamid-achik/veclite" },
        ],
      },
      {
        text: "Architecture Decisions",
        items: [
          {
            text: "Embedding Boundary & Spaces",
            link: "/adr/0001-embedding-boundary-and-named-vector-spaces",
          },
        ],
      },
    ],
    search: {
      provider: "local",
    },
    outline: {
      level: [2, 3],
      label: "On this page",
    },
    editLink: {
      pattern: "https://github.com/abdul-hamid-achik/veclite/edit/main/docs/:path",
      text: "Improve this page on GitHub",
    },
    lastUpdated: {
      text: "Updated",
      formatOptions: {
        dateStyle: "medium",
      },
    },
    socialLinks: [
      { icon: "github", link: "https://github.com/abdul-hamid-achik/veclite" },
    ],
  },
});
