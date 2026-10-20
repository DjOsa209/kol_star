<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { fieldLabel as t } from "@/utils/fieldI18n";
import {
  isMediaResource,
  matchScore,
  recommendationReasons,
  visibleRecommendations,
  type ResultFilter,
  type ResultSort
} from "./recommendationView";
import type { RecommendationSubscription } from "@/api/recommendations";

const props = defineProps<{
  form: {
    demandText: string;
    targetMarket: string;
    resourceType: string;
    platform: string;
    budget: number;
  };
  items: any[];
  selected: Record<number, boolean>;
  markets: string[];
  subscriptions: RecommendationSubscription[];
  subscriptionId: number | null;
  mode: "recent" | "saved";
  recentAvailable: boolean;
  savedView: "new" | "all";
  projects: any[];
  projectId: number | null;
  loading: boolean;
  savedLoading: boolean;
  saving: boolean;
  searchName: string;
  adding: boolean;
  attachments: any[];
  status: string;
  message: string;
  error: string;
  elapsed: string;
  works: Record<number, any[]>;
}>();
const emit = defineEmits<{
  updateForm: [value: Partial<typeof props.form>];
  generate: [];
  stop: [];
  save: [];
  searchNameChange: [name: string];
  clear: [];
  upload: [file: any];
  removeAttachment: [uid: string | number];
  marketChange: [market: string];
  removeMarket: [market: string, event: Event];
  selectSearch: [item: RecommendationSubscription];
  searchAction: [action: string, item: RecommendationSubscription];
  modeChange: [mode: "recent" | "saved"];
  savedViewChange: [view: "new" | "all"];
  projectChange: [id: number];
  toggle: [item: any];
  profile: [item: any];
  feedback: [id: number];
  add: [];
}>();
const filter = ref<ResultFilter>("all");
const showSave = ref(false);
const sort = ref<ResultSort>("match");
const brokenImages = reactive<Record<string, boolean>>({});
const items = computed(() =>
  visibleRecommendations(props.items, filter.value, sort.value)
);
const selectedItems = computed(() =>
  props.items.filter(item => props.selected[Number(item.id)])
);
const mediaCount = computed(() => props.items.filter(isMediaResource).length);
const filters = computed(() => [
  { key: "all" as const, label: "全部", count: props.items.length },
  {
    key: "creator" as const,
    label: "达人",
    count: props.items.length - mediaCount.value
  },
  { key: "media" as const, label: "媒体", count: mediaCount.value }
]);
const platforms = ["YouTube", "TikTok", "Instagram", "Newsletter", "Website"];
const money = (value: unknown) =>
  Number(value) > 0
    ? new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 }).format(
        Number(value)
      )
    : t("待确认");
const count = (value: unknown) =>
  Number(value) > 0
    ? new Intl.NumberFormat("en-US", {
        notation: "compact",
        maximumFractionDigits: 1
      }).format(Number(value))
    : "—";
const percent = (value: unknown) =>
  Number(value) > 0
    ? `${(Number(value) > 1 ? Number(value) : Number(value) * 100).toFixed(1)}%`
    : "—";
const initials = (name: string) =>
  String(name || "")
    .split(/\s+/)
    .slice(0, 2)
    .map(word => word[0])
    .join("");
function platformIcon(platform: string) {
  if (/youtube/i.test(platform)) return "ri:youtube-fill";
  if (/instagram/i.test(platform)) return "ri:instagram-line";
  if (/tiktok/i.test(platform)) return "ri:tiktok-fill";
  return "ri:global-line";
}
function pending(item: any) {
  return (
    [item.riskTip, ...(Array.isArray(item.downReasons) ? item.downReasons : [])]
      .filter(Boolean)
      .join("；") || t("档期与报价待确认")
  );
}
function scenarios(item: any): string {
  return Array.isArray(item.scenarios)
    ? item.scenarios
        .filter(value => typeof value === "string" && value.trim())
        .map(value => value.split(/[：:]/)[0].trim())
        .join(" · ")
    : "";
}
</script>

<template>
  <div class="recommendation-workbench">
    <header class="workbench-heading">
      <div>
        <h1>{{ t("达人/媒体 AI 推荐") }}</h1>
        <span class="library-label">{{ t("来自全球资源库") }}</span>
      </div>
      <span class="workbench-status" role="status">{{ t(status) }}</span>
    </header>

    <div class="workbench-columns">
      <aside class="brief-panel" :aria-label="t('寻找条件')">
        <div class="brief-heading">
          <h2>{{ t("寻找条件") }}</h2>
          <p>{{ t("描述推广需求，筛选资源库中的合适达人或媒体。") }}</p>
        </div>
        <form class="brief-form" @submit.prevent="emit('generate')">
          <label for="recommendation-demand" class="demand-label">{{
            t("推广需求")
          }}</label>
          <el-input
            id="recommendation-demand"
            :model-value="form.demandText"
            type="textarea"
            :rows="4"
            maxlength="1000"
            show-word-limit
            resize="vertical"
            :placeholder="t('输入市场、产品、预算、平台和投放目标...')"
            :disabled="loading"
            @update:model-value="emit('updateForm', { demandText: $event })"
          />
          <div class="brief-fields">
            <label class="brief-field" for="recommendation-market"
              ><span
                ><IconifyIconOnline icon="ri:global-line" />{{
                  t("市场")
                }}</span
              >
              <el-select
                id="recommendation-market"
                :model-value="form.targetMarket"
                :aria-label="t('市场')"
                clearable
                filterable
                allow-create
                default-first-option
                :placeholder="t('全部市场')"
                :disabled="loading"
                @update:model-value="
                  emit('updateForm', { targetMarket: $event })
                "
                @change="emit('marketChange', $event)"
              >
                <el-option
                  v-for="market in markets"
                  :key="market"
                  :label="t(market)"
                  :value="market"
                  ><div class="market-option">
                    <span>{{ t(market) }}</span
                    ><el-button
                      link
                      type="danger"
                      :aria-label="`${t('删除市场')} ${market}`"
                      @mousedown.stop
                      @click="emit('removeMarket', market, $event)"
                      >{{ t("删除") }}</el-button
                    >
                  </div></el-option
                >
              </el-select>
            </label>
            <label class="brief-field" for="recommendation-type"
              ><span
                ><IconifyIconOnline icon="ri:group-line" />{{
                  t("资源类型")
                }}</span
              >
              <el-select
                id="recommendation-type"
                :model-value="form.resourceType"
                :aria-label="t('资源类型')"
                clearable
                :placeholder="t('全部类型')"
                :disabled="loading"
                @update:model-value="
                  emit('updateForm', { resourceType: $event })
                "
                ><el-option
                  v-for="type in ['KOL', '创作者', '媒体']"
                  :key="type"
                  :label="t(type)"
                  :value="type"
              /></el-select>
            </label>
            <label class="brief-field" for="recommendation-platform"
              ><span
                ><IconifyIconOnline icon="ri:stack-line" />{{ t("平台") }}</span
              >
              <el-select
                id="recommendation-platform"
                :model-value="form.platform"
                :aria-label="t('平台')"
                clearable
                :placeholder="t('全部平台')"
                :disabled="loading"
                @update:model-value="emit('updateForm', { platform: $event })"
                ><el-option
                  v-for="platform in platforms"
                  :key="platform"
                  :label="platform"
                  :value="platform"
              /></el-select>
            </label>
            <label class="brief-field" for="recommendation-budget"
              ><span
                ><IconifyIconOnline icon="ri:money-dollar-circle-line" />{{
                  t("总预算")
                }}</span
              >
              <div class="budget-input">
                <span>USD</span
                ><el-input-number
                  id="recommendation-budget"
                  :model-value="form.budget"
                  :aria-label="t('总预算 USD')"
                  :controls="false"
                  :min="0"
                  :precision="0"
                  :disabled="loading"
                  @update:model-value="
                    emit('updateForm', { budget: $event ?? 0 })
                  "
                />
              </div>
            </label>
          </div>
          <el-upload
            class="brief-upload"
            accept=".txt,.md,.csv,.xlsx,.xls,.pdf,.doc,.docx"
            :auto-upload="false"
            :show-file-list="false"
            :disabled="loading"
            :on-change="file => emit('upload', file)"
          >
            <div class="upload-button">
              <IconifyIconOnline icon="ri:attachment-2" /><span
                ><strong
                  >{{ t("上传资料") }}
                  <small>{{ t("（可选）") }}</small></strong
                ><span>{{
                  t("支持文本与表格；PDF / Word 仅参考文件名")
                }}</span></span
              >
            </div>
          </el-upload>
          <ul v-if="attachments.length" class="brief-attachments">
            <li v-for="file in attachments" :key="file.uid">
              <span :title="file.summary">{{ file.name }}</span
              ><el-button
                link
                :disabled="loading"
                :aria-label="`${t('移除')} ${file.name}`"
                @click="emit('removeAttachment', file.uid)"
                ><IconifyIconOnline icon="ri:close-line"
              /></el-button>
            </li>
          </ul>
          <el-button
            v-if="loading"
            class="generate-button"
            native-type="button"
            @click="emit('stop')"
            ><IconifyIconOnline icon="ri:stop-circle-line" />{{ t("停止") }} ·
            {{ elapsed }}</el-button
          >
          <el-button v-else class="generate-button" native-type="submit"
            ><IconifyIconOnline icon="ri:sparkling-2-line" />{{
              t("生成推荐")
            }}</el-button
          >
          <el-input
            v-if="showSave"
            class="search-name-input"
            :model-value="searchName"
            :placeholder="t('寻找条件名称（选填）')"
            :aria-label="t('寻找条件名称（选填）')"
            :disabled="loading || saving"
            maxlength="60"
            @update:model-value="emit('searchNameChange', $event)"
          />
          <el-button
            v-if="showSave"
            class="save-search-button"
            :loading="saving"
            :disabled="loading || !form.demandText.trim()"
            @click="emit('save')"
            ><IconifyIconOnline icon="ri:bookmark-line" />{{
              t("保存寻找条件")
            }}</el-button
          >
        </form>

        <section class="searches-section">
          <h3>
            {{ t("已保存的条件")
            }}<button
              type="button"
              class="save-search-toggle"
              :disabled="loading || saving"
              :aria-expanded="showSave"
              @click="showSave = !showSave"
            >
              {{ t("保存寻找条件") }}
            </button>
          </h3>
          <p v-if="!subscriptions.length" class="searches-empty">
            {{ t("保存寻找条件后，每周检查库内新匹配。") }}
          </p>
          <article
            v-for="search in subscriptions"
            :key="search.id"
            class="saved-search"
            :class="{
              'is-current': mode === 'saved' && search.id === subscriptionId
            }"
          >
            <button
              type="button"
              class="search-select"
              :disabled="loading || savedLoading || saving || adding"
              :aria-pressed="mode === 'saved' && search.id === subscriptionId"
              @click="emit('selectSearch', search)"
            >
              <strong>{{ search.name }}</strong
              ><span>{{
                [
                  search.markets?.map(t).join(" / "),
                  search.platforms?.join(" / "),
                  search.budgetMax ? `USD ${money(search.budgetMax)}` : ""
                ]
                  .filter(Boolean)
                  .join(" · ") || t("查看已保存的推荐")
              }}</span
              ><small :class="{ paused: !search.active }"
                ><IconifyIconOnline
                  :icon="
                    search.active
                      ? 'ri:checkbox-circle-fill'
                      : 'ri:pause-circle-line'
                  "
                />{{ t(search.active ? "每周检查新匹配" : "已暂停") }}</small
              >
            </button>
            <el-dropdown
              trigger="click"
              :disabled="loading || savedLoading || saving || adding"
              @command="emit('searchAction', $event, search)"
              ><el-button
                text
                class="search-menu"
                :aria-label="`${t('管理寻找条件')} ${search.name}`"
                ><IconifyIconOnline icon="ri:more-2-fill" /></el-button
              ><template #dropdown
                ><el-dropdown-menu
                  ><el-dropdown-item command="refresh">{{
                    t("立即更新")
                  }}</el-dropdown-item
                  ><el-dropdown-item command="toggle">{{
                    t(search.active ? "暂停" : "恢复")
                  }}</el-dropdown-item
                  ><el-dropdown-item command="delete" divided>{{
                    t("删除")
                  }}</el-dropdown-item></el-dropdown-menu
                ></template
              ></el-dropdown
            >
          </article>
          <p v-if="subscriptions.length" class="weekly-note">
            {{ t("每周一 09:00（北京时间），有新匹配时通知。") }}
          </p>
        </section>
      </aside>

      <section
        class="results-panel"
        :aria-label="t('推荐结果')"
        :aria-busy="loading || savedLoading"
      >
        <div class="results-heading">
          <div>
            <h2>{{ t("推荐结果") }}</h2>
            <span>{{ items.length }} {{ t("位候选") }}</span>
          </div>
          <div
            v-if="recentAvailable && subscriptions.length"
            class="source-switch"
          >
            <button
              type="button"
              :class="{ active: mode === 'recent' }"
              :disabled="loading || savedLoading || adding"
              @click="emit('modeChange', 'recent')"
            >
              {{ t("本次推荐") }}</button
            ><button
              type="button"
              :class="{ active: mode === 'saved' }"
              :disabled="loading || savedLoading || adding"
              @click="emit('modeChange', 'saved')"
            >
              {{ t("已保存") }}
            </button>
          </div>
        </div>
        <div class="results-toolbar">
          <div class="result-tabs" role="tablist" :aria-label="t('资源类型')">
            <button
              v-for="tab in filters"
              :key="tab.key"
              type="button"
              role="tab"
              :aria-selected="filter === tab.key"
              :class="{ active: filter === tab.key }"
              @click="filter = tab.key"
            >
              {{ t(tab.label) }} <span>({{ tab.count }})</span>
            </button>
          </div>
          <el-select
            v-model="sort"
            :aria-label="t('排序方式')"
            class="result-sort"
            ><el-option value="match" :label="t('匹配度优先')" /><el-option
              value="cost"
              :label="t('预估费用从低到高')"
          /></el-select>
        </div>
        <div v-if="mode === 'saved'" class="saved-result-toolbar">
          <span>{{
            subscriptions.find(item => item.id === subscriptionId)?.name
          }}</span
          ><el-radio-group
            :model-value="savedView"
            size="small"
            :disabled="savedLoading || adding"
            @change="emit('savedViewChange', $event as 'new' | 'all')"
            ><el-radio-button value="all">{{ t("全部结果") }}</el-radio-button
            ><el-radio-button value="new">{{
              t("本周新推荐")
            }}</el-radio-button></el-radio-group
          >
        </div>
        <div
          v-if="loading || savedLoading"
          class="result-loading"
          role="status"
        >
          <el-skeleton :rows="5" animated />
          <p>
            {{
              t(loading ? "正在根据需求整理资源，请稍候…" : "正在读取推荐结果…")
            }}
          </p>
        </div>
        <el-alert
          v-if="error"
          :title="error"
          type="error"
          :closable="false"
          show-icon
          class="result-error"
        />
        <template v-if="!loading && !savedLoading">
          <div v-if="!items.length" class="results-empty">
            <IconifyIconOnline
              :icon="filter === 'all' ? 'ri:group-line' : 'ri:filter-3-line'"
            />
            <h3>
              {{
                t(
                  props.items.length
                    ? "当前分类暂无候选"
                    : "在这里发现合适的合作伙伴"
                )
              }}
            </h3>
            <p>
              {{
                t(
                  props.items.length
                    ? "切换到全部，查看其他类型的推荐。"
                    : "在左侧描述推广需求并生成推荐，或选择已保存的寻找条件。"
                )
              }}
            </p>
            <el-button v-if="props.items.length" @click="filter = 'all'">{{
              t("查看全部")
            }}</el-button>
          </div>
          <div v-else class="candidate-list">
            <article
              v-for="item in items"
              :key="item.id"
              class="candidate-row"
              :class="{ 'is-selected': selected[Number(item.id)] }"
            >
              <el-checkbox
                class="candidate-check"
                :model-value="Boolean(selected[Number(item.id)])"
                :aria-label="`${t('选择')} ${item.name}`"
                :disabled="adding"
                @change="emit('toggle', item)"
              />
              <div class="candidate-identity">
                <button
                  type="button"
                  class="candidate-profile"
                  @click="emit('profile', item)"
                >
                  <span class="candidate-avatar"
                    ><img
                      v-if="item.avatarUrl && !brokenImages[item.avatarUrl]"
                      :src="item.avatarUrl"
                      :alt="item.name"
                      loading="lazy"
                      @error="brokenImages[item.avatarUrl] = true"
                    /><span v-else>{{ initials(item.name) }}</span></span
                  ><span
                    ><strong>{{ item.name }}</strong
                    ><span class="candidate-platform"
                      ><IconifyIconOnline
                        :icon="platformIcon(item.platform || '')"
                        :class="{
                          youtube: /youtube/i.test(item.platform || '')
                        }"
                      />{{ item.platform || t("平台待补充")
                      }}<span>{{
                        t(isMediaResource(item) ? "媒体" : "达人")
                      }}</span></span
                    ></span
                  >
                </button>
                <p class="candidate-description">
                  {{
                    [
                      item.country,
                      item.industry || item.category,
                      item.language
                    ]
                      .filter(Boolean)
                      .map(t)
                      .join(" · ") || t("资源资料待补充")
                  }}
                </p>
                <div
                  v-if="
                    works[Number(item.id)]?.some(
                      work => work.coverUrl && !brokenImages[work.coverUrl]
                    )
                  "
                  class="candidate-works"
                >
                  <button
                    v-for="work in works[Number(item.id)]
                      .filter(
                        work => work.coverUrl && !brokenImages[work.coverUrl]
                      )
                      .slice(0, 3)"
                    :key="work.id"
                    type="button"
                    :aria-label="`${t('查看作品')} ${work.title || item.name}`"
                    @click="emit('profile', item)"
                  >
                    <img
                      :src="work.coverUrl"
                      :alt="work.title || t('合作作品')"
                      loading="lazy"
                      @error="brokenImages[work.coverUrl] = true"
                    />
                  </button>
                </div>
                <button
                  v-else
                  type="button"
                  class="view-content"
                  @click="emit('profile', item)"
                >
                  {{ t("查看内容与合作记录")
                  }}<IconifyIconOnline icon="ri:arrow-right-up-line" />
                </button>
              </div>
              <div class="candidate-metrics">
                <div class="performance-metrics">
                  <div>
                    <span>{{
                      t(isMediaResource(item) ? "平均阅读 / 播放" : "粉丝数")
                    }}</span
                    ><strong>{{
                      count(
                        isMediaResource(item) ? item.avgViews : item.followers
                      )
                    }}</strong>
                  </div>
                  <div>
                    <span>{{ t("互动率") }}</span
                    ><strong>{{ percent(item.engagementRate) }}</strong>
                  </div>
                </div>
                <div class="candidate-cost">
                  <span
                    >{{ t("预估费用") }} ({{
                      item.estimatedCostCurrency || "USD"
                    }})</span
                  ><strong
                    >{{ money(item.estimatedCost)
                    }}<el-tooltip :content="t('预估费用需最终报价确认')"
                      ><IconifyIconOnline
                        icon="ri:information-line" /></el-tooltip
                  ></strong>
                </div>
              </div>
              <div class="candidate-score">
                <span>{{ t("匹配度") }}</span
                ><strong
                  >{{ matchScore(item) ?? "—" }}<small>/100</small></strong
                ><el-progress
                  v-if="matchScore(item) !== null"
                  :percentage="matchScore(item)!"
                  :stroke-width="5"
                  :show-text="false"
                  color="#b6df00"
                /><span class="score-origin">{{
                  t(
                    item.source === "ai"
                      ? "AI 排序"
                      : item.source === "rule"
                        ? "规则匹配"
                        : "匹配评分"
                  )
                }}</span>
              </div>
              <div class="candidate-evidence">
                <h3>{{ t("匹配理由") }}</h3>
                <ul v-if="recommendationReasons(item).length">
                  <li
                    v-for="(reason, index) in recommendationReasons(item).slice(
                      0,
                      3
                    )"
                    :key="index"
                  >
                    {{ reason }}
                  </li>
                </ul>
                <p v-else>{{ t("匹配依据待补充") }}</p>
                <details
                  v-if="recommendationReasons(item).length > 3"
                  class="evidence-details"
                >
                  <summary>{{ t("更多匹配依据") }}</summary>
                  <ul>
                    <li
                      v-for="(reason, index) in recommendationReasons(
                        item
                      ).slice(3)"
                      :key="index"
                    >
                      {{ reason }}
                    </li>
                  </ul>
                </details>
                <h3>{{ t("合作切入点") }}</h3>
                <p class="collaboration-summary">
                  {{
                    scenarios(item) || item.advice || t("查看档案评估合作方向")
                  }}
                </p>
                <h3>{{ t("待确认") }}</h3>
                <p>{{ pending(item) }}</p>
                <details
                  v-if="item.advice || scenarios(item)"
                  class="evidence-details"
                >
                  <summary>{{ t("查看完整合作建议") }}</summary>
                  <p
                    v-for="(scenario, index) in Array.isArray(item.scenarios)
                      ? item.scenarios
                      : []"
                    :key="index"
                  >
                    {{ scenario }}
                  </p>
                  <p v-if="item.advice" class="collaboration-advice">
                    {{ item.advice }}
                  </p>
                  <small>{{
                    t(
                      item.adviceSource === "ai"
                        ? "AI 合作建议"
                        : item.adviceSource === "rule"
                          ? "规则合作建议"
                          : "合作建议"
                    )
                  }}</small>
                </details>
              </div>
              <div class="candidate-actions">
                <el-button @click="emit('profile', item)">{{
                  t("查看档案")
                }}</el-button
                ><el-dropdown
                  v-if="item.resultId"
                  trigger="click"
                  @command="emit('feedback', item.resultId)"
                  ><el-button text :aria-label="`${t('更多操作')} ${item.name}`"
                    ><IconifyIconOnline icon="ri:more-fill" /></el-button
                  ><template #dropdown
                    ><el-dropdown-menu
                      ><el-dropdown-item command="not_interested">{{
                        t("不感兴趣")
                      }}</el-dropdown-item></el-dropdown-menu
                    ></template
                  ></el-dropdown
                >
              </div>
            </article>
          </div>
          <p v-if="message && mode === 'recent'" class="result-note">
            {{ message }}
          </p>
          <el-button
            v-if="mode === 'recent' && recentAvailable"
            class="clear-results"
            text
            :disabled="adding"
            @click="emit('clear')"
            >{{ t("清空本次推荐") }}</el-button
          >
        </template>
      </section>
    </div>

    <footer class="selection-bar">
      <div class="selection-summary" aria-live="polite">
        <strong
          >{{ t("已选") }} <em>{{ selectedItems.length }}</em>
          {{ t("位") }}</strong
        >
        <div class="selection-chips">
          <button
            v-for="item in selectedItems"
            :key="item.id"
            type="button"
            :disabled="adding"
            :aria-label="`${t('取消选择')} ${item.name}`"
            @click="emit('toggle', item)"
          >
            <img
              v-if="item.avatarUrl && !brokenImages[item.avatarUrl]"
              :src="item.avatarUrl"
              alt=""
              @error="brokenImages[item.avatarUrl] = true"
            /><span>{{ item.name }}</span
            ><IconifyIconOnline icon="ri:close-line" />
          </button>
        </div>
      </div>
      <div class="selection-actions">
        <label for="recommendation-campaign">{{ t("加入到 Campaign") }}</label
        ><el-select
          id="recommendation-campaign"
          :model-value="projectId"
          :aria-label="t('选择 Campaign')"
          :placeholder="t('选择 Campaign')"
          filterable
          :disabled="adding"
          @update:model-value="emit('projectChange', $event)"
          ><el-option
            v-for="project in projects"
            :key="project.id"
            :label="project.name"
            :value="project.id" /></el-select
        ><el-button
          class="add-campaign-button"
          :loading="adding"
          :disabled="
            !selectedItems.length || !projectId || loading || savedLoading
          "
          @click="emit('add')"
          >{{ t("加入 Campaign") }}</el-button
        >
      </div>
      <p class="selection-note">
        {{ t("推荐来自现有资源库 · 费用与档期待确认") }}
      </p>
    </footer>
  </div>
</template>

<style scoped>
.recommendation-workbench {
  --accent: #d4fb00;
  --ink: #171717;
  --muted: #72757c;
  --line: #e8e8e8;

  font-size: 14px;
  color: var(--ink);
}

.workbench-heading,
.workbench-heading > div,
.results-heading,
.results-heading > div,
.results-toolbar,
.selection-bar,
.selection-summary,
.selection-actions {
  display: flex;
  align-items: center;
}

.workbench-heading {
  gap: 16px;
  justify-content: space-between;
  margin-bottom: 24px;
}

.workbench-heading > div {
  flex-wrap: wrap;
  gap: 18px;
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
}

.library-label {
  padding: 5px 12px;
  font-size: 12px;
  color: #51545b;
  background: #ececeb;
  border-radius: 5px;
}

.workbench-status {
  font-size: 12px;
  color: var(--muted);
  white-space: nowrap;
}

.workbench-columns {
  display: grid;
  grid-template-columns: 350px minmax(0, 1fr);
  gap: 12px;
  align-items: stretch;
}

.brief-panel,
.results-panel {
  min-width: 0;
  background: #fff;
  border: 1px solid var(--line);
  border-radius: 9px;
  box-shadow: none;
}

.brief-panel {
  padding: 22px 18px;
}

.brief-heading p {
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--muted);
}

.brief-form {
  margin-top: 24px;
}

.demand-label {
  display: block;
  margin-bottom: 8px;
  font-weight: 600;
}

.brief-form :deep(.el-textarea__inner) {
  min-height: 138px;
  padding: 12px;
  font-size: 14px;
  line-height: 1.8;
}

.brief-fields {
  display: grid;
  gap: 20px;
  margin: 26px 0 24px;
}

.brief-field {
  display: grid;
  grid-template-columns: 86px minmax(0, 1fr);
  gap: 8px;
  align-items: center;
}

.brief-field > span {
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 13px;
  white-space: nowrap;
}

.brief-field > span svg {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  color: #737780;
}

.brief-field :deep(.el-select__wrapper),
.budget-input {
  min-height: 36px;
}

.budget-input {
  display: flex;
  align-items: center;
  overflow: hidden;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
}

.budget-input > span {
  padding: 0 10px;
  font-size: 12px;
  border-right: 1px solid var(--line);
}

.budget-input :deep(.el-input-number) {
  width: 100%;
  min-width: 0;
}

.budget-input :deep(.el-input__wrapper) {
  padding: 0 8px;
  box-shadow: none;
}

.budget-input :deep(.el-input__inner) {
  text-align: left;
}

.budget-input:focus-within {
  outline: 2px solid #6a741f;
  outline-offset: 2px;
}

.brief-upload,
.brief-upload :deep(.el-upload) {
  display: block;
  width: 100%;
}

.upload-button {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  width: 100%;
  padding: 13px 12px;
  text-align: left;
  cursor: pointer;
  background: #fff;
  border: 1px solid #dcdfe6;
  border-radius: 6px;
}

.upload-button > svg {
  flex-shrink: 0;
  width: 21px;
  height: 21px;
}

.upload-button > span {
  display: grid;
  gap: 5px;
}

.upload-button strong {
  font-size: 13px;
  font-weight: 600;
}

.upload-button small {
  font-weight: 400;
}

.upload-button > span > span {
  font-size: 11px;
  line-height: 1.6;
  color: var(--muted);
}

.brief-attachments {
  padding: 0;
  margin: 10px 0;
  list-style: none;
}

.brief-attachments li {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}

.brief-attachments li > span {
  overflow-wrap: anywhere;
}

.generate-button {
  width: 100%;
  min-height: 42px;
  margin: 20px 0 0;
  font-weight: 600;
  color: #fff;
  background: #171717;
  border-color: #171717;
}

.generate-button:hover,
.generate-button:focus-visible {
  color: #fff;
  background: #353535;
  border-color: #353535;
}

.generate-button svg {
  margin-right: 8px;
  color: var(--accent);
}

.search-name-input {
  margin-top: 12px;
}

.save-search-button {
  width: 100%;
  min-height: 34px;
  margin: 8px 0 0 !important;
  font-size: 12px;
}

.save-search-button svg {
  margin-right: 6px;
}

.searches-section {
  padding-top: 22px;
  margin-top: 22px;
  border-top: 1px solid var(--line);
}

.searches-section h3 {
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
  font-size: 13px;
  font-weight: 650;
}

.save-search-toggle {
  padding: 0;
  font-size: 11px;
  font-weight: 400;
  color: #626b42;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.searches-section h3 > span {
  font-weight: 400;
  color: var(--muted);
}

.saved-search {
  position: relative;
  margin-top: 9px;
  border: 1px solid var(--line);
  border-radius: 6px;
}

.saved-search.is-current {
  background: #fafbf4;
  border-color: #a3b659;
}

.search-select {
  display: grid;
  gap: 7px;
  width: 100%;
  padding: 13px 34px 13px 12px;
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.search-select > strong {
  font-size: 13px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.search-select > span {
  font-size: 11px;
  line-height: 1.6;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.search-select small {
  display: flex;
  gap: 6px;
  align-items: center;
  font-size: 11px;
  color: #526328;
}

.search-select small.paused {
  color: var(--muted);
}

.saved-search > :deep(.el-dropdown) {
  position: absolute;
  top: 5px;
  right: 1px;
}

.search-menu {
  width: 32px;
  padding: 4px;
}

.weekly-note,
.searches-empty {
  margin-top: 12px;
  font-size: 11px;
  line-height: 1.7;
  color: var(--muted);
}

.results-panel {
  padding: 22px 14px 12px;
}

.results-heading {
  gap: 12px;
  justify-content: space-between;
  padding: 0 4px;
  margin-bottom: 18px;
}

.results-heading > div {
  gap: 12px;
}

.results-heading > div > span {
  font-size: 12px;
  color: var(--muted);
}

.source-switch {
  display: flex;
  gap: 2px !important;
  padding: 2px;
  background: #f5f5f4;
  border-radius: 5px;
}

.source-switch button {
  padding: 5px 7px;
  font-size: 11px;
  color: var(--muted);
  cursor: pointer;
  background: transparent;
  border: 0;
}

.source-switch button.active {
  color: var(--ink);
  background: #fff;
  border-radius: 3px;
}

.results-toolbar {
  gap: 14px;
  justify-content: space-between;
  margin: 0 4px 12px;
  border-bottom: 1px solid var(--line);
}

.result-tabs {
  display: flex;
  gap: 22px;
}

.result-tabs > button {
  position: relative;
  padding: 10px 2px 14px;
  font-size: 13px;
  color: #6c7079;
  white-space: nowrap;
  cursor: pointer;
  background: none;
  border: 0;
}

.result-tabs > button.active {
  font-weight: 600;
  color: var(--ink);
}

.result-tabs > button.active::after {
  position: absolute;
  bottom: 0;
  left: 0;
  width: 100%;
  height: 2px;
  content: "";
  background: var(--ink);
}

.result-sort {
  flex: none;
  width: 145px;
  margin-bottom: 5px;
}

.result-sort :deep(.el-select__wrapper) {
  font-size: 12px;
}

.saved-result-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 0 6px 12px;
  font-size: 12px;
  color: var(--muted);
}

.saved-result-toolbar > span {
  overflow-wrap: anywhere;
}

.candidate-row {
  position: relative;
  display: grid;
  grid-template-columns:
    20px minmax(150px, 1.2fr) minmax(110px, 0.75fr)
    70px minmax(160px, 1fr) 82px;
  gap: 14px;
  align-items: start;
  padding: 16px 12px;
  border: 1px solid transparent;
  border-bottom-color: var(--line);
  border-radius: 6px;
}

.candidate-row.is-selected {
  background: #fbfdf2;
  border-color: #b6df00;
}

.candidate-row + .candidate-row {
  margin-top: 4px;
}

.candidate-check {
  height: 30px;
  margin: 0;
}

.candidate-check :deep(.el-checkbox__inner) {
  width: 17px;
  height: 17px;
  border-radius: 4px;
}

.candidate-check :deep(.el-checkbox__input.is-checked .el-checkbox__inner) {
  background: var(--accent);
  border-color: #a5c700;
}

.candidate-check :deep(.el-checkbox__inner::after) {
  border-color: #171717;
}

.candidate-identity,
.candidate-metrics,
.candidate-evidence {
  min-width: 0;
}

.candidate-profile {
  display: flex;
  gap: 12px;
  align-items: center;
  width: 100%;
  padding: 0;
  text-align: left;
  cursor: pointer;
  background: none;
  border: 0;
}

.candidate-avatar {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 54px;
  height: 54px;
  overflow: hidden;
  color: #666b59;
  background: #f0f0ed;
  border-radius: 50%;
}

.candidate-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.candidate-profile > span:last-child {
  min-width: 0;
}

.candidate-profile strong {
  display: block;
  font-size: 16px;
  font-weight: 650;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.candidate-platform {
  display: flex;
  flex-wrap: wrap;
  gap: 3px 5px;
  align-items: center;
  margin-top: 5px;
  font-size: 11px;
  color: #6e727a;
}

.candidate-platform svg {
  flex: none;
  width: 13px;
  height: 13px;
}

.candidate-platform svg.youtube {
  color: #e12725;
}

.candidate-description {
  margin: 13px 0 10px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.candidate-works {
  display: flex;
  gap: 5px;
}

.candidate-works button {
  flex: 1;
  min-width: 0;
  padding: 0;
  cursor: pointer;
  background: none;
  border: 0;
}

.candidate-works img {
  width: 100%;
  height: 42px;
  object-fit: cover;
  border-radius: 4px;
}

.view-content {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  padding: 0;
  font-size: 11px;
  color: #5c6640;
  cursor: pointer;
  background: none;
  border: 0;
}

.candidate-metrics {
  padding-left: 13px;
  border-left: 1px solid var(--line);
}

.performance-metrics {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.performance-metrics span,
.candidate-cost > span,
.candidate-score > span:first-child {
  display: block;
  font-size: 11px;
  line-height: 1.6;
  color: var(--muted);
}

.performance-metrics strong {
  display: block;
  margin-top: 4px;
  font-size: 14px;
  font-weight: 550;
  font-variant-numeric: tabular-nums;
  line-height: 1.7;
}

.candidate-cost {
  margin-top: 26px;
}

.candidate-cost > strong {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-top: 5px;
  font-size: 14px;
  font-weight: 550;
  font-variant-numeric: tabular-nums;
}

.candidate-cost svg {
  width: 13px;
  height: 13px;
  color: #777b82;
}

.candidate-score {
  padding-left: 10px;
  border-left: 1px solid var(--line);
}

.candidate-score > strong {
  display: block;
  margin: 4px 0 8px;
  font-size: 24px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.candidate-score small {
  font-size: 10px;
  font-weight: 400;
  color: var(--muted);
}

.score-origin {
  display: inline-block;
  padding: 3px 5px;
  margin-top: 10px;
  font-size: 10px;
  line-height: 1.4;
  color: #586439;
  background: #f0f3e4;
  border-radius: 3px;
}

.candidate-evidence {
  padding-left: 13px;
  border-left: 1px solid var(--line);
}

.candidate-evidence h3 {
  margin: 0 0 6px;
  font-size: 11px;
  font-weight: 550;
  color: #62656b;
}

.candidate-evidence h3:not(:first-child) {
  margin-top: 10px;
}

.candidate-evidence ul {
  padding-left: 14px;
  margin: 0;
}

.candidate-evidence li,
.candidate-evidence p {
  font-size: 13px;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.candidate-evidence > p {
  color: #686c74;
}

.candidate-evidence small {
  display: block;
  margin-top: 6px;
  font-size: 10px;
  color: var(--muted);
}

.collaboration-advice {
  margin-top: 10px;
}

.collaboration-summary {
  display: -webkit-box;
  overflow: hidden;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.evidence-details {
  margin-top: 8px;
}

.evidence-details summary {
  font-size: 11px;
  line-height: 1.6;
  color: #5c6640;
  cursor: pointer;
}

.evidence-details summary:focus-visible {
  outline: 2px solid #65742c;
  outline-offset: 3px;
}

.evidence-details[open] p,
.evidence-details[open] ul {
  margin-top: 8px;
}

.candidate-actions {
  display: flex;
  gap: 2px;
  align-items: center;
}

.candidate-actions :deep(.el-button) {
  padding: 8px 10px;
  margin: 0;
  font-size: 11px;
}

.candidate-actions :deep(.el-button.is-text) {
  padding: 4px;
}

.candidate-actions :deep(.el-dropdown) {
  position: absolute;
  right: 6px;
  bottom: 12px;
}

.result-loading {
  padding: 30px 12px;
}

.result-loading p {
  margin-top: 22px;
  color: var(--muted);
}

.result-error {
  margin: 12px 0;
}

.results-empty {
  display: grid;
  align-content: center;
  justify-items: center;
  min-height: 460px;
  padding: 40px 24px;
  text-align: center;
}

.results-empty > svg {
  width: 36px;
  height: 36px;
  margin-bottom: 20px;
  color: #78815d;
}

.results-empty h3 {
  font-size: 18px;
  font-weight: 600;
}

.results-empty p {
  max-width: 350px;
  margin: 12px 0 18px;
  font-size: 13px;
  line-height: 1.8;
  color: var(--muted);
}

.result-note {
  margin: 18px 8px 0;
  font-size: 11px;
  line-height: 1.8;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.clear-results {
  margin-top: 8px;
  font-size: 11px;
}

.selection-bar {
  position: sticky;
  bottom: 0;
  z-index: 9;
  flex-wrap: wrap;
  gap: 10px 20px;
  justify-content: space-between;
  padding: 15px 8px 8px;
  margin-top: 12px;
  background: #fff;
  border-top: 1px solid var(--line);
}

.selection-summary {
  flex: 1;
  gap: 18px;
  min-width: 0;
}

.selection-summary > strong {
  flex: none;
  font-size: 14px;
  font-weight: 600;
}

.selection-summary em {
  padding: 0 4px;
  font-style: normal;
  color: #5d7100;
}

.selection-chips {
  display: flex;
  gap: 6px;
  min-width: 0;
  max-width: 450px;
  overflow: auto;
}

.selection-chips button {
  display: inline-flex;
  flex: none;
  gap: 6px;
  align-items: center;
  max-width: 185px;
  padding: 4px 7px;
  font-size: 11px;
  cursor: pointer;
  background: #f7f7f6;
  border: 1px solid var(--line);
  border-radius: 5px;
}

.selection-chips button > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.selection-chips img {
  width: 25px;
  height: 25px;
  object-fit: cover;
  border-radius: 50%;
}

.selection-chips svg {
  flex: none;
}

.selection-actions {
  flex: none;
  gap: 12px;
}

.selection-actions label {
  font-size: 12px;
  color: var(--muted);
}

.selection-actions > :deep(.el-select) {
  width: 185px;
}

.add-campaign-button {
  min-height: 38px;
  padding: 8px 20px;
  font-weight: 600;
  color: #171717;
  background: var(--accent);
  border-color: var(--accent);
}

.add-campaign-button:hover {
  color: #171717;
  background: #c4e900;
  border-color: #c4e900;
}

.add-campaign-button.is-disabled {
  color: #777b67;
  background: #edf3cc;
  border-color: #e2e7d2;
}

.selection-note {
  width: 100%;
  font-size: 10px;
  color: var(--muted);
  text-align: right;
}

.market-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

button:focus-visible {
  outline: 2px solid #65742c;
  outline-offset: 3px;
}

button:disabled {
  cursor: not-allowed;
}

@media (width <= 1400px) {
  .workbench-columns {
    grid-template-columns: 300px minmax(0, 1fr);
  }

  .candidate-row {
    grid-template-columns: 18px minmax(135px, 1fr) 116px 66px minmax(150px, 1fr);
    gap: 10px;
    padding: 20px 8px;
  }

  .candidate-actions {
    grid-column: 2 / -1;
    justify-content: flex-end;
  }

  .candidate-actions :deep(.el-dropdown) {
    position: static;
  }

  .candidate-avatar {
    width: 46px;
    height: 46px;
  }

  .candidate-profile {
    gap: 8px;
  }

  .selection-actions label {
    display: none;
  }
}

@media (width <= 1100px) {
  .workbench-columns {
    grid-template-columns: 280px minmax(0, 1fr);
  }

  .brief-panel {
    padding: 18px 14px;
  }

  .candidate-row {
    grid-template-columns: 20px minmax(0, 1fr) 70px;
    gap: 12px;
  }

  .candidate-identity {
    grid-column: 2;
  }

  .candidate-score {
    grid-row: 1;
    grid-column: 3;
  }

  .candidate-metrics {
    display: flex;
    grid-column: 2 / -1;
    gap: 25px;
    padding: 0;
    border-left: 0;
  }

  .performance-metrics {
    gap: 25px;
  }

  .candidate-cost {
    margin-top: 0;
  }

  .candidate-evidence {
    grid-column: 2 / -1;
    padding: 0;
    border: 0;
  }

  .candidate-actions {
    grid-column: 2 / -1;
  }

  .source-switch {
    flex-wrap: wrap;
  }

  .results-toolbar {
    flex-wrap: wrap;
    gap: 5px;
  }

  .result-tabs {
    gap: 14px;
  }

  .result-sort {
    width: 100%;
  }

  .selection-actions > :deep(.el-select) {
    width: 160px;
  }
}

@media (width <= 760px) {
  .workbench-heading {
    align-items: flex-start;
    margin-bottom: 18px;
  }

  .workbench-heading > div {
    gap: 8px;
  }

  h1 {
    font-size: 22px;
  }

  .workbench-status {
    padding-top: 6px;
  }

  .workbench-columns {
    grid-template-columns: minmax(0, 1fr);
  }

  .brief-panel,
  .results-panel {
    padding: 18px 14px;
  }

  .brief-form {
    margin-top: 18px;
  }

  .brief-fields {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
    margin: 20px 0;
  }

  .brief-field {
    grid-template-columns: minmax(0, 1fr);
  }

  .searches-section {
    padding-top: 18px;
    margin-top: 18px;
  }

  .results-toolbar {
    flex-wrap: nowrap;
  }

  .result-sort {
    width: 130px;
  }

  .saved-result-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .selection-bar {
    padding: 12px 0 8px;
  }

  .selection-summary {
    flex-basis: 100%;
  }

  .selection-actions {
    gap: 8px;
    width: 100%;
  }

  .selection-actions > :deep(.el-select) {
    flex: 1;
    width: auto;
    min-width: 0;
  }

  .add-campaign-button {
    padding: 8px 12px;
  }

  .selection-note {
    text-align: left;
  }

  .results-empty {
    min-height: 300px;
  }

  .search-select {
    min-height: 80px;
  }

  .search-menu {
    min-height: 44px;
  }
}

@media (width <= 480px) {
  .results-toolbar {
    flex-wrap: wrap;
    gap: 8px;
  }

  .result-tabs {
    justify-content: space-between;
    width: 100%;
  }

  .result-sort {
    width: 100%;
    margin-bottom: 10px;
  }
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    transition: none !important;
    animation: none !important;
  }
}
</style>
