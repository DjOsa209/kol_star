<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { getBusinessDashboard } from "@/api/business";

defineOptions({ name: "BusinessDashboard" });

type DistributionItem = {
  name: string;
  value: number;
};

const data = ref<Record<string, any>>({
  byCountry: [],
  byPlatform: [],
  byLevel: []
});

const metricCards = computed(() => [
  {
    key: "resourceTotal",
    label: "全球资源总量",
    hint: "已沉淀资源资产",
    color: "blue",
    value: data.value.resourceTotal ?? 0
  },
  {
    key: "activeResourceTotal",
    label: "可合作资源",
    hint: `可用率 ${rate(
      data.value.activeResourceTotal,
      data.value.resourceTotal
    )}%`,
    color: "green",
    value: data.value.activeResourceTotal ?? 0
  },
  {
    key: "saResourceTotal",
    label: "S/A 级资源",
    hint: `优质占比 ${rate(
      data.value.saResourceTotal,
      data.value.resourceTotal
    )}%`,
    color: "amber",
    value: data.value.saResourceTotal ?? 0
  },
  {
    key: "riskResourceTotal",
    label: "风险资源",
    hint: `风险占比 ${rate(
      data.value.riskResourceTotal,
      data.value.resourceTotal
    )}%`,
    color: "rose",
    value: data.value.riskResourceTotal ?? 0
  },
  {
    key: "projectTotal",
    label: "项目数",
    hint: "需求与投放闭环",
    color: "violet",
    value: data.value.projectTotal ?? 0
  },
  {
    key: "cooperationTotal",
    label: "合作记录",
    hint: "效果回填样本",
    color: "cyan",
    value: data.value.cooperationTotal ?? 0
  }
]);

const distributionSections = computed(() => [
  {
    title: "国家/地区分布",
    subtitle: "资源覆盖市场",
    data: normalizeDistribution(data.value.byCountry)
  },
  {
    title: "平台分布",
    subtitle: "渠道资产结构",
    data: normalizeDistribution(data.value.byPlatform)
  },
  {
    title: "评级分布",
    subtitle: "资源质量梯队",
    data: normalizeDistribution(data.value.byLevel)
  }
]);

const operationSignals = computed(() => [
  {
    label: "资源可用率",
    value: rate(data.value.activeResourceTotal, data.value.resourceTotal),
    type: "success"
  },
  {
    label: "优质资源占比",
    value: rate(data.value.saResourceTotal, data.value.resourceTotal),
    type: "warning"
  },
  {
    label: "风险资源占比",
    value: rate(data.value.riskResourceTotal, data.value.resourceTotal),
    type: "danger"
  }
]);

function numberText(value: unknown) {
  return Number(value || 0).toLocaleString("zh-CN");
}

function rate(value: unknown, total: unknown) {
  const num = Number(value || 0);
  const den = Number(total || 0);
  if (!den) return 0;
  return Math.round((num / den) * 100);
}

function normalizeDistribution(rows: DistributionItem[] = []) {
  const values = rows.map(item => Number(item.value || 0));
  const max = Math.max(...values, 1);
  return rows.map(item => ({
    ...item,
    percent: Math.round((Number(item.value || 0) / max) * 100)
  }));
}

async function loadData() {
  const res = await getBusinessDashboard();
  if (res.code === 0) data.value = res.data;
}

onMounted(loadData);
</script>

<template>
  <div class="business-dashboard">
    <section class="dashboard-hero">
      <div>
        <div class="eyebrow">Global KOL Operation Hub</div>
        <h1>全球传播资源智能运营平台</h1>
        <p>
          统一沉淀媒体、KOL、创作者与代理商资源，围绕质量、风险和项目复盘形成持续运营闭环。
        </p>
      </div>
      <div class="hero-panel">
        <span>今日运营关注</span>
        <strong>{{ numberText(data.riskResourceTotal) }}</strong>
        <small>个风险资源需要复核治理规则或合作记录</small>
      </div>
    </section>

    <section class="metric-grid">
      <article
        v-for="item in metricCards"
        :key="item.key"
        class="metric-card"
        :class="`metric-card--${item.color}`"
      >
        <div class="metric-meta">
          <span>{{ item.label }}</span>
          <i />
        </div>
        <strong>{{ numberText(item.value) }}</strong>
        <small>{{ item.hint }}</small>
      </article>
    </section>

    <section class="content-grid">
      <el-card shadow="never" class="ops-card">
        <template #header>
          <div class="section-header">
            <div>
              <strong>运营健康度</strong>
              <span>资源质量、可用状态与风险的核心比例</span>
            </div>
          </div>
        </template>
        <div class="signal-list">
          <div v-for="signal in operationSignals" :key="signal.label">
            <div class="signal-row">
              <span>{{ signal.label }}</span>
              <strong>{{ signal.value }}%</strong>
            </div>
            <el-progress
              :percentage="signal.value"
              :stroke-width="10"
              :show-text="false"
              :status="signal.type as any"
            />
          </div>
        </div>
      </el-card>

      <el-card shadow="never" class="workflow-card">
        <template #header>
          <div class="section-header">
            <div>
              <strong>推荐闭环</strong>
              <span>从自然语言需求到复盘评分</span>
            </div>
          </div>
        </template>
        <div class="workflow-list">
          <div
            v-for="step in ['需求解析', '规则过滤', '加入项目', '效果回填']"
            :key="step"
          >
            <span>{{ step }}</span>
          </div>
        </div>
      </el-card>
    </section>

    <section class="distribution-grid">
      <el-card
        v-for="section in distributionSections"
        :key="section.title"
        shadow="never"
        class="distribution-card"
      >
        <template #header>
          <div class="section-header">
            <div>
              <strong>{{ section.title }}</strong>
              <span>{{ section.subtitle }}</span>
            </div>
          </div>
        </template>
        <el-empty
          v-if="section.data.length === 0"
          description="暂无分布数据"
          :image-size="72"
        />
        <div v-else class="distribution-list">
          <div v-for="item in section.data" :key="item.name" class="dist-row">
            <div class="dist-label">
              <span>{{ item.name || "未填写" }}</span>
              <strong>{{ numberText(item.value) }}</strong>
            </div>
            <el-progress
              :percentage="item.percent"
              :stroke-width="8"
              :show-text="false"
            />
          </div>
        </div>
      </el-card>
    </section>
  </div>
</template>

<style scoped>
.business-dashboard {
  min-height: 100%;
  padding: 20px;
  background:
    linear-gradient(180deg, rgb(239 246 255 / 88%) 0%, rgb(248 250 252) 32%),
    #f8fafc;
}

.dashboard-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 20px;
  align-items: stretch;
  padding: 24px;
  margin-bottom: 16px;
  overflow: hidden;
  color: #0f172a;
  background:
    radial-gradient(circle at 88% 18%, rgb(14 165 233 / 18%), transparent 28%),
    linear-gradient(135deg, #fff 0%, #eff6ff 58%, #f0fdfa 100%);
  border: 1px solid rgb(148 163 184 / 24%);
  border-radius: 8px;
  box-shadow: 0 16px 38px rgb(15 23 42 / 8%);
}

.eyebrow {
  margin-bottom: 10px;
  font-size: 12px;
  font-weight: 700;
  color: #2563eb;
  text-transform: uppercase;
  letter-spacing: 0;
}

.dashboard-hero h1 {
  margin: 0;
  font-size: 30px;
  font-weight: 760;
  line-height: 1.24;
  letter-spacing: 0;
}

.dashboard-hero p {
  max-width: 760px;
  margin: 12px 0 0;
  font-size: 14px;
  line-height: 1.8;
  color: #475569;
}

.hero-panel {
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-height: 136px;
  padding: 18px;
  background: rgb(15 23 42 / 92%);
  border-radius: 8px;
}

.hero-panel span {
  font-size: 13px;
  color: #bfdbfe;
}

.hero-panel strong {
  margin-top: 8px;
  font-size: 38px;
  line-height: 1;
  color: #fff;
}

.hero-panel small {
  margin-top: 12px;
  line-height: 1.6;
  color: #cbd5e1;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.metric-card {
  min-height: 132px;
  padding: 16px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  box-shadow: 0 10px 24px rgb(15 23 42 / 5%);
}

.metric-meta {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: space-between;
  color: #64748b;
}

.metric-meta span {
  font-size: 13px;
  font-weight: 600;
}

.metric-meta i {
  width: 10px;
  height: 10px;
  background: var(--metric-color);
  border-radius: 99px;
  box-shadow: 0 0 0 5px var(--metric-soft);
}

.metric-card strong {
  display: block;
  margin-top: 18px;
  font-size: 30px;
  line-height: 1;
  color: #0f172a;
}

.metric-card small {
  display: block;
  margin-top: 12px;
  color: #64748b;
}

.metric-card--blue {
  --metric-color: #2563eb;
  --metric-soft: rgb(37 99 235 / 12%);
}

.metric-card--green {
  --metric-color: #059669;
  --metric-soft: rgb(5 150 105 / 12%);
}

.metric-card--amber {
  --metric-color: #d97706;
  --metric-soft: rgb(217 119 6 / 14%);
}

.metric-card--rose {
  --metric-color: #e11d48;
  --metric-soft: rgb(225 29 72 / 12%);
}

.metric-card--violet {
  --metric-color: #7c3aed;
  --metric-soft: rgb(124 58 237 / 12%);
}

.metric-card--cyan {
  --metric-color: #0891b2;
  --metric-soft: rgb(8 145 178 / 12%);
}

.content-grid,
.distribution-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.ops-card {
  grid-column: span 2;
}

.section-header {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.section-header div {
  display: grid;
  gap: 4px;
}

.section-header strong {
  font-size: 15px;
  color: #0f172a;
}

.section-header span {
  font-size: 12px;
  color: #64748b;
}

.signal-list {
  display: grid;
  gap: 18px;
}

.signal-row,
.dist-label {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.signal-row span,
.dist-label span {
  color: #475569;
}

.signal-row strong,
.dist-label strong {
  color: #0f172a;
}

.workflow-list {
  display: grid;
  gap: 10px;
}

.workflow-list div {
  position: relative;
  padding: 12px 12px 12px 34px;
  color: #334155;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.workflow-list div::before {
  position: absolute;
  top: 15px;
  left: 14px;
  width: 8px;
  height: 8px;
  content: "";
  background: #2563eb;
  border-radius: 99px;
}

.distribution-card {
  min-height: 320px;
}

.distribution-list {
  display: grid;
  gap: 14px;
}

:deep(.el-card) {
  border-radius: 8px;
}

:deep(.el-card__header) {
  padding: 16px 18px;
}

@media (width <= 1200px) {
  .metric-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (width <= 900px) {
  .dashboard-hero,
  .content-grid,
  .distribution-grid {
    grid-template-columns: 1fr;
  }

  .ops-card {
    grid-column: auto;
  }
}

@media (width <= 640px) {
  .business-dashboard {
    padding: 12px;
  }

  .dashboard-hero {
    padding: 18px;
  }

  .dashboard-hero h1 {
    font-size: 24px;
  }

  .metric-grid {
    grid-template-columns: 1fr;
  }
}
</style>
