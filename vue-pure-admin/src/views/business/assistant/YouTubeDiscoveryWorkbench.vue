<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { fieldLabel as t } from "@/utils/fieldI18n";
import { getResourceList, getResourceTagOptions } from "@/api/business";
import { resourceAvatarCandidates } from "../resources/resourcePresentation";
import {
  cancelDiscoveryJob,
  createDiscoveryJob,
  getDiscoveryJob,
  listDiscoveryJobs,
  recommendDiscoveryJob,
  retryDiscoveryJob,
  reviewDiscoveryCandidate,
  type DiscoveryCandidate,
  type DiscoveryJob,
  type DiscoveryPost,
  type DiscoveryRecommendation,
  type DiscoveryStatus
} from "@/api/creatorDiscovery";

const jobs = ref<DiscoveryJob[]>([]);
const job = ref<DiscoveryJob | null>(null);
const candidates = ref<DiscoveryCandidate[]>([]);
const selectedJobId = ref<number | null>(null);
const listLoading = ref(false);
const detailLoading = ref(false);
const creating = ref(false);
const cancelling = ref(false);
const reviewing = ref(false);
const recommending = ref(false);
const error = ref("");
const reviewError = ref("");
const recommendationError = ref("");
const recommendation = ref<DiscoveryRecommendation | null>(null);
const filter = ref("all");
const drawerVisible = ref(false);
const selectedCandidateId = ref<number | null>(null);
const brokenImages = reactive<Record<string, boolean>>({});
const taxonomyLoading = ref(false);
const taxonomyError = ref("");
const taxonomyLoaded = ref(false);
const industryOptions = ref<string[]>([]);
const categoryOptions = ref<string[]>([]);
const tags = ref<{ id: number; name: string; status: string }[]>([]);
const form = reactive({
  query: ""
});
const review = reactive({
  country: "",
  language: "",
  note: "",
  evidenceUrl: "",
  industry: "",
  category: "",
  tagIds: [] as number[],
  topicConfirmed: false
});
let pollTimer: ReturnType<typeof setTimeout> | undefined;
let detailSequence = 0;
let disposed = false;

const statusLabels: Record<DiscoveryStatus, string> = {
  queued: "等待开始",
  planning: "理解推广需求",
  searching: "寻找合适候选",
  collecting: "读取公开资料",
  recommending: "大模型正在分析",
  completed: "推荐已完成",
  partial: "部分流程未完成",
  failed: "推荐未完成",
  cancelled: "已取消"
};
const reviewLabels = {
  pending: "待核实",
  approved: "已核实入库",
  rejected: "已排除"
};
const active = computed(() =>
  ["queued", "planning", "searching", "collecting", "recommending"].includes(
    job.value?.status || ""
  )
);
const busy = computed(
  () =>
    creating.value || cancelling.value || reviewing.value || recommending.value
);
const approvedCount = computed(
  () => candidates.value.filter(item => item.status === "approved").length
);
const visibleCandidates = computed(() =>
  candidates.value.filter(
    item => filter.value === "all" || item.status === filter.value
  )
);
const selectedCandidate = computed(
  () =>
    candidates.value.find(item => item.id === selectedCandidateId.value) || null
);
const validReview = computed(
  () =>
    review.country.trim() &&
    review.language.trim() &&
    review.note.trim() &&
    safeUrl(review.evidenceUrl) &&
    review.topicConfirmed
);
const reviewTagOptions = computed(() =>
  tags.value.filter(
    tag => tag.status === "启用" || review.tagIds.includes(tag.id)
  )
);

async function loadTaxonomy() {
  if (taxonomyLoaded.value || taxonomyLoading.value) return;
  taxonomyLoading.value = true;
  taxonomyError.value = "";
  const results = await Promise.allSettled([
    getResourceTagOptions(),
    getResourceList({ currentPage: 1, pageSize: 200 })
  ]);
  try {
    if (results[0].status === "fulfilled") {
      tags.value = check(results[0].value) || [];
    } else throw results[0].reason;
    if (results[1].status === "fulfilled") {
      const rows = check(results[1].value)?.list || [];
      const values = (key: string) =>
        [
          ...new Set<string>(
            rows.map(row => String(row[key] || "").trim()).filter(Boolean)
          )
        ].sort();
      industryOptions.value = values("industry");
      categoryOptions.value = values("category");
    }
    taxonomyLoaded.value = true;
  } catch {
    taxonomyError.value = t(
      "分类与标签选项暂未加载，请重试。行业和资源领域也可手动填写。"
    );
  } finally {
    taxonomyLoading.value = false;
  }
}

function safeUrl(value?: string) {
  if (!value) return "";
  try {
    const url = new URL(value);
    return ["https:", "http:"].includes(url.protocol) ? url.href : "";
  } catch {
    return "";
  }
}
function count(value: unknown) {
  if (value === null || value === undefined || value === "") return t("未知");
  const numeric = Number(value);
  return Number.isFinite(numeric)
    ? new Intl.NumberFormat("zh-CN", {
        notation: "compact",
        maximumFractionDigits: 1
      }).format(numeric)
    : t("未知");
}
function date(value?: string) {
  if (!value) return t("时间未知");
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime())
    ? value
    : parsed.toLocaleString("zh-CN", { hour12: false });
}
function collectedCount(value: number | null | undefined, warning?: string) {
  return warning && value === 0 ? t("采集不完整") : count(value);
}
function postCount(value: number | null | undefined) {
  return Number(value) > 0 ? count(value) : "—";
}
function errorMessage(value: unknown) {
  return value instanceof Error ? value.message : t("请求未完成，请重试。");
}
function check<T>(response: { code: number; message: string; data: T }): T {
  if (response.code !== 0)
    throw new Error(response.message || t("请求未完成，请重试。"));
  return response.data;
}
function stopPolling() {
  if (pollTimer) clearTimeout(pollTimer);
  pollTimer = undefined;
}
async function refreshJobs() {
  listLoading.value = true;
  try {
    const data = check(await listDiscoveryJobs());
    if (!disposed) jobs.value = data.jobs || [];
  } catch (cause) {
    if (!disposed) error.value = errorMessage(cause);
  } finally {
    listLoading.value = false;
  }
}
async function loadJob(id: number, quiet = false) {
  stopPolling();
  const sequence = ++detailSequence;
  if (!quiet) detailLoading.value = true;
  try {
    const data = check(await getDiscoveryJob(id));
    if (disposed || sequence !== detailSequence || selectedJobId.value !== id)
      return;
    job.value = data.job;
    candidates.value = data.candidates || [];
    recommendation.value = data.job.recommendation || null;
    error.value = "";
    const existing = jobs.value.findIndex(item => item.id === id);
    if (existing >= 0) jobs.value.splice(existing, 1, data.job);
    else jobs.value.unshift(data.job);
    if (active.value) pollTimer = setTimeout(() => loadJob(id, true), 2500);
  } catch (cause) {
    if (!disposed && sequence === detailSequence)
      error.value = errorMessage(cause);
  } finally {
    if (sequence === detailSequence) detailLoading.value = false;
  }
}
async function selectJob(id: number) {
  selectedJobId.value = id;
  job.value = null;
  candidates.value = [];
  recommendation.value = null;
  recommendationError.value = "";
  drawerVisible.value = false;
  filter.value = "all";
  await loadJob(id);
  if (job.value) {
    form.query = recommendation.value?.demandText || job.value.query;
  }
}
async function createJob(retry = false) {
  if (busy.value || (!retry && active.value)) return;
  if (!retry && !form.query.trim()) {
    error.value = t("请先描述你的推广需求。");
    return;
  }
  creating.value = true;
  error.value = "";
  try {
    const data =
      retry && job.value
        ? check(await retryDiscoveryJob(job.value.id))
        : check(
            await createDiscoveryJob({
              query: form.query.trim()
            })
          );
    await selectJob(data.jobId);
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    creating.value = false;
  }
}
async function cancelJob() {
  if (!job.value) return;
  cancelling.value = true;
  try {
    check(await cancelDiscoveryJob(job.value.id));
    await loadJob(job.value.id, true);
  } catch (cause) {
    error.value = errorMessage(cause);
  } finally {
    cancelling.value = false;
  }
}
function openReview(candidate: DiscoveryCandidate) {
  void loadTaxonomy();
  selectedCandidateId.value = candidate.id;
  Object.assign(review, {
    country:
      candidate.country ||
      (candidate.snapshot.country === "IQ" ? "伊拉克" : ""),
    language:
      candidate.language ||
      (candidate.snapshot.language === "ar"
        ? "阿拉伯语"
        : candidate.snapshot.language || ""),
    note: candidate.note || "",
    evidenceUrl: candidate.evidenceUrl || candidate.snapshot.profileUrl || "",
    industry: candidate.industry || "",
    category: candidate.category || "",
    tagIds: [...(candidate.tagIds || [])],
    topicConfirmed: candidate.status === "approved"
  });
  reviewError.value = "";
  drawerVisible.value = true;
}
async function submitReview(action: "approve" | "reject") {
  if (!selectedCandidate.value || !job.value || active.value) return;
  if (action === "approve" && !validReview.value) {
    reviewError.value = t(
      "请补全国家、语言、核实说明和证据链接，并确认内容与需求相关。"
    );
    return;
  }
  if (action === "reject" && !review.note.trim()) {
    reviewError.value = t("请填写排除原因，保留审核记录。");
    return;
  }
  if (
    Array.from(review.industry).length > 128 ||
    Array.from(review.category).length > 128
  ) {
    reviewError.value = t("行业和资源领域最多填写 128 个字。");
    return;
  }
  reviewing.value = true;
  reviewError.value = "";
  try {
    check(
      await reviewDiscoveryCandidate({
        id: selectedCandidate.value.id,
        action,
        ...review
      })
    );
    await loadJob(job.value.id, true);
    drawerVisible.value = false;
    ElMessage.success(
      t(action === "approve" ? "已核实入库，资源状态为观察中" : "已排除此候选")
    );
  } catch (cause) {
    reviewError.value = errorMessage(cause);
  } finally {
    reviewing.value = false;
  }
}
async function recommend() {
  if (!job.value || !form.query.trim() || active.value || busy.value) return;
  recommending.value = true;
  recommendationError.value = "";
  recommendation.value = null;
  try {
    const data = check(
      await recommendDiscoveryJob(job.value.id, form.query.trim())
    );
    if (data.source !== "ai" || !data.modelInvoked)
      throw new Error(t("未收到大模型推荐结果，请重试。"));
    recommendation.value = data;
    job.value.recommendation = data;
    await loadJob(job.value.id, true);
  } catch (cause) {
    recommendationError.value = errorMessage(cause);
  } finally {
    recommending.value = false;
  }
}
function candidateFor(item: Record<string, any>) {
  if (item.candidateId)
    return candidates.value.find(
      candidate => candidate.id === Number(item.candidateId)
    );
  const resourceId = Number(item.resourceId || item.resource?.id);
  if (resourceId)
    return candidates.value.find(
      candidate => candidate.resourceId === resourceId
    );
  return (
    candidates.value.find(
      candidate => candidate.resourceId === Number(item.id)
    ) || candidates.value.find(candidate => candidate.id === Number(item.id))
  );
}
function recommendationSnapshot(item: Record<string, any>) {
  return candidateFor(item)?.snapshot;
}
function recommendationPosts(item: Record<string, any>): DiscoveryPost[] {
  return recommendationSnapshot(item)?.recentPosts || item.recentPosts || [];
}
function recommendationAvatar(item: Record<string, any>) {
  return (
    resourceAvatarCandidates(recommendationSnapshot(item) || item).find(
      url => !brokenImages[url]
    ) || ""
  );
}
function candidateAvatar(candidate: DiscoveryCandidate) {
  return (
    resourceAvatarCandidates(candidate.snapshot).find(
      url => !brokenImages[url]
    ) || ""
  );
}
function recommendationStatus(item: Record<string, any>) {
  return candidateFor(item)?.status || "pending";
}
function postUrl(post: DiscoveryPost) {
  return safeUrl(post.url || post.postUrl);
}
function recommendationName(item: Record<string, any>) {
  return (
    item.name ||
    item.resourceName ||
    item.resource?.name ||
    candidateFor(item)?.snapshot.name ||
    t("推荐达人")
  );
}
function reasons(item: Record<string, any>) {
  if (typeof item.reason === "string" && item.reason.trim())
    return item.reason.trim();
  if (typeof item.advice === "string" && item.advice.trim())
    return item.advice.trim();
  if (Array.isArray(item.hitRules) && item.hitRules.length)
    return item.hitRules.join("；");
  return "";
}
function matchingEvidence(item: Record<string, any>) {
  if (!Array.isArray(item.hitRules)) return "";
  const reason = reasons(item);
  return [...new Set(item.hitRules)]
    .filter(
      (value): value is string =>
        typeof value === "string" &&
        Boolean(value.trim()) &&
        !reason.includes(value.trim())
    )
    .map(value => value.trim())
    .join("；");
}
function modelScore(item: Record<string, any>): number | null {
  return typeof item.matchScore === "number" &&
    Number.isFinite(item.matchScore) &&
    item.matchScore >= 0 &&
    item.matchScore <= 100
    ? item.matchScore
    : null;
}

onMounted(async () => {
  await refreshJobs();
  if (!disposed && jobs.value.length) await selectJob(jobs.value[0].id);
});
onBeforeUnmount(() => {
  disposed = true;
  detailSequence++;
  stopPolling();
});
</script>

<template>
  <div class="discovery-workbench">
    <header class="discovery-heading">
      <div>
        <h1>{{ t("达人 / 媒体 AI 推荐") }}</h1>
        <p>{{ t("描述推广目标，AI 寻找候选并结合公开资料给出推荐。") }}</p>
      </div>
    </header>

    <div class="discovery-columns">
      <aside class="discovery-brief">
        <h2>{{ t("你在寻找什么？") }}</h2>
        <form @submit.prevent="createJob()">
          <label for="discovery-demand">{{ t("推广需求") }}</label>
          <el-input
            id="discovery-demand"
            v-model="form.query"
            type="textarea"
            :rows="7"
            maxlength="2000"
            show-word-limit
            resize="vertical"
            :placeholder="
              t(
                '例如：寻找面向伊拉克市场的手机科技达人，用于智能手机测评与新品推广。'
              )
            "
            :disabled="busy || active"
          />
          <el-button
            native-type="submit"
            class="primary-action create-action"
            :loading="creating || active"
            :disabled="busy || active || !form.query.trim()"
          >
            <IconifyIconOnline
              v-if="!creating && !active"
              icon="ri:sparkling-line"
            />
            {{ t(active ? "正在寻找与推荐" : "生成推荐") }}
          </el-button>
        </form>
        <p class="field-help">
          {{
            t(
              "推荐会附上真实资料与来源证据。选中合适的候选后，再核实并加入资源库。"
            )
          }}
        </p>
        <div class="history-heading">
          <h3>{{ t("最近的推荐") }}</h3>
          <el-button
            text
            :loading="listLoading"
            :disabled="busy"
            @click="refreshJobs"
            >{{ t("刷新") }}</el-button
          >
        </div>
        <el-select
          :model-value="selectedJobId"
          class="history-select"
          :placeholder="t('选择推荐记录')"
          :disabled="busy || listLoading"
          @change="selectJob"
        >
          <el-option
            v-for="item in jobs"
            :key="item.id"
            :value="item.id"
            :label="`#${item.id} · ${t(statusLabels[item.status])} · ${date(item.createdAt)}`"
          />
        </el-select>
      </aside>

      <main class="discovery-results" aria-live="polite">
        <div v-if="error" class="notice error-notice" role="alert">
          <IconifyIconOnline icon="ri:error-warning-line" />
          <div>
            <strong>{{ t("请求未完成") }}</strong>
            <p>{{ error }}</p>
          </div>
          <el-button
            v-if="selectedJobId"
            :disabled="busy"
            @click="loadJob(selectedJobId)"
            >{{ t("重新读取") }}</el-button
          >
        </div>
        <div
          v-if="detailLoading || (listLoading && !job)"
          class="empty-state"
          role="status"
        >
          <IconifyIconOnline class="spinning" icon="ri:loader-4-line" />
          <h2>{{ t("读取推荐记录") }}</h2>
        </div>
        <template v-else-if="job">
          <section class="job-progress" :aria-label="t('推荐进度')">
            <div class="section-heading">
              <div>
                <h2>
                  {{
                    t(
                      recommending ? "大模型正在分析" : statusLabels[job.status]
                    )
                  }}
                  <span class="job-number">#{{ job.id }}</span>
                </h2>
                <p>{{ job.message || t("等待任务返回进度") }}</p>
              </div>
              <el-button
                v-if="active"
                :loading="cancelling"
                :disabled="busy"
                @click="cancelJob"
                >{{ t("取消任务") }}</el-button
              >
              <el-button
                v-else
                :loading="creating"
                :disabled="busy"
                @click="createJob(true)"
                >{{ t("重试推荐") }}</el-button
              >
            </div>
            <ol class="flow-steps">
              <li :class="{ complete: candidates.length > 0 }">
                <IconifyIconOnline icon="ri:search-line" /><span>{{
                  t("寻找候选")
                }}</span>
                <strong>{{ candidates.length }} {{ t("位候选") }}</strong>
              </li>
              <li :class="{ complete: !!recommendation }">
                <IconifyIconOnline icon="ri:sparkling-line" /><span>{{
                  t("AI 推荐")
                }}</span>
                <strong>{{
                  recommendation
                    ? `${recommendation.recommendations.length} ${t("位推荐")}`
                    : t(
                        job.status === "recommending" || recommending
                          ? "分析中"
                          : "待生成"
                      )
                }}</strong>
              </li>
              <li :class="{ complete: approvedCount > 0 }">
                <IconifyIconOnline icon="ri:shield-check-line" /><span>{{
                  t("人工核实入库")
                }}</span>
                <strong>{{ approvedCount }} {{ t("位已入库") }}</strong>
              </li>
            </ol>
            <p class="job-time">
              {{ t("更新于") }} {{ date(job.updatedAt)
              }}<span v-if="active">
                ·
                {{
                  t(error ? "自动更新已暂停，请重新读取" : "自动更新中")
                }}</span
              >
            </p>
          </section>

          <section class="ai-section">
            <div class="section-heading">
              <div>
                <h2>{{ t("推荐结果") }}</h2>
                <p>
                  {{
                    t(
                      "结合实际内容与推广需求分析；待核实的候选可在推荐后确认入库。"
                    )
                  }}
                </p>
              </div>
              <el-button
                v-if="!active && candidates.length"
                :loading="recommending"
                :disabled="busy || !form.query.trim()"
                @click="recommend"
                >{{ t("重新分析候选") }}</el-button
              >
            </div>
            <div
              v-if="recommendationError"
              class="notice error-notice"
              role="alert"
            >
              {{ recommendationError }} {{ t("请重试生成推荐。") }}
            </div>
            <div
              v-if="!recommendation && (active || recommending)"
              class="empty-state"
              role="status"
            >
              <IconifyIconOnline class="spinning" icon="ri:loader-4-line" />
              <h3>
                {{
                  t(recommending ? "大模型正在分析" : statusLabels[job.status])
                }}
              </h3>
              <p>
                {{
                  t(
                    "AI 正在结合需求寻找候选、读取真实资料并生成推荐，结果会自动出现在这里。"
                  )
                }}
              </p>
            </div>
            <div v-else-if="!recommendation" class="empty-state">
              <IconifyIconOnline icon="ri:search-line" />
              <h3>
                {{
                  t(
                    job.status === "cancelled"
                      ? "本次推荐已取消"
                      : "暂未生成推荐"
                  )
                }}
              </h3>
              <p>
                {{
                  t(
                    candidates.length
                      ? "已找到的候选与证据仍可查看；可以重新分析这些候选。"
                      : "查看上方任务说明，调整需求后重新生成推荐。"
                  )
                }}
              </p>
            </div>
            <div v-if="recommendation" class="ai-result">
              <p class="model-label">
                <IconifyIconOnline icon="ri:sparkling-line" />
                {{ t("大模型") }} · {{ recommendation.model }} ·
                {{ t("已真实调用") }}
              </p>
              <p v-if="recommendation.message" class="recommendation-message">
                {{ recommendation.message }}
              </p>
              <p v-if="recommendation.demandText" class="ai-secondary">
                {{ t("生成依据") }}：{{ recommendation.demandText }}
                <span v-if="recommendation.generatedAt">
                  · {{ date(recommendation.generatedAt) }}
                </span>
              </p>
              <article
                v-for="(item, index) in recommendation.recommendations"
                :key="item.candidateId || item.id || item.resourceId || index"
                class="ai-candidate"
              >
                <div class="section-heading">
                  <div class="ai-card-title">
                    <div class="avatar">
                      <img
                        v-if="
                          recommendationAvatar(item) &&
                          !brokenImages[recommendationAvatar(item)]
                        "
                        :src="recommendationAvatar(item)"
                        :alt="recommendationName(item)"
                        loading="lazy"
                        @error="brokenImages[recommendationAvatar(item)] = true"
                      />
                      <IconifyIconOnline v-else icon="ri:user-line" />
                    </div>
                    <div class="ai-card-name">
                      <div class="ai-candidate-heading">
                        <h3>{{ recommendationName(item) }}</h3>
                        <span v-if="modelScore(item) !== null" class="ai-score">
                          {{ t("匹配度") }} {{ modelScore(item) }}/100
                        </span>
                        <span
                          v-if="
                            typeof item.priority === 'string' &&
                            item.priority.trim()
                          "
                          class="ai-priority"
                        >
                          {{ t("优先级") }}：{{ t(item.priority) }}
                        </span>
                      </div>
                      <p class="ai-source">
                        <IconifyIconOnline icon="ri:youtube-line" />{{
                          item.platform || "YouTube"
                        }}<span v-if="recommendationSnapshot(item)?.handle">
                          · {{ recommendationSnapshot(item).handle }}</span
                        >
                      </p>
                    </div>
                  </div>
                  <span
                    class="review-status"
                    :class="recommendationStatus(item)"
                    >{{ t(reviewLabels[recommendationStatus(item)]) }}</span
                  >
                </div>
                <dl class="candidate-metrics">
                  <div>
                    <dt>{{ t("订阅者") }}</dt>
                    <dd>
                      {{
                        count(
                          recommendationSnapshot(item)?.followers ??
                            item.followers
                        )
                      }}
                    </dd>
                  </div>
                  <div>
                    <dt>{{ t("累计播放") }}</dt>
                    <dd>
                      {{
                        collectedCount(
                          recommendationSnapshot(item)?.totalViews,
                          recommendationSnapshot(item)?.warning
                        )
                      }}
                    </dd>
                  </div>
                  <div>
                    <dt>{{ t("平台所在地") }}</dt>
                    <dd>
                      {{
                        recommendationSnapshot(item)?.country ||
                        item.platformReportedCountry ||
                        t("未提供")
                      }}
                    </dd>
                  </div>
                </dl>
                <p v-if="reasons(item)">{{ reasons(item) }}</p>
                <p v-if="matchingEvidence(item)" class="ai-secondary">
                  {{ t("匹配依据") }}：{{ matchingEvidence(item) }}
                </p>
                <p
                  v-if="Array.isArray(item.scenarios) && item.scenarios.length"
                  class="ai-secondary"
                >
                  {{ t("适用场景") }}：{{ item.scenarios.join("；") }}
                </p>
                <p v-if="item.riskTip || item.risks" class="ai-secondary">
                  {{ t("待确认") }}：{{
                    Array.isArray(item.risks)
                      ? item.risks.join("；")
                      : item.riskTip || item.risks
                  }}
                </p>
                <div
                  v-if="recommendationPosts(item).length"
                  class="post-strip ai-posts"
                >
                  <a
                    v-for="(post, postIndex) in recommendationPosts(item).slice(
                      0,
                      3
                    )"
                    :key="post.platformPostId || post.postId || postIndex"
                    :href="postUrl(post) || undefined"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="post-preview"
                    :aria-label="post.title || t('查看作品')"
                  >
                    <div class="post-image">
                      <img
                        v-if="
                          safeUrl(post.coverUrl) && !brokenImages[post.coverUrl]
                        "
                        :src="post.coverUrl"
                        :alt="post.title || t('作品封面')"
                        loading="lazy"
                        @error="brokenImages[post.coverUrl] = true"
                      /><IconifyIconOnline v-else icon="ri:video-line" />
                    </div>
                    <span dir="auto">{{
                      post.title || t("未提供作品标题")
                    }}</span>
                  </a>
                </div>
                <div class="candidate-footer">
                  <span
                    >{{ t("资料更新于") }}
                    {{
                      date(
                        recommendationSnapshot(item)?.collectedAt ||
                          item.collectedAt
                      )
                    }}</span
                  >
                  <el-button
                    v-if="candidateFor(item)"
                    :disabled="busy"
                    @click="openReview(candidateFor(item))"
                    >{{
                      t(
                        recommendationStatus(item) === "pending"
                          ? "查看证据并核实入库"
                          : "查看核实记录"
                      )
                    }}<IconifyIconOnline icon="ri:arrow-right-line"
                  /></el-button>
                </div>
              </article>
              <p
                v-if="!recommendation.recommendations.length"
                class="empty-recommendation"
              >
                {{ t("大模型未推荐任何候选。可调整推广需求后重试。") }}
              </p>
            </div>
          </section>
          <details class="candidate-evidence">
            <summary>
              {{ t("查看全部候选与证据") }} <span>{{ candidates.length }}</span
              ><IconifyIconOnline icon="ri:arrow-down-s-line" />
            </summary>
            <section class="candidate-section">
              <div class="candidate-toolbar">
                <h2>{{ t("全部候选") }}</h2>
                <div
                  class="filter-tabs"
                  role="group"
                  :aria-label="t('按核实状态筛选')"
                >
                  <button
                    v-for="option in [
                      { value: 'all', label: '全部' },
                      { value: 'pending', label: '待核实' },
                      { value: 'approved', label: '已入库' },
                      { value: 'rejected', label: '已排除' }
                    ]"
                    :key="option.value"
                    type="button"
                    :class="{ selected: filter === option.value }"
                    :aria-pressed="filter === option.value"
                    @click="filter = option.value"
                  >
                    {{ t(option.label) }}
                  </button>
                </div>
              </div>
              <p class="section-note">
                {{
                  t(
                    "频道所在地不等于受众国家。请结合公开介绍和实际作品核实目标市场与内容相关性。"
                  )
                }}
              </p>
              <article
                v-for="candidate in visibleCandidates"
                :key="candidate.id"
                class="candidate-row"
              >
                <div class="candidate-top">
                  <div class="avatar">
                    <img
                      v-if="candidateAvatar(candidate)"
                      :src="candidateAvatar(candidate)"
                      :alt="candidate.snapshot.name"
                      loading="lazy"
                      @error="brokenImages[candidateAvatar(candidate)] = true"
                    /><IconifyIconOnline v-else icon="ri:user-line" />
                  </div>
                  <div class="candidate-identity">
                    <h3>
                      <a
                        :href="safeUrl(candidate.snapshot.profileUrl)"
                        target="_blank"
                        rel="noopener noreferrer"
                        >{{ candidate.snapshot.name }}</a
                      >
                    </h3>
                    <p>
                      {{ candidate.snapshot.handle || candidate.channelId }}
                    </p>
                  </div>
                  <span class="review-status" :class="candidate.status">{{
                    t(reviewLabels[candidate.status])
                  }}</span>
                </div>
                <p class="candidate-description" dir="auto">
                  {{ candidate.snapshot.description || t("未获取频道简介") }}
                </p>
                <dl class="candidate-metrics">
                  <div>
                    <dt>{{ t("订阅者") }}</dt>
                    <dd>{{ count(candidate.snapshot.followers) }}</dd>
                  </div>
                  <div>
                    <dt>{{ t("累计播放") }}</dt>
                    <dd>
                      {{
                        collectedCount(
                          candidate.snapshot.totalViews,
                          candidate.snapshot.warning
                        )
                      }}
                    </dd>
                  </div>
                  <div>
                    <dt>{{ t("频道作品") }}</dt>
                    <dd>
                      {{
                        collectedCount(
                          candidate.snapshot.videoCount,
                          candidate.snapshot.warning
                        )
                      }}
                    </dd>
                  </div>
                  <div>
                    <dt>{{ t("平台所在地") }}</dt>
                    <dd>{{ candidate.snapshot.country || t("未提供") }}</dd>
                  </div>
                </dl>
                <div
                  v-if="candidate.snapshot.recentPosts?.length"
                  class="post-strip"
                >
                  <a
                    v-for="(
                      post, postIndex
                    ) in candidate.snapshot.recentPosts.slice(0, 3)"
                    :key="post.platformPostId || post.postId || postIndex"
                    :href="postUrl(post) || undefined"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="post-preview"
                    :aria-label="post.title || t('查看作品')"
                  >
                    <div class="post-image">
                      <img
                        v-if="
                          safeUrl(post.coverUrl) && !brokenImages[post.coverUrl]
                        "
                        :src="post.coverUrl"
                        :alt="post.title || t('作品封面')"
                        loading="lazy"
                        @error="brokenImages[post.coverUrl] = true"
                      /><IconifyIconOnline v-else icon="ri:video-line" />
                    </div>
                    <span dir="auto">{{
                      post.title || t("未提供作品标题")
                    }}</span>
                  </a>
                </div>
                <p v-else class="field-help">
                  {{ t("未采集到近期作品，请打开频道核实。") }}
                </p>
                <p v-if="candidate.snapshot.warning" class="candidate-warning">
                  {{ candidate.snapshot.warning }}
                </p>
                <div class="candidate-footer">
                  <span
                    >{{ t("采集于") }}
                    {{ date(candidate.snapshot.collectedAt) }}</span
                  ><el-button :disabled="busy" @click="openReview(candidate)"
                    >{{
                      t(
                        candidate.status === "pending"
                          ? "查看证据并核实"
                          : "查看核实记录"
                      )
                    }}<IconifyIconOnline icon="ri:arrow-right-line"
                  /></el-button>
                </div>
              </article>
              <div v-if="!visibleCandidates.length" class="empty-state">
                <IconifyIconOnline
                  :class="{ spinning: active }"
                  :icon="active ? 'ri:loader-4-line' : 'ri:search-line'"
                />
                <h3>
                  {{
                    t(
                      active
                        ? "正在寻找真实候选"
                        : candidates.length
                          ? "此状态下暂无候选"
                          : "此次暂无候选"
                    )
                  }}
                </h3>
                <p>
                  {{
                    t(
                      active
                        ? "发现和采集结果会在这里自动更新。"
                        : candidates.length
                          ? "切换筛选查看其他候选。"
                          : "查看上方任务说明，调整需求后重新推荐。"
                    )
                  }}
                </p>
              </div>
            </section>
          </details>
        </template>
        <div v-else class="empty-state initial-empty">
          <IconifyIconOnline icon="ri:sparkling-line" />
          <h2>{{ t("描述需求，找到合适的人选") }}</h2>
          <p>
            {{
              t(
                "告诉 AI 目标市场、产品和内容方向。推荐结果会包含真实资料、匹配理由和可追溯的来源证据。"
              )
            }}
          </p>
        </div>
      </main>
    </div>

    <el-drawer
      v-model="drawerVisible"
      :title="selectedCandidate?.snapshot.name || t('核实频道')"
      size="min(760px, 100%)"
      :close-on-click-modal="!reviewing"
      :close-on-press-escape="!reviewing"
      :show-close="!reviewing"
      class="discovery-review-drawer"
    >
      <div v-if="selectedCandidate" class="review-content">
        <div class="review-summary">
          <span class="review-status" :class="selectedCandidate.status">{{
            t(reviewLabels[selectedCandidate.status])
          }}</span
          ><a
            :href="safeUrl(selectedCandidate.snapshot.profileUrl)"
            target="_blank"
            rel="noopener noreferrer"
            >{{ t("打开 YouTube 频道") }}
            <IconifyIconOnline icon="ri:external-link-line"
          /></a>
        </div>
        <p dir="auto" class="review-description">
          {{ selectedCandidate.snapshot.description || t("未获取频道简介") }}
        </p>
        <p class="section-note">
          {{ t("平台所在地") }}：{{
            selectedCandidate.snapshot.country || t("未提供")
          }}
          · {{ t("频道所在地不等于受众国家，受众分布仍需合作前核实。") }}
        </p>
        <h3>{{ t("发现来源") }}</h3>
        <ul
          v-if="selectedCandidate.snapshot.evidence?.length"
          class="evidence-list"
        >
          <li
            v-for="(evidence, index) in selectedCandidate.snapshot.evidence"
            :key="index"
          >
            <a
              :href="safeUrl(evidence.url)"
              target="_blank"
              rel="noopener noreferrer"
              >{{ evidence.title || evidence.url }}
              <IconifyIconOnline icon="ri:external-link-line"
            /></a>
            <p v-if="evidence.query">{{ t("搜索词") }}：{{ evidence.query }}</p>
            <p v-if="evidence.summary" dir="auto">{{ evidence.summary }}</p>
          </li>
        </ul>
        <p v-else class="field-help">
          {{ t("暂无额外发现来源，请打开频道查看公开资料。") }}
        </p>
        <h3>{{ t("近期作品") }}</h3>
        <div
          v-if="selectedCandidate.snapshot.recentPosts?.length"
          class="review-post-list"
        >
          <article
            v-for="(post, index) in selectedCandidate.snapshot.recentPosts"
            :key="post.platformPostId || index"
          >
            <a
              class="review-post-image"
              :href="postUrl(post) || undefined"
              target="_blank"
              rel="noopener noreferrer"
              ><img
                v-if="safeUrl(post.coverUrl) && !brokenImages[post.coverUrl]"
                :src="post.coverUrl"
                :alt="post.title || t('作品封面')"
                loading="lazy"
                @error="brokenImages[post.coverUrl] = true" /><IconifyIconOnline
                v-else
                icon="ri:video-line"
            /></a>
            <div>
              <a
                :href="postUrl(post) || undefined"
                target="_blank"
                rel="noopener noreferrer"
                dir="auto"
                >{{ post.title || t("未提供作品标题") }}</a
              >
              <p>
                {{ t("播放") }} {{ postCount(post.viewCount) }} ·
                {{ t("点赞") }} {{ postCount(post.likeCount) }} ·
                {{ t("评论") }}
                {{ postCount(post.commentCount) }}
              </p>
              <p>{{ date(post.publishedAt) }}</p>
            </div>
          </article>
        </div>
        <p v-else class="field-help">
          {{ t("未采集到近期作品，请打开频道核实。") }}
        </p>
        <p
          v-if="selectedCandidate.snapshot.recentPosts?.length"
          class="field-help"
        >
          {{ t("作品指标为 0 或未获取时以 — 展示。") }}
        </p>
        <div class="review-form">
          <h3>
            {{
              t(
                selectedCandidate.status === "pending"
                  ? "核实后入库"
                  : "核实记录"
              )
            }}
          </h3>
          <p class="section-note">
            {{
              t("确认入库后，资源为观察中。合作报价、档期和受众分布仍待确认。")
            }}
          </p>
          <div v-if="active" class="notice">
            {{ t("AI 仍在处理本次需求，完成或取消后可核实入库。") }}
          </div>
          <el-form
            label-position="top"
            :disabled="
              reviewing || selectedCandidate.status !== 'pending' || active
            "
          >
            <div class="review-form-row">
              <el-form-item :label="t('已核实的国家 / 地区')" required
                ><el-input
                  v-model="review.country"
                  maxlength="64"
                  :placeholder="t('根据公开证据填写')" /></el-form-item
              ><el-form-item :label="t('内容语言')" required
                ><el-input
                  v-model="review.language"
                  maxlength="64"
                  :placeholder="t('如：阿拉伯语')"
              /></el-form-item>
            </div>
            <div class="review-form-row">
              <el-form-item :label="t('行业')"
                ><el-select
                  v-model="review.industry"
                  filterable
                  allow-create
                  default-first-option
                  clearable
                  :placeholder="t('选择或输入行业')"
                  ><el-option
                    v-for="value in industryOptions"
                    :key="value"
                    :value="value"
                    :label="value" /></el-select
              ></el-form-item>
              <el-form-item :label="t('资源领域')"
                ><el-select
                  v-model="review.category"
                  filterable
                  allow-create
                  default-first-option
                  clearable
                  :placeholder="t('选择或输入资源领域')"
                  ><el-option
                    v-for="value in categoryOptions"
                    :key="value"
                    :value="value"
                    :label="value" /></el-select
              ></el-form-item>
            </div>
            <el-form-item :label="t('资源标签')"
              ><el-select
                v-model="review.tagIds"
                multiple
                filterable
                clearable
                collapse-tags
                collapse-tags-tooltip
                :multiple-limit="50"
                :loading="taxonomyLoading"
                :placeholder="t('选择已有资源标签')"
                ><el-option
                  v-for="tag in reviewTagOptions"
                  :key="tag.id"
                  :value="tag.id"
                  :label="tag.name"
                  :disabled="tag.status !== '启用'" /></el-select
            ></el-form-item>
            <div v-if="taxonomyError" class="notice">
              <span>{{ taxonomyError }}</span
              ><el-button text @click="loadTaxonomy">{{ t("重试") }}</el-button>
            </div>
            <el-form-item :label="t('核实证据链接')" required
              ><el-input
                v-model="review.evidenceUrl"
                maxlength="1024"
                placeholder="https://www.youtube.com/..."
            /></el-form-item>
            <el-form-item :label="t('核实说明 / 排除原因')" required
              ><el-input
                v-model="review.note"
                type="textarea"
                :rows="4"
                maxlength="2000"
                show-word-limit
                :placeholder="
                  t(
                    '说明目标市场与内容的相关性，引用具体资料或作品；排除时说明原因。'
                  )
                "
            /></el-form-item>
            <el-checkbox v-model="review.topicConfirmed">{{
              t("已查看实际作品，确认内容与推广需求相关")
            }}</el-checkbox>
          </el-form>
          <p v-if="selectedCandidate.resourceId" class="field-help">
            {{ t("资源编号") }} #{{ selectedCandidate.resourceId }}
          </p>
          <div v-if="reviewError" class="notice error-notice" role="alert">
            {{ reviewError }}
          </div>
        </div>
      </div>
      <template #footer
        ><div class="review-actions">
          <el-button :disabled="reviewing" @click="drawerVisible = false">{{
            t("关闭")
          }}</el-button
          ><template v-if="selectedCandidate?.status === 'pending'"
            ><el-button
              :loading="reviewing"
              :disabled="reviewing || active || !review.note.trim()"
              @click="submitReview('reject')"
              >{{ t("排除此候选") }}</el-button
            ><el-button
              class="primary-action"
              :loading="reviewing"
              :disabled="reviewing || active || !validReview"
              @click="submitReview('approve')"
              >{{ t("确认核实并入库") }}</el-button
            ></template
          >
        </div></template
      >
    </el-drawer>
  </div>
</template>

<style scoped>
.discovery-workbench {
  --ink: #171717;
  --muted: #656971;
  --line: #e8e8e8;
  --accent: #d4fb00;
  --el-text-color-placeholder: #6b7077;

  font-size: 14px;
  color: var(--ink);
}

.discovery-workbench ::selection,
.review-content ::selection {
  color: #171717;
  background: #e7f6a0;
}

.discovery-workbench :focus-visible,
.review-content :focus-visible {
  outline: 2px solid #667d00;
  outline-offset: 3px;
}

.discovery-workbench :deep(input),
.discovery-workbench :deep(textarea) {
  caret-color: #536700;
}

.discovery-heading,
.section-heading,
.candidate-toolbar,
.candidate-top,
.candidate-footer,
.history-heading,
.ai-controls,
.review-summary,
.review-actions {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.discovery-heading {
  margin-bottom: 24px;
}

h1,
h2,
h3,
p {
  margin: 0;
}

h1 {
  font-size: 30px;
  font-weight: 700;
  line-height: 1.4;
}

h2 {
  font-size: 19px;
  font-weight: 650;
  line-height: 1.4;
}

h3 {
  font-size: 15px;
  font-weight: 650;
  line-height: 1.5;
}

.discovery-heading p {
  margin-top: 8px;
  line-height: 1.6;
  color: var(--muted);
}

.market-label {
  display: inline-flex;
  flex-shrink: 0;
  gap: 7px;
  align-items: center;
  padding: 7px 10px;
  font-size: 12px;
  color: #44474d;
  background: #fff;
  border: 1px solid var(--line);
  border-radius: 5px;
}

.market-label :deep(svg) {
  font-size: 18px;
  color: #c52929;
}

.discovery-columns {
  display: grid;
  grid-template-columns: 350px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
}

.discovery-brief,
.discovery-results {
  min-width: 0;
  background: #fff;
  border: 1px solid var(--line);
  border-radius: 9px;
}

.discovery-brief {
  padding: 22px 18px;
}

.discovery-brief h2 {
  margin-bottom: 24px;
}

.discovery-brief form > label,
.ai-section > label {
  display: block;
  margin: 20px 0 9px;
  font-size: 13px;
  font-weight: 600;
}

.discovery-brief form > label:first-child {
  margin-top: 0;
}

.field-help,
.section-note {
  margin-top: 10px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--muted, #656971);
}

.create-action {
  width: 100%;
  margin-top: 24px;
}

.primary-action.el-button {
  --el-button-bg-color: #171717;
  --el-button-text-color: #fff;
  --el-button-border-color: #171717;
  --el-button-hover-bg-color: #343434;
  --el-button-hover-text-color: #fff;
  --el-button-hover-border-color: #343434;
  --el-button-disabled-bg-color: #e9e9e7;
  --el-button-disabled-border-color: #e9e9e7;
  --el-button-disabled-text-color: #747474;

  min-height: 38px;
}

.el-button :deep(svg) {
  margin-right: 5px;
}

.history-heading {
  padding-top: 20px;
  margin-top: 28px;
  border-top: 1px solid var(--line);
}

.history-select {
  width: 100%;
  margin-top: 8px;
}

.job-progress {
  padding: 24px;
  border-bottom: 1px solid var(--line);
}

.section-heading {
  align-items: flex-start;
}

.section-heading p {
  margin-top: 7px;
  line-height: 1.7;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.section-heading > .el-button {
  flex-shrink: 0;
}

.job-number {
  margin-left: 4px;
  font-size: 13px;
  font-weight: 400;
  color: var(--muted);
}

.flow-steps {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  padding: 0;
  margin: 24px 0 16px;
  list-style: none;
}

.flow-steps li {
  display: grid;
  grid-template-columns: 20px 1fr;
  gap: 4px 8px;
  align-items: center;
  padding-top: 12px;
  font-size: 12px;
  color: var(--muted);
  border-top: 2px solid #e4e4e4;
}

.flow-steps li.complete {
  color: #3f4c07;
  border-color: #667d00;
}

.flow-steps li > :deep(svg) {
  font-size: 18px;
}

.flow-steps strong {
  grid-column: 2;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.job-time {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  color: var(--muted);
}

.candidate-section {
  padding: 24px 24px 0;
}

.candidate-evidence {
  border-top: 1px solid var(--line);
}

.candidate-evidence > summary {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 20px 24px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  list-style: none;
}

.candidate-evidence > summary::-webkit-details-marker {
  display: none;
}

.candidate-evidence > summary:hover {
  background: #f7f8f3;
}

.candidate-evidence > summary > span {
  font-size: 12px;
  font-weight: 400;
  font-variant-numeric: tabular-nums;
  color: var(--muted);
}

.candidate-evidence > summary > :deep(svg) {
  margin-left: auto;
  font-size: 18px;
}

.candidate-evidence[open] > summary > :deep(svg) {
  transform: rotate(180deg);
}

.candidate-evidence .candidate-section {
  padding-top: 4px;
}

.filter-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
}

.filter-tabs button {
  padding: 6px 9px;
  font-size: 12px;
  color: var(--muted);
  cursor: pointer;
  background: transparent;
  border: 0;
  border-radius: 4px;
}

.filter-tabs button:hover {
  background: #f3f3f0;
}

.filter-tabs button.selected {
  color: #273000;
  background: #eaf5b9;
}

.candidate-row {
  padding: 24px 0;
  border-bottom: 1px solid var(--line);
}

.candidate-row:last-child {
  border-bottom: 0;
}

.candidate-top {
  gap: 12px;
  justify-content: flex-start;
}

.avatar {
  display: grid;
  flex: 0 0 44px;
  place-items: center;
  width: 44px;
  height: 44px;
  overflow: hidden;
  font-size: 24px;
  color: var(--muted);
  background: #f0f0ed;
  border-radius: 50%;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.candidate-identity {
  flex: 1;
  min-width: 0;
}

.candidate-identity h3 {
  overflow-wrap: anywhere;
}

.candidate-identity p {
  margin-top: 3px;
  font-size: 11px;
  color: var(--muted);
  overflow-wrap: anywhere;
}

a {
  color: inherit;
  text-decoration: none;
  text-underline-offset: 3px;
}

a[href]:hover {
  color: #526800;
  text-decoration: underline;
}

.review-status {
  flex-shrink: 0;
  padding: 4px 7px;
  font-size: 11px;
  color: #6e581e;
  background: #f3f1e7;
  border-radius: 4px;
}

.review-status.approved {
  color: #365212;
  background: #eaf2dc;
}

.review-status.rejected {
  color: #656971;
  background: #f0f0ee;
}

.candidate-description {
  display: -webkit-box;
  margin-top: 14px;
  overflow: hidden;
  -webkit-line-clamp: 2;
  line-height: 1.7;
  color: #555a61;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
}

.candidate-metrics {
  display: flex;
  flex-wrap: wrap;
  gap: 20px 32px;
  margin: 18px 0;
}

.candidate-metrics dt {
  font-size: 11px;
  color: var(--muted);
}

.candidate-metrics dd {
  margin: 5px 0 0;
  font-size: 14px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.post-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.post-preview {
  min-width: 0;
}

.post-image {
  display: grid;
  place-items: center;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  font-size: 25px;
  color: #73766e;
  background: #f2f2f0;
  border-radius: 5px;
}

.post-image img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.post-preview > span {
  display: -webkit-box;
  margin-top: 7px;
  overflow: hidden;
  -webkit-line-clamp: 2;
  font-size: 11px;
  line-height: 1.5;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
}

.candidate-warning {
  margin-top: 12px;
  font-size: 12px;
  line-height: 1.6;
  color: #795719;
}

.candidate-footer {
  margin-top: 17px;
}

.candidate-footer > span {
  font-size: 11px;
  color: var(--muted);
}

.candidate-footer :deep(.el-button) {
  font-size: 12px;
}

.candidate-footer :deep(svg) {
  margin: 0 0 0 5px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 220px;
  padding: 32px;
  text-align: center;
}

.empty-state > :deep(svg) {
  margin-bottom: 16px;
  font-size: 30px;
  color: #6f7466;
}

.empty-state p {
  max-width: 45ch;
  margin-top: 10px;
  font-size: 13px;
  line-height: 1.8;
  color: var(--muted);
}

.initial-empty {
  min-height: 490px;
}

.notice {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 14px 16px;
  margin: 16px 0;
  font-size: 13px;
  line-height: 1.7;
  color: #6d5720;
  overflow-wrap: anywhere;
  background: #f6f4eb;
  border-radius: 5px;
}

.discovery-results > .notice {
  margin: 16px;
}

.error-notice {
  color: #96372b;
  background: #fbefed;
}

.notice > div {
  flex: 1;
}

.notice > :deep(svg) {
  flex-shrink: 0;
  margin-top: 3px;
  font-size: 18px;
}

.ai-section {
  padding: 24px;
  border-top: 1px solid var(--line);
}

.ai-controls {
  margin-top: 14px;
}

.ai-controls p,
.ai-secondary {
  font-size: 12px;
  line-height: 1.7;
  color: var(--muted);
}

.ai-result {
  margin-top: 24px;
}

.model-label {
  display: flex;
  gap: 7px;
  align-items: center;
  font-size: 12px;
  color: #495b09;
  overflow-wrap: anywhere;
}

.recommendation-message {
  margin-top: 12px;
  line-height: 1.8;
}

.ai-candidate {
  padding: 20px 0;
  border-bottom: 1px solid var(--line);
}

.ai-candidate:last-child {
  border-bottom: 0;
}

.ai-candidate-heading {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 10px;
  align-items: center;
  min-width: 0;
}

.ai-card-title {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  min-width: 0;
}

.ai-card-name {
  min-width: 0;
}

.ai-source {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
  margin-top: 7px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.ai-source :deep(svg) {
  flex-shrink: 0;
  font-size: 15px;
}

.ai-posts {
  margin-top: 16px;
}

.ai-candidate-heading h3 {
  overflow-wrap: anywhere;
}

.ai-score {
  flex-shrink: 0;
  padding: 4px 7px;
  font-size: 12px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: #354506;
  background: #edf5cf;
  border-radius: 4px;
}

.ai-priority {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--muted);
}

.ai-candidate > p {
  margin-top: 9px;
  line-height: 1.8;
  overflow-wrap: anywhere;
}

.empty-recommendation {
  margin-top: 16px;
  line-height: 1.8;
  color: var(--muted);
}

.review-content {
  color: #171717;
}

.review-summary {
  flex-wrap: wrap;
  font-size: 13px;
}

.review-summary a {
  display: inline-flex;
  gap: 5px;
  align-items: center;
}

.review-description {
  margin-top: 20px;
  line-height: 1.8;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.review-content h3 {
  margin: 28px 0 12px;
  font-size: 16px;
}

.evidence-list {
  padding-left: 20px;
  margin: 0;
}

.evidence-list li {
  margin: 12px 0;
  line-height: 1.6;
  overflow-wrap: anywhere;
}

.evidence-list p {
  margin-top: 4px;
  font-size: 12px;
  color: #656971;
}

.review-post-list article {
  display: flex;
  gap: 14px;
  align-items: flex-start;
  padding: 13px 0;
  border-bottom: 1px solid #e8e8e8;
}

.review-post-image {
  display: grid;
  flex: 0 0 160px;
  place-items: center;
  width: 160px;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  font-size: 25px;
  color: #73766e;
  background: #f2f2f0;
  border-radius: 5px;
}

.review-post-image img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.review-post-list article > div {
  min-width: 0;
  font-size: 13px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}

.review-post-list p {
  margin-top: 6px;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  color: #656971;
}

.review-form {
  padding-top: 1px;
  margin-top: 28px;
  border-top: 1px solid #e8e8e8;
}

.review-form .el-form {
  margin-top: 20px;
}

.review-form-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.review-form :deep(.el-checkbox) {
  height: auto;
  white-space: normal;
}

.review-form :deep(.el-checkbox__label) {
  line-height: 1.6;
  white-space: normal;
}

.review-actions {
  gap: 8px;
  justify-content: flex-end;
}

.review-actions .el-button + .el-button {
  margin-left: 0;
}

.spinning {
  animation: discovery-spin 1.2s linear infinite;
}

@keyframes discovery-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .spinning {
    animation: none;
  }
}

@media (width <= 1200px) {
  .discovery-columns {
    grid-template-columns: 300px minmax(0, 1fr);
  }

  .candidate-toolbar {
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
  }

  .candidate-metrics {
    gap: 16px 22px;
  }
}

@media (width <= 900px) {
  .discovery-columns {
    grid-template-columns: minmax(0, 1fr);
  }

  .discovery-brief {
    padding: 22px;
  }

  .discovery-heading {
    align-items: flex-start;
  }

  .discovery-heading h1 {
    font-size: 25px;
  }
}

@media (width <= 600px) {
  .discovery-heading {
    flex-direction: column;
    gap: 12px;
  }

  .discovery-heading h1 {
    font-size: 24px;
  }

  .discovery-heading p {
    font-size: 13px;
  }

  .discovery-brief,
  .job-progress,
  .candidate-section,
  .ai-section {
    padding: 18px;
  }

  .candidate-section {
    padding-bottom: 0;
  }

  .section-heading,
  .ai-controls {
    flex-wrap: wrap;
  }

  .flow-steps {
    gap: 10px;
  }

  .flow-steps li {
    grid-template-columns: 1fr;
    font-size: 11px;
  }

  .flow-steps strong {
    grid-column: 1;
  }

  .candidate-metrics {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 14px;
  }

  .post-strip {
    gap: 7px;
  }

  .candidate-footer {
    flex-direction: column;
    gap: 10px;
    align-items: flex-start;
  }

  .candidate-footer .el-button {
    align-self: flex-end;
  }

  .candidate-top {
    flex-wrap: wrap;
    gap: 9px;
  }

  .candidate-identity {
    min-width: 120px;
  }

  .review-post-image {
    flex-basis: 112px;
    width: 112px;
  }

  .review-form-row {
    grid-template-columns: 1fr;
    gap: 0;
  }

  .review-actions {
    flex-wrap: wrap;
  }

  .review-actions .el-button {
    padding-right: 10px;
    padding-left: 10px;
  }
}
</style>
