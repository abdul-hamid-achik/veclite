<script setup>
import DefaultTheme from "vitepress/theme";
import { useRoute } from "vitepress";
import { computed, watch } from "vue";

const { Layout } = DefaultTheme;
const route = useRoute();
const isHome = computed(() => route.path === "/" || route.path === "/index.html");

// Toggle a body class so global CSS can make the home page full-bleed
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
/* ---- Home page: full-bleed, no sidebar, no doc container constraints ---- */

/* Hide sidebar and aside */
body.vl-home .VPSidebar,
body.vl-home .VPDoc .aside {
  display: none !important;
}

/* Remove sidebar-related classes effect */
body.vl-home .VPNavBar.has-sidebar .content {
  padding-left: 0 !important;
}

body.vl-home .VPNavBar.has-sidebar .curtain {
  display: none !important;
}

/* Remove the has-sidebar / has-aside padding on the doc container */
body.vl-home .VPDoc.has-sidebar,
body.vl-home .VPDoc.has-aside {
  padding: 0 !important;
}

/* The container is a flex row (content + aside). Make it full-width column. */
body.vl-home .VPDoc .container {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
  display: block !important;
}

/* The content flex item */
body.vl-home .VPDoc .content {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
  min-width: 0 !important;
}

/* THE KEY: .content-container constrains the markdown to 688px with auto margins.
   Override it to be full-width. */
body.vl-home .VPDoc .content-container {
  max-width: none !important;
  margin: 0 !important;
  padding: 0 !important;
}

/* The vp-doc wrapper */
body.vl-home .vp-doc {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
}

/* The main element */
body.vl-home .VPDoc main {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
}

/* VPContent outer padding */
body.vl-home .VPContent {
  padding: 0 !important;
}

body.vl-home .VPContent .VPPage {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
}

/* Make the nav bar blend with the dark theme on home */
body.vl-home .VPNavBar {
  background-color: rgba(11, 8, 23, 0.72) !important;
  backdrop-filter: blur(16px) saturate(180%);
  -webkit-backdrop-filter: blur(16px) saturate(180%);
  border-bottom: 1px solid rgba(45, 38, 84, 0.5);
}

body.vl-home .VPNavBar .title {
  color: #f5f3ff !important;
}

body.vl-home .VPNavBar .content .menu .text,
body.vl-home .VPNavBar .content .menu-item .text {
  color: #c4b5fd !important;
}

/* Footer override */
body.vl-home .VPFooter {
  background: rgba(11, 8, 23, 0.8) !important;
  border-top: 1px solid rgba(45, 38, 84, 0.5);
}

body.vl-home .VPFooter .message {
  color: #8b85a8 !important;
}
</style>