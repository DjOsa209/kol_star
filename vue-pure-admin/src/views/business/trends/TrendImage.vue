<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { http } from "@/utils/http";
import { safeTrendImageURL } from "@/api/trends";

const props = withDefaults(
  defineProps<{ src?: string; alt: string; avatar?: boolean }>(),
  { src: "", avatar: false }
);
const host = ref<HTMLElement>();
const image = ref("");
const failed = ref(false);
let objectURL = "";
let observer: IntersectionObserver | undefined;
let visible = false;
let generation = 0;
async function load() {
  const current = ++generation;
  if (objectURL) URL.revokeObjectURL(objectURL);
  image.value = "";
  objectURL = "";
  failed.value = false;
  if (!props.src) {
    failed.value = true;
    return;
  }
  try {
    if (
      /^\/business\/trends\/media\/[a-z0-9_]+\.jpg$/.test(props.src) ||
      /^\/business\/trends\/local-media\/[a-f0-9]{64}\.img$/.test(props.src)
    ) {
      const blob = await http.request<Blob>("get", props.src, {
        responseType: "blob"
      });
      if (current !== generation) return;
      if (!blob.type.startsWith("image/")) throw new Error("Invalid image");
      objectURL = URL.createObjectURL(blob);
      image.value = objectURL;
    } else {
      image.value = safeTrendImageURL(props.src);
      failed.value = !image.value;
    }
  } catch {
    if (current === generation) failed.value = true;
  }
}
onMounted(() => {
  observer = new IntersectionObserver(
    entries => {
      if (entries.some(entry => entry.isIntersecting)) {
        visible = true;
        observer?.disconnect();
        load();
      }
    },
    { rootMargin: "160px" }
  );
  if (host.value) observer.observe(host.value);
});
watch(
  () => props.src,
  () => {
    if (visible) load();
  }
);
onBeforeUnmount(() => {
  generation++;
  observer?.disconnect();
  if (objectURL) URL.revokeObjectURL(objectURL);
});
</script>

<template>
  <span ref="host" class="trend-image" :class="{ 'is-avatar': avatar }">
    <img
      v-if="image && !failed"
      :src="image"
      :alt="alt"
      referrerpolicy="no-referrer"
      @error="failed = true"
    />
    <span v-else class="image-placeholder" :aria-label="alt">{{
      avatar ? alt.slice(0, 1) : "—"
    }}</span>
  </span>
</template>

<style scoped>
.trend-image {
  display: block;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #f1f3f5;
}
.trend-image img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.is-avatar {
  flex: 0 0 36px;
  width: 36px;
  height: 36px;
  border-radius: 50%;
}
.image-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  color: #64748b;
}
</style>
