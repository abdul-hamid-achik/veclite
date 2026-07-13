<script setup lang="ts">
import { computed, ref } from "vue";
import ProductIcon from "./ProductIcon.vue";

type SearchMode = "hybrid" | "vector" | "text";

const installCmd = "go get github.com/abdul-hamid-achik/veclite";
const copied = ref(false);
const paused = ref(false);
const activeMode = ref<SearchMode>("hybrid");

const modes: Array<{
  id: SearchMode;
  label: string;
  query: string;
  resultLabel: string;
  results: Array<{ path: string; detail: string; score: string; strength: string }>;
}> = [
  {
    id: "hybrid",
    label: "Hybrid",
    query: "crash-safe writes after a restart",
    resultLabel: "HNSW + BM25 fused with RRF",
    results: [
      { path: "guide/durability.md", detail: "vector + exact terms", score: "0.0328", strength: "96%" },
      { path: "wal.go", detail: "semantic match", score: "0.0317", strength: "82%" },
      { path: "guide/go-client.md", detail: "keyword match", score: "0.0161", strength: "55%" },
    ],
  },
  {
    id: "vector",
    label: "Vector",
    query: "recover completed mutations",
    resultLabel: "cosine similarity via HNSW",
    results: [
      { path: "wal.go", detail: "semantic match", score: "0.9421", strength: "94%" },
      { path: "guide/durability.md", detail: "semantic match", score: "0.9174", strength: "78%" },
      { path: "storage.go", detail: "semantic match", score: "0.8812", strength: "62%" },
    ],
  },
  {
    id: "text",
    label: "BM25",
    query: "WAL replay CRC",
    resultLabel: "exact text ranked with BM25",
    results: [
      { path: "wal.go", detail: "3 exact terms", score: "7.184", strength: "91%" },
      { path: "guide/durability.md", detail: "2 exact terms", score: "5.022", strength: "70%" },
      { path: "internal/storage/wal.go", detail: "2 exact terms", score: "4.631", strength: "58%" },
    ],
  },
];

const currentMode = computed(() => modes.find((mode) => mode.id === activeMode.value) ?? modes[0]);

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
  <section class="hero">
    <div class="hero__grid-lines" aria-hidden="true" />
    <div class="hero__orb hero__orb--one" aria-hidden="true" />
    <div class="hero__orb hero__orb--two" aria-hidden="true" />

    <div class="hero__inner">
      <div class="hero__content">
        <a class="hero__release" href="https://github.com/abdul-hamid-achik/veclite/releases/tag/v0.24.0">
          <span class="hero__release-mark" aria-hidden="true" />
          Open source · v0.24.0 · MIT
          <ProductIcon name="arrow" :size="14" />
        </a>

        <h1 class="hero__title">
          Vector search that lives
          <span>inside your app.</span>
        </h1>
        <p class="hero__tagline">
          Keep vectors, text, metadata, and indexes beside your Go code. VecLite
          gives you HNSW, BM25, hybrid ranking, and an optional crash-safe WAL in
          one embeddable database—without another service to deploy.
        </p>

        <div class="hero__actions">
          <a href="/guide/getting-started" class="vl-btn vl-btn--primary">
            Build your first index
            <ProductIcon name="arrow" :size="17" />
          </a>
          <a href="/guide/interfaces" class="vl-btn vl-btn--ghost">Choose an interface</a>
        </div>

        <button class="hero__install" type="button" @click="copyInstall">
          <span class="hero__install-prompt" aria-hidden="true">$</span>
          <code>{{ installCmd }}</code>
          <span class="hero__install-action" aria-live="polite">
            <ProductIcon :name="copied ? 'check' : 'copy'" :size="16" />
            {{ copied ? "Copied" : "Copy" }}
          </span>
        </button>

        <ul class="hero__proof" aria-label="VecLite at a glance">
          <li><span /> Portable snapshot file</li>
          <li><span /> Standard-library storage and search core</li>
          <li><span /> Go, CLI, HTTP, and MCP</li>
        </ul>
      </div>

      <div class="hero__demo" :class="{ 'is-paused': paused }">
        <div class="hero__demo-topbar">
          <div class="hero__demo-context">
            <span class="hero__demo-status" aria-hidden="true" />
            <span>vectors.veclite</span>
            <span class="hero__demo-slash">/</span>
            <strong>docs</strong>
          </div>
          <span class="hero__demo-local">local process</span>
        </div>

        <div class="hero__demo-body">
          <div class="hero__tabs" role="tablist" aria-label="Search mode">
            <button
              v-for="mode in modes"
              :id="`search-tab-${mode.id}`"
              :key="mode.id"
              type="button"
              role="tab"
              :aria-selected="activeMode === mode.id"
              :aria-controls="`search-panel-${mode.id}`"
              :class="{ 'is-active': activeMode === mode.id }"
              @click="activeMode = mode.id"
            >
              {{ mode.label }}
            </button>
          </div>

          <div class="hero__query">
            <ProductIcon name="search" :size="18" />
            <span>{{ currentMode.query }}</span>
            <kbd>⌘ K</kbd>
          </div>

          <div class="hero__routes" aria-label="Active ranking pipeline">
            <div class="hero__route" :class="{ 'is-active': activeMode !== 'text' }">
              <ProductIcon name="branch" :size="16" />
              <span>HNSW</span>
              <small>vector</small>
            </div>
            <div class="hero__route" :class="{ 'is-active': activeMode !== 'vector' }">
              <ProductIcon name="file" :size="16" />
              <span>BM25</span>
              <small>text</small>
            </div>
            <div class="hero__route hero__route--final" :class="{ 'is-active': activeMode === 'hybrid' }">
              <ProductIcon name="layers" :size="16" />
              <span>{{ activeMode === "hybrid" ? "RRF" : "Rank" }}</span>
              <small>merge</small>
            </div>
            <span class="hero__route-pulse" aria-hidden="true" />
          </div>

          <div
            :id="`search-panel-${activeMode}`"
            class="hero__results"
            role="tabpanel"
            :aria-labelledby="`search-tab-${activeMode}`"
          >
            <div class="hero__results-head">
              <span>Ranked results</span>
              <small>{{ currentMode.resultLabel }}</small>
            </div>
            <ol>
              <li v-for="(result, index) in currentMode.results" :key="result.path">
                <span class="hero__result-rank">0{{ index + 1 }}</span>
                <span class="hero__result-copy">
                  <strong>{{ result.path }}</strong>
                  <small>{{ result.detail }}</small>
                </span>
                <span class="hero__result-meter" aria-hidden="true">
                  <span :style="{ transform: `scaleX(${Number.parseInt(result.strength) / 100})` }" />
                </span>
                <code>{{ result.score }}</code>
              </li>
            </ol>
          </div>
        </div>

        <div class="hero__demo-footer">
          <span><ProductIcon name="shield" :size="15" /> No network hop</span>
          <button type="button" :aria-pressed="paused" @click="paused = !paused">
            <ProductIcon :name="paused ? 'play' : 'pause'" :size="14" />
            {{ paused ? "Resume motion" : "Pause motion" }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero {
  min-height: calc(100dvh - 64px);
  padding: 104px 24px 72px;
  position: relative;
  display: grid;
  align-items: center;
  overflow: hidden;
  background: var(--vl-bg);
}

.hero__grid-lines {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(238, 230, 210, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(238, 230, 210, 0.035) 1px, transparent 1px);
  background-size: 64px 64px;
  mask-image: linear-gradient(to bottom, black, transparent 86%);
  pointer-events: none;
}

.hero__orb {
  position: absolute;
  border: 1px solid rgba(217, 119, 87, 0.13);
  border-radius: 50%;
  pointer-events: none;
}

.hero__orb--one {
  width: 680px;
  height: 680px;
  right: -240px;
  top: -300px;
}

.hero__orb--two {
  width: 420px;
  height: 420px;
  right: -110px;
  top: -170px;
}

.hero__inner {
  width: 100%;
  max-width: var(--vl-max-w);
  margin: 0 auto;
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(520px, 1.1fr);
  gap: clamp(48px, 7vw, 96px);
  align-items: center;
  position: relative;
  z-index: 1;
}

.hero__content {
  min-width: 0;
}

.hero__release {
  width: fit-content;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--vl-text-2);
  font-size: 12px;
  font-weight: 650;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  text-decoration: none !important;
}

.hero__release:hover {
  color: var(--vl-text);
}

.hero__release-mark {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--vl-signal);
  box-shadow: 0 0 0 4px rgba(143, 164, 108, 0.12);
}

.hero__title {
  margin: 22px 0 0;
  max-width: 720px;
  color: var(--vl-text);
  font-size: clamp(44px, 5.2vw, 72px);
  font-weight: 720;
  line-height: 0.98;
  letter-spacing: -0.055em;
}

.hero__title span {
  color: var(--vl-accent-soft);
  display: block;
  position: relative;
}

.hero__title span::after {
  content: "";
  width: 96px;
  height: 4px;
  position: absolute;
  left: 2px;
  bottom: -13px;
  border-radius: 999px;
  background: var(--vl-accent);
}

.hero__tagline {
  margin: 34px 0 0;
  max-width: 620px;
  color: var(--vl-text-2);
  font-size: 18px;
  line-height: 1.7;
}

.hero__actions {
  margin-top: 30px;
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.hero__install {
  width: min(100%, 560px);
  margin-top: 22px;
  padding: 11px 11px 11px 16px;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  color: var(--vl-text);
  background: rgba(29, 29, 25, 0.88);
  border: 1px solid var(--vl-border);
  border-radius: 10px;
  cursor: pointer;
  text-align: left;
  transition: border-color 180ms ease, transform 180ms ease, background 180ms ease;
}

.hero__install:hover {
  border-color: var(--vl-border-strong);
  background: var(--vl-surface);
}

.hero__install:active {
  transform: scale(0.985);
}

.hero__install-prompt {
  color: var(--vl-accent);
  font-family: var(--vl-font-mono);
}

.hero__install code {
  min-width: 0;
  overflow: hidden;
  color: var(--vl-text-2);
  font-family: var(--vl-font-mono);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hero__install-action {
  padding: 6px 9px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--vl-text-2);
  background: rgba(246, 240, 225, 0.05);
  border-radius: 7px;
  font-size: 11px;
  font-weight: 650;
}

.hero__proof {
  margin: 25px 0 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 10px 18px;
  list-style: none;
}

.hero__proof li {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--vl-text-muted);
  font-size: 12px;
}

.hero__proof li span {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--vl-accent);
}

.hero__demo {
  min-width: 0;
  color: var(--vl-text);
  background: rgba(23, 23, 20, 0.94);
  border: 1px solid var(--vl-border-strong);
  border-radius: 18px;
  box-shadow: 0 32px 90px rgba(5, 5, 4, 0.34), inset 0 1px 0 rgba(255, 255, 255, 0.045);
  overflow: hidden;
  transform: rotate(0.6deg);
}

.hero__demo-topbar,
.hero__demo-footer {
  min-height: 48px;
  padding: 0 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-color: var(--vl-border);
}

.hero__demo-topbar {
  border-bottom: 1px solid var(--vl-border);
}

.hero__demo-context,
.hero__demo-footer span {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 11px;
}

.hero__demo-context strong {
  color: var(--vl-text-2);
  font-weight: 600;
}

.hero__demo-status {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--vl-signal);
  animation: statusPulse 2.4s ease-in-out infinite;
}

.hero__demo-slash {
  color: var(--vl-border-strong);
}

.hero__demo-local {
  padding: 4px 8px;
  color: var(--vl-signal);
  background: rgba(143, 164, 108, 0.09);
  border: 1px solid rgba(143, 164, 108, 0.16);
  border-radius: 999px;
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.hero__demo-body {
  padding: 18px;
}

.hero__tabs {
  width: fit-content;
  padding: 3px;
  display: flex;
  gap: 2px;
  background: rgba(246, 240, 225, 0.035);
  border: 1px solid var(--vl-border);
  border-radius: 8px;
}

.hero__tabs button {
  padding: 6px 11px;
  color: var(--vl-text-muted);
  background: transparent;
  border: 0;
  border-radius: 5px;
  cursor: pointer;
  font-family: inherit;
  font-size: 11px;
  font-weight: 650;
  transition: color 160ms ease, background 160ms ease, transform 160ms ease;
}

.hero__tabs button:hover {
  color: var(--vl-text);
}

.hero__tabs button:active {
  transform: scale(0.97);
}

.hero__tabs button.is-active {
  color: var(--vl-text);
  background: var(--vl-surface-2);
}

.hero__query {
  margin-top: 14px;
  padding: 13px 14px;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  color: var(--vl-text-2);
  background: #11110f;
  border: 1px solid var(--vl-border-strong);
  border-radius: 9px;
  font-size: 13px;
}

.hero__query svg {
  color: var(--vl-accent);
}

.hero__query kbd {
  padding: 3px 6px;
  color: var(--vl-text-muted);
  background: var(--vl-surface-2);
  border: 1px solid var(--vl-border);
  border-radius: 4px;
  box-shadow: none;
  font-family: var(--vl-font-mono);
  font-size: 9px;
}

.hero__routes {
  margin: 16px 0;
  padding: 0 8px;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  position: relative;
}

.hero__routes::before {
  content: "";
  height: 1px;
  position: absolute;
  left: 16%;
  right: 16%;
  top: 50%;
  background: var(--vl-border-strong);
}

.hero__route {
  min-width: 0;
  padding: 9px;
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0 7px;
  position: relative;
  z-index: 1;
  color: var(--vl-text-muted);
  background: var(--vl-surface);
  border: 1px solid var(--vl-border);
  border-radius: 8px;
  opacity: 0.45;
  transition: opacity 180ms ease, border-color 180ms ease, transform 180ms ease;
}

.hero__route.is-active {
  border-color: rgba(217, 119, 87, 0.38);
  opacity: 1;
  transform: translateY(-1px);
}

.hero__route svg {
  grid-row: span 2;
  align-self: center;
  color: var(--vl-accent);
}

.hero__route span {
  overflow: hidden;
  color: var(--vl-text-2);
  font-family: var(--vl-font-mono);
  font-size: 10px;
  font-weight: 650;
  text-overflow: ellipsis;
}

.hero__route small {
  color: var(--vl-text-muted);
  font-size: 9px;
}

.hero__route-pulse {
  width: 18px;
  height: 3px;
  position: absolute;
  left: 15%;
  top: calc(50% - 1px);
  z-index: 2;
  border-radius: 99px;
  background: var(--vl-accent);
  animation: routePulse 2.8s cubic-bezier(0.16, 1, 0.3, 1) infinite;
}

.hero__results {
  background: #11110f;
  border: 1px solid var(--vl-border);
  border-radius: 10px;
  overflow: hidden;
}

.hero__results-head {
  padding: 10px 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid var(--vl-border);
  font-size: 11px;
  font-weight: 650;
}

.hero__results-head small {
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 9px;
  font-weight: 400;
}

.hero__results ol {
  margin: 0;
  padding: 0;
  list-style: none;
}

.hero__results li {
  min-height: 54px;
  padding: 9px 12px;
  display: grid;
  grid-template-columns: auto minmax(130px, 1fr) 72px auto;
  align-items: center;
  gap: 11px;
  border-bottom: 1px solid rgba(52, 53, 45, 0.62);
}

.hero__results li:last-child {
  border-bottom: 0;
}

.hero__result-rank {
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 9px;
}

.hero__result-copy {
  min-width: 0;
  display: grid;
}

.hero__result-copy strong {
  overflow: hidden;
  color: var(--vl-text-2);
  font-family: var(--vl-font-mono);
  font-size: 10px;
  font-weight: 550;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hero__result-copy small {
  color: var(--vl-text-muted);
  font-size: 9px;
}

.hero__result-meter {
  height: 3px;
  display: block;
  background: var(--vl-surface-2);
  border-radius: 99px;
  overflow: hidden;
}

.hero__result-meter span {
  width: 100%;
  height: 100%;
  display: block;
  background: var(--vl-accent);
  border-radius: inherit;
  transform-origin: left;
}

.hero__results li > code {
  color: var(--vl-accent-soft);
  font-family: var(--vl-font-mono);
  font-size: 9px;
}

.hero__demo-footer {
  min-height: 44px;
  border-top: 1px solid var(--vl-border);
}

.hero__demo-footer span svg {
  color: var(--vl-signal);
}

.hero__demo-footer button {
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

.hero__demo-footer button:hover {
  color: var(--vl-text);
}

.hero__demo.is-paused *,
.hero__demo.is-paused *::before,
.hero__demo.is-paused *::after {
  animation-play-state: paused !important;
}

@keyframes routePulse {
  0% { opacity: 0; transform: translateX(0) scaleX(0.5); }
  15% { opacity: 1; }
  85% { opacity: 1; }
  100% { opacity: 0; transform: translateX(345px) scaleX(1); }
}

@keyframes statusPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.55; transform: scale(0.78); }
}

@media (max-width: 1100px) {
  .hero__inner {
    grid-template-columns: minmax(0, 1fr) minmax(450px, 1fr);
    gap: 44px;
  }

  .hero__result-meter {
    display: none;
  }

  .hero__results li {
    grid-template-columns: auto minmax(130px, 1fr) auto;
  }
}

@media (max-width: 900px) {
  .hero {
    padding-top: 88px;
  }

  .hero__inner {
    grid-template-columns: 1fr;
  }

  .hero__content {
    max-width: 720px;
  }

  .hero__demo {
    max-width: 680px;
    transform: none;
  }
}

@media (max-width: 560px) {
  .hero {
    padding: 72px 18px 52px;
  }

  .hero__title {
    font-size: clamp(40px, 13vw, 54px);
  }

  .hero__tagline {
    font-size: 16px;
  }

  .hero__actions .vl-btn {
    width: 100%;
    justify-content: center;
  }

  .hero__proof {
    display: grid;
  }

  .hero__demo-body {
    padding: 12px;
  }

  .hero__demo-local,
  .hero__query kbd,
  .hero__results-head small,
  .hero__result-meter {
    display: none;
  }

  .hero__routes {
    gap: 6px;
    padding: 0;
  }

  .hero__route {
    grid-template-columns: 1fr;
    justify-items: center;
    text-align: center;
  }

  .hero__route svg {
    grid-row: auto;
  }

  .hero__route small {
    display: none;
  }

  .hero__route-pulse {
    display: none;
  }

  .hero__results li {
    grid-template-columns: auto minmax(0, 1fr) auto;
  }

  .hero__demo-footer span {
    font-size: 9px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .hero__route-pulse,
  .hero__demo-status {
    animation: none;
  }
}
</style>
