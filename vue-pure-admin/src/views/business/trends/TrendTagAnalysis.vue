<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { TrendTagDetail, TrendRelatedVideo } from "@/api/trends";
import TrendChart from "./TrendChart.vue";
import TrendImage from "./TrendImage.vue";
import PlayIcon from "~icons/ri/play-circle-line";

defineProps<{ detail: TrendTagDetail }>();
const emit = defineEmits<{ preview: [video: TrendRelatedVideo] }>();
const { locale } = useI18n();
const text = (zh: string, en: string) => (locale.value === "en" ? en : zh);
function count(value: string) {
  const n = Number(value);
  return value !== "" && Number.isFinite(n)
    ? Intl.NumberFormat(locale.value === "en" ? "en" : "zh-CN", {
        notation: "compact",
        maximumFractionDigits: 1
      }).format(n)
    : "—";
}
function regionName(code: string) {
  return (
    new Intl.DisplayNames([locale.value === "en" ? "en" : "zh-CN"], {
      type: "region"
    }).of(code) || code
  );
}
function ageName(value: string) {
  return { "3": "18–24", "4": "25–34", "5": "≥35" }[value] || value;
}
function validPercent(value: unknown) {
  const n = Number(value);
  return Number.isFinite(n) ? Math.max(0, Math.min(100, n)) : 0;
}
const previewRelated = (video: TrendRelatedVideo) => emit("preview", video);
</script>

<template>
  <div class="tag-analysis">
    <div class="detail-summary">
      <div>
        <span>{{ text("发帖量", "Posts") }}</span
        ><strong>{{ count(detail.publishCnt) }}</strong>
      </div>
      <div>
        <span>{{ text("播放量", "Views") }}</span
        ><strong>{{ count(detail.vv) }}</strong>
      </div>
    </div>
    <section class="detail-section">
      <h2>{{ text("热度趋势", "Interest over time") }}</h2>
      <p class="detail-note">
        {{
          text(
            "热度为原站相对指数；日期以曲线实际返回时间为准。",
            "Interest is TikTok’s relative index. Dates reflect the returned data."
          )
        }}
      </p>
      <TrendChart
        v-if="detail.popularityCurve?.length"
        :points="detail.popularityCurve"
        :label="text('热度趋势', 'Interest over time')"
      /><el-empty v-else :description="text('暂无趋势数据', 'No trend data')" />
    </section>
    <div class="audience-grid">
      <section class="detail-section">
        <h2>{{ text("年龄分布", "Age range") }}</h2>
        <div
          v-for="age in detail.ageProfile || []"
          :key="age.ageLevel"
          class="age-row"
        >
          <div>
            <span>{{ ageName(age.ageLevel) }}</span
            ><strong>{{ age.vvPercent }}%</strong>
          </div>
          <el-progress
            :percentage="validPercent(age.vvPercent)"
            :show-text="false"
            color="#527f70"
          />
        </div>
      </section>
      <section class="detail-section">
        <h2>{{ text("热门地区", "Top regions") }}</h2>
        <div
          v-for="(region, i) in detail.representativeCountryProfile || []"
          :key="region.countryCode"
          class="region-row"
        >
          <span>{{ i + 1 }}</span
          ><span>{{ regionName(region.countryCode) }}</span
          ><strong>{{ region.countryTgiScore }}</strong>
        </div>
        <p class="detail-note">
          {{
            text(
              "地区热度为 TGI 指数，不是受众百分比。",
              "Regional popularity is a TGI index, not an audience percentage."
            )
          }}
        </p>
      </section>
    </div>
    <section class="detail-section">
      <h2>
        {{ text("关联视频", "Related videos") }}
        <small>{{ detail.videoList?.length || 0 }}</small>
      </h2>

      <div class="related-grid">
        <button
          v-for="video in detail.videoList || []"
          :key="video.itemID"
          :aria-label="text('预览视频', 'Preview video')"
          @click="previewRelated(video)"
        >
          <TrendImage
            :src="video.coverURL"
            :alt="text('关联视频封面', 'Related video cover')"
          /><span
            ><PlayIcon class="trend-ui-icon" />
            {{ text("预览", "Preview") }}</span
          >
        </button>
      </div>
      <p class="detail-note">
        {{
          text(
            "原站返回的热门视频样本，不代表该标签下的全部视频。",
            "Popular video samples returned by TikTok, not every video using this hashtag."
          )
        }}
      </p>
    </section>
  </div>
</template>

<style scoped lang="scss">
.tag-analysis {
  color: var(--el-text-color-primary);
  font-variant-numeric: tabular-nums;
}
.detail-summary {
  display: flex;
  gap: 32px;
  padding: 0 0 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.detail-summary > div {
  display: grid;
  gap: 5px;
  min-width: 112px;
}
.detail-summary > div + div {
  padding-left: 28px;
  border-left: 1px solid var(--el-border-color-lighter);
}
.detail-summary span {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.detail-summary strong {
  font-size: 28px;
  font-weight: 650;
  line-height: 1.3;
  letter-spacing: -0.03em;
}
.detail-section {
  margin: 18px 0 0;
  min-width: 0;
}
.detail-section h2 {
  margin: 0 0 10px;
  font-size: 16px;
  font-weight: 650;
}
.detail-section small {
  margin-left: 6px;
  font-size: 12px;
  font-weight: 400;
  color: var(--el-text-color-secondary);
}
.detail-note {
  margin: 0 0 10px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
}
.audience-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 24px;
}
.audience-grid > section + section {
  padding-left: 20px;
  border-left: 1px solid var(--el-border-color-lighter);
}
.age-row {
  margin: 10px 0;
  font-size: 13px;
}
.age-row > div:first-child {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 7px;
}
.age-row strong,
.region-row strong {
  font-weight: 500;
}
.age-row :deep(.el-progress-bar__outer) {
  height: 5px !important;
  background: var(--el-fill-color);
}
.region-row {
  display: grid;
  grid-template-columns: 14px minmax(0, 1fr) auto;
  gap: 8px;
  margin: 5px 0;
  font-size: 13px;
}
.region-row > span:first-child {
  color: var(--el-text-color-secondary);
}
.region-row + .detail-note {
  margin: 8px 0 0;
}
.related-grid {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: calc((100% - 24px) / 3);
  gap: 12px;
  overflow-x: auto;
  padding: 2px 2px 10px;
  scrollbar-width: thin;
}
.related-grid button {
  position: relative;
  display: block;
  aspect-ratio: 4 / 3;
  overflow: hidden;
  padding: 0;
  cursor: pointer;
  border: 0;
  border-radius: 6px;
  background: var(--el-fill-color-light);
}
.related-grid button > span:last-child {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  color: white;
  background: rgb(0 0 0 / 15%);
}
.related-grid :deep(.trend-ui-icon) {
  width: 24px;
  height: 24px;
}
.related-grid button:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}
@media (width <= 760px) {
  .audience-grid {
    grid-template-columns: 1fr;
    gap: 0;
  }
  .audience-grid > section + section {
    padding-left: 0;
    border: 0;
  }
  .detail-summary {
    gap: 20px;
  }
  .detail-summary > div {
    min-width: 0;
  }
  .detail-summary > div + div {
    padding-left: 20px;
  }
}
</style>
