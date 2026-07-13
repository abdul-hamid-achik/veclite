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
/* ============================================================
   HOME PAGE: Override VitePress default theme entirely
   The landing page is a custom dark-violet experience.
   All VitePress doc chrome (sidebar, aside, footer, pager,
   outline, TOC) is hidden. The nav bar is restyled to match.
   ============================================================ */

/* ---- 1. Override VitePress CSS variables for the dark violet theme ---- */
body.vl-home {
  --vp-c-bg: #0b0817;
  --vp-c-bg-alt: #16122a;
  --vp-c-bg-soft: #1e1838;
  --vp-c-bg-soft-up: #16122a;
  --vp-c-divider: #2d2654;
  --vp-c-divider-light: #2d2654;
  --vp-c-text-1: #f5f3ff;
  --vp-c-text-2: #c4b5fd;
  --vp-c-text-3: #8b85a8;
  --vp-c-text-4: #6b6588;
  --vp-c-text-code: #c4b5fd;
  --vp-c-brand-1: #a855f7;
  --vp-c-brand-2: #7c3aed;
  --vp-c-brand-3: #6366f1;
  --vp-c-brand-soft: rgba(168, 85, 247, 0.14);
  --vp-c-gutter: #2d2654;
  --vp-nav-bg: rgba(11, 8, 23, 0.72);
  --vp-nav-height: 64px;
}

/* ---- 2. Hide ALL doc chrome on the home page ---- */

/* Sidebar */
body.vl-home .VPSidebar {
  display: none !important;
}

/* Aside (table of contents / outline on the right) */
body.vl-home .VPDoc .aside,
body.vl-home .VPDoc .aside-container {
  display: none !important;
}

/* Doc footer: "last updated" + prev/next pager */
body.vl-home .VPDocFooter,
body.vl-home .VPDoc .prev-next,
body.vl-home .VPDoc .pager {
  display: none !important;
}

/* Doc outline / edit link */
body.vl-home .VPDoc .edit-link,
body.vl-home .VPDoc .edit-info,
body.vl-home .VPLastUpdated {
  display: none !important;
}

/* Carbon ads (if present) */
body.vl-home .VPCarbonAds {
  display: none !important;
}

/* ---- 3. Remove sidebar/aside class effects ---- */
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

/* ---- 4. Make the doc content full-bleed ---- */
body.vl-home .VPContent {
  padding: 0 !important;
}

body.vl-home .VPContent .VPDoc .container {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
  display: block !important;
}

body.vl-home .VPContent .VPDoc .content {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
  min-width: 0 !important;
}

body.vl-home .VPContent .VPDoc .content-container {
  max-width: none !important;
  margin: 0 !important;
  padding: 0 !important;
}

body.vl-home .vp-doc,
body.vl-home .VPContent .VPDoc main {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
}

body.vl-home .VPContent .VPPage {
  max-width: none !important;
  padding: 0 !important;
  margin: 0 !important;
}

/* ---- 5. Style the nav bar for the dark violet theme ---- */
body.vl-home .VPNavBar {
  background-color: rgba(11, 8, 23, 0.72) !important;
  backdrop-filter: blur(16px) saturate(180%);
  -webkit-backdrop-filter: blur(16px) saturate(180%);
  border-bottom: 1px solid rgba(45, 38, 84, 0.5);
}

/* Nav bar title (logo + site name) */
body.vl-home .VPNavBar .VPNavBarTitle .title {
  color: #f5f3ff !important;
  font-weight: 700;
}

/* Nav bar menu links */
body.vl-home .VPNavBar .VPNavBarMenuLink,
body.vl-home .VPNavBar .menu-item .text,
body.vl-home .VPNavBar .VPNavBarMenu .text {
  color: #c4b5fd !important;
  transition: color 0.2s ease;
}

body.vl-home .VPNavBar .VPNavBarMenuLink:hover,
body.vl-home .VPNavBar .menu-item:hover .text,
body.vl-home .VPNavBar .VPNavBarMenu .text:hover {
  color: #f5f3ff !important;
}

/* Active nav link */
body.vl-home .VPNavBar .VPNavBarMenuLink.active,
body.vl-home .VPNavBar .menu-item.active .text {
  color: #a855f7 !important;
}

/* Search button */
body.vl-home .VPNavBar .VPNavBarSearch .DocSearch-Button,
body.vl-home .VPNavBar .VPNavBarSearch .search {
  background-color: rgba(30, 24, 56, 0.6) !important;
  border: 1px solid rgba(45, 38, 84, 0.6) !important;
  color: #8b85a8 !important;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

body.vl-home .VPNavBar .VPNavBarSearch .DocSearch-Button:hover {
  border-color: rgba(124, 58, 237, 0.4) !important;
  background-color: rgba(30, 24, 56, 0.8) !important;
}

body.vl-home .VPNavBar .VPNavBarSearch .DocSearch-Button .DocSearch-Search-Icon {
  color: #8b85a8 !important;
}

/* GitHub social link icon */
body.vl-home .VPNavBar .VPNavBarSocialLinks .VPNavBarSocialLink {
  color: #c4b5fd !important;
}

body.vl-home .VPNavBar .VPNavBarSocialLinks .VPNavBarSocialLink:hover {
  color: #f5f3ff !important;
}

/* Mobile nav menu button */
body.vl-home .VPNavBar .VPFlyout .button .text,
body.vl-home .VPNavBar .VPNavBarHamburger .container .menu-text {
  color: #c4b5fd !important;
}

/* Mobile nav menu panel */
body.vl-home .VPNavScreen .VPNavScreenMenu {
  background-color: #0b0817 !important;
}

body.vl-home .VPNavScreen .VPNavScreenMenuLink {
  color: #c4b5fd !important;
  border-bottom-color: #2d2654 !important;
}

body.vl-home .VPNavScreen .VPNavScreenMenuLink:hover {
  color: #f5f3ff !important;
}

/* ---- 6. Footer (if it ever appears) ---- */
body.vl-home .VPFooter {
  background: rgba(11, 8, 23, 0.9) !important;
  border-top: 1px solid rgba(45, 38, 84, 0.5);
}

body.vl-home .VPFooter .message,
body.vl-home .VPFooter .copyright {
  color: #8b85a8 !important;
}

/* ---- 7. Remove the has-sidebar body padding ---- */
body.vl-home .Layout {
  padding-left: 0 !important;
}

/* ---- 8. Mobile adjustments ---- */
@media (max-width: 768px) {
  body.vl-home .VPNavBar.has-sidebar .content {
    padding-left: 0 !important;
  }
}
</style>