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
    ElMessage.success(editingProjectId.value ? "项目已更新" : "项目已创建");
    projectDialog.value = false;
    loadData();
  }
}

async function submitCooperation() {
  const payload = editingCooperationId.value
    ? { id: editingCooperationId.value, ...cooperationForm }
    : cooperationForm;
  const res = editingCooperationId.value
    ? await updateCooperation(payload)
    : await createCooperation(payload);
  if (res.code === 0) {
    ElMessage.success(
      editingCooperationId.value ? "合作记录已更新" : "合作记录已创建"
    );
    cooperationDialog.value = false;
    loadData();
  }
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

function accumulateProjectStat(stat: ReturnType<typeof emptyProjectStats>, row: any) {
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
    ElMessage.warning("请先选择导入项目");
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
    ElMessage.warning("请先选择导入项目");
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
        <span>Project Collaboration</span>
        <h1>项目合作</h1>
        <p>
          管理营销项目需求、候选资源、合作进度与效果回填，让资源评分持续来自真实复盘。
        </p>
      </div>
      <el-button type="primary" @click="openCreateProject">
        <IconifyIconOnline icon="ri:add-line" class="mr-1" />
        创建项目
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
          {{ ratioPercent(overallReview.totalEngagements, overallReview.totalReach) }}
        </strong>
        <p>转赞藏评 {{ formatCount(overallReview.totalEngagements) }}</p>
      </div>
      <div>
        <span>付费 CPM</span>
        <strong>{{ cpmText(overallReview.totalCost, overallReview.totalReach) }}</strong>
        <p>成本 {{ moneyText(overallReview.totalCost) }}</p>
      </div>
    </section>

    <el-card shadow="never" class="workspace-card">
      <el-tabs>
        <el-tab-pane label="项目需求">
          <div class="toolbar">
            <span class="toolbar-title">项目需求池</span>
          </div>
          <el-table :data="projects" stripe class="business-table">
            <el-table-column prop="name" label="项目名称" min-width="180" />
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
        <el-tab-pane label="项目复盘">
          <section class="review-report">
            <div class="review-report-heading">
              <div>
                <strong>项目效果复盘</strong>
                <span>逐项目展示达人/媒体、曝光、互动、CPM，最后一行为 SUM 总和</span>
              </div>
              <el-tag effect="plain">自动汇总</el-tag>
            </div>
            <el-table
              :data="projectReviewTableRows"
              border
              class="business-table"
              :row-class-name="reviewRowClassName"
            >
              <el-table-column prop="name" label="项目" min-width="180" fixed />
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
                  <el-tag effect="plain">{{ project.status || "进行中" }}</el-tag>
                  <el-button link type="primary" @click="openEditProject(project)">
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
                  <strong>{{ formatCount(project.review.totalEngagements) }}</strong>
                </div>
                <div>
                  <span>CPM</span>
                  <strong>
                    {{ cpmText(project.review.totalCost, project.review.totalReach) }}
                  </strong>
                </div>
              </div>
              <div class="project-ai-insight">
                <strong>智能效果摘要</strong>
                <p>{{ buildReviewInsight(project.name, project.review) }}</p>
              </div>
              <div class="best-item" v-if="project.review.bestItem">
                <span>最高触达内容</span>
                <el-link
                  v-if="project.review.bestItem.deliverableLinks"
                  type="primary"
                  :href="project.review.bestItem.deliverableLinks"
                  target="_blank"
                >
                  {{ project.review.bestItem.resourceName }}
                </el-link>
                <strong v-else>{{ project.review.bestItem.resourceName }}</strong>
                <span>{{ formatCount(primaryReach(project.review.bestItem)) }}</span>
              </div>
            </article>
          </div>
        </el-tab-pane>
        <el-tab-pane label="合作记录">
          <div class="toolbar">
            <el-select
              v-model="importProjectId"
              class="import-project-select"
              placeholder="选择导入项目"
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
                    numberValue(row.engagementCount) + numberValue(row.commentsCount),
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
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openEditCooperation(row)">
                  编辑
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog
      v-model="projectDialog"
      :title="editingProjectId ? '编辑项目需求' : '创建项目需求'"
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
          </el-select
        ></el-form-item>
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
        <el-button @click="cooperationDialog = false">取消</el-button>
        <el-button type="primary" @click="submitCooperation">保存</el-button>
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
  .project-review-metrics {
    grid-template-columns: 1fr;
  }

  .review-report-heading {
    align-items: flex-start;
  }

  .import-project-select {
    width: 100%;
  }
}
</style>
