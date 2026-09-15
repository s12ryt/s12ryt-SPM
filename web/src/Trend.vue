<script setup lang="ts">
import { computed } from "vue";
import type { Point } from "./types";
const props = defineProps<{
  points: Point[];
  metric: "cpu" | "memory" | "disk" | "rx" | "tx";
  title: string;
  unit: string;
}>();
const max = computed(() =>
  props.unit === "%"
    ? 100
    : Math.max(1, ...props.points.map((p) => p[props.metric] ?? 0)),
);
const path = computed(() => {
  let pen = false;
  const start = props.points[0]?.time ?? 0;
  const span = Math.max(1, (props.points.at(-1)?.time ?? start) - start);
  return props.points
    .map((p) => {
      const v = p[props.metric];
      if (v === null) {
        pen = false;
        return "";
      }
      const cmd = pen ? "L" : "M";
      pen = true;
      return `${cmd}${20 + ((p.time - start) / span) * 660},${160 - (v / max.value) * 135}`;
    })
    .join(" ");
});
const hasData = computed(() =>
  props.points.some((p) => p[props.metric] !== null),
);
</script>
<template>
  <article class="chart panel">
    <div class="section-title">
      <h3>{{ title }}</h3>
      <span class="muted">{{ unit === "%" ? "0–100%" : "位元組 / 秒" }}</span>
    </div>
    <svg
      v-if="hasData"
      viewBox="0 0 700 185"
      role="img"
      :aria-label="title + '歷史走勢'"
    >
      <path d="M20 25H680 M20 92H680 M20 160H680" class="grid-line" />
      <path :d="path" class="trend-line" />
      <circle
        v-if="points.length === 1"
        cx="20"
        :cy="160 - ((points[0]?.[metric] ?? 0) / max) * 135"
        r="3"
        fill="currentColor"
      />
    </svg>
    <div v-else class="chart-empty">尚無足夠的歷史資料</div>
    <div class="chart-footer">
      <span>{{
        points.length
          ? new Date(points[0]!.time).toLocaleTimeString("zh-TW")
          : "—"
      }}</span
      ><span>{{
        points.length
          ? new Date(points.at(-1)!.time).toLocaleTimeString("zh-TW")
          : "—"
      }}</span>
    </div>
  </article>
</template>
