import DefaultTheme from "vitepress/theme";
import type { Theme } from "vitepress";
import HomeHero from "./components/HomeHero.vue";
import StatsBar from "./components/StatsBar.vue";
import FeatureGrid from "./components/FeatureGrid.vue";
import CodeShowcase from "./components/CodeShowcase.vue";
import AgentMemory from "./components/AgentMemory.vue";
import CrossPlatform from "./components/CrossPlatform.vue";
import CTASection from "./components/CTASection.vue";
import Layout from "./Layout.vue";

import "./style.css";

export default {
  extends: DefaultTheme,
  Layout,
  enhanceApp({ app }) {
    app.component("HomeHero", HomeHero);
    app.component("StatsBar", StatsBar);
    app.component("FeatureGrid", FeatureGrid);
    app.component("CodeShowcase", CodeShowcase);
    app.component("AgentMemory", AgentMemory);
    app.component("CrossPlatform", CrossPlatform);
    app.component("CTASection", CTASection);

    // Scroll-reveal directive
    app.directive("reveal", {
      mounted(el: HTMLElement) {
        el.classList.add("vl-reveal");
        const io = new IntersectionObserver(
          (entries) => {
            entries.forEach((entry) => {
              if (entry.isIntersecting) {
                el.classList.add("vl-reveal--visible");
                io.unobserve(el);
              }
            });
          },
          { threshold: 0.1, rootMargin: "0px 0px -50px 0px" }
        );
        io.observe(el);
      },
    });
  },
} satisfies Theme;