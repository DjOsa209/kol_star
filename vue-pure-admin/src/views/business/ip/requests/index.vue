<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import dayjs from "dayjs";
import * as XLSX from "xlsx";
import RequestText from "./components/RequestText.vue";
import DeadlineStatus from "./components/DeadlineStatus.vue";
import {
  getIPResource,
  getIPRequest,
  listIPRequests,
  listIPResources,
  readIPFile,
  searchIPColleagues,
  saveIPRequest,
  deleteIPRequest,
  submitIPFeedback,
  submitIPMarketing,
  type IPColleague,
  type IPRequestInput
} from "@/api/ip";

defineOptions({ name: "IPRequests" });
const route = useRoute();
const router = useRouter();
const mode = ref<"list" | "new" | "detail">("list");
const loading = ref(false);
const saving = ref(false);
const rows = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const keyword = ref("");
const detail = ref<{ request: any; candidates: any[] } | null>(null);
const deletingID = ref<number | null>(null);
const editingSubmitted = computed(
  () => !!form.value.id && detail.value?.request.status !== "draft"
);
const candidateSelectionLocked = computed(
  () => !!form.value.id && !!detail.value?.request.candidatesLocked
);
const resourceOptions = ref<any[]>([]);
const ipImageUrls = ref<Record<number, string>>({});
const librarySelectionKey = ref(`ip-request-${crypto.randomUUID()}`);
const libraryHref = computed(
  () =>
    router.resolve({
      path: "/business/ip/resources",
      query:
        mode.value === "new"
          ? {
              selectForRequest: "1",
              selectionKey: librarySelectionKey.value,
              selected: form.value.candidateIds.join(",")
            }
          : {}
    }).href
);
const recommendationHref = computed(
  () =>
    router.resolve({
      path: "/business/ip/recommendations",
      query: {
        selectionKey: librarySelectionKey.value,
        selected: form.value.candidateIds.join(","),
        ...(form.value.id ? { requestId: String(form.value.id) } : {})
      }
    }).href
);
const markets = [
  "中国",
  "东南亚",
  "欧洲",
  "中东",
  "非洲",
  "拉美",
  "北美",
  "南亚",
  "全球"
];
const goals = ["新品发布", "品牌声量", "用户增长", "线下活动", "其他"];
const channels = ["线下快闪", "短视频共创", "新品联名", "社媒互动", "媒体投放"];
const briefFields = [
  { key: "owner", label: "项目负责人", hint: "填写姓名或团队", required: true },
  {
    key: "projectPeriod",
    label: "项目周期",
    hint: "启动至结案的预计周期",
    required: true
  },
  {
    key: "linkedProduct",
    label: "关联产品",
    hint: "例如 NOTE / GT / HOT",
    required: true
  },
  {
    key: "targetIPType",
    label: "目标IP类型",
    hint: "例如体育赛事、游戏或动漫",
    required: true
  },
  {
    key: "targetIPCount",
    label: "目标IP数量",
    hint: "预计需要评估的IP数量",
    required: true
  },
  {
    key: "cooperationMode",
    label: "合作模式",
    hint: "整合联动 / 衍生品授权 / 营销联动",
    required: true
  },
  {
    key: "projectNotes",
    label: "项目补充说明",
    hint: "其他协作约束（选填）",
    required: false
  }
];
const contactStages = [
  { key: "product", label: "产品" },
  { key: "ipMarketing", label: "IP营销" },
  { key: "imc", label: "IMC" },
  { key: "brand", label: "品牌运营" },
  { key: "user", label: "用户运营" },
  { key: "media", label: "媒介投放" },
  { key: "pr", label: "PR" }
];
const projectFields = briefFields.filter(field => field.key !== "projectNotes");
const requestBrief = computed(() => objectValue(detail.value?.request.brief));
const requestContacts = computed(() =>
  normalizedContactsByStage(requestBrief.value.contactsByStage)
);
const externalIPs = computed(() =>
  Array.isArray(requestBrief.value.externalIPs)
    ? requestBrief.value.externalIPs
    : []
);

function detailText(value: unknown, fallback = "待补充") {
  return value == null || String(value).trim() === ""
    ? fallback
    : String(value);
}

function requestDate(value: string, withTime = false) {
  if (!value) return "待确定";
  const date = dayjs(value);
  if (!withTime && date.isValid() && /^\d{4}-\d{2}-\d{2}/.test(value))
    return value.slice(0, 10);
  return date.isValid()
    ? date.format(withTime ? "YYYY-MM-DD HH:mm" : "YYYY-MM-DD")
    : value;
}
type Colleague = {
  id?: string | number;
  name: string;
  department?: string;
  employeeNo?: string;
};
function emptyContactsByStage(): Record<string, Colleague> {
  return Object.fromEntries(
    contactStages.map(stage => [stage.key, { name: "" }])
  );
}
function normalizedContactsByStage(value: unknown): Record<string, Colleague> {
  const saved = objectValue(value);
  return Object.fromEntries(
    contactStages.map(stage => {
      const item = saved[stage.key];
      return [
        stage.key,
        typeof item === "string"
          ? { name: item }
          : {
              name: String(item?.name || ""),
              ...(item?.id ? { id: item.id } : {}),
              ...(item?.department ? { department: item.department } : {}),
              ...(item?.employeeNo
                ? { employeeNo: String(item.employeeNo) }
                : {})
            }
      ];
    })
  );
}
const assessmentFields = [
  {
    key: "priceModel",
    label: "合作价格与模式",
    hint: "一次性授权、年度保底或保底加分成"
  },
  {
    key: "licenseContent",
    label: "授权内容",
    hint: "商标、形象、内容或线下活动"
  },
  {
    key: "licenseTerritory",
    label: "授权区域",
    hint: "全球、多区域或单点区域"
  },
  {
    key: "ipResources",
    label: "IP方可提供资源",
    hint: "素材、游戏活动、官方宣发、KOL等"
  },
  {
    key: "brandResources",
    label: "Infinix侧需提供资源",
    hint: "硬件、系统、营销、渠道等"
  },
  {
    key: "businessRisk",
    label: "商务风险评估",
    hint: "合规、预算、竞品冲突等"
  },
  { key: "publicRisk", label: "舆情与地缘风险", hint: "政治、舆情等问题" },
  {
    key: "ownershipClarity",
    label: "版权归属清晰度",
    hint: "主体、授权链及待核实问题"
  },
  {
    key: "pastCases",
    label: "相关合作案例",
    hint: "过往联名数据及玩法（选填）"
  },
  {
    key: "relatedDocuments",
    label: "相关文档与纪要",
    hint: "协议、排期、预算、对接纪要（选填）"
  },
  {
    key: "notes",
    label: "特殊约束与备注",
    hint: "排他、监修或时间限制（选填）"
  }
];
const requiredAssessmentFields = new Set([
  "licenseContent",
  "licenseTerritory",
  "ipResources",
  "brandResources",
  "businessRisk",
  "publicRisk",
  "ownershipClarity"
]);
const scoreFields = [
  { key: "audienceMatch", label: "受众匹配", weight: 0.3 },
  { key: "marketCoverage", label: "市场覆盖", weight: 0.25 },
  { key: "scheduleAvailability", label: "档期可用性", weight: 0.25 },
  { key: "licenseRisk", label: "授权风险控制", weight: 0.2 }
];
function weightedScore(assessment: Record<string, any>): number | null {
  const scores = objectValue(assessment.scoreDimensions);
  if (
    scoreFields.some(
      field =>
        scores[field.key] === null ||
        scores[field.key] === undefined ||
        scores[field.key] === "" ||
        !Number.isFinite(Number(scores[field.key])) ||
        Number(scores[field.key]) < 0 ||
        Number(scores[field.key]) > 100
    )
  )
    return null;
  return Math.round(
    scoreFields.reduce(
      (total, field) => total + Number(scores[field.key]) * field.weight,
      0
    )
  );
}
const marketingFields = [
  {
    key: "marketHeat",
    label: "IMC · IP热度",
    hint: "顶流 / 一线 / 二线 / 区域热门"
  },
  {
    key: "fanAudience",
    label: "IMC · 粉丝受众",
    hint: "核心粉丝画像与覆盖市场"
  },
  {
    key: "commercialValue",
    label: "IMC · 商业价值",
    hint: "预估商业转化与品牌收益"
  },
  {
    key: "marketingRisks",
    label: "IMC · 短板与风险",
    hint: "档期、舆情或执行风险"
  },
  {
    key: "recentTrend",
    label: "IMC · 近一年热度趋势",
    hint: "上升 / 平稳 / 下跌及依据"
  },
  {
    key: "socialFollowers",
    label: "IMC · 社媒粉丝总数",
    hint: "平台及粉丝规模"
  },
  {
    key: "brandValue",
    label: "IMC · 品牌调性价值",
    hint: "话题、品牌增益和年轻化效果"
  },
  {
    key: "ugcForecast",
    label: "品牌运营 · 用户内容预测",
    hint: "UGC产出、二创意愿、用户黏性"
  },
  {
    key: "audienceMatch",
    label: "用户运营 · 受众匹配",
    hint: "IP受众与品牌受众的匹配度"
  },
  {
    key: "cpmValue",
    label: "媒介投放 · CPM价值",
    hint: "预估ROI及投放成本对比（选填）"
  },
  { key: "prOpinion", label: "PR · 舆情判断", hint: "舆论风险与公关建议" }
];
const marketingFunctions = [
  {
    key: "imc",
    label: "IMC",
    fields: [
      "marketHeat",
      "fanAudience",
      "commercialValue",
      "marketingRisks",
      "recentTrend",
      "socialFollowers",
      "brandValue"
    ]
  },
  { key: "brand", label: "品牌运营", fields: ["ugcForecast"] },
  { key: "user", label: "用户运营", fields: ["audienceMatch"] },
  { key: "media", label: "媒介投放", fields: ["cpmValue"] },
  { key: "pr", label: "PR", fields: ["prOpinion"] }
];
const marketingFunctionKey = ref("");
const marketingFunctionDrafts = ref<
  Record<string, { comments: string; channels: string[] }>
>({});
const selectedMarketingFunction = computed(() =>
  marketingFunctions.find(item => item.key === marketingFunctionKey.value)
);
const marketingFunctionStates = computed(() =>
  objectValue(objectValue(detail.value?.request.marketingProfile).functions)
);
function marketingFunctionSubmitted(key: string) {
  return (
    Boolean(objectValue(marketingFunctionStates.value[key]).submittedAt) ||
    (detail.value?.request.status === "marketing_reviewed" &&
      !Object.keys(marketingFunctionStates.value).length)
  );
}
const submittedMarketingCount = computed(
  () =>
    marketingFunctions.filter(item => marketingFunctionSubmitted(item.key))
      .length
);
const canEditMarketingFunction = computed(
  () =>
    canMarketing.value &&
    !!selectedMarketingFunction.value &&
    !marketingFunctionSubmitted(marketingFunctionKey.value)
);
const selectedMarketingFields = computed(() =>
  marketingFields.filter(field =>
    selectedMarketingFunction.value?.fields.includes(field.key)
  )
);
const legacyMarketingComments = computed(() => {
  const saved = objectValue(detail.value?.request.marketingProfile);
  return (
    saved.legacyMarketingComments ||
    (!saved.functions ? detail.value?.request.marketingComments : "")
  );
});
const requiredMarketingFields = new Set([
  "marketHeat",
  "fanAudience",
  "commercialValue",
  "marketingRisks",
  "recentTrend",
  "socialFollowers",
  "ugcForecast",
  "audienceMatch",
  "prOpinion"
]);
function objectValue(value: unknown): Record<string, any> {
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
const statusText: Record<string, string> = {
  draft: "草稿",
  submitted: "待IP组反馈",
  ip_reviewed: "待营销补充",
  marketing_reviewed: "营销意见已提交"
};
function listStatusText(status: string) {
  if (menuKind.value === "feedback") {
    if (status === "ip_reviewed") return "已反馈 · 待营销补充";
    if (status === "marketing_reviewed") return "已反馈 · 营销已完成";
  }
  return statusText[status] || status;
}
const initialForm = (): IPRequestInput => ({
  projectName: "",
  department: "",
  markets: [],
  expectedLaunch: "",
  goal: "",
  description: "",
  budgetCurrency: "CNY",
  budgetMin: null,
  budgetMax: null,
  externalRecommendation: "",
  candidateIds: [],
  submit: false,
  brief: {
    externalIPs: [],
    candidateEvaluations: {},
    contactsByStage: emptyContactsByStage()
  }
});
const form = ref<IPRequestInput>(initialForm());
const briefOpen = ref(["brief"]);
const invalidField = ref("");
const brokenExternalImageUrls = ref<Record<number, boolean>>({});
const colleagueSearchStatus = ref<
  Record<string, "" | "empty" | "failed" | "limited">
>({});
const colleagueActiveQuery = ref<Record<string, string>>({});
const colleaguePickerOpen = ref(false);
const colleaguePickerStage = ref("");
const colleaguePickerQuery = ref("");
const colleaguePickerPage = ref(1);
const colleaguePickerHasNext = ref(false);
const colleaguePickerRows = ref<IPColleague[]>([]);
const colleaguePickerLoading = ref(false);
const colleaguePickerWarning = ref("");
let colleaguePickerRequest = 0;
type ColleagueSuggestion = Colleague & { value: string };
async function fetchColleagueSuggestions(
  stage: string,
  query: string,
  callback: (items: ColleagueSuggestion[]) => void
) {
  colleagueActiveQuery.value[stage] = query;
  if (!query.trim()) {
    colleagueSearchStatus.value[stage] = "";
    callback([]);
    return;
  }
  try {
    const response = await searchIPColleagues(query.trim());
    if (colleagueActiveQuery.value[stage] !== query) {
      callback([]);
      return;
    }
    const suggestions = (response.data || []).map(person => ({
      ...person,
      value: person.name
    }));
    colleagueSearchStatus.value[stage] = response.directoryWarning
      ? "limited"
      : suggestions.length
        ? ""
        : "empty";
    callback(suggestions);
  } catch {
    if (colleagueActiveQuery.value[stage] !== query) {
      callback([]);
      return;
    }
    colleagueSearchStatus.value[stage] = "failed";
    callback([]);
  }
}
function selectContact(stage: string, colleague: Record<string, any>) {
  form.value.brief.contactsByStage[stage] = {
    id: colleague.id,
    name: String(colleague.name || colleague.value || ""),
    department: String(colleague.department || ""),
    employeeNo: String(colleague.employeeNo || "")
  };
  invalidField.value = "";
}
function openColleaguePicker(stage: string) {
  colleaguePickerRequest++;
  colleaguePickerStage.value = stage;
  colleaguePickerQuery.value =
    form.value.brief.contactsByStage[stage].name || "";
  colleaguePickerPage.value = 1;
  colleaguePickerRows.value = [];
  colleaguePickerHasNext.value = false;
  colleaguePickerWarning.value = "";
  colleaguePickerOpen.value = true;
  if (colleaguePickerQuery.value.trim()) void loadColleaguePickerPage(1);
}
function onColleaguePickerQueryInput() {
  colleaguePickerRequest++;
  colleaguePickerLoading.value = false;
  colleaguePickerPage.value = 1;
  colleaguePickerRows.value = [];
  colleaguePickerHasNext.value = false;
  colleaguePickerWarning.value = "";
}
async function loadColleaguePickerPage(targetPage: number) {
  const query = colleaguePickerQuery.value.trim();
  if (!query || targetPage < 1) {
    colleaguePickerRows.value = [];
    colleaguePickerHasNext.value = false;
    colleaguePickerWarning.value = query ? "" : "请输入姓名或工号";
    return;
  }
  const request = ++colleaguePickerRequest;
  colleaguePickerLoading.value = true;
  try {
    const response = await searchIPColleagues(query, targetPage);
    if (request !== colleaguePickerRequest || !colleaguePickerOpen.value)
      return;
    colleaguePickerPage.value = targetPage;
    colleaguePickerRows.value = response.data || [];
    colleaguePickerHasNext.value = Boolean(response.hasNext);
    colleaguePickerWarning.value = response.directoryWarning || "";
  } catch {
    if (request !== colleaguePickerRequest || !colleaguePickerOpen.value)
      return;
    colleaguePickerRows.value = [];
    colleaguePickerHasNext.value = false;
    colleaguePickerWarning.value = "搜索失败，请重试";
  } finally {
    if (request === colleaguePickerRequest)
      colleaguePickerLoading.value = false;
  }
}
function chooseColleagueFromPicker(person: IPColleague) {
  selectContact(colleaguePickerStage.value, person);
  colleaguePickerOpen.value = false;
}
function onContactInput(stage: string, value: string | number) {
  form.value.brief.contactsByStage[stage] = { name: String(value) };
  colleagueActiveQuery.value[stage] = String(value);
  colleagueSearchStatus.value[stage] = "";
  invalidField.value = "";
}
function validationClass(key: string) {
  return invalidField.value === key ? "ip-invalid-field" : "";
}
async function showValidation(
  message: string,
  key: string,
  expandBrief = false
) {
  invalidField.value = key;
  if (expandBrief) briefOpen.value = ["brief"];
  ElMessage.warning(message);
  await nextTick();
  const target = document.querySelector<HTMLElement>(
    `[data-validation-key="${key}"]`
  );
  target?.scrollIntoView({ behavior: "smooth", block: "center" });
  target?.querySelector<HTMLElement>("input, textarea")?.focus();
}
function imageURLValid(value: unknown) {
  try {
    const url = new URL(String(value || ""));
    return (
      (url.protocol === "https:" || url.protocol === "http:") &&
      Boolean(url.hostname) &&
      !url.username &&
      !url.password
    );
  } catch {
    return false;
  }
}
const feedback = ref<any[]>([]);
const marketingCandidates = ref<Record<string, Record<string, string>>>({});
const addCandidateID = ref<number | null>(null);
const marketing = ref({
  marketHeat: "",
  fanAudience: "",
  commercialValue: "",
  marketingRisks: "",
  marketingChannels: [] as string[],
  marketingComments: "",
  marketingProfile: {} as Record<string, any>
});
const menuKind = computed(() =>
  route.path.endsWith("/feedback")
    ? "feedback"
    : route.path.endsWith("/marketing")
      ? "marketing"
      : "requests"
);
const pageTitle = computed(() =>
  menuKind.value === "feedback"
    ? "初步意向IP反馈"
    : menuKind.value === "marketing"
      ? "营销意见补充"
      : "IP需求管理"
);
const pageSubtitle = computed(() =>
  menuKind.value === "feedback"
    ? "评估意向IP的合作可行性与推荐顺序"
    : menuKind.value === "marketing"
      ? "补充市场热度、商业价值与传播建议"
      : "提起需求并追踪IP合作评估进度"
);
const canFeedback = computed(
  () => detail.value?.request.status === "submitted"
);
const canMarketing = computed(
  () => detail.value?.request.status === "ip_reviewed"
);

function arrayValue(value: unknown): string[] {
  if (Array.isArray(value)) return value;
  try {
    return JSON.parse(String(value || "[]"));
  } catch {
    return [];
  }
}
function priceText(row: any) {
  if (row.budgetMin == null && row.budgetMax == null) return "待填写";
  const symbol =
    row.budgetCurrency === "USD"
      ? "$"
      : row.budgetCurrency === "EUR"
        ? "€"
        : "¥";
  return `${symbol}${Number(row.budgetMin || 0).toLocaleString()} – ${symbol}${Number(row.budgetMax || 0).toLocaleString()}`;
}
async function loadRows() {
  loading.value = true;
  try {
    const statuses =
      menuKind.value === "feedback"
        ? ["submitted", "ip_reviewed", "marketing_reviewed"]
        : menuKind.value === "marketing"
          ? ["ip_reviewed", "marketing_reviewed"]
          : [];
    const result = await listIPRequests({
      keyword: keyword.value,
      statuses,
      page: page.value,
      pageSize: 20
    });
    rows.value = result.data.list || [];
    total.value = result.data.total || 0;
  } catch {
    rows.value = [];
    total.value = 0;
    ElMessage.error("IP需求列表加载失败，请稍后重试");
  } finally {
    loading.value = false;
  }
}
async function loadResources() {
  const result = await listIPResources({ keyword: "", page: 1, pageSize: 200 });
  resourceOptions.value = result.data.list || [];
  await loadMissingSelectedResources();
  await loadIPImages(
    resourceOptions.value.filter(item =>
      form.value.candidateIds.includes(Number(item.id))
    ),
    "id"
  );
}
async function loadMissingSelectedResources() {
  const missing = form.value.candidateIds.filter(id => !selectedIP(id));
  await Promise.allSettled(
    missing.map(async id => {
      const response = (await getIPResource(id)).data;
      if (!selectedIP(id))
        resourceOptions.value.push({
          ...response.resource,
          visualFileId: response.files.find(file => file.fileKind === "visual")
            ?.id
        });
    })
  );
}
async function loadIPImages(items: any[], idKey: string) {
  await Promise.allSettled(
    items
      .filter(
        item => item.visualFileId && !ipImageUrls.value[Number(item[idKey])]
      )
      .map(async item => {
        const id = Number(item[idKey]);
        const blob = await readIPFile(Number(item.visualFileId));
        ipImageUrls.value[id] = URL.createObjectURL(blob);
      })
  );
}
function goList() {
  router.push({ path: route.path });
}
function goNew() {
  form.value = initialForm();
  router.push({ path: "/business/ip/requests", query: { mode: "new" } });
}
function goDetail(id: number) {
  router.push({ path: route.path, query: { id: String(id) } });
}
function goEdit(id: number) {
  router.push({
    path: "/business/ip/requests",
    query: { id: String(id), mode: "edit" }
  });
}
async function deleteRequest(row: any) {
  if (!row.canDelete || deletingID.value != null) return;
  try {
    await ElMessageBox.confirm(
      `确定删除「${row.projectName}」？需求、IP评估意见和营销意见将一并删除，无法恢复。资源库和 AI 会话会保留，会话与该需求的关联将解除。`,
      "删除IP需求",
      {
        confirmButtonText: "确认删除",
        cancelButtonText: "取消",
        type: "warning",
        distinguishCancelAndClose: true,
        closeOnClickModal: false
      }
    );
  } catch {
    return;
  }
  deletingID.value = Number(row.id);
  try {
    await deleteIPRequest(Number(row.id));
    ElMessage.success("需求已删除");
    if (mode.value !== "list") {
      goList();
    } else {
      if (rows.value.length === 1 && page.value > 1) page.value--;
      await loadRows();
    }
  } catch (error) {
    ElMessage.error(
      (error as any)?.response?.data?.message || "删除失败，请刷新后重试"
    );
  } finally {
    deletingID.value = null;
  }
}
function storeRecommendationContext() {
  localStorage.setItem(
    librarySelectionKey.value,
    JSON.stringify(form.value.candidateIds)
  );
  localStorage.setItem(
    `${librarySelectionKey.value}-recommendation-criteria`,
    JSON.stringify({
      name: form.value.projectName
        ? `${form.value.projectName} · IP寻找`
        : "IP寻找",
      query: form.value.description,
      markets: form.value.markets,
      ipTypes: form.value.brief.targetIPType
        ? [String(form.value.brief.targetIPType)]
        : [],
      goal: form.value.goal,
      budgetCurrency: form.value.budgetCurrency,
      budgetMin: form.value.budgetMin,
      budgetMax: form.value.budgetMax
    })
  );
}
function canOpenIPRecommendations() {
  if (router.hasRoute("IPRecommendations")) return true;
  ElMessage.warning("当前账号尚未开通 IP AI 推荐，请联系管理员配置菜单权限");
  return false;
}
function openRecommendationsFromForm(event: MouseEvent) {
  if (!canOpenIPRecommendations()) {
    event.preventDefault();
    return;
  }
  storeRecommendationContext();
}
function openRecommendationsFromDetail() {
  if (!detail.value) return;
  if (!canOpenIPRecommendations()) return;
  if (detail.value.request.status === "draft") {
    editDraft();
    storeRecommendationContext();
    window.open(recommendationHref.value, "_blank", "noopener");
    return;
  }
  router.push({
    path: "/business/ip/recommendations",
    query: { requestId: String(detail.value.request.id) }
  });
}
async function loadDetail(id: number) {
  loading.value = true;
  try {
    if (Number(detail.value?.request.id) !== id)
      marketingFunctionKey.value = "";
    detail.value = (await getIPRequest(id)).data;
    void loadIPImages(detail.value.candidates, "ipId");
    feedback.value = detail.value.candidates.map(row => ({
      ipId: Number(row.ipId),
      name: row.name,
      priorityOrder: [0, 1, 2].includes(Number(row.priorityOrder))
        ? Number(row.priorityOrder)
        : 1,
      feasibility: row.feasibility || "",
      recommendation: row.recommendation || "",
      reason: row.reason || "",
      assessment: {
        ...objectValue(row.assessment),
        scoreDimensions: objectValue(
          objectValue(row.assessment).scoreDimensions
        )
      }
    }));
    if (detail.value.request.status === "submitted") await loadResources();
    const item = detail.value.request;
    const savedOpinions = objectValue(item.marketingProfile);
    const functions = objectValue(savedOpinions.functions);
    marketingFunctionDrafts.value = Object.fromEntries(
      marketingFunctions.map(item => {
        const state = objectValue(functions[item.key]);
        return [
          item.key,
          {
            comments: String(state.comments || ""),
            channels: arrayValue(state.channels)
          }
        ];
      })
    );
    marketingCandidates.value = objectValue(savedOpinions.candidates);
    for (const row of detail.value.candidates) {
      marketingCandidates.value[String(row.ipId)] ||= {};
    }
    marketing.value = {
      marketHeat: item.marketHeat || "",
      fanAudience: item.fanAudience || "",
      commercialValue: item.commercialValue || "",
      marketingRisks: item.marketingRisks || "",
      marketingChannels: arrayValue(item.marketingChannels),
      marketingComments: item.marketingComments || "",
      marketingProfile: savedOpinions
    };
  } catch {
    detail.value = null;
    ElMessage.error("IP需求详情加载失败，请稍后重试");
    router.replace({ path: route.path });
  } finally {
    loading.value = false;
  }
}
function addFeedbackCandidate() {
  const id = addCandidateID.value;
  if (!id || feedback.value.some(row => row.ipId === id)) return;
  if (feedback.value.length >= 8) {
    ElMessage.warning("最多评估8个IP（含IP组追加建议）");
    return;
  }
  feedback.value.push({
    ipId: id,
    name: selectedIP(id)?.name || "IP",
    priorityOrder: 1,
    feasibility: "",
    recommendation: "",
    reason: "",
    assessment: { scoreDimensions: {} }
  });
  addCandidateID.value = null;
}
function editDraft() {
  if (!detail.value) return;
  if (!detail.value.request.canEdit) {
    ElMessage.warning("只能编辑本人发起的需求");
    return;
  }
  const item = detail.value.request;
  form.value = {
    id: Number(item.id),
    projectName: item.projectName,
    department: item.department,
    markets: arrayValue(item.markets),
    expectedLaunch: item.expectedLaunch || "",
    goal: item.goal,
    description: item.description,
    budgetCurrency: item.budgetCurrency || "CNY",
    budgetMin: item.budgetMin == null ? null : Number(item.budgetMin),
    budgetMax: item.budgetMax == null ? null : Number(item.budgetMax),
    externalRecommendation: item.externalRecommendation || "",
    candidateIds: detail.value.candidates.map(row => Number(row.ipId)),
    submit: false,
    brief: {
      ...objectValue(item.brief),
      externalIPs: objectValue(item.brief).externalIPs || [],
      candidateEvaluations: objectValue(item.brief).candidateEvaluations || {},
      contactsByStage: normalizedContactsByStage(
        objectValue(item.brief).contactsByStage
      )
    }
  };
  briefOpen.value = ["brief"];
  invalidField.value = "";
  mode.value = "new";
  loadResources();
}
async function syncLibrarySelection() {
  if (
    !librarySelectionKey.value ||
    mode.value !== "new" ||
    candidateSelectionLocked.value
  )
    return;
  const saved = localStorage.getItem(librarySelectionKey.value);
  if (!saved) return;
  localStorage.removeItem(librarySelectionKey.value);
  try {
    form.value.candidateIds = JSON.parse(saved)
      .map(Number)
      .filter((id: number) => id > 0)
      .slice(0, 5);
    await loadResources();
    await loadMissingSelectedResources();
    await loadIPImages(
      resourceOptions.value.filter(item =>
        form.value.candidateIds.includes(Number(item.id))
      ),
      "id"
    );
    ensureCandidateEvaluations();
    ElMessage.success("已从IP资源库加入意向清单");
  } catch {
    ElMessage.warning("IP选择结果读取失败，请重新选择");
  }
}
function onLibraryStorage(event: StorageEvent) {
  if (event.key === librarySelectionKey.value) syncLibrarySelection();
}
function ipDetailHref(id: number) {
  return router.resolve({
    path: "/business/ip/resources",
    query: { id }
  }).href;
}
function openExternalIP(item: any) {
  window.open(
    router.resolve({
      path: "/business/ip/resources",
      query: {
        mode: "new",
        name: item.name,
        rightsOwner: item.rightsOwner,
        summary: item.reason,
        imageStyle: item.imageUrl
      }
    }).href,
    "_blank",
    "noopener"
  );
}
function selectedIP(id: number) {
  return resourceOptions.value.find(row => Number(row.id) === id);
}
function removeCandidate(id: number) {
  form.value.candidateIds = form.value.candidateIds.filter(
    value => value !== id
  );
}
function removeExternalIP(index: number) {
  form.value.brief.externalIPs.splice(index, 1);
  brokenExternalImageUrls.value = {};
  invalidField.value = "";
}
function ensureCandidateEvaluations() {
  const evaluations = (form.value.brief.candidateEvaluations ||= {});
  for (const id of form.value.candidateIds) {
    evaluations[id] ||= { reason: "", productEvaluation: "", technicalFit: "" };
  }
}
watch(() => form.value.candidateIds, ensureCandidateEvaluations, {
  deep: true
});
watch(
  () => form.value.candidateIds,
  () =>
    loadIPImages(
      resourceOptions.value.filter(item =>
        form.value.candidateIds.includes(Number(item.id))
      ),
      "id"
    ),
  { deep: true }
);
async function exportEvaluation(candidate: any) {
  if (!detail.value) return;
  const resourceDetail = (await getIPResource(Number(candidate.ipId))).data;
  const resource = resourceDetail.resource;
  const request = detail.value.request;
  const profile = objectValue(resource.profile);
  const brief = objectValue(request.brief);
  const assessment = objectValue(candidate.assessment);
  const savedOpinions = objectValue(request.marketingProfile);
  const opinion = objectValue(
    objectValue(savedOpinions.candidates)[candidate.ipId]
  );
  const candidateBrief = objectValue(
    objectValue(brief.candidateEvaluations)[candidate.ipId]
  );
  const copyrightFiles = resourceDetail.files
    .filter((file: any) => file.fileKind === "copyright")
    .map((file: any) => file.originalName);
  const caseSummaries = resourceDetail.cases.map((item: any) => {
    const names = resourceDetail.files
      .filter((file: any) => Number(file.caseId) === Number(item.id))
      .map((file: any) => file.originalName);
    return [item.title, item.summary, ...names].filter(Boolean).join("；");
  });
  const rows: [string, string, unknown][] = [
    ["基础信息", "IP(艺人/游戏/动漫/赛事等)名称", resource.name],
    [
      "基础信息",
      "介绍",
      [resource.summary, ...copyrightFiles].filter(Boolean).join("；")
    ],
    ["基础信息", "市场/区域", arrayValue(resource.markets).join("、")],
    ["基础信息", "IP生命周期分类", profile.lifecycle],
    [
      "基础信息",
      "合作筛选理由",
      candidateBrief.reason || brief.selectionReason
    ],
    ["受众情况", "用户画像-性别占比", profile.genderRatio],
    ["受众情况", "用户画像-主要年龄", profile.ageRange],
    ["受众情况", "用户画像-消费力评价", profile.spendingPower],
    [
      "受众情况",
      "用户内容预测",
      opinion.ugcForecast || savedOpinions.ugcForecast
    ],
    ["市场影响力", "IP热度", opinion.marketHeat || request.marketHeat],
    ["市场影响力", "注册用户", profile.registeredUsers],
    [
      "市场影响力",
      "近一年热度",
      opinion.recentTrend || savedOpinions.recentTrend
    ],
    [
      "市场影响力",
      "声量价值",
      opinion.socialFollowers || savedOpinions.socialFollowers
    ],
    [
      "市场影响力",
      "品牌调性价值",
      opinion.brandValue || savedOpinions.brandValue
    ],
    ["产品匹配", "联动产品", brief.linkedProduct],
    [
      "产品匹配",
      "产品维度评估指标汇总",
      candidateBrief.productEvaluation || brief.productEvaluation
    ],
    [
      "产品匹配",
      "技术匹配程度",
      candidateBrief.technicalFit || brief.technicalFit
    ],
    ["商业化/带货潜力", "销售转换力", profile.salesROI],
    ["商业化/带货潜力", "CPM价值", opinion.cpmValue || savedOpinions.cpmValue],
    ["联动条款", "合作模式", brief.cooperationMode],
    ["联动条款", "合作价格", assessment.priceModel],
    ["联动条款", "合作时间", brief.projectPeriod],
    ["联动条款", "授权内容", assessment.licenseContent],
    ["联动条款", "授权区域范围", assessment.licenseTerritory],
    ["联动条款", "IP方可提供资源", assessment.ipResources],
    ["联动条款", "Infinix侧需提供资源", assessment.brandResources],
    ["商务判断", "风险评估", assessment.businessRisk],
    ["商务判断", "优先级", `T${candidate.priorityOrder}`],
    [
      "商务判断",
      "合作案例",
      [assessment.pastCases, ...caseSummaries].filter(Boolean).join("；")
    ],
    ["商务判断", "相关文档/纪要", assessment.relatedDocuments],
    ["商务判断", "备注", assessment.notes],
    ["商务判断", "初判断", candidate.recommendation],
    ["法务判断", "风险评估", assessment.publicRisk],
    ["法务判断", "版权归属清晰度", assessment.ownershipClarity]
  ];
  const sheet = XLSX.utils.aoa_to_sheet([
    [
      "项目",
      request.projectName,
      "意向IP",
      resource.name,
      "综合评分",
      weightedScore(assessment) ?? "待评分"
    ],
    ["类别", "字段", "评估内容"],
    ...rows.map(([category, field, value]) => [
      category,
      field,
      value || "待补充"
    ])
  ]);
  sheet["!cols"] = [{ wch: 16 }, { wch: 28 }, { wch: 65 }];
  const book = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(book, sheet, "完整评估表");
  XLSX.writeFile(book, `${request.projectName}_${resource.name}_评估表.xlsx`);
}
async function save(submit: boolean) {
  const item = form.value;
  if (submit) {
    const baseRequired = [
      ["projectName", "项目名称", item.projectName],
      ["department", "需求发起部门", item.department],
      ["markets", "目标市场", item.markets.length ? "selected" : ""],
      ["goal", "合作目标", item.goal],
      ["description", "项目背景与需求说明", item.description]
    ];
    for (const [key, label, value] of baseRequired) {
      if (!String(value || "").trim()) {
        await showValidation(`请填写${label}`, String(key));
        return;
      }
    }
    if (
      item.budgetMin == null ||
      item.budgetMax == null ||
      item.budgetMin < 0 ||
      item.budgetMax < 0 ||
      item.budgetMin > item.budgetMax
    ) {
      await showValidation("请填写有效的授权费用预算范围", "budget");
      return;
    }
    for (const field of briefFields.filter(field => field.required)) {
      if (!String(item.brief[field.key] || "").trim()) {
        await showValidation(
          `请填写补充项目资料中的「${field.label}」`,
          `brief-${field.key}`,
          true
        );
        return;
      }
    }
    for (const stage of contactStages) {
      if (!String(item.brief.contactsByStage?.[stage.key]?.name || "").trim()) {
        await showValidation(
          `请填写「${stage.label}」环节对接人`,
          `contact-${stage.key}`,
          true
        );
        return;
      }
    }
    await loadMissingSelectedResources();
    for (const id of candidateSelectionLocked.value ? [] : item.candidateIds) {
      const candidateName = selectedIP(id)?.name;
      if (!candidateName) {
        await showValidation(
          `库内意向IP（ID ${id}）已不可用，请移除后重新选择`,
          `candidate-${id}`
        );
        return;
      }
      for (const [key, label] of [
        ["reason", "选择理由"],
        ["productEvaluation", "产品维度评估"],
        ["technicalFit", "技术匹配程度"]
      ]) {
        if (
          !String(item.brief.candidateEvaluations?.[id]?.[key] || "").trim()
        ) {
          await showValidation(
            `请补齐库内意向IP「${candidateName}」的${label}`,
            `candidate-${id}-${key}`
          );
          return;
        }
      }
    }
    if (!item.candidateIds.length && !item.brief.externalIPs.length) {
      await showValidation(
        "请至少选择一个库内IP或添加一个库外IP意向",
        "ip-intentions"
      );
      return;
    }
    for (const [index, candidate] of item.brief.externalIPs.entries()) {
      for (const [key, label] of [
        ["name", "IP名称"],
        ["rightsOwner", "版权方"],
        ["imageUrl", "IP形象示意链接"],
        ["reason", "选择理由"],
        ["productEvaluation", "产品维度评估"],
        ["technicalFit", "技术匹配程度"]
      ]) {
        if (!String(candidate[key] || "").trim()) {
          await showValidation(
            `请补齐库外IP ${index + 1} 的${label}`,
            `external-${index}-${key}`
          );
          return;
        }
      }
      if (!imageURLValid(candidate.imageUrl)) {
        await showValidation(
          `库外IP ${index + 1} 的形象示意链接需以 http:// 或 https:// 开头`,
          `external-${index}-imageUrl`
        );
        return;
      }
    }
  }
  // Keep historical free-text contacts in old drafts until all stages are filled.
  if (
    contactStages.every(stage =>
      String(item.brief.contactsByStage?.[stage.key]?.name || "").trim()
    )
  ) {
    item.brief.contacts = contactStages
      .map(
        stage => `${stage.label}：${item.brief.contactsByStage[stage.key].name}`
      )
      .join("；");
  }
  saving.value = true;
  try {
    const result = await saveIPRequest({ ...item, submit });
    ElMessage.success(
      editingSubmitted.value
        ? "需求已更新，已有意见及截止时间已保留"
        : submit
          ? "需求已提交，等待IP组反馈"
          : "草稿已保存"
    );
    if (Number(route.query.id) === Number(result.data.id)) {
      if (route.query.mode === "edit") {
        await router.replace({
          path: route.path,
          query: { id: String(result.data.id) }
        });
        return;
      }
      mode.value = "detail";
      await loadDetail(Number(result.data.id));
    } else
      router.push({
        path: "/business/ip/requests",
        query: { id: String(result.data.id) }
      });
  } catch (error) {
    ElMessage.error(
      (error as any)?.response?.data?.message || "需求保存失败，请稍后重试"
    );
  } finally {
    saving.value = false;
  }
}
async function sendFeedback(saveDraft = false) {
  if (!detail.value) return;
  if (!feedback.value.length) {
    ElMessage.warning("请先为需求关联意向IP");
    return;
  }
  if (
    !saveDraft &&
    feedback.value.some(
      row => !row.feasibility || !row.recommendation || !row.reason.trim()
    )
  ) {
    ElMessage.warning("请填写每个IP的可行性、推荐意见和理由");
    return;
  }
  if (
    feedback.value.some(row => ![0, 1, 2].includes(Number(row.priorityOrder)))
  ) {
    ElMessage.warning("请选择T0、T1或T2优先级");
    return;
  }
  if (
    !saveDraft &&
    feedback.value.some(row =>
      [...requiredAssessmentFields].some(
        key => !String(row.assessment[key] || "").trim()
      )
    )
  ) {
    ElMessage.warning("请补齐每个IP的授权、资源、商务与舆情风险信息");
    return;
  }
  if (
    !saveDraft &&
    feedback.value.some(row => weightedScore(row.assessment) === null)
  ) {
    ElMessage.warning("请为每个IP填写四项0至100分的评估分数");
    return;
  }
  saving.value = true;
  try {
    await submitIPFeedback({
      requestId: Number(detail.value.request.id),
      saveDraft,
      candidates: feedback.value
    });
    ElMessage.success(
      saveDraft ? "IP评估草稿已保存" : "IP反馈已提交，等待营销意见补充"
    );
    await loadDetail(Number(detail.value.request.id));
  } finally {
    saving.value = false;
  }
}
async function sendMarketing(saveDraft = false) {
  if (!detail.value || !selectedMarketingFunction.value) {
    ElMessage.warning("请选择本次填写的职能");
    return;
  }
  if (!canEditMarketingFunction.value) return;
  const role = selectedMarketingFunction.value;
  const draft = marketingFunctionDrafts.value[role.key];
  if (!saveDraft && !draft.comments.trim()) {
    ElMessage.warning(`请填写${role.label}的补充意见`);
    return;
  }
  if (
    !saveDraft &&
    detail.value.candidates.some(candidate =>
      role.fields.some(
        key =>
          requiredMarketingFields.has(key) &&
          !String(
            marketingCandidates.value[String(candidate.ipId)]?.[key] || ""
          ).trim()
      )
    )
  ) {
    ElMessage.warning(`请补齐每个意向IP的${role.label}意见，无需填写其他职能`);
    return;
  }
  saving.value = true;
  try {
    const result = await submitIPMarketing({
      requestId: Number(detail.value.request.id),
      functionKey: role.key,
      version:
        Number(objectValue(marketingFunctionStates.value[role.key]).version) ||
        0,
      saveDraft,
      comments: draft.comments,
      channels: draft.channels,
      candidates: Object.fromEntries(
        detail.value.candidates.map(candidate => [
          String(candidate.ipId),
          Object.fromEntries(
            role.fields.map(key => [
              key,
              marketingCandidates.value[String(candidate.ipId)]?.[key] || ""
            ])
          )
        ])
      ),
      ...(role.key === "imc"
        ? {
            summary: {
              marketHeat: marketing.value.marketHeat,
              fanAudience: marketing.value.fanAudience,
              commercialValue: marketing.value.commercialValue,
              marketingRisks: marketing.value.marketingRisks
            }
          }
        : {})
    });
    ElMessage.success(
      saveDraft
        ? `${role.label}意见草稿已保存`
        : result.data.status === "marketing_reviewed"
          ? "各职能意见已全部提交"
          : `${role.label}意见已提交，其他职能可继续补充`
    );
    await loadDetail(Number(detail.value.request.id));
  } catch (error) {
    const response = (error as any)?.response;
    if (Number(response?.status) === 409)
      ElMessage.warning("该职能意见已被更新，请刷新后重试");
    else
      ElMessage.error(
        Number(response?.status) === 400
          ? response.data?.message || "请检查本职能意见"
          : "意见未保存，请稍后重试"
      );
  } finally {
    saving.value = false;
  }
}
async function syncRoute() {
  const id = Number(route.query.id);
  if (id > 0) {
    mode.value = "detail";
    await loadDetail(id);
    if (
      route.query.mode === "edit" &&
      menuKind.value === "requests" &&
      detail.value
    )
      editDraft();
  } else if (route.query.mode === "new" && menuKind.value === "requests") {
    mode.value = "new";
    form.value = initialForm();
    const recommendedIds = String(route.query.recommendedIps || "")
      .split(",")
      .map(Number)
      .filter(id => Number.isInteger(id) && id > 0)
      .slice(0, 5);
    const recommendedId = Number(route.query.recommendedIp);
    if (recommendedIds.length) {
      form.value.candidateIds = [...new Set(recommendedIds)];
      ensureCandidateEvaluations();
    } else if (recommendedId > 0) form.value.candidateIds = [recommendedId];
    await loadResources();
    for (const id of recommendedIds) {
      form.value.brief.candidateEvaluations[id].reason = String(
        route.query.recommendationReason || ""
      );
    }
    if (recommendedId > 0) {
      if (!selectedIP(recommendedId)) {
        try {
          const response = (await getIPResource(recommendedId)).data;
          resourceOptions.value.push({
            ...response.resource,
            visualFileId: response.files.find(
              file => file.fileKind === "visual"
            )?.id
          });
        } catch {
          form.value.candidateIds = [];
          ElMessage.warning("推荐的 IP 已不可用，请重新选择");
        }
      }
      ensureCandidateEvaluations();
      if (form.value.candidateIds.includes(recommendedId))
        form.value.brief.candidateEvaluations[recommendedId].reason = String(
          route.query.recommendationReason || ""
        );
    }
  } else {
    mode.value = "list";
    loadRows();
  }
}
watch(() => route.fullPath, syncRoute);
onMounted(() => {
  syncRoute();
  window.addEventListener("focus", syncLibrarySelection);
  window.addEventListener("storage", onLibraryStorage);
});
onUnmounted(() => {
  window.removeEventListener("focus", syncLibrarySelection);
  window.removeEventListener("storage", onLibraryStorage);
  for (const url of Object.values(ipImageUrls.value)) URL.revokeObjectURL(url);
});
</script>

<template>
  <div
    v-loading="loading"
    class="ip-request-page"
    :class="{ 'ip-detail-page': mode === 'detail' }"
  >
    <div class="ip-header">
      <div>
        <div v-if="mode === 'new'" class="ip-kicker">
          需求管理 / {{ mode === "new" ? "提起IP需求" : pageTitle }}
        </div>
        <h1>
          {{
            mode === "list"
              ? pageTitle
              : mode === "new"
                ? form.id
                  ? "编辑IP需求"
                  : "提起IP需求"
                : menuKind === "requests"
                  ? "IP需求详情"
                  : pageTitle
          }}
        </h1>
        <p>
          {{
            mode === "list"
              ? pageSubtitle
              : mode === "new"
                ? editingSubmitted
                  ? "修改项目资料，保留已有意见、流程进度与反馈截止时间"
                  : "明确合作目标和授权预算，提交给IP组评估"
                : menuKind === "requests"
                  ? "查看需求、IP反馈与营销意见"
                  : pageSubtitle
          }}
        </p>
      </div>
      <div>
        <el-button v-if="mode !== 'list'" @click="goList">返回列表</el-button
        ><el-button
          v-if="menuKind === 'requests' && mode === 'list'"
          class="ip-primary"
          @click="goNew"
          >＋ 提起IP需求</el-button
        >
      </div>
    </div>

    <template v-if="mode === 'list'"
      ><div class="ip-card">
        <div class="ip-search">
          <el-input
            v-model="keyword"
            placeholder="搜索项目名称"
            clearable
            @keyup.enter="
              page = 1;
              loadRows();
            "
          /><el-button
            class="ip-primary"
            @click="
              page = 1;
              loadRows();
            "
            >搜索</el-button
          ><span>共 {{ total }} 条需求</span>
        </div>
        <el-table
          class="ip-request-table"
          :data="rows"
          :cell-style="{ textAlign: 'center' }"
          :header-cell-style="{ textAlign: 'center' }"
          empty-text="暂无符合条件的IP需求"
          stripe
          ><el-table-column label="项目名称" min-width="200"
            ><template #default="scope"
              ><button class="ip-link" @click="goDetail(scope.row.id)">
                {{ scope.row.projectName }}</button
              ><small
                >{{ scope.row.department }} · {{ scope.row.createdAt }}</small
              ></template
            ></el-table-column
          ><el-table-column label="目标市场" min-width="120"
            ><template #default="scope">{{
              arrayValue(scope.row.markets).join(" / ")
            }}</template></el-table-column
          ><el-table-column
            prop="goal"
            label="合作目标"
            min-width="100"
          /><el-table-column label="授权费用预算" min-width="150"
            ><template #default="scope">{{
              priceText(scope.row)
            }}</template></el-table-column
          ><el-table-column
            prop="candidateCount"
            label="意向IP"
            width="70"
          /><el-table-column label="进度" width="130"
            ><template #default="scope"
              ><el-tag
                :type="
                  scope.row.status === 'marketing_reviewed' ||
                  (menuKind === 'feedback' &&
                    scope.row.status === 'ip_reviewed')
                    ? 'success'
                    : scope.row.status === 'draft'
                      ? 'info'
                      : 'warning'
                "
                >{{ listStatusText(scope.row.status) }}</el-tag
              ></template
            ></el-table-column
          ><el-table-column label="最晚反馈（北京时间）" min-width="200">
            <template #default="{ row }">
              <DeadlineStatus
                v-if="row.status !== 'draft'"
                compact
                :deadline="
                  menuKind === 'feedback' ||
                  (menuKind === 'requests' && row.status === 'submitted')
                    ? row.deadlines?.ip
                    : row.deadlines?.marketing
                "
              />
              <span v-else>提交后开始计时</span>
            </template> </el-table-column
          ><el-table-column
            label="操作"
            :width="menuKind === 'requests' ? 200 : 115"
            fixed="right"
            ><template #default="scope"
              ><div class="ip-row-actions">
                <el-button link @click="goDetail(scope.row.id)">{{
                  menuKind === "feedback"
                    ? scope.row.status === "submitted"
                      ? "填写反馈"
                      : "查看反馈"
                    : menuKind === "marketing"
                      ? scope.row.status === "ip_reviewed"
                        ? "补充意见"
                        : "查看意见"
                      : "查看详情"
                }}</el-button>
                <template v-if="menuKind === 'requests'">
                  <span
                    :title="
                      scope.row.canEdit ? '编辑需求' : '仅发起人本人可编辑'
                    "
                  >
                    <el-button
                      link
                      :disabled="!scope.row.canEdit || deletingID != null"
                      :aria-label="`编辑${scope.row.projectName}`"
                      @click="goEdit(scope.row.id)"
                      >编辑</el-button
                    >
                  </span>
                  <span
                    :title="
                      scope.row.canDelete ? '删除需求' : '仅发起人本人可删除'
                    "
                  >
                    <el-button
                      link
                      type="danger"
                      :disabled="!scope.row.canDelete || deletingID != null"
                      :loading="deletingID === Number(scope.row.id)"
                      :aria-label="`删除${scope.row.projectName}`"
                      @click="deleteRequest(scope.row)"
                      >删除</el-button
                    >
                  </span>
                </template>
              </div></template
            ></el-table-column
          ></el-table
        >
        <div class="ip-pager">
          <el-pagination
            v-model:current-page="page"
            :page-size="20"
            layout="total, prev, pager, next"
            :total="total"
            @current-change="loadRows"
          />
        </div></div
    ></template>

    <template v-else-if="mode === 'new'"
      ><div v-if="!editingSubmitted" class="ip-steps">
        <span class="active"><b>1</b>基本信息</span>
        <span :class="{ active: form.candidateIds.length > 0 }"
          ><b>2</b>初步意向IP</span
        >
        <span><b>3</b>确认提交</span>
      </div>
      <div class="ip-grid">
        <div class="ip-stack">
          <div class="ip-card">
            <h2>项目基本信息</h2>
            <div class="ip-form-grid">
              <label
                data-validation-key="projectName"
                :class="validationClass('projectName')"
                ><span class="ip-field-label"
                  >项目名称
                  <span class="ip-required" aria-hidden="true">*</span></span
                ><el-input
                  v-model="form.projectName"
                  placeholder="请输入项目名称" /></label
              ><label
                data-validation-key="department"
                :class="validationClass('department')"
                ><span class="ip-field-label"
                  >需求发起部门
                  <span class="ip-required" aria-hidden="true">*</span></span
                ><el-input
                  v-model="form.department"
                  placeholder="例如：产品组" /></label
              ><label
                data-validation-key="markets"
                :class="validationClass('markets')"
                ><span class="ip-field-label"
                  >目标市场
                  <span class="ip-required" aria-hidden="true">*</span></span
                ><el-select
                  v-model="form.markets"
                  multiple
                  filterable
                  allow-create
                  default-first-option
                  placeholder="请选择目标市场"
                  ><el-option
                    v-for="market in markets"
                    :key="market"
                    :label="market"
                    :value="market" /></el-select></label
              ><label
                >预计上线时间<el-date-picker
                  v-model="form.expectedLaunch"
                  type="date"
                  value-format="YYYY-MM-DD"
                  placeholder="请选择日期" /></label
              ><label
                data-validation-key="budget"
                :class="validationClass('budget')"
                ><span class="ip-field-label"
                  >授权费用预算
                  <span class="ip-required" aria-hidden="true">*</span></span
                >
                <div class="ip-budget">
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
                    placeholder="最低预算"
                  /><span>—</span
                  ><el-input-number
                    v-model="form.budgetMax"
                    :min="0"
                    :controls="false"
                    placeholder="最高预算"
                  />
                </div>
                <small>预计授权费用，不含执行费用</small></label
              ><label
                data-validation-key="goal"
                :class="validationClass('goal')"
                ><span class="ip-field-label"
                  >合作目标
                  <span class="ip-required" aria-hidden="true">*</span></span
                ><el-select v-model="form.goal" placeholder="请选择"
                  ><el-option
                    v-for="goal in goals"
                    :key="goal"
                    :label="goal"
                    :value="goal" /></el-select></label
              ><label
                class="ip-span"
                data-validation-key="description"
                :class="validationClass('description')"
                ><span class="ip-field-label"
                  >项目背景与需求说明
                  <span class="ip-required" aria-hidden="true">*</span></span
                ><el-input
                  v-model="form.description"
                  type="textarea"
                  :rows="3"
                  maxlength="2000"
                  show-word-limit
                  placeholder="说明合作背景、希望达成的目标与执行方向"
              /></label>
            </div>
          </div>
          <div
            class="ip-card"
            data-validation-key="ip-intentions"
            :class="validationClass('ip-intentions')"
          >
            <div class="ip-card-head">
              <div>
                <h2>初步意向IP</h2>
                <p>
                  {{
                    candidateSelectionLocked
                      ? "IP组已开始反馈，意向IP名单和原始评估资料保持不变，可继续编辑其他项目资料。"
                      : "最多5个意向IP；可前往资源库查看完整档案、版权资料及历史合作。"
                  }}
                </p>
              </div>
              <a
                v-if="!candidateSelectionLocked"
                class="ip-outline-link"
                :href="libraryHref"
                target="_blank"
                rel="noopener"
                >前往IP资源库 ↗</a
              >
              <a
                v-if="!candidateSelectionLocked"
                class="ip-outline-link"
                :href="recommendationHref"
                target="_blank"
                rel="noopener"
                @click="openRecommendationsFromForm"
                >AI寻找匹配IP ↗</a
              >
            </div>
            <el-select
              v-model="form.candidateIds"
              :disabled="candidateSelectionLocked"
              multiple
              filterable
              placeholder="搜索并选择资源库中的IP"
              style="width: 100%"
              :multiple-limit="5"
              ><el-option
                v-for="item in resourceOptions"
                :key="item.id"
                :label="item.name"
                :value="Number(item.id)"
            /></el-select>
            <div v-if="form.candidateIds.length" class="ip-selected">
              <div
                v-for="id in form.candidateIds"
                :key="id"
                class="ip-selected-item"
                :data-validation-key="`candidate-${id}`"
                :class="validationClass(`candidate-${id}`)"
              >
                <div class="ip-selected-main">
                  <img
                    v-if="ipImageUrls[id]"
                    :src="ipImageUrls[id]"
                    :alt="selectedIP(id)?.name"
                  />
                  <div>
                    <strong>{{ selectedIP(id)?.name || "IP" }}</strong
                    ><small
                      >{{ selectedIP(id)?.ipType }} ·
                      {{
                        arrayValue(selectedIP(id)?.markets).join(" / ")
                      }}</small
                    >
                  </div>
                  <el-button
                    v-if="!candidateSelectionLocked"
                    link
                    @click="removeCandidate(id)"
                    >移除</el-button
                  >
                </div>
                <a
                  class="ip-text-link"
                  :href="ipDetailHref(id)"
                  target="_blank"
                  rel="noopener"
                  >查看IP详情 ↗</a
                >
                <div class="ip-form-grid ip-span">
                  <label
                    :data-validation-key="`candidate-${id}-reason`"
                    :class="validationClass(`candidate-${id}-reason`)"
                    ><span class="ip-field-label"
                      >选择理由
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input
                      v-model="form.brief.candidateEvaluations[id].reason"
                      :disabled="candidateSelectionLocked"
                      placeholder="商业潜力、品牌调性或话题契合"
                  /></label>
                  <label
                    :data-validation-key="`candidate-${id}-productEvaluation`"
                    :class="
                      validationClass(`candidate-${id}-productEvaluation`)
                    "
                    ><span class="ip-field-label"
                      >产品维度评估
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input
                      v-model="
                        form.brief.candidateEvaluations[id].productEvaluation
                      "
                      :disabled="candidateSelectionLocked"
                      placeholder="认知度、用户重叠率、机型适配"
                  /></label>
                  <label
                    class="ip-span"
                    :data-validation-key="`candidate-${id}-technicalFit`"
                    :class="validationClass(`candidate-${id}-technicalFit`)"
                    ><span class="ip-field-label"
                      >技术匹配程度
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input
                      v-model="form.brief.candidateEvaluations[id].technicalFit"
                      :disabled="candidateSelectionLocked"
                      placeholder="视觉惊喜度、开发及本地化难度"
                  /></label>
                </div>
              </div>
            </div>
            <div class="ip-other">
              <div class="ip-card-head">
                <strong>库外IP意向（最多3个，选填）</strong>
                <el-button
                  :disabled="form.brief.externalIPs.length >= 3"
                  @click="
                    form.brief.externalIPs.push({
                      name: '',
                      rightsOwner: '',
                      imageUrl: '',
                      reason: '',
                      productEvaluation: '',
                      technicalFit: ''
                    })
                  "
                  >＋ 添加</el-button
                >
              </div>
              <div
                v-for="(item, index) in form.brief.externalIPs"
                :key="index"
                class="ip-candidate"
              >
                <div class="ip-card-head">
                  <strong>库外IP {{ Number(index) + 1 }}</strong
                  ><el-button
                    link
                    type="danger"
                    @click="removeExternalIP(Number(index))"
                    >移除</el-button
                  >
                </div>
                <div class="ip-form-grid">
                  <label
                    :data-validation-key="`external-${index}-name`"
                    :class="validationClass(`external-${index}-name`)"
                    ><span class="ip-field-label"
                      >IP名称
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input v-model="item.name"
                  /></label>
                  <label
                    :data-validation-key="`external-${index}-rightsOwner`"
                    :class="validationClass(`external-${index}-rightsOwner`)"
                    ><span class="ip-field-label"
                      >版权方
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input v-model="item.rightsOwner"
                  /></label>
                  <label
                    class="ip-span"
                    :data-validation-key="`external-${index}-imageUrl`"
                    :class="validationClass(`external-${index}-imageUrl`)"
                    ><span class="ip-field-label"
                      >IP形象示意链接
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input
                      v-model="item.imageUrl"
                      placeholder="https://..."
                      @input="brokenExternalImageUrls[index] = false"
                    /><small
                      >可填图片直链或介绍页面链接；页面链接无法在此预览。</small
                    ></label
                  >
                  <img
                    v-if="
                      imageURLValid(item.imageUrl) &&
                      !brokenExternalImageUrls[index]
                    "
                    :src="item.imageUrl"
                    alt="库外IP形象示意"
                    class="ip-external-image"
                    @error="brokenExternalImageUrls[index] = true"
                  />
                  <div
                    v-if="
                      imageURLValid(item.imageUrl) &&
                      brokenExternalImageUrls[index]
                    "
                    class="ip-external-link-state ip-span"
                  >
                    链接已填写，当前无法直接预览图片。
                    <a :href="item.imageUrl" target="_blank" rel="noopener"
                      >打开链接 ↗</a
                    >
                  </div>
                  <label
                    class="ip-span"
                    :data-validation-key="`external-${index}-reason`"
                    :class="validationClass(`external-${index}-reason`)"
                    ><span class="ip-field-label"
                      >选择理由
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input v-model="item.reason"
                  /></label>
                  <label
                    class="ip-span"
                    :data-validation-key="`external-${index}-productEvaluation`"
                    :class="
                      validationClass(`external-${index}-productEvaluation`)
                    "
                    ><span class="ip-field-label"
                      >产品维度评估
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input
                      v-model="item.productEvaluation"
                      placeholder="市场认知、用户重叠、机型适配"
                  /></label>
                  <label
                    class="ip-span"
                    :data-validation-key="`external-${index}-technicalFit`"
                    :class="validationClass(`external-${index}-technicalFit`)"
                    ><span class="ip-field-label"
                      >技术匹配程度
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input
                      v-model="item.technicalFit"
                      placeholder="视觉、开发和本地化难度"
                  /></label>
                </div>
              </div>
            </div>
          </div>
          <div class="ip-card ip-optional-brief">
            <el-collapse v-model="briefOpen">
              <el-collapse-item name="brief">
                <template #title>
                  <div>
                    <h2>补充项目资料</h2>
                    <p>
                      带
                      <span class="ip-required" aria-hidden="true">*</span>
                      的项目资料与各环节对接人提交时必填。
                    </p>
                  </div>
                </template>
                <div class="ip-form-grid">
                  <label
                    v-for="field in briefFields"
                    :key="field.key"
                    class="ip-span"
                    :data-validation-key="`brief-${field.key}`"
                    :class="validationClass(`brief-${field.key}`)"
                  >
                    <span class="ip-field-label"
                      >{{ field.label
                      }}<span
                        v-if="field.required"
                        class="ip-required"
                        aria-hidden="true"
                        >*</span
                      ></span
                    ><el-input
                      v-model="form.brief[field.key]"
                      :placeholder="field.hint"
                    />
                  </label>
                </div>
                <div class="ip-stage-head">
                  <h3>各环节对接人</h3>
                  <p>输入姓名或工号搜索同事；未找到时也可直接填写姓名。</p>
                  <p>
                    选择 SSO
                    同事且有可用邮箱时，流程推进会发送飞书提醒；手动输入姓名可保存，但不会自动提醒。
                  </p>
                  <p
                    v-if="
                      form.brief.contacts &&
                      !contactStages.some(
                        stage => form.brief.contactsByStage?.[stage.key]?.name
                      )
                    "
                    class="ip-legacy-contacts"
                  >
                    历史草稿对接人：{{
                      form.brief.contacts
                    }}。请按环节补全后提交。
                  </p>
                </div>
                <div class="ip-contact-grid">
                  <label
                    v-for="stage in contactStages"
                    :key="stage.key"
                    :data-validation-key="`contact-${stage.key}`"
                    :class="validationClass(`contact-${stage.key}`)"
                  >
                    <span class="ip-field-label"
                      >{{ stage.label }}
                      <span class="ip-required" aria-hidden="true"
                        >*</span
                      ></span
                    >
                    <el-autocomplete
                      v-model="form.brief.contactsByStage[stage.key].name"
                      :fetch-suggestions="
                        (query, callback) =>
                          fetchColleagueSuggestions(stage.key, query, callback)
                      "
                      :debounce="300"
                      :trigger-on-focus="false"
                      clearable
                      placeholder="搜索姓名或工号"
                      @input="value => onContactInput(stage.key, value)"
                      @select="person => selectContact(stage.key, person)"
                    >
                      <template #default="{ item }">
                        <span>{{ item.name }}</span>
                        <small v-if="item.employeeNo || item.department">
                          {{
                            [item.employeeNo, item.department]
                              .filter(Boolean)
                              .join(" · ")
                          }}
                        </small>
                      </template>
                    </el-autocomplete>
                    <el-button
                      link
                      type="primary"
                      class="ip-colleague-more"
                      @click="openColleaguePicker(stage.key)"
                      >搜索更多同事</el-button
                    >
                    <small
                      v-if="colleagueSearchStatus[stage.key] === 'empty'"
                      class="ip-colleague-hint"
                      >未找到同事，可直接保留输入的姓名。</small
                    >
                    <small
                      v-if="colleagueSearchStatus[stage.key] === 'failed'"
                      class="ip-colleague-hint"
                      >搜索暂不可用，可直接填写姓名。</small
                    >
                    <small
                      v-if="colleagueSearchStatus[stage.key] === 'limited'"
                      class="ip-colleague-hint"
                      >企业目录暂不可用，仅显示已登录 XMP 的同事。</small
                    >
                  </label>
                </div>
              </el-collapse-item>
            </el-collapse>
          </div>
        </div>
        <div class="ip-stack">
          <div class="ip-card">
            <h2>需求摘要</h2>
            <dl>
              <dt>发起部门</dt>
              <dd>{{ form.department || "待填写" }}</dd>
              <dt>市场</dt>
              <dd>{{ form.markets.join(" / ") || "待选择" }}</dd>
              <dt>目标</dt>
              <dd>{{ form.goal || "待选择" }}</dd>
              <dt>授权预算</dt>
              <dd>{{ priceText(form) }}</dd>
              <dt>意向IP</dt>
              <dd>{{ form.candidateIds.length }} 个</dd>
            </dl>
            <div class="ip-note">
              提交后，IP意见需在10个工作日内反馈；IP意见提交后，营销意见需在5个工作日内补齐。按中国大陆法定节假日与调休、次日起算，截止日北京时间23:59。
            </div>
          </div>
        </div>
      </div>
      <div class="ip-bottom">
        <el-button
          v-if="!editingSubmitted"
          :disabled="saving"
          @click="save(false)"
          >保存草稿</el-button
        ><el-button class="ip-primary" :loading="saving" @click="save(true)">{{
          editingSubmitted ? "保存修改" : "提交需求 →"
        }}</el-button>
      </div></template
    >

    <template v-else-if="mode === 'detail' && detail"
      ><div class="ip-card ip-request-detail">
        <div class="ip-detail-title">
          <div>
            <h2>{{ detail.request.projectName }}</h2>
            <p>
              {{ detail.request.department }} ·
              {{ arrayValue(detail.request.markets).join(" / ") }} ·
              {{ requestDate(detail.request.createdAt, true) }}
            </p>
          </div>
          <el-tag
            :type="
              detail.request.status === 'marketing_reviewed'
                ? 'success'
                : 'warning'
            "
            >{{ statusText[detail.request.status] }}</el-tag
          >
        </div>
        <div class="ip-timeline ip-detail-timeline" aria-label="需求协作流程">
          <span :class="{ done: detail.request.status !== 'draft' }"
            ><b>1</b>需求已提交</span
          ><span
            :class="{
              done: ['ip_reviewed', 'marketing_reviewed'].includes(
                detail.request.status
              )
            }"
            ><b>2</b>IP组评估</span
          ><span
            :class="{ done: detail.request.status === 'marketing_reviewed' }"
            ><b>3</b>营销意见补充</span
          ><span><b>4</b>意向IP确认</span>
        </div>
        <section class="ip-deadline-summary" aria-label="意见反馈时效">
          <DeadlineStatus :deadline="detail.request.deadlines?.ip" />
          <DeadlineStatus :deadline="detail.request.deadlines?.marketing" />
          <p>
            按中国大陆法定节假日与调休，阶段开始次日起算；截止时间为北京时间。草稿保存不重置期限。
          </p>
        </section>
        <div class="ip-detail-summary">
          <div>
            <span>合作目标</span>
            <strong>{{ detailText(detail.request.goal, "待确定") }}</strong>
          </div>
          <div>
            <span>授权预算</span>
            <strong>{{ priceText(detail.request) }}</strong>
          </div>
          <div>
            <span>预计上线</span>
            <strong>{{ requestDate(detail.request.expectedLaunch) }}</strong>
          </div>
        </div>
        <div class="ip-detail-body">
          <div class="ip-detail-main">
            <section
              class="ip-detail-section"
              aria-labelledby="ip-project-info"
            >
              <h3 id="ip-project-info">项目资料</h3>
              <dl class="ip-detail-facts">
                <div
                  v-for="field in projectFields"
                  :key="field.key"
                  :class="{ 'ip-detail-wide': field.key === 'cooperationMode' }"
                >
                  <dt>{{ field.label }}</dt>
                  <dd>{{ detailText(requestBrief[field.key]) }}</dd>
                </div>
              </dl>
            </section>
            <section
              class="ip-detail-section"
              aria-labelledby="ip-request-description"
            >
              <h3 id="ip-request-description">项目背景与需求说明</h3>
              <RequestText :text="detail.request.description" />
            </section>
            <section
              class="ip-detail-section"
              aria-labelledby="ip-project-notes"
            >
              <h3 id="ip-project-notes">项目补充说明</h3>
              <RequestText :text="detailText(requestBrief.projectNotes)" />
            </section>
            <section
              v-if="detail.request.externalRecommendation || externalIPs.length"
              class="ip-detail-section"
              aria-labelledby="ip-external-recommendations"
            >
              <h3 id="ip-external-recommendations">库外IP推荐</h3>
              <RequestText
                v-if="detail.request.externalRecommendation"
                :text="detail.request.externalRecommendation"
              />
              <div
                v-for="(item, index) in externalIPs"
                :key="index"
                class="ip-detail-external"
              >
                <div class="ip-detail-external-head">
                  <strong>{{ detailText(item.name) }}</strong>
                  <el-button link @click="openExternalIP(item)"
                    >补充并入库 ↗</el-button
                  >
                </div>
                <p>版权方：{{ detailText(item.rightsOwner) }}</p>
                <RequestText :text="item.reason" />
              </div>
            </section>
          </div>
          <section
            class="ip-detail-contacts"
            aria-labelledby="ip-project-contacts"
          >
            <h3 id="ip-project-contacts">协作对接人</h3>
            <dl class="ip-detail-contact-list">
              <div v-for="stage in contactStages" :key="stage.key">
                <dt>{{ stage.label }}</dt>
                <dd>{{ detailText(requestContacts[stage.key]?.name) }}</dd>
              </div>
              <div
                v-if="
                  !Object.keys(objectValue(requestBrief.contactsByStage))
                    .length && requestBrief.contacts
                "
                class="ip-detail-wide"
              >
                <dt>原对接人</dt>
                <dd>{{ requestBrief.contacts }}</dd>
              </div>
            </dl>
          </section>
        </div>
        <div class="ip-detail-actions">
          <el-button
            v-if="menuKind === 'requests' && detail.request.canEdit"
            @click="goEdit(Number(detail.request.id))"
            >{{
              detail.request.status === "draft" ? "继续编辑草稿" : "编辑需求"
            }}</el-button
          >
          <el-button
            v-if="menuKind === 'requests' && detail.request.canDelete"
            type="danger"
            plain
            :loading="deletingID === Number(detail.request.id)"
            @click="deleteRequest(detail.request)"
            >删除需求</el-button
          >
          <el-button @click="openRecommendationsFromDetail"
            >根据此需求寻找匹配IP ↗</el-button
          >
        </div>
      </div>
      <div class="ip-grid">
        <div class="ip-stack">
          <div v-if="menuKind !== 'marketing'" class="ip-card">
            <h2>IP组评估</h2>
            <p>请根据评估结果填写推荐意见。评分各项均为百分制。</p>
            <p v-if="!feedback.length">
              暂无资源库内的意向IP。可先在资源库新建，再在下方加入评估。
            </p>
            <div v-if="canFeedback" class="ip-add-candidate">
              <el-select
                v-model="addCandidateID"
                filterable
                placeholder="从IP资源库补充评估对象"
                ><el-option
                  v-for="item in resourceOptions.filter(
                    row =>
                      !feedback.some(
                        candidate => candidate.ipId === Number(row.id)
                      )
                  )"
                  :key="item.id"
                  :label="item.name"
                  :value="Number(item.id)" /></el-select
              ><el-button @click="addFeedbackCandidate">加入评估</el-button
              ><a
                class="ip-outline-link"
                :href="libraryHref"
                target="_blank"
                rel="noopener"
                >前往IP资源库 ↗</a
              >
              <el-button @click="loadResources">刷新IP列表</el-button>
            </div>
            <div
              v-for="(item, index) in feedback"
              :key="item.ipId"
              class="ip-candidate"
            >
              <div class="ip-card-head">
                <div class="ip-candidate-identity">
                  <img
                    v-if="ipImageUrls[item.ipId]"
                    :src="ipImageUrls[item.ipId]"
                    :alt="item.name"
                  />
                  <div>
                    <strong>{{ item.name }}</strong
                    ><small>意向IP {{ index + 1 }}</small>
                  </div>
                </div>
                <strong class="ip-score"
                  >{{ weightedScore(item.assessment) ?? "—" }} / 100</strong
                >
                <a
                  class="ip-text-link"
                  :href="ipDetailHref(item.ipId)"
                  target="_blank"
                  rel="noopener"
                  >查看IP详情 ↗</a
                >
                <el-button link @click="exportEvaluation(item)"
                  >导出完整评估表</el-button
                >
              </div>
              <div class="ip-form-grid">
                <label
                  ><span class="ip-field-label"
                    >可行性判断
                    <span class="ip-required" aria-hidden="true">*</span></span
                  ><el-select
                    v-model="item.feasibility"
                    :disabled="!canFeedback"
                    ><el-option label="可行" value="可行" /><el-option
                      label="需进一步评估"
                      value="需进一步评估" /><el-option
                      label="不可行"
                      value="不可行" /></el-select></label
                ><label
                  ><span class="ip-field-label"
                    >推荐意见
                    <span class="ip-required" aria-hidden="true">*</span></span
                  ><el-select
                    v-model="item.recommendation"
                    :disabled="!canFeedback"
                    ><el-option label="优先合作" value="优先合作" /><el-option
                      label="备选合作"
                      value="备选合作" /><el-option
                      label="暂不合作"
                      value="暂不合作" /></el-select></label
                ><label
                  ><span class="ip-field-label"
                    >优先级
                    <span class="ip-required" aria-hidden="true">*</span></span
                  ><el-select
                    v-model="item.priorityOrder"
                    :disabled="!canFeedback"
                    ><el-option label="T0 · 最高优先级" :value="0" /><el-option
                      label="T1 · 优先推进"
                      :value="1" /><el-option
                      label="T2 · 后续考虑"
                      :value="2" /></el-select></label
                ><label class="ip-span"
                  ><span class="ip-field-label"
                    >理由说明
                    <span class="ip-required" aria-hidden="true">*</span></span
                  ><el-input
                    v-model="item.reason"
                    type="textarea"
                    :rows="2"
                    :disabled="!canFeedback"
                    placeholder="受众匹配、市场覆盖、档期和授权风险"
                /></label>
              </div>
              <el-collapse class="ip-assessment-collapse">
                <el-collapse-item
                  title="评分与授权明细（各项均为百分制）"
                  :name="item.ipId"
                >
                  <div class="ip-score-grid">
                    <label v-for="field in scoreFields" :key="field.key">
                      <span class="ip-field-label"
                        >{{ field.label }}
                        <span class="ip-required" aria-hidden="true">*</span> ·
                        权重 {{ Math.round(field.weight * 100) }}% </span
                      ><el-input-number
                        v-model="item.assessment.scoreDimensions[field.key]"
                        :min="0"
                        :max="100"
                        :precision="0"
                        :disabled="!canFeedback"
                      />
                    </label>
                  </div>
                  <div class="ip-form-grid">
                    <label
                      v-for="field in assessmentFields"
                      :key="field.key"
                      class="ip-span"
                    >
                      <span class="ip-field-label"
                        >{{ field.label
                        }}<span
                          v-if="requiredAssessmentFields.has(field.key)"
                          class="ip-required"
                          aria-hidden="true"
                          >*</span
                        ></span
                      >
                      <el-input
                        v-model="item.assessment[field.key]"
                        :disabled="!canFeedback"
                        :placeholder="field.hint"
                      />
                    </label>
                  </div>
                </el-collapse-item>
              </el-collapse>
            </div>
            <div v-if="canFeedback" class="ip-bottom">
              <el-button :loading="saving" @click="sendFeedback(true)"
                >保存草稿</el-button
              >
              <el-button
                class="ip-primary"
                :loading="saving"
                @click="sendFeedback(false)"
                >提交IP反馈</el-button
              >
            </div>
          </div>
          <div v-if="menuKind !== 'feedback'" class="ip-card">
            <div class="ip-marketing-heading">
              <h2>营销评估与建议</h2>
              <span
                >已提交 {{ submittedMarketingCount }} /
                {{ marketingFunctions.length }} 个职能</span
              >
            </div>
            <p class="ip-marketing-guidance">
              各职能独立填写、独立提交，只需完成自己负责的意见。全部职能提交后，通过飞书提醒需求发起人；营销环节共用
              5 个工作日反馈期限。
            </p>
            <template
              v-if="
                canMarketing || detail.request.status === 'marketing_reviewed'
              "
            >
              <el-radio-group
                v-model="marketingFunctionKey"
                aria-label="本次填写或查看的职能"
                :disabled="saving"
                class="ip-marketing-functions"
              >
                <el-radio-button
                  v-for="item in marketingFunctions"
                  :key="item.key"
                  :value="item.key"
                >
                  {{ item.label }} ·
                  {{
                    marketingFunctionSubmitted(item.key) ? "已提交" : "待提交"
                  }}
                </el-radio-button>
              </el-radio-group>
              <template v-if="selectedMarketingFunction">
                <div class="ip-marketing-function-meta">
                  <strong>{{ selectedMarketingFunction.label }}意见</strong>
                  <span
                    >对接人：{{
                      requestContacts[marketingFunctionKey]?.name || "待补充"
                    }}</span
                  >
                  <span
                    v-if="
                      objectValue(marketingFunctionStates[marketingFunctionKey])
                        .submittedAt
                    "
                    >已于
                    {{
                      requestDate(
                        objectValue(
                          marketingFunctionStates[marketingFunctionKey]
                        ).submittedAt,
                        true
                      )
                    }}
                    提交</span
                  >
                </div>
                <p
                  v-if="marketingFunctionSubmitted(marketingFunctionKey)"
                  class="ip-marketing-guidance"
                >
                  本职能已提交，以下为已保存内容。
                </p>
                <div
                  v-if="marketingFunctionKey === 'imc'"
                  class="ip-marketing-grid"
                >
                  <label
                    v-for="field in [
                      { key: 'marketHeat', label: '项目整体市场热度' },
                      { key: 'fanAudience', label: '项目整体粉丝受众' },
                      { key: 'commercialValue', label: '项目整体商业价值' },
                      { key: 'marketingRisks', label: '项目整体风险与提醒' }
                    ]"
                    :key="field.key"
                    class="ip-channel-row"
                  >
                    <strong>{{ field.label }}（选填）</strong>
                    <el-input
                      v-model="marketing[field.key]"
                      :disabled="!canEditMarketingFunction || saving"
                    />
                  </label>
                </div>
                <div
                  v-for="candidate in detail.candidates"
                  :key="candidate.ipId"
                  class="ip-candidate"
                >
                  <h3>
                    {{ candidate.name }} ·
                    {{ selectedMarketingFunction.label }}意见
                  </h3>
                  <div class="ip-form-grid">
                    <label
                      v-for="field in selectedMarketingFields"
                      :key="field.key"
                      class="ip-span"
                    >
                      <span class="ip-field-label"
                        >{{ field.label
                        }}<span
                          v-if="requiredMarketingFields.has(field.key)"
                          class="ip-required"
                          aria-hidden="true"
                          >*</span
                        ></span
                      >
                      <el-input
                        v-model="
                          marketingCandidates[String(candidate.ipId)][field.key]
                        "
                        :disabled="!canEditMarketingFunction || saving"
                        :placeholder="field.hint"
                        :aria-required="requiredMarketingFields.has(field.key)"
                      />
                    </label>
                  </div>
                </div>
                <div class="ip-marketing-grid">
                  <label class="ip-channel-row">
                    <strong
                      >{{
                        selectedMarketingFunction.label
                      }}推荐传播方向（选填）</strong
                    >
                    <el-checkbox-group
                      v-model="
                        marketingFunctionDrafts[marketingFunctionKey].channels
                      "
                      :disabled="!canEditMarketingFunction || saving"
                    >
                      <el-checkbox
                        v-for="channel in channels"
                        :key="channel"
                        :value="channel"
                        >{{ channel }}</el-checkbox
                      >
                    </el-checkbox-group>
                  </label>
                  <label class="ip-channel-row">
                    <span class="ip-field-label"
                      ><strong
                        >{{ selectedMarketingFunction.label }}补充意见<span
                          class="ip-required"
                          aria-hidden="true"
                          >*</span
                        ></strong
                      ></span
                    >
                    <el-input
                      v-model="
                        marketingFunctionDrafts[marketingFunctionKey].comments
                      "
                      type="textarea"
                      :rows="3"
                      :disabled="!canEditMarketingFunction || saving"
                      :placeholder="`填写${selectedMarketingFunction.label}意见；无需填写其他职能内容`"
                      aria-required="true"
                    />
                  </label>
                </div>
                <div v-if="canEditMarketingFunction" class="ip-bottom">
                  <el-button :loading="saving" @click="sendMarketing(true)"
                    >保存本职能草稿</el-button
                  >
                  <el-button
                    class="ip-primary"
                    :loading="saving"
                    @click="sendMarketing(false)"
                    >提交{{ selectedMarketingFunction.label }}意见</el-button
                  >
                </div>
              </template>
              <p v-else class="ip-marketing-guidance">
                请先选择你负责的职能，查看或填写对应意见。
              </p>
              <section
                v-if="legacyMarketingComments"
                class="ip-marketing-legacy"
              >
                <h3>历史营销补充意见</h3>
                <RequestText :text="String(legacyMarketingComments)" />
              </section>
            </template>
            <p v-else>待IP组提交初步反馈后，由各职能伙伴补充意见。</p>
          </div>
        </div>
        <div class="ip-stack">
          <div v-if="menuKind !== 'marketing'" class="ip-card">
            <h2>评估依据</h2>
            <p>基于以下维度进行综合评估，供参考。</p>
            <div
              v-for="field in scoreFields"
              :key="field.key"
              class="ip-weight-row"
            >
              <span>{{ field.label }}</span>
              <strong>{{ Math.round(field.weight * 100) }}%</strong>
            </div>
            <div class="ip-note">
              当前评分是初步判断，需结合后续沟通进一步确认。
            </div>
          </div>
          <div v-if="menuKind === 'marketing'" class="ip-card">
            <h2>IP组反馈摘要</h2>
            <div
              v-for="item in feedback"
              :key="item.ipId"
              class="ip-feedback-summary"
            >
              <img
                v-if="ipImageUrls[item.ipId]"
                :src="ipImageUrls[item.ipId]"
                :alt="item.name"
              />
              <div>
                <strong>{{ item.name }}</strong>
                <small
                  >{{ item.recommendation }} · 优先级 T{{
                    item.priorityOrder
                  }}</small
                >
                <p>{{ item.reason }}</p>
              </div>
            </div>
          </div>
          <div class="ip-card">
            <h2>协作记录</h2>
            <div class="ip-record">
              <b>1</b
              ><span
                >需求已提交<small>{{
                  requestDate(detail.request.createdAt, true)
                }}</small></span
              >
            </div>
            <div class="ip-record">
              <b>2</b
              ><span
                >IP组评估<small>{{
                  detail.request.status === "submitted" ? "待进行" : "已完成"
                }}</small></span
              >
            </div>
            <div class="ip-record">
              <b>3</b
              ><span
                >营销意见补充<small>{{
                  detail.request.status === "marketing_reviewed"
                    ? "已完成"
                    : "待进行"
                }}</small></span
              >
            </div>
            <div class="ip-record">
              <b>4</b><span>意向IP确认<small>待进行</small></span>
            </div>
          </div>
        </div>
      </div>
    </template>
    <el-dialog
      v-model="colleaguePickerOpen"
      title="搜索同事"
      width="640px"
      @closed="colleaguePickerRequest++"
    >
      <div class="ip-colleague-picker-search">
        <el-input
          v-model="colleaguePickerQuery"
          clearable
          placeholder="输入姓名或工号"
          @input="onColleaguePickerQueryInput"
          @keyup.enter="loadColleaguePickerPage(1)"
        />
        <el-button type="primary" @click="loadColleaguePickerPage(1)"
          >搜索</el-button
        >
      </div>
      <p v-if="colleaguePickerWarning" class="ip-colleague-hint">
        {{ colleaguePickerWarning }}
      </p>
      <div
        v-loading="colleaguePickerLoading"
        class="ip-colleague-picker-results"
      >
        <button
          v-for="person in colleaguePickerRows"
          :key="String(person.id)"
          type="button"
          class="ip-colleague-picker-person"
          @click="chooseColleagueFromPicker(person)"
        >
          <strong>{{ person.name }}</strong>
          <span>{{
            [person.employeeNo, person.department].filter(Boolean).join(" · ")
          }}</span>
        </button>
        <p v-if="!colleaguePickerLoading && !colleaguePickerRows.length">
          {{
            colleaguePickerQuery.trim()
              ? "暂无匹配同事"
              : "输入姓名或工号开始搜索"
          }}
        </p>
      </div>
      <div class="ip-colleague-picker-pages">
        <el-button
          :disabled="colleaguePickerLoading || colleaguePickerPage <= 1"
          @click="loadColleaguePickerPage(colleaguePickerPage - 1)"
          >上一页</el-button
        >
        <span>第 {{ colleaguePickerPage }} 页</span>
        <el-button
          :disabled="colleaguePickerLoading || !colleaguePickerHasNext"
          @click="loadColleaguePickerPage(colleaguePickerPage + 1)"
          >下一页</el-button
        >
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.ip-request-page {
  min-height: calc(100vh - 150px);
  padding: 16px 28px 36px;
  color: #161a1d;
  background: #f8f8f6;
}

.ip-header,
.ip-card-head,
.ip-detail-title,
.ip-search,
.ip-bottom,
.ip-add-candidate {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.ip-header {
  margin-bottom: 14px;
}

.ip-header h1 {
  margin: 5px 0;
  font-size: 30px;
}

.ip-header p,
.ip-card p {
  margin: 4px 0;
  color: #777e86;
}

.ip-kicker {
  margin-bottom: 8px;
  font-size: 12px;
  color: #858d92;
}

.ip-primary {
  font-weight: 700;
  color: #111 !important;
  background: #caff00 !important;
  border-color: #caff00 !important;
}

.ip-primary:hover {
  background: #b7eb00 !important;
}

.ip-card {
  padding: 18px;
  margin-bottom: 16px;
  background: #fff;
  border: 1px solid #dedfdb;
  border-radius: 9px;
  box-shadow: 0 2px 10px #20240b06;
}

.ip-card h2 {
  margin: 0 0 11px;
  font-size: 18px;
}

.ip-search {
  justify-content: flex-start;
  margin-bottom: 18px;
}

.ip-search .el-input {
  max-width: 450px;
}

.ip-search span {
  margin-left: auto;
  color: #90969b;
}

.ip-link {
  padding: 0;
  font-weight: 700;
  color: #17191d;
  cursor: pointer;
  background: none;
  border: 0;
}

.ip-link:hover {
  color: #698000;
}

.ip-link + small,
.ip-selected-item small,
.ip-candidate small {
  display: block;
  margin-top: 4px;
  color: #9b9fa4;
}

.ip-pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

.ip-row-actions {
  display: inline-flex;
  gap: 14px;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
}

.ip-request-table :deep(.ip-deadline),
.ip-request-table :deep(.deadline-label) {
  align-items: center;
  justify-content: center;
}

.ip-request-table :deep(.cell) {
  overflow-wrap: anywhere;
}

.ip-steps,
.ip-timeline {
  display: flex;
  gap: 12px;
  justify-content: space-between;
  padding: 10px 20px;
  margin-bottom: 12px;
  background: white;
  border: 1px solid #e2e3dc;
  border-radius: 9px;
}

.ip-steps span,
.ip-timeline span {
  display: inline-flex;
  gap: 10px;
  align-items: center;
  font-weight: 600;
  color: #a3a8a8;
}

.ip-steps b,
.ip-timeline b {
  display: inline-grid;
  place-items: center;
  width: 28px;
  height: 28px;
  color: #4e5356;
  background: #eceeed;
  border-radius: 50%;
}

.ip-steps .active,
.ip-timeline .done {
  color: #252d17;
}

.ip-steps .active b,
.ip-timeline .done b {
  color: #172000;
  background: #caff00;
}

.ip-grid {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(280px, 1fr);
  gap: 16px;
}

.ip-stack {
  min-width: 0;
}

.ip-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 24px;
}

.ip-outline-link {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  height: 32px;
  padding: 0 12px;
  font-size: 14px;
  color: #242a31;
  text-decoration: none;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
}

.ip-outline-link:hover,
.ip-text-link:hover {
  color: #647e00;
  border-color: #a2c400;
}

.ip-text-link {
  font-size: 14px;
  font-weight: 600;
  color: #5b7600;
  text-decoration: none;
}

.ip-optional-brief :deep(.el-collapse),
.ip-optional-brief :deep(.el-collapse-item__header),
.ip-optional-brief :deep(.el-collapse-item__wrap) {
  border: 0;
}

.ip-optional-brief :deep(.el-collapse-item__header) {
  height: auto;
  min-height: 48px;
  line-height: 1.5;
}

.ip-optional-brief :deep(.el-collapse-item__content) {
  padding: 18px 0 0;
}

.ip-optional-brief h2 {
  margin: 0 0 4px;
}

.ip-optional-brief p {
  margin: 0;
  font-weight: 400;
  color: #8a9195;
}

.ip-form-grid label,
.ip-other,
.ip-contact-grid label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-weight: 600;
}

.ip-invalid-field {
  outline: 2px solid #f3a24c;
  background: #fff9f1;
  border-radius: 6px;
}

.ip-stage-head {
  margin-top: 22px;
  margin-bottom: 14px;
}

.ip-stage-head h3 {
  margin: 0 0 5px;
  font-size: 16px;
}

.ip-contact-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 24px;
}

.ip-contact-grid .el-autocomplete {
  width: 100%;
}

.ip-colleague-more {
  align-self: flex-start;
  margin: -3px 0 0;
}

.ip-colleague-picker-search,
.ip-colleague-picker-pages {
  display: flex;
  gap: 12px;
  align-items: center;
}

.ip-colleague-picker-results {
  min-height: 120px;
  max-height: 360px;
  margin: 16px 0;
  overflow: auto;
}

.ip-colleague-picker-person {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 12px;
  text-align: left;
  cursor: pointer;
  background: #fff;
  border: 0;
  border-bottom: 1px solid #edf0e9;
}

.ip-colleague-picker-person:hover {
  background: #f5f8e9;
}

.ip-colleague-picker-person span {
  font-size: 12px;
  color: #7e858b;
}

.ip-colleague-picker-pages {
  justify-content: flex-end;
}

.ip-legacy-contacts,
.ip-colleague-hint {
  display: block;
  color: #8b6300 !important;
}

.ip-external-link-state {
  font-size: 13px;
  font-weight: 400;
  color: #777e86;
}

.ip-external-link-state a {
  margin-left: 8px;
  color: #5b7600;
}

.ip-form-grid small,
.ip-other small {
  font-weight: 400;
  color: #969b9e;
}

.ip-span {
  grid-column: 1/-1;
}

.ip-budget {
  display: flex;
  gap: 8px;
  align-items: center;
}

.ip-budget .el-select {
  flex: none;
  width: 110px;
}

.ip-budget .el-input-number {
  width: 100%;
  min-width: 0;
}

.ip-selected {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 15px 0;
}

.ip-selected-item,
.ip-candidate {
  padding: 12px;
  border: 1px solid #e8e9e3;
  border-radius: 8px;
}

.ip-selected-item {
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: stretch;
}

.ip-selected-main {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
}

.ip-selected-main img,
.ip-candidate-identity img,
.ip-feedback-summary img {
  flex: none;
  width: 60px;
  height: 64px;
  object-fit: cover;
  border-radius: 6px;
}

.ip-selected-main > div {
  min-width: 0;
}

.ip-selected-main .el-button {
  margin-left: auto;
}

.ip-candidate-identity {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
}

.ip-candidate-identity strong {
  font-size: 16px;
}

.ip-external-image {
  max-width: 180px;
  max-height: 120px;
  object-fit: contain;
  border-radius: 6px;
}

.ip-other {
  margin-top: 18px;
}

.ip-bottom {
  justify-content: flex-end;
  margin-top: 16px;
}

.ip-card dl {
  display: grid;
  grid-template-columns: 100px 1fr;
  gap: 16px;
}

.ip-card dl dd {
  padding-bottom: 12px;
  border-bottom: 1px solid #edf0ec;
}

.ip-card dt {
  color: #858b91;
}

.ip-card dd {
  margin: 0;
  font-weight: 600;
}

.ip-note {
  padding: 14px;
  margin-top: 22px;
  color: #557000;
  background: #f2f8e8;
  border: 1px solid #e4edd2;
  border-radius: 8px;
}

.ip-detail-title h2 {
  margin-bottom: 4px;
  font-size: 22px;
}

.ip-detail-page .ip-header,
.ip-request-detail .ip-detail-title {
  flex-wrap: wrap;
  align-items: flex-start;
}

.ip-detail-page .ip-header > div:first-child,
.ip-request-detail .ip-detail-title > div {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}

.ip-detail-page .ip-header h1 {
  line-height: 1.35;
}

.ip-request-detail {
  padding: 24px;
}

.ip-request-detail .ip-detail-title {
  margin-bottom: 20px;
}

.ip-request-detail .ip-detail-title p {
  font-size: 13px;
  line-height: 1.7;
  color: #687078;
}

.ip-detail-title .el-tag {
  flex: none;
}

.ip-detail-timeline {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  padding: 16px;
  margin-bottom: 20px;
}

.ip-detail-timeline span {
  min-width: 0;
  font-size: 13px;
  color: #687078;
}

.ip-detail-timeline b {
  flex: none;
}

.ip-detail-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px 24px;
  padding: 16px 20px;
  margin-bottom: 24px;
  background: #f7f8f4;
  border-radius: 6px;
}

.ip-detail-summary > div {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.ip-detail-summary span {
  font-size: 12px;
  color: #687078;
}

.ip-detail-summary strong {
  font-size: 16px;
  font-variant-numeric: tabular-nums;
  line-height: 1.5;
  color: #15191c;
  overflow-wrap: anywhere;
}

.ip-detail-body {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(240px, 1fr);
  gap: 32px;
}

.ip-detail-main {
  min-width: 0;
}

.ip-request-detail h3 {
  margin: 0 0 16px;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.5;
}

.ip-detail-section + .ip-detail-section {
  padding-top: 24px;
  margin-top: 24px;
  border-top: 1px solid #eceee9;
}

dl.ip-detail-facts,
dl.ip-detail-contact-list {
  margin: 0;
}

dl.ip-detail-facts {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 24px;
}

.ip-detail-facts > div {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 12px;
  align-items: baseline;
}

.ip-detail-wide {
  grid-column: 1 / -1;
}

.ip-request-detail dt {
  font-size: 13px;
  line-height: 1.7;
  color: #687078;
}

.ip-request-detail dl dd {
  padding: 0;
  font-size: 14px;
  font-weight: 500;
  line-height: 1.7;
  color: #30363b;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  border: 0;
}

.ip-detail-contacts {
  min-width: 0;
  padding-left: 24px;
  border-left: 1px solid #eceee9;
}

dl.ip-detail-contact-list {
  grid-template-columns: 1fr;
  gap: 0;
}

.ip-detail-contact-list > div {
  display: grid;
  grid-template-columns: 88px minmax(0, 1fr);
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #eceee9;
}

.ip-detail-contact-list > div:first-child {
  padding-top: 0;
}

.ip-detail-contact-list > div:last-child {
  border-bottom: 0;
}

.ip-detail-external {
  padding-top: 16px;
  margin-top: 16px;
  border-top: 1px solid #eceee9;
}

.ip-detail-external-head {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  align-items: center;
  justify-content: space-between;
  overflow-wrap: anywhere;
}

.ip-detail-external > p {
  margin: 8px 0;
  font-size: 13px;
  color: #687078;
  overflow-wrap: anywhere;
}

.ip-detail-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  padding-top: 20px;
  margin-top: 24px;
  border-top: 1px solid #eceee9;
}

.ip-detail-actions .el-button {
  min-height: 36px;
  margin: 0;
}

.ip-request-detail ::selection {
  color: #172000;
  background: #e2f5a0;
}

.ip-candidate {
  margin-bottom: 14px;
}

.ip-candidate .ip-card-head {
  margin-bottom: 15px;
}

.ip-assessment-collapse {
  margin-top: 16px;
  border-top: 1px solid #e8ebe5;
}

.ip-assessment-collapse :deep(.el-collapse-item__header) {
  font-weight: 700;
  color: #647e00;
}

.ip-score {
  margin-left: auto;
  font-size: 18px;
  white-space: nowrap;
}

.ip-score-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  padding: 12px;
  margin-bottom: 16px;
  background: #f8faf4;
  border: 1px solid #e5e9dc;
  border-radius: 8px;
}

.ip-score-grid label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
  font-weight: 600;
}

.ip-score-grid .el-input-number {
  width: 100%;
}

.ip-add-candidate {
  justify-content: flex-start;
  margin-bottom: 16px;
}

.ip-add-candidate .el-select {
  width: 330px;
}

.ip-weight-row {
  display: flex;
  justify-content: space-between;
  padding: 14px 0;
  border-bottom: 1px solid #edf0ec;
}

.ip-weight-row strong {
  font-size: 17px;
}

.ip-marketing-grid {
  margin-bottom: 22px;
  border: 1px solid #e7e9e3;
  border-radius: 8px;
}

.ip-metric-row {
  display: grid;
  grid-template-columns: minmax(150px, 35%) minmax(0, 1fr);
  gap: 18px;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid #e7e9e3;
}

.ip-metric-row small {
  display: block;
  margin-top: 4px;
  font-weight: 400;
  color: #90969b;
}

.ip-metric-risk {
  background: #fffaf2;
}

.ip-channel-row {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border-bottom: 1px solid #e7e9e3;
}

.ip-channel-row:last-child {
  border-bottom: 0;
}

.ip-channel-row .el-checkbox-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.ip-channel-row :deep(.el-checkbox) {
  padding: 7px 10px;
  margin: 0;
  border: 1px solid #dfe4d7;
  border-radius: 6px;
}

.ip-feedback-summary {
  display: flex;
  gap: 12px;
  padding: 12px;
  margin-top: 10px;
  border: 1px solid #e8e9e3;
  border-radius: 8px;
}

.ip-feedback-summary small {
  display: block;
  margin: 4px 0;
  color: #748400;
}

.ip-feedback-summary p {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.ip-record {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 10px 0;
}

.ip-record b {
  display: grid;
  flex: none;
  place-items: center;
  width: 26px;
  height: 26px;
  font-size: 12px;
  color: #1d2b00;
  background: #d9f56c;
  border-radius: 50%;
}

.ip-record span {
  font-weight: 600;
}

.ip-record small {
  display: block;
  font-weight: 400;
  color: #90969b;
}

@media (width <= 1100px) {
  .ip-grid {
    grid-template-columns: 1fr;
  }

  .ip-score-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .ip-detail-body {
    grid-template-columns: 1fr;
    gap: 24px;
  }

  .ip-detail-contacts {
    padding: 24px 0 0;
    border-top: 1px solid #eceee9;
    border-left: 0;
  }

  dl.ip-detail-contact-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 24px;
  }

  .ip-detail-contact-list > div:first-child {
    padding-top: 12px;
  }

  .ip-detail-timeline {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (width <= 700px) {
  .ip-request-page {
    padding: 16px;
  }

  .ip-header,
  .ip-search,
  .ip-add-candidate {
    flex-wrap: wrap;
  }

  .ip-form-grid,
  .ip-contact-grid,
  .ip-selected,
  .ip-score-grid,
  .ip-metric-row {
    grid-template-columns: 1fr;
  }

  .ip-steps,
  .ip-timeline {
    flex-wrap: wrap;
    font-size: 12px;
  }

  .ip-request-detail {
    padding: 16px;
  }

  .ip-detail-summary {
    grid-template-columns: 1fr;
    gap: 12px;
    padding: 16px;
  }

  .ip-detail-summary > div {
    display: grid;
    grid-template-columns: 72px minmax(0, 1fr);
    gap: 12px;
    align-items: baseline;
  }

  .ip-detail-summary strong {
    font-size: 14px;
  }

  dl.ip-detail-facts,
  dl.ip-detail-contact-list {
    grid-template-columns: 1fr;
  }

  .ip-detail-facts > div {
    grid-template-columns: 88px minmax(0, 1fr);
  }

  .ip-detail-timeline {
    gap: 16px 8px;
    padding: 12px;
  }

  .ip-detail-timeline span {
    gap: 6px;
    font-size: 12px;
  }
}
</style>

<style scoped lang="scss">
.ip-deadline-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 32px;
  padding: 20px 0;
  margin-bottom: 8px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.ip-deadline-summary p {
  grid-column: 1 / -1;
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--el-text-color-regular);
}

@media (width <= 640px) {
  .ip-deadline-summary {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>

<style scoped lang="scss">
.ip-required {
  margin-left: 4px;
  color: #d03030;
}

.ip-marketing-heading,
.ip-marketing-function-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  align-items: baseline;
}

.ip-marketing-heading {
  justify-content: space-between;
}

.ip-marketing-heading span,
.ip-marketing-function-meta span,
.ip-marketing-guidance {
  font-size: 13px;
  line-height: 1.7;
  color: var(--el-text-color-regular);
}

.ip-marketing-functions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 16px 0 24px;
}

.ip-marketing-functions :deep(.el-radio-button__inner) {
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  box-shadow: none;
}

.ip-marketing-function-meta {
  margin-bottom: 16px;
}

.ip-marketing-legacy {
  margin-top: 24px;
}
</style>
