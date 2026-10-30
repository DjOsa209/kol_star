<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { useMediaQuery } from "@vueuse/core";
import {
  getTrendDetail,
  getTrendList,
  getTrendOptions,
  type TrendMeta,
  type TrendRelatedVideo,
  type TrendOptions,
  type TrendQuery,
  type TrendTag,
  type TrendTagDetail,
  type TrendVideo
} from "@/api/trends";
import TrendImage from "./TrendImage.vue";
import MonthlyTrends from "./MonthlyTrends.vue";
import TrendCollector from "./TrendCollector.vue";
import TrendLocalCollector from "./TrendLocalCollector.vue";
import TrendChart from "./TrendChart.vue";
import TrendTagAnalysis from "./TrendTagAnalysis.vue";
import TrendVideoPlayer from "./TrendVideoPlayer.vue";
import RefreshIcon from "~icons/ri/refresh-line";
import ArchiveIcon from "~icons/ri/archive-line";
import GlobalIcon from "~icons/ri/global-line";
import PlayIcon from "~icons/ri/play-circle-line";
import { trendTikTokVideoURL } from "@/utils/trendLinks";

defineOptions({ name: "BusinessTrends" });
const route = useRoute();
const router = useRouter();
const { locale } = useI18n();
const activeTab = ref<"tag" | "video" | "monthly">(
  route.query.tab === "monthly"
    ? "monthly"
    : route.query.tab === "video"
      ? "video"
      : "tag"
);
const hasDetailSpace = useMediaQuery("(min-width: 1180px)");
const inlineTag = computed(
  () => activeTab.value === "tag" && hasDetailSpace.value
);
const text = (zh: string, en: string) => (locale.value === "en" ? en : zh);
const filters = reactive<TrendQuery>({
  tab: route.query.tab === "video" ? "video" : "tag",
  region: String(route.query.region || "US"),
  period: Number(route.query.period) || 30,
  orderByMetric: Number(route.query.orderByMetric) || 1,
  currentPage: 1,
  pageSize: 20,
  live: route.query.live === "1" ? 1 : 0
});
const options = ref<TrendOptions>({
  tagRegions: ["US"],
  videoRegions: ["US"],
  tagPeriods: [7, 30, 90],
  videoPeriods: [7, 30]
});
const regions = computed(() =>
  filters.tab === "tag" ? options.value.tagRegions : options.value.videoRegions
);
const periods = computed(() =>
  filters.tab === "tag" ? options.value.tagPeriods : options.value.videoPeriods
);
const tags = ref<TrendTag[]>([]);
const videos = ref<TrendVideo[]>([]);
const loading = ref(false);
const collectorVisible = ref(false);
const collectorActive = ref(false);
const collectorQuery = ref<TrendQuery>({ ...filters });
const error = ref("");
const total = ref(0);
const hasMore = ref(false);
const meta = ref<TrendMeta>();
const drawer = ref(false);
const selectedTag = ref<TrendTag>();
const relatedVisible = ref(false);
const relatedVideo = ref<TrendRelatedVideo>();
function previewRelated(video: TrendRelatedVideo) {
  relatedVideo.value = video;
  relatedVisible.value = true;
}
const detailLoading = ref(false);
const detailError = ref("");
const tagDetail = ref<TrendTagDetail>();
const videoDetail = ref<TrendVideo>();
let listRequest = 0;
let detailRequest = 0;
let ready = false;
const sorts = computed(() => [
  { value: 1, label: text("最高播放量", "Highest video views") },
  { value: 2, label: text("最高互动率", "Highest engagement") },
  { value: 3, label: text("最高 6 秒观看率", "Highest 6s views") }
]);
const currentCount = computed(() =>
  filters.tab === "tag" ? tags.value.length : videos.value.length
);
const isArchive = computed(() => meta.value?.source === "archive");
const regionNames = computed(
  () =>
    new Intl.DisplayNames([locale.value === "en" ? "en" : "zh-CN"], {
      type: "region"
    })
);
const regionName = (code: string) => regionNames.value.of(code) || code;
const selectedTitle = computed(() =>
  tagDetail.value
    ? `#${tagDetail.value.hashtagName}`
    : videoDetail.value?.author?.name || text("内容详情", "Content details")
);

function count(value: unknown) {
  if (value === null || value === undefined || value === "") return "—";
  const n = Number(value);
  if (!Number.isFinite(n)) return "—";
  return Intl.NumberFormat(locale.value === "en" ? "en" : "zh-CN", {
    notation: "compact",
    maximumFractionDigits: 1
  }).format(n);
}
function percentage(value: unknown) {
  if (value === null || value === undefined || value === "") return "—";
  const n = Number(value);
  return Number.isFinite(n) ? `${(n * 100).toFixed(2)}%` : "—";
}
function date(value?: string) {
  if (!value) return "—";
  return value.slice(0, 10);
}
function syncQuery() {
  router.replace({
    path: route.path,
    query: {
      tab: filters.tab,
      region: filters.region,
      period: String(filters.period),
      ...(filters.tab === "video"
        ? { orderByMetric: String(filters.orderByMetric) }
        : {}),
      ...(filters.live ? { live: "1" } : {})
    }
  });
}
async function load(force = false) {
  closeDetail();
  selectedTag.value = undefined;
  tagDetail.value = undefined;
  videoDetail.value = undefined;
  const generation = ++listRequest;
  const query = { ...filters, refresh: force ? 1 : undefined };
  loading.value = true;
  error.value = "";
  tags.value = [];
  videos.value = [];
  total.value = 0;
  hasMore.value = false;
  meta.value = undefined;
  syncQuery();
  try {
    const result = await getTrendList(query);
    if (generation !== listRequest) return;
    if (result.code !== 0) throw new Error(result.message);
    const d = result.data;
    meta.value = d.meta;
    total.value = d.total;
    if (d.meta.pageMismatch) {
      error.value = text(
        "未获取到这一页的数据，请返回上一页或稍后重试。",
        "The requested page is unavailable. Go back or try again later."
      );
      return;
    }
    if (query.tab === "tag") tags.value = d.list as TrendTag[];
    else videos.value = d.list as TrendVideo[];
    hasMore.value = d.hasMore;
    if (inlineTag.value && tags.value.length) openDetail(tags.value[0]);
  } catch {
    if (generation === listRequest)
      error.value = text(
        "暂时无法读取趋势数据，请稍后重试。",
        "Trend data is unavailable. Please try again later."
      );
  } finally {
    if (generation === listRequest) loading.value = false;
  }
}
function selectTab() {
  closeDetail();
  if (activeTab.value === "monthly") {
    listRequest++;
    loading.value = false;
    router.replace({ path: route.path, query: { tab: "monthly" } });
    return;
  }
  filters.tab = activeTab.value;
  changeFilters();
}
function changeFilters() {
  if (!ready) return;
  if (!regions.value.includes(filters.region)) filters.region = "US";
  if (!periods.value.includes(filters.period)) filters.period = 30;
  filters.currentPage = 1;
  filters.live = 0;
  closeDetail();
  load();
}
function changePage(page: number) {
  filters.currentPage = page;
  load();
}
function refresh(live = 1) {
  filters.live = live;
  filters.currentPage = 1;
  closeDetail();
  if (live) {
    collectorQuery.value = { ...filters };
    collectorVisible.value = true;
  } else load(true);
}
function collectionComplete(query: TrendQuery) {
  if (
    query.tab === filters.tab &&
    query.region === filters.region &&
    query.period === filters.period &&
    query.orderByMetric === filters.orderByMetric
  ) {
    filters.live = 1;
    filters.currentPage = 1;
    load();
  }
}
function closeDetail() {
  drawer.value = false;
  relatedVisible.value = false;
  relatedVideo.value = undefined;
  detailRequest++;
  detailLoading.value = false;
}
async function openDetail(row: TrendTag | TrendVideo) {
  const generation = ++detailRequest;
  const isTag = filters.tab === "tag";
  tagDetail.value = undefined;
  videoDetail.value = undefined;
  detailError.value = "";
  if (isTag) selectedTag.value = row as TrendTag;
  drawer.value = !isTag || !inlineTag.value;
  detailLoading.value = true;
  const id = isTag ? (row as TrendTag).hashtagID : (row as TrendVideo).itemID;
  try {
    const result = await getTrendDetail({ ...filters, id });
    if (generation !== detailRequest) return;
    if (result.code !== 0) throw new Error(result.message);
    if (isTag) tagDetail.value = result.data.detail as TrendTagDetail;
    else videoDetail.value = result.data.detail as TrendVideo;
  } catch {
    if (generation === detailRequest)
      detailError.value = text(
        "详情暂时无法读取，请重试。",
        "Details are unavailable. Please try again."
      );
  } finally {
    if (generation === detailRequest) detailLoading.value = false;
  }
}
watch(hasDetailSpace, () => {
  if (filters.tab !== "tag" || !tags.value.length) return;
  const tag = selectedTag.value || tags.value[0];
  drawer.value = !hasDetailSpace.value && !!selectedTag.value;
  if (hasDetailSpace.value && !tagDetail.value && !detailLoading.value)
    openDetail(tag);
});
function cancelDrawerRequest() {
  if (!inlineTag.value) {
    detailRequest++;
    detailLoading.value = false;
  }
}
watch(
  () => route.query,
  query => {
    if (!ready) return;
    if (query.tab === "monthly") {
      activeTab.value = "monthly";
      listRequest++;
      closeDetail();
      return;
    }
    const wasMonthly = activeTab.value === "monthly";
    activeTab.value = query.tab === "video" ? "video" : "tag";
    const tab = query.tab === "video" ? "video" : "tag";
    const region = String(query.region || "US");
    const period = Number(query.period) || 30;
    const order = Number(query.orderByMetric) || 1;
    const live = query.live === "1" ? 1 : 0;
    if (
      !wasMonthly &&
      tab === filters.tab &&
      region === filters.region &&
      period === filters.period &&
      order === filters.orderByMetric &&
      live === filters.live
    )
      return;
    Object.assign(filters, {
      tab,
      region,
      period,
      orderByMetric: order,
      live,
      currentPage: 1
    });
    if (!regions.value.includes(filters.region)) filters.region = "US";
    if (!periods.value.includes(filters.period)) filters.period = 30;
    closeDetail();
    load();
  }
);
onMounted(async () => {
  try {
    const result = await getTrendOptions();
    if (result.code !== 0) throw new Error(result.message);
    options.value = result.data;
    if (!regions.value.includes(filters.region)) filters.region = "US";
    if (!periods.value.includes(filters.period)) filters.period = 30;
    if (![1, 2, 3].includes(filters.orderByMetric)) filters.orderByMetric = 1;
    ready = true;
    if (activeTab.value !== "monthly") load();
  } catch {
    error.value = text(
      "筛选项加载失败，请刷新页面重试。",
      "Filters could not load. Reload the page."
    );
  }
});
</script>

<template>
  <div class="business-page trends-page">
    <component
      :is="
        options.collectionMode === 'local'
          ? TrendLocalCollector
          : TrendCollector
      "
      v-model="collectorVisible"
      :query="collectorQuery"
      :regions="
        collectorQuery.tab === 'tag' ? options.tagRegions : options.videoRegions
      "
      @complete="collectionComplete"
      @active="collectorActive = $event"
    />
    <header class="resource-toolbar">
      <h1>{{ text("趋势洞察", "Trend insights") }}</h1>
      <p>
        {{
          text(
            "发现热门内容、标签与每月创作灵感。",
            "Explore trending content, hashtags and monthly creative ideas."
          )
        }}
      </p>
    </header>
    <div class="trends-workspace" :class="{ 'is-tag-layout': inlineTag }">
      <section
        class="trend-browser"
        :aria-label="text('趋势榜单', 'Trend rankings')"
      >
        <el-tabs v-model="activeTab" @tab-change="selectTab">
          <el-tab-pane name="tag" label="Tag" /><el-tab-pane
            name="video"
            label="Video"
          /><el-tab-pane
            name="monthly"
            :label="text('每月趋势', 'Monthly trends')"
          />
        </el-tabs>
        <MonthlyTrends v-if="activeTab === 'monthly'" />
        <template v-else>
          <div class="filter-bar">
            <div class="filter-field">
              <label for="trend-region">{{
                text("国家 / 地区", "Country / region")
              }}</label
              ><el-select
                id="trend-region"
                v-model="filters.region"
                filterable
                @change="changeFilters"
                ><el-option
                  v-for="code in regions"
                  :key="code"
                  :value="code"
                  :label="regionName(code)"
              /></el-select>
            </div>
            <div class="filter-field">
              <label for="trend-period">{{
                text("时间范围", "Time range")
              }}</label
              ><el-select
                id="trend-period"
                v-model="filters.period"
                @change="changeFilters"
                ><el-option
                  v-for="period in periods"
                  :key="period"
                  :value="period"
                  :label="text(`近 ${period} 天`, `Last ${period} days`)"
              /></el-select>
            </div>
            <div class="filter-actions">
              <el-button
                type="primary"
                :loading="loading"
                @click="collectorActive ? (collectorVisible = true) : refresh()"
                ><RefreshIcon class="trend-ui-icon mr-6" />{{
                  collectorActive
                    ? text("查看采集进度", "Collection progress")
                    : text("读取最新", "Read latest")
                }}</el-button
              ><el-button
                v-if="
                  filters.live &&
                  filters.region === 'US' &&
                  filters.period === 30 &&
                  (filters.tab === 'tag' || filters.orderByMetric === 1)
                "
                text
                @click="refresh(0)"
                >{{ text("查看已采集数据", "View saved data") }}</el-button
              >
            </div>
          </div>

          <div v-if="meta" class="source-note" role="status">
            <component
              :is="isArchive ? ArchiveIcon : GlobalIcon"
              class="trend-ui-icon"
            />
            <span v-if="isArchive"
              >{{ text("已采集数据", "Saved collection") }} ·
              {{ date(meta.capturedAt) }}
              <template v-if="meta.curveEnd">
                · {{ text("数据截至", "Data through") }}
                {{ meta.curveEnd }}</template
              >
              · {{ total }} {{ text("条", "records") }}</span
            >
            <span v-else-if="meta.coverage === 'complete'"
              >{{
                text(
                  "完整榜单及详情已更新",
                  "Complete ranking and details updated"
                )
              }}
              · {{ total }} {{ text("条", "records") }}
              <template v-if="meta.curveEnd">
                · {{ text("数据截至", "Data through") }} {{ meta.curveEnd }}
              </template></span
            >
            <span v-else
              >{{
                text(
                  "原站当前可访问结果；可能受登录和访问范围限制，并非完整榜单。",
                  "Currently accessible TikTok results; sign-in and access limits may prevent a complete ranking."
                )
              }}<template v-if="meta.curveEnd">
                {{ text("数据截至", "Data through") }}
                {{ meta.curveEnd }}</template
              ></span
            >
          </div>

          <div v-if="filters.tab === 'video'" class="sort-bar">
            <el-radio-group
              v-model="filters.orderByMetric"
              class="desktop-sort"
              @change="changeFilters"
              ><el-radio-button
                v-for="sort in sorts"
                :key="sort.value"
                :value="sort.value"
                >{{ sort.label }}</el-radio-button
              ></el-radio-group
            >
            <div class="mobile-sort filter-field">
              <label for="trend-sort">{{ text("排序方式", "Sort by") }}</label
              ><el-select
                id="trend-sort"
                v-model="filters.orderByMetric"
                @change="changeFilters"
                ><el-option
                  v-for="sort in sorts"
                  :key="sort.value"
                  :value="sort.value"
                  :label="sort.label"
              /></el-select>
            </div>
          </div>

          <el-alert
            v-if="error"
            :title="error"
            type="error"
            :closable="false"
            show-icon
            class="result-alert"
          />

          <div v-loading="loading" class="results" :aria-busy="loading">
            <div
              v-if="filters.tab === 'tag' && tags.length"
              class="tag-ranking"
            >
              <div class="tag-list-header" aria-hidden="true">
                <span>{{ text("排名", "Rank") }}</span
                ><span>{{ text("标签", "Hashtag") }}</span>
                <span>{{ text("发帖量", "Posts") }}</span
                ><span>{{ text("播放量", "Views") }}</span
                ><span>{{ text("热度趋势", "Trend") }}</span>
              </div>
              <div
                :key="filters.currentPage"
                class="tag-list-scroll"
                :aria-label="text('标签排名', 'Hashtag rankings')"
                tabindex="0"
              >
                <button
                  v-for="(tag, index) in tags"
                  :key="tag.hashtagID"
                  class="tag-row"
                  :class="{
                    'is-selected': selectedTag?.hashtagID === tag.hashtagID
                  }"
                  :aria-pressed="selectedTag?.hashtagID === tag.hashtagID"
                  @click="openDetail(tag)"
                >
                  <span class="rank-number">{{
                    tag.rankIndex ||
                    (filters.currentPage - 1) * filters.pageSize + index + 1
                  }}</span>
                  <span class="tag-identity"
                    ><strong :title="`#${tag.hashtagName}`"
                      >#{{ tag.hashtagName }}</strong
                    >
                    <span class="creator-stack"
                      ><TrendImage
                        v-for="creator in (tag.topCreators || []).slice(0, 3)"
                        :key="creator.ttUID || creator.nickname"
                        :src="creator.avatarURL"
                        :alt="creator.nickname"
                        avatar
                        :title="creator.nickname"
                    /></span>
                  </span>
                  <span class="tag-value" :title="tag.publishCnt"
                    ><small>{{ text("发帖量", "Posts") }}</small
                    >{{ count(tag.publishCnt) }}</span
                  >
                  <span class="tag-value" :title="tag.vv"
                    ><small>{{ text("播放量", "Views") }}</small
                    >{{ count(tag.vv) }}</span
                  >
                  <TrendChart
                    :points="tag.popularityCurve || []"
                    compact
                    :label="`#${tag.hashtagName}`"
                  />
                </button>
              </div>
            </div>
            <div
              v-else-if="filters.tab === 'video' && videos.length"
              class="video-grid"
            >
              <article
                v-for="(video, index) in videos"
                :key="video.itemID"
                class="video-item"
              >
                <button
                  class="video-cover"
                  :aria-label="
                    text('查看视频详情', 'View video details') +
                    ': ' +
                    (video.author?.name || '')
                  "
                  @click="openDetail(video)"
                >
                  <TrendImage
                    :src="video.coverURL"
                    :alt="video.title || text('视频封面', 'Video cover')"
                  />
                  <span class="video-rank">{{
                    (filters.currentPage - 1) * filters.pageSize + index + 1
                  }}</span>
                  <span class="cover-action"
                    ><PlayIcon class="trend-ui-icon"
                  /></span>
                </button>
                <div class="video-body">
                  <div class="video-author">
                    <TrendImage
                      :src="video.author?.avatarURL"
                      :alt="video.author?.name || 'TikTok'"
                      avatar
                    />
                    <div>
                      <strong :title="video.author?.name">{{
                        video.author?.name || "—"
                      }}</strong
                      ><small
                        >{{ count(video.author?.followers) }}
                        {{ text("粉丝", "followers") }}</small
                      >
                    </div>
                  </div>
                  <button
                    class="video-caption"
                    :title="video.title"
                    @click="openDetail(video)"
                  >
                    {{
                      video.title ||
                      text("未提供视频文案", "No caption provided")
                    }}
                  </button>
                  <dl class="video-metrics">
                    <div :class="{ 'is-active': filters.orderByMetric === 1 }">
                      <dt>{{ text("播放量", "Views") }}</dt>
                      <dd>{{ count(video.metrics?.videoViews) }}</dd>
                    </div>
                    <div :class="{ 'is-active': filters.orderByMetric === 2 }">
                      <dt>{{ text("互动率", "Engagement") }}</dt>
                      <dd>{{ percentage(video.metrics?.engagementRate) }}</dd>
                    </div>
                    <div :class="{ 'is-active': filters.orderByMetric === 3 }">
                      <dt>{{ text("6 秒观看率", "6s views") }}</dt>
                      <dd>{{ percentage(video.metrics?.sixSecondsVTR) }}</dd>
                    </div>
                  </dl>
                </div>
              </article>
            </div>
            <el-empty
              v-if="!loading && !error && !currentCount"
              :description="
                text(
                  '该地区和时间范围暂无可访问的数据',
                  'No accessible data for this region and time range'
                )
              "
            />
          </div>
          <footer
            v-if="currentCount || filters.currentPage > 1"
            class="pagination-bar"
          >
            <span
              >{{ text("本页", "This page") }} {{ currentCount }}
              {{ text("条", "records") }} ·
              {{ text("原站总数", "Reported total") }} {{ total }}</span
            ><el-pagination
              v-if="isArchive"
              :current-page="filters.currentPage"
              :page-size="filters.pageSize"
              :total="total"
              layout="prev, pager, next"
              @current-change="changePage"
            />
            <div v-else class="live-pagination">
              <el-button
                :disabled="loading || filters.currentPage <= 1"
                @click="changePage(filters.currentPage - 1)"
                >{{ text("上一页", "Previous") }}</el-button
              ><span>{{ filters.currentPage }}</span
              ><el-button
                :disabled="loading || !hasMore"
                @click="changePage(filters.currentPage + 1)"
                >{{ text("下一页", "Next") }}</el-button
              >
            </div>
          </footer>
        </template>
      </section>
      <aside
        v-if="inlineTag"
        class="tag-detail-panel"
        :aria-label="text('标签分析', 'Hashtag analytics')"
        :aria-busy="detailLoading"
      >
        <div v-loading="detailLoading" class="inline-detail-content">
          <header v-if="selectedTag" class="tag-detail-heading">
            <h2>#{{ selectedTag.hashtagName }}</h2>
            <span>{{ text("标签分析", "Hashtag analytics") }}</span>
          </header>
          <el-alert
            v-if="detailError"
            :title="detailError"
            type="error"
            :closable="false"
            show-icon
          />
          <el-button
            v-if="detailError && selectedTag"
            class="detail-retry"
            @click="openDetail(selectedTag)"
            >{{ text("重试", "Retry") }}</el-button
          >
          <TrendTagAnalysis
            v-if="tagDetail"
            :detail="tagDetail"
            @preview="previewRelated"
          />
          <el-empty
            v-else-if="!detailLoading && !detailError"
            :description="
              text(
                '选择标签，查看趋势与受众分析',
                'Select a hashtag to explore its trends and audience'
              )
            "
          />
        </div>
      </aside>
    </div>
    <el-drawer
      v-model="drawer"
      :title="selectedTitle"
      size="min(960px, 100vw)"
      destroy-on-close
      @close="cancelDrawerRequest"
    >
      <div v-loading="detailLoading" class="detail-body">
        <el-alert
          v-if="detailError"
          :title="detailError"
          type="error"
          :closable="false"
          show-icon
        />
        <el-button
          v-if="detailError && selectedTag && filters.tab === 'tag'"
          class="detail-retry"
          @click="openDetail(selectedTag)"
          >{{ text("重试", "Retry") }}</el-button
        >
        <TrendTagAnalysis
          v-if="tagDetail"
          :detail="tagDetail"
          @preview="previewRelated"
        />
        <template v-if="videoDetail">
          <div class="video-detail-grid">
            <div class="detail-cover">
              <TrendVideoPlayer
                :item-i-d="videoDetail.itemID"
                :src="videoDetail.videoURL"
                :cover="videoDetail.coverURL"
                :title="videoDetail.title"
              />
            </div>
            <div>
              <div class="video-author">
                <TrendImage
                  :src="videoDetail.author?.avatarURL"
                  :alt="videoDetail.author?.name || 'TikTok'"
                  avatar
                />
                <div>
                  <strong>{{ videoDetail.author?.name }}</strong
                  ><small
                    >{{ count(videoDetail.author?.followers) }}
                    {{ text("粉丝", "followers") }}</small
                  >
                </div>
              </div>
              <p class="detail-caption">
                {{
                  videoDetail.title ||
                  text("未提供视频文案", "No caption provided")
                }}
              </p>
              <el-link
                v-if="
                  trendTikTokVideoURL(
                    videoDetail.itemID,
                    videoDetail.author?.handle
                  )
                "
                :href="
                  trendTikTokVideoURL(
                    videoDetail.itemID,
                    videoDetail.author?.handle
                  )
                "
                target="_blank"
                rel="noopener noreferrer"
                type="primary"
                >{{
                  text("在 TikTok 查看视频", "View video on TikTok")
                }}</el-link
              >
            </div>
          </div>
          <section class="detail-section">
            <h2>{{ text("表现数据", "Performance") }}</h2>
            <dl class="video-statistics">
              <div>
                <dt>{{ text("累计播放量", "Lifetime video views") }}</dt>
                <dd>{{ count(videoDetail.metrics?.videoViewsLifeTime) }}</dd>
              </div>
              <div>
                <dt>{{ text("期间播放量", "Period video views") }}</dt>
                <dd>{{ count(videoDetail.metrics?.videoViews) }}</dd>
              </div>
              <div>
                <dt>{{ text("互动率", "Engagement rate") }}</dt>
                <dd>{{ percentage(videoDetail.metrics?.engagementRate) }}</dd>
              </div>
              <div>
                <dt>{{ text("6 秒观看率", "6s view-through rate") }}</dt>
                <dd>{{ percentage(videoDetail.metrics?.sixSecondsVTR) }}</dd>
              </div>
            </dl>
          </section>
        </template>
      </div>
    </el-drawer>
    <el-dialog
      v-model="relatedVisible"
      :title="text('视频预览', 'Video preview')"
      width="min(600px, 94vw)"
      destroy-on-close
    >
      <div class="related-player">
        <TrendVideoPlayer
          v-if="relatedVideo"
          :item-i-d="relatedVideo.itemID"
          :src="relatedVideo.videoURL"
          :cover="relatedVideo.coverURL"
        />
      </div>
      <template #footer>
        <el-link
          v-if="relatedVideo && trendTikTokVideoURL(relatedVideo.itemID)"
          :href="trendTikTokVideoURL(relatedVideo.itemID)"
          target="_blank"
          rel="noopener noreferrer"
          type="primary"
          >{{ text("在 TikTok 查看视频", "View video on TikTok") }}</el-link
        >
      </template>
    </el-dialog>
  </div>
</template>

<style scoped lang="scss">
.trends-page {
  --el-text-color-secondary: #73776e;
  min-height: 100%;
  padding: 12px 14px 20px;
  color: var(--el-text-color-primary);
  background-image: none !important;
  font-variant-numeric: tabular-nums;
}
.resource-toolbar {
  padding: 0 0 22px;
  h1 {
    margin: 0;
    font-size: 28px;
    font-weight: 750;
    line-height: 1.3;
    letter-spacing: -0.025em;
  }
  p {
    margin: 10px 0 0;
    font-size: 14px;
    line-height: 1.6;
    color: var(--el-text-color-secondary);
  }
}
.trends-workspace,
.trend-browser {
  min-width: 0;
}
.trends-workspace.is-tag-layout {
  position: relative;
  margin-top: -8px;
  display: grid;
  grid-template-columns: minmax(0, 44fr) minmax(0, 56fr);
  overflow: hidden;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  .trend-browser {
    padding: 0 18px;
  }
  .filter-field {
    flex: 1;
    width: auto;
    min-width: 0;
  }
  .filter-bar {
    gap: 14px;
    padding: 12px 0 16px;
  }
  .filter-actions {
    position: absolute;
    top: 18px;
    right: 20px;
    max-width: calc(56% - 48px);
    justify-content: flex-end;
    margin: 0;
  }
  .source-note {
    margin: 0 0 18px;
    font-size: 12px;
  }
  .pagination-bar {
    flex-wrap: wrap;
    justify-content: center;
    gap: 12px;
    padding: 18px 0;
  }
  .pagination-bar > span {
    width: 100%;
    text-align: center;
    font-size: 12px;
  }
}
.trend-browser :deep(.el-tabs__header) {
  margin: 0 0 8px;
}
.trend-browser :deep(.el-tabs__item) {
  height: 48px;
  font-size: 15px;
  padding: 0 22px;
}
.trend-browser :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--el-border-color-light);
}
.trend-browser :deep(.el-tabs__content) {
  display: none;
}
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 18px;
  padding: 14px 0 20px;
}
.filter-field {
  width: 228px;
  label {
    display: block;
    margin-bottom: 8px;
    color: var(--el-text-color-regular);
    font-size: 13px;
  }
  .el-select {
    width: 100%;
  }
}
.filter-bar :deep(.el-select__wrapper),
.filter-actions :deep(.el-button) {
  min-height: 36px;
}
.filter-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-left: auto;
}
.filter-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}
.filter-actions :deep(.el-button--primary) {
  min-width: 128px;
}
.source-note {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin: 0 0 24px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.7;
}
.source-note > svg {
  margin-top: 3px;
}
.sort-bar {
  margin: 0 0 22px;
}
.sort-bar :deep(.el-radio-button__inner) {
  padding: 10px 20px;
}
.mobile-sort {
  display: none;
}
.results {
  min-height: 280px;
}
.result-alert {
  margin: 12px 0;
}
.trends-page :deep(.trend-ui-icon),
.detail-body :deep(.trend-ui-icon) {
  display: inline-block;
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  vertical-align: middle;
}
.tag-list-header,
.tag-row {
  display: grid;
  grid-template-columns: 26px minmax(0, 1fr) 64px 64px 80px;
  align-items: center;
  column-gap: 10px;
}
.tag-list-header {
  min-height: 44px;
  padding: 0 10px;
  background: var(--el-fill-color-lighter);
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.tag-list-header > span:nth-child(3),
.tag-list-header > span:nth-child(4) {
  text-align: right;
}
.tag-list-header > span:last-child {
  text-align: center;
}
.tag-list-scroll {
  max-height: 620px;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-gutter: stable;
}
.tag-row {
  position: relative;
  width: 100%;
  min-height: 100px;
  padding: 16px 10px;
  color: var(--el-text-color-primary);
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: none;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.tag-row:hover {
  background: var(--el-fill-color-lighter);
}
.tag-row.is-selected {
  background: var(--el-color-primary-light-9);
  box-shadow: inset 2px 0 var(--el-color-primary);
}
.rank-number {
  color: var(--el-text-color-secondary);
  font-size: 14px;
}
.tag-row.is-selected .rank-number {
  color: var(--el-color-primary);
  font-weight: 600;
}
.tag-identity {
  min-width: 0;
}
.tag-identity > strong {
  display: block;
  overflow-wrap: anywhere;
  font-size: 14px;
  line-height: 1.5;
  font-weight: 650;
}
.creator-stack {
  display: flex;
  margin-top: 9px;
}
.creator-stack :deep(.is-avatar) {
  flex: 0 0 22px;
  width: 22px;
  height: 22px;
  margin-left: -4px;
  border: 2px solid var(--el-bg-color);
}
.creator-stack :deep(.is-avatar:first-child) {
  margin-left: 0;
}
.tag-value {
  text-align: right;
  font-size: 13px;
  white-space: nowrap;
}
.tag-value small {
  display: none;
}
.tag-row :deep(.compact) {
  width: 100%;
  height: 42px;
}
.tag-detail-panel {
  min-width: 0;
  padding: 18px 24px;
  border-left: 1px solid var(--el-border-color-light);
  border-radius: 0 !important;
  box-shadow: none;
}
.tag-detail-panel :deep(.trend-chart:not(.compact)) {
  height: 190px;
}
.inline-detail-content {
  min-height: 420px;
}
.tag-detail-heading {
  min-width: 0;
  min-height: 36px;
  padding-right: 140px;
  margin-bottom: 12px;
}
.tag-detail-heading h2 {
  margin: 0 0 4px;
  overflow-wrap: anywhere;
  font-size: 21px;
  font-weight: 700;
  line-height: 1.4;
  letter-spacing: -0.025em;
}
.tag-detail-heading > span {
  display: block;
  line-height: 18px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.detail-retry {
  margin-top: 12px;
}
.pagination-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 28px 0 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.live-pagination {
  display: flex;
  align-items: center;
  gap: 14px;
}
.video-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 30px 22px;
}
.video-item {
  min-width: 0;
}
.video-cover {
  position: relative;
  display: block;
  width: 100%;
  aspect-ratio: 4 / 5;
  padding: 0;
  overflow: hidden;
  cursor: pointer;
  background: var(--el-fill-color-light);
  border: none;
  border-radius: 7px;
}
.video-rank {
  position: absolute;
  top: 9px;
  left: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 27px;
  height: 27px;
  padding: 0 6px;
  border-radius: 4px;
  background: rgb(82 102 0 / 88%);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
}
.cover-action {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: rgb(0 0 0 / 8%);
  transition: background-color 160ms ease-out;
}
.video-cover .cover-action :deep(.trend-ui-icon) {
  width: 46px;
  height: 46px;
  filter: drop-shadow(0 1px 3px rgb(0 0 0 / 28%));
}
.video-cover:hover .cover-action,
.video-cover:focus-visible .cover-action {
  background: rgb(0 0 0 / 22%);
}
.video-body {
  padding-top: 12px;
}
.video-author {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}
.video-author > div {
  min-width: 0;
}
.video-author strong {
  display: block;
  overflow: hidden;
  font-size: 14px;
  line-height: 1.5;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.video-author small {
  display: block;
  margin-top: 1px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.video-caption {
  display: -webkit-box;
  width: 100%;
  height: 44px;
  padding: 0;
  margin: 9px 0 12px;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  text-align: left;
  font-size: 14px;
  line-height: 22px;
  color: var(--el-text-color-regular);
  overflow-wrap: anywhere;
  cursor: pointer;
  background: transparent;
  border: 0;
}
.video-caption:hover {
  color: var(--el-color-primary);
}
.video-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin: 0;
}
.video-metrics > div {
  min-width: 0;
}
.video-metrics dt {
  margin: 0 0 5px;
  color: var(--el-text-color-secondary);
  font-size: 11px;
  line-height: 1.5;
}
.video-metrics dd {
  margin: 0;
  font-size: 14px;
  font-weight: 550;
  white-space: nowrap;
}
.video-metrics .is-active dd {
  color: var(--el-color-primary);
  font-weight: 700;
}
.detail-body {
  --el-text-color-secondary: #73776e;
  min-height: 260px;
}
.detail-section {
  margin: 28px 0;
}
.detail-section h2 {
  margin: 0 0 16px;
  font-size: 17px;
  font-weight: 600;
}
.video-detail-grid {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 28px;
}
.detail-cover {
  aspect-ratio: 3 / 4;
  overflow: hidden;
  border-radius: 8px;
}
.detail-caption {
  margin: 20px 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.video-statistics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}
.video-statistics dt {
  margin-bottom: 6px;
  color: var(--el-text-color-secondary);
}
.video-statistics dd {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
}
.related-player {
  height: min(65vh, 580px);
}
button:focus-visible,
.tag-list-scroll:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: -2px;
}
:global(.dark) .trends-page,
:global(.dark) .detail-body {
  --el-text-color-secondary: #a3aab5;
}

.filter-actions :deep(.el-button--primary) {
  color: #fff !important;
  background: #66752a !important;
  border-color: #66752a !important;
}
.filter-actions :deep(.el-button--primary:hover),
.filter-actions :deep(.el-button--primary:focus-visible) {
  background: #526121 !important;
  border-color: #526121 !important;
}

@media (width <= 1360px) {
  .trends-page {
    padding: 12px 14px 20px;
  }
  .trends-workspace.is-tag-layout {
    grid-template-columns: minmax(0, 46fr) minmax(0, 54fr);
  }
  .tag-detail-panel {
    padding: 20px;
  }
  .tag-list-header,
  .tag-row {
    grid-template-columns: 26px minmax(0, 1fr) 60px 60px 72px;
    gap: 8px;
  }
}
@media (width <= 1179px) {
  .video-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .tag-list-header,
  .tag-row {
    grid-template-columns: 44px minmax(0, 1fr) 100px 100px 140px;
  }
  .tag-list-scroll {
    max-height: none;
  }
}
@media (width <= 760px) {
  .trends-page {
    padding: 20px 14px;
  }
  .resource-toolbar {
    padding-bottom: 18px;
  }
  .resource-toolbar h1 {
    font-size: 25px;
  }
  .resource-toolbar p {
    font-size: 13px;
  }
  .filter-bar {
    gap: 12px;
  }
  .filter-field {
    flex: 1 1 120px;
    width: auto;
    min-width: 0;
  }
  .filter-actions {
    width: 100%;
    margin: 0;
  }
  .source-note {
    margin-bottom: 18px;
    font-size: 12px;
  }
  .desktop-sort {
    display: none;
  }
  .mobile-sort {
    display: block;
    width: 100%;
  }
  .tag-list-header {
    display: none;
  }
  .tag-row {
    grid-template-columns: 24px minmax(0, 1fr) minmax(0, 1fr) 90px;
    row-gap: 12px;
    padding: 18px 4px;
  }
  .tag-identity {
    grid-column: 2 / -1;
  }
  .tag-identity > strong {
    font-size: 14px;
  }
  .creator-stack {
    display: none;
  }
  .tag-value {
    text-align: left;
    font-size: 14px;
  }
  .tag-value:nth-child(3) {
    grid-column: 2;
  }
  .tag-value small {
    display: block;
    margin-bottom: 5px;
    color: var(--el-text-color-secondary);
    font-size: 12px;
  }
  .tag-row :deep(.compact) {
    grid-column: 4;
  }
  .video-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 24px 14px;
  }
  .video-author {
    gap: 7px;
  }
  .video-author :deep(.is-avatar) {
    flex-basis: 28px;
    width: 28px;
    height: 28px;
  }
  .video-author strong {
    font-size: 13px;
  }
  .video-caption {
    font-size: 13px;
  }
  .video-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }
  .video-metrics > div:last-child {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .video-metrics > div:last-child dt {
    margin: 0;
  }
  .video-metrics dd {
    font-size: 13px;
  }
  .pagination-bar {
    align-items: flex-start;
    flex-direction: column;
  }
  .video-detail-grid {
    grid-template-columns: 1fr;
  }
  .detail-cover {
    width: min(100%, 280px);
  }
}
@media (prefers-reduced-motion: reduce) {
  .cover-action {
    transition: none;
  }
}
</style>
