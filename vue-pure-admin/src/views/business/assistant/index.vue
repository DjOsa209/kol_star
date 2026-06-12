<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import * as XLSX from "xlsx";
import {
  createMarketOption,
  deleteMarketOption,
  getMarketOptions,
  recommendResources
} from "@/api/business";

defineOptions({ name: "BusinessAssistant" });

const loading = ref(false);
const elapsedSeconds = ref(0);
const parsed = ref<Record<string, any>>({});
const recommendations = ref<any[]>([]);
const filteredSummary = ref<Record<string, number>>({});
const message = ref("");
const submittedDemand = ref("");
const stopped = ref(false);
const attachedFiles = ref<any[]>([]);
const marketOptions = ref<string[]>([]);
const avatarLoadFailed = reactive<Record<string, boolean>>({});
const avatarLoaded = reactive<Record<string, boolean>>({});
let timer: ReturnType<typeof setInterval> | undefined;
let requestSeq = 0;
const recommendationStorageKey = "business-assistant-last-recommendation";
const recommendationStorageVersion = 2;

const form = reactive({
  demandText:
    "我们要在美国推广一款 AI 录音笔，预算 35000 美元，想找英语科技媒体和 YouTube 评测创作者，目标是新品曝光和点击转化。",
  targetMarket: "美国",
  resourceType: "",
  platform: "",
  budget: 0
});

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
const typeOptions = ["KOL", "创作者", "媒体"];
const platformOptions = [
  "YouTube",
  "TikTok",
  "Instagram",
  "Newsletter",
  "Website"
];

const parsedItems = computed(() =>
  Object.entries(parsed.value).filter(([, value]) => {
    return value !== "" && value !== 0 && value !== null && value !== undefined;
  })
);
const attachmentSummary = computed(() =>
  attachedFiles.value
    .map(file => `${file.name}：${file.summary || "已上传，等待模型参考文件信息"}`)
    .join("\n")
);
const filterItems = computed(() => Object.entries(filteredSummary.value));
const hasStarted = computed(() => Boolean(submittedDemand.value));
const hasResult = computed(
  () =>
    parsedItems.value.length > 0 ||
    recommendations.value.length > 0 ||
    filterItems.value.length > 0 ||
    Boolean(message.value)
);
const isInterrupted = computed(
  () => hasStarted.value && !loading.value && !hasResult.value && !stopped.value
);
const statusText = computed(() => {
  if (loading.value) return "执行中";
  if (stopped.value) return "已停止";
  if (hasResult.value) return "已完成";
  if (isInterrupted.value) return "待重试";
  return "待执行";
});
const elapsedText = computed(() => {
  if (!hasStarted.value) return "未执行";
  if (loading.value) return `已用 ${elapsedSeconds.value.toFixed(1)}s · 约 8s`;
  if (stopped.value) return `已停止 · ${elapsedSeconds.value.toFixed(1)}s`;
  if (isInterrupted.value)
    return `未完成 · ${elapsedSeconds.value.toFixed(1)}s`;
  return `完成 · ${elapsedSeconds.value.toFixed(1)}s`;
});
const panelTitle = computed(() => {
  if (loading.value && activeStepIndex.value >= 3) return "等待模型返回...";
  if (loading.value) return "智能体执行中";
  if (hasResult.value) return "执行结果";
  if (isInterrupted.value) return "执行未完成";
  return "等待执行";
});
const topRecommendations = computed(() => recommendations.value.slice(0, 6));
const recommendationLogic = computed(() => {
  if (topRecommendations.value.length === 0) {
    return "输入项目需求后，系统会结合目标市场、平台、资源类型、预算与风险等级生成推荐逻辑。";
  }
  const names = topRecommendations.value
    .slice(0, 3)
    .map(item => item.name)
    .join("、");
  const market = parsed.value.targetMarket || form.targetMarket || "目标市场";
  const budget = Number(parsed.value.budget || form.budget || 0);
  return `本次推广目标市场为${market}${budget ? `，预算约 ${moneyText(budget)} 美元` : ""}。系统优先选择与市场、平台和内容方向匹配度更高的资源：${names}。其余资源会因平台、地区、风险或预算匹配度较弱而降权或过滤。`;
});

const activeStepIndex = computed(() => {
  if (!hasStarted.value) return -1;
  if (!loading.value && hasResult.value) return 5;
  if (elapsedSeconds.value < 0.6) return 0;
  if (elapsedSeconds.value < 1.2) return 1;
  if (elapsedSeconds.value < 2.0) return 2;
  return 3;
});

const analysisSteps = computed(() => {
  const steps = [
    {
      title: "理解需求",
      details: `读取目标、筛选条件和${attachedFiles.value.length} 份资料 · 市场 ${form.targetMarket || "全部"} · 类型 ${form.resourceType || "全部"} · 平台 ${form.platform || "全部"}`
    },
    {
      title: "检索资源池",
      details: recommendations.value.length
        ? `候选资源已整理（${recommendations.value.length} 条）`
        : "整理候选资源、平台特征与预算约束"
    },
    {
      title: "发送给模型",
      details: "把需求、产品信息、策略方案、筛选条件和候选资源打包给配置模型"
    },
    {
      title: "等待模型返回...",
      details: loading.value
        ? "模型正在分析资源匹配度、推荐理由与过滤逻辑"
        : "模型响应完成或已进入本地规则回退"
    },
    {
      title: "整理推荐结论",
      details: hasResult.value
        ? "已生成推荐排序、匹配理由与过滤逻辑"
        : "生成推荐排序、匹配理由与风险提示"
    }
  ];

  return steps.map((step, index) => ({
    ...step,
    status: stepStatus(index)
  }));
});

async function generate() {
  if (!form.demandText.trim() && attachedFiles.value.length === 0) {
    ElMessage.warning("请输入项目需求或上传产品资料");
    return;
  }
  const current = ++requestSeq;
  submittedDemand.value = buildDemandWithAttachments();
  stopped.value = false;
  loading.value = true;
  elapsedSeconds.value = 0;
  startTimer();

  try {
    const res = await recommendResources({
      demandText: submittedDemand.value,
      targetMarket: form.targetMarket,
      language: "",
      resourceType: form.resourceType,
      platform: form.platform,
      budget: form.budget,
      includeWatch: false
    });

    if (current !== requestSeq) return;
    if (res.code === 0) {
      parsed.value = res.data.parsed || {};
      recommendations.value = res.data.recommendations || [];
      filteredSummary.value = res.data.filteredSummary || {};
      message.value = res.data.message || "";
      saveLastRecommendation();
    } else {
      ElMessage.warning(res.message || "推荐生成失败，请稍后重试");
    }
  } catch (error) {
    if (current === requestSeq) {
      ElMessage.warning(resolveRecommendError(error));
    }
  } finally {
    if (current === requestSeq) {
      loading.value = false;
      stopTimer();
    }
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
    if (form.targetMarket === name) form.targetMarket = "";
    ElMessage.success("市场选项已删除");
  } else {
    ElMessage.warning(res.message || "市场删除失败");
  }
}

function buildDemandWithAttachments() {
  const parts = [form.demandText.trim()].filter(Boolean);
  if (attachmentSummary.value) {
    parts.push(`\n【上传资料摘要】\n${attachmentSummary.value}`);
  }
  return parts.join("\n");
}

async function handleAttachmentChange(file: any) {
  const rawFile = file.raw;
  if (!rawFile) return;
  const existingIndex = attachedFiles.value.findIndex(
    item => item.uid === file.uid || item.name === rawFile.name
  );
  const item = {
    uid: file.uid,
    name: rawFile.name,
    size: rawFile.size,
    type: rawFile.type || file.name?.split(".").pop() || "",
    summary: await extractAttachmentSummary(rawFile)
  };
  if (existingIndex >= 0) {
    attachedFiles.value.splice(existingIndex, 1, item);
  } else {
    attachedFiles.value.push(item);
  }
}

function removeAttachment(uid: string | number) {
  attachedFiles.value = attachedFiles.value.filter(item => item.uid !== uid);
}

async function extractAttachmentSummary(file: File) {
  const name = file.name.toLowerCase();
  if (name.endsWith(".xlsx") || name.endsWith(".xls") || name.endsWith(".csv")) {
    const buffer = await file.arrayBuffer();
    const workbook = XLSX.read(buffer, { type: "array", cellDates: true });
    const lines = workbook.SheetNames.slice(0, 3).map(sheetName => {
      const rows = XLSX.utils.sheet_to_json<any[]>(workbook.Sheets[sheetName], {
        header: 1,
        defval: "",
        blankrows: false
      });
      const preview = rows
        .slice(0, 5)
        .map(row => row.filter(Boolean).join(" / "))
        .filter(Boolean)
        .join("；");
      return `${sheetName}：${preview || "空表"}`;
    });
    return lines.join("\n").slice(0, 1800);
  }

  if (
    file.type.startsWith("text/") ||
    name.endsWith(".md") ||
    name.endsWith(".txt")
  ) {
    return (await file.text()).slice(0, 1800);
  }

  return `${file.name}，${Math.ceil(file.size / 1024)}KB。浏览器端暂不解析正文，模型将参考文件名、类型和人工输入需求。`;
}

function stopGeneration() {
  if (!loading.value) return;
  requestSeq++;
  stopped.value = true;
  loading.value = false;
  stopTimer();
  ElMessage.info("已停止本次分析展示");
}

function saveLastRecommendation() {
  try {
    localStorage.setItem(
      recommendationStorageKey,
      JSON.stringify({
        version: recommendationStorageVersion,
        parsed: parsed.value,
        recommendations: recommendations.value,
        filteredSummary: filteredSummary.value,
        message: message.value,
        submittedDemand: submittedDemand.value,
        elapsedSeconds: elapsedSeconds.value,
        savedAt: Date.now()
      })
    );
  } catch {
    ElMessage.warning("推荐结果已生成，但浏览器本地保存失败");
  }
}

function loadLastRecommendation() {
  try {
    const raw = localStorage.getItem(recommendationStorageKey);
    if (!raw) return;
    const cache = JSON.parse(raw);
    if (cache.version !== recommendationStorageVersion) {
      localStorage.removeItem(recommendationStorageKey);
      return;
    }
    parsed.value = cache.parsed || {};
    recommendations.value = Array.isArray(cache.recommendations)
      ? cache.recommendations
      : [];
    filteredSummary.value = cache.filteredSummary || {};
    message.value = cache.message || "";
    submittedDemand.value = cache.submittedDemand || "上次推荐";
    elapsedSeconds.value = Number(cache.elapsedSeconds || 0);
  } catch {
    localStorage.removeItem(recommendationStorageKey);
  }
}

function clearLastRecommendation() {
  localStorage.removeItem(recommendationStorageKey);
  parsed.value = {};
  recommendations.value = [];
  filteredSummary.value = {};
  message.value = "";
  submittedDemand.value = "";
  elapsedSeconds.value = 0;
  stopped.value = false;
  Object.keys(avatarLoadFailed).forEach(key => delete avatarLoadFailed[key]);
  Object.keys(avatarLoaded).forEach(key => delete avatarLoaded[key]);
  ElMessage.success("上次推荐已清空");
}

function startTimer() {
  stopTimer();
  timer = setInterval(() => {
    elapsedSeconds.value = Number((elapsedSeconds.value + 0.1).toFixed(1));
  }, 100);
}

function stopTimer() {
  if (timer) clearInterval(timer);
  timer = undefined;
}

function priorityType(priority: string) {
  if (priority === "高") return "success";
  if (priority === "中") return "warning";
  return "info";
}

function stepStatus(index: number) {
  if (!hasStarted.value) return "pending";
  if (!loading.value && hasResult.value) return "done";
  if (stopped.value) return index < activeStepIndex.value ? "done" : "pending";
  if (!loading.value) return index < activeStepIndex.value ? "done" : "pending";
  if (index < activeStepIndex.value) return "done";
  if (index === activeStepIndex.value) return "active";
  return "pending";
}

function moneyText(value: unknown) {
  return Number(value || 0).toLocaleString("zh-CN");
}

function resolveRecommendError(error: unknown) {
  const err = error as {
    code?: string;
    message?: string;
    response?: { status?: number; data?: { message?: string } };
  };
  if (err.response?.data?.message) return err.response.data.message;
  if (err.response?.status === 401) return "登录已过期，请重新登录后再试";
  if (err.response?.status === 403) return "当前账号没有智能资源助手权限";
  if (err.code === "ECONNABORTED") {
    return "模型响应时间较长，本次请求已超时，请稍后重试或调低模型超时时间";
  }
  if (err.message?.includes("timeout")) {
    return "模型响应时间较长，本次请求已超时，请稍后重试";
  }
  return "推荐接口暂不可用，请确认后端服务已启动";
}

function compactNumber(value: unknown) {
  const number = Number(value || 0);
  if (number >= 100000000) return `${Math.round(number / 100000000)}亿`;
  if (number >= 10000) return `${Math.round(number / 10000)}万`;
  return number.toLocaleString("zh-CN");
}

function initials(name: unknown) {
  return String(name || "AI")
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map(word => word[0])
    .join("")
    .toUpperCase();
}

function avatarKey(row: any) {
  return String(row.id || row.name || row.avatarUrl || "");
}

function markAvatarFailed(row: any) {
  avatarLoadFailed[avatarKey(row)] = true;
}

function markAvatarLoaded(row: any) {
  avatarLoaded[avatarKey(row)] = true;
}

onMounted(() => {
  loadLastRecommendation();
  loadMarkets();
});
onBeforeUnmount(stopTimer);
</script>

<template>
  <div class="assistant-page">
    <header class="assistant-header">
      <div class="brand-block">
        <div class="brand-icon">
          <IconifyIconOnline icon="ri:sparkling-2-line" />
        </div>
        <div>
          <h1>资源智能助手</h1>
          <p>单次任务执行 · 全球媒体 / KOL 资源智能体</p>
        </div>
      </div>
      <div class="status-pill" :class="{ 'is-loading': loading }">
        {{ statusText }}
      </div>
    </header>

    <section class="intro-panel">
      嗨，我是你的资源智能助手。把项目需求、产品信息或策略方案给我--<strong>目标市场、预算、想要的资源类型和投放目标</strong>。我会从资源库里挑出最匹配的资源，并给你<strong>推荐排序、匹配理由和过滤逻辑</strong>。
    </section>

    <section class="composer-card">
      <el-input
        v-model="form.demandText"
        type="textarea"
        :rows="5"
        maxlength="1000"
        resize="none"
        show-word-limit
        placeholder="输入市场、产品、预算、平台和投放目标..."
      />

      <div class="upload-panel">
        <div>
          <strong>产品信息 / 策略方案</strong>
          <span>支持 TXT、Markdown、CSV、Excel；PDF/Docx 会先作为文件上下文进入推荐。</span>
        </div>
        <el-upload
          accept=".txt,.md,.csv,.xlsx,.xls,.pdf,.doc,.docx"
          :auto-upload="false"
          :show-file-list="false"
          :on-change="handleAttachmentChange"
        >
          <el-button>
            <IconifyIconOnline icon="ri:attachment-2" class="mr-1" />
            上传资料
          </el-button>
        </el-upload>
      </div>

      <div v-if="attachedFiles.length" class="attachment-list">
        <div v-for="file in attachedFiles" :key="file.uid">
          <IconifyIconOnline icon="ri:file-text-line" />
          <div>
            <strong>{{ file.name }}</strong>
            <span>{{ file.summary }}</span>
          </div>
          <el-button link type="danger" @click="removeAttachment(file.uid)">
            移除
          </el-button>
        </div>
      </div>

      <div class="composer-toolbar">
        <div class="filter-group">
          <el-select
            v-model="form.targetMarket"
            allow-create
            filterable
            default-first-option
            class="filter-select"
            @change="handleMarketChange"
          >
            <template #prefix>市场</template>
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
          </el-select>
          <el-select
            v-model="form.resourceType"
            clearable
            class="filter-select"
            placeholder="全部"
          >
            <template #prefix>类型</template>
            <el-option
              v-for="type in typeOptions"
              :key="type"
              :label="type"
              :value="type"
            />
          </el-select>
          <el-select
            v-model="form.platform"
            clearable
            class="filter-select"
            placeholder="全部"
          >
            <template #prefix>平台</template>
            <el-option
              v-for="platform in platformOptions"
              :key="platform"
              :label="platform"
              :value="platform"
            />
          </el-select>
        </div>

        <div class="action-group">
          <el-button
            class="stop-button"
            :disabled="!loading"
            @click="stopGeneration"
          >
            停止
          </el-button>
          <el-button type="primary" :loading="loading" @click="generate">
            {{ loading ? "生成中..." : "生成推荐" }}
          </el-button>
        </div>
      </div>
    </section>

    <section class="analysis-panel">
      <div class="panel-heading">
        <div class="panel-title">
          <span class="panel-icon">
            <IconifyIconOnline icon="ri:sparkling-2-line" />
          </span>
          <strong>{{ panelTitle }}</strong>
        </div>
        <div class="elapsed-pill">{{ elapsedText }}</div>
      </div>

      <div v-if="hasStarted" class="step-list">
        <div
          v-for="step in analysisSteps"
          :key="step.title"
          class="step-item"
          :class="`is-${step.status}`"
        >
          <span class="step-dot">
            <IconifyIconOnline
              :icon="
                step.status === 'done'
                  ? 'ri:check-line'
                  : step.status === 'active'
                    ? 'ri:loader-4-line'
                    : 'ri:circle-line'
              "
            />
          </span>
          <div>
            <strong>{{ step.title }}</strong>
            <p>{{ step.details }}</p>
          </div>
        </div>
      </div>

      <div v-else class="agent-empty">
        <span class="agent-empty-icon">
          <IconifyIconOnline icon="ri:play-circle-line" />
        </span>
        <div>
          <strong>等待执行</strong>
          <p>填写需求后点击生成推荐，智能体才会开始分析步骤。</p>
        </div>
      </div>

      <div v-if="hasResult" class="draft-box">
        {{ recommendationLogic }}
      </div>
    </section>

    <el-alert
      v-if="hasResult"
      class="review-alert"
      type="warning"
      :closable="false"
      show-icon
      title="以下内容由智能助手基于资源库与需求生成，请按需复核与调整。"
    />

    <section class="recommend-section">
      <div class="section-heading">
        <h2>推荐资源</h2>
        <el-button
          v-if="hasResult"
          link
          type="danger"
          :disabled="loading"
          @click="clearLastRecommendation"
        >
          清空上次推荐
        </el-button>
      </div>
      <div v-if="topRecommendations.length === 0" class="empty-block">
        暂无推荐结果
      </div>
      <div v-else class="resource-grid">
        <article v-for="item in topRecommendations" :key="item.id">
          <div class="resource-head">
            <div class="resource-avatar">
              <span>{{ initials(item.name) }}</span>
              <img
                v-if="item.avatarUrl && !avatarLoadFailed[avatarKey(item)]"
                v-show="avatarLoaded[avatarKey(item)]"
                :src="item.avatarUrl"
                :alt="item.name"
                @load="markAvatarLoaded(item)"
                @error="markAvatarFailed(item)"
              />
            </div>
            <div class="resource-title">
              <div>
                <strong>{{ item.name }}</strong>
                <span>
                  {{ item.resourceType || "-" }} · {{ item.platform || "-" }}
                </span>
              </div>
              <el-tag :type="priorityType(item.priority)" effect="light">
                {{ item.level || "-" }} 匹配
              </el-tag>
            </div>
          </div>

          <div class="metric-grid">
            <div>
              <strong>{{ compactNumber(item.followers) }}</strong>
              <span>粉丝 / 订阅</span>
            </div>
            <div>
              <strong>{{ compactNumber(item.avgViews) }}</strong>
              <span>平均播放 / 阅读</span>
            </div>
            <div>
              <strong>{{ item.engagementRate || "-" }}%</strong>
              <span>互动率</span>
            </div>
            <div>
              <strong>{{ item.country || "-" }}</strong>
              <span>所在地</span>
            </div>
          </div>

          <div class="reason-box">
            <strong>推荐理由</strong>
            <span>{{ item.reason || "综合匹配度较高，适合进入候选池。" }}</span>
          </div>
        </article>
      </div>
    </section>

    <section class="logic-section">
      <h2>推荐逻辑</h2>
      <p>{{ recommendationLogic }}</p>
      <div class="filter-row">
        <el-tag
          v-for="[reason, count] in filterItems"
          :key="reason"
          type="warning"
          effect="plain"
        >
          {{ reason }}：{{ count }}
        </el-tag>
        <span v-if="filterItems.length === 0">无过滤记录</span>
      </div>
    </section>
  </div>
</template>

<style scoped>
.assistant-page {
  min-height: 100%;
  padding: 16px;
  color: #171717;
  background: #f7f8fa;
}

.assistant-header {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.brand-block {
  display: flex;
  gap: 10px;
  align-items: center;
}

.brand-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  font-size: 22px;
  color: #fff;
  background: #171717;
  border-radius: 8px;
}

.brand-block h1 {
  margin: 0;
  font-size: 22px;
  font-weight: 760;
  line-height: 1.2;
  letter-spacing: 0;
}

.brand-block p {
  margin: 5px 0 0;
  font-size: 13px;
  color: #7a7a7a;
}

.status-pill,
.elapsed-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 32px;
  padding: 0 14px;
  font-size: 13px;
  font-weight: 700;
  color: #737373;
  background: #fff;
  border: 1px solid #dedede;
  border-radius: 999px;
}

.status-pill.is-loading {
  color: #171717;
}

.intro-panel,
.composer-card,
.analysis-panel,
.recommend-section,
.logic-section {
  background: #fff;
  border: 1px solid #e2e2e2;
  border-radius: 8px;
  box-shadow: 0 8px 22px rgb(15 23 42 / 4%);
}

.intro-panel {
  padding: 16px 18px;
  margin-bottom: 12px;
  font-size: 14px;
  font-weight: 520;
  line-height: 1.75;
}

.intro-panel strong {
  font-weight: 780;
}

.composer-card {
  padding: 12px;
  margin-bottom: 14px;
}

.upload-panel {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  margin-top: 10px;
  background: #fbfbfb;
  border: 1px solid #e8e8e8;
  border-radius: 8px;
}

.upload-panel > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.upload-panel strong {
  font-size: 13px;
  color: #171717;
}

.upload-panel span {
  font-size: 12px;
  color: #7a7a7a;
}

.attachment-list {
  display: grid;
  gap: 8px;
  margin-top: 10px;
}

.attachment-list > div {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  padding: 10px 12px;
  background: #fff;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
}

.attachment-list svg {
  color: #ea580c;
}

.attachment-list strong {
  display: block;
  margin-bottom: 4px;
  font-size: 13px;
  color: #171717;
}

.attachment-list span {
  display: -webkit-box;
  overflow: hidden;
  font-size: 12px;
  line-height: 1.5;
  color: #7a7a7a;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.composer-toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: space-between;
  padding-top: 10px;
  margin-top: 8px;
  border-top: 1px solid #e8e8e8;
}

.filter-group,
.action-group,
.panel-heading,
.panel-title,
.resource-head,
.resource-title,
.filter-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.filter-group {
  flex-wrap: wrap;
}

.filter-select {
  width: 148px;
}

.action-group {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.stop-button {
  color: #ef4444;
  border-color: #fecaca;
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

.analysis-panel {
  padding: 16px 18px;
  margin-bottom: 14px;
}

.panel-heading {
  justify-content: space-between;
  margin-bottom: 14px;
}

.panel-title strong {
  font-size: 15px;
}

.panel-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  font-size: 15px;
  color: #ea580c;
  background: #fff7ed;
  border: 1px solid #fed7aa;
  border-radius: 8px;
}

.step-list {
  display: grid;
  gap: 0;
}

.step-item {
  position: relative;
  display: grid;
  grid-template-columns: 32px minmax(0, 1fr);
  min-height: 64px;
}

.step-item::before {
  position: absolute;
  top: 26px;
  bottom: -4px;
  left: 12px;
  width: 2px;
  content: "";
  background: #d9f99d;
}

.step-item:last-child::before {
  display: none;
}

.step-dot {
  z-index: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  font-size: 14px;
  color: #a3a3a3;
  background: #fff;
  border: 2px solid #d4d4d4;
  border-radius: 99px;
}

.step-item.is-done .step-dot {
  color: #fff;
  background: #059669;
  border-color: #059669;
}

.step-item.is-active .step-dot {
  color: #ea580c;
  border-color: #ea580c;
  animation: agent-pulse 1.2s ease-in-out infinite;
}

.step-item.is-active strong {
  color: #c2410c;
}

.step-item strong {
  display: block;
  margin-top: 2px;
  font-size: 14px;
  color: #171717;
}

.step-item p {
  margin: 7px 0 0;
  font-size: 13px;
  line-height: 1.55;
  color: #8a8a8a;
}

.agent-empty {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 14px;
  color: #737373;
  background: #fbfbfb;
  border: 1px dashed #d8d8d8;
  border-radius: 8px;
}

.agent-empty-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  font-size: 18px;
  color: #171717;
  background: #fff;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
}

.agent-empty strong {
  display: block;
  margin-bottom: 4px;
  font-size: 14px;
  color: #171717;
}

.agent-empty p {
  margin: 0;
  font-size: 13px;
  line-height: 1.55;
}

.draft-box {
  padding: 12px 14px;
  margin-left: 32px;
  font-size: 13px;
  line-height: 1.75;
  color: #333;
  background: #fbfbfb;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
}

.review-alert {
  margin-bottom: 14px;
}

.recommend-section,
.logic-section {
  padding: 16px 18px;
  margin-bottom: 14px;
}

.recommend-section h2,
.logic-section h2 {
  margin: 0;
  font-size: 16px;
  letter-spacing: 0;
}

.section-heading {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.resource-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.resource-grid article {
  display: grid;
  gap: 12px;
  padding: 14px;
  border: 1px solid #e2e2e2;
  border-radius: 8px;
}

.resource-avatar {
  position: relative;
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  overflow: hidden;
  font-size: 16px;
  font-weight: 780;
  color: #059669;
  background: #ecfdf5;
  border-radius: 8px;
}

.resource-avatar img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.resource-title {
  flex: 1;
  justify-content: space-between;
  min-width: 0;
}

.resource-title > div {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 8px;
  align-items: baseline;
  min-width: 0;
}

.resource-title strong {
  font-size: 16px;
  color: #171717;
}

.resource-title span,
.metric-grid span,
.reason-box span,
.logic-section p,
.filter-row span {
  color: #7a7a7a;
}

.resource-title span,
.metric-grid span,
.reason-box span,
.filter-row span {
  font-size: 12px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.metric-grid strong {
  display: block;
  font-size: 16px;
  color: #171717;
}

.metric-grid span {
  display: block;
  margin-top: 4px;
}

.reason-box {
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.65;
  background: #fbfbfb;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
}

.reason-box strong {
  margin-right: 8px;
  color: #c2410c;
}

.logic-section p {
  margin: 0;
  font-size: 14px;
  line-height: 1.85;
}

.filter-row {
  flex-wrap: wrap;
  margin-top: 12px;
}

.empty-block {
  padding: 22px;
  font-size: 13px;
  color: #8a8a8a;
  text-align: center;
  border: 1px dashed #d4d4d4;
  border-radius: 8px;
}

:deep(.composer-card .el-textarea__inner) {
  min-height: 150px !important;
  padding: 14px 16px;
  font-size: 14px;
  line-height: 1.75;
  border: 0;
  box-shadow: none;
}

:deep(.filter-select .el-select__wrapper) {
  min-height: 34px;
  border-radius: 999px;
  box-shadow: 0 0 0 1px #e5e5e5 inset;
}

:deep(.filter-select .el-select__prefix) {
  margin-right: 8px;
  font-weight: 700;
  color: #9a9a9a;
}

:deep(.action-group .el-button) {
  min-height: 34px;
  padding: 0 14px;
  font-size: 13px;
  font-weight: 700;
  border-radius: 8px;
}

:deep(.action-group .el-button--primary) {
  --el-button-bg-color: #171717;
  --el-button-border-color: #171717;
  --el-button-hover-bg-color: #2a2a2a;
  --el-button-hover-border-color: #2a2a2a;
  --el-button-active-bg-color: #000;
  --el-button-active-border-color: #000;
}

@keyframes agent-pulse {
  0%,
  100% {
    transform: scale(1);
  }

  50% {
    transform: scale(1.08);
  }
}

@media (width <= 1180px) {
  .composer-toolbar {
    align-items: flex-start;
  }

  .composer-toolbar,
  .upload-panel,
  .panel-heading {
    display: grid;
  }

  .action-group {
    justify-content: flex-start;
  }

  .resource-grid {
    grid-template-columns: 1fr;
  }
}

@media (width <= 720px) {
  .assistant-page {
    padding: 12px;
  }

  .assistant-header,
  .brand-block,
  .resource-head,
  .resource-title {
    align-items: flex-start;
  }

  .assistant-header,
  .composer-toolbar,
  .upload-panel,
  .filter-group,
  .action-group,
  .resource-head,
  .resource-title {
    display: grid;
  }

  .status-pill,
  .filter-select,
  .action-group .el-button {
    width: 100%;
  }

  .intro-panel,
  .analysis-panel,
  .recommend-section,
  .logic-section {
    padding: 14px;
  }

  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .draft-box {
    margin-left: 0;
  }
}
</style>
