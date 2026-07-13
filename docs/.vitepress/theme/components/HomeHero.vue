<script setup lang="ts">
import { ref, onMounted } from "vue";

const copied = ref(false);
const installCmd = "go get github.com/abdul-hamid-achik/veclite";

async function copyInstall() {
  try {
    await navigator.clipboard.writeText(installCmd);
    copied.value = true;
    setTimeout(() => (copied.value = false), 2000);
  } catch {
    /* clipboard unavailable */
  }
}

const techBadges = [
  "HNSW", "BM25", "Hybrid Search", "Named Vector Spaces", "RRF Fusion",
  "WAL Durability", "Embedding Profiles", "Agent Memory", "MCP Server",
  "Single-File", "CLI", "HTTP API", "Knowledge Graph", "Episodic Memory",
  "TTL", "Importance Decay", "Subscriptions", "Consolidation",
];

// duplicate for seamless marquee loop
const marqueeBadges = [...techBadges, ...techBadges];
</script>

<template>
  <section class="hero">
    <!-- gradient mesh background -->
    <div class="hero__mesh" />
    <!-- ambient glows -->
    <div class="vl-glow vl-glow--violet" style="width: 520px; height: 520px; top: -100px; left: 8%;" />
    <div class="vl-glow vl-glow--indigo" style="width: 400px; height: 400px; bottom: -80px; right: 5%;" />
    <div class="vl-glow vl-glow--violet" style="width: 300px; height: 300px; top: 40%; left: 45%; opacity: 0.12;" />

    <!-- floating particles -->
    <div class="hero__particles">
      <span v-for="i in 12" :key="i" class="hero__particle" :style="{ '--i': i, '--x': `${(i * 37) % 100}%`, '--d': `${i * 0.3}s` }" />
    </div>

    <div class="hero__inner">
      <!-- animated SVG: one record fanning into named vector spaces -->
      <div class="hero__viz">
        <svg viewBox="0 0 520 360" class="hero__svg" aria-hidden="true">
          <defs>
            <linearGradient id="hero-line" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stop-color="#6366f1" />
              <stop offset="0.5" stop-color="#7c3aed" />
              <stop offset="1" stop-color="#a855f7" />
            </linearGradient>
            <radialGradient id="hero-node" cx="50%" cy="50%" r="50%">
              <stop offset="0" stop-color="#c4b5fd" />
              <stop offset="1" stop-color="#7c3aed" />
            </radialGradient>
            <radialGradient id="hero-glow" cx="50%" cy="50%" r="50%">
              <stop offset="0" stop-color="#a855f7" stop-opacity="0.3" />
              <stop offset="1" stop-color="#a855f7" stop-opacity="0" />
            </radialGradient>
          </defs>

          <!-- ambient glow behind origin -->
          <circle cx="260" cy="230" r="120" fill="url(#hero-glow)" class="hero__ambient" />

          <!-- connection lines (drawn on load) -->
          <g class="hero__lines" stroke="url(#hero-line)" stroke-width="2.5" stroke-linecap="round" fill="none">
            <line x1="260" y1="230" x2="100" y2="80" class="hero__line" style="--d:0s" />
            <line x1="260" y1="230" x2="430" y2="100" class="hero__line" style="--d:0.2s" />
            <line x1="260" y1="230" x2="390" y2="300" class="hero__line" style="--d:0.4s" />
            <line x1="260" y1="230" x2="130" y2="310" class="hero__line" style="--d:0.6s" />
          </g>

          <!-- space nodes -->
          <g class="hero__nodes">
            <circle cx="100" cy="80" r="14" fill="url(#hero-node)" class="hero__node" style="--d:0.3s" />
            <circle cx="430" cy="100" r="12" fill="url(#hero-node)" class="hero__node" style="--d:0.5s" />
            <circle cx="390" cy="300" r="12" fill="url(#hero-node)" class="hero__node" style="--d:0.7s" />
            <circle cx="130" cy="310" r="11" fill="url(#hero-node)" class="hero__node" style="--d:0.9s" />
          </g>

          <!-- space labels -->
          <g class="hero__labels" font-size="12" font-weight="600" fill="#c4b5fd" font-family="system-ui">
            <text x="100" y="58" text-anchor="middle" opacity="0" class="hero__label" style="--d:1s">text</text>
            <text x="430" y="78" text-anchor="middle" opacity="0" class="hero__label" style="--d:1.1s">image</text>
            <text x="390" y="326" text-anchor="middle" opacity="0" class="hero__label" style="--d:1.2s">audio</text>
            <text x="130" y="338" text-anchor="middle" opacity="0" class="hero__label" style="--d:1.3s">default</text>
          </g>

          <!-- origin record -->
          <circle cx="260" cy="230" r="22" fill="#fff" class="hero__origin" />
          <circle cx="260" cy="230" r="10" fill="#7c3aed" />
        </svg>
      </div>

      <!-- headline + CTA -->
      <div class="hero__content">
        <span class="vl-tag hero__badge">
          <span class="hero__badge-dot" /> Open Source · v0.24.0 · MIT
        </span>
        <h1 class="hero__title">
          The embeddable vector database
          <span class="vl-gradient-text">built in Go</span>
        </h1>
        <p class="hero__tagline">
          Built in Go. Drive from any language through CLI, HTTP, and MCP. Store
          vectors, text, and metadata in a single file with HNSW, BM25, hybrid
          ranking, and multimodal named vector spaces — no database server required.
        </p>

        <!-- install bar -->
        <div class="hero__install" @click="copyInstall">
          <span class="hero__install-prompt">$</span>
          <code class="hero__install-cmd">{{ installCmd }}</code>
          <span class="hero__install-copy" :class="{ 'hero__install-copy--done': copied }">
            {{ copied ? "✓ copied!" : "copy" }}
          </span>
        </div>

        <!-- CTA buttons -->
        <div class="hero__cta">
          <a href="/guide/getting-started" class="vl-btn vl-btn--primary">
            Get Started
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M5 12h14M13 5l7 7-7 7" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </a>
          <a href="/guide/named-vector-spaces" class="vl-btn vl-btn--ghost">Named Vector Spaces</a>
          <a href="https://github.com/abdul-hamid-achik/veclite" class="vl-btn vl-btn--ghost">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.44 9.8 8.21 11.4.6.1.82-.26.82-.58v-2.03c-3.34.73-4.04-1.6-4.04-1.6-.55-1.4-1.34-1.77-1.34-1.77-1.09-.74.08-.73.08-.73 1.2.08 1.84 1.24 1.84 1.24 1.07 1.84 2.8 1.3 3.49 1 .1-.78.42-1.3.76-1.6-2.67-.3-5.47-1.33-5.47-5.93 0-1.3.47-2.37 1.24-3.2-.13-.3-.54-1.52.1-3.18 0 0 1.01-.32 3.3 1.23a11.5 11.5 0 0 1 6 0c2.3-1.55 3.3-1.23 3.3-1.23.65 1.66.24 2.88.12 3.18.77.83 1.23 1.9 1.23 3.2 0 4.6-2.8 5.62-5.48 5.92.43.37.81 1.1.81 2.22v3.29c0 .32.21.69.83.58A12 12 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
            </svg>
            GitHub
          </a>
        </div>
      </div>
    </div>

    <!-- tech marquee -->
    <div class="hero__marquee vl-marquee">
      <div class="vl-marquee__track">
        <span v-for="(badge, i) in marqueeBadges" :key="i" class="vl-marquee__badge">{{ badge }}</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero {
  position: relative;
  padding: 120px 24px 0;
  overflow: hidden;
  background: var(--vl-bg);
}

/* Gradient mesh background */
.hero__mesh {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse 60% 40% at 20% 10%, rgba(99, 102, 241, 0.12), transparent 60%),
    radial-gradient(ellipse 50% 50% at 80% 20%, rgba(168, 85, 247, 0.1), transparent 60%),
    radial-gradient(ellipse 40% 30% at 50% 80%, rgba(124, 58, 237, 0.08), transparent 60%);
  pointer-events: none;
  z-index: 0;
}

/* Floating particles */
.hero__particles {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
}

.hero__particle {
  position: absolute;
  bottom: 0;
  left: var(--x);
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: rgba(168, 85, 247, 0.4);
  animation: floatUp 8s ease-in infinite;
  animation-delay: var(--d);
}

@keyframes floatUp {
  0% { transform: translateY(0) scale(0); opacity: 0; }
  10% { opacity: 1; transform: translateY(-20px) scale(1); }
  90% { opacity: 0.6; }
  100% { transform: translateY(-600px) scale(0); opacity: 0; }
}

.hero__inner {
  max-width: var(--vl-max-w);
  margin: 0 auto;
  display: grid;
  grid-template-columns: 1.1fr 0.9fr;
  gap: 48px;
  align-items: center;
  position: relative;
  z-index: 2;
  min-height: 480px;
}

/* ---- animation canvas ---- */
.hero__viz {
  display: flex;
  justify-content: center;
}

.hero__svg {
  width: 100%;
  max-width: 520px;
  height: auto;
  filter: drop-shadow(0 0 40px rgba(124, 58, 237, 0.15));
}

/* ambient pulse */
.hero__ambient {
  animation: ambientPulse 3s ease-in-out infinite;
  transform-origin: 260px 230px;
}

@keyframes ambientPulse {
  0%, 100% { opacity: 0.6; transform: scale(1); }
  50% { opacity: 1; transform: scale(1.15); }
}

/* line draw-in */
.hero__line {
  stroke-dasharray: 260;
  stroke-dashoffset: 260;
  animation: drawLine 0.8s ease forwards;
  animation-delay: var(--d);
}

@keyframes drawLine {
  to { stroke-dashoffset: 0; }
}

/* node pop-in */
.hero__node {
  opacity: 0;
  transform-origin: center;
  transform: scale(0);
  animation: popIn 0.4s cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
  animation-delay: var(--d);
}

@keyframes popIn {
  to { opacity: 1; transform: scale(1); }
}

/* label fade */
.hero__label {
  animation: fadeIn 0.4s ease forwards;
  animation-delay: var(--d);
}

@keyframes fadeIn {
  to { opacity: 1; }
}

/* origin pulse */
.hero__origin {
  animation: originPulse 2.4s ease-in-out infinite;
  transform-origin: 260px 230px;
}

@keyframes originPulse {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.1); opacity: 0.85; }
}

/* ---- content ---- */
.hero__content {
  text-align: left;
}

.hero__badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.hero__badge-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #4ade80;
  box-shadow: 0 0 8px #4ade80;
  animation: dotPulse 2s ease-in-out infinite;
}

@keyframes dotPulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.hero__title {
  font-size: clamp(34px, 5vw, 56px);
  font-weight: 800;
  line-height: 1.1;
  letter-spacing: -0.03em;
  color: var(--vl-text);
  margin: 16px 0 0;
}

.hero__tagline {
  font-size: 19px;
  line-height: 1.6;
  color: var(--vl-text-2);
  margin: 20px 0 0;
  max-width: 520px;
}

/* ---- install bar (glassmorphism) ---- */
.hero__install {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 32px 0 28px;
  padding: 14px 18px;
  background: rgba(22, 18, 42, 0.5);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid var(--vl-border);
  border-radius: var(--vl-radius-sm);
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  width: fit-content;
  max-width: 100%;
}

.hero__install:hover {
  border-color: var(--vl-primary);
  box-shadow: 0 0 24px -4px rgba(124, 58, 237, 0.3);
}

.hero__install-prompt {
  color: var(--vl-primary-light);
  font-family: var(--vt-font-family-mono);
  font-size: 14px;
  user-select: none;
}

.hero__install-cmd {
  font-family: var(--vt-font-family-mono);
  font-size: 14px;
  color: var(--vl-text);
  white-space: nowrap;
  overflow-x: auto;
}

.hero__install-copy {
  font-size: 12px;
  font-weight: 600;
  color: var(--vl-text-muted);
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.06);
  white-space: nowrap;
  flex-shrink: 0;
  transition: color 0.2s ease, background 0.2s ease;
}

.hero__install-copy--done {
  color: #4ade80;
  background: rgba(74, 222, 128, 0.1);
}

/* ---- CTA ---- */
.hero__cta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

/* ---- marquee ---- */
.hero__marquee {
  margin-top: 64px;
  padding: 20px 0;
  position: relative;
  z-index: 2;
}

@media (max-width: 860px) {
  .hero {
    padding: 80px 20px 0;
  }
  .hero__inner {
    grid-template-columns: 1fr;
    gap: 32px;
    min-height: auto;
  }
  .hero__viz {
    order: -1;
  }
  .hero__svg {
    max-width: 360px;
  }
  .hero__marquee {
    margin-top: 40px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .hero__line,
  .hero__node,
  .hero__label,
  .hero__origin,
  .hero__ambient,
  .hero__particle,
  .hero__badge-dot {
    animation: none;
    opacity: 1;
    transform: none;
    stroke-dashoffset: 0;
  }
}
</style>