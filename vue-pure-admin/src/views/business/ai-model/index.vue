<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import {
  getAIModelConfig,
  saveAIModelConfig,
  testAIModelConfig
} from "@/api/business";

defineOptions({ name: "BusinessAIModel" });

const loading = ref(false);
const saving = ref(false);
const testing = ref(false);
const updatedAt = ref<number | string>("");
const enabled = ref(true);
const apiKeyInput = ref("");
const testResult = ref<{
  type: "success" | "info" | "warning";
  text: string;
}>();

const providerOptions = ref([
  "OpenAI",
  "Azure OpenAI",
  "Claude",
  "Gemini",
  "DeepSeek",
  "本地服务"
]);

const form = reactive({
  name: "AI 模型配置",
  provider: "",
  model: "",
  baseUrl: "",
  apiKeyConfigured: false,
  apiKeyLast4: "",
  enableDemandParsing: true,
  enableRecommendationReason: true,
  timeoutSeconds: 30,
  temperature: 0.2
});

const statusItems = computed(() => [
  ["供应商", form.provider || "未配置"],
  ["模型", form.model || "未配置"],
  ["API Key", form.apiKeyConfigured ? "已配置" : "未配置"]
]);

function parseContent(value: unknown) {
  if (!value) return {};
  if (typeof value !== "string") return value as Record<string, any>;
  try {
    return JSON.parse(value);
  } catch {
    return {};
  }
}

function updatedText(value: unknown) {
  if (!value) return "-";
  const time = Number(value);
  if (!Number.isFinite(time)) return String(value);
  return new Date(time).toLocaleString("zh-CN");
}

async function loadData() {
  loading.value = true;
  const res = await getAIModelConfig();
  loading.value = false;
  if (res.code !== 0) return;
  const data = res.data || {};
  Object.assign(form, parseContent(data.content));
  form.name = data.name || form.name;
  enabled.value =
    data.enabled === 1 || data.enabled === "1" || data.enabled === true;
  updatedAt.value = data.updatedAt || "";
}

function buildContent() {
  const { enableFallback, fallbackStrategy, ...content } = form as any;
  void enableFallback;
  void fallbackStrategy;
  if (apiKeyInput.value.trim()) {
    const key = apiKeyInput.value.trim();
    content.apiKeyConfigured = true;
    content.apiKeyLast4 = key.slice(-4);
  }
  return content;
}

async function save() {
  if (!form.provider || !form.model) {
    ElMessage.warning("请先配置供应商和模型");
    return;
  }
  saving.value = true;
  const res = await saveAIModelConfig({
    name: form.name,
    content: JSON.stringify(buildContent()),
    enabled: enabled.value,
    apiKey: apiKeyInput.value.trim()
  });
  saving.value = false;
  if (res.code === 0) {
    apiKeyInput.value = "";
    ElMessage.success("AI 模型配置已保存");
    loadData();
  }
}

async function testModel() {
  if (!form.provider || !form.model) {
    ElMessage.warning("请先配置供应商和模型");
    return;
  }
  testing.value = true;
  testResult.value = undefined;
  const res = await testAIModelConfig({
    ...buildContent(),
    apiKey: apiKeyInput.value.trim()
  });
  testing.value = false;
  if (res.code === 0) {
    testResult.value = {
      type: res.data?.realRequest ? "success" : "info",
      text: res.data?.message || "测试通过"
    };
    ElMessage.success("模型测试完成");
    return;
  }
  testResult.value = {
    type: "warning",
    text: res.message || "模型测试失败"
  };
}

onMounted(loadData);
</script>

<template>
  <div class="ai-model-page">
    <section class="page-hero">
      <div>
        <span>AI Integration</span>
        <h1>AI 模型配置</h1>
        <p>配置推荐助手使用的模型服务，并在保存前测试连接状态。</p>
      </div>
      <div class="hero-status">
        <span>{{ enabled ? "启用" : "停用" }}</span>
        <el-switch v-model="enabled" />
      </div>
    </section>

    <section class="status-grid">
      <div v-for="[label, value] in statusItems" :key="label">
        <span>{{ label }}</span>
        <strong>{{ value }}</strong>
      </div>
    </section>

    <section v-loading="loading" class="config-panel">
      <div class="panel-header">
        <div>
          <strong>模型接入</strong>
          <span>保存后推荐助手会按当前配置尝试接入模型服务</span>
        </div>
        <el-tag effect="plain">最近更新：{{ updatedText(updatedAt) }}</el-tag>
      </div>

      <el-form label-position="top" class="config-form">
        <el-alert
          v-if="testResult"
          class="test-alert"
          :type="testResult.type"
          :title="testResult.text"
          :closable="false"
          show-icon
        />

        <el-row :gutter="14">
          <el-col :xs="24" :md="12">
            <el-form-item label="配置名称">
              <el-input v-model="form.name" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="供应商">
              <el-select
                v-model="form.provider"
                allow-create
                filterable
                default-first-option
                placeholder="选择或输入供应商"
              >
                <el-option
                  v-for="provider in providerOptions"
                  :key="provider"
                  :label="provider"
                  :value="provider"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="模型">
              <el-input v-model="form.model" placeholder="如 gpt-4.1-mini" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="Base URL">
              <el-input
                v-model="form.baseUrl"
                placeholder="https://api.openai.com/v1"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="更新 API Key">
              <el-input
                v-model="apiKeyInput"
                type="password"
                show-password
                placeholder="留空则不更新"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="API Key 状态">
              <div class="key-status">
                <strong>{{
                  form.apiKeyConfigured ? "已配置" : "未配置"
                }}</strong>
                <span v-if="form.apiKeyLast4">尾号 {{ form.apiKeyLast4 }}</span>
              </div>
            </el-form-item>
          </el-col>
        </el-row>

        <div class="capability-grid">
          <label>
            <span>
              <strong>需求解析</strong>
              <small>将用户自然语言需求解析成结构化条件</small>
            </span>
            <el-switch v-model="form.enableDemandParsing" />
          </label>
          <label>
            <span>
              <strong>推荐解释</strong>
              <small>生成匹配理由、风险提示和推荐摘要</small>
            </span>
            <el-switch v-model="form.enableRecommendationReason" />
          </label>
        </div>

        <el-row :gutter="14">
          <el-col :xs="24" :md="12">
            <el-form-item label="超时时间（秒）">
              <el-input-number
                v-model="form.timeoutSeconds"
                :min="5"
                :max="120"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="Temperature">
              <el-input-number
                v-model="form.temperature"
                :min="0"
                :max="1"
                :step="0.1"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <div class="action-bar">
        <el-button :loading="loading" @click="loadData">重新加载</el-button>
        <el-button :loading="testing" @click="testModel">
          <IconifyIconOnline icon="ri:pulse-line" class="mr-1" />
          测试模型
        </el-button>
        <el-button type="primary" :loading="saving" @click="save">
          保存配置
        </el-button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.ai-model-page {
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
  margin-bottom: 14px;
  background: #fff;
  border: 1px solid #e2e8f0;
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
  font-size: 28px;
  line-height: 1.25;
  color: #0f172a;
  letter-spacing: 0;
}

.page-hero p {
  margin: 8px 0 0;
  color: #64748b;
}

.hero-status {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: flex-end;
  min-width: 110px;
}

.hero-status span {
  color: #0f172a;
  text-transform: none;
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}

.status-grid > div {
  display: grid;
  gap: 6px;
  padding: 14px 16px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.status-grid span,
.panel-header span,
.capability-grid small,
.key-status span {
  font-size: 12px;
  color: #64748b;
}

.status-grid strong,
.panel-header strong,
.capability-grid strong,
.key-status strong {
  color: #0f172a;
}

.config-panel {
  overflow: hidden;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.panel-header {
  display: flex;
  gap: 14px;
  align-items: center;
  justify-content: space-between;
  padding: 16px 18px;
  border-bottom: 1px solid #e2e8f0;
}

.panel-header > div {
  display: grid;
  gap: 4px;
}

.config-form {
  padding: 18px;
}

.key-status {
  display: grid;
  gap: 4px;
  min-height: 32px;
  padding: 0 2px;
}

.test-alert {
  margin-bottom: 16px;
}

.capability-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.capability-grid label {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.capability-grid label span {
  display: grid;
  gap: 4px;
}

.action-bar {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  padding: 12px 18px;
  background: #fff;
  border-top: 1px solid #e2e8f0;
}

.mr-1 {
  margin-right: 4px;
}

:deep(.el-select),
:deep(.el-input-number) {
  width: 100%;
}

@media (width <= 960px) {
  .page-hero,
  .panel-header {
    display: grid;
  }

  .hero-status,
  .panel-header .el-tag {
    justify-self: start;
  }

  .status-grid,
  .capability-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (width <= 640px) {
  .ai-model-page {
    padding: 12px;
  }

  .page-hero h1 {
    font-size: 24px;
  }

  .status-grid,
  .capability-grid {
    grid-template-columns: 1fr;
  }
}
</style>
