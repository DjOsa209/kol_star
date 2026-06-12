<script setup lang="ts">
import { reactive, ref, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { getBriefTemplateList, createBriefTemplate } from "@/api/business";

defineOptions({ name: "BusinessBriefs" });

const list = ref<any[]>([]);
const dialogVisible = ref(false);
const form = reactive({
  name: "",
  platform: "YouTube",
  market: "",
  contentType: "",
  language: "",
  status: "启用",
  owner: "",
  template: ""
});

async function loadData() {
  const { code, data } = await getBriefTemplateList();
  if (code === 0) list.value = data.list;
}

async function submit() {
  const res = await createBriefTemplate(form);
  if (res.code === 0) {
    ElMessage.success("模板已保存");
    dialogVisible.value = false;
    loadData();
  }
}

onMounted(loadData);
</script>

<template>
  <div class="business-page">
    <div class="toolbar">
      <el-button type="primary" @click="dialogVisible = true"
        >新增模板</el-button
      >
    </div>
    <el-table :data="list" border>
      <el-table-column prop="name" label="模板名称" min-width="180" />
      <el-table-column prop="platform" label="平台" width="120" />
      <el-table-column prop="market" label="市场" width="120" />
      <el-table-column prop="contentType" label="内容类型" width="140" />
      <el-table-column prop="language" label="语言" width="100" />
      <el-table-column prop="status" label="状态" width="90" />
      <el-table-column prop="owner" label="负责人" width="120" />
      <el-table-column
        prop="template"
        label="模板内容"
        min-width="300"
        show-overflow-tooltip
      />
    </el-table>

    <el-dialog v-model="dialogVisible" title="新增 Brief 模板" width="720px">
      <el-form :model="form" label-width="96px">
        <el-row :gutter="12">
          <el-col :span="12"
            ><el-form-item label="模板名称"
              ><el-input v-model="form.name" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="平台"
              ><el-input v-model="form.platform" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="市场"
              ><el-input v-model="form.market" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="内容类型"
              ><el-input v-model="form.contentType" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="语言"
              ><el-input v-model="form.language" /></el-form-item
          ></el-col>
          <el-col :span="12"
            ><el-form-item label="负责人"
              ><el-input v-model="form.owner" /></el-form-item
          ></el-col>
          <el-col :span="24"
            ><el-form-item label="模板内容"
              ><el-input
                v-model="form.template"
                type="textarea"
                :rows="8" /></el-form-item
          ></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.business-page {
  padding: 16px;
}

.toolbar {
  margin-bottom: 12px;
}
</style>
