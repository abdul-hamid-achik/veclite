<script setup lang="ts">
const features = [
  {
    icon: "⚡",
    title: "HNSW Vector Index",
    body: "Fast approximate nearest neighbor search at 6× brute-force speed with >95% recall. Tune M, efConstruction, and efSearch per collection.",
    span: true,
  },
  {
    icon: "📝",
    title: "BM25 Text Search",
    body: "Full-text ranking over record content and payload fields. Standard BM25 parameters, no extra dependencies.",
  },
  {
    icon: "🔀",
    title: "Hybrid Search + RRF",
    body: "Fuse vector similarity and BM25 with Reciprocal Rank Fusion. Configurable weights for semantic-plus-keyword queries.",
  },
  {
    icon: "🎨",
    title: "Named Vector Spaces",
    body: "Multiple independent embeddings per record — text + image + audio — each with its own dimension, distance metric, and HNSW index.",
    span: true,
  },
  {
    icon: "💾",
    title: "Single-File Storage",
    body: "The entire database is one .veclite file. CRC32 checksums, owner-only permissions on POSIX.",
  },
  {
    icon: "🔒",
    title: "Crash-Safe WAL",
    body: "Optional write-ahead log with per-batch fsync. Replay recovers every completed mutation. Auto-checkpoint at 64 MiB.",
  },
  {
    icon: "🏷️",
    title: "Embedding Profiles",
    body: "First-class type validates dimensions and detects index-invalidating provider/model changes via Compatible checks.",
  },
  {
    icon: "🔎",
    title: "Rich Metadata Filters",
    body: "Equality, range, glob, prefix, logical AND. Timestamp filters for creation, update, access, and expiration.",
    span: true,
  },
  {
    icon: "📦",
    title: "Batch + Streaming",
    body: "Batch inserts for throughput. Stream results via callback without materializing the full set. Pagination with offset/limit.",
  },
];
</script>

<template>
  <section class="features vl-section">
    <div class="vl-inner">
      <div class="vl-center features__head">
        <span class="vl-eyebrow">Capabilities</span>
        <h2 class="vl-h2">Everything you need for<br />local-first vector search</h2>
        <p class="vl-sub">
          Import as a Go library, or drive from any language via CLI, HTTP, and MCP.
          VecLite owns storage, indexing, filtering, and ranking — you own your data.
        </p>
      </div>

      <div class="features__grid">
        <div
          v-for="f in features"
          :key="f.title"
          class="vl-card features__card"
          :class="{ 'features__card--span': f.span }"
        >
          <div class="vl-card__icon">{{ f.icon }}</div>
          <h3 class="vl-card__title">{{ f.title }}</h3>
          <p class="vl-card__body">{{ f.body }}</p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.features__head {
  margin-bottom: 56px;
}

.features__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px;
}

.features__card {
  display: flex;
  flex-direction: column;
}

/* Bento-style: featured cards span 2 columns */
.features__card--span {
  grid-column: span 2;
  background: linear-gradient(135deg, rgba(22, 18, 42, 0.8), rgba(30, 24, 56, 0.6));
}

.features__card--span .vl-card__body {
  max-width: 560px;
}

@media (max-width: 900px) {
  .features__grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .features__card--span {
    grid-column: span 2;
  }
}

@media (max-width: 600px) {
  .features__grid {
    grid-template-columns: 1fr;
  }
  .features__card--span {
    grid-column: span 1;
  }
}
</style>