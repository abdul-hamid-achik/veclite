import { defineConfig } from "vitepress";

// Base path is "/" for root deploys (Vercel, local preview) and "/veclite/" for
// GitHub Pages project pages. The Pages workflow sets DOCS_BASE=/veclite/.
// Head link hrefs are not auto-prefixed by VitePress, so we prefix them with base.
const base = process.env.DOCS_BASE || "/";

export default defineConfig({
  base,
  title: "VecLite",
  description: "Embeddable vector database for Go.",
  cleanUrls: true,
  lastUpdated: true,
  head: [
    ["link", { rel: "icon", type: "image/svg+xml", href: `${base}favicon.svg` }],
    ["link", { rel: "icon", type: "image/png", sizes: "32x32", href: `${base}favicon-32x32.png` }],
    ["link", { rel: "icon", type: "image/png", sizes: "16x16", href: `${base}favicon-16x16.png` }],
    ["link", { rel: "apple-touch-icon", href: `${base}apple-touch-icon.png` }],
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
