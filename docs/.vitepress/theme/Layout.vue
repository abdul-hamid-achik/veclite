<script setup>
import DefaultTheme from "vitepress/theme";
import { useRoute } from "vitepress";
import { computed, watch } from "vue";

const { Layout } = DefaultTheme;
const route = useRoute();
const isHome = computed(() => route.path === "/" || route.path === "/index.html");

watch(
  isHome,
  (val) => {
    if (typeof document !== "undefined") {
      document.body.classList.toggle("vl-home", val);
    }
  },
  { immediate: true }
);
</script>

<template>
  <Layout />
</template>

<style>
body.vl-home {
  --vp-c-bg: var(--vl-bg);
  --vp-c-bg-alt: var(--vl-bg-raised);
  --vp-c-bg-soft: var(--vl-surface);
  --vp-c-bg-soft-up: var(--vl-bg-raised);
  --vp-c-divider: var(--vl-border);
  --vp-c-divider-light: var(--vl-border);
  --vp-c-text-1: var(--vl-text);
  --vp-c-text-2: var(--vl-text-2);
  --vp-c-text-3: var(--vl-text-muted);
  --vp-c-text-code: var(--vl-text-2);
  --vp-c-brand-1: var(--vl-accent-soft);
  --vp-c-brand-2: var(--vl-accent);
  --vp-c-brand-3: var(--vl-accent-strong);
  --vp-c-brand-soft: rgba(217, 119, 87, 0.12);
  --vp-c-gutter: var(--vl-border);
  --vp-nav-bg: rgba(18, 18, 15, 0.82);
  --vp-nav-height: 64px;
  --vp-nav-bg-image: none;
  background: var(--vl-bg);
}

body.vl-home .VPSidebar,
body.vl-home .VPDoc .aside,
body.vl-home .VPDoc .aside-container,
body.vl-home .VPDocFooter,
body.vl-home .VPDoc .prev-next,
body.vl-home .VPDoc .pager,
body.vl-home .VPDoc .edit-link,
body.vl-home .VPDoc .edit-info,
body.vl-home .VPLastUpdated,
body.vl-home .VPCarbonAds {
  display: none !important;
}

body.vl-home .VPLocalNav,
body.vl-home .VPNavBarAppearance {
  display: none !important;
}

body.vl-home .VPNavBar.has-sidebar .title {
  position: static !important;
  width: auto !important;
  flex-shrink: 0;
  padding-right: 16px;
}

body.vl-home .VPNavBar.has-sidebar .content {
  padding-left: 0 !important;
}
body.vl-home .VPNavBar.has-sidebar .curtain {
  display: none !important;
}
body.vl-home .VPDoc.has-sidebar,
body.vl-home .VPDoc.has-aside {
  padding: 0 !important;
}

body.vl-home .Layout {
  padding-left: 0 !important;
}

body.vl-home .VPContent {
  padding: 0 !important;
}

body.vl-home .VPContent .VPDoc .container,
body.vl-home .VPContent .VPDoc .content,
body.vl-home .VPContent .VPDoc .content-container,
body.vl-home .vp-doc,
body.vl-home .VPContent .VPDoc main {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
  min-width: 0 !important;
}

body.vl-home .VPContent .VPDoc .container {
  display: block !important;
}

body.vl-home .VPContent .VPPage {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
}

body.vl-home .VPNavBar {
  position: sticky !important;
  top: 0;
  z-index: 100;
  background-color: rgba(18, 18, 15, 0.84) !important;
  backdrop-filter: blur(18px) saturate(125%);
  -webkit-backdrop-filter: blur(18px) saturate(125%);
  border-bottom: 1px solid rgba(72, 72, 61, 0.58);
}

body.vl-home .VPNavBar,
body.vl-home .VPNavBar .wrapper,
body.vl-home .VPNavBar .container,
body.vl-home .VPNavBar .content,
body.vl-home .VPNavBar .content-body {
  background-image: none !important;
}

body.vl-home .VPNavBar .content-body {
  background-color: transparent !important;
}

body.vl-home .VPNavBar .VPNavBarTitle .title {
  color: var(--vl-text) !important;
  font-weight: 720;
  border-bottom: none !important;
}

body.vl-home .VPNavBar .VPNavBarTitle a.title {
  border-bottom: none !important;
}

body.vl-home .VPNavBar .VPNavBarMenuLink,
body.vl-home .VPNavBar .menu-item .text,
body.vl-home .VPNavBar .VPNavBarMenu .text {
  color: var(--vl-text-2) !important;
  transition: color 0.2s ease;
}

body.vl-home .VPNavBar .VPNavBarMenuLink:hover,
body.vl-home .VPNavBar .menu-item:hover .text,
body.vl-home .VPNavBar .VPNavBarMenu .text:hover {
  color: var(--vl-text) !important;
}

body.vl-home .VPNavBar .VPNavBarMenuLink.active {
  color: var(--vl-accent-soft) !important;
}

body.vl-home .VPNavBar .DocSearch-Button {
  background-color: rgba(246, 240, 225, 0.035) !important;
  border: 1px solid var(--vl-border) !important;
  color: var(--vl-text-muted) !important;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  box-shadow: none !important;
  transition: border-color 0.2s ease, background-color 0.2s ease;
}
body.vl-home .VPNavBar .DocSearch-Button:hover {
  border-color: var(--vl-border-strong) !important;
  background-color: rgba(246, 240, 225, 0.065) !important;
}

body.vl-home .VPNavBar .DocSearch-Search-Icon,
body.vl-home .VPNavBar .DocSearch-Button-Placeholder {
  color: var(--vl-text-muted) !important;
}

body.vl-home .VPNavBar .DocSearch-Button-Key {
  color: var(--vl-text-2) !important;
  border-color: var(--vl-border) !important;
}

body.vl-home .VPNavBar .VPNavBarSocialLinks .VPNavBarSocialLink {
  color: var(--vl-text-2) !important;
}

body.vl-home .VPNavBar .VPNavBarSocialLinks .VPNavBarSocialLink:hover {
  color: var(--vl-text) !important;
}

body.vl-home .VPNavBar .VPSwitch {
  border-color: var(--vl-border) !important;
  background-color: rgba(246, 240, 225, 0.04) !important;
}

body.vl-home .VPNavBar .VPSwitch .vpi-sun,
body.vl-home .VPNavBar .VPSwitch .vpi-moon {
  color: var(--vl-text-2) !important;
}

body.vl-home .VPNavBar .VPFlyout .button .text,
body.vl-home .VPNavBar .VPNavBarHamburger .container .menu-text {
  color: var(--vl-text-2) !important;
}

body.vl-home .VPNavScreen {
  background-color: var(--vl-bg) !important;
  background-image: none !important;
}

body.vl-home .VPNavScreen .VPNavScreenMenuLink {
  color: var(--vl-text-2) !important;
  border-bottom-color: var(--vl-border) !important;
}

body.vl-home .VPNavScreen .VPNavScreenMenuLink:hover {
  color: var(--vl-text) !important;
}

@media (max-width: 768px) {
  body.vl-home .VPNavBar.has-sidebar .content {
    padding-left: 0 !important;
  }
}
</style>
