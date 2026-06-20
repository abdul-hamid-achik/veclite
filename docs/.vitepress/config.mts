import { defineConfig } from "vitepress";

export default defineConfig({
  title: "VecLite",
  description: "Embeddable vector database for Go.",
  cleanUrls: true,
  lastUpdated: true,
  themeConfig: {
    nav: [
      { text: "Guide", link: "/guide/getting-started" },
      { text: "Embeddings", link: "/embeddings" },
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
          { text: "Embedding Strategy", link: "/embeddings" },
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
