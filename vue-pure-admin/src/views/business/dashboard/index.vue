<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import echarts from "@/plugins/echarts";
import { getBusinessDashboard } from "@/api/business";
import { fieldLabel } from "@/utils/fieldI18n";

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
const { locale } = useI18n();
const isEnglish = computed(() => locale.value === "en");
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
    key: "totalPostViews",
    label: "总触达与曝光",
    value: compactNumber(data.value.totalPostViews),
    hint: "当前筛选内容累计播放",
    icon: "ri:line-chart-line",
    accent: "lime"
  },
  {
    key: "activeResourceTotal",
    label: "活跃合作资源",
    value: numberText(data.value.activeResourceTotal),
    hint: `${rate(data.value.activeResourceTotal, data.value.resourceTotal)}% 可合作覆盖`,
    icon: "ri:team-line",
    accent: "cyan"
  },
  {
    key: "totalPostInteractions",
    label: "总互动",
    value: compactNumber(data.value.totalPostInteractions),
    hint: "点赞、评论与分享汇总",
    icon: "ri:pulse-line",
    accent: "blue"
  },
  {
    key: "totalPostCount",
    label: "内容资产",
    value: numberText(data.value.totalPostCount),
    hint: `${numberText(data.value.hotPostCount)} 条百万级爆款`,
    icon: "ri:video-line",
    accent: "orange"
  }
]);

const regionalCards = computed(() => {
  const rows = [...countryOptions.value]
    .sort((a, b) => Number(b.value || 0) - Number(a.value || 0))
    .slice(0, 4);
  const max = Math.max(...rows.map(item => Number(item.value || 0)), 1);
  return rows.map((item, index) => ({
    ...item,
    rank: String(index + 1).padStart(2, "0"),
    percentage: Math.max(8, Math.round((Number(item.value || 0) / max) * 100))
  }));
});

const insightCards = computed(() => {
  const top = rankedResources.value[0];
  const postCount = Number(data.value.totalPostCount || 0);
  const hotCount = Number(data.value.hotPostCount || 0);
  const engagementRate = Number(data.value.postEngagementRate || 0);
  const activeResources = Number(data.value.activeResourceTotal || 0);
  const resourceTotal = Number(data.value.resourceTotal || 0);
  if (isEnglish.value) {
    return [
      {
        title: top
          ? `${top.name} generated the highest exposure`
          : "No content exposure yet",
        detail: top
          ? `${top.platform || "Unknown platform"} accumulated ${compactNumber(top.exposure)} impressions. Consider reusing its content direction.`
          : "Adjust the filters or sync platform content to generate top-resource insights."
      },
      {
        title: `Million-view viral content share: ${rate(hotCount, postCount)}%`,
        detail: hotCount
          ? `${numberText(hotCount)} items reached one million impressions and can be captured in the case library.`
          : "No content has reached one million impressions under the current filters. Review topics and publishing times."
      },
      {
        title: `Overall engagement rate: ${percentText(engagementRate)}`,
        detail:
          engagementRate >= 0.05
            ? "Engagement efficiency is strong. Prioritize similar resources and content themes."
            : "Exposure is converting poorly into engagement. Improve the content hook and CTA."
      },
      {
        title: `Available resource coverage: ${rate(activeResources, resourceTotal)}%`,
        detail: `${numberText(activeResources)} resources are currently available. Prioritize outreach using the popularity ranking.`
      }
    ];
  }
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
  return Number(value || 0).toLocaleString(
    isEnglish.value ? "en-US" : "zh-CN",
    {
      maximumFractionDigits: 0
    }
  );
}

function compactNumber(value: unknown) {
  return new Intl.NumberFormat(isEnglish.value ? "en-US" : "zh-CN", {
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
      color: ["#ccff00", "#111116", "#0099ff"],
      tooltip: {
        trigger: "axis",
        valueFormatter: (value: number) => compactNumber(value)
      },
      legend: {
        top: 0,
        left: 0,
        itemWidth: 12,
        itemHeight: 7,
        textStyle: { color: "#6a6963", fontFamily: "JetBrains Mono" }
      },
      grid: { top: 48, right: 18, bottom: 24, left: 18, containLabel: true },
      xAxis: {
        type: "category",
        data: rows.map(item => item.date.slice(5)),
        axisTick: { show: false },
        axisLine: { lineStyle: { color: "#d9d6cc" } },
        axisLabel: {
          color: "#757470",
          hideOverlap: true,
          fontFamily: "JetBrains Mono"
        }
      },
      yAxis: [
        {
          type: "value",
          axisLabel: {
            color: "#757470",
            formatter: compactNumber,
            fontFamily: "JetBrains Mono"
          },
          splitLine: { lineStyle: { color: "#e8e5dc" } }
        },
        {
          type: "value",
          axisLabel: { show: false },
          splitLine: { show: false }
        }
      ],
      series: [
        {
          name: fieldLabel("发布数"),
          type: "bar",
          yAxisIndex: 1,
          data: rows.map(item => item.postCount),
          barMaxWidth: 18,
          itemStyle: { borderRadius: [3, 3, 0, 0], opacity: 0.9 }
        },
        {
          name: fieldLabel("曝光"),
          type: "line",
          smooth: true,
          symbol: "none",
          lineStyle: { width: 3 },
          data: rows.map(item => item.exposure)
        },
        {
          name: fieldLabel("互动"),
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
    <section class="cockpit-heading">
      <div class="cockpit-title">
        <div class="brand-line">
          <span>TRANSSION</span>
          <strong>PULSE</strong>
          <i>V3.4 ENTERPRISE</i>
        </div>
        <div class="title-line">
          <h1>{{ fieldLabel("全球 KOL 营销驾驶舱") }}</h1>
          <span class="live-pill"><i /> LIVE PIPELINE SYNC</span>
        </div>
        <p>
          {{ fieldLabel("覆盖全球重点市场的创作者资源、内容表现与合作效率") }}
        </p>
      </div>
      <div class="heading-actions">
        <div class="updated-at">
          <IconifyIconOnline icon="ri:database-2-line" />
          <span>
            {{ fieldLabel("数据更新时间") }}
            {{
              updatedAt
                ? updatedAt.toLocaleString(isEnglish ? "en-US" : "zh-CN")
                : "-"
            }}
          </span>
        </div>
        <el-button class="icon-button" aria-label="刷新看板" @click="loadData">
          <IconifyIconOnline icon="ri:refresh-line" />
        </el-button>
        <el-button
          class="primary-action"
          @click="router.push('/business/resources')"
        >
          <IconifyIconOnline icon="ri:add-circle-line" />
          {{ fieldLabel("新增资源") }}
        </el-button>
      </div>
    </section>

    <section class="action-strip">
      <div class="action-copy">
        <span class="action-icon">
          <IconifyIconOnline icon="ri:alarm-warning-line" />
        </span>
        <div>
          <div class="action-labels">
            <strong>ACTION REQUIRED</strong>
            <span>PRIORITY HIGH</span>
          </div>
          <p>
            <b>{{ numberText(data.hotPostCount) }}</b>
            {{ fieldLabel("条百万级内容待沉淀为案例") }} ·
            <b>{{ numberText(data.activeResourceTotal) }}</b>
            {{ fieldLabel("个活跃资源可进入下一轮合作") }}
          </p>
        </div>
      </div>
      <button type="button" @click="router.push('/business/projects')">
        {{ fieldLabel("查看营销项目") }}
        <IconifyIconOnline icon="ri:arrow-right-line" />
      </button>
    </section>

    <section class="filter-panel">
      <div class="filter-row">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          range-separator="-"
          :start-placeholder="fieldLabel('开始日期')"
          :end-placeholder="fieldLabel('结束日期')"
          :clearable="false"
        />
        <el-select
          v-model="filters.country"
          clearable
          :placeholder="fieldLabel('全部市场')"
        >
          <el-option
            v-for="item in countryOptions"
            :key="item.name"
            :label="item.name || fieldLabel('未填写')"
            :value="item.name"
          />
        </el-select>
        <el-select
          v-model="filters.resourceType"
          clearable
          :placeholder="fieldLabel('全部资源类型')"
        >
          <el-option
            v-for="item in ['KOL', '媒体', 'IP', '其他']"
            :key="item"
            :label="fieldLabel(item)"
            :value="item"
          />
        </el-select>
        <el-select
          v-model="filters.platform"
          clearable
          :placeholder="fieldLabel('全部平台')"
        >
          <el-option
            v-for="item in platformOptions"
            :key="item.name"
            :label="item.name || fieldLabel('未填写')"
            :value="item.name"
          />
        </el-select>
        <el-button type="success" @click="loadData">{{
          fieldLabel("搜索")
        }}</el-button>
        <el-button @click="resetFilters">{{ fieldLabel("重置") }}</el-button>
        <button
          type="button"
          class="advanced-trigger"
          @click="advancedVisible = !advancedVisible"
        >
          <IconifyIconOnline icon="ri:equalizer-2-line" />
          {{ fieldLabel("高级筛选") }}
          <IconifyIconOnline
            :icon="
              advancedVisible ? 'ri:arrow-up-s-line' : 'ri:arrow-down-s-line'
            "
          />
        </button>
      </div>
      <div v-if="advancedVisible" class="advanced-content">
        {{
          fieldLabel(
            "当前支持按市场、资源类型和平台联动筛选；更多维度可进入资源库继续筛选。"
          )
        }}
        <el-button
          link
          type="success"
          @click="router.push('/business/resources')"
        >
          {{ fieldLabel("前往全球资源库") }}
        </el-button>
      </div>
    </section>

    <section class="metric-grid">
      <article
        v-for="item in metricCards"
        :key="item.key"
        class="metric-card"
        :class="`metric-card--${item.accent}`"
      >
        <div class="metric-topline">
          <span class="metric-icon">
            <IconifyIconOnline :icon="item.icon" />
          </span>
          <span class="metric-status">LIVE</span>
        </div>
        <span class="metric-label">{{ fieldLabel(item.label) }}</span>
        <strong>{{ item.value }}</strong>
        <small>{{ fieldLabel(item.hint) }}</small>
      </article>
    </section>

    <section class="regional-panel">
      <div class="section-heading section-heading--bordered">
        <div>
          <span class="section-kicker">GLOBAL MARKET MAP</span>
          <h2>{{ fieldLabel("重点市场资源分布") }}</h2>
          <p>{{ fieldLabel("按当前筛选条件展示资源规模最高的市场") }}</p>
        </div>
        <span class="engagement-pill">
          {{ fieldLabel("平均互动率") }}
          <b>{{ percentText(data.postEngagementRate) }}</b>
        </span>
      </div>
      <div v-if="regionalCards.length" class="region-grid">
        <article v-for="item in regionalCards" :key="item.name">
          <div class="region-card-head">
            <span>{{ item.rank }}</span>
            <IconifyIconOnline icon="ri:map-pin-2-line" />
          </div>
          <strong>{{ item.name || fieldLabel("未填写市场") }}</strong>
          <p>{{ numberText(item.value) }} {{ fieldLabel("个资源") }}</p>
          <div class="region-progress">
            <i :style="{ width: `${item.percentage}%` }" />
          </div>
        </article>
      </div>
      <el-empty
        v-else
        :description="fieldLabel('当前筛选范围暂无市场数据')"
        :image-size="64"
      />
    </section>

    <section class="analysis-grid">
      <article class="analysis-card trend-card">
        <div class="section-heading section-heading--bordered">
          <div>
            <span class="section-kicker">CAMPAIGN VELOCITY</span>
            <h2>{{ fieldLabel("内容发布与转化节奏") }}</h2>
            <p>{{ fieldLabel("按天追踪发布数、曝光和互动变化") }}</p>
          </div>
          <el-tag effect="dark" color="#111116">{{
            fieldLabel("实时")
          }}</el-tag>
        </div>
        <el-empty
          v-if="!data.trend?.length"
          :description="fieldLabel('当前筛选范围暂无帖子数据')"
        />
        <div v-else ref="trendChartRef" class="trend-chart" />
      </article>

      <article class="insight-panel">
        <div class="section-heading section-heading--bordered">
          <div>
            <span class="section-kicker">AI EXECUTIVE BRIEF</span>
            <h2>{{ fieldLabel("智能复盘摘要") }}</h2>
            <p>{{ fieldLabel("基于实时业务数据动态生成") }}</p>
          </div>
          <span class="ai-status"><i /> SYNCED</span>
        </div>
        <div class="insight-list">
          <article v-for="(item, index) in insightCards" :key="item.title">
            <b>{{ String(index + 1).padStart(2, "0") }}</b>
            <div>
              <h3>{{ item.title }}</h3>
              <p>{{ item.detail }}</p>
            </div>
          </article>
        </div>
      </article>
    </section>

    <section class="ranking-card">
      <div class="section-heading section-heading--bordered ranking-heading">
        <div>
          <span class="section-kicker">TOP PERFORMERS</span>
          <h2>{{ fieldLabel("创作者表现排行榜") }}</h2>
          <p>{{ fieldLabel("按曝光、互动和内容效率识别优先合作资源") }}</p>
        </div>
        <el-button-group>
          <el-button
            :type="rankingMode === 'exposure' ? 'success' : 'default'"
            @click="rankingMode = 'exposure'"
          >
            {{ fieldLabel("按曝光") }}
          </el-button>
          <el-button
            :type="rankingMode === 'interactions' ? 'success' : 'default'"
            @click="rankingMode = 'interactions'"
          >
            {{ fieldLabel("按互动") }}
          </el-button>
          <el-button
            :type="rankingMode === 'engagementRate' ? 'success' : 'default'"
            @click="rankingMode = 'engagementRate'"
          >
            {{ fieldLabel("按互动率") }}
          </el-button>
        </el-button-group>
      </div>
      <el-empty
        v-if="!rankedResources.length"
        :description="fieldLabel('当前筛选范围暂无排行数据')"
        :image-size="72"
      />
      <div v-else class="ranking-table">
        <div class="ranking-row ranking-row--head">
          <span>{{ fieldLabel("创作者与平台") }}</span>
          <span>{{ fieldLabel("内容") }}</span>
          <span>{{ fieldLabel("曝光") }}</span>
          <span>{{ fieldLabel("互动率") }}</span>
          <span>{{ fieldLabel("状态") }}</span>
        </div>
        <div
          v-for="(item, index) in rankedResources"
          :key="item.id"
          class="ranking-row"
        >
          <div>
            <b>{{ index + 1 }}</b>
            <span>
              <strong>{{ item.name || fieldLabel("未命名资源") }}</strong>
              <small>{{ item.platform || fieldLabel("未填写平台") }}</small>
            </span>
          </div>
          <strong>{{ numberText(item.postCount) }}</strong>
          <strong>{{ compactNumber(item.exposure) }}</strong>
          <strong>{{ percentText(item.engagementRate) }}</strong>
          <span class="status-pill">ACTIVE</span>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.business-dashboard {
  min-height: 100%;
  padding: 24px;
  font-family:
    "Plus Jakarta Sans", "PingFang SC", "Microsoft YaHei", sans-serif;
  color: #16161a;
  background: #f6f5f1;
}

.cockpit-heading,
.heading-actions,
.brand-line,
.title-line,
.action-strip,
.action-copy,
.action-labels,
.filter-row,
.section-heading,
.metric-topline,
.region-card-head,
.ranking-row,
.ranking-row > div {
  display: flex;
  align-items: center;
}

.cockpit-heading,
.action-strip,
.section-heading,
.metric-topline,
.region-card-head,
.ranking-row {
  justify-content: space-between;
}

.cockpit-heading {
  gap: 24px;
  padding: 12px 4px 18px;
  color: #111116;
  background: transparent;
}

.brand-line {
  gap: 8px;
  margin-bottom: 10px;
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  letter-spacing: 0.12em;
}

.brand-line span {
  font-weight: 800;
}

.brand-line strong {
  color: #cf0;
}

.brand-line i {
  padding: 3px 7px;
  font-style: normal;
  color: #cf0;
  background: #111116;
  border: 1px solid #536500;
  border-radius: 4px;
}

.title-line {
  flex-wrap: wrap;
  gap: 12px;
}

.cockpit-heading h1,
.section-heading h2,
.insight-list h3 {
  margin: 0;
}

.cockpit-heading h1 {
  font-family: "Space Grotesk", "PingFang SC", sans-serif;
  font-size: clamp(24px, 2.25vw, 34px);
  font-weight: 800;
  line-height: 1.1;
  letter-spacing: -0.025em;
}

.live-pill {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  padding: 6px 10px;
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  font-weight: 800;
  color: #cf0;
  background: #111116;
  border: 1px solid #3c480d;
  border-radius: 999px;
}

.live-pill i,
.ai-status i {
  width: 7px;
  height: 7px;
  background: #cf0;
  border-radius: 50%;
  box-shadow: 0 0 10px rgb(204 255 0 / 75%);
}

.cockpit-heading p,
.section-heading p,
.insight-list p {
  margin: 6px 0 0;
  color: #757470;
}

.cockpit-heading p {
  font-family: "JetBrains Mono", monospace;
  font-size: 11px;
  color: #6a6963;
}

.heading-actions {
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.updated-at {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 0 10px;
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  color: #6a6963;
}

.heading-actions :deep(.el-button) {
  margin-left: 0;
}

.heading-actions :deep(.icon-button),
.heading-actions :deep(.primary-action) {
  height: 36px;
  color: #fff;
  background: #111116;
  border-color: #111116;
}

.heading-actions :deep(.icon-button) {
  width: 36px;
  padding: 0;
}

.heading-actions :deep(.primary-action) {
  gap: 6px;
  font-weight: 800;
  color: #111116;
  background: #cf0;
  border-color: #cf0;
}

.action-strip {
  gap: 20px;
  padding: 11px 14px;
  margin-bottom: 14px;
  background: #fff;
  border: 1px solid #dedbd1;
  border-left: 3px solid #ff4500;
  border-radius: 10px;
}

.action-copy {
  gap: 12px;
  min-width: 0;
}

.action-icon {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  color: #111116;
  background: #cf0;
  border-radius: 7px;
}

.action-labels {
  gap: 8px;
  font-family: "JetBrains Mono", monospace;
  font-size: 9px;
}

.action-labels strong {
  letter-spacing: 0.1em;
}

.action-labels span {
  padding: 2px 5px;
  color: #fff;
  background: #ef4444;
  border-radius: 3px;
}

.action-copy p {
  margin: 4px 0 0;
  font-size: 12px;
  color: #6a6963;
}

.action-copy p b {
  color: #111116;
}

.action-strip > button,
.advanced-trigger {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.action-strip > button {
  flex: 0 0 auto;
  padding: 8px 12px;
  font-size: 11px;
  font-weight: 800;
  color: #cf0;
  background: #111116;
  border-radius: 7px;
}

.filter-panel,
.metric-card,
.regional-panel,
.insight-panel,
.analysis-card,
.ranking-card {
  background: #fff;
  border: 1px solid #dedbd1;
  border-radius: 10px;
  box-shadow: 0 2px 10px rgb(18 18 22 / 4%);
}

.filter-panel {
  padding: 12px;
  margin-bottom: 16px;
}

.filter-row {
  gap: 8px;
}

.filter-row :deep(.el-date-editor) {
  width: 285px;
}

.filter-row :deep(.el-select) {
  flex: 1;
  min-width: 170px;
}

.advanced-trigger {
  flex: 0 0 auto;
  padding: 8px 10px;
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  font-weight: 700;
  color: #4e4d49;
}

.advanced-content {
  padding: 10px 12px;
  margin-top: 10px;
  font-size: 12px;
  color: #6a6963;
  background: #f6f5f1;
  border: 1px solid #e7e4da;
  border-radius: 8px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.metric-card {
  position: relative;
  min-height: 154px;
  padding: 16px 17px;
  overflow: hidden;
  border-top: 3px solid #111116;
}

.metric-card--lime {
  border-top-color: #cf0;
}

.metric-card--cyan {
  border-top-color: #00dc82;
}

.metric-card--blue {
  border-top-color: #09f;
}

.metric-card--orange {
  border-top-color: #ff7a00;
}

.metric-topline {
  margin-bottom: 14px;
}

.metric-icon {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  font-size: 17px;
  color: #cf0;
  background: #111116;
  border-radius: 6px;
}

.metric-status {
  font-family: "JetBrains Mono", monospace;
  font-size: 9px;
  font-weight: 700;
  color: #87857f;
  letter-spacing: 0.12em;
}

.metric-label {
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  font-weight: 700;
  color: #5d5c57;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.metric-card > strong {
  display: block;
  margin-top: 7px;
  font-family: "Space Grotesk", sans-serif;
  font-size: 34px;
  font-weight: 800;
  line-height: 1;
  letter-spacing: -0.04em;
}

.metric-card small {
  display: block;
  margin-top: 9px;
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  color: #757470;
}

.regional-panel,
.analysis-card,
.insight-panel,
.ranking-card {
  padding: 17px;
  margin-top: 16px;
}

.section-heading {
  gap: 12px;
}

.section-heading--bordered {
  padding-bottom: 13px;
  border-bottom: 1px solid #e8e5dc;
}

.section-kicker {
  display: block;
  margin-bottom: 4px;
  font-family: "JetBrains Mono", monospace;
  font-size: 9px;
  font-weight: 700;
  color: #7b7a74;
  letter-spacing: 0.13em;
}

.section-heading h2 {
  font-family: "Space Grotesk", "PingFang SC", sans-serif;
  font-size: 17px;
  font-weight: 800;
}

.section-heading p,
.insight-list p {
  font-size: 11px;
}

.engagement-pill {
  padding: 7px 10px;
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  color: #6a6963;
  background: #f6f5f1;
  border: 1px solid #dedbd1;
  border-radius: 6px;
}

.engagement-pill b {
  margin-left: 6px;
  font-size: 13px;
  color: #111116;
}

.region-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-top: 14px;
}

.region-grid article {
  padding: 14px;
  background: #faf9f6;
  border: 1px solid #e5e3db;
  border-radius: 8px;
}

.region-card-head {
  margin-bottom: 16px;
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  color: #87857f;
}

.region-card-head svg {
  font-size: 17px;
  color: #111116;
}

.region-grid article > strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  font-family: "Space Grotesk", sans-serif;
  font-size: 16px;
  white-space: nowrap;
}

.region-grid article > p {
  margin: 5px 0 12px;
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  color: #757470;
}

.region-progress {
  height: 4px;
  overflow: hidden;
  background: #dedbd1;
  border-radius: 4px;
}

.region-progress i {
  display: block;
  height: 100%;
  background: #cf0;
  border-right: 2px solid #111116;
}

.ai-status {
  display: flex;
  gap: 7px;
  align-items: center;
  padding: 6px 8px;
  font-family: "JetBrains Mono", monospace;
  font-size: 9px;
  font-weight: 800;
  color: #111116;
  background: #cf0;
  border-radius: 5px;
}

.ai-status i {
  background: #111116;
  box-shadow: none;
}

.insight-list {
  margin-top: 14px;
}

.insight-list article {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 10px;
  padding: 11px 0;
  border-bottom: 1px solid #ece9e1;
}

.insight-list article:last-child {
  border-bottom: 0;
}

.insight-list article > b {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  font-family: "JetBrains Mono", monospace;
  font-size: 9px;
  color: #111116;
  background: #cf0;
  border-radius: 5px;
}

.insight-list h3 {
  font-size: 12px;
}

.insight-list p {
  display: -webkit-box;
  overflow: hidden;
  text-overflow: ellipsis;
  -webkit-line-clamp: 2;
  line-height: 1.55;
  -webkit-box-orient: vertical;
}

.analysis-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.65fr) minmax(320px, 0.75fr);
  gap: 16px;
}

.analysis-card {
  min-height: 388px;
}

.trend-chart {
  width: 100%;
  height: 310px;
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
  margin-top: 12px;
}

.ranking-row {
  display: grid;
  grid-template-columns: minmax(220px, 1.5fr) 80px 110px 90px 88px;
  gap: 12px;
  min-height: 58px;
  padding: 0 10px;
  border-bottom: 1px solid #ece9e1;
}

.ranking-row > div {
  gap: 10px;
  min-width: 0;
}

.ranking-row > div > b {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  font-family: "JetBrains Mono", monospace;
  font-size: 10px;
  color: #757470;
  text-align: center;
  background: #f1efe8;
  border-radius: 5px;
}

.ranking-row:nth-child(2) > div > b,
.ranking-row:nth-child(3) > div > b,
.ranking-row:nth-child(4) > div > b {
  color: #111116;
  background: #cf0;
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
  font-family: "JetBrains Mono", monospace;
  font-size: 9px;
  color: #87857f;
}

.ranking-row--head {
  min-height: 36px;
  font-family: "JetBrains Mono", monospace;
  font-size: 9px;
  font-weight: 700;
  color: #6a6963;
  letter-spacing: 0.04em;
  background: #f6f5f1;
  border: 0;
  border-radius: 6px;
}

.status-pill {
  justify-self: start;
  padding: 4px 7px;
  font-family: "JetBrains Mono", monospace;
  font-size: 9px;
  font-weight: 800;
  color: #111116;
  background: #cf0;
  border-radius: 4px;
}

.mr-1 {
  margin-right: 4px;
}

@media (width <= 1100px) {
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .region-grid {
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

  .cockpit-heading,
  .action-strip {
    flex-direction: column;
    align-items: flex-start;
  }

  .metric-grid,
  .region-grid {
    grid-template-columns: 1fr;
  }

  .heading-actions {
    justify-content: flex-start;
  }

  .ranking-table {
    overflow-x: auto;
  }

  .ranking-row {
    min-width: 760px;
  }

  .filter-row > *,
  .filter-row :deep(.el-date-editor),
  .filter-row :deep(.el-select) {
    width: 100%;
  }
}
</style>
