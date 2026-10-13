import dayjs from "dayjs";
import { getOperationLogsList } from "@/api/system";
import { usePublicHooks } from "@/views/system/hooks";
import type { PaginationProps } from "@pureadmin/table";
import { reactive, ref, onMounted } from "vue";

export function useRole() {
  const form = reactive({
    module: "",
    status: "",
    operatingTime: [] as Date[]
  });
  const dataList = ref([]);
  const loading = ref(false);
  const { tagStyle } = usePublicHooks();
  const pagination = reactive<PaginationProps>({
    total: 0,
    pageSize: 10,
    currentPage: 1,
    background: true
  });
  const columns: TableColumnList = [
    { label: "序号", prop: "id", minWidth: 80 },
    { label: "操作人员", prop: "username", minWidth: 120 },
    { label: "所属模块", prop: "module", minWidth: 130 },
    { label: "操作概要", prop: "summary", minWidth: 180 },
    { label: "请求方法", prop: "method", minWidth: 110 },
    { label: "操作 IP", prop: "ip", minWidth: 140 },
    { label: "操作地点", prop: "address", minWidth: 140 },
    { label: "操作系统", prop: "system", minWidth: 100 },
    { label: "浏览器", prop: "browser", minWidth: 100 },
    {
      label: "操作状态",
      prop: "status",
      minWidth: 100,
      cellRenderer: ({ row, props }) => (
        <el-tag size={props.size} style={tagStyle.value(row.status)}>
          {row.status === 1 ? "成功" : "失败"}
        </el-tag>
      )
    },
    {
      label: "操作时间",
      prop: "operationTime",
      minWidth: 180,
      formatter: ({ operationTime }) =>
        dayjs(operationTime).format("YYYY-MM-DD HH:mm:ss")
    }
  ];

  async function load() {
    loading.value = true;
    try {
      const { code, data } = await getOperationLogsList({
        ...form,
        pageSize: pagination.pageSize,
        currentPage: pagination.currentPage
      });
      if (code === 0 && data) {
        dataList.value = data.list ?? [];
        pagination.total = data.total ?? 0;
        pagination.currentPage = data.currentPage ?? 1;
      }
    } catch {
      dataList.value = [];
      pagination.total = 0;
    } finally {
      loading.value = false;
    }
  }

  function onSearch() {
    pagination.currentPage = 1;
    void load();
  }

  function handleSizeChange(size: number) {
    pagination.pageSize = size;
    pagination.currentPage = 1;
    void load();
  }

  function handleCurrentChange(page: number) {
    pagination.currentPage = page;
    void load();
  }

  function resetForm(formEl) {
    if (!formEl) return;
    formEl.resetFields();
    onSearch();
  }

  onMounted(onSearch);

  return {
    form,
    loading,
    columns,
    dataList,
    pagination,
    onSearch,
    resetForm,
    handleSizeChange,
    handleCurrentChange
  };
}
