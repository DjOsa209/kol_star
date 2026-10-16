<script setup lang="ts">
import { useIPLocale } from "@/utils/ipI18n";
const { ipText } = useIPLocale();
import { nextTick, onMounted, onUnmounted, ref, useId, watch } from "vue";

const props = withDefaults(
  defineProps<{ text?: string; fallback?: string }>(),
  { text: "", fallback: "待补充" }
);
const contentId = useId();
const content = ref<HTMLElement>();
const expanded = ref(false);
const overflowing = ref(false);
let observer: ResizeObserver | undefined;

function measure() {
  if (!content.value) return;
  const lineHeight = Number.parseFloat(
    getComputedStyle(content.value).lineHeight
  );
  overflowing.value = content.value.scrollHeight > lineHeight * 6 + 1;
}

watch(
  () => props.text,
  async () => {
    expanded.value = false;
    await nextTick();
    measure();
  }
);
onMounted(() => {
  observer = new ResizeObserver(measure);
  if (content.value) observer.observe(content.value);
});
onUnmounted(() => observer?.disconnect());
</script>

<template>
  <div class="request-text">
    <p
      :id="contentId"
      ref="content"
      class="request-text-content"
      :class="{ collapsed: !expanded, empty: !text?.trim() }"
    >
      {{ text?.trim() ? text : ipText(fallback) }}
    </p>
    <el-button
      v-if="overflowing"
      link
      class="request-text-toggle"
      :aria-expanded="expanded"
      :aria-controls="contentId"
      @click="expanded = !expanded"
    >
      {{ ipText(expanded ? "收起全文" : "展开全文") }}
    </el-button>
  </div>
</template>

<style scoped>
.request-text {
  min-width: 0;
}

.request-text-content {
  max-width: 88ch;
  margin: 0;
  font-size: 14px;
  line-height: 1.8;
  color: #30363b;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.request-text-content.collapsed {
  max-height: calc(6 * 1.8 * 14px);
  overflow: hidden;
}

.request-text-content.empty {
  color: #687078;
}

.request-text-toggle {
  min-height: 32px;
  margin-top: 8px;
  font-size: 13px;
  font-weight: 600;
  color: #526900;
}

.request-text-toggle:hover {
  color: #374700;
}

.request-text-toggle:focus-visible {
  outline: 2px solid #698000;
  outline-offset: 2px;
}
</style>
