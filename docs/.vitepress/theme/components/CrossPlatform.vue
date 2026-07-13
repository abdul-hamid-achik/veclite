<script setup lang="ts">
const surfaces = [
  {
    icon: "💻",
    title: "CLI",
    desc: "Manage databases from the terminal. JSON output with --json for scripting. Read-only commands use lock-free shared reads.",
    points: ["veclite insert", "veclite search --json", "veclite serve --port 8080"],
  },
  {
    icon: "🌐",
    title: "HTTP Server",
    desc: "RESTful JSON API for multi-client access. CORS support, write-heavy workloads via the Go client from any process.",
    points: ["POST /collections", "POST /search", "DELETE /records/:id"],
  },
  {
    icon: "🤖",
    title: "MCP Server",
    desc: "Expose VecLite as tools for AI agents — Claude Code, Cursor, and any MCP-compatible client. Search, insert, and manage via tool calls.",
    points: ["vector_search", "insert_record", "text_search"],
  },
];
</script>

<template>
  <section class="cross vl-section">
    <div class="vl-inner">
      <div class="vl-center cross__head">
        <span class="vl-eyebrow">Access From Anywhere</span>
        <h2 class="vl-h2">Not just a Go library</h2>
        <p class="vl-sub">
          The CLI and HTTP server speak JSON — a stable, cross-language contract
          that future Python and TypeScript drivers will build on.
        </p>
      </div>

      <div class="cross__grid">
        <div v-for="s in surfaces" :key="s.title" class="vl-card cross__card">
          <div class="vl-card__icon">{{ s.icon }}</div>
          <h3 class="vl-card__title">{{ s.title }}</h3>
          <p class="vl-card__body">{{ s.desc }}</p>
          <div class="cross__points">
            <code v-for="p in s.points" :key="p">{{ p }}</code>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.cross__head {
  margin-bottom: 56px;
}

.cross__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px;
}

.cross__card {
  display: flex;
  flex-direction: column;
}

.cross__points {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 20px;
}

.cross__points code {
  font-family: var(--vt-font-family-mono);
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(99, 102, 241, 0.1);
  border: 1px solid rgba(99, 102, 241, 0.2);
  color: var(--vl-text-2);
}

@media (max-width: 800px) {
  .cross__grid {
    grid-template-columns: 1fr;
  }
}
</style>