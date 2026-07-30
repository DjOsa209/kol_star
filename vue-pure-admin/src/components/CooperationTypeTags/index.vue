<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    value?: string | string[] | null;
    emptyText?: string;
  }>(),
  { emptyText: "未设置合作类型" }
);

const tags = computed(() => {
  const source = Array.isArray(props.value)
    ? props.value
    : String(props.value || "").split(/[、,，/；;]+/);
  return Array.from(
    new Set(source.map(item => String(item).trim()).filter(Boolean))
  );
});

function tagType(value: string) {
  if (value.includes("付费")) return "danger";
  if (value.includes("置换")) return "warning";
  if (value.includes("联盟")) return "success";
  if (value.includes("活动")) return "primary";
  return "info";
}

function tagClass(value: string) {
  if (value.includes("付费")) return "is-paid";
  if (value.includes("置换")) return "is-barter";
  return "is-other";
}
</script>

<template>
  <span v-if="tags.length" class="cooperation-type-tags">
    <el-tag
      v-for="tag in tags"
      :key="tag"
      :type="tagType(tag)"
      :class="tagClass(tag)"
      effect="light"
      size="small"
    >
      {{ tag }}
    </el-tag>
  </span>
  <span v-else class="cooperation-type-tags__empty">{{ emptyText }}</span>
</template>

<style scoped>
.cooperation-type-tags {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  vertical-align: middle;
}

.cooperation-type-tags :deep(.el-tag) {
  gap: 4px;
  height: 22px;
  padding: 0 7px;
  font-weight: 600;
  border-width: 1px;
  border-radius: 0;
}

.cooperation-type-tags :deep(.el-tag.is-paid) {
  color: #c2413b;
  background: #fff1f0;
  border-color: #ffc9c5;
}

.cooperation-type-tags :deep(.el-tag.is-barter) {
  color: #a16207;
  background: #fff8df;
  border-color: #f7d982;
}

.cooperation-type-tags__empty {
  color: var(--el-text-color-placeholder);
}
</style>
