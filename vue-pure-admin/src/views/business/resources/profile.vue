<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import CooperationTypeTags from "@/components/CooperationTypeTags/index.vue";
import PlatformIconBadge from "@/components/PlatformIconBadge/index.vue";
import {
  getCooperationList,
  getResourceList,
  getResourcePosts
} from "@/api/business";
import { fieldLabel } from "@/utils/fieldI18n";

defineOptions({ name: "BusinessResourceProfile" });

const route = useRoute();
const router = useRouter();
const { locale } = useI18n();
const loading = ref(false);
const resource = ref<any>(null);
const cooperations = ref<any[]>([]);
const posts = ref<any[]>([]);
const activePlatform = ref("");
const avatarFailed = ref(false);
const cooperationPage = ref(1);
const cooperationPageSize = ref(10);

const metricDefinitions = [
  { key: "followers", label: "粉丝量", icon: "ri:user-3-line" },
  { key: "views", label: "近30天平均播放量", icon: "ri:play-circle-line" },
  { key: "interactions", label: "近30天平均互动量", icon: "ri:chat-3-line" },
  {
    key: "interactionRate",
    label: "近30天平均互动率",
    icon: "ri:heart-3-line"
  },
  {
    key: "contentCount",
    label: "近30天内容发布数",
    icon: "ri:file-list-3-line"
  }
];

const profileAccounts = computed(() => platformAccounts(resource.value));
const activeAccount = computed(
  () =>
    profileAccounts.value.find(
      item => item.platform === activePlatform.value
    ) ||
    profileAccounts.value[0] ||
    null
);
const selectedCooperations = computed(() =>
  cooperations.value
    .filter(item => Number(item.resourceId) === Number(resource.value?.id || 0))
    .sort((a, b) => dateRank(b) - dateRank(a))
);
const pagedCooperations = computed(() => {
  const start = (cooperationPage.value - 1) * cooperationPageSize.value;
  return selectedCooperations.value.slice(
    start,
    start + cooperationPageSize.value
  );
});
const latestCooperation = computed(() => selectedCooperations.value[0] || null);
const cooperationStats = computed(() =>
  selectedCooperations.value.reduce(
    (stat, item) => {
      stat.count += 1;
      stat.totalReach += primaryReach(item);
      stat.totalEngagements += numberValue(item.engagementCount);
      stat.totalCost += numberValue(item.quoteAmount);
      return stat;
    },
    { count: 0, totalReach: 0, totalEngagements: 0, totalCost: 0 }
  )
);
const averageReach = computed(() =>
  cooperationStats.value.count
    ? cooperationStats.value.totalReach / cooperationStats.value.count
    : 0
);
const engagementRate = computed(() =>
  ratioPercent(
    cooperationStats.value.totalEngagements,
    cooperationStats.value.totalReach
  )
);
const resourceKind = computed(() =>
  isMediaResource(resource.value) ? "媒体" : "达人"
);
const activePosts = computed(() => {
  const selected = normalizePlatform(activePlatform.value);
  if (!selected) return [...posts.value];
  const matched = posts.value.filter(
    item => normalizePlatform(item.platform) === selected
  );
  return matched.sort(
    (left, right) =>
      numberValue(right.publishedAt) - numberValue(left.publishedAt)
  );
});
const visiblePosts = computed(() => activePosts.value.slice(0, 10));
const contentTotals = computed(() =>
  activePosts.value.reduce(
    (total, post) => {
      total.views += numberValue(post.viewCount);
      total.likes += numberValue(post.likeCount);
      total.comments += numberValue(post.commentCount);
      total.shares += numberValue(post.shareCount);
      total.saves += numberValue(post.saveCount);
      return total;
    },
    { views: 0, likes: 0, comments: 0, shares: 0, saves: 0 }
  )
);
const topicTags = computed(() => buildTopicTags(activePosts.value));

function displayText(value: unknown, fallback = "-") {
  const text = String(value ?? "").trim();
  if (!text || text === "<nil>" || text === "undefined") return fallback;
  return text;
}

function numberValue(value: unknown) {
  const number = Number(value || 0);
  return Number.isFinite(number) ? number : 0;
}

function normalizePlatform(value: unknown) {
  const text = String(value || "")
    .trim()
    .toLowerCase();
  if (text.includes("youtube")) return "youtube";
  if (text.includes("instagram")) return "instagram";
  if (text.includes("tiktok")) return "tiktok";
  if (text.includes("facebook")) return "facebook";
  return text;
}

function formatCount(value: unknown) {
  const number = numberValue(value);
  if (number <= 0) return "-";
  return number.toLocaleString(locale.value === "en" ? "en-US" : "zh-CN");
}

function compactCount(value: unknown) {
  const number = numberValue(value);
  if (number <= 0) return "-";
  if (locale.value === "en") {
    return new Intl.NumberFormat("en-US", {
      notation: "compact",
      maximumFractionDigits: 1
    }).format(number);
  }
  if (number >= 100000000) return `${(number / 100000000).toFixed(1)}亿`;
  if (number >= 10000) return `${(number / 10000).toFixed(1)}万`;
  return number.toLocaleString("zh-CN");
}

function percentText(value: unknown) {
  const number = numberValue(value);
  if (number <= 0) return "-";
  const percent = number > 1 ? number : number * 100;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function ratioPercent(numerator: unknown, denominator: unknown) {
  const top = numberValue(numerator);
  const bottom = numberValue(denominator);
  if (top <= 0 || bottom <= 0) return "-";
  const percent = (top / bottom) * 100;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function isMediaResource(row: any) {
  return /媒体|media/i.test(String(row?.resourceType || ""));
}

function localizedText(row: any, field: string, fallback = "-") {
  const value =
    locale.value === "en" && row?.localized?.[field]
      ? row.localized[field]
      : row?.[field];
  return displayText(value, fallback);
}

function domainText(row: any) {
  return fieldLabel(
    displayText(
      row?.category
        ? localizedText(row, "category")
        : localizedText(row, "industry")
    )
  );
}

function marketText(row: any) {
  const parts = [
    localizedText(row, "region", ""),
    row?.market ||
      localizedText(row, "country", "") ||
      localizedText(row, "city", "")
  ]
    .map(item => displayText(item, ""))
    .filter(Boolean);
  return parts.length ? Array.from(new Set(parts)).join(" - ") : "-";
}

function tierText(row: any) {
  return fieldLabel(displayText(row?.tier, "待同步"));
}

function resourceAudience(row: any) {
  if (isMediaResource(row)) return numberValue(row?.umvMonth);
  return numberValue(row?.followers) || numberValue(row?.audienceSize);
}

function averageInteractions(row: any) {
  const rate = numberValue(row?.engagementRate);
  const normalized = rate > 1 ? rate / 100 : rate;
  return Math.round(numberValue(row?.avgViews) * normalized);
}

function platformAccounts(row: any) {
  if (!row) return [];
  const raw = Array.isArray(row.platformAccounts)
    ? row.platformAccounts
    : Array.isArray(row.platforms)
      ? row.platforms
      : [];
  const normalized = raw
    .map((item: any) =>
      typeof item === "string"
        ? { platform: item, followers: 0 }
        : {
            ...item,
            platform: item?.platform || item?.name,
            followers: numberValue(
              item?.followers ?? item?.audienceSize ?? item?.visits
            )
          }
    )
    .filter((item: any) => item.platform);
  if (row.platform) {
    normalized.push({
      platform: row.platform,
      platformUrl: row.platformUrl,
      platformHandle: row.platformHandle,
      contact: row.contact,
      followers: resourceAudience(row),
      avgViews: numberValue(row.avgViews),
      avgInteractions: averageInteractions(row),
      engagementRate: numberValue(row.engagementRate),
      contentCount: numberValue(row.videoCount),
      weeklyDeltas: row.weeklyDeltas,
      viewVolatility: row.viewVolatility
    });
  }
  const unique = new Map<string, any>();
  normalized.forEach((item: any) => {
    const key = String(item.platform).trim().toLowerCase();
    const existing = unique.get(key);
    if (!existing || item.followers > existing.followers) unique.set(key, item);
  });
  return Array.from(unique.values()).sort(
    (left, right) => right.followers - left.followers
  );
}

function metricValue(account: any, key: string) {
  if (!account) return "-";
  if (key === "followers") return compactCount(account.followers);
  if (key === "views") return compactCount(account.avgViews);
  if (key === "interactions") return compactCount(account.avgInteractions);
  if (key === "interactionRate") return percentText(account.engagementRate);
  if (key === "contentCount") return formatCount(account.contentCount);
  return "-";
}

function metricDelta(row: any, key: string) {
  const aliases: Record<string, string[]> = {
    followers: ["audience", "followers", "audienceSize"],
    views: ["views", "avgViews", "averageViews"],
    interactions: ["interactions", "avgInteractions"],
    interactionRate: ["interactionRate", "engagementRate"],
    contentCount: ["contentCount", "posts"]
  };
  const source = row?.weeklyDeltas || row?.weekOverWeek || {};
  for (const alias of aliases[key] || [key]) {
    const raw = source?.[alias] ?? row?.[`${alias}WeeklyDelta`];
    if (raw === null || raw === undefined || raw === "") continue;
    const value = Number(raw);
    if (Number.isFinite(value)) return value;
  }
  return null;
}

function deltaClass(account: any, key: string) {
  const value = metricDelta(account, key);
  if (value === null) return "is-empty";
  return value >= 0 ? "is-up" : "is-down";
}

function deltaText(account: any, key: string) {
  const value = metricDelta(account, key);
  return value === null ? "--" : `${Math.abs(value).toFixed(1)}%`;
}

function deltaIcon(account: any, key: string) {
  const value = metricDelta(account, key);
  return value !== null && value >= 0
    ? "ri:arrow-up-line"
    : "ri:arrow-down-line";
}

function accountFromHomepage(value: unknown) {
  const raw = String(value || "").trim();
  if (!raw) return "-";
  try {
    const url = new URL(/^https?:\/\//i.test(raw) ? raw : `https://${raw}`);
    const segments = decodeURIComponent(url.pathname)
      .split("/")
      .map(item => item.trim())
      .filter(Boolean);
    const handle = segments.find(item => item.startsWith("@"));
    if (handle) return handle;
    const last = segments.at(-1);
    return last ? (last.startsWith("@") ? last : `@${last}`) : url.hostname;
  } catch {
    return raw;
  }
}

function accountTitle(row: any) {
  const handle = displayText(row?.platformHandle, "");
  if (handle) {
    if (/^https?:\/\//i.test(handle)) return accountFromHomepage(handle);
    return handle.startsWith("@") ? handle : `@${handle}`;
  }
  return accountFromHomepage(row?.platformUrl);
}

function contactText() {
  const uploaded = selectedCooperations.value
    .map(item => item.primaryContact || item.contact)
    .find(Boolean);
  return (
    displayText(
      uploaded || activeAccount.value?.contact || resource.value?.contact,
      ""
    ) || "/"
  );
}

function notesText() {
  return (
    displayText(latestCooperation.value?.notes || resource.value?.notes, "") ||
    "/"
  );
}

function costRange() {
  const costs = selectedCooperations.value
    .map(item => numberValue(item.quoteAmount))
    .filter(value => value > 0);
  if (!costs.length) return "-";
  const minimum = Math.min(...costs);
  const maximum = Math.max(...costs);
  const usd = (value: number) =>
    `$${value.toLocaleString("en-US", { maximumFractionDigits: 0 })}`;
  return minimum === maximum
    ? usd(minimum)
    : `${usd(minimum)} – ${usd(maximum)}`;
}

function averageCpm() {
  const rows = selectedCooperations.value.filter(
    item => numberValue(item.quoteAmount) > 0 && primaryReach(item) > 0
  );
  if (!rows.length) return "-";
  const value =
    rows.reduce(
      (sum, item) =>
        sum + (numberValue(item.quoteAmount) * 1000) / primaryReach(item),
      0
    ) / rows.length;
  return `$${value.toFixed(2)}`;
}

function coefficientOfVariation(values: number[]) {
  const valid = values.filter(value => Number.isFinite(value) && value > 0);
  if (valid.length < 2) return null;
  const mean = valid.reduce((sum, value) => sum + value, 0) / valid.length;
  const variance =
    valid.reduce((sum, value) => sum + (value - mean) ** 2, 0) / valid.length;
  return mean > 0 ? (Math.sqrt(variance) / mean) * 100 : null;
}

function cooperationVolatility() {
  return coefficientOfVariation(
    selectedCooperations.value.map(item => primaryReach(item))
  );
}

function volatilityText(value: unknown) {
  if (value === null || value === undefined || value === "") return "/";
  const number = Number(value);
  return Number.isFinite(number) ? `${number.toFixed(1)}%` : "/";
}

function stabilityText(value: unknown) {
  if (value === null || value === undefined || value === "") return "待评估";
  const number = Number(value);
  if (!Number.isFinite(number)) return "待评估";
  if (number <= 30) return "稳定";
  if (number <= 60) return "较稳定";
  return "波动较大";
}

function primaryReach(row: any) {
  return numberValue(row?.impressions) || numberValue(row?.views);
}

function dateRank(row: any) {
  const parsed = new Date(row?.publishTime || row?.releaseDate || 0).getTime();
  return Number.isFinite(parsed) ? parsed : numberValue(row?.updatedAt);
}

function cooperationDate(row: any) {
  const raw = row?.publishTime || row?.releaseDate;
  if (!raw) return "/";
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return String(raw).slice(0, 10);
  return parsed.toLocaleDateString("en-CA");
}

function cooperationLink(row: any) {
  return (
    String(row?.finalLink || "").trim() ||
    String(row?.deliverableLinks || "")
      .split(/[\n,;]/)[0]
      ?.trim() ||
    ""
  );
}

function cooperationCover(row: any) {
  return (
    row?.contentCoverUrl ||
    row?.contentCoverLocalUrl ||
    row?.contentCoverRemoteUrl ||
    ""
  );
}

function openUrl(url: string) {
  if (url) window.open(url, "_blank", "noopener,noreferrer");
}

function handleCooperationPageSize(size: number) {
  cooperationPageSize.value = size;
  cooperationPage.value = 1;
}

function publishedDate(value: unknown) {
  const time = Number(value || 0);
  if (!Number.isFinite(time) || time <= 0) return "-";
  return new Date(time).toLocaleDateString("en-CA");
}

function durationText(value: unknown) {
  const total = Math.max(0, Math.floor(numberValue(value)));
  if (!total) return "--:--";
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  return hours
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`
    : `${minutes}:${String(seconds).padStart(2, "0")}`;
}

function buildTopicTags(rows: any[]) {
  if (!rows.length) return [];
  const counts = new Map<string, number>();
  rows.forEach(post => {
    const source = `${post.title || ""} ${post.description || ""}`;
    for (const match of source.matchAll(/[#＃]([\p{L}\p{N}_-]{2,30})/gu)) {
      const tag = match[1].trim();
      if (tag) counts.set(tag, (counts.get(tag) || 0) + 1);
    }
  });
  if (!counts.size) {
    const fallbacks = [
      ...(Array.isArray(resource.value?.tagNames)
        ? resource.value.tagNames
        : []),
      resource.value?.category,
      resource.value?.industry
    ];
    fallbacks.filter(Boolean).forEach(tag => counts.set(String(tag), 1));
  }
  const total = Array.from(counts.values()).reduce(
    (sum, count) => sum + count,
    0
  );
  return Array.from(counts.entries())
    .sort((left, right) => right[1] - left[1])
    .slice(0, 5)
    .map(([name, count], index) => ({
      name,
      count,
      share: total ? (count / total) * 100 : 0,
      scale: 1.45 - index * 0.13
    }));
}

async function loadProfile() {
  const id = Number(route.query.id || 0);
  if (!id) {
    resource.value = null;
    return;
  }
  loading.value = true;
  try {
    const [resourceResponse, cooperationResponse, postResponse] =
      await Promise.all([
        getResourceList({
          id,
          currentPage: 1,
          pageSize: 1,
          locale: locale.value
        }),
        getCooperationList(),
        getResourcePosts({
          resourceId: id,
          currentPage: 1,
          pageSize: 100,
          locale: locale.value
        })
      ]);
    resource.value = resourceResponse.data?.list?.[0] || null;
    cooperations.value = Array.isArray(cooperationResponse.data?.list)
      ? cooperationResponse.data.list
      : [];
    posts.value = Array.isArray(postResponse.data?.list)
      ? postResponse.data.list
      : [];
    activePlatform.value = platformAccounts(resource.value)[0]?.platform || "";
    cooperationPage.value = 1;
    avatarFailed.value = false;
    if (!resource.value) ElMessage.warning("未找到该资源档案");
  } catch {
    resource.value = null;
    posts.value = [];
    ElMessage.warning("资源档案加载失败，请稍后重试");
  } finally {
    loading.value = false;
  }
}

watch(() => route.query.id, loadProfile);
watch(
  () => selectedCooperations.value.length,
  total => {
    const lastPage = Math.max(1, Math.ceil(total / cooperationPageSize.value));
    cooperationPage.value = Math.min(cooperationPage.value, lastPage);
  }
);
onMounted(loadProfile);
</script>

<template>
  <main v-loading="loading" class="creator-profile-page">
    <template v-if="resource">
      <header class="profile-hero">
        <div class="profile-person">
          <span class="profile-avatar">
            <span>{{ displayText(resource.name, "?").slice(0, 1) }}</span>
            <img
              v-if="resource.avatarUrl && !avatarFailed"
              :src="resource.avatarUrl"
              :alt="resource.name"
              @error="avatarFailed = true"
            />
          </span>
          <div class="profile-identity">
            <div class="profile-name-line">
              <h1>{{ displayText(resource.name) }}</h1>
              <span>{{ accountTitle(resource) }}</span>
            </div>
            <div class="profile-meta-line">
              <span>{{ resourceKind }}</span
              ><i>·</i> <span>{{ domainText(resource) }}</span
              ><i>·</i> <span>{{ marketText(resource) }}</span
              ><i>·</i>
              <button
                v-for="account in profileAccounts"
                :key="`link-${account.platform}`"
                type="button"
                class="platform-link"
                :title="`${account.platform} 主页`"
                :disabled="!account.platformUrl"
                @click="openUrl(account.platformUrl)"
              >
                <PlatformIconBadge :platform="account.platform" />
              </button>
            </div>
          </div>
        </div>
        <div class="hero-actions">
          <span class="tier-badge"
            ><IconifyIconOnline icon="ri:vip-crown-2-fill" />
            {{ tierText(resource) }}{{ resourceKind }}</span
          >
          <el-button text @click="router.push('/business/resources')">
            <IconifyIconOnline icon="ri:arrow-left-line" /> 返回资源库
          </el-button>
        </div>
      </header>

      <section class="profile-section">
        <div class="section-heading">
          <strong
            ><IconifyIconOnline icon="ri:bar-chart-box-line" />
            {{ resourceKind }}信息</strong
          >
          <div class="section-heading-side">
            <span class="refresh-note"
              ><IconifyIconOnline icon="ri:information-line" />
              数据按周维度更新，百分比为周环比</span
            >
            <div class="platform-tabs">
              <button
                v-for="account in profileAccounts"
                :key="`tab-${account.platform}`"
                type="button"
                :class="['platform-tab', { active: account === activeAccount }]"
                @click="activePlatform = account.platform"
              >
                <PlatformIconBadge :platform="account.platform" />
                <span>{{ account.platform }}</span>
              </button>
            </div>
          </div>
        </div>
        <div class="basic-grid">
          <article
            v-for="metric in metricDefinitions"
            :key="metric.key"
            class="metric-card"
          >
            <span class="metric-icon"
              ><IconifyIconOnline :icon="metric.icon"
            /></span>
            <span class="metric-label">{{ fieldLabel(metric.label) }}</span>
            <strong>{{ metricValue(activeAccount, metric.key) }}</strong>
            <div class="metric-footer">
              <span :class="['delta', deltaClass(activeAccount, metric.key)]">
                <IconifyIconOnline
                  v-if="metricDelta(activeAccount, metric.key) !== null"
                  :icon="deltaIcon(activeAccount, metric.key)"
                />
                {{ deltaText(activeAccount, metric.key) }}
              </span>
              <el-tooltip
                v-if="metric.key === 'views'"
                content="近7天单日播放量标准差 ÷ 近7天单日播放量平均值 × 100%"
                placement="top"
              >
                <span class="volatility"
                  >波动指数 {{ volatilityText(activeAccount?.viewVolatility) }}
                  <IconifyIconOnline icon="ri:information-line"
                /></span>
              </el-tooltip>
            </div>
          </article>
          <article class="metric-card">
            <span class="metric-icon"
              ><IconifyIconOnline icon="ri:mail-line"
            /></span>
            <span class="metric-label">联系方式</span>
            <strong class="contact-value">{{ contactText() }}</strong>
            <span class="contact-hint">项目上传信息</span>
          </article>
        </div>
      </section>

      <section v-if="resourceKind === '达人'" class="profile-section">
        <div class="section-heading">
          <strong><IconifyIconOnline icon="ri:video-line" /> 内容数据</strong>
          <span>{{ activeAccount?.platform || "全部平台" }} · 近期作品</span>
        </div>
        <div class="content-summary">
          <article>
            <span class="summary-icon is-blue"
              ><IconifyIconOnline icon="ri:eye-line"
            /></span>
            <span>总曝光量</span>
            <strong>{{ compactCount(contentTotals.views) }}</strong>
          </article>
          <article>
            <span class="summary-icon is-orange"
              ><IconifyIconOnline icon="ri:thumb-up-line"
            /></span>
            <span>总点赞量</span>
            <strong>{{ compactCount(contentTotals.likes) }}</strong>
          </article>
          <article>
            <span class="summary-icon is-green"
              ><IconifyIconOnline icon="ri:chat-3-line"
            /></span>
            <span>总评论量</span>
            <strong>{{ compactCount(contentTotals.comments) }}</strong>
          </article>
          <article>
            <span class="summary-icon is-purple"
              ><IconifyIconOnline icon="ri:share-forward-line"
            /></span>
            <span>总分享量</span>
            <strong>{{ compactCount(contentTotals.shares) }}</strong>
          </article>
          <article>
            <span class="summary-icon is-red"
              ><IconifyIconOnline icon="ri:bookmark-line"
            /></span>
            <span>总收藏量</span>
            <strong>{{ compactCount(contentTotals.saves) }}</strong>
          </article>
        </div>

        <div v-if="visiblePosts.length" class="content-gallery">
          <button
            v-for="post in visiblePosts"
            :key="post.id"
            type="button"
            class="content-card"
            :title="post.title || '打开作品'"
            @click="openUrl(post.postUrl)"
          >
            <span class="content-cover">
              <img
                v-if="post.coverUrl"
                :src="post.coverUrl"
                :alt="post.title || '作品封面'"
              />
              <span v-else class="cover-placeholder"
                ><PlatformIconBadge :platform="post.platform" /> 暂无封面</span
              >
              <i class="cover-date">{{ publishedDate(post.publishedAt) }}</i>
              <i class="cover-duration">{{
                durationText(post.durationSeconds)
              }}</i>
            </span>
            <span class="content-metrics">
              <span
                ><IconifyIconOnline icon="ri:eye-line" />
                {{ compactCount(post.viewCount) }}</span
              >
              <span
                ><IconifyIconOnline icon="ri:thumb-up-line" />
                {{ compactCount(post.likeCount) }}</span
              >
              <span
                ><IconifyIconOnline icon="ri:chat-3-line" />
                {{ compactCount(post.commentCount) }}</span
              >
              <span
                ><IconifyIconOnline icon="ri:share-forward-line" />
                {{ compactCount(post.shareCount) }}</span
              >
              <span
                ><IconifyIconOnline icon="ri:bookmark-line" />
                {{ compactCount(post.saveCount) }}</span
              >
            </span>
          </button>
        </div>
        <el-empty v-else :image-size="54" description="该平台暂无已同步作品" />
      </section>

      <section v-if="resourceKind === '达人'" class="profile-section">
        <div class="section-heading">
          <strong
            ><IconifyIconOnline icon="ri:bubble-chart-line" /> 内容分析</strong
          >
          <span>内容主题标签按作品占比 TOP5 展示</span>
        </div>
        <div v-if="topicTags.length" class="analysis-grid">
          <article class="word-cloud-card">
            <span class="analysis-title">词云</span>
            <div class="word-cloud">
              <span
                v-for="(tag, index) in topicTags"
                :key="`cloud-${tag.name}`"
                :class="`tone-${(index % 5) + 1}`"
                :style="{ fontSize: `${tag.scale}rem` }"
                >#{{ tag.name }}</span
              >
            </div>
          </article>
          <article class="topic-card">
            <span class="analysis-title">内容主题标签 · TOP5</span>
            <div class="topic-list">
              <div v-for="(tag, index) in topicTags" :key="tag.name">
                <span class="topic-rank">{{ index + 1 }}</span>
                <strong>#{{ tag.name }}</strong>
                <span class="topic-track"
                  ><i :style="{ width: `${Math.max(tag.share, 5)}%` }"
                /></span>
                <em>{{ tag.share.toFixed(1) }}%</em>
              </div>
            </div>
          </article>
        </div>
        <el-empty v-else :image-size="54" description="暂无可分析的内容标签" />
      </section>

      <section class="profile-section">
        <div class="section-heading">
          <strong
            ><IconifyIconOnline icon="ri:shake-hands-line" />
            {{ resourceKind }}合作表现</strong
          >
        </div>
        <div class="cooperation-grid">
          <article class="metric-card">
            <span class="metric-label">合作费用区间（USD）</span>
            <strong>{{ costRange() }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">合作内容总曝光量</span>
            <strong>{{ compactCount(cooperationStats.totalReach) }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">合作内容总互动量</span>
            <strong>{{
              compactCount(cooperationStats.totalEngagements)
            }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">合作次数</span>
            <strong>{{ cooperationStats.count }}次</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">合作平均曝光量</span>
            <strong>{{ compactCount(averageReach) }}</strong>
            <div class="metric-footer">
              <el-tooltip
                content="历次合作作品曝光量标准差 ÷ 历次合作作品曝光量平均值 × 100%"
                placement="top"
              >
                <span class="volatility"
                  >波动指数 {{ volatilityText(cooperationVolatility()) }}
                  <IconifyIconOnline icon="ri:information-line"
                /></span>
              </el-tooltip>
              <span class="stability">{{
                stabilityText(cooperationVolatility())
              }}</span>
            </div>
          </article>
          <article class="metric-card">
            <span class="metric-label">合作平均互动率</span>
            <strong>{{ engagementRate }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">合作平均CPM</span>
            <strong>{{ averageCpm() }}</strong>
          </article>
          <article class="metric-card note-card">
            <span class="metric-label">{{ resourceKind }}备注</span>
            <strong>{{ notesText() }}</strong>
          </article>
        </div>
      </section>

      <section class="profile-section detail-section">
        <div class="section-heading">
          <strong
            ><IconifyIconOnline icon="ri:table-line" />
            {{ resourceKind }}合作明细</strong
          >
        </div>
        <el-table :data="pagedCooperations" border class="cooperation-table">
          <el-table-column
            prop="projectName"
            label="项目名称"
            min-width="140"
          />
          <el-table-column label="合作形式" width="94">
            <template #default="{ row }">
              <CooperationTypeTags
                :value="row.cooperationType"
                empty-text="-"
              />
            </template>
          </el-table-column>
          <el-table-column label="发布日期" width="100">
            <template #default="{ row }">{{ cooperationDate(row) }}</template>
          </el-table-column>
          <el-table-column label="发布作品" width="116">
            <template #default="{ row }">
              <button
                v-if="cooperationLink(row)"
                type="button"
                class="work-thumbnail"
                @click="openUrl(cooperationLink(row))"
              >
                <img
                  v-if="cooperationCover(row)"
                  :src="cooperationCover(row)"
                  :alt="row.creativeName || row.projectName"
                />
                <span v-else class="work-placeholder"
                  ><IconifyIconOnline icon="ri:play-circle-line"
                /></span>
                <i><IconifyIconOnline icon="ri:play-fill" /></i>
              </button>
              <span v-else>/</span>
            </template>
          </el-table-column>
          <el-table-column label="曝光量" width="88">
            <template #default="{ row }">{{
              formatCount(primaryReach(row))
            }}</template>
          </el-table-column>
          <el-table-column label="互动量" width="88">
            <template #default="{ row }">{{
              formatCount(row.engagementCount)
            }}</template>
          </el-table-column>
          <el-table-column prop="owner" label="对接人" width="88" />
          <el-table-column prop="vendor" label="合作供应商" min-width="120" />
        </el-table>
        <div v-if="selectedCooperations.length" class="detail-pagination">
          <el-pagination
            v-model:current-page="cooperationPage"
            :page-size="cooperationPageSize"
            :page-sizes="[10, 20, 50]"
            :total="selectedCooperations.length"
            layout="total, sizes, prev, pager, next, jumper"
            background
            @size-change="handleCooperationPageSize"
          />
        </div>
      </section>
    </template>

    <el-empty v-else-if="!loading" description="未找到资源档案">
      <el-button type="primary" @click="router.push('/business/resources')"
        >返回全球资源库</el-button
      >
    </el-empty>
  </main>
</template>

<style scoped>
.creator-profile-page {
  display: grid;
  gap: 10px;
  min-height: calc(100vh - 126px);
  padding: 10px;
  color: #13213c;
  background: #f5f8fd;
}

.profile-hero,
.profile-section {
  min-width: 0;
  background: #fff;
  border: 1px solid #e7edf6;
  border-radius: 9px;
}

.profile-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 88px;
  padding: 10px 18px;
}

.profile-person,
.profile-name-line,
.profile-meta-line,
.hero-actions,
.section-heading,
.section-heading-side,
.platform-tabs,
.metric-footer {
  display: flex;
  align-items: center;
}

.profile-person {
  gap: 14px;
  min-width: 0;
}

.profile-avatar {
  position: relative;
  display: grid;
  flex: 0 0 62px;
  width: 62px;
  height: 62px;
  overflow: hidden;
  color: #64748b;
  place-items: center;
  background: #eef3f9;
  border: 2px solid #edf2f8;
  border-radius: 50%;
}

.profile-avatar img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.profile-identity {
  min-width: 0;
}

.profile-name-line {
  gap: 9px;
}

.profile-name-line h1 {
  margin: 0;
  font-size: 21px;
  line-height: 1.2;
  color: #12213d;
}

.profile-name-line > span {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
  color: #7786a3;
  white-space: nowrap;
}

.profile-meta-line {
  gap: 7px;
  min-height: 25px;
  margin-top: 4px;
  font-size: 12px;
  color: #60708f;
}

.profile-meta-line i {
  color: #a7b3c8;
  font-style: normal;
}

.platform-link {
  display: inline-flex;
  padding: 0;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.platform-link:disabled {
  cursor: default;
  opacity: 0.72;
}

.hero-actions {
  gap: 8px;
}

.tier-badge {
  display: inline-flex;
  gap: 5px;
  align-items: center;
  height: 30px;
  padding: 0 11px;
  font-size: 12px;
  font-weight: 700;
  color: #2878ef;
  background: #eaf3ff;
  border-radius: 7px;
}

.profile-section {
  padding: 11px 14px 13px;
}

.section-heading {
  justify-content: space-between;
  min-height: 24px;
  margin-bottom: 8px;
}

.section-heading strong {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  font-size: 15px;
  color: #14233e;
}

.section-heading strong svg {
  color: #267bf2;
}

.section-heading > span,
.refresh-note {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  font-size: 11px;
  color: #8290aa;
}

.section-heading-side {
  gap: 12px;
}

.platform-tabs {
  gap: 5px;
}

.platform-tab {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  justify-content: center;
  min-width: 90px;
  height: 30px;
  padding: 0 10px;
  font-size: 11px;
  color: #5c6b87;
  cursor: pointer;
  background: #f5f7fb;
  border: 1px solid transparent;
  border-radius: 7px;
}

.platform-tab:hover,
.platform-tab.active {
  color: #1769e8;
  background: #f8fbff;
  border-color: #3b82f6;
}

.basic-grid,
.cooperation-grid,
.content-summary,
.content-gallery,
.analysis-grid {
  display: grid;
  gap: 8px;
}

.basic-grid {
  grid-template-columns: repeat(6, minmax(0, 1fr));
}

.cooperation-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.content-summary {
  grid-template-columns: repeat(5, minmax(0, 1fr));
  padding: 8px 10px;
  margin-bottom: 10px;
  background: #f8faff;
  border: 1px solid #e8eef8;
  border-radius: 8px;
}

.content-summary article {
  display: grid;
  grid-template-columns: 30px 1fr;
  grid-template-rows: auto auto;
  column-gap: 8px;
  align-items: center;
  min-width: 0;
  padding: 4px 10px;
  border-right: 1px solid #e5ebf5;
}

.content-summary article:last-child {
  border-right: 0;
}

.summary-icon {
  display: inline-flex;
  grid-row: 1 / 3;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  font-size: 15px;
  border-radius: 8px;
}

.summary-icon.is-blue {
  color: #2878ef;
  background: #eaf3ff;
}

.summary-icon.is-orange {
  color: #e89418;
  background: #fff4dd;
}

.summary-icon.is-green {
  color: #18a66b;
  background: #e6f8f1;
}

.summary-icon.is-purple {
  color: #8b5cf6;
  background: #f1ebff;
}

.summary-icon.is-red {
  color: #ef5261;
  background: #ffeaed;
}

.content-summary article > span:not(.summary-icon) {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 10px;
  color: #7887a1;
  white-space: nowrap;
}

.content-summary strong {
  font-size: 15px;
  color: #172640;
}

.content-gallery {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.content-card {
  display: flex;
  min-width: 0;
  padding: 0;
  overflow: hidden;
  text-align: left;
  cursor: pointer;
  background: #fff;
  border: 1px solid #e3eaf4;
  border-radius: 8px;
  flex-direction: column;
  transition:
    border-color 0.16s ease,
    box-shadow 0.16s ease;
}

.content-card:hover {
  border-color: #9cc3fb;
  box-shadow: 0 6px 16px rgb(37 99 235 / 10%);
}

.content-cover {
  position: relative;
  display: flex;
  width: 100%;
  aspect-ratio: 4 / 3;
  overflow: hidden;
  color: #7b8ba5;
  align-items: center;
  justify-content: center;
  background: #eef3f9;
}

.content-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cover-placeholder {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  font-size: 11px;
}

.cover-date,
.cover-duration {
  position: absolute;
  bottom: 5px;
  padding: 2px 5px;
  font-size: 9px;
  font-style: normal;
  color: #fff;
  background: rgb(15 23 42 / 70%);
  border-radius: 4px;
}

.cover-date {
  left: 5px;
}

.cover-duration {
  right: 5px;
}

.content-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 5px 7px;
  width: 100%;
  padding: 8px;
}

.content-metrics > span {
  display: inline-flex;
  gap: 3px;
  align-items: center;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 9px;
  color: #61708b;
  white-space: nowrap;
}

.content-metrics svg {
  flex: 0 0 auto;
  color: #8795ad;
}

.analysis-grid {
  grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
}

.word-cloud-card,
.topic-card {
  min-width: 0;
  min-height: 148px;
  padding: 12px 14px;
  background: #fbfcff;
  border: 1px solid #e5ebf5;
  border-radius: 8px;
}

.analysis-title {
  font-size: 11px;
  font-weight: 700;
  color: #65748f;
}

.word-cloud {
  display: flex;
  gap: 14px 20px;
  align-content: center;
  align-items: center;
  justify-content: center;
  min-height: 105px;
  padding: 8px;
  flex-wrap: wrap;
}

.word-cloud span {
  font-weight: 700;
  line-height: 1;
}

.tone-1 {
  color: #2878ef;
}

.tone-2 {
  color: #8b5cf6;
}

.tone-3 {
  color: #16a568;
}

.tone-4 {
  color: #e89418;
}

.tone-5 {
  color: #ef5261;
}

.topic-list {
  display: grid;
  gap: 7px;
  margin-top: 10px;
}

.topic-list > div {
  display: grid;
  grid-template-columns: 19px minmax(72px, 0.5fr) minmax(100px, 1fr) 42px;
  gap: 8px;
  align-items: center;
  min-width: 0;
  font-size: 10px;
}

.topic-rank {
  display: grid;
  width: 18px;
  height: 18px;
  font-weight: 700;
  color: #2878ef;
  place-items: center;
  background: #eaf3ff;
  border-radius: 5px;
}

.topic-list strong {
  overflow: hidden;
  text-overflow: ellipsis;
  color: #263550;
  white-space: nowrap;
}

.topic-track {
  height: 6px;
  overflow: hidden;
  background: #e8edf5;
  border-radius: 999px;
}

.topic-track i {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, #2778ef, #79b4ff);
  border-radius: inherit;
}

.topic-list em {
  font-style: normal;
  color: #71809a;
  text-align: right;
}

.metric-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
  min-height: 84px;
  padding: 10px 12px;
  background: #fff;
  border: 1px solid #e4ebf5;
  border-radius: 8px;
  box-shadow: 0 2px 7px rgb(30 64 175 / 3%);
}

.metric-icon {
  position: absolute;
  top: 9px;
  right: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  font-size: 15px;
  color: #2f7df3;
  background: #edf5ff;
  border-radius: 50%;
}

.metric-label {
  padding-right: 24px;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 11px;
  color: #687895;
  white-space: nowrap;
}

.metric-card strong {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 18px;
  line-height: 1.2;
  color: #13213c;
  white-space: nowrap;
}

.metric-footer {
  gap: 9px;
  min-height: 17px;
  margin-top: auto;
}

.delta,
.volatility,
.contact-hint {
  display: inline-flex;
  gap: 2px;
  align-items: center;
  font-size: 10px;
}

.delta {
  font-weight: 700;
}

.delta.is-up {
  color: #16a568;
}

.delta.is-down {
  color: #ef4458;
}

.delta.is-empty {
  color: #a1aec1;
}

.volatility,
.contact-hint {
  color: #7786a0;
}

.contact-value,
.note-card strong {
  padding-right: 24px;
  font-size: 13px !important;
}

.note-card strong {
  white-space: normal;
}

.stability {
  padding: 2px 7px;
  margin-left: auto;
  font-size: 10px;
  font-weight: 700;
  color: #159760;
  background: #eaf9f1;
  border-radius: 999px;
}

:deep(.cooperation-table) {
  overflow: hidden;
  font-size: 12px;
  border-radius: 7px;
}

:deep(.cooperation-table th.el-table__cell) {
  height: 34px;
  padding: 4px 0;
  color: #60708d;
  background: #f7f9fc;
}

:deep(.cooperation-table td.el-table__cell) {
  height: 52px;
  padding: 4px 0;
  color: #2e3d58;
}

.detail-pagination {
  display: flex;
  justify-content: flex-end;
  padding-top: 10px;
}

:deep(.detail-pagination .el-pagination) {
  --el-pagination-button-height: 28px;
  --el-pagination-button-width: 28px;

  font-size: 11px;
}

.work-thumbnail {
  position: relative;
  display: block;
  width: 96px;
  height: 40px;
  padding: 0;
  overflow: hidden;
  cursor: pointer;
  background: #eaf0f8;
  border: 0;
  border-radius: 5px;
}

.work-thumbnail img,
.work-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.work-placeholder {
  font-size: 20px;
  color: #7890b4;
}

.work-thumbnail i {
  position: absolute;
  top: 50%;
  left: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  color: #fff;
  background: rgb(15 23 42 / 68%);
  border-radius: 50%;
  transform: translate(-50%, -50%);
}

@media (width <= 980px) {
  .creator-profile-page {
    overflow-x: auto;
  }

  .profile-hero,
  .profile-section {
    min-width: 900px;
  }
}
</style>
