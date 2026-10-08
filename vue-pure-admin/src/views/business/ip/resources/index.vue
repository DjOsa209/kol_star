<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useUserStoreHook } from "@/store/modules/user";
import { ElMessage, ElMessageBox } from "element-plus";
import * as XLSX from "xlsx";
import {
  deleteIPFile,
  getIPResource,
  importIPResources,
  listIPResources,
  previewIPImport,
  readIPFile,
  saveIPCase,
  saveIPResource,
  uploadIPFile,
  type IPResourceInput
} from "@/api/ip";

defineOptions({ name: "IPResources" });
const router = useRouter();
const route = useRoute();
const mode = ref<"list" | "form" | "detail">("list");
const selectingForRequest = computed(
  () => route.query.selectForRequest === "1"
);
const selectedForRequest = ref<number[]>([]);
const activeTab = ref<"single" | "bulk">("single");
const loading = ref(false);
const saving = ref(false);
const advancedFilters = ref(false);
const rows = ref<any[]>([]);
const total = ref(0);
const stats = ref({ total: 0, cooperable: 0, pendingFiles: 0, recent: 0 });
const filter = reactive({
  keyword: "",
  ipType: "",
  status: "",
  market: "",
  completeness: "",
  page: 1,
  pageSize: 20
});
const updatedRange = ref<string[]>([]);
const types = [
  "体育IP",
  "动画/影视",
  "音乐/活动",
  "电竞",
  "游戏",
  "文化艺术",
  "其他"
];
const statuses = ["可合作", "洽谈中", "待评估", "暂不可合作"];
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
const profileFields = [
  {
    key: "imageStyle",
    label: "IP形象与风格",
    hint: "代表形象、视觉风格或图片链接"
  },
  { key: "lifecycle", label: "IP生命周期", hint: "常青 / 新晋 / 短期爆款" },
  { key: "genderRatio", label: "受众性别占比", hint: "例如：男性55%，女性45%" },
  { key: "ageRange", label: "主要年龄", hint: "核心及次核心年龄区间" },
  {
    key: "spendingPower",
    label: "消费力评价",
    hint: "ARPU、购机能力或换机周期"
  },
  {
    key: "registeredUsers",
    label: "注册用户 / 赛事参与",
    hint: "游戏MAU、下载量或赛事观赛量"
  },
  {
    key: "salesROI",
    label: "历史销售转化力",
    hint: "如有联名合作ROI，可在此说明"
  },
  {
    key: "showcase",
    label: "IP展示与合作表现",
    hint: "代表作品、其他品牌案例及与本品牌的历史表现"
  },
  {
    key: "cooperationTips",
    label: "合作建议与提醒",
    hint: "联动内容、监修风险等"
  }
];
function objectValue(value: unknown): Record<string, string> {
  if (value && typeof value === "object" && !Array.isArray(value))
    return value as Record<string, string>;
  try {
    const parsed = JSON.parse(String(value || "{}"));
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed
      : {};
  } catch {
    return {};
  }
}
const emptyForm = (): IPResourceInput => ({
  name: "",
  ipType: "",
  rightsOwner: "",
  contact: "",
  markets: [],
  audience: "",
  summary: "",
  cooperationStatus: "待评估",
  currency: "CNY",
  priceMin: null,
  priceMax: null,
  licenseNotes: "",
  profile: {}
});
const form = ref<IPResourceInput>(emptyForm());
function manualDraftKey() {
  return `ip-resource-draft:${useUserStoreHook().username || "current"}`;
}
function saveManualDraft() {
  localStorage.setItem(
    manualDraftKey(),
    JSON.stringify({
      form: form.value,
      caseTitle: caseTitle.value,
      caseSummary: caseSummary.value
    })
  );
  ElMessage.success("文字草稿已保存在当前浏览器，附件需重新选择");
}
function restoreManualDraft() {
  try {
    const saved = JSON.parse(localStorage.getItem(manualDraftKey()) || "null");
    if (!saved?.form || typeof saved.form !== "object") return;
    form.value = {
      ...emptyForm(),
      ...saved.form,
      id: undefined,
      markets: Array.isArray(saved.form.markets) ? saved.form.markets : [],
      profile: objectValue(saved.form.profile)
    };
    caseTitle.value = String(saved.caseTitle || "");
    caseSummary.value = String(saved.caseSummary || "");
  } catch {
    localStorage.removeItem(manualDraftKey());
  }
}
const detail = ref<{ resource: any; cases: any[]; files: any[] } | null>(null);
const drawerVisible = ref(false);
const drawerLoading = ref(false);
const drawerDetail = ref<{ resource: any; cases: any[]; files: any[] } | null>(
  null
);
const drawerVisualURL = ref("");
const thumbUrls = ref<Record<number, string>>({});
const thumbFileIds = ref<Record<number, number>>({});
const copyrightFiles = ref<File[]>([]);
const visualFile = ref<File | null>(null);
const visualURL = ref("");
const caseFile = ref<File | null>(null);
const caseTitle = ref("");
const caseSummary = ref("");
const previewVisible = ref(false);
const previewURL = ref("");
const importFileName = ref("");
const importRows = ref<IPResourceInput[]>([]);
const importPreview = ref<any[]>([]);
const importLoading = ref(false);
const importKeys = ref<string[]>([]);
const importCaseTitles = ref<string[]>([]);
const importCaseSummaries = ref<string[]>([]);
const batchFiles = ref<File[]>([]);

const readyCount = computed(
  () => importPreview.value.filter(row => row.status === "ready").length
);
const duplicateCount = computed(
  () => importPreview.value.filter(row => row.status === "duplicate").length
);
const errorCount = computed(
  () => importPreview.value.filter(row => row.status === "error").length
);

function arrayValue(value: unknown): string[] {
  if (Array.isArray(value)) return value;
  try {
    return JSON.parse(String(value || "[]"));
  } catch {
    return [];
  }
}
function priceText(item: any) {
  if (item?.priceMin == null && item?.priceMax == null) return "待询价";
  const symbol =
    item.currency === "USD" ? "$" : item.currency === "EUR" ? "€" : "¥";
  const min =
    item.priceMin == null ? "—" : Number(item.priceMin).toLocaleString();
  const max =
    item.priceMax == null ? "—" : Number(item.priceMax).toLocaleString();
  return `${symbol}${min} – ${symbol}${max}`;
}
async function loadList() {
  loading.value = true;
  try {
    const result = await listIPResources({
      ...filter,
      updatedFrom: updatedRange.value[0] || "",
      updatedTo: updatedRange.value[1] || ""
    });
    rows.value = result.data.list || [];
    total.value = result.data.total || 0;
    stats.value = result.data.stats || {
      total: 0,
      cooperable: 0,
      pendingFiles: 0,
      recent: 0
    };
    const activeIds = new Set(rows.value.map(row => Number(row.id)));
    for (const [id, url] of Object.entries(thumbUrls.value)) {
      const row = rows.value.find(item => Number(item.id) === Number(id));
      if (
        !activeIds.has(Number(id)) ||
        Number(row?.visualFileId) !== thumbFileIds.value[Number(id)]
      ) {
        URL.revokeObjectURL(url);
        delete thumbUrls.value[Number(id)];
        delete thumbFileIds.value[Number(id)];
      }
    }
    void Promise.allSettled(
      rows.value
        .filter(row => row.visualFileId && !thumbUrls.value[Number(row.id)])
        .map(async row => {
          const blob = await readIPFile(Number(row.visualFileId));
          if (
            rows.value.some(
              item =>
                Number(item.id) === Number(row.id) &&
                Number(item.visualFileId) === Number(row.visualFileId)
            )
          ) {
            thumbUrls.value[Number(row.id)] = URL.createObjectURL(blob);
            thumbFileIds.value[Number(row.id)] = Number(row.visualFileId);
          }
        })
    );
  } finally {
    loading.value = false;
  }
}
function resetFilters() {
  Object.assign(filter, {
    keyword: "",
    ipType: "",
    status: "",
    market: "",
    completeness: "",
    page: 1
  });
  updatedRange.value = [];
  loadList();
}
function goList() {
  drawerVisible.value = false;
  router.push({ path: "/business/ip/resources" });
}
async function openDrawer(id: number) {
  drawerVisible.value = true;
  drawerLoading.value = true;
  drawerDetail.value = null;
  try {
    drawerDetail.value = (await getIPResource(id)).data;
    if (drawerVisualURL.value) URL.revokeObjectURL(drawerVisualURL.value);
    drawerVisualURL.value = "";
    const visual = drawerDetail.value.files.find(
      file => file.fileKind === "visual"
    );
    if (visual)
      drawerVisualURL.value = URL.createObjectURL(
        await readIPFile(Number(visual.id))
      );
  } catch {
    drawerVisible.value = false;
    ElMessage.error("IP档案加载失败，请重试");
  } finally {
    drawerLoading.value = false;
  }
}
function goNew(tab: "single" | "bulk" = "single") {
  activeTab.value = tab;
  router.push({ path: "/business/ip/resources", query: { mode: "new", tab } });
}
function goDetail(id: number) {
  if (selectingForRequest.value) {
    window.open(
      router.resolve({
        path: "/business/ip/resources",
        query: { id: String(id) }
      }).href,
      "_blank",
      "noopener"
    );
    return;
  }
  drawerVisible.value = false;
  router.push({ path: "/business/ip/resources", query: { id: String(id) } });
}
function toggleForRequest(id: number) {
  if (selectedForRequest.value.includes(id))
    selectedForRequest.value = selectedForRequest.value.filter(
      value => value !== id
    );
  else if (selectedForRequest.value.length < 5)
    selectedForRequest.value.push(id);
  else ElMessage.warning("最多选择5个意向IP");
}
function confirmForRequest() {
  const key = String(route.query.selectionKey || "");
  if (!key) return;
  localStorage.setItem(key, JSON.stringify(selectedForRequest.value));
  ElMessage.success("已加入需求意向清单，请返回需求页面");
  window.close();
}
async function loadDetail(id: number) {
  loading.value = true;
  try {
    detail.value = (await getIPResource(id)).data;
    if (visualURL.value) URL.revokeObjectURL(visualURL.value);
    visualURL.value = "";
    const visual = detail.value.files.find(file => file.fileKind === "visual");
    if (visual)
      visualURL.value = URL.createObjectURL(
        await readIPFile(Number(visual.id))
      );
  } finally {
    loading.value = false;
  }
}
function editDetail() {
  if (!detail.value) return;
  const item = detail.value.resource;
  form.value = {
    id: Number(item.id),
    name: item.name,
    ipType: item.ipType,
    rightsOwner: item.rightsOwner || "",
    contact: item.contact || "",
    markets: arrayValue(item.markets),
    audience: item.audience || "",
    summary: item.summary || "",
    cooperationStatus: item.cooperationStatus || "待评估",
    currency: item.currency || "CNY",
    priceMin: item.priceMin == null ? null : Number(item.priceMin),
    priceMax: item.priceMax == null ? null : Number(item.priceMax),
    licenseNotes: item.licenseNotes || "",
    profile: objectValue(item.profile)
  };
  mode.value = "form";
  activeTab.value = "single";
}
function pdfChanged(event: Event, kind: "copyright" | "case") {
  const files = Array.from((event.target as HTMLInputElement).files || []);
  const invalid = files.some(
    file => file.type !== "application/pdf" || file.size > 20 * 1024 * 1024
  );
  if (invalid) {
    ElMessage.error("仅支持20MB以内的PDF文件");
    return;
  }
  if (kind === "copyright") copyrightFiles.value.push(...files);
  else caseFile.value = files[0] || null;
  (event.target as HTMLInputElement).value = "";
}
function visualChanged(event: Event) {
  visualFile.value = null;
  const file = (event.target as HTMLInputElement).files?.[0];
  (event.target as HTMLInputElement).value = "";
  if (!file) return;
  if (
    !["image/png", "image/jpeg", "image/webp"].includes(file.type) ||
    file.size > 5 * 1024 * 1024
  ) {
    ElMessage.warning("IP形象仅支持5MB以内的PNG、JPG或WebP图片");
    return;
  }
  visualFile.value = file;
}
async function addVisual(event: Event) {
  visualChanged(event);
  if (!visualFile.value || !detail.value) return;
  const id = Number(detail.value.resource.id);
  const file = visualFile.value;
  visualFile.value = null;
  await uploadIPFile(id, "visual", file);
  await loadDetail(id);
  ElMessage.success("IP形象已上传");
}
async function saveManual() {
  const item = form.value;
  if (!item.name.trim() || !item.ipType || !item.markets.length) {
    ElMessage.warning("请填写IP名称、类型和覆盖市场");
    return;
  }
  if (
    item.priceMin != null &&
    item.priceMax != null &&
    item.priceMin > item.priceMax
  ) {
    ElMessage.warning("价格下限不能高于上限");
    return;
  }
  if (caseFile.value && !caseTitle.value.trim()) {
    ElMessage.warning("上传结案文件前请填写历史合作名称");
    return;
  }
  saving.value = true;
  try {
    const result = await saveIPResource(item);
    const id = Number(result.data.id);
    for (const file of copyrightFiles.value)
      await uploadIPFile(id, "copyright", file);
    if (visualFile.value) await uploadIPFile(id, "visual", visualFile.value);
    if (caseTitle.value.trim()) {
      const caseResult = await saveIPCase({
        ipId: id,
        title: caseTitle.value.trim(),
        summary: caseSummary.value
      });
      if (caseFile.value)
        await uploadIPFile(
          id,
          "case",
          caseFile.value,
          Number(caseResult.data.id)
        );
    }
    copyrightFiles.value = [];
    visualFile.value = null;
    caseFile.value = null;
    caseTitle.value = "";
    caseSummary.value = "";
    ElMessage.success(item.id ? "IP资料已更新" : "IP已创建");
    if (!item.id) localStorage.removeItem(manualDraftKey());
    if (Number(route.query.id) === id) {
      mode.value = "detail";
      await loadDetail(id);
    } else goDetail(id);
  } finally {
    saving.value = false;
  }
}
async function addCase() {
  if (!detail.value || !caseTitle.value.trim()) {
    ElMessage.warning("请填写合作案例名称");
    return;
  }
  saving.value = true;
  try {
    const id = Number(detail.value.resource.id);
    const result = await saveIPCase({
      ipId: id,
      title: caseTitle.value.trim(),
      summary: caseSummary.value
    });
    if (caseFile.value)
      await uploadIPFile(id, "case", caseFile.value, Number(result.data.id));
    caseTitle.value = "";
    caseSummary.value = "";
    caseFile.value = null;
    await loadDetail(id);
    ElMessage.success("合作案例已添加");
  } finally {
    saving.value = false;
  }
}
async function addCopyright(event: Event) {
  const files = Array.from((event.target as HTMLInputElement).files || []);
  (event.target as HTMLInputElement).value = "";
  if (!detail.value || !files.length) return;
  if (
    files.some(
      file => file.type !== "application/pdf" || file.size > 20 * 1024 * 1024
    )
  ) {
    ElMessage.error("仅支持20MB以内的PDF文件");
    return;
  }
  saving.value = true;
  try {
    const id = Number(detail.value.resource.id);
    for (const file of files) await uploadIPFile(id, "copyright", file);
    await loadDetail(id);
    ElMessage.success("版权资料已上传");
  } finally {
    saving.value = false;
  }
}
async function openFile(file: any, download = false) {
  const blob = await readIPFile(Number(file.id));
  const url = URL.createObjectURL(
    new Blob([blob], { type: "application/pdf" })
  );
  if (download) {
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = file.originalName;
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 60000);
  } else {
    if (previewURL.value) URL.revokeObjectURL(previewURL.value);
    previewURL.value = url;
    previewVisible.value = true;
  }
}
async function removeFile(file: any) {
  await ElMessageBox.confirm(`删除「${file.originalName}」？`, "确认删除", {
    type: "warning"
  });
  await deleteIPFile(Number(file.id));
  if (detail.value) await loadDetail(Number(detail.value.resource.id));
  ElMessage.success("文件已删除");
}
function downloadTemplate() {
  const headers = [
    "导入编号",
    "IP名称*",
    "IP类型*",
    "覆盖市场*",
    "版权方",
    "联系人",
    "目标受众",
    "IP简介",
    "合作状态",
    "币种",
    "基础权益价格下限",
    "基础权益价格上限",
    "授权范围与限制",
    ...profileFields.map(field => field.label),
    "历史合作名称",
    "历史合作概述"
  ];
  const sheet = XLSX.utils.aoa_to_sheet([headers]);
  sheet["!cols"] = headers.map(() => ({ wch: 24 }));
  const book = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(book, sheet, "IP导入模板");
  XLSX.writeFile(book, "IP导入模板.xlsx");
}
function parseMoney(value: unknown): number | null {
  if (value == null || String(value).trim() === "") return null;
  const amount = Number(String(value).replace(/,/g, ""));
  return Number.isFinite(amount) ? amount : -1;
}
async function readImport(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  (event.target as HTMLInputElement).value = "";
  if (!file) return;
  try {
    const book = XLSX.read(await file.arrayBuffer());
    const sheet = book.Sheets[book.SheetNames[0]];
    const records = XLSX.utils.sheet_to_json<Record<string, any>>(sheet, {
      defval: ""
    });
    if (!records.length || records.length > 500) {
      ElMessage.warning("每次请选择1至500条IP");
      return;
    }
    importKeys.value = records.map(row => String(row["导入编号"] || "").trim());
    importCaseTitles.value = records.map(row =>
      String(row["历史合作名称"] || "").trim()
    );
    importCaseSummaries.value = records.map(row =>
      String(row["历史合作概述"] || "").trim()
    );
    importRows.value = records.map(row => ({
      name: String(row["IP名称*"] || row["IP名称"] || "").trim(),
      ipType: String(row["IP类型*"] || row["IP类型"] || "").trim(),
      markets: String(row["覆盖市场*"] || row["覆盖市场"] || "")
        .split(/[,，、;/]+/)
        .map(s => s.trim())
        .filter(Boolean),
      rightsOwner: String(row["版权方"] || "").trim(),
      contact: String(row["联系人"] || "").trim(),
      audience: String(row["目标受众"] || "").trim(),
      summary: String(row["IP简介"] || "").trim(),
      cooperationStatus: String(row["合作状态"] || "待评估").trim(),
      currency: String(row["币种"] || "CNY").trim(),
      priceMin: parseMoney(row["基础权益价格下限"]),
      priceMax: parseMoney(row["基础权益价格上限"]),
      licenseNotes: String(row["授权范围与限制"] || "").trim(),
      profile: Object.fromEntries(
        profileFields.map(field => [
          field.key,
          String(row[field.label] || "").trim()
        ])
      )
    }));
    importFileName.value = file.name;
    importLoading.value = true;
    importPreview.value = (await previewIPImport(importRows.value)).data || [];
  } catch {
    ElMessage.error("文件解析失败，请使用下载的模板");
  } finally {
    importLoading.value = false;
  }
}
function readBatchFiles(event: Event) {
  const files = Array.from((event.target as HTMLInputElement).files || []);
  (event.target as HTMLInputElement).value = "";
  if (
    files.some(
      file => file.type !== "application/pdf" || file.size > 20 * 1024 * 1024
    )
  ) {
    ElMessage.error("仅支持20MB以内的PDF文件");
    return;
  }
  batchFiles.value = files;
}
function batchFileKind(
  fileName: string,
  key: string
): "copyright" | "case" | "" {
  if (
    fileName.startsWith(`${key}_版权介绍`) &&
    fileName.toLowerCase().endsWith(".pdf")
  )
    return "copyright";
  if (
    fileName.startsWith(`${key}_结案`) &&
    fileName.toLowerCase().endsWith(".pdf")
  )
    return "case";
  return "";
}
async function runImport() {
  if (!readyCount.value) return;
  if (batchFiles.value.length) {
    const keys = importPreview.value
      .filter(row => row.status === "ready")
      .map(row => importKeys.value[Number(row.row) - 2]);
    if (keys.some(key => !key) || new Set(keys).size !== keys.length) {
      ElMessage.warning("批量关联PDF时，请为可导入行填写不重复的导入编号");
      return;
    }
    if (
      batchFiles.value.some(
        file => !keys.some(key => batchFileKind(file.name, key))
      )
    ) {
      ElMessage.warning("有PDF文件名未匹配可导入行的编号或附件类型");
      return;
    }
  }
  importLoading.value = true;
  try {
    const result = await importIPResources(importRows.value);
    let uploaded = 0;
    let failed = 0;
    for (const created of result.data.created || []) {
      const index = Number(created.row) - 2;
      const key = importKeys.value[index];
      if (!key) continue;
      const matched = batchFiles.value.filter(file =>
        batchFileKind(file.name, key)
      );
      let caseId = 0;
      for (const file of matched) {
        try {
          if (batchFileKind(file.name, key) === "copyright") {
            await uploadIPFile(created.id, "copyright", file);
            uploaded++;
          } else if (batchFileKind(file.name, key) === "case") {
            if (!caseId) {
              const saved = await saveIPCase({
                ipId: created.id,
                title: importCaseTitles.value[index] || "历史合作",
                summary: importCaseSummaries.value[index] || ""
              });
              caseId = Number(saved.data.id);
            }
            await uploadIPFile(created.id, "case", file, caseId);
            uploaded++;
          }
        } catch {
          failed++;
        }
      }
    }
    ElMessage.success(
      `成功导入${result.data.imported}条IP，关联${uploaded}份PDF${failed ? `，${failed}份附件上传失败，请在详情页补充` : ""}`
    );
    importRows.value = [];
    importPreview.value = [];
    importFileName.value = "";
    batchFiles.value = [];
    goList();
    await loadList();
  } finally {
    importLoading.value = false;
  }
}
function syncRoute() {
  if (selectingForRequest.value && route.query.selected) {
    selectedForRequest.value = String(route.query.selected)
      .split(",")
      .map(Number)
      .filter(value => value > 0)
      .slice(0, 5);
  }
  const id = Number(route.query.id);
  if (id > 0) {
    mode.value = "detail";
    loadDetail(id);
  } else if (route.query.mode === "new") {
    mode.value = "form";
    form.value = emptyForm();
    if (!route.query.name && !route.query.rightsOwner && !route.query.summary)
      restoreManualDraft();
    form.value.name = String(route.query.name || "");
    form.value.rightsOwner = String(route.query.rightsOwner || "");
    form.value.summary = String(route.query.summary || "");
    if (route.query.imageStyle)
      form.value.profile.imageStyle = String(route.query.imageStyle);
    activeTab.value = route.query.tab === "bulk" ? "bulk" : "single";
  } else {
    mode.value = "list";
    loadList();
  }
}
watch(() => route.fullPath, syncRoute);
watch(previewVisible, open => {
  if (!open && previewURL.value) {
    URL.revokeObjectURL(previewURL.value);
    previewURL.value = "";
  }
});
onMounted(syncRoute);
onUnmounted(() => {
  if (visualURL.value) URL.revokeObjectURL(visualURL.value);
  if (drawerVisualURL.value) URL.revokeObjectURL(drawerVisualURL.value);
  for (const url of Object.values(thumbUrls.value)) URL.revokeObjectURL(url);
});
</script>

<template>
  <div v-loading="loading" class="ip-page">
    <div class="ip-head">
      <div>
        <div v-if="mode !== 'list'" class="ip-eyebrow">
          IP资源库 / {{ mode === "detail" ? detail?.resource.name : "新增IP" }}
        </div>
        <h1>
          {{
            mode === "list"
              ? "IP资源库"
              : mode === "detail"
                ? detail?.resource.name || "IP详情"
                : "新增IP"
          }}
        </h1>
        <p>
          {{
            mode === "list"
              ? "统一沉淀IP资产，支持需求快速匹配"
              : mode === "detail"
                ? "版权资料、参考权益价格与历史合作"
                : "支持单条录入与模板批量导入"
          }}
        </p>
      </div>
      <div class="ip-actions">
        <el-button v-if="mode !== 'list'" @click="goList">返回资源库</el-button>
        <el-button v-if="mode === 'list'" class="ip-primary" @click="goNew()"
          >＋ 新增IP</el-button
        >
        <el-button
          v-if="mode === 'list' && selectingForRequest"
          class="ip-primary"
          @click="confirmForRequest"
          >确认选择 {{ selectedForRequest.length }} 个IP</el-button
        >
        <el-button
          v-if="mode === 'detail'"
          class="ip-primary"
          @click="editDetail"
          >编辑IP资料</el-button
        >
      </div>
    </div>

    <template v-if="mode === 'list'">
      <div class="ip-stats">
        <div class="ip-stat">
          <div class="ip-stat-icon">
            <IconifyIconOnline icon="ri:stack-line" />
          </div>
          <div>
            <span>IP总数</span><strong>{{ stats.total }}</strong>
          </div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-icon">
            <IconifyIconOnline icon="ri:group-line" />
          </div>
          <div>
            <span>可合作</span><strong>{{ stats.cooperable }}</strong>
          </div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-icon">
            <IconifyIconOnline icon="ri:file-list-3-line" />
          </div>
          <div>
            <span>待版权资料</span><strong>{{ stats.pendingFiles }}</strong>
          </div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-icon">
            <IconifyIconOnline icon="ri:line-chart-line" />
          </div>
          <div>
            <span>近30天新增</span><strong>{{ stats.recent }}</strong>
          </div>
        </div>
      </div>
      <div class="ip-card">
        <div class="ip-filter">
          <el-input
            v-model="filter.keyword"
            placeholder="搜索IP名称、版权方或市场"
            clearable
            @keyup.enter="loadList"
          />
          <el-select v-model="filter.ipType" placeholder="全部类型" clearable
            ><el-option
              v-for="type in types"
              :key="type"
              :label="type"
              :value="type"
          /></el-select>
          <el-select v-model="filter.status" placeholder="合作状态" clearable
            ><el-option
              v-for="status in statuses"
              :key="status"
              :label="status"
              :value="status"
          /></el-select>
          <el-select
            v-model="filter.market"
            placeholder="目标市场"
            clearable
            filterable
          >
            <el-option
              v-for="market in markets"
              :key="market"
              :label="market"
              :value="market"
            />
          </el-select>
          <el-button
            class="ip-primary"
            @click="
              filter.page = 1;
              loadList();
            "
            >搜索</el-button
          >
          <el-button @click="resetFilters">重置</el-button>
          <el-button link @click="advancedFilters = !advancedFilters">{{
            advancedFilters ? "收起筛选" : "高级筛选"
          }}</el-button>
        </div>
        <div v-if="advancedFilters" class="ip-filter ip-advanced-filter">
          <el-select
            v-model="filter.completeness"
            placeholder="资料完成度"
            clearable
          >
            <el-option label="资料完整" value="complete" />
            <el-option label="待补充" value="incomplete" />
          </el-select>
          <el-date-picker
            v-model="updatedRange"
            type="daterange"
            value-format="YYYY-MM-DD"
            start-placeholder="更新起始"
            end-placeholder="更新截止"
          />
        </div>
        <el-table :data="rows" empty-text="暂无IP，点击右上角新增" stripe>
          <el-table-column v-if="selectingForRequest" label="选择" width="65">
            <template #default="scope"
              ><el-checkbox
                :model-value="selectedForRequest.includes(Number(scope.row.id))"
                @change="toggleForRequest(Number(scope.row.id))"
            /></template>
          </el-table-column>
          <el-table-column prop="name" label="IP名称" min-width="260"
            ><template #default="scope"
              ><div class="ip-name-cell">
                <img
                  v-if="thumbUrls[Number(scope.row.id)]"
                  :src="thumbUrls[Number(scope.row.id)]"
                  :alt="scope.row.name"
                />
                <div v-else class="ip-name-empty">IP</div>
                <div>
                  <button class="ip-link" @click="openDrawer(scope.row.id)">
                    {{ scope.row.name }}</button
                  ><small>{{
                    scope.row.summary || scope.row.rightsOwner || "版权方待补充"
                  }}</small>
                </div>
              </div></template
            ></el-table-column
          >
          <el-table-column label="类型" min-width="110"
            ><template #default="scope"
              ><el-tag effect="light">{{ scope.row.ipType }}</el-tag></template
            ></el-table-column
          >
          <el-table-column label="覆盖市场" min-width="150"
            ><template #default="scope">{{
              arrayValue(scope.row.markets).join(" / ")
            }}</template></el-table-column
          >
          <el-table-column
            prop="audience"
            label="目标受众"
            min-width="140"
            show-overflow-tooltip
          />
          <el-table-column label="基础权益价格范围" min-width="180"
            ><template #header
              >基础权益价格范围 <small class="ip-muted">供参考</small></template
            ><template #default="scope">{{
              priceText(scope.row)
            }}</template></el-table-column
          >
          <el-table-column label="合作状态" min-width="110"
            ><template #default="scope"
              ><el-tag
                :type="
                  scope.row.cooperationStatus === '可合作'
                    ? 'success'
                    : 'warning'
                "
                >{{ scope.row.cooperationStatus }}</el-tag
              ></template
            ></el-table-column
          >
          <el-table-column label="资料完成度" width="130">
            <template #default="scope"
              ><el-progress
                :percentage="Number(scope.row.completeness || 0)"
                :show-text="true"
            /></template>
          </el-table-column>
          <el-table-column prop="updatedAt" label="最近更新" min-width="145" />
          <el-table-column label="操作" width="100"
            ><template #default="scope"
              ><el-button link type="primary" @click="openDrawer(scope.row.id)"
                >查看档案</el-button
              ></template
            ></el-table-column
          >
        </el-table>
        <div class="ip-pagination">
          <el-pagination
            v-model:current-page="filter.page"
            :page-size="filter.pageSize"
            layout="total, prev, pager, next"
            :total="total"
            @current-change="loadList"
          />
        </div>
      </div>
      <el-drawer
        v-model="drawerVisible"
        title="IP档案"
        size="420px"
        class="ip-resource-drawer"
      >
        <div v-loading="drawerLoading" class="ip-drawer-content">
          <template v-if="drawerDetail">
            <div class="ip-drawer-intro">
              <img
                v-if="drawerVisualURL"
                :src="drawerVisualURL"
                :alt="drawerDetail.resource.name"
              />
              <div>
                <h2>{{ drawerDetail.resource.name }}</h2>
                <el-tag>{{ drawerDetail.resource.ipType }}</el-tag>
                <p>{{ drawerDetail.resource.audience || "受众待补充" }}</p>
              </div>
            </div>
            <p>{{ drawerDetail.resource.summary || "暂无IP简介" }}</p>
            <div class="ip-drawer-section">
              <h3>核心信息</h3>
              <dl class="ip-dl">
                <dt>版权方</dt>
                <dd>{{ drawerDetail.resource.rightsOwner || "待补充" }}</dd>
                <dt>覆盖市场</dt>
                <dd>
                  {{ arrayValue(drawerDetail.resource.markets).join(" / ") }}
                </dd>
                <dt>合作状态</dt>
                <dd>{{ drawerDetail.resource.cooperationStatus }}</dd>
              </dl>
            </div>
            <div class="ip-drawer-section">
              <h3>参考权益价格</h3>
              <strong class="ip-price">{{
                priceText(drawerDetail.resource)
              }}</strong>
              <p class="ip-muted">供参考，根据具体需求更新</p>
            </div>
            <div class="ip-drawer-section">
              <h3>可查看资料</h3>
              <div
                v-for="file in drawerDetail.files.filter(
                  item => item.fileKind !== 'visual'
                )"
                :key="file.id"
                class="ip-file"
              >
                <span
                  ><b class="ip-file-icon">PDF</b>{{ file.originalName }}</span
                >
                <el-button link @click="openFile(file)">预览</el-button>
              </div>
              <p
                v-if="
                  !drawerDetail.files.some(item => item.fileKind !== 'visual')
                "
                class="ip-muted"
              >
                暂无附件
              </p>
            </div>
            <el-button
              class="ip-primary ip-drawer-action"
              @click="goDetail(Number(drawerDetail.resource.id))"
              >查看详情 →</el-button
            >
          </template>
        </div>
      </el-drawer>
    </template>

    <template v-else-if="mode === 'detail' && detail">
      <div class="ip-card ip-hero">
        <div class="ip-hero-media">
          <img v-if="visualURL" :src="visualURL" :alt="detail.resource.name" />
          <div v-else class="ip-hero-empty">暂无IP形象</div>
          <label class="ip-hero-upload"
            >＋ 上传形象<input
              type="file"
              accept="image/png,image/jpeg,image/webp"
              hidden
              @change="addVisual"
          /></label>
        </div>
        <div class="ip-hero-copy">
          <strong>{{ detail.resource.name }}</strong>
          <p>
            {{
              detail.resource.summary ||
              "暂无IP简介，请补充背景、特点和影响力。"
            }}
          </p>
          <div class="ip-hero-tags">
            <el-tag>{{ detail.resource.ipType }}</el-tag>
            <el-tag type="success">{{
              detail.resource.cooperationStatus
            }}</el-tag>
            <el-tag
              v-if="objectValue(detail.resource.profile).lifecycle"
              type="info"
            >
              {{ objectValue(detail.resource.profile).lifecycle }}
            </el-tag>
          </div>
        </div>
      </div>
      <div class="ip-detail-grid">
        <div class="ip-stack">
          <div class="ip-card">
            <h2>基础权益价格范围</h2>
            <div class="ip-price">{{ priceText(detail.resource) }}</div>
            <p class="ip-muted">
              供参考，根据具体需求更新；最终以版权方报价为准。
            </p>
          </div>
          <div class="ip-card">
            <div class="ip-card-head">
              <h2>版权介绍文件</h2>
              <label class="ip-upload-inline"
                >＋ 上传版权资料<input
                  type="file"
                  accept="application/pdf,.pdf"
                  multiple
                  hidden
                  @change="addCopyright"
              /></label>
            </div>
            <div
              v-for="file in detail.files.filter(
                f => f.fileKind === 'copyright'
              )"
              :key="file.id"
              class="ip-file"
            >
              <span
                ><b class="ip-file-icon">PDF</b>{{ file.originalName }}
                <small
                  >{{ (Number(file.sizeBytes) / 1024 / 1024).toFixed(1) }} MB ·
                  {{ file.createdAt }}</small
                ></span
              >
              <div>
                <el-button link @click="openFile(file)">预览</el-button
                ><el-button link @click="openFile(file, true)">下载</el-button
                ><el-button link type="danger" @click="removeFile(file)"
                  >删除</el-button
                >
              </div>
            </div>
            <el-empty
              v-if="!detail.files.some(f => f.fileKind === 'copyright')"
              description="暂无版权介绍文件"
              :image-size="68"
            />
          </div>
          <div class="ip-card">
            <h2>历史合作与结案</h2>
            <div v-for="item in detail.cases" :key="item.id" class="ip-case">
              <strong>{{ item.title }}</strong>
              <p>{{ item.summary || "暂无案例说明" }}</p>
              <div
                v-for="file in detail.files.filter(
                  f => Number(f.caseId) === Number(item.id)
                )"
                :key="file.id"
                class="ip-file"
              >
                <span
                  ><b class="ip-file-icon">PDF</b>{{ file.originalName }}</span
                >
                <div>
                  <el-button link @click="openFile(file)">预览</el-button
                  ><el-button link @click="openFile(file, true)">下载</el-button
                  ><el-button link type="danger" @click="removeFile(file)"
                    >删除</el-button
                  >
                </div>
              </div>
            </div>
            <p v-if="!detail.cases.length" class="ip-muted">暂无历史合作</p>
            <div class="ip-case-form">
              <el-input
                v-model="caseTitle"
                placeholder="合作案例名称"
              /><el-input
                v-model="caseSummary"
                placeholder="案例概述（选填）"
              /><label class="ip-upload-inline"
                >{{ caseFile ? caseFile.name : "＋ 添加结案PDF"
                }}<input
                  type="file"
                  accept="application/pdf,.pdf"
                  hidden
                  @change="pdfChanged($event, 'case')" /></label
              ><el-button :loading="saving" @click="addCase"
                >添加历史合作</el-button
              >
            </div>
          </div>
        </div>
        <div class="ip-stack">
          <div class="ip-card">
            <h2>基础信息</h2>
            <dl class="ip-dl">
              <dt>IP类型</dt>
              <dd>{{ detail.resource.ipType }}</dd>
              <dt>版权方</dt>
              <dd>{{ detail.resource.rightsOwner || "待补充" }}</dd>
              <dt>联系人</dt>
              <dd>{{ detail.resource.contact || "待补充" }}</dd>
              <dt>覆盖市场</dt>
              <dd>{{ arrayValue(detail.resource.markets).join(" / ") }}</dd>
              <dt>目标受众</dt>
              <dd>{{ detail.resource.audience || "待补充" }}</dd>
              <dt>合作状态</dt>
              <dd>{{ detail.resource.cooperationStatus }}</dd>
              <template v-for="field in profileFields" :key="field.key">
                <dt>{{ field.label }}</dt>
                <dd>
                  {{
                    objectValue(detail.resource.profile)[field.key] || "待补充"
                  }}
                </dd>
              </template>
            </dl>
          </div>
          <div class="ip-card">
            <h2>合作建议与授权</h2>
            <p>
              {{
                objectValue(detail.resource.profile).cooperationTips ||
                "暂无合作建议"
              }}
            </p>
            <h3>授权范围与限制</h3>
            <p>{{ detail.resource.licenseNotes || "待版权方补充" }}</p>
          </div>
          <div class="ip-card">
            <h2>资料维护</h2>
            <p class="ip-muted">最近更新：{{ detail.resource.updatedAt }}</p>
          </div>
        </div>
      </div>
    </template>

    <template v-else-if="mode === 'form'">
      <div v-if="!form.id" class="ip-tabs">
        <button
          :class="{ active: activeTab === 'single' }"
          @click="activeTab = 'single'"
        >
          单条录入 <small>适合补充少量IP</small></button
        ><button
          :class="{ active: activeTab === 'bulk' }"
          @click="activeTab = 'bulk'"
        >
          批量导入 <small>适合集中 sourcing</small>
        </button>
      </div>
      <template v-if="activeTab === 'single'">
        <div class="ip-detail-grid">
          <div class="ip-stack">
            <div class="ip-card">
              <h2>基础信息</h2>
              <div class="ip-form-grid ip-basic-grid">
                <label
                  >IP名称 *<el-input
                    v-model="form.name"
                    placeholder="请输入IP名称"
                    maxlength="255"
                /></label>
                <label
                  >IP类型 *<el-select v-model="form.ipType" placeholder="请选择"
                    ><el-option
                      v-for="type in types"
                      :key="type"
                      :label="type"
                      :value="type" /></el-select
                ></label>
                <label
                  >版权方<el-input
                    v-model="form.rightsOwner"
                    placeholder="版权方名称"
                /></label>
                <label
                  >联系人<el-input
                    v-model="form.contact"
                    placeholder="联系人或联系方式"
                /></label>
                <label
                  >覆盖市场 *<el-select
                    v-model="form.markets"
                    multiple
                    filterable
                    allow-create
                    default-first-option
                    placeholder="请选择市场"
                    ><el-option
                      v-for="market in markets"
                      :key="market"
                      :label="market"
                      :value="market" /></el-select
                ></label>
                <label
                  >合作状态<el-select v-model="form.cooperationStatus"
                    ><el-option
                      v-for="status in statuses"
                      :key="status"
                      :label="status"
                      :value="status" /></el-select
                ></label>
                <label
                  >目标受众<el-input
                    v-model="form.audience"
                    placeholder="例如：18–30岁年轻人"
                /></label>
                <label class="ip-span"
                  >IP简介<el-input
                    v-model="form.summary"
                    type="textarea"
                    :rows="3"
                    placeholder="背景、特点和影响力"
                /></label>
              </div>
            </div>
            <div class="ip-card">
              <h2>商业与授权</h2>
              <div class="ip-form-grid">
                <label
                  >基础权益价格范围（参考）
                  <div class="ip-money">
                    <el-select v-model="form.currency"
                      ><el-option label="CNY ¥" value="CNY" /><el-option
                        label="USD $"
                        value="USD" /><el-option
                        label="EUR €"
                        value="EUR" /></el-select
                    ><el-input-number
                      v-model="form.priceMin"
                      :min="0"
                      :controls="false"
                      placeholder="最低价"
                    /><span>—</span
                    ><el-input-number
                      v-model="form.priceMax"
                      :min="0"
                      :controls="false"
                      placeholder="最高价"
                    />
                  </div>
                  <small>供参考，根据具体需求更新</small></label
                ><label
                  >授权范围与限制<el-input
                    v-model="form.licenseNotes"
                    type="textarea"
                    :rows="3"
                    placeholder="授权地域、渠道或使用限制"
                /></label>
              </div>
            </div>
            <div class="ip-card">
              <h2>资料附件</h2>
              <div class="ip-form-grid">
                <label
                  >IP形象（PNG / JPG / WebP）<span class="ip-upload"
                    ><input
                      type="file"
                      accept="image/png,image/jpeg,image/webp"
                      @change="visualChanged"
                    />{{ visualFile?.name || "选择图片" }}</span
                  ></label
                >
                <label
                  >版权介绍文件（PDF）<span class="ip-upload"
                    ><input
                      type="file"
                      accept="application/pdf,.pdf"
                      multiple
                      @change="pdfChanged($event, 'copyright')"
                    />选择PDF文件</span
                  ><small v-for="file in copyrightFiles" :key="file.name">{{
                    file.name
                  }}</small></label
                ><label
                  >历史合作结案文件（选填）<el-input
                    v-model="caseTitle"
                    placeholder="历史合作名称"
                  /><el-input
                    v-model="caseSummary"
                    placeholder="案例概述"
                  /><span class="ip-upload"
                    ><input
                      type="file"
                      accept="application/pdf,.pdf"
                      @change="pdfChanged($event, 'case')"
                    />{{ caseFile?.name || "选择结案PDF" }}</span
                  ></label
                >
              </div>
            </div>
            <div class="ip-card ip-optional-profile">
              <el-collapse>
                <el-collapse-item name="profile">
                  <template #title>
                    <div>
                      <h2>补充档案信息（选填）</h2>
                      <p>完善受众、影响力和合作建议，便于后续需求匹配。</p>
                    </div>
                  </template>
                  <div class="ip-form-grid">
                    <label v-for="field in profileFields" :key="field.key">
                      {{ field.label }}
                      <el-input
                        v-model="form.profile[field.key]"
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
              <h2>录入提示</h2>
              <p>填写名称、类型和覆盖市场后即可创建。</p>
              <p>价格仅作参考，以最终授权报价为准。</p>
              <p>上传的PDF会显示在IP详情页。</p>
            </div>
            <div class="ip-card">
              <h2>历史合作</h2>
              <div
                v-for="item in detail?.cases || []"
                :key="item.id"
                class="ip-case"
              >
                <strong>{{ item.title }}</strong>
                <p>{{ item.summary }}</p>
              </div>
              <p v-if="!detail?.cases?.length" class="ip-muted">
                暂无历史合作案例，可在左侧补充结案资料。
              </p>
            </div>
          </div>
        </div>
        <div class="ip-bottom">
          <el-button v-if="!form.id" @click="saveManualDraft"
            >保存草稿</el-button
          ><el-button v-else @click="goList">取消</el-button
          ><el-button
            class="ip-primary"
            :loading="saving"
            @click="saveManual"
            >{{ form.id ? "保存修改" : "创建IP" }}</el-button
          >
        </div>
      </template>
      <template v-else>
        <div class="ip-import-top">
          <div class="ip-card ip-import-step">
            <span class="ip-step">1</span>
            <div class="ip-import-main">
              <div>
                <h2>下载模板</h2>
                <p>先下载 Excel 模板，按要求填写后再上传文件。</p>
                <el-button @click="downloadTemplate"
                  >下载IP导入模板.xlsx</el-button
                >
              </div>
              <div class="ip-template-fields">
                <strong>模板主要字段</strong>
                <div>
                  <span>IP名称 *</span><span>IP类型 *</span>
                  <span>覆盖市场 *</span><span>版权方</span>
                  <span>合作状态</span><span>参考价格范围</span>
                  <span>目标受众</span><span>联系人</span>
                </div>
                <small>一行一个IP，价格填写参考范围。</small>
              </div>
            </div>
          </div>
          <div class="ip-card ip-import-rules">
            <h2>导入规则</h2>
            <p>1. 按名称与版权方检查重复</p>
            <p>2. 错误行修正后可重新校验</p>
            <p>3. 已有IP不会被覆盖</p>
          </div>
        </div>
        <div class="ip-card ip-import-step">
          <span class="ip-step">2</span>
          <div>
            <h2>上传文件</h2>
            <p>
              支持 .xlsx /
              .xls，最多500条。PDF为可选；文件名使用“导入编号_版权介绍.pdf”或“导入编号_结案.pdf”，导入后自动关联。
            </p>
            <div class="ip-batch-upload">
              <label class="ip-upload ip-upload-drop"
                ><input
                  type="file"
                  accept=".xlsx,.xls"
                  @change="readImport"
                /><strong>选择Excel文件</strong
                ><small>{{
                  importFileName || "点击上传已填写的导入模板"
                }}</small></label
              ><label class="ip-upload ip-upload-drop"
                ><input
                  type="file"
                  accept="application/pdf,.pdf"
                  multiple
                  @change="readBatchFiles"
                /><strong>添加PDF附件（可选）</strong
                ><small>{{
                  batchFiles.length
                    ? `已选${batchFiles.length}份PDF`
                    : "按模板中的导入编号关联，可一次上传多份"
                }}</small></label
              >
            </div>
          </div>
        </div>
        <div v-loading="importLoading" class="ip-card ip-import-step">
          <span class="ip-step">3</span>
          <div class="ip-import-body">
            <div class="ip-card-head">
              <div>
                <h2>校验预览</h2>
                <p>导入前检查必填项、价格范围和重复IP。</p>
              </div>
              <strong v-if="importPreview.length"
                >共{{ importPreview.length }}条 · 可导入{{ readyCount }}条 ·
                重复{{ duplicateCount }}条 · 错误{{ errorCount }}条</strong
              >
            </div>
            <el-table
              :data="importPreview"
              max-height="340"
              empty-text="上传模板后显示校验结果"
              ><el-table-column
                prop="row"
                label="行号"
                width="80" /><el-table-column
                prop="name"
                label="IP名称"
                min-width="180" /><el-table-column label="校验结果" width="120"
                ><template #default="scope"
                  ><el-tag
                    :type="
                      scope.row.status === 'ready'
                        ? 'success'
                        : scope.row.status === 'duplicate'
                          ? 'warning'
                          : 'danger'
                    "
                    >{{
                      scope.row.status === "ready"
                        ? "通过"
                        : scope.row.status === "duplicate"
                          ? "疑似重复"
                          : "字段错误"
                    }}</el-tag
                  ></template
                ></el-table-column
              ><el-table-column prop="message" label="问题" min-width="220"
            /></el-table>
          </div>
        </div>
        <div class="ip-bottom">
          <el-button @click="goList">取消</el-button
          ><el-button
            class="ip-primary"
            :loading="importLoading"
            :disabled="!readyCount"
            @click="runImport"
            >导入{{ readyCount }}条IP</el-button
          >
        </div>
      </template>
    </template>
    <el-dialog v-model="previewVisible" title="PDF预览" width="80%" top="5vh"
      ><iframe
        v-if="previewURL"
        :src="previewURL"
        class="ip-pdf"
        title="PDF预览"
    /></el-dialog>
  </div>
</template>

<style scoped>
.ip-batch-upload {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-top: 12px;
}

.ip-upload-drop {
  display: flex !important;
  flex-direction: column;
  justify-content: center;
  width: 100% !important;
  min-height: 88px;
  padding: 14px !important;
  text-align: center;
  border-color: #cfd6c9 !important;
}

.ip-upload-drop small {
  margin-top: 5px;
  font-weight: 400;
  color: #8a9195;
}

.ip-page {
  min-height: calc(100vh - 150px);
  padding: 16px 28px 36px;
  color: #17191d;
  background: #f8f8f6;
}

.ip-head,
.ip-card-head,
.ip-filter,
.ip-actions,
.ip-bottom {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.ip-head {
  margin-bottom: 14px;
}

.ip-head h1 {
  margin: 5px 0;
  font-size: 30px;
  font-weight: 750;
}

.ip-head p,
.ip-card p {
  margin: 4px 0;
  color: #777e86;
}

.ip-eyebrow {
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

.ip-card,
.ip-stat {
  background: white;
  border: 1px solid #dedfdb;
  border-radius: 9px;
  box-shadow: 0 2px 10px #20240b06;
}

.ip-card {
  padding: 18px;
  margin-bottom: 16px;
}

.ip-card h2 {
  margin: 0 0 11px;
  font-size: 18px;
}

.ip-card h3 {
  margin: 22px 0 8px;
  font-size: 15px;
}

.ip-stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 18px;
}

.ip-stat {
  display: flex;
  gap: 16px;
  align-items: center;
  min-height: 90px;
  padding: 18px 22px;
}

.ip-stat-icon {
  display: grid;
  flex: none;
  place-items: center;
  width: 48px;
  height: 48px;
  font-size: 25px;
  color: #17191d;
  background: #f7f8f3;
  border-radius: 50%;
}

.ip-stat span {
  display: block;
  color: #777e86;
}

.ip-stat strong {
  font-size: 30px;
  line-height: 1.4;
}

.ip-filter {
  flex-wrap: wrap;
  justify-content: flex-start;
  margin-bottom: 18px;
}

.ip-filter .el-input {
  max-width: 350px;
}

.ip-filter .el-select {
  width: 170px;
}

.ip-advanced-filter {
  padding: 12px;
  margin-top: -8px;
  background: #f8faf2;
  border: 1px solid #e8eddc;
  border-radius: 7px;
}

.ip-link {
  padding: 0;
  font-weight: 700;
  color: #15181e;
  cursor: pointer;
  background: none;
  border: 0;
}

.ip-link:hover {
  color: #698000;
}

.ip-link + small,
.ip-file small {
  display: block;
  margin-top: 4px;
  color: #9b9fa4;
}

.ip-name-cell {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
}

.ip-name-cell img,
.ip-name-empty {
  flex: none;
  width: 58px;
  height: 62px;
  border-radius: 6px;
}

.ip-name-cell img {
  object-fit: cover;
}

.ip-name-empty {
  display: grid;
  place-items: center;
  font-size: 13px;
  font-weight: 800;
  color: #6e7d34;
  background: #edf4d8;
}

.ip-name-cell > div:last-child {
  min-width: 0;
}

.ip-name-cell small {
  display: -webkit-box;
  overflow: hidden;
  color: #8c9296;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.ip-muted {
  font-size: 12px !important;
  color: #90969b;
}

.ip-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

.ip-detail-grid {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(280px, 1fr);
  gap: 16px;
}

.ip-hero {
  display: grid;
  grid-template-columns: minmax(230px, 36%) minmax(0, 1fr);
  gap: 28px;
  min-height: 220px;
}

.ip-hero-media {
  position: relative;
  min-height: 185px;
}

.ip-hero-media img,
.ip-hero-empty {
  width: 100%;
  height: 100%;
  max-height: 250px;
  border-radius: 6px;
}

.ip-hero-media img {
  object-fit: cover;
}

.ip-hero-empty {
  display: grid;
  place-items: center;
  color: #8e9697;
  background: #f0f1ed;
  border: 1px dashed #c9cec6;
}

.ip-hero-upload {
  position: absolute;
  right: 10px;
  bottom: 10px;
  padding: 6px 10px;
  font-size: 12px;
  font-weight: 700;
  color: #161a1d;
  cursor: pointer;
  background: #fff;
  border-radius: 5px;
}

.ip-hero-copy {
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.ip-hero-copy > strong {
  font-size: 20px;
}

.ip-hero-copy p {
  max-width: 750px;
  margin: 12px 0 18px;
  line-height: 1.8;
}

.ip-hero-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.ip-stack {
  min-width: 0;
}

.ip-price {
  font-size: 29px;
  font-weight: 800;
  color: #324e00;
}

.ip-dl {
  display: grid;
  grid-template-columns: 100px 1fr;
  gap: 14px;
}

.ip-dl dt {
  color: #858b91;
}

.ip-dl dd {
  margin: 0;
}

.ip-file {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 11px 13px;
  margin-top: 10px;
  border: 1px solid #e8e9e3;
  border-radius: 8px;
}

.ip-file span {
  min-width: 0;
  overflow-wrap: anywhere;
}

.ip-file-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 32px;
  margin-right: 10px;
  font-size: 10px;
  color: #fff;
  vertical-align: middle;
  background: #e94340;
  border-radius: 4px;
}

.ip-drawer-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 70vh;
  padding: 0 4px 22px;
}

.ip-drawer-intro {
  display: grid;
  grid-template-columns: 98px 1fr;
  gap: 14px;
  align-items: start;
}

.ip-drawer-intro img {
  width: 98px;
  height: 112px;
  object-fit: cover;
  border-radius: 6px;
}

.ip-drawer-intro h2 {
  margin: 0 0 8px;
  font-size: 17px;
}

.ip-drawer-content > p {
  margin: 0;
  line-height: 1.7;
  color: #646c72;
}

.ip-drawer-section {
  padding-top: 14px;
  border-top: 1px solid #e8e9e3;
}

.ip-drawer-section h3 {
  margin: 0 0 12px;
  font-size: 15px;
}

.ip-drawer-section .ip-dl {
  gap: 10px;
  font-size: 13px;
}

.ip-drawer-action {
  width: 100%;
  margin-top: auto;
}

.ip-case {
  padding: 14px;
  margin-bottom: 12px;
  border: 1px solid #eceee8;
  border-radius: 9px;
}

.ip-case-form {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 16px;
}

.ip-case-form .el-button {
  width: max-content;
}

.ip-upload-inline {
  font-weight: 600;
  color: #668000;
  cursor: pointer;
}

.ip-tabs {
  display: flex;
  gap: 0;
  margin-bottom: 12px;
}

.ip-tabs button {
  min-width: 220px;
  padding: 9px 28px;
  font-weight: 700;
  cursor: pointer;
  background: #fff;
  border: 1px solid #dbded6;
}

.ip-tabs button:first-child {
  border-radius: 9px 0 0 9px;
}

.ip-tabs button:last-child {
  border-radius: 0 9px 9px 0;
}

.ip-tabs button.active {
  background: #efffc3;
  border-color: #caff00;
}

.ip-tabs small {
  display: block;
  font-weight: 400;
  color: #8a9195;
}

.ip-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 24px;
}

.ip-optional-profile :deep(.el-collapse) {
  border: 0;
}

.ip-optional-profile :deep(.el-collapse-item__header) {
  height: auto;
  min-height: 48px;
  line-height: 1.5;
  border: 0;
}

.ip-optional-profile :deep(.el-collapse-item__wrap) {
  border: 0;
}

.ip-optional-profile :deep(.el-collapse-item__content) {
  padding: 18px 0 0;
}

.ip-optional-profile h2 {
  margin: 0 0 4px;
}

.ip-optional-profile p {
  margin: 0;
  font-weight: 400;
  color: #8a9195;
}

.ip-form-grid label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-weight: 600;
}

.ip-basic-grid label {
  display: grid;
  grid-template-columns: 104px minmax(0, 1fr);
  gap: 12px;
  align-items: center;
}

.ip-basic-grid label.ip-span {
  align-items: start;
}

.ip-basic-grid label.ip-span > .el-textarea {
  width: 100%;
}

.ip-form-grid label small {
  font-weight: 400;
  color: #969b9e;
}

.ip-span {
  grid-column: 1/-1;
}

.ip-money {
  display: flex;
  gap: 8px;
  align-items: center;
}

.ip-money .el-select {
  flex: none;
  width: 120px;
}

.ip-money .el-input-number {
  width: 100%;
  min-width: 0;
}

.ip-upload {
  position: relative;
  display: inline-flex;
  align-items: center;
  width: max-content;
  padding: 9px 14px;
  font-weight: 600;
  color: #536d00;
  cursor: pointer;
  border: 1px dashed #aeb9a4;
  border-radius: 7px;
}

.ip-upload input {
  position: absolute;
  inset: 0;
  cursor: pointer;
  opacity: 0;
}

.ip-bottom {
  position: sticky;
  bottom: 0;
  z-index: 5;
  justify-content: flex-end;
  padding: 12px 20px;
  margin-top: 16px;
  background: #fffffff2;
  border-top: 1px solid #e6e8e3;
}

.ip-import-step {
  display: flex;
  gap: 18px;
}

.ip-import-main {
  display: grid;
  flex: 1;
  grid-template-columns: minmax(0, 1fr) minmax(260px, 1fr);
  gap: 18px;
}

.ip-template-fields {
  padding: 14px;
  background: #f7f8f5;
  border-radius: 8px;
}

.ip-template-fields > div {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin: 12px 0;
  font-size: 12px;
}

.ip-template-fields span::before {
  margin-right: 5px;
  color: #80a000;
  content: "✓";
}

.ip-template-fields small {
  color: #8a9195;
}

.ip-import-top {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(260px, 1fr);
  gap: 16px;
}

.ip-import-rules {
  background: #fafff0;
  border-color: #e4efc9;
}

.ip-import-rules p {
  padding: 9px 0;
  border-bottom: 1px solid #e4efc9;
}

.ip-step {
  display: grid;
  flex: none;
  place-items: center;
  width: 34px;
  height: 34px;
  font-weight: 800;
  background: #caff00;
  border-radius: 50%;
}

.ip-import-body {
  width: 100%;
  min-width: 0;
}

.ip-pdf {
  width: 100%;
  height: 70vh;
  border: 0;
}

.ip-visual {
  display: block;
  max-width: 100%;
  max-height: 420px;
  object-fit: contain;
  border-radius: 8px;
}

@media (width <= 1100px) {
  .ip-detail-grid {
    grid-template-columns: 1fr;
  }

  .ip-import-top {
    grid-template-columns: 1fr;
  }

  .ip-hero {
    grid-template-columns: 1fr 1fr;
  }

  .ip-stats {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (width <= 700px) {
  .ip-page {
    padding: 16px;
  }

  .ip-head,
  .ip-filter {
    flex-wrap: wrap;
  }

  .ip-form-grid,
  .ip-case-form,
  .ip-stats,
  .ip-hero,
  .ip-import-main,
  .ip-batch-upload {
    grid-template-columns: 1fr;
  }

  .ip-tabs button {
    flex: 1;
    min-width: 0;
  }

  .ip-filter .el-input,
  .ip-filter .el-select {
    width: 100%;
    max-width: none;
  }
}
</style>
