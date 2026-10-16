<script setup lang="ts">
import { canonicalIPImportHeader, canonicalIPOption } from "@/utils/ipI18n";
import { useIPLocale } from "@/utils/ipI18n";
const { ipText, locale } = useIPLocale();
import IPTranslationNotice from "../components/IPTranslationNotice.vue";
import {
  translatedIPRecord,
  applyIPTranslationUpdates,
  type IPTranslationUpdate
} from "@/utils/ipContentTranslation";
import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useUserStoreHook } from "@/store/modules/user";
import { ElMessage, ElMessageBox } from "element-plus";
import { formatBytes } from "@pureadmin/utils";
import * as XLSX from "xlsx";
import VuePdfEmbed from "vue-pdf-embed";
import {
  IP_DOCUMENT_TIMEOUT_MS,
  deleteIPFile,
  getIPResource,
  importIPResources,
  listIPResources,
  previewIPImport,
  readIPFile,
  renameIPFile,
  saveIPCase,
  saveIPResource,
  uploadIPFile,
  type IPResourceInput
} from "@/api/ip";

const showOriginal = ref(false);

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
const legacyTypes = ["体育IP", "动画/影视", "音乐/活动", "电竞"];
const filterTypes = [...types, ...legacyTypes];
const statuses = ["可合作", "洽谈中", "待评估", "暂不可合作"];
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
  marketPenetrations: {},
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
const formTypes = computed(() =>
  form.value.ipType && !types.includes(form.value.ipType)
    ? [...types, form.value.ipType]
    : types
);
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
  ElMessage.success(ipText("文字草稿已保存在当前浏览器，附件需重新选择"));
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
      marketPenetrations: penetrationValues(saved.form.marketPenetrations),
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
const previewFileName = ref("");
const previewPageCount = ref(0);
const previewLoading = ref(false);
const previewDownloading = ref(false);
const previewDownloaded = ref(0);
const previewTotal = ref(0);
const previewError = ref("");
const previewFile = ref<any>(null);
let previewRequestId = 0;
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
    ? ipText("正在下载 PDF · {0} / {1}（{2}%）", [
        formatBytes(previewDownloaded.value),
        formatBytes(previewTotal.value),
        previewDownloadPercent.value
      ])
    : previewDownloaded.value > 0
      ? ipText("正在下载 PDF · 已下载 {0}", [
          formatBytes(previewDownloaded.value)
        ])
      : "正在连接并下载 PDF…"
);
const importFileName = ref("");
const importRows = ref<IPResourceInput[]>([]);
const importPreview = ref<any[]>([]);
const importLoading = ref(false);
const importKeys = ref<string[]>([]);
const importExcelRows = ref<number[]>([]);
const importCaseTitles = ref<string[]>([]);
const importCaseSummaries = ref<string[]>([]);
const importLocalIssues = ref<string[]>([]);
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
function penetrationValues(value: unknown): Record<string, number> {
  let parsed = value;
  if (typeof value === "string") {
    try {
      parsed = JSON.parse(value);
    } catch {
      return {};
    }
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
  const result: Record<string, number> = {};
  for (const [market, amount] of Object.entries(parsed)) {
    if (amount === null || amount === "") continue;
    const numeric = Number(amount);
    if (Number.isFinite(numeric)) result[market] = numeric;
  }
  return result;
}
function marketCoverageText(item: any) {
  const penetrations = penetrationValues(item.marketPenetrations);
  return arrayValue(item.markets)
    .map(market =>
      penetrations[market] === undefined
        ? market
        : `${market} ${penetrations[market]}%`
    )
    .join(" / ");
}
function setMarketPenetration(market: string, value: number | undefined) {
  const penetrations = (form.value.marketPenetrations ||= {});
  if (value == null || !Number.isFinite(value)) delete penetrations[market];
  else penetrations[market] = value;
}
function syncMarketPenetrations() {
  form.value.marketPenetrations = Object.fromEntries(
    Object.entries(form.value.marketPenetrations || {}).filter(([market]) =>
      form.value.markets.includes(market)
    )
  );
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
    ElMessage.error(ipText("IP档案加载失败，请重试"));
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
  else ElMessage.warning(ipText("最多选择5个意向IP"));
}
function confirmForRequest() {
  const key = String(route.query.selectionKey || "");
  if (!key) return;
  localStorage.setItem(key, JSON.stringify(selectedForRequest.value));
  ElMessage.success(ipText("已加入需求意向清单，请返回需求页面"));
  window.close();
}
const displayedRows = computed(() =>
  rows.value.map(row => translatedIPRecord(row, locale.value))
);
const displayDetail = computed(() =>
  detail.value
    ? {
        ...detail.value,
        resource: translatedIPRecord(
          detail.value.resource,
          locale.value,
          showOriginal.value
        ),
        cases: detail.value.cases.map(row =>
          translatedIPRecord(row, locale.value, showOriginal.value)
        )
      }
    : null
);
const displayDrawerDetail = computed(() =>
  drawerDetail.value
    ? {
        ...drawerDetail.value,
        resource: translatedIPRecord(
          drawerDetail.value.resource,
          locale.value,
          showOriginal.value
        )
      }
    : null
);
function updateContentTranslations(updates: IPTranslationUpdate[]) {
  if (!detail.value) return;
  applyIPTranslationUpdates([detail.value.resource], "ip_resource", updates);
  applyIPTranslationUpdates(detail.value.cases, "ip_case", updates);
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
    marketPenetrations: penetrationValues(item.marketPenetrations),
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
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files || []);
  input.value = "";
  const invalid = files.some(
    file =>
      !file.name.toLowerCase().endsWith(".pdf") || file.size > 20 * 1024 * 1024
  );
  if (invalid) {
    ElMessage.error(ipText("仅支持20MB以内的PDF文件"));
    return;
  }
  if (kind === "copyright") {
    const selectedNames = new Set(
      copyrightFiles.value.map(file => file.name.toLocaleLowerCase())
    );
    const additions = files.filter(file => {
      const key = file.name.toLocaleLowerCase();
      if (selectedNames.has(key)) return false;
      selectedNames.add(key);
      return true;
    });
    copyrightFiles.value.push(...additions);
    if (additions.length !== files.length)
      ElMessage.warning(
        ipText("同名版权文件已跳过；如需添加，请先重命名已选文件")
      );
  } else if (files[0]) {
    caseFile.value = files[0];
  }
}
function visualChanged(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  if (
    !["image/png", "image/jpeg", "image/webp"].includes(file.type) ||
    file.size > 5 * 1024 * 1024
  ) {
    ElMessage.warning(ipText("IP形象仅支持5MB以内的PNG、JPG或WebP图片"));
    return;
  }
  visualFile.value = file;
}
function selectedFileExtension(file: File) {
  return file.name.match(/\.[^.]+$/)?.[0] || "";
}
async function askFileName(currentName: string, extension: string) {
  try {
    const { value } = await ElMessageBox.prompt(
      ipText("请输入新的文件名"),
      ipText("重命名文件"),
      {
        inputValue: currentName,
        confirmButtonText: ipText("保存"),
        cancelButtonText: ipText("取消"),
        inputValidator: value => {
          const name = String(value || "").trim();
          const finalName = name.toLowerCase().endsWith(extension.toLowerCase())
            ? name
            : `${name}${extension}`;
          if (
            !name ||
            !finalName.slice(0, -extension.length).trim() ||
            new TextEncoder().encode(finalName).length > 255 ||
            /[\\/\x00-\x1f\x7f]/.test(name)
          )
            return "文件名需为1至255字节，且不能包含斜杠或控制字符";
          if (
            /\.[^./\\]+$/.test(name) &&
            !name.toLowerCase().endsWith(extension.toLowerCase())
          )
            return `请保留 ${extension} 文件格式`;
          return true;
        }
      }
    );
    const name = String(value).trim();
    return name.toLowerCase().endsWith(extension.toLowerCase())
      ? name
      : `${name}${extension}`;
  } catch {
    return "";
  }
}
async function renameSelectedFile(
  kind: "copyright" | "visual" | "case",
  index = 0
) {
  const file =
    kind === "copyright"
      ? copyrightFiles.value[index]
      : kind === "visual"
        ? visualFile.value
        : caseFile.value;
  if (!file) return;
  const name = await askFileName(file.name, selectedFileExtension(file));
  if (!name || name === file.name) return;
  if (
    kind === "copyright" &&
    copyrightFiles.value.some(
      (item, itemIndex) =>
        itemIndex !== index &&
        item.name.toLocaleLowerCase() === name.toLocaleLowerCase()
    )
  ) {
    ElMessage.warning(ipText("已选择同名版权文件，请使用不同文件名"));
    return;
  }
  const renamed = new File([file], name, {
    type: file.type,
    lastModified: file.lastModified
  });
  if (kind === "copyright") copyrightFiles.value[index] = renamed;
  else if (kind === "visual") visualFile.value = renamed;
  else caseFile.value = renamed;
}
async function renameStoredFile(file: any) {
  const originalName = String(file.originalName || "");
  const extension = originalName.match(/\.[^.]+$/)?.[0] || "";
  const name = await askFileName(originalName, extension);
  if (!name || name === originalName) return;
  await renameIPFile(Number(file.id), name);
  if (detail.value) await loadDetail(Number(detail.value.resource.id));
  ElMessage.success(ipText("文件名已更新"));
}
function removeSelectedFile(kind: "copyright" | "visual" | "case", index = 0) {
  if (kind === "copyright") copyrightFiles.value.splice(index, 1);
  else if (kind === "visual") visualFile.value = null;
  else caseFile.value = null;
}
async function addVisual(event: Event) {
  visualChanged(event);
  if (!visualFile.value || !detail.value) return;
  const id = Number(detail.value.resource.id);
  const file = visualFile.value;
  visualFile.value = null;
  await uploadIPFile(id, "visual", file);
  await loadDetail(id);
  ElMessage.success(ipText("IP形象已上传"));
}
async function saveManual() {
  const item = form.value;
  if (!item.name.trim() || !item.ipType || !item.markets.length) {
    ElMessage.warning(ipText("请填写IP名称、类型和覆盖市场"));
    return;
  }
  const marketPenetrations = Object.fromEntries(
    Object.entries(item.marketPenetrations || {}).filter(([market]) =>
      item.markets.includes(market)
    )
  );
  if (
    Object.values(marketPenetrations).some(
      value => !Number.isFinite(value) || value < 0 || value > 100
    )
  ) {
    ElMessage.warning(ipText("市场渗透率请输入0至100之间的数值"));
    return;
  }
  if (
    item.priceMin != null &&
    item.priceMax != null &&
    item.priceMin > item.priceMax
  ) {
    ElMessage.warning(ipText("价格下限不能高于上限"));
    return;
  }
  if (caseFile.value && !caseTitle.value.trim()) {
    ElMessage.warning(ipText("上传结案文件前请填写历史合作名称"));
    return;
  }
  saving.value = true;
  try {
    const result = await saveIPResource({ ...item, marketPenetrations });
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
    ElMessage.success(
      ipText(item.id ? ipText("IP资料已更新") : ipText("IP已创建"))
    );
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
    ElMessage.warning(ipText("请填写合作案例名称"));
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
    ElMessage.success(ipText("合作案例已添加"));
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
    ElMessage.error(ipText("仅支持20MB以内的PDF文件"));
    return;
  }
  saving.value = true;
  try {
    const id = Number(detail.value.resource.id);
    for (const file of files) await uploadIPFile(id, "copyright", file);
    await loadDetail(id);
    ElMessage.success(ipText("版权资料已上传"));
  } finally {
    saving.value = false;
  }
}
async function openFile(file: any, download = false) {
  const requestId = download ? 0 : ++previewRequestId;
  if (!download) {
    if (previewURL.value) URL.revokeObjectURL(previewURL.value);
    previewURL.value = "";
    previewFile.value = file;
    previewFileName.value = String(file.originalName || "IP资料.pdf");
    previewPageCount.value = 0;
    previewLoading.value = false;
    previewDownloading.value = true;
    previewDownloaded.value = 0;
    previewTotal.value = 0;
    previewError.value = "";
    previewVisible.value = true;
  }
  try {
    const blob = await readIPFile(Number(file.id), {
      timeout: IP_DOCUMENT_TIMEOUT_MS,
      onDownloadProgress: download
        ? undefined
        : progress => {
            if (requestId !== previewRequestId || !previewVisible.value) return;
            previewDownloaded.value = progress.loaded;
            if (progress.lengthComputable && progress.total > 0)
              previewTotal.value = progress.total;
          }
    });
    if (!download && (requestId !== previewRequestId || !previewVisible.value))
      return;
    const url = URL.createObjectURL(
      new Blob([blob], { type: "application/pdf" })
    );
    if (download) {
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = file.originalName;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 60000);
    } else {
      previewURL.value = url;
      previewDownloading.value = false;
      previewLoading.value = true;
    }
  } catch {
    if (download) {
      ElMessage.error(ipText("文件下载失败，请稍后重试"));
    } else if (requestId === previewRequestId && previewVisible.value) {
      previewDownloading.value = false;
      previewError.value = "文件下载失败，请检查网络后重试";
    }
  }
}
function pdfLoaded(doc: { numPages: number }) {
  previewPageCount.value = doc.numPages;
}
function pdfFailed() {
  previewLoading.value = false;
  previewError.value = "PDF无法预览，请下载文件查看";
}
async function removeFile(file: any) {
  await ElMessageBox.confirm(
    ipText("删除「{0}」？", [file.originalName]),
    ipText("确认删除"),
    {
      type: "warning"
    }
  );
  await deleteIPFile(Number(file.id));
  if (detail.value) await loadDetail(Number(detail.value.resource.id));
  ElMessage.success(ipText("文件已删除"));
}
function downloadTemplate() {
  const anchor = document.createElement("a");
  anchor.href = `${import.meta.env.BASE_URL}${locale.value.startsWith("en") ? "IP-import-template.en.xlsx" : "IP导入模板.xlsx"}`;
  anchor.download = ipText("IP导入模板.xlsx");
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}
function splitImportMarkets(value: unknown) {
  return String(value || "")
    .split(/[,，、;；/]+/)
    .map(market => canonicalIPOption(market))
    .filter(Boolean);
}
function parseImportMarkets(row: Record<string, any>) {
  const selected: string[] = [];
  const marketPenetrations: Record<string, number> = {};
  let issue = "";
  for (let index = 1; index <= 9; index++) {
    const choice =
      row[`覆盖市场${index}${index === 1 ? "*" : ""}`] ||
      row[`覆盖市场${index}`];
    const names = splitImportMarkets(choice);
    selected.push(...names);
    const rate = [
      row[`渗透率${index}（%）`],
      row[`渗透率${index}(%)`],
      row[`渗透率${index}%`]
    ].find(value => value != null && String(value).trim() !== "");
    if (rate === undefined) continue;
    if (names.length !== 1) {
      issue ||= names.length
        ? ipText("覆盖市场{0}有多个市场，请拆分后填写渗透率", [index])
        : ipText("渗透率{0}已填写，请先选择覆盖市场{1}", [index, index]);
      continue;
    }
    const text = String(rate)
      .trim()
      .replace(/[%％]$/, "")
      .trim();
    const amount = Number(text);
    if (!text || !Number.isFinite(amount) || amount < 0 || amount > 100) {
      issue ||= ipText("渗透率{0}应为0至100的数字", [index]);
      marketPenetrations[names[0]] = -1;
      continue;
    }
    marketPenetrations[names[0]] = amount;
  }
  selected.push(...splitImportMarkets(row["覆盖市场*"] || row["覆盖市场"]));
  return { markets: selected, marketPenetrations, issue };
}
function excelRow(backendRow: number) {
  return importExcelRows.value[Number(backendRow) - 2] || backendRow;
}
function importKeyForRow(backendRow: number) {
  return importKeys.value[Number(backendRow) - 2] || "—";
}
function importCoverageText(backendRow: number) {
  const item = importRows.value[Number(backendRow) - 2];
  return item ? marketCoverageText(item) : "—";
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
  importRows.value = [];
  importPreview.value = [];
  importExcelRows.value = [];
  importKeys.value = [];
  importLocalIssues.value = [];
  importFileName.value = "";
  try {
    const book = XLSX.read(await file.arrayBuffer());
    const sheet = book.Sheets[book.SheetNames[0]];
    const records = XLSX.utils
      .sheet_to_json<Record<string, any>>(sheet, {
        defval: ""
      })
      .map(record => {
        const row: Record<string, any> = Object.fromEntries(
          Object.entries(record).map(([key, value]) => [
            canonicalIPImportHeader(key),
            value
          ])
        );
        if (typeof record.__rowNum__ === "number")
          Object.defineProperty(row, "__rowNum__", {
            value: record.__rowNum__,
            enumerable: false
          });
        return row;
      });
    const headers =
      XLSX.utils.sheet_to_json<string[]>(sheet, {
        header: 1,
        range: 0,
        blankrows: false
      })[0] || [];
    const rateColumns = headers
      .map(header => canonicalIPImportHeader(String(header)))
      .flatMap((header, column) =>
        /^渗透率[1-9](（%）|\(%\)|%)$/.test(String(header))
          ? [{ header, column }]
          : []
      );
    records.forEach((row, index) => {
      const excelRow =
        typeof row.__rowNum__ === "number" ? row.__rowNum__ : index + 1;
      for (const { header, column } of rateColumns) {
        const displayed =
          sheet[XLSX.utils.encode_cell({ r: excelRow, c: column })]?.w;
        if (typeof displayed === "string" && /[%％]\s*$/.test(displayed))
          row[header] = displayed;
      }
    });
    if (!records.length || records.length > 500) {
      ElMessage.warning(ipText("每次请选择1至500条IP"));
      return;
    }
    importExcelRows.value = records.map((row, index) =>
      typeof row.__rowNum__ === "number" ? row.__rowNum__ + 1 : index + 2
    );
    importKeys.value = records.map(row => String(row["导入编号"] ?? "").trim());
    importCaseTitles.value = records.map(row =>
      String(row["历史合作名称"] || "").trim()
    );
    importCaseSummaries.value = records.map(row =>
      String(row["历史合作概述"] || "").trim()
    );
    importRows.value = records.map(row => {
      const marketInput = parseImportMarkets(row);
      importLocalIssues.value.push(marketInput.issue);
      return {
        name: String(row["IP名称*"] || row["IP名称"] || "").trim(),
        ipType: canonicalIPOption(
          String(row["IP类型*"] || row["IP类型"] || "")
        ),
        markets: marketInput.markets,
        marketPenetrations: marketInput.marketPenetrations,
        rightsOwner: String(row["版权方"] || "").trim(),
        contact: String(row["联系人"] || "").trim(),
        audience: String(row["目标受众"] || "").trim(),
        summary: String(row["IP简介"] || "").trim(),
        cooperationStatus: canonicalIPOption(
          String(row["合作状态"] || "待评估")
        ),
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
      };
    });
    importFileName.value = file.name;
    importLoading.value = true;
    importPreview.value = (
      (await previewIPImport(importRows.value)).data || []
    ).map((result, index) =>
      importLocalIssues.value[index]
        ? {
            ...result,
            status: "error",
            message: importLocalIssues.value[index]
          }
        : result
    );
  } catch {
    ElMessage.error(ipText("文件解析失败，请使用下载的模板"));
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
    ElMessage.error(ipText("仅支持20MB以内的PDF文件"));
    return;
  }
  batchFiles.value = files;
}
function batchFileKind(
  fileName: string,
  key: string
): "copyright" | "case" | "" {
  if (
    [`${key}_版权介绍`, `${key}_copyright`].some(prefix =>
      fileName.startsWith(prefix)
    ) &&
    fileName.toLowerCase().endsWith(".pdf")
  )
    return "copyright";
  if (
    [`${key}_结案`, `${key}_case`].some(prefix =>
      fileName.startsWith(prefix)
    ) &&
    fileName.toLowerCase().endsWith(".pdf")
  )
    return "case";
  return "";
}
async function runImport() {
  if (!readyCount.value) return;
  const readyIndexes = importPreview.value
    .filter(row => row.status === "ready")
    .map(row => Number(row.row) - 2);
  if (batchFiles.value.length) {
    const keys = importPreview.value
      .filter(row => row.status === "ready")
      .map(row => importKeys.value[Number(row.row) - 2]);
    if (keys.some(key => !key) || new Set(keys).size !== keys.length) {
      ElMessage.warning(
        ipText("批量关联PDF时，请为可导入行填写不重复的导入编号")
      );
      return;
    }
    if (
      batchFiles.value.some(
        file => !keys.some(key => batchFileKind(file.name, key))
      )
    ) {
      ElMessage.warning(ipText("有PDF文件名未匹配可导入行的编号或附件类型"));
      return;
    }
  }
  importLoading.value = true;
  try {
    const result = await importIPResources(
      readyIndexes.map(index => importRows.value[index])
    );
    let uploaded = 0;
    let failed = 0;
    for (const created of result.data.created || []) {
      const index = readyIndexes[Number(created.row) - 2];
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
      ipText("成功导入{0}条IP，关联{1}份PDF{2}", [
        result.data.imported,
        uploaded,
        failed ? `，${failed}份附件上传失败，请在详情页补充` : ""
      ])
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
  if (!open) {
    ++previewRequestId;
    previewDownloading.value = false;
    if (previewURL.value) {
      URL.revokeObjectURL(previewURL.value);
      previewURL.value = "";
    }
  }
});
onMounted(syncRoute);
onUnmounted(() => {
  ++previewRequestId;
  if (previewURL.value) URL.revokeObjectURL(previewURL.value);
  if (visualURL.value) URL.revokeObjectURL(visualURL.value);
  if (drawerVisualURL.value) URL.revokeObjectURL(drawerVisualURL.value);
  for (const url of Object.values(thumbUrls.value)) URL.revokeObjectURL(url);
});
</script>

<template>
  <div
    v-loading="loading"
    class="ip-page"
    :lang="locale === 'en' ? 'en' : 'zh-CN'"
  >
    <div class="ip-head">
      <div>
        <div v-if="mode !== 'list'" class="ip-eyebrow">
          {{ ipText("IP资源库 /") }}
          {{
            mode === "detail" ? displayDetail?.resource.name : ipText("新增IP")
          }}
        </div>
        <h1>
          {{
            mode === "list"
              ? ipText("IP资源库")
              : mode === "detail"
                ? displayDetail?.resource.name || ipText("IP详情")
                : ipText("新增IP")
          }}
        </h1>
        <p>
          {{
            ipText(
              mode === "list"
                ? ipText("统一沉淀IP资产，支持需求快速匹配")
                : mode === "detail"
                  ? ipText("版权资料、参考权益价格与历史合作")
                  : ipText("支持单条录入与模板批量导入")
            )
          }}
        </p>
      </div>
      <div class="ip-actions">
        <el-button v-if="mode !== 'list'" @click="goList">{{
          ipText("返回资源库")
        }}</el-button>
        <el-button v-if="mode === 'list'" class="ip-primary" @click="goNew()">{{
          ipText("＋ 新增IP")
        }}</el-button>
        <el-button
          v-if="mode === 'list' && selectingForRequest"
          class="ip-primary"
          @click="confirmForRequest"
          >{{ ipText("确认选择") }} {{ ipText(selectedForRequest.length) }}
          {{ ipText("个IP") }}</el-button
        >
        <el-button
          v-if="mode === 'detail'"
          class="ip-primary"
          @click="editDetail"
          >{{ ipText("编辑IP资料") }}</el-button
        >
      </div>
    </div>

    <p v-if="mode !== 'list' && mode !== 'detail'" class="ip-muted">
      {{ ipText("编辑保留录入原文，保存后自动生成另一语言译文") }}
    </p>
    <template v-if="mode === 'list'">
      <div class="ip-stats">
        <div class="ip-stat">
          <div class="ip-stat-icon">
            <IconifyIconOnline icon="ri:stack-line" />
          </div>
          <div>
            <span>{{ ipText("IP总数") }}</span
            ><strong>{{ ipText(stats.total) }}</strong>
          </div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-icon">
            <IconifyIconOnline icon="ri:group-line" />
          </div>
          <div>
            <span>{{ ipText("可合作") }}</span
            ><strong>{{ ipText(stats.cooperable) }}</strong>
          </div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-icon">
            <IconifyIconOnline icon="ri:file-list-3-line" />
          </div>
          <div>
            <span>{{ ipText("待版权资料") }}</span
            ><strong>{{ ipText(stats.pendingFiles) }}</strong>
          </div>
        </div>
        <div class="ip-stat">
          <div class="ip-stat-icon">
            <IconifyIconOnline icon="ri:line-chart-line" />
          </div>
          <div>
            <span>{{ ipText("近30天新增") }}</span
            ><strong>{{ ipText(stats.recent) }}</strong>
          </div>
        </div>
      </div>
      <div class="ip-card">
        <div class="ip-filter">
          <el-input
            v-model="filter.keyword"
            :placeholder="ipText('搜索IP名称、版权方或市场')"
            clearable
            @keyup.enter="loadList"
          />
          <el-select
            v-model="filter.ipType"
            popper-class="ip-option-popper"
            :placeholder="ipText('全部类型')"
            clearable
            ><el-option
              v-for="type in filterTypes"
              :key="type"
              :label="ipText(type)"
              :value="type"
          /></el-select>
          <el-select
            v-model="filter.status"
            popper-class="ip-option-popper"
            :placeholder="ipText('合作状态')"
            clearable
            ><el-option
              v-for="status in statuses"
              :key="status"
              :label="ipText(status)"
              :value="status"
          /></el-select>
          <el-select
            v-model="filter.market"
            popper-class="ip-option-popper"
            :placeholder="ipText('目标市场')"
            clearable
            filterable
          >
            <el-option
              v-for="market in markets"
              :key="market"
              :label="ipText(market)"
              :value="market"
            />
          </el-select>
          <el-button
            class="ip-primary"
            @click="
              filter.page = 1;
              loadList();
            "
            >{{ ipText("搜索") }}</el-button
          >
          <el-button @click="resetFilters">{{ ipText("重置") }}</el-button>
          <el-button link @click="advancedFilters = !advancedFilters">{{
            ipText(advancedFilters ? ipText("收起筛选") : ipText("高级筛选"))
          }}</el-button>
        </div>
        <div v-if="advancedFilters" class="ip-filter ip-advanced-filter">
          <el-select
            v-model="filter.completeness"
            popper-class="ip-option-popper"
            :placeholder="ipText('资料完成度')"
            clearable
          >
            <el-option :label="ipText('资料完整')" value="complete" />
            <el-option :label="ipText('待补充')" value="incomplete" />
          </el-select>
          <el-date-picker
            v-model="updatedRange"
            type="daterange"
            value-format="YYYY-MM-DD"
            :start-placeholder="ipText('更新起始')"
            :end-placeholder="ipText('更新截止')"
          />
        </div>
        <el-table
          class="ip-resource-table"
          :data="displayedRows"
          :empty-text="ipText('暂无IP，点击右上角新增')"
          stripe
        >
          <el-table-column
            v-if="selectingForRequest"
            :label="ipText('选择')"
            width="65"
          >
            <template #default="scope"
              ><el-checkbox
                :model-value="selectedForRequest.includes(Number(scope.row.id))"
                @change="toggleForRequest(Number(scope.row.id))"
            /></template>
          </el-table-column>
          <el-table-column prop="name" :label="ipText('IP名称')" min-width="260"
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
                    scope.row.summary ||
                    scope.row.rightsOwner ||
                    ipText("版权方待补充")
                  }}</small>
                </div>
              </div></template
            ></el-table-column
          >
          <el-table-column :label="ipText('类型')" min-width="160"
            ><template #default="scope"
              ><el-tag effect="light">{{
                ipText(scope.row.ipType)
              }}</el-tag></template
            ></el-table-column
          >
          <el-table-column :label="ipText('覆盖市场及渗透率')" min-width="220"
            ><template #default="scope">{{
              ipText(marketCoverageText(scope.row))
            }}</template></el-table-column
          >
          <el-table-column
            prop="audience"
            :label="ipText('目标受众')"
            min-width="140"
            show-overflow-tooltip
          />
          <el-table-column :label="ipText('基础权益价格范围')" min-width="180"
            ><template #header
              >{{ ipText("基础权益价格范围") }}
              <small class="ip-muted">{{ ipText("供参考") }}</small></template
            ><template #default="scope">{{
              ipText(priceText(scope.row))
            }}</template></el-table-column
          >
          <el-table-column :label="ipText('合作状态')" min-width="150"
            ><template #default="scope"
              ><el-tag
                :type="
                  scope.row.cooperationStatus === '可合作'
                    ? 'success'
                    : 'warning'
                "
                >{{ ipText(scope.row.cooperationStatus) }}</el-tag
              ></template
            ></el-table-column
          >
          <el-table-column :label="ipText('资料完成度')" width="130">
            <template #default="scope"
              ><el-progress
                :percentage="Number(scope.row.completeness || 0)"
                :show-text="true"
            /></template>
          </el-table-column>
          <el-table-column
            prop="updatedAt"
            :label="ipText('最近更新')"
            min-width="145"
          />
          <el-table-column :label="ipText('操作')" width="130" fixed="right"
            ><template #default="scope"
              ><el-button
                link
                type="primary"
                @click="openDrawer(scope.row.id)"
                >{{ ipText("查看档案") }}</el-button
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
        :lang="locale === 'en' ? 'en' : 'zh-CN'"
        :title="ipText('IP档案')"
        size="420px"
        class="ip-resource-drawer"
      >
        <div v-loading="drawerLoading" class="ip-drawer-content">
          <template v-if="displayDrawerDetail">
            <div class="ip-drawer-intro">
              <img
                v-if="drawerVisualURL"
                :src="drawerVisualURL"
                :alt="displayDrawerDetail.resource.name"
              />
              <div>
                <h2>{{ displayDrawerDetail.resource.name }}</h2>
                <el-tag>{{
                  ipText(displayDrawerDetail.resource.ipType)
                }}</el-tag>
                <p>
                  {{
                    displayDrawerDetail.resource.audience ||
                    ipText("受众待补充")
                  }}
                </p>
              </div>
            </div>
            <p>
              {{ displayDrawerDetail.resource.summary || ipText("暂无IP简介") }}
            </p>
            <div class="ip-drawer-section">
              <h3>{{ ipText("核心信息") }}</h3>
              <dl class="ip-dl">
                <dt>{{ ipText("版权方") }}</dt>
                <dd>
                  {{
                    displayDrawerDetail.resource.rightsOwner || ipText("待补充")
                  }}
                </dd>
                <dt>{{ ipText("覆盖市场及渗透率") }}</dt>
                <dd>
                  {{ ipText(marketCoverageText(displayDrawerDetail.resource)) }}
                </dd>
                <dt>{{ ipText("合作状态") }}</dt>
                <dd>
                  {{ ipText(displayDrawerDetail.resource.cooperationStatus) }}
                </dd>
              </dl>
            </div>
            <div class="ip-drawer-section">
              <h3>{{ ipText("参考权益价格") }}</h3>
              <strong class="ip-price">{{
                ipText(priceText(displayDrawerDetail.resource))
              }}</strong>
              <p class="ip-muted">{{ ipText("供参考，根据具体需求更新") }}</p>
            </div>
            <div class="ip-drawer-section">
              <h3>{{ ipText("可查看资料") }}</h3>
              <div
                v-for="file in displayDrawerDetail.files.filter(
                  item => item.fileKind !== 'visual'
                )"
                :key="file.id"
                class="ip-file"
              >
                <span
                  ><b class="ip-file-icon">PDF</b>{{ file.originalName }}</span
                >
                <el-button link @click="openFile(file)">{{
                  ipText("预览")
                }}</el-button>
              </div>
              <p
                v-if="
                  !displayDrawerDetail.files.some(
                    item => item.fileKind !== 'visual'
                  )
                "
                class="ip-muted"
              >
                {{ ipText("暂无附件") }}
              </p>
            </div>
            <el-button
              class="ip-primary ip-drawer-action"
              @click="goDetail(Number(displayDrawerDetail.resource.id))"
              >{{ ipText("查看详情 →") }}</el-button
            >
          </template>
        </div>
      </el-drawer>
    </template>

    <template v-else-if="mode === 'detail' && detail">
      <IPTranslationNotice
        :id="Number(detail.resource.id)"
        v-model:original="showOriginal"
        type="ip_resource"
        :records="[detail.resource, ...detail.cases]"
        @updated="updateContentTranslations"
      />
      <div class="ip-card ip-hero">
        <div class="ip-hero-media">
          <img
            v-if="visualURL"
            :src="visualURL"
            :alt="displayDetail.resource.name"
          />
          <div v-else class="ip-hero-empty">{{ ipText("暂无IP形象") }}</div>
          <label class="ip-hero-upload"
            >{{ ipText("＋ 上传形象")
            }}<input
              type="file"
              accept="image/png,image/jpeg,image/webp"
              hidden
              @change="addVisual"
          /></label>
        </div>
        <div class="ip-hero-copy">
          <strong>{{ displayDetail.resource.name }}</strong>
          <p>
            {{
              displayDetail.resource.summary ||
              ipText("暂无IP简介，请补充背景、特点和影响力。")
            }}
          </p>
          <div class="ip-hero-tags">
            <el-tag>{{ ipText(displayDetail.resource.ipType) }}</el-tag>
            <el-tag type="success">{{
              ipText(displayDetail.resource.cooperationStatus)
            }}</el-tag>
            <el-tag
              v-if="objectValue(displayDetail.resource.profile).lifecycle"
              type="info"
            >
              {{
                ipText(objectValue(displayDetail.resource.profile).lifecycle)
              }}
            </el-tag>
          </div>
        </div>
      </div>
      <div class="ip-detail-grid">
        <div class="ip-stack">
          <div class="ip-card">
            <h2>{{ ipText("基础权益价格范围") }}</h2>
            <div class="ip-price">
              {{ ipText(priceText(displayDetail.resource)) }}
            </div>
            <p class="ip-muted">
              {{ ipText("供参考，根据具体需求更新；最终以版权方报价为准。") }}
            </p>
          </div>
          <div class="ip-card">
            <div class="ip-card-head">
              <h2>{{ ipText("版权介绍文件") }}</h2>
              <label class="ip-upload-inline"
                >{{ ipText("＋ 上传版权资料")
                }}<input
                  type="file"
                  accept="application/pdf,.pdf"
                  multiple
                  hidden
                  @change="addCopyright"
              /></label>
            </div>
            <div
              v-for="file in displayDetail.files.filter(
                f => f.fileKind === 'copyright'
              )"
              :key="file.id"
              class="ip-file"
            >
              <span
                ><b class="ip-file-icon">PDF</b>{{ file.originalName }}
                <small
                  >{{
                    ipText((Number(file.sizeBytes) / 1024 / 1024).toFixed(1))
                  }}
                  MB · {{ ipText(file.createdAt) }}</small
                ></span
              >
              <div>
                <el-button link @click="openFile(file)">{{
                  ipText("预览")
                }}</el-button
                ><el-button link @click="openFile(file, true)">{{
                  ipText("下载")
                }}</el-button
                ><el-button link @click="renameStoredFile(file)">{{
                  ipText("重命名")
                }}</el-button
                ><el-button link type="danger" @click="removeFile(file)">{{
                  ipText("删除")
                }}</el-button>
              </div>
            </div>
            <el-empty
              v-if="!displayDetail.files.some(f => f.fileKind === 'copyright')"
              :description="ipText('暂无版权介绍文件')"
              :image-size="68"
            />
          </div>
          <div class="ip-card">
            <h2>{{ ipText("历史合作与结案") }}</h2>
            <div
              v-for="item in displayDetail.cases"
              :key="item.id"
              class="ip-case"
            >
              <strong>{{ item.title }}</strong>
              <p>{{ item.summary || ipText("暂无案例说明") }}</p>
              <div
                v-for="file in displayDetail.files.filter(
                  f => Number(f.caseId) === Number(item.id)
                )"
                :key="file.id"
                class="ip-file"
              >
                <span
                  ><b class="ip-file-icon">PDF</b>{{ file.originalName }}</span
                >
                <div>
                  <el-button link @click="openFile(file)">{{
                    ipText("预览")
                  }}</el-button
                  ><el-button link @click="openFile(file, true)">{{
                    ipText("下载")
                  }}</el-button
                  ><el-button link @click="renameStoredFile(file)">{{
                    ipText("重命名")
                  }}</el-button
                  ><el-button link type="danger" @click="removeFile(file)">{{
                    ipText("删除")
                  }}</el-button>
                </div>
              </div>
            </div>
            <p v-if="!displayDetail.cases.length" class="ip-muted">
              {{ ipText("暂无历史合作") }}
            </p>
            <div class="ip-case-form">
              <el-input
                v-model="caseTitle"
                :placeholder="ipText('合作案例名称')"
              /><el-input
                v-model="caseSummary"
                :placeholder="ipText('案例概述（选填）')"
              />
              <div class="ip-detail-case-file">
                <label class="ip-upload-inline"
                  >{{ ipText("＋ 添加结案PDF")
                  }}<input
                    type="file"
                    accept="application/pdf,.pdf"
                    hidden
                    @change="pdfChanged($event, 'case')"
                /></label>
                <span v-if="caseFile" class="ip-selected-file">
                  <span class="ip-selected-file-name">{{ caseFile.name }}</span>
                  <el-button link @click="renameSelectedFile('case')">{{
                    ipText("重命名")
                  }}</el-button>
                  <el-button
                    link
                    type="danger"
                    @click="removeSelectedFile('case')"
                    >{{ ipText("删除") }}</el-button
                  >
                </span>
              </div>
              <el-button :loading="saving" @click="addCase">{{
                ipText("添加历史合作")
              }}</el-button>
            </div>
          </div>
        </div>
        <div class="ip-stack">
          <div class="ip-card">
            <h2>{{ ipText("基础信息") }}</h2>
            <dl class="ip-dl">
              <dt>{{ ipText("IP类型") }}</dt>
              <dd>{{ ipText(displayDetail.resource.ipType) }}</dd>
              <dt>{{ ipText("版权方") }}</dt>
              <dd>
                {{ displayDetail.resource.rightsOwner || ipText("待补充") }}
              </dd>
              <dt>{{ ipText("联系人") }}</dt>
              <dd>{{ displayDetail.resource.contact || ipText("待补充") }}</dd>
              <dt>{{ ipText("覆盖市场及渗透率") }}</dt>
              <dd>{{ ipText(marketCoverageText(displayDetail.resource)) }}</dd>
              <dt>{{ ipText("目标受众") }}</dt>
              <dd>{{ displayDetail.resource.audience || ipText("待补充") }}</dd>
              <dt>{{ ipText("合作状态") }}</dt>
              <dd>{{ ipText(displayDetail.resource.cooperationStatus) }}</dd>
              <template v-for="field in profileFields" :key="field.key">
                <dt>{{ ipText(field.label) }}</dt>
                <dd>
                  {{
                    ipText(
                      objectValue(displayDetail.resource.profile)[field.key] ||
                        ipText("待补充")
                    )
                  }}
                </dd>
              </template>
            </dl>
          </div>
          <div class="ip-card">
            <h2>{{ ipText("合作建议与授权") }}</h2>
            <p>
              {{
                ipText(
                  objectValue(displayDetail.resource.profile).cooperationTips ||
                    ipText("暂无合作建议")
                )
              }}
            </p>
            <h3>{{ ipText("授权范围与限制") }}</h3>
            <p>
              {{
                ipText(
                  displayDetail.resource.licenseNotes || ipText("待版权方补充")
                )
              }}
            </p>
          </div>
          <div class="ip-card">
            <h2>{{ ipText("资料维护") }}</h2>
            <p class="ip-muted">
              {{ ipText("最近更新：")
              }}{{ ipText(displayDetail.resource.updatedAt) }}
            </p>
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
          {{ ipText("单条录入") }}
          <small>{{ ipText("适合补充少量IP") }}</small></button
        ><button
          :class="{ active: activeTab === 'bulk' }"
          @click="activeTab = 'bulk'"
        >
          {{ ipText("批量导入") }}
          <small>{{ ipText("适合集中 sourcing") }}</small>
        </button>
      </div>
      <template v-if="activeTab === 'single'">
        <div class="ip-detail-grid">
          <div class="ip-stack">
            <div class="ip-card">
              <h2>{{ ipText("基础信息") }}</h2>
              <div class="ip-form-grid ip-basic-grid">
                <label
                  >{{ ipText("IP名称 *")
                  }}<el-input
                    v-model="form.name"
                    :placeholder="ipText('请输入IP名称')"
                    maxlength="255"
                /></label>
                <label
                  >{{ ipText("IP类型 *")
                  }}<el-select
                    v-model="form.ipType"
                    popper-class="ip-option-popper"
                    :placeholder="ipText('请选择')"
                    ><el-option
                      v-for="type in formTypes"
                      :key="type"
                      :label="ipText(type)"
                      :value="type" /></el-select
                ></label>
                <label
                  >{{ ipText("版权方")
                  }}<el-input
                    v-model="form.rightsOwner"
                    :placeholder="ipText('版权方名称')"
                /></label>
                <label
                  >{{ ipText("联系人")
                  }}<el-input
                    v-model="form.contact"
                    :placeholder="ipText('联系人或联系方式')"
                /></label>
                <div class="ip-market-field ip-span">
                  <strong>{{ ipText("覆盖市场 *") }}</strong>
                  <el-select
                    v-model="form.markets"
                    popper-class="ip-option-popper"
                    multiple
                    filterable
                    allow-create
                    default-first-option
                    :aria-label="ipText('覆盖市场')"
                    :placeholder="ipText('请选择市场')"
                    @change="syncMarketPenetrations"
                  >
                    <el-option
                      v-for="market in markets"
                      :key="market"
                      :label="ipText(market)"
                      :value="market"
                    />
                  </el-select>
                  <div
                    v-if="form.markets.length"
                    class="ip-market-penetrations"
                  >
                    <div
                      v-for="market in form.markets"
                      :key="market"
                      class="ip-market-penetration-row"
                    >
                      <span>{{ ipText(market) }}</span>
                      <el-input-number
                        :model-value="form.marketPenetrations?.[market]"
                        :min="0"
                        :max="100"
                        :precision="2"
                        :controls="false"
                        :aria-label="
                          ipText('{0}市场渗透率（%）', [ipText(market)])
                        "
                        :placeholder="ipText('输入渗透率')"
                        @update:model-value="
                          value => setMarketPenetration(market, value)
                        "
                      />
                      <span>%</span>
                    </div>
                  </div>
                  <small>{{
                    ipText(
                      "每个市场可填写渗透率（0–100），例如25表示25%。各市场独立填写，暂无数据可留空。"
                    )
                  }}</small>
                </div>
                <label
                  >{{ ipText("合作状态")
                  }}<el-select
                    v-model="form.cooperationStatus"
                    popper-class="ip-option-popper"
                    ><el-option
                      v-for="status in statuses"
                      :key="status"
                      :label="ipText(status)"
                      :value="status" /></el-select
                ></label>
                <label
                  >{{ ipText("目标受众")
                  }}<el-input
                    v-model="form.audience"
                    :placeholder="ipText('例如：18–30岁年轻人')"
                /></label>
                <label class="ip-span"
                  >{{ ipText("IP简介")
                  }}<el-input
                    v-model="form.summary"
                    type="textarea"
                    :rows="3"
                    :placeholder="ipText('背景、特点和影响力')"
                /></label>
              </div>
            </div>
            <div class="ip-card">
              <h2>{{ ipText("商业与授权") }}</h2>
              <div class="ip-form-grid">
                <label
                  >{{ ipText("基础权益价格范围（参考）") }}
                  <div class="ip-money">
                    <el-select
                      v-model="form.currency"
                      popper-class="ip-option-popper"
                      ><el-option label="CNY ¥" value="CNY" /><el-option
                        label="USD $"
                        value="USD" /><el-option
                        label="EUR €"
                        value="EUR" /></el-select
                    ><el-input-number
                      v-model="form.priceMin"
                      :min="0"
                      :controls="false"
                      :placeholder="ipText('最低价')"
                    /><span>—</span
                    ><el-input-number
                      v-model="form.priceMax"
                      :min="0"
                      :controls="false"
                      :placeholder="ipText('最高价')"
                    />
                  </div>
                  <small>{{ ipText("供参考，根据具体需求更新") }}</small></label
                ><label
                  >{{ ipText("授权范围与限制")
                  }}<el-input
                    v-model="form.licenseNotes"
                    type="textarea"
                    :rows="3"
                    :placeholder="ipText('授权地域、渠道或使用限制')"
                /></label>
              </div>
            </div>
            <div class="ip-card">
              <h2>{{ ipText("资料附件") }}</h2>
              <div class="ip-form-grid">
                <div class="ip-attachment-field">
                  <strong>{{ ipText("IP形象（PNG / JPG / WebP）") }}</strong
                  ><span class="ip-upload"
                    ><input
                      type="file"
                      accept="image/png,image/jpeg,image/webp"
                      @change="visualChanged"
                    />{{ ipText("选择图片") }}</span
                  ><span v-if="visualFile" class="ip-selected-file">
                    <span class="ip-selected-file-name">{{
                      visualFile.name
                    }}</span>
                    <el-button link @click="renameSelectedFile('visual')">{{
                      ipText("重命名")
                    }}</el-button>
                    <el-button
                      link
                      type="danger"
                      @click="removeSelectedFile('visual')"
                      >{{ ipText("删除") }}</el-button
                    >
                  </span>
                </div>
                <div class="ip-attachment-field">
                  <strong>{{ ipText("版权介绍文件（PDF）") }}</strong
                  ><span class="ip-upload"
                    ><input
                      type="file"
                      accept="application/pdf,.pdf"
                      multiple
                      @change="pdfChanged($event, 'copyright')"
                    />{{ ipText("选择PDF文件") }}</span
                  ><span
                    v-for="(file, index) in copyrightFiles"
                    :key="`${index}:${file.name}`"
                    class="ip-selected-file"
                  >
                    <span class="ip-selected-file-name">{{ file.name }}</span>
                    <el-button
                      link
                      @click="renameSelectedFile('copyright', index)"
                      >{{ ipText("重命名") }}</el-button
                    >
                    <el-button
                      link
                      type="danger"
                      @click="removeSelectedFile('copyright', index)"
                      >{{ ipText("删除") }}</el-button
                    >
                  </span>
                </div>
                <div class="ip-attachment-field">
                  <strong>{{ ipText("历史合作结案文件（选填）") }}</strong
                  ><el-input
                    v-model="caseTitle"
                    :placeholder="ipText('历史合作名称')"
                  /><el-input
                    v-model="caseSummary"
                    :placeholder="ipText('案例概述')"
                  /><span class="ip-upload"
                    ><input
                      type="file"
                      accept="application/pdf,.pdf"
                      @change="pdfChanged($event, 'case')"
                    />{{ ipText("选择结案PDF") }}</span
                  ><span v-if="caseFile" class="ip-selected-file">
                    <span class="ip-selected-file-name">{{
                      caseFile.name
                    }}</span>
                    <el-button link @click="renameSelectedFile('case')">{{
                      ipText("重命名")
                    }}</el-button>
                    <el-button
                      link
                      type="danger"
                      @click="removeSelectedFile('case')"
                      >{{ ipText("删除") }}</el-button
                    >
                  </span>
                </div>
              </div>
            </div>
            <div class="ip-card ip-optional-profile">
              <el-collapse>
                <el-collapse-item name="profile">
                  <template #title>
                    <div>
                      <h2>{{ ipText("补充档案信息（选填）") }}</h2>
                      <p>
                        {{
                          ipText(
                            "完善受众、影响力和合作建议，便于后续需求匹配。"
                          )
                        }}
                      </p>
                    </div>
                  </template>
                  <div class="ip-form-grid">
                    <label v-for="field in profileFields" :key="field.key">
                      {{ ipText(field.label) }}
                      <el-input
                        v-model="form.profile[field.key]"
                        :placeholder="ipText(field.hint)"
                      />
                    </label>
                  </div>
                </el-collapse-item>
              </el-collapse>
            </div>
          </div>
          <div class="ip-stack">
            <div class="ip-card">
              <h2>{{ ipText("录入提示") }}</h2>
              <p>
                {{
                  ipText(
                    "填写名称、类型和覆盖市场后即可创建；渗透率暂无数据可留空。"
                  )
                }}
              </p>
              <p>{{ ipText("价格仅作参考，以最终授权报价为准。") }}</p>
              <p>{{ ipText("上传的PDF会显示在IP详情页。") }}</p>
            </div>
            <div class="ip-card">
              <h2>{{ ipText("历史合作") }}</h2>
              <div
                v-for="item in displayDetail?.cases || []"
                :key="item.id"
                class="ip-case"
              >
                <strong>{{ item.title }}</strong>
                <p>{{ item.summary }}</p>
              </div>
              <p v-if="!displayDetail?.cases?.length" class="ip-muted">
                {{ ipText("暂无历史合作案例，可在左侧补充结案资料。") }}
              </p>
            </div>
          </div>
        </div>
        <div class="ip-bottom">
          <el-button v-if="!form.id" @click="saveManualDraft">{{
            ipText("保存草稿")
          }}</el-button
          ><el-button v-else @click="goList">{{ ipText("取消") }}</el-button
          ><el-button
            class="ip-primary"
            :loading="saving"
            @click="saveManual"
            >{{
              ipText(form.id ? ipText("保存修改") : ipText("创建IP"))
            }}</el-button
          >
        </div>
      </template>
      <template v-else>
        <div class="ip-import-top">
          <div class="ip-card ip-import-step">
            <span class="ip-step">1</span>
            <div class="ip-import-main">
              <div>
                <h2>{{ ipText("下载模板") }}</h2>
                <p>
                  {{ ipText("先下载 Excel 模板，按要求填写后再上传文件。") }}
                </p>
                <el-button @click="downloadTemplate">{{
                  ipText("下载IP导入模板.xlsx")
                }}</el-button>
              </div>
              <div class="ip-template-fields">
                <strong>{{ ipText("模板主要字段") }}</strong>
                <div>
                  <span>{{ ipText("IP名称 *") }}</span
                  ><span>{{ ipText("IP类型 *") }}</span>
                  <span>{{ ipText("覆盖市场 *") }}</span
                  ><span>{{ ipText("市场渗透率") }}</span>
                  <span>{{ ipText("版权方") }}</span>
                  <span>{{ ipText("合作状态") }}</span
                  ><span>{{ ipText("参考价格范围") }}</span>
                  <span>{{ ipText("目标受众") }}</span
                  ><span>{{ ipText("联系人") }}</span>
                </div>
                <small>{{
                  ipText(
                    "一行一个IP；“渗透率1”至“渗透率9”逐列对应“覆盖市场1”至“覆盖市场9”，填写0至100的数字，例如25表示25%。可留空。"
                  )
                }}</small>
              </div>
            </div>
          </div>
          <div class="ip-card ip-import-rules">
            <h2>{{ ipText("导入规则") }}</h2>
            <p>{{ ipText("1. 按名称与版权方检查重复") }}</p>
            <p>{{ ipText("2. 重复市场及无效渗透率需修正后重新校验") }}</p>
            <p>{{ ipText("3. 已有IP不会被覆盖") }}</p>
          </div>
        </div>
        <div class="ip-card ip-import-step">
          <span class="ip-step">2</span>
          <div>
            <h2>{{ ipText("上传文件") }}</h2>
            <p>
              {{
                ipText(
                  "支持 .xlsx / .xls，最多500条。PDF为可选；文件名使用“导入编号_版权介绍.pdf”或“导入编号_结案.pdf”，导入后自动关联。"
                )
              }}
            </p>
            <div class="ip-batch-upload">
              <label class="ip-upload ip-upload-drop"
                ><input
                  type="file"
                  accept=".xlsx,.xls"
                  @change="readImport"
                /><strong>{{ ipText("选择Excel文件") }}</strong
                ><small>{{
                  ipText(importFileName || ipText("点击上传已填写的导入模板"))
                }}</small></label
              ><label class="ip-upload ip-upload-drop"
                ><input
                  type="file"
                  accept="application/pdf,.pdf"
                  multiple
                  @change="readBatchFiles"
                /><strong>{{ ipText("添加PDF附件（可选）") }}</strong
                ><small>{{
                  ipText(
                    batchFiles.length
                      ? ipText("已选{0}份PDF", [batchFiles.length])
                      : ipText("按模板中的导入编号关联，可一次上传多份")
                  )
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
                <h2>{{ ipText("校验预览") }}</h2>
                <p>
                  {{
                    ipText(
                      "“Excel行号”是文件中的实际行数；“导入编号”来自模板，用于匹配PDF附件。"
                    )
                  }}
                </p>
              </div>
              <strong v-if="importPreview.length"
                >{{ ipText("共") }}{{ ipText(importPreview.length)
                }}{{ ipText("条 · 可导入") }}{{ ipText(readyCount)
                }}{{ ipText("条 · 重复") }}{{ ipText(duplicateCount)
                }}{{ ipText("条 · 错误") }}{{ ipText(errorCount)
                }}{{ ipText("条") }}</strong
              >
            </div>
            <el-table
              :data="importPreview"
              max-height="340"
              :empty-text="ipText('上传模板后显示校验结果')"
              ><el-table-column :label="ipText('Excel行号')" width="110"
                ><template #default="scope">{{
                  ipText(excelRow(scope.row.row))
                }}</template> </el-table-column
              ><el-table-column :label="ipText('导入编号')" width="120"
                ><template #default="scope">{{
                  ipText(importKeyForRow(scope.row.row))
                }}</template> </el-table-column
              ><el-table-column
                prop="name"
                :label="ipText('IP名称')"
                min-width="180" /><el-table-column
                :label="ipText('覆盖市场及渗透率')"
                min-width="220"
                ><template #default="scope">{{
                  ipText(importCoverageText(scope.row.row))
                }}</template></el-table-column
              ><el-table-column :label="ipText('校验结果')" width="120"
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
                      ipText(
                        scope.row.status === "ready"
                          ? ipText("通过")
                          : scope.row.status === "duplicate"
                            ? ipText("疑似重复")
                            : ipText("字段错误")
                      )
                    }}</el-tag
                  ></template
                ></el-table-column
              ><el-table-column
                prop="message"
                :label="ipText('问题')"
                min-width="220"
            /></el-table>
          </div>
        </div>
        <div class="ip-bottom">
          <el-button @click="goList">{{ ipText("取消") }}</el-button
          ><el-button
            class="ip-primary"
            :loading="importLoading"
            :disabled="!readyCount"
            @click="runImport"
            >{{ ipText("导入") }}{{ ipText(readyCount)
            }}{{ ipText("条IP") }}</el-button
          >
        </div>
      </template>
    </template>
    <el-dialog
      v-model="previewVisible"
      class="ip-preview-dialog"
      :lang="locale === 'en' ? 'en' : 'zh-CN'"
      :title="ipText('PDF预览')"
      width="80%"
      top="5vh"
    >
      <div v-if="previewURL" class="ip-pdf-preview">
        <div class="ip-pdf-toolbar">
          <span>{{
            ipText(
              previewPageCount
                ? ipText("共 {0} 页 · 向下滚动查看", [previewPageCount])
                : ipText("正在读取 PDF…")
            )
          }}</span>
          <a :href="previewURL" :download="previewFileName">{{
            ipText("下载 PDF")
          }}</a>
        </div>
        <div v-if="previewError" class="ip-pdf-error">
          {{ ipText(previewError) }}
        </div>
        <div v-else class="ip-pdf">
          <p v-if="previewLoading" class="ip-pdf-loading">
            {{ ipText("正在渲染 PDF…") }}
          </p>
          <vue-pdf-embed
            :key="previewURL"
            :source="previewURL"
            @loaded="pdfLoaded"
            @rendered="previewLoading = false"
            @loading-failed="pdfFailed"
            @rendering-failed="pdfFailed"
          />
        </div>
      </div>
      <div v-else-if="previewError" class="ip-pdf-error">
        <p>{{ ipText(previewError) }}</p>
        <el-button v-if="previewFile" @click="openFile(previewFile)">{{
          ipText("重试")
        }}</el-button>
      </div>
      <div v-else-if="previewDownloading" class="ip-pdf-transfer">
        <p>{{ ipText(previewDownloadText) }}</p>
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
.ip-page :deep(.el-table th.el-table__cell > .cell) {
  display: block;
  word-break: normal;
  overflow-wrap: anywhere;
  white-space: normal;
}

.ip-page :deep(.el-table .el-tag) {
  height: auto;
  min-height: 24px;
  text-align: center;
  white-space: normal;
}

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
  min-width: 0;
  min-height: calc(100vh - 150px);
  padding: 16px 28px 36px;
  container: ip-resources / inline-size;
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
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.ip-head > div:first-child {
  flex: 1 1 280px;
  min-width: 0;
}

.ip-actions,
.ip-card-head,
.ip-bottom {
  flex-wrap: wrap;
}

.ip-actions :deep(.el-button + .el-button),
.ip-bottom :deep(.el-button + .el-button) {
  margin-left: 0;
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
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 240px), 1fr));
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
  flex: 2 1 280px;
  min-width: 0;
}

.ip-filter .el-select {
  flex: 1 1 190px;
  width: auto;
  min-width: 0;
  max-width: 300px;
}

.ip-filter :deep(.el-button + .el-button) {
  margin-left: 0;
}

.ip-filter :deep(.el-date-editor) {
  flex: 1 1 280px;
  max-width: 100%;
}

.ip-resource-table .ip-muted {
  display: block;
}

.ip-resource-table :deep(.cell > .el-button) {
  padding-inline: 4px;
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
  -webkit-line-clamp: 2;
  color: #8c9296;
  -webkit-box-orient: vertical;
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
  grid-template-columns: minmax(100px, 0.8fr) minmax(0, 1.2fr);
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
  vertical-align: middle;
  color: #fff;
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

.ip-detail-case-file {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
  min-width: 0;
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
  min-width: 0;
  font-weight: 600;
  line-height: 1.5;
}

.ip-attachment-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
  min-width: 0;
}

.ip-selected-file {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;
  min-width: 0;
  padding: 5px 9px;
  background: #f8faf2;
  border: 1px solid #e6edda;
  border-radius: 6px;
}

.ip-selected-file-name {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
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

.ip-page:lang(en) .ip-basic-grid label {
  display: flex;
  align-items: stretch;
}

.ip-market-field {
  display: flex;
  flex-direction: column;
  gap: 9px;
}

.ip-market-field > strong {
  font-size: 14px;
  font-weight: 600;
}

.ip-market-field > .el-select {
  width: 100%;
}

.ip-market-field > small {
  color: #626b56;
}

.ip-market-penetrations {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 16px;
  padding: 12px;
  background: #f8faf2;
  border: 1px solid #e6edda;
  border-radius: 7px;
}

.ip-market-penetration-row {
  display: flex;
  gap: 8px;
  align-items: center;
  min-width: 0;
}

.ip-market-penetration-row > span:first-child {
  min-width: 56px;
  max-width: 40%;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.ip-market-penetration-row > .el-input-number {
  width: 100%;
  min-width: 0;
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
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.ip-money .el-select {
  flex: none;
  width: 120px;
}

.ip-money .el-input-number {
  flex: 1 1 100px;
  width: auto;
  min-width: 0;
}

.ip-upload {
  position: relative;
  display: inline-flex;
  align-items: center;
  width: max-content;
  max-width: 100%;
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
  min-height: 55vh;
  max-height: 70vh;
  padding: 12px;
  overflow: hidden auto;
  overscroll-behavior: contain;
  background: #f3f4f0;
}

.ip-pdf-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.ip-pdf-toolbar a {
  font-size: 13px;
  font-weight: 700;
  color: #567811;
}

.ip-pdf-loading {
  margin: 0 0 12px;
  color: #737373;
}

.ip-pdf-transfer {
  min-height: 100px;
  padding: 24px 16px;
}

.ip-pdf-transfer p {
  margin: 0 0 14px;
  color: #535a47;
}

.ip-pdf :deep(.vue-pdf-embed > div + div) {
  margin-top: 12px;
}

.ip-pdf :deep(canvas) {
  display: block;
  max-width: 100%;
  height: auto !important;
  margin: 0 auto;
}

.ip-pdf-error {
  padding: 48px 16px;
  color: #737373;
  text-align: center;
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

@container ip-resources (width <= 850px) {
  .ip-detail-grid,
  .ip-import-top,
  .ip-import-main {
    grid-template-columns: minmax(0, 1fr);
  }
}

@container ip-resources (width <= 580px) {
  .ip-form-grid,
  .ip-case-form,
  .ip-hero,
  .ip-batch-upload {
    grid-template-columns: minmax(0, 1fr);
  }

  .ip-basic-grid label {
    display: flex;
    align-items: stretch;
  }

  .ip-head h1 {
    font-size: 26px;
  }
}

@media (width <= 700px) {
  .ip-money {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  }

  .ip-money .el-select {
    grid-column: 1 / -1;
    width: 100%;
  }

  .ip-money .el-input-number {
    width: 100%;
  }

  .ip-market-penetrations {
    grid-template-columns: 1fr;
  }

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
    flex-basis: 100%;
    width: 100%;
    max-width: none;
  }
}
</style>
