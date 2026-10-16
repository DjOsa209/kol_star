<script setup lang="ts">
import { useIPLocale } from "@/utils/ipI18n";
const { ipText } = useIPLocale();

import { computed, ref, watch } from "vue";
import type { IPExternalResearch } from "@/api/recommendations";

const props = defineProps<{ research?: IPExternalResearch; busy: boolean }>();
const emit = defineEmits<{
  ask: [message: string];
  "compare-change": [count: number];
}>();
const selected = ref<string[]>([]);
const candidates = computed(() => props.research?.candidates || []);
const comparison = computed(() =>
  candidates.value.filter(item => selected.value.includes(item.id))
);
const scope = computed(() => {
  const brief = props.research?.brief;
  if (!brief) return "";
  return [
    brief.topic,
    brief.markets?.join(" / ") || "市场待定",
    brief.goal || "用途待定"
  ]
    .filter(Boolean)
    .map(value => ipText(value))
    .join(" · ");
});
watch(
  () => props.research,
  () => {
    selected.value = selected.value.filter(id =>
      candidates.value.some(item => item.id === id)
    );
  }
);
watch(
  () => comparison.value.length,
  count => emit("compare-change", count),
  { immediate: true }
);
function safeURL(value: string) {
  try {
    const url = new URL(value);
    return ["https:", "http:"].includes(url.protocol) &&
      !url.username &&
      !url.password
      ? url.href
      : undefined;
  } catch {
    return undefined;
  }
}
function toggle(id: string) {
  selected.value = selected.value.includes(id)
    ? selected.value.filter(value => value !== id)
    : [...selected.value, id];
}
</script>

<template>
  <section class="external-research" :aria-label="ipText('库外 IP 研究')">
    <template v-if="!research">
      <h3>{{ ipText("发现库外合作候选") }}</h3>
      <p>
        {{ ipText("开启联网搜索，告诉我想找的 IP 方向、目标市场和合作用途。") }}
      </p>
      <p class="muted">
        {{ ipText("每个候选都会附上身份来源与待核实事项。") }}
      </p>
    </template>
    <template v-else>
      <p v-if="scope" class="scope">{{ scope }}</p>
      <template v-if="research.phase === 'clarify'">
        <h3>{{ ipText("先明确研究范围") }}</h3>
        <p>
          {{ ipText("在左侧补充市场与用途，已有条件会沿用，预算可以待定。") }}
        </p>
        <p class="muted">{{ ipText("暂时未定时，可回复“先探索”。") }}</p>
      </template>
      <template v-else-if="!research.phase">
        <h3>{{ ipText("历史网页线索") }}</h3>
        <p>{{ ipText("这轮记录还未整理为具体 IP 候选，可以重新研究。") }}</p>
        <details>
          <summary>
            {{ ipText("查看原始来源 ·") }} {{ ipText(research.results.length) }}
          </summary>
          <a
            v-for="source in research.results"
            :key="source.url"
            :href="safeURL(source.url)"
            target="_blank"
            rel="noopener noreferrer"
            >{{ source.title }}</a
          >
        </details>
        <button
          :disabled="busy"
          @click="emit('ask', ipText('重新研究这次需求的具体IP候选'))"
        >
          {{ ipText("重新研究") }}
        </button>
      </template>
      <template v-else>
        <div class="research-summary">
          <h3>{{ ipText("具体候选 ·") }} {{ ipText(candidates.length) }}</h3>
          <span>{{ ipText(research.rounds || 0) }} {{ ipText("轮研究") }}</span>
        </div>
        <p v-if="research.phase === 'partial'" class="muted" role="status">
          {{ ipText("本轮研究未全部完成，以下保留已核对的候选。") }}
        </p>
        <p v-if="!candidates.length">
          {{
            ipText("本轮没有证据充分的候选，请查看研究缺口后调整范围或重试。")
          }}
        </p>
        <p v-else class="muted">
          {{ ipText("勾选 2–3 个候选，可在下方对比资料。") }}
        </p>
        <article v-for="item in candidates" :key="item.id" class="candidate">
          <div class="candidate-heading">
            <h4>
              {{ item.name }} <span>{{ ipText(item.kind) }}</span>
            </h4>
            <label
              ><input
                type="checkbox"
                :checked="selected.includes(item.id)"
                :disabled="!selected.includes(item.id) && selected.length >= 3"
                :aria-label="ipText('将{0}加入库外对比', [item.name])"
                @change="toggle(item.id)"
              />{{ ipText("对比") }}</label
            >
          </div>
          <p class="identity">{{ item.identity }}</p>
          <dl>
            <dt>{{ ipText("推荐理由") }}</dt>
            <dd>{{ item.reason }}</dd>
            <dt>{{ ipText("合作切入") }}</dt>
            <dd>{{ ipText(item.entryPoints.join("；")) }}</dd>
            <dt>{{ ipText("待核实") }}</dt>
            <dd class="muted">{{ ipText(item.pending.join("；")) }}</dd>
          </dl>
          <details>
            <summary>
              {{ ipText("身份依据与来源 ·") }}
              {{ ipText(item.evidence.length) }}
            </summary>
            <blockquote>{{ item.identityQuote }}</blockquote>
            <div
              v-for="source in item.evidence"
              :key="source.url"
              class="source"
            >
              <a
                :href="safeURL(source.url)"
                target="_blank"
                rel="noopener noreferrer"
                >{{ source.title }}</a
              >
              <small
                >{{ ipText(source.sourceName || source.source)
                }}<template v-if="source.publishedAt">
                  · {{ ipText(source.publishedAt) }}</template
                ></small
              >
            </div>
          </details>
          <button
            class="text-action"
            :disabled="busy"
            @click="
              emit(
                'ask',
                ipText('继续补查「{0}」的合作依据与目标市场适配', [item.name])
              )
            "
          >
            {{ ipText("继续补查") }}
          </button>
        </article>
        <section
          v-if="comparison.length >= 2"
          class="comparison"
          :aria-label="ipText('库外候选对比')"
        >
          <h3>{{ ipText("候选对比 ·") }} {{ ipText(comparison.length) }}</h3>
          <div
            class="table-scroll"
            tabindex="0"
            :aria-label="ipText('可横向滚动的对比表')"
          >
            <table>
              <thead>
                <tr>
                  <th scope="col">{{ ipText("比较维度") }}</th>
                  <th v-for="item in comparison" :key="item.id" scope="col">
                    {{ item.name }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <th scope="row">{{ ipText("类型") }}</th>
                  <td v-for="item in comparison" :key="item.id">
                    {{ ipText(item.kind) }}
                  </td>
                </tr>
                <tr>
                  <th scope="row">{{ ipText("契合依据") }}</th>
                  <td v-for="item in comparison" :key="item.id">
                    {{ item.reason }}
                  </td>
                </tr>
                <tr>
                  <th scope="row">{{ ipText("合作切入") }}</th>
                  <td v-for="item in comparison" :key="item.id">
                    {{ ipText(item.entryPoints.join("；")) }}
                  </td>
                </tr>
                <tr>
                  <th scope="row">{{ ipText("待核实") }}</th>
                  <td v-for="item in comparison" :key="item.id">
                    {{ ipText(item.pending.join("；")) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="muted">
            {{ ipText("对比基于公开资料；授权范围和价格尚未确认。") }}
          </p>
        </section>
        <details v-if="research.gaps?.length" class="gaps" open>
          <summary>
            {{ ipText("研究缺口 ·") }} {{ ipText(research.gaps.length) }}
          </summary>
          <ul>
            <li v-for="gap in research.gaps" :key="gap">{{ ipText(gap) }}</li>
          </ul>
        </details>
        <details v-if="research.queries?.length" class="query-log">
          <summary>{{ ipText("查看本轮检索词") }}</summary>
          <ul>
            <li v-for="query in research.queries" :key="query">
              {{ ipText(query) }}
            </li>
          </ul>
        </details>
        <button
          v-if="research.phase !== 'unavailable'"
          :disabled="busy"
          @click="emit('ask', ipText('继续深入研究，补查候选与合作依据'))"
        >
          {{ ipText("继续深入研究") }}
        </button>
      </template>
    </template>
  </section>
</template>

<style scoped lang="scss">
.external-research {
  min-width: 0;
  font-size: 13px;
  line-height: 1.7;
  color: #202621;
  overflow-wrap: anywhere;
}

h3 {
  margin: 0 0 10px;
  font-size: 17px;
  font-weight: 650;
}

h4 {
  margin: 0;
  font-size: 17px;
  font-weight: 650;
}

p {
  margin: 8px 0;
}

.muted,
small,
.scope,
.research-summary > span {
  color: #616b63;
}

.scope {
  margin: 0 0 22px;
}

.research-summary,
.candidate-heading {
  display: flex;
  gap: 12px;
  align-items: baseline;
  justify-content: space-between;
}

.candidate {
  padding: 24px 0;
  border-bottom: 1px solid #e4e8e2;
}

h4 span {
  margin-left: 6px;
  font-size: 12px;
  font-weight: 400;
  color: #616b63;
}

label {
  display: inline-flex;
  flex-shrink: 0;
  gap: 6px;
  align-items: center;
  cursor: pointer;
}

input {
  width: 15px;
  height: 15px;
  accent-color: #26332a;
  cursor: pointer;
}

.identity {
  font-size: 12px;
  color: #616b63;
}

dl {
  display: grid;
  grid-template-columns: 60px minmax(0, 1fr);
  gap: 10px 14px;
  margin: 18px 0;
}

dt {
  font-weight: 600;
}

dd {
  margin: 0;
}

details {
  margin: 14px 0;
}

summary {
  color: #435c69;
  cursor: pointer;
}

blockquote {
  padding-left: 12px;
  margin: 14px 0;
  color: #616b63;
  border-left: 1px solid #d6ddd5;
}

a {
  display: block;
  color: #385c6d;
  text-decoration: underline;
  text-underline-offset: 3px;
}

a:hover {
  color: #1c3948;
}

small {
  display: block;
  margin-top: 3px;
  font-size: 11px;
}

.source {
  margin: 14px 0;
}

button {
  padding: 7px 12px;
  color: #26332a;
  cursor: pointer;
  background: transparent;
  border: 1px solid #cbd2c8;
  border-radius: 6px;
  transition: border-color 180ms ease-out;
}

button:hover {
  border-color: #26332a;
}

button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.text-action {
  padding: 3px 0;
  border: 0;
  border-radius: 0;
}

button:focus-visible,
input:focus-visible,
summary:focus-visible,
a:focus-visible,
.table-scroll:focus-visible {
  outline: 2px solid #435c69;
  outline-offset: 3px;
}

.comparison {
  margin: 28px 0;
}

.table-scroll {
  max-width: 100%;
  overflow-x: auto;
  scrollbar-color: #adb7ad #f5f6f3;
  scrollbar-width: thin;
}

table {
  width: 100%;
  min-width: 480px;
  table-layout: fixed;
  border-collapse: collapse;
}

th,
td {
  padding: 10px;
  vertical-align: top;
  text-align: left;
  border: 1px solid #e4e8e2;
}

th {
  font-weight: 600;
}

th:first-child {
  width: 82px;
}

ul {
  padding-left: 18px;
  color: #616b63;
}

.query-log {
  font-size: 12px;
}

::selection {
  color: #17221b;
  background: #e2ebd1;
}
</style>
