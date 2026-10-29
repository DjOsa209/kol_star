<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import {
  createTrendLocalJob,
  getTrendLocalJob,
  cancelTrendLocalJob,
  downloadTrendLocalClient,
  type TrendLocalJob,
  type TrendQuery
} from "@/api/trends";
const props = defineProps<{
  modelValue: boolean;
  query: TrendQuery;
  regions: string[];
}>();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  complete: [query: TrendQuery];
  active: [value: boolean];
}>();
const visible = computed({
  get: () => props.modelValue,
  set: value => emit("update:modelValue", value)
});
const platform = ref("darwin-arm64");
const selectedRegions = ref<string[]>([]);
const regionNames = new Intl.DisplayNames(["zh-CN"], { type: "region" });
const regionLabel = (region: string) =>
  `${regionNames.of(region) || region} (${region})`;
const job = ref<TrendLocalJob>();
const error = ref("");
const busy = ref(false);
const downloading = ref(false);
const active = computed(
  () => job.value && ["waiting_upload", "processing"].includes(job.value.state)
);
let taskFile = "";
let submitted: TrendQuery;
let timer: ReturnType<typeof setTimeout>;
let disposed = false;
const filename = computed(
  () =>
    `kol-trend-collector-${platform.value}${platform.value.startsWith("windows") ? ".exe" : ""}`
);
const taskFilename = computed(() =>
  job.value ? `trend-job-${job.value.id}.json` : "trend-job.json"
);
const command = computed(() =>
  platform.value.startsWith("windows")
    ? `.\\${filename.value} local-trends --job ${taskFilename.value}`
    : `chmod +x ./${filename.value}\n./${filename.value} local-trends --job ${taskFilename.value}`
);
function download(blob: Blob, name: string) {
  const link = document.createElement("a");
  const href = URL.createObjectURL(blob);
  link.href = href;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(href), 30000);
}
function downloadTask() {
  if (job.value && Date.now() >= Date.parse(job.value.expiresAt)) {
    error.value = "本次授权已过期，请点击重新采集获取新的任务。";
    clearTimeout(timer);
    void poll();
    return;
  }
  if (taskFile)
    download(
      new Blob([taskFile], { type: "application/json" }),
      taskFilename.value
    );
}
async function begin() {
  if (busy.value || active.value) return;
  if (!selectedRegions.value.length) {
    error.value = "请至少选择一个采集地区。";
    return;
  }
  busy.value = true;
  error.value = "";
  try {
    submitted = { ...props.query, region: selectedRegions.value[0] };
    const response = await createTrendLocalJob({
      ...submitted,
      regions: selectedRegions.value.join(",")
    });
    if (response.code !== 0) throw new Error("create failed");
    if (disposed) return;
    const { token, ...status } = response.data;
    job.value = status;
    const server = new URL(
      import.meta.env.VITE_API_BASE_URL || "/api",
      window.location.origin
    ).href.replace(/\/$/, "");
    taskFile = JSON.stringify(
      {
        version: 2,
        server,
        job: status.id,
        token,
        expiresAt: status.expiresAt,
        regions: status.regions.map(item => item.region),
        query: {
          tab: submitted.tab,
          region: submitted.region,
          period: String(submitted.period),
          orderByMetric: String(submitted.orderByMetric)
        }
      },
      null,
      2
    );
    emit("active", true);
    poll();
  } catch {
    error.value = "无法创建本地采集授权，请检查后台登录或服务器配置。";
  } finally {
    busy.value = false;
  }
}
async function poll() {
  if (disposed || !job.value || !active.value) return;
  try {
    const response = await getTrendLocalJob(job.value.id);
    if (disposed) return;
    if (response.code !== 0) throw new Error("status failed");
    job.value = response.data;
    error.value = "";
    if (!active.value) {
      taskFile = "";
      emit("active", false);
    }
    if (["complete", "partial"].includes(job.value.state)) {
      for (const completed of job.value.regions.filter(
        item => item.state === "complete"
      )) {
        emit("complete", { ...submitted, region: completed.region });
      }
    }
  } catch {
    if (!disposed) error.value = "暂时无法读取上传状态，请检查网络。";
  }
  if (!disposed && active.value) timer = setTimeout(poll, 3000);
}
async function client() {
  downloading.value = true;
  error.value = "";
  try {
    const blob = await downloadTrendLocalClient(platform.value);
    if (blob.type.includes("json")) throw new Error("unpublished");
    download(blob, filename.value);
  } catch {
    error.value =
      "该平台的采集程序尚未发布或下载失败，请联系管理员运行构建脚本。";
  } finally {
    downloading.value = false;
  }
}
async function cancel() {
  if (!job.value) return;
  try {
    const response = await cancelTrendLocalJob(job.value.id);
    if (response.code !== 0) throw new Error("cancel failed");
    job.value = response.data;
    taskFile = "";
    emit("active", false);
    clearTimeout(timer);
  } catch {
    error.value = "取消失败，请重试。";
  }
}
watch(
  () => props.modelValue,
  value => {
    if (value && !active.value) {
      selectedRegions.value = [props.query.region];
      job.value = undefined;
      error.value = "";
      taskFile = "";
    }
  }
);
onBeforeUnmount(() => {
  disposed = true;
  clearTimeout(timer);
  taskFile = "";
});
</script>
<template>
  <el-dialog
    v-model="visible"
    title="本地采集并上传"
    width="min(760px, 94vw)"
    :close-on-click-modal="false"
    append-to-body
  >
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      show-icon
    />
    <p role="status">{{ busy ? "正在创建本次采集授权…" : job?.message }}</p>
    <p>采集地区（最多 5 个，按选择顺序依次采集）</p>
    <el-select
      v-model="selectedRegions"
      multiple
      filterable
      :multiple-limit="5"
      :disabled="Boolean(active) || busy"
      aria-label="采集地区"
      style="width: 100%"
    >
      <el-option
        v-for="region in regions"
        :key="region"
        :label="regionLabel(region)"
        :value="region"
      />
    </el-select>
    <el-table
      v-if="job?.regions?.length"
      :data="job.regions"
      size="small"
      style="margin-top: 16px"
    >
      <el-table-column label="地区" width="150"
        ><template #default="{ row }">{{
          regionLabel(row.region)
        }}</template></el-table-column
      >
      <el-table-column prop="message" label="进度" />
    </el-table>
    <template v-if="job?.state === 'waiting_upload'">
      <p>
        在你的电脑登录 Google 和 TikTok
        Ads，采集程序只上传榜单、详情及图片链接。服务器负责保存图片和更新缓存。
      </p>
      <el-select v-model="platform" aria-label="本机系统" style="width: 240px">
        <el-option label="macOS · Apple 芯片" value="darwin-arm64" /><el-option
          label="macOS · Intel"
          value="darwin-amd64"
        /><el-option label="Windows · x64" value="windows-amd64" /><el-option
          label="Linux · x64"
          value="linux-amd64"
        />
      </el-select>
      <el-button :loading="downloading" @click="client">下载采集程序</el-button>
      <el-button type="primary" @click="downloadTask">下载本次任务</el-button>
      <p>将两个文件放在同一个目录，在该目录打开终端并执行：</p>
      <pre class="local-command">{{ command }}</pre>
      <p>
        先在普通 Chrome 登录 Ads，再打开
        <code>chrome://inspect/#remote-debugging</code>
        启用调试。运行时确认上传目的地，并允许 Chrome 的连接提示。
      </p>
      <p>
        任务文件包含本次上传授权，有效期至
        {{ new Date(job.expiresAt).toLocaleString() }}，请勿分享。无需安装扩展或
        Node.js。
      </p>
    </template>
    <el-alert
      v-if="job?.state === 'processing'"
      title="上传已收到，服务器正在下载图片。可以收起窗口，稍后查看结果。"
      type="info"
      :closable="false"
    />
    <el-result
      v-if="job?.state === 'complete' || job?.state === 'partial'"
      :icon="
        job.imagesFailed || job.state === 'partial' ? 'warning' : 'success'
      "
      :title="job.message"
      :sub-title="`图片已保存 ${job.imagesSaved} 张，未保存 ${job.imagesFailed} 张。`"
    />
    <template #footer>
      <el-button v-if="active" @click="cancel">取消本次授权</el-button>
      <el-button v-if="!active && !busy" type="primary" @click="begin">{{
        job ? "重新采集所选地区" : "生成采集任务"
      }}</el-button>
      <el-button @click="visible = false">{{
        active ? "收起窗口" : "关闭"
      }}</el-button>
    </template>
  </el-dialog>
</template>
<style scoped>
.local-command {
  padding: 16px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
  overflow-x: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
p {
  margin: 14px 0;
  line-height: 1.65;
}
</style>
