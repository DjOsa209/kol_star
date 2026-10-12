<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { useUserStoreHook } from "@/store/modules/user";
import {
  decideProductCandidate,
  getDiscoveryProductDetail,
  listProductCandidates,
  refreshProductCandidates,
  searchProductDiscovery,
  type DiscoveryMode,
  type DiscoveryProduct,
  type ProductCandidate,
  type ProductCooperation
} from "@/api/productDiscovery";

const props = defineProps<{
  mode: Exclude<DiscoveryMode, "resource">;
  initialKeyword?: string;
}>();
const emit = defineEmits<{ selectBrand: [name: string] }>();
const router = useRouter();
const userStore = useUserStoreHook();
const canReview = computed(() =>
  userStore.roles.some(role => role === "admin" || role === "operation")
);
const keyword = ref("");
const loading = ref(false);
const rows = ref<Record<string, any>[]>([]);
const total = ref(0);
const currentPage = ref(1);
const pageSize = ref(12);
const detailVisible = ref(false);
const detailLoading = ref(false);
const selectedProduct = ref<DiscoveryProduct | null>(null);
const cooperations = ref<ProductCooperation[]>([]);
const candidatesVisible = ref(false);
const candidatesLoading = ref(false);
const candidateRows = ref<ProductCandidate[]>([]);
const candidateTotal = ref(0);
const candidatePage = ref(1);
const candidateInputs = reactive<
  Record<number, { brandName: string; productName: string }>
>({});
const decidingId = ref<number | null>(null);
const refreshing = ref(false);

const title = computed(() =>
  props.mode === "brand" ? "已合作品牌" : "产品合作发现"
);
const placeholder = computed(() =>
  props.mode === "brand" ? "搜索已确认合作的品牌" : "搜索已确认合作的产品或品牌"
);

function compactNumber(value: unknown) {
  const number = Number(value || 0);
  if (!Number.isFinite(number)) return "-";
  if (number >= 100000000) return `${(number / 100000000).toFixed(1)}亿`;
  if (number >= 10000) return `${(number / 10000).toFixed(1)}万`;
  return String(Math.round(number));
}

function sourceLabel(value: string | undefined) {
  if (value === "creative_name") return "同步作品标题";
  if (value === "project_name") return "历史项目名称";
  return "内部合作记录";
}

async function loadResults() {
  loading.value = true;
  try {
    const response = await searchProductDiscovery({
      mode: props.mode,
      keyword: keyword.value.trim(),
      currentPage: currentPage.value,
      pageSize: pageSize.value
    });
    if (response.code !== 0) throw new Error(response.message);
    rows.value = response.data?.list || [];
    total.value = Number(response.data?.total || 0);
  } catch (error) {
    rows.value = [];
    total.value = 0;
    ElMessage.error(
      error instanceof Error ? error.message : "合作发现加载失败"
    );
  } finally {
    loading.value = false;
  }
}

function search() {
  currentPage.value = 1;
  loadResults();
}

async function openProduct(row: Record<string, any>) {
  detailVisible.value = true;
  detailLoading.value = true;
  selectedProduct.value = row as DiscoveryProduct;
  cooperations.value = [];
  try {
    const response = await getDiscoveryProductDetail(Number(row.id));
    if (response.code !== 0) throw new Error(response.message);
    selectedProduct.value = response.data.product;
    cooperations.value = response.data.cooperations || [];
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "产品详情加载失败"
    );
  } finally {
    detailLoading.value = false;
  }
}

async function loadCandidates() {
  candidatesLoading.value = true;
  try {
    const response = await listProductCandidates({
      status: "pending",
      currentPage: candidatePage.value,
      pageSize: 10
    });
    if (response.code !== 0) throw new Error(response.message);
    candidateRows.value = response.data?.list || [];
    candidateTotal.value = Number(response.data?.total || 0);
    candidateRows.value.forEach(row => {
      candidateInputs[row.id] ||= {
        brandName: "",
        productName: row.suggestedProduct || ""
      };
    });
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "待核实列表加载失败"
    );
  } finally {
    candidatesLoading.value = false;
  }
}

function openCandidates() {
  candidatePage.value = 1;
  candidatesVisible.value = true;
  loadCandidates();
}

async function refreshCandidates() {
  refreshing.value = true;
  try {
    const response = await refreshProductCandidates();
    if (response.code !== 0) throw new Error(response.message);
    ElMessage.success(
      `扫描 ${response.data.scanned} 条合作，新增 ${response.data.created} 条待核实候选`
    );
    if (candidatesVisible.value) loadCandidates();
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : "历史合作识别失败"
    );
  } finally {
    refreshing.value = false;
  }
}

async function decide(
  row: ProductCandidate,
  decision: "confirmed" | "rejected"
) {
  const values = candidateInputs[row.id];
  if (
    decision === "confirmed" &&
    (!values?.brandName.trim() || !values?.productName.trim())
  ) {
    ElMessage.warning("请填写品牌和产品名称后确认");
    return;
  }
  decidingId.value = row.id;
  try {
    const response = await decideProductCandidate({
      candidateId: row.id,
      decision,
      brandName: values?.brandName.trim(),
      productName: values?.productName.trim()
    });
    if (response.code !== 0) throw new Error(response.message);
    ElMessage.success(
      decision === "confirmed" ? "已确认产品合作" : "已忽略候选"
    );
    await Promise.all([loadCandidates(), loadResults()]);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "处理失败");
  } finally {
    decidingId.value = null;
  }
}

watch(
  () => [props.mode, props.initialKeyword],
  () => {
    currentPage.value = 1;
    keyword.value = props.initialKeyword || "";
    loadResults();
  },
  { immediate: true }
);
</script>

<template>
  <div class="product-discovery">
    <section class="discovery-search-panel">
      <div class="discovery-panel-heading">
        <div>
          <p class="eyebrow">INTERNAL COOPERATION DISCOVERY</p>
          <h2>{{ title }}</h2>
          <p>仅展示已确认产品信息且已有发布证据的内部合作记录。</p>
        </div>
        <div v-if="canReview" class="panel-actions">
          <el-button :loading="refreshing" @click="refreshCandidates"
            >识别历史合作</el-button
          >
          <el-button @click="openCandidates">核实产品候选</el-button>
        </div>
      </div>
      <div class="discovery-search-row">
        <el-input
          v-model="keyword"
          :placeholder="placeholder"
          clearable
          @keyup.enter="search"
        >
          <template #prefix
            ><IconifyIconOnline icon="ri:search-line"
          /></template>
        </el-input>
        <el-button type="primary" @click="search">搜索</el-button>
      </div>
      <p class="search-scope">
        <IconifyIconOnline icon="ri:information-line" />
        数据范围：已同步合作项目；未核实的文本提及不计入合作统计。
      </p>
    </section>

    <section v-loading="loading" class="discovery-results">
      <div class="results-heading">
        <h3>{{ title }}</h3>
        <span>共 {{ total }} 条</span>
      </div>
      <el-empty
        v-if="!rows.length && !loading"
        description="暂无已确认的合作结果，可先识别并核实历史项目"
      />
      <div v-else class="discovery-grid">
        <article v-for="row in rows" :key="row.id" class="discovery-card">
          <div class="discovery-card-cover">
            <img v-if="row.coverUrl" :src="row.coverUrl" :alt="row.name" />
            <IconifyIconOnline
              v-else
              :icon="
                mode === 'brand' ? 'ri:price-tag-3-line' : 'ri:smartphone-line'
              "
            />
          </div>
          <div class="discovery-card-body">
            <span class="verified-tag"
              ><IconifyIconOnline icon="ri:checkbox-circle-fill" />
              已确认合作</span
            >
            <h4>{{ row.name }}</h4>
            <p v-if="mode === 'product'">{{ row.brandName || "品牌待补充" }}</p>
            <p v-else>已确认产品 {{ row.productCount || 0 }} 款</p>
            <div class="discovery-metrics">
              <span
                >合作资源 <strong>{{ row.resourceCount || 0 }}</strong></span
              >
              <span
                >发布内容 <strong>{{ row.contentCount || 0 }}</strong></span
              >
              <span
                >观看/曝光
                <strong>{{ compactNumber(row.totalViews) }}</strong></span
              >
            </div>
            <button
              v-if="mode === 'product'"
              type="button"
              class="detail-link"
              @click="openProduct(row)"
            >
              查看合作证据 <IconifyIconOnline icon="ri:arrow-right-line" />
            </button>
            <button
              v-else
              type="button"
              class="detail-link"
              @click="emit('selectBrand', String(row.name))"
            >
              查看合作产品 <IconifyIconOnline icon="ri:arrow-right-line" />
            </button>
          </div>
        </article>
      </div>
      <el-pagination
        v-if="total > pageSize"
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="loadResults"
      />
    </section>

    <el-drawer
      v-model="detailVisible"
      :title="selectedProduct?.name || '产品合作详情'"
      size="720px"
    >
      <div v-loading="detailLoading" class="product-detail">
        <p class="detail-intro">
          {{ selectedProduct?.brandName }} ·
          {{ cooperations.length }} 条已确认且有发布证据的合作
        </p>
        <el-empty
          v-if="!cooperations.length && !detailLoading"
          description="暂无可展示的已发布合作"
        />
        <article
          v-for="item in cooperations"
          :key="item.cooperationId"
          class="cooperation-evidence"
        >
          <img
            v-if="item.coverUrl"
            :src="item.coverUrl"
            :alt="item.projectName"
          />
          <div>
            <strong>{{ item.resourceName }}</strong>
            <p>{{ item.projectName }} · {{ item.platform || "平台待补充" }}</p>
            <small
              >{{ item.publishTime || "发布时间待补充" }} · 观看/曝光
              {{ compactNumber(item.views || item.impressions) }}</small
            >
            <p class="evidence-trace">
              识别来源：{{ sourceLabel(item.sourceField) }}
              <span v-if="item.sourceText">「{{ item.sourceText }}」</span>
            </p>
            <p class="evidence-trace">
              合作确认：{{
                item.confirmedByName ||
                (item.confirmedBy ? `用户 #${item.confirmedBy}` : "运营")
              }}
              <span v-if="item.confirmedAt"> · {{ item.confirmedAt }}</span>
            </p>
            <div class="evidence-actions">
              <el-button
                link
                type="primary"
                @click="
                  router.push({
                    path: '/business/resources/profile',
                    query: { id: item.resourceId }
                  })
                "
                >查看达人档案</el-button
              >
              <el-link
                v-if="item.contentUrl"
                :href="item.contentUrl"
                target="_blank"
                type="primary"
                >查看原始内容 ↗</el-link
              >
              <el-link
                v-if="
                  item.reviewedEvidenceUrl &&
                  item.reviewedEvidenceUrl !== item.contentUrl
                "
                :href="item.reviewedEvidenceUrl"
                target="_blank"
                type="primary"
                >查看确认时证据 ↗</el-link
              >
            </div>
          </div>
        </article>
      </div>
    </el-drawer>

    <el-drawer v-model="candidatesVisible" title="待核实产品候选" size="780px">
      <div v-loading="candidatesLoading" class="candidate-list">
        <p class="detail-intro">
          系统从历史项目和作品信息中提取线索。只有人工确认后，产品才会进入搜索结果和合作统计。
        </p>
        <el-empty
          v-if="!candidateRows.length && !candidatesLoading"
          description="暂无待核实候选"
        />
        <article
          v-for="row in candidateRows"
          :key="row.id"
          class="candidate-card"
        >
          <strong>{{ row.projectName }}</strong>
          <p>
            {{ row.resourceName }} · 建议产品：{{
              row.suggestedProduct || "待填写"
            }}
          </p>
          <small
            >识别依据：{{ sourceLabel(row.sourceField) }}
            <span v-if="row.sourceText">「{{ row.sourceText }}」</span></small
          >
          <div class="candidate-fields">
            <el-input
              v-model="candidateInputs[row.id].brandName"
              placeholder="确认品牌名称"
            />
            <el-input
              v-model="candidateInputs[row.id].productName"
              placeholder="确认产品名称"
            />
          </div>
          <div class="candidate-actions">
            <el-link
              v-if="row.evidenceUrl"
              :href="row.evidenceUrl"
              target="_blank"
              type="primary"
              >查看证据 ↗</el-link
            >
            <el-button
              :loading="decidingId === row.id"
              @click="decide(row, 'rejected')"
              >忽略</el-button
            >
            <el-button
              type="primary"
              :loading="decidingId === row.id"
              @click="decide(row, 'confirmed')"
              >确认合作产品</el-button
            >
          </div>
        </article>
        <el-pagination
          v-if="candidateTotal > 10"
          v-model:current-page="candidatePage"
          :page-size="10"
          :total="candidateTotal"
          layout="prev, pager, next"
          @current-change="loadCandidates"
        />
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.product-discovery {
  display: grid;
  gap: 18px;
}
.discovery-search-panel,
.discovery-results {
  background: #fff;
  border: 1px solid #e8e5dc;
  border-radius: 14px;
  padding: 24px;
  box-shadow: 0 6px 20px rgb(28 27 22 / 3%);
}
.discovery-panel-heading,
.results-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}
.eyebrow {
  margin: 0 0 8px;
  color: #77776d;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.14em;
}
h2,
h3,
h4,
p {
  margin-top: 0;
}
h2 {
  margin-bottom: 6px;
  font-size: 26px;
  color: #17181b;
}
.discovery-panel-heading p:not(.eyebrow),
.detail-intro {
  color: #777970;
  font-size: 13px;
}
.panel-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.discovery-search-row {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}
.discovery-search-row .el-input {
  flex: 1;
}
.discovery-search-row :deep(.el-input__wrapper) {
  min-height: 46px;
}
.discovery-search-row :deep(.el-button--primary),
.candidate-actions :deep(.el-button--primary) {
  background: #d4fb00;
  border-color: #d4fb00;
  color: #17181b;
  font-weight: 700;
}
.search-scope {
  display: flex;
  gap: 6px;
  align-items: center;
  margin: 10px 0 0;
  color: #8b8c88;
  font-size: 12px;
}
.results-heading {
  align-items: center;
  margin-bottom: 20px;
}
.results-heading h3 {
  margin: 0;
  font-size: 18px;
}
.results-heading span {
  color: #8b8c88;
  font-size: 13px;
}
.discovery-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(310px, 1fr));
  gap: 14px;
}
.discovery-card {
  overflow: hidden;
  border: 1px solid #e8e8e2;
  border-radius: 12px;
  background: #fff;
}
.discovery-card-cover {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 138px;
  background: #f6f7ef;
  color: #91a900;
  font-size: 52px;
}
.discovery-card-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.discovery-card-body {
  padding: 16px;
}
.verified-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #28844f;
  font-size: 12px;
}
.discovery-card h4 {
  margin: 8px 0 4px;
  color: #17181b;
  font-size: 18px;
}
.discovery-card p {
  margin-bottom: 12px;
  color: #888;
  font-size: 13px;
}
.discovery-metrics {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 14px;
  padding-top: 12px;
  border-top: 1px solid #ecece6;
  color: #777;
  font-size: 12px;
}
.discovery-metrics strong {
  color: #17181b;
}
.detail-link {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 16px;
  padding: 0;
  border: 0;
  background: none;
  color: #657a00;
  font-weight: 700;
  cursor: pointer;
}
.product-detail,
.candidate-list {
  display: grid;
  gap: 12px;
}
.cooperation-evidence {
  display: flex;
  gap: 14px;
  padding: 14px 0;
  border-bottom: 1px solid #ededeb;
}
.cooperation-evidence img {
  width: 124px;
  height: 84px;
  object-fit: cover;
  border-radius: 8px;
}
.cooperation-evidence strong {
  color: #17181b;
}
.cooperation-evidence p {
  margin: 5px 0;
  color: #666;
}
.cooperation-evidence small,
.candidate-card small {
  color: #92928c;
}
.cooperation-evidence .evidence-trace {
  color: #85877e;
  font-size: 12px;
  overflow-wrap: anywhere;
}
.evidence-actions {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 8px;
}
.candidate-card {
  padding: 16px;
  border: 1px solid #ecece6;
  border-radius: 10px;
}
.candidate-card p {
  margin: 7px 0;
  color: #666;
}
.candidate-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 14px;
}
.candidate-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}
.candidate-actions .el-link {
  margin-right: auto;
}
@media (max-width: 800px) {
  .discovery-panel-heading,
  .discovery-search-row {
    flex-direction: column;
  }
  .candidate-fields {
    grid-template-columns: 1fr;
  }
}
</style>
