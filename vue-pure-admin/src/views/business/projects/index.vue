<script setup lang="ts">
import { computed, nextTick, reactive, ref, shallowRef, onMounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import * as XLSX from "xlsx";
import {
  getProjectList,
  createProject,
  importProjects,
  previewProjectExcelImport,
  updateProject,
  deleteProject,
  getCooperationList,
  createCooperation,
  updateCooperation,
  syncCooperation,
  importCooperations,
  getMarketOptions,
  createMarketOption,
  deleteMarketOption
} from "@/api/business";

defineOptions({ name: "BusinessProjects" });

const router = useRouter();
const projects = ref<any[]>([]);
const selectedProjectRows = ref<any[]>([]);
const cooperations = ref<any[]>([]);
const projectDialog = ref(false);
const projectImportDialog = ref(false);
const projectImportLoading = ref(false);
const projectImportRows = ref<any[]>([]);
const projectImportFileName = ref("");
const cooperationDialog = ref(false);
const importDialog = ref(false);
const importLoading = ref(false);
const importParsing = ref(false);
const importParseError = ref("");
const contentUploadKey = ref(0);
const editingProjectId = ref<number | null>(null);
const editingCooperationId = ref<number | null>(null);
const importProjectId = ref<number | null>(null);
const importProjectNameDraft = ref("");
const importProjectCreating = ref(false);
const importWorkbookSheets = shallowRef<{ name: string; rows: any[] }[]>([]);
const selectedImportSheets = ref<string[]>([]);
const importRows = shallowRef<any[]>([]);
const importPreviewLimit = ref(20);
const importPreviewTableRef = ref<any>();
const importFileName = ref("");
const marketOptions = ref<string[]>([]);
const selectedProjectId = ref<number | null>(null);
const activePipelineStage = ref("all");
const activeCooperation = ref<any>(null);
const syncingCooperationIds = reactive<Record<number, boolean>>({});
const savingCooperation = ref(false);
const projectSearch = ref("");
const projectStatusFilter = ref("all");

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

const campaignWizardDialog = ref(false);
const wizardActiveStep = ref(0);
const wizardGenerating = ref(false);
const wizardSubmitting = ref(false);
const wizardSteps = [
  {
    key: "brand",
    label: "基础信息",
    description: "输入官网并生成品牌基础信息"
  },
  {
    key: "settings",
    label: "投放设置",
    description: "设置平台、市场、阈值与人群"
  },
  {
    key: "matches",
    label: "样本匹配",
    description: "用正负反馈校准匹配方向"
  },
  {
    key: "budget",
    label: "预算预测",
    description: "确认预算、预测结果并发布"
  }
];
const productTypeOptions = [
  "网页应用",
  "消费电子",
  "移动应用",
  "电商产品",
  "AI 工具",
  "游戏"
];
const platformOptions = ["TikTok", "YouTube", "Instagram", "Facebook", "X"];
const influencerSettingGroups = [
  {
    title: "表现与活跃度",
    fields: [
      {
        key: "minimumViews",
        label: "最低预计播放",
        suffix: "播放"
      },
      { key: "minimumFollowers", label: "最低粉丝数", suffix: "粉丝" },
      { key: "postingFrequency", label: "发布频率", suffix: "天" },
      { key: "lastPostWithin", label: "最近发布时间", suffix: "天" }
    ]
  },
  {
    title: "价格",
    fields: [
      { key: "maximumCpm", label: "最高 CPM", suffix: "USD" },
      {
        key: "maximumPrice",
        label: "单达人最高报价",
        suffix: "USD"
      }
    ]
  },
  {
    title: "受众",
    fields: [
      { key: "audienceGender", label: "受众性别", suffix: "" },
      { key: "audienceAge", label: "受众年龄", suffix: "" }
    ]
  }
];
const campaignExampleTemplates = [
  {
    title: "时尚品牌扩大触达",
    summary: "通过达人内容扩大产品曝光，同时减少跨区域执行沟通成本。"
  },
  {
    title: "科技产品启动推广",
    summary: "用精确达人筛选和统一执行节奏提升新品认知与试用转化。"
  },
  {
    title: "活动传播预热",
    summary: "匹配目标受众达人，推动活动报名、关注与内容二次传播。"
  },
  {
    title: "健康品牌建立信任",
    summary: "用可信创作者、透明报价和效果追踪降低合作风险。"
  }
];
const sampleInfluencers = reactive([
  {
    id: 1,
    name: "AMadSimple",
    handle: "@madsimple",
    country: "United States",
    language: "English",
    followers: "8.9K",
    engagement: "7.4%",
    predictedViews: "14.6K - 57K",
    predictedCpm: "$3.16 - $18.9",
    price: 520,
    reason: "受众与 AI、效率工具内容高度契合，历史互动表现稳定。",
    matched: null as null | boolean
  },
  {
    id: 2,
    name: "FuturePrompt",
    handle: "@futureprompt",
    country: "United Kingdom",
    language: "English",
    followers: "46.5K",
    engagement: "5.1%",
    predictedViews: "60K - 120K",
    predictedCpm: "$4.1 - $9.7",
    price: 880,
    reason: "创作者科技受众重合度高，长期产出教育型短视频内容。",
    matched: null as null | boolean
  },
  {
    id: 3,
    name: "EverydayAIQ",
    handle: "@everydayaiq",
    country: "Canada",
    language: "English",
    followers: "31.2K",
    engagement: "6.2%",
    predictedViews: "35K - 92K",
    predictedCpm: "$2.8 - $8.4",
    price: 760,
    reason: "测评节奏稳定，适合触达关注实用软件教程的用户。",
    matched: null as null | boolean
  }
]);

const projectForm = reactive({
  name: "",
  targetMarket: "",
  language: "",
  platform: "",
  campaignType: "",
  budget: 0,
  currency: "USD",
  status: "需求创建",
  owner: "",
  brief: ""
});

const campaignWizardForm = reactive({
  websiteUrl: "",
  businessLogo: "",
  businessName: "",
  productType: "网页应用",
  searchableBrands: "品牌名, 产品名, 系列名",
  businessIntroduction: "",
  campaignGoal: "达人营销推广",
  targetMarket: "美国",
  language: "English",
  platform: ["TikTok"],
  contentPreference: "产品测评、教程、对比、UGC 风格演示",
  campaignType: "官网引流",
  owner: "",
  currency: "USD",
  budget: 10000,
  cycleStartDate: "",
  cycleEndDate: "",
  idealScriptSubmission: false,
  automaticApproval: false,
  settings: {
    minimumViews: 3000,
    minimumFollowers: 1000,
    postingFrequency: 14,
    lastPostWithin: 30,
    maximumCpm: 18,
    maximumPrice: 1200,
    audienceGender: "All",
    audienceAge: "18-34"
  } as Record<string, any>
});

const wizardMatchedCount = computed(
  () => sampleInfluencers.filter(item => item.matched === true).length
);
const wizardRejectedCount = computed(
  () => sampleInfluencers.filter(item => item.matched === false).length
);
const wizardForecast = computed(() => {
  const budget = numberValue(campaignWizardForm.budget);
  const avgPrice =
    sampleInfluencers.reduce((sum, item) => sum + item.price, 0) /
    sampleInfluencers.length;
  const influencerCount = Math.max(1, Math.floor(budget / avgPrice));
  const estimatedViews = influencerCount * 42000;
  const estimatedClicks = Math.round(estimatedViews * 0.018);
  return {
    influencerCount,
    estimatedViews,
    estimatedClicks,
    cpm: budget > 0 ? (budget / estimatedViews) * 1000 : 0,
    cpc: estimatedClicks > 0 ? budget / estimatedClicks : 0
  };
});

const cooperationForm = reactive({
  projectId: 1,
  resourceId: 1,
  cooperationType: "",
  quoteAmount: 0,
  currency: "USD",
  status: "邀约中",
  deliverableStatus: "未开始",
  impressions: 0,
  views: 0,
  clicks: 0,
  conversions: 0,
  engagementCount: 0,
  commentsCount: 0,
  roi: 0,
  teamRating: 0,
  releaseDate: "",
  deliverableLinks: "",
  notes: ""
});

const validImportRows = computed(() =>
  importRows.value.filter(row => (row.errors || []).length === 0)
);

const invalidImportRows = computed(() =>
  importRows.value.filter(row => (row.errors || []).length > 0)
);

const duplicateImportRows = computed(() =>
  importRows.value.filter(row => row.duplicate)
);

const linkedImportRows = computed(() =>
  validImportRows.value.filter(row => String(row.deliverableLinks || "").trim())
);

const visibleImportRows = computed(() =>
  importRows.value.slice(0, importPreviewLimit.value)
);

const hasMoreImportRows = computed(
  () => visibleImportRows.value.length < importRows.value.length
);

const validProjectImportRows = computed(() =>
  projectImportRows.value.filter(row => row.errors.length === 0)
);

const invalidProjectImportRows = computed(() =>
  projectImportRows.value.filter(row => row.errors.length > 0)
);
const projectReviewRows = computed(() =>
  projects.value.map(project => ({
    ...project,
    review: projectStats(project.id)
  }))
);
const overallReview = computed(() => {
  const stat = emptyProjectStats();
  cooperations.value.forEach(item => accumulateProjectStat(stat, item));
  return stat;
});
const projectReviewTableRows = computed(() => [
  ...projectReviewRows.value.map(project => ({
    ...project,
    isSum: false
  })),
  {
    id: "sum",
    name: "SUM 总和",
    targetMarket: "全部市场",
    platform: "全部平台",
    owner: "-",
    status: "汇总",
    review: overallReview.value,
    isSum: true
  }
]);
const overallReviewInsight = computed(() =>
  buildReviewInsight("全部项目", overallReview.value, true)
);
const selectedProject = computed(() =>
  projects.value.find(
    item => Number(item.id) === Number(selectedProjectId.value)
  )
);
const selectedCooperations = computed(() =>
  cooperations.value.filter(
    item => Number(item.projectId) === Number(selectedProjectId.value)
  )
);
const selectedProjectReview = computed(() =>
  selectedProject.value
    ? projectStats(selectedProject.value.id)
    : emptyProjectStats()
);
const pipelineStages = computed(() => {
  const stageDefinitions = [
    {
      key: "inviting",
      label: "邀约 / 议价",
      icon: "ri:send-plane-line",
      description: "等待达人回复或确认价格"
    },
    {
      key: "confirmed",
      label: "已确认合作",
      icon: "ri:handshake-line",
      description: "合作已确认，等待启动交付"
    },
    {
      key: "production",
      label: "内容制作",
      icon: "ri:movie-2-line",
      description: "脚本、稿件或内容正在制作"
    },
    {
      key: "pending_publish",
      label: "待发布",
      icon: "ri:calendar-event-line",
      description: "内容已确认，等待按计划发布"
    },
    {
      key: "published",
      label: "已发布",
      icon: "ri:checkbox-circle-line",
      description: "进入数据回收与复盘"
    }
  ];
  return stageDefinitions.map(stage => ({
    ...stage,
    count: selectedCooperations.value.filter(
      item => cooperationStage(item) === stage.key
    ).length
  }));
});
const pipelineRows = computed(() =>
  activePipelineStage.value === "all"
    ? selectedCooperations.value
    : selectedCooperations.value.filter(
        item => cooperationStage(item) === activePipelineStage.value
      )
);
const pendingActions = computed(() =>
  selectedCooperations.value
    .map(item => ({
      ...item,
      action: cooperationAction(item)
    }))
    .filter(item => Boolean(item.action))
    .slice(0, 6)
);
const focusedCooperation = computed(() => {
  const rows = selectedCooperations.value;
  const activeId = Number(activeCooperation.value?.id || 0);
  if (activeId) {
    const current = rows.find(item => Number(item.id) === activeId);
    if (current) return current;
  }
  return pendingActions.value[0] || pipelineRows.value[0] || rows[0] || null;
});
const campaignHealth = computed(() => {
  const rows = selectedCooperations.value;
  const budget = numberValue(selectedProject.value?.budget);
  const spent = rows.reduce(
    (total, item) => total + numberValue(item.quoteAmount),
    0
  );
  const published = rows.filter(
    item => cooperationStage(item) === "published"
  ).length;
  const missingData = rows.filter(
    item =>
      cooperationStage(item) === "published" &&
      primaryReach(item) <= 0 &&
      numberValue(item.clicks) <= 0
  ).length;
  return {
    budget,
    spent,
    remaining: Math.max(budget - spent, 0),
    budgetRate:
      budget > 0 ? Math.min(Math.round((spent / budget) * 100), 100) : 0,
    published,
    completionRate:
      rows.length > 0 ? Math.round((published / rows.length) * 100) : 0,
    missingData
  };
});

const visibleProjects = computed(() => {
  const keyword = projectSearch.value.trim().toLowerCase();
  return projects.value.filter(project => {
    const status = String(project.status || "").toLowerCase();
    if (
      projectStatusFilter.value !== "all" &&
      !status.includes(projectStatusFilter.value)
    )
      return false;
    if (!keyword) return true;
    return [
      project.name,
      project.targetMarket,
      project.platform,
      project.campaignType,
      project.owner
    ]
      .filter(Boolean)
      .some(value => String(value).toLowerCase().includes(keyword));
  });
});

const centerOverview = computed(() => {
  const totals = cooperations.value.reduce(
    (summary, item) => {
      summary.creators.add(Number(item.resourceId || 0));
      summary.content += cooperationStage(item) === "published" ? 1 : 0;
      summary.reach += primaryReach(item);
      summary.cost += numberValue(item.quoteAmount);
      return summary;
    },
    { creators: new Set<number>(), content: 0, reach: 0, cost: 0 }
  );
  return {
    active: projects.value.filter(
      project => !/paused|结束|archived/i.test(String(project.status || ""))
    ).length,
    creators: Array.from(totals.creators).filter(Boolean).length,
    content: totals.content,
    reach: totals.reach,
    cost: totals.cost
  };
});

async function loadData() {
  try {
    const [projectRes, cooperationRes] = await Promise.all([
      getProjectList(),
      getCooperationList()
    ]);
    projects.value =
      projectRes.code === 0 && Array.isArray(projectRes.data?.list)
        ? projectRes.data.list
        : [];
    cooperations.value =
      cooperationRes.code === 0 && Array.isArray(cooperationRes.data?.list)
        ? cooperationRes.data.list
        : [];

    if (
      !projects.value.some(
        project => Number(project.id) === Number(selectedProjectId.value)
      )
    ) {
      selectedProjectId.value = projects.value[0]?.id ?? null;
    }
  } catch {
    projects.value = [];
    cooperations.value = [];
    selectedProjectId.value = null;
    ElMessage.warning("项目列表加载失败，请稍后重试");
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
    if (projectForm.targetMarket === name) projectForm.targetMarket = "";
    ElMessage.success("市场选项已删除");
  } else {
    ElMessage.warning(res.message || "市场删除失败");
  }
}

function resetProjectForm() {
  editingProjectId.value = null;
  Object.assign(projectForm, {
    name: "",
    targetMarket: "",
    language: "",
    platform: "",
    campaignType: "",
    budget: 0,
    currency: "USD",
    status: "需求创建",
    owner: "",
    brief: ""
  });
}

function resetCampaignWizard() {
  wizardActiveStep.value = 0;
  wizardGenerating.value = false;
  wizardSubmitting.value = false;
  Object.assign(campaignWizardForm, {
    websiteUrl: "",
    businessLogo: "",
    businessName: "",
    productType: "网页应用",
    searchableBrands: "品牌名, 产品名, 系列名",
    businessIntroduction: "",
    campaignGoal: "达人营销推广",
    targetMarket: marketOptions.value[0] || "美国",
    language: "English",
    platform: ["TikTok"],
    contentPreference: "产品测评、教程、对比、UGC 风格演示",
    campaignType: "官网引流",
    owner: "",
    currency: "USD",
    budget: 10000,
    cycleStartDate: "",
    cycleEndDate: "",
    idealScriptSubmission: false,
    automaticApproval: false,
    settings: {
      minimumViews: 3000,
      minimumFollowers: 1000,
      postingFrequency: 14,
      lastPostWithin: 30,
      maximumCpm: 18,
      maximumPrice: 1200,
      audienceGender: "All",
      audienceAge: "18-34"
    }
  });
  sampleInfluencers.forEach(item => {
    item.matched = null;
  });
}

function inferBusinessName(url: string) {
  const normalized = String(url || "").trim();
  if (!normalized) return "";
  try {
    const parsed = new URL(
      normalized.startsWith("http") ? normalized : `https://${normalized}`
    );
    const host = parsed.hostname.replace(/^www\./, "");
    return host
      .split(".")[0]
      .split(/[-_]/)
      .filter(Boolean)
      .map(item => item.charAt(0).toUpperCase() + item.slice(1))
      .join(" ");
  } catch {
    return normalized.replace(/^https?:\/\//, "").split(/[./]/)[0];
  }
}

async function generateCampaignProfile() {
  if (!campaignWizardForm.websiteUrl.trim()) {
    ElMessage.warning("请先输入官网 URL");
    return;
  }
  wizardGenerating.value = true;
  await new Promise(resolve => window.setTimeout(resolve, 650));
  const name =
    campaignWizardForm.businessName ||
    inferBusinessName(campaignWizardForm.websiteUrl) ||
    "New Brand";
  campaignWizardForm.businessName = name;
  campaignWizardForm.businessLogo = name.slice(0, 1).toUpperCase();
  campaignWizardForm.searchableBrands = `${name}, ${name} Pro, ${name} App`;
  campaignWizardForm.businessIntroduction = `${name} 正在准备全球达人营销项目。项目需要清晰表达产品价值，匹配受众真实契合的创作者，并在发布后追踪播放、点击、CPM 和 CPC。`;
  campaignWizardForm.campaignGoal = `${name} ${campaignWizardForm.campaignType}`;
  wizardGenerating.value = false;
}

function wizardCanNext() {
  if (wizardActiveStep.value === 0) {
    return Boolean(
      campaignWizardForm.businessName.trim() &&
      campaignWizardForm.businessIntroduction.trim()
    );
  }
  if (wizardActiveStep.value === 1) {
    return Boolean(
      campaignWizardForm.targetMarket &&
      campaignWizardForm.language &&
      campaignWizardForm.platform.length
    );
  }
  if (wizardActiveStep.value === 2) {
    return sampleInfluencers.some(item => item.matched !== null);
  }
  return true;
}

function nextCampaignWizardStep() {
  if (!wizardCanNext()) {
    ElMessage.warning("请先补全当前步骤的关键信息");
    return;
  }
  wizardActiveStep.value = Math.min(
    wizardActiveStep.value + 1,
    wizardSteps.length - 1
  );
}

function previousCampaignWizardStep() {
  wizardActiveStep.value = Math.max(wizardActiveStep.value - 1, 0);
}

function setSampleMatch(id: number, matched: boolean) {
  const current = sampleInfluencers.find(item => item.id === id);
  if (current) current.matched = matched;
}

function buildCampaignBrief() {
  const matched = sampleInfluencers
    .filter(item => item.matched === true)
    .map(item => item.name)
    .join(", ");
  const rejected = sampleInfluencers
    .filter(item => item.matched === false)
    .map(item => item.name)
    .join(", ");
  const settings = campaignWizardForm.settings;
  return [
    `[营销项目创建向导]`,
    `官网 URL：${campaignWizardForm.websiteUrl || "-"}`,
    `品牌介绍：${campaignWizardForm.businessIntroduction}`,
    `可搜索品牌：${campaignWizardForm.searchableBrands || "-"}`,
    `内容偏好：${campaignWizardForm.contentPreference || "-"}`,
    `高级设置：最低预计播放 ${settings.minimumViews}，最低粉丝数 ${settings.minimumFollowers}，发布频率 ${settings.postingFrequency} 天，最近发布 ${settings.lastPostWithin} 天内，最高 CPM ${settings.maximumCpm}，单达人最高报价 ${settings.maximumPrice}，受众 ${settings.audienceGender}/${settings.audienceAge}。`,
    `样本反馈：匹配 ${matched || "-"}；不匹配 ${rejected || "-"}。`,
    `预算预测：${wizardForecast.value.influencerCount} 位达人，预计播放 ${formatCount(wizardForecast.value.estimatedViews)}，预计点击 ${formatCount(wizardForecast.value.estimatedClicks)}，CPM ${moneyText(wizardForecast.value.cpm)}，CPC ${moneyText(wizardForecast.value.cpc)}。`
  ].join("\n");
}

function resetCooperationForm() {
  editingCooperationId.value = null;
  Object.assign(cooperationForm, {
    projectId: projects.value[0]?.id || 1,
    resourceId: 1,
    cooperationType: "",
    quoteAmount: 0,
    currency: "USD",
    status: "邀约中",
    deliverableStatus: "未开始",
    impressions: 0,
    views: 0,
    clicks: 0,
    conversions: 0,
    engagementCount: 0,
    commentsCount: 0,
    roi: 0,
    teamRating: 0,
    releaseDate: "",
    deliverableLinks: "",
    notes: ""
  });
}

function openCreateProject() {
  resetCampaignWizard();
  campaignWizardDialog.value = true;
}

function openQuickProject() {
  resetProjectForm();
  projectDialog.value = true;
}

function openEditProject(row: any) {
  resetProjectForm();
  editingProjectId.value = Number(row.id);
  Object.assign(projectForm, {
    name: row.name || "",
    targetMarket: row.targetMarket || "",
    language: row.language || "",
    platform: row.platform || "",
    campaignType: row.campaignType || "",
    budget: numberValue(row.budget),
    currency: row.currency || "USD",
    status: row.status || "需求创建",
    owner: row.owner || "",
    brief: row.brief || ""
  });
  projectDialog.value = true;
}

function projectStatusTag(status: unknown) {
  const value = String(status || "").toLowerCase();
  if (/active|进行|执行/.test(value)) return "success";
  if (/pause|暂停/.test(value)) return "info";
  if (/draft|需求|创建/.test(value)) return "warning";
  return "primary";
}

function openCreateCooperation() {
  resetCooperationForm();
  cooperationDialog.value = true;
}

function openEditCooperation(row: any) {
  resetCooperationForm();
  editingCooperationId.value = Number(row.id);
  Object.assign(cooperationForm, {
    projectId: Number(row.projectId || 1),
    resourceId: Number(row.resourceId || 1),
    cooperationType: row.cooperationType || "",
    quoteAmount: numberValue(row.quoteAmount),
    currency: row.currency || "USD",
    status: row.status || "邀约中",
    deliverableStatus: row.deliverableStatus || "未开始",
    impressions: numberValue(row.impressions),
    views: numberValue(row.views),
    clicks: numberValue(row.clicks),
    conversions: numberValue(row.conversions),
    engagementCount: numberValue(row.engagementCount),
    commentsCount: numberValue(row.commentsCount),
    roi: numberValue(row.roi),
    teamRating: numberValue(row.teamRating),
    releaseDate: row.releaseDate || "",
    deliverableLinks: row.deliverableLinks || "",
    notes: row.notes || ""
  });
  cooperationDialog.value = true;
}

async function submitProject() {
  await handleMarketChange(projectForm.targetMarket);
  const payload = editingProjectId.value
    ? { id: editingProjectId.value, ...projectForm }
    : projectForm;
  const res = editingProjectId.value
    ? await updateProject(payload)
    : await createProject(payload);
  if (res.code === 0) {
    ElMessage.success(editingProjectId.value ? "项目已更新" : "项目已创建");
    projectDialog.value = false;
    loadData();
  }
}

async function releaseCampaignFromWizard() {
  if (!wizardCanNext()) {
    ElMessage.warning("请先确认样本匹配反馈");
    return;
  }
  await handleMarketChange(campaignWizardForm.targetMarket);
  wizardSubmitting.value = true;
  try {
    const payload = {
      name: campaignWizardForm.campaignGoal || campaignWizardForm.businessName,
      targetMarket: campaignWizardForm.targetMarket,
      language: campaignWizardForm.language,
      platform: campaignWizardForm.platform.join(", "),
      campaignType: campaignWizardForm.campaignType,
      budget: campaignWizardForm.budget,
      currency: campaignWizardForm.currency,
      status: "Active",
      owner: campaignWizardForm.owner,
      brief: buildCampaignBrief(),
      cycleStartDate: campaignWizardForm.cycleStartDate,
      cycleEndDate: campaignWizardForm.cycleEndDate,
      reportUpdateDate: new Date().toISOString().slice(0, 10)
    };
    const res = await createProject(payload);
    if (res.code === 0) {
      ElMessage.success("项目已发布，正在进入执行中心");
      campaignWizardDialog.value = false;
      await loadData();
      const created = projects.value.find(item => item.name === payload.name);
      if (created?.id) openCampaignDetail(created.id);
    }
  } finally {
    wizardSubmitting.value = false;
  }
}

async function submitCooperation() {
  if (savingCooperation.value) return;
  const payload = editingCooperationId.value
    ? { id: editingCooperationId.value, ...cooperationForm }
    : cooperationForm;
  savingCooperation.value = true;
  try {
    const res = editingCooperationId.value
      ? await updateCooperation(payload)
      : await createCooperation(payload);
    if (res.code === 0) {
      ElMessage.success(
        res.data?.postSync?.synced
          ? `${editingCooperationId.value ? "合作记录已更新" : "合作记录已创建"}，${res.data.postSync.message}`
          : editingCooperationId.value
            ? "合作记录已更新"
            : "合作记录已创建"
      );
      if (res.data?.postSync?.message && !res.data.postSync.synced) {
        ElMessage.warning(res.data.postSync.message);
      }
      cooperationDialog.value = false;
      await loadData();
    }
  } finally {
    savingCooperation.value = false;
  }
}

async function syncCooperationPost(row: any) {
  const id = Number(row.id || 0);
  if (!id) return;
  syncingCooperationIds[id] = true;
  const res = await syncCooperation({ id });
  syncingCooperationIds[id] = false;
  if (res.code !== 0) return;
  if (res.data?.synced) {
    ElMessage.success(res.data.message || "合作作品数据同步成功");
    await loadData();
    return;
  }
  ElMessage.warning(res.data?.message || "未找到匹配的合作作品");
}

function normalizeHeader(value: string) {
  return String(value)
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]/gu, "");
}

const headerAliases = {
  influencer: [
    "姓名",
    "Influencer",
    "达人",
    "KOL",
    "Creator",
    "Publication",
    "Outlet",
    "媒体名称"
  ],
  category: ["领域", "Category", "Type", "Industry", "类型"],
  platform: ["平台", "Platform", "Channel", "渠道"],
  followerNumber: [
    "粉丝数",
    "Follower Number",
    "Followers",
    "Fans",
    "UVM",
    "MUV"
  ],
  country: ["国家", "国家地区", "Country", "Location", "Market"],
  releaseDate: [
    "发布日期",
    "发布时间",
    "Release Date",
    "Publish Date",
    "Published At"
  ],
  deliverableLinks: [
    "发布链接",
    "内容链接",
    "作品链接",
    "Deliverable Links",
    "Published Link",
    "Post URL",
    "Post Link",
    "Link",
    "URL"
  ],
  views: ["播放量", "浏览量", "Views", "View Count", "Impressions", "曝光量"],
  engagementCount: [
    "转赞藏数",
    "互动量",
    "Likes+Fav+Share",
    "Likes Fav Share",
    "Engagement",
    "Interactions"
  ],
  commentsCount: ["评论数", "Comments", "Comment Count"],
  quoteAmount: ["报价", "费用", "Cost", "Quote", "Paid Amount"],
  rating: ["评级", "Rating"]
};

const projectImportAliases = {
  name: ["项目名称", "项目", "Project Name", "Project", "Name"],
  targetMarket: ["目标市场", "市场", "Target Market", "Market"],
  language: ["语言", "Language"],
  platform: ["平台", "Platform", "Channel"],
  campaignType: ["合作目标", "营销类型", "Campaign Type", "Objective"],
  budget: ["预算", "Budget"],
  currency: ["币种", "货币", "Currency"],
  status: ["状态", "Status"],
  owner: ["负责人", "Owner", "Project Owner"],
  brief: ["项目说明", "简介", "Brief", "Description"],
  cycleStartDate: ["开始日期", "项目开始", "Start Date", "Cycle Start"],
  cycleEndDate: ["结束日期", "项目结束", "End Date", "Cycle End"]
};

function headerMatches(header: string, alias: string) {
  const normalizedHeader = normalizeHeader(header);
  const normalizedAlias = normalizeHeader(alias);
  return (
    normalizedHeader === normalizedAlias ||
    (normalizedAlias.length >= 3 && normalizedHeader.includes(normalizedAlias))
  );
}

function matchingHeader(row: Record<string, any>, aliases: string[]) {
  return Object.keys(row).find(header =>
    aliases.some(alias => headerMatches(header, alias))
  );
}

function pickValue(row: Record<string, any>, aliases: string[]) {
  const key = matchingHeader(row, aliases);
  return key ? row[key] : "";
}

function rawCellValue(value: any) {
  return value && typeof value === "object" && "value" in value
    ? value.value
    : value;
}

function cellText(value: any) {
  const rawValue = rawCellValue(value);
  if (rawValue === null || rawValue === undefined) return "";
  return String(rawValue).trim();
}

function cellHyperlink(value: any) {
  if (
    !value ||
    typeof value !== "object" ||
    typeof value.hyperlink !== "string"
  ) {
    return "";
  }
  return value.hyperlink
    .replace(/&amp;/gi, "&")
    .replace(/&quot;/gi, '"')
    .trim();
}

function isHTTPURL(value: any) {
  try {
    const parsed = new URL(cellText(value));
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

function pickCooperationLink(row: Record<string, any>) {
  const mappedValue = pickValue(row, headerAliases.deliverableLinks);
  const mapped = cellHyperlink(mappedValue) || cellText(mappedValue);
  if (mapped) return mapped;
  const urlEntry = Object.entries(row).find(
    ([header, value]) =>
      (isHTTPURL(cellHyperlink(value)) || isHTTPURL(value)) &&
      !/账号|主页|profile|homepage|channelurl|website|domain/i.test(
        String(header)
      )
  );
  return urlEntry ? cellHyperlink(urlEntry[1]) || cellText(urlEntry[1]) : "";
}

function formatDateValue(value: any) {
  value = rawCellValue(value);
  if (!value) return "";
  if (value instanceof Date && !Number.isNaN(value.getTime())) {
    return value.toISOString().slice(0, 10);
  }
  if (typeof value === "number") {
    const parsed = XLSX.SSF.parse_date_code(value);
    if (parsed) {
      const month = String(parsed.m).padStart(2, "0");
      const day = String(parsed.d).padStart(2, "0");
      return `${parsed.y}-${month}-${day}`;
    }
  }
  const text = String(value).trim();
  if (!text) return "";
  const normalized = text.replace(/\//g, "-");
  const match = normalized.match(/^(\d{4})-(\d{1,2})-(\d{1,2})/);
  if (match) {
    return `${match[1]}-${match[2].padStart(2, "0")}-${match[3].padStart(
      2,
      "0"
    )}`;
  }
  const monthFirst = normalized.match(/^(\d{1,2})-(\d{1,2})-(\d{2,4})/);
  if (!monthFirst) return text;
  const year =
    monthFirst[3].length === 2 ? `20${monthFirst[3]}` : monthFirst[3];
  return `${year}-${monthFirst[1].padStart(2, "0")}-${monthFirst[2].padStart(
    2,
    "0"
  )}`;
}

function parseNumber(value: any) {
  if (value === "" || value === null || value === undefined) return 0;
  const text = cellText(value).toLowerCase();
  if (["/", "-", "n/a", "na"].includes(text)) return 0;
  const match = text
    .replace(/,/g, "")
    .match(/(-?[\d.]+)\s*([kmb]|thousand|million|billion)?/i);
  if (!match) return 0;
  const multiplier =
    {
      k: 1_000,
      thousand: 1_000,
      m: 1_000_000,
      million: 1_000_000,
      b: 1_000_000_000,
      billion: 1_000_000_000
    }[String(match[2] || "").toLowerCase()] || 1;
  const num = Number(match[1]) * multiplier;
  return Number.isFinite(num) ? num : 0;
}

function normalizeProjectImportRow(
  row: Record<string, any>,
  rowNo: number,
  sourceSheet: string
) {
  const startDate = formatDateValue(
    pickValue(row, projectImportAliases.cycleStartDate)
  );
  const endDate = formatDateValue(
    pickValue(row, projectImportAliases.cycleEndDate)
  );
  const normalized: any = {
    rowNo,
    sourceSheet,
    name: cellText(pickValue(row, projectImportAliases.name)),
    targetMarket: cellText(pickValue(row, projectImportAliases.targetMarket)),
    language: cellText(pickValue(row, projectImportAliases.language)),
    platform: cellText(pickValue(row, projectImportAliases.platform)),
    campaignType: cellText(pickValue(row, projectImportAliases.campaignType)),
    budget: Math.max(
      0,
      parseNumber(pickValue(row, projectImportAliases.budget))
    ),
    currency:
      cellText(pickValue(row, projectImportAliases.currency)).toUpperCase() ||
      "USD",
    status: cellText(pickValue(row, projectImportAliases.status)) || "需求创建",
    owner: cellText(pickValue(row, projectImportAliases.owner)),
    brief: cellText(pickValue(row, projectImportAliases.brief)),
    cycleStartDate: startDate,
    cycleEndDate: endDate,
    duplicate: false,
    errors: [] as string[]
  };

  if (!normalized.name) normalized.errors.push("缺少项目名称");
  if (startDate && !/^\d{4}-\d{2}-\d{2}$/.test(startDate)) {
    normalized.errors.push("开始日期格式不正确");
  }
  if (endDate && !/^\d{4}-\d{2}-\d{2}$/.test(endDate)) {
    normalized.errors.push("结束日期格式不正确");
  }
  return normalized;
}

function parseProjectImportSheet(
  worksheet: XLSX.WorkSheet,
  sourceSheet: string
) {
  const rows = XLSX.utils.sheet_to_json<Record<string, any>>(worksheet, {
    defval: "",
    raw: true
  });
  return rows
    .map((row, index) => normalizeProjectImportRow(row, index + 2, sourceSheet))
    .filter(row =>
      Object.values(row).some(value => value && typeof value !== "object")
    );
}

function downloadProjectImportTemplate() {
  const headers = [
    "项目名称",
    "目标市场",
    "语言",
    "平台",
    "合作目标",
    "预算",
    "币种",
    "状态",
    "负责人",
    "项目说明",
    "开始日期",
    "结束日期"
  ];
  const worksheet = XLSX.utils.aoa_to_sheet([
    headers,
    [
      "Summer Creator Launch",
      "美国",
      "English",
      "TikTok, Instagram",
      "新品种草",
      25000,
      "USD",
      "需求创建",
      "Mia",
      "面向北美市场的夏季新品推广",
      "2026-07-01",
      "2026-08-31"
    ]
  ]);
  worksheet["!cols"] = headers.map(header => ({
    wch: Math.max(14, header.length * 2 + 4)
  }));
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, worksheet, "项目导入模板");
  XLSX.writeFile(workbook, "XMP_Project_Import_Template.xlsx");
}

function monitoringSheetNames(workbook: XLSX.WorkBook) {
  return workbook.SheetNames.filter(sheetName => {
    const rows = XLSX.utils.sheet_to_json<any[]>(workbook.Sheets[sheetName], {
      header: 1,
      defval: "",
      blankrows: false,
      range: 0
    });
    return rows.slice(0, 6).some(values => {
      const headers = values.map(value => String(value || "").trim());
      const hasResource = headers.some(header =>
        headerAliases.influencer.some(alias => headerMatches(header, alias))
      );
      const hasLink = headers.some(header =>
        headerAliases.deliverableLinks.some(alias =>
          headerMatches(header, alias)
        )
      );
      return hasResource && hasLink;
    });
  });
}

function openContentImportWorkbook(workbook: XLSX.WorkBook, fileName: string) {
  importProjectId.value = null;
  importProjectNameDraft.value = "";
  importFileName.value = fileName;
  importWorkbookSheets.value = workbook.SheetNames.map(name => ({
    name,
    rows: parseImportSheet(workbook.Sheets[name])
  }));
  const reviewSheets = importWorkbookSheets.value
    .filter(sheet => sheet.name.includes("测评内容"))
    .map(sheet => sheet.name);
  const detectedSheets = monitoringSheetNames(workbook);
  selectedImportSheets.value =
    reviewSheets.length > 0 ? reviewSheets : detectedSheets;
  refreshImportRows();
  importDialog.value = true;
}

async function handleProjectImportFile(file: any) {
  // Keep legacy upload controls on the same workflow: one workbook becomes one
  // project, then the user chooses the Sheet(s) whose content should be imported.
  await handleUploadFile(file);
}

async function submitProjectImport() {
  if (!validProjectImportRows.value.length) {
    ElMessage.warning("没有可导入的有效项目");
    return;
  }
  projectImportLoading.value = true;
  try {
    const res = await importProjects({ rows: validProjectImportRows.value });
    if (res.code !== 0) {
      ElMessage.warning(res.message || "项目导入失败");
      return;
    }
    ElMessage.success(
      `已导入 ${res.data.imported || 0} 个项目，跳过 ${res.data.skipped || 0} 个重复项目`
    );
    projectImportDialog.value = false;
    projectImportRows.value = [];
    await loadData();
  } finally {
    projectImportLoading.value = false;
  }
}

function normalizeImportRow(
  row: Record<string, any>,
  index: number,
  previousResource: Record<string, any>
) {
  const sourceName = cellText(pickValue(row, headerAliases.influencer));
  const nameHeader = matchingHeader(row, headerAliases.influencer) || "";
  const sourcePlatform = cellText(pickValue(row, headerAliases.platform));
  const sourceCategory = cellText(pickValue(row, headerAliases.category));
  const sourceCountry = cellText(pickValue(row, headerAliases.country));
  const sourceLink = pickCooperationLink(row);
  const inferredMedia =
    /publication|outlet|媒体|media/i.test(nameHeader) ||
    /^website$/i.test(sourcePlatform);
  const normalized: any = {
    rowNo: index + 1,
    influencer: sourceName || previousResource.influencer || "",
    category: sourceCategory || previousResource.category || "",
    platform: sourcePlatform || previousResource.platform || "",
    country: sourceCountry,
    resourceType:
      inferredMedia || previousResource.resourceType === "媒体"
        ? "媒体"
        : "KOL",
    mediaOutlet: inferredMedia
      ? sourceName || previousResource.influencer || ""
      : "",
    followerNumber: parseNumber(pickValue(row, headerAliases.followerNumber)),
    releaseDate: formatDateValue(pickValue(row, headerAliases.releaseDate)),
    deliverableLinks: sourceLink,
    views: parseNumber(pickValue(row, headerAliases.views)),
    engagementCount: parseNumber(pickValue(row, headerAliases.engagementCount)),
    commentsCount: parseNumber(pickValue(row, headerAliases.commentsCount)),
    quoteAmount: parseNumber(pickValue(row, headerAliases.quoteAmount)),
    rating: cellText(pickValue(row, headerAliases.rating)),
    sourceHasIdentity: Boolean(
      sourceName ||
      sourcePlatform ||
      sourceCategory ||
      sourceCountry ||
      sourceLink
    ),
    duplicate: false,
    errors: []
  };

  if (!normalized.influencer) normalized.errors.push("缺少资源名称");
  if (!normalized.deliverableLinks) {
    normalized.errors.push("缺少发布链接");
  } else if (!isHTTPURL(normalized.deliverableLinks)) {
    normalized.errors.push("发布链接必须是有效 URL");
  }
  [
    "followerNumber",
    "views",
    "engagementCount",
    "commentsCount",
    "quoteAmount"
  ].forEach(key => {
    if (normalized[key] < 0) {
      normalized[key] = 0;
    }
  });
  return normalized;
}

function isHeaderRow(values: any[]) {
  const headers = values
    .map(value => String(value || "").trim())
    .filter(Boolean);
  const matches = headers.filter(header =>
    Object.values(headerAliases).some(aliases =>
      aliases.some(alias => headerMatches(header, alias))
    )
  );
  return matches.length >= 2;
}

function parseImportSheet(worksheet: XLSX.WorkSheet) {
  const matrix = XLSX.utils.sheet_to_json<any[]>(worksheet, {
    header: 1,
    defval: "",
    blankrows: false
  });
  let headers: string[] = [];
  let previousResource: Record<string, any> = {};
  const rows: any[] = [];

  matrix.forEach((values, index) => {
    const cells = values.map(value => String(value ?? "").trim());
    if (isHeaderRow(cells)) {
      headers = cells;
      previousResource = {};
      return;
    }
    if (headers.length === 0 || cells.every(value => !value)) return;
    const rawRow = Object.fromEntries(
      headers.map((header, column) => [
        header || `Column ${column + 1}`,
        {
          value: values[column] ?? "",
          hyperlink: String(
            (worksheet[XLSX.utils.encode_cell({ r: index, c: column })] as any)
              ?.l?.Target || ""
          ).trim()
        }
      ])
    );
    const normalized = normalizeImportRow(rawRow, index, previousResource);
    if (!normalized.sourceHasIdentity) return;
    const hasContent = [
      normalized.influencer,
      normalized.platform,
      normalized.category,
      normalized.deliverableLinks,
      normalized.views,
      normalized.engagementCount
    ].some(Boolean);
    if (!hasContent) return;
    if (normalized.influencer && normalized.deliverableLinks) {
      previousResource = normalized;
    }
    rows.push(normalized);
  });
  return rows;
}

function numberValue(value: unknown) {
  const number = Number(value || 0);
  return Number.isFinite(number) ? number : 0;
}

function formatCount(value: unknown) {
  const number = numberValue(value);
  if (number <= 0) return "-";
  return number.toLocaleString("zh-CN");
}

function moneyText(value: unknown, currency = "USD") {
  const number = numberValue(value);
  if (number <= 0) return "-";
  return `${currency} ${number.toLocaleString("zh-CN", {
    maximumFractionDigits: 0
  })}`;
}

function projectStatusText(status: unknown) {
  const value = String(status || "").trim();
  if (/^active$/i.test(value)) return "进行中";
  if (/^paused$/i.test(value)) return "已暂停";
  if (/^completed$/i.test(value)) return "已完成";
  return value || "进行中";
}

function primaryReach(row: any) {
  return numberValue(row.impressions) || numberValue(row.views);
}

function ratioPercent(numerator: unknown, denominator: unknown) {
  const top = numberValue(numerator);
  const bottom = numberValue(denominator);
  if (top <= 0 || bottom <= 0) return "-";
  const percent = (top / bottom) * 100;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function cpmText(cost: unknown, reach: unknown, currency = "USD") {
  const costNumber = numberValue(cost);
  const reachNumber = numberValue(reach);
  if (costNumber <= 0 || reachNumber <= 0) return "-";
  return moneyText((costNumber / reachNumber) * 1000, currency);
}

function cooperationStage(row: any) {
  const status = `${row.status || ""} ${row.deliverableStatus || ""}`;
  if (/已发布|已完成|完成发布|数据回收/.test(status)) return "published";
  if (/待发布|排期|发布中/.test(status)) return "pending_publish";
  if (/制作|脚本|稿件|审核|修改|交付中/.test(status)) return "production";
  if (/确认合作|已确认|合作建立|待启动/.test(status)) return "confirmed";
  return "inviting";
}

function cooperationStageLabel(row: any) {
  return (
    pipelineStages.value.find(item => item.key === cooperationStage(row))
      ?.label || "邀约 / 议价"
  );
}

function cooperationStageTag(row: any) {
  const stage = cooperationStage(row);
  if (stage === "published") return "success";
  if (stage === "pending_publish") return "warning";
  if (stage === "production") return "primary";
  if (stage === "confirmed") return "info";
  return "warning";
}

function cooperationAction(row: any) {
  const stage = cooperationStage(row);
  if (stage === "inviting") return "确认报价与合作意向";
  if (stage === "confirmed") return "启动内容制作与交付";
  if (stage === "production") return "审核内容或跟进修改";
  if (stage === "pending_publish") return "确认发布排期与链接";
  if (
    stage === "published" &&
    primaryReach(row) <= 0 &&
    numberValue(row.clicks) <= 0
  ) {
    return "回收发布效果数据";
  }
  return "";
}

function updatedTimeText(row: any) {
  const value = row.updatedAt || row.createdAt || row.releaseDate;
  if (!value) return "等待跟进";
  const timestamp = new Date(value).getTime();
  if (!Number.isFinite(timestamp)) return "等待跟进";
  const days = Math.max(
    0,
    Math.floor((Date.now() - timestamp) / (24 * 60 * 60 * 1000))
  );
  if (days === 0) return "今天有更新";
  return `已等待 ${days} 天`;
}

function openExecutionDetail(row: any) {
  activeCooperation.value = row;
}

function openCampaignDetail(projectId = selectedProjectId.value) {
  if (!projectId) {
    ElMessage.warning("请先选择项目");
    return;
  }
  router.push({
    path: "/business/projects/detail",
    query: { id: String(projectId) }
  });
}

function editFromExecutionDetail() {
  if (!focusedCooperation.value) return;
  openEditCooperation(focusedCooperation.value);
}

function emptyProjectStats() {
  return {
    cooperationCount: 0,
    resourceCount: 0,
    resourceIds: new Set<number>(),
    totalReach: 0,
    totalViews: 0,
    totalEngagements: 0,
    totalCost: 0,
    bestItem: null as any
  };
}

function accumulateProjectStat(
  stat: ReturnType<typeof emptyProjectStats>,
  row: any
) {
  const reach = primaryReach(row);
  const resourceId = Number(row.resourceId || 0);
  stat.cooperationCount += 1;
  if (resourceId) stat.resourceIds.add(resourceId);
  stat.resourceCount = stat.resourceIds.size;
  stat.totalReach += reach;
  stat.totalViews += numberValue(row.views);
  stat.totalEngagements +=
    numberValue(row.engagementCount) + numberValue(row.commentsCount);
  stat.totalCost += numberValue(row.quoteAmount);
  if (!stat.bestItem || reach > primaryReach(stat.bestItem)) {
    stat.bestItem = row;
  }
}

function projectStats(projectId: number) {
  const stat = emptyProjectStats();
  cooperations.value
    .filter(item => Number(item.projectId) === Number(projectId))
    .forEach(item => accumulateProjectStat(stat, item));
  return stat;
}

function reviewSummaryText(stat: ReturnType<typeof emptyProjectStats>) {
  if (stat.cooperationCount === 0) return "暂无合作数据，等待导入或回填。";
  const cpm = cpmText(stat.totalCost, stat.totalReach);
  const cpmPart = cpm === "-" ? "未录入付费成本" : `CPM ${cpm}`;
  return `共合作 ${stat.resourceCount} 个资源 / ${stat.cooperationCount} 条内容，总触达 ${formatCount(stat.totalReach)}，互动率 ${ratioPercent(stat.totalEngagements, stat.totalReach)}，${cpmPart}。`;
}

function buildReviewInsight(
  name: string,
  stat: ReturnType<typeof emptyProjectStats>,
  overall = false
) {
  if (stat.cooperationCount === 0) {
    return `${name}暂无可复盘数据。建议优先回填合作资源、曝光或播放、互动及付费金额。`;
  }

  const engagementRate =
    stat.totalReach > 0 ? (stat.totalEngagements / stat.totalReach) * 100 : 0;
  const cpm =
    stat.totalCost > 0 && stat.totalReach > 0
      ? (stat.totalCost / stat.totalReach) * 1000
      : 0;
  const parts = [
    `${name}共合作 ${stat.resourceCount} 个达人/媒体，产出 ${stat.cooperationCount} 条合作内容，总触达 ${formatCount(stat.totalReach)}。`
  ];

  if (engagementRate > 0) {
    parts.push(
      `整体互动率为 ${ratioPercent(stat.totalEngagements, stat.totalReach)}，${engagementRate >= 3 ? "互动表现较好，可优先复用高互动内容方向" : "互动表现仍有提升空间，建议复盘内容选题与发布节奏"}。`
    );
  } else {
    parts.push("互动数据尚未完整回填，暂无法判断内容互动效果。");
  }

  if (cpm > 0) {
    parts.push(
      `付费 CPM 为 ${moneyText(cpm)}，建议结合目标市场及同类项目基准判断成本效率。`
    );
  } else {
    parts.push("付费金额或触达数据不完整，暂无法计算 CPM。");
  }

  if (!overall && stat.bestItem) {
    parts.push(
      `当前最高触达内容来自 ${stat.bestItem.resourceName || "未命名资源"}，触达 ${formatCount(primaryReach(stat.bestItem))}，可作为后续合作参考。`
    );
  }
  return parts.join("");
}

function reviewRowClassName({ row }: any) {
  return row.isSum ? "review-sum-row" : "";
}

function executionRowClassName({ row }: any) {
  return Number(row.id) === Number(focusedCooperation.value?.id)
    ? "execution-selected-row"
    : "";
}

function refreshImportRows() {
  const selected = new Set(selectedImportSheets.value);
  const rows = importWorkbookSheets.value
    .filter(sheet => selected.has(sheet.name))
    .flatMap(sheet => sheet.rows);
  const seen = new Set<string>();
  rows.forEach(row => {
    row.duplicate = false;
    const key = [row.influencer, row.platform, row.deliverableLinks]
      .join("|")
      .toLowerCase();
    if (row.influencer && row.platform && row.deliverableLinks) {
      if (seen.has(key)) row.duplicate = true;
      seen.add(key);
    }
  });
  importRows.value = rows;
  importPreviewLimit.value = 20;
}

function loadMoreImportRows() {
  if (!hasMoreImportRows.value) return;
  importPreviewLimit.value = Math.min(
    importPreviewLimit.value + 20,
    importRows.value.length
  );
}

function handleImportTableScroll({ scrollTop }: { scrollTop: number }) {
  const root = importPreviewTableRef.value?.$el || importPreviewTableRef.value;
  const scrollWrap = root?.querySelector?.(".el-scrollbar__wrap") as
    | HTMLElement
    | undefined;
  if (!scrollWrap) return;
  const position = Number.isFinite(scrollTop) ? scrollTop : scrollWrap.scrollTop;
  if (position + scrollWrap.clientHeight >= scrollWrap.scrollHeight - 48) {
    loadMoreImportRows();
  }
}

let importRowsRefreshFrame: number | undefined;

function scheduleImportRowsRefresh() {
  if (importRowsRefreshFrame) cancelAnimationFrame(importRowsRefreshFrame);
  importRowsRefreshFrame = requestAnimationFrame(() => {
    importRowsRefreshFrame = undefined;
    refreshImportRows();
  });
}

function normalizeImportPreviewSheet(sheet: any) {
  return {
    name: String(sheet?.name || ""),
    rows: Array.isArray(sheet?.rows)
      ? sheet.rows
          .filter(row => row && typeof row === "object")
          .map(row => ({
            ...row,
            errors: Array.isArray(row.errors) ? row.errors : [],
            duplicate: Boolean(row.duplicate)
          }))
      : []
  };
}

function handleImportSheetChange() {
  scheduleImportRowsRefresh();
}

function selectAllImportSheets() {
  selectedImportSheets.value = importWorkbookSheets.value.map(
    sheet => sheet.name
  );
  scheduleImportRowsRefresh();
}

function queryImportProjects(query: string, callback: (items: any[]) => void) {
  const keyword = query.trim().toLowerCase();
  callback(
    projects.value.filter(project =>
      !keyword || String(project.name || "").toLowerCase().includes(keyword)
    )
  );
}

function selectImportProject(project: any) {
  importProjectId.value = Number(project?.id) || null;
  importProjectNameDraft.value = String(project?.name || "");
}

async function ensureImportProject() {
  const name = importProjectNameDraft.value.trim();
  importProjectId.value = null;
  if (!name) {
    ElMessage.warning("请输入或选择归属项目");
    return false;
  }
  const existing = projects.value.find(project => project.name === name);
  if (existing) {
    importProjectId.value = Number(existing.id);
    importProjectNameDraft.value = String(existing.name || name);
    return true;
  }
  importProjectCreating.value = true;
  try {
    const res = await createProject({
      name,
      targetMarket: "",
      language: "",
      platform: "",
      campaignType: "内容导入",
      budget: 0,
      currency: "USD",
      status: "需求创建",
      owner: "",
      brief: "由合作内容导入时创建"
    });
    if (res.code !== 0) {
      ElMessage.warning(res.message || "项目创建失败");
      return false;
    }
    const id = Number(res.data?.id || 0);
    await loadData();
    const created = projects.value.find(project => Number(project.id) === id);
    if (!created) {
      ElMessage.warning("项目已创建，但未能读取项目编号，请重新选择");
      return false;
    }
    importProjectId.value = Number(created.id);
    importProjectNameDraft.value = String(created.name || name);
    ElMessage.success(`已创建项目「${created.name}」`);
    return true;
  } finally {
    importProjectCreating.value = false;
  }
}

async function removeProjects(rows: any[]) {
  const projectRows = rows.filter(row => Number(row?.id) > 0);
  if (!projectRows.length) return;
  try {
    await ElMessageBox.confirm(
      `确认删除 ${projectRows.length} 个项目吗？关联的达人、内容、合作记录和报表数据也会一并删除，且无法恢复。`,
      "删除项目",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" }
    );
  } catch {
    return;
  }
  const projectIDs = projectRows.map(project => Number(project.id));
  const res = await deleteProject({ ids: projectIDs });
  if (res.code !== 0) {
    ElMessage.warning(res.message || "项目删除失败");
    return;
  }
  if (projectIDs.includes(Number(importProjectId.value))) {
    importProjectId.value = null;
    importProjectNameDraft.value = "";
  }
  selectedProjectRows.value = selectedProjectRows.value.filter(
    project => !projectIDs.includes(Number(project.id))
  );
  await loadData();
  ElMessage.success(`已删除 ${res.data?.deleted || projectIDs.length} 个项目`);
}

async function handleUploadFile(file: any) {
  const rawFile = file.raw;
  if (!rawFile) return;
  // Render the modal before waiting for the backend. Without this yield, a large
  // workbook response can keep the browser busy long enough that the user never
  // sees the Sheet-selection dialog.
  importDialog.value = true;
  await nextTick();
  await new Promise<void>(resolve => requestAnimationFrame(() => resolve()));
  importParsing.value = true;
  importParseError.value = "";
  importRows.value = [];
  try {
    projectImportDialog.value = false;
    const res = await previewProjectExcelImport(rawFile);
    if (res.code !== 0) {
      importParseError.value = res.message || "Excel 解析失败";
      return;
    }
    importProjectId.value = null;
    importProjectNameDraft.value = "";
    importFileName.value = res.data?.fileName || rawFile.name;
    importWorkbookSheets.value = Array.isArray(res.data?.sheets)
      ? res.data.sheets
          .map(normalizeImportPreviewSheet)
          .filter(sheet => sheet.name)
      : [];
    // The preview response already contains each Sheet's parsed rows. Select
    // the non-empty ones by default so the dialog opens with a useful preview
    // instead of an empty Sheet selector and table.
    selectedImportSheets.value = importWorkbookSheets.value
      .filter(sheet => Array.isArray(sheet.rows) && sheet.rows.length > 0)
      .map(sheet => sheet.name);
    refreshImportRows();
  } catch {
    importParseError.value = "Excel 解析失败，请确认文件未损坏后重试。";
    ElMessage.error(importParseError.value);
  } finally {
    importParsing.value = false;
    contentUploadKey.value += 1;
  }
}

async function submitImport() {
  if (importProjectCreating.value) {
    ElMessage.warning("项目正在创建，请稍候");
    return;
  }
  if (!(await ensureImportProject())) return;
  if (validImportRows.value.length === 0) {
    ElMessage.warning("没有可导入的有效行");
    return;
  }
  importLoading.value = true;
  const res = await importCooperations({
    projectId: importProjectId.value,
    rows: validImportRows.value
  });
  importLoading.value = false;
  if (res.code === 0) {
    ElMessage.success(
      `导入成功 ${res.data.imported} 行，其中内容 ${res.data.importedContent || 0} 条；平台数据正在后台同步`
    );
    if (res.data.failed) {
      const failures = (res.data.errors || [])
        .slice(0, 3)
        .map((item: any) => `第 ${item.row || "-"} 行：${item.message || "导入失败"}`)
        .join("；");
      ElMessage.warning(
        `另有 ${res.data.failed} 行未导入${failures ? `：${failures}` : ""}`
      );
    }
    importDialog.value = false;
    importRows.value = [];
    loadData();
  }
}

function importRowClassName({ row }: any) {
  if (row.errors.length > 0) return "import-error-row";
  if (row.duplicate) return "import-duplicate-row";
  return "";
}

onMounted(() => {
  loadMarkets();
  loadData();
});
</script>

<template>
  <div class="business-projects-page">
    <div class="campaign-center">
      <header class="center-header">
        <div>
          <h1>营销项目执行中心</h1>
        </div>
        <div class="center-header-actions">
          <el-upload
            :key="contentUploadKey"
            accept=".xlsx,.xls,.csv"
            :auto-upload="false"
            :show-file-list="false"
            :on-change="handleUploadFile"
          >
            <el-button type="primary"
              ><IconifyIconOnline icon="ri:upload-2-line" /> 上传项目</el-button
            >
          </el-upload>
          <el-button @click="openQuickProject"
            ><IconifyIconOnline icon="ri:add-line" /> 创建项目</el-button
          >
        </div>
      </header>

      <section class="center-stats" aria-label="项目总体指标">
        <article>
          <span>进行中项目</span><strong>{{ centerOverview.active }}</strong
          ><small>全部 {{ projects.length }} 个项目</small>
        </article>
        <article>
          <span>合作达人</span><strong>{{ centerOverview.creators }}</strong
          ><small>跨项目去重统计</small>
        </article>
        <article>
          <span>已发布内容</span><strong>{{ centerOverview.content }}</strong
          ><small>已进入效果回收</small>
        </article>
        <article>
          <span>累计曝光 / 播放</span
          ><strong>{{ formatCount(centerOverview.reach) }}</strong
          ><small>达人内容真实数据</small>
        </article>
        <article>
          <span>达人投入</span
          ><strong>{{ moneyText(centerOverview.cost) }}</strong
          ><small>合作报价汇总</small>
        </article>
      </section>

      <section class="projects-workspace">
        <div class="projects-heading">
          <div>
            <h2>全部项目</h2>
            <p>选择项目进入概览、达人和内容工作台。</p>
          </div>
          <el-button link type="primary" @click="openCreateProject"
            >使用完整创建向导</el-button
          >
          <el-button
            type="danger"
            plain
            :disabled="selectedProjectRows.length === 0"
            @click="removeProjects(selectedProjectRows)"
          >
            删除所选{{ selectedProjectRows.length ? ` (${selectedProjectRows.length})` : "" }}
          </el-button>
        </div>
        <div class="projects-toolbar">
          <el-input
            v-model="projectSearch"
            clearable
            placeholder="搜索项目、市场、平台或负责人"
            class="project-search"
          >
            <template #prefix
              ><IconifyIconOnline icon="ri:search-line"
            /></template>
          </el-input>
          <el-select v-model="projectStatusFilter" class="status-filter">
            <el-option label="全部状态" value="all" />
            <el-option label="进行中" value="active" />
            <el-option label="已暂停" value="paused" />
            <el-option label="创建中" value="需求" />
          </el-select>
          <span>{{ visibleProjects.length }} 个项目</span>
        </div>
        <el-table
          :data="visibleProjects"
          class="projects-table"
          @row-click="row => openCampaignDetail(row.id)"
          @selection-change="rows => (selectedProjectRows = rows)"
        >
          <el-table-column type="selection" width="54" />
          <el-table-column label="项目" min-width="270" sortable>
            <template #default="{ row }">
              <div class="project-name-cell">
                <span class="project-icon"
                  ><IconifyIconOnline icon="ri:megaphone-line"
                /></span>
                <div>
                  <strong>{{ row.name }}</strong
                  ><small
                    >{{ row.campaignType || "达人营销" }} ·
                    {{ row.targetMarket || "未设置市场" }}</small
                  >
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="130" sortable>
            <template #default="{ row }"
              ><el-tag :type="projectStatusTag(row.status)" effect="plain">{{
                projectStatusText(row.status)
              }}</el-tag></template
            >
          </el-table-column>
          <el-table-column label="达人" width="100" align="center" sortable>
            <template #default="{ row }">{{
              projectStats(row.id).resourceCount
            }}</template>
          </el-table-column>
          <el-table-column label="内容" width="100" align="center" sortable>
            <template #default="{ row }">{{
              projectStats(row.id).cooperationCount
            }}</template>
          </el-table-column>
          <el-table-column
            label="曝光 / 播放"
            width="150"
            align="right"
            sortable
          >
            <template #default="{ row }">{{
              formatCount(projectStats(row.id).totalReach)
            }}</template>
          </el-table-column>
          <el-table-column label="互动率" width="120" align="right">
            <template #default="{ row }">{{
              ratioPercent(
                projectStats(row.id).totalEngagements,
                projectStats(row.id).totalReach
              )
            }}</template>
          </el-table-column>
          <el-table-column label="预算" width="150" align="right" sortable>
            <template #default="{ row }">{{
              moneyText(row.budget, row.currency)
            }}</template>
          </el-table-column>
          <el-table-column label="负责人" width="130"
            ><template #default="{ row }">{{
              row.owner || "未指定"
            }}</template></el-table-column
          >
          <el-table-column label="操作" width="166" fixed="right" align="right">
            <template #default="{ row }">
              <el-button
                link
                type="primary"
                @click.stop="openCampaignDetail(row.id)"
                >进入项目</el-button
              >
              <el-button link @click.stop="openEditProject(row)"
                >编辑</el-button
              >
              <el-button link type="danger" @click.stop="removeProjects([row])"
                >删除</el-button
              >
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!visibleProjects.length" description="没有匹配的项目" />
      </section>

      <el-dialog
        v-model="projectDialog"
        :title="editingProjectId ? '编辑项目' : '创建项目'"
        width="640px"
      >
        <el-form :model="projectForm" label-width="96px">
          <el-form-item label="项目名称"
            ><el-input
              v-model="projectForm.name"
              placeholder="例如：Infinix NOTE 60 新品推广"
          /></el-form-item>
          <el-form-item label="目标市场"
            ><el-select
              v-model="projectForm.targetMarket"
              allow-create
              filterable
              default-first-option
              class="w-full!"
              @change="handleMarketChange"
              ><el-option
                v-for="market in marketOptions"
                :key="market"
                :label="market"
                :value="market" /></el-select
          ></el-form-item>
          <el-form-item label="平台"
            ><el-input
              v-model="projectForm.platform"
              placeholder="TikTok, Instagram"
          /></el-form-item>
          <el-form-item label="合作目标"
            ><el-input v-model="projectForm.campaignType"
          /></el-form-item>
          <el-form-item label="预算"
            ><el-input-number
              v-model="projectForm.budget"
              :min="0"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="负责人"
            ><el-input v-model="projectForm.owner"
          /></el-form-item>
          <el-form-item label="项目说明"
            ><el-input v-model="projectForm.brief" type="textarea" :rows="4"
          /></el-form-item>
        </el-form>
        <template #footer
          ><el-button @click="projectDialog = false">取消</el-button
          ><el-button type="primary" @click="submitProject"
            >保存项目</el-button
          ></template
        >
      </el-dialog>

      <el-dialog v-model="projectImportDialog" title="上传项目" width="780px">
        <div class="project-import-intro">
          <div>
            <strong>从 Excel 批量创建项目</strong>
            <p>必填：项目名称。重复的「项目名称 + 目标市场」会自动跳过。</p>
          </div>
          <el-button link type="primary" @click="downloadProjectImportTemplate"
            >下载模板</el-button
          >
        </div>
        <el-upload
          :key="contentUploadKey"
          accept=".xlsx,.xls,.csv"
          :auto-upload="false"
          :show-file-list="false"
          :on-change="handleProjectImportFile"
        >
          <el-button
            ><IconifyIconOnline icon="ri:upload-2-line" /> 选择 Excel
            文件</el-button
          >
        </el-upload>
        <el-alert
          v-if="projectImportFileName"
          class="mt-3"
          type="info"
          :closable="false"
          :title="`文件：${projectImportFileName}，共 ${projectImportRows.length} 行，可导入 ${validProjectImportRows.length} 行，异常 ${invalidProjectImportRows.length} 行`"
        />
        <el-table
          v-if="projectImportRows.length"
          :data="projectImportRows"
          border
          height="360"
          class="mt-3"
          :row-class-name="importRowClassName"
        >
          <el-table-column prop="sourceSheet" label="Sheet" width="110" />
          <el-table-column prop="rowNo" label="行号" width="70" />
          <el-table-column prop="name" label="项目名称" min-width="180" />
          <el-table-column prop="targetMarket" label="目标市场" width="120" />
          <el-table-column prop="platform" label="平台" width="130" />
          <el-table-column
            prop="campaignType"
            label="合作目标"
            min-width="130"
          />
          <el-table-column
            prop="budget"
            label="预算"
            width="110"
            align="right"
          />
          <el-table-column prop="owner" label="负责人" width="110" />
          <el-table-column label="状态" min-width="150" fixed="right">
            <template #default="{ row }">
              <el-tag v-if="row.errors.length === 0" type="success"
                >可导入</el-tag
              >
              <el-tag v-if="row.duplicate" class="ml-2" type="warning"
                >文件内重复</el-tag
              >
              <el-tag v-if="row.errors.length" type="danger">{{
                row.errors.join("；")
              }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
        <el-empty
          v-else
          description="选择 Excel 后会在此处预览项目数据"
          :image-size="72"
        />
        <template #footer>
          <el-button @click="projectImportDialog = false">取消</el-button>
          <el-button
            type="primary"
            :loading="projectImportLoading"
            :disabled="!validProjectImportRows.length"
            @click="submitProjectImport"
            >导入 {{ validProjectImportRows.length }} 个项目</el-button
          >
        </template>
      </el-dialog>

      <el-dialog
        v-model="importDialog"
        title="确认项目内容导入"
        width="92%"
        top="5vh"
        append-to="body"
        :z-index="5000"
      >
        <el-alert
          v-if="importParsing"
          class="mb-3"
          type="info"
          :closable="false"
          title="正在读取 Excel 并识别可导入的 Sheet…"
        />
        <el-alert
          v-else-if="importParseError"
          class="mb-3"
          type="error"
          :closable="false"
          :title="importParseError"
        />
        <el-form label-width="80px" class="mb-3">
          <el-form-item label="解析 Sheet" required>
            <el-select
              v-model="selectedImportSheets"
              multiple
              collapse-tags
              collapse-tags-tooltip
              :teleported="false"
              class="import-sheet-select"
              placeholder="请选择要解析的 Sheet"
              @change="handleImportSheetChange"
            >
              <el-option
                v-for="sheet in importWorkbookSheets"
                :key="sheet.name"
                :label="`${sheet.name}（${sheet.rows.length} 条）`"
                :value="sheet.name"
              />
            </el-select>
            <el-button link type="primary" @click="selectAllImportSheets">
              全选
            </el-button>
          </el-form-item>
          <el-form-item label="项目名称" required>
            <el-autocomplete
              v-model="importProjectNameDraft"
              :fetch-suggestions="queryImportProjects"
              :trigger-on-focus="true"
              clearable
              class="import-project-select"
              placeholder="输入项目名称新建，或选择已有项目"
              :loading="importProjectCreating"
              @select="selectImportProject"
            >
              <template #default="{ item }">
                <div class="import-project-option">
                  <span>{{ item.name }}</span>
                  <small>{{ item.targetMarket || "未设置市场" }}</small>
                </div>
              </template>
            </el-autocomplete>
          </el-form-item>
        </el-form>
        <el-alert
          class="mb-3"
          type="info"
          :closable="false"
          :title="`当前 Excel 将作为一个项目导入。文件：${importFileName || '-'}，共 ${importRows.length} 行，可导入 ${validImportRows.length} 行，其中带发布链接的内容行 ${linkedImportRows.length} 行，异常 ${invalidImportRows.length} 行，疑似重复 ${duplicateImportRows.length} 行`"
        />
        <el-table
          ref="importPreviewTableRef"
          :data="visibleImportRows"
          border
          height="460"
          :row-class-name="importRowClassName"
          @scroll="handleImportTableScroll"
        >
          <el-table-column prop="sourceSheet" label="Sheet" width="140" fixed />
          <el-table-column prop="rowNo" label="行号" width="70" fixed />
          <el-table-column prop="influencer" label="姓名" min-width="140" />
          <el-table-column prop="category" label="领域" min-width="120" />
          <el-table-column prop="platform" label="平台" width="110" />
          <el-table-column prop="followerNumber" label="粉丝数" width="110" />
          <el-table-column prop="releaseDate" label="发布日期" width="120" />
          <el-table-column
            prop="deliverableLinks"
            label="发布链接"
            min-width="220"
            show-overflow-tooltip
          />
          <el-table-column prop="views" label="播放量" width="110" />
          <el-table-column
            prop="engagementCount"
            label="转赞藏数"
            width="110"
          />
          <el-table-column prop="commentsCount" label="评论数" width="100" />
          <el-table-column prop="quoteAmount" label="报价/费用" width="110" />
          <el-table-column prop="rating" label="评级" width="90" />
          <el-table-column label="状态" min-width="180" fixed="right">
            <template #default="{ row }">
              <el-tag v-if="row.errors.length === 0" type="success"
                >可导入</el-tag
              >
              <el-tag v-if="row.duplicate" class="ml-2" type="warning"
                >疑似重复</el-tag
              >
              <el-tag v-if="row.errors.length > 0" type="danger">
                {{ row.errors.join("；") }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
        <p v-if="hasMoreImportRows" class="import-preview-more">
          已展示 {{ visibleImportRows.length }} / {{ importRows.length }} 条，向下滚动加载更多
        </p>
        <template #footer>
          <el-button @click="importDialog = false">取消</el-button>
          <el-button
            type="primary"
            :loading="importLoading || importProjectCreating"
            :disabled="
              importParsing ||
              Boolean(importParseError) ||
              selectedImportSheets.length === 0 ||
              validImportRows.length === 0 ||
              importProjectCreating
            "
            @click="submitImport"
          >
            确认导入 {{ validImportRows.length }} 行（内容 {{ linkedImportRows.length }} 条）
          </el-button>
        </template>
      </el-dialog>
    </div>

    <div v-if="false" class="business-page">
      <section class="page-hero">
        <div>
          <span>项目运营工作台</span>
          <h1>营销项目执行中心</h1>
          <p>
            从达人邀约、议价、内容交付到发布复盘，统一掌握每个营销项目
            的执行节奏与待处理事项。
          </p>
        </div>
        <el-button type="primary" @click="openCreateProject">
          <IconifyIconOnline icon="ri:add-line" class="mr-1" />
          创建营销项目
        </el-button>
      </section>

      <section class="review-summary-grid">
        <div>
          <span>总体合作资源</span>
          <strong>{{ overallReview.resourceCount }}</strong>
          <p>已回填合作记录 {{ overallReview.cooperationCount }} 条</p>
        </div>
        <div>
          <span>总体曝光 / 播放</span>
          <strong>{{ formatCount(overallReview.totalReach) }}</strong>
          <p>播放 {{ formatCount(overallReview.totalViews) }}</p>
        </div>
        <div>
          <span>总体互动率</span>
          <strong>
            {{
              ratioPercent(
                overallReview.totalEngagements,
                overallReview.totalReach
              )
            }}
          </strong>
          <p>转赞藏评 {{ formatCount(overallReview.totalEngagements) }}</p>
        </div>
        <div>
          <span>付费 CPM</span>
          <strong>{{
            cpmText(overallReview.totalCost, overallReview.totalReach)
          }}</strong>
          <p>成本 {{ moneyText(overallReview.totalCost) }}</p>
        </div>
      </section>

      <el-card shadow="never" class="workspace-card">
        <el-tabs>
          <el-tab-pane label="执行总览">
            <section class="overview-dashboard">
              <header class="overview-header">
                <div>
                  <span>营销项目总览</span>
                  <el-select
                    v-model="selectedProjectId"
                    filterable
                    placeholder="选择营销项目"
                  >
                    <el-option
                      v-for="project in projects"
                      :key="project.id"
                      :label="project.name"
                      :value="project.id"
                    />
                  </el-select>
                  <h2>{{ selectedProject?.name || "暂无营销项目" }}</h2>
                  <p>
                    目标：{{ selectedProject?.campaignType || "未设置" }}
                    <span />
                    市场：{{ selectedProject?.targetMarket || "未设置" }}
                    <span />
                    负责人：{{ selectedProject?.owner || "未指定" }}
                  </p>
                </div>
                <div class="overview-actions">
                  <el-tag type="success" round>
                    {{ projectStatusText(selectedProject?.status) }}
                  </el-tag>
                  <el-button
                    type="primary"
                    :disabled="!selectedProject"
                    @click="openCampaignDetail()"
                  >
                    <IconifyIconOnline icon="ri:external-link-line" />
                    进入执行页
                  </el-button>
                  <el-button
                    :disabled="!selectedProject"
                    @click="openEditProject(selectedProject)"
                  >
                    编辑项目
                  </el-button>
                </div>
              </header>

              <section class="overview-metrics">
                <article>
                  <span>合作资源</span>
                  <strong>{{ selectedProjectReview.resourceCount }}</strong>
                  <p>{{ selectedProjectReview.cooperationCount }} 条合作记录</p>
                </article>
                <article>
                  <span>预算使用</span>
                  <strong>{{
                    moneyText(campaignHealth.spent, selectedProject?.currency)
                  }}</strong>
                  <p>
                    总预算
                    {{
                      moneyText(
                        campaignHealth.budget,
                        selectedProject?.currency
                      )
                    }}
                  </p>
                </article>
                <article>
                  <span>发布完成率</span>
                  <strong>{{ campaignHealth.completionRate }}%</strong>
                  <p>{{ campaignHealth.published }} 条内容已发布</p>
                </article>
                <article>
                  <span>触达 / CPM</span>
                  <strong>{{
                    formatCount(selectedProjectReview.totalReach)
                  }}</strong>
                  <p>
                    {{
                      cpmText(
                        selectedProjectReview.totalCost,
                        selectedProjectReview.totalReach
                      )
                    }}
                  </p>
                </article>
              </section>

              <section class="overview-flow">
                <button
                  v-for="stage in pipelineStages"
                  :key="stage.key"
                  type="button"
                  :class="{ active: activePipelineStage === stage.key }"
                  @click="activePipelineStage = stage.key"
                >
                  <IconifyIconOnline :icon="stage.icon" />
                  <span>{{ stage.label }}</span>
                  <strong>{{ stage.count }}</strong>
                </button>
              </section>

              <section class="overview-grid">
                <article class="overview-panel">
                  <div class="section-heading">
                    <div>
                      <strong>待处理事项</strong>
                      <span>{{ pendingActions.length }} 项需要人工确认</span>
                    </div>
                  </div>
                  <div
                    v-if="pendingActions.length"
                    class="overview-pending-list"
                  >
                    <button
                      v-for="item in pendingActions"
                      :key="item.id"
                      type="button"
                      @click="openCampaignDetail(item.projectId)"
                    >
                      <div class="creator-avatar">
                        {{ String(item.resourceName || "R").slice(0, 1) }}
                      </div>
                      <div>
                        <strong>{{ item.action }}</strong>
                        <p>
                          {{ item.resourceName || "未命名资源" }} ·
                          {{ updatedTimeText(item) }}
                        </p>
                      </div>
                      <IconifyIconOnline icon="ri:arrow-right-s-line" />
                    </button>
                  </div>
                  <el-empty v-else description="当前没有需要人工处理的动作" />
                </article>

                <article class="overview-panel">
                  <div class="section-heading">
                    <div>
                      <strong>最近合作</strong>
                      <span>查看关键状态，详细交付进入执行页处理</span>
                    </div>
                    <el-button
                      link
                      type="primary"
                      @click="openCreateCooperation"
                    >
                      新增合作
                    </el-button>
                  </div>
                  <el-table
                    :data="pipelineRows.slice(0, 6)"
                    stripe
                    class="business-table"
                  >
                    <el-table-column
                      prop="resourceName"
                      label="资源"
                      min-width="160"
                    />
                    <el-table-column label="阶段" width="140">
                      <template #default="{ row }">
                        <el-tag :type="cooperationStageTag(row)" effect="light">
                          {{ cooperationStageLabel(row) }}
                        </el-tag>
                      </template>
                    </el-table-column>
                    <el-table-column label="报价" width="120">
                      <template #default="{ row }">
                        {{ moneyText(row.quoteAmount, row.currency) }}
                      </template>
                    </el-table-column>
                    <el-table-column label="操作" width="110">
                      <template #default="{ row }">
                        <el-button
                          link
                          type="primary"
                          @click="openCampaignDetail(row.projectId)"
                        >
                          处理
                        </el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                </article>
              </section>
            </section>

            <div v-if="false" class="campaign-detail-shell">
              <aside class="campaign-side-nav">
                <button type="button" class="active">
                  <IconifyIconOnline icon="ri:team-line" />
                  <span>协作执行</span>
                </button>
                <button type="button">
                  <IconifyIconOnline icon="ri:mail-line" />
                  <span>Cold email</span>
                </button>
                <button type="button">
                  <IconifyIconOnline icon="ri:bar-chart-box-line" />
                  <span>报告与结算</span>
                </button>
                <button type="button">
                  <IconifyIconOnline icon="ri:wallet-3-line" />
                  <span>Budget</span>
                </button>
                <button type="button">
                  <IconifyIconOnline icon="ri:file-list-3-line" />
                  <span>项目信息</span>
                </button>
              </aside>

              <section class="campaign-detail-main">
                <header class="campaign-topbar">
                  <div class="campaign-title-block">
                    <el-button circle text @click="activePipelineStage = 'all'">
                      <IconifyIconOnline icon="ri:arrow-left-line" />
                    </el-button>
                    <div class="campaign-logo-box">
                      <IconifyIconOnline icon="ri:megaphone-line" />
                    </div>
                    <div>
                      <el-select
                        v-model="selectedProjectId"
                        filterable
                        placeholder="选择项目"
                      >
                        <el-option
                          v-for="project in projects"
                          :key="project.id"
                          :label="project.name"
                          :value="project.id"
                        />
                      </el-select>
                      <h2>{{ selectedProject?.name || "暂无项目" }}</h2>
                      <p>
                        Objective:
                        {{ selectedProject?.campaignType || "未设置合作目标" }}
                        <span />
                        Market:
                        {{ selectedProject?.targetMarket || "未设置市场" }}
                        <span />
                        负责人：
                        {{ selectedProject?.owner || "未指定负责人" }}
                      </p>
                    </div>
                  </div>
                  <div class="campaign-topbar-actions">
                    <el-tag type="success" round>
                      {{ projectStatusText(selectedProject?.status) }}
                    </el-tag>
                    <el-button
                      type="primary"
                      :disabled="!selectedProject"
                      @click="openCampaignDetail()"
                    >
                      <IconifyIconOnline icon="ri:external-link-line" />
                      进入执行页
                    </el-button>
                    <el-button
                      :disabled="!selectedProject"
                      @click="openEditProject(selectedProject)"
                    >
                      <IconifyIconOnline icon="ri:pause-line" />
                      编辑
                    </el-button>
                    <el-button circle>
                      <IconifyIconOnline icon="ri:more-line" />
                    </el-button>
                  </div>
                </header>

                <section class="campaign-progress-card">
                  <div
                    v-for="(stage, index) in pipelineStages"
                    :key="stage.key"
                    class="campaign-progress-step"
                    :class="{
                      active: activePipelineStage === stage.key,
                      done: stage.count > 0 || index < 2
                    }"
                    @click="activePipelineStage = stage.key"
                  >
                    <span>
                      <IconifyIconOnline
                        :icon="
                          stage.count > 0 || index < 2
                            ? 'ri:check-line'
                            : stage.icon
                        "
                      />
                    </span>
                    <div>
                      <strong>{{ stage.label }}</strong>
                      <p>{{ stage.count }} 位达人 · {{ stage.description }}</p>
                    </div>
                  </div>
                </section>

                <section class="collaboration-panel">
                  <div class="collaboration-heading">
                    <div>
                      <h2>协作执行</h2>
                      <p>
                        {{ selectedProject?.platform || "全平台" }} ·
                        {{ selectedProjectReview.cooperationCount }}
                        条合作执行记录
                      </p>
                    </div>
                    <el-button type="primary" @click="openCreateCooperation">
                      <IconifyIconOnline icon="ri:add-line" />
                      新增合作
                    </el-button>
                  </div>

                  <div class="assurance-strip">
                    <div>
                      <IconifyIconOnline icon="ri:user-star-line" />
                      <strong>真实内容流量</strong>
                      <span>真实达人内容沉淀为可复盘数据</span>
                    </div>
                    <div>
                      <IconifyIconOnline icon="ri:shield-check-line" />
                      <strong>发布保障</strong>
                      <span
                        >{{ campaignHealth.completionRate }}% 发布完成率</span
                      >
                    </div>
                    <div>
                      <IconifyIconOnline icon="ri:line-chart-line" />
                      <strong>当前 CPM</strong>
                      <span>
                        {{
                          cpmText(
                            selectedProjectReview.totalCost,
                            selectedProjectReview.totalReach
                          )
                        }}
                      </span>
                    </div>
                  </div>

                  <section
                    v-if="campaignHealth.missingData > 0"
                    class="tip-bar"
                  >
                    <strong>Tip!</strong>
                    <span>
                      有 {{ campaignHealth.missingData }}
                      条已发布内容尚未回收效果数据，建议补充曝光、播放或点击。
                    </span>
                  </section>

                  <section class="pending-actions-row">
                    <div class="section-heading">
                      <div>
                        <strong>待处理事项</strong>
                        <span>{{ pendingActions.length }} 项需要人工确认</span>
                      </div>
                      <el-tag effect="plain"
                        >All {{ selectedCooperations.length }}</el-tag
                      >
                    </div>
                    <div v-if="pendingActions.length" class="pending-card-row">
                      <button
                        v-for="item in pendingActions"
                        :key="item.id"
                        type="button"
                        :class="{
                          active:
                            Number(item.id) === Number(focusedCooperation?.id)
                        }"
                        @click="openExecutionDetail(item)"
                      >
                        <div class="creator-avatar">
                          {{ String(item.resourceName || "R").slice(0, 1) }}
                        </div>
                        <div>
                          <span>{{ item.action }}</span>
                          <strong>{{
                            item.resourceName || "未命名资源"
                          }}</strong>
                          <p>{{ updatedTimeText(item) }} · 等待确认</p>
                        </div>
                        <IconifyIconOnline icon="ri:arrow-right-s-line" />
                      </button>
                    </div>
                    <el-empty v-else description="当前没有需要人工处理的动作" />
                  </section>

                  <section class="influencer-workspace">
                    <div class="influencer-list-panel">
                      <div class="section-heading">
                        <div>
                          <strong>Influencer</strong>
                          <span>点击行即可在页面内查看交付详情</span>
                        </div>
                      </div>
                      <div class="stage-filter">
                        <button
                          type="button"
                          :class="{ active: activePipelineStage === 'all' }"
                          @click="activePipelineStage = 'all'"
                        >
                          All {{ selectedCooperations.length }}
                        </button>
                        <button
                          v-for="stage in pipelineStages"
                          :key="`filter-${stage.key}`"
                          type="button"
                          :class="{ active: activePipelineStage === stage.key }"
                          @click="activePipelineStage = stage.key"
                        >
                          {{ stage.label }} {{ stage.count }}
                        </button>
                      </div>
                      <el-table
                        :data="pipelineRows"
                        stripe
                        class="business-table influencer-table"
                        :row-class-name="executionRowClassName"
                        @row-click="openExecutionDetail"
                      >
                        <el-table-column
                          prop="resourceName"
                          label="Name"
                          min-width="180"
                        />
                        <el-table-column label="Status" width="150">
                          <template #default="{ row }">
                            <el-tag
                              :type="cooperationStageTag(row)"
                              effect="light"
                            >
                              {{ cooperationStageLabel(row) }}
                            </el-tag>
                          </template>
                        </el-table-column>
                        <el-table-column label="Best price" width="130">
                          <template #default="{ row }">
                            {{ moneyText(row.quoteAmount, row.currency) }}
                          </template>
                        </el-table-column>
                        <el-table-column label="Lowest CPM" width="130">
                          <template #default="{ row }">
                            {{
                              cpmText(
                                row.quoteAmount,
                                primaryReach(row),
                                row.currency
                              )
                            }}
                          </template>
                        </el-table-column>
                        <el-table-column
                          label="Actions"
                          width="110"
                          fixed="right"
                        >
                          <template #default="{ row }">
                            <el-button
                              circle
                              size="small"
                              @click.stop="openExecutionDetail(row)"
                            >
                              <IconifyIconOnline icon="ri:star-line" />
                            </el-button>
                          </template>
                        </el-table-column>
                      </el-table>
                    </div>

                    <article
                      v-if="focusedCooperation"
                      class="creator-detail-panel"
                    >
                      <header class="creator-detail-header">
                        <div class="creator-avatar large">
                          {{
                            String(
                              focusedCooperation.resourceName || "R"
                            ).slice(0, 1)
                          }}
                        </div>
                        <div>
                          <h3>
                            {{
                              focusedCooperation.resourceName || "未命名资源"
                            }}
                          </h3>
                          <p>
                            {{
                              focusedCooperation.projectName ||
                              selectedProject?.name
                            }}
                            <span />
                            {{
                              focusedCooperation.cooperationType ||
                              "未设置合作形式"
                            }}
                            <span />
                            {{ selectedProject?.platform || "全平台" }}
                          </p>
                        </div>
                        <el-button @click="editFromExecutionDetail">
                          更新记录
                        </el-button>
                      </header>

                      <div class="quality-strip">
                        <strong>
                          <IconifyIconOnline icon="ri:verified-badge-line" />
                          系统辅助评估合作质量
                        </strong>
                        <div>
                          <span>近期活跃</span>
                          <span>互动表现较好</span>
                          <span>数据可信</span>
                          <span>履约记录良好</span>
                        </div>
                      </div>

                      <div class="detail-tabs">
                        <button type="button">概览</button>
                        <button type="button" class="active">内容交付</button>
                        <button type="button">评价</button>
                      </div>

                      <section class="content-info-block">
                        <h3>内容信息</h3>
                        <div class="tracking-link">
                          <span>
                            为该资源生成的专属追踪链接，用于发布内容效果追踪。
                          </span>
                          <el-link
                            v-if="focusedCooperation.deliverableLinks"
                            type="primary"
                            :href="focusedCooperation.deliverableLinks"
                            target="_blank"
                          >
                            {{ focusedCooperation.deliverableLinks }}
                          </el-link>
                          <span v-else>等待达人提交内容链接</span>
                        </div>
                      </section>

                      <section class="delivery-timeline">
                        <div
                          v-for="stage in pipelineStages"
                          :key="`page-timeline-${stage.key}`"
                          :class="{
                            completed:
                              pipelineStages.findIndex(
                                item => item.key === stage.key
                              ) <=
                              pipelineStages.findIndex(
                                item =>
                                  item.key ===
                                  cooperationStage(focusedCooperation)
                              )
                          }"
                        >
                          <span />
                          <article>
                            <div>
                              <strong>{{ stage.label }}</strong>
                              <el-tag
                                v-if="
                                  stage.key ===
                                  cooperationStage(focusedCooperation)
                                "
                                size="small"
                                effect="plain"
                              >
                                {{
                                  focusedCooperation.deliverableStatus ||
                                  "进行中"
                                }}
                              </el-tag>
                            </div>
                            <p>
                              {{
                                stage.key ===
                                cooperationStage(focusedCooperation)
                                  ? updatedTimeText(focusedCooperation)
                                  : stage.description
                              }}
                            </p>
                            <div
                              v-if="
                                stage.key ===
                                cooperationStage(focusedCooperation)
                              "
                              class="timeline-note"
                            >
                              <strong>
                                {{
                                  cooperationAction(focusedCooperation) ||
                                  "等待效果复盘"
                                }}
                              </strong>
                              <p>
                                报价
                                {{
                                  moneyText(
                                    focusedCooperation.quoteAmount,
                                    focusedCooperation.currency
                                  )
                                }}
                                · 触达
                                {{
                                  formatCount(primaryReach(focusedCooperation))
                                }}
                                · 互动率
                                {{
                                  ratioPercent(
                                    numberValue(
                                      focusedCooperation.engagementCount
                                    ) +
                                      numberValue(
                                        focusedCooperation.commentsCount
                                      ),
                                    primaryReach(focusedCooperation)
                                  )
                                }}
                              </p>
                            </div>
                          </article>
                        </div>
                      </section>
                    </article>
                  </section>
                </section>
              </section>
            </div>
          </el-tab-pane>
          <el-tab-pane label="项目管理">
            <div class="toolbar">
              <span class="toolbar-title">营销项目需求池</span>
            </div>
            <el-table :data="projects" stripe class="business-table">
              <el-table-column prop="name" label="项目名称" min-width="180" />
              <el-table-column
                prop="targetMarket"
                label="目标市场"
                width="120"
              />
              <el-table-column prop="platform" label="平台" width="140" />
              <el-table-column
                prop="campaignType"
                label="合作目标"
                width="120"
              />
              <el-table-column prop="budget" label="预算" width="120" />
              <el-table-column label="状态" width="120">
                <template #default="{ row }">
                  {{ projectStatusText(row.status) }}
                </template>
              </el-table-column>
              <el-table-column prop="owner" label="负责人" width="120" />
              <el-table-column label="合作复盘" min-width="250">
                <template #default="{ row }">
                  {{ reviewSummaryText(projectStats(row.id)) }}
                </template>
              </el-table-column>
              <el-table-column
                prop="brief"
                label="需求摘要"
                min-width="260"
                show-overflow-tooltip
              />
              <el-table-column label="操作" width="150" fixed="right">
                <template #default="{ row }">
                  <el-button
                    link
                    type="primary"
                    @click="openCampaignDetail(row.id)"
                  >
                    进入
                  </el-button>
                  <el-button link type="primary" @click="openEditProject(row)">
                    编辑
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
          <el-tab-pane label="效果复盘">
            <section class="review-report">
              <div class="review-report-heading">
                <div>
                  <strong>项目效果复盘</strong>
                  <span
                    >逐项目展示达人/媒体、曝光、互动、CPM，最后一行为 SUM
                    总和</span
                  >
                </div>
                <el-tag effect="plain">自动汇总</el-tag>
              </div>
              <el-table
                :data="projectReviewTableRows"
                border
                class="business-table"
                :row-class-name="reviewRowClassName"
              >
                <el-table-column
                  prop="name"
                  label="项目"
                  min-width="180"
                  fixed
                />
                <el-table-column prop="targetMarket" label="市场" width="120" />
                <el-table-column prop="platform" label="平台" width="130" />
                <el-table-column label="合作达人 / 媒体" width="140">
                  <template #default="{ row }">
                    {{ row.review.resourceCount }}
                  </template>
                </el-table-column>
                <el-table-column label="合作内容" width="110">
                  <template #default="{ row }">
                    {{ row.review.cooperationCount }}
                  </template>
                </el-table-column>
                <el-table-column label="曝光 / 触达" width="140">
                  <template #default="{ row }">
                    {{ formatCount(row.review.totalReach) }}
                  </template>
                </el-table-column>
                <el-table-column label="互动" width="120">
                  <template #default="{ row }">
                    {{ formatCount(row.review.totalEngagements) }}
                  </template>
                </el-table-column>
                <el-table-column label="互动率" width="110">
                  <template #default="{ row }">
                    {{
                      ratioPercent(
                        row.review.totalEngagements,
                        row.review.totalReach
                      )
                    }}
                  </template>
                </el-table-column>
                <el-table-column label="付费金额" width="130">
                  <template #default="{ row }">
                    {{ moneyText(row.review.totalCost) }}
                  </template>
                </el-table-column>
                <el-table-column label="CPM" width="120">
                  <template #default="{ row }">
                    {{ cpmText(row.review.totalCost, row.review.totalReach) }}
                  </template>
                </el-table-column>
              </el-table>
            </section>

            <section class="ai-review-summary">
              <div class="ai-review-title">
                <IconifyIconOnline icon="ri:sparkling-2-line" />
                <div>
                  <strong>智能复盘摘要（草稿）</strong>
                  <span>根据当前已回填数据自动生成，提交复盘前请人工确认</span>
                </div>
              </div>
              <p>{{ overallReviewInsight }}</p>
            </section>

            <div class="project-review-list">
              <article v-for="project in projectReviewRows" :key="project.id">
                <div class="project-review-head">
                  <div>
                    <strong>{{ project.name }}</strong>
                    <span>
                      {{ project.targetMarket || "-" }} ·
                      {{ project.platform || "全平台" }} ·
                      {{ project.owner || "未指定负责人" }}
                    </span>
                  </div>
                  <div class="project-card-actions">
                    <el-tag effect="plain">{{
                      projectStatusText(project.status)
                    }}</el-tag>
                    <el-button
                      link
                      type="primary"
                      @click="openEditProject(project)"
                    >
                      编辑
                    </el-button>
                  </div>
                </div>
                <div class="project-review-metrics">
                  <div>
                    <span>达人 / 媒体</span>
                    <strong>{{ project.review.resourceCount }}</strong>
                  </div>
                  <div>
                    <span>总触达</span>
                    <strong>{{
                      formatCount(project.review.totalReach)
                    }}</strong>
                  </div>
                  <div>
                    <span>总互动</span>
                    <strong>{{
                      formatCount(project.review.totalEngagements)
                    }}</strong>
                  </div>
                  <div>
                    <span>CPM</span>
                    <strong>
                      {{
                        cpmText(
                          project.review.totalCost,
                          project.review.totalReach
                        )
                      }}
                    </strong>
                  </div>
                </div>
                <div class="project-ai-insight">
                  <strong>智能效果摘要</strong>
                  <p>{{ buildReviewInsight(project.name, project.review) }}</p>
                </div>
                <div v-if="project.review.bestItem" class="best-item">
                  <span>最高触达内容</span>
                  <el-link
                    v-if="project.review.bestItem.deliverableLinks"
                    type="primary"
                    :href="project.review.bestItem.deliverableLinks"
                    target="_blank"
                  >
                    {{ project.review.bestItem.resourceName }}
                  </el-link>
                  <strong v-else>{{
                    project.review.bestItem.resourceName
                  }}</strong>
                  <span>{{
                    formatCount(primaryReach(project.review.bestItem))
                  }}</span>
                </div>
              </article>
            </div>
          </el-tab-pane>
          <el-tab-pane label="合作记录">
            <div class="toolbar">
              <el-upload
                accept=".xlsx,.xls,.csv"
                :auto-upload="false"
                :show-file-list="false"
                :on-change="handleUploadFile"
              >
                <el-button>上传数据表</el-button>
              </el-upload>
              <el-button type="primary" @click="openCreateCooperation">
                <IconifyIconOnline icon="ri:add-line" class="mr-1" />
                新增合作
              </el-button>
            </div>
            <el-table :data="cooperations" stripe class="business-table">
              <el-table-column
                prop="projectName"
                label="项目"
                min-width="160"
              />
              <el-table-column
                prop="resourceName"
                label="资源"
                min-width="160"
              />
              <el-table-column
                prop="cooperationType"
                label="合作形式"
                width="120"
              />
              <el-table-column label="报价" width="120">
                <template #default="{ row }">
                  {{ moneyText(row.quoteAmount, row.currency) }}
                </template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="120" />
              <el-table-column
                prop="deliverableStatus"
                label="交付状态"
                width="120"
              />
              <el-table-column prop="impressions" label="曝光" width="110" />
              <el-table-column prop="views" label="播放量" width="110" />
              <el-table-column
                prop="engagementCount"
                label="转赞藏"
                width="110"
              />
              <el-table-column prop="commentsCount" label="评论" width="90" />
              <el-table-column label="互动率" width="100">
                <template #default="{ row }">
                  {{
                    ratioPercent(
                      numberValue(row.engagementCount) +
                        numberValue(row.commentsCount),
                      primaryReach(row)
                    )
                  }}
                </template>
              </el-table-column>
              <el-table-column label="CPM" width="110">
                <template #default="{ row }">
                  {{
                    cpmText(row.quoteAmount, primaryReach(row), row.currency)
                  }}
                </template>
              </el-table-column>
              <el-table-column
                prop="releaseDate"
                label="发布日期"
                width="120"
              />
              <el-table-column
                prop="deliverableLinks"
                label="发布链接"
                min-width="180"
              >
                <template #default="{ row }">
                  <el-link
                    v-if="row.deliverableLinks"
                    type="primary"
                    :href="row.deliverableLinks"
                    target="_blank"
                  >
                    {{ row.deliverableLinks }}
                  </el-link>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column prop="clicks" label="点击" width="110" />
              <el-table-column prop="roi" label="ROI" width="90" />
              <el-table-column
                prop="notes"
                label="备注"
                min-width="200"
                show-overflow-tooltip
              />
              <el-table-column label="操作" width="150" fixed="right">
                <template #default="{ row }">
                  <el-button
                    link
                    type="primary"
                    :loading="!!syncingCooperationIds[Number(row.id || 0)]"
                    @click="syncCooperationPost(row)"
                  >
                    同步作品
                  </el-button>
                  <el-button
                    link
                    type="primary"
                    @click="openEditCooperation(row)"
                  >
                    编辑
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </el-card>

      <el-dialog
        v-model="campaignWizardDialog"
        title="新建营销项目"
        width="1080px"
        top="4vh"
        class="campaign-wizard-dialog"
      >
        <div class="campaign-wizard-layout">
          <aside class="wizard-rail">
            <el-steps
              :active="wizardActiveStep"
              direction="vertical"
              finish-status="success"
            >
              <el-step
                v-for="step in wizardSteps"
                :key="step.key"
                :title="step.label"
                :description="step.description"
              />
            </el-steps>
          </aside>

          <section class="wizard-stage">
            <div v-if="wizardActiveStep === 0" class="wizard-section">
              <div class="ai-generator">
                <div>
                  <strong>智能生成项目基础信息</strong>
                  <p>
                    输入官网 URL 后生成品牌、产品和项目初稿，再由你校准后发布。
                  </p>
                </div>
                <div class="generator-row">
                  <el-input
                    v-model="campaignWizardForm.websiteUrl"
                    placeholder="https://example.com"
                    clearable
                  />
                  <el-button
                    type="primary"
                    :loading="wizardGenerating"
                    @click="generateCampaignProfile"
                  >
                    生成
                  </el-button>
                </div>
              </div>

              <el-alert
                type="info"
                :closable="false"
                title="以下内容由创建向导生成，请根据实际项目检查并调整。"
              />

              <div class="wizard-form-card">
                <div class="wizard-card-title">
                  <strong>品牌与项目基础信息</strong>
                  <span>基础信息</span>
                </div>
                <el-form label-position="top">
                  <div class="logo-upload-row">
                    <div class="wizard-logo-preview">
                      {{ campaignWizardForm.businessLogo || "A" }}
                    </div>
                    <el-button
                      @click="
                        campaignWizardForm.businessLogo =
                          campaignWizardForm.businessName
                            .slice(0, 1)
                            .toUpperCase()
                      "
                    >
                      生成首字母标识
                    </el-button>
                    <span>如需正式 Logo，可后续在项目资料中补充。</span>
                  </div>
                  <div class="wizard-two-col">
                    <el-form-item label="品牌名称">
                      <el-input v-model="campaignWizardForm.businessName" />
                    </el-form-item>
                    <el-form-item label="产品/服务类型">
                      <el-select
                        v-model="campaignWizardForm.productType"
                        class="w-full!"
                      >
                        <el-option
                          v-for="item in productTypeOptions"
                          :key="item"
                          :label="item"
                          :value="item"
                        />
                      </el-select>
                    </el-form-item>
                  </div>
                  <el-form-item label="可搜索品牌">
                    <el-input v-model="campaignWizardForm.searchableBrands" />
                  </el-form-item>
                  <el-form-item label="品牌介绍">
                    <el-input
                      v-model="campaignWizardForm.businessIntroduction"
                      type="textarea"
                      :rows="4"
                    />
                  </el-form-item>
                </el-form>
              </div>

              <div class="campaign-examples">
                <article
                  v-for="item in campaignExampleTemplates"
                  :key="item.title"
                >
                  <strong>{{ item.title }}</strong>
                  <p>{{ item.summary }}</p>
                </article>
              </div>
            </div>

            <div v-else-if="wizardActiveStep === 1" class="wizard-section">
              <div class="wizard-card-title">
                <strong>达人匹配设置</strong>
                <span>控制匹配规模、成本、人群和审批策略。</span>
              </div>
              <el-form label-position="top">
                <div class="wizard-two-col">
                  <el-form-item label="项目目标">
                    <el-input v-model="campaignWizardForm.campaignType" />
                  </el-form-item>
                  <el-form-item label="目标市场">
                    <el-select
                      v-model="campaignWizardForm.targetMarket"
                      allow-create
                      filterable
                      default-first-option
                      class="w-full!"
                    >
                      <el-option
                        v-for="market in marketOptions"
                        :key="market"
                        :label="market"
                        :value="market"
                      />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="投放平台">
                    <el-select
                      v-model="campaignWizardForm.platform"
                      multiple
                      class="w-full!"
                    >
                      <el-option
                        v-for="item in platformOptions"
                        :key="item"
                        :label="item"
                        :value="item"
                      />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="语言">
                    <el-input v-model="campaignWizardForm.language" />
                  </el-form-item>
                </div>
                <el-form-item label="内容偏好">
                  <el-input
                    v-model="campaignWizardForm.contentPreference"
                    type="textarea"
                    :rows="3"
                  />
                </el-form-item>

                <div class="wizard-switches">
                  <div>
                    <strong>创意/脚本提交</strong>
                    <span>要求达人在制作前先提交创意方向或脚本。</span>
                    <el-switch
                      v-model="campaignWizardForm.idealScriptSubmission"
                    />
                  </div>
                  <div>
                    <strong>自动审批</strong>
                    <span>低风险达人通过关键质量检查后自动进入下一步。</span>
                    <el-switch v-model="campaignWizardForm.automaticApproval" />
                  </div>
                </div>

                <section
                  v-for="group in influencerSettingGroups"
                  :key="group.title"
                  class="setting-group"
                >
                  <h3>{{ group.title }}</h3>
                  <div class="wizard-two-col">
                    <el-form-item
                      v-for="field in group.fields"
                      :key="field.key"
                      :label="field.label"
                    >
                      <el-input
                        v-if="/audience/i.test(field.key)"
                        v-model="campaignWizardForm.settings[field.key]"
                      />
                      <el-input-number
                        v-else
                        v-model="campaignWizardForm.settings[field.key]"
                        :min="0"
                        class="w-full!"
                      />
                    </el-form-item>
                  </div>
                </section>
              </el-form>
            </div>

            <div v-else-if="wizardActiveStep === 2" class="wizard-section">
              <div class="wizard-card-title">
                <strong>预览样本达人并校准匹配方向</strong>
                <span> 发布前先对样本达人给出匹配/不匹配反馈。 </span>
              </div>
              <div class="sample-match-list">
                <article
                  v-for="item in sampleInfluencers"
                  :key="item.id"
                  :class="{
                    positive: item.matched === true,
                    negative: item.matched === false
                  }"
                >
                  <div class="creator-avatar">{{ item.name.slice(0, 1) }}</div>
                  <div>
                    <strong>{{ item.name }}</strong>
                    <p>
                      {{ item.handle }} · {{ item.country }} ·
                      {{ item.language }} · {{ item.followers }} 粉丝 ·
                      {{ item.engagement }} 互动率
                    </p>
                    <span>{{ item.reason }}</span>
                  </div>
                  <div class="sample-metrics">
                    <span>报价 {{ moneyText(item.price) }}</span>
                    <span>预计播放 {{ item.predictedViews }}</span>
                    <span>CPM {{ item.predictedCpm }}</span>
                  </div>
                  <div class="sample-actions">
                    <el-button
                      :type="item.matched === false ? 'danger' : 'default'"
                      @click="setSampleMatch(item.id, false)"
                    >
                      不匹配
                    </el-button>
                    <el-button
                      :type="item.matched === true ? 'success' : 'primary'"
                      @click="setSampleMatch(item.id, true)"
                    >
                      匹配
                    </el-button>
                  </div>
                </article>
              </div>
              <el-alert
                type="success"
                :closable="false"
                :title="`已记录反馈：${wizardMatchedCount} 个匹配，${wizardRejectedCount} 个不匹配。`"
              />
            </div>

            <div v-else class="wizard-section">
              <div class="wizard-card-title">
                <strong>预算与效果预测</strong>
                <span>确认预算、预计结果和项目周期。</span>
              </div>
              <el-form label-position="top">
                <div class="wizard-two-col">
                  <el-form-item label="项目名称">
                    <el-input v-model="campaignWizardForm.campaignGoal" />
                  </el-form-item>
                  <el-form-item label="负责人">
                    <el-input v-model="campaignWizardForm.owner" />
                  </el-form-item>
                  <el-form-item label="达人营销预算">
                    <el-input-number
                      v-model="campaignWizardForm.budget"
                      :min="0"
                      :step="1000"
                      class="w-full!"
                    />
                  </el-form-item>
                  <el-form-item label="币种">
                    <el-input v-model="campaignWizardForm.currency" />
                  </el-form-item>
                  <el-form-item label="开始日期">
                    <el-date-picker
                      v-model="campaignWizardForm.cycleStartDate"
                      value-format="YYYY-MM-DD"
                      type="date"
                      class="w-full!"
                    />
                  </el-form-item>
                  <el-form-item label="结束日期">
                    <el-date-picker
                      v-model="campaignWizardForm.cycleEndDate"
                      value-format="YYYY-MM-DD"
                      type="date"
                      class="w-full!"
                    />
                  </el-form-item>
                </div>
              </el-form>
              <div class="forecast-grid">
                <div>
                  <span>预计达人</span>
                  <strong>{{ wizardForecast.influencerCount }}</strong>
                </div>
                <div>
                  <span>预计播放</span>
                  <strong>{{
                    formatCount(wizardForecast.estimatedViews)
                  }}</strong>
                </div>
                <div>
                  <span>预计点击</span>
                  <strong>{{
                    formatCount(wizardForecast.estimatedClicks)
                  }}</strong>
                </div>
                <div>
                  <span>预计 CPM</span>
                  <strong>{{ moneyText(wizardForecast.cpm) }}</strong>
                </div>
                <div>
                  <span>预计 CPC</span>
                  <strong>{{ moneyText(wizardForecast.cpc) }}</strong>
                </div>
              </div>
              <el-table :data="sampleInfluencers" class="business-table" stripe>
                <el-table-column prop="name" label="达人" min-width="160" />
                <el-table-column prop="price" label="预计报价" width="150">
                  <template #default="{ row }">{{
                    moneyText(row.price)
                  }}</template>
                </el-table-column>
                <el-table-column
                  prop="predictedCpm"
                  label="预计 CPM"
                  width="150"
                />
                <el-table-column
                  prop="predictedViews"
                  label="预计播放"
                  width="160"
                />
                <el-table-column label="反馈" width="130">
                  <template #default="{ row }">
                    <el-tag v-if="row.matched === true" type="success"
                      >匹配</el-tag
                    >
                    <el-tag v-else-if="row.matched === false" type="danger"
                      >不匹配</el-tag
                    >
                    <el-tag v-else effect="plain">待确认</el-tag>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </section>
        </div>
        <template #footer>
          <el-button @click="campaignWizardDialog = false">保存草稿</el-button>
          <el-button
            :disabled="wizardActiveStep === 0"
            @click="previousCampaignWizardStep"
          >
            上一步
          </el-button>
          <el-button
            v-if="wizardActiveStep < wizardSteps.length - 1"
            type="primary"
            @click="nextCampaignWizardStep"
          >
            下一步
          </el-button>
          <el-button
            v-else
            type="primary"
            :loading="wizardSubmitting"
            @click="releaseCampaignFromWizard"
          >
            发布项目
          </el-button>
        </template>
      </el-dialog>

      <el-dialog
        v-model="projectDialog"
        :title="editingProjectId ? '编辑项目' : '创建项目'"
        width="640px"
      >
        <el-form :model="projectForm" label-width="96px">
          <el-form-item label="项目名称"
            ><el-input v-model="projectForm.name"
          /></el-form-item>
          <el-form-item label="目标市场"
            ><el-select
              v-model="projectForm.targetMarket"
              allow-create
              filterable
              default-first-option
              placeholder="选择或输入目标市场"
              class="w-full!"
              @change="handleMarketChange"
            >
              <el-option
                v-for="market in marketOptions"
                :key="market"
                :label="market"
                :value="market"
              >
                <div class="market-option">
                  <span>{{ market }}</span>
                  <el-button
                    link
                    type="danger"
                    @mousedown.stop
                    @click="removeMarketOption(market, $event)"
                  >
                    删除
                  </el-button>
                </div>
              </el-option>
            </el-select></el-form-item
          >
          <el-form-item label="平台"
            ><el-input v-model="projectForm.platform"
          /></el-form-item>
          <el-form-item label="合作目标"
            ><el-input v-model="projectForm.campaignType"
          /></el-form-item>
          <el-form-item label="预算"
            ><el-input-number
              v-model="projectForm.budget"
              :min="0"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="状态"
            ><el-input v-model="projectForm.status"
          /></el-form-item>
          <el-form-item label="负责人"
            ><el-input v-model="projectForm.owner"
          /></el-form-item>
          <el-form-item label="Brief"
            ><el-input v-model="projectForm.brief" type="textarea" :rows="4"
          /></el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="projectDialog = false">取消</el-button>
          <el-button type="primary" @click="submitProject">保存</el-button>
        </template>
      </el-dialog>

      <el-dialog
        v-model="cooperationDialog"
        :title="editingCooperationId ? '编辑合作记录' : '新增合作记录'"
        width="640px"
      >
        <el-form :model="cooperationForm" label-width="100px">
          <el-form-item label="项目ID"
            ><el-input-number
              v-model="cooperationForm.projectId"
              :min="1"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="资源ID"
            ><el-input-number
              v-model="cooperationForm.resourceId"
              :min="1"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="合作形式"
            ><el-input v-model="cooperationForm.cooperationType"
          /></el-form-item>
          <el-form-item label="报价"
            ><el-input-number
              v-model="cooperationForm.quoteAmount"
              :min="0"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="币种"
            ><el-input v-model="cooperationForm.currency"
          /></el-form-item>
          <el-form-item label="曝光"
            ><el-input-number
              v-model="cooperationForm.impressions"
              :min="0"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="播放/阅读"
            ><el-input-number
              v-model="cooperationForm.views"
              :min="0"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="转赞藏"
            ><el-input-number
              v-model="cooperationForm.engagementCount"
              :min="0"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="评论"
            ><el-input-number
              v-model="cooperationForm.commentsCount"
              :min="0"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="点击"
            ><el-input-number
              v-model="cooperationForm.clicks"
              :min="0"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="转化"
            ><el-input-number
              v-model="cooperationForm.conversions"
              :min="0"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="ROI"
            ><el-input-number
              v-model="cooperationForm.roi"
              :min="0"
              :step="0.1"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="团队评分"
            ><el-input-number
              v-model="cooperationForm.teamRating"
              :min="0"
              :max="5"
              class="w-full!"
          /></el-form-item>
          <el-form-item label="发布日期">
            <el-date-picker
              v-model="cooperationForm.releaseDate"
              value-format="YYYY-MM-DD"
              type="date"
              class="w-full!"
            />
          </el-form-item>
          <el-form-item label="发布链接"
            ><el-input v-model="cooperationForm.deliverableLinks"
          /></el-form-item>
          <el-form-item label="状态"
            ><el-input v-model="cooperationForm.status"
          /></el-form-item>
          <el-form-item label="交付状态"
            ><el-input v-model="cooperationForm.deliverableStatus"
          /></el-form-item>
          <el-form-item label="备注"
            ><el-input v-model="cooperationForm.notes" type="textarea"
          /></el-form-item>
        </el-form>
        <template #footer>
          <el-button
            :disabled="savingCooperation"
            @click="cooperationDialog = false"
            >取消</el-button
          >
          <el-button
            type="primary"
            :loading="savingCooperation"
            @click="submitCooperation"
            >保存</el-button
          >
        </template>
      </el-dialog>

    </div>
  </div>
</template>

<style scoped>
.business-page {
  min-height: 100%;
  padding: 20px;
  background: #f8fafc;
}

.page-hero {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  padding: 20px;
  margin-bottom: 16px;
  background:
    radial-gradient(circle at 86% 20%, rgb(20 184 166 / 18%), transparent 26%),
    linear-gradient(135deg, #fff 0%, #eef2ff 58%, #ecfeff 100%);
  border: 1px solid rgb(148 163 184 / 22%);
  border-radius: 8px;
}

.page-hero span {
  font-size: 12px;
  font-weight: 700;
  color: #2563eb;
  text-transform: uppercase;
}

.page-hero h1 {
  margin: 8px 0 0;
  font-size: 26px;
  line-height: 1.25;
  color: #0f172a;
  letter-spacing: 0;
}

.page-hero p {
  margin: 8px 0 0;
  color: #64748b;
}

.workspace-card {
  border-radius: 8px;
}

.campaign-wizard-layout {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 18px;
  min-height: 620px;
}

.wizard-rail {
  padding: 16px 12px;
  background: #fbfcfe;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.wizard-stage {
  min-width: 0;
  max-height: 68vh;
  padding-right: 4px;
  overflow: auto;
}

.wizard-section,
.wizard-form-card,
.setting-group {
  display: grid;
  gap: 16px;
}

.ai-generator,
.wizard-form-card,
.setting-group,
.sample-match-list article {
  padding: 16px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.ai-generator {
  gap: 14px;
}

.ai-generator strong,
.wizard-card-title strong,
.sample-match-list strong {
  color: #111827;
}

.ai-generator p,
.wizard-card-title span,
.logo-upload-row span,
.campaign-examples p,
.sample-match-list p,
.sample-match-list article > div:nth-child(2) > span,
.sample-metrics span,
.forecast-grid span,
.wizard-switches span {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: #6b7280;
}

.generator-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
}

.wizard-card-title {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  align-items: baseline;
  justify-content: space-between;
}

.logo-upload-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.wizard-logo-preview {
  display: grid;
  place-items: center;
  width: 60px;
  height: 60px;
  font-size: 28px;
  font-weight: 800;
  color: #f26522;
  background: #fff7ed;
  border: 1px solid #fed7aa;
  border-radius: 8px;
}

.wizard-two-col {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.campaign-examples {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.campaign-examples article {
  padding: 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.campaign-examples strong {
  color: #334155;
}

.wizard-switches {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.wizard-switches > div {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 6px 12px;
  align-items: center;
  padding: 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.wizard-switches strong {
  color: #334155;
}

.wizard-switches span {
  grid-column: 1;
}

.setting-group h3 {
  margin: 0;
  font-size: 15px;
  color: #0f172a;
}

.sample-match-list {
  display: grid;
  gap: 12px;
}

.sample-match-list article {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) minmax(150px, 0.45fr) auto;
  gap: 14px;
  align-items: center;
}

.sample-match-list article.positive {
  border-color: #86efac;
  box-shadow: 0 10px 24px rgb(34 197 94 / 9%);
}

.sample-match-list article.negative {
  border-color: #fecaca;
  box-shadow: 0 10px 24px rgb(239 68 68 / 8%);
}

.sample-match-list article > div:nth-child(2),
.sample-metrics {
  display: grid;
  gap: 5px;
  min-width: 0;
}

.sample-match-list p,
.sample-match-list article > div:nth-child(2) > span {
  overflow: hidden;
  text-overflow: ellipsis;
}

.sample-actions {
  display: flex;
  gap: 8px;
}

.forecast-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}

.forecast-grid > div {
  display: grid;
  gap: 6px;
  min-height: 84px;
  padding: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.forecast-grid strong {
  font-size: 20px;
  color: #0f172a;
}

.overview-dashboard {
  display: grid;
  gap: 16px;
}

.overview-header {
  display: flex;
  gap: 18px;
  align-items: center;
  justify-content: space-between;
  padding: 18px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.overview-header > div:first-child {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.overview-header span,
.overview-header p,
.overview-metrics p,
.overview-panel p,
.overview-flow span {
  margin: 0;
  font-size: 12px;
  color: #64748b;
}

.overview-header > div:first-child > span {
  font-weight: 700;
  color: #2563eb;
}

.overview-header h2 {
  margin: 0;
  overflow: hidden;
  font-size: 20px;
  color: #0f172a;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overview-header p span {
  display: inline-block;
  width: 1px;
  height: 12px;
  margin: 0 10px;
  vertical-align: -2px;
  background: #d7dce3;
}

.overview-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 10px;
  align-items: center;
}

.overview-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.overview-metrics article,
.overview-panel {
  padding: 14px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.overview-metrics article {
  display: grid;
  gap: 7px;
  min-height: 96px;
}

.overview-metrics span {
  font-size: 12px;
  color: #64748b;
}

.overview-metrics strong {
  font-size: 22px;
  line-height: 1.2;
  color: #0f172a;
}

.overview-flow {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
}

.overview-flow button,
.overview-pending-list button {
  font: inherit;
  text-align: left;
  cursor: pointer;
  background: #fff;
  border: 1px solid #e5e7eb;
}

.overview-flow button {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  min-height: 68px;
  padding: 12px;
  border-radius: 8px;
}

.overview-flow button.active,
.overview-flow button:hover {
  border-color: #2563eb;
  box-shadow: 0 8px 20px rgb(37 99 235 / 8%);
}

.overview-flow svg {
  font-size: 20px;
  color: #2563eb;
}

.overview-flow strong {
  font-size: 20px;
  color: #0f172a;
}

.overview-grid {
  display: grid;
  grid-template-columns: minmax(320px, 0.8fr) minmax(0, 1.2fr);
  gap: 12px;
  align-items: start;
}

.overview-panel {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.overview-pending-list {
  display: grid;
  gap: 10px;
}

.overview-pending-list button {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  min-height: 72px;
  padding: 12px;
  border-radius: 8px;
}

.overview-pending-list button:hover {
  border-color: #f59e0b;
  box-shadow: 0 8px 20px rgb(245 158 11 / 9%);
}

.overview-pending-list button > div:nth-child(2) {
  min-width: 0;
}

.overview-pending-list strong {
  display: block;
  overflow: hidden;
  color: #0f172a;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.campaign-detail-shell {
  display: grid;
  grid-template-columns: 184px minmax(0, 1fr);
  gap: 18px;
  min-height: 720px;
}

.campaign-side-nav {
  position: sticky;
  top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-self: start;
  min-height: 360px;
  padding: 14px 10px;
  background: #fff;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.campaign-side-nav button,
.campaign-progress-step,
.pending-card-row button,
.detail-tabs button {
  font: inherit;
  cursor: pointer;
  border: 0;
}

.campaign-side-nav button {
  display: flex;
  gap: 9px;
  align-items: center;
  width: 100%;
  padding: 10px 12px;
  color: #334155;
  text-align: left;
  background: transparent;
  border-radius: 6px;
}

.campaign-side-nav button.active,
.campaign-side-nav button:hover {
  color: #9a4b2f;
  background: #fff4e8;
}

.campaign-side-nav svg {
  flex: 0 0 auto;
  font-size: 17px;
}

.campaign-detail-main {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.campaign-topbar {
  display: flex;
  gap: 18px;
  align-items: center;
  justify-content: space-between;
  min-height: 88px;
  padding: 14px 18px;
  background: #fff;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.campaign-title-block {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
}

.campaign-logo-box {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 42px;
  height: 42px;
  color: #f26522;
  background: #fff7ed;
  border: 1px solid #fed7aa;
  border-radius: 8px;
}

.campaign-title-block h2 {
  margin: 8px 0 4px;
  overflow: hidden;
  font-size: 18px;
  color: #20242a;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.campaign-title-block p,
.collaboration-heading p,
.pending-card-row p,
.creator-detail-header p,
.content-info-block span,
.delivery-timeline p,
.tracking-link span {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: #7a828f;
}

.campaign-title-block p span,
.creator-detail-header p span {
  display: inline-block;
  width: 1px;
  height: 12px;
  margin: 0 10px;
  vertical-align: -2px;
  background: #d7dce3;
}

.campaign-topbar-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 10px;
  align-items: center;
}

.campaign-progress-card {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0;
  padding: 20px 18px;
  background: #fff;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.campaign-progress-step {
  position: relative;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  min-width: 0;
  padding-right: 18px;
  text-align: left;
  background: transparent;
}

.campaign-progress-step::after {
  position: absolute;
  top: 13px;
  right: 10px;
  left: 34px;
  height: 2px;
  content: "";
  background: #f26522;
}

.campaign-progress-step:last-child::after {
  display: none;
}

.campaign-progress-step > span {
  z-index: 1;
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  color: #f26522;
  background: #fff;
  border: 2px solid #f26522;
  border-radius: 50%;
}

.campaign-progress-step.done > span {
  color: #fff;
  background: #f26522;
}

.campaign-progress-step strong {
  display: block;
  overflow: hidden;
  color: #20242a;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.campaign-progress-step p {
  margin: 5px 0 0;
  overflow: hidden;
  font-size: 12px;
  color: #8a919c;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.campaign-progress-step.active strong {
  color: #f26522;
}

.collaboration-panel {
  display: grid;
  gap: 16px;
  padding: 22px;
  background: #fff;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.collaboration-heading {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.collaboration-heading h2,
.content-info-block h3,
.creator-detail-header h3 {
  margin: 0;
  color: #20242a;
  letter-spacing: 0;
}

.collaboration-heading h2 {
  font-size: 24px;
}

.assurance-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.assurance-strip > div {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 3px 10px;
  align-items: center;
  min-height: 62px;
  padding: 12px 16px;
  border-right: 1px solid #e9edf2;
}

.assurance-strip > div:last-child {
  border-right: 0;
}

.assurance-strip svg {
  grid-row: span 2;
  font-size: 20px;
  color: #48a763;
}

.assurance-strip strong {
  color: #30343a;
}

.assurance-strip span {
  overflow: hidden;
  font-size: 12px;
  color: #8a919c;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tip-bar {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 12px 14px;
  background: #eef6ff;
  border-radius: 8px;
}

.tip-bar strong {
  flex: 0 0 auto;
  padding: 2px 8px;
  color: #fff;
  background: #5b8def;
  border-radius: 999px;
}

.tip-bar span {
  color: #30343a;
}

.pending-actions-row {
  display: grid;
  gap: 12px;
}

.pending-card-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.pending-card-row button {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  min-height: 92px;
  padding: 14px;
  text-align: left;
  background: #fff;
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.pending-card-row button:hover,
.pending-card-row button.active {
  border-color: #f26522;
  box-shadow: 0 10px 24px rgb(242 101 34 / 12%);
}

.pending-card-row button > div:nth-child(2) {
  min-width: 0;
}

.pending-card-row span {
  font-size: 12px;
  color: #686f7a;
}

.pending-card-row strong {
  display: block;
  margin-top: 4px;
  overflow: hidden;
  color: #20242a;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.creator-avatar {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  font-weight: 700;
  color: #f26522;
  background: #fff1e8;
  border-radius: 50%;
}

.creator-avatar.large {
  width: 64px;
  height: 64px;
  font-size: 24px;
}

.influencer-workspace {
  display: grid;
  grid-template-columns: minmax(420px, 0.9fr) minmax(460px, 1.1fr);
  gap: 16px;
  align-items: start;
}

.influencer-list-panel,
.creator-detail-panel {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.creator-detail-panel {
  padding: 18px;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.creator-detail-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 14px;
  align-items: center;
  min-width: 0;
}

.creator-detail-header h3 {
  font-size: 22px;
}

.quality-strip {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.quality-strip strong {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 12px;
  color: #48a763;
}

.quality-strip > div {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.quality-strip span {
  padding: 8px 10px;
  overflow: hidden;
  font-size: 12px;
  color: #4b5563;
  text-overflow: ellipsis;
  white-space: nowrap;
  background: #f8fafc;
  border-radius: 6px;
}

.detail-tabs {
  display: flex;
  gap: 28px;
  border-bottom: 1px solid #e5e7eb;
}

.detail-tabs button {
  padding: 0 0 10px;
  color: #4b5563;
  background: transparent;
  border-bottom: 2px solid transparent;
}

.detail-tabs button.active {
  color: #f26522;
  border-bottom-color: #f26522;
}

.content-info-block {
  display: grid;
  gap: 12px;
}

.tracking-link {
  display: grid;
  gap: 4px;
  padding: 12px;
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.delivery-timeline {
  display: grid;
  gap: 0;
  padding-top: 8px;
}

.delivery-timeline > div {
  position: relative;
  display: grid;
  grid-template-columns: 22px minmax(0, 1fr);
  gap: 14px;
  padding-bottom: 24px;
}

.delivery-timeline > div::before {
  position: absolute;
  top: 14px;
  bottom: 0;
  left: 7px;
  width: 2px;
  content: "";
  background: #ffd5bf;
}

.delivery-timeline > div:last-child {
  padding-bottom: 0;
}

.delivery-timeline > div:last-child::before {
  display: none;
}

.delivery-timeline > div > span {
  z-index: 1;
  width: 16px;
  height: 16px;
  margin-top: 2px;
  background: #fff;
  border: 4px solid #f26522;
  border-radius: 50%;
}

.delivery-timeline > div.completed > span {
  background: #f26522;
}

.delivery-timeline article {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.delivery-timeline article > div:first-child {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.delivery-timeline strong {
  color: #20242a;
}

.timeline-note {
  display: grid;
  gap: 6px;
  max-width: 640px;
  padding: 14px;
  background: #fffbea;
  border-radius: 8px;
}

.campaign-switcher {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: 16px;
  padding: 16px;
  margin-bottom: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.campaign-switcher > div:first-child {
  display: grid;
  gap: 8px;
}

.campaign-switcher span,
.campaign-switcher p,
.section-heading span,
.execution-metrics span,
.execution-metrics p,
.pipeline-grid p,
.pending-grid span,
.pending-grid p,
.execution-alert p,
.drawer-profile p,
.drawer-decision-grid span,
.drawer-next-action span,
.drawer-next-action p,
.execution-timeline p {
  margin: 0;
  font-size: 12px;
  color: #64748b;
}

.campaign-switcher-main,
.campaign-switcher-actions,
.section-heading,
.drawer-profile {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.campaign-switcher-main > div:first-child {
  display: grid;
  gap: 6px;
}

.campaign-switcher-main strong {
  font-size: 18px;
  color: #0f172a;
}

.campaign-switcher-actions {
  justify-content: flex-end;
}

.execution-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}

.execution-metrics article {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  align-items: start;
  min-height: 112px;
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.execution-metrics article > svg {
  padding: 8px;
  font-size: 22px;
  color: #2563eb;
  background: #eff6ff;
  border-radius: 8px;
}

.execution-metrics article > div {
  display: grid;
  gap: 5px;
}

.execution-metrics strong {
  font-size: 20px;
  color: #0f172a;
}

.execution-metrics .el-progress {
  grid-column: 1 / -1;
}

.pipeline-section,
.pending-section,
.creator-pipeline,
.drawer-section {
  display: grid;
  gap: 12px;
  padding: 16px;
  margin-bottom: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.section-heading > div {
  display: grid;
  gap: 4px;
}

.section-heading strong {
  color: #0f172a;
}

.pipeline-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
}

.pipeline-grid button,
.pending-grid button,
.stage-filter button {
  font: inherit;
  text-align: left;
  cursor: pointer;
  background: #fff;
  border: 1px solid #e2e8f0;
}

.pipeline-grid button {
  position: relative;
  display: grid;
  gap: 5px;
  min-height: 124px;
  padding: 12px;
  border-radius: 8px;
}

.pipeline-grid button::after {
  position: absolute;
  top: 26px;
  right: -10px;
  z-index: 1;
  width: 10px;
  height: 2px;
  content: "";
  background: #cbd5e1;
}

.pipeline-grid button:last-child::after {
  display: none;
}

.pipeline-grid button:hover,
.pipeline-grid button.active {
  border-color: #2563eb;
  box-shadow: 0 8px 20px rgb(37 99 235 / 10%);
}

.pipeline-grid svg {
  font-size: 20px;
  color: #2563eb;
}

.pipeline-grid strong {
  font-size: 24px;
  color: #0f172a;
}

.pipeline-grid span {
  font-size: 13px;
  font-weight: 700;
  color: #334155;
}

.pending-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.pending-grid button {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  padding: 12px;
  border-radius: 8px;
}

.pending-grid button:hover {
  border-color: #f59e0b;
  box-shadow: 0 8px 20px rgb(245 158 11 / 10%);
}

.pending-grid button > div:nth-child(2) {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.pending-grid button > svg {
  color: #94a3b8;
}

.pending-icon {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  color: #ea580c;
  background: #fff7ed;
  border-radius: 50%;
}

.pending-grid strong {
  overflow: hidden;
  text-overflow: ellipsis;
  color: #0f172a;
  white-space: nowrap;
}

.execution-alert {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 14px;
  margin-bottom: 14px;
  color: #9a3412;
  background: #fff7ed;
  border: 1px solid #fed7aa;
  border-radius: 8px;
}

.execution-alert svg {
  flex: 0 0 auto;
  font-size: 20px;
}

.execution-alert div {
  display: grid;
  gap: 4px;
}

.stage-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.stage-filter button {
  padding: 6px 10px;
  font-size: 12px;
  color: #475569;
  border-radius: 999px;
}

.stage-filter button.active {
  color: #fff;
  background: #0f172a;
  border-color: #0f172a;
}

.drawer-profile {
  padding: 14px;
  margin-bottom: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.drawer-profile > div:nth-child(2) {
  display: grid;
  flex: 1;
  gap: 5px;
}

.drawer-profile h3 {
  margin: 0;
  color: #0f172a;
}

.drawer-avatar {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 48px;
  height: 48px;
  font-size: 18px;
  font-weight: 700;
  color: #1d4ed8;
  background: #dbeafe;
  border-radius: 50%;
}

.drawer-decision-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.drawer-decision-grid > div {
  display: grid;
  gap: 6px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.drawer-decision-grid strong {
  font-size: 18px;
  color: #0f172a;
}

.drawer-next-action {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 14px;
  margin-bottom: 14px;
  color: #9a3412;
  background: #fff7ed;
  border: 1px solid #fed7aa;
  border-radius: 8px;
}

.drawer-next-action > div {
  display: grid;
  gap: 4px;
}

.drawer-next-action svg {
  flex: 0 0 auto;
  font-size: 20px;
}

.drawer-section {
  margin-bottom: 14px;
}

.execution-timeline {
  display: grid;
}

.execution-timeline > div {
  position: relative;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  padding-bottom: 18px;
}

.execution-timeline > div::before {
  position: absolute;
  top: 26px;
  bottom: 0;
  left: 15px;
  width: 2px;
  content: "";
  background: #e2e8f0;
}

.execution-timeline > div:last-child {
  padding-bottom: 0;
}

.execution-timeline > div:last-child::before {
  display: none;
}

.execution-timeline > div > span {
  z-index: 1;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  color: #94a3b8;
  background: #f1f5f9;
  border-radius: 50%;
}

.execution-timeline > div.completed > span {
  color: #fff;
  background: #2563eb;
}

.execution-timeline > div > div {
  display: grid;
  gap: 4px;
}

.execution-timeline strong {
  color: #334155;
}

.drawer-info-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 0;
}

.drawer-info-list > div {
  display: grid;
  gap: 4px;
  padding: 10px;
  background: #f8fafc;
  border-radius: 8px;
}

.drawer-info-list dt {
  font-size: 12px;
  color: #64748b;
}

.drawer-info-list dd {
  margin: 0;
  color: #0f172a;
}

.review-report {
  display: grid;
  gap: 12px;
  margin-bottom: 14px;
}

.review-report-heading,
.ai-review-title {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.review-report-heading > div,
.ai-review-title > div {
  display: grid;
  gap: 4px;
}

.review-report-heading strong,
.ai-review-title strong,
.project-ai-insight strong {
  color: #0f172a;
}

.review-report-heading span,
.ai-review-title span {
  font-size: 12px;
  color: #64748b;
}

.ai-review-summary {
  display: grid;
  gap: 10px;
  padding: 14px;
  margin-bottom: 14px;
  background: #f8fafc;
  border: 1px solid #cbd5e1;
  border-left: 4px solid #2563eb;
  border-radius: 8px;
}

.ai-review-title {
  justify-content: flex-start;
}

.ai-review-title > svg {
  flex: 0 0 auto;
  font-size: 20px;
  color: #2563eb;
}

.ai-review-summary p,
.project-ai-insight p {
  margin: 0;
  font-size: 13px;
  line-height: 1.8;
  color: #475569;
}

.project-ai-insight {
  display: grid;
  gap: 6px;
  padding: 10px 12px;
  background: #f8fafc;
  border-left: 3px solid #94a3b8;
  border-radius: 4px;
}

.project-ai-insight strong {
  font-size: 12px;
}

.review-summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.review-summary-grid > div {
  display: grid;
  gap: 6px;
  min-height: 92px;
  padding: 14px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.review-summary-grid span,
.review-summary-grid p,
.project-review-head span,
.project-review-list p,
.best-item span {
  font-size: 12px;
  color: #64748b;
}

.review-summary-grid strong {
  font-size: 22px;
  line-height: 1.2;
  color: #0f172a;
}

.review-summary-grid p,
.project-review-list p {
  margin: 0;
  line-height: 1.65;
}

.project-review-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.project-review-list article {
  display: grid;
  gap: 12px;
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.project-review-head,
.project-card-actions,
.best-item {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
}

.project-card-actions {
  justify-content: flex-end;
}

.project-review-head > div {
  display: grid;
  gap: 5px;
  min-width: 0;
}

.project-review-head strong,
.best-item strong {
  color: #0f172a;
}

.project-review-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.project-review-metrics > div {
  display: grid;
  gap: 5px;
  padding: 10px;
  background: #f8fafc;
  border-radius: 8px;
}

.project-review-metrics span {
  font-size: 12px;
  color: #64748b;
}

.project-review-metrics strong {
  font-size: 16px;
  color: #0f172a;
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  justify-content: flex-start;
  margin-bottom: 12px;
}

.toolbar-title {
  font-size: 13px;
  font-weight: 700;
  color: #334155;
}

.import-project-select {
  width: min(100%, 420px);
}

.import-sheet-select {
  width: min(100%, 520px);
}

.import-preview-more {
  margin: 10px 0 0;
  color: #64748b;
  font-size: 12px;
  text-align: center;
}

.import-project-option {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.mb-3 {
  margin-bottom: 12px;
}

.ml-2 {
  margin-left: 8px;
}

.mr-1 {
  margin-right: 4px;
}

.market-option {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

:deep(.business-table) {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

:deep(.business-table th.el-table__cell) {
  color: #475569;
  background: #f8fafc;
}

:deep(.review-sum-row td.el-table__cell) {
  font-weight: 700;
  color: #0f172a;
  background: #eaf2ff !important;
}

:deep(.execution-selected-row td.el-table__cell) {
  background: #fff7ed !important;
}

:deep(.influencer-table .el-table__row) {
  cursor: pointer;
}

:deep(.workspace-card .el-tabs__header) {
  margin-bottom: 16px;
}

:deep(.el-card__body) {
  padding: 18px;
}

:deep(.import-error-row) {
  --el-table-tr-bg-color: var(--el-color-danger-light-9);
}

:deep(.import-duplicate-row) {
  --el-table-tr-bg-color: var(--el-color-warning-light-9);
}

@media (width <= 760px) {
  .business-page {
    padding: 12px;
  }

  .page-hero {
    flex-direction: column;
    align-items: stretch;
  }

  .review-summary-grid,
  .project-review-list,
  .project-review-metrics,
  .campaign-wizard-layout,
  .generator-row,
  .wizard-two-col,
  .campaign-examples,
  .wizard-switches,
  .sample-match-list article,
  .forecast-grid,
  .overview-metrics,
  .overview-flow,
  .overview-grid,
  .campaign-detail-shell,
  .campaign-progress-card,
  .assurance-strip,
  .pending-card-row,
  .influencer-workspace,
  .quality-strip > div,
  .campaign-switcher,
  .execution-metrics,
  .pipeline-grid,
  .pending-grid,
  .drawer-decision-grid,
  .drawer-info-list {
    grid-template-columns: 1fr;
  }

  .campaign-switcher-main,
  .overview-header,
  .overview-actions,
  .campaign-topbar,
  .collaboration-heading,
  .creator-detail-header,
  .drawer-profile {
    flex-direction: column;
    align-items: flex-start;
  }

  .campaign-side-nav {
    position: static;
    min-height: 0;
  }

  .campaign-topbar,
  .collaboration-heading {
    display: grid;
  }

  .campaign-progress-step::after {
    display: none;
  }

  .assurance-strip > div {
    border-right: 0;
    border-bottom: 1px solid #e9edf2;
  }

  .assurance-strip > div:last-child {
    border-bottom: 0;
  }

  .creator-detail-header {
    grid-template-columns: 1fr;
  }

  .pipeline-grid button::after {
    display: none;
  }

  .review-report-heading {
    align-items: flex-start;
  }

  .import-project-select {
    width: 100%;
  }

  .import-sheet-select {
    width: 100%;
  }
}
/* Campaign center: a compact project-first workspace. */
.campaign-center {
  min-height: 100%;
  padding: 22px;
  color: #25262c;
  background: #fff;
}
.project-import-intro {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 14px;
}
.project-import-intro strong {
  display: block;
  color: #25262c;
  font-size: 14px;
}
.project-import-intro p {
  margin: 4px 0 0;
  color: #7a7d86;
  font-size: 12px;
  line-height: 1.5;
}
.center-header {
  display: flex;
  gap: 20px;
  align-items: flex-end;
  justify-content: space-between;
  max-width: 1720px;
  margin: 0 auto 20px;
}
.eyebrow {
  color: #73767d;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.center-header h1 {
  margin: 5px 0 4px;
  font-size: 25px;
  line-height: 1.2;
  letter-spacing: -0.035em;
}
.center-header p {
  margin: 0;
  color: #7d8088;
  font-size: 14px;
}
.center-header-actions {
  display: flex;
  gap: 10px;
}
.center-stats {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  max-width: 1720px;
  margin: 0 auto 20px;
  overflow: hidden;
  border: 1px solid #e4e5e8;
  border-radius: 12px;
}
.center-stats article {
  min-width: 0;
  padding: 14px 16px;
  background: #fff;
}
.center-stats article + article {
  border-left: 1px solid #e7e8eb;
}
.center-stats span,
.center-stats small {
  display: block;
  color: #85878d;
  font-size: 13px;
}
.center-stats strong {
  display: block;
  margin: 8px 0 6px;
  overflow: hidden;
  color: #25262c;
  font-size: 23px;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: -0.035em;
}
.projects-workspace {
  max-width: 1720px;
  margin: 0 auto;
  border-top: 1px solid #e9eaed;
}
.projects-heading {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  padding: 18px 0 14px;
}
.projects-heading h2 {
  margin: 0;
  font-size: 18px;
  letter-spacing: -0.02em;
}
.projects-heading p {
  margin: 6px 0 0;
  color: #85878d;
  font-size: 13px;
}
.projects-toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 0 0 14px;
}
.projects-toolbar > span {
  margin-left: auto;
  color: #85878d;
  font-size: 13px;
}
.project-search {
  width: min(430px, 100%);
}
.status-filter {
  width: 145px;
}
.projects-table {
  width: 100%;
  border-top: 1px solid #e5e6e9;
}
.project-name-cell {
  display: flex;
  gap: 11px;
  align-items: center;
  min-width: 0;
}
.project-icon {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 34px;
  height: 34px;
  color: #e79b26;
  background: #fffaf0;
  border: 1px solid #f0e5ca;
  border-radius: 9px;
}
.project-name-cell > div {
  display: grid;
  min-width: 0;
  gap: 4px;
}
.project-name-cell strong {
  overflow: hidden;
  color: #2c2e34;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.project-name-cell small {
  overflow: hidden;
  color: #85878d;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:deep(.campaign-center .el-button--primary) {
  --el-button-bg-color: #2f63e7;
  --el-button-border-color: #2f63e7;
  --el-button-hover-bg-color: #2558d7;
  --el-button-hover-border-color: #2558d7;
  border-radius: 9px;
}
:deep(.campaign-center .el-input__wrapper),
:deep(.campaign-center .el-select__wrapper) {
  min-height: 40px;
  border-radius: 10px;
  box-shadow: 0 0 0 1px #e1e2e6 inset;
}
:deep(.campaign-center .el-table) {
  --el-table-border-color: #e7e8eb;
  --el-table-header-bg-color: #fff;
  --el-table-row-hover-bg-color: #f8f9fb;
  font-size: 14px;
}
:deep(.campaign-center .el-table th.el-table__cell) {
  height: 60px;
  color: #282a30;
  font-weight: 650;
}
:deep(.campaign-center .el-table td.el-table__cell) {
  height: 70px;
}
:deep(.campaign-center .el-table__row) {
  cursor: pointer;
}
@media (max-width: 1120px) {
  .center-stats {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .center-stats article:nth-child(4) {
    border-left: 0;
    border-top: 1px solid #e7e8eb;
  }
  .center-stats article:nth-child(5) {
    border-top: 1px solid #e7e8eb;
  }
}
@media (max-width: 720px) {
  .campaign-center {
    padding: 20px 16px;
  }
  .center-header {
    align-items: flex-start;
    flex-direction: column;
  }
  .center-header-actions {
    width: 100%;
  }
  .center-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .center-stats article:nth-child(n) {
    border-top: 1px solid #e7e8eb;
  }
  .center-stats article:nth-child(1),
  .center-stats article:nth-child(2) {
    border-top: 0;
  }
  .center-stats article:nth-child(odd) {
    border-left: 0;
  }
  .projects-heading,
  .projects-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
  .projects-toolbar > span {
    margin-left: 0;
  }
  .project-search {
    width: 100%;
  }
  .project-import-intro {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }
}
</style>
