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
import { formatBytes } from "@pureadmin/utils";
import VuePdfEmbed, { useVuePdfEmbed } from "vue-pdf-embed";
import { Icon } from "@iconify/vue";
import dayjs from "dayjs";
import { storeToRefs } from "pinia";
import { useIPRecommendationSessionsStore } from "@/store/modules/ipRecommendationSessions";
import { useUserStoreHook } from "@/store/modules/user";
import { getToken } from "@/utils/auth";
import { useAppStoreHook } from "@/store/modules/app";
import { useSettingStoreHook } from "@/store/modules/settings";
import * as XLSX from "xlsx";
import { workspaceIcons as icons } from "./icons";
import ExternalResearchWorkbench from "./ExternalResearchWorkbench.vue";
import {
  streamIPRecommendations,
  type IPRecommendationPresentation,
  type IPRecommendationMessage,
  deleteRecommendationSubscription,
  listRecommendationResults,
  listRecommendationSubscriptions,
  saveRecommendationSubscription,
  setRecommendationSubscriptionActive,
  sendRecommendationFeedback,
  type RecommendationResult,
  type RecommendationSubscription,
  type RecommendationSubscriptionInput
} from "@/api/recommendations";
import {
  IP_DOCUMENT_TIMEOUT_MS,
  getIPRequest,
  getIPResource,
  readIPFile
} from "@/api/ip";

defineOptions({ name: "IPRecommendations" });

const route = useRoute();
const router = useRouter();
const loading = ref(false);
const saving = ref(false);
const sessionStore = useIPRecommendationSessionsStore();
const sessionToken = getToken();
const sessionOwnerKey = sessionToken
  ? `${useUserStoreHook().username}:${sessionToken.sessionExpires}`
  : "";
sessionStore.bindOwner(sessionOwnerKey);
function sessionViewCurrent() {
  return !disposed && sessionStore.ownerKey === sessionOwnerKey;
}
const { subscriptions, selectedId } = storeToRefs(sessionStore);
const appStore = useAppStoreHook();
const settingStore = useSettingStoreHook();
const showInlineSessions = computed(
  () =>
    settingStore.hiddenSideBar ||
    appStore.layout === "horizontal" ||
    !appStore.sidebar.opened ||
    appStore.device === "mobile"
);
const results = ref<RecommendationResult[]>([]);
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
const previewDownloading = ref(false);
const previewDownloaded = ref(0);
const previewTotal = ref(0);
const previewError = ref("");
const previewFile = ref<any>(null);
let previewRequestId = 0;
let disposed = false;
const previewDownloadPercent = computed(() =>
  previewTotal.value > 0
    ? Math.min(
        99,
        Math.floor((previewDownloaded.value / previewTotal.value) * 100)
      )
    : 0
);
const previewDownloadText = computed(() =>
  previewTotal.value > 0
    ? `正在下载 PDF · ${formatBytes(previewDownloaded.value)} / ${formatBytes(previewTotal.value)}（${previewDownloadPercent.value}%）`
    : previewDownloaded.value > 0
      ? `正在下载 PDF · 已下载 ${formatBytes(previewDownloaded.value)}`
      : "正在连接并下载 PDF…"
);
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
const visibleResults = computed(() =>
  results.value.filter(item => item.feedback !== "not_interested")
);

function blankForm(): RecommendationSubscriptionInput {
  return {
    id: undefined,
    domain: "ip",
    name: "",
    query: "",
    markets: [],
    ipTypes: [],
    excludedIPTypes: [],
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

function scenarioItems(value: RecommendationResult["scenarios"]): string[] {
  return Array.isArray(value)
    ? value.filter(item => typeof item === "string" && item.trim())
    : [];
}

function evidenceItems(value: RecommendationResult["evidence"]) {
  return Array.isArray(value)
    ? value.filter(item => item && (item.title || item.source))
    : [];
}

function safeExternalUrl(value: unknown): string | undefined {
  try {
    const url = new URL(String(value || ""));
    return url.protocol === "https:" ? url.href : undefined;
  } catch {
    return undefined;
  }
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
  const requestId = ++previewRequestId;
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value);
  previewUrl.value = "";
  previewLoading.value = false;
  previewDownloading.value = true;
  previewDownloaded.value = 0;
  previewTotal.value = 0;
  previewError.value = "";
  previewFile.value = file;
  previewFileName.value = String(file.originalName || "版权资料.pdf");
  previewPages.value = 0;
  previewVisible.value = true;
  try {
    const blob = await readIPFile(Number(file.id), {
      timeout: IP_DOCUMENT_TIMEOUT_MS,
      onDownloadProgress: progress => {
        if (requestId !== previewRequestId || !previewVisible.value || disposed)
          return;
        previewDownloaded.value = progress.loaded;
        if (progress.lengthComputable && progress.total > 0)
          previewTotal.value = progress.total;
      }
    });
    if (requestId !== previewRequestId || !previewVisible.value || disposed)
      return;
    previewUrl.value = URL.createObjectURL(
      new Blob([blob], { type: "application/pdf" })
    );
    previewDownloading.value = false;
    previewLoading.value = true;
  } catch {
    if (requestId === previewRequestId && previewVisible.value) {
      previewDownloading.value = false;
      previewError.value = "资料下载失败，请检查网络后重试";
    }
  }
}

function closePreview() {
  ++previewRequestId;
  previewDownloading.value = false;
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value);
  previewUrl.value = "";
}

function pdfFailed() {
  previewLoading.value = false;
  previewError.value = "PDF 无法预览，请下载文件查看";
}

async function loadSubscriptions(selectId?: number, force = false) {
  try {
    await sessionStore.refresh(async () => {
      const response = await listRecommendationSubscriptions("ip");
      return response.data.list || [];
    }, force);
  } catch (error) {
    if (!subscriptions.value.length) throw error;
  }
  if (!sessionViewCurrent()) return;
  const requested = selectId || Number(route.query.subscriptionId);
  if (requested && subscriptions.value.some(item => item.id === requested))
    selectedId.value = requested;
  else if (
    !selectedId.value ||
    !subscriptions.value.some(item => item.id === selectedId.value)
  )
    selectedId.value = subscriptions.value[0]?.id || null;
}

let resultsRequestSequence = 0;
async function loadResults() {
  const sequence = ++resultsRequestSequence;
  const subscriptionId = selectedId.value;
  if (!selectedId.value) {
    results.value = [];
    return;
  }
  loading.value = true;
  try {
    const response = await listRecommendationResults(
      "ip",
      subscriptionId,
      "all"
    );
    if (
      sequence !== resultsRequestSequence ||
      selectedId.value !== subscriptionId ||
      !sessionViewCurrent()
    )
      return;
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
  } catch (error) {
    if (sequence === resultsRequestSequence)
      ElMessage.error(
        error instanceof Error ? error.message : "候选暂时无法加载，请重试"
      );
  } finally {
    if (sequence === resultsRequestSequence) loading.value = false;
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
  if (conversationBusy.value || attachmentReading.value) return;
  if (selectedId.value === id) return;
  ++resultsRequestSequence;
  results.value = [];
  if (selectedId.value !== id)
    chosenIds.value =
      selectionKey.value || route.query.selected ? selectedFromRequest() : [];
  selectedId.value = id;
  composerText.value = "";
  conversationError.value = "";
  attachment.value = null;
  historyOpen.value = false;
  editing.value = false;
  activeView.value = "all";
  compareIds.value = [];
  workbenchTab.value = "candidates";
  setForm(selectedSubscription.value || {});
  const query = { ...route.query };
  delete query.resultId;
  query.subscriptionId = String(id);
  router.replace({
    path: route.path,
    query
  });
}

function startNew() {
  if (conversationBusy.value || attachmentReading.value) return;
  setForm({ requestId: requestId.value || undefined });
  ++resultsRequestSequence;
  loading.value = false;
  selectedId.value = null;
  results.value = [];
  compareIds.value = [];
  chosenIds.value = [];
  composerText.value = "";
  conversationError.value = "";
  streamingPresentation.value = null;
  pendingUserMessage.value = null;
  resumableSessionId.value = null;
  attachment.value = null;
  historyOpen.value = false;
  workbenchTab.value = "candidates";
  editing.value = false;
  activeView.value = "all";
  const query = { ...route.query };
  delete query.subscriptionId;
  delete query.resultId;
  void router.replace({ path: route.path, query });
}

function editSubscription(item: RecommendationSubscription) {
  setForm({
    ...item,
    budgetMin: item.budgetMin ?? null,
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
    if (!sessionViewCurrent()) return;
    editing.value = false;
    activeView.value = "all";
    await loadSubscriptions(response.data.id, true);
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

async function toggleSubscription(item: RecommendationSubscription) {
  if (conversationBusy.value || attachmentReading.value || saving.value) return;
  saving.value = true;
  try {
    const response = await setRecommendationSubscriptionActive(
      "ip",
      item.id,
      !item.active
    );
    if (!sessionViewCurrent()) return;
    sessionStore.upsert({ ...item, active: !item.active });
    ElMessage.success(item.active ? "已暂停每周推荐" : "已恢复每周推荐");
    if (response.data.warning) ElMessage.warning(response.data.warning);
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "更新提醒状态失败"
    );
  } finally {
    saving.value = false;
  }
}

async function removeSubscription(item: RecommendationSubscription) {
  if (conversationBusy.value || attachmentReading.value || saving.value) return;
  try {
    await ElMessageBox.confirm(
      `删除项目会话「${item.name}」及其对话、推荐历史？删除后无法恢复。`,
      "删除项目会话",
      {
        type: "warning",
        confirmButtonText: "删除会话",
        cancelButtonText: "取消"
      }
    );
  } catch {
    return;
  }
  if (conversationBusy.value || attachmentReading.value || saving.value) return;
  saving.value = true;
  try {
    await deleteRecommendationSubscription("ip", item.id);
    if (!sessionViewCurrent()) return;
    const wasSelected = selectedId.value === item.id;
    sessionStore.remove(item.id);
    if (wasSelected) {
      const next = subscriptions.value[0]?.id;
      if (next) selectSubscription(next);
      else startNew();
    }
    ElMessage.success("项目会话已删除");
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "删除项目会话失败"
    );
  } finally {
    saving.value = false;
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
        selectedId.value = null;
        results.value = [];
        editing.value = false;
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
      const existing = subscriptions.value.find(
        value => value.requestId === requestId.value
      );
      selectedId.value = existing?.id || null;
      if (existing) {
        setForm(existing);
      } else results.value = [];
      editing.value = false;
      expectedLaunch.value = String(item.expectedLaunch || "").slice(0, 10);
    } catch {
      ElMessage.warning("未能读取关联需求，请手动填写寻找条件");
    }
  }
}

let workspaceReady = false;
onMounted(async () => {
  try {
    await loadSubscriptions();
    if (disposed) return;
    await loadRequestContext();
    if (disposed) return;
    await loadResults();
    if (disposed) return;
    if (selectedSubscription.value) setForm(selectedSubscription.value);
    chosenIds.value = selectedFromRequest();
  } catch {
    if (!disposed) ElMessage.error("IP 推荐暂时无法加载，请稍后重试");
  } finally {
    workspaceReady = !disposed;
  }
});
watch(
  () =>
    String(route.query.subscriptionId || "") +
    ":" +
    String(route.query.resultId || ""),
  async () => {
    if (!workspaceReady || disposed) return;
    const id = Number(route.query.subscriptionId);
    if (
      id > 0 &&
      subscriptions.value.some(item => item.id === id) &&
      selectedId.value !== id
    ) {
      ++conversationSequence;
      conversationController?.abort();
      conversationBusy.value = false;
      composerText.value = "";
      attachment.value = null;
      selectedId.value = id;
      setForm(selectedSubscription.value || {});
    }
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

const composerText = ref("");
const webSearchMode = ref(false);
const conversationBusy = ref(false);
const conversationError = ref("");
const streamingPresentation = ref<IPRecommendationPresentation | null>(null);
const streamingStatus = ref("");
const pendingUserMessage = ref<IPRecommendationMessage | null>(null);
const resumableSessionId = ref<number | null>(null);
let conversationController: AbortController | null = null;
let failedConversation: {
  action: "clarify" | "recommend" | "compare" | "discuss" | "search";
  text: string;
} | null = null;
const workbenchTab = ref<"candidates" | "compare" | "intent" | "external">(
  "candidates"
);
const compareIds = ref<number[]>([]);
const externalCompareCount = ref(0);
const activeCandidateId = ref<number | null>(null);
const expectedLaunch = ref("");
const historyOpen = ref(false);
const conditionsOpen = ref(false);
const stageLabels = ["需求理解", "条件确认", "候选研究", "比较决策"];
const attachmentReading = ref(false);
const attachment = ref<{ name: string; text: string } | null>(null);
const canSendMessage = computed(
  () =>
    !conversationBusy.value &&
    !attachmentReading.value &&
    composerText.value.trim().length > 0
);
const attachmentInput = ref<HTMLInputElement>();
const pdfSource = ref<{ data: Uint8Array } | undefined>();
const pdfReadError = ref("");
const { doc: attachmentPdf } = useVuePdfEmbed({
  source: pdfSource,
  onError: () => {
    pdfReadError.value = "PDF 无法读取，请上传文本资料或直接填写需求";
  }
});
let conversationSequence = 0;
const conversationMessages = computed<IPRecommendationMessage[]>(
  () => selectedSubscription.value?.ipWorkspace?.messages || []
);
const latestUser = computed(
  () =>
    pendingUserMessage.value ||
    [...conversationMessages.value]
      .reverse()
      .find(message => message.role === "user")
);
const assistantMessages = computed(() =>
  conversationMessages.value.filter(message => message.role === "assistant")
);
const latestWebSearch = computed(
  () =>
    streamingPresentation.value?.webSearch ||
    [...assistantMessages.value]
      .reverse()
      .find(message => message.presentation?.webSearch)?.presentation?.webSearch
);
const latestAssistant = computed(() =>
  conversationBusy.value ? undefined : assistantMessages.value.at(-1)
);
const currentCandidate = computed(
  () =>
    visibleResults.value.find(
      item => item.resourceId === activeCandidateId.value
    ) || visibleResults.value[0]
);
const comparison = computed(() =>
  visibleResults.value.filter(item =>
    compareIds.value.includes(item.resourceId)
  )
);
const intentItems = computed(() =>
  visibleResults.value.filter(item => chosenIds.value.includes(item.resourceId))
);
const currentStage = computed(() => {
  if (!selectedSubscription.value) return 0;
  if (
    workbenchTab.value === "external" &&
    latestWebSearch.value?.candidates?.length
  )
    return externalCompareCount.value >= 2 ? 3 : 2;
  if (
    selectedSubscription.value.ipWorkspace?.stage === "confirm" ||
    !visibleResults.value.length
  )
    return 1;
  return selectedSubscription.value.ipWorkspace?.stage === "compare" ? 3 : 2;
});
const presentation = computed<IPRecommendationPresentation | null>(() => {
  if (streamingPresentation.value) return streamingPresentation.value;
  if (conversationBusy.value) return null;
  if (latestAssistant.value?.presentation)
    return latestAssistant.value.presentation;
  const item = currentCandidate.value;
  if (!item) return null;
  return {
    headline:
      item.assessment?.verdict ||
      `进一步评估「${item.resourceName}」与当前需求的合作适配`,
    explanation: item.assessment?.fit || item.advice || item.reason,
    basis: item.assessment?.basis || [
      { label: "筛选依据", value: item.reason },
      { label: "区域条件", value: "具体授权地域须与版权方确认" }
    ],
    adjustments: [],
    questions: [],
    suggestedPrompts: ["为什么优先它？", "更看重产品设计与礼盒联名"],
    primaryResourceId: item.resourceId,
    source: item.adviceSource === "ai" ? "ai" : "rule"
  };
});
const primaryCandidate = computed(
  () =>
    visibleResults.value.find(
      item => item.resourceId === presentation.value?.primaryResourceId
    ) || currentCandidate.value
);
const evidenceCandidates = computed(() => {
  const ids = [
    presentation.value?.primaryResourceId,
    presentation.value?.alternativeResourceId
  ];
  return visibleResults.value
    .filter(item => ids.includes(item.resourceId))
    .slice(0, 2);
});
const criteria = computed(() => selectedSubscription.value || form);
const criteriaText = computed(() =>
  [
    criteria.value.markets?.join(" / "),
    criteria.value.ipTypes?.join(" / "),
    criteria.value.excludedIPTypes?.length
      ? `排除${criteria.value.excludedIPTypes.join(" / ")}`
      : "",
    criteria.value.budgetMax
      ? `授权预算 ${criteria.value.budgetCurrency} ${criteria.value.budgetMax.toLocaleString()}`
      : "预算待确认"
  ]
    .filter(Boolean)
    .join(" · ")
);
const projectLabel = computed(
  () =>
    selectedSubscription.value?.name || currentRequestName.value || "新合作研究"
);
const comparisonFields = [
  { key: "fit", label: "需求契合" },
  { key: "entry", label: "合作切入" },
  { key: "pending", label: "待确认" }
];
function comparisonValue(item: RecommendationResult, field: string) {
  if (field === "fit")
    return item.assessment?.fit || item.reason || "资料待完善";
  if (field === "entry")
    return (
      (item.assessment?.entryPoints || scenarioItems(item.scenarios)).join(
        " / "
      ) || "需与版权方讨论合作方式"
    );
  const pending = [
    ...new Set([...(item.assessment?.pending || []), ...riskItems(item.risks)])
  ].filter(
    value =>
      ![
        "授权范围、报价与档期待确认",
        "授权范围、报价与档期须与版权方确认"
      ].includes(value)
  );
  return pending.join("；") || "授权范围、报价与档期待确认";
}
function showConditions() {
  if (selectedSubscription.value) editSubscription(selectedSubscription.value);
  else editing.value = true;
}
function selectStage(index: number) {
  if (index < 2) {
    conditionsOpen.value = !conditionsOpen.value;
    return;
  }
  if (
    latestWebSearch.value?.candidates?.length &&
    (workbenchTab.value === "external" || !visibleResults.value.length)
  ) {
    workbenchTab.value = "external";
    return;
  }
  workbenchTab.value = index === 3 ? "compare" : "candidates";
}
function toggleCompare(item: RecommendationResult) {
  if (compareIds.value.includes(item.resourceId))
    compareIds.value = compareIds.value.filter(id => id !== item.resourceId);
  else if (compareIds.value.length >= 3) ElMessage.info("最多同时对比 3 个 IP");
  else compareIds.value = [...compareIds.value, item.resourceId];
}
async function converse(
  action: "clarify" | "recommend" | "compare" | "discuss" | "search",
  text = composerText.value.trim()
) {
  if (conversationBusy.value || attachmentReading.value) return;
  if (action === "search" && (!text || [...text].length > 200)) {
    ElMessage.info("请填写 200 字以内的公开搜索关键词");
    return;
  }
  if (!text && !criteria.value.query && !attachment.value) {
    ElMessage.info("请先描述项目需求");
    return;
  }
  if (action === "compare" && comparison.value.length < 2) {
    workbenchTab.value = "candidates";
    ElMessage.info("请先选择 2 至 3 个候选 IP");
    return;
  }
  const sequence = ++conversationSequence;
  const sessionId = selectedId.value || resumableSessionId.value;
  conversationBusy.value = true;
  conversationError.value = "";
  streamingPresentation.value = null;
  streamingStatus.value = "正在理解项目需求与偏好…";
  pendingUserMessage.value = text
    ? {
        role: "user",
        content: text,
        createdAt: dayjs().format("YYYY-MM-DD HH:mm")
      }
    : null;
  const controller = new AbortController();
  conversationController = controller;
  try {
    const response = await streamIPRecommendations(
      {
        subscriptionId: sessionId || undefined,
        criteria: sessionId
          ? undefined
          : {
              ...form,
              query: form.query || text,
              name: form.name || text.slice(0, 30),
              requestId: requestId.value || undefined
            },
        message: text,
        action,
        compareIds: action === "compare" ? compareIds.value : [],
        documentText:
          attachment.value && (action === "clarify" || action === "recommend")
            ? `【${attachment.value.name.slice(0, 120)}】\n${attachment.value.text}`
            : undefined
      },
      {
        signal: controller.signal,
        onStatus(message) {
          if (sequence === conversationSequence && !disposed)
            streamingStatus.value = message;
        },
        onSession(id) {
          if (sequence === conversationSequence && !disposed)
            resumableSessionId.value = id;
        },
        onPresentation(value) {
          if (sequence === conversationSequence && !disposed)
            streamingPresentation.value = value;
        }
      }
    );
    if (sequence !== conversationSequence || !sessionViewCurrent()) return;
    const updated = response.data.subscription;
    sessionStore.upsert(updated);
    selectedId.value = updated.id;
    setForm(updated);
    // Comparisons return the selected subset; preserve the full candidate set.
    if (action !== "compare") results.value = response.data.results || [];
    compareIds.value = compareIds.value.filter(id =>
      visibleResults.value.some(item => item.resourceId === id)
    );
    activeCandidateId.value =
      response.data.presentation.primaryResourceId ||
      visibleResults.value[0]?.resourceId ||
      null;
    if (action === "compare") workbenchTab.value = "compare";
    else if (action === "search") {
      workbenchTab.value = "external";
      webSearchMode.value = true;
    } else if (action === "recommend") workbenchTab.value = "candidates";
    composerText.value = "";
    streamingPresentation.value = null;
    pendingUserMessage.value = null;
    resumableSessionId.value = null;
    failedConversation = null;
    if (action === "clarify" || action === "recommend") attachment.value = null;
    void loadImages(results.value);
  } catch (error) {
    if (sequence === conversationSequence) {
      failedConversation = { action, text };
      conversationError.value = controller.signal.aborted
        ? "已停止生成，当前回答尚未完成；可保留需求并重试。"
        : error instanceof Error
          ? error.message
          : "对话生成失败，请保留需求并重试";
    }
  } finally {
    if (sequence === conversationSequence) {
      conversationBusy.value = false;
      conversationController = null;
    }
  }
}
function stopConversation() {
  conversationController?.abort();
}
function retryConversation() {
  if (failedConversation)
    return converse(
      failedConversation.action,
      composerText.value.trim() || failedConversation.text
    );
  return sendMessage();
}
function sendMessage() {
  if (!canSendMessage.value) return;
  const text = composerText.value.trim();
  if (webSearchMode.value) return converse("search", text);
  if (
    !selectedSubscription.value ||
    selectedSubscription.value.ipWorkspace?.stage === "confirm"
  )
    return converse("clarify", text);
  if (attachment.value) return converse("recommend", text);
  if (
    visibleResults.value.length &&
    /^(为什么|为何|解释|说明|说说)|推荐依据|判断依据|推荐理由|适配原因/.test(
      text
    )
  )
    return converse("discuss", text);
  if (
    comparison.value.length >= 2 &&
    /(?:比较|对比).*(?:这|它们|两个|选中|候选)/.test(text)
  )
    return converse("compare", text);
  return converse("recommend", text);
}
function composerKeydown(event: KeyboardEvent) {
  if (
    event.key === "Enter" &&
    (event.ctrlKey || event.metaKey) &&
    !event.isComposing
  ) {
    event.preventDefault();
    void sendMessage();
  }
}
function followUp(text: string) {
  composerText.value = text;
  void converse(
    webSearchMode.value || presentation.value?.webSearch
      ? "search"
      : text.startsWith("为什么")
        ? "discuss"
        : "recommend",
    text
  );
}
function queueIntent(item: RecommendationResult) {
  if (chosenIds.value.includes(item.resourceId)) {
    workbenchTab.value = "intent";
    return;
  }
  if (chosenIds.value.length >= 5) {
    ElMessage.info("初步意向最多选择 5 个 IP");
    return;
  }
  chosenIds.value = [...chosenIds.value, item.resourceId];
  workbenchTab.value = "intent";
}
function removeIntent(id: number) {
  chosenIds.value = chosenIds.value.filter(value => value !== id);
}
function confirmIntent() {
  if (!chosenIds.value.length && primaryCandidate.value)
    queueIntent(primaryCandidate.value);
  if (!chosenIds.value.length) return;
  if (selectionKey.value) {
    localStorage.setItem(selectionKey.value, JSON.stringify(chosenIds.value));
    ElMessage.success("已同步到当前提需草稿，请返回需求页保存");
    return;
  }
  router.push({
    path: "/business/ip/requests",
    query: {
      mode: "new",
      recommendedIps: chosenIds.value.join(","),
      recommendationReason:
        presentation.value?.explanation || primaryCandidate.value?.reason || ""
    }
  });
}
async function readAttachment(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  input.value = "";
  if (file.size > 15 * 1024 * 1024) {
    ElMessage.warning("需求资料请控制在 15MB 以内");
    return;
  }
  attachmentReading.value = true;
  try {
    const extension = file.name.split(".").pop()?.toLowerCase();
    let text = "";
    if (["txt", "md", "csv"].includes(extension || ""))
      text = await file.text();
    else if (["xlsx", "xls"].includes(extension || "")) {
      const workbook = XLSX.read(await file.arrayBuffer(), { type: "array" });
      text = workbook.SheetNames.slice(0, 3)
        .map(
          name => `${name}\n${XLSX.utils.sheet_to_csv(workbook.Sheets[name])}`
        )
        .join("\n");
    } else if (extension === "pdf") {
      pdfReadError.value = "";
      pdfSource.value = undefined;
      await nextTick();
      const ready = new Promise<void>((resolve, reject) => {
        const timer = window.setTimeout(() => {
          stop();
          reject(new Error("PDF 读取超时，请换用文本资料"));
        }, 20000);
        const stop = watch(
          [attachmentPdf, pdfReadError],
          ([document, error]) => {
            if (!document && !error) return;
            stop();
            window.clearTimeout(timer);
            if (error) reject(new Error(error));
            else resolve();
          }
        );
      });
      pdfSource.value = { data: new Uint8Array(await file.arrayBuffer()) };
      await ready;
      const document = attachmentPdf.value;
      if (!document) throw new Error("PDF 正文未能读取");
      const pages = [];
      for (let page = 1; page <= Math.min(document.numPages, 12); page++) {
        const content = await (await document.getPage(page)).getTextContent();
        pages.push(
          content.items.map(item => ("str" in item ? item.str : "")).join(" ")
        );
      }
      text = pages.join("\n");
    } else throw new Error("请上传 PDF、TXT、Markdown 或表格资料");
    if (!text.trim())
      throw new Error("未能提取正文；扫描版 PDF 请先转为文本或直接粘贴需求");
    attachment.value = { name: file.name, text: text.slice(0, 3800) };
    if (text.length > 3800)
      ElMessage.info("资料已读取，将使用前 3800 字作为需求补充");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "资料读取失败");
  } finally {
    attachmentReading.value = false;
    pdfSource.value = undefined;
  }
}
watch(selectedId, () => {
  if (conversationBusy.value || attachmentReading.value) return;
  ++conversationSequence;
  conversationError.value = "";
  streamingPresentation.value = null;
  pendingUserMessage.value = null;
  resumableSessionId.value = null;
  compareIds.value = [];
  webSearchMode.value =
    !!selectedSubscription.value?.ipWorkspace?.messages.at(-1)?.presentation
      ?.webSearch;
  workbenchTab.value = webSearchMode.value ? "external" : "candidates";
  activeCandidateId.value = null;
});
watch(
  () =>
    selectedSubscription.value?.ipWorkspace?.messages.at(-1)?.presentation
      ?.webSearch,
  research => {
    if (!conversationBusy.value) {
      webSearchMode.value = !!research;
      workbenchTab.value = research ? "external" : "candidates";
    }
  },
  { immediate: true }
);
watch(visibleResults, () => {
  if (!compareIds.value.length && visibleResults.value.length >= 2)
    compareIds.value = visibleResults.value
      .slice(0, 2)
      .map(item => item.resourceId);
});
onUnmounted(() => {
  ++conversationSequence;
  conversationController?.abort();
  if (sessionStore.ownerKey === sessionOwnerKey) sessionStore.releaseView();
});

watch(
  () => conversationBusy.value || attachmentReading.value || saving.value,
  busy => {
    if (sessionStore.ownerKey === sessionOwnerKey) sessionStore.busy = busy;
  },
  { immediate: true, flush: "sync" }
);
watch(
  () => sessionStore.navigation,
  async request => {
    if (!request || sessionStore.busy) return;
    if (request.action === "new") startNew();
    else if (request.action === "select") selectSubscription(request.id);
    else if (request.action === "delete") {
      const item = subscriptions.value.find(value => value.id === request.id);
      if (item) await removeSubscription(item);
    } else {
      try {
        await loadSubscriptions(undefined, true);
        await loadResults();
      } catch {
        ElMessage.error("项目会话暂时无法加载，请稍后重试");
      }
    }
  }
);
</script>

<template>
  <div v-loading="loading" class="ip-recommend-page">
    <div class="research-main">
      <div class="research-session-bar">
        <div v-if="showInlineSessions" class="session-switch">
          <span>项目会话</span>
          <el-select
            class="project-session-select"
            :model-value="selectedId || 0"
            :disabled="conversationBusy || attachmentReading"
            aria-label="选择项目会话"
            @change="value => (value ? selectSubscription(value) : startNew())"
          >
            <el-option label="新合作研究" :value="0" />
            <el-option
              v-for="item in subscriptions"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            />
          </el-select>
          <el-button
            :disabled="conversationBusy || attachmentReading"
            @click="startNew"
          >
            <Icon :icon="icons.plus" />新建会话
          </el-button>
        </div>
        <h2 v-else class="project-title" :title="projectLabel">
          {{ projectLabel }}
        </h2>
        <div class="research-tools">
          <el-dropdown
            trigger="click"
            :disabled="conversationBusy"
            @command="value => selectStage(Number(value))"
          >
            <button
              type="button"
              class="quiet-button stage-switch"
              :disabled="conversationBusy"
              :aria-label="`研究阶段：${stageLabels[currentStage]}`"
            >
              <span class="stage-indicator" aria-hidden="true" />{{
                stageLabels[currentStage]
              }}<Icon icon="ri:arrow-down-s-line" />
            </button>
            <template #dropdown>
              <el-dropdown-menu aria-label="推荐研究阶段">
                <el-dropdown-item
                  v-for="(label, index) in stageLabels"
                  :key="label"
                  :command="index"
                  :aria-current="currentStage === index ? 'step' : undefined"
                  :disabled="
                    index > 1 &&
                    !visibleResults.length &&
                    !latestWebSearch?.candidates?.length
                  "
                >
                  {{ label
                  }}<Icon v-if="currentStage === index" :icon="icons.check" />
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <button
            type="button"
            class="quiet-button conditions-toggle"
            :aria-expanded="conditionsOpen"
            aria-controls="research-criteria"
            @click="conditionsOpen = !conditionsOpen"
          >
            <Icon :icon="icons.file" />需求条件<Icon
              :icon="
                conditionsOpen ? 'ri:arrow-up-s-line' : 'ri:arrow-down-s-line'
              "
            />
          </button>
        </div>
      </div>
      <div
        v-if="conditionsOpen"
        id="research-criteria"
        class="expanded-criteria"
      >
        <div class="criteria-summary">
          <span
            >{{ criteria.goal || "合作目标待确认" }} · {{ criteriaText }}</span
          >
          <button
            type="button"
            class="quiet-button"
            :disabled="conversationBusy"
            @click="showConditions"
          >
            <Icon :icon="icons.edit" />编辑条件
          </button>
        </div>
        <span v-if="expectedLaunch">预计上线：{{ expectedLaunch }}</span>
        <router-link
          v-if="requestId"
          :to="{ path: '/business/ip/requests', query: { id: requestId } }"
          class="context-link"
          ><Icon :icon="icons.external" />查看原始提需</router-link
        >
        <p>{{ criteria.query || "尚未填写项目需求" }}</p>
        <span v-if="criteria.audience">目标受众：{{ criteria.audience }}</span>
        <span v-if="criteria.product">关联产品：{{ criteria.product }}</span>
        <button
          v-if="selectedSubscription"
          type="button"
          :disabled="conversationBusy"
          @click="toggleSubscription(selectedSubscription)"
        >
          {{
            selectedSubscription.active
              ? "暂停每周检查库内新匹配"
              : "开启每周检查库内新匹配"
          }}
        </button>
      </div>
      <div class="research-columns">
        <section class="conversation-panel" aria-label="项目研究对话">
          <div class="conversation-body">
            <template v-if="!presentation && !conversationBusy">
              <div class="conversation-welcome">
                <Icon :icon="icons.spark" />
                <h2>这次想寻找怎样的合作 IP？</h2>
                <p>
                  描述项目方向，或带入已有提需与资料。我们会一起确认偏好，再寻找值得进一步评估的候选。
                </p>
                <div class="entry-actions">
                  <button type="button" @click="showConditions">
                    <Icon :icon="icons.file" />填写 / 关联提需</button
                  ><button type="button" @click="attachmentInput?.click()">
                    <Icon :icon="icons.attach" />上传需求资料
                  </button>
                </div>
                <button
                  type="button"
                  class="welcome-example"
                  @click="
                    composerText =
                      '为新品寻找合作IP，重点是沉浸体验和设计表达，优先东南亚市场。'
                  "
                >
                  试试：为新品寻找设计与沉浸体验方向的 IP
                  <Icon :icon="icons.arrow" />
                </button>
              </div>
            </template>
            <button
              v-if="conversationMessages.length > 2"
              type="button"
              class="history-toggle"
              :aria-expanded="historyOpen"
              @click="historyOpen = !historyOpen"
            >
              {{
                historyOpen
                  ? "收起历史对话"
                  : `查看历史对话 · ${conversationMessages.length - 2} 条`
              }}
            </button>
            <div v-if="historyOpen" class="conversation-history">
              <div
                v-for="(message, index) in conversationMessages.slice(0, -2)"
                :key="index"
                :class="message.role"
              >
                <small
                  >{{
                    message.role === "user"
                      ? "我"
                      : message.presentation?.source === "ai"
                        ? "AI推荐助手"
                        : "规则建议"
                  }}
                  · {{ formatTime(message.createdAt) }}</small
                >
                <p>{{ message.content }}</p>
              </div>
            </div>
            <div v-if="latestUser" class="user-message">
              <small
                >我
                <time>{{
                  formatTime(latestUser.createdAt).slice(-5)
                }}</time></small
              >
              <p>{{ latestUser.content }}</p>
            </div>
            <article
              v-if="presentation"
              class="assistant-message"
              aria-live="polite"
            >
              <div class="assistant-identity">
                <span>{{
                  presentation.source === "ai"
                    ? "AI"
                    : presentation.webSearch
                      ? "检索"
                      : "规则"
                }}</span
                ><strong>{{
                  presentation.source === "ai"
                    ? "AI 推荐助手"
                    : presentation.webSearch
                      ? "IP 研究助手"
                      : "规则合作建议"
                }}</strong
                ><time v-if="latestAssistant && !streamingPresentation">{{
                  formatTime(latestAssistant.createdAt).slice(-5)
                }}</time>
              </div>
              <div class="assistant-content">
                <h2>{{ presentation.headline }}</h2>
                <p class="recommendation-explanation">
                  {{ presentation.explanation }}
                </p>
                <div
                  v-if="presentation.webSearch?.results.length"
                  class="external-evidence"
                >
                  <h3>
                    联网来源 · {{ presentation.webSearch.results.length }}
                  </h3>
                  <a
                    v-for="source in presentation.webSearch.results.slice(0, 5)"
                    :key="source.url"
                    :href="safeExternalUrl(source.url)"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {{ source.title }} <Icon :icon="icons.external" />
                  </a>
                  <small>库外线索尚未入库，授权与报价待核实。</small>
                </div>
                <div v-if="evidenceCandidates.length" class="judgment-sources">
                  <button
                    v-for="item in evidenceCandidates"
                    :key="item.resourceId"
                    type="button"
                    @click="openResourceDrawer(item.resourceId)"
                  >
                    <Icon :icon="icons.file" />{{ item.resourceName }} ·
                    库内资料
                  </button>
                </div>
                <div v-if="presentation.basis?.length" class="judgment-basis">
                  <h3>判断依据</h3>
                  <dl>
                    <div v-for="row in presentation.basis" :key="row.label">
                      <dt>{{ row.label }}</dt>
                      <dd>{{ row.value }}</dd>
                    </div>
                  </dl>
                </div>
                <div
                  v-if="presentation.questions?.length"
                  class="clarification-questions"
                >
                  <h3>还需要确认</h3>
                  <p v-for="question in presentation.questions" :key="question">
                    {{ question }}
                  </p>
                </div>
                <div
                  v-if="presentation.adjustments?.length"
                  class="round-adjustments"
                >
                  <strong>本轮调整</strong>
                  <div>
                    <p>{{ presentation.adjustments.join(" · ") }}</p>
                    <small>具体授权范围、报价与档期仍需版权方确认。</small>
                  </div>
                </div>
                <div
                  v-if="
                    evidenceCandidates.some(
                      item => evidenceItems(item.evidence).length
                    )
                  "
                  class="external-evidence"
                >
                  <h3>公开背景资料</h3>
                  <template
                    v-for="item in evidenceCandidates"
                    :key="item.resourceId"
                    ><a
                      v-for="(source, index) in evidenceItems(item.evidence)"
                      :key="`${item.id}-${index}`"
                      :href="safeExternalUrl(source.url)"
                      target="_blank"
                      rel="noopener noreferrer"
                      >{{ source.title || source.source }}
                      <Icon :icon="icons.external" /></a></template
                  ><small>用于核对一般背景，授权与报价以版权方确认为准。</small>
                </div>
              </div>
            </article>
            <div
              v-if="conversationBusy"
              class="conversation-progress"
              role="status"
            >
              <Icon :icon="icons.spark" /><span>{{ streamingStatus }}</span>
              <button
                type="button"
                class="quiet-button"
                @click="stopConversation"
              >
                停止生成
              </button>
            </div>
            <div
              v-if="presentation?.suggestedPrompts?.length"
              class="follow-up-actions"
            >
              <button
                v-for="text in presentation.suggestedPrompts"
                :key="text"
                type="button"
                :disabled="conversationBusy"
                @click="followUp(text)"
              >
                {{ text }}
              </button>
            </div>
            <div
              v-if="conversationError"
              class="conversation-error"
              role="alert"
            >
              <p>{{ conversationError }}</p>
              <button type="button" @click="retryConversation">重试</button>
            </div>
          </div>
          <div class="conversation-dock">
            <p class="composer-note">
              {{
                webSearchMode
                  ? "联网模式：根据公开 IP 方向、市场与用途改写检索词，预算和附件不用于搜索。"
                  : "支持 PDF、文本与表格资料；AI 建议需结合真实版权资料评估。"
              }}
            </p>
            <div class="conversation-composer">
              <textarea
                v-model="composerText"
                :disabled="conversationBusy || attachmentReading"
                :maxlength="webSearchMode ? 200 : 1200"
                rows="2"
                aria-label="继续描述或调整项目需求"
                :placeholder="
                  webSearchMode
                    ? '输入公开搜索关键词，例如：东南亚艺术 IP 联名…'
                    : '继续追问，调整偏好，或补充项目资料…'
                "
                @keydown="composerKeydown"
              />
              <div v-if="attachment" class="attached-document">
                <Icon :icon="icons.file" /><span>{{ attachment.name }}</span
                ><button
                  type="button"
                  aria-label="移除资料"
                  @click="attachment = null"
                >
                  <Icon :icon="icons.close" />
                </button>
              </div>
              <div class="composer-actions">
                <button
                  type="button"
                  :disabled="
                    conversationBusy || attachmentReading || webSearchMode
                  "
                  @click="attachmentInput?.click()"
                >
                  <Icon :icon="icons.attach" />{{
                    attachmentReading ? "正在读取资料…" : "添加资料"
                  }}
                </button>
                <button
                  type="button"
                  :aria-pressed="webSearchMode"
                  aria-label="联网搜索"
                  :title="webSearchMode ? '关闭联网搜索' : '开启联网搜索'"
                  :disabled="conversationBusy || attachmentReading"
                  class="web-search-toggle"
                  @click="webSearchMode = !webSearchMode"
                >
                  <Icon :icon="icons.external" /><span>联网搜索</span>
                  <svg
                    class="web-search-outline"
                    aria-hidden="true"
                    focusable="false"
                  >
                    <rect x="0.5" y="0.5" rx="6" pathLength="100" />
                  </svg>
                </button>
                <span>Ctrl / ⌘ + Enter 发送</span
                ><button
                  type="button"
                  class="send-message"
                  :disabled="!canSendMessage"
                  :aria-label="
                    webSearchMode ? '搜索库外资源' : '发送需求或调整偏好'
                  "
                  @click="sendMessage"
                >
                  <Icon :icon="icons.arrow" />
                </button>
              </div>
              <input
                ref="attachmentInput"
                type="file"
                class="attachment-input"
                accept=".pdf,.txt,.md,.csv,.xlsx,.xls"
                @change="readAttachment"
              />
            </div>
          </div>
        </section>
        <section
          class="cooperation-workbench"
          aria-label="候选对比与意向工作区"
        >
          <div class="workbench-heading">
            <h2>合作工作区</h2>
            <button
              v-if="selectedSubscription"
              type="button"
              class="quiet-button"
              :disabled="conversationBusy"
              aria-label="重新推荐"
              @click="converse('recommend', '按当前条件重新推荐')"
            >
              <Icon :icon="icons.refresh" />
            </button>
          </div>
          <div class="workbench-toolbar">
            <div
              class="workbench-tabs"
              role="tablist"
              aria-label="合作工作区视图"
            >
              <button
                id="candidate-tab"
                type="button"
                role="tab"
                :aria-selected="workbenchTab === 'candidates'"
                aria-controls="candidate-workspace"
                :class="{ active: workbenchTab === 'candidates' }"
                @click="workbenchTab = 'candidates'"
              >
                候选 <b>{{ visibleResults.length }}</b>
              </button>
              <button
                id="compare-tab"
                type="button"
                role="tab"
                :aria-selected="workbenchTab === 'compare'"
                aria-controls="compare-workspace"
                :class="{ active: workbenchTab === 'compare' }"
                @click="workbenchTab = 'compare'"
              >
                对比 <b>{{ comparison.length }}</b>
              </button>
              <button
                id="intent-tab"
                type="button"
                role="tab"
                :aria-selected="workbenchTab === 'intent'"
                aria-controls="intent-workspace"
                :class="{ active: workbenchTab === 'intent' }"
                @click="workbenchTab = 'intent'"
              >
                意向 <b>{{ chosenIds.length }}</b>
              </button>
              <button
                id="external-tab"
                type="button"
                role="tab"
                :aria-selected="workbenchTab === 'external'"
                aria-controls="external-workspace"
                :class="{ active: workbenchTab === 'external' }"
                @click="workbenchTab = 'external'"
              >
                库外 <b>{{ latestWebSearch?.candidates?.length || 0 }}</b>
              </button>
            </div>
            <button
              v-if="workbenchTab === 'compare' && comparison.length >= 2"
              type="button"
              class="compare-research"
              :disabled="conversationBusy"
              @click="
                converse(
                  'compare',
                  `比较${comparison.map(item => item.resourceName).join('和')}，哪个更适合这次需求？`
                )
              "
            >
              AI 对比判断 <Icon :icon="icons.arrow" />
            </button>
          </div>
          <div
            v-if="workbenchTab === 'external'"
            id="external-workspace"
            class="external-workspace"
            role="tabpanel"
            aria-labelledby="external-tab"
          >
            <ExternalResearchWorkbench
              :research="latestWebSearch"
              :busy="conversationBusy"
              @ask="text => converse('search', text)"
              @compare-change="externalCompareCount = $event"
            />
          </div>
          <div v-else-if="!visibleResults.length" class="workbench-empty">
            <Icon :icon="icons.library" />
            <h3>
              {{
                selectedSubscription
                  ? "确认条件，开始寻找候选"
                  : "候选将在这里呈现"
              }}
            </h3>
            <p>
              对话用于理解与调整需求，工作区用于查看资料、比较候选和形成意向。
            </p>
            <button
              v-if="selectedSubscription"
              type="button"
              class="workspace-primary"
              :disabled="conversationBusy"
              @click="converse('recommend', '确认当前需求并开始推荐')"
            >
              确认并开始推荐 <Icon :icon="icons.arrow" />
            </button>
          </div>
          <div
            v-else-if="workbenchTab === 'candidates'"
            id="candidate-workspace"
            role="tabpanel"
            aria-labelledby="candidate-tab"
            class="candidate-workspace"
          >
            <p class="workspace-hint">
              选择候选查看资料，可同时对比 2 至 3 个 IP。
            </p>
            <article
              v-for="item in visibleResults"
              :id="`ip-recommendation-${item.id}`"
              :key="item.id"
              class="candidate-row"
              :class="{
                selected: currentCandidate?.resourceId === item.resourceId
              }"
            >
              <button
                type="button"
                class="candidate-select"
                :aria-label="`查看${item.resourceName}推荐依据`"
                @click="activeCandidateId = item.resourceId"
              >
                <img
                  v-if="imageUrls[Number(item.resource?.visualFileId)]"
                  :src="imageUrls[Number(item.resource.visualFileId)]"
                  :alt="item.resourceName"
                /><span v-else class="visual-empty"
                  ><Icon :icon="icons.image" /><small>暂无视觉资料</small></span
                ><span class="candidate-copy"
                  ><strong>{{ item.resourceName }}</strong
                  ><small>{{ item.resource?.ipType || "类型待完善" }}</small
                  ><span>{{ item.assessment?.fit || item.reason }}</span></span
                >
              </button>
              <div class="candidate-row-actions">
                <label
                  ><input
                    type="checkbox"
                    :checked="compareIds.includes(item.resourceId)"
                    @change="toggleCompare(item)"
                  />加入对比</label
                ><button
                  type="button"
                  @click="openResourceDrawer(item.resourceId)"
                >
                  资料 <Icon :icon="icons.arrow" /></button
                ><button
                  type="button"
                  :disabled="conversationBusy"
                  @click="feedback(item, 'not_interested')"
                >
                  不感兴趣</button
                ><button type="button" @click="queueIntent(item)">
                  {{
                    chosenIds.includes(item.resourceId)
                      ? "已加入意向"
                      : "加入意向"
                  }}
                </button>
              </div>
            </article>
            <div v-if="currentCandidate" class="candidate-details">
              <h3>{{ currentCandidate.resourceName }} · 合作方向</h3>
              <p>
                {{
                  currentCandidate.assessment?.verdict ||
                  currentCandidate.advice ||
                  currentCandidate.reason
                }}
              </p>
              <dl>
                <div>
                  <dt>合作切入</dt>
                  <dd>{{ comparisonValue(currentCandidate, "entry") }}</dd>
                </div>
                <div>
                  <dt>待确认</dt>
                  <dd>{{ comparisonValue(currentCandidate, "pending") }}</dd>
                </div>
                <div>
                  <dt>权益参考价</dt>
                  <dd>{{ priceText(currentCandidate.resource || {}) }}</dd>
                </div>
              </dl>
            </div>
            <button
              type="button"
              class="workspace-primary"
              :disabled="comparison.length < 2 || conversationBusy"
              @click="
                converse(
                  'compare',
                  `比较${comparison.map(item => item.resourceName).join('和')}，哪个更适合这次需求？`
                )
              "
            >
              对比已选 {{ comparison.length }} 个 <Icon :icon="icons.arrow" />
            </button>
          </div>
          <div
            v-else-if="workbenchTab === 'compare'"
            id="compare-workspace"
            role="tabpanel"
            aria-labelledby="compare-tab"
            class="compare-workspace"
          >
            <template v-if="comparison.length >= 2">
              <div
                class="comparison-media"
                :style="{ '--compare-count': comparison.length }"
              >
                <div v-for="item in comparison" :key="item.resourceId">
                  <button
                    type="button"
                    @click="openResourceDrawer(item.resourceId)"
                  >
                    <img
                      v-if="imageUrls[Number(item.resource?.visualFileId)]"
                      :src="imageUrls[Number(item.resource.visualFileId)]"
                      :alt="item.resourceName"
                    /><span v-else class="visual-empty"
                      ><Icon :icon="icons.image" /><small
                        >暂无视觉资料</small
                      ></span
                    ><strong>{{ item.resourceName }}</strong
                    ><small>{{ item.resource?.ipType || "类型待完善" }}</small>
                  </button>
                </div>
              </div>
              <div class="comparison-table-wrap">
                <table class="comparison-table">
                  <caption class="sr-only">
                    按当前需求比较候选 IP
                  </caption>
                  <thead class="sr-only">
                    <tr>
                      <th>对比维度</th>
                      <th v-for="item in comparison" :key="item.resourceId">
                        {{ item.resourceName }}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="field in comparisonFields" :key="field.key">
                      <th scope="row">{{ field.label }}</th>
                      <td v-for="item in comparison" :key="item.resourceId">
                        {{ comparisonValue(item, field.key) }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </template>
            <div v-else class="comparison-empty">
              <p>请先选择至少 2 个候选进行对比。</p>
              <button type="button" @click="workbenchTab = 'candidates'">
                选择候选 <Icon :icon="icons.arrow" />
              </button>
            </div>
            <div
              v-if="
                visibleResults.some(
                  item => !compareIds.includes(item.resourceId)
                )
              "
              class="other-candidates"
            >
              <h3>其他候选</h3>
              <button
                v-for="item in visibleResults
                  .filter(item => !compareIds.includes(item.resourceId))
                  .slice(0, 2)"
                :key="item.resourceId"
                type="button"
                @click="
                  activeCandidateId = item.resourceId;
                  workbenchTab = 'candidates';
                "
              >
                <img
                  v-if="imageUrls[Number(item.resource?.visualFileId)]"
                  :src="imageUrls[Number(item.resource.visualFileId)]"
                  :alt="item.resourceName"
                /><span
                  ><strong>{{ item.resourceName }}</strong
                  ><small>{{
                    item.assessment?.fit || item.reason
                  }}</small></span
                ><Icon :icon="icons.arrow" />
              </button>
            </div>
          </div>
          <div
            v-else
            id="intent-workspace"
            role="tabpanel"
            aria-labelledby="intent-tab"
            class="intent-workspace"
          >
            <p v-if="!chosenIds.length">还未加入初步意向，请从候选中选择。</p>
            <div
              v-for="item in intentItems"
              :key="item.resourceId"
              class="intent-row"
            >
              <Icon :icon="icons.check" /><img
                v-if="imageUrls[Number(item.resource?.visualFileId)]"
                :src="imageUrls[Number(item.resource.visualFileId)]"
                :alt="item.resourceName"
              />
              <div>
                <strong>{{ item.resourceName }}</strong
                ><small>{{ item.assessment?.fit || item.reason }}</small>
              </div>
              <button type="button" @click="removeIntent(item.resourceId)">
                移除
              </button>
            </div>
            <p
              v-if="chosenIds.length > intentItems.length"
              class="workspace-hint"
            >
              包含
              {{ chosenIds.length - intentItems.length }} 个当前推荐之外的已选
              IP，返回提需可查看完整清单。
            </p>
          </div>
          <div v-if="workbenchTab === 'external'" class="workbench-footer">
            <p class="workspace-hint">
              库外候选可在此对比；核实并入库后，才能加入正式意向或回填提需。
            </p>
            <router-link to="/business/ip/resources"
              >前往 IP 资源库 <Icon :icon="icons.arrow"
            /></router-link>
          </div>
          <div v-else-if="visibleResults.length" class="workbench-footer">
            <div v-if="workbenchTab !== 'intent'" class="intent-preview">
              <h3>初步意向 · {{ chosenIds.length }}</h3>
              <div v-if="intentItems.length" class="intent-row">
                <Icon :icon="icons.check" /><img
                  v-if="
                    imageUrls[Number(intentItems[0].resource?.visualFileId)]
                  "
                  :src="imageUrls[Number(intentItems[0].resource.visualFileId)]"
                  :alt="intentItems[0].resourceName"
                />
                <div>
                  <strong>{{ intentItems[0].resourceName }}</strong
                  ><small>{{
                    intentItems[0].assessment?.fit || intentItems[0].reason
                  }}</small>
                </div>
                <button type="button" @click="workbenchTab = 'intent'">
                  查看
                </button>
              </div>
              <p v-else>选择候选，形成待评估的合作方向。</p>
            </div>
            <div class="handoff-actions">
              <div>
                <p class="handoff-note">下一步由 IP 组核对授权与合作条件</p>
                <router-link to="/business/ip/requests">返回提需</router-link>
              </div>
              <button
                type="button"
                class="workspace-primary"
                @click="confirmIntent"
              >
                {{ chosenIds.length ? "确认初步意向" : "加入初步意向" }}
                <Icon :icon="icons.arrow" />
              </button>
            </div>
          </div>
        </section>
      </div>
    </div>
    <el-dialog
      v-model="editing"
      class="ip-workspace-dialog"
      title="需求与推荐条件"
      width="680px"
      :close-on-click-modal="false"
      ><div class="condition-editor">
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
          <label class="excluded-types"
            >排除的IP类型<el-select
              v-model="form.excludedIPTypes"
              multiple
              clearable
              placeholder="选择明确不考虑的类型"
              ><el-option
                v-for="kind in ipTypes"
                :key="kind"
                :label="kind"
                :value="kind" /></el-select
          ></label>
          <div class="editor-footer">
            <span
              >保存后立即生成推荐；开启每周检查时，每周一 09:00
              检查新匹配。</span
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
      </div>
      <div v-if="selectedSubscription" class="conversation-management">
        <span>{{
          selectedSubscription.active
            ? "每周检查库内新匹配已开启"
            : "每周检查库内新匹配已暂停"
        }}</span
        ><el-button link @click="toggleSubscription(selectedSubscription)">{{
          selectedSubscription.active ? "暂停" : "开启"
        }}</el-button
        ><el-button
          link
          type="danger"
          @click="
            removeSubscription(selectedSubscription);
            editing = false;
          "
          >删除会话</el-button
        >
      </div></el-dialog
    >
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
      class="ip-workspace-dialog"
      :title="previewFileName"
      width="80%"
      top="5vh"
      @close="closePreview"
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
      <div v-else-if="previewError" class="pdf-error">
        <p>{{ previewError }}</p>
        <el-button v-if="previewFile" @click="openDocument(previewFile)">
          重试
        </el-button>
      </div>
      <div v-else-if="previewDownloading" class="pdf-transfer">
        <p>{{ previewDownloadText }}</p>
        <el-progress
          v-if="previewTotal > 0"
          :percentage="previewDownloadPercent"
          :stroke-width="8"
          :show-text="false"
          color="#9ac000"
        />
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
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
  line-height: 1.6;
  color: #777f81;
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
  gap: 8px;
  align-items: center;
  justify-content: space-between;
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
  padding: 12px;
  overflow: hidden auto;
  overscroll-behavior: contain;
  background: #f3f4f0;
}

.pdf-loading {
  margin: 0 0 12px;
  color: #737373;
}

.pdf-transfer {
  min-height: 100px;
  padding: 24px 16px;
}

.pdf-transfer p {
  margin: 0 0 14px;
  color: #535a47;
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

.ip-recommend-page {
  --workspace-ink: #18201c;
  --workspace-muted: #687068;
  --workspace-line: #e3e6df;
  --workspace-accent: #caff00;

  position: relative;
  min-width: 0;
  min-height: calc(100vh - 150px);
  padding: 16px 28px 36px;
  container: ip-recommendation / inline-size;
  font-size: 14px;
  line-height: 1.6;
  color: var(--workspace-ink);
  background: #fafaf7;
}

.ip-recommend-page * {
  box-sizing: border-box;
}

.ip-recommend-page :where(button, textarea) {
  font: inherit;
}

.ip-recommend-page button {
  color: inherit;
  cursor: pointer;
}

.ip-recommend-page button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.ip-recommend-page a {
  color: #345d75;
  text-decoration: none;
  text-underline-offset: 4px;
}

.ip-recommend-page a:hover {
  text-decoration: underline;
}

.ip-recommend-page button:focus-visible,
.ip-recommend-page a:focus-visible {
  outline: 2px solid #779500;
  outline-offset: 4px;
}

.ip-recommend-page ::selection {
  color: #18201c;
  background: #e4f7a0;
}

.research-main {
  min-width: 0;
}

.research-session-bar {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
  padding-bottom: 12px;
}

.session-switch {
  display: flex;
  flex: 1;
  gap: 12px;
  align-items: center;
  min-width: 0;
  font-size: 13px;
  color: var(--workspace-muted);
}

.session-switch > span,
.session-switch :deep(.el-button) {
  flex: none;
}

.project-session-select {
  flex: 1;
  min-width: 120px;
  max-width: 280px;
}

.session-switch :deep(.el-button > span) {
  display: flex;
  gap: 8px;
  align-items: center;
}

.session-switch svg {
  width: 16px;
  height: 16px;
}

.project-title {
  flex: 1;
  min-width: 0;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.5;
  white-space: nowrap;
}

.research-tools {
  display: flex;
  flex: none;
  gap: 16px;
  align-items: center;
}

.stage-switch,
.conditions-toggle {
  font-size: 13px;
}

.stage-indicator {
  flex: none;
  width: 8px;
  height: 8px;
  background: var(--workspace-accent);
  border: 1px solid #526350;
  border-radius: 50%;
}

.quiet-button {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  justify-content: center;
  min-height: 32px;
  padding: 0 4px;
  color: #526350;
  background: none;
  border: 0;
}

.quiet-button svg {
  width: 18px;
  height: 18px;
}

.context-link {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  font-size: 12px;
}

.context-link svg {
  width: 18px;
  height: 18px;
}

.expanded-criteria {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 24px;
  max-height: 240px;
  padding: 12px 0;
  margin-bottom: 12px;
  overflow: auto;
  font-size: 13px;
  border-top: 1px solid var(--workspace-line);
  border-bottom: 1px solid var(--workspace-line);
}

.expanded-criteria p {
  flex: 0 0 100%;
  margin: 0;
  line-height: 1.8;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.expanded-criteria button:not(.quiet-button) {
  padding: 0;
  color: #526b30;
  background: none;
  border: 0;
}

.criteria-summary {
  display: flex;
  flex: 0 0 100%;
  gap: 12px;
  align-items: flex-start;
}

.criteria-summary > span {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}

.criteria-summary .quiet-button {
  flex: none;
}

.research-columns {
  display: grid;
  grid-template-rows: minmax(0, 1fr);
  grid-template-columns: minmax(0, 1.12fr) minmax(360px, 1fr);
  gap: 0;
  align-items: stretch;
  height: clamp(560px, calc(100dvh - 240px), 960px);
  overflow: hidden;
  background: #fff;
  border: 1px solid var(--workspace-line);
  border-radius: 12px;
}

.conversation-panel {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding: 24px 28px;
}

.conversation-body {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: auto;
  scrollbar-gutter: stable;
  overscroll-behavior-y: contain;
}

.conversation-dock {
  flex: none;
  padding-top: 18px;
  margin-top: auto;
}

.conversation-welcome {
  padding: 24px 12px 36px;
}

.conversation-welcome > svg {
  width: 32px;
  height: 32px;
  color: #62772a;
}

.conversation-welcome h2 {
  max-width: 20ch;
  margin: 20px 0 12px;
  font-size: 30px;
  line-height: 1.5;
  letter-spacing: -0.02em;
}

.conversation-welcome p {
  max-width: 52ch;
  margin: 0;
  line-height: 1.8;
  color: var(--workspace-muted);
}

.entry-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 28px;
}

.entry-actions button {
  display: flex;
  gap: 8px;
  align-items: center;
  min-height: 40px;
  padding: 10px 14px;
  background: #fff;
  border: 1px solid var(--workspace-line);
  border-radius: 8px;
}

.entry-actions svg {
  width: 18px;
  height: 18px;
}

.welcome-example {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 0;
  margin-top: 28px;
  color: #526b30 !important;
  text-align: left;
  background: none;
  border: 0;
}

.welcome-example svg {
  flex: none;
  width: 18px;
  height: 18px;
}

.history-toggle {
  display: block;
  padding: 0;
  margin: 0 0 16px;
  font-size: 12px !important;
  color: #687962 !important;
  background: none;
  border: 0;
}

.conversation-history {
  max-height: 320px;
  padding-right: 12px;
  margin-bottom: 24px;
  overflow-y: auto;
  scrollbar-width: thin;
}

.conversation-history > div {
  padding: 12px 16px;
  margin-bottom: 8px;
  background: #f0f2ed;
  border-radius: 8px;
}

.conversation-history > .user {
  margin-left: 32px;
}

.conversation-history small {
  font-size: 11px;
  color: var(--workspace-muted);
}

.conversation-history p {
  margin: 6px 0 0;
  font-size: 13px;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.user-message {
  width: fit-content;
  max-width: 88%;
  margin: 0 12px 20px auto;
}

.user-message small {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  text-align: right;
}

.user-message time {
  margin-left: 8px;
  color: var(--workspace-muted);
}

.user-message p {
  padding: 14px 18px;
  margin: 0;
  font-size: 13px;
  line-height: 1.8;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  background: #edf0ed;
  border-radius: 10px;
}

.assistant-identity {
  display: flex;
  gap: 12px;
  align-items: center;
  padding-left: 0;
  margin-bottom: 14px;
  font-size: 13px;
}

.assistant-identity > span {
  display: grid;
  flex: none;
  place-items: center;
  min-width: 34px;
  height: 34px;
  padding: 0 4px;
  font-size: 14px;
  font-weight: 700;
  color: white;
  background: #18201c;
  border: 1px solid #b5d54c;
  border-radius: 50%;
}

.assistant-identity time {
  font-size: 12px;
  color: var(--workspace-muted);
}

.assistant-content {
  padding: 0 12px 0 46px;
}

.assistant-content > h2 {
  margin: 0 0 8px;
  font-size: clamp(24px, 2.2cqw, 30px);
  font-weight: 650;
  line-height: 1.3;
  color: #101713;
  letter-spacing: -0.025em;
  text-wrap: pretty;
  overflow-wrap: anywhere;
}

.recommendation-explanation {
  max-width: 52ch;
  margin: 0;
  line-height: 1.8;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.judgment-sources {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin: 12px 0 10px;
}

.judgment-sources button {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  min-height: 30px;
  padding: 4px 8px;
  font-size: 12px;
  color: #345d75;
  background: transparent;
  border: 1px solid #e5e9e2;
  border-radius: 6px;
}

.judgment-sources svg {
  flex: none;
  width: 16px;
  height: 16px;
}

.judgment-basis,
.clarification-questions,
.external-evidence {
  padding-top: 10px;
  margin-top: 12px;
  border-top: 1px solid var(--workspace-line);
}

.assistant-content h3,
.cooperation-workbench h3 {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
}

.judgment-basis dl {
  margin: 0;
}

.judgment-basis dl > div {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  border-bottom: 1px solid var(--workspace-line);
}

.judgment-basis dt {
  padding: 8px 12px;
  font-size: 13px;
  color: #334431;
  background: #f1f3ef;
}

.judgment-basis dd {
  padding: 8px 12px;
  margin: 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}

.round-adjustments {
  display: flex;
  gap: 20px;
  padding: 12px 0;
  margin-top: 6px;
  font-size: 12px;
}

.round-adjustments strong {
  flex: none;
  font-weight: 500;
}

.round-adjustments p {
  margin: 0 0 4px;
  overflow-wrap: anywhere;
}

.round-adjustments small {
  font-size: 12px;
  color: var(--workspace-muted);
}

.clarification-questions p {
  margin: 8px 0;
  line-height: 1.8;
}

.external-evidence {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
}

.external-evidence svg {
  width: 14px;
  height: 14px;
  vertical-align: middle;
}

.external-evidence small {
  color: var(--workspace-muted);
}

.follow-up-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 0 12px 0 46px;
  margin-top: 12px;
}

.follow-up-actions button {
  min-height: 34px;
  padding: 5px 12px;
  font-size: 12px;
  background: transparent;
  border: 1px solid #dce1d7;
  border-radius: 8px;
}

.follow-up-actions button:hover {
  background: #eef3e8;
}

.conversation-progress {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 20px 12px;
  color: #55653a;
}

.conversation-progress svg {
  width: 20px;
  height: 20px;
}

.conversation-progress > span {
  flex: 1;
  min-width: 0;
}

.conversation-progress > button {
  flex: none;
}

.conversation-error {
  display: flex;
  gap: 16px;
  align-items: center;
  padding: 12px;
  font-size: 13px;
  color: #8c3d2c;
  background: #fff2ec;
  border-radius: 8px;
}

.conversation-error p {
  flex: 1;
  margin: 0;
}

.conversation-error button {
  padding: 4px 10px;
  background: white;
  border: 1px solid #dfb8a7;
  border-radius: 4px;
}

.conversation-composer {
  padding: 16px 16px 12px;
  background: white;
  border: 1px solid var(--workspace-line);
  border-radius: 12px;
}

.conversation-composer:focus-within {
  outline: 1px solid #879e62;
}

.conversation-composer textarea {
  display: block;
  width: 100%;
  min-height: 50px;
  max-height: 200px;
  padding: 0;
  font-size: 13px;
  line-height: 1.7;
  color: #18201c;
  caret-color: #688800;
  resize: vertical;
  outline: none;
  background: transparent;
  border: 0;
}

.conversation-composer textarea::placeholder {
  color: #747c72;
}

.composer-actions {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 10px;
}

.composer-actions button {
  display: flex;
  gap: 8px;
  align-items: center;
  min-height: 32px;
  padding: 0;
  font-size: 12px;
  background: none;
  border: 0;
}

.composer-actions svg {
  width: 18px;
  height: 18px;
}

.composer-actions > span {
  margin-left: auto;
  font-size: 10px;
  color: #767e72;
}

.composer-actions .web-search-toggle {
  position: relative;
  flex: none;
  white-space: nowrap;
}

.composer-actions .web-search-outline {
  position: absolute;
  inset: 0 -6px;
  width: calc(100% + 12px);
  height: 100%;
  overflow: visible;
  pointer-events: none;
}

.web-search-outline rect {
  width: calc(100% - 1px);
  height: calc(100% - 1px);
  opacity: 0;
  fill: none;
  stroke: #79913c;
  stroke-width: 1;
  stroke-dasharray: 100;
  stroke-dashoffset: 100;
  transition:
    stroke-dashoffset 160ms ease-out,
    opacity 120ms ease-out;
}

.web-search-toggle[aria-pressed="true"] .web-search-outline rect {
  opacity: 1;
  stroke-dashoffset: 0;
  transition:
    stroke-dashoffset 480ms cubic-bezier(0.16, 1, 0.3, 1),
    opacity 120ms ease-out;
}

@media (prefers-reduced-motion: reduce) {
  .web-search-toggle[aria-pressed] .web-search-outline rect {
    transition: none;
  }
}

@media (pointer: coarse) {
  .research-tools .quiet-button,
  .composer-actions .web-search-toggle {
    min-height: 44px;
  }
}

.external-workspace {
  padding-top: 20px;
}

.external-workspace > p {
  margin: 0 0 8px;
  line-height: 1.7;
}

.external-workspace small {
  display: block;
  font-size: 12px;
  color: var(--workspace-muted);
}

.external-resource {
  padding: 20px 0;
  border-bottom: 1px solid var(--workspace-line);
}

.external-resource a {
  display: flex;
  gap: 8px;
  align-items: baseline;
  margin-bottom: 8px;
  font-size: 15px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.external-resource a svg {
  flex: none;
  width: 15px;
  height: 15px;
}

.external-resource p {
  margin: 10px 0 0;
  font-size: 13px;
  line-height: 1.8;
  overflow-wrap: anywhere;
}

.composer-actions .send-message {
  display: grid;
  flex: none;
  place-items: center;
  width: 42px;
  height: 40px;
  margin-left: auto;
  color: var(--workspace-accent);
  background: #18201c;
  border-radius: 8px;
}

.send-message svg {
  width: 24px;
  height: 24px;
}

.composer-note {
  padding: 0 8px;
  margin: 0 0 10px;
  font-size: 11px;
  color: #737c70;
}

.attachment-input {
  display: none;
}

.attached-document {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 8px 10px;
  margin: 8px 0 0;
  font-size: 12px;
  background: #f1f4ee;
  border-radius: 6px;
}

.attached-document > span {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}

.attached-document svg {
  width: 17px;
  height: 17px;
}

.attached-document button {
  display: grid;
  flex: none;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: none;
  border: 0;
}

.cooperation-workbench {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding: 24px 28px;
  border-left: 1px solid var(--workspace-line);
}

.workbench-heading {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.workbench-heading h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 650;
  line-height: 1.4;
  letter-spacing: -0.02em;
}

.workbench-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.workbench-tabs {
  display: flex;
  gap: 24px;
  margin: 8px 0 14px;
}

.workbench-tabs button {
  min-height: 36px;
  padding: 0 4px 8px;
  font-size: 13px;
  color: #657063;
  background: none;
  border: 0;
  border-bottom: 2px solid transparent;
}

.workbench-tabs b {
  margin-left: 8px;
  font-weight: 400;
}

.workbench-tabs .active {
  font-weight: 600;
  color: #17221a;
  border-bottom-color: #18201c;
}

.workbench-tabs .active b {
  font-weight: 600;
}

.workbench-empty {
  padding: 42px 0;
  text-align: center;
}

.workbench-empty > svg {
  width: 40px;
  height: 40px;
  margin-bottom: 20px;
  color: #849177;
}

.workbench-empty h3 {
  font-size: 18px;
}

.workbench-empty p {
  max-width: 40ch;
  margin: 10px auto 28px;
  line-height: 1.8;
  color: #61715a;
}

.workspace-hint {
  margin: 0 0 16px;
  font-size: 12px;
  color: #64705e;
}

.candidate-row {
  padding: 16px 0;
  border-bottom: 1px solid #dbe1d3;
}

.candidate-row:first-of-type {
  padding-top: 0;
}

.candidate-select {
  display: flex;
  gap: 14px;
  align-items: center;
  width: 100%;
  padding: 0;
  text-align: left;
  background: none;
  border: 0;
}

.candidate-select img,
.candidate-select .visual-empty {
  flex: none;
  width: 88px;
  height: 88px;
  object-fit: cover;
  border-radius: 8px;
}

.candidate-copy {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.candidate-copy strong {
  font-size: 16px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.candidate-copy small {
  font-size: 12px;
  color: #64705e;
}

.candidate-copy > span {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.candidate-row.selected .candidate-copy strong {
  color: #4c681e;
}

.visual-empty {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
  justify-content: center;
  color: #76866b;
  background: #e6ebe0;
}

.visual-empty svg {
  width: 24px;
  height: 24px;
}

.visual-empty small {
  font-size: 10px;
}

.candidate-row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  align-items: center;
  justify-content: space-between;
  padding-top: 10px;
  font-size: 11px;
}

.candidate-row-actions label {
  display: inline-flex;
  gap: 5px;
  align-items: center;
  cursor: pointer;
}

.candidate-row-actions input {
  accent-color: #6b851c;
}

.candidate-row-actions button {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  min-height: 28px;
  padding: 0;
  color: #47633a;
  background: none;
  border: 0;
}

.candidate-row-actions svg {
  width: 14px;
  height: 14px;
}

.candidate-details {
  padding: 18px 0;
}

.candidate-details p {
  margin: 0 0 16px;
  font-size: 12px;
  line-height: 1.8;
}

.candidate-details dl {
  margin: 0;
  font-size: 12px;
}

.candidate-details dl > div {
  display: grid;
  grid-template-columns: 80px minmax(0, 1fr);
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid #dbe1d3;
}

.candidate-details dt {
  color: #64705e;
}

.candidate-details dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.workspace-primary {
  display: inline-flex;
  gap: 12px;
  align-items: center;
  justify-content: center;
  min-height: 44px;
  padding: 10px 20px;
  font-size: 13px;
  font-weight: 600;
  color: #172000 !important;
  background: var(--workspace-accent);
  border: 0;
  border-radius: 8px;
}

.workspace-primary svg {
  flex: none;
  width: 18px;
  height: 18px;
}

.workspace-primary:hover {
  background: #bdec00;
}

.comparison-media {
  display: grid;
  grid-template-columns: repeat(var(--compare-count, 2), minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.comparison-media > div {
  min-width: 0;
}

.comparison-media button {
  display: block;
  width: 100%;
  padding: 0;
  text-align: left;
  background: none;
  border: 0;
}

.comparison-media img,
.comparison-media .visual-empty {
  display: flex;
  width: 100%;
  height: clamp(130px, 11.5vw, 190px);
  margin-bottom: 8px;
  object-fit: cover;
  border-radius: 8px;
}

.comparison-media strong {
  display: block;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.comparison-media small {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: #6c7864;
}

.comparison-table-wrap {
  overflow-x: auto;
}

.comparison-table {
  width: 100%;
  font-size: 12px;
  line-height: 1.7;
  table-layout: fixed;
  border-collapse: collapse;
}

.comparison-table th {
  width: 80px;
  font-weight: 400;
  text-align: left;
}

.comparison-table td,
.comparison-table th {
  padding: 8px;
  vertical-align: top;
  overflow-wrap: anywhere;
  border-top: 1px solid #dbe1d3;
}

.comparison-table td {
  border-left: 1px solid #e1e6da;
}

.compare-research {
  display: inline-flex;
  flex: none;
  gap: 8px;
  align-items: center;
  min-height: 32px;
  padding: 0;
  margin: 0 0 6px;
  font-size: 12px !important;
  color: #466135 !important;
  background: none;
  border: 0;
}

.compare-research svg {
  width: 16px;
  height: 16px;
}

.comparison-empty {
  padding: 24px 0;
  font-size: 13px;
}

.comparison-empty button {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  padding: 0;
  color: #466135;
  background: none;
  border: 0;
}

.comparison-empty svg {
  width: 16px;
  height: 16px;
}

.other-candidates {
  padding-top: 12px;
  margin-top: 12px;
  border-top: 1px solid #dbe1d3;
}

.other-candidates > button {
  display: flex;
  gap: 14px;
  align-items: center;
  width: 100%;
  padding: 4px 0;
  text-align: left;
  background: none;
  border: 0;
}

.other-candidates img {
  flex: none;
  width: 68px;
  height: 56px;
  object-fit: cover;
  border-radius: 4px;
}

.other-candidates > button > span {
  flex: 1;
  min-width: 0;
}

.other-candidates strong {
  display: block;
  font-size: 13px;
}

.other-candidates small {
  display: -webkit-box;
  margin-top: 4px;
  overflow: hidden;
  -webkit-line-clamp: 2;
  font-size: 11px;
  color: #687660;
  -webkit-box-orient: vertical;
}

.other-candidates > button > svg {
  flex: none;
  width: 18px;
  height: 18px;
}

.workbench-footer {
  flex: none;
  padding-top: 12px;
  margin-top: auto;
  border-top: 1px solid #dbe1d3;
}

.candidate-workspace,
.compare-workspace,
.intent-workspace {
  padding-bottom: 12px;
}

.workbench-heading,
.workbench-toolbar {
  flex: none;
}

.external-workspace,
.workbench-empty,
.candidate-workspace,
.compare-workspace,
.intent-workspace {
  flex: 1;
  min-height: 0;
  overflow: auto;
  scrollbar-gutter: stable;
  overscroll-behavior-y: contain;
}

.intent-preview p {
  margin: 0;
  font-size: 12px;
  color: #687660;
}

.intent-row {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 4px 0;
}

.intent-row > svg {
  flex: none;
  width: 22px;
  height: 22px;
  padding: 2px;
  color: #172000;
  background: var(--workspace-accent);
  border-radius: 4px;
}

.intent-row img {
  flex: none;
  width: 66px;
  height: 48px;
  object-fit: cover;
  border-radius: 4px;
}

.intent-row > div {
  flex: 1;
  min-width: 0;
}

.intent-row strong {
  display: block;
  font-size: 13px;
  overflow-wrap: anywhere;
}

.intent-row small {
  display: -webkit-box;
  margin-top: 4px;
  overflow: hidden;
  -webkit-line-clamp: 2;
  font-size: 11px;
  color: #687660;
  -webkit-box-orient: vertical;
}

.intent-row button {
  flex: none;
  padding: 0;
  font-size: 11px;
  color: #47633a;
  background: none;
  border: 0;
}

.handoff-note {
  margin: 0 0 8px;
  font-size: 12px;
  color: #344a30;
}

.handoff-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  margin-top: 16px;
}

.handoff-actions a {
  font-size: 12px;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  white-space: nowrap;
  border: 0;
  clip-path: inset(50%);
}

.condition-editor .editor-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 24px;
}

.condition-editor label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
}

.condition-editor .span-2 {
  grid-column: 1 / -1;
}

.condition-editor .budget {
  display: flex;
  gap: 8px;
}

.condition-editor .budget .el-select {
  width: 100px;
}

.condition-editor .budget .el-input-number {
  flex: 1;
  width: auto;
  min-width: 0;
}

.condition-editor .excluded-types {
  margin-top: 20px;
}

.condition-editor .editor-footer {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  padding-top: 24px;
  margin-top: 24px;
  border-top: 1px solid #e3e6df;
}

.condition-editor .editor-footer > span {
  max-width: 340px;
  font-size: 12px;
  color: #687068;
}

.condition-editor .primary-button {
  color: #172000;
  background: #caff00;
  border-color: #caff00;
}

.conversation-management {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: center;
  padding-top: 20px;
  font-size: 12px;
  color: #687068;
}

:global(.ip-workspace-dialog) {
  max-width: calc(100vw - 32px);
}

@container ip-recommendation (width <= 1050px) {
  .research-columns {
    grid-template-columns: minmax(0, 1fr) minmax(320px, 0.95fr);
  }

  .conversation-panel,
  .cooperation-workbench {
    padding: 20px;
  }

  .assistant-content,
  .follow-up-actions {
    padding-left: 12px;
  }

  .composer-actions > span {
    display: none;
  }
}

@container ip-recommendation (width <= 780px) {
  .research-columns {
    grid-template-rows: repeat(2, clamp(560px, 75dvh, 760px));
    grid-template-columns: 1fr;
    height: auto;
  }

  .cooperation-workbench {
    border-top: 1px solid var(--workspace-line);
    border-left: 0;
  }
}

@container ip-recommendation (width <= 540px) {
  .research-session-bar {
    flex-wrap: wrap;
    gap: 8px;
  }

  .session-switch {
    flex-basis: 100%;
    gap: 8px;
  }

  .session-switch > span {
    display: none;
  }

  .project-session-select {
    max-width: none;
  }

  .research-tools {
    gap: 12px;
  }

  .research-tools .quiet-button {
    min-height: 44px;
  }

  .conversation-welcome {
    padding: 12px 0 24px;
  }

  .conversation-welcome h2 {
    font-size: 24px;
  }

  .assistant-content {
    padding: 0;
  }

  .assistant-content > h2 {
    font-size: 24px;
  }

  .recommendation-explanation {
    font-size: 13px;
  }

  .user-message {
    max-width: 96%;
    margin-right: 0;
  }

  .user-message p {
    padding: 12px;
  }

  .judgment-basis dl > div {
    grid-template-columns: 76px minmax(0, 1fr);
  }

  .judgment-basis dt,
  .judgment-basis dd {
    padding: 8px;
    font-size: 12px;
  }

  .round-adjustments {
    gap: 12px;
  }

  .follow-up-actions {
    padding: 0;
  }

  .conversation-panel,
  .cooperation-workbench {
    padding: 20px 16px;
  }

  .workbench-heading h2 {
    font-size: 22px;
  }

  .workbench-toolbar {
    flex-wrap: wrap;
    gap: 0;
  }

  .workbench-tabs {
    gap: 16px;
  }

  .comparison-media {
    gap: 12px;
  }

  .comparison-media img,
  .comparison-media .visual-empty {
    height: 116px;
  }

  .comparison-media strong {
    font-size: 13px;
  }

  .comparison-media small {
    font-size: 10px;
  }

  .comparison-table th {
    width: 60px;
  }

  .comparison-table th,
  .comparison-table td {
    padding: 8px 5px;
    font-size: 11px;
  }

  .candidate-select {
    gap: 10px;
  }

  .candidate-select img,
  .candidate-select .visual-empty {
    width: 68px;
    height: 68px;
  }

  .candidate-copy strong {
    font-size: 14px;
  }

  .condition-editor .editor-grid {
    grid-template-columns: 1fr;
  }

  .condition-editor .editor-footer > span {
    max-width: none;
  }
}

@media (width <= 600px) {
  .ip-recommend-page {
    padding: 16px;
  }
}
</style>
