<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getPlatformSyncControl,
  savePlatformSyncControl
} from "@/api/system";
import { syncAllResources } from "@/api/business";

defineOptions({ name: "SystemPlatformSyncControl" });

const loading = ref(false);
const saving = ref(false);
const syncing = ref(false);
const settings = ref<any[]>([]);
const tokenStatus = ref<Record<string, any>>({});
const apiConfig = ref<any>({
  youtubeApiKey: "",
  youtubeApiKeyConfigured: false,
  youtubeApiKeyLast4: "",
  youtubeProxyUrl: "",
  metaGraphApiVersion: "v21.0",
  instagramAccessToken: "",
  instagramAccessTokenConfigured: false,
  instagramAccessTokenLast4: "",
  instagramUserId: "",
  tiktokAccessToken: "",
  tiktokAccessTokenConfigured: false,
  tiktokAccessTokenLast4: "",
  tikhubApiKey: "",
  tikhubApiKeyConfigured: false,
  tikhubApiKeyLast4: ""
});
const latestJob = ref<any>(null);
const resourceCounts = ref<any[]>([]);
const lastResourceSyncAt = ref<any>(null);
let pollTimer: ReturnType<typeof setInterval> | null = null;

const syncRunning = computed(() => latestJob.value?.status === "运行中");
const progress = computed(() => {
  const job = latestJob.value;
  if (!job?.totalCount) return 0;
  const done =
    Number(job.successCount || 0) +
    Number(job.failedCount || 0) +
    Number(job.skippedCount || 0);
  return Math.min(100, Math.round((done / Number(job.totalCount)) * 100));
});

function formatDateTime(value: unknown) {
  if (!value) return "-";
  const time = Number(value);
  if (!Number.isFinite(time)) return String(value);
  return new Date(time).toLocaleString("zh-CN");
}

function platformCount(platform: string) {
  const normalized = platform.toLowerCase();
  return resourceCounts.value
    .filter(item => String(item.platform || "").toLowerCase() === normalized)
    .reduce((sum, item) => sum + Number(item.total || 0), 0);
}

function normalizeSwitch(value: unknown) {
  return value === true || value === 1 || value === "1";
}

async function loadData() {
  loading.value = true;
  const res = await getPlatformSyncControl();
  loading.value = false;
  if (res.code !== 0) return;
  const data = res.data || {};
  settings.value = (data.settings || []).map((row: any) => ({
    ...row,
    enabled: normalizeSwitch(row.enabled),
    syncProfile: normalizeSwitch(row.syncProfile),
    syncPosts: normalizeSwitch(row.syncPosts),
    postLimit: Number(row.postLimit || 25)
  }));
  tokenStatus.value = data.tokenStatus || {};
  Object.assign(apiConfig.value, data.apiConfig || {});
  apiConfig.value.youtubeApiKey = "";
  apiConfig.value.instagramAccessToken = "";
  apiConfig.value.tiktokAccessToken = "";
  apiConfig.value.tikhubApiKey = "";
  latestJob.value = data.latestJob || null;
  resourceCounts.value = data.resourceCounts || [];
  lastResourceSyncAt.value = data.lastResourceSyncAt || null;
  if (syncRunning.value) startPolling();
}

async function save() {
  const typedSecrets = {
    youtubeApiKey: apiConfig.value.youtubeApiKey,
    instagramAccessToken: apiConfig.value.instagramAccessToken,
    tiktokAccessToken: apiConfig.value.tiktokAccessToken,
    tikhubApiKey: apiConfig.value.tikhubApiKey
  };
  saving.value = true;
  const res = await savePlatformSyncControl({
    settings: settings.value,
    apiConfig: apiConfig.value
  });
  saving.value = false;
  if (res.code === 0) {
    ElMessage.success("抓取控制已保存");
    await loadData();
    apiConfig.value.youtubeApiKey = typedSecrets.youtubeApiKey;
    apiConfig.value.instagramAccessToken = typedSecrets.instagramAccessToken;
    apiConfig.value.tiktokAccessToken = typedSecrets.tiktokAccessToken;
    apiConfig.value.tikhubApiKey = typedSecrets.tikhubApiKey;
  }
}

async function startSync() {
  await ElMessageBox.confirm(
    "将按当前平台开关，在后台同步全球资源库中的可同步资源。",
    "启动异步同步",
    {
      type: "info",
      confirmButtonText: "启动",
      cancelButtonText: "取消"
    }
  );
  syncing.value = true;
  const res = await syncAllResources();
  if (res.code === 0) {
    ElMessage.success(res.data?.message || "同步任务已启动");
    await loadData();
    startPolling();
  } else {
    syncing.value = false;
    ElMessage.warning(res.message || "启动失败");
  }
}

function startPolling() {
  if (pollTimer) return;
  pollTimer = setInterval(async () => {
    await loadData();
    if (!syncRunning.value) {
      stopPolling();
      syncing.value = false;
    }
  }, 3000);
}

function stopPolling() {
  if (!pollTimer) return;
  clearInterval(pollTimer);
  pollTimer = null;
}

onMounted(loadData);
onUnmounted(stopPolling);
</script>

<template>
  <div class="platform-sync-page">
    <section class="page-hero">
      <div>
        <span>Platform Sync</span>
        <h1>抓取控制</h1>
        <p>控制平台数据同步开关、授权状态和全局异步同步任务。</p>
      </div>
      <div class="hero-actions">
        <el-button :loading="saving" type="primary" @click="save">
          <IconifyIconOnline icon="ri:save-3-line" class="mr-1" />
          保存配置
        </el-button>
        <el-button
          :loading="syncing || syncRunning"
          type="success"
          @click="startSync"
        >
          <IconifyIconOnline icon="ri:cloud-line" class="mr-1" />
          启动异步同步
        </el-button>
      </div>
    </section>

    <section class="status-grid">
      <div>
        <span>最近同步</span>
        <strong>{{ formatDateTime(lastResourceSyncAt) }}</strong>
      </div>
      <div>
        <span>任务状态</span>
        <strong>{{ latestJob?.status || "未运行" }}</strong>
      </div>
      <div>
        <span>成功/失败</span>
        <strong>
          {{ latestJob?.successCount || 0 }}/{{ latestJob?.failedCount || 0 }}
        </strong>
      </div>
    </section>

    <section v-if="latestJob" class="sync-panel">
      <div class="panel-header">
        <div>
          <strong>异步任务</strong>
          <span>{{ latestJob.message || "等待同步" }}</span>
        </div>
        <el-tag :type="syncRunning ? 'warning' : 'success'" effect="plain">
          {{ latestJob.status }}
        </el-tag>
      </div>
      <el-progress
        :percentage="progress"
        :status="latestJob.status === '失败' ? 'exception' : undefined"
      />
      <div class="job-meta">
        <span>总数 {{ latestJob.totalCount || 0 }}</span>
        <span>成功 {{ latestJob.successCount || 0 }}</span>
        <span>失败 {{ latestJob.failedCount || 0 }}</span>
        <span>跳过 {{ latestJob.skippedCount || 0 }}</span>
        <span v-if="latestJob.currentResourceName">
          当前：{{ latestJob.currentResourceName }}
        </span>
      </div>
    </section>

    <section class="api-panel">
      <div class="panel-header">
        <div>
          <strong>API 接入配置</strong>
          <span>敏感字段留空表示不修改现有配置。</span>
        </div>
      </div>
      <el-form label-position="top" class="api-form">
        <el-row :gutter="14">
          <el-col :xs="24" :md="12">
            <el-form-item label="YouTube API Key">
              <el-input
                v-model="apiConfig.youtubeApiKey"
                type="password"
                show-password
                placeholder="留空不修改"
              />
              <div class="field-tip">
                {{
                  apiConfig.youtubeApiKeyConfigured
                    ? `已配置，尾号 ${apiConfig.youtubeApiKeyLast4}`
                    : "未配置"
                }}
              </div>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="TikHub API Key">
              <el-input
                v-model="apiConfig.tikhubApiKey"
                type="password"
                show-password
                placeholder="留空不修改"
              />
              <div class="field-tip">
                {{
                  apiConfig.tikhubApiKeyConfigured
                    ? `已配置，尾号 ${apiConfig.tikhubApiKeyLast4}`
                    : "未配置"
                }}
              </div>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="YouTube 代理地址">
              <el-input
                v-model="apiConfig.youtubeProxyUrl"
                placeholder="例如 http://127.0.0.1:7890"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="TikHub 接入说明">
              <el-input model-value="TikTok 与 Instagram 抓取共用该 Key" disabled />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </section>

    <section v-loading="loading" class="settings-panel">
      <div class="panel-header">
        <div>
          <strong>平台开关</strong>
          <span>关闭后，一键同步会跳过对应平台资源。</span>
        </div>
      </div>
      <el-table :data="settings" stripe class="settings-table">
        <el-table-column prop="platform" label="平台" width="140" />
        <el-table-column label="启用抓取" width="120">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" />
          </template>
        </el-table-column>
        <el-table-column label="Token 状态" min-width="220">
          <template #default="{ row }">
            <el-tag
              :type="tokenStatus[row.platform]?.configured ? 'success' : 'warning'"
              effect="plain"
            >
              {{ tokenStatus[row.platform]?.message || "未检测" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="资源数" width="100">
          <template #default="{ row }">
            {{ platformCount(row.platform) }}
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.updatedAt) }}
          </template>
        </el-table-column>
      </el-table>
    </section>
  </div>
</template>

<style scoped>
.platform-sync-page {
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
  background: linear-gradient(135deg, #fff 0%, #eff6ff 58%, #ecfdf5 100%);
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
  justify-content: flex-end;
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.status-grid > div,
.sync-panel,
.api-panel,
.settings-panel {
  padding: 16px;
  background: #fff;
  border: 1px solid rgb(148 163 184 / 18%);
  border-radius: 8px;
}

.status-grid span,
.panel-header span,
.job-meta {
  font-size: 12px;
  color: #64748b;
}

.status-grid strong,
.panel-header strong {
  display: block;
  margin-top: 6px;
  color: #0f172a;
}

.sync-panel {
  margin-bottom: 16px;
}

.api-panel {
  margin-bottom: 16px;
}

.api-form {
  max-width: 980px;
}

.field-tip {
  margin-top: 6px;
  font-size: 12px;
  color: #64748b;
}

.panel-header {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.job-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 10px;
}

.settings-table {
  width: 100%;
}

@media (max-width: 900px) {
  .page-hero {
    align-items: flex-start;
    flex-direction: column;
  }

  .status-grid {
    grid-template-columns: 1fr;
  }
}
</style>
