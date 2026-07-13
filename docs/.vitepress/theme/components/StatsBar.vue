<script setup lang="ts">
import ProductIcon from "./ProductIcon.vue";

const proof = [
  {
    icon: "database",
    value: "Portable snapshot",
    label: "One database file to move, inspect, and back up",
  },
  {
    icon: "search",
    value: "HNSW + BM25",
    label: "Semantic and exact retrieval in one collection",
  },
  {
    icon: "layers",
    value: "Four access paths",
    label: "Embedded Go, CLI, HTTP, and MCP",
  },
  {
    icon: "shield",
    value: "WAL recovery",
    label: "Optional crash-safe writes with automatic replay",
  },
];
</script>

<template>
  <section class="proof vl-section--tight" aria-label="Core product characteristics">
    <div class="vl-inner proof__grid">
      <div v-for="item in proof" :key="item.value" class="proof__item">
        <ProductIcon :name="item.icon" :size="18" />
        <span>
          <strong>{{ item.value }}</strong>
          <small>{{ item.label }}</small>
        </span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.proof {
  background: var(--vl-bg-raised);
  border-top: 1px solid var(--vl-border);
  border-bottom: 1px solid var(--vl-border);
}

.proof__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.proof__item {
  min-width: 0;
  padding: 4px 24px;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 13px;
  border-right: 1px solid var(--vl-border);
}

.proof__item:first-child {
  padding-left: 0;
}

.proof__item:last-child {
  padding-right: 0;
  border-right: 0;
}

.proof__item svg {
  color: var(--vl-accent-soft);
}

.proof__item span {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.proof__item strong {
  color: var(--vl-text);
  font-size: 13px;
  font-weight: 680;
}

.proof__item small {
  color: var(--vl-text-muted);
  font-size: 10px;
  line-height: 1.45;
}

@media (max-width: 900px) {
  .proof__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 22px 0;
  }

  .proof__item:nth-child(2) {
    padding-right: 0;
    border-right: 0;
  }

  .proof__item:nth-child(3) {
    padding-left: 0;
  }
}

@media (max-width: 540px) {
  .proof__grid {
    grid-template-columns: 1fr;
    gap: 0;
  }

  .proof__item,
  .proof__item:first-child,
  .proof__item:nth-child(3),
  .proof__item:last-child {
    padding: 15px 0;
    border-right: 0;
    border-bottom: 1px solid var(--vl-border);
  }

  .proof__item:last-child {
    border-bottom: 0;
  }
}
</style>
