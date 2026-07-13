<script setup lang="ts">
import ProductIcon from "./ProductIcon.vue";
</script>

<template>
  <section class="capabilities vl-section">
    <div class="vl-inner">
      <header class="capabilities__head">
        <div>
          <span class="vl-eyebrow">One retrieval engine</span>
          <h2 class="vl-h2">One record. Every useful way to find it.</h2>
        </div>
        <p class="vl-sub">
          Store the source text, payload, and multiple embeddings together. Query
          by meaning, exact language, metadata, or all three—then return the same
          logical record instead of reconciling separate systems.
        </p>
      </header>

      <div class="capabilities__grid">
        <article v-reveal class="capability capability--retrieval vl-panel">
          <div class="capability__copy">
            <span class="vl-tag">Ranking pipeline</span>
            <h3>Semantic when it helps. Exact when it matters.</h3>
            <p>
              Search HNSW vectors, BM25 text, and payload filters independently—or
              fuse them with Reciprocal Rank Fusion for resilient hybrid retrieval.
            </p>
            <a href="/guide/search">
              Choose a search mode
              <ProductIcon name="arrow" :size="15" />
            </a>
          </div>

          <div class="pipeline" aria-label="Hybrid retrieval pipeline illustration">
            <div class="pipeline__record">
              <span class="pipeline__record-id">record / 1042</span>
              <strong>Crash-safe writes with WAL replay</strong>
              <div>
                <code>vector</code>
                <code>content</code>
                <code>payload</code>
              </div>
            </div>
            <div class="pipeline__lanes">
              <span><ProductIcon name="branch" :size="15" /> HNSW</span>
              <span><ProductIcon name="file" :size="15" /> BM25</span>
              <span><ProductIcon name="filter" :size="15" /> Filters</span>
            </div>
            <div class="pipeline__result">
              <small>fused result</small>
              <strong>guide/durability.md</strong>
              <code>rank 01</code>
            </div>
          </div>
        </article>

        <article v-reveal class="capability capability--spaces vl-panel">
          <div class="vl-icon"><ProductIcon name="layers" :size="21" /></div>
          <span class="vl-tag">Named vector spaces</span>
          <h3>Multimodal without duplicated records.</h3>
          <p>
            Give one item text, image, or audio embeddings. Each space keeps its own
            dimension, metric, profile, and optional HNSW index.
          </p>
          <div class="space-stack" aria-hidden="true">
            <div><span class="space-stack__dot" /> <strong>default</strong><small>text · 1536d</small></div>
            <div><span class="space-stack__dot" /> <strong>image</strong><small>CLIP · 512d</small></div>
            <div><span class="space-stack__dot" /> <strong>audio</strong><small>custom · 768d</small></div>
          </div>
          <a href="/guide/named-vector-spaces">
            Model multimodal data
            <ProductIcon name="arrow" :size="15" />
          </a>
        </article>

        <article v-reveal class="capability capability--durability vl-panel">
          <div class="capability__topline">
            <div class="vl-icon"><ProductIcon name="shield" :size="21" /></div>
            <span class="vl-tag">Persistence</span>
          </div>
          <h3>Portable by default. Crash-safe when you need it.</h3>
          <p>
            Atomic snapshots keep the database easy to move. Enable the optional WAL
            to fsync completed mutations, replay after a crash, and checkpoint automatically.
          </p>
          <div class="durability-line" aria-hidden="true">
            <span>mutation</span><i /><span>WAL</span><i /><span>snapshot</span>
          </div>
          <a href="/guide/durability">
            Pick a durability mode
            <ProductIcon name="arrow" :size="15" />
          </a>
        </article>

        <article v-reveal class="capability capability--profile vl-panel">
          <div class="capability__topline">
            <div class="vl-icon"><ProductIcon name="spark" :size="21" /></div>
            <span class="vl-tag">Embedding profiles</span>
          </div>
          <h3>Catch model drift before it corrupts retrieval.</h3>
          <p>
            Persist provider, model, dimension, distance, normalization, and version.
            Compatibility checks tell your app when an index needs a rebuild.
          </p>
          <pre aria-label="Embedding profile example"><code>ollama / nomic-embed-text
768d · cosine · normalized
version: chunker-v2</code></pre>
          <a href="/embeddings">
            Design an embedding strategy
            <ProductIcon name="arrow" :size="15" />
          </a>
        </article>
      </div>
    </div>
  </section>
</template>

<style scoped>
.capabilities {
  background:
    linear-gradient(180deg, var(--vl-bg-raised), var(--vl-bg) 38%),
    var(--vl-bg);
}

.capabilities__head {
  margin-bottom: 52px;
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(360px, 0.72fr);
  gap: 64px;
  align-items: end;
}

.capabilities__head .vl-sub {
  margin: 0;
}

.capabilities__grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 18px;
}

.capability {
  min-width: 0;
  padding: 30px;
}

.capability h3 {
  margin: 16px 0 0;
  color: var(--vl-text);
  font-size: clamp(22px, 2.4vw, 31px);
  font-weight: 690;
  line-height: 1.12;
  letter-spacing: -0.035em;
}

.capability p {
  margin: 13px 0 0;
  color: var(--vl-text-2);
  font-size: 14px;
  line-height: 1.7;
}

.capability > a,
.capability__copy > a {
  width: fit-content;
  margin-top: 24px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--vl-accent-soft);
  font-size: 12px;
  font-weight: 660;
  text-decoration: none !important;
}

.capability > a:hover,
.capability__copy > a:hover {
  color: var(--vl-text);
}

.capability__topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.capability--retrieval {
  min-height: 450px;
  grid-column: span 8;
  display: grid;
  grid-template-columns: minmax(220px, 0.72fr) minmax(340px, 1.28fr);
  gap: 34px;
  align-items: center;
  background: #1a1a16;
}

.capability--spaces {
  grid-column: span 4;
  grid-row: span 2;
  display: flex;
  flex-direction: column;
  background: #20201b;
}

.capability--spaces .vl-tag {
  width: fit-content;
  margin-top: 24px;
}

.capability--spaces > a {
  margin-top: auto;
  padding-top: 26px;
}

.capability--durability,
.capability--profile {
  min-height: 330px;
  grid-column: span 4;
  display: flex;
  flex-direction: column;
}

.capability--profile {
  grid-column: span 4;
  background: #191915;
}

.capability--durability > a,
.capability--profile > a {
  margin-top: auto;
  padding-top: 22px;
}

.pipeline {
  min-width: 0;
  padding: 18px;
  display: grid;
  gap: 16px;
  position: relative;
  background: #11110f;
  border: 1px solid var(--vl-border-strong);
  border-radius: 13px;
}

.pipeline::before {
  content: "";
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(246, 240, 225, 0.025) 1px, transparent 1px),
    linear-gradient(90deg, rgba(246, 240, 225, 0.025) 1px, transparent 1px);
  background-size: 26px 26px;
  pointer-events: none;
}

.pipeline__record,
.pipeline__result {
  padding: 14px;
  display: grid;
  gap: 6px;
  position: relative;
  z-index: 1;
  background: var(--vl-surface);
  border: 1px solid var(--vl-border);
  border-radius: 9px;
}

.pipeline__record-id,
.pipeline__result small {
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 9px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.pipeline__record strong,
.pipeline__result strong {
  color: var(--vl-text-2);
  font-family: var(--vl-font-mono);
  font-size: 11px;
  font-weight: 560;
}

.pipeline__record div {
  margin-top: 4px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.pipeline__record code,
.pipeline__result code {
  width: fit-content;
  padding: 3px 6px;
  color: var(--vl-accent-soft);
  background: rgba(217, 119, 87, 0.07);
  border-radius: 4px;
  font-family: var(--vl-font-mono);
  font-size: 8px;
}

.pipeline__lanes {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  position: relative;
  z-index: 1;
}

.pipeline__lanes span {
  padding: 10px 7px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--vl-text-2);
  background: var(--vl-surface-2);
  border: 1px solid var(--vl-border);
  border-radius: 8px;
  font-family: var(--vl-font-mono);
  font-size: 9px;
}

.pipeline__lanes svg {
  color: var(--vl-accent);
}

.pipeline__result {
  grid-template-columns: minmax(0, 1fr) auto;
}

.pipeline__result small {
  grid-column: 1 / -1;
}

.pipeline__result code {
  align-self: center;
}

.space-stack {
  margin-top: 26px;
  display: grid;
  gap: 8px;
}

.space-stack > div {
  padding: 12px;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  background: #171713;
  border: 1px solid var(--vl-border);
  border-radius: 8px;
}

.space-stack__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--vl-accent);
}

.space-stack strong,
.space-stack small {
  font-family: var(--vl-font-mono);
  font-size: 9px;
}

.space-stack strong {
  color: var(--vl-text-2);
  font-weight: 560;
}

.space-stack small {
  color: var(--vl-text-muted);
}

.durability-line {
  margin-top: 26px;
  display: flex;
  align-items: center;
  gap: 7px;
}

.durability-line span {
  padding: 6px 8px;
  color: var(--vl-text-muted);
  background: #151512;
  border: 1px solid var(--vl-border);
  border-radius: 6px;
  font-family: var(--vl-font-mono);
  font-size: 8px;
}

.durability-line i {
  height: 1px;
  flex: 1;
  background: var(--vl-accent);
  opacity: 0.55;
}

.capability--profile pre {
  margin: 24px 0 0;
  padding: 12px;
  color: var(--vl-text-muted);
  background: #11110f;
  border: 1px solid var(--vl-border);
  border-radius: 8px;
  font-family: var(--vl-font-mono);
  font-size: 9px;
  line-height: 1.7;
  white-space: pre-wrap;
}

@media (max-width: 1020px) {
  .capability--retrieval {
    grid-column: span 12;
  }

  .capability--spaces,
  .capability--durability,
  .capability--profile {
    grid-column: span 6;
  }
}

@media (max-width: 760px) {
  .capabilities__head {
    grid-template-columns: 1fr;
    gap: 24px;
  }

  .capabilities__grid {
    grid-template-columns: 1fr;
  }

  .capability--retrieval,
  .capability--spaces,
  .capability--durability,
  .capability--profile {
    min-height: 0;
    grid-column: auto;
    grid-row: auto;
  }

  .capability--retrieval {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 480px) {
  .capability {
    padding: 22px;
  }

  .pipeline {
    padding: 12px;
  }

  .pipeline__lanes span {
    display: grid;
    justify-items: center;
  }
}
</style>
