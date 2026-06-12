<script setup lang="ts">
import { reactive, ref, computed, onMounted } from "vue";
import { ElMessage } from "element-plus";
import AddLine from "~icons/ri/add-line";
import { getTagList, createTag } from "@/api/business";

defineOptions({ name: "BusinessTags" });

type TagItem = {
  id: number;
  name: string;
  category: string;
  color: string;
  status: string;
};

const list = ref<TagItem[]>([]);
const dialogVisible = ref(false);
const form = reactive({
  name: "",
  category: "基础标签",
  color: "#409EFF",
  status: "启用"
});

const categoryMeta: Record<string, { desc: string; tone: string }> = {
  基础标签: { desc: "国家、语言、平台等资源基础画像", tone: "blue" },
  内容标签: { desc: "内容领域、行业主题与垂类方向", tone: "amber" },
  人群标签: { desc: "受众圈层、粉丝画像与传播对象", tone: "violet" },
  能力标签: { desc: "交付形式、创作能力与内容风格", tone: "slate" },
  合作标签: { desc: "履约表现、合作状态与复盘信号", tone: "green" },
  风险标签: { desc: "异常数据、舆情风险与治理标记", tone: "rose" },
  自定义标签: { desc: "导入或运营过程中沉淀的补充标签", tone: "cyan" }
};

const categoryOrder = Object.keys(categoryMeta);

const categoryGroups = computed(() => {
  const groups = list.value.reduce<Record<string, TagItem[]>>((acc, item) => {
    const category = item.category || "未分类";
    acc[category] ||= [];
    acc[category].push(item);
    return acc;
  }, {});

  return Object.entries(groups)
    .sort(([a], [b]) => {
      const aIndex = categoryOrder.indexOf(a);
      const bIndex = categoryOrder.indexOf(b);
      if (aIndex === -1 && bIndex === -1) return a.localeCompare(b, "zh-CN");
      if (aIndex === -1) return 1;
      if (bIndex === -1) return -1;
      return aIndex - bIndex;
    })
    .map(([category, items]) => ({
      category,
      items,
      meta: categoryMeta[category] || {
        desc: "运营自定义分类，按当前标签数据自动归组",
        tone: "slate"
      }
    }));
});

const totalCount = computed(() => list.value.length);
const activeCount = computed(
  () => list.value.filter(item => item.status === "启用").length
);
const categoryCount = computed(() => categoryGroups.value.length);

async function loadData() {
  const { code, data } = await getTagList();
  if (code === 0) list.value = data;
}

async function submit() {
  const res = await createTag(form);
  if (res.code === 0) {
    ElMessage.success("保存成功");
    dialogVisible.value = false;
    form.name = "";
    loadData();
  }
}

onMounted(loadData);
</script>

<template>
  <div class="tag-system-page">
    <section class="page-header">
      <div>
        <span class="eyebrow">Tag Taxonomy</span>
        <h1>标签体系</h1>
        <p>按分类平铺展示资源标签，快速识别颜色、状态与运营归属。</p>
      </div>
      <el-button type="primary" :icon="AddLine" @click="dialogVisible = true">
        新增标签
      </el-button>
    </section>

    <section class="summary-grid">
      <div class="summary-card summary-card--blue">
        <span>标签总量</span>
        <strong>{{ totalCount }}</strong>
      </div>
      <div class="summary-card summary-card--green">
        <span>启用标签</span>
        <strong>{{ activeCount }}</strong>
      </div>
      <div class="summary-card summary-card--slate">
        <span>分类数量</span>
        <strong>{{ categoryCount }}</strong>
      </div>
    </section>

    <el-empty v-if="categoryGroups.length === 0" description="暂无标签" />

    <section v-else class="category-grid">
      <article
        v-for="group in categoryGroups"
        :key="group.category"
        class="category-card"
        :class="`category-card--${group.meta.tone}`"
      >
        <header class="category-header">
          <div>
            <span class="category-kicker">{{ group.items.length }} 个标签</span>
            <h2>{{ group.category }}</h2>
            <p>{{ group.meta.desc }}</p>
          </div>
          <i />
        </header>

        <div class="tag-grid">
          <div
            v-for="item in group.items"
            :key="item.id"
            class="tag-card"
            :style="{ '--tag-color': item.color || '#2563eb' }"
          >
            <span class="tag-dot" />
            <span class="tag-name">{{ item.name }}</span>
            <span
              class="tag-status"
              :class="{ 'is-disabled': item.status !== '启用' }"
            >
              {{ item.status }}
            </span>
          </div>
        </div>
      </article>
    </section>

    <el-dialog v-model="dialogVisible" title="新增标签" width="420px">
      <el-form :model="form" label-width="88px">
        <el-form-item label="标签名称"
          ><el-input v-model="form.name"
        /></el-form-item>
        <el-form-item label="标签分类">
          <el-select v-model="form.category" class="w-full!">
            <el-option label="基础标签" value="基础标签" />
            <el-option label="内容标签" value="内容标签" />
            <el-option label="人群标签" value="人群标签" />
            <el-option label="能力标签" value="能力标签" />
            <el-option label="合作标签" value="合作标签" />
            <el-option label="风险标签" value="风险标签" />
          </el-select>
        </el-form-item>
        <el-form-item label="颜色"
          ><el-color-picker v-model="form.color"
        /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.tag-system-page {
  min-height: 100%;
  padding: 20px;
  background: #f8fafc;
}

.page-header {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  padding: 22px 24px;
  margin-bottom: 16px;
  background:
    linear-gradient(135deg, rgb(255 255 255 / 96%) 0%, rgb(240 253 250) 100%),
    #fff;
  border: 1px solid #dbe4ee;
  border-radius: 8px;
  box-shadow: 0 14px 32px rgb(15 23 42 / 6%);
}

.eyebrow,
.category-kicker {
  display: block;
  font-size: 12px;
  font-weight: 700;
  color: #0f766e;
  text-transform: uppercase;
  letter-spacing: 0;
}

.page-header h1 {
  margin: 6px 0 0;
  font-size: 26px;
  font-weight: 760;
  line-height: 1.25;
  color: #0f172a;
  letter-spacing: 0;
}

.page-header p {
  margin: 8px 0 0;
  font-size: 14px;
  line-height: 1.7;
  color: #64748b;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.summary-card {
  min-height: 96px;
  padding: 16px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  box-shadow: 0 10px 24px rgb(15 23 42 / 5%);
}

.summary-card span {
  font-size: 13px;
  font-weight: 600;
  color: #64748b;
}

.summary-card strong {
  display: block;
  margin-top: 12px;
  font-size: 28px;
  line-height: 1;
  color: var(--summary-color);
}

.summary-card--blue {
  --summary-color: #2563eb;
}

.summary-card--green {
  --summary-color: #059669;
}

.summary-card--slate {
  --summary-color: #334155;
}

.category-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.category-card {
  min-height: 268px;
  padding: 18px;
  overflow: hidden;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  box-shadow: 0 12px 26px rgb(15 23 42 / 5%);
}

.category-header {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  justify-content: space-between;
  padding-bottom: 14px;
  margin-bottom: 14px;
  border-bottom: 1px solid #eef2f7;
}

.category-header h2 {
  margin: 6px 0 0;
  font-size: 18px;
  font-weight: 740;
  line-height: 1.3;
  color: #0f172a;
  letter-spacing: 0;
}

.category-header p {
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 1.7;
  color: #64748b;
}

.category-header i {
  flex: 0 0 auto;
  width: 12px;
  height: 12px;
  margin-top: 4px;
  background: var(--category-color);
  border-radius: 99px;
  box-shadow: 0 0 0 6px var(--category-soft);
}

.tag-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.tag-card {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  max-width: 100%;
  min-height: 34px;
  padding: 7px 10px;
  background: color-mix(in srgb, var(--tag-color) 8%, #fff);
  border: 1px solid color-mix(in srgb, var(--tag-color) 26%, #e2e8f0);
  border-radius: 8px;
}

.tag-dot {
  flex: 0 0 auto;
  width: 8px;
  height: 8px;
  background: var(--tag-color);
  border-radius: 99px;
}

.tag-name {
  max-width: 132px;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 13px;
  font-weight: 650;
  color: #1e293b;
  white-space: nowrap;
}

.tag-status {
  flex: 0 0 auto;
  padding: 2px 6px;
  font-size: 12px;
  line-height: 1.2;
  color: #047857;
  background: rgb(16 185 129 / 12%);
  border-radius: 999px;
}

.tag-status.is-disabled {
  color: #64748b;
  background: #e2e8f0;
}

.category-card--blue {
  --category-color: #2563eb;
  --category-soft: rgb(37 99 235 / 12%);
}

.category-card--amber {
  --category-color: #d97706;
  --category-soft: rgb(217 119 6 / 14%);
}

.category-card--violet {
  --category-color: #7c3aed;
  --category-soft: rgb(124 58 237 / 12%);
}

.category-card--slate {
  --category-color: #475569;
  --category-soft: rgb(71 85 105 / 12%);
}

.category-card--green {
  --category-color: #059669;
  --category-soft: rgb(5 150 105 / 12%);
}

.category-card--rose {
  --category-color: #e11d48;
  --category-soft: rgb(225 29 72 / 12%);
}

.category-card--cyan {
  --category-color: #0891b2;
  --category-soft: rgb(8 145 178 / 12%);
}

@media (width <= 1200px) {
  .category-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (width <= 768px) {
  .tag-system-page {
    padding: 12px;
  }

  .page-header {
    display: grid;
    align-items: flex-start;
    padding: 18px;
  }

  .page-header,
  .summary-grid,
  .category-grid {
    grid-template-columns: 1fr;
  }
}
</style>
