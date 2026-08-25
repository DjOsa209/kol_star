<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { fieldLabel } from "@/utils/fieldI18n";
import {
  createStandardImportOption,
  deleteStandardImportOption,
  getStandardImportOptions,
  updateStandardImportOption
} from "@/api/system";

defineOptions({ name: "SystemStandardImportOptions" });

const fieldDefinitions = [
  {
    key: "resourceType",
    label: "类型",
    description: "项目资源类型，对应模板 resourceType 列。",
    icon: "ri:user-star-line",
    readonly: false
  },
  {
    key: "category",
    label: "领域",
    description: "合作方内容领域，对应模板 category 列。",
    icon: "ri:price-tag-3-line",
    readonly: false
  },
  {
    key: "collaboratorTier",
    label: "层级",
    description:
      "系统按同名合作方的多平台粉丝数或媒体月独立访客（UMV）自动分级：头部 > 100 万，腰部 10 万–100 万，尾部 < 10 万。",
    icon: "ri:bar-chart-grouped-line",
    readonly: true
  },
  {
    key: "platform",
    label: "平台",
    description: "合作方所在平台，对应模板 platform 列。",
    icon: "ri:global-line",
    readonly: false
  },
  {
    key: "cooperationType",
    label: "合作类型",
    description: "项目合作方式，对应模板 collaborationType 列。",
    icon: "ri:shake-hands-line",
    readonly: false
  },
  {
    key: "contentType",
    label: "内容类型",
    description: "合作内容分类，对应模板 contentType 列。",
    icon: "ri:movie-2-line",
    readonly: false
  },
  {
    key: "projectDivision",
    label: "项目一级分类",
    description:
      "项目导入时的组织归属。保留“区域”选项可让导入人员继续选择世界国家和地区。",
    icon: "ri:organization-chart",
    readonly: false
  },
  {
    key: "projectProductLine",
    label: "项目产品线",
    description: "项目导入时的二级分类，用于统一项目名称中的产品线表述。",
    icon: "ri:archive-stack-line",
    readonly: false
  }
];

const loading = ref(false);
const rows = ref<any[]>([]);
const dialogVisible = ref(false);
const saving = ref(false);
const editingId = ref<number | null>(null);
const form = reactive({ fieldKey: "resourceType", value: "", sortOrder: 0 });

const groups = computed(() =>
  fieldDefinitions.map(field => ({
    ...field,
    options: rows.value.filter(row => row.fieldKey === field.key)
  }))
);

async function loadData() {
  loading.value = true;
  try {
    const res = await getStandardImportOptions();
    if (res.code === 0) {
      rows.value = Array.isArray(res.data) ? res.data : [];
    }
  } finally {
    loading.value = false;
  }
}

function openCreate(fieldKey: string) {
  editingId.value = null;
  Object.assign(form, { fieldKey, value: "", sortOrder: 0 });
  dialogVisible.value = true;
}

function openEdit(row: any) {
  editingId.value = Number(row.id);
  Object.assign(form, {
    fieldKey: row.fieldKey,
    value: String(row.value || ""),
    sortOrder: Number(row.sortOrder || 0)
  });
  dialogVisible.value = true;
}

async function saveOption() {
  const value = form.value.trim();
  if (!value) {
    ElMessage.warning("请输入选项值");
    return;
  }
  saving.value = true;
  const res = editingId.value
    ? await updateStandardImportOption({
        id: editingId.value,
        value,
        sortOrder: form.sortOrder
      })
    : await createStandardImportOption({
        fieldKey: form.fieldKey,
        value,
        sortOrder: form.sortOrder
      });
  saving.value = false;
  if (res.code !== 0) {
    ElMessage.warning(res.message || "保存失败");
    return;
  }
  ElMessage.success(editingId.value ? "选项已更新" : "选项已新增");
  dialogVisible.value = false;
  await loadData();
}

async function removeOption(row: any) {
  try {
    await ElMessageBox.confirm(
      `确认删除选项「${row.value}」吗？删除后新下载的模板将不再提供该选项。`,
      "删除标准选项",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" }
    );
  } catch {
    return;
  }
  const res = await deleteStandardImportOption({ id: row.id });
  if (res.code !== 0) {
    ElMessage.warning(res.message || "删除失败");
    return;
  }
  ElMessage.success("选项已删除");
  await loadData();
}

onMounted(loadData);
</script>

<template>
  <div v-loading="loading" class="standard-options-page">
    <section class="page-hero">
      <div>
        <span>Standard Fields</span>
        <h1>标准字段配置</h1>
        <p>
          维护项目统一模板和项目命名的预设选项。保存后，项目导入会立即使用最新配置。
        </p>
      </div>
    </section>

    <section class="option-grid">
      <article v-for="group in groups" :key="group.key" class="option-card">
        <header>
          <div class="field-icon">
            <IconifyIconOnline :icon="group.icon" />
          </div>
          <div>
            <h2>{{ group.label }}</h2>
            <p>{{ group.description }}</p>
          </div>
          <el-button
            v-if="!group.readonly"
            type="primary"
            plain
            @click="openCreate(group.key)"
          >
            <IconifyIconOnline icon="ri:add-line" class="mr-1" />
            新增
          </el-button>
        </header>

        <div class="option-list">
          <div v-for="item in group.options" :key="item.id" class="option-row">
            <div>
              <strong>{{ item.value }}</strong>
              <span>{{ item.source }} · 排序 {{ item.sortOrder }}</span>
            </div>
            <div v-if="!group.readonly">
              <el-button link type="primary" @click="openEdit(item)">
                编辑
              </el-button>
              <el-button link type="danger" @click="removeOption(item)">
                删除
              </el-button>
            </div>
            <el-tag v-else type="info" effect="plain">系统自动计算</el-tag>
          </div>
          <el-empty
            v-if="group.options.length === 0"
            description="暂无可用选项"
            :image-size="70"
          />
        </div>
      </article>
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑标准选项' : '新增标准选项'"
      width="460px"
    >
      <el-form label-position="top">
        <el-form-item :label="fieldLabel('字段')">
          <el-select v-model="form.fieldKey" class="w-full!" disabled>
            <el-option
              v-for="field in fieldDefinitions"
              :key="field.key"
              :label="field.label"
              :value="field.key"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="fieldLabel('选项值')" required>
          <el-input
            v-model="form.value"
            maxlength="128"
            show-word-limit
            placeholder="请输入标准选项"
            @keyup.enter="saveOption"
          />
        </el-form-item>
        <el-form-item :label="fieldLabel('排序')">
          <el-input-number
            v-model="form.sortOrder"
            :min="0"
            :max="9999"
            class="w-full!"
          />
          <span class="form-tip"
            >数字越小越靠前；填 0 时新增选项自动排在末尾。</span
          >
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveOption">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.standard-options-page {
  min-height: 100%;
  padding: 24px;
  background: #f5f7fb;
}

.page-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 26px 30px;
  margin-bottom: 20px;
  color: #fff;
  background: linear-gradient(135deg, #111827, #1d4ed8);
  border-radius: 18px;
  box-shadow: 0 14px 35px rgb(15 23 42 / 16%);
}

.page-hero span {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  color: #bfdbfe;
  text-transform: uppercase;
}

.page-hero h1 {
  margin: 6px 0 8px;
  font-size: 28px;
}

.page-hero p {
  max-width: 720px;
  margin: 0;
  color: #dbeafe;
}

.option-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.option-card {
  padding: 20px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 16px;
  box-shadow: 0 8px 24px rgb(15 23 42 / 6%);
}

.option-card header {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 12px;
  align-items: center;
  padding-bottom: 16px;
  border-bottom: 1px solid #eef2f7;
}

.field-icon {
  display: grid;
  width: 42px;
  height: 42px;
  font-size: 21px;
  color: #1d4ed8;
  background: #eff6ff;
  border-radius: 12px;
  place-items: center;
}

.option-card h2 {
  margin: 0;
  font-size: 17px;
  color: #111827;
}

.option-card p {
  margin: 4px 0 0;
  font-size: 13px;
  color: #6b7280;
}

.option-list {
  max-height: 390px;
  margin-top: 8px;
  overflow-y: auto;
}

.option-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 58px;
  padding: 9px 4px;
  border-bottom: 1px solid #f0f2f5;
}

.option-row > div:first-child {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.option-row strong {
  color: #1f2937;
}

.option-row span,
.form-tip {
  font-size: 12px;
  color: #9ca3af;
}

.form-tip {
  display: block;
  margin-top: 6px;
}

@media (width <= 900px) {
  .option-grid {
    grid-template-columns: 1fr;
  }

  .standard-options-page {
    padding: 14px;
  }
}
</style>
