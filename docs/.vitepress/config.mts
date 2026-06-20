import { defineConfig } from "vitepress";

// The docs site is served at the domain root: it is deployed to Vercel via
// vercel.json (the linked .vercel project auto-deploys on push). The base path is
// therefore the VitePress default "/". Do not add a second deploy target.
export default defineConfig({
  title: "VecLite",
  description: "Embeddable vector database for Go.",
  cleanUrls: true,
  lastUpdated: true,
  head: [
    ["link", { rel: "icon", type: "image/svg+xml", href: "/favicon.svg" }],
    ["link", { rel: "icon", type: "image/png", sizes: "32x32", href: "/favicon-32x32.png" }],
    ["link", { rel: "icon", type: "image/png", sizes: "16x16", href: "/favicon-16x16.png" }],
    ["link", { rel: "apple-touch-icon", href: "/apple-touch-icon.png" }],
    ["meta", { name: "theme-color", content: "#7C3AED" }],
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
