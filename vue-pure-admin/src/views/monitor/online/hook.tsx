import dayjs from "dayjs";
import { message } from "@/utils/message";
import { forceOnlineSessionOffline, getOnlineLogsList } from "@/api/system";
import { reactive, ref, onMounted } from "vue";
import type { PaginationProps } from "@pureadmin/table";

export function useRole() {
  const form = reactive({ username: "" });
  const dataList = ref([]);
  const loading = ref(false);
  const pagination = reactive<PaginationProps>({
    total: 0,
    pageSize: 10,
    currentPage: 1,
    background: true
  });
  const columns: TableColumnList = [
    { label: "序号", prop: "id", minWidth: 70 },
    { label: "用户名", prop: "username", minWidth: 120 },
    { label: "昵称", prop: "nickname", minWidth: 120 },
    { label: "登录 IP", prop: "ip", minWidth: 140 },
    { label: "登录地点", prop: "address", minWidth: 140 },
    { label: "操作系统", prop: "system", minWidth: 100 },
    { label: "浏览器", prop: "browser", minWidth: 100 },
    {
      label: "登录时间",
      prop: "loginTime",
      minWidth: 180,
      formatter: ({ loginTime }) =>
        dayjs(loginTime).format("YYYY-MM-DD HH:mm:ss")
    },
    {
      label: "最近活动",
      prop: "lastActiveTime",
      minWidth: 180,
      formatter: ({ lastActiveTime }) =>
        dayjs(lastActiveTime).format("YYYY-MM-DD HH:mm:ss")
    },
    { label: "操作", fixed: "right", slot: "operation" }
  ];

  async function load() {
    loading.value = true;
    try {
      const { code, data } = await getOnlineLogsList({
        username: form.username,
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

  async function handleOffline(row: { id: number; username: string }) {
    try {
      const { code, message: error } = await forceOnlineSessionOffline({
        id: row.id
      });
      if (code !== 0) {
        message(error || "强制下线失败", { type: "error" });
        return;
      }
      message(`${row.username} 的当前会话已下线`, { type: "success" });
      await load();
    } catch {
      message("强制下线失败，请稍后重试", { type: "error" });
    }
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
    handleOffline,
    handleSizeChange,
    handleCurrentChange
  };
}
