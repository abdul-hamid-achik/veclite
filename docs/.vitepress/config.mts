import { defineConfig } from "vitepress";

// The docs site is served at the domain root: it is deployed to Vercel via
// vercel.json (the linked .vercel project auto-deploys on push). The base path is
// therefore the VitePress default "/". Do not add a second deploy target.
//
// Custom domain: veclite.dev (deployed to Vercel, auto-deploys on push to main).
const SITE_URL = "https://veclite.dev";
const OG_IMAGE = "/og-image.png";

const ogTitle = "VecLite — Embeddable Vector Database for Go";
const ogDescription =
  "Store vectors, text, and metadata in a single file. Search with HNSW, BM25, hybrid ranking, and multimodal named vector spaces. No database server required.";

const jsonLd = JSON.stringify({
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "VecLite",
  applicationCategory: "DeveloperApplication",
  operatingSystem: "Cross-platform",
  description: ogDescription,
  url: SITE_URL,
  downloadUrl: "https://github.com/abdul-hamid-achik/veclite/releases",
  softwareVersion: "0.24.0",
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
  titleTemplate: "Embeddable Vector Database for Go",
  description: ogDescription,
  cleanUrls: true,
  lastUpdated: true,

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
    ["meta", { name: "theme-color", content: "#7C3AED" }],
    ["meta", { name: "color-scheme", content: "dark light" }],

    // Canonical
    ["link", { rel: "canonical", href: SITE_URL }],

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
    ["meta", { property: "og:title", content: ogTitle }],
    ["meta", { property: "og:description", content: ogDescription }],
    ["meta", { property: "og:url", content: SITE_URL }],
    ["meta", { property: "og:image", content: `${SITE_URL}${OG_IMAGE}` }],
    ["meta", { property: "og:image:width", content: "1200" }],
    ["meta", { property: "og:image:height", content: "630" }],
    ["meta", { property: "og:locale", content: "en_US" }],

    // Twitter Card
    ["meta", { name: "twitter:card", content: "summary_large_image" }],
    ["meta", { name: "twitter:title", content: ogTitle }],
    ["meta", { name: "twitter:description", content: ogDescription }],
    ["meta", { name: "twitter:image", content: `${SITE_URL}${OG_IMAGE}` }],

    // Structured data
    ["script", { type: "application/ld+json" }, jsonLd],
  ],

  themeConfig: {
    logo: "/logo.svg",
    nav: [
      { text: "Guide", link: "/guide/getting-started" },
      { text: "Named Vector Spaces", link: "/guide/named-vector-spaces" },
      { text: "Embeddings", link: "/embeddings" },
      { text: "Status", link: "/project-status" },
      {
        text: "Architecture",
        link: "/adr/0001-embedding-boundary-and-named-vector-spaces",
      },
      { text: "GitHub", link: "https://github.com/abdul-hamid-achik/veclite" },
    ],
    sidebar: [
      {
        text: "Guide",
        items: [
          { text: "Getting Started", link: "/guide/getting-started" },
          { text: "Using VecLite", link: "/guide/using-veclite" },
          { text: "Named Vector Spaces", link: "/guide/named-vector-spaces" },
          { text: "Durability & WAL", link: "/guide/durability" },
          { text: "Go Client", link: "/guide/go-client" },
          { text: "Embedding Strategy", link: "/embeddings" },
          { text: "Project Status", link: "/project-status" },
        ],
      },
      {
        text: "Architecture",
        items: [
          {
            text: "Embedding Boundary",
            link: "/adr/0001-embedding-boundary-and-named-vector-spaces",
          },
        ],
      },
    ],
    search: {
      provider: "local",
    },
    socialLinks: [
      { icon: "github", link: "https://github.com/abdul-hamid-achik/veclite" },
    ],
  },
});