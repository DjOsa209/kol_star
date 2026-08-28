<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from "vue";
import dayjs from "dayjs";
import { getProjectImportSyncJobs } from "@/api/system";

defineOptions({ name: "ProjectImportSyncMonitor" });

const loading = ref(false);
const rows = ref<any[]>([]);
const total = ref(0);
const query = reactive({
  status: "",
  uploader: "",
  pageSize: 10,
  currentPage: 1
});
let refreshTimer: number | undefined;

const statusOptions = ["运行中", "成功", "部分失败", "失败"];

function statusType(status: string) {
  if (status === "成功") return "success";
  if (status === "运行中") return "primary";
  if (status === "部分失败") return "warning";
  return "danger";
}

function timeText(value: unknown) {
  const timestamp = Number(value || 0);
  return timestamp ? dayjs(timestamp).format("YYYY-MM-DD HH:mm:ss") : "-";
}

function progressText(row: any) {
  const completed =
    Number(row.successCount || 0) + Number(row.failedCount || 0);
  return `${completed}/${Number(row.totalCount || 0)}`;
}

async function loadData() {
  loading.value = true;
  try {
    const res = await getProjectImportSyncJobs({ ...query });
    if (res.code !== 0 || !res.data) return;
    rows.value = res.data.list || [];
    total.value = Number(res.data.total || 0);
  } finally {
    loading.value = false;
  }
}

function search() {
  query.currentPage = 1;
  loadData();
}

function reset() {
  query.status = "";
  query.uploader = "";
  search();
}

function changePage(page: number) {
  query.currentPage = page;
  loadData();
}

function changePageSize(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadData();
}

onMounted(() => {
  loadData();
  refreshTimer = window.setInterval(() => {
    if (rows.value.some(row => row.status === "运行中")) loadData();
  }, 5000);
});

onUnmounted(() => {
  if (refreshTimer) window.clearInterval(refreshTimer);
});
</script>

<template>
  <div class="import-sync-monitor">
    <el-card shadow="never">
      <template #header>
        <div class="card-heading">
          <div>
            <h2>导入同步监控</h2>
            <p>集中查看项目导入后的后台同步结果、异常详情与上传人。</p>
          </div>
          <el-button :loading="loading" @click="loadData">刷新</el-button>
        </div>
      </template>

      <el-form :inline="true" :model="query" class="filters">
        <el-form-item label="同步状态">
          <el-select
            v-model="query.status"
            clearable
            placeholder="全部状态"
            class="status-filter"
          >
            <el-option
              v-for="status in statusOptions"
              :key="status"
              :label="status"
              :value="status"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="上传人">
          <el-input
            v-model="query.uploader"
            clearable
            placeholder="输入上传人"
            @keyup.enter="search"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="search">查询</el-button>
          <el-button @click="reset">重置</el-button>
        </el-form-item>
      </el-form>

      <el-table
        v-loading="loading"
        :data="rows"
        border
        stripe
        empty-text="暂无导入同步记录"
      >
        <el-table-column prop="id" label="任务 ID" width="90" align="center" />
        <el-table-column label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" effect="light">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="uploader"
          label="上传人"
          width="140"
          align="center"
        />
        <el-table-column label="同步进度" width="110" align="center">
          <template #default="{ row }">{{ progressText(row) }}</template>
        </el-table-column>
        <el-table-column
          prop="currentStage"
          label="当前阶段"
          width="120"
          align="center"
        />
        <el-table-column label="同步详情" min-width="420">
          <template #default="{ row }">
            <pre class="message-cell">{{ row.message || "-" }}</pre>
          </template>
        </el-table-column>
        <el-table-column label="开始时间" width="180" align="center">
          <template #default="{ row }">{{ timeText(row.startedAt) }}</template>
        </el-table-column>
        <el-table-column label="完成时间" width="180" align="center">
          <template #default="{ row }">{{ timeText(row.finishedAt) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-if="total > 0"
        class="pagination"
        background
        layout="total, sizes, prev, pager, next"
        :total="total"
        :page-size="query.pageSize"
        :current-page="query.currentPage"
        :page-sizes="[10, 20, 50, 100]"
        @current-change="changePage"
        @size-change="changePageSize"
      />
    </el-card>
  </div>
</template>

<style scoped>
.import-sync-monitor {
  padding: 20px;
}

.card-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-heading h2 {
  margin: 0;
  font-size: 18px;
}

.card-heading p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.filters {
  margin-bottom: 4px;
}

.status-filter {
  width: 160px;
}

.message-cell {
  margin: 0;
  font: inherit;
  line-height: 1.55;
  color: var(--el-text-color-regular);
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.pagination {
  justify-content: flex-end;
  margin-top: 18px;
}
</style>
