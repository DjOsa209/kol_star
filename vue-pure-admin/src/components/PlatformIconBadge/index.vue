<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{ platform?: string | null }>();

const platform = computed(() => String(props.platform || "").trim());
const normalizedPlatform = computed(() => platform.value.toLowerCase());
const icon = computed(() => {
  const normalized = normalizedPlatform.value;
  if (normalized === "youtube") return "/api/uploads/images/youtube.png";
  if (normalized === "tiktok" || normalized === "tik tok") {
    return "/api/uploads/images/tiktok.png";
  }
  if (["instagram", "ins", "ig"].includes(normalized)) {
    return "/api/uploads/images/ins.png";
  }
  if (["小红书", "xiaohongshu", "xhs", "rednote"].includes(normalized)) {
    return "/api/uploads/images/xiaohongshu.png";
  }
  if (["x", "twitter", "x.com"].includes(normalized)) {
    return "/api/uploads/images/x.png";
  }
  if (["facebook", "fb"].includes(normalized)) {
    return "/api/uploads/images/facebook.png";
  }
  if (normalized === "linkedin") {
    return "/api/uploads/images/linkedin.png";
  }
  if (normalized === "reddit") {
    return "/api/uploads/images/reddit.png";
  }
  if (["website", "web", "媒体网站"].includes(normalized)) {
    return "/api/uploads/images/web.png";
  }
  return "";
});
const iconSizeClass = computed(() => {
  if (["x", "twitter", "x.com"].includes(normalizedPlatform.value)) {
    return "platform-icon-badge--x";
  }
  if (["website", "web", "媒体网站"].includes(normalizedPlatform.value)) {
    return "platform-icon-badge--web";
  }
  return "";
});
</script>

<template>
  <span
    v-if="icon"
    :class="['platform-icon-badge', iconSizeClass]"
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

.platform-icon-badge--x img {
  width: 17px;
  height: 17px;
}

.platform-icon-badge--web img {
  width: 14px;
  height: 14px;
}
</style>
