<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import { getIPContentTranslations } from "@/api/ip";
import { useIPLocale } from "@/utils/ipI18n";
import type {
  IPTranslatedRecord,
  IPTranslationUpdate
} from "@/utils/ipContentTranslation";
const { ipText } = useIPLocale();
const props = defineProps<{
  type: "ip_resource" | "ip_request";
  id: number;
  records: IPTranslatedRecord[];
  original: boolean;
}>();
const emit = defineEmits<{
  updated: [updates: IPTranslationUpdate[]];
  "update:original": [value: boolean];
}>();
const busy = ref(false);
const networkError = ref(false);
const state = computed(() => {
  const items = props.records.map(record => record.translation).filter(Boolean);
  if (!items.length || items.some(item => item.state === "unavailable"))
    return "unavailable";
  if (items.some(item => item.pending > 0 || item.state === "pending"))
    return "pending";
  if (items.some(item => item.failed > 0 || item.state === "failed"))
    return "failed";
  return "completed";
});
let timer: ReturnType<typeof setTimeout>;
let polls = 0;
let generation = 0;
function schedule() {
  clearTimeout(timer);
  if (state.value === "pending" && polls < 40 && props.id > 0)
    timer = setTimeout(() => void refresh(false), 3000);
}
async function refresh(retry: boolean) {
  if (busy.value) return;
  const token = generation;
  busy.value = true;
  try {
    const response = await getIPContentTranslations(
      props.type,
      props.id,
      retry
    );
    if (token !== generation) return;
    networkError.value = false;
    if (response.code === 0) emit("updated", response.data);
  } catch {
    if (token === generation) networkError.value = true;
  } finally {
    if (token === generation) {
      busy.value = false;
      polls++;
      schedule();
    }
  }
}
watch(
  () => [props.type, props.id],
  () => {
    generation++;
    busy.value = false;
    polls = 0;
    networkError.value = false;
    schedule();
  },
  { immediate: true }
);
watch(state, schedule);
onUnmounted(() => {
  generation++;
  clearTimeout(timer);
});
</script>
<template>
  <div class="ip-translation-notice">
    <span role="status">{{
      ipText(
        networkError
          ? "翻译状态暂时无法更新，原文已保留"
          : state === "pending"
            ? "AI正在生成双语内容，未完成部分暂时显示原文"
            : state === "failed" || state === "unavailable"
              ? "部分内容尚无译文，原文已保留"
              : "双语内容已就绪，按界面语言展示"
      )
    }}</span>
    <el-checkbox
      :model-value="original"
      @update:model-value="value => emit('update:original', Boolean(value))"
      >{{ ipText("查看录入原文") }}</el-checkbox
    >
    <el-button
      v-if="networkError || state !== 'completed'"
      link
      :loading="busy"
      @click="
        polls = 0;
        refresh(state !== 'pending' && !networkError);
      "
      >{{
        ipText(
          state === "pending" || networkError ? "刷新翻译状态" : "重试AI翻译"
        )
      }}</el-button
    >
  </div>
</template>
<style scoped>
.ip-translation-notice {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  align-items: center;
  padding: 10px 0 16px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--el-text-color-regular);
}

.ip-translation-notice > span {
  flex: 1 1 280px;
  min-width: 0;
  overflow-wrap: anywhere;
}

.ip-translation-notice :deep(.el-checkbox) {
  margin: 0;
}
</style>
