<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  listRecommendationNotifications,
  markRecommendationNotificationsRead,
  type RecommendationDomain,
  type RecommendationNotification
} from "@/api/recommendations";
import { useIPWorkflowNotificationsStore } from "@/store/modules/ipWorkflowNotifications";
import type { IPWorkflowNotification } from "@/api/ip";
import BellIcon from "~icons/lucide/bell";

const router = useRouter();
const workflow = useIPWorkflowNotificationsStore();
const { t } = useI18n();
const dropdownRef = ref();
const activeDomain = ref<RecommendationDomain | "workflow">("creator");
const notices = ref<RecommendationNotification[]>([]);
const unread = ref(0);
const loading = ref(false);
let refreshTimer: ReturnType<typeof setInterval> | undefined;

const visibleNotices = computed(() =>
  activeDomain.value === "workflow"
    ? workflow.items.map(item => ({ ...item, domain: "workflow" as const }))
    : notices.value.filter(item => item.domain === activeDomain.value)
);
const unreadInActiveDomain = computed(() =>
  visibleNotices.value.filter(item => !item.readAt)
);
const creatorCount = computed(
  () =>
    notices.value.filter(item => item.domain === "creator" && !item.readAt)
      .length
);
const ipCount = computed(
  () =>
    notices.value.filter(item => item.domain === "ip" && !item.readAt).length
);

async function refresh(silent = false) {
  if (loading.value) return;
  loading.value = true;
  try {
    const response = await listRecommendationNotifications();
    notices.value = response.data.list || [];
    unread.value = Number(response.data.unread) || 0;
  } catch {
    if (!silent) notices.value = [];
  } finally {
    loading.value = false;
  }
}

async function markRead(ids: number[]) {
  if (!ids.length) return;
  await markRecommendationNotificationsRead(ids);
  const idSet = new Set(ids);
  const newlyRead = notices.value.filter(
    item => idSet.has(item.id) && !item.readAt
  ).length;
  const time = new Date().toISOString();
  notices.value.forEach(item => {
    if (idSet.has(item.id)) item.readAt = time;
  });
  unread.value = Math.max(0, unread.value - newlyRead);
}

async function openNotice(
  item:
    | RecommendationNotification
    | (IPWorkflowNotification & { domain: "workflow" })
) {
  if (item.domain === "workflow") {
    try {
      if (!item.readAt) await workflow.markRead([item.id]);
    } finally {
      dropdownRef.value?.handleClose();
      if (
        /^\/business\/ip\/(requests|feedback|marketing)\?id=\d+$/.test(
          item.targetPath
        )
      )
        router.push(item.targetPath);
    }
    return;
  }
  try {
    if (!item.readAt) await markRead([item.id]);
  } finally {
    dropdownRef.value?.handleClose();
    const fallback =
      item.domain === "ip"
        ? "/business/ip/recommendations"
        : "/business/assistant";
    const validTarget =
      item.domain === "ip"
        ? item.targetPath?.startsWith("/business/ip/recommendations")
        : item.targetPath?.startsWith("/business/assistant");
    router.push(validTarget ? item.targetPath : fallback);
  }
}

async function markDomainRead() {
  try {
    const ids = unreadInActiveDomain.value.map(item => item.id);
    if (activeDomain.value === "workflow") await workflow.markRead(ids);
    else await markRead(ids);
  } catch {
    // Keep the unread marker when the server rejects the update.
  }
}

function onVisibleChange(visible: boolean) {
  if (visible) {
    if (
      /^\/business\/ip\/(requests|feedback|marketing)/.test(
        router.currentRoute.value.path
      )
    )
      activeDomain.value = "workflow";
    refresh(true);
    if (workflow.owner) void workflow.refresh().catch(() => {});
  }
}

function formatTime(value: string) {
  return String(value || "")
    .replace("T", " ")
    .slice(0, 16);
}

function openDomain() {
  dropdownRef.value?.handleClose();
  router.push(
    activeDomain.value === "workflow"
      ? "/business/ip/requests"
      : activeDomain.value === "ip"
        ? "/business/ip/recommendations"
        : "/business/assistant"
  );
}

onMounted(() => {
  refresh(true);
  refreshTimer = setInterval(() => refresh(true), 60000);
});
onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>

<template>
  <el-dropdown
    ref="dropdownRef"
    trigger="click"
    placement="bottom-end"
    @visible-change="onVisibleChange"
  >
    <span
      class="dropdown-badge navbar-bg-hover select-none mr-1.75"
      aria-label="消息提醒"
    >
      <el-badge
        :value="unread + workflow.unread"
        :max="99"
        :hidden="unread + workflow.unread === 0"
      >
        <span class="header-notice-icon"
          ><IconifyIconOffline :icon="BellIcon"
        /></span>
      </el-badge>
    </span>
    <template #dropdown>
      <el-dropdown-menu class="recommendation-notice-menu">
        <div class="notice-head">
          <strong>消息提醒</strong>
        </div>
        <el-tabs v-model="activeDomain" stretch class="notice-tabs">
          <el-tab-pane
            name="workflow"
            :label="`IP需求 ${workflow.unread || ''}`"
          />
          <el-tab-pane
            name="creator"
            :label="`${t('menus.creatorRecommendations')} ${creatorCount || ''}`"
          />
          <el-tab-pane
            name="ip"
            :label="`${t('menus.ipRecommendations')} ${ipCount || ''}`"
          />
        </el-tabs>
        <el-scrollbar max-height="350px" class="notice-scroll">
          <el-empty
            v-if="!visibleNotices.length"
            :description="
              activeDomain === 'workflow' ? '暂无需求提醒' : '暂无新推荐提醒'
            "
            :image-size="60"
          />
          <button
            v-for="item in visibleNotices"
            :key="`${item.domain}-${item.id}`"
            class="notice-item"
            :class="{ unread: !item.readAt }"
            type="button"
            @click.stop="openNotice(item)"
          >
            <span class="notice-title"
              ><i v-if="!item.readAt" />{{ item.title }}</span
            >
            <span class="notice-description">{{ item.description }}</span>
            <time>{{ formatTime(item.createdAt) }}</time>
          </button>
        </el-scrollbar>
        <div class="notice-foot">
          <el-button size="small" text @click.stop="markDomainRead">{{
            t("buttons.pureMarkAsRead")
          }}</el-button>
          <el-button size="small" text @click.stop="openDomain">{{
            activeDomain === "workflow" ? "查看需求 →" : "查看推荐 →"
          }}</el-button>
        </div>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<style lang="scss" scoped>
@keyframes pure-bell-ring {
  0%,
  100% {
    transform-origin: top;
  }

  15% {
    transform: rotateZ(10deg);
  }

  30% {
    transform: rotateZ(-10deg);
  }

  45% {
    transform: rotateZ(5deg);
  }

  60% {
    transform: rotateZ(-5deg);
  }

  75% {
    transform: rotateZ(2deg);
  }
}

.dropdown-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 48px;
  cursor: pointer;
}

.header-notice-icon {
  font-size: 16px;
}

.dropdown-badge:hover .header-notice-icon :deep(svg) {
  animation: pure-bell-ring 1s both;
}

.recommendation-notice-menu {
  width: 360px;
  padding: 0;
}

.notice-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 17px 3px;
}

.notice-head strong {
  font-size: 14px;
}

.notice-head small {
  color: #92999a;
}

.notice-tabs {
  padding: 0 12px;
}

.notice-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.notice-scroll {
  padding: 4px 12px;
}

.notice-item {
  display: flex;
  flex-direction: column;
  gap: 5px;
  width: 100%;
  padding: 11px 10px;
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: 0;
  border-bottom: 1px solid #ebede7;
}

.notice-item:hover {
  background: #f7f9ed;
}

.notice-item.unread {
  background: #faffeb;
}

.notice-title {
  display: flex;
  gap: 6px;
  align-items: center;
  font-size: 13px;
  font-weight: 700;
  color: #161a1d;
}

.notice-title i {
  flex: none;
  width: 6px;
  height: 6px;
  background: #a5d900;
  border-radius: 50%;
}

.notice-description {
  display: -webkit-box;
  overflow: hidden;
  -webkit-line-clamp: 2;
  font-size: 12px;
  line-height: 1.5;
  color: #747d7f;
  -webkit-box-orient: vertical;
}

.notice-item time {
  font-size: 11px;
  color: #9ca4a5;
}

.notice-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 10px;
  border-top: 1px solid #e8eae3;
}
</style>
