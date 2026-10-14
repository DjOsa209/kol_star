<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { useAppStoreHook } from "@/store/modules/app";
import { useIPRecommendationSessionsStore } from "@/store/modules/ipRecommendationSessions";
import Chat from "~icons/ri/chat-1-line";
import Plus from "~icons/ri/add-line";

defineProps<{ collapsed: boolean }>();
const route = useRoute();
const sessions = useIPRecommendationSessionsStore();
const visible = computed(() => route.path === "/business/ip/recommendations");
const stageLabels = {
  confirm: "需求确认",
  research: "候选研究",
  compare: "比较决策"
};

function openSession(id: number | null) {
  if (sessions.busy) return;
  sessions.navigate(id ? { action: "select", id } : { action: "new" });
  const app = useAppStoreHook();
  if (app.device === "mobile" && app.sidebar.opened) void app.toggleSideBar();
}
</script>

<template>
  <section
    v-if="visible"
    class="ip-menu-sessions"
    :class="{ collapsed }"
    aria-label="项目会话"
  >
    <el-tooltip v-if="collapsed" content="展开项目会话" placement="right">
      <button
        type="button"
        class="session-expand"
        aria-label="展开项目会话"
        @click="useAppStoreHook().toggleSideBar()"
      >
        <IconifyIconOffline :icon="Chat" />
      </button>
    </el-tooltip>
    <template v-else>
      <div class="session-heading">
        <h2>项目会话</h2>
        <span>{{ sessions.subscriptions.length }}</span>
      </div>
      <p v-if="!sessions.loaded" class="session-empty" role="status">
        正在加载会话…
      </p>
      <div v-else-if="sessions.loadError" class="session-empty">
        <p>会话暂时无法加载</p>
        <button
          type="button"
          :disabled="sessions.busy"
          @click="sessions.navigate({ action: 'reload' })"
        >
          重新加载
        </button>
      </div>
      <p v-else-if="!sessions.subscriptions.length" class="session-empty">
        新建会话，开始项目研究。
      </p>
      <nav v-else class="session-list" aria-label="项目会话列表">
        <button
          v-for="item in sessions.subscriptions"
          :key="item.id"
          type="button"
          class="session-item"
          :class="{ selected: sessions.selectedId === item.id }"
          :aria-current="sessions.selectedId === item.id ? 'true' : undefined"
          :title="item.name"
          :disabled="sessions.busy"
          @click="openSession(item.id)"
        >
          <IconifyIconOffline :icon="Chat" />
          <span
            ><strong>{{ item.name }}</strong
            ><small>{{
              item.ipWorkspace?.stage
                ? stageLabels[item.ipWorkspace.stage]
                : "项目研究"
            }}</small></span
          >
        </button>
      </nav>
      <button
        type="button"
        class="session-new"
        :class="{ selected: !sessions.selectedId }"
        :disabled="sessions.busy"
        @click="openSession(null)"
      >
        <IconifyIconOffline :icon="Plus" /><span>新建会话</span>
      </button>
      <p v-if="sessions.busy" class="session-empty" role="status">
        当前操作完成后可切换会话
      </p>
    </template>
  </section>
</template>

<style scoped>
.ip-menu-sessions {
  padding: 20px 12px 12px;
  margin-top: 16px;
  font-size: 13px;
  color: var(--pure-theme-menu-text);
  border-top: 1px solid var(--pure-border-color);
}

.session-heading {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  margin-bottom: 12px;
}

.session-heading h2 {
  margin: 0;
  font-size: 12px;
  font-weight: 500;
}

.session-heading > span {
  font-size: 11px;
}

.session-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: min(36vh, 360px);
  overflow-y: auto;
  scrollbar-color: var(--pure-theme-menu-text) transparent;
  scrollbar-width: thin;
}

.ip-menu-sessions button {
  font: inherit;
  color: inherit;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.session-item,
.session-new {
  display: flex;
  gap: 10px;
  align-items: center;
  width: 100%;
  min-height: 44px;
  padding: 10px 12px;
  text-align: left;
  border-radius: 8px;
}

.ip-menu-sessions svg {
  flex: none;
  width: 18px;
  height: 18px;
}

.session-item > span {
  flex: 1;
  min-width: 0;
}

.session-item strong {
  display: -webkit-box;
  overflow: hidden;
  -webkit-line-clamp: 2;
  font-size: 12px;
  font-weight: 500;
  line-height: 1.6;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
}

.session-item small {
  display: block;
  margin-top: 4px;
  font-size: 11px;
}

.ip-menu-sessions .selected,
.ip-menu-sessions button:hover:not(:disabled) {
  color: var(--pure-theme-sub-menu-active-text);
  background: var(--pure-theme-menu-hover);
}

.session-item.selected {
  border-left: 1px solid var(--pure-theme-menu-active-before);
}

.session-item.selected > svg {
  color: var(--pure-theme-menu-active-before);
}

.session-new {
  margin-top: 8px;
}

.ip-menu-sessions button:focus-visible {
  outline: 2px solid var(--pure-theme-menu-active-before);
  outline-offset: -2px;
}

.ip-menu-sessions button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.session-empty {
  padding: 0 12px;
  font-size: 12px;
  line-height: 1.8;
}

.session-empty > p {
  margin: 0 0 8px;
}

.session-empty > button {
  min-height: 32px;
  text-decoration: underline;
  text-underline-offset: 4px;
}

.collapsed {
  padding: 12px 4px;
}

.session-expand {
  display: grid;
  place-items: center;
  width: 100%;
  min-height: 44px;
  border-radius: 8px;
}
</style>
