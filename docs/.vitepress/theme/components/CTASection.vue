<script setup lang="ts">
import { ref } from "vue";
import ProductIcon from "./ProductIcon.vue";

const installCmd = "go get github.com/abdul-hamid-achik/veclite";
const copied = ref(false);

async function copyInstall() {
  try {
    await navigator.clipboard.writeText(installCmd);
    copied.value = true;
    window.setTimeout(() => (copied.value = false), 1800);
  } catch {
    copied.value = false;
  }
}
</script>

<template>
  <section class="cta vl-section">
    <div class="cta__ring cta__ring--one" aria-hidden="true" />
    <div class="cta__ring cta__ring--two" aria-hidden="true" />
    <div v-reveal class="vl-inner cta__inner">
      <span class="vl-eyebrow">Keep search close</span>
      <h2>Put the index beside the code that understands it.</h2>
      <p>
        Start with a four-dimensional demo today. Grow into HNSW, hybrid ranking,
        named spaces, durable writes, and agent memory without changing databases.
      </p>

      <div class="cta__actions">
        <a href="/guide/getting-started" class="vl-btn vl-btn--primary">
          Build with VecLite
          <ProductIcon name="arrow" :size="16" />
        </a>
        <a href="https://github.com/abdul-hamid-achik/veclite" class="vl-btn vl-btn--ghost">
          View on GitHub
        </a>
      </div>

      <button class="cta__install" type="button" @click="copyInstall">
        <span aria-hidden="true">$</span>
        <code>{{ installCmd }}</code>
        <span aria-live="polite">
          <ProductIcon :name="copied ? 'check' : 'copy'" :size="15" />
          {{ copied ? "Copied" : "Copy" }}
        </span>
      </button>

      <footer class="cta__footer">
        <span>VecLite v0.24.0</span>
        <span>Go 1.25+</span>
        <span>MIT licensed</span>
        <a href="/project-status">Compatibility</a>
        <a href="https://github.com/abdul-hamid-achik/veclite/releases">Releases</a>
      </footer>
    </div>
  </section>
</template>

<style scoped>
.cta {
  min-height: 720px;
  display: grid;
  place-items: center;
  overflow: hidden;
  text-align: center;
  background: #11110e;
}

.cta::before {
  content: "";
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(246, 240, 225, 0.027) 1px, transparent 1px),
    linear-gradient(90deg, rgba(246, 240, 225, 0.027) 1px, transparent 1px);
  background-size: 54px 54px;
  mask-image: radial-gradient(circle at center, black, transparent 72%);
  pointer-events: none;
}

.cta__ring {
  position: absolute;
  left: 50%;
  top: 50%;
  border: 1px solid rgba(217, 119, 87, 0.12);
  border-radius: 50%;
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.cta__ring--one {
  width: 680px;
  height: 680px;
}

.cta__ring--two {
  width: 920px;
  height: 920px;
  border-color: rgba(217, 119, 87, 0.065);
}

.cta__inner {
  display: grid;
  justify-items: center;
}

.cta .vl-eyebrow::before {
  display: none;
}

.cta h2 {
  max-width: 860px;
  margin: 0;
  color: var(--vl-text);
  font-size: clamp(44px, 6.2vw, 78px);
  font-weight: 720;
  line-height: 0.98;
  letter-spacing: -0.055em;
}

.cta > .cta__inner > p {
  max-width: 620px;
  margin: 24px 0 0;
  color: var(--vl-text-2);
  font-size: 16px;
  line-height: 1.7;
}

.cta__actions {
  margin-top: 30px;
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px;
}

.cta__install {
  width: min(100%, 560px);
  margin-top: 22px;
  padding: 11px 11px 11px 16px;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  color: var(--vl-text);
  background: rgba(29, 29, 25, 0.82);
  border: 1px solid var(--vl-border-strong);
  border-radius: 10px;
  cursor: pointer;
  text-align: left;
  transition: border-color 180ms ease, transform 180ms ease, background 180ms ease;
}

.cta__install:hover {
  background: var(--vl-surface);
  border-color: #5b5b4e;
}

.cta__install:active {
  transform: scale(0.985);
}

.cta__install > span:first-child {
  color: var(--vl-accent);
  font-family: var(--vl-font-mono);
}

.cta__install code {
  min-width: 0;
  overflow: hidden;
  color: var(--vl-text-2);
  font-family: var(--vl-font-mono);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cta__install > span:last-child {
  padding: 6px 9px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--vl-text-muted);
  background: rgba(246, 240, 225, 0.045);
  border-radius: 6px;
  font-size: 10px;
}

.cta__footer {
  margin-top: 48px;
  padding-top: 22px;
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 9px 18px;
  border-top: 1px solid var(--vl-border);
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 9px;
}

.cta__footer a {
  color: var(--vl-text-2);
  text-decoration: none !important;
}

.cta__footer a:hover {
  color: var(--vl-accent-soft);
}

@media (max-width: 620px) {
  .cta {
    min-height: 650px;
  }

  .cta__actions,
  .cta__actions .vl-btn {
    width: 100%;
  }

  .cta__footer {
    display: grid;
  }
}
</style>
