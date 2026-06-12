<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { getResourcePosts } from "@/api/business";

defineOptions({ name: "BusinessResourcePosts" });

const route = useRoute();
const router = useRouter();
const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const currentPage = ref(1);
const pageSize = ref(12);
const resource = ref<any>({});
const stats = ref<any>({});
const creatorAvatarFailed = ref(false);

const search = reactive({
  resourceId: Number(route.query.resourceId || 0),
  platform: "",
  keyword: ""
});

const hasResource = computed(() => Boolean(resource.value?.id));
const statItems = computed(() => [
  ["作品数", stats.value.postCount || 0],
  ["总播放", stats.value.totalViews || 0],
  ["平均播放", stats.value.avgViews || 0],
  ["点赞", stats.value.totalLikes || 0],
  ["评论", stats.value.totalComments || 0],
  ["分享", stats.value.totalShares || 0]
]);

function formatDateTime(value: unknown) {
  if (!value) return "-";
  const time = Number(value);
  if (!Number.isFinite(time)) return String(value);
  return new Date(time).toLocaleString("zh-CN");
}

function formatCount(value: unknown) {
  const number = Number(value || 0);
  if (!Number.isFinite(number)) return "0";
  return number.toLocaleString("zh-CN");
}

function avatarText(row: any) {
  const name = String(row.name || row.resourceName || "?").trim();
  return name.slice(0, 1).toUpperCase() || "?";
}

function markCreatorAvatarFailed() {
  creatorAvatarFailed.value = true;
}

function durationText(value: unknown) {
  const seconds = Number(value || 0);
  if (!seconds) return "-";
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return `${minutes}:${String(rest).padStart(2, "0")}`;
}

function openUrl(url: string) {
  if (!url) return;
  window.open(url, "_blank", "noopener,noreferrer");
}

async function loadData() {
  loading.value = true;
  const res = await getResourcePosts({
    ...search,
    currentPage: currentPage.value,
    pageSize: pageSize.value
  });
  loading.value = false;
  if (res.code !== 0) return;
  const data = res.data || {};
  list.value = data.list || [];
  total.value = Number(data.total || 0);
  resource.value = data.resource || {};
  creatorAvatarFailed.value = false;
  stats.value = data.stats || {};
}

function searchData() {
  currentPage.value = 1;
  syncQuery();
  loadData();
}

function resetFilter() {
  search.resourceId = 0;
  search.platform = "";
  search.keyword = "";
  currentPage.value = 1;
  syncQuery();
  loadData();
}

function handleCurrentChange(page: number) {
  currentPage.value = page;
  loadData();
}

function handleSizeChange(size: number) {
  pageSize.value = size;
  currentPage.value = 1;
  loadData();
}

function syncQuery() {
  router.replace({
    path: "/business/resource-posts",
    query: search.resourceId ? { resourceId: search.resourceId } : {}
  });
}

watch(
  () => route.query.resourceId,
  value => {
    search.resourceId = Number(value || 0);
    currentPage.value = 1;
    loadData();
  }
);

onMounted(loadData);
</script>

<template>
  <div class="posts-page">
    <section class="page-hero">
      <div>
        <span>Content Data</span>
        <h1>作品数据</h1>
        <p>查看同步下来的近期视频/帖子，核对播放、点赞、评论、分享和原始作品链接。</p>
      </div>
    </section>

    <section v-if="hasResource" class="overview">
      <div class="creator-card">
        <div class="creator-avatar">
          <img
            v-if="resource.avatarUrl && !creatorAvatarFailed"
            :src="resource.avatarUrl"
            :alt="resource.name"
            @error="markCreatorAvatarFailed"
          />
          <span v-else>{{ avatarText(resource) }}</span>
        </div>
        <div>
          <strong>{{ resource.name }}</strong>
          <span>{{ resource.platform }} {{ resource.platformHandle || "" }}</span>
          <span>粉丝 {{ formatCount(resource.followers) }}</span>
          <span>上次同步 {{ formatDateTime(resource.lastSyncAt) }}</span>
        </div>
      </div>
      <div class="stats-grid">
        <div v-for="[label, value] in statItems" :key="label">
          <span>{{ label }}</span>
          <strong>{{ formatCount(value) }}</strong>
        </div>
      </div>
    </section>

    <section class="filter-panel">
      <el-form :model="search" inline>
        <el-form-item label="资源ID">
          <el-input-number v-model="search.resourceId" :min="0" />
        </el-form-item>
        <el-form-item label="平台">
          <el-select v-model="search.platform" clearable placeholder="全部" class="filter-select">
            <el-option label="YouTube" value="YouTube" />
            <el-option label="Instagram" value="Instagram" />
            <el-option label="TikTok" value="TikTok" />
          </el-select>
        </el-form-item>
        <el-form-item label="关键词">
          <el-input v-model="search.keyword" clearable placeholder="标题/描述/达人" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="searchData">
            <IconifyIconOnline icon="ri:search-line" class="mr-1" />
            查询
          </el-button>
          <el-button @click="resetFilter">重置</el-button>
        </el-form-item>
      </el-form>
    </section>

    <section v-loading="loading" class="post-list">
      <article v-for="post in list" :key="post.id" class="post-card">
        <button class="cover-button" type="button" @click="openUrl(post.postUrl)">
          <img v-if="post.coverUrl" :src="post.coverUrl" :alt="post.title" />
          <span v-else>{{ post.platform }}</span>
        </button>
        <div class="post-body">
          <div class="post-title">
            <button type="button" @click="openUrl(post.postUrl)">
              {{ post.title || "未命名作品" }}
            </button>
            <el-tag effect="plain">{{ post.platform }}</el-tag>
          </div>
          <p>{{ post.description || "暂无描述" }}</p>
          <div class="post-meta">
            <span>{{ formatDateTime(post.publishedAt) }}</span>
            <span>时长 {{ durationText(post.durationSeconds) }}</span>
            <span v-if="post.mediaType">{{ post.mediaType }}</span>
          </div>
          <div class="post-stats">
            <span>播放 {{ formatCount(post.viewCount) }}</span>
            <span>点赞 {{ formatCount(post.likeCount) }}</span>
            <span>评论 {{ formatCount(post.commentCount) }}</span>
            <span>分享 {{ formatCount(post.shareCount) }}</span>
          </div>
          <div class="post-actions">
            <el-button link type="primary" :disabled="!post.postUrl" @click="openUrl(post.postUrl)">
              <IconifyIconOnline icon="ri:external-link-line" class="mr-1" />
              打开作品
            </el-button>
            <span>同步 {{ formatDateTime(post.syncedAt) }}</span>
          </div>
        </div>
      </article>
      <el-empty v-if="!loading && list.length === 0" description="暂无作品数据" />
    </section>

    <div class="table-footer">
      <span>共 {{ total }} 条作品</span>
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[12, 24, 48, 96]"
        :total="total"
        layout="sizes, prev, pager, next, jumper"
        background
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>
  </div>
</template>

<style scoped>
.posts-page {
  min-height: 100%;
  padding: 20px;
  background: #f8fafc;
}

.page-hero,
.filter-panel,
.overview,
.post-card {
  border: 1px solid rgb(148 163 184 / 22%);
  border-radius: 8px;
  background: #fff;
}

.page-hero {
  padding: 20px;
  margin-bottom: 16px;
  background: linear-gradient(135deg, #fff 0%, #eff6ff 58%, #ecfdf5 100%);
}

.page-hero span {
  font-size: 12px;
  font-weight: 700;
  color: #2563eb;
  text-transform: uppercase;
}

.page-hero h1 {
  margin: 8px 0 0;
  font-size: 26px;
  line-height: 1.25;
  color: #0f172a;
  letter-spacing: 0;
}

.page-hero p,
.creator-card span,
.post-body p,
.post-meta,
.post-actions,
.table-footer {
  color: #64748b;
}

.overview {
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 16px;
  padding: 16px;
  margin-bottom: 16px;
}

.creator-card {
  display: flex;
  gap: 12px;
  align-items: center;
}

.creator-card > div:last-child {
  display: grid;
  gap: 4px;
}

.creator-card strong {
  color: #0f172a;
}

.creator-avatar {
  display: grid;
  width: 56px;
  height: 56px;
  overflow: hidden;
  font-weight: 800;
  color: #475569;
  background: #e2e8f0;
  border-radius: 50%;
  place-items: center;
}

.creator-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 10px;
}

.stats-grid > div {
  padding: 12px;
  background: #f8fafc;
  border: 1px solid rgb(148 163 184 / 18%);
  border-radius: 8px;
}

.stats-grid span {
  display: block;
  font-size: 12px;
  color: #64748b;
}

.stats-grid strong {
  display: block;
  margin-top: 6px;
  color: #0f172a;
}

.filter-panel {
  padding: 16px;
  margin-bottom: 16px;
}

.filter-select {
  width: 150px;
}

.post-list {
  display: grid;
  gap: 12px;
}

.post-card {
  display: grid;
  grid-template-columns: 220px 1fr;
  gap: 16px;
  padding: 12px;
}

.cover-button {
  display: grid;
  width: 100%;
  min-height: 124px;
  overflow: hidden;
  color: #64748b;
  cursor: pointer;
  background: #e2e8f0;
  border: 0;
  border-radius: 8px;
  place-items: center;
}

.cover-button img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.post-body {
  min-width: 0;
}

.post-title {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  justify-content: space-between;
}

.post-title button {
  padding: 0;
  overflow: hidden;
  font-weight: 700;
  color: #0f172a;
  text-align: left;
  text-overflow: ellipsis;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.post-body p {
  display: -webkit-box;
  margin: 8px 0;
  overflow: hidden;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.post-meta,
.post-stats,
.post-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  font-size: 12px;
}

.post-stats {
  margin-top: 10px;
  color: #0f172a;
}

.post-actions {
  justify-content: space-between;
  margin-top: 10px;
}

.table-footer {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  margin-top: 16px;
}

@media (max-width: 1100px) {
  .overview,
  .post-card {
    grid-template-columns: 1fr;
  }

  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
