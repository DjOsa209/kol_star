<script setup lang="ts">
import { getTopMenu } from "@/router/utils";
import { useNav } from "@/layout/hooks/useNav";

defineProps({
  collapse: Boolean
});

const { title, getLogo } = useNav();
</script>

<template>
  <div class="sidebar-logo-container" :class="{ collapses: collapse }">
    <transition name="sidebarLogoFade">
      <router-link
        v-if="collapse"
        key="collapse"
        :title="title"
        :aria-label="title"
        class="sidebar-logo-link"
        :to="getTopMenu()?.path ?? '/'"
      >
        <img :src="getLogo()" alt="Infinix" />
      </router-link>
      <router-link
        v-else
        key="expand"
        :title="title"
        :aria-label="title"
        class="sidebar-logo-link"
        :to="getTopMenu()?.path ?? '/'"
      >
        <img :src="getLogo()" alt="Infinix" />
      </router-link>
    </transition>
  </div>
</template>

<style lang="scss" scoped>
.sidebar-logo-container {
  position: relative;
  width: 100%;
  height: 44px;
  overflow: hidden;

  .sidebar-logo-link {
    display: flex;
    flex-wrap: nowrap;
    align-items: center;
    height: 100%;
    padding: 0 20px;

    img {
      display: block;
      flex: none;
      width: 114px;
      height: auto;
      object-fit: contain;
      filter: invert(1);
    }
  }

  &.collapses .sidebar-logo-link {
    justify-content: center;
    padding: 0 6px;

    img {
      width: 42px;
    }
  }
}
</style>
