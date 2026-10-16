<script setup lang="ts">
import { useIPLocale } from "@/utils/ipI18n";
const { ipText } = useIPLocale();

import { computed } from "vue";
import type { IPStageDeadline } from "@/api/ip";
const props = defineProps<{ deadline?: IPStageDeadline; compact?: boolean }>();
const status = computed(() => {
  const d = props.deadline;
  if (!d) return "待核定";
  switch (d.state) {
    case "waiting":
      return "上一环节完成后开始计时";
    case "unknown_start":
      return "历史需求 · 阶段时间缺失";
    case "calendar_pending":
      return "年度日历待核定";
    case "completed":
      return d.late
        ? "已反馈 · 逾期提交"
        : d.warning
          ? "已反馈 · 时效待核定"
          : "已按时反馈";
    case "due_today":
      return "今日截止";
    case "overdue":
      return d.overdueWorkdays
        ? ipText("已逾期 {0} 个工作日", [d.overdueWorkdays])
        : "已逾期";
    default:
      return ipText("剩余 {0} 个工作日", [d.remainingWorkdays]);
  }
});
// Always show Beijing civil time, regardless of the viewer's browser timezone.
const dueText = computed(() =>
  props.deadline?.dueAt?.slice(0, 16).replace("T", " ")
);
</script>
<template>
  <div class="ip-deadline" :class="{ compact }">
    <span class="deadline-label"
      >{{ ipText(deadline?.label || ipText("反馈"))
      }}<small v-if="!compact"
        >{{ ipText(deadline?.workingDays) }} {{ ipText("个工作日") }}</small
      ></span
    >
    <time v-if="dueText" :datetime="deadline?.dueAt">{{
      ipText("{0} 前", [dueText])
    }}</time>
    <span
      class="deadline-state"
      :class="{ late: deadline?.late, urgent: deadline?.state === 'due_today' }"
      >{{ ipText(status) }}</span
    >
    <small v-if="deadline?.warning" class="deadline-warning">{{
      ipText(deadline.warning)
    }}</small>
  </div>
</template>
<style scoped lang="scss">
.ip-deadline {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  font-size: 14px;
  line-height: 1.5;
  color: var(--el-text-color-primary);
}

.deadline-label {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: baseline;
  font-weight: 600;
}

.deadline-label small,
.deadline-warning {
  font-size: 12px;
  font-weight: 400;
  color: var(--el-text-color-regular);
}

time {
  font-variant-numeric: tabular-nums;
}

.deadline-state {
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.deadline-state.late {
  color: color-mix(
    in srgb,
    var(--el-text-color-primary) 40%,
    var(--el-color-danger) 60%
  );
}

.deadline-state.urgent {
  color: color-mix(
    in srgb,
    var(--el-text-color-primary) 40%,
    var(--el-color-warning) 60%
  );
}

.compact {
  gap: 3px;
  font-size: 12px;
}

.compact .deadline-warning {
  max-width: 24ch;
}
</style>
