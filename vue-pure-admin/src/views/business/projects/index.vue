<script setup lang="ts">
import { computed, reactive, ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import * as XLSX from "xlsx";
import {
  getProjectList,
  createProject,
  updateProject,
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

const projects = ref<any[]>([]);
const cooperations = ref<any[]>([]);
const projectDialog = ref(false);
const cooperationDialog = ref(false);
const importDialog = ref(false);
const importLoading = ref(false);
const editingProjectId = ref<number | null>(null);
const editingCooperationId = ref<number | null>(null);
const importProjectId = ref<number | null>(null);
const importRows = ref<any[]>([]);
const importFileName = ref("");
const marketOptions = ref<string[]>([]);
const selectedProjectId = ref<number | null>(null);
const activePipelineStage = ref("all");
const executionDrawer = ref(false);
const activeCooperation = ref<any>(null);
const syncingCooperationIds = reactive<Record<number, boolean>>({});
const savingCooperation = ref(false);

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
  importRows.value.filter(row => row.errors.length === 0)
);

const invalidImportRows = computed(() =>
  importRows.value.filter(row => row.errors.length > 0)
);

const duplicateImportRows = computed(() =>
  importRows.value.filter(row => row.duplicate)
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

async function loadData() {
  const [projectRes, cooperationRes] = await Promise.all([
    getProjectList(),
    getCooperationList()
  ]);
  if (projectRes.code === 0) {
    projects.value = projectRes.data.list;
    if (!importProjectId.value && projects.value.length > 0) {
      importProjectId.value = projects.value[0].id;
    }
    if (!selectedProjectId.value && projects.value.length > 0) {
      selectedProjectId.value = projects.value[0].id;
    }
  }
  if (cooperationRes.code === 0) cooperations.value = cooperationRes.data.list;
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
    ElMessage.success(
      editingProjectId.value ? "Campaign 已更新" : "Campaign 已创建"
    );
    projectDialog.value = false;
    loadData();
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
    .replace(/[\s_+\-/()（）]/g, "");
}

const headerAliases = {
  influencer: ["姓名", "Influencer"],
  category: ["领域", "Category"],
  platform: ["平台", "Platform"],
  followerNumber: ["粉丝数", "Follower Number", "Followers"],
  releaseDate: ["发布日期", "Release Date"],
  deliverableLinks: ["发布链接", "Deliverable Links", "Link", "URL"],
  views: ["播放量", "Views"],
  engagementCount: ["转赞藏数", "Likes+Fav+Share", "Likes Fav Share"],
  commentsCount: ["评论数", "Comments"],
  quoteAmount: ["报价", "费用", "Cost", "Quote", "Paid Amount"],
  rating: ["评级", "Rating"]
};

function pickValue(row: Record<string, any>, aliases: string[]) {
  const entries = Object.entries(row);
  const normalizedAliases = aliases.map(normalizeHeader);
  const match = entries.find(([key]) =>
    normalizedAliases.includes(normalizeHeader(key))
  );
  return match ? match[1] : "";
}

function formatDateValue(value: any) {
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
  if (!match) return text;
  return `${match[1]}-${match[2].padStart(2, "0")}-${match[3].padStart(
    2,
    "0"
  )}`;
}

function parseNumber(value: any) {
  if (value === "" || value === null || value === undefined) return 0;
  const num = Number(String(value).replace(/,/g, "").trim());
  return Number.isFinite(num) ? num : NaN;
}

function normalizeImportRow(row: Record<string, any>, index: number) {
  const normalized: any = {
    rowNo: index + 2,
    influencer: String(pickValue(row, headerAliases.influencer)).trim(),
    category: String(pickValue(row, headerAliases.category)).trim(),
    platform: String(pickValue(row, headerAliases.platform)).trim(),
    followerNumber: parseNumber(pickValue(row, headerAliases.followerNumber)),
    releaseDate: formatDateValue(pickValue(row, headerAliases.releaseDate)),
    deliverableLinks: String(
      pickValue(row, headerAliases.deliverableLinks)
    ).trim(),
    views: parseNumber(pickValue(row, headerAliases.views)),
    engagementCount: parseNumber(pickValue(row, headerAliases.engagementCount)),
    commentsCount: parseNumber(pickValue(row, headerAliases.commentsCount)),
    quoteAmount: parseNumber(pickValue(row, headerAliases.quoteAmount)),
    rating: String(pickValue(row, headerAliases.rating)).trim(),
    duplicate: false,
    errors: []
  };

  if (!normalized.influencer) normalized.errors.push("缺少姓名/Influencer");
  [
    "followerNumber",
    "views",
    "engagementCount",
    "commentsCount",
    "quoteAmount"
  ].forEach(key => {
    if (Number.isNaN(normalized[key]) || normalized[key] < 0) {
      normalized.errors.push(`${key} 必须是非负数字`);
    }
  });
  return normalized;
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
  executionDrawer.value = true;
}

function editFromExecutionDetail() {
  if (!activeCooperation.value) return;
  executionDrawer.value = false;
  openEditCooperation(activeCooperation.value);
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

async function handleUploadFile(file: any) {
  if (!importProjectId.value) {
    ElMessage.warning("请先选择导入 Campaign");
    return;
  }
  const rawFile = file.raw;
  if (!rawFile) return;
  importFileName.value = rawFile.name;
  const buffer = await rawFile.arrayBuffer();
  const workbook = XLSX.read(buffer, { type: "array", cellDates: true });
  const firstSheet = workbook.Sheets[workbook.SheetNames[0]];
  const rawRows = XLSX.utils.sheet_to_json<Record<string, any>>(firstSheet, {
    defval: ""
  });
  const rows = rawRows
    .map(normalizeImportRow)
    .filter(row =>
      [row.influencer, row.category, row.platform, row.deliverableLinks].some(
        Boolean
      )
    );

  const seen = new Set<string>();
  rows.forEach(row => {
    const key = [row.influencer, row.platform, row.deliverableLinks]
      .join("|")
      .toLowerCase();
    if (row.influencer && row.platform && row.deliverableLinks) {
      if (seen.has(key)) row.duplicate = true;
      seen.add(key);
    }
  });

  importRows.value = rows;
  importDialog.value = true;
}

async function submitImport() {
  if (!importProjectId.value) {
    ElMessage.warning("请先选择导入 Campaign");
    return;
  }
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
      `导入成功 ${res.data.imported} 条，新增资源 ${res.data.createdResources} 个`
    );
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
  <div class="business-page">
    <section class="page-hero">
      <div>
        <span>Campaign Operations Center</span>
        <h1>Campaign 执行中心</h1>
        <p>
          从达人邀约、议价、内容交付到发布复盘，统一掌握每个 Campaign
          的执行节奏与待处理事项。
        </p>
      </div>
      <el-button type="primary" @click="openCreateProject">
        <IconifyIconOnline icon="ri:add-line" class="mr-1" />
        创建 Campaign
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
          <section class="campaign-switcher">
            <div>
              <span>当前 Campaign</span>
              <el-select
                v-model="selectedProjectId"
                filterable
                placeholder="选择 Campaign"
              >
                <el-option
                  v-for="project in projects"
                  :key="project.id"
                  :label="project.name"
                  :value="project.id"
                />
              </el-select>
            </div>
            <div class="campaign-switcher-main">
              <div>
                <strong>{{ selectedProject?.name || "暂无 Campaign" }}</strong>
                <p>
                  {{ selectedProject?.targetMarket || "未设置市场" }} ·
                  {{ selectedProject?.platform || "全平台" }} ·
                  {{ selectedProject?.campaignType || "未设置合作目标" }}
                </p>
              </div>
              <div class="campaign-switcher-actions">
                <el-tag effect="plain">
                  {{ selectedProject?.status || "待配置" }}
                </el-tag>
                <el-button
                  v-if="selectedProject"
                  link
                  type="primary"
                  @click="openEditProject(selectedProject)"
                >
                  编辑 Campaign
                </el-button>
              </div>
            </div>
          </section>

          <section class="execution-metrics">
            <article>
              <IconifyIconOnline icon="ri:user-search-line" />
              <div>
                <span>合作达人 / 媒体</span>
                <strong>{{ selectedProjectReview.resourceCount }}</strong>
                <p>{{ selectedProjectReview.cooperationCount }} 条执行记录</p>
              </div>
            </article>
            <article>
              <IconifyIconOnline icon="ri:wallet-3-line" />
              <div>
                <span>预算执行</span>
                <strong>
                  {{
                    moneyText(campaignHealth.spent, selectedProject?.currency)
                  }}
                </strong>
                <p>
                  预算
                  {{
                    moneyText(campaignHealth.budget, selectedProject?.currency)
                  }}
                </p>
              </div>
              <el-progress
                :percentage="campaignHealth.budgetRate"
                :stroke-width="6"
                :show-text="false"
              />
            </article>
            <article>
              <IconifyIconOnline icon="ri:checkbox-circle-line" />
              <div>
                <span>发布完成率</span>
                <strong>{{ campaignHealth.completionRate }}%</strong>
                <p>{{ campaignHealth.published }} 条内容已发布</p>
              </div>
              <el-progress
                :percentage="campaignHealth.completionRate"
                :stroke-width="6"
                :show-text="false"
                status="success"
              />
            </article>
            <article>
              <IconifyIconOnline icon="ri:line-chart-line" />
              <div>
                <span>当前触达</span>
                <strong>{{
                  formatCount(selectedProjectReview.totalReach)
                }}</strong>
                <p>
                  CPM
                  {{
                    cpmText(
                      selectedProjectReview.totalCost,
                      selectedProjectReview.totalReach
                    )
                  }}
                </p>
              </div>
            </article>
          </section>

          <section class="pipeline-section">
            <div class="section-heading">
              <div>
                <strong>Campaign 执行流程</strong>
                <span>按合作记录与交付状态自动归类</span>
              </div>
              <el-tag type="success" effect="plain">
                {{ selectedProject?.owner || "未指定负责人" }}
              </el-tag>
            </div>
            <div class="pipeline-grid">
              <button
                v-for="stage in pipelineStages"
                :key="stage.key"
                type="button"
                :class="{ active: activePipelineStage === stage.key }"
                @click="activePipelineStage = stage.key"
              >
                <IconifyIconOnline :icon="stage.icon" />
                <strong>{{ stage.count }}</strong>
                <span>{{ stage.label }}</span>
                <p>{{ stage.description }}</p>
              </button>
            </div>
          </section>

          <section class="pending-section">
            <div class="section-heading">
              <div>
                <strong>待处理动作</strong>
                <span>把最需要人工判断的节点集中到这里</span>
              </div>
              <el-tag type="warning" effect="plain">
                {{ pendingActions.length }} 项待处理
              </el-tag>
            </div>
            <div v-if="pendingActions.length" class="pending-grid">
              <button
                v-for="item in pendingActions"
                :key="item.id"
                type="button"
                @click="openExecutionDetail(item)"
              >
                <div class="pending-icon">
                  <IconifyIconOnline icon="ri:user-follow-line" />
                </div>
                <div>
                  <span>{{ item.action }}</span>
                  <strong>{{ item.resourceName || "未命名资源" }}</strong>
                  <p>
                    {{ cooperationStageLabel(item) }} ·
                    {{ updatedTimeText(item) }}
                  </p>
                </div>
                <IconifyIconOnline icon="ri:arrow-right-line" />
              </button>
            </div>
            <el-empty v-else description="当前没有需要人工处理的动作" />
          </section>

          <section
            v-if="campaignHealth.missingData > 0"
            class="execution-alert"
          >
            <IconifyIconOnline icon="ri:alert-line" />
            <div>
              <strong
                >有
                {{ campaignHealth.missingData }}
                条已发布内容尚未回收效果数据</strong
              >
              <p>建议补充曝光、播放或点击数据，以便完成 Campaign 复盘。</p>
            </div>
          </section>

          <section class="creator-pipeline">
            <div class="section-heading">
              <div>
                <strong>达人执行看板</strong>
                <span>集中查看报价、交付、效果和下一步动作</span>
              </div>
              <el-button
                v-if="activePipelineStage !== 'all'"
                link
                type="primary"
                @click="activePipelineStage = 'all'"
              >
                查看全部
              </el-button>
            </div>
            <div class="stage-filter">
              <button
                type="button"
                :class="{ active: activePipelineStage === 'all' }"
                @click="activePipelineStage = 'all'"
              >
                全部 {{ selectedCooperations.length }}
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
            <el-table :data="pipelineRows" stripe class="business-table">
              <el-table-column
                prop="resourceName"
                label="达人 / 媒体"
                min-width="180"
              />
              <el-table-column label="执行阶段" width="140">
                <template #default="{ row }">
                  <el-tag :type="cooperationStageTag(row)" effect="light">
                    {{ cooperationStageLabel(row) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="status" label="合作状态" width="130" />
              <el-table-column
                prop="deliverableStatus"
                label="交付状态"
                width="130"
              />
              <el-table-column label="报价" width="130">
                <template #default="{ row }">
                  {{ moneyText(row.quoteAmount, row.currency) }}
                </template>
              </el-table-column>
              <el-table-column label="当前 CPM" width="130">
                <template #default="{ row }">
                  {{
                    cpmText(row.quoteAmount, primaryReach(row), row.currency)
                  }}
                </template>
              </el-table-column>
              <el-table-column label="触达" width="120">
                <template #default="{ row }">
                  {{ formatCount(primaryReach(row)) }}
                </template>
              </el-table-column>
              <el-table-column label="下一步" min-width="190">
                <template #default="{ row }">
                  {{ cooperationAction(row) || "等待数据复盘" }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="100" fixed="right">
                <template #default="{ row }">
                  <el-button
                    link
                    type="primary"
                    @click="openExecutionDetail(row)"
                  >
                    查看
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </section>
        </el-tab-pane>
        <el-tab-pane label="Campaign 管理">
          <div class="toolbar">
            <span class="toolbar-title">Campaign 需求池</span>
          </div>
          <el-table :data="projects" stripe class="business-table">
            <el-table-column
              prop="name"
              label="Campaign 名称"
              min-width="180"
            />
            <el-table-column prop="targetMarket" label="目标市场" width="120" />
            <el-table-column prop="platform" label="平台" width="140" />
            <el-table-column prop="campaignType" label="合作目标" width="120" />
            <el-table-column prop="budget" label="预算" width="120" />
            <el-table-column prop="status" label="状态" width="120" />
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
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
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
                <strong>Campaign 效果复盘</strong>
                <span
                  >逐 Campaign 展示达人/媒体、曝光、互动、CPM，最后一行为 SUM
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
                label="Campaign"
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
                    project.status || "进行中"
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
                  <strong>{{ formatCount(project.review.totalReach) }}</strong>
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
            <el-select
              v-model="importProjectId"
              class="import-project-select"
              placeholder="选择导入 Campaign"
            >
              <el-option
                v-for="project in projects"
                :key="project.id"
                :label="project.name"
                :value="project.id"
              />
            </el-select>
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
            <el-table-column prop="projectName" label="项目" min-width="160" />
            <el-table-column prop="resourceName" label="资源" min-width="160" />
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
                {{ cpmText(row.quoteAmount, primaryReach(row), row.currency) }}
              </template>
            </el-table-column>
            <el-table-column prop="releaseDate" label="发布日期" width="120" />
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

    <el-drawer
      v-model="executionDrawer"
      title="达人执行详情"
      size="620px"
      class="execution-drawer"
    >
      <template v-if="activeCooperation">
        <section class="drawer-profile">
          <div class="drawer-avatar">
            {{ String(activeCooperation.resourceName || "R").slice(0, 1) }}
          </div>
          <div>
            <el-tag
              :type="cooperationStageTag(activeCooperation)"
              effect="light"
            >
              {{ cooperationStageLabel(activeCooperation) }}
            </el-tag>
            <h3>{{ activeCooperation.resourceName || "未命名资源" }}</h3>
            <p>
              {{ activeCooperation.projectName || selectedProject?.name }} ·
              {{ activeCooperation.cooperationType || "未设置合作形式" }}
            </p>
          </div>
          <el-button type="primary" @click="editFromExecutionDetail">
            更新执行记录
          </el-button>
        </section>

        <section class="drawer-decision-grid">
          <div>
            <span>合作报价</span>
            <strong>
              {{
                moneyText(
                  activeCooperation.quoteAmount,
                  activeCooperation.currency
                )
              }}
            </strong>
          </div>
          <div>
            <span>当前 CPM</span>
            <strong>
              {{
                cpmText(
                  activeCooperation.quoteAmount,
                  primaryReach(activeCooperation),
                  activeCooperation.currency
                )
              }}
            </strong>
          </div>
          <div>
            <span>曝光 / 播放</span>
            <strong>{{ formatCount(primaryReach(activeCooperation)) }}</strong>
          </div>
          <div>
            <span>互动率</span>
            <strong>
              {{
                ratioPercent(
                  numberValue(activeCooperation.engagementCount) +
                    numberValue(activeCooperation.commentsCount),
                  primaryReach(activeCooperation)
                )
              }}
            </strong>
          </div>
        </section>

        <section class="drawer-next-action">
          <IconifyIconOnline icon="ri:focus-3-line" />
          <div>
            <span>建议下一步</span>
            <strong>
              {{ cooperationAction(activeCooperation) || "进入效果复盘" }}
            </strong>
            <p>{{ updatedTimeText(activeCooperation) }}</p>
          </div>
        </section>

        <section class="drawer-section">
          <div class="section-heading">
            <div>
              <strong>执行时间线</strong>
              <span>根据当前合作与交付状态生成</span>
            </div>
          </div>
          <div class="execution-timeline">
            <div
              v-for="stage in pipelineStages"
              :key="`timeline-${stage.key}`"
              :class="{
                completed:
                  pipelineStages.findIndex(item => item.key === stage.key) <=
                  pipelineStages.findIndex(
                    item => item.key === cooperationStage(activeCooperation)
                  )
              }"
            >
              <span><IconifyIconOnline :icon="stage.icon" /></span>
              <div>
                <strong>{{ stage.label }}</strong>
                <p>{{ stage.description }}</p>
              </div>
            </div>
          </div>
        </section>

        <section class="drawer-section">
          <div class="section-heading">
            <div>
              <strong>执行信息</strong>
              <span>用于协作交接与风险判断</span>
            </div>
          </div>
          <dl class="drawer-info-list">
            <div>
              <dt>合作状态</dt>
              <dd>{{ activeCooperation.status || "-" }}</dd>
            </div>
            <div>
              <dt>交付状态</dt>
              <dd>{{ activeCooperation.deliverableStatus || "-" }}</dd>
            </div>
            <div>
              <dt>发布日期</dt>
              <dd>{{ activeCooperation.releaseDate || "-" }}</dd>
            </div>
            <div>
              <dt>点击 / 转化</dt>
              <dd>
                {{ formatCount(activeCooperation.clicks) }} /
                {{ formatCount(activeCooperation.conversions) }}
              </dd>
            </div>
            <div>
              <dt>团队评分</dt>
              <dd>{{ activeCooperation.teamRating || "-" }}</dd>
            </div>
            <div>
              <dt>备注</dt>
              <dd>{{ activeCooperation.notes || "暂无备注" }}</dd>
            </div>
          </dl>
          <el-link
            v-if="activeCooperation.deliverableLinks"
            type="primary"
            :href="activeCooperation.deliverableLinks"
            target="_blank"
          >
            查看已发布内容
          </el-link>
        </section>
      </template>
    </el-drawer>

    <el-dialog
      v-model="projectDialog"
      :title="editingProjectId ? '编辑 Campaign' : '创建 Campaign'"
      width="640px"
    >
      <el-form :model="projectForm" label-width="96px">
        <el-form-item label="Campaign 名称"
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

    <el-dialog
      v-model="importDialog"
      title="上传数据表预览"
      width="92%"
      top="5vh"
    >
      <el-alert
        class="mb-3"
        type="info"
        :closable="false"
        :title="`文件：${importFileName || '-'}，共 ${importRows.length} 行，可导入 ${validImportRows.length} 行，异常 ${invalidImportRows.length} 行，疑似重复 ${duplicateImportRows.length} 行`"
      />
      <el-table
        :data="importRows"
        border
        height="460"
        :row-class-name="importRowClassName"
      >
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
        <el-table-column prop="engagementCount" label="转赞藏数" width="110" />
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
      <template #footer>
        <el-button @click="importDialog = false">取消</el-button>
        <el-button
          type="primary"
          :loading="importLoading"
          :disabled="validImportRows.length === 0"
          @click="submitImport"
        >
          确认导入 {{ validImportRows.length }} 行
        </el-button>
      </template>
    </el-dialog>
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
  width: 220px;
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
  .campaign-switcher,
  .execution-metrics,
  .pipeline-grid,
  .pending-grid,
  .drawer-decision-grid,
  .drawer-info-list {
    grid-template-columns: 1fr;
  }

  .campaign-switcher-main,
  .drawer-profile {
    flex-direction: column;
    align-items: flex-start;
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
}
</style>
