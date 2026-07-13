<script setup lang="ts">
import { ref, computed } from "vue";

type Tab = "go" | "cli" | "http";

const activeTab = ref<Tab>("go");

const tabs: { id: Tab; label: string; filename: string }[] = [
  { id: "go", label: "Go Library", filename: "main.go" },
  { id: "cli", label: "CLI", filename: "terminal" },
  { id: "http", label: "HTTP API", filename: "curl" },
];

const codeGo = `package main

import (
    "fmt"
    "github.com/abdul-hamid-achik/veclite"
)

func main() {
    db, _ := veclite.Open("vectors.veclite")
    defer db.Close()

    coll, _ := db.CreateCollection("docs",
        veclite.WithDimension(384),
        veclite.WithHNSW(16, 200),
        veclite.WithTextIndex("title", "path"),
    )

    // Store a vector + text + metadata in one record
    vec := make([]float32, 384)
    coll.InsertDocument(vec, "Go is a typed language",
        map[string]any{"title": "Go", "path": "README.md"})

    // Vector search, BM25 text search, or hybrid
    results, _ := coll.HybridSearch(
        queryVec, "typed language",
        veclite.TopK(10),
        veclite.WithVectorWeight(1.0),
        veclite.WithTextWeight(0.5),
    )

    for _, r := range results {
        fmt.Printf("Score: %.4f\\n", r.Score)
    }`;

const codeCli = `# Create a collection with HNSW + text index
veclite create-collection docs \\
  --dimension 384 --hnsw-m 16 --hnsw-ef 200 \\
  --text-index title,path

# Insert a document with vector and metadata
veclite insert docs \\
  --content "Go is a typed language" \\
  --metadata '{"title":"Go","path":"README.md"}'

# Hybrid search (vector + BM25)
veclite hybrid-search docs \\
  --query "typed language" \\
  --vector 0.1,0.2,0.3,...  \\
  --top-k 10 --json

# Serve over HTTP
veclite serve vectors.veclite --port 8080 --cors`;

const codeHttp = `# Create collection
curl -X POST http://localhost:8080/collections \\
  -H "Content-Type: application/json" \\
  -d '{"name":"docs","dimension":384,
       "hnsw":{"m":16,"efConstruction":200}}'

# Insert a document
curl -X POST http://localhost:8080/collections/docs/records \\
  -H "Content-Type: application/json" \\
  -d '{"vector":[0.1,0.2,0.3],
       "content":"Go is a typed language",
       "metadata":{"title":"Go"}}'

# Hybrid search
curl -X POST http://localhost:8080/collections/docs/hybrid-search \\
  -H "Content-Type: application/json" \\
  -d '{"query":"typed language","topK":10}'`;

const currentCode = computed(() => {
  switch (activeTab.value) {
    case "go": return codeGo;
    case "cli": return codeCli;
    case "http": return codeHttp;
  }
});

const currentFilename = computed(() => {
  return tabs.find((t) => t.id === activeTab.value)?.filename || "";
});

function highlightCode(src: string, lang: Tab): string {
  let html = src
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");

  if (lang === "go") {
    html = html.replace(/(\/\/.*)/g, '<span style="color:#6b7280">$1</span>');
    html = html.replace(/(&quot;[^&]*?&quot;|`[^`]*?`)/g, '<span style="color:#a5f3a7">$1</span>');
    html = html.replace(/\b(package|import|func|main|defer|for|range|return|if|else|var|const|type|struct|map|make|nil|true|false)\b/g, '<span style="color:#c4b5fd">$1</span>');
    html = html.replace(/\b(string|int|int32|float32|float64|bool|byte|any)\b/g, '<span style="color:#7dd3fc">$1</span>');
    html = html.replace(/\b(\d+(?:\.\d+)?)\b/g, '<span style="color:#fbbf24">$1</span>');
    html = html.replace(/\.([A-Z][a-zA-Z0-9_]*)\b/g, '.<span style="color:#f0abfc">$1</span>');
  } else if (lang === "cli") {
    html = html.replace(/(#[^\n]*)/g, '<span style="color:#6b7280">$1</span>');
    html = html.replace(/(veclite[\w-]*)/g, '<span style="color:#c4b5fd">$1</span>');
    html = html.replace(/(--[\w-]+)/g, '<span style="color:#7dd3fc">$1</span>');
    html = html.replace(/(&quot;[^&]*?&quot;|'[^']*?')/g, '<span style="color:#a5f3a7">$1</span>');
    html = html.replace(/\b(curl|POST|GET|DELETE|PUT)\b/g, '<span style="color:#f0abfc">$1</span>');
  } else {
    html = html.replace(/(#[^\n]*)/g, '<span style="color:#6b7280">$1</span>');
    html = html.replace(/\b(curl|POST|GET|DELETE|PUT)\b/g, '<span style="color:#f0abfc">$1</span>');
    html = html.replace(/(-[XH]\s+\w+)/g, '<span style="color:#7dd3fc">$1</span>');
    html = html.replace(/(&quot;[^&]*?&quot;|'[^']*?')/g, '<span style="color:#a5f3a7">$1</span>');
  }

  return html;
}

const highlighted = computed(() => highlightCode(currentCode.value, activeTab.value));

const checklist = [
  "Vector, text, and hybrid search in one API",
  "In-memory mode (:memory:) for tests",
  "Thread-safe — concurrent reads + serialized writes",
  "Pluggable embedder: OpenAI, Ollama, local ONNX",
];
</script>

<template>
  <section class="code-section vl-section">
    <div class="vl-glow vl-glow--violet" style="width: 400px; height: 400px; top: 20%; right: -5%;" />
    <div class="vl-inner">
      <div class="code-section__grid">
        <div class="code-section__text">
          <span class="vl-eyebrow">Developer Experience</span>
          <h2 class="vl-h2">Three lines to a<br />working vector database</h2>
          <p class="vl-sub">
            No server to configure. No external dependencies for core storage and search.
            Open a file, create a collection, insert. That's it.
          </p>
          <ul class="code-section__list">
            <li v-for="item in checklist" :key="item">
              <span class="code-section__check">✓</span>
              <span v-html="item.replace(/:memory:/g, '<code>:memory:</code>')"></span>
            </li>
          </ul>
        </div>

        <div class="vl-code code-section__code">
          <!-- tab bar -->
          <div class="code-section__tabs">
            <button
              v-for="tab in tabs"
              :key="tab.id"
              class="code-section__tab"
              :class="{ 'code-section__tab--active': activeTab === tab.id }"
              @click="activeTab = tab.id"
            >
              {{ tab.label }}
            </button>
          </div>
          <!-- code bar with filename -->
          <div class="vl-code__bar">
            <span class="vl-code__dot vl-code__dot--red"></span>
            <span class="vl-code__dot vl-code__dot--yellow"></span>
            <span class="vl-code__dot vl-code__dot--green"></span>
            <span class="vl-code__filename">{{ currentFilename }}</span>
          </div>
          <pre><code v-html="highlighted"></code></pre>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.code-section__grid {
  display: grid;
  grid-template-columns: 0.9fr 1.1fr;
  gap: 48px;
  align-items: center;
  position: relative;
  z-index: 1;
}

.code-section__list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.code-section__list li {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  font-size: 15px;
  color: var(--vl-text-2);
  line-height: 1.5;
}

.code-section__list :deep(code) {
  font-family: var(--vt-font-family-mono);
  font-size: 13px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(168, 85, 247, 0.1);
  color: var(--vl-primary-light);
}

.code-section__check {
  color: #4ade80;
  font-weight: 700;
  flex-shrink: 0;
}

/* tabs */
.code-section__tabs {
  display: flex;
  gap: 4px;
  padding: 6px 8px 0;
  border-bottom: 1px solid var(--vl-border);
  background: rgba(255, 255, 255, 0.02);
}

.code-section__tab {
  padding: 8px 16px;
  border: none;
  background: transparent;
  color: var(--vl-text-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  border-radius: 8px 8px 0 0;
  transition: color 0.2s ease, background 0.2s ease;
  font-family: inherit;
}

.code-section__tab:hover {
  color: var(--vl-text-2);
  background: rgba(255, 255, 255, 0.03);
}

.code-section__tab--active {
  color: var(--vl-text);
  background: rgba(124, 58, 237, 0.1);
  border-bottom: 2px solid var(--vl-primary);
}

.code-section__code pre {
  max-height: 520px;
}

@media (max-width: 860px) {
  .code-section__grid {
    grid-template-columns: 1fr;
    gap: 32px;
  }
}
</style>