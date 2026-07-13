<script setup lang="ts">
import ProductIcon from "./ProductIcon.vue";

const surfaces = [
  {
    icon: "bolt",
    name: "Embedded Go",
    fit: "Single-process applications",
    description: "Lowest-friction path for local reads and writes. Open a file and call the library directly.",
    sample: 'veclite.Open("data.veclite")',
    href: "/guide/getting-started",
  },
  {
    icon: "globe",
    name: "HTTP server",
    fit: "Multiple local clients",
    description: "Let one writer process own the database and expose the stable JSON contract to trusted clients.",
    sample: "veclite serve data.veclite",
    href: "/reference/http-api",
  },
  {
    icon: "terminal",
    name: "CLI + JSON",
    fit: "Shells and language bridges",
    description: "Build scripts and early integrations around deterministic commands and JSON output.",
    sample: "veclite search … --json",
    href: "/reference/cli",
  },
  {
    icon: "bot",
    name: "MCP server",
    fit: "Agents and coding tools",
    description: "Expose search, records, memory, episodes, and graph operations as MCP tools over stdio.",
    sample: "veclite mcp data.veclite",
    href: "/guide/interfaces#mcp-for-ai-agents",
  },
];
</script>

<template>
  <section class="interfaces vl-section">
    <div class="vl-inner interfaces__layout">
      <div v-reveal class="interfaces__head">
        <span class="vl-eyebrow">Use the right boundary</span>
        <h2 class="vl-h2">Embedded when you can. A local service when you need one.</h2>
        <p class="vl-sub">
          VecLite is a Go library first, with additive CLI, HTTP, and MCP surfaces.
          Choose based on process ownership and language—not a different storage engine.
        </p>
        <div class="interfaces__note">
          <ProductIcon name="shield" :size="18" />
          <p>
            The HTTP server has no built-in authentication or TLS. Keep it local or
            put a trusted authenticated proxy in front of it.
          </p>
        </div>
      </div>

      <div v-reveal class="interfaces__list">
        <a v-for="(surface, index) in surfaces" :key="surface.name" :href="surface.href" class="interface-row">
          <span class="interface-row__index">0{{ index + 1 }}</span>
          <span class="interface-row__icon"><ProductIcon :name="surface.icon" :size="20" /></span>
          <span class="interface-row__copy">
            <span class="interface-row__title">
              <strong>{{ surface.name }}</strong>
              <small>{{ surface.fit }}</small>
            </span>
            <span class="interface-row__description">{{ surface.description }}</span>
            <code>{{ surface.sample }}</code>
          </span>
          <ProductIcon class="interface-row__arrow" name="arrow" :size="17" />
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.interfaces {
  background: var(--vl-bg);
}

.interfaces__layout {
  display: grid;
  grid-template-columns: minmax(0, 0.72fr) minmax(520px, 1.28fr);
  gap: clamp(56px, 9vw, 112px);
  align-items: start;
}

.interfaces__head {
  position: sticky;
  top: 104px;
}

.interfaces__note {
  margin-top: 32px;
  padding: 16px;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  background: rgba(217, 119, 87, 0.045);
  border: 1px solid rgba(217, 119, 87, 0.16);
  border-radius: 10px;
}

.interfaces__note svg {
  color: var(--vl-accent-soft);
}

.interfaces__note p {
  margin: 0;
  color: var(--vl-text-muted);
  font-size: 10px;
  line-height: 1.55;
}

.interfaces__list {
  border-top: 1px solid var(--vl-border-strong);
}

.interface-row {
  min-width: 0;
  padding: 24px 8px;
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  align-items: start;
  gap: 15px;
  color: inherit;
  border-bottom: 1px solid var(--vl-border-strong);
  text-decoration: none !important;
  transition: background 180ms ease, transform 180ms ease;
}

.interface-row:hover {
  background: rgba(246, 240, 225, 0.025);
  transform: translateX(3px);
}

.interface-row:active {
  transform: scale(0.99);
}

.interface-row__index {
  padding-top: 10px;
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 8px;
}

.interface-row__icon {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  color: var(--vl-accent-soft);
  background: var(--vl-surface);
  border: 1px solid var(--vl-border);
  border-radius: 9px;
}

.interface-row__copy {
  min-width: 0;
  display: grid;
  gap: 8px;
}

.interface-row__title {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
}

.interface-row__title strong {
  color: var(--vl-text);
  font-size: 16px;
  font-weight: 680;
  letter-spacing: -0.02em;
}

.interface-row__title small {
  color: var(--vl-text-muted);
  font-family: var(--vl-font-mono);
  font-size: 8px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.interface-row__description {
  color: var(--vl-text-2);
  font-size: 11px;
  line-height: 1.55;
}

.interface-row code {
  width: fit-content;
  max-width: 100%;
  overflow: hidden;
  color: var(--vl-accent-soft);
  font-family: var(--vl-font-mono);
  font-size: 9px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.interface-row__arrow {
  margin-top: 11px;
  color: var(--vl-text-muted);
  transition: color 180ms ease, transform 180ms ease;
}

.interface-row:hover .interface-row__arrow {
  color: var(--vl-accent-soft);
  transform: translateX(2px);
}

@media (max-width: 960px) {
  .interfaces__layout {
    grid-template-columns: 1fr;
  }

  .interfaces__head {
    max-width: 760px;
    position: static;
  }
}

@media (max-width: 560px) {
  .interface-row {
    grid-template-columns: auto minmax(0, 1fr) auto;
  }

  .interface-row__index {
    display: none;
  }

  .interface-row__title {
    display: grid;
    gap: 3px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .interface-row,
  .interface-row__arrow {
    transition: none;
  }

  .interface-row:hover,
  .interface-row:active {
    transform: none;
  }
}
</style>
