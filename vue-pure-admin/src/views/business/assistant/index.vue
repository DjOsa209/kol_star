<script setup lang="ts">
import {
  computed,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch
} from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useI18n } from "vue-i18n";
import * as XLSX from "xlsx";
import { fieldLabel } from "@/utils/fieldI18n";
import {
  deleteRecommendationSubscription,
  listRecommendationResults,
  listRecommendationSubscriptions,
  runRecommendationSubscription,
  saveRecommendationSubscription,
  sendRecommendationFeedback,
  type RecommendationResult,
  type RecommendationSubscription
} from "@/api/recommendations";
import {
  addProjectResource,
  createMarketOption,
  deleteMarketOption,
  getCooperationList,
  getMarketOptions,
  getProjectList,
  getResourcePosts,
  recommendResources,
  translateResources
} from "@/api/business";

import RecommendationWorkbench from "./RecommendationWorkbench.vue";
import { buildRecentCooperationWorks } from "../resources/recentWorks";

defineOptions({ name: "BusinessAssistant" });

const { locale } = useI18n();
const isEnglish = computed(() => locale.value === "en");

const loading = ref(false);
const resultMode = ref<"recent" | "saved">("recent");
const resultError = ref("");
let savedRequestSeq = 0;
const elapsedSeconds = ref(0);
const parsed = ref<Record<string, any>>({});
const recommendations = ref<any[]>([]);
const filteredSummary = ref<Record<string, number>>({});
const message = ref("");
const submittedDemand = ref("");
const stopped = ref(false);
const attachedFiles = ref<any[]>([]);
const marketOptions = ref<string[]>([]);
const avatarLoadFailed = reactive<Record<string, boolean>>({});
const avatarLoaded = reactive<Record<string, boolean>>({});
const projects = ref<any[]>([]);
const cooperations = ref<any[]>([]);
const matchedResourceIds = reactive<Record<number, boolean>>({});
const selectedProjectId = ref<number | null>(null);
const addingToCampaign = ref(false);
const savedSubscriptions = ref<RecommendationSubscription[]>([]);
const selectedSubscriptionId = ref<number | null>(null);
const savedResults = ref<RecommendationResult[]>([]);
const savedView = ref<"new" | "all">("all");
const savedLoading = ref(false);
const subscriptionSaving = ref(false);
const subscriptionName = ref("");
const profileDrawerVisible = ref(false);
const profileLoading = ref(false);
const profileTab = ref("overview");
const selectedResource = ref<any | null>(null);
const selectedResourcePosts = ref<any[]>([]);
const selectedResourcePostStats = ref<any>({});
let timer: ReturnType<typeof setInterval> | undefined;
let requestSeq = 0;
const recommendationStorageKey = "business-assistant-last-recommendation";
const recommendationStorageVersion = 3;

const form = reactive({
  demandText: "",
  targetMarket: "",
  resourceType: "",
  platform: "",
  budget: 0
});
const recentCriteria = ref({ ...form });

const defaultMarketOptions = [
  "美国",
  "英国",
  "欧洲",
  "德国",
  "日本",
  "中东北非",
  "东非",
  "西非",
  "东南亚",
  "拉美"
];
const parsedItems = computed(() =>
  Object.entries(parsed.value).filter(([, value]) => {
    return value !== "" && value !== 0 && value !== null && value !== undefined;
  })
);
const attachmentSummary = computed(() =>
  attachedFiles.value
    .map(
      file => `${file.name}：${file.summary || "已上传，等待模型参考文件信息"}`
    )
    .join("\n")
);
const filterItems = computed(() => Object.entries(filteredSummary.value));
const hasStarted = computed(() => Boolean(submittedDemand.value));
const hasResult = computed(
  () =>
    parsedItems.value.length > 0 ||
    recommendations.value.length > 0 ||
    filterItems.value.length > 0 ||
    Boolean(message.value)
);
const isInterrupted = computed(
  () => hasStarted.value && !loading.value && !hasResult.value && !stopped.value
);
const statusText = computed(() => {
  if (loading.value) return "执行中";
  if (resultError.value) return "待重试";
  if (stopped.value) return "已停止";
  if (hasResult.value) return "已完成";
  if (isInterrupted.value) return "待重试";
  return "待执行";
});
const elapsedText = computed(() => {
  if (!hasStarted.value) return "未执行";
  if (loading.value) return `已用 ${elapsedSeconds.value.toFixed(1)}s`;
  if (stopped.value) return `已停止 · ${elapsedSeconds.value.toFixed(1)}s`;
  if (isInterrupted.value)
    return `未完成 · ${elapsedSeconds.value.toFixed(1)}s`;
  return `完成 · ${elapsedSeconds.value.toFixed(1)}s`;
});
const resultCandidates = computed(() =>
  resultMode.value === "saved"
    ? savedResults.value.map(savedResource)
    : recommendations.value
);
const matchedRecommendations = computed(() =>
  resultCandidates.value.filter(item => matchedResourceIds[Number(item.id)])
);
const candidateWorks = computed(() =>
  Object.fromEntries(
    resultCandidates.value.map(item => [
      Number(item.id),
      buildRecentCooperationWorks(
        cooperations.value.filter(
          row => Number(row.resourceId) === Number(item.id)
        )
      )
    ])
  )
);
watch(resultCandidates, () => {
  Object.keys(matchedResourceIds).forEach(
    key => delete matchedResourceIds[Number(key)]
  );
});
function applySavedCriteria(item: RecommendationSubscription) {
  Object.assign(form, {
    demandText: item.query,
    targetMarket: item.markets?.[0] || "",
    resourceType: item.resourceTypes?.[0] || "",
    platform: item.platforms?.[0] || "",
    budget: Number(item.budgetMax || 0)
  });
}
function selectSavedSearch(item: RecommendationSubscription) {
  resultMode.value = "saved";
  selectedSubscriptionId.value = item.id;
  applySavedCriteria(item);
  savedView.value = "all";
  loadSavedResults();
}
function changeResultMode(mode: "recent" | "saved") {
  resultMode.value = mode;
  resultError.value = "";
  if (mode === "recent") Object.assign(form, recentCriteria.value);
  if (mode === "saved" && selectedSavedSubscription.value)
    selectSavedSearch(selectedSavedSubscription.value);
}
async function manageSavedSearch(
  action: string,
  item: RecommendationSubscription
) {
  if (action === "refresh") {
    selectedSubscriptionId.value = item.id;
    resultMode.value = "saved";
    applySavedCriteria(item);
    await refreshSavedSearch();
  } else if (action === "toggle") await toggleSavedSearch(item);
  else if (action === "delete") await removeSavedSearch(item);
}
function changeSavedView(view: "new" | "all") {
  savedView.value = view;
  loadSavedResults();
}
function feedbackById(id: number) {
  const item = savedResults.value.find(row => row.id === id);
  if (item) feedbackSavedResult(item);
}
const selectedSavedSubscription = computed(() =>
  savedSubscriptions.value.find(
    item => item.id === selectedSubscriptionId.value
  )
);
const selectedResourceCooperations = computed(() => {
  const resourceId = Number(selectedResource.value?.id || 0);
  return cooperations.value.filter(
    item => Number(item.resourceId) === resourceId
  );
});
const selectedResourceCooperationStats = computed(() => {
  const result = {
    count: 0,
    reach: 0,
    engagement: 0,
    cost: 0,
    completed: 0
  };
  selectedResourceCooperations.value.forEach(item => {
    result.count += 1;
    result.reach += primaryReach(item);
    result.engagement +=
      numberValue(item.engagementCount) + numberValue(item.commentsCount);
    result.cost += numberValue(item.quoteAmount);
    if (/已发布|已完成/.test(`${item.status} ${item.deliverableStatus}`)) {
      result.completed += 1;
    }
  });
  return result;
});
async function generate() {
  if (!form.demandText.trim() && attachedFiles.value.length === 0) {
    ElMessage.warning("请输入项目需求或上传产品资料");
    return;
  }
  const current = ++requestSeq;
  resultError.value = "";
  recommendations.value = [];
  parsed.value = {};
  filteredSummary.value = {};
  message.value = "";
  localStorage.removeItem(recommendationStorageKey);
  resultMode.value = "recent";
  Object.keys(matchedResourceIds).forEach(
    key => delete matchedResourceIds[Number(key)]
  );
  submittedDemand.value = buildDemandWithAttachments();
  stopped.value = false;
  loading.value = true;
  elapsedSeconds.value = 0;
  startTimer();

  try {
    const res = await recommendResources({
      demandText: submittedDemand.value,
      targetMarket: form.targetMarket,
      language: "",
      resourceType: form.resourceType,
      platform: form.platform,
      budget: form.budget,
      includeWatch: false
    });

    if (current !== requestSeq) return;
    if (res.code === 0) {
      if (
        res.data.fallback !== false ||
        (res.data.recommendations || []).some(item => item.source !== "ai")
      ) {
        resultError.value = "本页面仅展示大模型推荐，请确认后端模型配置后重试";
        return;
      }
      recentCriteria.value = { ...form };
      parsed.value = res.data.parsed || {};
      recommendations.value = res.data.recommendations || [];
      filteredSummary.value = res.data.filteredSummary || {};
      message.value = res.data.message || "";
      saveLastRecommendation();
    } else {
      resultError.value = res.message || "推荐生成失败，请稍后重试";
      ElMessage.warning(resultError.value);
    }
  } catch (error) {
    if (current === requestSeq) {
      resultError.value = resolveRecommendError(error);
      ElMessage.warning(resultError.value);
    }
  } finally {
    if (current === requestSeq) {
      loading.value = false;
      stopTimer();
    }
  }
}

async function loadSavedSubscriptions(preferredId?: number) {
  try {
    const response = await listRecommendationSubscriptions("creator");
    if (response.code !== 0) throw new Error(response.message);
    savedSubscriptions.value = response.data?.list || [];
    const nextId = preferredId || selectedSubscriptionId.value;
    selectedSubscriptionId.value = savedSubscriptions.value.some(
      item => item.id === nextId
    )
      ? nextId
      : savedSubscriptions.value[0]?.id || null;
    if (resultMode.value === "saved" && selectedSavedSubscription.value) {
      applySavedCriteria(selectedSavedSubscription.value);
    }
    await loadSavedResults();
  } catch (error) {
    ElMessage.warning(
      error instanceof Error ? error.message : "寻找条件加载失败"
    );
  }
}

async function loadSavedResults() {
  const current = ++savedRequestSeq;
  const subscriptionId = selectedSubscriptionId.value;
  if (!subscriptionId) {
    savedResults.value = [];
    savedLoading.value = false;
    return;
  }
  savedLoading.value = true;
  resultError.value = "";
  try {
    const response = await listRecommendationResults(
      "creator",
      subscriptionId,
      savedView.value
    );
    if (current !== savedRequestSeq) return;
    if (response.code !== 0) throw new Error(response.message);
    savedResults.value = (response.data?.list || []).filter(
      item => item.source === "ai" && item.adviceSource === "ai"
    );
  } catch (error) {
    if (current !== savedRequestSeq) return;
    savedResults.value = [];
    const errorMessage =
      error instanceof Error ? error.message : "推荐结果加载失败";
    if (resultMode.value === "saved") resultError.value = errorMessage;
    ElMessage.warning(errorMessage);
  } finally {
    if (current === savedRequestSeq) savedLoading.value = false;
  }
}

async function saveCurrentSearch() {
  if (!form.demandText.trim()) {
    ElMessage.warning("请先描述寻找需求");
    return;
  }
  subscriptionSaving.value = true;
  try {
    const response = await saveRecommendationSubscription({
      domain: "creator",
      name:
        subscriptionName.value.trim() || form.demandText.trim().slice(0, 30),
      query: form.demandText.trim(),
      markets: form.targetMarket ? [form.targetMarket] : [],
      platforms: form.platform ? [form.platform] : [],
      resourceTypes: form.resourceType ? [form.resourceType] : [],
      budgetCurrency: "USD",
      budgetMin: null,
      budgetMax: Number(form.budget) > 0 ? Number(form.budget) : null,
      active: true
    });
    if (response.code !== 0) throw new Error(response.message);
    resultMode.value = "saved";
    selectedSubscriptionId.value = Number(response.data?.id || 0) || null;
    savedView.value = "all";
    await loadSavedSubscriptions(selectedSubscriptionId.value || undefined);
    subscriptionName.value = "";
    if (response.data?.warning) {
      resultError.value = response.data.warning;
      ElMessage.warning(response.data.warning);
    } else {
      ElMessage.success(
        "已保存寻找条件，并生成首批推荐；每周一有新匹配时将提醒你"
      );
    }
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "保存寻找条件失败"
    );
  } finally {
    subscriptionSaving.value = false;
  }
}

async function refreshSavedSearch() {
  if (!selectedSubscriptionId.value) return;
  savedLoading.value = true;
  resultError.value = "";
  savedResults.value = [];
  try {
    const response = await runRecommendationSubscription(
      "creator",
      selectedSubscriptionId.value
    );
    if (response.code !== 0) throw new Error(response.message);
    savedView.value = "all";
    await loadSavedResults();
    ElMessage.success("推荐已更新");
  } catch (error) {
    resultError.value = resolveRecommendError(error);
    ElMessage.error(resultError.value);
  } finally {
    savedLoading.value = false;
  }
}

async function toggleSavedSearch(item: RecommendationSubscription) {
  try {
    const response = await saveRecommendationSubscription({
      ...item,
      domain: "creator",
      active: !item.active
    });
    if (response.code !== 0) throw new Error(response.message);
    await loadSavedSubscriptions(item.id);
    ElMessage.success(item.active ? "已暂停每周推荐" : "已恢复每周推荐");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "更新失败");
  }
}

async function removeSavedSearch(item: RecommendationSubscription) {
  try {
    await ElMessageBox.confirm(
      `删除「${item.name}」及其推荐记录？`,
      "删除寻找条件",
      {
        type: "warning"
      }
    );
    const response = await deleteRecommendationSubscription("creator", item.id);
    if (response.code !== 0) throw new Error(response.message);
    await loadSavedSubscriptions();
    ElMessage.success("已删除寻找条件");
  } catch (error) {
    if (error !== "cancel") ElMessage.warning("删除未完成");
  }
}

function savedResource(item: RecommendationResult) {
  return {
    ...item.resource,
    id: item.resourceId,
    resultId: item.id,
    name: item.resourceName,
    matchScore: item.score,
    source: item.source,
    advice: item.advice,
    adviceSource: item.adviceSource,
    scenarios: item.scenarios,
    reason: item.reason,
    riskTip: Array.isArray(item.risks) ? item.risks.join("；") : item.risks,
    priority: Number(item.score) >= 80 ? "高" : "中"
  };
}

async function feedbackSavedResult(item: RecommendationResult) {
  try {
    const response = await sendRecommendationFeedback(
      "creator",
      item.id,
      "not_interested"
    );
    if (response.code !== 0) throw new Error(response.message);
    await loadSavedResults();
    ElMessage.success("已减少类似推荐");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "反馈失败");
  }
}

async function loadBusinessContext() {
  const [projectResult, cooperationResult] = await Promise.allSettled([
    getProjectList(),
    getCooperationList()
  ]);
  if (projectResult.status === "fulfilled" && projectResult.value.code === 0) {
    projects.value = projectResult.value.data.list || [];
    if (!selectedProjectId.value && projects.value.length > 0) {
      selectedProjectId.value = projects.value[0].id;
    }
  } else {
    ElMessage.warning("Campaign 列表加载失败，请刷新页面重试");
  }
  if (
    cooperationResult.status === "fulfilled" &&
    cooperationResult.value.code === 0
  ) {
    cooperations.value = cooperationResult.value.data.list || [];
  }
}

async function loadMarkets() {
  marketOptions.value = defaultMarketOptions;
  try {
    const res = await getMarketOptions();
    if (res.code === 0 && Array.isArray(res.data)) {
      const names = res.data
        .map(item => String(item.name || "").trim())
        .filter(Boolean);
      marketOptions.value = Array.from(new Set(names));
    }
  } catch {
    marketOptions.value = defaultMarketOptions;
  }
}

async function handleMarketChange(value: string) {
  const name = String(value || "").trim();
  if (!name) return;
  if (!marketOptions.value.includes(name)) {
    marketOptions.value.push(name);
  }
  try {
    const res = await createMarketOption({ name });
    if (res.code !== 0) ElMessage.warning(res.message || "市场保存失败");
  } catch {
    ElMessage.warning("市场保存到后台失败，请稍后重试");
  }
}

async function removeMarketOption(value: string, event?: Event) {
  event?.stopPropagation();
  const name = String(value || "").trim();
  if (!name) return;
  try {
    await ElMessageBox.confirm(`确认删除市场选项「${name}」吗？`, "删除市场", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消"
    });
  } catch {
    return;
  }
  const res = await deleteMarketOption({ name });
  if (res.code === 0) {
    marketOptions.value = marketOptions.value.filter(item => item !== name);
    if (form.targetMarket === name) form.targetMarket = "";
    ElMessage.success("市场选项已删除");
  } else {
    ElMessage.warning(res.message || "市场删除失败");
  }
}

function buildDemandWithAttachments() {
  const parts = [form.demandText.trim()].filter(Boolean);
  if (attachmentSummary.value) {
    parts.push(`\n【上传资料摘要】\n${attachmentSummary.value}`);
  }
  return parts.join("\n");
}

async function handleAttachmentChange(file: any) {
  const rawFile = file.raw;
  if (!rawFile) return;
  const existingIndex = attachedFiles.value.findIndex(
    item => item.uid === file.uid || item.name === rawFile.name
  );
  const item = {
    uid: file.uid,
    name: rawFile.name,
    size: rawFile.size,
    type: rawFile.type || file.name?.split(".").pop() || "",
    summary: await extractAttachmentSummary(rawFile)
  };
  if (existingIndex >= 0) {
    attachedFiles.value.splice(existingIndex, 1, item);
  } else {
    attachedFiles.value.push(item);
  }
}

function removeAttachment(uid: string | number) {
  attachedFiles.value = attachedFiles.value.filter(item => item.uid !== uid);
}

async function extractAttachmentSummary(file: File) {
  const name = file.name.toLowerCase();
  if (
    name.endsWith(".xlsx") ||
    name.endsWith(".xls") ||
    name.endsWith(".csv")
  ) {
    const buffer = await file.arrayBuffer();
    const workbook = XLSX.read(buffer, { type: "array", cellDates: true });
    const lines = workbook.SheetNames.slice(0, 3).map(sheetName => {
      const rows = XLSX.utils.sheet_to_json<any[]>(workbook.Sheets[sheetName], {
        header: 1,
        defval: "",
        blankrows: false
      });
      const preview = rows
        .slice(0, 5)
        .map(row => row.filter(Boolean).join(" / "))
        .filter(Boolean)
        .join("；");
      return `${sheetName}：${preview || "空表"}`;
    });
    return lines.join("\n").slice(0, 1800);
  }

  if (
    file.type.startsWith("text/") ||
    name.endsWith(".md") ||
    name.endsWith(".txt")
  ) {
    return (await file.text()).slice(0, 1800);
  }

  return `${file.name}，${Math.ceil(file.size / 1024)}KB。浏览器端暂不解析正文，模型将参考文件名、类型和人工输入需求。`;
}

function stopGeneration() {
  if (!loading.value) return;
  requestSeq++;
  stopped.value = true;
  loading.value = false;
  stopTimer();
  ElMessage.info("已停止本次分析展示");
}

function saveLastRecommendation() {
  try {
    localStorage.setItem(
      recommendationStorageKey,
      JSON.stringify({
        version: recommendationStorageVersion,
        form: { ...form },
        parsed: parsed.value,
        recommendations: recommendations.value,
        filteredSummary: filteredSummary.value,
        message: message.value,
        submittedDemand: submittedDemand.value,
        elapsedSeconds: elapsedSeconds.value,
        savedAt: Date.now()
      })
    );
  } catch {
    ElMessage.warning("推荐结果已生成，但浏览器本地保存失败");
  }
}

function loadLastRecommendation() {
  try {
    const raw = localStorage.getItem(recommendationStorageKey);
    if (!raw) return;
    const cache = JSON.parse(raw);
    if (cache.version !== recommendationStorageVersion) {
      localStorage.removeItem(recommendationStorageKey);
      return;
    }
    Object.assign(
      form,
      cache.form || {
        demandText: cache.submittedDemand || "",
        targetMarket: cache.parsed?.targetMarket || "",
        resourceType: cache.parsed?.resourceType || "",
        platform: cache.parsed?.platform || "",
        budget: Number(cache.parsed?.budget || 0)
      }
    );
    recentCriteria.value = { ...form };
    parsed.value = cache.parsed || {};
    recommendations.value = Array.isArray(cache.recommendations)
      ? cache.recommendations.filter(item => item.source === "ai")
      : [];
    filteredSummary.value = cache.filteredSummary || {};
    message.value = cache.message || "";
    submittedDemand.value = cache.submittedDemand || "上次推荐";
    elapsedSeconds.value = Number(cache.elapsedSeconds || 0);
  } catch {
    localStorage.removeItem(recommendationStorageKey);
  }
}

function clearLastRecommendation() {
  localStorage.removeItem(recommendationStorageKey);
  resultError.value = "";
  parsed.value = {};
  recommendations.value = [];
  filteredSummary.value = {};
  message.value = "";
  submittedDemand.value = "";
  elapsedSeconds.value = 0;
  stopped.value = false;
  Object.keys(avatarLoadFailed).forEach(key => delete avatarLoadFailed[key]);
  Object.keys(avatarLoaded).forEach(key => delete avatarLoaded[key]);
  ElMessage.success("上次推荐已清空");
}

function startTimer() {
  stopTimer();
  timer = setInterval(() => {
    elapsedSeconds.value = Number((elapsedSeconds.value + 0.1).toFixed(1));
  }, 100);
}

function stopTimer() {
  if (timer) clearInterval(timer);
  timer = undefined;
}

function moneyText(value: unknown) {
  return Number(value || 0).toLocaleString(isEnglish.value ? "en-US" : "zh-CN");
}

function resolveRecommendError(error: unknown) {
  const err = error as {
    code?: string;
    message?: string;
    response?: { status?: number; data?: { message?: string } };
  };
  if (err.response?.data?.message) return err.response.data.message;
  if (err.response?.status === 401) return "登录已过期，请重新登录后再试";
  if (err.response?.status === 403) return "当前账号没有智能资源助手权限";
  if (err.code === "ECONNABORTED") {
    return "模型响应时间较长，本次请求已超时，请稍后重试或调低模型超时时间";
  }
  if (err.message?.includes("timeout")) {
    return "模型响应时间较长，本次请求已超时，请稍后重试";
  }
  return "推荐接口暂不可用，请确认后端服务已启动";
}

function compactNumber(value: unknown) {
  const number = Number(value || 0);
  if (isEnglish.value) {
    return new Intl.NumberFormat("en-US", {
      notation: "compact",
      maximumFractionDigits: 1
    }).format(number);
  }
  if (number >= 100000000) return `${Math.round(number / 100000000)}亿`;
  if (number >= 10000) return `${Math.round(number / 10000)}万`;
  return number.toLocaleString("zh-CN");
}

function numberValue(value: unknown) {
  const number = Number(value || 0);
  return Number.isFinite(number) ? number : 0;
}

function formatCount(value: unknown) {
  const number = numberValue(value);
  if (number <= 0) return "-";
  return number.toLocaleString(isEnglish.value ? "en-US" : "zh-CN");
}

function percentText(value: unknown) {
  const number = numberValue(value);
  if (number <= 0) return "-";
  const percent = number > 1 ? number : number * 100;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function ratioPercent(numerator: unknown, denominator: unknown) {
  const top = numberValue(numerator);
  const bottom = numberValue(denominator);
  if (top <= 0 || bottom <= 0) return "-";
  return `${((top / bottom) * 100).toFixed(1)}%`;
}

function primaryReach(row: any) {
  return numberValue(row.impressions) || numberValue(row.views);
}

function moneyWithCurrency(value: unknown, currency = "USD") {
  const number = numberValue(value);
  if (number <= 0) return "-";
  return `${currency} ${number.toLocaleString(
    isEnglish.value ? "en-US" : "zh-CN",
    {
      maximumFractionDigits: 0
    }
  )}`;
}

function toggleMatched(item: any) {
  const id = Number(item.id || 0);
  if (!id) return;
  matchedResourceIds[id] = !matchedResourceIds[id];
}

async function addMatchedToCampaign() {
  if (!selectedProjectId.value) {
    ElMessage.warning("请先选择 Campaign");
    return;
  }
  const selected = [...matchedRecommendations.value];
  if (!selected.length || addingToCampaign.value) return;
  addingToCampaign.value = true;
  try {
    const results = await Promise.allSettled(
      selected.map(item =>
        addProjectResource({
          projectId: selectedProjectId.value,
          resourceId: item.id,
          status: "候选",
          source: "AI 推荐",
          reason: item.reason,
          priority: item.priority,
          estimatedCost: item.estimatedCost,
          riskTip: item.riskTip
        })
      )
    );
    let successCount = 0;
    results.forEach((result, index) => {
      if (result.status === "fulfilled" && result.value.code === 0) {
        delete matchedResourceIds[Number(selected[index].id)];
        successCount++;
      }
    });
    if (successCount)
      ElMessage.success(`已将 ${successCount} 位资源加入 Campaign 候选池`);
    if (successCount < selected.length)
      ElMessage.warning("部分资源加入失败，已保留选择，请重试");
  } finally {
    addingToCampaign.value = false;
  }
}

async function addResourceToCampaign(item: any) {
  if (!selectedProjectId.value) {
    ElMessage.warning("请先选择 Campaign");
    return;
  }
  addingToCampaign.value = true;
  try {
    const res = await addProjectResource({
      projectId: selectedProjectId.value,
      resourceId: item.id,
      status: "候选",
      source: "AI 推荐",
      reason: item.reason,
      priority: item.priority,
      estimatedCost: item.estimatedCost,
      riskTip: item.riskTip
    });
    if (res.code !== 0) throw new Error(res.message || "加入 Campaign 失败");
    delete matchedResourceIds[Number(item.id)];
    ElMessage.success(`${item.name} 已加入 Campaign 候选池`);
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "加入 Campaign 失败"
    );
  } finally {
    addingToCampaign.value = false;
  }
}

async function openResourceProfile(item: any) {
  selectedResource.value = item;
  selectedResourcePosts.value = [];
  selectedResourcePostStats.value = {};
  profileTab.value = "overview";
  profileDrawerVisible.value = true;
  profileLoading.value = true;
  try {
    if (locale.value === "en") {
      await translateResources({
        ids: [Number(item.id)],
        targetLanguage: "en"
      });
    }
    const res = await getResourcePosts({
      resourceId: item.id,
      currentPage: 1,
      pageSize: 12,
      locale: locale.value
    });
    if (res.code === 0) {
      selectedResource.value = {
        ...item,
        ...(res.data.resource || {})
      };
      selectedResourcePosts.value = (res.data.list || []).map((post: any) => ({
        ...post,
        title:
          locale.value === "en" && post.localized?.title
            ? post.localized.title
            : post.title,
        description:
          locale.value === "en" && post.localized?.description
            ? post.localized.description
            : post.description
      }));
      selectedResourcePostStats.value = res.data.stats || {};
    }
  } finally {
    profileLoading.value = false;
  }
}

function deliveryStage(item: any) {
  const status = `${item.status || ""} ${item.deliverableStatus || ""}`;
  if (/已发布|已完成/.test(status)) return "已完成";
  if (/待发布|排期/.test(status)) return "待发布";
  if (/制作|脚本|审核|修改/.test(status)) return "内容制作";
  if (/确认/.test(status)) return "已确认合作";
  return "沟通 / 议价";
}

function deliveryType(item: any) {
  const stage = deliveryStage(item);
  if (stage === "已完成") return "success";
  if (stage === "待发布") return "warning";
  if (stage === "内容制作") return "primary";
  return "info";
}

function formatDateTime(value: unknown) {
  if (!value) return "-";
  const timestamp = Number(value);
  if (Number.isFinite(timestamp) && timestamp > 0) {
    return new Date(timestamp).toLocaleDateString("zh-CN");
  }
  return String(value);
}

function initials(name: unknown) {
  return String(name || "AI")
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map(word => word[0])
    .join("")
    .toUpperCase();
}

function avatarKey(row: any) {
  return String(row.id || row.name || row.avatarUrl || "");
}

function markAvatarFailed(row: any) {
  avatarLoadFailed[avatarKey(row)] = true;
}

function markAvatarLoaded(row: any) {
  avatarLoaded[avatarKey(row)] = true;
}

onMounted(() => {
  loadLastRecommendation();
  loadMarkets();
  loadBusinessContext();
  loadSavedSubscriptions().then(() => {
    if (
      !hasStarted.value &&
      !recommendations.value.length &&
      selectedSavedSubscription.value
    )
      selectSavedSearch(selectedSavedSubscription.value);
  });
});
onBeforeUnmount(stopTimer);
</script>

<template>
  <div class="assistant-page">
    <RecommendationWorkbench
      :form="form"
      :items="resultCandidates"
      :selected="matchedResourceIds"
      :markets="marketOptions"
      :subscriptions="savedSubscriptions"
      :subscription-id="selectedSubscriptionId"
      :mode="resultMode"
      :recent-available="hasResult"
      :saved-view="savedView"
      :projects="projects"
      :project-id="selectedProjectId"
      :loading="loading"
      :saved-loading="resultMode === 'saved' && savedLoading"
      :search-name="subscriptionName"
      :saving="subscriptionSaving"
      :adding="addingToCampaign"
      :attachments="attachedFiles"
      :status="
        resultMode === 'saved'
          ? savedLoading
            ? '读取中'
            : '已保存的推荐'
          : statusText
      "
      :message="message"
      :error="resultError"
      :elapsed="elapsedText"
      :works="candidateWorks"
      @update-form="Object.assign(form, $event)"
      @generate="generate"
      @stop="stopGeneration"
      @search-name-change="subscriptionName = $event"
      @save="saveCurrentSearch"
      @clear="clearLastRecommendation"
      @upload="handleAttachmentChange"
      @remove-attachment="removeAttachment"
      @market-change="handleMarketChange"
      @remove-market="removeMarketOption"
      @select-search="selectSavedSearch"
      @search-action="manageSavedSearch"
      @mode-change="changeResultMode"
      @saved-view-change="changeSavedView"
      @project-change="selectedProjectId = $event"
      @toggle="toggleMatched"
      @profile="openResourceProfile"
      @feedback="feedbackById"
      @add="addMatchedToCampaign"
    />

    <el-drawer
      v-model="profileDrawerVisible"
      size="72%"
      class="creator-profile-drawer"
      :with-header="false"
    >
      <div
        v-if="selectedResource"
        v-loading="profileLoading"
        class="creator-profile"
      >
        <header class="creator-profile-header">
          <el-button
            circle
            :aria-label="fieldLabel('关闭档案')"
            @click="profileDrawerVisible = false"
          >
            <IconifyIconOnline icon="ri:arrow-left-line" />
          </el-button>
          <div class="profile-avatar-large">
            <span>{{ initials(selectedResource.name) }}</span>
            <img
              v-if="
                selectedResource.avatarUrl &&
                !avatarLoadFailed[avatarKey(selectedResource)]
              "
              v-show="avatarLoaded[avatarKey(selectedResource)]"
              :src="selectedResource.avatarUrl"
              :alt="selectedResource.name"
              @load="markAvatarLoaded(selectedResource)"
              @error="markAvatarFailed(selectedResource)"
            />
          </div>
          <div class="profile-heading">
            <div>
              <h2>{{ selectedResource.name }}</h2>
              <el-tag type="warning" effect="light">
                {{ selectedResource.matchScore || selectedResource.score || 0 }}
                {{ fieldLabel("分匹配") }}
              </el-tag>
            </div>
            <p>
              {{ selectedResource.platform || "-" }} ·
              {{ selectedResource.country || "-" }} ·
              {{ selectedResource.language || "-" }} ·
              {{ selectedResource.resourceType || "-" }}
            </p>
          </div>
          <div class="profile-header-actions">
            <el-button @click="toggleMatched(selectedResource)">
              {{
                matchedResourceIds[Number(selectedResource.id)]
                  ? fieldLabel("取消匹配")
                  : fieldLabel("标记匹配")
              }}
            </el-button>
            <el-button
              type="primary"
              :loading="addingToCampaign"
              @click="addResourceToCampaign(selectedResource)"
            >
              {{ fieldLabel("加入 Campaign") }}
            </el-button>
          </div>
        </header>

        <section class="quality-strip">
          <div>
            <IconifyIconOnline icon="ri:time-line" />
            <span
              ><strong>{{ fieldLabel("近期活跃") }}</strong
              >{{ fieldLabel("平台数据持续更新") }}</span
            >
          </div>
          <div>
            <IconifyIconOnline icon="ri:bar-chart-grouped-line" />
            <span
              ><strong>{{ fieldLabel("互动表现") }}</strong
              >{{ percentText(selectedResource.engagementRate) }}</span
            >
          </div>
          <div>
            <IconifyIconOnline icon="ri:shield-check-line" />
            <span
              ><strong>{{ fieldLabel("风险等级") }}</strong
              >{{ fieldLabel(selectedResource.riskLevel || "低") }}</span
            >
          </div>
          <div>
            <IconifyIconOnline icon="ri:star-line" />
            <span
              ><strong>{{ fieldLabel("内部评分") }}</strong
              >{{ selectedResource.score || "-" }}</span
            >
          </div>
        </section>

        <el-tabs v-model="profileTab" class="profile-tabs">
          <el-tab-pane :label="fieldLabel('达人概览')" name="overview">
            <section class="profile-section">
              <div class="profile-section-heading">
                <div>
                  <h3>{{ fieldLabel("关键指标") }}</h3>
                  <span>{{ fieldLabel("平台基础数据与过往合作表现") }}</span>
                </div>
              </div>
              <div class="key-metrics">
                <div>
                  <span>{{ fieldLabel("粉丝 / 订阅") }}</span>
                  <strong>{{ formatCount(selectedResource.followers) }}</strong>
                </div>
                <div>
                  <span>{{ fieldLabel("平台平均播放") }}</span>
                  <strong>{{ formatCount(selectedResource.avgViews) }}</strong>
                </div>
                <div>
                  <span>{{ fieldLabel("平台互动率") }}</span>
                  <strong>{{
                    percentText(selectedResource.engagementRate)
                  }}</strong>
                </div>
                <div>
                  <span>{{ fieldLabel("历史合作次数") }}</span>
                  <strong>{{ selectedResourceCooperationStats.count }}</strong>
                </div>
                <div>
                  <span>{{ fieldLabel("合作总触达") }}</span>
                  <strong>{{
                    formatCount(selectedResourceCooperationStats.reach)
                  }}</strong>
                </div>
                <div>
                  <span>{{ fieldLabel("合作内容互动率") }}</span>
                  <strong>
                    {{
                      ratioPercent(
                        selectedResourceCooperationStats.engagement,
                        selectedResourceCooperationStats.reach
                      )
                    }}
                  </strong>
                </div>
              </div>
            </section>

            <section class="profile-section">
              <div class="profile-section-heading">
                <div>
                  <h3>{{ fieldLabel("AI 匹配判断") }}</h3>
                  <span>{{ fieldLabel("为什么适合当前推广需求") }}</span>
                </div>
              </div>
              <div class="ai-match-panel">
                <strong>{{
                  selectedResource.reason || fieldLabel("综合匹配度较高")
                }}</strong>
                <p>
                  {{ selectedResource.riskTip || fieldLabel("暂无明显风险") }}
                </p>
                <div>
                  <el-tag
                    v-for="rule in selectedResource.hitRules || []"
                    :key="rule"
                    effect="plain"
                    type="success"
                  >
                    {{ rule }}
                  </el-tag>
                </div>
              </div>
            </section>
          </el-tab-pane>

          <el-tab-pane :label="fieldLabel('历史合作')" name="history">
            <section class="profile-section">
              <div class="profile-section-heading">
                <div>
                  <h3>{{ fieldLabel("历史合作记录") }}</h3>
                  <span>{{ fieldLabel("报价、发布表现与团队复盘") }}</span>
                </div>
              </div>
              <el-table
                :data="selectedResourceCooperations"
                class="profile-table"
              >
                <el-table-column
                  prop="projectName"
                  label="Campaign"
                  min-width="180"
                />
                <el-table-column
                  prop="cooperationType"
                  :label="fieldLabel('合作形式')"
                  width="130"
                />
                <el-table-column :label="fieldLabel('报价')" width="130">
                  <template #default="{ row }">
                    {{ moneyWithCurrency(row.quoteAmount, row.currency) }}
                  </template>
                </el-table-column>
                <el-table-column :label="fieldLabel('触达')" width="120">
                  <template #default="{ row }">
                    {{ formatCount(primaryReach(row)) }}
                  </template>
                </el-table-column>
                <el-table-column prop="roi" label="ROI" width="90" />
                <el-table-column
                  prop="teamRating"
                  :label="fieldLabel('团队评分')"
                  width="110"
                />
                <el-table-column
                  prop="notes"
                  :label="fieldLabel('复盘备注')"
                  min-width="200"
                />
              </el-table>
              <el-empty
                v-if="selectedResourceCooperations.length === 0"
                :description="fieldLabel('暂无历史合作记录')"
              />
            </section>
          </el-tab-pane>

          <el-tab-pane :label="fieldLabel('内容交付')" name="delivery">
            <section class="profile-section">
              <div class="profile-section-heading">
                <div>
                  <h3>{{ fieldLabel("内容交付时间线") }}</h3>
                  <span>{{
                    fieldLabel("查看每次合作的制作、审核与发布状态")
                  }}</span>
                </div>
              </div>
              <div class="delivery-list">
                <article
                  v-for="item in selectedResourceCooperations"
                  :key="item.id"
                >
                  <span class="delivery-dot" />
                  <div>
                    <div>
                      <strong>{{ item.projectName }}</strong>
                      <el-tag :type="deliveryType(item)" effect="light">
                        {{ deliveryStage(item) }}
                      </el-tag>
                    </div>
                    <p>
                      {{ item.cooperationType || fieldLabel("未设置合作形式") }}
                      ·
                      {{ item.deliverableStatus || fieldLabel("未开始") }}
                    </p>
                    <span>{{ item.notes || fieldLabel("暂无交付备注") }}</span>
                    <el-link
                      v-if="item.deliverableLinks"
                      :href="item.deliverableLinks"
                      type="primary"
                      target="_blank"
                    >
                      {{ fieldLabel("查看交付内容") }}
                    </el-link>
                  </div>
                  <time>{{
                    item.releaseDate || formatDateTime(item.updatedAt)
                  }}</time>
                </article>
              </div>
              <el-empty
                v-if="selectedResourceCooperations.length === 0"
                :description="fieldLabel('暂无内容交付记录')"
              />
            </section>
          </el-tab-pane>

          <el-tab-pane :label="fieldLabel('平台作品')" name="posts">
            <section class="profile-section">
              <div class="profile-section-heading">
                <div>
                  <h3>{{ fieldLabel("近期平台作品") }}</h3>
                  <span>
                    {{ selectedResourcePostStats.postCount || 0 }}
                    {{ fieldLabel("条作品") }} ·
                    {{ fieldLabel("平均播放") }}
                    {{ formatCount(selectedResourcePostStats.avgViews) }}
                  </span>
                </div>
              </div>
              <div class="profile-post-grid">
                <article v-for="post in selectedResourcePosts" :key="post.id">
                  <img
                    v-if="post.coverUrl"
                    :src="post.coverUrl"
                    :alt="post.title"
                  />
                  <div v-else class="post-cover-empty">
                    <IconifyIconOnline icon="ri:video-line" />
                  </div>
                  <div>
                    <strong>{{
                      post.title || fieldLabel("未命名作品")
                    }}</strong>
                    <p>{{ post.description || fieldLabel("暂无描述") }}</p>
                    <span>
                      {{ fieldLabel("播放") }}
                      {{ formatCount(post.viewCount) }} ·
                      {{ fieldLabel("点赞") }}
                      {{ formatCount(post.likeCount) }} ·
                      {{ fieldLabel("评论") }}
                      {{ formatCount(post.commentCount) }}
                    </span>
                  </div>
                </article>
              </div>
              <el-empty
                v-if="selectedResourcePosts.length === 0"
                :description="fieldLabel('暂无同步作品数据')"
              />
            </section>
          </el-tab-pane>
        </el-tabs>
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.assistant-page {
  min-height: 100%;
  padding: 20px 18px 0;
  color: #171717;
  background: #f7f7f5 !important;
}

/* Let the selection bar stick to the real scrollbar, rather than an
   overflow-hidden intermediate view. This selector only affects this page. */
:global(.el-scrollbar__view:has(.assistant-page)) {
  overflow: visible !important;
}

@media (width <= 760px) {
  .assistant-page {
    padding: 16px 10px 0;
  }
}

.creator-profile {
  min-height: 100%;
  padding: 22px;
  color: #171717;
  background: #f8f8f7;
}

.creator-profile-header {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  gap: 14px;
  align-items: center;
  padding: 18px;
  background: #fff;
  border: 1px solid #e8e8e8;
  border-radius: 14px;
}

.profile-avatar-large {
  position: relative;
  display: grid;
  place-items: center;
  width: 64px;
  height: 64px;
  overflow: hidden;
  font-size: 20px;
  font-weight: 800;
  color: #c2410c;
  background: #fff1e8;
  border-radius: 50%;
}

.profile-avatar-large img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.profile-heading {
  display: grid;
  gap: 7px;
}

.profile-heading > div {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.profile-heading h2 {
  margin: 0;
  font-size: 24px;
  letter-spacing: 0;
}

.profile-heading p {
  margin: 0;
  font-size: 13px;
  color: #777;
}

.profile-header-actions {
  display: flex;
  gap: 8px;
}

.quality-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin: 14px 0;
  overflow: hidden;
  background: #fff;
  border: 1px solid #dceee1;
  border-radius: 12px;
}

.quality-strip > div {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 14px;
  border-right: 1px solid #e4eee7;
}

.quality-strip > div:last-child {
  border-right: 0;
}

.quality-strip svg {
  flex: 0 0 auto;
  font-size: 20px;
  color: #16a34a;
}

.quality-strip span {
  display: grid;
  gap: 3px;
  font-size: 11px;
  color: #8a8a8a;
}

.quality-strip strong {
  font-size: 13px;
  color: #2f5137;
}

.profile-tabs {
  padding: 0 18px 18px;
  background: #fff;
  border: 1px solid #e8e8e8;
  border-radius: 14px;
}

.profile-section {
  padding-top: 8px;
}

.profile-section-heading {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.profile-section-heading > div {
  display: grid;
  gap: 4px;
}

.profile-section-heading h3 {
  margin: 0;
  font-size: 18px;
  letter-spacing: 0;
}

.profile-section-heading span {
  font-size: 12px;
  color: #8a8a8a;
}

.key-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid #e8e8e8;
  border-radius: 12px;
}

.key-metrics > div {
  display: grid;
  gap: 7px;
  min-height: 92px;
  padding: 16px;
  border-right: 1px solid #e8e8e8;
  border-bottom: 1px solid #e8e8e8;
}

.key-metrics > div:nth-child(3n) {
  border-right: 0;
}

.key-metrics > div:nth-last-child(-n + 3) {
  border-bottom: 0;
}

.key-metrics span {
  font-size: 12px;
  color: #8a8a8a;
}

.key-metrics strong {
  font-size: 22px;
  color: #171717;
}

.ai-match-panel {
  display: grid;
  gap: 10px;
  padding: 16px;
  margin-top: 18px;
  background: #fffaf6;
  border: 1px solid #fed7aa;
  border-radius: 12px;
}

.ai-match-panel p {
  margin: 0;
  font-size: 13px;
  color: #8a5b43;
}

.ai-match-panel > div {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.profile-table {
  border: 1px solid #e8e8e8;
  border-radius: 10px;
}

.delivery-list {
  display: grid;
  padding-left: 8px;
}

.delivery-list article {
  position: relative;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 14px;
  min-height: 120px;
  padding-bottom: 20px;
}

.delivery-list article::before {
  position: absolute;
  top: 20px;
  bottom: 0;
  left: 6px;
  width: 2px;
  content: "";
  background: #fed7aa;
}

.delivery-list article:last-child::before {
  display: none;
}

.delivery-dot {
  z-index: 1;
  width: 14px;
  height: 14px;
  margin-top: 4px;
  background: #ff6a2a;
  border: 3px solid #fff3ea;
  border-radius: 50%;
}

.delivery-list article > div {
  display: grid;
  gap: 7px;
}

.delivery-list article > div > div {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.delivery-list p,
.delivery-list span,
.delivery-list time {
  margin: 0;
  font-size: 12px;
  color: #8a8a8a;
}

.profile-post-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.profile-post-grid article {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  gap: 12px;
  overflow: hidden;
  border: 1px solid #e8e8e8;
  border-radius: 10px;
}

.profile-post-grid img,
.post-cover-empty {
  width: 120px;
  height: 100px;
  object-fit: cover;
}

.post-cover-empty {
  display: grid;
  place-items: center;
  font-size: 28px;
  color: #b8b8b8;
  background: #f1f1f1;
}

.profile-post-grid article > div:last-child {
  display: grid;
  gap: 6px;
  align-content: center;
  min-width: 0;
  padding: 10px 10px 10px 0;
}

.profile-post-grid strong,
.profile-post-grid p,
.profile-post-grid span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-post-grid p,
.profile-post-grid span {
  margin: 0;
  font-size: 11px;
  color: #8a8a8a;
}

.logic-section p {
  margin: 0;
  font-size: 14px;
  line-height: 1.85;
}

@media (width <= 1180px) {
  .creator-profile-header {
    grid-template-columns: auto auto minmax(0, 1fr);
  }

  .profile-header-actions {
    grid-column: 2 / -1;
  }
}

@media (width <= 760px) {
  :global(.creator-profile-drawer) {
    width: 100% !important;
  }

  .quality-strip,
  .key-metrics,
  .profile-post-grid {
    grid-template-columns: 1fr;
  }

  .quality-strip > div,
  .key-metrics > div {
    border-right: 0;
    border-bottom: 1px solid #e8e8e8;
  }

  .creator-profile {
    padding: 12px;
  }

  .creator-profile-header {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .profile-heading {
    grid-column: 2;
  }

  .profile-avatar-large {
    grid-column: 1;
  }

  .profile-header-actions {
    display: flex;
    flex-wrap: wrap;
    grid-column: 1 / -1;
  }
}
</style>
