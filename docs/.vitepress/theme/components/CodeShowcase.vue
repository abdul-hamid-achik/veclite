<script setup lang="ts">
import { computed, ref } from "vue";
import ProductIcon from "./ProductIcon.vue";

type Surface = "go" | "cli" | "http";

const active = ref<Surface>("go");
const copied = ref(false);

const examples: Array<{
  id: Surface;
  label: string;
  filename: string;
  note: string;
  code: string;
}> = [
  {
    id: "go",
    label: "Embedded Go",
    filename: "main.go",
    note: "Runs in-process with no server",
    code: `package main

import (
    "fmt"
    "log"

    "github.com/abdul-hamid-achik/veclite"
)

func main() {
    db, err := veclite.Open(":memory:") // or "search.veclite"
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    docs := db.Collection("docs")
    _, err = docs.Insert(
        []float32{0.1, 0.2, 0.3, 0.4},
        map[string]any{"file": "README.md"},
    )
    if err != nil {
        log.Fatal(err)
    }

    results, err := docs.Search(
        []float32{0.15, 0.25, 0.35, 0.45},
        veclite.TopK(1),
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(results[0].Record.Payload["file"])
}`,
  },
  {
    id: "cli",
    label: "CLI + JSON",
    filename: "terminal",
    note: "Script the same database from any language",
    code: `# Create a four-dimensional HNSW collection
veclite create-collection demo.veclite docs \\
  --dimension=4 --hnsw --json

# Insert one vector and its payload
veclite insert demo.veclite docs \\
  --vector='[0.1,0.2,0.3,0.4]' \\
  --payload='{"file":"README.md"}' --json

# Search and receive machine-readable JSON
veclite search demo.veclite docs \\
  --query='[0.15,0.25,0.35,0.45]' \\
  --top-k=1 --json`,
  },
  {
    id: "http",
    label: "HTTP API",
    filename: "curl",
    note: "Use one writer process for multi-client access",
    code: `# Start the local JSON server
veclite serve demo.veclite --port=8080

# Create a collection
curl -X POST localhost:8080/collections \\
  -H 'Content-Type: application/json' \\
  -d '{"name":"docs","dimension":4,"hnsw":true}'

# Insert a vector
curl -X POST localhost:8080/collections/docs/vectors \\
  -H 'Content-Type: application/json' \\
  -d '{"vector":[0.1,0.2,0.3,0.4],
       "payload":{"file":"README.md"}}'

# Search it
curl -X POST localhost:8080/collections/docs/search \\
  -H 'Content-Type: application/json' \\
  -d '{"query":[0.15,0.25,0.35,0.45],"top_k":1}'`,
  },
];

const current = computed(() => examples.find((example) => example.id === active.value) ?? examples[0]);

async function copyCode() {
  try {
    await navigator.clipboard.writeText(current.value.code);
    copied.value = true;
    window.setTimeout(() => (copied.value = false), 1800);
  } catch {
    copied.value = false;
  }
}
</script>

<template>
  <section class="quickstart vl-section">
    <div class="vl-inner quickstart__grid">
      <div v-reveal class="quickstart__copy">
        <span class="vl-eyebrow">A working result, first</span>
        <h2 class="vl-h2">From import to nearest match in one file.</h2>
        <p class="vl-sub">
          Start in memory, point the same code at a file when you want persistence,
          and add HNSW, BM25, filters, or a WAL only when the workload calls for them.
        </p>

        <ol class="quickstart__steps">
          <li>
            <span>01</span>
            <div><strong>Open</strong><small>Use <code>:memory:</code> or a database path.</small></div>
          </li>
          <li>
            <span>02</span>
            <div><strong>Insert</strong><small>Keep vectors and payload data together.</small></div>
          </li>
          <li>
            <span>03</span>
            <div><strong>Search</strong><small>Return records, not detached vector IDs.</small></div>
          </li>
        </ol>

        <div class="quickstart__links">
          <a class="vl-btn vl-btn--primary" href="/guide/getting-started">
            Follow the quickstart
            <ProductIcon name="arrow" :size="16" />
          </a>
          <a class="vl-btn vl-btn--ghost" href="https://github.com/abdul-hamid-achik/veclite/tree/main/examples">
            Browse examples
          </a>
        </div>
      </div>

      <div v-reveal class="quickstart__code vl-code">
        <div class="quickstart__tabs" role="tablist" aria-label="VecLite interface examples">
          <button
            v-for="example in examples"
            :id="`surface-tab-${example.id}`"
            :key="example.id"
            type="button"
            role="tab"
            :aria-selected="active === example.id"
            :aria-controls="`surface-panel-${example.id}`"
            :class="{ 'is-active': active === example.id }"
            @click="active = example.id"
          >
            {{ example.label }}
          </button>
        </div>
        <div class="vl-code__bar">
          <span class="vl-code__dot" />
          <span class="vl-code__dot" />
          <span class="vl-code__dot" />
          <span class="vl-code__filename">{{ current.filename }}</span>
          <button type="button" @click="copyCode">
            <ProductIcon :name="copied ? 'check' : 'copy'" :size="14" />
            <span aria-live="polite">{{ copied ? "Copied" : "Copy" }}</span>
          </button>
        </div>
        <div
          :id="`surface-panel-${active}`"
          role="tabpanel"
          :aria-labelledby="`surface-tab-${active}`"
        >
          <pre><code>{{ current.code }}</code></pre>
          <div class="quickstart__code-note">
            <span class="quickstart__code-status" />
            {{ current.note }}
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.quickstart {
  background: var(--vl-bg);
  border-bottom: 1px solid var(--vl-border);
}

.quickstart__grid {
  display: grid;
  grid-template-columns: minmax(0, 0.76fr) minmax(520px, 1.24fr);
  gap: clamp(48px, 7vw, 88px);
  align-items: center;
}

.quickstart__steps {
  margin: 38px 0 0;
  padding: 0;
  display: grid;
  list-style: none;
  border-top: 1px solid var(--vl-border);
}

.quickstart__steps li {
  padding: 16px 0;
  display: grid;
  grid-template-columns: 38px 1fr;
  gap: 12px;
  border-bottom: 1px solid var(--vl-border);
}

.quickstart__steps li > span {
  color: var(--vl-accent);
  font-family: var(--vl-font-mono);
  font-size: 10px;
}

.quickstart__steps div {
  display: grid;
  gap: 3px;
}

.quickstart__steps strong {
  color: var(--vl-text);
  font-size: 13px;
  font-weight: 680;
}

.quickstart__steps small {
  color: var(--vl-text-muted);
  font-size: 11px;
  line-height: 1.5;
}

.quickstart__steps code {
  color: var(--vl-accent-soft);
  font-family: var(--vl-font-mono);
}

.quickstart__links {
  margin-top: 28px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.quickstart__code {
  min-width: 0;
  box-shadow: 0 26px 70px rgba(5, 5, 4, 0.28);
}

.quickstart__tabs {
  padding: 8px 8px 0;
  display: flex;
  gap: 3px;
  background: var(--vl-surface);
  border-bottom: 1px solid var(--vl-border);
}

.quickstart__tabs button {
  padding: 9px 12px;
  color: var(--vl-text-muted);
  background: transparent;
  border: 0;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  font-family: inherit;
  font-size: 11px;
  font-weight: 650;
  transition: color 160ms ease, border-color 160ms ease, transform 160ms ease;
}

.quickstart__tabs button:hover {
  color: var(--vl-text);
}

.quickstart__tabs button:active {
  transform: scale(0.97);
}

.quickstart__tabs button.is-active {
  color: var(--vl-text);
  border-bottom-color: var(--vl-accent);
}

.quickstart__code .vl-code__bar button {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--vl-text-muted);
  background: transparent;
  border: 0;
  cursor: pointer;
  font-family: inherit;
  font-size: 10px;
}

.quickstart__code .vl-code__bar button:hover {
  color: var(--vl-text);
}

.quickstart__code pre {
  height: 492px;
  max-height: 58dvh;
}

.quickstart__code-note {
  min-height: 40px;
  padding: 0 15px;
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--vl-text-muted);
  background: var(--vl-surface);
  border-top: 1px solid var(--vl-border);
  font-size: 10px;
}

.quickstart__code-status {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--vl-signal);
}

@media (max-width: 960px) {
  .quickstart__grid {
    grid-template-columns: 1fr;
  }

  .quickstart__copy {
    max-width: 720px;
  }
}

@media (max-width: 560px) {
  .quickstart__links .vl-btn {
    width: 100%;
  }

  .quickstart__tabs {
    overflow-x: auto;
  }

  .quickstart__tabs button {
    white-space: nowrap;
  }

  .quickstart__code pre {
    height: 430px;
    max-height: none;
    font-size: 10px;
  }
}
</style>
