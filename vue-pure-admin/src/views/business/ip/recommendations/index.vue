<script setup lang="ts">
import {
  computed,
  nextTick,
  onMounted,
  onUnmounted,
  reactive,
  ref,
  watch
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import VuePdfEmbed from "vue-pdf-embed";
import {
  deleteRecommendationSubscription,
  listRecommendationResults,
  listRecommendationSubscriptions,
  runRecommendationSubscription,
  saveRecommendationSubscription,
  sendRecommendationFeedback,
  type RecommendationResult,
  type RecommendationSubscription,
  type RecommendationSubscriptionInput
} from "@/api/recommendations";
import { getIPRequest, getIPResource, readIPFile } from "@/api/ip";

defineOptions({ name: "IPRecommendations" });

const route = useRoute();
const router = useRouter();
const loading = ref(false);
const saving = ref(false);
const running = ref(false);
const subscriptions = ref<RecommendationSubscription[]>([]);
const results = ref<RecommendationResult[]>([]);
const selectedId = ref<number | null>(null);
const activeView = ref<"new" | "all" | "subscriptions">("all");
const editing = ref(false);
const imageUrls = ref<Record<number, string>>({});
const currentRequestName = ref("");
const chosenIds = ref<number[]>([]);
const drawerVisible = ref(false);
const drawerLoading = ref(false);
const drawerDetail = ref<{ resource: any; files: any[]; cases: any[] } | null>(
  null
);
const drawerVisualUrl = ref("");
const previewVisible = ref(false);
const previewUrl = ref("");
const previewFileName = ref("");
const previewPages = ref(0);
const previewLoading = ref(false);
const previewError = ref("");
let disposed = false;
const markets = [
  "中国",
  "东南亚",
  "南亚",
  "欧洲",
  "中东",
  "非洲",
  "拉美",
  "北美",
  "全球"
];
const ipTypes = [
  "游戏",
  "动画",
  "影视",
  "综艺",
  "体育",
  "潮牌",
  "潮玩",
  "品牌",
  "文化艺术",
  "线下活动",
  "其他"
];
const goals = ["新品发布", "品牌声量", "用户增长", "线下活动", "其他"];
const selectionKey = computed(() => {
  const key = String(route.query.selectionKey || "");
  return /^ip-request-[0-9a-f-]{36}$/i.test(key) ? key : "";
});
const requestId = computed(() => Number(route.query.requestId) || 0);
const selectedSubscription = computed(() =>
  subscriptions.value.find(item => item.id === selectedId.value)
);
const activeCount = computed(
  () => subscriptions.value.filter(item => item.active).length
);
const newCount = computed(
  () => results.value.filter(item => item.isNew && !item.feedback).length
);
const visibleResults = computed(() =>
  activeView.value === "new"
    ? results.value.filter(
        item => item.isNew && item.feedback !== "not_interested"
      )
    : results.value.filter(item => item.feedback !== "not_interested")
);
const resultCountText = computed(() =>
  activeView.value === "new"
    ? `${visibleResults.value.length} 条新推荐`
    : `${visibleResults.value.length} 条推荐`
);

function blankForm(): RecommendationSubscriptionInput {
  return {
    id: undefined,
    domain: "ip",
    name: "",
    query: "",
    markets: [],
    ipTypes: [],
    audience: "",
    goal: "",
    budgetCurrency: "CNY",
    budgetMin: null,
    budgetMax: null,
    active: true
  };
}
const form = reactive<RecommendationSubscriptionInput>(blankForm());

function parseTextArray(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String);
  try {
    const parsed = JSON.parse(String(value || "[]"));
    return Array.isArray(parsed) ? parsed.map(String) : [];
  } catch {
    return [];
  }
}

function normalizeIPTypes(value: unknown): string[] {
  const selected = new Set<string>();
  for (const raw of parseTextArray(value)) {
    if (ipTypes.includes(raw)) selected.add(raw);
    if (/体育|赛事/.test(raw)) selected.add("体育");
    if (/动漫|动画/.test(raw)) selected.add("动画");
    if (/影视/.test(raw)) selected.add("影视");
    if (/音乐|演唱会|活动/.test(raw)) selected.add("线下活动");
    if (/游戏/.test(raw)) selected.add("游戏");
    if (/电竞/.test(raw)) selected.add("游戏");
    if (/文化|艺术/.test(raw)) selected.add("文化艺术");
  }
  return [...selected];
}

function parseObject(value: unknown): Record<string, any> {
  if (value && typeof value === "object" && !Array.isArray(value))
    return value as Record<string, any>;
  try {
    const parsed = JSON.parse(String(value || "{}"));
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed
      : {};
  } catch {
    return {};
  }
}

function setForm(value: Partial<RecommendationSubscriptionInput>) {
  Object.assign(form, blankForm(), value, {
    domain: "ip",
    markets: parseTextArray(value.markets),
    ipTypes: normalizeIPTypes(value.ipTypes)
  });
}

function priceText(resource: Record<string, any>) {
  const min = resource.priceMin;
  const max = resource.priceMax;
  if (min == null && max == null) return "价格待咨询";
  const code = String(resource.currency || "CNY");
  const symbol = code === "USD" ? "$" : code === "EUR" ? "€" : "¥";
  if (min != null && max != null)
    return `${symbol}${Number(min).toLocaleString()}–${symbol}${Number(max).toLocaleString()}`;
  if (min != null) return `${symbol}${Number(min).toLocaleString()} 起`;
  return `${symbol}${Number(max).toLocaleString()} 以内`;
}

function riskItems(value: RecommendationResult["risks"]): string[] {
  if (Array.isArray(value)) return value.filter(Boolean);
  return value ? [String(value)] : [];
}

function formatTime(value: string | undefined | null) {
  return value ? String(value).replace("T", " ").slice(0, 16) : "尚未运行";
}

async function openResourceDrawer(id: number) {
  drawerVisible.value = true;
  drawerLoading.value = true;
  drawerDetail.value = null;
  if (drawerVisualUrl.value) URL.revokeObjectURL(drawerVisualUrl.value);
  drawerVisualUrl.value = "";
  try {
    drawerDetail.value = (await getIPResource(id)).data;
    const visual = drawerDetail.value.files.find(
      file => file.fileKind === "visual"
    );
    if (visual)
      drawerVisualUrl.value = URL.createObjectURL(
        await readIPFile(Number(visual.id))
      );
  } catch {
    drawerVisible.value = false;
    ElMessage.error("IP 档案加载失败，请稍后重试");
  } finally {
    drawerLoading.value = false;
  }
}

async function openDocument(file: any) {
  previewLoading.value = true;
  previewError.value = "";
  previewFileName.value = String(file.originalName || "版权资料.pdf");
  previewPages.value = 0;
  try {
    const blob = await readIPFile(Number(file.id));
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value);
    previewUrl.value = URL.createObjectURL(
      new Blob([blob], { type: "application/pdf" })
    );
    previewVisible.value = true;
  } catch {
    previewLoading.value = false;
    ElMessage.error("资料读取失败，请稍后重试");
  }
}

function closePreview() {
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value);
  previewUrl.value = "";
}

function pdfFailed() {
  previewLoading.value = false;
  previewError.value = "PDF 无法预览，请下载文件查看";
}

async function loadSubscriptions(selectId?: number) {
  const response = await listRecommendationSubscriptions("ip");
  subscriptions.value = response.data.list || [];
  const requested = selectId || Number(route.query.subscriptionId);
  if (requested && subscriptions.value.some(item => item.id === requested))
    selectedId.value = requested;
  else if (
    !selectedId.value ||
    !subscriptions.value.some(item => item.id === selectedId.value)
  )
    selectedId.value = subscriptions.value[0]?.id || null;
}

async function loadResults() {
  if (!selectedId.value) {
    results.value = [];
    return;
  }
  loading.value = true;
  try {
    const response = await listRecommendationResults(
      "ip",
      selectedId.value,
      "all"
    );
    results.value = response.data.list || [];
    void loadImages(results.value);
    const focusId = Number(route.query.resultId);
    if (focusId > 0 && results.value.some(item => item.id === focusId)) {
      activeView.value = "all";
      await nextTick();
      document
        .getElementById(`ip-recommendation-${focusId}`)
        ?.scrollIntoView({ block: "center" });
    }
  } finally {
    loading.value = false;
  }
}

async function loadImages(items: RecommendationResult[]) {
  await Promise.allSettled(
    items.map(async item => {
      const fileId = Number(item.resource?.visualFileId);
      if (!fileId || imageUrls.value[fileId]) return;
      const blob = await readIPFile(fileId);
      if (disposed) return;
      imageUrls.value[fileId] = URL.createObjectURL(blob);
    })
  );
}

function selectSubscription(id: number) {
  selectedId.value = id;
  editing.value = false;
  activeView.value = "all";
  const query = { ...route.query };
  delete query.resultId;
  query.subscriptionId = String(id);
  router.replace({
    path: route.path,
    query
  });
  loadResults();
}

function startNew() {
  setForm({ requestId: requestId.value || undefined });
  editing.value = true;
  activeView.value = "subscriptions";
}

function editSubscription(item: RecommendationSubscription) {
  setForm({
    ...item,
    budgetMin: item.budgetMin || null,
    budgetMax: item.budgetMax || null
  });
  editing.value = true;
  activeView.value = "subscriptions";
}

async function saveSubscription() {
  if (!form.name.trim() || !form.query.trim()) {
    ElMessage.warning("请填写寻找条件名称与需求描述");
    return;
  }
  if (
    form.budgetMin != null &&
    form.budgetMax != null &&
    form.budgetMin > form.budgetMax
  ) {
    ElMessage.warning("最低预算不能大于最高预算");
    return;
  }
  saving.value = true;
  try {
    const response = await saveRecommendationSubscription({
      ...form,
      name: form.name.trim(),
      query: form.query.trim(),
      markets: form.markets || [],
      ipTypes: form.ipTypes || []
    });
    editing.value = false;
    activeView.value = "all";
    await loadSubscriptions(response.data.id);
    await loadResults();
    if (response.data.warning) ElMessage.warning(response.data.warning);
    else ElMessage.success("寻找条件已保存，推荐已更新");
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "保存寻找条件失败"
    );
  } finally {
    saving.value = false;
  }
}

async function runNow() {
  if (!selectedId.value) return;
  running.value = true;
  try {
    await runRecommendationSubscription("ip", selectedId.value);
    await loadSubscriptions(selectedId.value);
    await loadResults();
    activeView.value = "all";
    ElMessage.success("已完成本次推荐检查");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "推荐检查失败");
  } finally {
    running.value = false;
  }
}

async function toggleSubscription(item: RecommendationSubscription) {
  try {
    await saveRecommendationSubscription({
      ...item,
      domain: "ip",
      active: !item.active
    });
    await loadSubscriptions(item.id);
    ElMessage.success(item.active ? "已暂停每周推荐" : "已恢复每周推荐");
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "更新提醒状态失败"
    );
  }
}

async function removeSubscription(item: RecommendationSubscription) {
  try {
    await ElMessageBox.confirm(
      `删除寻找条件「${item.name}」及其推荐历史？`,
      "删除确认",
      { type: "warning" }
    );
  } catch {
    return;
  }
  try {
    await deleteRecommendationSubscription("ip", item.id);
    await loadSubscriptions();
    await loadResults();
    ElMessage.success("寻找条件已删除");
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "删除寻找条件失败"
    );
  }
}

async function feedback(
  item: RecommendationResult,
  value: "interested" | "not_interested"
) {
  try {
    await sendRecommendationFeedback("ip", item.id, value);
    item.feedback = value;
    item.isNew = false;
    ElMessage.success(
      value === "interested" ? "已标记感兴趣" : "已减少类似推荐"
    );
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "保存反馈失败");
  }
}

function selectedFromRequest(): number[] {
  try {
    const saved = localStorage.getItem(selectionKey.value);
    const value = saved
      ? JSON.parse(saved)
      : chosenIds.value.length
        ? chosenIds.value
        : String(route.query.selected || "").split(",");
    return Array.isArray(value) ? value.map(Number).filter(id => id > 0) : [];
  } catch {
    return [];
  }
}

async function addToIntent(item: RecommendationResult) {
  const id = Number(item.resourceId);
  if (!id) return;
  if (selectionKey.value) {
    const selected = selectedFromRequest();
    if (selected.includes(id)) {
      ElMessage.info("该 IP 已在当前意向清单中");
      return;
    }
    if (selected.length >= 5) {
      ElMessage.warning("初步意向最多选择 5 个 IP");
      return;
    }
    try {
      await ElMessageBox.confirm(
        `将「${item.resourceName}」加入当前需求的初步意向？`,
        "加入意向",
        { type: "info" }
      );
    } catch {
      return;
    }
    chosenIds.value = [...selected, id];
    localStorage.setItem(selectionKey.value, JSON.stringify(chosenIds.value));
    ElMessage.success("已加入意向清单，请返回需求页确认并保存");
    return;
  }
  try {
    await ElMessageBox.confirm(
      `将「${item.resourceName}」加入新 IP 需求的初步意向？`,
      "新建需求",
      { type: "info" }
    );
  } catch {
    return;
  }
  router.push({
    path: "/business/ip/requests",
    query: {
      mode: "new",
      recommendedIp: String(id),
      recommendationReason: item.reason
    }
  });
}

async function loadRequestContext() {
  if (selectionKey.value) {
    try {
      const data = JSON.parse(
        localStorage.getItem(`${selectionKey.value}-recommendation-criteria`) ||
          "null"
      );
      if (data && typeof data === "object") {
        setForm({
          ...data,
          domain: "ip",
          requestId: requestId.value || undefined
        });
        editing.value = true;
        activeView.value = "subscriptions";
        return;
      }
    } catch {
      // A malformed local draft should not block access to saved recommendations.
    }
  }
  if (requestId.value) {
    try {
      const response = await getIPRequest(requestId.value);
      const item = response.data.request;
      const brief = parseObject(item.brief);
      currentRequestName.value = item.projectName || "";
      setForm({
        name: item.projectName ? `${item.projectName} · IP寻找` : "IP寻找",
        query: item.description || "",
        markets: parseTextArray(item.markets),
        ipTypes: brief.targetIPType ? [String(brief.targetIPType)] : [],
        goal: item.goal || "",
        budgetCurrency: item.budgetCurrency || "CNY",
        budgetMin: item.budgetMin == null ? null : Number(item.budgetMin),
        budgetMax: item.budgetMax == null ? null : Number(item.budgetMax),
        requestId: requestId.value
      });
      editing.value = true;
      activeView.value = "subscriptions";
    } catch {
      ElMessage.warning("未能读取关联需求，请手动填写寻找条件");
    }
  }
}

onMounted(async () => {
  try {
    await loadSubscriptions();
    await loadResults();
    await loadRequestContext();
  } catch {
    ElMessage.error("IP 推荐暂时无法加载，请稍后重试");
  }
});
watch(
  () =>
    String(route.query.subscriptionId || "") +
    ":" +
    String(route.query.resultId || ""),
  async () => {
    const id = Number(route.query.subscriptionId);
    if (id > 0 && subscriptions.value.some(item => item.id === id))
      selectedId.value = id;
    editing.value = false;
    await loadResults();
  }
);
onUnmounted(() => {
  disposed = true;
  for (const url of Object.values(imageUrls.value)) URL.revokeObjectURL(url);
  if (drawerVisualUrl.value) URL.revokeObjectURL(drawerVisualUrl.value);
  closePreview();
});
</script>

<template>
  <div class="ip-recommend-page">
    <section class="hero">
      <div>
        <p class="eyebrow">IP OPERATIONS / DISCOVERY</p>
        <h1>找到更契合的 IP</h1>
        <p class="hero-description">
          描述合作方向，系统从现有 IP 资源库寻找匹配对象，每周为你检查新资源。
        </p>
        <p v-if="currentRequestName" class="context-pill">
          来自需求：{{ currentRequestName }}
        </p>
      </div>
      <div class="hero-actions">
        <div class="hero-stat">
          <strong>{{ activeCount }}</strong
          ><span>运行中的寻找条件</span>
        </div>
        <el-button class="primary-button" @click="startNew"
          >＋ 新建寻找条件</el-button
        >
      </div>
    </section>

    <div class="workspace">
      <aside class="subscription-panel">
        <div class="section-heading">
          <h2>我的寻找条件</h2>
          <span>{{ subscriptions.length }}</span>
        </div>
        <div v-if="!subscriptions.length" class="sidebar-empty">
          尚未保存寻找条件。填写一次，后续每周自动检查新匹配。
        </div>
        <button
          v-for="item in subscriptions"
          :key="item.id"
          class="subscription-item"
          :class="{ selected: selectedId === item.id && !editing }"
          type="button"
          @click="selectSubscription(item.id)"
        >
          <span class="subscription-title">{{ item.name }}</span>
          <span class="subscription-query">{{ item.query }}</span>
          <span class="subscription-foot"
            ><i :class="{ paused: !item.active }" />{{
              item.active ? "每周检查中" : "已暂停"
            }}<small>{{ formatTime(item.lastRunAt) }}</small></span
          >
        </button>
      </aside>

      <main class="main-panel">
        <template v-if="editing">
          <div class="section-heading">
            <div>
              <p class="eyebrow">SEARCH BRIEF</p>
              <h2>{{ form.id ? "编辑寻找条件" : "告诉我们你想找什么" }}</h2>
            </div>
          </div>
          <div class="editor-card">
            <div class="editor-grid">
              <label
                >条件名称 *<el-input
                  v-model="form.name"
                  maxlength="80"
                  placeholder="例如：东南亚年轻用户向游戏 IP"
              /></label>
              <label
                >合作目标<el-select
                  v-model="form.goal"
                  clearable
                  placeholder="请选择合作目标"
                  ><el-option
                    v-for="item in goals"
                    :key="item"
                    :label="item"
                    :value="item" /></el-select
              ></label>
              <label class="span-2"
                >需求描述 *<el-input
                  v-model="form.query"
                  type="textarea"
                  :rows="3"
                  maxlength="2000"
                  show-word-limit
                  placeholder="描述产品、目标受众、联动方式和希望达到的效果"
              /></label>
              <label
                >目标市场<el-select
                  v-model="form.markets"
                  multiple
                  filterable
                  placeholder="可选择多个市场"
                  ><el-option
                    v-for="item in markets"
                    :key="item"
                    :label="item"
                    :value="item" /></el-select
              ></label>
              <label
                >IP 类型<el-select
                  v-model="form.ipTypes"
                  multiple
                  filterable
                  placeholder="不限类型"
                  ><el-option
                    v-for="item in ipTypes"
                    :key="item"
                    :label="item"
                    :value="item" /></el-select
              ></label>
              <label class="span-2"
                >目标受众<el-input
                  v-model="form.audience"
                  placeholder="例如：18–30 岁，偏好电竞与手机游戏"
              /></label>
              <label class="span-2"
                >授权费用预算
                <div class="budget">
                  <el-select v-model="form.budgetCurrency"
                    ><el-option label="CNY ¥" value="CNY" /><el-option
                      label="USD $"
                      value="USD" /><el-option
                      label="EUR €"
                      value="EUR" /></el-select
                  ><el-input-number
                    v-model="form.budgetMin"
                    :min="0"
                    :controls="false"
                    placeholder="最低"
                  /><span>至</span
                  ><el-input-number
                    v-model="form.budgetMax"
                    :min="0"
                    :controls="false"
                    placeholder="最高"
                  /></div
              ></label>
            </div>
            <div class="editor-footer">
              <span
                >保存后立即生成推荐；每周一 09:00 检查新匹配并在站内提醒。</span
              >
              <div>
                <el-button
                  v-if="subscriptions.length"
                  @click="
                    editing = false;
                    activeView = 'all';
                  "
                  >取消</el-button
                ><el-button
                  class="primary-button"
                  :loading="saving"
                  @click="saveSubscription"
                  >保存并获取推荐 →</el-button
                >
              </div>
            </div>
          </div>
        </template>

        <template v-else>
          <div class="section-heading result-heading">
            <div>
              <p class="eyebrow">RECOMMENDATION FEED</p>
              <h2>{{ selectedSubscription?.name || "IP 推荐" }}</h2>
              <p v-if="selectedSubscription" class="heading-hint">
                {{ selectedSubscription.query }}
              </p>
            </div>
            <div v-if="selectedSubscription" class="heading-actions">
              <el-button @click="editSubscription(selectedSubscription)"
                >编辑条件</el-button
              ><el-button :loading="running" @click="runNow"
                >立即刷新</el-button
              >
            </div>
          </div>
          <div v-if="selectedSubscription" class="result-toolbar">
            <div class="view-tabs">
              <button
                type="button"
                :class="{ active: activeView === 'new' }"
                @click="activeView = 'new'"
              >
                本周新推荐 <b>{{ newCount }}</b></button
              ><button
                type="button"
                :class="{ active: activeView === 'all' }"
                @click="activeView = 'all'"
              >
                全部结果</button
              ><button
                type="button"
                :class="{ active: activeView === 'subscriptions' }"
                @click="activeView = 'subscriptions'"
              >
                条件管理
              </button>
            </div>
            <span>{{
              activeView === "subscriptions" ? "管理自动检查" : resultCountText
            }}</span>
          </div>

          <div v-if="!selectedSubscription" class="empty-state">
            <strong>从一个寻找条件开始</strong>
            <p>
              告诉系统你需要的市场、受众和授权预算，推荐将基于真实 IP 档案生成。
            </p>
            <el-button class="primary-button" @click="startNew"
              >创建寻找条件</el-button
            >
          </div>
          <div
            v-else-if="activeView === 'subscriptions'"
            class="management-card"
          >
            <div>
              <h3>{{ selectedSubscription.name }}</h3>
              <p>{{ selectedSubscription.query }}</p>
              <small
                >最近检查：{{ formatTime(selectedSubscription.lastRunAt) }} ·
                {{
                  selectedSubscription.active ? "每周检查中" : "已暂停"
                }}</small
              >
            </div>
            <div class="management-actions">
              <el-button @click="toggleSubscription(selectedSubscription)">{{
                selectedSubscription.active ? "暂停提醒" : "恢复提醒"
              }}</el-button
              ><el-button
                type="danger"
                plain
                @click="removeSubscription(selectedSubscription)"
                >删除条件</el-button
              >
            </div>
          </div>
          <div v-else v-loading="loading" class="results">
            <div v-if="!visibleResults.length" class="empty-state">
              <strong>{{
                activeView === "new" ? "本周暂无新推荐" : "暂无符合条件的 IP"
              }}</strong>
              <p>可调整市场、类型或预算，也可以立即刷新再检查资源库。</p>
              <el-button @click="editSubscription(selectedSubscription)"
                >调整寻找条件</el-button
              >
            </div>
            <article
              v-for="item in visibleResults"
              :id="`ip-recommendation-${item.id}`"
              :key="item.id"
              class="result-card"
              :class="{ focused: Number(route.query.resultId) === item.id }"
            >
              <div class="result-visual">
                <img
                  v-if="imageUrls[Number(item.resource?.visualFileId)]"
                  :src="imageUrls[Number(item.resource.visualFileId)]"
                  :alt="item.resourceName"
                /><span v-else>{{
                  item.resourceName?.slice(0, 2) || "IP"
                }}</span>
              </div>
              <div class="result-main">
                <div class="result-topline">
                  <div>
                    <span v-if="item.isNew" class="new-pill">NEW</span
                    ><span class="score-pill">匹配度 {{ item.score }}%</span
                    ><span class="source-pill">{{
                      item.source === "ai" ? "AI 排序 · 实时依据" : "规则推荐"
                    }}</span>
                  </div>
                  <time>{{ formatTime(item.createdAt) }}</time>
                </div>
                <h3>{{ item.resourceName }}</h3>
                <p class="resource-meta">
                  {{ item.resource?.ipType || "类型待完善" }} ·
                  {{
                    parseTextArray(item.resource?.markets).join(" / ") ||
                    "市场待完善"
                  }}
                  · {{ item.resource?.cooperationStatus || "合作状态待确认" }}
                </p>
                <p class="reason">
                  <b>推荐理由</b>{{ item.reason || "符合当前寻找条件" }}
                </p>
                <div v-if="riskItems(item.risks).length" class="risk">
                  <b>待关注</b>{{ riskItems(item.risks).join("；") }}
                </div>
                <div class="resource-footer">
                  <span
                    >基础权益参考价
                    <strong>{{ priceText(item.resource || {}) }}</strong></span
                  ><span
                    v-if="
                      item.resource?.currency &&
                      selectedSubscription?.budgetCurrency &&
                      item.resource.currency !==
                        selectedSubscription.budgetCurrency
                    "
                    class="currency-note"
                    >币种不同，价格需另行确认</span
                  >
                </div>
                <div class="card-actions">
                  <button
                    type="button"
                    class="detail-link"
                    @click="openResourceDrawer(item.resourceId)"
                  >
                    查看档案与版权资料 ↗
                  </button>
                  <div>
                    <el-button
                      v-if="item.feedback !== 'interested'"
                      link
                      @click="feedback(item, 'interested')"
                      >感兴趣</el-button
                    ><el-button link @click="feedback(item, 'not_interested')"
                      >不感兴趣</el-button
                    ><el-button
                      class="primary-button"
                      size="small"
                      @click="addToIntent(item)"
                      >加入初步意向 →</el-button
                    >
                  </div>
                </div>
              </div>
            </article>
          </div>
        </template>
      </main>
    </div>
    <el-drawer v-model="drawerVisible" title="IP 档案与授权资料" size="500px">
      <div v-loading="drawerLoading" class="resource-drawer">
        <template v-if="drawerDetail">
          <div class="drawer-intro">
            <img
              v-if="drawerVisualUrl"
              :src="drawerVisualUrl"
              :alt="drawerDetail.resource.name"
            />
            <div>
              <h2>{{ drawerDetail.resource.name }}</h2>
              <el-tag>{{ drawerDetail.resource.ipType }}</el-tag>
              <p>{{ drawerDetail.resource.audience || "受众待补充" }}</p>
            </div>
          </div>
          <p class="drawer-summary">
            {{ drawerDetail.resource.summary || "暂无 IP 简介" }}
          </p>
          <dl class="drawer-facts">
            <dt>版权方</dt>
            <dd>{{ drawerDetail.resource.rightsOwner || "待补充" }}</dd>
            <dt>覆盖市场</dt>
            <dd>
              {{
                parseTextArray(drawerDetail.resource.markets).join(" / ") ||
                "待补充"
              }}
            </dd>
            <dt>合作状态</dt>
            <dd>{{ drawerDetail.resource.cooperationStatus || "待确认" }}</dd>
            <dt>参考权益价</dt>
            <dd>
              <strong>{{ priceText(drawerDetail.resource) }}</strong
              ><small>供参考，根据具体需求更新</small>
            </dd>
          </dl>
          <section class="drawer-section">
            <h3>版权介绍文件</h3>
            <div
              v-for="file in drawerDetail.files.filter(
                item => item.fileKind === 'copyright'
              )"
              :key="file.id"
              class="document-row"
            >
              <span>PDF · {{ file.originalName }}</span
              ><el-button link @click="openDocument(file)">预览</el-button>
            </div>
            <p
              v-if="
                !drawerDetail.files.some(item => item.fileKind === 'copyright')
              "
              class="drawer-muted"
            >
              暂无版权介绍文件
            </p>
          </section>
          <section class="drawer-section">
            <h3>历史合作与结案</h3>
            <p v-if="!drawerDetail.cases.length" class="drawer-muted">
              暂无历史合作记录
            </p>
            <div
              v-for="item in drawerDetail.cases"
              :key="item.id"
              class="case-row"
            >
              <strong>{{ item.title }}</strong>
              <p>{{ item.summary || "暂无说明" }}</p>
              <div
                v-for="file in drawerDetail.files.filter(
                  file => Number(file.caseId) === Number(item.id)
                )"
                :key="file.id"
                class="document-row"
              >
                <span>PDF · {{ file.originalName }}</span
                ><el-button link @click="openDocument(file)">预览</el-button>
              </div>
            </div>
          </section>
        </template>
      </div>
    </el-drawer>
    <el-dialog
      v-model="previewVisible"
      :title="previewFileName"
      width="80%"
      top="5vh"
      @closed="closePreview"
    >
      <div v-if="previewUrl" class="pdf-preview">
        <div class="pdf-toolbar">
          <span>{{
            previewPages
              ? `共 ${previewPages} 页 · 向下滚动查看`
              : "正在读取 PDF…"
          }}</span>
          <a :href="previewUrl" :download="previewFileName">下载 PDF</a>
        </div>
        <p v-if="previewError" class="pdf-error">{{ previewError }}</p>
        <div v-else class="pdf-canvas">
          <p v-if="previewLoading" class="pdf-loading">正在渲染 PDF…</p>
          <vue-pdf-embed
            :key="previewUrl"
            :source="previewUrl"
            @loaded="doc => (previewPages = doc.numPages)"
            @rendered="previewLoading = false"
            @loading-failed="pdfFailed"
            @rendering-failed="pdfFailed"
          />
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.ip-recommend-page {
  min-height: calc(100vh - 150px);
  padding: 22px 28px 40px;
  color: #17191d;
  background: #f8f8f6;
}
.hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  padding: 28px 32px;
  margin-bottom: 18px;
  background: linear-gradient(115deg, #fff 36%, #f0f6e4 100%);
  border: 1px solid #dedfdb;
  border-radius: 12px;
}
.eyebrow {
  margin: 0 0 8px;
  font-family: monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.13em;
  color: #697352;
}
.hero h1 {
  margin: 0 0 8px;
  font-size: 32px;
  letter-spacing: -0.03em;
}
.hero-description,
.heading-hint {
  margin: 0;
  color: #717981;
}
.context-pill {
  display: inline-block;
  padding: 5px 11px;
  margin: 12px 0 0;
  font-size: 12px;
  background: #e8efdc;
  border-radius: 20px;
}
.hero-actions {
  display: flex;
  align-items: flex-end;
  gap: 20px;
}
.hero-stat {
  display: flex;
  flex-direction: column;
  text-align: right;
}
.hero-stat strong {
  font-size: 31px;
  line-height: 1;
}
.hero-stat span {
  font-size: 12px;
  color: #717981;
}
.primary-button {
  font-weight: 700;
  color: #111 !important;
  background: #caff00 !important;
  border-color: #caff00 !important;
}
.primary-button:hover {
  background: #b7eb00 !important;
}
.workspace {
  display: grid;
  grid-template-columns: 270px minmax(0, 1fr);
  gap: 18px;
  align-items: start;
}
.subscription-panel,
.main-panel {
  min-width: 0;
  padding: 18px;
  background: #fff;
  border: 1px solid #dedfdb;
  border-radius: 10px;
  box-shadow: 0 2px 10px #20240b06;
}
.subscription-panel {
  position: sticky;
  top: 14px;
}
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
.section-heading h2 {
  margin: 0;
  font-size: 19px;
}
.section-heading > span {
  color: #91999d;
}
.sidebar-empty {
  padding: 20px 8px;
  font-size: 13px;
  line-height: 1.7;
  color: #8a9294;
}
.subscription-item {
  display: flex;
  flex-direction: column;
  width: 100%;
  padding: 13px;
  margin: 0 0 8px;
  text-align: left;
  cursor: pointer;
  background: #f8f9f6;
  border: 1px solid transparent;
  border-radius: 8px;
}
.subscription-item:hover,
.subscription-item.selected {
  background: #f6fbdc;
  border-color: #d2e782;
}
.subscription-title {
  font-weight: 700;
}
.subscription-query {
  display: -webkit-box;
  overflow: hidden;
  margin: 6px 0 10px;
  font-size: 12px;
  color: #777e86;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
.subscription-foot {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: #58633f;
}
.subscription-foot i {
  width: 7px;
  height: 7px;
  background: #7daf00;
  border-radius: 50%;
}
.subscription-foot i.paused {
  background: #aaa;
}
.subscription-foot small {
  margin-left: auto;
  color: #a4aaac;
}
.main-panel {
  min-height: 450px;
}
.editor-card {
  padding: 4px;
}
.editor-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}
.editor-grid label {
  display: flex;
  flex-direction: column;
  gap: 7px;
  font-size: 13px;
  font-weight: 650;
}
.editor-grid label :deep(.el-select),
.editor-grid label :deep(.el-input),
.editor-grid label :deep(.el-textarea) {
  width: 100%;
}
.span-2 {
  grid-column: span 2;
}
.budget {
  display: flex;
  align-items: center;
  gap: 9px;
}
.budget :deep(.el-select) {
  max-width: 125px;
}
.budget :deep(.el-input-number) {
  flex: 1;
  width: 0;
}
.budget > span {
  color: #858c90;
}
.editor-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding-top: 20px;
  margin-top: 20px;
  border-top: 1px solid #e6e7e2;
}
.editor-footer > span {
  font-size: 12px;
  color: #80878a;
}
.editor-footer > div {
  display: flex;
  gap: 8px;
}
.result-heading {
  align-items: flex-start;
}
.heading-hint {
  max-width: 700px;
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.5;
}
.heading-actions {
  display: flex;
  gap: 8px;
}
.result-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  border-bottom: 1px solid #eaebe5;
}
.result-toolbar > span {
  font-size: 12px;
  color: #838a8d;
}
.view-tabs {
  display: flex;
  gap: 18px;
}
.view-tabs button {
  padding: 0 2px 12px;
  color: #7d8588;
  cursor: pointer;
  background: none;
  border: 0;
  border-bottom: 2px solid transparent;
}
.view-tabs button.active {
  font-weight: 700;
  color: #161a1d;
  border-color: #b0e700;
}
.view-tabs b {
  padding: 2px 5px;
  margin-left: 4px;
  font-size: 10px;
  background: #caff00;
  border-radius: 10px;
}
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 260px;
  text-align: center;
}
.empty-state strong {
  font-size: 17px;
}
.empty-state p {
  max-width: 430px;
  color: #858d90;
}
.management-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 20px;
  background: #f9faf5;
  border: 1px solid #e8e9e3;
  border-radius: 9px;
}
.management-card h3 {
  margin: 0 0 8px;
}
.management-card p {
  margin: 0 0 8px;
  color: #6b7375;
}
.management-card small {
  color: #8f9798;
}
.management-actions {
  display: flex;
  gap: 8px;
}
.results {
  display: grid;
  gap: 14px;
}
.result-card {
  display: grid;
  grid-template-columns: 158px minmax(0, 1fr);
  gap: 18px;
  padding: 14px;
  border: 1px solid #e2e4de;
  border-radius: 10px;
  box-shadow: 0 2px 9px #20240b05;
}
.result-card.focused {
  border-color: #a8d900;
  box-shadow: 0 0 0 2px #caff0066;
}
.result-visual {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 180px;
  overflow: hidden;
  font-size: 28px;
  font-weight: 800;
  color: #4b6121;
  background: linear-gradient(135deg, #e9f5cc, #f5f7ed);
  border-radius: 7px;
}
.result-visual img {
  width: 100%;
  height: 100%;
  max-height: 250px;
  object-fit: cover;
}
.result-main {
  min-width: 0;
}
.result-topline,
.card-actions,
.resource-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.result-topline > div {
  display: flex;
  align-items: center;
  gap: 6px;
}
.result-topline time {
  font-size: 11px;
  color: #9ba1a2;
}
.new-pill,
.score-pill,
.source-pill {
  padding: 3px 7px;
  font-size: 10px;
  font-weight: 700;
  border-radius: 3px;
}
.new-pill {
  color: #1e2608;
  background: #caff00;
}
.score-pill {
  color: #477000;
  background: #edf7d6;
}
.source-pill {
  color: #727a7a;
  background: #f0f1ef;
}
.result-card h3 {
  margin: 10px 0 4px;
  font-size: 19px;
}
.resource-meta {
  margin: 0 0 11px;
  font-size: 12px;
  color: #858d90;
}
.reason,
.risk {
  display: flex;
  gap: 8px;
  margin: 7px 0;
  font-size: 13px;
  line-height: 1.6;
}
.reason b,
.risk b {
  flex: none;
  color: #5f722b;
}
.risk {
  color: #c16c35;
}
.risk b {
  color: #c16c35;
}
.resource-footer {
  justify-content: flex-start;
  flex-wrap: wrap;
  margin-top: 10px;
  font-size: 12px;
  color: #787f80;
}
.resource-footer strong {
  color: #17191d;
}
.currency-note {
  color: #bf742d;
}
.card-actions {
  padding-top: 12px;
  margin-top: 12px;
  border-top: 1px solid #eff0eb;
}
.detail-link {
  padding: 0;
  font-size: 12px;
  font-weight: 700;
  color: #557513;
  cursor: pointer;
  background: none;
  border: 0;
}
.card-actions > div {
  display: flex;
  align-items: center;
  gap: 4px;
}
.resource-drawer {
  min-height: 160px;
  padding: 0 4px 18px;
}
.drawer-intro {
  display: flex;
  gap: 14px;
  align-items: center;
}
.drawer-intro img {
  width: 88px;
  height: 88px;
  object-fit: cover;
  border-radius: 8px;
}
.drawer-intro h2 {
  margin: 0 0 7px;
  font-size: 22px;
}
.drawer-intro p,
.drawer-summary,
.case-row p {
  color: #777f81;
  line-height: 1.6;
}
.drawer-facts {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 12px;
  padding: 15px;
  margin: 18px 0;
  background: #f8f9f5;
  border: 1px solid #e9ebe3;
  border-radius: 8px;
}
.drawer-facts dt {
  color: #8a9292;
}
.drawer-facts dd {
  margin: 0;
}
.drawer-facts small {
  display: block;
  margin-top: 4px;
  color: #999;
}
.drawer-section {
  padding: 14px 0;
  border-top: 1px solid #e9ebe4;
}
.drawer-section h3 {
  margin: 0 0 12px;
  font-size: 15px;
}
.drawer-muted {
  margin: 0;
  font-size: 13px;
  color: #999;
}
.document-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 9px 10px;
  margin: 7px 0;
  font-size: 12px;
  background: #f6f8f0;
  border-radius: 6px;
}
.document-row span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.case-row {
  padding: 12px;
  margin-bottom: 10px;
  border: 1px solid #e6e8e0;
  border-radius: 7px;
}
.case-row p {
  margin: 6px 0;
  font-size: 13px;
}
.pdf-preview {
  min-height: 240px;
}
.pdf-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}
.pdf-toolbar a {
  font-size: 13px;
  font-weight: 700;
  color: #567811;
}
.pdf-canvas {
  min-height: 240px;
  max-height: 72vh;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 12px;
  background: #f3f4f0;
}
.pdf-loading {
  margin: 0 0 12px;
  color: #737373;
}
.pdf-canvas :deep(.vue-pdf-embed > div + div) {
  margin-top: 12px;
}
.pdf-canvas :deep(canvas) {
  display: block;
  max-width: 100%;
  height: auto !important;
  margin: 0 auto;
}
.pdf-error {
  color: #c3483f;
}
@media (max-width: 1000px) {
  .workspace {
    grid-template-columns: 1fr;
  }
  .subscription-panel {
    position: static;
  }
  .hero,
  .hero-actions,
  .editor-footer,
  .management-card {
    align-items: flex-start;
    flex-direction: column;
  }
  .hero-stat {
    text-align: left;
  }
  .result-card {
    grid-template-columns: 120px minmax(0, 1fr);
  }
}
@media (max-width: 680px) {
  .ip-recommend-page {
    padding: 12px;
  }
  .hero {
    padding: 20px;
  }
  .editor-grid {
    grid-template-columns: 1fr;
  }
  .span-2 {
    grid-column: auto;
  }
  .result-card {
    grid-template-columns: 1fr;
  }
  .result-visual {
    height: 150px;
  }
  .card-actions,
  .card-actions > div,
  .result-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
  .view-tabs {
    flex-wrap: wrap;
  }
}
</style>
