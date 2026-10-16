<script setup lang="ts">
import { useIPLocale } from "@/utils/ipI18n";
const { ipText } = useIPLocale();

import { computed, ref } from "vue";
import { useRoute } from "vue-router";
import { useAppStoreHook } from "@/store/modules/app";
import { useIPRecommendationSessionsStore } from "@/store/modules/ipRecommendationSessions";
import Chat from "~icons/ri/chat-1-line";
import Plus from "~icons/ri/add-line";
import More from "~icons/ri/more-line";

defineProps<{ collapsed: boolean }>();
const route = useRoute();
const sessions = useIPRecommendationSessionsStore();
const visible = computed(() => route.path === "/business/ip/recommendations");
const moreMenuId = ref<number | null>(null);

function menuVisibility(id: number, open: boolean) {
  if (open) moreMenuId.value = id;
  else if (moreMenuId.value === id) moreMenuId.value = null;
}

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
    :aria-label="ipText('项目会话')"
  >
    <el-tooltip
      v-if="collapsed"
      :content="ipText('展开项目会话')"
      placement="right"
    >
      <button
        type="button"
        class="session-expand"
        :aria-label="ipText('展开项目会话')"
        @click="useAppStoreHook().toggleSideBar()"
      >
        <IconifyIconOffline :icon="Chat" />
      </button>
    </el-tooltip>
    <template v-else>
      <div class="session-heading">
        <h2>{{ ipText("项目会话") }}</h2>
        <span>{{ ipText(sessions.subscriptions.length) }}</span>
      </div>
      <p v-if="!sessions.loaded" class="session-empty" role="status">
        {{ ipText("正在加载会话…") }}
      </p>
      <div
        v-else-if="sessions.loadError && !sessions.subscriptions.length"
        class="session-empty"
      >
        <p>{{ ipText("会话暂时无法加载") }}</p>
        <button
          type="button"
          :disabled="sessions.busy"
          @click="sessions.navigate({ action: 'reload' })"
        >
          {{ ipText("重新加载") }}
        </button>
      </div>
      <p v-else-if="!sessions.subscriptions.length" class="session-empty">
        {{ ipText("新建会话，开始项目研究。") }}
      </p>
      <nav v-else class="session-list" :aria-label="ipText('项目会话列表')">
        <div
          v-for="item in sessions.subscriptions"
          :key="item.id"
          class="session-row"
          :class="{ 'menu-open': moreMenuId === item.id }"
        >
          <button
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
              ><strong>{{ item.name }}</strong></span
            >
          </button>
          <el-dropdown
            trigger="click"
            placement="bottom-end"
            :disabled="sessions.busy"
            @visible-change="open => menuVisibility(item.id, open)"
            @command="sessions.navigate({ action: 'delete', id: item.id })"
          >
            <button
              type="button"
              class="session-more"
              :aria-label="ipText('{0}的更多操作', [item.name])"
              :aria-expanded="moreMenuId === item.id"
              :disabled="sessions.busy"
            >
              <IconifyIconOffline :icon="More" />
            </button>
            <template #dropdown>
              <el-dropdown-menu
                ><el-dropdown-item command="delete" :disabled="sessions.busy">{{
                  ipText("删除会话")
                }}</el-dropdown-item></el-dropdown-menu
              >
            </template>
          </el-dropdown>
        </div>
      </nav>
      <p
        v-if="
          sessions.loaded && sessions.loadError && sessions.subscriptions.length
        "
        class="session-empty"
        role="status"
      >
        {{ ipText("会话更新失败，已保留现有列表") }}
        <button
          type="button"
          :disabled="sessions.busy"
          @click="sessions.navigate({ action: 'reload' })"
        >
          {{ ipText("重试") }}
        </button>
      </p>
      <button
        type="button"
        class="session-new"
        :class="{ selected: !sessions.selectedId }"
        :disabled="sessions.busy"
        @click="openSession(null)"
      >
        <IconifyIconOffline :icon="Plus" /><span>{{ ipText("新建会话") }}</span>
      </button>
      <p v-if="sessions.busy" class="session-empty" role="status">
        {{ ipText("当前操作完成后可切换会话") }}
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
  gap: 0;
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

.session-row {
  display: flex;
  align-items: center;
  min-width: 0;
}

.session-row .session-item {
  flex: 1;
  width: auto;
  min-width: 0;
}

.session-row :deep(.el-dropdown) {
  display: flex;
  flex: none;
  color: inherit;
}

.session-more {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  margin-right: 4px;
  pointer-events: none;
  opacity: 0;
}

.session-row:hover .session-more,
.session-row:focus-within .session-more,
.session-row.menu-open .session-more {
  pointer-events: auto;
  opacity: 1;
}

@media (hover: none), (pointer: coarse) {
  .session-more {
    width: 44px;
    height: 44px;
    pointer-events: auto;
    opacity: 1;
  }
}

.session-item strong {
  display: -webkit-box;
  overflow: hidden;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  font-size: 13px;
  font-weight: 400;
  line-height: 1.6;
  overflow-wrap: anywhere;
}

.session-new > span {
  min-width: 0;
  line-height: 1.5;
  text-align: left;
  overflow-wrap: anywhere;
}

.ip-menu-sessions .selected,
.ip-menu-sessions button:hover:not(:disabled) {
  color: var(--pure-theme-sub-menu-active-text);
}

.session-item.selected strong,
.session-new.selected {
  font-weight: 600;
}

.session-new {
  margin-top: 8px;
}

.ip-menu-sessions button:focus-visible {
  outline: 1px solid currentcolor;
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
}
</style>
