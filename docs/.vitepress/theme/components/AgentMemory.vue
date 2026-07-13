<script setup lang="ts">
import ProductIcon from "./ProductIcon.vue";

const memoryTools = [
  {
    icon: "clock",
    title: "Shape time",
    body: "Set TTLs and importance, start background cleanup when you want it, and apply temporal decay to vector retrieval.",
  },
  {
    icon: "branch",
    title: "Keep conversation context",
    body: "Store session turns, parent-child threads, roles, and turn order beside the vectors that retrieve them.",
  },
  {
    icon: "layers",
    title: "Organize long memory",
    body: "Caller-driven consolidation and episode APIs help group related records while preserving the originals.",
  },
  {
    icon: "spark",
    title: "Connect and react",
    body: "Subscriptions notify on matching inserts, while the knowledge graph adds typed entities, edges, and traversal.",
  },
];
</script>

<template>
  <section class="memory vl-section">
    <div class="vl-inner memory__grid">
      <div v-reveal class="memory__copy">
        <span class="vl-eyebrow">Agent memory toolkit</span>
        <h2 class="vl-h2">Retrieval is the start of memory, not the end.</h2>
        <p class="vl-sub">
          VecLite adds explicit APIs for the lifecycle around retrieval: recency,
          importance, conversations, episodes, consolidation, notifications, and
          relationships. Your application decides when each policy runs.
        </p>

        <div class="memory__tools">
          <div v-for="tool in memoryTools" :key="tool.title" class="memory__tool">
            <ProductIcon :name="tool.icon" :size="18" />
            <div>
              <strong>{{ tool.title }}</strong>
              <p>{{ tool.body }}</p>
            </div>
          </div>
        </div>

        <a href="/guide/agent-memory" class="vl-btn vl-btn--ghost">
          Design an agent memory store
          <ProductIcon name="arrow" :size="16" />
        </a>
      </div>

      <div v-reveal class="memory__console vl-panel" aria-label="Agent memory record illustration">
        <div class="memory__console-bar">
          <span><ProductIcon name="bot" :size="15" /> session / incident-42</span>
          <code>3 turns</code>
        </div>

        <div class="memory__timeline">
          <div class="memory__turn">
            <span class="memory__avatar">U</span>
            <div><small>user · turn 01</small><p>Why did the write survive the restart?</p></div>
          </div>
          <div class="memory__turn">
            <span class="memory__avatar memory__avatar--tool">T</span>
            <div><small>tool · search</small><p>Matched the WAL recovery guide and replay code.</p></div>
          </div>
          <div class="memory__turn">
            <span class="memory__avatar memory__avatar--agent">A</span>
            <div><small>assistant · turn 03</small><p>The completed mutation was replayed over the last snapshot.</p></div>
          </div>
        </div>

        <div class="memory__record">
          <div class="memory__record-head">
            <span class="vl-tag">retrieved memory</span>
            <code>#1042</code>
          </div>
          <p>WAL replay restores completed mutations after an interrupted writer.</p>
          <div class="memory__signals">
            <span><small>importance</small><strong>0.88</strong></span>
            <span><small>expires</small><strong>23h</strong></span>
            <span><small>accesses</small><strong>12</strong></span>
          </div>
          <div class="memory__meter" aria-hidden="true">
            <span style="transform: scaleX(0.88)" />
          </div>
        </div>

        <div class="memory__relations" aria-hidden="true">
          <span class="memory__node memory__node--main">memory</span>
          <span class="memory__node memory__node--one">episode</span>
          <span class="memory__node memory__node--two">entity</span>
          <span class="memory__node memory__node--three">source</span>
          <i class="memory__edge memory__edge--one" />
          <i class="memory__edge memory__edge--two" />
          <i class="memory__edge memory__edge--three" />
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.memory {
  background: var(--vl-bg-raised);
  border-top: 1px solid var(--vl-border);
  border-bottom: 1px solid var(--vl-border);
}

.memory__grid {
  display: grid;
  grid-template-columns: minmax(0, 0.88fr) minmax(480px, 1.12fr);
  gap: clamp(50px, 8vw, 100px);
  align-items: center;
}

.memory__tools {
  margin: 36px 0 30px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 22px;
  border-top: 1px solid var(--vl-border);
}

.memory__tool {
  padding: 20px 0;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  border-bottom: 1px solid var(--vl-border);
}

.memory__tool svg {
  margin-top: 2px;
  color: var(--vl-accent-soft);
}

.memory__tool strong {
  color: var(--vl-text);
  font-size: 12px;
  font-weight: 680;
}

.memory__tool p {
  margin: 5px 0 0;
  color: var(--vl-text-muted);
  font-size: 10px;
  line-height: 1.55;
}

.memory__console {
  min-width: 0;
  padding: 16px;
  background: #191915;
  box-shadow: 0 28px 78px rgba(5, 5, 4, 0.24);
}

.memory__console-bar {
  min-height: 38px;
  margin: -16px -16px 14px;
  padding: 0 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--vl-text-muted);
  background: var(--vl-surface);
  border-bottom: 1px solid var(--vl-border);
  font-family: var(--vl-font-mono);
  font-size: 9px;
}

.memory__console-bar span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.memory__console-bar span svg {
  color: var(--vl-accent-soft);
}

.memory__console-bar code {
  color: var(--vl-signal);
}

.memory__timeline {
  padding: 8px 4px 8px 10px;
  display: grid;
  gap: 12px;
  position: relative;
}

.memory__timeline::before {
  content: "";
  width: 1px;
  position: absolute;
  left: 22px;
  top: 24px;
  bottom: 24px;
  background: var(--vl-border-strong);
}

.memory__turn {
  display: grid;
  grid-template-columns: 26px minmax(0, 1fr);
  align-items: start;
  gap: 11px;
  position: relative;
  z-index: 1;
}

.memory__avatar {
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  color: var(--vl-text-2);
  background: var(--vl-surface-2);
  border: 1px solid var(--vl-border-strong);
  border-radius: 50%;
  font-family: var(--vl-font-mono);
  font-size: 8px;
}

.memory__avatar--tool {
  color: var(--vl-accent-soft);
  border-color: rgba(217, 119, 87, 0.35);
}

.memory__avatar--agent {
  color: var(--vl-signal);
}

.memory__turn div {
  padding-top: 1px;
}

.memory__turn small {
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 8px;
}

.memory__turn p {
  margin: 3px 0 0;
  color: var(--vl-text-2);
  font-size: 10px;
  line-height: 1.45;
}

.memory__record {
  margin-top: 10px;
  padding: 15px;
  background: #11110f;
  border: 1px solid var(--vl-border-strong);
  border-radius: 10px;
}

.memory__record-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.memory__record-head code {
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 8px;
}

.memory__record > p {
  margin: 12px 0;
  color: var(--vl-text-2);
  font-size: 11px;
  line-height: 1.55;
}

.memory__signals {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}

.memory__signals span {
  display: grid;
  gap: 2px;
}

.memory__signals small {
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 7px;
  text-transform: uppercase;
}

.memory__signals strong {
  color: var(--vl-text);
  font-family: var(--vl-font-mono);
  font-size: 10px;
  font-weight: 550;
}

.memory__meter {
  height: 3px;
  margin-top: 12px;
  background: var(--vl-surface-2);
  border-radius: 999px;
  overflow: hidden;
}

.memory__meter span {
  width: 100%;
  height: 100%;
  display: block;
  background: var(--vl-accent);
  transform-origin: left;
}

.memory__relations {
  height: 116px;
  margin-top: 14px;
  position: relative;
  background:
    linear-gradient(rgba(246, 240, 225, 0.02) 1px, transparent 1px),
    linear-gradient(90deg, rgba(246, 240, 225, 0.02) 1px, transparent 1px);
  background-size: 22px 22px;
  border: 1px solid var(--vl-border);
  border-radius: 9px;
  overflow: hidden;
}

.memory__node {
  padding: 5px 8px;
  position: absolute;
  z-index: 2;
  color: var(--vl-text-muted);
  background: var(--vl-surface-2);
  border: 1px solid var(--vl-border-strong);
  border-radius: 999px;
  font-family: var(--vl-font-mono);
  font-size: 7px;
}

.memory__node--main {
  left: calc(50% - 27px);
  top: 43px;
  color: var(--vl-accent-soft);
  border-color: rgba(217, 119, 87, 0.38);
}

.memory__node--one { left: 8%; top: 18px; }
.memory__node--two { right: 9%; top: 16px; }
.memory__node--three { right: 15%; bottom: 13px; }

.memory__edge {
  height: 1px;
  position: absolute;
  z-index: 1;
  background: var(--vl-border-strong);
  transform-origin: left;
}

.memory__edge--one { width: 38%; left: 19%; top: 40px; transform: rotate(13deg); }
.memory__edge--two { width: 34%; left: 54%; top: 56px; transform: rotate(-18deg); }
.memory__edge--three { width: 30%; left: 52%; top: 61px; transform: rotate(18deg); }

@media (max-width: 960px) {
  .memory__grid {
    grid-template-columns: 1fr;
  }

  .memory__copy {
    max-width: 760px;
  }

  .memory__console {
    max-width: 680px;
  }
}

@media (max-width: 560px) {
  .memory__tools {
    grid-template-columns: 1fr;
  }

  .memory__copy .vl-btn {
    width: 100%;
  }
}
</style>
