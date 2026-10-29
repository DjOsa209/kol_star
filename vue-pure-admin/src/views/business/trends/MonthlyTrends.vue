<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  getMonthlyTrends,
  refreshMonthlyTrends,
  type MonthlyTrend,
  type MonthlyTrendResult
} from "@/api/trends";
import TrendImage from "./TrendImage.vue";
import PlayIcon from "~icons/ri/play-circle-line";
import MusicIcon from "~icons/ri/music-2-line";
import RefreshIcon from "~icons/ri/refresh-line";
import VideoIcon from "~icons/ri/video-line";
const route = useRoute(),
  router = useRouter();
const { locale } = useI18n();
const text = (zh: string, en: string) => (locale.value === "en" ? en : zh);
const data = ref<MonthlyTrendResult>();
const loading = ref(false),
  updating = ref(false),
  error = ref("");
const updateMessage = ref("");
const selected = ref<MonthlyTrend>();
const visible = ref(false);
let generation = 0;
const month = computed(() => String(route.query.month || ""));
const summaries: Record<string, string> = {
  "My reaction": "表情代替评分，让反应成为内容。",
  "Getting robbed": "专注看手机，忽略身后发生的反转。",
  "October is all about me": "用简单画面，记录属于自己的十月。",
  "Text message from -A": "用神秘短信制造悬念，让观众参与猜测。",
  "I just want to be OK": "用轻松画面，对照内心的小焦虑。",
  "Upside down": "跟随音乐，用转场展示前后变化。"
};
const summary = (row: MonthlyTrend) =>
  locale.value === "en" ? row.summary : summaries[row.title] || row.summary;
function audioLink(value: string) {
  try {
    const url = new URL(value);
    if (
      url.protocol === "https:" &&
      !url.username &&
      !url.password &&
      !url.port &&
      !url.hash &&
      ["www.tiktok.com", "vm.tiktok.com"].includes(url.hostname)
    )
      return url.href;
  } catch {
    // Invalid source references must not become clickable links.
  }
  return "";
}
function monthLabel(value: string) {
  const [year, number] = value.split("-");
  return text(`${year}年${Number(number)}月`, value);
}
function dateLabel(row: MonthlyTrend) {
  return row.publishedAt
    ? text(
        `${Number(row.publishedAt.slice(5, 7))}月${Number(row.publishedAt.slice(8, 10))}日`,
        row.publishedAt
      )
    : row.sourceDate || "—";
}
async function load() {
  const current = ++generation;
  loading.value = true;
  error.value = "";
  data.value = undefined;
  visible.value = false;
  try {
    const result = await getMonthlyTrends(month.value);
    if (current !== generation) return;
    if (result.code !== 0) throw new Error("Monthly data unavailable");
    data.value = result.data;
  } catch {
    if (current === generation)
      error.value = text(
        "月度内容暂时无法读取，请重试。",
        "Monthly trends are unavailable. Please retry."
      );
  } finally {
    if (current === generation) loading.value = false;
  }
}
function selectMonth(value: string) {
  updateMessage.value = "";
  router.replace({ path: route.path, query: { tab: "monthly", month: value } });
}
async function refresh() {
  if (updating.value) return;
  updating.value = true;
  error.value = "";
  try {
    updateMessage.value = "";
    const result = await refreshMonthlyTrends(data.value?.month || month.value);
    if (result.code !== 0) throw new Error("Monthly refresh failed");
    await load();
    const covers = result.data.covers;
    updateMessage.value = text(
      `内容已更新 · 封面新增 ${covers.added} 张，已就绪 ${covers.ready} 张${covers.pending ? `，${covers.pending} 张待重试` : ""}`,
      `Content updated · ${covers.added} covers added, ${covers.ready} ready${covers.pending ? `, ${covers.pending} pending retry` : ""}`
    );
  } catch {
    error.value = text(
      "更新失败，已保留上次采集内容。请稍后重试。",
      "Update failed. The saved collection is preserved. Try again later."
    );
  } finally {
    updating.value = false;
  }
}
function preview(row: MonthlyTrend) {
  selected.value = row;
  visible.value = true;
}
const playerURL = computed(() => {
  const id = selected.value?.videoID || "";
  return /^[0-9]{10,24}$/.test(id)
    ? `https://www.tiktok.com/player/v1/${id}?music_info=1&description=0&autoplay=1`
    : "";
});
watch(month, load);
onMounted(load);
onBeforeUnmount(() => {
  generation++;
});
</script>
<template>
  <section
    class="monthly-trends"
    :aria-label="text('每月趋势', 'Monthly trends')"
  >
    <div class="monthly-toolbar">
      <el-select
        :model-value="data?.month || month"
        :aria-label="text('选择月份', 'Select month')"
        :placeholder="text('选择月份', 'Select month')"
        :disabled="loading || updating"
        @change="selectMonth"
        ><el-option
          v-for="option in data?.months || []"
          :key="option.month"
          :label="monthLabel(option.month)"
          :value="option.month"
      /></el-select>
      <strong v-if="data" class="monthly-count"
        >{{ data.total }} {{ text("条趋势", "trends") }}</strong
      >
      <p v-if="data" class="monthly-source" role="status">
        {{ text("来源 Later 编辑精选", "Curated by Later") }} ·
        {{ text("原文更新", "Article updated") }} {{ data.sourceUpdatedAt }}
      </p>
      <el-button
        type="primary"
        class="monthly-refresh"
        :loading="updating"
        @click="refresh"
        ><RefreshIcon />{{ text("更新内容", "Update content") }}</el-button
      >
    </div>
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      show-icon
      class="monthly-error"
    />
    <el-button v-if="error && !data" class="monthly-retry" @click="load">{{
      text("重试读取", "Retry")
    }}</el-button>
    <p v-if="updateMessage" class="monthly-update-message" role="status">
      {{ updateMessage }}
    </p>
    <div v-loading="loading" :aria-busy="loading" class="monthly-content">
      <div v-if="data?.list.length" class="monthly-grid">
        <article v-for="row in data.list" :key="row.id" class="monthly-card">
          <button
            class="monthly-cover"
            :disabled="!row.videoID"
            :aria-label="text('预览趋势视频：', 'Preview trend: ') + row.title"
            @click="preview(row)"
          >
            <TrendImage
              v-if="row.coverURL"
              :src="row.coverURL"
              :alt="row.title"
            />
            <span v-else class="monthly-cover-empty"
              ><VideoIcon /><span>{{
                text("封面待同步", "Cover pending")
              }}</span></span
            >
            <PlayIcon v-if="row.videoID" class="monthly-play" />
            <span class="monthly-video-label">{{
              row.videoID
                ? text("示例视频", "Example video")
                : text("视频链接待解析", "Video link pending")
            }}</span>
          </button>
          <h2>{{ row.title }}</h2>
          <div class="monthly-date">
            <time :datetime="row.publishedAt || undefined">{{
              dateLabel(row)
            }}</time
            ><el-tooltip
              v-if="row.dateConflict"
              :content="
                text(
                  '原文日期与月度分组不一致，保留原始日期。',
                  'The original date differs from its month group.'
                )
              "
              ><span class="date-warning">{{
                text("原文日期待核对", "Check source date")
              }}</span></el-tooltip
            >
          </div>
          <p class="monthly-summary" :title="summary(row)">
            {{ summary(row) }}
          </p>
          <div class="monthly-audio">
            <span class="monthly-music"><MusicIcon /></span>
            <div class="monthly-audio-label">
              <strong :title="row.audioLabel">{{
                row.audioLabel || text("原声音频", "Original audio")
              }}</strong
              ><span>{{
                audioLink(row.audioURL)
                  ? text("在 TikTok 查看原声", "View original audio on TikTok")
                  : text("原文未提供音频链接", "No audio link provided")
              }}</span>
            </div>
            <el-button
              v-if="audioLink(row.audioURL)"
              tag="a"
              :href="audioLink(row.audioURL)"
              target="_blank"
              rel="noopener noreferrer"
              :aria-label="
                text(
                  '在新标签页打开原声：',
                  'Open original audio in a new tab: '
                ) + (row.audioLabel || row.title)
              "
              >{{ text("打开原声", "Open audio") }}</el-button
            >
          </div>
        </article>
      </div>
      <el-empty
        v-else-if="!loading && !error"
        :description="
          text('本月暂无已采集的趋势', 'No collected trends for this month')
        "
      />
    </div>
    <p v-if="data" class="monthly-footnote">
      {{
        text(
          "本页为 Later 编辑精选，非全站热度榜。视频按源站可用状态加载。",
          "Editorial picks from Later, not a platform-wide ranking. Video availability depends on TikTok."
        )
      }}<span
        >{{ text("采集于", "Collected") }}
        {{ data.capturedAt.slice(0, 10) }}</span
      >
    </p>
    <el-dialog
      v-model="visible"
      :title="selected?.title"
      width="min(440px, 94vw)"
      destroy-on-close
      @closed="selected = undefined"
    >
      <iframe
        v-if="visible && playerURL"
        :key="playerURL"
        :src="playerURL"
        :title="selected?.title"
        class="monthly-player"
        sandbox="allow-scripts allow-same-origin allow-presentation"
        allow="autoplay; encrypted-media; fullscreen; picture-in-picture"
        referrerpolicy="no-referrer"
        allowfullscreen
      />
      <p class="monthly-player-note">
        {{
          text(
            "由 TikTok 播放器提供，源站不可用时可能无法播放。",
            "Provided by TikTok. Playback may be unavailable when the source is restricted."
          )
        }}
      </p>
    </el-dialog>
  </section>
</template>
<style scoped lang="scss">
.monthly-trends {
  padding-top: 0;
  font-variant-numeric: tabular-nums;
}
.monthly-toolbar {
  display: flex;
  align-items: center;
  gap: 22px;
  margin: 0 0 22px;
}
.monthly-toolbar > .el-select {
  flex: 0 0 220px;
  width: 220px;
}
.monthly-toolbar :deep(.el-select__wrapper) {
  min-height: 38px;
}
.monthly-count {
  font-size: 15px;
  font-weight: 650;
  white-space: nowrap;
}
.monthly-source {
  min-width: 0;
  padding-left: 22px;
  margin: 0;
  border-left: 1px solid var(--el-border-color);
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}
.monthly-refresh {
  flex-shrink: 0;
  margin-left: auto;
  min-height: 38px;
  min-width: 140px;
  color: #fff !important;
  background: #66752a !important;
  border-color: #66752a !important;
}
.monthly-refresh:hover {
  background: #526121 !important;
  border-color: #526121 !important;
}
.monthly-refresh > :deep(span) {
  gap: 8px;
}
.monthly-refresh svg {
  width: 16px;
  height: 16px;
}
.monthly-content {
  min-height: 280px;
}
.monthly-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 24px;
}
.monthly-card {
  box-shadow: none;
  border: 0;
  background: transparent;
  min-width: 0;
}
.monthly-cover {
  position: relative;
  display: block;
  width: 100%;
  aspect-ratio: 1.88;
  padding: 0;
  overflow: hidden;
  border: 0;
  border-radius: 7px;
  background: var(--el-fill-color-light);
  cursor: pointer;
}
.monthly-cover:disabled {
  cursor: default;
}
.monthly-cover-empty {
  display: flex;
  flex-direction: column;
  height: 100%;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.monthly-cover-empty svg {
  width: 28px;
  height: 28px;
}
.monthly-play {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 42px;
  height: 42px;
  transform: translate(-50%, -50%);
  color: #fff;
  filter: drop-shadow(0 1px 4px rgb(0 0 0 / 35%));
}
.monthly-video-label {
  position: absolute;
  left: 8px;
  bottom: 8px;
  padding: 3px 6px;
  border-radius: 4px;
  color: #fff;
  background: rgb(0 0 0 / 55%);
  font-size: 11px;
  line-height: 1.5;
}
.monthly-card h2 {
  margin: 11px 0 4px;
  font-size: 18px;
  line-height: 1.4;
  font-weight: 700;
  letter-spacing: -0.02em;
  overflow-wrap: anywhere;
}
.monthly-date {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.date-warning {
  color: var(--el-color-warning-dark-2);
  font-size: 12px;
}
.monthly-summary {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
  height: 22px;
  overflow: hidden;
  margin: 6px 0 10px;
  color: var(--el-text-color-regular);
  font-size: 14px;
  line-height: 22px;
}
.monthly-update-message {
  margin: 0 0 16px;
  color: var(--el-color-primary);
  font-size: 13px;
}
.monthly-audio {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.monthly-music {
  display: flex;
  flex: 0 0 38px;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 50%;
}
.monthly-music svg {
  width: 20px;
  height: 20px;
}
.monthly-audio-label {
  flex: 1;
  min-width: 0;
}
.monthly-audio-label strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 500;
}
.monthly-audio-label > span {
  display: block;
  margin-top: 2px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.monthly-audio > .el-button {
  min-width: 88px;
  margin: 0;
}
.monthly-footnote {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
  padding: 12px 0 4px;
  margin: 20px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.7;
}
.monthly-error {
  margin-bottom: 16px;
}
.monthly-retry {
  margin-bottom: 12px;
}
.monthly-player {
  display: block;
  width: 100%;
  height: min(65vh, 640px);
  border: 0;
  border-radius: 6px;
  background: #101014;
}
.monthly-player-note {
  margin: 12px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
}
button:focus-visible,
a:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 3px;
}
@media (width<=1180px) {
  .monthly-toolbar {
    flex-wrap: wrap;
    gap: 14px;
  }
  .monthly-source {
    order: 4;
    width: 100%;
    padding: 0;
    border: 0;
  }
  .monthly-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (width<=760px) {
  .monthly-toolbar > .el-select {
    flex: 1 1 160px;
    width: auto;
  }
  .monthly-count {
    font-size: 13px;
  }
  .monthly-refresh {
    margin: 0;
  }
  .monthly-grid {
    grid-template-columns: 1fr;
    gap: 28px;
  }
  .monthly-summary {
    height: auto;
    min-height: 22px;
  }
}
</style>
