<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import {
  startTrendCollection,
  getTrendCollection,
  cancelTrendCollection,
  getTrendLoginScreen,
  sendTrendLoginInput,
  restartTrendLogin,
  resumeTrendCollection,
  openTrendDesktop,
  type TrendCollection,
  type TrendQuery,
  type TrendLoginScreen,
  type TrendLoginInput
} from "@/api/trends";

const props = defineProps<{ modelValue: boolean; query: TrendQuery }>();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  complete: [query: TrendQuery];
  active: [value: boolean];
}>();
const visible = computed({
  get: () => props.modelValue,
  set: value => emit("update:modelValue", value)
});
const job = ref<TrendCollection>();
const screen = ref<TrendLoginScreen>();
const selectedTarget = ref("auto");
const restarting = ref(false);
const error = ref("");
const screenError = ref("");
const starting = ref(false);
const openingDesktop = ref(false);

async function openDesktop() {
  if (!job.value || openingDesktop.value) return;
  const popup = window.open("about:blank", "_blank");
  if (!popup) {
    error.value = "请允许此后台打开登录窗口后重试。";
    return;
  }
  popup.opener = null;
  openingDesktop.value = true;
  try {
    const result = await openTrendDesktop(job.value.id);
    const target = new URL(result.data?.url || "", window.location.origin);
    if (
      result.code !== 0 ||
      target.origin !== window.location.origin ||
      target.pathname !== "/trend-desktop/vnc.html"
    )
      throw new Error("desktop unavailable");
    popup.location.replace(target.href);
    error.value = "";
  } catch {
    popup.close();
    error.value =
      "无法打开服务器浏览器，请检查远程桌面配置，或先取消正在执行的采集。";
  } finally {
    openingDesktop.value = false;
  }
}
const active = computed(
  () =>
    !!job.value &&
    !["complete", "failed", "cancelled"].includes(job.value.state)
);
let submittedQuery: TrendQuery;
let disposed = false;
let polling = false;
let screening = false;
let generation = 0;
let statusTimer: ReturnType<typeof setTimeout>;
let screenTimer: ReturnType<typeof setTimeout>;
let inputQueue = Promise.resolve();
let queuedInputs = 0;

async function begin() {
  if (starting.value || active.value) return;
  const run = ++generation;
  starting.value = true;
  error.value = "";
  screen.value = undefined;
  selectedTarget.value = "auto";
  submittedQuery = { ...props.query };
  try {
    const result = await startTrendCollection(submittedQuery);
    if (disposed || run !== generation) return;
    if (result.code !== 0) throw new Error("start failed");
    job.value = result.data;
    emit("active", true);
    poll();
  } catch {
    error.value = "采集任务未启动。请确认采集浏览器已配置，或稍后重试。";
  } finally {
    starting.value = false;
  }
}
async function poll() {
  if (disposed || polling || !active.value) return;
  polling = true;
  const id = job.value.id;
  try {
    const result = await getTrendCollection(id);
    if (disposed || job.value?.id !== id) return;
    if (result.code !== 0) throw new Error("status unavailable");
    job.value = result.data;
    error.value = "";
    if (result.data.state !== "needs_login") screen.value = undefined;
    if (result.data.state === "complete") {
      visible.value = false;
      emit("complete", submittedQuery);
    }
    emit("active", active.value);
  } catch {
    error.value = "暂时无法读取进度，正在重试；请检查后台登录是否有效。";
  } finally {
    polling = false;
    if (!disposed && active.value) statusTimer = setTimeout(poll, 2500);
  }
}
async function updateScreen() {
  if (
    disposed ||
    restarting.value ||
    screening ||
    !visible.value ||
    job.value?.state !== "needs_login" ||
    job.value?.loginMode === "external"
  )
    return;
  screening = true;
  const id = job.value.id;
  try {
    const result = await getTrendLoginScreen(
      id,
      selectedTarget.value === "auto" ? "" : selectedTarget.value
    );
    if (
      disposed ||
      !visible.value ||
      job.value?.id !== id ||
      job.value.state !== "needs_login"
    )
      return;
    if (
      result.code !== 0 ||
      !result.data.image.startsWith("data:image/jpeg;base64,")
    )
      throw new Error("screen unavailable");
    screen.value = result.data;
    screenError.value = "";
  } catch {
    screenError.value =
      "登录画面暂时不可用，正在重试。若登录页已关闭，请选择自动切换。";
  } finally {
    screening = false;
    if (!disposed && visible.value && job.value?.state === "needs_login")
      screenTimer = setTimeout(updateScreen, 1200);
  }
}
function send(data: Omit<TrendLoginInput, "target">) {
  if (!screen.value || job.value?.state !== "needs_login" || queuedInputs >= 64)
    return;
  const id = job.value.id;
  const target = screen.value.target;
  queuedInputs++;
  inputQueue = inputQueue
    .then(async () => {
      if (disposed || job.value?.id !== id || job.value.state !== "needs_login")
        return;
      try {
        const result = await sendTrendLoginInput(id, { ...data, target });
        if (result.code !== 0) throw new Error("input rejected");
      } catch {
        screenError.value = "操作未完成，请重新点击或输入。";
      }
    })
    .finally(() => {
      queuedInputs--;
    });
}
function point(event: MouseEvent | WheelEvent) {
  const box = (event.currentTarget as HTMLElement).getBoundingClientRect();
  return {
    x: Math.min(
      1279,
      Math.max(0, ((event.clientX - box.left) * 1280) / box.width)
    ),
    y: Math.min(
      799,
      Math.max(0, ((event.clientY - box.top) * 800) / box.height)
    )
  };
}
function click(event: MouseEvent) {
  (event.currentTarget as HTMLElement).focus();
  send({ kind: "click", ...point(event) });
}
function wheel(event: WheelEvent) {
  send({
    kind: "wheel",
    ...point(event),
    delta: Math.max(-800, Math.min(800, event.deltaY))
  });
}
function key(event: KeyboardEvent) {
  if (event.isComposing || event.metaKey || event.ctrlKey || event.altKey)
    return;
  if (event.key.length === 1) {
    event.preventDefault();
    send({ kind: "text", text: event.key });
  } else if (
    [
      "Enter",
      "Tab",
      "Backspace",
      "Delete",
      "Escape",
      "ArrowLeft",
      "ArrowRight",
      "ArrowUp",
      "ArrowDown",
      "Home",
      "End"
    ].includes(event.key)
  ) {
    event.preventDefault();
    event.stopPropagation();
    send({ kind: "key", key: event.key });
  }
}
function paste(event: ClipboardEvent) {
  event.preventDefault();
  const text = event.clipboardData?.getData("text/plain") || "";
  if (
    text &&
    new TextEncoder().encode(text).length <= 512 &&
    !/[\u0000-\u001f\u007f]/.test(text)
  )
    send({ kind: "text", text });
}
async function resume() {
  if (!job.value) return;
  try {
    const result = await resumeTrendCollection(job.value.id);
    if (result.code !== 0) throw new Error("resume failed");
    job.value = result.data;
  } catch {
    error.value = "无法继续采集，请确认 Chrome 已打开并允许连接。";
  }
}
async function restartLogin() {
  if (!job.value || restarting.value) return;
  restarting.value = true;
  screen.value = undefined;
  selectedTarget.value = "auto";
  clearTimeout(screenTimer);
  try {
    const result = await restartTrendLogin(job.value.id);
    if (result.code !== 0) throw new Error("login page unavailable");
    screenError.value = "";
  } catch {
    screenError.value = "无法重新打开登录页，请检查 Google 网络连接。";
  } finally {
    restarting.value = false;
    updateScreen();
  }
}
async function cancel() {
  if (!job.value) return;
  try {
    await cancelTrendCollection(job.value.id);
    job.value = { ...job.value, state: "cancelled", message: "已取消采集" };
    screen.value = undefined;
    emit("active", false);
  } catch {
    error.value = "取消未完成，请重试。";
  }
}
watch(
  () => props.modelValue,
  value => {
    if (value) {
      if (!active.value) begin();
      else updateScreen();
    } else {
      screen.value = undefined;
      clearTimeout(screenTimer);
    }
  }
);
watch(
  () => job.value?.state,
  state => {
    if (state === "needs_login" && visible.value) updateScreen();
    else {
      clearTimeout(screenTimer);
      screen.value = undefined;
    }
  }
);
onBeforeUnmount(() => {
  disposed = true;
  generation++;
  clearTimeout(statusTimer);
  clearTimeout(screenTimer);
  screen.value = undefined;
  // Leaving this view must not leave an invisible login task accepting input.
  if (active.value && job.value)
    cancelTrendCollection(job.value.id).catch(() => {});
});
</script>

<template>
  <el-dialog
    v-model="visible"
    title="更新趋势数据"
    width="min(1100px, 96vw)"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    append-to-body
  >
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      show-icon
    />
    <p role="status" class="collector-status">
      {{ starting ? "正在创建采集任务…" : job?.message }}
    </p>
    <el-progress
      v-if="job?.state === 'collecting'"
      :percentage="Math.round((job.completed / Math.max(job.total, 1)) * 100)"
    />
    <div
      v-if="
        job?.desktopAvailable &&
        ['needs_login', 'failed', 'cancelled'].includes(job.state)
      "
      class="collector-help"
    >
      <el-button :loading="openingDesktop" @click="openDesktop"
        >打开服务器浏览器</el-button
      >
      <p>
        在新窗口完成远程桌面认证和 Ads
        登录，再返回此处继续采集。首次使用时，请在服务器 Chrome 的
        chrome://inspect/#remote-debugging 中启用远程调试，并允许连接。
      </p>
    </div>
    <template
      v-if="job?.state === 'needs_login' && job.loginMode === 'external'"
    >
      <el-alert
        :title="
          job.desktopAvailable
            ? '采集连接已断开，请打开服务器浏览器登录 Ads'
            : '采集连接已断开，请在普通 Chrome 窗口登录 Ads'
        "
        type="info"
        :closable="false"
        show-icon
      />
      <p class="collector-help">
        在 Chrome 打开 TikTok Ads 创意中心，通过 Google 手动登录。保持 Chrome
        打开，完成后点击下方按钮，采集会从原筛选条件继续。无需导出 Cookie
        或安装扩展。
      </p>
      <el-button type="primary" @click="resume">登录完成，继续采集</el-button>
    </template>
    <template v-else-if="job?.state === 'needs_login'">
      <el-alert
        title="这是服务器上的专用浏览器。请在 Ads 页面选择 Google 登录；完成后将自动继续采集。"
        type="info"
        :closable="false"
        show-icon
      />
      <p class="collector-help">
        点击下方画面后可直接输入或粘贴，按 Tab
        切换输入框。登录信息仅用于操作当前用户的采集浏览器。
      </p>
      <p class="collector-help">
        若 Google 提示“此浏览器或应用可能不安全”，当前环境无法完成 Google
        登录。请取消采集；只有账号已支持时才使用 Ads 的其他登录方式。
      </p>
      <div class="collector-tabs">
        <el-select
          v-model="selectedTarget"
          aria-label="登录页面"
          style="width: 320px"
        >
          <el-option label="自动切换登录窗口" value="auto" />
          <el-option
            v-for="tab in screen?.tabs || []"
            :key="tab.id"
            :value="tab.id"
            :label="tab.origin"
          />
        </el-select>
        <span>{{ screen?.origin }}</span>
        <el-button :loading="restarting" @click="restartLogin"
          >重新打开登录页</el-button
        >
      </div>
      <el-alert
        v-if="screenError"
        :title="screenError"
        type="warning"
        :closable="false"
      />
      <img
        v-if="screen"
        :src="screen.image"
        class="collector-screen"
        tabindex="0"
        role="application"
        aria-label="远程 Ads 登录浏览器，点击后使用键盘操作"
        draggable="false"
        @click="click"
        @keydown="key"
        @paste="paste"
        @wheel.prevent="wheel"
      />
      <el-skeleton v-else :rows="6" animated />
    </template>
    <template #footer>
      <el-button v-if="active" @click="cancel">取消采集</el-button>
      <el-button v-if="!active && !starting" type="primary" @click="begin"
        >重新采集</el-button
      >
      <el-button @click="visible = false">{{
        active ? "收起窗口" : "关闭"
      }}</el-button>
    </template>
  </el-dialog>
</template>
<style scoped>
.collector-status {
  margin: 16px 0;
}
.collector-help {
  margin: 12px 0;
  color: var(--el-text-color-secondary);
}
.collector-tabs {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 12px 0;
  flex-wrap: wrap;
}
.collector-screen {
  display: block;
  width: 100%;
  aspect-ratio: 8 / 5;
  margin-top: 12px;
  border: 1px solid var(--el-border-color);
  border-radius: 6px;
}
.collector-screen:focus {
  outline: 2px solid var(--el-color-primary);
}
</style>
