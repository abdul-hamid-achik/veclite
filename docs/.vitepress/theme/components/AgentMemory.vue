<script setup lang="ts">
const features = [
  {
    icon: "⏱️",
    title: "TTL & Expiration",
    body: "Records expire automatically after a duration. Background cleanup sweeps expired entries on a configurable interval.",
  },
  {
    icon: "📊",
    title: "Importance & Decay",
    body: "Assign 0.0–1.0 importance scores. Apply exponential, linear, or gaussian temporal decay to search rankings.",
  },
  {
    icon: "💬",
    title: "Conversation Memory",
    body: "Session-based turns with threading support. Retrieve conversation context by session, role, or turn number.",
  },
  {
    icon: "🔔",
    title: "Real-time Subscriptions",
    body: "Get notified when new records match a query. Build reactive agent memory that updates as evidence arrives.",
  },
  {
    icon: "🧠",
    title: "Memory Consolidation",
    body: "Cluster and consolidate similar memories automatically. Reduce noise, preserve signal across long sessions.",
  },
  {
    icon: "🎬",
    title: "Episodic Memory",
    body: "Group related memories into coherent episodes. Retrieve by episode, not just by individual record.",
  },
  {
    icon: "🕸️",
    title: "Knowledge Graph",
    body: "Entity-relationship graph with traversal and vector search. Connect records with typed edges and query neighborhoods.",
  },
  {
    icon: "🧹",
    title: "Memory Pressure",
    body: "Automatic eviction with FIFO, LRU, or importance policies when collections exceed capacity.",
  },
];
</script>

<template>
  <section class="memory vl-section">
    <div class="vl-glow vl-glow--indigo" style="width: 500px; height: 500px; top: 10%; left: -10%;" />
    <div class="vl-inner">
      <div class="vl-center memory__head">
        <span class="vl-eyebrow">Agent Memory</span>
        <h2 class="vl-h2">Built for AI agent<br />memory systems</h2>
        <p class="vl-sub">
          VecLite ships first-class features for intelligent storage, retrieval, and
          management of agent memories — temporal awareness, importance scoring,
          and relationship tracking out of the box.
        </p>
      </div>

      <div class="memory__grid">
        <div v-for="(f, i) in features" :key="f.title" class="vl-card memory__card" :style="{ transitionDelay: `${i * 50}ms` }">
          <div class="vl-card__icon">{{ f.icon }}</div>
          <h3 class="vl-card__title">{{ f.title }}</h3>
          <p class="vl-card__body">{{ f.body }}</p>
        </div>
      </div>

      <div class="memory__note vl-glass">
        <div class="memory__note-glow" />
        <span class="vl-tag">RAG-ready</span>
        <p>
          Combine vector search, BM25, hybrid ranking, filters, and temporal decay
          in a single query — the building blocks for retrieval-augmented generation
          and long-lived agent memory without a separate database server.
        </p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.memory {
  background:
    linear-gradient(180deg, rgba(22, 18, 42, 0.4), transparent 500px),
    var(--vl-bg);
}

.memory__head {
  margin-bottom: 56px;
}

.memory__grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 20px;
  position: relative;
  z-index: 1;
}

.memory__card {
  display: flex;
  flex-direction: column;
}

.memory__note {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-top: 48px;
  padding: 28px 32px;
  position: relative;
  z-index: 1;
  overflow: hidden;
}

.memory__note-glow {
  position: absolute;
  top: -50%;
  right: -10%;
  width: 300px;
  height: 300px;
  background: radial-gradient(circle, rgba(168, 85, 247, 0.1), transparent 70%);
  pointer-events: none;
}

.memory__note p {
  font-size: 15px;
  line-height: 1.6;
  color: var(--vl-text-2);
  margin: 0;
  position: relative;
}

.memory__note .vl-tag {
  flex-shrink: 0;
  margin-top: 2px;
}

@media (max-width: 1000px) {
  .memory__grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 600px) {
  .memory__grid {
    grid-template-columns: 1fr;
  }
  .memory__note {
    flex-direction: column;
    gap: 12px;
  }
}
</style>