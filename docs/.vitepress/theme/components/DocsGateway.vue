<script setup lang="ts">
import ProductIcon from "./ProductIcon.vue";

const paths = [
  {
    number: "01",
    icon: "bolt",
    title: "Start with a working index",
    body: "Install VecLite, run a complete Go example, then repeat the same flow from the CLI.",
    meta: "Go + CLI quickstart",
    href: "/guide/getting-started",
    featured: true,
  },
  {
    number: "02",
    icon: "search",
    title: "Choose how to search",
    body: "Compare vector, BM25, hybrid RRF, multi-space fusion, filters, and score semantics.",
    meta: "Search guide",
    href: "/guide/search",
  },
  {
    number: "03",
    icon: "layers",
    title: "Model multimodal records",
    body: "Keep several embedding spaces on one logical record and fuse their ranked results.",
    meta: "Named spaces",
    href: "/guide/named-vector-spaces",
  },
  {
    number: "04",
    icon: "shield",
    title: "Operate it safely",
    body: "Pick a WAL mode, understand snapshots and recovery, and choose a multi-process topology.",
    meta: "Durability + access",
    href: "/guide/durability",
  },
  {
    number: "05",
    icon: "terminal",
    title: "Look up the contract",
    body: "Use exact CLI flags, HTTP request shapes, response envelopes, and error behavior.",
    meta: "Interface reference",
    href: "/reference/cli",
  },
];
</script>

<template>
  <section class="docs-gateway vl-section">
    <div class="vl-inner docs-gateway__layout">
      <header v-reveal class="docs-gateway__head">
        <span class="docs-gateway__eyebrow">Documentation field guide</span>
        <h2>Enter through the task you have.</h2>
        <p>
          Tutorials teach a workflow. Guides help you make a decision. Reference
          pages give you the exact contract. The new docs keep those jobs separate.
        </p>
        <a href="/guide/getting-started" class="docs-gateway__all">
          Open the documentation
          <ProductIcon name="arrow" :size="16" />
        </a>
      </header>

      <div class="docs-gateway__paths">
        <a
          v-for="path in paths"
          :key="path.title"
          v-reveal
          :href="path.href"
          class="docs-path"
          :class="{ 'docs-path--featured': path.featured }"
        >
          <span class="docs-path__number">{{ path.number }}</span>
          <span class="docs-path__icon"><ProductIcon :name="path.icon" :size="20" /></span>
          <span class="docs-path__copy">
            <small>{{ path.meta }}</small>
            <strong>{{ path.title }}</strong>
            <span>{{ path.body }}</span>
          </span>
          <ProductIcon class="docs-path__arrow" name="arrow" :size="17" />
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.docs-gateway {
  color: #28261f;
  background: #ede7da;
}

.docs-gateway::before {
  content: "";
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(74, 68, 55, 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgba(74, 68, 55, 0.055) 1px, transparent 1px);
  background-size: 48px 48px;
  pointer-events: none;
}

.docs-gateway__layout {
  display: grid;
  grid-template-columns: minmax(0, 0.64fr) minmax(560px, 1.36fr);
  gap: clamp(54px, 8vw, 104px);
  align-items: start;
}

.docs-gateway__head {
  position: sticky;
  top: 104px;
}

.docs-gateway__eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #a84f36;
  font-family: var(--vl-font-mono);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.docs-gateway__eyebrow::before {
  content: "";
  width: 24px;
  height: 1px;
  background: #bd5d42;
}

.docs-gateway__head h2 {
  margin: 18px 0 0;
  color: #24221c;
  font-size: clamp(38px, 4.5vw, 56px);
  font-weight: 720;
  line-height: 1.02;
  letter-spacing: -0.05em;
}

.docs-gateway__head p {
  margin: 22px 0 0;
  color: #5f5a4e;
  font-size: 15px;
  line-height: 1.72;
}

.docs-gateway__all {
  margin-top: 28px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #9d492f;
  font-size: 12px;
  font-weight: 700;
  text-decoration: none !important;
}

.docs-gateway__all:hover {
  color: #24221c;
}

.docs-gateway__paths {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.docs-path {
  min-width: 0;
  min-height: 220px;
  padding: 22px;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-content: start;
  gap: 20px 12px;
  position: relative;
  color: #28261f;
  background: rgba(249, 246, 238, 0.58);
  border: 1px solid rgba(91, 83, 67, 0.2);
  border-radius: 12px;
  text-decoration: none !important;
  transition: transform 190ms ease, background 190ms ease, border-color 190ms ease;
}

.docs-path:hover {
  background: rgba(252, 250, 244, 0.9);
  border-color: rgba(154, 72, 48, 0.38);
  transform: translateY(-2px);
}

.docs-path:active {
  transform: scale(0.985);
}

.docs-path--featured {
  grid-column: 1 / -1;
  min-height: 200px;
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  align-items: center;
  background: #25231d;
  border-color: #25231d;
  color: #f4eddd;
}

.docs-path__number {
  color: #9b9486;
  font-family: var(--vl-font-mono);
  font-size: 9px;
}

.docs-path--featured .docs-path__number {
  align-self: start;
  color: #7e796e;
}

.docs-path__icon {
  width: 40px;
  height: 40px;
  grid-column: 1;
  display: grid;
  place-items: center;
  color: #ad5035;
  background: rgba(180, 80, 52, 0.09);
  border: 1px solid rgba(180, 80, 52, 0.18);
  border-radius: 9px;
}

.docs-path--featured .docs-path__icon {
  grid-column: auto;
  color: var(--vl-accent-soft);
  background: rgba(217, 119, 87, 0.09);
  border-color: rgba(217, 119, 87, 0.2);
}

.docs-path__copy {
  grid-column: 1 / -1;
  display: grid;
  gap: 8px;
}

.docs-path--featured .docs-path__copy {
  grid-column: auto;
}

.docs-path__copy small {
  color: #9b4b34;
  font-family: var(--vl-font-mono);
  font-size: 8px;
  font-weight: 700;
  letter-spacing: 0.055em;
  text-transform: uppercase;
}

.docs-path--featured .docs-path__copy small {
  color: var(--vl-accent-soft);
}

.docs-path__copy strong {
  color: #28261f;
  font-size: 18px;
  font-weight: 700;
  line-height: 1.15;
  letter-spacing: -0.025em;
}

.docs-path--featured .docs-path__copy strong {
  color: #f4eddd;
  font-size: 23px;
}

.docs-path__copy > span {
  color: #625d51;
  font-size: 11px;
  line-height: 1.55;
}

.docs-path--featured .docs-path__copy > span {
  max-width: 480px;
  color: #aaa397;
}

.docs-path__arrow {
  justify-self: end;
  color: #a49c8d;
  transition: color 180ms ease, transform 180ms ease;
}

.docs-path--featured .docs-path__arrow {
  color: #817a6e;
}

.docs-path:hover .docs-path__arrow {
  color: #a84f36;
  transform: translateX(2px);
}

@media (max-width: 960px) {
  .docs-gateway__layout {
    grid-template-columns: 1fr;
  }

  .docs-gateway__head {
    max-width: 720px;
    position: static;
  }
}

@media (max-width: 580px) {
  .docs-gateway__paths {
    grid-template-columns: 1fr;
  }

  .docs-path,
  .docs-path--featured {
    grid-column: auto;
    grid-template-columns: auto minmax(0, 1fr) auto;
    min-height: 0;
  }

  .docs-path--featured .docs-path__number {
    display: none;
  }

  .docs-path__icon,
  .docs-path--featured .docs-path__icon {
    grid-column: 1;
  }

  .docs-path__copy,
  .docs-path--featured .docs-path__copy {
    grid-column: 1 / -1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .docs-path,
  .docs-path__arrow {
    transition: none;
  }

  .docs-path:hover,
  .docs-path:active {
    transform: none;
  }
}
</style>
