<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { getPlatformSyncControl, savePlatformSyncControl } from "@/api/system";
import { syncAllResources } from "@/api/business";

defineOptions({ name: "SystemPlatformSyncControl" });

const loading = ref(false);
const saving = ref(false);
const syncing = ref(false);
const syncDialogVisible = ref(false);
const syncScope = ref<"all" | "selected">("all");
const selectedSyncPlatforms = ref<string[]>(["YouTube", "Instagram", "TikTok"]);
const syncPlatformOptions = ["YouTube", "Instagram", "TikTok"];
const apiConfigDirty = ref(false);
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
const syncJobTagType = computed(() => {
  const status = latestJob.value?.status;
  if (status === "运行中" || status === "已中止") return "warning";
  if (status === "失败" || status === "部分失败") return "danger";
  return "success";
});
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
  if (!apiConfigDirty.value) {
    Object.assign(apiConfig.value, data.apiConfig || {});
    apiConfig.value.youtubeApiKey = "";
    apiConfig.value.instagramAccessToken = "";
    apiConfig.value.tiktokAccessToken = "";
    apiConfig.value.tikhubApiKey = "";
  }
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
    apiConfigDirty.value = false;
    await loadData();
    apiConfig.value.youtubeApiKey = typedSecrets.youtubeApiKey;
    apiConfig.value.instagramAccessToken = typedSecrets.instagramAccessToken;
    apiConfig.value.tiktokAccessToken = typedSecrets.tiktokAccessToken;
    apiConfig.value.tikhubApiKey = typedSecrets.tikhubApiKey;
  }
}

async function startSync() {
  syncDialogVisible.value = true;
}

async function confirmSync() {
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
  syncing.value = true;
  const res = await syncAllResources({ platforms });
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

    <el-dialog v-model="syncDialogVisible" title="启动异步同步" width="480px">
      <div class="sync-platform-dialog">
        <p>选择本次需要同步的平台。已停用的平台仍会按抓取控制设置跳过。</p>
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
        <el-button type="primary" :loading="syncing" @click="confirmSync">
          启动
        </el-button>
      </template>
    </el-dialog>

    <section v-if="latestJob" class="sync-panel">
      <div class="panel-header">
        <div>
          <strong>异步任务</strong>
          <span>{{ latestJob.message || "等待同步" }}</span>
        </div>
        <el-tag :type="syncJobTagType" effect="plain">
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
                @input="apiConfigDirty = true"
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
                @input="apiConfigDirty = true"
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
            <el-form-item label="YouTube 代理地址（可选）">
              <el-input
                v-model="apiConfig.youtubeProxyUrl"
                clearable
                placeholder="留空则直连，例如 http://127.0.0.1:7890"
                @input="apiConfigDirty = true"
              />
              <div class="field-tip">
                仅在服务器无法直接访问 Google API 时填写。
              </div>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="TikHub 接入说明">
              <el-input
                model-value="TikTok 与 Instagram 抓取共用该 Key"
                disabled
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </section>

    <section v-loading="loading" class="settings-panel">
      <div class="panel-header">
        <div>
          <strong>平台抓取设置</strong>
          <span>配置平台启用状态、是否抓取作品，以及每次默认抓取数量。</span>
        </div>
      </div>
      <el-table :data="settings" stripe class="settings-table">
        <el-table-column prop="platform" label="平台" width="140" />
        <el-table-column label="启用抓取" width="120">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" />
          </template>
        </el-table-column>
        <el-table-column label="抓取作品" width="120">
          <template #default="{ row }">
            <el-switch v-model="row.syncPosts" />
          </template>
        </el-table-column>
        <el-table-column label="默认作品数" width="150">
          <template #default="{ row }">
            <el-input-number
              v-model="row.postLimit"
              :min="1"
              :max="50"
              controls-position="right"
              class="post-limit-input"
            />
          </template>
        </el-table-column>
        <el-table-column label="Token 状态" min-width="220">
          <template #default="{ row }">
            <el-tag
              :type="
                tokenStatus[row.platform]?.configured ? 'success' : 'warning'
              "
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

.post-limit-input {
  width: 110px;
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
