<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import echarts from "@/plugins/echarts";
import type { TrendPoint } from "@/api/trends";

const props = withDefaults(
  defineProps<{ points: TrendPoint[]; compact?: boolean; label?: string }>(),
  { compact: false, label: "" }
);
const host = ref<HTMLElement>();
let chart: ReturnType<typeof echarts.init>;
let observer: ResizeObserver;
const chartDescription = computed(() => {
  if (!props.points.length) return props.label;
  const first = props.points[0];
  const last = props.points[props.points.length - 1];
  const label = (point: TrendPoint) => {
    const date = new Date(Number(point.timestamp) * 1000);
    return `${Number.isFinite(date.getTime()) ? date.toISOString().slice(0, 10) : "—"}: ${Number(point.value).toFixed(2)}`;
  };
  return `${props.label}. ${label(first)}; ${label(last)}`;
});
function draw() {
  if (!chart) return;
  chart.setOption(
    {
      animation: false,
      grid: props.compact
        ? { top: 6, bottom: 6, left: 2, right: 2 }
        : { top: 22, bottom: 30, left: 42, right: 18 },
      tooltip: { trigger: "axis", renderMode: "richText", confine: true },
      xAxis: {
        type: "category",
        show: !props.compact,
        boundaryGap: false,
        data: props.points.map(p => {
          const date = new Date(Number(p.timestamp) * 1000);
          return Number.isFinite(date.getTime())
            ? date.toISOString().slice(0, 10)
            : "—";
        }),
        axisLabel: { color: "#73776e", hideOverlap: true }
      },
      yAxis: {
        type: "value",
        show: !props.compact,
        axisLabel: { color: "#73776e" },
        splitLine: { lineStyle: { color: "#e9ebe5", type: "dashed" } }
      },
      series: [
        {
          type: "line",
          data: props.points.map(p => Number(p.value) || 0),
          showSymbol: false,
          lineStyle: { color: "#527f70", width: props.compact ? 2 : 2.5 },
          areaStyle: props.compact ? undefined : { color: "#edf4ef" }
        }
      ]
    },
    true
  );
}
onMounted(() => {
  chart = echarts.init(host.value, undefined, { renderer: "svg" });
  observer = new ResizeObserver(() => chart.resize());
  observer.observe(host.value);
  draw();
});
watch(() => props.points, draw, { deep: true });
onBeforeUnmount(() => {
  observer?.disconnect();
  chart?.dispose();
});
</script>

<template>
  <div
    ref="host"
    class="trend-chart"
    :class="{ compact }"
    role="img"
    :aria-label="chartDescription"
  />
</template>

<style scoped>
.trend-chart {
  width: 100%;
  height: 250px;
}
.compact {
  width: 100%;
  height: 52px;
}
@media (prefers-reduced-motion: reduce) {
  .trend-chart {
    scroll-behavior: auto;
  }
}
</style>
