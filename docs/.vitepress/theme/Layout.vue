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
/* ============================================================
   HOME PAGE: completely override VitePress default theme.
   The landing page is a custom dark-violet experience — no
   sidebar, no aside, no doc footer, no old theme showing through.
   The nav bar is rebuilt to match the dark glassmorphism aesthetic.
   ============================================================ */

/* ---- 1. Override VitePress CSS variables for dark violet ---- */
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
  /* Kill the old VitePress nav background image (it uses a semi-transparent
     white gradient that shows through under our dark bg) */
  --vp-nav-bg-image: none;
}

/* ---- 2. Hide ALL doc chrome ---- */
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

/* ---- 3. Kill has-sidebar / has-aside effects ---- */
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
/* Remove the left padding VitePress reserves for the sidebar on the layout */
body.vl-home .Layout {
  padding-left: 0 !important;
}

/* ---- 4. Make content full-bleed ---- */
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

/* ---- 5. Rebuild the nav bar for the dark violet theme ---- */

/* The nav bar itself: dark glassmorphism, sticky at top */
body.vl-home .VPNavBar {
  position: sticky !important;
  top: 0;
  z-index: 100;
  background-color: rgba(11, 8, 23, 0.8) !important;
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  border-bottom: 1px solid rgba(45, 38, 84, 0.6);
}

/* Kill the old VitePress background-image gradient on the nav bar */
body.vl-home .VPNavBar,
body.vl-home .VPNavBar .wrapper,
body.vl-home .VPNavBar .container,
body.vl-home .VPNavBar .content,
body.vl-home .VPNavBar .content-body {
  background-image: none !important;
}

/* content-body: VitePress sets this to --vp-c-bg (gray #1b1b1f in dark mode).
   Override to transparent so our nav bar's glassmorphism shows through. */
body.vl-home .VPNavBar .content-body {
  background-color: transparent !important;
}

/* Logo + site name */
body.vl-home .VPNavBar .VPNavBarTitle .title {
  color: #f5f3ff !important;
  font-weight: 700;
  border-bottom: none !important;
}
body.vl-home .VPNavBar .VPNavBarTitle a.title {
  border-bottom: none !important;
}

/* Nav menu links */
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
body.vl-home .VPNavBar .VPNavBarMenuLink.active {
  color: #a855f7 !important;
}

/* Search button */
body.vl-home .VPNavBar .DocSearch-Button {
  background-color: rgba(30, 24, 56, 0.6) !important;
  border: 1px solid rgba(45, 38, 84, 0.6) !important;
  color: #8b85a8 !important;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  box-shadow: none !important;
  transition: border-color 0.2s ease, background-color 0.2s ease;
}
body.vl-home .VPNavBar .DocSearch-Button:hover {
  border-color: rgba(124, 58, 237, 0.4) !important;
  background-color: rgba(30, 24, 56, 0.8) !important;
}
body.vl-home .VPNavBar .DocSearch-Search-Icon {
  color: #8b85a8 !important;
}
body.vl-home .VPNavBar .DocSearch-Button-Placeholder {
  color: #8b85a8 !important;
}
body.vl-home .VPNavBar .DocSearch-Button-Key {
  color: #c4b5fd !important;
  border-color: #2d2654 !important;
}

/* Social links (GitHub icon) */
body.vl-home .VPNavBar .VPNavBarSocialLinks .VPNavBarSocialLink {
  color: #c4b5fd !important;
}
body.vl-home .VPNavBar .VPNavBarSocialLinks .VPNavBarSocialLink:hover {
  color: #f5f3ff !important;
}

/* Appearance toggle (dark/light switch) — restyle to match */
body.vl-home .VPNavBar .VPSwitch {
  border-color: #2d2654 !important;
  background-color: rgba(30, 24, 56, 0.6) !important;
}
body.vl-home .VPNavBar .VPSwitch .vpi-sun,
body.vl-home .VPNavBar .VPSwitch .vpi-moon {
  color: #c4b5fd !important;
}

/* Mobile hamburger / menu text */
body.vl-home .VPNavBar .VPFlyout .button .text,
body.vl-home .VPNavBar .VPNavBarHamburger .container .menu-text {
  color: #c4b5fd !important;
}

/* Mobile nav screen */
body.vl-home .VPNavScreen {
  background-color: #0b0817 !important;
  background-image: none !important;
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

/* ---- 7. Mobile ---- */
@media (max-width: 768px) {
  body.vl-home .VPNavBar.has-sidebar .content {
    padding-left: 0 !important;
  }
}
</style>