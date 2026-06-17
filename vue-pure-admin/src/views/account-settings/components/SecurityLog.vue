<script setup lang="ts">
import dayjs from "dayjs";
import { getMineLogs } from "@/api/user";
import { reactive, ref, onMounted } from "vue";
import { deviceDetection } from "@pureadmin/utils";
import type { PaginationProps } from "@pureadmin/table";

defineOptions({
  name: "SecurityLog"
});

const loading = ref(true);
const dataList = ref([]);
const pagination = reactive<PaginationProps>({
  total: 0,
  pageSize: 10,
  currentPage: 1,
  background: true,
  layout: "prev, pager, next"
});
const columns: TableColumnList = [
  {
    label: "详情",
    prop: "summary",
    minWidth: 140
  },
  {
    label: "IP 地址",
    prop: "ip",
    minWidth: 100
  },
  {
    label: "地点",
    prop: "address",
    minWidth: 140
  },
  {
    label: "操作系统",
    prop: "system",
    minWidth: 100
  },
  {
    label: "浏览器类型",
    prop: "browser",
    minWidth: 100
  },
  {
    label: "状态",
    prop: "status",
    minWidth: 80,
    formatter: ({ status }) => (Number(status) === 1 ? "成功" : "失败")
  },
  {
    label: "时间",
    prop: "operatingTime",
    minWidth: 180,
    formatter: ({ operatingTime }) =>
      dayjs(operatingTime).format("YYYY-MM-DD HH:mm:ss")
  }
];

async function onSearch() {
  loading.value = true;
  const { code, data } = await getMineLogs({
    pageSize: pagination.pageSize,
    currentPage: pagination.currentPage
  });
  if (code === 0) {
    dataList.value = data.list;
    pagination.total = data.total;
    pagination.pageSize = data.pageSize;
    pagination.currentPage = data.currentPage;
  }

  setTimeout(() => {
    loading.value = false;
  }, 200);
}

function handleSizeChange(value: number) {
  pagination.pageSize = value;
  pagination.currentPage = 1;
  onSearch();
}

function handleCurrentChange(value: number) {
  pagination.currentPage = value;
  onSearch();
}

onMounted(() => {
  onSearch();
});
</script>

<template>
  <div :class="['min-w-45', deviceDetection() ? 'max-w-full' : 'max-w-[70%]']">
    <h3 class="my-8!">安全日志</h3>
    <pure-table
      row-key="id"
      table-layout="auto"
      :loading="loading"
      :data="dataList"
      :columns="columns"
      :pagination="pagination"
      @page-size-change="handleSizeChange"
      @page-current-change="handleCurrentChange"
    />
  </div>
</template>
