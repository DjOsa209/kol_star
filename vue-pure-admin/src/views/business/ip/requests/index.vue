<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import * as XLSX from "xlsx";
import {
  getIPResource,
  getIPRequest,
  listIPRequests,
  listIPResources,
  readIPFile,
  saveIPRequest,
  submitIPFeedback,
  submitIPMarketing,
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
const markets = [
  "中国",
  "东南亚",
  "欧洲",
  "中东",
  "非洲",
  "拉美",
  "北美",
  "全球"
];
const goals = ["新品发布", "品牌声量", "用户增长", "线下活动", "其他"];
const channels = ["线下快闪", "短视频共创", "新品联名", "社媒互动", "媒体投放"];
const briefFields = [
  { key: "owner", label: "项目负责人", hint: "填写姓名或团队" },
  { key: "projectPeriod", label: "项目周期", hint: "启动至结案的预计周期" },
  { key: "linkedProduct", label: "关联产品", hint: "例如 NOTE / GT / HOT" },
  {
    key: "targetIPType",
    label: "目标IP类型",
    hint: "例如体育赛事、游戏或动漫"
  },
  { key: "targetIPCount", label: "目标IP数量", hint: "预计需要评估的IP数量" },
  {
    key: "cooperationMode",
    label: "合作模式",
    hint: "整合联动 / 衍生品授权 / 营销联动"
  },
  {
    key: "contacts",
    label: "各环节对接人",
    hint: "产品、IP营销、IMC、品牌、用户、媒介、PR"
  },
  { key: "projectNotes", label: "项目补充说明", hint: "其他协作约束（选填）" }
];
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
    label: "媒体投放 · CPM价值",
    hint: "预估ROI及投放成本对比（选填）"
  },
  { key: "prOpinion", label: "PR · 舆情判断", hint: "舆论风险与公关建议" }
];
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
  brief: { externalIPs: [], candidateEvaluations: {} }
});
const form = ref<IPRequestInput>(initialForm());
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
    const status =
      menuKind.value === "feedback"
        ? "submitted"
        : menuKind.value === "marketing"
          ? "ip_reviewed"
          : "";
    const result = await listIPRequests({
      keyword: keyword.value,
      status,
      page: page.value,
      pageSize: 20
    });
    rows.value = result.data.list || [];
    total.value = result.data.total || 0;
  } finally {
    loading.value = false;
  }
}
async function loadResources() {
  const result = await listIPResources({ keyword: "", page: 1, pageSize: 200 });
  resourceOptions.value = result.data.list || [];
  await loadIPImages(
    resourceOptions.value.filter(item =>
      form.value.candidateIds.includes(Number(item.id))
    ),
    "id"
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
async function loadDetail(id: number) {
  loading.value = true;
  try {
    detail.value = (await getIPRequest(id)).data;
    await loadIPImages(detail.value.candidates, "ipId");
    feedback.value = detail.value.candidates.map((row, index) => ({
      ipId: Number(row.ipId),
      name: row.name,
      priorityOrder: Number(row.priorityOrder) || index + 1,
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
    priorityOrder: feedback.value.length + 1,
    feasibility: "",
    recommendation: "",
    reason: "",
    assessment: { scoreDimensions: {} }
  });
  addCandidateID.value = null;
}
function editDraft() {
  if (!detail.value) return;
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
      candidateEvaluations: objectValue(item.brief).candidateEvaluations || {}
    }
  };
  mode.value = "new";
  loadResources();
}
async function syncLibrarySelection() {
  if (!librarySelectionKey.value || mode.value !== "new") return;
  const saved = localStorage.getItem(librarySelectionKey.value);
  if (!saved) return;
  localStorage.removeItem(librarySelectionKey.value);
  try {
    form.value.candidateIds = JSON.parse(saved)
      .map(Number)
      .filter((id: number) => id > 0)
      .slice(0, 5);
    await loadResources();
    for (const id of form.value.candidateIds) {
      if (!selectedIP(id)) {
        const result = (await getIPResource(id)).data;
        resourceOptions.value.push({
          ...result.resource,
          visualFileId: result.files.find(file => file.fileKind === "visual")
            ?.id
        });
      }
    }
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
  if (
    submit &&
    (!item.projectName.trim() ||
      !item.department.trim() ||
      !item.markets.length ||
      !item.goal ||
      !item.description.trim())
  ) {
    ElMessage.warning("请填写项目、部门、目标市场、合作目标和需求说明");
    return;
  }
  if (
    submit &&
    (item.budgetMin == null ||
      item.budgetMax == null ||
      item.budgetMin > item.budgetMax)
  ) {
    ElMessage.warning("请填写有效的授权费用预算范围");
    return;
  }
  if (
    submit &&
    [
      "owner",
      "projectPeriod",
      "linkedProduct",
      "cooperationMode",
      "contacts",
      "targetIPType",
      "targetIPCount"
    ].some(key => !String(item.brief[key] || "").trim())
  ) {
    ElMessage.warning(
      "请补齐负责人、项目周期、关联产品、合作模式、对接人及目标IP信息"
    );
    return;
  }
  if (
    submit &&
    item.candidateIds.some(id =>
      ["reason", "productEvaluation", "technicalFit"].some(
        key =>
          !String(item.brief.candidateEvaluations?.[id]?.[key] || "").trim()
      )
    )
  ) {
    ElMessage.warning("请补齐每个库内意向IP的选择理由、产品评估和技术匹配");
    return;
  }
  if (
    submit &&
    !item.candidateIds.length &&
    !item.externalRecommendation.trim() &&
    !item.brief.externalIPs.length
  ) {
    ElMessage.warning("请选择意向IP或填写库外IP推荐");
    return;
  }
  if (
    submit &&
    item.brief.externalIPs.some(
      (candidate: any) =>
        !candidate.name?.trim() ||
        !candidate.rightsOwner?.trim() ||
        !candidate.reason?.trim() ||
        !candidate.productEvaluation?.trim() ||
        !candidate.technicalFit?.trim()
    )
  ) {
    ElMessage.warning("库外IP请补齐名称、版权方、选择理由、产品评估和技术匹配");
    return;
  }
  saving.value = true;
  try {
    const result = await saveIPRequest({ ...item, submit });
    ElMessage.success(submit ? "需求已提交，等待IP组反馈" : "草稿已保存");
    if (Number(route.query.id) === Number(result.data.id)) {
      mode.value = "detail";
      await loadDetail(Number(result.data.id));
    } else
      router.push({
        path: "/business/ip/requests",
        query: { id: String(result.data.id) }
      });
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
  const priorities = feedback.value.map(row => Number(row.priorityOrder));
  if (
    !saveDraft &&
    (new Set(priorities).size !== priorities.length ||
      priorities.some(value => value < 1 || value > priorities.length))
  ) {
    ElMessage.warning("请设置不重复的推荐顺序");
    return;
  }
  const required = [
    "licenseContent",
    "licenseTerritory",
    "ipResources",
    "brandResources",
    "businessRisk",
    "publicRisk",
    "ownershipClarity"
  ];
  if (
    !saveDraft &&
    feedback.value.some(row =>
      required.some(key => !String(row.assessment[key] || "").trim())
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
  if (!detail.value) return;
  if (!saveDraft && !marketing.value.marketingComments.trim()) {
    ElMessage.warning("请填写营销补充意见");
    return;
  }
  const required = [
    "marketHeat",
    "fanAudience",
    "commercialValue",
    "marketingRisks",
    "recentTrend",
    "socialFollowers",
    "ugcForecast",
    "audienceMatch",
    "prOpinion"
  ];
  if (
    !saveDraft &&
    detail.value.candidates.some(candidate =>
      required.some(
        key => !marketingCandidates.value[String(candidate.ipId)]?.[key]?.trim()
      )
    )
  ) {
    ElMessage.warning("请补齐每个意向IP的IMC、品牌运营、用户运营和PR意见");
    return;
  }
  saving.value = true;
  try {
    await submitIPMarketing({
      requestId: Number(detail.value.request.id),
      saveDraft,
      ...marketing.value,
      marketingProfile: {
        ...marketing.value.marketingProfile,
        candidates: marketingCandidates.value
      }
    });
    ElMessage.success(saveDraft ? "营销意见草稿已保存" : "营销意见已提交");
    await loadDetail(Number(detail.value.request.id));
  } finally {
    saving.value = false;
  }
}
function syncRoute() {
  const id = Number(route.query.id);
  if (id > 0) {
    mode.value = "detail";
    loadDetail(id);
  } else if (route.query.mode === "new" && menuKind.value === "requests") {
    mode.value = "new";
    form.value = initialForm();
    loadResources();
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
  <div v-loading="loading" class="ip-request-page">
    <div class="ip-header">
      <div>
        <div v-if="mode !== 'list'" class="ip-kicker">
          需求管理 / {{ mode === "new" ? "提起IP需求" : pageTitle }}
        </div>
        <h1>
          {{
            mode === "list"
              ? pageTitle
              : mode === "new"
                ? "提起IP需求"
                : menuKind === "requests"
                  ? detail?.request.projectName || "需求详情"
                  : pageTitle
          }}
        </h1>
        <p>
          {{
            mode === "list"
              ? pageSubtitle
              : mode === "new"
                ? "明确合作目标和授权预算，提交给IP组评估"
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
        <el-table :data="rows" empty-text="暂无符合条件的IP需求" stripe
          ><el-table-column label="项目名称" min-width="220"
            ><template #default="scope"
              ><button class="ip-link" @click="goDetail(scope.row.id)">
                {{ scope.row.projectName }}</button
              ><small
                >{{ scope.row.department }} · {{ scope.row.createdAt }}</small
              ></template
            ></el-table-column
          ><el-table-column label="目标市场" min-width="150"
            ><template #default="scope">{{
              arrayValue(scope.row.markets).join(" / ")
            }}</template></el-table-column
          ><el-table-column
            prop="goal"
            label="合作目标"
            min-width="120"
          /><el-table-column label="授权费用预算" min-width="180"
            ><template #default="scope">{{
              priceText(scope.row)
            }}</template></el-table-column
          ><el-table-column
            prop="candidateCount"
            label="意向IP"
            width="90"
          /><el-table-column label="进度" width="160"
            ><template #default="scope"
              ><el-tag
                :type="
                  scope.row.status === 'marketing_reviewed'
                    ? 'success'
                    : scope.row.status === 'draft'
                      ? 'info'
                      : 'warning'
                "
                >{{ statusText[scope.row.status] || scope.row.status }}</el-tag
              ></template
            ></el-table-column
          ><el-table-column label="操作" width="115"
            ><template #default="scope"
              ><el-button link type="primary" @click="goDetail(scope.row.id)">{{
                menuKind === "feedback"
                  ? "填写反馈"
                  : menuKind === "marketing"
                    ? "补充意见"
                    : "查看详情"
              }}</el-button></template
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
      ><div class="ip-steps">
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
                >项目名称 *<el-input
                  v-model="form.projectName"
                  placeholder="请输入项目名称" /></label
              ><label
                >需求发起部门 *<el-input
                  v-model="form.department"
                  placeholder="例如：产品组" /></label
              ><label
                >目标市场 *<el-select
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
                >授权费用预算 *
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
                >合作目标 *<el-select v-model="form.goal" placeholder="请选择"
                  ><el-option
                    v-for="goal in goals"
                    :key="goal"
                    :label="goal"
                    :value="goal" /></el-select></label
              ><label class="ip-span"
                >项目背景与需求说明 *<el-input
                  v-model="form.description"
                  type="textarea"
                  :rows="3"
                  maxlength="2000"
                  show-word-limit
                  placeholder="说明合作背景、希望达成的目标与执行方向"
              /></label>
            </div>
          </div>
          <div class="ip-card">
            <div class="ip-card-head">
              <div>
                <h2>初步意向IP</h2>
                <p>
                  最多5个意向IP；可前往资源库查看完整档案、版权资料及历史合作。
                </p>
              </div>
              <a
                class="ip-outline-link"
                :href="libraryHref"
                target="_blank"
                rel="noopener"
                >前往IP资源库 ↗</a
              >
            </div>
            <el-select
              v-model="form.candidateIds"
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
                  <el-button link @click="removeCandidate(id)">移除</el-button>
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
                    >选择理由<el-input
                      v-model="form.brief.candidateEvaluations[id].reason"
                      placeholder="商业潜力、品牌调性或话题契合"
                  /></label>
                  <label
                    >产品维度评估<el-input
                      v-model="
                        form.brief.candidateEvaluations[id].productEvaluation
                      "
                      placeholder="认知度、用户重叠率、机型适配"
                  /></label>
                  <label class="ip-span"
                    >技术匹配程度<el-input
                      v-model="form.brief.candidateEvaluations[id].technicalFit"
                      placeholder="视觉惊喜度、开发及本地化难度"
                  /></label>
                </div>
              </div>
            </div>
            <label class="ip-other"
              >其他IP推荐（选填）<el-input
                v-model="form.externalRecommendation"
                type="textarea"
                :rows="2"
                placeholder="资源库外的IP名称、链接或推荐理由"
              /><small
                >库外推荐会随需求提交，由IP组评估后再决定是否入库。</small
              ></label
            >
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
                    @click="form.brief.externalIPs.splice(index, 1)"
                    >移除</el-button
                  >
                </div>
                <div class="ip-form-grid">
                  <label>IP名称<el-input v-model="item.name" /></label>
                  <label>版权方<el-input v-model="item.rightsOwner" /></label>
                  <label class="ip-span"
                    >IP形象示意链接<el-input
                      v-model="item.imageUrl"
                      placeholder="https://..."
                  /></label>
                  <img
                    v-if="item.imageUrl"
                    :src="item.imageUrl"
                    alt="库外IP形象示意"
                    class="ip-external-image"
                  />
                  <label class="ip-span"
                    >选择理由<el-input v-model="item.reason"
                  /></label>
                  <label class="ip-span"
                    >产品维度评估<el-input
                      v-model="item.productEvaluation"
                      placeholder="市场认知、用户重叠、机型适配"
                  /></label>
                  <label class="ip-span"
                    >技术匹配程度<el-input
                      v-model="item.technicalFit"
                      placeholder="视觉、开发和本地化难度"
                  /></label>
                </div>
              </div>
            </div>
          </div>
          <div class="ip-card ip-optional-brief">
            <el-collapse>
              <el-collapse-item name="brief">
                <template #title>
                  <div>
                    <h2>补充项目资料（选填）</h2>
                    <p>填写项目周期、负责人和合作模式等信息，便于后续协同。</p>
                  </div>
                </template>
                <div class="ip-form-grid">
                  <label
                    v-for="field in briefFields"
                    :key="field.key"
                    class="ip-span"
                  >
                    {{ field.label
                    }}<el-input
                      v-model="form.brief[field.key]"
                      :placeholder="field.hint"
                    />
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
            <div class="ip-note">提交后流转至IP组进行初步意向反馈。</div>
          </div>
        </div>
      </div>
      <div class="ip-bottom">
        <el-button :loading="saving" @click="save(false)">保存草稿</el-button
        ><el-button class="ip-primary" :loading="saving" @click="save(true)"
          >提交需求 →</el-button
        >
      </div></template
    >

    <template v-else-if="mode === 'detail' && detail"
      ><div class="ip-card">
        <div class="ip-detail-title">
          <div>
            <h2>{{ detail.request.projectName }}</h2>
            <p>
              {{ detail.request.department }} ·
              {{ arrayValue(detail.request.markets).join(" / ") }} ·
              {{ detail.request.createdAt }}
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
        <div class="ip-timeline">
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
        <div class="ip-summary">
          <span
            >合作目标：<strong>{{ detail.request.goal }}</strong></span
          ><span
            >授权预算：<strong>{{ priceText(detail.request) }}</strong></span
          ><span
            >预计上线：<strong>{{
              detail.request.expectedLaunch || "待确定"
            }}</strong></span
          >
        </div>
        <p>{{ detail.request.description }}</p>
        <p v-if="detail.request.externalRecommendation">
          <strong>库外IP推荐：</strong
          >{{ detail.request.externalRecommendation }}
        </p>
        <div
          v-if="Object.keys(objectValue(detail.request.brief)).length"
          class="ip-form-grid"
        >
          <div v-for="field in briefFields" :key="field.key">
            <strong>{{ field.label }}：</strong
            >{{ objectValue(detail.request.brief)[field.key] || "待补充" }}
          </div>
          <div
            v-for="(item, index) in objectValue(detail.request.brief)
              .externalIPs || []"
            :key="index"
            class="ip-span"
          >
            <strong>库外IP {{ Number(index) + 1 }}：</strong>{{ item.name }} ·
            {{ item.rightsOwner }} · {{ item.reason }}
            <el-button link @click="openExternalIP(item)"
              >补充并入库 ↗</el-button
            >
          </div>
        </div>
        <el-button v-if="detail.request.status === 'draft'" @click="editDraft"
          >继续编辑草稿</el-button
        >
      </div>
      <div class="ip-grid">
        <div class="ip-stack">
          <div v-if="menuKind !== 'marketing'" class="ip-card">
            <h2>IP组评估</h2>
            <p>请根据评估结果填写推荐意见，并给出优先级排序。</p>
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
                  >可行性判断<el-select
                    v-model="item.feasibility"
                    :disabled="!canFeedback"
                    ><el-option label="可行" value="可行" /><el-option
                      label="需进一步评估"
                      value="需进一步评估" /><el-option
                      label="不可行"
                      value="不可行" /></el-select></label
                ><label
                  >推荐意见<el-select
                    v-model="item.recommendation"
                    :disabled="!canFeedback"
                    ><el-option label="优先合作" value="优先合作" /><el-option
                      label="备选合作"
                      value="备选合作" /><el-option
                      label="暂不合作"
                      value="暂不合作" /></el-select></label
                ><label
                  >优先级<el-input-number
                    v-model="item.priorityOrder"
                    :min="1"
                    :max="8"
                    :disabled="!canFeedback" /></label
                ><label class="ip-span"
                  >理由说明<el-input
                    v-model="item.reason"
                    type="textarea"
                    :rows="2"
                    :disabled="!canFeedback"
                    placeholder="受众匹配、市场覆盖、档期和授权风险"
                /></label>
              </div>
              <el-collapse class="ip-assessment-collapse">
                <el-collapse-item title="评分与授权明细" :name="item.ipId">
                  <div class="ip-score-grid">
                    <label v-for="field in scoreFields" :key="field.key">
                      {{ field.label }} · {{ Math.round(field.weight * 100) }}%
                      <el-input-number
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
                      {{ field.label }}
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
            <h2>营销评估与建议</h2>
            <div
              v-if="
                canMarketing || detail.request.status === 'marketing_reviewed'
              "
              class="ip-marketing-grid"
            >
              <label class="ip-metric-row">
                <span
                  ><strong>市场热度</strong
                  ><small>综合社媒讨论度、搜索趋势及同类案例表现</small></span
                >
                <el-input
                  v-model="marketing.marketHeat"
                  :disabled="!canMarketing"
                  placeholder="填写热度等级与依据"
                />
              </label>
              <label class="ip-metric-row">
                <span
                  ><strong>粉丝受众</strong
                  ><small>核心受众画像及匹配度分析</small></span
                >
                <el-input
                  v-model="marketing.fanAudience"
                  type="textarea"
                  :rows="2"
                  :disabled="!canMarketing"
                />
              </label>
              <label class="ip-metric-row">
                <span
                  ><strong>商业价值</strong
                  ><small>基于过往合作案例与转化潜力评估</small></span
                >
                <el-input
                  v-model="marketing.commercialValue"
                  :disabled="!canMarketing"
                  placeholder="填写商业价值与预算判断"
                />
              </label>
              <label class="ip-metric-row ip-metric-risk">
                <span
                  ><strong>风险与提醒</strong
                  ><small>潜在风险点及应对建议</small></span
                >
                <el-input
                  v-model="marketing.marketingRisks"
                  type="textarea"
                  :rows="2"
                  :disabled="!canMarketing"
                />
              </label>
              <label class="ip-channel-row">
                <strong>推荐传播方向</strong>
                <el-checkbox-group
                  v-model="marketing.marketingChannels"
                  :disabled="!canMarketing"
                  ><el-checkbox
                    v-for="channel in channels"
                    :key="channel"
                    :value="channel"
                    >{{ channel }}</el-checkbox
                  ></el-checkbox-group
                >
              </label>
              <label class="ip-channel-row">
                <strong>补充意见 *</strong>
                <el-input
                  v-model="marketing.marketingComments"
                  type="textarea"
                  :rows="3"
                  :disabled="!canMarketing"
                  placeholder="填写营销补充意见"
                />
              </label>
            </div>
            <template
              v-if="
                canMarketing || detail.request.status === 'marketing_reviewed'
              "
            >
              <div
                v-for="candidate in detail.candidates"
                :key="candidate.ipId"
                class="ip-candidate"
              >
                <h3>{{ candidate.name }} · 分IP意见</h3>
                <div class="ip-form-grid">
                  <label
                    v-for="field in marketingFields"
                    :key="field.key"
                    class="ip-span"
                  >
                    {{ field.label
                    }}<el-input
                      v-model="
                        marketingCandidates[String(candidate.ipId)][field.key]
                      "
                      :disabled="!canMarketing"
                      :placeholder="field.hint"
                    />
                  </label>
                </div>
              </div>
            </template>
            <p v-else>待IP组提交初步反馈后，由营销团队补充意见。</p>
            <div v-if="canMarketing" class="ip-bottom">
              <el-button :loading="saving" @click="sendMarketing(true)"
                >保存意见草稿</el-button
              >
              <el-button
                class="ip-primary"
                :loading="saving"
                @click="sendMarketing(false)"
                >提交营销意见</el-button
              >
            </div>
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
                  >{{ item.recommendation }} · 优先级
                  {{ item.priorityOrder }}</small
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
                >需求已提交<small>{{ detail.request.createdAt }}</small></span
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
.ip-other {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-weight: 600;
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
  align-items: stretch;
  gap: 10px;
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

.ip-summary {
  display: flex;
  gap: 30px;
  padding: 16px 0;
  margin-bottom: 15px;
  color: #858b91;
  border-bottom: 1px solid #eceee9;
}

.ip-summary strong {
  color: #15191c;
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
}

@media (width <= 700px) {
  .ip-request-page {
    padding: 16px;
  }

  .ip-header,
  .ip-search,
  .ip-summary,
  .ip-add-candidate {
    flex-wrap: wrap;
  }

  .ip-form-grid,
  .ip-selected,
  .ip-score-grid,
  .ip-metric-row {
    grid-template-columns: 1fr;
  }

  .ip-steps,
  .ip-timeline {
    font-size: 12px;
    flex-wrap: wrap;
  }
}
</style>
