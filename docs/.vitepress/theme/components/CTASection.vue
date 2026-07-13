<script setup lang="ts">
const links = [
  { label: "Get Started", href: "/guide/getting-started", primary: true },
  { label: "Named Vector Spaces", href: "/guide/named-vector-spaces" },
  { label: "Embedding Strategy", href: "/embeddings" },
  { label: "Durability & WAL", href: "/guide/durability" },
  { label: "GitHub", href: "https://github.com/abdul-hamid-achik/veclite" },
];
</script>

<template>
  <section class="cta vl-section">
    <!-- gradient mesh -->
    <div class="cta__mesh" />
    <div class="vl-glow vl-glow--violet" style="width: 600px; height: 300px; top: 50%; left: 50%; transform: translate(-50%, -50%); opacity: 0.15;" />
    <div class="vl-inner vl-center cta__inner">
      <span class="vl-eyebrow cta__eyebrow">Ready to build?</span>
      <h2 class="vl-h2">Start building with<br /><span class="vl-gradient-text">VecLite</span> today</h2>
      <p class="vl-sub cta__sub">
        One file. No server. HNSW, BM25, hybrid search, named vector spaces,
        and agent memory — all in a Go import.
      </p>
      <div class="cta__links">
        <a
          v-for="l in links"
          :key="l.label"
          :href="l.href"
          class="vl-btn"
          :class="l.primary ? 'vl-btn--primary' : 'vl-btn--ghost'"
        >
          {{ l.label }}
        </a>
      </div>
      <div class="cta__install vl-glass">
        <span class="cta__install-prompt">$</span>
        <code>go get github.com/abdul-hamid-achik/veclite</code>
      </div>
      <p class="cta__footer">MIT Licensed · Go 1.25+ · Single-file · No server required</p>
    </div>
  </section>
</template>

<style scoped>
.cta {
  text-align: center;
  overflow: hidden;
}

.cta__mesh {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse 60% 50% at 50% 50%, rgba(124, 58, 237, 0.08), transparent 60%);
  pointer-events: none;
  z-index: 0;
}

.cta__inner {
  position: relative;
  z-index: 1;
}

.cta__eyebrow {
  justify-content: center;
}

.cta__eyebrow::before {
  display: none;
}

.cta__sub {
  margin-left: auto;
  margin-right: auto;
}

.cta__links {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
  margin-bottom: 28px;
}

.cta__install {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 12px 20px;
  border-radius: var(--vl-radius-sm);
}

.cta__install-prompt {
  color: var(--vl-primary-light);
  font-family: var(--vt-font-family-mono);
  font-size: 14px;
}

.cta__install code {
  font-family: var(--vt-font-family-mono);
  font-size: 14px;
  color: var(--vl-text);
}

.cta__footer {
  margin-top: 24px;
  font-size: 13px;
  color: var(--vl-text-muted);
  letter-spacing: 0.02em;
}
</style>