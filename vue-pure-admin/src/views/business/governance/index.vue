<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { getGovernanceRules, saveGovernanceRule } from "@/api/business";

defineOptions({ name: "BusinessGovernance" });

type RuleItem = {
  ruleType: string;
  name: string;
  content: Record<string, any>;
  enabled: boolean;
  updatedAt?: number | string;
};

const loading = ref(false);
const savingType = ref("");
const rules = ref<RuleItem[]>([]);
const activeType = ref("");
const effectiveMode = "下次推荐生效";

const ruleNav = [
  {
    type: "scoring_model",
    title: "评分权重",
    desc: "影响资源等级计算",
    icon: "ri:percent-line"
  },
  {
    type: "level_threshold",
    title: "等级阈值",
    desc: "S/A/B/C 映射规则",
    icon: "ri:bar-chart-grouped-line"
  },
  {
    type: "required_fields",
    title: "必填字段",
    desc: "不同资源类型的数据要求",
    icon: "ri:list-check-3"
  },
  {
    type: "update_frequency",
    title: "更新频率",
    desc: "不同等级的维护周期",
    icon: "ri:calendar-check-line"
  },
  {
    type: "data_trust",
    title: "数据可信度",
    desc: "来源等级、证据和资源库联动",
    icon: "ri:shield-check-line"
  },
  {
    type: "recommendation",
    title: "推荐策略",
    desc: "候选过滤与风险策略",
    icon: "ri:filter-3-line"
  },
  {
    type: "warning",
    title: "预警规则",
    desc: "触发运营提醒的阈值",
    icon: "ri:alarm-warning-line"
  }
];

const scoreFields = [
  ["influence", "影响力", "粉丝、播放、传播半径"],
  ["activity", "活跃度", "近期发布和响应节奏"],
  ["interactionQuality", "互动质量", "评论、互动率和内容质量"],
  ["brandFit", "品牌匹配", "行业、受众和调性匹配"],
  ["deliveryPerformance", "履约表现", "准时交付和协作稳定性"],
  ["conversionEffect", "转化效果", "点击、转化和复用效果"]
];

const requiredFieldGroups = [
  ["creator", "创作者/KOL", "ri:user-star-line"],
  ["media", "媒体", "ri:newspaper-line"],
  ["agency", "代理商", "ri:building-4-line"]
];

const defaultFieldOptions = [
  "Profile URL",
  "平台官网",
  "平台",
  "国家",
  "地区",
  "城市",
  "语言",
  "行业",
  "粉丝数",
  "联系人",
  "Email",
  "负责人",
  "合作范围",
  "官网URL",
  "公司名称"
];

const trustLevels = ["A", "B", "C", "D"];

const trustBadgeMap: Record<string, string> = {
  A: "直接可信",
  B: "可复核",
  C: "估算",
  D: "待补全"
};

const trustFieldOptions = [
  "粉丝数",
  "平均播放",
  "互动率",
  "总播放",
  "联系方式",
  "报价",
  "合作效果",
  "履约表现"
];

const sourceFlow = [
  {
    title: "导入/新增资源",
    desc: "写入来源类型、证据、采集时间"
  },
  {
    title: "全球资源库字段",
    desc: "保存字段来源、负责人、更新时间"
  },
  {
    title: "治理规则",
    desc: "映射可信等级和折算系数"
  },
  {
    title: "评分/推荐/预警",
    desc: "按可信度降权或提醒补全"
  }
];

const activeRule = computed(() =>
  rules.value.find(rule => rule.ruleType === activeType.value)
);

const activeNav = computed(() =>
  ruleNav.find(item => item.type === activeType.value)
);

const enabledCount = computed(
  () => rules.value.filter(rule => rule.enabled).length
);

const disabledCount = computed(() => rules.value.length - enabledCount.value);

const scoringRule = computed(() =>
  rules.value.find(rule => rule.ruleType === "scoring_model")
);

const scoringTotal = computed(() => {
  const content = scoringRule.value?.content || {};
  return scoreFields.reduce(
    (total, [key]) => total + Number(content[key] || 0),
    0
  );
});

const activeScoringTotal = computed(() => {
  if (activeRule.value?.ruleType !== "scoring_model") return scoringTotal.value;
  return scoreFields.reduce(
    (total, [key]) => total + Number(activeRule.value?.content[key] || 0),
    0
  );
});

const completenessRule = computed(() =>
  rules.value.find(rule => rule.ruleType === "recommendation")
);

const summaryItems = computed(() => {
  const rule = activeRule.value;
  if (!rule) return [];
  const content = rule.content;
  switch (rule.ruleType) {
    case "scoring_model":
      return [
        ["权重合计", `${activeScoringTotal.value}%`],
        ["状态", activeScoringTotal.value === 100 ? "可生效" : "需调整"]
      ];
    case "level_threshold":
      return [
        ["S 级", `${content.S} 分起`],
        ["A 级", `${content.A} 分起`],
        ["B 级", `${content.B} 分起`],
        ["C 级", `${content.C} 分起`]
      ];
    case "required_fields":
      return requiredFieldGroups.map(([key, label]) => [
        label,
        `${(content[key] || []).length} 项`
      ]);
    case "update_frequency":
      return [
        ["S/A", `${content.SA} 天`],
        ["B/C", `${content.BC} 天`],
        ["D", `${content.D} 天`]
      ];
    case "data_trust": {
      const factors = trustLevels.map(level =>
        Number(content[level]?.factor || 0)
      );
      const fieldCount = new Set(
        trustLevels.flatMap(level => content[level]?.appliesTo || [])
      ).size;
      return [
        ["A 级来源", content.A?.source || "未配置"],
        ["证据要求", content.A?.evidence || "未配置"],
        ["最低折算", `${Math.min(...factors).toFixed(2)}x`],
        ["覆盖字段", `${fieldCount} 项`]
      ];
    }
    case "recommendation":
      return [
        ["最低等级", content.minimumLevel],
        ["完整度", `${content.minimumCompleteness}%`],
        ["未更新", `${content.maxDaysSinceUpdate} 天内`]
      ];
    case "warning":
      return [
        ["评分下降", `${content.scoreDrop} 分`],
        ["报价偏高", `${content.costAbovePeerAveragePercent}%`],
        ["延迟次数", `${content.deliveryDelayTimes} 次`]
      ];
    default:
      return [];
  }
});

const validationTips = computed(() => {
  const rule = activeRule.value;
  if (!rule) return [];
  const tips: string[] = [];
  const content = rule.content;
  if (rule.ruleType === "scoring_model" && activeScoringTotal.value !== 100) {
    tips.push("评分权重合计建议保持 100%，否则等级计算口径会失衡。");
  }
  if (rule.ruleType === "level_threshold") {
    if (
      !(content.S > content.A && content.A > content.B && content.B > content.C)
    ) {
      tips.push("等级阈值应保持 S > A > B > C。");
    }
  }
  if (rule.ruleType === "data_trust") {
    const factors = trustLevels.map(level => Number(content[level]?.factor));
    const ordered = factors.every(
      (factor, index) =>
        index === 0 || Number(content[trustLevels[index - 1]]?.factor) >= factor
    );
    if (!ordered) {
      tips.push("可信度折算建议保持 A ≥ B ≥ C ≥ D。");
    }
    if (
      trustLevels.some(
        level => !content[level]?.source || !content[level]?.evidence
      )
    ) {
      tips.push("每个可信等级都需要来源判定和证据要求。");
    }
  }
  if (rule.ruleType === "recommendation" && content.minimumCompleteness < 60) {
    tips.push("资料完整度下限偏低，可能放入不完整资源。");
  }
  return tips;
});

function defaultContent(type: string) {
  const defaults: Record<string, Record<string, any>> = {
    scoring_model: {
      influence: 20,
      activity: 15,
      interactionQuality: 20,
      brandFit: 15,
      deliveryPerformance: 20,
      conversionEffect: 10
    },
    level_threshold: { S: 90, A: 80, B: 65, C: 50 },
    required_fields: {
      creator: ["Profile URL", "平台", "国家", "语言", "粉丝数", "负责人"],
      media: ["官网URL", "国家", "语言", "行业", "联系人", "负责人"],
      agency: ["公司名称", "国家", "联系人", "合作范围", "负责人"]
    },
    update_frequency: { SA: 30, BC: 90, D: 180 },
    data_trust: {
      A: {
        source: "官方 API 或授权后台数据",
        evidence: "接口同步记录、授权截图或平台后台导出文件",
        appliesTo: ["粉丝数", "平均播放", "互动率", "总播放"],
        factor: 1
      },
      B: {
        source: "创作者后台截图或录屏",
        evidence: "带日期的截图、录屏、邮件确认或表单回传",
        appliesTo: ["粉丝数", "平均播放", "联系方式", "报价"],
        factor: 0.9
      },
      C: {
        source: "第三方工具估算",
        evidence: "工具名称、查询日期、链接或导出文件",
        appliesTo: ["粉丝数", "平均播放", "互动率"],
        factor: 0.8
      },
      D: {
        source: "人工公开页面采集或无来源旧数据",
        evidence: "页面链接、采集人和采集时间；缺失则进入待补全",
        appliesTo: ["联系方式", "报价", "合作效果"],
        factor: 0.7
      }
    },
    recommendation: {
      minimumLevel: "B",
      excludeBlacklisted: true,
      includeWatchingByDefault: false,
      minimumCompleteness: 80,
      maxDaysSinceUpdate: 180,
      overBudgetPolicy: "filter",
      highRiskPolicy: "downgrade_or_filter"
    },
    warning: {
      scoreDrop: 10,
      costAbovePeerAveragePercent: 50,
      deliveryDelayTimes: 2,
      staleContact: true
    }
  };
  return JSON.parse(JSON.stringify(defaults[type] || {}));
}

function normalizeDataTrustContent(content: Record<string, any>) {
  const defaults = defaultContent("data_trust");
  trustLevels.forEach(level => {
    const item = content[level] || {};
    content[level] = {
      ...defaults[level],
      ...item,
      appliesTo: Array.isArray(item.appliesTo)
        ? item.appliesTo
        : defaults[level].appliesTo
    };
  });
  return content;
}

function parseContent(value: any, type: string) {
  let content = value;
  if (typeof value === "string") {
    try {
      content = JSON.parse(value);
    } catch {
      content = {};
    }
  }
  const merged = { ...defaultContent(type), ...(content || {}) };
  return type === "data_trust" ? normalizeDataTrustContent(merged) : merged;
}

function normalizeRule(rule: any): RuleItem {
  return {
    ...rule,
    content: parseContent(rule.content, rule.ruleType),
    enabled: rule.enabled === 1 || rule.enabled === "1" || rule.enabled === true
  };
}

async function loadData() {
  loading.value = true;
  const res = await getGovernanceRules();
  loading.value = false;
  if (res.code === 0) {
    rules.value = (res.data || [])
      .map(normalizeRule)
      .filter(rule => rule.ruleType !== "ai_model");
    if (!rules.value.some(rule => rule.ruleType === activeType.value)) {
      activeType.value = rules.value[0]?.ruleType || "";
    }
  }
}

function resetActiveRule() {
  if (!activeRule.value) return;
  activeRule.value.content = defaultContent(activeRule.value.ruleType);
}

function normalizeScoringWeights() {
  const rule = activeRule.value;
  if (!rule || rule.ruleType !== "scoring_model") return;
  const total = activeScoringTotal.value;
  if (total <= 0) {
    rule.content = defaultContent("scoring_model");
    return;
  }
  let remaining = 100;
  scoreFields.forEach(([key], index) => {
    const value =
      index === scoreFields.length - 1
        ? remaining
        : Math.round((Number(rule.content[key] || 0) / total) * 100);
    rule.content[key] = value;
    remaining -= value;
  });
}

function buildContent(rule: RuleItem) {
  const content = { ...rule.content };
  if (rule.ruleType === "data_trust") {
    trustLevels.forEach(level => {
      const item = content[level] || {};
      content[level] = {
        ...item,
        appliesTo: Array.from(
          new Set(
            (item.appliesTo || [])
              .map((field: string) => field.trim())
              .filter(Boolean)
          )
        )
      };
    });
    return content;
  }
  if (rule.ruleType !== "required_fields") return rule.content;
  requiredFieldGroups.forEach(([key]) => {
    content[key] = Array.from(
      new Set(
        (content[key] || []).map((item: string) => item.trim()).filter(Boolean)
      )
    );
  });
  return content;
}

async function save(rule: RuleItem) {
  if (rule.ruleType === "scoring_model" && activeScoringTotal.value !== 100) {
    ElMessage.warning("评分权重合计需要等于 100");
    return;
  }
  if (rule.ruleType === "level_threshold") {
    const content = rule.content;
    if (
      !(content.S > content.A && content.A > content.B && content.B > content.C)
    ) {
      ElMessage.warning("等级阈值需要保持 S > A > B > C");
      return;
    }
  }

  savingType.value = rule.ruleType;
  const content = buildContent(rule);
  const res = await saveGovernanceRule({
    ruleType: rule.ruleType,
    name: rule.name,
    content: JSON.stringify(content),
    enabled: rule.enabled,
    effectiveMode,
    impactSummary: `${activeNav.value?.title || rule.name} 已保存，将在下次推荐生效`
  });
  savingType.value = "";
  if (res.code === 0) {
    ElMessage.success("规则已保存");
    loadData();
  }
}

function updatedText(value: unknown) {
  if (!value) return "-";
  const time = Number(value);
  if (!Number.isFinite(time)) return String(value);
  return new Date(time).toLocaleString("zh-CN");
}

function navMeta(type: string) {
  return ruleNav.find(item => item.type === type);
}

onMounted(loadData);
</script>

<template>
  <div class="governance-page">
    <section class="page-hero">
      <div>
        <span>Governance Center</span>
        <h1>治理规则配置中心</h1>
        <p>
          把评分、推荐、预警和数据质量规则集中配置，保存后自动生成规则版本记录。
        </p>
      </div>
      <div class="hero-actions">
        <el-tag class="effective-tag" effect="plain" type="warning">
          {{ effectiveMode }}
        </el-tag>
      </div>
    </section>

    <section class="overview-grid">
      <div class="metric-tile">
        <span>启用规则</span>
        <strong>{{ enabledCount }}/{{ rules.length }}</strong>
      </div>
      <div class="metric-tile">
        <span>停用规则</span>
        <strong>{{ disabledCount }}</strong>
      </div>
      <div class="metric-tile">
        <span>评分权重</span>
        <strong :class="{ danger: scoringTotal !== 100 }">
          {{ scoringTotal }}%
        </strong>
      </div>
      <div class="metric-tile">
        <span>推荐完整度</span>
        <strong>
          {{ completenessRule?.content.minimumCompleteness ?? "-" }}%
        </strong>
      </div>
    </section>

    <section class="governance-layout">
      <aside class="rule-sidebar">
        <button
          v-for="rule in rules"
          :key="rule.ruleType"
          type="button"
          class="rule-nav-item"
          :class="{ active: activeType === rule.ruleType }"
          @click="activeType = rule.ruleType"
        >
          <IconifyIconOnline
            :icon="navMeta(rule.ruleType)?.icon || 'ri:settings-4-line'"
            class="rule-nav-icon"
          />
          <span>
            <strong>{{ navMeta(rule.ruleType)?.title || rule.name }}</strong>
            <small>{{ navMeta(rule.ruleType)?.desc }}</small>
            <em>{{ updatedText(rule.updatedAt) }}</em>
          </span>
          <el-tag
            size="small"
            :type="rule.enabled ? 'success' : 'info'"
            effect="plain"
          >
            {{ rule.enabled ? "启用" : "停用" }}
          </el-tag>
        </button>
      </aside>

      <main v-if="activeRule" v-loading="loading" class="editor-panel">
        <div class="editor-header">
          <div class="header-title">
            <IconifyIconOnline
              :icon="activeNav?.icon || 'ri:settings-4-line'"
              class="header-icon"
            />
            <div>
              <strong>{{ activeNav?.title || activeRule.name }}</strong>
              <span>{{ activeNav?.desc }}</span>
            </div>
          </div>
          <el-switch
            v-model="activeRule.enabled"
            active-text="启用"
            inactive-text="停用"
          />
        </div>

        <div class="summary-strip">
          <div
            v-for="[label, value] in summaryItems"
            :key="`${label}-${value}`"
            class="summary-chip"
          >
            <span>{{ label }}</span>
            <strong>{{ value }}</strong>
          </div>
        </div>

        <el-alert
          v-if="validationTips.length > 0"
          class="rule-alert"
          type="warning"
          :closable="false"
          show-icon
        >
          <template #title>
            <span>{{ validationTips.join(" ") }}</span>
          </template>
        </el-alert>

        <el-form label-position="top" class="rule-form">
          <section class="form-section">
            <div class="section-title">
              <strong>基础信息</strong>
              <span>用于识别规则版本和配置含义</span>
            </div>
            <el-row :gutter="14">
              <el-col :xs="24" :md="12">
                <el-form-item label="规则名称">
                  <el-input v-model="activeRule.name" />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="12">
                <el-form-item label="最近更新">
                  <el-input
                    :model-value="updatedText(activeRule.updatedAt)"
                    disabled
                  />
                </el-form-item>
              </el-col>
            </el-row>
          </section>

          <section
            v-if="activeRule.ruleType === 'scoring_model'"
            class="form-section"
          >
            <div class="section-title">
              <strong>权重配置</strong>
              <span>滑杆适合粗调，数字框适合精调</span>
            </div>
            <div class="weight-toolbar">
              <el-progress
                :percentage="Math.min(activeScoringTotal, 100)"
                :status="activeScoringTotal === 100 ? 'success' : 'warning'"
              />
              <span :class="{ danger: activeScoringTotal !== 100 }">
                合计 {{ activeScoringTotal }}%
              </span>
              <el-button @click="normalizeScoringWeights">归一到 100</el-button>
            </div>
            <div class="weight-grid">
              <div v-for="[key, label, desc] in scoreFields" :key="key">
                <div class="weight-label">
                  <strong>{{ label }}</strong>
                  <span>{{ desc }}</span>
                </div>
                <el-slider
                  v-model="activeRule.content[key]"
                  :min="0"
                  :max="100"
                  :step="1"
                />
                <el-input-number
                  v-model="activeRule.content[key]"
                  :min="0"
                  :max="100"
                  controls-position="right"
                />
              </div>
            </div>
          </section>

          <section
            v-if="activeRule.ruleType === 'level_threshold'"
            class="form-section"
          >
            <div class="section-title">
              <strong>等级门槛</strong>
              <span>D 级为低于 C 级门槛的资源</span>
            </div>
            <div class="threshold-grid">
              <div v-for="level in ['S', 'A', 'B', 'C']" :key="level">
                <strong>{{ level }}</strong>
                <span>最低分</span>
                <el-input-number
                  v-model="activeRule.content[level]"
                  :min="0"
                  :max="100"
                  controls-position="right"
                />
              </div>
            </div>
          </section>

          <section
            v-if="activeRule.ruleType === 'required_fields'"
            class="form-section"
          >
            <div class="section-title">
              <strong>必填字段</strong>
              <span>可从常用字段中选择，也可以直接输入新字段</span>
            </div>
            <div class="field-group-grid">
              <div
                v-for="[key, label, icon] in requiredFieldGroups"
                :key="key"
                class="field-group"
              >
                <div class="field-group-title">
                  <IconifyIconOnline :icon="icon" />
                  <strong>{{ label }}</strong>
                </div>
                <el-select
                  v-model="activeRule.content[key]"
                  multiple
                  allow-create
                  filterable
                  default-first-option
                  collapse-tags
                  collapse-tags-tooltip
                  placeholder="选择或输入字段"
                >
                  <el-option
                    v-for="field in defaultFieldOptions"
                    :key="field"
                    :label="field"
                    :value="field"
                  />
                </el-select>
                <div class="selected-fields">
                  <el-tag
                    v-for="field in activeRule.content[key]"
                    :key="field"
                    closable
                    @close="
                      activeRule.content[key] = activeRule.content[key].filter(
                        item => item !== field
                      )
                    "
                  >
                    {{ field }}
                  </el-tag>
                </div>
              </div>
            </div>
          </section>

          <section
            v-if="activeRule.ruleType === 'update_frequency'"
            class="form-section"
          >
            <div class="section-title">
              <strong>维护周期</strong>
              <span>到期后可进入待更新提醒</span>
            </div>
            <div class="threshold-grid">
              <div
                v-for="[key, label] in [
                  ['SA', 'S/A 级'],
                  ['BC', 'B/C 级'],
                  ['D', 'D 级']
                ]"
                :key="key"
              >
                <strong>{{ label }}</strong>
                <span>更新周期（天）</span>
                <el-input-number
                  v-model="activeRule.content[key]"
                  :min="1"
                  :max="365"
                  controls-position="right"
                />
              </div>
            </div>
          </section>

          <section
            v-if="activeRule.ruleType === 'data_trust'"
            class="form-section"
          >
            <div class="section-title">
              <strong>来源判定规则</strong>
              <span>资源库存来源元数据，规则负责可信等级和折算口径</span>
            </div>
            <div class="linkage-panel">
              <div class="linkage-copy">
                <strong>与全球资源库联动</strong>
                <span>
                  新增、导入、编辑、平台同步写入来源类型、证据、采集时间和负责人；
                  评分、推荐、预警读取来源等级后再做降权或提醒。
                </span>
              </div>
              <div class="source-flow">
                <div v-for="step in sourceFlow" :key="step.title">
                  <strong>{{ step.title }}</strong>
                  <span>{{ step.desc }}</span>
                </div>
              </div>
            </div>
            <div class="trust-grid">
              <div v-for="level in trustLevels" :key="level" class="trust-card">
                <div class="trust-card-header">
                  <strong>{{ level }} 级</strong>
                  <el-tag effect="plain">{{ trustBadgeMap[level] }}</el-tag>
                </div>
                <el-form-item label="判定来源">
                  <el-input
                    v-model="activeRule.content[level].source"
                    placeholder="来源类型"
                  />
                </el-form-item>
                <el-form-item label="证据要求">
                  <el-input
                    v-model="activeRule.content[level].evidence"
                    placeholder="截图、链接、同步记录等"
                  />
                </el-form-item>
                <el-form-item label="适用字段">
                  <el-select
                    v-model="activeRule.content[level].appliesTo"
                    multiple
                    allow-create
                    filterable
                    default-first-option
                    collapse-tags
                    collapse-tags-tooltip
                    placeholder="选择或输入字段"
                  >
                    <el-option
                      v-for="field in trustFieldOptions"
                      :key="field"
                      :label="field"
                      :value="field"
                    />
                  </el-select>
                </el-form-item>
                <el-form-item label="折算系数">
                  <el-input-number
                    v-model="activeRule.content[level].factor"
                    :min="0"
                    :max="1"
                    :step="0.05"
                    controls-position="right"
                  />
                </el-form-item>
              </div>
            </div>
          </section>

          <section
            v-if="activeRule.ruleType === 'recommendation'"
            class="form-section"
          >
            <div class="section-title">
              <strong>推荐过滤</strong>
              <span>控制候选资源进入推荐池前的硬性规则</span>
            </div>
            <el-row :gutter="14">
              <el-col :xs="24" :md="8">
                <el-form-item label="最低推荐等级">
                  <el-select v-model="activeRule.content.minimumLevel">
                    <el-option
                      v-for="level in ['S', 'A', 'B', 'C', 'D']"
                      :key="level"
                      :label="level"
                      :value="level"
                    />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="8">
                <el-form-item label="资料完整度下限">
                  <el-input-number
                    v-model="activeRule.content.minimumCompleteness"
                    :min="0"
                    :max="100"
                    controls-position="right"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="8">
                <el-form-item label="最长未更新天数">
                  <el-input-number
                    v-model="activeRule.content.maxDaysSinceUpdate"
                    :min="1"
                    :max="365"
                    controls-position="right"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="12">
                <el-form-item label="超预算策略">
                  <el-radio-group v-model="activeRule.content.overBudgetPolicy">
                    <el-radio-button value="filter">过滤</el-radio-button>
                    <el-radio-button value="downgrade">降权</el-radio-button>
                  </el-radio-group>
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="12">
                <el-form-item label="高风险策略">
                  <el-select v-model="activeRule.content.highRiskPolicy">
                    <el-option label="降权或过滤" value="downgrade_or_filter" />
                    <el-option label="仅提示风险" value="warn_only" />
                    <el-option label="直接过滤" value="filter" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>
            <div class="switch-list">
              <label>
                <span>
                  <strong>排除黑名单资源</strong>
                  <small>黑名单和已归档资源不进入推荐候选</small>
                </span>
                <el-switch v-model="activeRule.content.excludeBlacklisted" />
              </label>
              <label>
                <span>
                  <strong>默认包含观察中资源</strong>
                  <small>打开后观察中资源会进入推荐结果</small>
                </span>
                <el-switch
                  v-model="activeRule.content.includeWatchingByDefault"
                />
              </label>
            </div>
          </section>

          <section
            v-if="activeRule.ruleType === 'warning'"
            class="form-section"
          >
            <div class="section-title">
              <strong>预警触发</strong>
              <span>达到阈值时进入运营待处理事项</span>
            </div>
            <el-row :gutter="14">
              <el-col :xs="24" :md="8">
                <el-form-item label="评分下降预警">
                  <el-input-number
                    v-model="activeRule.content.scoreDrop"
                    :min="1"
                    :max="100"
                    controls-position="right"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="8">
                <el-form-item label="报价高于同类均值">
                  <el-input-number
                    v-model="activeRule.content.costAbovePeerAveragePercent"
                    :min="1"
                    :max="500"
                    controls-position="right"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="8">
                <el-form-item label="交付延迟次数">
                  <el-input-number
                    v-model="activeRule.content.deliveryDelayTimes"
                    :min="1"
                    :max="20"
                    controls-position="right"
                  />
                </el-form-item>
              </el-col>
            </el-row>
            <div class="switch-list">
              <label>
                <span>
                  <strong>联系人长期未更新时预警</strong>
                  <small>适合推动资源库联系人信息复核</small>
                </span>
                <el-switch v-model="activeRule.content.staleContact" />
              </label>
            </div>
          </section>
        </el-form>

        <div class="action-bar">
          <el-button @click="resetActiveRule">恢复默认</el-button>
          <el-button
            type="primary"
            :loading="savingType === activeRule.ruleType"
            @click="save(activeRule)"
          >
            保存规则
          </el-button>
        </div>
      </main>

      <main v-else class="editor-panel empty-panel">
        <el-empty description="暂无治理规则" />
      </main>
    </section>
  </div>
</template>

<style scoped>
.governance-page {
  min-height: 100%;
  padding: 20px;
  background: #f8fafc;
}

.page-hero {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  padding: 20px;
  margin-bottom: 14px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.page-hero span {
  font-size: 12px;
  font-weight: 700;
  color: #2563eb;
  text-transform: uppercase;
}

.page-hero h1 {
  margin: 8px 0 0;
  font-size: 28px;
  line-height: 1.25;
  color: #0f172a;
  letter-spacing: 0;
}

.page-hero p {
  margin: 8px 0 0;
  color: #64748b;
}

.hero-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

.effective-tag {
  justify-content: center;
  min-width: 116px;
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}

.metric-tile {
  display: grid;
  gap: 6px;
  padding: 14px 16px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.metric-tile span {
  font-size: 12px;
  color: #64748b;
}

.metric-tile strong {
  font-size: 22px;
  color: #0f172a;
}

.danger {
  color: #dc2626 !important;
}

.governance-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 14px;
  align-items: start;
}

.rule-sidebar {
  position: sticky;
  top: 72px;
  display: grid;
  gap: 8px;
}

.rule-nav-item {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  width: 100%;
  padding: 12px;
  text-align: left;
  cursor: pointer;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.rule-nav-item.active {
  border-color: #2563eb;
  box-shadow: inset 3px 0 0 #2563eb;
}

.rule-nav-icon,
.header-icon {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  color: #2563eb;
  background: #eff6ff;
  border-radius: 8px;
}

.rule-nav-item span {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.rule-nav-item strong,
.editor-header strong,
.section-title strong {
  color: #0f172a;
}

.rule-nav-item small,
.rule-nav-item em,
.editor-header span,
.section-title span {
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
  font-style: normal;
  color: #64748b;
  white-space: nowrap;
}

.editor-panel {
  min-width: 0;
  overflow: hidden;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.editor-header {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 16px 18px;
  border-bottom: 1px solid #e2e8f0;
}

.header-title {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
}

.header-title div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.summary-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  padding: 14px 18px;
  background: #f8fafc;
  border-bottom: 1px solid #e2e8f0;
}

.summary-chip {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.summary-chip span {
  font-size: 12px;
  color: #64748b;
}

.summary-chip strong {
  overflow: hidden;
  text-overflow: ellipsis;
  color: #0f172a;
  white-space: nowrap;
}

.rule-alert {
  margin: 14px 18px 0;
}

.rule-form {
  padding: 18px;
}

.form-section + .form-section {
  padding-top: 18px;
  margin-top: 18px;
  border-top: 1px solid #e2e8f0;
}

.section-title {
  display: grid;
  gap: 4px;
  margin-bottom: 14px;
}

.inline-setting {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  min-height: 32px;
  padding: 0 2px;
}

.weight-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
}

.weight-toolbar span {
  font-weight: 700;
  color: #0f172a;
}

.weight-grid,
.threshold-grid,
.field-group-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.weight-grid > div,
.threshold-grid > div,
.field-group {
  display: grid;
  gap: 10px;
  padding: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.weight-label {
  display: grid;
  gap: 3px;
}

.weight-label span,
.threshold-grid span {
  font-size: 12px;
  color: #64748b;
}

.threshold-grid strong {
  font-size: 20px;
  color: #0f172a;
}

.field-group-title {
  display: flex;
  gap: 8px;
  align-items: center;
}

.selected-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-height: 28px;
}

.linkage-panel {
  display: grid;
  gap: 14px;
  padding: 14px;
  margin-bottom: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.linkage-copy {
  display: grid;
  gap: 4px;
}

.linkage-copy strong,
.trust-card-header strong {
  color: #0f172a;
}

.linkage-copy span,
.source-flow span {
  font-size: 12px;
  line-height: 1.6;
  color: #64748b;
}

.source-flow {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.source-flow > div {
  display: grid;
  gap: 4px;
  min-height: 82px;
  padding: 12px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.source-flow strong {
  font-size: 13px;
  color: #2563eb;
}

.trust-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.trust-card {
  display: grid;
  gap: 8px;
  padding: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.trust-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 2px;
}

.trust-card :deep(.el-form-item) {
  margin-bottom: 6px;
}

.switch-list {
  display: grid;
  gap: 10px;
  margin-top: 8px;
}

.switch-list label {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.switch-list label span {
  display: grid;
  gap: 3px;
}

.switch-list small {
  color: #64748b;
}

.action-bar {
  position: sticky;
  bottom: 0;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  padding: 12px 18px;
  background: rgb(255 255 255 / 94%);
  border-top: 1px solid #e2e8f0;
  backdrop-filter: blur(8px);
}

.empty-panel {
  padding: 32px;
}

:deep(.el-select),
:deep(.el-input-number),
:deep(.el-radio-group) {
  width: 100%;
}

:deep(.el-radio-button) {
  flex: 1;
}

:deep(.el-radio-button__inner) {
  width: 100%;
}

@media (width <= 1180px) {
  .governance-layout {
    grid-template-columns: 260px minmax(0, 1fr);
  }

  .overview-grid,
  .summary-strip,
  .weight-grid,
  .threshold-grid,
  .field-group-grid,
  .source-flow,
  .trust-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (width <= 900px) {
  .page-hero,
  .governance-layout,
  .overview-grid,
  .summary-strip,
  .weight-grid,
  .threshold-grid,
  .field-group-grid,
  .source-flow,
  .trust-grid {
    grid-template-columns: 1fr;
  }

  .page-hero {
    display: grid;
  }

  .rule-sidebar {
    position: static;
  }

  .weight-toolbar {
    grid-template-columns: 1fr;
  }
}

@media (width <= 640px) {
  .governance-page {
    padding: 12px;
  }

  .page-hero h1 {
    font-size: 24px;
  }

  .rule-nav-item {
    grid-template-columns: 32px minmax(0, 1fr);
  }

  .rule-nav-item .el-tag {
    grid-column: 2;
    justify-self: start;
  }
}
</style>
