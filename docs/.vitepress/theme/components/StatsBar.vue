<script setup lang="ts">
const stats = [
  { value: "6×", label: "Faster search with HNSW" },
  { value: ">95%", label: "Recall vs brute force" },
  { value: "1 file", label: "Single-file persistence" },
  { value: "0", label: "External deps for core" },
];
</script>

<template>
  <section class="stats vl-section--tight">
    <div class="vl-inner">
      <div class="stats__grid">
        <div v-for="(s, i) in stats" :key="s.label" class="stats__item" :style="{ transitionDelay: `${i * 80}ms` }">
          <span class="stats__value vl-gradient-text">{{ s.value }}</span>
          <span class="stats__label">{{ s.label }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.stats {
  border-top: 1px solid var(--vl-border);
  border-bottom: 1px solid var(--vl-border);
  background: rgba(22, 18, 42, 0.3);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

.stats__grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 24px;
}

.stats__item {
  text-align: center;
  padding: 16px 0;
  position: relative;
}

/* divider between items */
.stats__item:not(:last-child)::after {
  content: "";
  position: absolute;
  right: 0;
  top: 25%;
  height: 50%;
  width: 1px;
  background: var(--vl-border);
}

.stats__value {
  display: block;
  font-size: clamp(28px, 4vw, 42px);
  font-weight: 800;
  letter-spacing: -0.02em;
  line-height: 1.1;
}

.stats__label {
  display: block;
  margin-top: 8px;
  font-size: 14px;
  color: var(--vl-text-muted);
  line-height: 1.4;
}

@media (max-width: 768px) {
  .stats__grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 32px 16px;
  }
  .stats__item:nth-child(2)::after {
    display: none;
  }
}
</style>