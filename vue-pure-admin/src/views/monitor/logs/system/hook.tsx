import dayjs from "dayjs";
import Detail from "./detail.vue";
import { message } from "@/utils/message";
import { addDialog } from "@/components/ReDialog";
import type { PaginationProps } from "@pureadmin/table";
import { reactive, ref, onMounted, toRaw } from "vue";
import { useCopyToClipboard } from "@pureadmin/utils";
import { getSystemLogsList, getSystemLogsDetail } from "@/api/system";
import Info from "~icons/ri/question-line";

export function useRole() {
  const form = reactive({
    module: "",
    requestTime: [] as Date[]
  });
  const dataList = ref([]);
  const loading = ref(true);
  const { copied, update } = useCopyToClipboard();

  const pagination = reactive<PaginationProps>({
    total: 0,
    pageSize: 10,
    currentPage: 1,
    background: true
  });

  const columns: TableColumnList = [
    {
      label: "ID",
      prop: "id",
      minWidth: 90
    },
    {
      label: "所属模块",
      prop: "module",
      minWidth: 100
    },
    {
      headerRenderer: () => (
        <span class="flex-c">
          请求接口
          <iconify-icon-offline
            icon={Info}
            class="ml-1 cursor-help"
            v-tippy={{
              content: "双击下面请求接口进行拷贝"
            }}
          />
        </span>
      ),
      prop: "url",
      minWidth: 140
    },
    {
      label: "请求方法",
      prop: "method",
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
      label: "请求耗时",
      prop: "takesTime",
      minWidth: 100,
      cellRenderer: ({ row, props }) => (
        <el-tag
          size={props.size}
          type={row.takesTime < 1000 ? "success" : "warning"}
          effect="plain"
        >
          {row.takesTime} ms
        </el-tag>
      )
    },
    {
      label: "请求时间",
      prop: "requestTime",
      minWidth: 180,
      formatter: ({ requestTime }) =>
        dayjs(requestTime).format("YYYY-MM-DD HH:mm:ss")
    },
    {
      label: "操作",
      fixed: "right",
      slot: "operation"
    }
  ];

  function handleSizeChange(val: number) {
    pagination.pageSize = val;
    pagination.currentPage = 1;
    void load();
  }

  function handleCurrentChange(val: number) {
    pagination.currentPage = val;
    void load();
  }

  /** 拷贝请求接口，表格单元格被双击时触发 */
  function handleCellDblclick({ url }, { property }) {
    if (property !== "url") return;
    update(url);
    copied.value
      ? message(`${url} 已拷贝`, { type: "success" })
      : message("拷贝失败", { type: "warning" });
  }

  function onDetail(row) {
    getSystemLogsDetail({ id: row.id }).then(res => {
      if (res.code !== 0 || !res.data) return;
      addDialog({
        title: "系统日志详情",
        fullscreen: true,
        hideFooter: true,
        contentRenderer: () => Detail,
        props: {
          data: [res.data]
        }
      });
    });
  }

  async function load() {
    loading.value = true;
    try {
      const { code, data } = await getSystemLogsList({
        ...toRaw(form),
        pageSize: pagination.pageSize,
        currentPage: pagination.currentPage
      });
      if (code === 0 && data) {
        dataList.value = data.list ?? [];
        pagination.total = data.total ?? 0;
        pagination.pageSize = data.pageSize ?? pagination.pageSize;
        pagination.currentPage = data.currentPage ?? pagination.currentPage;
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

  const resetForm = formEl => {
    if (!formEl) return;
    formEl.resetFields();
    onSearch();
  };

  onMounted(() => {
    onSearch();
  });

  return {
    form,
    loading,
    columns,
    dataList,
    pagination,
    onSearch,
    onDetail,
    resetForm,
    handleSizeChange,
    handleCellDblclick,
    handleCurrentChange
  };
}
