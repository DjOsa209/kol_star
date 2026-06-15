<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import echarts from "@/plugins/echarts";
import { getBusinessDashboard } from "@/api/business";

defineOptions({ name: "BusinessDashboard" });

type DistributionItem = { name: string; value: number };
type TrendItem = {
  date: string;
  postCount: number;
  exposure: number;
  interactions: number;
};
type RankingItem = {
  id: number;
  name: string;
  platform: string;
  postCount: number;
  exposure: number;
  interactions: number;
  engagementRate: number;
};

const router = useRouter();
const loading = ref(false);
const updatedAt = ref<Date | null>(null);
const trendChartRef = ref<HTMLElement>();
const rankingMode = ref<"exposure" | "interactions" | "engagementRate">(
  "exposure"
);
const advancedVisible = ref(false);
const dateRange = ref<[string, string]>(defaultDateRange());
const filters = ref({
  country: "",
  resourceType: "",
  platform: ""
});
const data = ref<Record<string, any>>({
  byCountry: [],
  byPlatform: [],
  trend: [],
  topResources: []
});
let trendChart: ReturnType<typeof echarts.init> | undefined;

const metricCards = computed(() => [
  {
    key: "totalPostCount",
    label: "总发布数",
    value: numberText(data.value.totalPostCount),
    hint: `${numberText(data.value.hotPostCount)} 条百万级爆款`,
    color: "#16a34a",
    path: "M2 31 L13 27 L24 29 L35 19 L46 22 L57 12 L68 17 L79 8 L90 15 L101 6"
  },
  {
    key: "totalPostViews",
    label: "总曝光",
    value: compactNumber(data.value.totalPostViews),
    hint: "当前筛选内容累计播放",
    color: "#2563eb",
    path: "M2 30 L13 34 L24 26 L35 28 L46 17 L57 23 L68 11 L79 18 L90 8 L101 16"
  },
  {
    key: "totalPostInteractions",
    label: "总互动",
    value: compactNumber(data.value.totalPostInteractions),
    hint: "点赞、评论与分享汇总",
    color: "#7c3aed",
    path: "M2 33 L13 29 L24 31 L35 22 L46 25 L57 13 L68 19 L79 9 L90 17 L101 7"
  },
  {
    key: "hotPostCount",
    label: "爆款",
    value: numberText(data.value.hotPostCount),
    hint: "单条曝光达到 1M",
    color: "#ea580c",
    path: "M2 31 L13 30 L24 21 L35 25 L46 17 L57 26 L68 18 L79 28 L90 7 L101 19"
  },
  {
    key: "postEngagementRate",
    label: "平均互动率",
    value: percentText(data.value.postEngagementRate),
    hint: "互动量 / 曝光量",
    color: "#0891b2",
    path: "M2 32 L13 33 L24 26 L35 27 L46 18 L57 24 L68 14 L79 20 L90 10 L101 18"
  }
]);

const insightCards = computed(() => {
  const top = rankedResources.value[0];
  const postCount = Number(data.value.totalPostCount || 0);
  const hotCount = Number(data.value.hotPostCount || 0);
  const engagementRate = Number(data.value.postEngagementRate || 0);
  const activeResources = Number(data.value.activeResourceTotal || 0);
  const resourceTotal = Number(data.value.resourceTotal || 0);
  return [
    {
      title: top ? `${top.name} 贡献最高曝光` : "当前暂无内容曝光",
      detail: top
        ? `${top.platform || "未知平台"}累计 ${compactNumber(top.exposure)} 曝光，建议复用其内容方向。`
        : "调整筛选条件或同步平台内容后，可生成头部资源洞察。"
    },
    {
      title: `百万级爆款占比 ${rate(hotCount, postCount)}%`,
      detail: hotCount
        ? `${numberText(hotCount)} 条内容达到百万曝光，可进入案例库沉淀。`
        : "当前筛选范围尚未出现百万级内容，建议复盘选题与发布时间。"
    },
    {
      title: `整体互动率 ${percentText(engagementRate)}`,
      detail:
        engagementRate >= 0.05
          ? "互动效率表现较好，可优先扩展相似资源与内容主题。"
          : "曝光转化为互动的效率偏低，建议优化内容钩子与 CTA。"
    },
    {
      title: `可合作资源覆盖 ${rate(activeResources, resourceTotal)}%`,
      detail: `${numberText(activeResources)} 个资源当前可合作，可结合热门排行优先推进邀约。`
    }
  ];
});

const rankedResources = computed(() => {
  const rows = [...((data.value.topResources || []) as RankingItem[])];
  return rows
    .sort(
      (a, b) =>
        Number(b[rankingMode.value] || 0) - Number(a[rankingMode.value] || 0)
    )
    .slice(0, 6);
});

const countryOptions = computed(
  () => (data.value.byCountry || []) as DistributionItem[]
);
const platformOptions = computed(
  () => (data.value.byPlatform || []) as DistributionItem[]
);

function defaultDateRange(): [string, string] {
  const end = new Date();
  const start = new Date();
  start.setDate(end.getDate() - 29);
  return [dateText(start), dateText(end)];
}

function dateText(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function numberText(value: unknown) {
  return Number(value || 0).toLocaleString("zh-CN", {
    maximumFractionDigits: 0
  });
}

function compactNumber(value: unknown) {
  return new Intl.NumberFormat("zh-CN", {
    notation: "compact",
    maximumFractionDigits: 1
  }).format(Number(value || 0));
}

function percentText(value: unknown) {
  return `${(Number(value || 0) * 100).toFixed(1)}%`;
}

function rate(value: unknown, total: unknown) {
  const denominator = Number(total || 0);
  return denominator ? Math.round((Number(value || 0) / denominator) * 100) : 0;
}

function resetFilters() {
  dateRange.value = defaultDateRange();
  filters.value = { country: "", resourceType: "", platform: "" };
  loadData();
}

async function loadData() {
  loading.value = true;
  try {
    const res = await getBusinessDashboard({
      startDate: dateRange.value?.[0],
      endDate: dateRange.value?.[1],
      ...filters.value
    });
    if (res.code === 0) {
      data.value = res.data;
      updatedAt.value = new Date();
      await nextTick();
      renderTrendChart();
    }
  } finally {
    loading.value = false;
  }
}

function renderTrendChart() {
  if (!trendChartRef.value) return;
  trendChart ||= echarts.init(trendChartRef.value, undefined, {
    renderer: "svg"
  });
  const rows = (data.value.trend || []) as TrendItem[];
  trendChart.setOption(
    {
      animationDuration: 500,
      color: ["#22c55e", "#3b82f6", "#8b5cf6"],
      tooltip: {
        trigger: "axis",
        valueFormatter: (value: number) => compactNumber(value)
      },
      legend: {
        top: 0,
        left: 0,
        itemWidth: 12,
        itemHeight: 7,
        textStyle: { color: "#475569" }
      },
      grid: { top: 48, right: 18, bottom: 24, left: 18, containLabel: true },
      xAxis: {
        type: "category",
        data: rows.map(item => item.date.slice(5)),
        axisTick: { show: false },
        axisLine: { lineStyle: { color: "#e2e8f0" } },
        axisLabel: { color: "#94a3b8", hideOverlap: true }
      },
      yAxis: [
        {
          type: "value",
          axisLabel: { color: "#94a3b8", formatter: compactNumber },
          splitLine: { lineStyle: { color: "#eef2f7" } }
        },
        {
          type: "value",
          axisLabel: { show: false },
          splitLine: { show: false }
        }
      ],
      series: [
        {
          name: "发布数",
          type: "bar",
          yAxisIndex: 1,
          data: rows.map(item => item.postCount),
          barMaxWidth: 18,
          itemStyle: { borderRadius: [4, 4, 0, 0], opacity: 0.62 }
        },
        {
          name: "曝光",
          type: "line",
          smooth: true,
          symbol: "none",
          lineStyle: { width: 3 },
          data: rows.map(item => item.exposure)
        },
        {
          name: "互动",
          type: "line",
          smooth: true,
          symbol: "none",
          lineStyle: { width: 3 },
          data: rows.map(item => item.interactions)
        }
      ]
    },
    true
  );
}

function handleResize() {
  trendChart?.resize();
}

onMounted(() => {
  window.addEventListener("resize", handleResize);
  loadData();
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", handleResize);
  trendChart?.dispose();
});
</script>

<template>
  <div v-loading="loading" class="business-dashboard">
    <section class="dashboard-heading">
      <div>
        <h1>CreatorLoop</h1>
        <p>创作者资源运营与合作效果洞察工作台</p>
      </div>
      <div class="heading-actions">
        <span>
          数据更新时间：
          {{ updatedAt ? updatedAt.toLocaleString("zh-CN") : "-" }}
        </span>
        <el-button circle aria-label="刷新看板" @click="loadData">
          <IconifyIconOnline icon="ri:refresh-line" />
        </el-button>
        <el-button type="success" @click="router.push('/business/resources')">
          <IconifyIconOnline icon="ri:add-line" class="mr-1" />
          新增资源
        </el-button>
      </div>
    </section>

    <section class="filter-panel">
      <div class="filter-row">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          range-separator="-"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          :clearable="false"
        />
        <el-select v-model="filters.country" clearable placeholder="全部市场">
          <el-option
            v-for="item in countryOptions"
            :key="item.name"
            :label="item.name || '未填写'"
            :value="item.name"
          />
        </el-select>
        <el-select
          v-model="filters.resourceType"
          clearable
          placeholder="全部资源类型"
        >
          <el-option
            v-for="item in ['KOL', '媒体', 'IP', '其他']"
            :key="item"
            :label="item"
            :value="item"
          />
        </el-select>
        <el-select v-model="filters.platform" clearable placeholder="全部平台">
          <el-option
            v-for="item in platformOptions"
            :key="item.name"
            :label="item.name || '未填写'"
            :value="item.name"
          />
        </el-select>
        <el-button type="success" @click="loadData">搜索</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </div>
      <button
        type="button"
        class="advanced-trigger"
        @click="advancedVisible = !advancedVisible"
      >
        <IconifyIconOnline
          :icon="advancedVisible ? 'ri:arrow-down-s-fill' : 'ri:arrow-right-s-fill'"
        />
        高级筛选
      </button>
      <div v-if="advancedVisible" class="advanced-content">
        当前支持按市场、资源类型和平台联动筛选；更多维度可进入资源库继续筛选。
        <el-button link type="success" @click="router.push('/business/resources')">
          前往全球资源库
        </el-button>
      </div>
    </section>

    <section class="metric-grid">
      <article
        v-for="item in metricCards"
        :key="item.key"
        class="metric-card"
        :style="{ '--tone': item.color }"
      >
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
        <small>{{ item.hint }}</small>
        <svg viewBox="0 0 104 40" aria-hidden="true">
          <path :d="item.path" />
        </svg>
      </article>
    </section>

    <section class="insight-panel">
      <div class="section-heading">
        <div>
          <h2>AI 复盘摘要</h2>
          <p>基于当前筛选数据自动生成，不是固定模板</p>
        </div>
        <span class="ai-status">
          <i />
          数据驱动洞察
        </span>
      </div>
      <div class="insight-grid">
        <article v-for="item in insightCards" :key="item.title">
          <span>数据驱动洞察</span>
          <h3>{{ item.title }}</h3>
          <p>{{ item.detail }}</p>
        </article>
      </div>
    </section>

    <section class="analysis-grid">
      <article class="analysis-card trend-card">
        <div class="section-heading">
          <div>
            <h2>发布与互动趋势</h2>
            <p>按天查看发布数、曝光和互动变化</p>
          </div>
          <el-tag effect="plain">按日</el-tag>
        </div>
        <el-empty
          v-if="!data.trend?.length"
          description="当前筛选范围暂无帖子数据"
        />
        <div v-else ref="trendChartRef" class="trend-chart" />
      </article>

      <article class="analysis-card ranking-card">
        <div class="section-heading ranking-heading">
          <div>
            <h2>热门资源排行</h2>
            <p>按当前筛选内容表现排序</p>
          </div>
          <el-button-group>
            <el-button
              :type="rankingMode === 'exposure' ? 'success' : 'default'"
              @click="rankingMode = 'exposure'"
            >
              按曝光
            </el-button>
            <el-button
              :type="rankingMode === 'interactions' ? 'success' : 'default'"
              @click="rankingMode = 'interactions'"
            >
              按互动
            </el-button>
            <el-button
              :type="rankingMode === 'engagementRate' ? 'success' : 'default'"
              @click="rankingMode = 'engagementRate'"
            >
              按互动率
            </el-button>
          </el-button-group>
        </div>
        <el-empty
          v-if="!rankedResources.length"
          description="当前筛选范围暂无排行数据"
          :image-size="72"
        />
        <div v-else class="ranking-table">
          <div class="ranking-row ranking-row--head">
            <span>资源</span>
            <span>曝光</span>
            <span>互动率</span>
          </div>
          <div
            v-for="(item, index) in rankedResources"
            :key="item.id"
            class="ranking-row"
          >
            <div>
              <b>{{ index + 1 }}</b>
              <span>
                <strong>{{ item.name || "未命名资源" }}</strong>
                <small>{{ item.platform || "未填写平台" }}</small>
              </span>
            </div>
            <strong>{{ compactNumber(item.exposure) }}</strong>
            <strong>{{ percentText(item.engagementRate) }}</strong>
          </div>
        </div>
      </article>
    </section>
  </div>
</template>

<style scoped>
.business-dashboard {
  min-height: 100%;
  padding: 18px;
  color: #0f172a;
  background: #f5f7fb;
}

.dashboard-heading,
.heading-actions,
.filter-row,
.section-heading,
.ranking-row,
.ranking-row > div {
  display: flex;
  align-items: center;
}

.dashboard-heading,
.section-heading,
.ranking-row {
  justify-content: space-between;
}

.dashboard-heading {
  gap: 20px;
  margin-bottom: 14px;
}

.dashboard-heading h1,
.section-heading h2,
.insight-grid h3 {
  margin: 0;
}

.dashboard-heading h1 {
  font-size: 30px;
  line-height: 1;
}

.dashboard-heading p,
.section-heading p,
.insight-grid p {
  margin: 6px 0 0;
  color: #64748b;
}

.dashboard-heading p {
  font-size: 13px;
}

.heading-actions {
  flex-wrap: wrap;
  gap: 10px;
}

.heading-actions > span {
  font-size: 12px;
  color: #64748b;
}

.filter-panel,
.metric-card,
.insight-panel,
.analysis-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
}

.filter-panel {
  padding: 13px;
  margin-bottom: 14px;
}

.filter-row {
  gap: 10px;
}

.filter-row :deep(.el-date-editor) {
  width: 285px;
}

.filter-row :deep(.el-select) {
  min-width: 170px;
  flex: 1;
}

.advanced-trigger {
  display: flex;
  gap: 2px;
  align-items: center;
  padding: 9px 0 0;
  font-size: 13px;
  font-weight: 700;
  color: #15803d;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.advanced-content {
  padding: 10px 12px;
  margin-top: 10px;
  font-size: 12px;
  color: #64748b;
  background: #f8fafc;
  border-radius: 8px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}

.metric-card {
  position: relative;
  min-height: 142px;
  padding: 16px;
  overflow: hidden;
}

.metric-card > span {
  font-size: 13px;
  font-weight: 700;
  color: #475569;
}

.metric-card > strong {
  display: block;
  margin-top: 10px;
  font-size: 30px;
  line-height: 1;
}

.metric-card small {
  display: block;
  max-width: calc(100% - 72px);
  margin-top: 9px;
  color: #64748b;
}

.metric-card svg {
  position: absolute;
  right: 13px;
  bottom: 13px;
  width: 86px;
  height: 35px;
}

.metric-card path {
  fill: none;
  stroke: var(--tone);
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 2.5;
}

.insight-panel {
  padding: 16px;
  margin-top: 14px;
}

.section-heading {
  gap: 12px;
}

.section-heading h2 {
  font-size: 18px;
}

.section-heading p,
.insight-grid p {
  font-size: 12px;
}

.ai-status {
  display: flex;
  gap: 7px;
  align-items: center;
  font-size: 12px;
  font-weight: 700;
  color: #15803d;
}

.ai-status i {
  width: 7px;
  height: 7px;
  background: #22c55e;
  border-radius: 50%;
  box-shadow: 0 0 0 4px #dcfce7;
}

.insight-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 14px;
}

.insight-grid article {
  min-height: 136px;
  padding: 15px;
  text-align: center;
  background: linear-gradient(180deg, #fbfefc, #fff);
  border: 1px solid #dce9df;
  border-radius: 10px;
}

.insight-grid article > span {
  font-size: 12px;
  font-weight: 700;
  color: #15803d;
}

.insight-grid h3 {
  margin-top: 15px;
  font-size: 15px;
}

.insight-grid p {
  line-height: 1.7;
}

.analysis-grid {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(360px, 0.9fr);
  gap: 14px;
  margin-top: 14px;
}

.analysis-card {
  min-height: 405px;
  padding: 16px;
}

.trend-chart {
  width: 100%;
  height: 330px;
  margin-top: 10px;
}

.ranking-heading {
  align-items: flex-start;
}

.ranking-heading :deep(.el-button) {
  padding: 7px 9px;
  font-size: 12px;
}

.ranking-table {
  margin-top: 18px;
}

.ranking-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 82px 66px;
  gap: 8px;
  min-height: 50px;
  border-bottom: 1px solid #eef2f7;
}

.ranking-row > div {
  min-width: 0;
  gap: 10px;
}

.ranking-row > div > b {
  width: 22px;
  color: #94a3b8;
  text-align: center;
}

.ranking-row:nth-child(2) > div > b,
.ranking-row:nth-child(3) > div > b,
.ranking-row:nth-child(4) > div > b {
  color: #15803d;
}

.ranking-row span {
  min-width: 0;
}

.ranking-row span strong,
.ranking-row span small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ranking-row span small {
  margin-top: 3px;
  color: #94a3b8;
}

.ranking-row--head {
  min-height: 38px;
  color: #64748b;
  background: #f8fafc;
  border: 0;
  border-radius: 7px;
}

.ranking-row--head span:first-child {
  padding-left: 12px;
}

.mr-1 {
  margin-right: 4px;
}

@media (width <= 1100px) {
  .metric-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .insight-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (width <= 1080px) {
  .filter-row {
    flex-wrap: wrap;
  }

  .analysis-grid {
    grid-template-columns: 1fr;
  }
}

@media (width <= 760px) {
  .business-dashboard {
    padding: 12px;
  }

  .dashboard-heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .metric-grid,
  .insight-grid {
    grid-template-columns: 1fr;
  }

  .filter-row > *,
  .filter-row :deep(.el-date-editor),
  .filter-row :deep(.el-select) {
    width: 100%;
  }
}
</style>
