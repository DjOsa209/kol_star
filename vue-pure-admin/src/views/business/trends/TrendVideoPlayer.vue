<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import PlayIcon from "~icons/ri/play-circle-line";
import { safeTrendVideoURL } from "@/api/trends";
import TrendImage from "./TrendImage.vue";
import { trendTikTokPlayerURL } from "@/utils/trendLinks";

const props = defineProps<{
  src?: string | Record<string, string>;
  cover?: string;
  title?: string;
  itemID?: string;
}>();
const { locale } = useI18n();
const playing = ref(false);
const failed = ref(false);
const embedSource = computed(() => trendTikTokPlayerURL(props.itemID));
const source = computed(() =>
  safeTrendVideoURL(
    typeof props.src === "string"
      ? props.src
      : props.src?.default || props.src?.play_addr || ""
  )
);
const label = computed(() =>
  locale.value === "en" ? "Play video" : "播放视频"
);
watch(
  () => [props.src, props.itemID],
  () => {
    playing.value = false;
    failed.value = false;
  }
);
</script>

<template>
  <div class="trend-player">
    <iframe
      v-if="embedSource && (!source || failed)"
      :key="embedSource"
      :src="embedSource"
      :title="title || label"
      class="trend-embed"
      sandbox="allow-scripts allow-same-origin allow-presentation"
      allow="autoplay; encrypted-media; fullscreen; picture-in-picture"
      referrerpolicy="no-referrer"
      allowfullscreen
    />
    <video
      v-else-if="playing && source && !failed"
      :src="source"
      controls
      autoplay
      playsinline
      preload="metadata"
      :aria-label="title || label"
      @error="
        failed = true;
        playing = false;
      "
    />
    <template v-else
      ><TrendImage :src="cover" :alt="title || label" /><button
        v-if="source && !failed"
        class="player-start"
        :aria-label="label"
        @click="playing = true"
      >
        <PlayIcon /><span>{{ label }}</span>
      </button>
      <p v-else class="player-note">
        {{
          locale === "en" ? "Video is currently unavailable" : "视频暂不可播放"
        }}
      </p></template
    >
  </div>
</template>

<style scoped>
.trend-player {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 200px;
  overflow: hidden;
  background: #15171a;
  border-radius: var(--el-border-radius-base);
}
.trend-player video {
  display: block;
  width: 100%;
  height: 100%;
  max-height: 65vh;
  object-fit: contain;
}
.trend-embed {
  display: block;
  width: 100%;
  height: min(65vh, 560px);
  min-height: 300px;
  border: 0;
}
.player-start {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: white;
  cursor: pointer;
  background: rgb(0 0 0 / 28%);
  border: none;
}
.player-start svg {
  width: 48px;
  height: 48px;
}
.player-start:focus-visible {
  outline: 3px solid var(--el-color-primary);
  outline-offset: -3px;
}
.player-note {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  padding: 10px;
  margin: 0;
  font-size: 12px;
  color: white;
  text-align: center;
  background: rgb(0 0 0 / 70%);
}
</style>
