<script setup lang="ts">
import { computed, reactive, ref, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import * as XLSX from "xlsx";
import {
  getResourceList,
  createResource,
  updateResource,
  deleteResource,
  syncResource,
  syncAllResources,
  getResourceSyncStatus,
  importResources,
  getTagList,
  createTag,
  getCooperationList
} from "@/api/business";

defineOptions({ name: "BusinessResources" });

const router = useRouter();
const loading = ref(false);
const syncingAll = ref(false);
const showSyncCard = ref(false);
const syncingResourceIds = reactive<Record<number, boolean>>({});
const dialogVisible = ref(false);
const profileDialogVisible = ref(false);
const importDialog = ref(false);
const importLoading = ref(false);
const editingId = ref<number | null>(null);
const list = ref<any[]>([]);
const allCooperations = ref<any[]>([]);
const selectedResource = ref<any | null>(null);
const total = ref(0);
const currentPage = ref(1);
const pageSize = ref(10);
const workbookSheets = ref<any[]>([]);
const selectedSheets = ref<string[]>([]);
const importFileName = ref("");
const tagOptions = ref<any[]>([]);
const syncStatus = ref<any>({});
const avatarLoadFailed = reactive<Record<string, boolean>>({});
const avatarLoaded = reactive<Record<string, boolean>>({});
let syncPollTimer: ReturnType<typeof setInterval> | null = null;
const defaultPlatformOptions = [
  "YouTube",
  "TikTok",
  "Instagram",
  "Newsletter",
  "Website",
  "X",
  "Facebook",
  "LinkedIn"
];
const platformOptions = ref<string[]>(loadPlatformOptions());

const search = reactive({
  name: "",
  country: "",
  platform: "",
  status: "",
  level: ""
});

const form = reactive({
  name: "",
  resourceType: "KOL",
  country: "",
  region: "",
  city: "",
  language: "",
  platform: "YouTube",
  industry: "",
  category: "",
  mediaOutlet: "",
  tier: "",
  title: "",
  contact: "",
  owner: "",
  regionTeam: "",
  referenceSource: "",
  shippingAddress: "",
  website: "",
  tagNames: [] as string[],
  status: "可合作",
  followers: 0,
  engagementRate: 0,
  avgViews: 0,
  contentTypes: "",
  platformUrl: "",
  score: 70,
  riskLevel: "低",
  notes: ""
});

const parsedImportRows = computed(() => parseSelectedSheets());
const validImportRows = computed(() =>
  parsedImportRows.value.filter(row => row.errors.length === 0)
);
const invalidImportRows = computed(() =>
  parsedImportRows.value.filter(row => row.errors.length > 0)
);
const duplicateImportRows = computed(() =>
  parsedImportRows.value.filter(row => row.duplicate)
);
const invalidSheets = computed(() => validateSelectedSheets());
const canSubmitImport = computed(
  () =>
    workbookSheets.value.length > 0 &&
    selectedSheets.value.length > 0 &&
    invalidSheets.value.length === 0 &&
    invalidImportRows.value.length === 0 &&
    validImportRows.value.length > 0
);
const latestSyncJob = computed(() => syncStatus.value?.latestJob || null);
const syncRunning = computed(() => latestSyncJob.value?.status === "运行中");
const syncProgress = computed(() => {
  const job = latestSyncJob.value;
  if (!job?.totalCount) return 0;
  const done =
    Number(job.successCount || 0) +
    Number(job.failedCount || 0) +
    Number(job.skippedCount || 0);
  return Math.min(100, Math.round((done / Number(job.totalCount)) * 100));
});
const cooperationStatsByResource = computed(() => {
  const map = new Map<number, any>();
  allCooperations.value.forEach(item => {
    const resourceId = Number(item.resourceId || 0);
    if (!resourceId) return;
    if (!map.has(resourceId)) {
      map.set(resourceId, {
        count: 0,
        totalReach: 0,
        totalViews: 0,
        totalEngagements: 0,
        totalCost: 0,
        latest: null as any
      });
    }
    const stat = map.get(resourceId);
    const reach = primaryReach(item);
    stat.count += 1;
    stat.totalReach += reach;
    stat.totalViews += numberValue(item.views);
    stat.totalEngagements +=
      numberValue(item.engagementCount) + numberValue(item.commentsCount);
    stat.totalCost += numberValue(item.quoteAmount);
    if (!stat.latest || dateRank(item) > dateRank(stat.latest)) {
      stat.latest = item;
    }
  });
  return map;
});
const selectedCooperations = computed(() => {
  const id = Number(selectedResource.value?.id || 0);
  return allCooperations.value.filter(item => Number(item.resourceId) === id);
});
const selectedCooperationStats = computed(() =>
  selectedResource.value
    ? cooperationStats(selectedResource.value)
    : emptyCooperationStats()
);

function loadPlatformOptions() {
  try {
    const stored = JSON.parse(
      localStorage.getItem("businessResourcePlatforms") || "[]"
    );
    const values = Array.isArray(stored) ? stored : [];
    return Array.from(new Set([...defaultPlatformOptions, ...values])).filter(
      Boolean
    );
  } catch {
    return defaultPlatformOptions;
  }
}

function persistPlatformOptions() {
  localStorage.setItem(
    "businessResourcePlatforms",
    JSON.stringify(platformOptions.value)
  );
}

function addPlatformOption(value: string) {
  const platform = String(value || "").trim();
  if (!platform) return;
  if (!platformOptions.value.includes(platform)) {
    platformOptions.value.push(platform);
    persistPlatformOptions();
  }
}

function handlePlatformChange(value: string) {
  addPlatformOption(value);
}

function removePlatformOption(value: string, event?: Event) {
  event?.stopPropagation();
  platformOptions.value = platformOptions.value.filter(item => item !== value);
  if (form.platform === value) form.platform = "";
  if (search.platform === value) search.platform = "";
  persistPlatformOptions();
}

async function loadTags() {
  const res = await getTagList();
  if (res.code === 0) {
    tagOptions.value = res.data.filter(item => item.status === "启用");
  }
}

async function ensureTagIds(names: string[]) {
  const normalizedNames = Array.from(
    new Set(names.map(name => String(name || "").trim()).filter(Boolean))
  );
  const tagIds: number[] = [];
  for (const name of normalizedNames) {
    let tag = tagOptions.value.find(
      item => String(item.name).toLowerCase() === name.toLowerCase()
    );
    if (!tag) {
      const res = await createTag({
        name,
        category: "自定义标签",
        color: "#2563eb",
        status: "启用"
      });
      if (res.code !== 0) {
        ElMessage.warning(`标签 ${name} 创建失败`);
        continue;
      }
      await loadTags();
      tag = tagOptions.value.find(
        item => String(item.name).toLowerCase() === name.toLowerCase()
      );
    }
    if (tag?.id) tagIds.push(Number(tag.id));
  }
  return tagIds;
}

async function loadData() {
  loading.value = true;
  const params = {
    ...search,
    currentPage: currentPage.value,
    pageSize: pageSize.value
  };
  const [resourceRes, cooperationRes] = await Promise.all([
    getResourceList(params),
    getCooperationList()
  ]);
  if (resourceRes.code === 0) {
    const { data } = resourceRes;
    list.value = data.list;
    total.value = data.total;
  }
  if (cooperationRes.code === 0) {
    allCooperations.value = cooperationRes.data.list || [];
  }
  loading.value = false;
}

function formatDateTime(value: unknown) {
  if (!value) return "-";
  const time = Number(value);
  if (!Number.isFinite(time)) return String(value);
  return new Date(time).toLocaleString("zh-CN");
}

function displayText(value: unknown, fallback = "-") {
  const text = String(value ?? "").trim();
  if (!text || text === "<nil>" || text === "undefined") return fallback;
  return text;
}

function formatCount(value: unknown) {
  const number = Number(value || 0);
  if (!Number.isFinite(number) || number <= 0) return "-";
  return number.toLocaleString("zh-CN");
}

function numberValue(value: unknown) {
  const number = Number(value || 0);
  return Number.isFinite(number) ? number : 0;
}

function dateRank(row: any) {
  const releaseTime = row?.releaseDate ? new Date(row.releaseDate).getTime() : 0;
  return Number.isFinite(releaseTime) && releaseTime > 0
    ? releaseTime
    : Number(row?.updatedAt || 0);
}

function primaryReach(row: any) {
  return numberValue(row.impressions) || numberValue(row.views);
}

function currencyText(value: unknown, currency = "USD") {
  const number = numberValue(value);
  if (number <= 0) return "-";
  return `${currency} ${number.toLocaleString("zh-CN", {
    maximumFractionDigits: 0
  })}`;
}

function percentText(value: unknown) {
  const number = Number(value || 0);
  if (!Number.isFinite(number) || number <= 0) return "-";
  const percent = number > 1 ? number : number * 100;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function ratioPercent(numerator: unknown, denominator: unknown) {
  const top = numberValue(numerator);
  const bottom = numberValue(denominator);
  if (top <= 0 || bottom <= 0) return "-";
  const percent = (top / bottom) * 100;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function cooperationStats(row: any) {
  return (
    cooperationStatsByResource.value.get(Number(row.id)) ||
    emptyCooperationStats()
  );
}

function emptyCooperationStats() {
  return {
    count: 0,
    totalReach: 0,
    totalViews: 0,
    totalEngagements: 0,
    totalCost: 0,
    latest: null
  };
}

function cooperationEngagementText(row: any) {
  const stat = cooperationStats(row);
  return ratioPercent(stat.totalEngagements, stat.totalReach);
}

function cooperationCpmText(row: any) {
  const stat = cooperationStats(row);
  if (stat.totalCost <= 0 || stat.totalReach <= 0) return "-";
  return currencyText((stat.totalCost / stat.totalReach) * 1000);
}

function performanceDeltaText(row: any) {
  const stat = cooperationStats(row);
  const avgPlatformViews = numberValue(row.avgViews);
  if (stat.count <= 0 || avgPlatformViews <= 0 || stat.totalViews <= 0) {
    return "暂无可比均值";
  }
  const avgCooperationViews = stat.totalViews / stat.count;
  const delta = ((avgCooperationViews - avgPlatformViews) / avgPlatformViews) * 100;
  const prefix = delta >= 0 ? "高于平台均值" : "低于平台均值";
  return `${prefix} ${Math.abs(delta).toFixed(0)}%`;
}

function locationText(row: any) {
  return displayText(row.country || row.region || row.city);
}

function mediaAccountTitle(row: any) {
  return displayText(row.mediaOutlet || row.platformHandle || row.title || row.platformUrl);
}

function mediaAccountSub(row: any) {
  return [row.tier, row.contact].map(item => displayText(item, "")).filter(Boolean).join(" · ");
}

function avatarKey(row: any) {
  return String(row.id || row.platformUrl || row.name || "");
}

function avatarText(row: any) {
  const name = String(row.name || row.platformHandle || "?").trim();
  return name.slice(0, 1).toUpperCase() || "?";
}

function markAvatarFailed(row: any) {
  avatarLoadFailed[avatarKey(row)] = true;
}

function markAvatarLoaded(row: any) {
  avatarLoaded[avatarKey(row)] = true;
}

async function loadSyncStatus(autoPoll = false) {
  const res = await getResourceSyncStatus();
  if (res.code !== 0) return;
  syncStatus.value = res.data || {};
  if (autoPoll && syncRunning.value) startSyncPolling();
}

function startSyncPolling() {
  if (syncPollTimer) return;
  syncPollTimer = setInterval(async () => {
    const res = await getResourceSyncStatus();
    if (res.code !== 0) return;
    syncStatus.value = res.data || {};
    if (!syncRunning.value) {
      stopSyncPolling();
      syncingAll.value = false;
      loadData();
    }
  }, 3000);
}

function stopSyncPolling() {
  if (!syncPollTimer) return;
  clearInterval(syncPollTimer);
  syncPollTimer = null;
}

function searchData() {
  currentPage.value = 1;
  loadData();
}

function handleCurrentChange(page: number) {
  currentPage.value = page;
  loadData();
}

function handleSizeChange(size: number) {
  pageSize.value = size;
  currentPage.value = 1;
  loadData();
}

function resetForm() {
  editingId.value = null;
  Object.assign(form, {
    name: "",
    resourceType: "KOL",
    country: "",
    region: "",
    city: "",
    language: "",
    platform: "YouTube",
    industry: "",
    category: "",
    mediaOutlet: "",
    tier: "",
    title: "",
    contact: "",
    owner: "",
    regionTeam: "",
    referenceSource: "",
    shippingAddress: "",
    website: "",
    tagNames: [],
    status: "可合作",
    followers: 0,
    engagementRate: 0,
    avgViews: 0,
    contentTypes: "",
    platformUrl: "",
    score: 70,
    riskLevel: "低",
    notes: ""
  });
}

function openCreate() {
  resetForm();
  dialogVisible.value = true;
}

function openEdit(row: any) {
  resetForm();
  editingId.value = row.id;
  Object.assign(form, row);
  dialogVisible.value = true;
}

function openProfile(row: any) {
  selectedResource.value = row;
  profileDialogVisible.value = true;
}

function openPosts(row: any) {
  router.push({
    path: "/business/resource-posts",
    query: { resourceId: row.id }
  });
}

async function submit() {
  addPlatformOption(form.platform);
  const tagIds = await ensureTagIds(form.tagNames);
  const payload = editingId.value
    ? { id: editingId.value, ...form, tagIds }
    : { ...form, tagIds };
  const res = editingId.value
    ? await updateResource(payload)
    : await createResource(payload);
  if (res.code === 0) {
    ElMessage.success("保存成功");
    dialogVisible.value = false;
    loadData();
  }
}

function remove(row: any) {
  ElMessageBox.confirm(`确认删除 ${row.name} 吗？`, "提示", {
    type: "warning"
  }).then(async () => {
    const res = await deleteResource({ id: row.id });
    if (res.code === 0) {
      ElMessage.success("删除成功");
      loadData();
    }
  });
}

async function syncAll() {
  await ElMessageBox.confirm(
    "将按抓取控制中的平台开关，在后台异步同步 YouTube、Instagram、TikTok 资源数据。",
    "立即同步KOL数据",
    {
      type: "info",
      confirmButtonText: "开始同步",
      cancelButtonText: "取消"
    }
  );
  showSyncCard.value = true;
  syncingAll.value = true;
  const res = await syncAllResources();
  if (res.code === 0) {
    ElMessage.success(res.data?.message || "异步同步任务已启动");
    await loadSyncStatus(true);
    startSyncPolling();
  } else {
    showSyncCard.value = false;
    syncingAll.value = false;
    ElMessage.warning(res.message || "启动同步失败");
  }
}

function isSyncablePlatform(platform: string) {
  return ["youtube", "instagram", "ins", "tiktok"].includes(
    String(platform || "")
      .trim()
      .toLowerCase()
  );
}

async function syncOne(row: any) {
  const id = Number(row.id || 0);
  if (!id) return;
  if (!isSyncablePlatform(row.platform)) {
    ElMessage.warning("当前平台暂不支持同步");
    return;
  }
  syncingResourceIds[id] = true;
  try {
    const res = await syncResource({ id });
    if (res.code === 0) {
      const warnings = Array.isArray(res.data?.warnings)
        ? res.data.warnings.filter(Boolean)
        : [];
      if (warnings.length > 0) {
        ElMessage.warning(`同步完成，${warnings.join("；")}`);
      } else {
        ElMessage.success("同步成功");
      }
      await Promise.all([loadData(), loadSyncStatus()]);
    } else {
      ElMessage.warning(res.message || "同步失败");
      await loadData();
    }
  } finally {
    syncingResourceIds[id] = false;
  }
}

const importTemplateColumns = [
  { key: "name", label: "Name", required: true },
  { key: "email", label: "Email", required: true },
  { key: "mediaOutlet", label: "Media Outlet", required: false },
  { key: "category", label: "Category", required: false },
  { key: "tier", label: "Tier", required: false },
  { key: "title", label: "Title", required: false },
  { key: "base", label: "Base", required: false },
  { key: "platform", label: "Platform", required: false },
  { key: "followers", label: "Followers", required: false },
  { key: "pic", label: "PIC", required: false },
  { key: "status", label: "Status", required: false },
  { key: "reference", label: "Reference", required: false },
  { key: "shippingAddress", label: "Shipping Address", required: false },
  { key: "website", label: "Website", required: false },
  { key: "notes", label: "Notes", required: false }
];

function normalizeHeader(value: string) {
  return String(value).trim();
}

function cellText(value: any) {
  if (value === null || value === undefined) return "";
  if (value instanceof Date && !Number.isNaN(value.getTime())) {
    return value.toISOString().slice(0, 10);
  }
  return String(value).trim();
}

function extractEmail(value: string) {
  const match = value.match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i);
  return match ? match[0] : value.trim();
}

function isValidEmail(value: string) {
  return /^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$/i.test(value);
}

function templateHeaders() {
  return importTemplateColumns.map(column => column.label);
}

function sheetHeaders(sheet: any) {
  return (sheet.rows[0] || [])
    .slice(0, importTemplateColumns.length)
    .map((value: any) => normalizeHeader(value));
}

function validateSheetTemplate(sheet: any) {
  const expected = templateHeaders();
  const actual = sheetHeaders(sheet);
  const extraValues = (sheet.rows[0] || [])
    .slice(importTemplateColumns.length)
    .filter((value: any) => String(value || "").trim());
  const errors: string[] = [];
  if (actual.length !== expected.length) {
    errors.push(`表头列数必须为 ${expected.length} 列`);
  }
  expected.forEach((header, index) => {
    if (actual[index] !== header) {
      errors.push(`第 ${index + 1} 列必须是 ${header}`);
    }
  });
  if (extraValues.length > 0) {
    errors.push("存在模板外字段");
  }
  return errors;
}

function validateSelectedSheets() {
  const selected = new Set(selectedSheets.value);
  return workbookSheets.value
    .filter(sheet => selected.has(sheet.name))
    .map(sheet => ({
      sheet: sheet.name,
      errors: validateSheetTemplate(sheet)
    }))
    .filter(item => item.errors.length > 0);
}

function parseSelectedSheets() {
  const rows: any[] = [];
  const selected = new Set(selectedSheets.value);
  workbookSheets.value
    .filter(sheet => selected.has(sheet.name))
    .filter(sheet => validateSheetTemplate(sheet).length === 0)
    .forEach(sheet => {
      sheet.rows.slice(1).forEach((rawRow: any[], rowIndex) => {
        const normalized: any = {
          sourceSheet: sheet.name,
          rowNo: rowIndex + 2,
          name: "",
          email: "",
          mediaOutlet: "",
          category: "",
          tier: "",
          title: "",
          base: "",
          platform: "",
          pic: "",
          status: "",
          reference: "",
          shippingAddress: "",
          website: "",
          notes: "",
          duplicate: false,
          errors: []
        };
        importTemplateColumns.forEach((column, index) => {
          const value = cellText(rawRow[index]);
          normalized[column.key] =
            column.key === "email" ? extractEmail(value) : value;
        });
        if (
          importTemplateColumns.every(
            column => !String(normalized[column.key] || "").trim()
          )
        ) {
          return;
        }
        if (!normalized.name) normalized.errors.push("Name 不能为空");
        if (!normalized.email) {
          normalized.errors.push("Email 不能为空");
        } else if (!isValidEmail(normalized.email)) {
          normalized.errors.push("Email 格式不正确");
        }
        rows.push(normalized);
      });
    });

  const seen = new Set<string>();
  rows.forEach(row => {
    const key = [row.name.toLowerCase(), row.email.toLowerCase()].join("|");
    if (row.name && row.email && seen.has(key)) row.duplicate = true;
    if (row.name && row.email) seen.add(key);
  });
  return rows;
}

async function handleMasterListUpload(file: any) {
  const rawFile = file.raw;
  if (!rawFile) return;
  importFileName.value = rawFile.name;
  const buffer = await rawFile.arrayBuffer();
  const workbook = XLSX.read(buffer, { type: "array", cellDates: true });
  workbookSheets.value = workbook.SheetNames.map(name => {
    const worksheet = workbook.Sheets[name];
    return {
      name,
      rows: XLSX.utils.sheet_to_json<any[]>(worksheet, {
        header: 1,
        defval: "",
        blankrows: false
      })
    };
  }).filter(sheet => sheet.rows.length > 0);
  selectedSheets.value = workbookSheets.value.map(sheet => sheet.name);
}

function downloadImportTemplate() {
  const worksheet = XLSX.utils.aoa_to_sheet([templateHeaders()]);
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, worksheet, "Media List Template");
  XLSX.writeFile(workbook, "KOL_Media_List_Template.xlsx");
}

function openImportDialog() {
  importDialog.value = true;
}

function handleSheetChange(values: string[]) {
  if (values.includes("__all__")) {
    selectedSheets.value = workbookSheets.value.map(sheet => sheet.name);
  }
}

async function submitImportResources() {
  if (invalidSheets.value.length > 0) {
    ElMessage.warning("存在不符合标准模板的 Sheet，请修正后重新上传");
    return;
  }
  if (invalidImportRows.value.length > 0) {
    ElMessage.warning("存在必填内容错误，请修正后重新上传");
    return;
  }
  if (validImportRows.value.length === 0) {
    ElMessage.warning("没有可导入的数据");
    return;
  }
  importLoading.value = true;
  const res = await importResources({ rows: validImportRows.value });
  importLoading.value = false;
  if (res.code === 0) {
    ElMessage.success(
      `导入 ${res.data.imported} 条，新增 ${res.data.created} 条，更新 ${res.data.updated} 条`
    );
    importDialog.value = false;
    workbookSheets.value = [];
    selectedSheets.value = [];
    loadData();
  }
}

function importRowClassName({ row }: any) {
  if (row.errors.length > 0) return "import-error-row";
  if (row.duplicate) return "import-duplicate-row";
  return "";
}

onMounted(() => {
  loadTags();
  loadData();
  loadSyncStatus(false);
});

onUnmounted(() => {
  stopSyncPolling();
});
</script>

<template>
  <div class="business-page">
    <section class="page-hero">
      <div>
        <span>Resource Library</span>
        <h1>全球资源库</h1>
        <p>
          集中维护媒体、KOL、创作者与代理商主档，沉淀评分、风险、平台数据和导入来源。
        </p>
      </div>
      <div class="hero-actions">
        <div class="sync-action">
          <el-button
            :loading="syncingAll || (showSyncCard && syncRunning)"
            type="success"
            @click="syncAll"
          >
            <IconifyIconOnline icon="ri:cloud-line" class="mr-1" />
            立即同步KOL数据
          </el-button>
          <span>上一次同步：{{ formatDateTime(syncStatus.lastResourceSyncAt) }}</span>
        </div>
        <el-button type="primary" @click="openCreate">
          <IconifyIconOnline icon="ri:add-line" class="mr-1" />
          新增资源
        </el-button>
      </div>
    </section>

    <el-card v-if="showSyncCard && latestSyncJob" shadow="never" class="sync-card">
      <div class="sync-card-main">
        <div>
          <strong>平台数据同步</strong>
          <span>
            上次同步：{{ formatDateTime(syncStatus.lastResourceSyncAt) }}
          </span>
        </div>
        <el-tag :type="syncRunning ? 'warning' : 'success'" effect="plain">
          {{ latestSyncJob.status }}
        </el-tag>
      </div>
      <el-progress
        :percentage="syncProgress"
        :status="latestSyncJob.status === '失败' ? 'exception' : undefined"
      />
      <div class="sync-meta">
        <span>成功 {{ latestSyncJob.successCount || 0 }}</span>
        <span>失败 {{ latestSyncJob.failedCount || 0 }}</span>
        <span>跳过 {{ latestSyncJob.skippedCount || 0 }}</span>
        <span v-if="latestSyncJob.currentResourceName">
          当前：{{ latestSyncJob.currentResourceName }}
        </span>
        <span v-if="latestSyncJob.message">{{ latestSyncJob.message }}</span>
      </div>
    </el-card>

    <el-card shadow="never" class="filter-card">
      <el-form :model="search" inline>
        <el-form-item label="名称">
          <el-input v-model="search.name" clearable placeholder="账号/媒体名" />
        </el-form-item>
        <el-form-item label="国家">
          <el-input
            v-model="search.country"
            clearable
            placeholder="国家/地区"
          />
        </el-form-item>
        <el-form-item label="平台">
          <el-select
            v-model="search.platform"
            clearable
            filterable
            placeholder="全部"
            class="filter-select-wide"
          >
            <el-option
              v-for="platform in platformOptions"
              :key="platform"
              :label="platform"
              :value="platform"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="等级">
          <el-select
            v-model="search.level"
            clearable
            placeholder="全部"
            class="filter-select-wide"
          >
            <el-option
              v-for="item in ['S', 'A', 'B', 'C', 'D']"
              :key="item"
              :label="item"
              :value="item"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="searchData">
            <IconifyIconOnline icon="ri:search-line" class="mr-1" />
            查询
          </el-button>
          <el-button @click="openImportDialog">
            <IconifyIconOnline icon="ri:upload-cloud-2-line" class="mr-1" />
            上传主名单
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="mt-3 table-card" shadow="never">
      <template #header>
        <div class="table-card-header">
          <div>
            <strong>资源清单</strong>
            <span>共 {{ total }} 条资源，可按市场、平台、等级继续筛选</span>
          </div>
        </div>
      </template>
      <el-table v-loading="loading" :data="list" stripe class="business-table">
        <el-table-column label="头像" width="72" fixed>
          <template #default="{ row }">
            <span class="avatar-box">
              <span class="avatar-letter">{{ avatarText(row) }}</span>
              <img
                v-if="row.avatarUrl && !avatarLoadFailed[avatarKey(row)]"
                v-show="avatarLoaded[avatarKey(row)]"
                :src="row.avatarUrl"
                :alt="row.name"
                @load="markAvatarLoaded(row)"
                @error="markAvatarFailed(row)"
              />
            </span>
          </template>
        </el-table-column>
        <el-table-column label="姓名" min-width="190" fixed>
          <template #default="{ row }">
            <div class="primary-cell">
              <el-button link type="primary" @click="openProfile(row)">
                {{ displayText(row.name) }}
              </el-button>
              <span>{{ displayText(row.owner || row.status) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="媒体 / 账号" min-width="230">
          <template #default="{ row }">
            <div class="stack-cell">
              <strong>{{ mediaAccountTitle(row) }}</strong>
              <span>{{ mediaAccountSub(row) || "-" }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="150">
          <template #default="{ row }">
            <div class="stack-cell">
              <strong>{{ displayText(row.resourceType) }}</strong>
              <span>{{ displayText(row.category || row.industry) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="所在地" width="150">
          <template #default="{ row }">
            <div class="stack-cell">
              <strong>{{ locationText(row) }}</strong>
              <span>{{ displayText(row.language) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="平台" width="145">
          <template #default="{ row }">
            <div class="stack-cell">
              <strong>{{ displayText(row.platform) }}</strong>
              <span>{{ row.lastSyncStatus ? `同步 ${row.lastSyncStatus}` : "未同步" }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="粉丝 / 访问" width="145">
          <template #default="{ row }">
            <div class="metric-cell">
              <strong>{{ formatCount(row.followers) }}</strong>
              <span>粉丝 / 订阅</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="平均播放 / 阅读" width="160">
          <template #default="{ row }">
            <div class="metric-cell">
              <strong>{{ formatCount(row.avgViews) }}</strong>
              <span>{{ Number(row.videoCount || 0) > 0 ? `近 ${row.videoCount} 条均值` : "均值" }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="互动" width="130">
          <template #default="{ row }">
            <div class="metric-cell">
              <strong>{{ percentText(row.engagementRate) }}</strong>
              <span>平均互动</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="过往合作" width="155">
          <template #default="{ row }">
            <div class="metric-cell">
              <strong>{{ cooperationStats(row).count }} 次</strong>
              <span>
                总触达 {{ formatCount(cooperationStats(row).totalReach) }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="合作内容表现" width="170">
          <template #default="{ row }">
            <div class="metric-cell">
              <strong>{{ cooperationEngagementText(row) }}</strong>
              <span>{{ performanceDeltaText(row) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="评分" width="115">
          <template #default="{ row }">
            <el-tag class="score-pill" effect="plain" type="success">
              {{ displayText(row.level, "B") }} · {{ row.score || 0 }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="标签" min-width="170">
          <template #default="{ row }">
            <div v-if="row.tagNames?.length" class="tag-list">
              <el-tag
                v-for="tag in row.tags || []"
                :key="tag.id"
                size="small"
                effect="plain"
                :style="{ borderColor: tag.color, color: tag.color }"
              >
                {{ tag.name }}
              </el-tag>
            </div>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="上次同步" width="170">
          <template #default="{ row }">
            <div class="stack-cell">
              <span>{{ formatDateTime(row.lastSyncAt) }}</span>
              <span>{{ displayText(row.lastSyncError, "") }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openProfile(row)"
              >档案</el-button
            >
            <el-button link type="primary" @click="openPosts(row)"
              >作品</el-button
            >
            <el-button
              link
              type="primary"
              :loading="!!syncingResourceIds[Number(row.id || 0)]"
              :disabled="syncRunning || !isSyncablePlatform(row.platform)"
              @click="syncOne(row)"
              >同步</el-button
            >
            <el-button link type="primary" @click="openEdit(row)"
              >编辑</el-button
            >
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="table-footer">
        <span>共 {{ total }} 条资源</span>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          layout="sizes, prev, pager, next, jumper"
          background
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑资源' : '新增资源'"
      width="920px"
    >
      <el-form :model="form" label-width="110px">
        <el-row :gutter="12">
          <el-col :span="12"
            ><el-form-item label="资源名称"
              ><el-input v-model="form.name" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="资源类型"
              ><el-input v-model="form.resourceType" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="Media Outlet"
              ><el-input v-model="form.mediaOutlet" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="Tier"
              ><el-input v-model="form.tier" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="国家/地区"
              ><el-input v-model="form.country" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="区域"
              ><el-input v-model="form.region" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="城市"
              ><el-input v-model="form.city" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="语言"
              ><el-input v-model="form.language" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="平台"
              ><el-select
                v-model="form.platform"
                allow-create
                filterable
                default-first-option
                placeholder="选择或输入平台"
                class="w-full!"
                @change="handlePlatformChange"
              >
                <el-option
                  v-for="platform in platformOptions"
                  :key="platform"
                  :label="platform"
                  :value="platform"
                >
                  <div class="platform-option">
                    <span>{{ platform }}</span>
                    <el-button
                      link
                      type="danger"
                      @mousedown.stop
                      @click="removePlatformOption(platform, $event)"
                    >
                      删除
                    </el-button>
                  </div>
                </el-option>
              </el-select></el-form-item
            ></el-col
          >
          <el-col :span="12"
            ><el-form-item label="行业垂类"
              ><el-input v-model="form.industry" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="Category"
              ><el-input v-model="form.category" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="Title"
              ><el-input v-model="form.title" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="Email"
              ><el-input v-model="form.contact" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="PIC/负责人"
              ><el-input v-model="form.owner" /></el-form-item
          ></el-col>
          <el-col :span="24">
            <el-form-item label="资源标签">
              <el-select
                v-model="form.tagNames"
                multiple
                allow-create
                filterable
                default-first-option
                collapse-tags
                collapse-tags-tooltip
                placeholder="选择或输入自定义标签"
                class="w-full!"
              >
                <el-option
                  v-for="tag in tagOptions"
                  :key="tag.id"
                  :label="tag.name"
                  :value="tag.name"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12"
            ><el-form-item label="粉丝数"
              ><el-input-number
                v-model="form.followers"
                :min="0"
                class="w-full!" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="平均播放"
              ><el-input-number
                v-model="form.avgViews"
                :min="0"
                class="w-full!" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="互动率"
              ><el-input-number
                v-model="form.engagementRate"
                :min="0"
                :step="0.01"
                class="w-full!" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="评分"
              ><el-input-number
                v-model="form.score"
                :min="0"
                :max="100"
                class="w-full!" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="状态"
              ><el-input v-model="form.status" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="风险等级"
              ><el-input v-model="form.riskLevel" /></el-form-item
          ></el-col>
          <el-col :span="24"
            ><el-form-item label="平台链接"
              ><el-input v-model="form.platformUrl" /></el-form-item
          ></el-col>
          <el-col :span="24"
            ><el-form-item label="Website"
              ><el-input v-model="form.website" /></el-form-item
          ></el-col>
          <el-col :span="24"
            ><el-form-item label="Reference"
              ><el-input v-model="form.referenceSource" /></el-form-item
          ></el-col>
          <el-col :span="24"
            ><el-form-item label="Shipping"
              ><el-input
                v-model="form.shippingAddress"
                type="textarea"
                :rows="2" /></el-form-item
          ></el-col>
          <el-col :span="24"
            ><el-form-item label="备注"
              ><el-input v-model="form.notes" type="textarea" /></el-form-item
          ></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="profileDialogVisible"
      title="资源合作档案"
      width="1080px"
      top="5vh"
    >
      <section v-if="selectedResource" class="profile-drawer">
        <div class="profile-header">
          <span class="avatar-box profile-avatar">
            <span class="avatar-letter">{{ avatarText(selectedResource) }}</span>
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
          </span>
          <div>
            <h2>{{ displayText(selectedResource.name) }}</h2>
            <p>
              {{ displayText(selectedResource.resourceType) }} ·
              {{ displayText(selectedResource.platform) }} ·
              {{ locationText(selectedResource) }}
            </p>
          </div>
          <el-button type="primary" @click="openPosts(selectedResource)">
            查看平台作品
          </el-button>
        </div>

        <div class="profile-metrics">
          <div>
            <span>平台粉丝 / 访问</span>
            <strong>{{ formatCount(selectedResource.followers) }}</strong>
          </div>
          <div>
            <span>平台均值播放 / 阅读</span>
            <strong>{{ formatCount(selectedResource.avgViews) }}</strong>
          </div>
          <div>
            <span>平台平均互动率</span>
            <strong>{{ percentText(selectedResource.engagementRate) }}</strong>
          </div>
          <div>
            <span>过往合作次数</span>
            <strong>{{ selectedCooperationStats.count }} 次</strong>
          </div>
          <div>
            <span>合作内容总触达</span>
            <strong>{{ formatCount(selectedCooperationStats.totalReach) }}</strong>
          </div>
          <div>
            <span>合作内容互动率</span>
            <strong>
              {{
                ratioPercent(
                  selectedCooperationStats.totalEngagements,
                  selectedCooperationStats.totalReach
                )
              }}
            </strong>
          </div>
          <div>
            <span>付费合作 CPM</span>
            <strong>{{ cooperationCpmText(selectedResource) }}</strong>
          </div>
          <div>
            <span>合作 vs 平台均值</span>
            <strong>{{ performanceDeltaText(selectedResource) }}</strong>
          </div>
        </div>

        <div class="profile-section-title">
          <strong>过往合作追踪</strong>
          <span>合作链接、单条内容表现和项目归属会沉淀在这里</span>
        </div>
        <el-table
          :data="selectedCooperations"
          border
          height="360"
          class="business-table"
        >
          <el-table-column prop="projectName" label="项目" min-width="160" />
          <el-table-column prop="cooperationType" label="合作形式" width="120" />
          <el-table-column prop="releaseDate" label="发布日期" width="120" />
          <el-table-column label="触达" width="120">
            <template #default="{ row }">{{ formatCount(primaryReach(row)) }}</template>
          </el-table-column>
          <el-table-column prop="views" label="播放 / 阅读" width="120" />
          <el-table-column prop="engagementCount" label="转赞藏" width="110" />
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
          <el-table-column label="发布链接" min-width="220">
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
          <el-table-column
            prop="notes"
            label="复盘备注"
            min-width="180"
            show-overflow-tooltip
          />
        </el-table>
      </section>
    </el-dialog>

    <el-dialog
      v-model="importDialog"
      title="上传媒体/KOL主名单"
      width="92%"
      top="5vh"
    >
      <div class="import-toolbar">
        <el-button @click="downloadImportTemplate">下载标准模板</el-button>
        <el-upload
          accept=".xlsx,.xls,.csv"
          :auto-upload="false"
          :show-file-list="false"
          :on-change="handleMasterListUpload"
        >
          <el-button type="primary">选择 Excel 文件</el-button>
        </el-upload>
        <el-select
          v-model="selectedSheets"
          multiple
          collapse-tags
          collapse-tags-tooltip
          placeholder="选择 Sheet"
          class="sheet-select"
          @change="handleSheetChange"
        >
          <el-option label="全部 Sheet" value="__all__" />
          <el-option
            v-for="sheet in workbookSheets"
            :key="sheet.name"
            :label="`${sheet.name}（${sheet.rows.length} 行）`"
            :value="sheet.name"
          />
        </el-select>
      </div>
      <el-alert
        class="mt-3"
        type="warning"
        :closable="false"
        title="上传文件必须使用标准模板，第一行表头需完全一致；Name 和 Email 为必填，去重规则为 Name + Email。"
      />
      <el-alert
        v-if="workbookSheets.length > 0"
        class="mt-3"
        type="info"
        :closable="false"
        :title="`文件：${importFileName}，Sheet ${selectedSheets.length}/${workbookSheets.length}，可导入 ${validImportRows.length} 行，异常 ${invalidImportRows.length} 行，疑似重复 ${duplicateImportRows.length} 行`"
      />
      <el-alert
        v-if="invalidSheets.length > 0"
        class="mt-3"
        type="error"
        :closable="false"
        :title="`模板不符：${invalidSheets.map(item => `${item.sheet}（${item.errors[0]}）`).join('；')}`"
      />
      <el-table
        class="mt-3"
        :data="parsedImportRows.slice(0, 300)"
        border
        height="460"
        :row-class-name="importRowClassName"
      >
        <el-table-column prop="sourceSheet" label="Sheet" width="180" fixed />
        <el-table-column prop="rowNo" label="行号" width="70" />
        <el-table-column prop="name" label="Name" min-width="140" />
        <el-table-column prop="email" label="Email" min-width="180" />
        <el-table-column
          prop="mediaOutlet"
          label="Media Outlet"
          min-width="150"
        />
        <el-table-column prop="category" label="Category" min-width="120" />
        <el-table-column prop="tier" label="Tier" width="90" />
        <el-table-column prop="base" label="Base" width="110" />
        <el-table-column prop="platform" label="Platform" width="110" />
        <el-table-column prop="title" label="Title" min-width="120" />
        <el-table-column prop="followers" label="Followers" width="110" />
        <el-table-column prop="pic" label="PIC" width="100" />
        <el-table-column prop="status" label="Status" width="110" />
        <el-table-column label="状态" min-width="190" fixed="right">
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
          :disabled="!canSubmitImport"
          @click="submitImportResources"
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
    radial-gradient(circle at 90% 18%, rgb(245 158 11 / 16%), transparent 26%),
    linear-gradient(135deg, #fff 0%, #eff6ff 56%, #f0fdfa 100%);
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

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: flex-start;
  justify-content: flex-end;
}

.sync-action {
  display: grid;
  gap: 6px;
  justify-items: start;
}

.sync-action span {
  font-size: 12px;
  color: #64748b;
}

.sync-card {
  margin-bottom: 16px;
  border-radius: 8px;
}

.sync-card-main {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.sync-card-main > div {
  display: grid;
  gap: 4px;
}

.sync-card-main strong {
  color: #0f172a;
}

.sync-card-main span,
.sync-meta {
  font-size: 12px;
  color: #64748b;
}

.sync-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 10px;
}

.avatar-box {
  position: relative;
  display: inline-grid;
  width: 32px;
  height: 32px;
  overflow: hidden;
  vertical-align: middle;
  background: #e2e8f0;
  border-radius: 50%;
  place-items: center;
}

.avatar-letter {
  font-size: 12px;
  font-weight: 700;
  color: #475569;
}

.avatar-box img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.primary-cell,
.stack-cell,
.metric-cell {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.primary-cell :deep(.el-button) {
  justify-content: flex-start;
  height: auto;
  padding: 0;
  font-size: 14px;
  font-weight: 700;
  white-space: normal;
}

.primary-cell span,
.stack-cell span,
.metric-cell span {
  overflow: hidden;
  font-size: 12px;
  line-height: 1.45;
  color: #64748b;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stack-cell strong,
.metric-cell strong {
  overflow: hidden;
  font-size: 14px;
  line-height: 1.4;
  color: #0f172a;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.metric-cell strong {
  font-size: 15px;
  font-weight: 700;
}

.profile-drawer {
  display: grid;
  gap: 16px;
}

.profile-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 14px;
  align-items: center;
  padding: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.profile-avatar {
  width: 52px;
  height: 52px;
}

.profile-header h2 {
  margin: 0;
  font-size: 20px;
  line-height: 1.25;
  color: #0f172a;
  letter-spacing: 0;
}

.profile-header p {
  margin: 6px 0 0;
  font-size: 13px;
  color: #64748b;
}

.profile-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.profile-metrics > div {
  display: grid;
  gap: 6px;
  min-height: 78px;
  padding: 12px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.profile-metrics span,
.profile-section-title span {
  font-size: 12px;
  color: #64748b;
}

.profile-metrics strong {
  font-size: 18px;
  line-height: 1.25;
  color: #0f172a;
}

.profile-section-title {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: baseline;
  justify-content: space-between;
}

.profile-section-title strong {
  color: #0f172a;
}

.score-pill {
  height: 28px;
  padding: 0 12px;
  font-weight: 700;
  border-color: transparent;
  border-radius: 999px;
  background: #e7f8ef;
}

.filter-card,
.table-card {
  border-radius: 8px;
}

.table-card-header {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.table-card-header div {
  display: grid;
  gap: 4px;
}

.table-card-header strong {
  color: #0f172a;
}

.table-card-header span {
  font-size: 12px;
  color: #64748b;
}

.filter-select-wide {
  width: 180px;
}

.mt-3 {
  margin-top: 12px;
}

.table-footer {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-top: 12px;
  color: var(--el-text-color-secondary);
}

.import-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
}

.sheet-select {
  min-width: 360px;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.platform-option {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.ml-2 {
  margin-left: 8px;
}

.mr-1 {
  margin-right: 4px;
}

:deep(.business-table) {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

:deep(.business-table th.el-table__cell) {
  color: #475569;
  background: #f8fafc;
}

:deep(.el-card__header) {
  padding: 16px 18px;
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

  .page-hero,
  .table-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .profile-header,
  .profile-metrics {
    grid-template-columns: 1fr;
  }

  .sheet-select {
    min-width: 100%;
  }
}
</style>
