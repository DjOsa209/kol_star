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
  getCooperationList,
  getResourcePosts,
  getProjectList,
  createCooperation,
  updateCooperation,
  syncCooperation
} from "@/api/business";

defineOptions({ name: "BusinessResources" });

const router = useRouter();
const loading = ref(false);
const syncingAll = ref(false);
const showSyncCard = ref(false);
const syncDialogVisible = ref(false);
const syncScope = ref<"all" | "selected">("all");
const selectedSyncPlatforms = ref<string[]>(["YouTube", "Instagram", "TikTok"]);
const syncPlatformOptions = ["YouTube", "Instagram", "TikTok"];
const syncingResourceIds = reactive<Record<number, boolean>>({});
const syncingCooperationIds = reactive<Record<number, boolean>>({});
const savingCooperation = ref(false);
const dialogVisible = ref(false);
const profileDialogVisible = ref(false);
const cooperationDialogVisible = ref(false);
const cooperationEditorVisible = ref(false);
const importDialog = ref(false);
const importLoading = ref(false);
const editingId = ref<number | null>(null);
const list = ref<any[]>([]);
const allCooperations = ref<any[]>([]);
const projectOptionsForEdit = ref<any[]>([]);
const recentPosts = ref<any[]>([]);
const selectedResource = ref<any | null>(null);
const selectedProject = ref("");
const editingCooperationId = ref<number | null>(null);
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
  resourceType: "",
  country: "",
  platform: "",
  industry: "",
  status: "",
  tier: ""
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
const cooperationForm = reactive({
  projectId: null as number | null,
  resourceId: null as number | null,
  cooperationType: "付费合作",
  quoteAmount: 0,
  currency: "USD",
  status: "邀约中",
  deliverableStatus: "未开始",
  impressions: 0,
  views: 0,
  engagementCount: 0,
  commentsCount: 0,
  clicks: 0,
  conversions: 0,
  roi: 0,
  teamRating: 0,
  releaseDate: "",
  deliverableLinks: "",
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
const syncJobTagType = computed(() => {
  const status = latestSyncJob.value?.status;
  if (status === "运行中" || status === "已中止") return "warning";
  if (status === "失败" || status === "部分失败") return "danger";
  return "success";
});
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
  allCooperations.value
    .filter(
      item =>
        !selectedProject.value || item.projectName === selectedProject.value
    )
    .forEach(item => {
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
  return cooperationsFor(selectedResource.value);
});
const projectOptions = computed(() =>
  Array.from(
    new Set(
      allCooperations.value
        .map(item => displayText(item.projectName, ""))
        .filter(Boolean)
    )
  )
);
const postsByResource = computed(() => {
  const map = new Map<number, any[]>();
  recentPosts.value.forEach(post => {
    const resourceId = Number(post.resourceId || 0);
    if (!resourceId) return;
    if (!map.has(resourceId)) map.set(resourceId, []);
    const items = map.get(resourceId)!;
    if (items.length < 2) items.push(post);
  });
  return map;
});
const selectedCooperationStats = computed(() =>
  selectedResource.value
    ? cooperationStats(selectedResource.value)
    : emptyCooperationStats()
);
const editorCooperations = computed(() => {
  const id = Number(editingId.value || 0);
  return allCooperations.value.filter(item => Number(item.resourceId) === id);
});
const editorCooperationStats = computed(() =>
  summarizeCooperations(editorCooperations.value)
);
const editorCooperationTypes = computed(
  () =>
    Array.from(
      new Set(
        editorCooperations.value
          .map(item => displayText(item.cooperationType, ""))
          .filter(Boolean)
      )
    ).join("、") || "-"
);
const editorCooperationProjects = computed(
  () =>
    Array.from(
      new Set(
        editorCooperations.value
          .map(item => displayText(item.projectName, ""))
          .filter(Boolean)
      )
    ).join("、") || "-"
);

function cooperationsFor(row: any) {
  return allCooperations.value.filter(
    item =>
      Number(item.resourceId) === Number(row?.id) &&
      (!selectedProject.value || item.projectName === selectedProject.value)
  );
}

function postsFor(row: any) {
  return postsByResource.value.get(Number(row.id)) || [];
}

function openUrl(url: string) {
  if (!url) return;
  window.open(url, "_blank", "noopener,noreferrer");
}

function platformIcon(platform: unknown) {
  const value = String(platform || "").toLowerCase();
  if (value.includes("youtube")) return "ri:youtube-line";
  if (value.includes("tiktok")) return "ri:tiktok-line";
  if (value.includes("instagram")) return "ri:instagram-line";
  if (value === "x" || value.includes("twitter")) return "ri:twitter-x-line";
  if (value.includes("facebook")) return "ri:facebook-circle-line";
  if (value.includes("website") || value.includes("newsletter")) {
    return "ri:global-line";
  }
  return "ri:links-line";
}

function marketText(row: any) {
  const parts = [row.region, row.country || row.city]
    .map(item => displayText(item, ""))
    .filter(Boolean);
  return parts.length > 0 ? Array.from(new Set(parts)).join(" - ") : "-";
}

function domainText(row: any) {
  return displayText(row.category || row.industry);
}

function tierText(row: any) {
  return displayText(row.tier || row.level, "待分级");
}

function cooperationTypes(row: any) {
  const values = cooperationsFor(row)
    .map(item => displayText(item.cooperationType, ""))
    .filter(Boolean);
  return Array.from(new Set(values)).join("、") || "-";
}

function cooperationProjects(row: any) {
  const values = cooperationsFor(row)
    .map(item => displayText(item.projectName, ""))
    .filter(Boolean);
  return Array.from(new Set(values)).join("、") || "-";
}

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
  const [resourceRes, cooperationRes, projectRes] = await Promise.all([
    getResourceList(params),
    getCooperationList(),
    getProjectList({ currentPage: 1, pageSize: 200 })
  ]);
  if (resourceRes.code === 0) {
    const { data } = resourceRes;
    list.value = data.list;
    total.value = data.total;
    const postResults = await Promise.all(
      data.list.map((row: any) =>
        getResourcePosts({
          resourceId: row.id,
          currentPage: 1,
          pageSize: 2
        })
      )
    );
    recentPosts.value = postResults.flatMap(result =>
      result.code === 0 ? result.data.list || [] : []
    );
  }
  if (cooperationRes.code === 0) {
    allCooperations.value = cooperationRes.data.list || [];
  }
  if (projectRes.code === 0) {
    projectOptionsForEdit.value = projectRes.data.list || [];
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

function compactCount(value: unknown) {
  const number = Number(value || 0);
  if (!Number.isFinite(number) || number <= 0) return "-";
  if (number >= 100000000) return `${(number / 100000000).toFixed(1)}亿`;
  if (number >= 10000) return `${(number / 10000).toFixed(1)}万`;
  return number.toLocaleString("zh-CN");
}

function avgInteractions(row: any) {
  const rate = numberValue(row.engagementRate);
  const normalizedRate = rate > 1 ? rate / 100 : rate;
  return Math.round(numberValue(row.avgViews) * normalizedRate);
}

function numberValue(value: unknown) {
  const number = Number(value || 0);
  return Number.isFinite(number) ? number : 0;
}

function todayDate() {
  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function dateRank(row: any) {
  const releaseTime = row?.releaseDate
    ? new Date(row.releaseDate).getTime()
    : 0;
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

function summarizeCooperations(rows: any[]) {
  return rows.reduce((stat, item) => {
    stat.count += 1;
    stat.totalReach += primaryReach(item);
    stat.totalViews += numberValue(item.views);
    stat.totalEngagements +=
      numberValue(item.engagementCount) + numberValue(item.commentsCount);
    stat.totalCost += numberValue(item.quoteAmount);
    return stat;
  }, emptyCooperationStats());
}

function cpmText(stat: any) {
  if (stat.totalCost <= 0 || stat.totalReach <= 0) return "-";
  return currencyText((stat.totalCost / stat.totalReach) * 1000);
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
  const delta =
    ((avgCooperationViews - avgPlatformViews) / avgPlatformViews) * 100;
  const prefix = delta >= 0 ? "高于平台均值" : "低于平台均值";
  return `${prefix} ${Math.abs(delta).toFixed(0)}%`;
}

function locationText(row: any) {
  return displayText(row.country || row.region || row.city);
}

function mediaAccountTitle(row: any) {
  return displayText(
    row.mediaOutlet || row.platformHandle || row.title || row.platformUrl
  );
}

function mediaAccountSub(row: any) {
  return [row.tier, row.contact]
    .map(item => displayText(item, ""))
    .filter(Boolean)
    .join(" · ");
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

function resetSearch() {
  Object.assign(search, {
    name: "",
    resourceType: "",
    country: "",
    platform: "",
    industry: "",
    status: "",
    tier: ""
  });
  selectedProject.value = "";
  searchData();
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
  form.followers = numberValue(row.followers);
  form.avgViews = numberValue(row.avgViews);
  form.engagementRate = numberValue(row.engagementRate);
  form.score = numberValue(row.score);
  cooperationEditorVisible.value = false;
  dialogVisible.value = true;
}

function resetCooperationForm() {
  editingCooperationId.value = null;
  Object.assign(cooperationForm, {
    projectId: projectOptionsForEdit.value[0]?.id || null,
    resourceId: editingId.value,
    cooperationType: "付费合作",
    quoteAmount: 0,
    currency: "USD",
    status: "邀约中",
    deliverableStatus: "未开始",
    impressions: 0,
    views: 0,
    engagementCount: 0,
    commentsCount: 0,
    clicks: 0,
    conversions: 0,
    roi: 0,
    teamRating: 0,
    releaseDate: todayDate(),
    deliverableLinks: "",
    notes: ""
  });
}

function openCreateCooperation() {
  if (!editingId.value) {
    ElMessage.warning("请先保存资源，再新增合作记录");
    return;
  }
  resetCooperationForm();
  cooperationEditorVisible.value = true;
}

function openEditCooperation(row: any) {
  resetCooperationForm();
  editingCooperationId.value = Number(row.id);
  Object.assign(cooperationForm, row);
  cooperationForm.projectId = Number(row.projectId || 0) || null;
  cooperationForm.resourceId = Number(row.resourceId || 0) || null;
  cooperationForm.quoteAmount = numberValue(row.quoteAmount);
  cooperationForm.impressions = numberValue(row.impressions);
  cooperationForm.views = numberValue(row.views);
  cooperationForm.engagementCount = numberValue(row.engagementCount);
  cooperationForm.commentsCount = numberValue(row.commentsCount);
  cooperationForm.clicks = numberValue(row.clicks);
  cooperationForm.conversions = numberValue(row.conversions);
  cooperationForm.roi = numberValue(row.roi);
  cooperationForm.teamRating = numberValue(row.teamRating);
  cooperationEditorVisible.value = true;
}

async function submitCooperation() {
  if (savingCooperation.value) return;
  const payload = editingCooperationId.value
    ? { id: editingCooperationId.value, ...cooperationForm, currency: "USD" }
    : { ...cooperationForm, currency: "USD" };
  savingCooperation.value = true;
  try {
    const res = editingCooperationId.value
      ? await updateCooperation(payload)
      : await createCooperation(payload);
    if (res.code === 0) {
      ElMessage.success(
        res.data?.postSync?.synced
          ? `合作记录保存成功，${res.data.postSync.message}`
          : "合作记录保存成功"
      );
      if (res.data?.postSync?.message && !res.data.postSync.synced) {
        ElMessage.warning(res.data.postSync.message);
      }
      cooperationEditorVisible.value = false;
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

function openProfile(row: any) {
  selectedResource.value = row;
  profileDialogVisible.value = true;
}

function editFromProfile() {
  if (!selectedResource.value) return;
  profileDialogVisible.value = false;
  openEdit(selectedResource.value);
}

function openPosts(row: any) {
  router.push({
    path: "/business/resource-posts",
    query: { resourceId: row.id }
  });
}

async function submit() {
  if (cooperationEditorVisible.value) {
    await ElMessageBox.confirm(
      "当前合作记录尚未保存。继续保存资源主档将不会保存这条合作记录。",
      "合作记录未保存",
      {
        type: "warning",
        confirmButtonText: "仍然保存资源",
        cancelButtonText: "返回保存合作"
      }
    );
  }
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
  syncDialogVisible.value = true;
}

async function confirmSyncAll() {
  if (
    syncScope.value === "selected" &&
    selectedSyncPlatforms.value.length === 0
  ) {
    ElMessage.warning("请至少选择一个平台");
    return;
  }
  const platforms =
    syncScope.value === "all" ? [] : selectedSyncPlatforms.value;
  syncDialogVisible.value = false;
  showSyncCard.value = true;
  syncingAll.value = true;
  const res = await syncAllResources({ platforms });
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
          <span
            >上一次同步：{{
              formatDateTime(syncStatus.lastResourceSyncAt)
            }}</span
          >
        </div>
        <el-button type="primary" @click="openCreate">
          <IconifyIconOnline icon="ri:add-line" class="mr-1" />
          新增资源
        </el-button>
      </div>
    </section>

    <el-card
      v-if="showSyncCard && latestSyncJob"
      shadow="never"
      class="sync-card"
    >
      <div class="sync-card-main">
        <div>
          <strong>平台数据同步</strong>
          <span>
            上次同步：{{ formatDateTime(syncStatus.lastResourceSyncAt) }}
          </span>
        </div>
        <el-tag :type="syncJobTagType" effect="plain">
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
        <el-form-item label="资源类型">
          <el-select
            v-model="search.resourceType"
            clearable
            placeholder="全部类型"
            class="filter-select-wide"
          >
            <el-option
              v-for="item in ['KOL', '媒体', 'IP', '其他']"
              :key="item"
              :label="item"
              :value="item"
            />
          </el-select>
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
        <el-form-item label="资源领域">
          <el-select
            v-model="search.industry"
            clearable
            filterable
            placeholder="全部领域"
            class="filter-select-wide"
          >
            <el-option
              v-for="item in [
                '科技',
                '生活方式',
                '商业',
                '综合新闻',
                '游戏',
                '校园',
                '设计'
              ]"
              :key="item"
              :label="item"
              :value="item"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="分级">
          <el-select
            v-model="search.tier"
            clearable
            placeholder="全部"
            class="filter-select-wide"
          >
            <el-option
              v-for="item in ['T0', 'T1', 'T2', 'T3']"
              :key="item"
              :label="item"
              :value="item"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="合作项目">
          <el-select
            v-model="selectedProject"
            clearable
            filterable
            placeholder="全部历史合作"
            class="filter-select-wide"
          >
            <el-option
              v-for="project in projectOptions"
              :key="project"
              :label="project"
              :value="project"
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
          <el-button @click="resetSearch">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="mt-3 table-card" shadow="never">
      <template #header>
        <div class="table-card-header">
          <div>
            <strong>资源清单</strong>
            <span
              >共
              {{ total }} 条资源，点击达人查看完整档案，编辑入口独立维护</span
            >
          </div>
          <el-tag v-if="selectedProject" type="warning" effect="plain">
            当前合作数据：{{ selectedProject }}
          </el-tag>
        </div>
      </template>
      <section v-loading="loading" class="compact-resource-list">
        <div class="compact-list-head" aria-hidden="true">
          <span>序号</span>
          <span>资源身份</span>
          <span>基础表现</span>
          <span>合作数据</span>
          <span>最近平台作品</span>
          <span>操作</span>
        </div>
        <article
          v-for="(row, index) in list"
          :key="row.id"
          class="compact-resource-row"
        >
          <span class="compact-index">
            {{ (currentPage - 1) * pageSize + index + 1 }}
          </span>
          <div class="compact-identity">
            <span class="avatar-box compact-avatar">
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
            <div>
              <button type="button" @click="openProfile(row)">
                {{ displayText(row.name) }}
              </button>
              <span>{{ mediaAccountTitle(row) }}</span>
              <p>
                <el-tag size="small" effect="plain">{{
                  displayText(row.resourceType)
                }}</el-tag>
                <el-tag size="small" type="warning" effect="plain">{{
                  domainText(row)
                }}</el-tag>
              </p>
              <p class="compact-identity-meta">
                <span>{{ marketText(row) }}</span>
                <span>{{ tierText(row) }}</span>
                <span>
                  <IconifyIconOnline :icon="platformIcon(row.platform)" />
                  {{ displayText(row.platform) }}
                </span>
              </p>
            </div>
          </div>

          <dl class="compact-base-metrics">
            <div>
              <dt>粉丝数 / 访问量</dt>
              <dd>{{ compactCount(row.followers) }}</dd>
            </div>
            <div>
              <dt>月均播放量</dt>
              <dd>{{ compactCount(row.avgViews) }}</dd>
            </div>
            <div>
              <dt>月均互动量</dt>
              <dd>{{ compactCount(avgInteractions(row)) }}</dd>
            </div>
          </dl>

          <div class="compact-cooperation">
            <div class="compact-cooperation__meta">
              <span>{{ cooperationTypes(row) }}</span>
              <strong>{{ cooperationProjects(row) }}</strong>
            </div>
            <dl>
              <div>
                <dt>合作费用</dt>
                <dd>{{ currencyText(cooperationStats(row).totalCost) }}</dd>
              </div>
              <div>
                <dt>合作曝光量</dt>
                <dd>{{ compactCount(cooperationStats(row).totalReach) }}</dd>
              </div>
              <div>
                <dt>合作互动量</dt>
                <dd>
                  {{ compactCount(cooperationStats(row).totalEngagements) }}
                </dd>
              </div>
              <div>
                <dt>效果分数</dt>
                <dd>{{ displayText(row.score) }}</dd>
              </div>
              <div>
                <dt>CPM</dt>
                <dd>{{ cooperationCpmText(row) }}</dd>
              </div>
            </dl>
          </div>

          <div class="compact-content">
            <button
              v-for="post in postsFor(row)"
              :key="post.id"
              type="button"
              @click="openUrl(post.postUrl)"
            >
              <img
                v-if="post.coverUrl"
                :src="post.coverUrl"
                :alt="post.title"
              />
              <span v-else
                ><IconifyIconOnline icon="ri:play-circle-line"
              /></span>
              <small>{{ compactCount(post.viewCount) }}</small>
            </button>
            <button
              v-if="!postsFor(row).length"
              type="button"
              class="compact-content-empty"
              @click="openPosts(row)"
            >
              <IconifyIconOnline icon="ri:play-list-2-line" />
              <span>查看作品数据</span>
            </button>
          </div>

          <div class="compact-actions">
            <el-button link type="primary" @click="openProfile(row)"
              >档案</el-button
            >
            <el-button link type="primary" @click="openEdit(row)"
              >编辑</el-button
            >
            <el-dropdown trigger="click">
              <el-button link>
                <IconifyIconOnline icon="ri:more-2-fill" />
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="openPosts(row)"
                    >查看作品</el-dropdown-item
                  >
                  <el-dropdown-item
                    :disabled="syncRunning || !isSyncablePlatform(row.platform)"
                    @click="syncOne(row)"
                    >同步平台数据</el-dropdown-item
                  >
                  <el-dropdown-item divided @click="remove(row)"
                    >删除资源</el-dropdown-item
                  >
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </article>
        <el-empty
          v-if="!loading && list.length === 0"
          description="暂无匹配资源"
        />
      </section>
      <el-table
        v-if="false"
        v-loading="loading"
        :data="list"
        stripe
        class="business-table"
      >
        <el-table-column type="expand" width="52">
          <template #default="{ row }">
            <div class="resource-expand">
              <section class="detail-group">
                <div class="detail-group__heading">
                  <i><IconifyIconOnline icon="ri:user-star-line" /></i>
                  <div>
                    <strong>资源画像</strong>
                    <span>身份、领域、市场与平台信息</span>
                  </div>
                </div>
                <dl class="detail-grid">
                  <div>
                    <dt>资源类型</dt>
                    <dd>{{ displayText(row.resourceType) }}</dd>
                  </div>
                  <div>
                    <dt>资源领域</dt>
                    <dd>{{ domainText(row) }}</dd>
                  </div>
                  <div>
                    <dt>所属市场</dt>
                    <dd>{{ marketText(row) }}</dd>
                  </div>
                  <div>
                    <dt>分级</dt>
                    <dd>{{ tierText(row) }}</dd>
                  </div>
                  <div>
                    <dt>平台</dt>
                    <dd>{{ displayText(row.platform) }}</dd>
                  </div>
                  <div>
                    <dt>语言</dt>
                    <dd>{{ displayText(row.language) }}</dd>
                  </div>
                  <div>
                    <dt>媒体 / 账号</dt>
                    <dd>{{ mediaAccountTitle(row) }}</dd>
                  </div>
                  <div>
                    <dt>状态</dt>
                    <dd>{{ displayText(row.status) }}</dd>
                  </div>
                </dl>
              </section>

              <section class="detail-group">
                <div class="detail-group__heading">
                  <i><IconifyIconOnline icon="ri:line-chart-line" /></i>
                  <div>
                    <strong>规模与内容表现</strong>
                    <span>平台规模及日常内容表现</span>
                  </div>
                </div>
                <dl class="detail-grid detail-grid--metrics">
                  <div>
                    <dt>粉丝数 / 访问量</dt>
                    <dd>{{ formatCount(row.followers) }}</dd>
                  </div>
                  <div>
                    <dt>月均播放量</dt>
                    <dd>{{ formatCount(row.avgViews) }}</dd>
                  </div>
                  <div>
                    <dt>平均互动率</dt>
                    <dd>{{ percentText(row.engagementRate) }}</dd>
                  </div>
                  <div>
                    <dt>累计平台播放</dt>
                    <dd>{{ formatCount(row.totalViews) }}</dd>
                  </div>
                  <div>
                    <dt>已同步内容数</dt>
                    <dd>{{ formatCount(row.videoCount) }}</dd>
                  </div>
                  <div>
                    <dt>合作效果分数</dt>
                    <dd>TBC</dd>
                  </div>
                </dl>
              </section>

              <section class="detail-group">
                <div class="detail-group__heading">
                  <i><IconifyIconOnline icon="ri:briefcase-4-line" /></i>
                  <div>
                    <strong>合作与效果</strong>
                    <span>默认汇总全部历史合作，可从项目名称识别单独项目</span>
                  </div>
                </div>
                <dl class="detail-grid detail-grid--metrics">
                  <div>
                    <dt>合作类型</dt>
                    <dd>{{ cooperationTypes(row) }}</dd>
                  </div>
                  <div>
                    <dt>合作项目</dt>
                    <dd>{{ cooperationProjects(row) }}</dd>
                  </div>
                  <div>
                    <dt>合作费用</dt>
                    <dd>{{ currencyText(cooperationStats(row).totalCost) }}</dd>
                  </div>
                  <div>
                    <dt>合作曝光量</dt>
                    <dd>{{ formatCount(cooperationStats(row).totalReach) }}</dd>
                  </div>
                  <div>
                    <dt>合作互动量</dt>
                    <dd>
                      {{ formatCount(cooperationStats(row).totalEngagements) }}
                    </dd>
                  </div>
                  <div>
                    <dt>CPM</dt>
                    <dd>{{ cooperationCpmText(row) }}</dd>
                  </div>
                </dl>
                <el-table
                  :data="cooperationsFor(row)"
                  empty-text="暂无历史合作内容"
                  class="cooperation-table"
                >
                  <el-table-column
                    prop="projectName"
                    label="合作项目"
                    min-width="150"
                  />
                  <el-table-column
                    prop="cooperationType"
                    label="合作类型"
                    width="120"
                  />
                  <el-table-column label="合作费用" width="130">
                    <template #default="{ row: cooperation }">
                      {{
                        currencyText(
                          cooperation.quoteAmount,
                          cooperation.currency
                        )
                      }}
                    </template>
                  </el-table-column>
                  <el-table-column label="曝光量" width="120">
                    <template #default="{ row: cooperation }">
                      {{ formatCount(primaryReach(cooperation)) }}
                    </template>
                  </el-table-column>
                  <el-table-column label="互动量" width="120">
                    <template #default="{ row: cooperation }">
                      {{
                        formatCount(
                          numberValue(cooperation.engagementCount) +
                            numberValue(cooperation.commentsCount)
                        )
                      }}
                    </template>
                  </el-table-column>
                  <el-table-column label="合作内容" min-width="220">
                    <template #default="{ row: cooperation }">
                      <el-link
                        v-if="cooperation.deliverableLinks"
                        type="primary"
                        :href="cooperation.deliverableLinks"
                        target="_blank"
                      >
                        查看内容链接
                      </el-link>
                      <span v-else>-</span>
                    </template>
                  </el-table-column>
                </el-table>
              </section>

              <section class="detail-group">
                <div class="detail-group__heading">
                  <i><IconifyIconOnline icon="ri:contacts-line" /></i>
                  <div>
                    <strong>联系与管理</strong>
                    <span>对接、来源与内部备注</span>
                  </div>
                </div>
                <dl class="detail-grid">
                  <div>
                    <dt>联系方式</dt>
                    <dd>{{ displayText(row.contact) }}</dd>
                  </div>
                  <div>
                    <dt>对接人 / 供应商</dt>
                    <dd>{{ displayText(row.owner) }}</dd>
                  </div>
                  <div>
                    <dt>平台链接</dt>
                    <dd>{{ displayText(row.platformUrl) }}</dd>
                  </div>
                  <div>
                    <dt>Website</dt>
                    <dd>{{ displayText(row.website) }}</dd>
                  </div>
                  <div>
                    <dt>数据来源</dt>
                    <dd>{{ displayText(row.referenceSource) }}</dd>
                  </div>
                  <div class="detail-grid__wide">
                    <dt>备注</dt>
                    <dd>{{ displayText(row.notes) }}</dd>
                  </div>
                </dl>
              </section>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="序号" width="72" fixed>
          <template #default="{ $index }">
            {{ (currentPage - 1) * pageSize + $index + 1 }}
          </template>
        </el-table-column>
        <el-table-column label="名称" min-width="220" fixed>
          <template #default="{ row }">
            <div class="resource-name-cell">
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
              <div class="primary-cell">
                <el-button link type="primary" @click="openProfile(row)">
                  {{ displayText(row.name) }}
                </el-button>
                <span>{{ mediaAccountTitle(row) }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="资源类型 / 领域" min-width="180">
          <template #default="{ row }">
            <div class="stack-cell">
              <strong>{{ displayText(row.resourceType) }}</strong>
              <span>{{ domainText(row) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="所属市场" min-width="160">
          <template #default="{ row }">
            <div class="stack-cell">
              <strong>{{ marketText(row) }}</strong>
              <span>{{ displayText(row.language) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="分级 / 平台" width="150">
          <template #default="{ row }">
            <div class="stack-cell">
              <strong>{{ tierText(row) }}</strong>
              <span>{{ displayText(row.platform) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="粉丝 / 访问量" width="140">
          <template #default="{ row }">
            <div class="metric-cell">
              <strong>{{ formatCount(row.followers) }}</strong>
              <span>{{ displayText(row.platform) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="月均播放 / 互动" width="160">
          <template #default="{ row }">
            <div class="metric-cell">
              <strong>{{ formatCount(row.avgViews) }}</strong>
              <span>互动率 {{ percentText(row.engagementRate) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="合作表现" width="170">
          <template #default="{ row }">
            <div class="metric-cell">
              <strong>{{
                formatCount(cooperationStats(row).totalReach)
              }}</strong>
              <span
                >{{ cooperationStats(row).count }} 次合作 · CPM
                {{ cooperationCpmText(row) }}</span
              >
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
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
      v-model="syncDialogVisible"
      title="立即同步KOL数据"
      width="480px"
    >
      <div class="sync-platform-dialog">
        <p>选择本次需要同步的平台。抓取控制中已停用的平台仍会跳过。</p>
        <el-radio-group v-model="syncScope">
          <el-radio value="all">全部平台</el-radio>
          <el-radio value="selected">指定平台</el-radio>
        </el-radio-group>
        <el-checkbox-group
          v-if="syncScope === 'selected'"
          v-model="selectedSyncPlatforms"
          class="sync-platform-checkboxes"
        >
          <el-checkbox
            v-for="platform in syncPlatformOptions"
            :key="platform"
            :value="platform"
          >
            {{ platform }}
          </el-checkbox>
        </el-checkbox-group>
      </div>
      <template #footer>
        <el-button @click="syncDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="syncingAll" @click="confirmSyncAll">
          开始同步
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-if="false"
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
              ><el-select v-model="form.resourceType" class="w-full!">
                <el-option
                  v-for="item in ['KOL', '媒体', 'IP', '其他']"
                  :key="item"
                  :label="item"
                  :value="item" /></el-select></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="Media Outlet"
              ><el-input v-model="form.mediaOutlet" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="Tier"
              ><el-select
                v-model="form.tier"
                clearable
                placeholder="选择资源分级"
                class="w-full!"
              >
                <el-option
                  v-for="item in ['T0', 'T1', 'T2', 'T3']"
                  :key="item"
                  :label="item"
                  :value="item" /></el-select></el-form-item
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
              ><el-select
                v-model="form.category"
                allow-create
                filterable
                default-first-option
                placeholder="选择或输入资源领域"
                class="w-full!"
              >
                <el-option
                  v-for="item in [
                    '科技',
                    '生活方式',
                    '商业',
                    '综合新闻',
                    '游戏',
                    '校园',
                    '设计'
                  ]"
                  :key="item"
                  :label="item"
                  :value="item" /></el-select></el-form-item
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
      v-model="dialogVisible"
      :show-close="false"
      width="min(1380px, 96vw)"
      top="2vh"
      class="resource-editor-dialog"
    >
      <template #header>
        <div class="editor-header">
          <div>
            <span>{{ editingId ? `资源 #${editingId}` : "新资源" }}</span>
            <h2>{{ editingId ? "完整资源档案" : "新增完整资源档案" }}</h2>
            <p>
              资源主档与历史合作记录分区维护，确保全部业务字段可见、可追溯。
            </p>
          </div>
          <div class="editor-header__actions">
            <el-button @click="dialogVisible = false">关闭</el-button>
          </div>
        </div>
      </template>

      <el-form :model="form" label-position="top" class="editor-form">
        <section class="editor-section">
          <div class="editor-section__title">
            <i><IconifyIconOnline icon="ri:user-star-line" /></i>
            <div>
              <strong>基础身份</strong>
              <span
                >序号由系统生成；名称、类型、领域和分级用于资源识别与筛选</span
              >
            </div>
          </div>
          <div class="editor-form-grid">
            <el-form-item label="名称">
              <el-input v-model="form.name" placeholder="资源名称" />
            </el-form-item>
            <el-form-item label="资源类型">
              <el-select v-model="form.resourceType" class="w-full!">
                <el-option
                  v-for="item in ['KOL', '媒体', 'IP', '其他']"
                  :key="item"
                  :label="item"
                  :value="item"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="资源领域">
              <el-select
                v-model="form.category"
                allow-create
                filterable
                class="w-full!"
                placeholder="选择或输入资源领域"
              >
                <el-option
                  v-for="item in [
                    '科技',
                    '生活方式',
                    '商业',
                    '综合新闻',
                    '游戏',
                    '校园',
                    '设计'
                  ]"
                  :key="item"
                  :label="item"
                  :value="item"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="分级">
              <el-select
                v-model="form.tier"
                clearable
                class="w-full!"
                placeholder="T0 / T1 / T2 / T3"
              >
                <el-option
                  v-for="item in ['T0', 'T1', 'T2', 'T3']"
                  :key="item"
                  :label="item"
                  :value="item"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="媒体 / 账号名称">
              <el-input v-model="form.mediaOutlet" />
            </el-form-item>
            <el-form-item label="账号 Title">
              <el-input v-model="form.title" />
            </el-form-item>
            <el-form-item label="资源标签" class="editor-form-grid__wide">
              <el-select
                v-model="form.tagNames"
                multiple
                allow-create
                filterable
                collapse-tags
                class="w-full!"
                placeholder="选择或输入标签"
              >
                <el-option
                  v-for="tag in tagOptions"
                  :key="tag.id"
                  :label="tag.name"
                  :value="tag.name"
                />
              </el-select>
            </el-form-item>
          </div>
        </section>

        <section class="editor-section">
          <div class="editor-section__title">
            <i><IconifyIconOnline icon="ri:global-line" /></i>
            <div>
              <strong>市场与平台</strong>
              <span
                >所属市场按“区域 - 具体市场”组合展示，例如：中东 - 沙特</span
              >
            </div>
          </div>
          <div class="editor-form-grid">
            <el-form-item label="区域">
              <el-input v-model="form.region" placeholder="例如：中东" />
            </el-form-item>
            <el-form-item label="具体市场">
              <el-input v-model="form.country" placeholder="例如：沙特" />
            </el-form-item>
            <el-form-item label="城市">
              <el-input v-model="form.city" />
            </el-form-item>
            <el-form-item label="语言">
              <el-input v-model="form.language" />
            </el-form-item>
            <el-form-item label="平台">
              <el-select
                v-model="form.platform"
                allow-create
                filterable
                class="w-full!"
                @change="handlePlatformChange"
              >
                <el-option
                  v-for="platform in [
                    'Website',
                    'X',
                    'YouTube',
                    'TikTok',
                    'Facebook',
                    'Instagram'
                  ]"
                  :key="platform"
                  :label="platform"
                  :value="platform"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="平台主页链接">
              <el-input v-model="form.platformUrl" />
            </el-form-item>
            <el-form-item label="Website" class="editor-form-grid__wide">
              <el-input v-model="form.website" />
            </el-form-item>
          </div>
        </section>

        <section class="editor-section">
          <div class="editor-section__title">
            <i><IconifyIconOnline icon="ri:line-chart-line" /></i>
            <div>
              <strong>规模表现</strong>
              <span
                >网站填写 MUV；KOL
                填写主平台最大粉丝数。月均互动量根据播放量与互动率计算</span
              >
            </div>
          </div>
          <div class="editor-form-grid editor-form-grid--metrics">
            <el-form-item label="粉丝数 / 访问量（MUV）">
              <el-input-number
                v-model="form.followers"
                :min="0"
                class="w-full!"
              />
            </el-form-item>
            <el-form-item label="月均播放量">
              <el-input-number
                v-model="form.avgViews"
                :min="0"
                class="w-full!"
              />
            </el-form-item>
            <el-form-item label="月均互动量">
              <el-input
                :model-value="formatCount(avgInteractions(form))"
                disabled
              />
            </el-form-item>
            <el-form-item label="互动率（计算月均互动量）">
              <el-input-number
                v-model="form.engagementRate"
                :min="0"
                :step="0.01"
                class="w-full!"
              />
            </el-form-item>
            <el-form-item
              label="MUV / 粉丝数据来源"
              class="editor-form-grid__wide"
            >
              <el-input
                v-model="form.referenceSource"
                placeholder="例如：Similarweb、Semrush、平台同步或业务提供"
              />
            </el-form-item>
          </div>
        </section>

        <section class="editor-section editor-section--cooperation">
          <div class="editor-section__title editor-section__title--between">
            <div class="editor-section__title-group">
              <i><IconifyIconOnline icon="ri:briefcase-4-line" /></i>
              <div>
                <strong>合作与效果</strong>
                <span
                  >合作字段来自历史合作记录；默认展示全部合作汇总，可在列表页按项目切换</span
                >
              </div>
            </div>
            <el-button type="primary" plain @click="openCreateCooperation">
              <IconifyIconOnline icon="ri:add-line" class="mr-1" />
              新增合作记录
            </el-button>
          </div>
          <div class="cooperation-field-summary">
            <div>
              <span>合作类型</span><strong>{{ editorCooperationTypes }}</strong>
            </div>
            <div>
              <span>合作费用（USD）</span
              ><strong>{{
                currencyText(editorCooperationStats.totalCost)
              }}</strong>
            </div>
            <div>
              <span>合作项目</span
              ><strong>{{ editorCooperationProjects }}</strong>
            </div>
            <div>
              <span>合作曝光量</span
              ><strong>{{
                formatCount(editorCooperationStats.totalReach)
              }}</strong>
            </div>
            <div>
              <span>合作互动量</span
              ><strong>{{
                formatCount(editorCooperationStats.totalEngagements)
              }}</strong>
            </div>
            <div><span>合作效果分数</span><strong>TBC</strong></div>
            <div>
              <span>CPM（付费合作）</span
              ><strong>{{ cpmText(editorCooperationStats) }}</strong>
            </div>
          </div>
          <el-alert
            type="info"
            :closable="false"
            title="媒体合作曝光需由媒体提供特定文章访问量；KOL 合作曝光取对应平台内容总播放量。互动量汇总 Likes、Comments、Shares、Saves（当前记录按转赞藏与评论回填）。"
          />
          <div
            v-if="cooperationEditorVisible"
            class="inline-cooperation-editor"
          >
            <div class="inline-cooperation-editor__header">
              <div>
                <strong>{{
                  editingCooperationId ? "编辑合作记录" : "新增合作记录"
                }}</strong>
                <span
                  >合作费用统一按 USD 维护，保存后自动更新曝光、互动与 CPM
                  汇总。</span
                >
              </div>
              <el-button link @click="cooperationEditorVisible = false"
                >收起</el-button
              >
            </div>
            <div class="cooperation-form-grid">
              <el-form-item label="合作项目">
                <el-select v-model="cooperationForm.projectId" class="w-full!">
                  <el-option
                    v-for="project in projectOptionsForEdit"
                    :key="project.id"
                    :label="project.name"
                    :value="project.id"
                  />
                </el-select>
              </el-form-item>
              <el-form-item label="合作类型">
                <el-select
                  v-model="cooperationForm.cooperationType"
                  class="w-full!"
                >
                  <el-option label="产品置换" value="产品置换" />
                  <el-option label="付费合作" value="付费合作" />
                </el-select>
              </el-form-item>
              <el-form-item label="合作费用（USD）">
                <el-input-number
                  v-model="cooperationForm.quoteAmount"
                  :min="0"
                  class="w-full!"
                />
              </el-form-item>
              <el-form-item label="合作曝光量">
                <el-input-number
                  v-model="cooperationForm.impressions"
                  :min="0"
                  class="w-full!"
                />
              </el-form-item>
              <el-form-item label="合作播放 / 阅读量">
                <el-input-number
                  v-model="cooperationForm.views"
                  :min="0"
                  class="w-full!"
                />
              </el-form-item>
              <el-form-item label="Likes / Shares / Saves">
                <el-input-number
                  v-model="cooperationForm.engagementCount"
                  :min="0"
                  class="w-full!"
                />
              </el-form-item>
              <el-form-item label="Comments">
                <el-input-number
                  v-model="cooperationForm.commentsCount"
                  :min="0"
                  class="w-full!"
                />
              </el-form-item>
              <el-form-item label="发布日期">
                <el-date-picker
                  v-model="cooperationForm.releaseDate"
                  type="date"
                  value-format="YYYY-MM-DD"
                  class="w-full!"
                />
              </el-form-item>
              <el-form-item
                label="合作内容链接"
                class="cooperation-form-grid__wide"
              >
                <el-input
                  v-model="cooperationForm.deliverableLinks"
                  placeholder="业务端提供的文章、视频或社媒内容链接"
                />
              </el-form-item>
              <el-form-item label="备注" class="cooperation-form-grid__wide">
                <el-input
                  v-model="cooperationForm.notes"
                  type="textarea"
                  :rows="2"
                />
              </el-form-item>
            </div>
            <div class="inline-cooperation-editor__footer">
              <el-button
                :disabled="savingCooperation"
                @click="cooperationEditorVisible = false"
                >取消</el-button
              >
              <el-button
                type="primary"
                :loading="savingCooperation"
                @click="submitCooperation"
                >保存合作记录</el-button
              >
            </div>
          </div>
          <el-table
            :data="editorCooperations"
            empty-text="暂无历史合作记录"
            class="cooperation-editor-table"
          >
            <el-table-column
              prop="projectName"
              label="合作项目"
              min-width="150"
            />
            <el-table-column
              prop="cooperationType"
              label="合作类型"
              width="120"
            />
            <el-table-column label="合作费用（USD）" width="150">
              <template #default="{ row }">{{
                currencyText(row.quoteAmount, "USD")
              }}</template>
            </el-table-column>
            <el-table-column label="合作内容" min-width="180">
              <template #default="{ row }">
                <el-link
                  v-if="row.deliverableLinks"
                  type="primary"
                  :href="row.deliverableLinks"
                  target="_blank"
                  >查看业务链接</el-link
                >
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column label="合作曝光量" width="130">
              <template #default="{ row }">{{
                formatCount(primaryReach(row))
              }}</template>
            </el-table-column>
            <el-table-column label="合作互动量" width="130">
              <template #default="{ row }">{{
                formatCount(
                  numberValue(row.engagementCount) +
                    numberValue(row.commentsCount)
                )
              }}</template>
            </el-table-column>
            <el-table-column label="操作" width="140" fixed="right">
              <template #default="{ row }">
                <el-button
                  link
                  type="primary"
                  :loading="!!syncingCooperationIds[Number(row.id || 0)]"
                  @click="syncCooperationPost(row)"
                  >同步作品</el-button
                >
                <el-button link type="primary" @click="openEditCooperation(row)"
                  >编辑</el-button
                >
              </template>
            </el-table-column>
          </el-table>
        </section>

        <section class="editor-section">
          <div class="editor-section__title">
            <i><IconifyIconOnline icon="ri:contacts-line" /></i>
            <div>
              <strong>联系与管理</strong>
              <span>业务对接、供应商与内部备注</span>
            </div>
          </div>
          <div class="editor-form-grid">
            <el-form-item label="联系方式">
              <el-input v-model="form.contact" />
            </el-form-item>
            <el-form-item label="对接人 / 供应商">
              <el-input v-model="form.owner" />
            </el-form-item>
            <el-form-item label="状态">
              <el-input v-model="form.status" />
            </el-form-item>
            <el-form-item label="备注" class="editor-form-grid__wide">
              <el-input v-model="form.notes" type="textarea" :rows="3" />
            </el-form-item>
          </div>
        </section>
      </el-form>

      <template #footer>
        <div class="editor-footer">
          <span :class="{ 'editor-footer__warning': cooperationEditorVisible }">
            {{
              cooperationEditorVisible
                ? "合作记录尚未保存，请先保存合作记录或确认放弃后再保存资源。"
                : "资源主档保存后，历史合作记录仍会独立留存。"
            }}
          </span>
          <div>
            <el-button @click="dialogVisible = false">取消</el-button>
            <el-button type="primary" @click="submit">保存资源主档</el-button>
          </div>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-if="false"
      v-model="cooperationDialogVisible"
      :title="editingCooperationId ? '编辑合作记录' : '新增合作记录'"
      width="860px"
      append-to-body
    >
      <el-form :model="cooperationForm" label-position="top">
        <div class="cooperation-form-grid">
          <el-form-item label="合作项目">
            <el-select v-model="cooperationForm.projectId" class="w-full!">
              <el-option
                v-for="project in projectOptionsForEdit"
                :key="project.id"
                :label="project.name"
                :value="project.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="合作类型">
            <el-select
              v-model="cooperationForm.cooperationType"
              class="w-full!"
            >
              <el-option label="产品置换" value="产品置换" />
              <el-option label="付费合作" value="付费合作" />
            </el-select>
          </el-form-item>
          <el-form-item label="合作费用（USD）">
            <el-input-number
              v-model="cooperationForm.quoteAmount"
              :min="0"
              class="w-full!"
            />
          </el-form-item>
          <el-form-item label="合作曝光量">
            <el-input-number
              v-model="cooperationForm.impressions"
              :min="0"
              class="w-full!"
            />
          </el-form-item>
          <el-form-item label="合作播放 / 阅读量">
            <el-input-number
              v-model="cooperationForm.views"
              :min="0"
              class="w-full!"
            />
          </el-form-item>
          <el-form-item label="Likes / Shares / Saves">
            <el-input-number
              v-model="cooperationForm.engagementCount"
              :min="0"
              class="w-full!"
            />
          </el-form-item>
          <el-form-item label="Comments">
            <el-input-number
              v-model="cooperationForm.commentsCount"
              :min="0"
              class="w-full!"
            />
          </el-form-item>
          <el-form-item label="发布日期">
            <el-date-picker
              v-model="cooperationForm.releaseDate"
              type="date"
              value-format="YYYY-MM-DD"
              class="w-full!"
            />
          </el-form-item>
          <el-form-item
            label="合作内容链接"
            class="cooperation-form-grid__wide"
          >
            <el-input
              v-model="cooperationForm.deliverableLinks"
              placeholder="业务端提供的文章、视频或社媒内容链接"
            />
          </el-form-item>
          <el-form-item label="备注" class="cooperation-form-grid__wide">
            <el-input
              v-model="cooperationForm.notes"
              type="textarea"
              :rows="3"
            />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button
          :disabled="savingCooperation"
          @click="cooperationDialogVisible = false"
          >取消</el-button
        >
        <el-button
          type="primary"
          :loading="savingCooperation"
          @click="submitCooperation"
          >保存合作记录</el-button
        >
      </template>
    </el-dialog>

    <el-dialog
      v-model="profileDialogVisible"
      title="完整资源档案"
      width="min(1200px, 94vw)"
      top="5vh"
    >
      <section v-if="selectedResource" class="profile-drawer">
        <div class="profile-header">
          <span class="avatar-box profile-avatar">
            <span class="avatar-letter">{{
              avatarText(selectedResource)
            }}</span>
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
          <div class="profile-header-actions">
            <el-button @click="openPosts(selectedResource)"
              >查看平台作品</el-button
            >
            <el-button type="primary" plain @click="editFromProfile"
              >编辑资源</el-button
            >
          </div>
        </div>

        <dl class="profile-facts">
          <div>
            <dt>资源类型</dt>
            <dd>{{ displayText(selectedResource.resourceType) }}</dd>
          </div>
          <div>
            <dt>资源领域</dt>
            <dd>{{ domainText(selectedResource) }}</dd>
          </div>
          <div>
            <dt>所属市场</dt>
            <dd>{{ marketText(selectedResource) }}</dd>
          </div>
          <div>
            <dt>分级</dt>
            <dd>{{ tierText(selectedResource) }}</dd>
          </div>
          <div>
            <dt>平台</dt>
            <dd>{{ displayText(selectedResource.platform) }}</dd>
          </div>
          <div>
            <dt>合作类型</dt>
            <dd>{{ cooperationTypes(selectedResource) }}</dd>
          </div>
          <div>
            <dt>合作项目</dt>
            <dd>{{ cooperationProjects(selectedResource) }}</dd>
          </div>
          <div>
            <dt>合作费用</dt>
            <dd>{{ currencyText(selectedCooperationStats.totalCost) }}</dd>
          </div>
          <div>
            <dt>合作曝光量</dt>
            <dd>{{ formatCount(selectedCooperationStats.totalReach) }}</dd>
          </div>
          <div>
            <dt>合作互动量</dt>
            <dd>
              {{ formatCount(selectedCooperationStats.totalEngagements) }}
            </dd>
          </div>
          <div>
            <dt>合作效果分数</dt>
            <dd>TBC</dd>
          </div>
          <div>
            <dt>CPM</dt>
            <dd>{{ cooperationCpmText(selectedResource) }}</dd>
          </div>
          <div>
            <dt>联系方式</dt>
            <dd>{{ displayText(selectedResource.contact) }}</dd>
          </div>
          <div>
            <dt>对接人 / 供应商</dt>
            <dd>{{ displayText(selectedResource.owner) }}</dd>
          </div>
          <div class="profile-facts__wide">
            <dt>备注</dt>
            <dd>{{ displayText(selectedResource.notes) }}</dd>
          </div>
        </dl>

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
            <span>月均互动量</span>
            <strong>{{
              formatCount(avgInteractions(selectedResource))
            }}</strong>
          </div>
          <div>
            <span>过往合作次数</span>
            <strong>{{ selectedCooperationStats.count }} 次</strong>
          </div>
          <div>
            <span>合作内容总触达</span>
            <strong>{{
              formatCount(selectedCooperationStats.totalReach)
            }}</strong>
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
          <el-table-column
            prop="cooperationType"
            label="合作形式"
            width="120"
          />
          <el-table-column prop="releaseDate" label="发布日期" width="120" />
          <el-table-column label="触达" width="120">
            <template #default="{ row }">{{
              formatCount(primaryReach(row))
            }}</template>
          </el-table-column>
          <el-table-column prop="views" label="播放 / 阅读" width="120" />
          <el-table-column prop="engagementCount" label="转赞藏" width="110" />
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
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button
                link
                type="primary"
                :loading="!!syncingCooperationIds[Number(row.id || 0)]"
                @click="syncCooperationPost(row)"
                >同步作品</el-button
              >
            </template>
          </el-table-column>
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

.sync-platform-dialog p {
  margin: 0 0 16px;
  color: #64748b;
}

.sync-platform-checkboxes {
  display: flex;
  gap: 8px 20px;
  padding: 14px 16px;
  margin-top: 14px;
  background: #f8fafc;
  border-radius: 8px;
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

.compact-resource-list {
  overflow: hidden;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
}

.compact-list-head,
.compact-resource-row {
  display: grid;
  grid-template-columns:
    32px minmax(190px, 0.95fr) minmax(130px, 0.6fr)
    minmax(270px, 1.35fr) minmax(180px, 0.9fr) 112px;
  gap: 10px;
  align-items: center;
}

.compact-list-head {
  min-height: 38px;
  padding: 0 14px;
  font-size: 11px;
  font-weight: 700;
  color: #64748b;
  background: #f8fafc;
  border-bottom: 1px solid #e5e7eb;
}

.compact-list-head span:first-child,
.compact-list-head span:last-child {
  text-align: center;
}

.compact-resource-row {
  min-height: 126px;
  padding: 11px 14px;
  border-bottom: 1px solid #edf0f3;
  transition: background 0.18s ease;
}

.compact-resource-row:last-child {
  border-bottom: 0;
}

.compact-resource-row:hover {
  background: #fafcff;
}

.compact-index {
  font-size: 12px;
  font-weight: 700;
  color: #94a3b8;
  text-align: center;
}

.compact-identity {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  align-items: center;
  min-width: 0;
}

.compact-avatar {
  width: 58px;
  height: 58px;
  border: 2px solid #fff;
  box-shadow: 0 0 0 1px #e2e8f0;
}

.compact-identity > div {
  display: grid;
  gap: 5px;
  min-width: 0;
}

.compact-identity button {
  padding: 0;
  overflow: hidden;
  font-size: 14px;
  font-weight: 750;
  color: #0f172a;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.compact-identity button:hover {
  color: var(--el-color-primary);
}

.compact-identity > div > span {
  overflow: hidden;
  font-size: 12px;
  color: #64748b;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compact-identity p {
  display: flex;
  gap: 5px;
  align-items: center;
  min-width: 0;
  margin: 0;
  overflow: hidden;
}

.compact-identity p > span:last-child {
  overflow: hidden;
  font-size: 11px;
  color: #94a3b8;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compact-identity .compact-identity-meta {
  gap: 0;
  font-size: 11px;
  color: #64748b;
}

.compact-identity-meta span {
  display: inline-flex;
  gap: 3px;
  align-items: center;
  max-width: 110px;
}

.compact-identity-meta span + span::before {
  margin: 0 6px;
  color: #cbd5e1;
  content: "·";
}

.compact-base-metrics {
  display: grid;
  gap: 9px;
  min-width: 0;
  margin: 0;
}

.compact-base-metrics > div {
  display: flex;
  gap: 10px;
  align-items: baseline;
  justify-content: space-between;
  min-width: 0;
}

.compact-base-metrics dt,
.compact-cooperation dt {
  font-size: 11px;
  color: #94a3b8;
}

.compact-base-metrics dd,
.compact-cooperation dd {
  margin: 0;
  overflow: hidden;
  font-size: 12px;
  font-weight: 700;
  color: #1e293b;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compact-cooperation {
  display: grid;
  gap: 10px;
  min-width: 0;
}

.compact-cooperation__meta {
  display: flex;
  gap: 7px;
  align-items: center;
  min-width: 0;
}

.compact-cooperation__meta span,
.compact-cooperation__meta strong {
  overflow: hidden;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.compact-cooperation__meta span {
  flex: none;
  max-width: 92px;
  padding: 3px 7px;
  color: #b45309;
  background: #fff7ed;
  border-radius: 999px;
}

.compact-cooperation__meta strong {
  color: #475569;
}

.compact-cooperation dl {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px 14px;
  min-width: 0;
  margin: 0;
}

.compact-cooperation dl > div {
  min-width: 0;
}

.compact-cooperation dt {
  margin-bottom: 2px;
}

.compact-content {
  display: flex;
  gap: 7px;
  align-items: center;
  justify-content: center;
  min-width: 0;
}

.compact-content button {
  position: relative;
  flex: 1;
  width: auto;
  min-width: 0;
  height: 64px;
  padding: 0;
  overflow: hidden;
  cursor: pointer;
  background: #f1f5f9;
  border: 0;
  border-radius: 8px;
}

.compact-content img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.compact-content button > span {
  display: grid;
  height: 100%;
  color: #64748b;
  place-items: center;
}

.compact-content small {
  position: absolute;
  right: 5px;
  bottom: 5px;
  padding: 2px 5px;
  font-size: 10px;
  color: #fff;
  background: rgb(15 23 42 / 75%);
  border-radius: 999px;
}

.compact-content .compact-content-empty {
  display: inline-flex;
  flex: none;
  gap: 5px;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 36px;
  padding: 0 10px;
  font-size: 11px;
  color: #64748b;
  background: #f8fafc;
  border: 1px dashed #cbd5e1;
}

.compact-content .compact-content-empty:hover {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-color: var(--el-color-primary-light-5);
}

.compact-actions {
  display: flex;
  flex-wrap: nowrap;
  gap: 8px;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
}

.compact-actions .el-button + .el-button {
  margin-left: 0;
}

.compact-actions .el-dropdown {
  display: inline-flex;
}

.resource-list {
  display: grid;
  gap: 12px;
}

.resource-card {
  overflow: hidden;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgb(15 23 42 / 4%);
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease;
}

.resource-card:hover {
  border-color: #cbd5e1;
  box-shadow: 0 8px 24px rgb(15 23 42 / 8%);
}

.resource-card__main {
  display: grid;
  grid-template-columns:
    minmax(225px, 1.2fr) minmax(165px, 0.82fr) minmax(185px, 0.9fr)
    minmax(215px, 1fr) auto;
  gap: 12px;
  align-items: center;
  min-height: 150px;
  padding: 16px 18px;
}

.resource-identity {
  position: relative;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  align-items: center;
  min-width: 0;
  padding-left: 22px;
}

.resource-index {
  position: absolute;
  top: -4px;
  left: 0;
  font-size: 11px;
  font-weight: 700;
  color: #94a3b8;
}

.resource-avatar {
  width: 64px;
  height: 64px;
  border: 3px solid #fff;
  box-shadow: 0 0 0 1px #e2e8f0;
}

.resource-identity__body {
  display: grid;
  gap: 7px;
  min-width: 0;
}

.resource-identity__body > button {
  padding: 0;
  overflow: hidden;
  font-size: 15px;
  font-weight: 750;
  line-height: 1.25;
  color: #0f172a;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.resource-identity__body > button:hover {
  color: var(--el-color-primary);
}

.resource-handle,
.identity-market {
  overflow: hidden;
  font-size: 12px;
  color: #64748b;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.identity-tags {
  display: flex;
  gap: 5px;
  min-width: 0;
}

.identity-tags :deep(.el-tag) {
  max-width: 92px;
  border-radius: 999px;
}

.identity-market {
  display: flex;
  gap: 4px;
  align-items: center;
}

.resource-metrics,
.cooperation-summary,
.recent-content {
  min-width: 0;
  padding-left: 12px;
  border-left: 1px solid #edf0f3;
}

.section-label {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-bottom: 10px;
  font-size: 12px;
  font-weight: 700;
  color: #64748b;
}

.section-label--between {
  justify-content: space-between;
}

.section-label--between > span {
  display: flex;
  gap: 6px;
  align-items: center;
}

.metric-pairs,
.cooperation-mini-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 14px;
  margin: 0;
}

.metric-pairs dt,
.cooperation-mini-grid dt {
  margin-bottom: 3px;
  font-size: 11px;
  color: #94a3b8;
}

.metric-pairs dd,
.cooperation-mini-grid dd {
  margin: 0;
  overflow: hidden;
  font-size: 14px;
  font-weight: 750;
  color: #1e293b;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cooperation-score {
  display: flex;
  gap: 6px;
  align-items: baseline;
  margin-bottom: 9px;
}

.cooperation-score strong {
  font-size: 22px;
  line-height: 1;
  color: #ea580c;
}

.cooperation-score span {
  font-size: 11px;
  color: #94a3b8;
}

.content-thumbnails {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.content-thumbnails button,
.content-empty {
  position: relative;
  display: block;
  min-width: 0;
  overflow: hidden;
  cursor: pointer;
  background: #f1f5f9;
  border: 0;
  border-radius: 9px;
}

.content-thumbnails button {
  height: 80px;
  padding: 0;
}

.content-thumbnails img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.2s ease;
}

.content-thumbnails button:hover img {
  transform: scale(1.04);
}

.content-thumbnails button > span {
  display: grid;
  height: 100%;
  color: #64748b;
  place-items: center;
}

.content-thumbnails small {
  position: absolute;
  right: 6px;
  bottom: 6px;
  display: flex;
  gap: 3px;
  align-items: center;
  padding: 3px 6px;
  font-size: 10px;
  color: #fff;
  background: rgb(15 23 42 / 78%);
  border-radius: 999px;
}

.content-empty {
  display: grid;
  width: 100%;
  height: 80px;
  gap: 3px;
  font-size: 12px;
  color: #94a3b8;
  place-content: center;
}

.content-empty svg {
  margin: auto;
  font-size: 20px;
}

.resource-actions {
  display: flex;
  gap: 6px;
  align-items: center;
  justify-content: flex-end;
  min-width: 108px;
}

.editor-header,
.editor-footer,
.editor-section__title--between {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.editor-header span {
  font-size: 12px;
  font-weight: 700;
  color: #2563eb;
  text-transform: uppercase;
}

.editor-header h2 {
  margin: 4px 0;
  font-size: 22px;
  color: #0f172a;
}

.editor-header p,
.editor-footer span {
  margin: 0;
  font-size: 12px;
  color: #64748b;
}

.editor-footer__warning {
  font-weight: 650;
  color: #c2410c !important;
}

.editor-header__actions,
.editor-footer > div {
  display: flex;
  gap: 8px;
}

.field-coverage {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  padding: 12px 16px;
  margin-bottom: 14px;
  color: #475569;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
}

.field-coverage strong {
  margin-right: 4px;
  color: #0f172a;
}

.field-coverage span {
  padding: 5px 9px;
  font-size: 12px;
  background: #fff;
  border: 1px solid #dbe4ee;
  border-radius: 999px;
}

.editor-form {
  display: grid;
  flex: 1;
  width: 100%;
  min-width: 0;
  min-height: 0;
  gap: 14px;
  grid-auto-rows: max-content;
  max-height: none;
  padding-right: 4px;
  overflow-x: hidden;
  overflow-y: auto;
}

.editor-section {
  width: 100%;
  min-width: 0;
  max-width: 100%;
  padding: 18px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
}

.editor-section--cooperation {
  background: #fffdf9;
  border-color: #fed7aa;
}

.editor-section__title,
.editor-section__title-group {
  display: flex;
  gap: 10px;
  align-items: center;
}

.editor-section__title {
  padding-bottom: 12px;
  margin-bottom: 14px;
  border-bottom: 1px solid #eef2f7;
}

.editor-section__title i {
  display: grid;
  flex: 0 0 auto;
  width: 36px;
  height: 36px;
  font-size: 18px;
  color: #2563eb;
  background: #eff6ff;
  border-radius: 9px;
  place-items: center;
}

.editor-section__title > div,
.editor-section__title-group > div {
  display: grid;
  gap: 3px;
}

.editor-section__title strong,
.editor-section__title-group strong {
  color: #0f172a;
}

.editor-section__title span,
.editor-section__title-group span {
  font-size: 12px;
  color: #64748b;
}

.editor-form-grid,
.cooperation-form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 14px;
}

.editor-form-grid--metrics {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.editor-form-grid__wide,
.cooperation-form-grid__wide {
  grid-column: span 2;
}

.cooperation-field-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 12px;
}

.cooperation-field-summary > div {
  display: grid;
  gap: 6px;
  min-width: 0;
  padding: 10px;
  background: #fff;
  border: 1px solid #ffedd5;
  border-radius: 9px;
}

.cooperation-field-summary span {
  font-size: 11px;
  color: #94a3b8;
}

.cooperation-field-summary strong {
  overflow: hidden;
  font-size: 14px;
  color: #1e293b;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cooperation-editor-table {
  width: 100% !important;
  max-width: 100%;
  margin-top: 12px;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.inline-cooperation-editor {
  padding: 14px;
  margin-top: 12px;
  background: #fff;
  border: 1px solid #fdba74;
  border-radius: 10px;
  box-shadow: 0 8px 24px rgb(234 88 12 / 8%);
}

.inline-cooperation-editor__header,
.inline-cooperation-editor__footer {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.inline-cooperation-editor__header {
  padding-bottom: 10px;
  margin-bottom: 12px;
  border-bottom: 1px solid #ffedd5;
}

.inline-cooperation-editor__header > div {
  display: grid;
  gap: 3px;
}

.inline-cooperation-editor__header strong {
  color: #9a3412;
}

.inline-cooperation-editor__header span {
  font-size: 12px;
  color: #78716c;
}

.inline-cooperation-editor__footer {
  justify-content: flex-end;
}

:global(.resource-editor-dialog) {
  display: flex !important;
  flex-direction: column;
  max-height: calc(100vh - 40px);
  overflow: hidden !important;
}

:global(.resource-editor-dialog .el-dialog__header),
:global(.resource-editor-dialog .el-dialog__footer) {
  flex: none;
}

:global(.resource-editor-dialog .el-dialog__body) {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding-top: 8px;
  overflow: hidden !important;
}

:global(.resource-editor-dialog .el-form-item) {
  min-width: 0;
  margin-bottom: 14px;
}

:global(.resource-editor-dialog .el-form-item__label) {
  font-size: 12px;
  font-weight: 650;
  color: #475569;
}

:global(.resource-editor-dialog .el-table),
:global(.resource-editor-dialog .el-table__inner-wrapper),
:global(.resource-editor-dialog .el-scrollbar),
:global(.resource-editor-dialog .el-scrollbar__wrap) {
  max-width: 100%;
}

.resource-card > .resource-expand {
  padding: 16px 18px;
  border-top: 1px solid #edf0f3;
}

.resource-card > .resource-expand .detail-group:nth-child(3) {
  grid-column: 1 / -1;
}

.resource-name-cell {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  align-items: center;
}

.resource-expand {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  padding: 14px 18px 18px 52px;
  background: #f8fafc;
}

.detail-group {
  min-width: 0;
  padding: 14px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 9px;
}

.detail-group:nth-child(3),
.detail-group:nth-child(4) {
  grid-column: 1 / -1;
}

.detail-group__heading {
  display: flex;
  gap: 10px;
  align-items: center;
  padding-bottom: 12px;
  margin-bottom: 12px;
  border-bottom: 1px solid #eef2f7;
}

.detail-group__heading i {
  display: grid;
  width: 34px;
  height: 34px;
  font-size: 17px;
  color: #15803d;
  background: #ecfdf3;
  border-radius: 8px;
  place-items: center;
}

.detail-group__heading div {
  display: grid;
  gap: 3px;
}

.detail-group__heading strong {
  color: #0f172a;
}

.detail-group__heading span,
.detail-grid dt {
  font-size: 12px;
  color: #64748b;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin: 0;
}

.detail-grid > div {
  min-width: 0;
}

.detail-grid dt {
  margin-bottom: 5px;
}

.detail-grid dd {
  margin: 0;
  overflow-wrap: anywhere;
  font-weight: 650;
  line-height: 1.55;
  color: #334155;
}

.detail-grid--metrics dd {
  font-size: 16px;
  color: #0f172a;
}

.detail-grid__wide {
  grid-column: span 3;
}

.cooperation-table {
  margin-top: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
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

.profile-header-actions {
  display: flex;
  gap: 8px;
}

.profile-facts {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0;
  margin: 0;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.profile-facts > div {
  min-width: 0;
  padding: 10px 12px;
  border-right: 1px solid #eef2f7;
  border-bottom: 1px solid #eef2f7;
}

.profile-facts dt {
  margin-bottom: 4px;
  font-size: 11px;
  color: #94a3b8;
}

.profile-facts dd {
  margin: 0;
  overflow: hidden;
  font-size: 13px;
  font-weight: 650;
  color: #334155;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-facts__wide {
  grid-column: span 5;
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
  .profile-metrics,
  .resource-expand,
  .detail-grid {
    grid-template-columns: 1fr;
  }

  .resource-card__main {
    grid-template-columns: 1fr;
    gap: 14px;
  }

  .compact-list-head {
    display: none;
  }

  .compact-resource-row,
  .profile-facts {
    grid-template-columns: 1fr;
  }

  .compact-cooperation dl {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .compact-index,
  .compact-content {
    display: none;
  }

  .compact-actions {
    justify-content: flex-start;
  }

  .profile-facts__wide {
    grid-column: auto;
  }

  .resource-metrics,
  .cooperation-summary,
  .recent-content {
    padding: 14px 0 0;
    border-top: 1px solid #edf0f3;
    border-left: 0;
  }

  .resource-actions {
    justify-content: space-between;
  }

  .editor-header,
  .editor-footer,
  .editor-section__title--between {
    align-items: stretch;
    flex-direction: column;
  }

  .editor-form-grid,
  .editor-form-grid--metrics,
  .cooperation-form-grid,
  .cooperation-field-summary {
    grid-template-columns: 1fr;
  }

  .editor-form-grid__wide,
  .cooperation-form-grid__wide {
    grid-column: auto;
  }

  .detail-group:nth-child(3),
  .detail-group:nth-child(4),
  .detail-grid__wide {
    grid-column: auto;
  }

  .resource-expand {
    padding: 12px;
  }

  .sheet-select {
    min-width: 100%;
  }
}

@media (width > 760px) and (width <= 1180px) {
  .compact-list-head,
  .compact-resource-row {
    grid-template-columns:
      32px minmax(205px, 1fr) minmax(150px, 0.75fr)
      minmax(300px, 1.5fr) 112px;
  }

  .compact-list-head span:nth-child(5),
  .compact-content {
    display: none;
  }

  .resource-card__main {
    grid-template-columns:
      minmax(250px, 1.2fr) repeat(2, minmax(210px, 1fr))
      auto;
  }

  .recent-content {
    grid-column: 2 / 4;
  }
}
</style>
