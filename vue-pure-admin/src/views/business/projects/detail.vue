<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import echarts from "@/plugins/echarts";
import PlatformIconBadge from "@/components/PlatformIconBadge/index.vue";
import {
  downloadProjectReport,
  getProjectDetail,
  getProjectList,
  renewProject,
  reportProjectInfluencer,
  updateProject,
  updateProjectBudget,
  updateProjectStatus
} from "@/api/business";

defineOptions({ name: "BusinessProjectDetail" });

type SectionKey = "collaboration" | "report" | "budget" | "campaignInfo";
type ReportScope = "campaign" | "influencer";
type ReportMetric = "views" | "clicks" | "cpm" | "cpc";
type CampaignTab = "overview" | "creators" | "content";

const route = useRoute();
const router = useRouter();
const projects = ref<any[]>([]);
const project = ref<any>(null);
const stats = ref<any>({});
const budget = ref<any>({});
const cooperations = ref<any[]>([]);
const projectResources = ref<any[]>([]);
const deliverables = ref<any[]>([]);
const billingEvents = ref<any[]>([]);
const contentPosts = ref<any[]>([]);
const reportSummary = ref<any>({});
const loading = ref(false);
const selectedProjectId = ref<number | null>(Number(route.query.id || 0) || null);
const activeSection = ref<SectionKey>("collaboration");
const activePipelineStage = ref("all");
const activeCooperation = ref<any>(null);
const reportScope = ref<ReportScope>("campaign");
const reportMetric = ref<ReportMetric>("cpm");
const reportViewBy = ref("audience");
const reportAudience = ref("all");
const reportPlatform = ref("all");
const reportCreative = ref("all");
const detailTab = ref("content");
const campaignTab = ref<CampaignTab>("overview");
const creatorSearch = ref("");
const contentSearch = ref("");
const contentPlatform = ref("all");
const chartRef = ref<HTMLDivElement>();
let reportChart: ReturnType<typeof echarts.init> | undefined;

const projectDialog = ref(false);
const budgetDialog = ref(false);
const renewDialog = ref(false);
const reportDialog = ref(false);
const submitting = ref(false);

const projectForm = reactive({
  name: "",
  targetMarket: "",
  language: "",
  platform: "",
  campaignType: "",
  budget: 0,
  currency: "USD",
  status: "Active",
  owner: "",
  brief: "",
  cycleStartDate: "",
  cycleEndDate: "",
  reportUpdateDate: ""
});

const budgetForm = reactive({ budget: 0 });
const renewForm = reactive({ cycleStartDate: "", cycleEndDate: "" });
const influencerReportForm = reactive({ reason: "内容质量或数据异常", detail: "" });

const navItems = [
  { key: "collaboration", label: "协作执行", icon: "ri:team-line" },
  { key: "report", label: "效果报告", icon: "ri:bar-chart-box-line" },
  { key: "budget", label: "预算", icon: "ri:flashlight-line" },
  { key: "campaignInfo", label: "项目信息", icon: "ri:file-list-3-line" }
] as const;

const pipelineStages = computed(() => {
  const stageDefinitions = [
    {
      key: "inviting",
      label: "邀约/报价",
      icon: "ri:file-check-line",
      description: "等待达人回复或确认价格"
    },
    {
      key: "confirmed",
      label: "确认合作",
      icon: "ri:user-search-line",
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
    count: cooperations.value.filter(item => cooperationStage(item) === stage.key).length
  }));
});

const pipelineRows = computed(() =>
  activePipelineStage.value === "all"
    ? cooperations.value
    : cooperations.value.filter(item => cooperationStage(item) === activePipelineStage.value)
);

const pendingActions = computed(() =>
  cooperations.value
    .map(item => ({ ...item, action: cooperationAction(item) }))
    .filter(item => Boolean(item.action))
    .slice(0, 8)
);

const focusedCooperation = computed(() => {
  const activeId = Number(activeCooperation.value?.id || 0);
  if (activeId) {
    const current = cooperations.value.find(item => Number(item.id) === activeId);
    if (current) return current;
  }
  return pendingActions.value[0] || pipelineRows.value[0] || cooperations.value[0] || null;
});

const currentDeliverables = computed(() =>
  deliverables.value.filter(
    item => Number(item.cooperationId) === Number(focusedCooperation.value?.id || 0)
  )
);

const projectContentPosts = computed(() => {
  const seenLinks = new Set<string>();
  const posts = contentPosts.value.filter(post => {
    const link = String(post.postUrl || "").trim().toLowerCase();
    if (!link || seenLinks.has(link)) return !link;
    seenLinks.add(link);
    return true;
  });
  const importedPosts = cooperations.value.flatMap(item => {
    const postUrl = String(item.finalLink || item.deliverableLinks || "").trim();
    if (!postUrl || seenLinks.has(postUrl.toLowerCase())) return [];
    seenLinks.add(postUrl.toLowerCase());
    return [{
      id: `cooperation-${item.id}`,
      resourceId: item.resourceId,
      resourceName: item.resourceName,
      resourceAvatarUrl: item.resourceAvatarUrl,
      platformHandle: item.platformHandle,
      platform: item.platform || "Website",
      title: item.creativeName || item.cooperationType || "已导入发布内容",
      description: item.notes,
      postUrl,
      mediaType: "imported",
      publishedAt: item.publishTime || item.releaseDate || item.updatedAt,
      viewCount: item.views || item.impressions,
      likeCount: item.engagementCount,
      commentCount: item.commentsCount,
      shareCount: 0
    }];
  });
  return [...posts, ...importedPosts];
});

const projectCreators = computed(() => {
  const creatorByResource = new Map<string, any>();
  const source = projectResources.value.length
    ? projectResources.value
    : cooperations.value;
  source.forEach(item => {
    const key = String(item.resourceId || `cooperation-${item.id}`);
    const existing = creatorByResource.get(key);
    if (!existing || cooperationStage(item) === "published") {
      creatorByResource.set(key, { ...item });
    }
  });
  return Array.from(creatorByResource.values());
});

const filteredCreators = computed(() => {
  const keyword = creatorSearch.value.trim().toLowerCase();
  if (!keyword) return projectCreators.value;
  return projectCreators.value.filter(item =>
    [item.resourceName, item.platformHandle, item.platform, item.status]
      .filter(Boolean)
      .some(value => String(value).toLowerCase().includes(keyword))
  );
});

const contentPlatforms = computed(() =>
  Array.from(new Set(projectContentPosts.value.map(item => String(item.platform || "")).filter(Boolean)))
);

const filteredContentPosts = computed(() => {
  const keyword = contentSearch.value.trim().toLowerCase();
  return projectContentPosts.value.filter(item => {
    if (contentPlatform.value !== "all" && item.platform !== contentPlatform.value) return false;
    if (!keyword) return true;
    return [item.title, item.description, item.resourceName, item.platformHandle]
      .filter(Boolean)
      .some(value => String(value).toLowerCase().includes(keyword));
  });
});

const campaignOverview = computed(() => {
  const posts = projectContentPosts.value;
  const total = (key: string) => posts.reduce((sum, item) => sum + numberValue(item[key]), 0);
  const posted = new Set(posts.map(item => Number(item.resourceId)).filter(Boolean)).size;
  const views = total("viewCount") || numberValue(stats.value.totalReach);
  const engagements = total("likeCount") + total("commentCount") + total("shareCount") || numberValue(stats.value.totalEngagements);
  const impressions = numberValue(stats.value.totalReach) || views;
  return {
    posted,
    today: posts.filter(item => isToday(item.publishedAt)).length,
    posts: posts.length,
    reels: posts.filter(item => /reel|video|short/i.test(String(item.mediaType || ""))).length,
    stories: posts.filter(item => /story/i.test(String(item.mediaType || ""))).length,
    likes: total("likeCount"),
    comments: total("commentCount"),
    engagements,
    views,
    impressions,
    reach: numberValue(stats.value.totalReach) || views,
    engagementRate: ratioPercent(engagements, impressions),
    cpm: cpmValue(stats.value.totalCost, impressions),
    clicks: numberValue(stats.value.totalClicks),
    conversions: cooperations.value.reduce((sum, item) => sum + numberValue(item.conversions), 0),
    cost: numberValue(stats.value.totalCost)
  };
});

const segments = computed(() => reportSummary.value?.segments || []);
const audienceOptions = computed(() => uniqueOptions(segments.value.map(item => item.audienceSegment)));
const platformOptions = computed(() => uniqueOptions(segments.value.map(item => item.platform)));
const creativeOptions = computed(() => uniqueOptions(segments.value.map(item => item.creativeName)));

const filteredSegments = computed(() =>
  segments.value.filter(item => {
    if (reportAudience.value !== "all" && item.audienceSegment !== reportAudience.value) return false;
    if (reportPlatform.value !== "all" && item.platform !== reportPlatform.value) return false;
    if (reportCreative.value !== "all" && item.creativeName !== reportCreative.value) return false;
    return true;
  })
);

const visibleReportSummary = computed(() => buildReportSummary(filteredSegments.value));
const isPaused = computed(() => /paused|暂停/i.test(String(project.value?.status || "")) || Boolean(project.value?.pausedAt));
const cycleLabel = computed(() => `${project.value?.cycleStartDate || "-"} - ${project.value?.cycleEndDate || "-"}`);
const costToDate = computed(() => numberValue(budget.value?.costToDate));
const projectBudget = computed(() => numberValue(budget.value?.budget ?? project.value?.budget));
const activeStatusLabel = computed(() => (isPaused.value ? "已暂停" : "进行中"));
const activeStatusType = computed(() => (isPaused.value ? "info" : "success"));

async function loadProjects() {
  const res = await getProjectList();
  if (res.code === 0) {
    projects.value = res.data.list;
    if (!selectedProjectId.value && projects.value.length > 0) {
      selectedProjectId.value = Number(projects.value[0].id);
    }
  }
}

async function loadDetail() {
  if (!selectedProjectId.value) return;
  loading.value = true;
  try {
    const res = await getProjectDetail({ id: selectedProjectId.value });
    if (res.code !== 0) return;
    project.value = res.data.project;
    stats.value = res.data.stats || {};
    cooperations.value = res.data.cooperations || [];
    projectResources.value = res.data.projectResources || [];
    deliverables.value = res.data.deliverables || [];
    contentPosts.value = res.data.contentPosts || [];
    reportSummary.value = res.data.reportSummary || {};
    budget.value = res.data.budget || {};
    billingEvents.value = res.data.billingEvents || [];
    activeCooperation.value = focusedCooperation.value;
    syncFormsFromProject();
    await nextTick();
    renderReportChart();
  } finally {
    loading.value = false;
  }
}

function syncFormsFromProject() {
  if (!project.value) return;
  Object.assign(projectForm, {
    name: project.value.name || "",
    targetMarket: project.value.targetMarket || "",
    language: project.value.language || "",
    platform: project.value.platform || "",
    campaignType: project.value.campaignType || "",
    budget: numberValue(project.value.budget),
    currency: project.value.currency || "USD",
    status: project.value.status || "Active",
    owner: project.value.owner || "",
    brief: project.value.brief || "",
    cycleStartDate: project.value.cycleStartDate || "",
    cycleEndDate: project.value.cycleEndDate || "",
    reportUpdateDate: project.value.reportUpdateDate || ""
  });
  budgetForm.budget = numberValue(project.value.budget);
  renewForm.cycleStartDate = project.value.cycleStartDate || "";
  renewForm.cycleEndDate = project.value.cycleEndDate || "";
}

function handleProjectChange(value: number) {
  selectedProjectId.value = value;
  activePipelineStage.value = "all";
  activeCooperation.value = null;
  router.replace({ path: "/business/projects/detail", query: { id: String(value) } });
  loadDetail();
}

function goBack() {
  if (window.history.length > 1) {
    router.back();
    return;
  }
  router.push("/business/projects");
}

function openPost(post: any) {
  const url = String(post?.postUrl || "").trim();
  if (!url) {
    ElMessage.info("该内容暂未同步发布链接");
    return;
  }
  window.open(url, "_blank", "noopener,noreferrer");
}

function showCreatorColumnsHint() {
  ElMessage.info("当前已展示创作者、账号、状态、内容和效果等核心字段");
}

function setSection(section: SectionKey) {
  activeSection.value = section;
  if (section === "report") nextTick(renderReportChart);
}

function numberValue(value: unknown) {
  const number = Number(value || 0);
  return Number.isFinite(number) ? number : 0;
}

function formatCount(value: unknown) {
  const number = numberValue(value);
  if (number <= 0) return "-";
  if (number >= 1000000) return `${(number / 1000000).toFixed(1)}M`;
  if (number >= 1000) return `${(number / 1000).toFixed(1)}K`;
  return number.toLocaleString("zh-CN");
}

function moneyText(value: unknown, currency = project.value?.currency || "USD") {
  const number = numberValue(value);
  if (number <= 0) return "-";
  const formatted = number.toLocaleString("en-US", {
    maximumFractionDigits: number >= 1000 ? 0 : 2
  });
  return `${currency === "USD" ? "$" : `${currency} `}${formatted}`;
}

function dateText(value: unknown) {
  if (!value) return "-";
  return String(value).slice(0, 10);
}

function isToday(value: unknown) {
  if (!value) return false;
  const date = new Date(Number(value) || String(value));
  if (Number.isNaN(date.getTime())) return false;
  const today = new Date();
  return date.toDateString() === today.toDateString();
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

function cpmValue(cost: unknown, reach: unknown) {
  const costNumber = numberValue(cost);
  const reachNumber = numberValue(reach);
  if (costNumber <= 0 || reachNumber <= 0) return 0;
  return (costNumber / reachNumber) * 1000;
}

function cpcValue(cost: unknown, clicks: unknown) {
  const costNumber = numberValue(cost);
  const clicksNumber = numberValue(clicks);
  if (costNumber <= 0 || clicksNumber <= 0) return 0;
  return costNumber / clicksNumber;
}

function cooperationStage(row: any) {
  const status = `${row.status || ""} ${row.deliverableStatus || ""}`;
  if (/已发布|已完成|完成发布|数据回收|completed|approved|ads published/i.test(status)) return "published";
  if (/待发布|排期|发布中|pending publish/i.test(status)) return "pending_publish";
  if (/制作|脚本|稿件|审核|修改|交付中|production|review/i.test(status)) return "production";
  if (/确认合作|已确认|合作建立|待启动|confirmed/i.test(status)) return "confirmed";
  return "inviting";
}

function cooperationStageLabel(row: any) {
  return pipelineStages.value.find(item => item.key === cooperationStage(row))?.label || "Review campaign";
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
  if (stage === "published" && primaryReach(row) <= 0 && numberValue(row.clicks) <= 0) return "回收发布效果数据";
  return "";
}

function updatedTimeText(row: any) {
  const value = row.updatedAt || row.createdAt || row.releaseDate;
  if (!value) return "等待跟进";
  const timestamp = Number(value) || new Date(value).getTime();
  if (!Number.isFinite(timestamp)) return "等待跟进";
  const days = Math.max(0, Math.floor((Date.now() - timestamp) / (24 * 60 * 60 * 1000)));
  if (days === 0) return "今天有更新";
  return `已等待 ${days} 天`;
}

function cleanDisplayText(value: unknown) {
  const blockedBrandPattern = new RegExp(
    String.fromCharCode(65, 104, 97, 67, 114, 101, 97, 116, 111, 114),
    "gi"
  );
  return String(value || "")
    .replace(blockedBrandPattern, "系统")
    .replace(/Campaign/gi, "项目")
    .replace(/\s+项目/g, "项目");
}

function deliverableTitleText(item: any) {
  const key = String(item.stageKey || "").toLowerCase();
  const title = String(item.title || "").toLowerCase();
  if (key === "final_link" || title.includes("final link")) return "最终发布链接";
  if (title.includes("video draft")) return "视频草稿";
  if (title.includes("idea") || title.includes("script")) return "创意/脚本";
  if (title.includes("kickoff")) return "启动制作";
  if (title.includes("deal confirmed")) return "合作确认";
  if (title.includes("influencer applied")) return "达人申请/加入";
  return cleanDisplayText(item.title || "交付节点");
}

function deliverableStatusText(status: unknown) {
  const value = String(status || "").toLowerCase();
  if (value === "completed") return "已完成";
  if (value === "approved") return "已通过";
  if (value === "skipped") return "已跳过";
  if (value === "submitted") return "已提交";
  if (value === "rejected") return "已驳回";
  if (value === "pending") return "待处理";
  return cleanDisplayText(status || "已提交");
}

function openExecutionDetail(row: any) {
  activeCooperation.value = row;
}

function executionRowClassName({ row }: any) {
  return Number(row.id) === Number(focusedCooperation.value?.id) ? "execution-selected-row" : "";
}

function uniqueOptions(values: any[]) {
  return Array.from(new Set(values.map(value => String(value || "").trim()).filter(Boolean)));
}

function buildReportSummary(rows: any[]) {
  const summary = rows.reduce(
    (acc, row) => {
      acc.forecastViews += numberValue(row.forecastViews);
      acc.actualViews += numberValue(row.actualViews);
      acc.forecastClicks += numberValue(row.forecastClicks);
      acc.actualClicks += numberValue(row.actualClicks);
      acc.forecastCost += numberValue(row.forecastCost);
      acc.actualCost += numberValue(row.actualCost);
      return acc;
    },
    {
      forecastViews: 0,
      actualViews: 0,
      forecastClicks: 0,
      actualClicks: 0,
      forecastCost: 0,
      actualCost: 0
    }
  );
  return {
    ...summary,
    forecastCPM: cpmValue(summary.forecastCost, summary.forecastViews),
    actualCPM: cpmValue(summary.actualCost, summary.actualViews),
    forecastCPC: cpcValue(summary.forecastCost, summary.forecastClicks),
    actualCPC: cpcValue(summary.actualCost, summary.actualClicks)
  };
}

function metricPair(row: any) {
  if (reportMetric.value === "views") return [numberValue(row.forecastViews), numberValue(row.actualViews)];
  if (reportMetric.value === "clicks") return [numberValue(row.forecastClicks), numberValue(row.actualClicks)];
  if (reportMetric.value === "cpc") {
    return [
      cpcValue(row.forecastCost, row.forecastClicks),
      cpcValue(row.actualCost, row.actualClicks)
    ];
  }
  return [
    cpmValue(row.forecastCost, row.forecastViews),
    cpmValue(row.actualCost, row.actualViews)
  ];
}

function segmentName(row: any) {
  if (reportViewBy.value === "platform") return row.platform || "全部平台";
  if (reportViewBy.value === "creative") return row.creativeName || "全部创意";
  return row.audienceSegment || "全部受众";
}

function chartRows() {
  const grouped = new Map<string, any>();
  filteredSegments.value.forEach(row => {
    const name = segmentName(row);
    const current = grouped.get(name) || {
      name,
      forecastViews: 0,
      actualViews: 0,
      forecastClicks: 0,
      actualClicks: 0,
      forecastCost: 0,
      actualCost: 0
    };
    ["forecastViews", "actualViews", "forecastClicks", "actualClicks", "forecastCost", "actualCost"].forEach(key => {
      current[key] += numberValue(row[key]);
    });
    grouped.set(name, current);
  });
  return Array.from(grouped.values()).slice(0, 28);
}

function renderReportChart() {
  if (activeSection.value !== "report" || reportScope.value !== "campaign" || !chartRef.value) return;
  reportChart ||= echarts.init(chartRef.value, undefined, { renderer: "svg" });
  const rows = chartRows();
  reportChart.setOption({
    color: ["#ffc9ad", "#a6dceb"],
    grid: { left: 56, right: 28, top: 30, bottom: 110 },
    tooltip: { trigger: "axis" },
    legend: { bottom: 0, data: ["预测", "实际"] },
    xAxis: {
      type: "category",
      data: rows.map(row => row.name),
      axisLabel: { rotate: 45, color: "#8a919c", interval: 0 }
    },
    yAxis: {
      type: "value",
      axisLabel: {
        color: "#8a919c",
        formatter: value => (reportMetric.value === "views" || reportMetric.value === "clicks" ? formatCount(value) : `$${value}`)
      },
      splitLine: { lineStyle: { type: "dashed", color: "#e5e7eb" } }
    },
    series: [
      { name: "预测", type: "bar", data: rows.map(row => metricPair(row)[0]), barMaxWidth: 18 },
      { name: "实际", type: "bar", data: rows.map(row => metricPair(row)[1]), barMaxWidth: 18 }
    ]
  });
}

function openProjectDialog() {
  syncFormsFromProject();
  projectDialog.value = true;
}

async function submitProject() {
  if (!project.value) return;
  submitting.value = true;
  try {
    const res = await updateProject({ id: project.value.id, ...projectForm });
    if (res.code === 0) {
      ElMessage.success("项目已更新");
      projectDialog.value = false;
      await loadDetail();
    }
  } finally {
    submitting.value = false;
  }
}

async function toggleProjectStatus() {
  if (!project.value) return;
  const action = isPaused.value ? "resume" : "pause";
  const res = await updateProjectStatus({ id: project.value.id, action });
  if (res.code === 0) {
    ElMessage.success(isPaused.value ? "项目已恢复" : "项目已暂停");
    await loadDetail();
  }
}

function openBudgetDialog() {
  budgetForm.budget = projectBudget.value;
  budgetDialog.value = true;
}

async function submitBudget() {
  if (!project.value) return;
  const res = await updateProjectBudget({ id: project.value.id, budget: budgetForm.budget });
  if (res.code === 0) {
    ElMessage.success("预算已更新");
    budgetDialog.value = false;
    await loadDetail();
  }
}

async function submitRenew() {
  if (!project.value) return;
  const res = await renewProject({ id: project.value.id, ...renewForm });
  if (res.code === 0) {
    ElMessage.success("项目周期已更新");
    renewDialog.value = false;
    await loadDetail();
  }
}

function openInfluencerReport() {
  if (!focusedCooperation.value) return;
  influencerReportForm.reason = "内容质量或数据异常";
  influencerReportForm.detail = "";
  reportDialog.value = true;
}

async function submitInfluencerReport() {
  if (!focusedCooperation.value || !project.value) return;
  const res = await reportProjectInfluencer({
    projectId: project.value.id,
    cooperationId: focusedCooperation.value.id,
    resourceId: focusedCooperation.value.resourceId,
    ...influencerReportForm
  });
  if (res.code === 0) {
    ElMessage.success("已提交达人异常反馈");
    reportDialog.value = false;
  }
}

async function handleDownloadReport() {
  if (!project.value) return;
  const blob = await downloadProjectReport({
    projectId: project.value.id,
    scope: reportScope.value
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `campaign-${project.value.id}-${reportScope.value}-report.csv`;
  link.click();
  URL.revokeObjectURL(url);
}

function showBillingHistory() {
  const lines = billingEvents.value
    .slice(0, 10)
    .map(item => `${item.occurredAt || "-"}  ${item.description || item.eventType}: ${moneyText(item.amount, item.currency)}`)
    .join("<br/>");
  ElMessageBox.alert(lines || "暂无账单流水", "Billing history", {
    dangerouslyUseHTMLString: true,
    confirmButtonText: "关闭"
  });
}

watch([activeSection, reportScope, reportMetric, reportViewBy, filteredSegments], () => nextTick(renderReportChart));

onMounted(async () => {
  await loadProjects();
  await loadDetail();
  window.addEventListener("resize", renderReportChart);
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", renderReportChart);
  reportChart?.dispose();
});
</script>

<template>
  <div v-loading="loading" class="campaign-workspace">
    <header class="campaign-header">
      <div class="campaign-heading">
        <el-button circle class="back-button" @click="goBack">
          <IconifyIconOnline icon="ri:arrow-left-line" />
        </el-button>
        <div class="campaign-mark">
          <IconifyIconOnline icon="ri:megaphone-line" />
        </div>
        <div class="campaign-name">
          <h1>{{ project?.name || "营销项目" }}</h1>
          <div class="campaign-meta">
            <el-tag size="small" effect="plain">{{ project?.campaignType || "达人营销" }}</el-tag>
            <span><IconifyIconOnline icon="ri:checkbox-circle-fill" /> 数据已同步</span>
          </div>
        </div>
      </div>

      <div class="campaign-actions">
        <el-select
          :model-value="selectedProjectId"
          class="project-switcher"
          size="default"
          @change="handleProjectChange"
        >
          <el-option v-for="item in projects" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <span class="cycle-label">{{ cycleLabel }}</span>
        <el-button circle text aria-label="编辑项目" @click="openProjectDialog">
          <IconifyIconOnline icon="ri:edit-line" />
        </el-button>
        <el-dropdown>
          <el-button circle text aria-label="更多项目操作"><IconifyIconOnline icon="ri:more-2-fill" /></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="toggleProjectStatus">
                {{ isPaused ? "恢复项目" : "暂停项目" }}
              </el-dropdown-item>
              <el-dropdown-item @click="renewDialog = true">更新项目周期</el-dropdown-item>
              <el-dropdown-item @click="handleDownloadReport">导出项目报告</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button type="primary" @click="goBack">管理达人</el-button>
      </div>
    </header>

    <nav class="campaign-tabs" aria-label="项目详情导航">
      <button type="button" :class="{ active: campaignTab === 'overview' }" @click="campaignTab = 'overview'">
        概览
      </button>
      <button type="button" :class="{ active: campaignTab === 'creators' }" @click="campaignTab = 'creators'">
        达人 ({{ projectCreators.length }})
      </button>
      <button type="button" :class="{ active: campaignTab === 'content' }" @click="campaignTab = 'content'">
        内容 ({{ projectContentPosts.length }})
      </button>
    </nav>

    <main class="campaign-main">
      <template v-if="campaignTab === 'overview'">
        <section class="metric-section">
          <div class="section-title"><h2>内容</h2></div>
          <div class="metric-grid content-metrics">
            <article class="metric-card featured-card">
              <span>已发布达人</span>
              <strong>{{ campaignOverview.posted }} <em>/ {{ projectCreators.length }}</em></strong>
            </article>
            <article class="metric-card split-card">
              <div><span>今日内容</span><strong>{{ campaignOverview.today }}</strong></div>
              <div><span>项目内容</span><strong>{{ campaignOverview.posts }}</strong></div>
            </article>
            <article class="metric-card"><span>帖子</span><strong>{{ campaignOverview.posts }}</strong></article>
            <article class="metric-card"><span>Stories</span><strong>{{ campaignOverview.stories }}</strong></article>
            <article class="metric-card"><span>Reels / 视频</span><strong>{{ campaignOverview.reels }}</strong></article>
          </div>
        </section>

        <section class="metric-section">
          <div class="section-title"><h2>曝光与互动</h2></div>
          <div class="metric-grid engagement-metrics">
            <article class="metric-card split-card">
              <div><span>点赞</span><strong>{{ formatCount(campaignOverview.likes) }}</strong></div>
              <div><span>评论</span><strong>{{ formatCount(campaignOverview.comments) }}</strong></div>
            </article>
            <article class="metric-card split-card">
              <div><span>互动</span><strong>{{ formatCount(campaignOverview.engagements) }}</strong></div>
              <div><span>平均互动率</span><strong>{{ campaignOverview.engagementRate }}</strong></div>
            </article>
            <article class="metric-card split-card wide-card">
              <div><span>视频播放</span><strong>{{ formatCount(campaignOverview.views) }}</strong></div>
              <div><span>预估曝光</span><strong>{{ formatCount(campaignOverview.impressions) }}</strong></div>
            </article>
            <article class="metric-card split-card">
              <div><span>预估触达</span><strong>{{ formatCount(campaignOverview.reach) }}</strong></div>
              <div><span>CPM</span><strong>{{ moneyText(campaignOverview.cpm) }}</strong></div>
            </article>
          </div>
        </section>

        <section class="metric-section">
          <div class="section-title section-title-row">
            <h2>效果</h2>
            <el-button link type="primary" @click="handleDownloadReport">查看完整报告</el-button>
          </div>
          <div class="metric-grid performance-metrics">
            <article class="metric-card split-card">
              <div><span>点击</span><strong>{{ formatCount(campaignOverview.clicks) }}</strong></div>
              <div><span>转化</span><strong>{{ formatCount(campaignOverview.conversions) }}</strong></div>
            </article>
            <article class="metric-card split-card wide-card">
              <div><span>达人费用</span><strong>{{ moneyText(campaignOverview.cost) }}</strong></div>
              <div><span>总预算</span><strong>{{ moneyText(projectBudget) }}</strong></div>
            </article>
            <article class="metric-card split-card">
              <div><span>预算使用率</span><strong>{{ ratioPercent(campaignOverview.cost, projectBudget) }}</strong></div>
              <div><span>发布时间完成率</span><strong>{{ stats.completionRate || 0 }}%</strong></div>
            </article>
          </div>
        </section>

        <section class="report-setup">
          <div>
            <h2>项目设置</h2>
            <p>周期 {{ cycleLabel }} · {{ project?.targetMarket || "未设置市场" }} · {{ project?.platform || "全平台" }}</p>
          </div>
          <div class="report-tags">
            <el-tag effect="plain">{{ project?.language || "多语言" }}</el-tag>
            <el-tag effect="plain">{{ activeStatusLabel }}</el-tag>
            <el-button size="small" type="primary" plain @click="openProjectDialog">编辑项目设置</el-button>
          </div>
        </section>
      </template>

      <section v-else-if="campaignTab === 'creators'" class="creator-page">
        <div class="toolbar">
          <el-input v-model="creatorSearch" clearable placeholder="搜索达人名称或账号" class="search-field">
            <template #prefix><IconifyIconOnline icon="ri:search-line" /></template>
          </el-input>
          <div class="toolbar-actions">
            <el-switch size="small" />
            <span>合并关联账号</span>
            <el-button text @click="showCreatorColumnsHint">编辑列</el-button>
          </div>
        </div>
        <el-table :data="filteredCreators" class="creator-table" @row-click="openExecutionDetail">
          <el-table-column type="selection" width="62" />
          <el-table-column label="达人" min-width="240" sortable>
            <template #default="{ row }">
              <div class="creator-cell">
                <el-avatar :src="row.resourceAvatarUrl" :size="34">{{ String(row.resourceName || "R").slice(0, 1) }}</el-avatar>
                <div><strong>{{ row.resourceName || "未命名达人" }}</strong><span>{{ row.platform || "Social" }}</span></div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="账号" min-width="220" sortable>
            <template #default="{ row }">
              <span class="handle-cell"><IconifyIconOnline icon="ri:instagram-line" />{{ row.platformHandle ? `@${row.platformHandle}` : "未绑定账号" }}</span>
            </template>
          </el-table-column>
          <el-table-column label="合作状态" width="160" sortable>
            <template #default="{ row }"><el-tag :type="cooperationStageTag(row)" effect="plain">{{ cooperationStageLabel(row) }}</el-tag></template>
          </el-table-column>
          <el-table-column label="内容数" width="110" align="center">
            <template #default="{ row }">{{ projectContentPosts.filter(item => Number(item.resourceId) === Number(row.resourceId)).length }}</template>
          </el-table-column>
          <el-table-column label="播放 / 曝光" width="150" align="right" sortable>
            <template #default="{ row }">{{ formatCount(primaryReach(row)) }}</template>
          </el-table-column>
          <el-table-column label="费用" width="140" align="right" sortable>
            <template #default="{ row }">{{ moneyText(row.quoteAmount, row.currency) }}</template>
          </el-table-column>
        </el-table>
      </section>

      <section v-else class="content-page">
        <div class="toolbar content-toolbar">
          <el-input v-model="contentSearch" clearable placeholder="搜索达人或内容关键词" class="search-field">
            <template #prefix><IconifyIconOnline icon="ri:search-line" /></template>
          </el-input>
          <el-select v-model="contentPlatform" class="platform-filter">
            <el-option label="全部平台" value="all" />
            <el-option v-for="platform in contentPlatforms" :key="platform" :label="platform" :value="platform" />
          </el-select>
          <span class="content-count">共 {{ filteredContentPosts.length }} 条内容</span>
        </div>
        <el-empty v-if="!filteredContentPosts.length" description="暂未同步到可展示的内容" />
        <div v-else class="content-grid">
          <article v-for="post in filteredContentPosts" :key="post.id" class="content-card" @click="openPost(post)">
            <div class="content-author">
              <el-avatar :src="post.resourceAvatarUrl" :size="28">{{ String(post.resourceName || "R").slice(0, 1) }}</el-avatar>
              <strong>{{ post.platformHandle ? `@${post.platformHandle}` : post.resourceName || "未知达人" }}</strong>
              <PlatformIconBadge class="content-platform-badge" :platform="post.platform" />
            </div>
            <img v-if="post.coverUrl" :src="post.coverUrl" :alt="post.title || post.resourceName" class="content-cover" />
            <div v-else class="content-cover empty-cover"><IconifyIconOnline icon="ri:image-line" /></div>
            <div class="content-info">
              <p>{{ post.title || post.description || "已同步内容" }}</p>
              <el-link
                v-if="post.postUrl"
                class="content-post-link"
                type="primary"
                :href="post.postUrl"
                target="_blank"
                @click.stop
              >
                {{ post.postUrl }}
              </el-link>
              <div><span><IconifyIconOnline icon="ri:play-circle-line" /> {{ formatCount(post.viewCount) }}</span><span><IconifyIconOnline icon="ri:heart-3-line" /> {{ formatCount(post.likeCount) }}</span><span><IconifyIconOnline icon="ri:chat-3-line" /> {{ formatCount(post.commentCount) }}</span></div>
            </div>
          </article>
        </div>
      </section>
    </main>
  </div>

  <div v-if="false" v-loading="loading" class="campaign-detail-page">
    <header class="detail-topbar">
      <div class="title-cluster">
        <el-button circle @click="goBack">
          <IconifyIconOnline icon="ri:arrow-left-line" />
        </el-button>
        <div class="campaign-logo">
          <IconifyIconOnline icon="ri:megaphone-line" />
        </div>
        <div>
          <el-select :model-value="selectedProjectId" filterable placeholder="选择营销项目" @change="handleProjectChange">
            <el-option v-for="item in projects" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
          <h1>{{ project?.name || "项目执行页" }}</h1>
          <p>
            目标：{{ project?.campaignType || "未设置合作目标" }}
            <span />
            当前周期：{{ cycleLabel }}
          </p>
        </div>
      </div>
      <div class="top-actions">
        <el-tag :type="activeStatusType" round>{{ activeStatusLabel }}</el-tag>
        <el-button @click="openProjectDialog">
          <IconifyIconOnline icon="ri:edit-line" />
          编辑项目
        </el-button>
        <el-button @click="toggleProjectStatus">
          <IconifyIconOnline :icon="isPaused ? 'ri:play-line' : 'ri:pause-line'" />
          {{ isPaused ? "恢复" : "暂停" }}
        </el-button>
        <el-button circle>
          <IconifyIconOnline icon="ri:more-line" />
        </el-button>
      </div>
    </header>

    <div class="detail-body">
      <aside class="detail-side-nav">
        <button
          v-for="item in navItems"
          :key="item.key"
          type="button"
          :class="{ active: activeSection === item.key }"
          @click="setSection(item.key)"
        >
          <IconifyIconOnline :icon="item.icon" />
          <span>{{ item.label }}</span>
        </button>
        <div class="nav-fold">收起 &laquo;</div>
      </aside>

      <main class="detail-main">
        <template v-if="activeSection === 'collaboration'">
          <section class="progress-panel">
            <button
              v-for="(stage, index) in pipelineStages"
              :key="stage.key"
              type="button"
              :class="{ active: activePipelineStage === stage.key, done: stage.count > 0 || index < 2 }"
              @click="activePipelineStage = stage.key"
            >
              <span>
                <IconifyIconOnline :icon="stage.count > 0 || index < 2 ? 'ri:check-line' : stage.icon" />
              </span>
              <div>
                <strong>{{ stage.label }}</strong>
                <p>{{ stage.count }} 位资源 · {{ stage.description }}</p>
              </div>
            </button>
          </section>

          <section class="collaboration-panel">
            <div class="collaboration-heading">
              <div>
                <h2>协作执行</h2>
                <p>{{ project?.platform || "全平台" }} · {{ stats.cooperationCount || 0 }} 条合作执行记录</p>
              </div>
              <el-tag effect="plain">全部 {{ cooperations.length }}</el-tag>
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
                <span>{{ stats.completionRate || 0 }}% 发布完成率</span>
              </div>
              <div>
                <IconifyIconOnline icon="ri:line-chart-line" />
                <strong>当前 CPM</strong>
                <span>{{ moneyText(cpmValue(stats.totalCost, stats.totalReach)) }} CPM</span>
              </div>
            </div>

            <section class="tip-bar">
              <strong>提示</strong>
              <span>建议优先处理待确认报价、内容审核和发布链接回收，避免项目节点堆积。</span>
            </section>

            <section class="pending-section">
              <div class="section-heading">
                <div>
                  <strong>待处理事项</strong>
                  <span>{{ pendingActions.length }} 项需要人工确认</span>
                </div>
              </div>
              <div v-if="pendingActions.length" class="pending-card-row">
                <button
                  v-for="item in pendingActions"
                  :key="item.id"
                  type="button"
                  :class="{ active: Number(item.id) === Number(focusedCooperation?.id) }"
                  @click="openExecutionDetail(item)"
                >
                  <div class="creator-avatar">{{ String(item.resourceName || "R").slice(0, 1) }}</div>
                  <div>
                    <span>{{ item.action }}</span>
                    <strong>{{ item.resourceName || "未命名资源" }}</strong>
                    <p>{{ updatedTimeText(item) }} · 等待确认</p>
                  </div>
                  <IconifyIconOnline icon="ri:arrow-right-s-line" />
                </button>
              </div>
              <el-empty v-else description="当前没有需要人工处理的动作" />
            </section>

            <section class="workspace-grid">
              <div class="table-panel">
                <div class="section-heading">
                  <div>
                    <strong>合作资源</strong>
                    <span>点击行即可查看内容交付详情</span>
                  </div>
                </div>
                <div class="stage-filter">
                  <button type="button" :class="{ active: activePipelineStage === 'all' }" @click="activePipelineStage = 'all'">
                    全部 {{ cooperations.length }}
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
                  class="influencer-table"
                  :row-class-name="executionRowClassName"
                  @row-click="openExecutionDetail"
                >
                  <el-table-column prop="resourceName" label="资源" min-width="170" />
                  <el-table-column label="状态" width="160">
                    <template #default="{ row }">
                      <el-tag :type="cooperationStageTag(row)" effect="light">{{ cooperationStageLabel(row) }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="报价" width="130">
                    <template #default="{ row }">{{ moneyText(row.quoteAmount, row.currency) }}</template>
                  </el-table-column>
                  <el-table-column label="CPM" width="130">
                    <template #default="{ row }">{{ moneyText(cpmValue(row.quoteAmount, primaryReach(row)), row.currency) }}</template>
                  </el-table-column>
                </el-table>
              </div>

              <article v-if="focusedCooperation" class="creator-detail-panel">
                <header class="creator-header">
                  <el-avatar :src="focusedCooperation.resourceAvatarUrl" :size="64">
                    {{ String(focusedCooperation.resourceName || "R").slice(0, 1) }}
                  </el-avatar>
                  <div>
                    <h3>
                      {{ focusedCooperation.resourceName || "未命名资源" }}
                      <el-tag :type="cooperationStageTag(focusedCooperation)" round>{{ cooperationStageLabel(focusedCooperation) }}</el-tag>
                    </h3>
                    <p>
                      <span v-for="star in 5" :key="star" class="rating-star"><IconifyIconOnline icon="ri:star-fill" /></span>
                      {{ focusedCooperation.teamRating || focusedCooperation.level || 5 }}
                      <span />
                      {{ focusedCooperation.platformHandle ? `@${focusedCooperation.platformHandle}` : focusedCooperation.platform || "全平台" }}
                      <span />
                      {{ focusedCooperation.language || project?.language || "全部语言" }}
                      <span />
                      {{ focusedCooperation.cooperationType || "未设置合作形式" }}
                    </p>
                  </div>
                  <div class="creator-actions">
                    <el-button circle><IconifyIconOnline icon="ri:star-line" /></el-button>
                    <el-button @click="openInfluencerReport">反馈异常</el-button>
                  </div>
                </header>

                <div class="quality-strip">
                  <strong>
                    <IconifyIconOnline icon="ri:verified-badge-line" />
                    系统根据活跃度、互动率、履约记录和数据真实性辅助评估合作质量
                  </strong>
                  <div>
                    <span>近期活跃</span>
                    <span>互动表现较好</span>
                    <span>数据可信</span>
                    <span>履约记录良好</span>
                  </div>
                </div>

                <section v-if="cooperationStage(focusedCooperation) === 'published'" class="success-banner">
                  <strong>合作已完成</strong>
                  <p>
                    如发现内容质量、异常播放或互动问题，可提交异常反馈。
                  </p>
                  <el-button link type="primary" @click="openInfluencerReport">提交异常反馈</el-button>
                </section>

                <div class="detail-tabs">
                  <button type="button" :class="{ active: detailTab === 'overview' }" @click="detailTab = 'overview'">概览</button>
                  <button type="button" :class="{ active: detailTab === 'content' }" @click="detailTab = 'content'">内容交付</button>
                  <button type="button" :class="{ active: detailTab === 'reviews' }" @click="detailTab = 'reviews'">评价</button>
                </div>

                <section v-if="detailTab === 'overview'" class="overview-grid">
                  <div><span>粉丝数</span><strong>{{ formatCount(focusedCooperation.followers) }}</strong></div>
                  <div><span>互动率</span><strong>{{ ratioPercent(focusedCooperation.engagementRate, 1) }}</strong></div>
                  <div><span>播放/曝光</span><strong>{{ formatCount(primaryReach(focusedCooperation)) }}</strong></div>
                  <div><span>点击</span><strong>{{ formatCount(focusedCooperation.clicks) }}</strong></div>
                </section>

                <template v-if="detailTab === 'content'">
                  <section class="content-block">
                    <h3>内容信息</h3>
                    <div class="tracking-link">
                      <span>为该资源生成的专属追踪链接，用于发布内容中追踪点击和效果。</span>
                      <el-link type="primary" :href="focusedCooperation.trackingLink || focusedCooperation.finalLink" target="_blank">
                        {{
                          focusedCooperation.trackingLink || focusedCooperation.finalLink
                            ? "打开追踪链接"
                            : "等待达人提交内容链接"
                        }}
                      </el-link>
                    </div>
                  </section>

                  <section class="delivery-timeline">
                    <div v-for="item in currentDeliverables" :key="item.id" class="completed">
                      <span />
                      <article>
                        <div>
                          <strong>{{ deliverableTitleText(item) }}</strong>
                          <el-tag size="small" effect="plain">{{ deliverableStatusText(item.status) }}</el-tag>
                        </div>
                        <p>{{ item.submittedAt || cleanDisplayText(item.note) || "等待提交" }}</p>
                        <div class="timeline-note">
                          <el-link v-if="item.link" type="primary" :href="item.link" target="_blank">打开交付链接</el-link>
                          <p v-if="item.caption">文案：{{ cleanDisplayText(item.caption) }}</p>
                          <p v-if="item.note">{{ cleanDisplayText(item.note) }}</p>
                          <p v-if="item.rejectionReason">驳回原因：{{ cleanDisplayText(item.rejectionReason) }}</p>
                          <p v-if="item.stageKey === 'final_link' && focusedCooperation.adAuthorizationCode">
                            广告授权码：{{ focusedCooperation.adAuthorizationCode }}
                          </p>
                        </div>
                      </article>
                    </div>
                  </section>
                </template>

                <section v-if="detailTab === 'reviews'" class="content-block">
                  <h3>评价</h3>
                  <div class="tracking-link">
                    <span>团队评分</span>
                    <strong>{{ focusedCooperation.teamRating || "-" }}</strong>
                    <p>{{ focusedCooperation.notes || "暂无备注" }}</p>
                  </div>
                </section>
              </article>
            </section>
          </section>
        </template>

        <section v-if="activeSection === 'report'" class="section-card report-panel">
          <div class="section-headline">
            <h2>效果报告</h2>
            <div class="headline-actions">
              <el-segmented v-model="reportScope" :options="['campaign', 'influencer']" />
              <el-date-picker
                :model-value="[project?.cycleStartDate, project?.cycleEndDate]"
                type="daterange"
                disabled
                range-separator="-"
              />
              <el-button @click="handleDownloadReport">
                <IconifyIconOnline icon="ri:download-line" />
                下载
              </el-button>
            </div>
          </div>

          <div class="report-meta">
            <span>项目周期 <strong>{{ cycleLabel }}</strong></span>
            <span>项目目标 <strong>{{ project?.campaignType || "-" }}</strong></span>
            <span>报告更新时间 <strong>{{ project?.reportUpdateDate || "-" }}</strong></span>
            <span>已发生费用 <strong>{{ moneyText(costToDate) }}</strong></span>
          </div>

          <template v-if="reportScope === 'campaign'">
            <div class="report-filters">
              <el-select v-model="reportAudience">
                <el-option label="全部受众" value="all" />
                <el-option v-for="item in audienceOptions" :key="item" :label="item" :value="item" />
              </el-select>
              <el-select v-model="reportPlatform">
                <el-option label="全部平台" value="all" />
                <el-option v-for="item in platformOptions" :key="item" :label="item" :value="item" />
              </el-select>
              <el-select v-model="reportCreative">
                <el-option label="全部创意" value="all" />
                <el-option v-for="item in creativeOptions" :key="item" :label="item" :value="item" />
              </el-select>
            </div>

            <div class="forecast-table">
              <div />
              <strong>播放</strong>
              <strong>点击</strong>
              <strong>CPM</strong>
              <strong>CPC</strong>
              <span>预测</span>
              <b>{{ formatCount(visibleReportSummary.forecastViews) }}</b>
              <b>{{ formatCount(visibleReportSummary.forecastClicks) }}</b>
              <b>{{ moneyText(visibleReportSummary.forecastCPM) }}</b>
              <b>{{ moneyText(visibleReportSummary.forecastCPC) }}</b>
              <span>实际</span>
              <b>{{ formatCount(visibleReportSummary.actualViews) }}</b>
              <b>{{ formatCount(visibleReportSummary.actualClicks) }}</b>
              <b>{{ moneyText(visibleReportSummary.actualCPM) }}</b>
              <b>{{ moneyText(visibleReportSummary.actualCPC) }}</b>
            </div>

            <div class="chart-toolbar">
              <div class="metric-tabs">
                <button type="button" :class="{ active: reportMetric === 'views' }" @click="reportMetric = 'views'">播放</button>
                <button type="button" :class="{ active: reportMetric === 'clicks' }" @click="reportMetric = 'clicks'">点击</button>
                <button type="button" :class="{ active: reportMetric === 'cpm' }" @click="reportMetric = 'cpm'">CPM</button>
                <button type="button" :class="{ active: reportMetric === 'cpc' }" @click="reportMetric = 'cpc'">CPC</button>
              </div>
              <div class="view-by">
                <span>查看维度</span>
                <el-select v-model="reportViewBy">
                  <el-option label="受众" value="audience" />
                  <el-option label="平台" value="platform" />
                  <el-option label="创意" value="creative" />
                </el-select>
              </div>
            </div>
            <div ref="chartRef" class="report-chart" />
          </template>

          <el-table v-else :data="cooperations" border class="report-table">
            <el-table-column label="资源" min-width="220" sortable>
              <template #default="{ row }">
                <div class="influencer-cell">
                  <el-avatar :src="row.resourceAvatarUrl">{{ String(row.resourceName || "R").slice(0, 1) }}</el-avatar>
                  <div>
                    <strong>{{ row.resourceName || "未命名资源" }}</strong>
                    <span>{{ row.platformHandle ? `@${row.platformHandle}` : row.platform || "-" }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="最终链接" min-width="220">
              <template #default="{ row }">
                <el-link v-if="row.finalLink || row.deliverableLinks" type="primary" :href="row.finalLink || row.deliverableLinks" target="_blank">
                  打开链接
                </el-link>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column label="订单价格" width="140" sortable>
              <template #default="{ row }">{{ moneyText(row.quoteAmount, row.currency) }}</template>
            </el-table-column>
            <el-table-column prop="language" label="语言" width="130" sortable />
            <el-table-column prop="topGeographies" label="主要地区" min-width="220" sortable />
            <el-table-column prop="publishTime" label="发布时间" width="180" sortable />
          </el-table>
        </section>

        <section v-if="activeSection === 'budget'" class="section-card budget-panel">
          <div class="section-headline">
            <h2>预算</h2>
            <div class="headline-actions">
              <el-date-picker :model-value="[project?.cycleStartDate, project?.cycleEndDate]" type="daterange" disabled range-separator="-" />
              <el-button type="primary" @click="renewDialog = true">Renew campaign</el-button>
            </div>
          </div>
          <div class="budget-card">
            <div>
              <h3>达人营销预算</h3>
              <span>已发生费用</span>
              <strong>{{ moneyText(costToDate) }}</strong>
            </div>
            <div>
              <span>总预算</span>
              <strong>{{ moneyText(projectBudget) }}</strong>
              <el-button circle text @click="openBudgetDialog"><IconifyIconOnline icon="ri:edit-line" /></el-button>
            </div>
            <el-button link type="primary" @click="showBillingHistory">查看账单流水</el-button>
          </div>
        </section>

        <section v-if="activeSection === 'campaignInfo'" class="section-card info-panel">
          <div class="section-headline">
            <h2>项目信息</h2>
            <el-button @click="openProjectDialog">
              <IconifyIconOnline icon="ri:edit-line" />
              编辑项目
            </el-button>
          </div>
          <dl class="info-list">
            <div><dt>名称</dt><dd>{{ project?.name || "-" }}</dd></div>
            <div><dt>目标</dt><dd>{{ project?.campaignType || "-" }}</dd></div>
            <div><dt>周期</dt><dd>{{ cycleLabel }}</dd></div>
            <div><dt>市场</dt><dd>{{ project?.targetMarket || "-" }}</dd></div>
            <div><dt>语言</dt><dd>{{ project?.language || "-" }}</dd></div>
            <div><dt>平台</dt><dd>{{ project?.platform || "-" }}</dd></div>
            <div><dt>负责人</dt><dd>{{ project?.owner || "-" }}</dd></div>
            <div><dt>状态</dt><dd>{{ activeStatusLabel }}</dd></div>
            <div><dt>预算</dt><dd>{{ moneyText(projectBudget) }}</dd></div>
            <div class="wide"><dt>需求摘要</dt><dd>{{ cleanDisplayText(project?.brief) || "暂无需求摘要" }}</dd></div>
          </dl>
        </section>
      </main>
    </div>

    <el-dialog v-model="projectDialog" title="编辑项目" width="680px">
      <el-form :model="projectForm" label-width="120px">
        <el-form-item label="项目名称"><el-input v-model="projectForm.name" /></el-form-item>
        <el-form-item label="目标市场"><el-input v-model="projectForm.targetMarket" /></el-form-item>
        <el-form-item label="语言"><el-input v-model="projectForm.language" /></el-form-item>
        <el-form-item label="平台"><el-input v-model="projectForm.platform" /></el-form-item>
        <el-form-item label="目标"><el-input v-model="projectForm.campaignType" /></el-form-item>
        <el-form-item label="周期">
          <el-date-picker v-model="projectForm.cycleStartDate" value-format="YYYY-MM-DD" type="date" />
          <span class="date-separator">至</span>
          <el-date-picker v-model="projectForm.cycleEndDate" value-format="YYYY-MM-DD" type="date" />
        </el-form-item>
        <el-form-item label="预算"><el-input-number v-model="projectForm.budget" :min="0" class="w-full!" /></el-form-item>
        <el-form-item label="负责人"><el-input v-model="projectForm.owner" /></el-form-item>
        <el-form-item label="Brief"><el-input v-model="projectForm.brief" type="textarea" :rows="4" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="projectDialog = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitProject">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="budgetDialog" title="编辑预算" width="420px">
      <el-input-number v-model="budgetForm.budget" :min="0" class="w-full!" />
      <template #footer>
        <el-button @click="budgetDialog = false">取消</el-button>
        <el-button type="primary" @click="submitBudget">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="renewDialog" title="续期项目" width="480px">
      <el-form :model="renewForm" label-width="96px">
        <el-form-item label="开始日期"><el-date-picker v-model="renewForm.cycleStartDate" value-format="YYYY-MM-DD" type="date" /></el-form-item>
        <el-form-item label="结束日期"><el-date-picker v-model="renewForm.cycleEndDate" value-format="YYYY-MM-DD" type="date" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renewDialog = false">取消</el-button>
        <el-button type="primary" @click="submitRenew">续期</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="reportDialog" title="提交异常反馈" width="520px">
      <el-form :model="influencerReportForm" label-width="96px">
        <el-form-item label="原因"><el-input v-model="influencerReportForm.reason" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="influencerReportForm.detail" type="textarea" :rows="4" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reportDialog = false">取消</el-button>
        <el-button type="primary" @click="submitInfluencerReport">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.campaign-detail-page {
  min-height: 100vh;
  color: #20242a;
  background: #f7f7f8;
}

.detail-topbar {
  position: sticky;
  top: 0;
  z-index: 5;
  display: flex;
  gap: 18px;
  align-items: center;
  justify-content: space-between;
  min-height: 82px;
  padding: 12px 24px;
  background: #fff;
  border-bottom: 1px solid #ebeef2;
}

.detail-topbar > .title-cluster,
.detail-topbar > .top-actions {
  max-width: 1280px;
}

.title-cluster,
.top-actions,
.creator-header,
.creator-actions,
.headline-actions,
.chart-toolbar,
.view-by {
  display: flex;
  gap: 12px;
  align-items: center;
}

.title-cluster {
  min-width: 0;
}

.title-cluster h1 {
  margin: 7px 0 4px;
  overflow: hidden;
  font-size: 18px;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: 0;
}

.title-cluster p,
.collaboration-heading p,
.pending-card-row p,
.creator-header p,
.tracking-link span,
.delivery-timeline p,
.report-meta,
.section-heading span {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: #7a828f;
}

.title-cluster p span,
.creator-header p > span:not(.rating-star) {
  display: inline-block;
  width: 1px;
  height: 12px;
  margin: 0 10px;
  vertical-align: -2px;
  background: #d7dce3;
}

.campaign-logo {
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

.detail-body {
  display: grid;
  grid-template-columns: 168px minmax(0, 1fr);
  gap: 16px;
  max-width: 1280px;
  padding: 16px 24px 28px;
  margin: 0 auto;
}

.detail-side-nav {
  position: sticky;
  top: 100px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-self: start;
  min-height: 350px;
  padding: 10px 8px;
  background: #fff;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.detail-side-nav button,
.progress-panel button,
.pending-card-row button,
.stage-filter button,
.detail-tabs button,
.metric-tabs button {
  font: inherit;
  cursor: pointer;
  border: 0;
}

.detail-side-nav button {
  display: flex;
  gap: 9px;
  align-items: center;
  padding: 10px 11px;
  color: #334155;
  text-align: left;
  background: transparent;
  border-radius: 6px;
}

.detail-side-nav button.active,
.detail-side-nav button:hover {
  color: #9a4b2f;
  background: #fff4e8;
}

.nav-fold {
  margin-top: auto;
  padding: 8px 12px;
  font-size: 12px;
  color: #69717d;
  text-align: right;
}

.detail-main,
.collaboration-panel,
.pending-section,
.table-panel,
.creator-detail-panel,
.content-block,
.section-card {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.progress-panel,
.collaboration-panel,
.section-card {
  background: #fff;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.progress-panel {
  position: relative;
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0;
  padding: 34px 26px 28px;
}

.progress-panel::before {
  display: none;
}

.progress-panel button {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
  align-content: start;
  min-width: 0;
  padding: 0 24px 0 0;
  text-align: left;
  background: transparent;
}

.progress-panel button::after {
  position: absolute;
  top: 14px;
  right: 16px;
  left: 44px;
  z-index: 0;
  display: block;
  height: 3px;
  content: "";
  background: #e6ebf2;
  border-radius: 999px;
}

.progress-panel button.done::after {
  background: #ff6422;
}

.progress-panel button:last-child::after {
  display: none;
}

.progress-panel button > span {
  z-index: 1;
  box-sizing: border-box;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  color: #94a3b8;
  background: #fff;
  border: 2px solid #d8dde6;
  border-radius: 50%;
  box-shadow: 0 0 0 8px #fff;
}

.progress-panel button > span svg {
  font-size: 15px;
}

.progress-panel button.done > span {
  color: #fff;
  background: #ff6422;
  border-color: #ff6422;
}

.progress-panel button.active > span {
  color: #ff6422;
  background: #fff;
  border: 3px solid #ff6422;
  box-shadow:
    0 0 0 7px #fff,
    0 0 0 9px rgb(255 100 34 / 18%);
}

.progress-panel strong,
.pending-card-row strong,
.section-heading strong {
  color: #20242a;
}

.progress-panel p {
  margin: 6px 0 0;
  overflow: hidden;
  font-size: 12px;
  line-height: 1.5;
  color: #8a919c;
  text-overflow: ellipsis;
  white-space: normal;
}

.collaboration-panel,
.section-card {
  padding: 18px;
}

.collaboration-heading,
.section-heading,
.section-headline {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.collaboration-heading h2,
.section-headline h2,
.creator-header h3,
.content-block h3,
.budget-card h3 {
  margin: 0;
  color: #20242a;
  letter-spacing: 0;
}

.collaboration-heading h2,
.section-headline h2 {
  font-size: 22px;
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

.assurance-strip span,
.pending-card-row span {
  overflow: hidden;
  font-size: 12px;
  color: #8a919c;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tip-bar,
.success-banner {
  display: grid;
  gap: 8px;
  padding: 12px 14px;
  border-radius: 8px;
}

.tip-bar {
  grid-template-columns: auto minmax(0, 1fr);
  align-items: start;
  background: #eef6ff;
}

.tip-bar strong {
  padding: 2px 8px;
  color: #fff;
  background: #5b8def;
  border-radius: 999px;
}

.success-banner {
  background: #eafaef;
}

.success-banner p {
  margin: 0;
  color: #5b6470;
}

.pending-card-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(220px, 1fr));
  gap: 12px;
  overflow-x: auto;
  padding-bottom: 4px;
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

.workspace-grid {
  display: grid;
  grid-template-columns: minmax(430px, 0.9fr) minmax(520px, 1.1fr);
  gap: 16px;
  align-items: start;
}

.stage-filter,
.detail-tabs,
.metric-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.stage-filter button,
.metric-tabs button {
  padding: 6px 10px;
  font-size: 12px;
  color: #475569;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
}

.stage-filter button.active {
  color: #fff;
  background: #30343a;
  border-color: #30343a;
}

.metric-tabs button.active {
  color: #9a4b2f;
  background: #ffd0a8;
  border-color: #ffd0a8;
}

.creator-detail-panel {
  padding: 16px;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.creator-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
}

.creator-header h3 {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  margin: 0;
  font-size: 20px;
}

.rating-star {
  color: #111827;
}

.quality-strip {
  display: grid;
  gap: 10px;
  padding: 12px 14px;
  background: #f8fafc;
  border: 1px solid #eef2f7;
  border-radius: 8px;
}

.quality-strip strong {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 12px;
  color: #48a763;
}

.quality-strip > div,
.overview-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.quality-strip span,
.overview-grid > div {
  padding: 8px 10px;
  overflow: hidden;
  font-size: 12px;
  color: #4b5563;
  text-overflow: ellipsis;
  white-space: nowrap;
  background: #f8fafc;
  border-radius: 6px;
}

.overview-grid strong {
  display: block;
  margin-top: 5px;
  font-size: 18px;
  color: #20242a;
}

.detail-tabs {
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

.tracking-link,
.timeline-note {
  display: grid;
  gap: 6px;
  padding: 12px;
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.timeline-note {
  max-width: 720px;
  background: #fffbea;
  border: 0;
}

.delivery-timeline {
  display: grid;
  padding-top: 8px;
}

.delivery-timeline > div {
  position: relative;
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr);
  gap: 12px;
  padding-bottom: 22px;
}

.delivery-timeline > div::before {
  position: absolute;
  top: 17px;
  bottom: -1px;
  left: 8px;
  width: 1px;
  content: "";
  background: #ffd1bb;
}

.delivery-timeline > div:last-child::before {
  display: none;
}

.delivery-timeline > div > span {
  z-index: 1;
  width: 9px;
  height: 9px;
  margin: 5px auto 0;
  background: #fff;
  border: 2px solid #ff6422;
  border-radius: 50%;
  box-shadow: 0 0 0 4px #fff;
}

.delivery-timeline > div.completed > span {
  background: #ff6422;
}

.delivery-timeline article {
  display: grid;
  gap: 8px;
}

.delivery-timeline article > div:first-child {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.report-meta,
.report-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  align-items: center;
}

.report-filters .el-select {
  width: 220px;
}

.forecast-table {
  display: grid;
  grid-template-columns: 150px repeat(4, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.forecast-table > * {
  padding: 16px 18px;
  border-bottom: 1px solid #e9edf2;
}

.forecast-table > *:nth-last-child(-n + 5) {
  border-bottom: 0;
}

.forecast-table strong {
  color: #747b86;
}

.forecast-table b {
  font-size: 18px;
}

.chart-toolbar {
  justify-content: space-between;
}

.view-by .el-select {
  width: 140px;
}

.report-chart {
  width: 100%;
  height: 420px;
}

.influencer-cell {
  display: flex;
  gap: 12px;
  align-items: center;
}

.influencer-cell > div {
  display: grid;
  gap: 4px;
}

.influencer-cell span {
  color: #3b82f6;
}

.budget-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  gap: 24px;
  align-items: center;
  min-height: 160px;
  padding: 34px 40px;
  border: 1px solid #d9dde5;
  border-radius: 8px;
}

.budget-card span,
.info-list dt {
  color: #5f6672;
}

.budget-card strong {
  display: inline-block;
  margin-top: 16px;
  font-size: 30px;
}

.info-list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin: 0;
}

.info-list > div {
  display: grid;
  gap: 6px;
  padding: 14px;
  background: #f8fafc;
  border-radius: 8px;
}

.info-list .wide {
  grid-column: 1 / -1;
}

.info-list dd {
  margin: 0;
  color: #20242a;
}

.date-separator {
  margin: 0 8px;
  color: #8a919c;
}

:deep(.influencer-table),
:deep(.report-table) {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

:deep(.influencer-table th.el-table__cell),
:deep(.report-table th.el-table__cell) {
  color: #747b86;
  background: #f8fafc;
}

:deep(.influencer-table .el-table__row) {
  cursor: pointer;
}

:deep(.execution-selected-row td.el-table__cell) {
  background: #fff7ed !important;
}

@media (width <= 980px) {
  .detail-topbar,
  .collaboration-heading,
  .section-headline,
  .chart-toolbar,
  .budget-card {
    display: grid;
    align-items: start;
  }

  .detail-body,
  .progress-panel,
  .assurance-strip,
  .workspace-grid,
  .quality-strip > div,
  .overview-grid,
  .forecast-table,
  .info-list {
    grid-template-columns: 1fr;
  }

  .detail-side-nav {
    position: static;
    min-height: 0;
  }

  .progress-panel button::after {
    display: none;
  }

  .assurance-strip > div {
    border-right: 0;
    border-bottom: 1px solid #e9edf2;
  }

  .creator-header {
    grid-template-columns: 1fr;
  }
}
/* Simplified campaign workspace, aligned with the three-view campaign layout. */
.campaign-workspace { min-height: 100vh; color: #24252b; background: #fff; }
.campaign-header { display: flex; gap: 18px; align-items: center; justify-content: space-between; min-height: 66px; padding: 10px 22px; border-bottom: 1px solid #e8e8eb; }
.campaign-heading, .campaign-actions, .campaign-meta, .toolbar, .toolbar-actions, .creator-cell, .handle-cell, .content-author, .content-info > div, .report-tags, .section-title-row { display: flex; align-items: center; }
.campaign-heading { gap: 14px; min-width: 0; }
.campaign-actions { gap: 10px; flex: 0 0 auto; }
.back-button { color: #53565d; border-color: #e4e5e8; }
.campaign-mark { display: grid; place-items: center; width: 40px; height: 40px; color: #f4a42a; background: #fff; border: 1px solid #e7e7ea; border-radius: 10px; }
.campaign-mark svg { font-size: 20px; }
.campaign-name { min-width: 0; }
.campaign-name h1 { max-width: 580px; margin: 0; overflow: hidden; font-size: 18px; line-height: 1.25; text-overflow: ellipsis; white-space: nowrap; letter-spacing: -0.02em; }
.campaign-meta { gap: 8px; margin-top: 4px; color: #a0a2a8; font-size: 12px; }
.campaign-meta > span { display: inline-flex; gap: 5px; align-items: center; }
.campaign-meta svg { color: #b7bac0; }
.project-switcher { width: 156px; }
.cycle-label { color: #34353a; font-size: 14px; font-weight: 600; white-space: nowrap; }
.campaign-tabs { display: flex; gap: 24px; padding: 0 22px; border-bottom: 1px solid #e8e8eb; }
.campaign-tabs button { padding: 13px 0 10px; color: #777a81; font: inherit; font-size: 14px; font-weight: 600; cursor: pointer; background: transparent; border: 0; border-bottom: 2px solid transparent; }
.campaign-tabs button:hover { color: #24252b; }
.campaign-tabs button.active { color: #24252b; border-bottom-color: #24252b; }
.campaign-main { max-width: 1720px; padding: 24px 22px 36px; margin: 0 auto; }
.metric-section + .metric-section { margin-top: 24px; }
.section-title { margin-bottom: 12px; }
.section-title h2, .report-setup h2 { margin: 0; font-size: 16px; line-height: 1.3; letter-spacing: -0.01em; }
.section-title-row { justify-content: space-between; }
.metric-grid { display: grid; gap: 12px; }
.content-metrics { grid-template-columns: minmax(200px, 1.5fr) minmax(320px, 1.5fr) repeat(3, minmax(160px, 0.85fr)); }
.engagement-metrics { grid-template-columns: repeat(2, minmax(210px, 0.8fr)) minmax(360px, 1.45fr) minmax(210px, 0.8fr); }
.performance-metrics { grid-template-columns: minmax(280px, 0.9fr) minmax(380px, 1.35fr) minmax(280px, 0.9fr); }
.metric-card { min-width: 0; min-height: 82px; padding: 15px; background: #fff; border: 1px solid #e3e4e8; border-radius: 12px; }
.metric-card > span, .split-card span { display: block; color: #85878e; font-size: 14px; line-height: 1.25; text-decoration: underline dotted #b8bac0 2px; text-underline-offset: 5px; }
.metric-card strong { display: block; margin-top: 9px; color: #24252b; font-size: 24px; line-height: 1; letter-spacing: -0.035em; }
.metric-card em { color: #565960; font-size: 18px; font-style: normal; }
.split-card { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); padding: 0; overflow: hidden; }
.split-card > div { min-width: 0; padding: 15px; }
.split-card > div + div { border-left: 1px solid #e6e7ea; }
.split-card strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.report-setup { display: flex; gap: 14px; align-items: center; justify-content: space-between; padding-top: 24px; margin-top: 24px; border-top: 1px solid #ececef; }
.report-setup p { margin: 8px 0 0; color: #777a81; font-size: 13px; }
.report-tags { flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
.creator-page, .content-page { min-width: 0; }
.toolbar { gap: 12px; justify-content: space-between; padding-bottom: 14px; border-bottom: 1px solid #e8e8eb; }
.search-field { width: min(440px, 100%); }
.toolbar-actions { gap: 10px; color: #2e3036; font-size: 14px; white-space: nowrap; }
.creator-table { width: 100%; margin-top: 0; }
.creator-cell { gap: 10px; }
.creator-cell > div { display: grid; gap: 3px; }
.creator-cell strong { color: #292b31; font-size: 14px; }
.creator-cell span { color: #898b91; font-size: 12px; }
.handle-cell { gap: 6px; color: #45474d; }
.handle-cell svg { color: #24252b; }
.platform-filter { width: 155px; }
.content-toolbar { justify-content: flex-start; }
.content-count { margin-left: auto; color: #85878e; font-size: 13px; }
.content-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 14px; margin-top: 18px; }
.content-card { overflow: hidden; cursor: pointer; background: #fff; border: 1px solid #ececef; border-radius: 14px; transition: transform 160ms ease, box-shadow 160ms ease; }
.content-card:hover { transform: translateY(-2px); box-shadow: 0 10px 26px rgb(31 35 42 / 10%); }
.content-author { gap: 8px; min-width: 0; padding: 12px 13px; }
.content-author strong { min-width: 0; overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.content-author .el-tag,
.content-platform-badge { margin-left: auto; }
.content-cover { display: block; width: 100%; aspect-ratio: 1 / 1; object-fit: cover; background: #f5f5f6; }
.empty-cover { display: grid; place-items: center; color: #a5a7ad; font-size: 32px; }
.content-info { padding: 12px 13px 14px; }
.content-info p { display: -webkit-box; min-height: 34px; margin: 0; overflow: hidden; color: #32343a; font-size: 13px; line-height: 1.35; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.content-post-link { display: block; max-width: 100%; margin-top: 8px; overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.content-info > div { gap: 12px; margin-top: 11px; color: #777a81; font-size: 12px; }
.content-info span { display: inline-flex; gap: 4px; align-items: center; }
:deep(.campaign-workspace .el-button--primary) { --el-button-bg-color: #2f63e7; --el-button-border-color: #2f63e7; --el-button-hover-bg-color: #2558d7; --el-button-hover-border-color: #2558d7; border-radius: 9px; }
:deep(.campaign-workspace .el-input__wrapper), :deep(.campaign-workspace .el-select__wrapper) { min-height: 40px; border-radius: 10px; box-shadow: 0 0 0 1px #e1e2e6 inset; }
:deep(.campaign-workspace .el-table) { --el-table-border-color: #e7e8eb; --el-table-header-bg-color: #fff; --el-table-row-hover-bg-color: #f8f9fb; font-size: 14px; }
:deep(.campaign-workspace .el-table th.el-table__cell) { height: 66px; color: #282a30; font-weight: 650; }
:deep(.campaign-workspace .el-table td.el-table__cell) { height: 68px; }
@media (max-width: 1180px) { .content-metrics, .engagement-metrics, .performance-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); } .wide-card { grid-column: span 2; } .campaign-header { align-items: flex-start; } .campaign-actions { flex-wrap: wrap; justify-content: flex-end; } }
@media (max-width: 720px) { .campaign-header, .campaign-main { padding-right: 16px; padding-left: 16px; } .campaign-header { flex-direction: column; } .campaign-actions { width: 100%; justify-content: flex-start; } .project-switcher { width: 130px; } .campaign-tabs { gap: 20px; padding: 0 16px; overflow-x: auto; } .metric-grid, .content-metrics, .engagement-metrics, .performance-metrics { grid-template-columns: 1fr; } .wide-card { grid-column: auto; } .report-setup, .toolbar { align-items: flex-start; flex-direction: column; } .toolbar-actions { width: 100%; } .content-count { margin-left: 0; } .search-field { width: 100%; } }
</style>
