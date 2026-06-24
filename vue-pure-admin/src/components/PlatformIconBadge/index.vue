<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{ platform?: string | null }>();

const platform = computed(() => String(props.platform || "").trim());
const icon = computed(() => {
  const normalized = platform.value.toLowerCase();
  if (normalized === "youtube") return "/api/uploads/images/youtube.png";
  if (normalized === "tiktok" || normalized === "tik tok") {
    return "/api/uploads/images/tiktok.png";
  }
  if (["instagram", "ins", "ig"].includes(normalized)) {
    return "/api/uploads/images/ins.png";
  }
  return "";
});
</script>

<template>
  <span
    v-if="icon"
    class="platform-icon-badge"
    :title="platform"
    :aria-label="platform"
  >
    <img :src="icon" :alt="platform" />
  </span>
  <el-tag v-else size="small" effect="plain">{{ platform || "Social" }}</el-tag>
</template>

<style scoped>
.platform-icon-badge {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  overflow: hidden;
  border-radius: 0;
}

.platform-icon-badge img {
  display: block;
  width: 24px;
  height: 24px;
  object-fit: contain;
}
</style>
