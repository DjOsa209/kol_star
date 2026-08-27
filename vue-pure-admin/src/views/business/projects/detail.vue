<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { ElMessage, ElMessageBox } from "element-plus";
import echarts from "@/plugins/echarts";
import PlatformIconBadge from "@/components/PlatformIconBadge/index.vue";
import CooperationTypeTags from "@/components/CooperationTypeTags/index.vue";
import { fieldLabel } from "@/utils/fieldI18n";
import {
  addProjectResource,
  deleteProjectContent,
  deleteProjectResource,
  downloadProjectData,
  getProjectDetail,
  getProjectList,
  getProjectResourceOptions,
  renewProject,
  reportProjectInfluencer,
  searchOnlineProjectResource,
  syncCooperation,
  updateProject,
  updateProjectBudget,
  updateProjectContent,
  updateProjectResource,
  updateProjectStatus
} from "@/api/business";
import {
  countryOptionsWithLegacyValues,
  parseProjectTargetMarkets,
  serializeProjectTargetMarkets
} from "@/utils/worldCountries";

defineOptions({ name: "BusinessProjectDetail" });

type SectionKey = "collaboration" | "report" | "budget" | "campaignInfo";
type ReportScope = "campaign" | "influencer";
type ReportMetric = "views" | "clicks" | "cpm" | "cpc";
type CampaignTab = "overview" | "creators" | "content";
type PlatformMetric = "exposure" | "engagement";

const route = useRoute();
const router = useRouter();
const { locale } = useI18n();
const projects = ref<any[]>([]);
const project = ref<any>(null);
const stats = ref<any>({});
const budget = ref<any>({});
const cooperations = ref<any[]>([]);
const projectResources = ref<any[]>([]);
const deliverables = ref<any[]>([]);
const billingEvents = ref<any[]>([]);
const contentPosts = ref<any[]>([]);
const reportSummary = ref<any>({});
const loading = ref(false);
const selectedProjectId = ref<number | null>(
  Number(route.query.id || 0) || null
);
const activeSection = ref<SectionKey>("collaboration");
const activePipelineStage = ref("all");
const activeCooperation = ref<any>(null);
const reportScope = ref<ReportScope>("campaign");
const reportMetric = ref<ReportMetric>("cpm");
const reportViewBy = ref("audience");
const reportAudience = ref("all");
const reportPlatform = ref("all");
const reportCreative = ref("all");
const detailTab = ref("content");
const campaignTab = ref<CampaignTab>(
  route.query.contentId ? "content" : "overview"
);
const creatorSearch = ref("");
const creatorCategory = ref("all");
const creatorPlatform = ref("all");
const creatorTier = ref("all");
const contentSearch = ref("");
const contentPlatform = ref("all");
const contentSort = ref("latest");
const contentEditing = ref(false);
const contentSaving = ref(false);
const editingContentPost = ref<any>(null);
const websiteScreenshotLoading = ref(false);
const syncingContentIds = reactive<Record<string, boolean>>({});
const failedContentAvatarUrls = reactive(new Set<string>());
const attemptedWebsiteScreenshotIds = new Set<string>();
const scheduledContentTitleProjectIds = new Set<number>();
const contentTitleRefreshTimers = new Set<number>();
const contentEditForm = reactive({
  contentId: "",
  cooperationId: 0,
  resourceId: 0,
  platform: "Website",
  postUrl: ""
});
const editableContentPlatformOptions = [
  "TikTok",
  "YouTube",
  "Instagram",
  "X",
  "Facebook",
  "LinkedIn",
  "Reddit",
  "Website"
];
const chartRef = ref<HTMLDivElement>();
const platformDistributionChartRef = ref<HTMLDivElement>();
const platformPerformanceChartRef = ref<HTMLDivElement>();
const platformMetric = ref<PlatformMetric>("exposure");
let reportChart: ReturnType<typeof echarts.init> | undefined;
let platformDistributionChart: ReturnType<typeof echarts.init> | undefined;
let platformPerformanceChart: ReturnType<typeof echarts.init> | undefined;

const projectDialog = ref(false);
const budgetDialog = ref(false);
const renewDialog = ref(false);
const reportDialog = ref(false);
const creatorDialog = ref(false);
const creatorDialogMode = ref<"create" | "edit">("create");
const creatorOptions = ref<any[]>([]);
const creatorOptionsLoading = ref(false);
const creatorLibraryKeyword = ref("");
const onlineSearchExpanded = ref(false);
const onlineSearchLoading = ref(false);
const onlineSearchResult = ref<any>(null);
const submitting = ref(false);
const onlineSearchOptionValue = -1;

const projectForm = reactive({
  name: "",
  targetMarkets: [] as string[],
  language: "",
  campaignType: "",
  budget: 0,
  currency: "USD",
  status: "Active",
  owner: "",
  brief: "",
  cycleStartDate: "",
  cycleEndDate: "",
  reportUpdateDate: ""
});

const projectCountryOptions = computed(() =>
  countryOptionsWithLegacyValues(projectForm.targetMarkets)
);

const projectCycleRange = computed({
  get: () =>
    projectForm.cycleStartDate && projectForm.cycleEndDate
      ? [projectForm.cycleStartDate, projectForm.cycleEndDate]
      : [],
  set: (value: string[]) => {
    projectForm.cycleStartDate = value?.[0] || "";
    projectForm.cycleEndDate = value?.[1] || "";
  }
});

const budgetForm = reactive({ budget: 0 });
const renewForm = reactive({ cycleStartDate: "", cycleEndDate: "" });
const influencerReportForm = reactive({
  reason: "内容质量或数据异常",
  detail: ""
});
const creatorForm = reactive({
  resourceId: null as number | null,
  resourceName: "",
  resourceType: "KOL",
  category: "",
  market: "",
  platform: "YouTube",
  platformUrl: "",
  primaryContact: "",
  followers: 0,
  audienceSize: 0,
  collaboratorTier: ""
});
const onlineSearchForm = reactive({
  platform: "Instagram",
  query: "",
  resourceType: "KOL"
});

const visibleCreatorOptions = computed(() => {
  const keyword = creatorLibraryKeyword.value.trim().toLowerCase();
  if (!keyword) return creatorOptions.value;
  return creatorOptions.value.filter(item =>
    creatorOptionLabel(item).toLowerCase().includes(keyword)
  );
});

const navItems = [
  { key: "collaboration", label: "协作执行", icon: "ri:team-line" },
  { key: "report", label: "效果报告", icon: "ri:bar-chart-box-line" },
  { key: "budget", label: "预算", icon: "ri:flashlight-line" },
  { key: "campaignInfo", label: "项目信息", icon: "ri:file-list-3-line" }
] as const;

const pipelineStages = computed(() => {
  const stageDefinitions = [
    {
      key: "inviting",
      label: "邀约/报价",
      icon: "ri:file-check-line",
      description: "等待达人回复或确认价格"
    },
    {
      key: "confirmed",
      label: "确认合作",
      icon: "ri:user-search-line",
      description: "合作已确认，等待启动交付"
    },
    {
      key: "production",
      label: "内容制作",
      icon: "ri:movie-2-line",
      description: "脚本、稿件或内容正在制作"
    },
    {
      key: "pending_publish",
      label: "待发布",
      icon: "ri:calendar-event-line",
      description: "内容已确认，等待按计划发布"
    },
    {
      key: "published",
      label: "已发布",
      icon: "ri:checkbox-circle-line",
      description: "进入数据回收与复盘"
    }
  ];
  return stageDefinitions.map(stage => ({
    ...stage,
    count: cooperations.value.filter(
      item => cooperationStage(item) === stage.key
    ).length
  }));
});

const pipelineRows = computed(() =>
  activePipelineStage.value === "all"
    ? cooperations.value
    : cooperations.value.filter(
        item => cooperationStage(item) === activePipelineStage.value
      )
);

const pendingActions = computed(() =>
  cooperations.value
    .map(item => ({ ...item, action: cooperationAction(item) }))
    .filter(item => Boolean(item.action))
    .slice(0, 8)
);

const focusedCooperation = computed(() => {
  const activeId = Number(activeCooperation.value?.id || 0);
  if (activeId) {
    const current = cooperations.value.find(
      item => Number(item.id) === activeId
    );
    if (current) return current;
  }
  return (
    pendingActions.value[0] ||
    pipelineRows.value[0] ||
    cooperations.value[0] ||
    null
  );
});

const currentDeliverables = computed(() =>
  deliverables.value.filter(
    item =>
      Number(item.cooperationId) === Number(focusedCooperation.value?.id || 0)
  )
);

function youtubeThumbnailFromContentUrl(value: unknown) {
  const raw = String(value || "").trim();
  const candidate = raw.match(/https?:\/\/[^\s,，;；]+/)?.[0] || raw;
  if (!candidate) return "";
  try {
    const parsed = new URL(candidate);
    const host = parsed.hostname.toLowerCase().replace(/^www\./, "");
    let videoId = "";
    if (host === "youtu.be") {
      videoId = parsed.pathname.split("/").filter(Boolean)[0] || "";
    } else if (host.endsWith("youtube.com")) {
      videoId = parsed.searchParams.get("v") || "";
      if (!videoId) {
        const segments = parsed.pathname.split("/").filter(Boolean);
        if (["shorts", "embed"].includes(segments[0])) {
          videoId = segments[1] || "";
        }
      }
    }
    return videoId
      ? `https://i.ytimg.com/vi/${encodeURIComponent(videoId)}/hqdefault.jpg`
      : "";
  } catch {
    return "";
  }
}

function importedContentCover(item: any, postUrl: string) {
  if (
    String(item.platform || "")
      .trim()
      .toLowerCase() === "youtube"
  ) {
    const youtubeCover = youtubeThumbnailFromContentUrl(postUrl);
    if (youtubeCover) return youtubeCover;
  }
  return "";
}

function importedContentTitle(item: any) {
  const creativeName = String(item?.creativeName || "").trim();
  const cooperationType = String(item?.cooperationType || "").trim();
  if (
    creativeName &&
    creativeName !== cooperationType &&
    !["已导入发布内容", "已同步内容"].includes(creativeName)
  ) {
    return creativeName;
  }
  return contentTitleFromUrl(item?.finalLink || item?.deliverableLinks);
}

function contentTitleFromUrl(value: unknown) {
  const raw = String(value || "")
    .match(/https?:\/\/[^\s,，;；]+/)?.[0]
    ?.trim();
  if (!raw) return "";
  try {
    const parsed = new URL(raw);
    const segments = decodeURIComponent(parsed.pathname)
      .split("/")
      .filter(Boolean);
    const last = segments.at(-1) || "";
    if (last && !/^\d+$/.test(last)) {
      return last.replace(/[-_]+/g, " ").trim();
    }
    return `${parsed.hostname.replace(/^www\./, "")}${parsed.pathname}`;
  } catch {
    return "";
  }
}

const projectContentPosts = computed(() => {
  const seenLinks = new Set<string>();
  const posts = contentPosts.value
    .filter(post => {
      const link = String(post.postUrl || "")
        .trim()
        .toLowerCase();
      if (!link || seenLinks.has(link)) return !link;
      seenLinks.add(link);
      return true;
    })
    .map(post => ({
      ...post,
      cooperationId: contentCooperation(post)?.id || 0
    }));
  const importedPosts = cooperations.value.flatMap(item => {
    const postUrl = String(
      item.finalLink || item.deliverableLinks || ""
    ).trim();
    if (!postUrl || seenLinks.has(postUrl.toLowerCase())) return [];
    seenLinks.add(postUrl.toLowerCase());
    return [
      {
        id: `cooperation-${item.id}`,
        resourceId: item.resourceId,
        resourceName: item.resourceName,
        resourceAvatarUrl: item.resourceAvatarUrl,
        resourceAvatarRemoteUrl: item.resourceAvatarRemoteUrl,
        platformHandle: item.platformHandle,
        cooperationId: item.id,
        platform: item.contentPlatform || item.platform || "Website",
        cooperationType: item.cooperationType,
        contentType: item.contentType,
        title: importedContentTitle(item),
        description: item.notes,
        postUrl,
        coverUrl: item.contentCoverUrl || importedContentCover(item, postUrl),
        coverRemoteUrl: item.contentCoverRemoteUrl || "",
        coverLocalUrl: item.contentCoverLocalUrl || "",
        mediaType: "imported",
        publishedAt: item.publishTime || item.releaseDate || item.createdAt,
        viewCount: item.views || item.impressions,
        engagementCount: item.engagementCount,
        likeCount: item.engagementCount,
        commentCount: item.commentsCount,
        shareCount: 0,
        saveCount: 0
      }
    ];
  });
  return [...posts, ...importedPosts];
});

const projectCreators = computed(() => {
  const creatorGroups = new Map<string, any>();
  const source = projectResources.value.length
    ? projectResources.value
    : cooperations.value;
  source.forEach(item => {
    const name = creatorNameKey(item);
    const fallback = String(item.resourceId || `cooperation-${item.id}`);
    const key = name || fallback;
    const existing = creatorGroups.get(key);
    const platform = normalizePlatformName(item.platform);
    if (!existing) {
      const resourceIds = Array.isArray(item.resourceIds)
        ? item.resourceIds.map(Number).filter(Boolean)
        : [Number(item.resourceId)].filter(Boolean);
      const platforms = Array.isArray(item.platforms)
        ? item.platforms.map(normalizePlatformName).filter(Boolean)
        : [];
      if (platform && !platforms.includes(platform)) platforms.push(platform);
      creatorGroups.set(key, {
        ...item,
        resourceIds,
        resources: [{ ...item }],
        platforms
      });
      return;
    }
    if (
      cooperationStage(item) === "published" ||
      (!existing.resourceAvatarUrl && item.resourceAvatarUrl)
    ) {
      Object.assign(existing, item);
    }
    if (
      item.resourceId &&
      !existing.resourceIds.includes(Number(item.resourceId))
    ) {
      existing.resourceIds.push(Number(item.resourceId));
      existing.resources.push({ ...item });
    }
    if (platform && !existing.platforms.includes(platform)) {
      existing.platforms.push(platform);
    }
  });
  return Array.from(creatorGroups.values());
});

const creatorCategoryOptions = computed(() =>
  uniqueOptions(projectCreators.value.map(item => item.category))
);
const creatorPlatformOptions = computed(() =>
  uniqueOptions(projectCreators.value.flatMap(item => creatorPlatforms(item)))
);
const creatorTierOptions = computed(() =>
  uniqueOptions(projectCreators.value.map(item => item.collaboratorTier))
);

const filteredCreators = computed(() => {
  const keyword = creatorSearch.value.trim().toLowerCase();
  return projectCreators.value.filter(item => {
    if (
      creatorCategory.value !== "all" &&
      item.category !== creatorCategory.value
    )
      return false;
    if (
      creatorPlatform.value !== "all" &&
      !creatorPlatforms(item).includes(creatorPlatform.value)
    )
      return false;
    if (
      creatorTier.value !== "all" &&
      item.collaboratorTier !== creatorTier.value
    )
      return false;
    if (!keyword) return true;
    return [
      item.resourceName,
      ...creatorPlatforms(item),
      item.resourceType,
      item.category
    ]
      .filter(Boolean)
      .some(value => String(value).toLowerCase().includes(keyword));
  });
});

const influencerRows = computed(() =>
  filteredCreators.value.filter(item => !isMediaResource(item))
);
const mediaRows = computed(() =>
  filteredCreators.value.filter(item => isMediaResource(item))
);

const contentPlatforms = computed(() =>
  Array.from(
    new Set(
      projectContentPosts.value
        .map(item => String(item.platform || ""))
        .filter(Boolean)
    )
  )
);

const filteredContentPosts = computed(() => {
  const keyword = contentSearch.value.trim().toLowerCase();
  const rows = projectContentPosts.value.filter(item => {
    if (
      contentPlatform.value !== "all" &&
      item.platform !== contentPlatform.value
    )
      return false;
    if (!keyword) return true;
    return [
      item.title,
      item.description,
      item.resourceName,
      item.platformHandle
    ]
      .filter(Boolean)
      .some(value => String(value).toLowerCase().includes(keyword));
  });
  return [...rows].sort((left, right) => {
    if (contentSort.value === "views")
      return postExposure(right) - postExposure(left);
    if (contentSort.value === "engagement")
      return postEngagement(right) - postEngagement(left);
    return contentDateRank(right) - contentDateRank(left);
  });
});

const selectedContentPost = computed(() => {
  const contentId = String(route.query.contentId || "");
  if (!contentId) return null;
  return (
    projectContentPosts.value.find(item => String(item.id) === contentId) ||
    null
  );
});

const contentDetailView = computed(() => selectedContentPost.value);

const campaignOverview = computed(() => {
  const posts = projectContentPosts.value;
  const creatorNames = new Set(
    projectCreators.value
      .map(item =>
        String(item.resourceName || item.platformHandle || "")
          .trim()
          .toLowerCase()
      )
      .filter(Boolean)
  );
  const views =
    posts.reduce((sum, item) => sum + postExposure(item), 0) ||
    numberValue(stats.value.totalReach);
  const engagements =
    posts.reduce((sum, item) => sum + postEngagement(item), 0) ||
    numberValue(stats.value.totalEngagements);
  const paidRows = cooperations.value.filter(isPaidKOLCooperation);
  const paidResourceIds = new Set(
    paidRows.map(item => Number(item.resourceId)).filter(Boolean)
  );
  const paidPosts = posts.filter(item =>
    paidResourceIds.has(Number(item.resourceId))
  );
  const paidCost = paidRows.reduce(
    (sum, item) => sum + numberValue(item.quoteAmount),
    0
  );
  const paidExposure =
    paidPosts.reduce((sum, item) => sum + postExposure(item), 0) ||
    paidRows.reduce((sum, item) => sum + primaryReach(item), 0);
  const paidEngagement =
    paidPosts.reduce((sum, item) => sum + postEngagement(item), 0) ||
    paidRows.reduce((sum, item) => sum + numberValue(item.engagementCount), 0);
  return {
    collaborators: creatorNames.size,
    today: posts.filter(item => isToday(item.publishedAt)).length,
    posts: posts.length,
    engagements,
    views,
    engagementRate: ratioPercent(engagements, views),
    paidCost,
    cpm: paidExposure > 0 ? (paidCost * 1000) / paidExposure : 0,
    cpe: paidEngagement > 0 ? paidCost / paidEngagement : 0
  };
});

const platformPerformance = computed(() => {
  const grouped = new Map<
    string,
    {
      platform: string;
      contentCount: number;
      exposure: number;
      engagement: number;
      color: string;
    }
  >();
  projectContentPosts.value.forEach(post => {
    const platform = normalizePlatformName(post.platform);
    const current = grouped.get(platform) || {
      platform,
      contentCount: 0,
      exposure: 0,
      engagement: 0,
      color: platformColor(platform)
    };
    current.contentCount += 1;
    current.exposure += postExposure(post);
    current.engagement += postEngagement(post);
    grouped.set(platform, current);
  });
  return Array.from(grouped.values()).sort(
    (left, right) =>
      right.contentCount - left.contentCount ||
      right.exposure - left.exposure ||
      left.platform.localeCompare(right.platform)
  );
});

const platformTotals = computed(() =>
  platformPerformance.value.reduce(
    (summary, item) => {
      summary.content += item.contentCount;
      summary.exposure += item.exposure;
      summary.engagement += item.engagement;
      return summary;
    },
    { content: 0, exposure: 0, engagement: 0 }
  )
);

const segments = computed(() => reportSummary.value?.segments || []);
const audienceOptions = computed(() =>
  uniqueOptions(segments.value.map(item => item.audienceSegment))
);
const platformOptions = computed(() =>
  uniqueOptions(segments.value.map(item => item.platform))
);
const creativeOptions = computed(() =>
  uniqueOptions(segments.value.map(item => item.creativeName))
);

const filteredSegments = computed(() =>
  segments.value.filter(item => {
    if (
      reportAudience.value !== "all" &&
      item.audienceSegment !== reportAudience.value
    )
      return false;
    if (
      reportPlatform.value !== "all" &&
      item.platform !== reportPlatform.value
    )
      return false;
    if (
      reportCreative.value !== "all" &&
      item.creativeName !== reportCreative.value
    )
      return false;
    return true;
  })
);

const visibleReportSummary = computed(() =>
  buildReportSummary(filteredSegments.value)
);
const isPaused = computed(
  () =>
    /paused|暂停/i.test(String(project.value?.status || "")) ||
    Boolean(project.value?.pausedAt)
);
const cycleLabel = computed(
  () =>
    `${project.value?.cycleStartDate || "-"} - ${project.value?.cycleEndDate || "-"}`
);
const createdDateLabel = computed(() => dateText(project.value?.createdAt));
const costToDate = computed(() => numberValue(budget.value?.costToDate));
const projectBudget = computed(() =>
  numberValue(budget.value?.budget ?? project.value?.budget)
);
const activeStatusLabel = computed(() =>
  isPaused.value ? "已暂停" : "进行中"
);
const activeStatusType = computed(() => (isPaused.value ? "info" : "success"));

async function loadProjects() {
  const res = await getProjectList();
  if (res.code === 0) {
    projects.value = res.data.list;
    if (!selectedProjectId.value && projects.value.length > 0) {
      selectedProjectId.value = Number(projects.value[0].id);
    }
  }
}

async function loadDetail() {
  if (!selectedProjectId.value) return;
  loading.value = true;
  try {
    const res = await getProjectDetail({
      id: selectedProjectId.value,
      _t: Date.now()
    });
    if (res.code !== 0) return;
    project.value = res.data.project;
    stats.value = res.data.stats || {};
    cooperations.value = res.data.cooperations || [];
    projectResources.value = res.data.projectResources || [];
    deliverables.value = res.data.deliverables || [];
    contentPosts.value = res.data.contentPosts || [];
    reportSummary.value = res.data.reportSummary || {};
    budget.value = res.data.budget || {};
    billingEvents.value = res.data.billingEvents || [];
    scheduleContentTitleReload(
      Number(project.value?.id || selectedProjectId.value)
    );
    activeCooperation.value = focusedCooperation.value;
    syncFormsFromProject();
    await nextTick();
    renderReportChart();
  } finally {
    loading.value = false;
  }
}

function scheduleContentTitleReload(projectId: number) {
  if (!projectId || scheduledContentTitleProjectIds.has(projectId)) return;
  const hasMissingTitle = cooperations.value.some(item => {
    const title = String(item.creativeName || "").trim();
    const cooperationType = String(item.cooperationType || "").trim();
    const hasContentURL = Boolean(
      String(item.finalLink || item.deliverableLinks || "").trim()
    );
    return (
      hasContentURL &&
      (!title ||
        title === cooperationType ||
        ["已导入发布内容", "已同步内容"].includes(title))
    );
  });
  if (!hasMissingTitle) return;

  scheduledContentTitleProjectIds.add(projectId);
  const timer = window.setTimeout(() => {
    contentTitleRefreshTimers.delete(timer);
    if (Number(selectedProjectId.value) === projectId) void loadDetail();
  }, 7500);
  contentTitleRefreshTimers.add(timer);
}

function syncFormsFromProject() {
  if (!project.value) return;
  Object.assign(projectForm, {
    name: project.value.name || "",
    targetMarkets: parseProjectTargetMarkets(project.value.targetMarket),
    language: project.value.language || "",
    campaignType: project.value.campaignType || "",
    budget: numberValue(project.value.budget),
    currency: project.value.currency || "USD",
    status: project.value.status || "Active",
    owner: project.value.owner || "",
    brief: project.value.brief || "",
    cycleStartDate: project.value.cycleStartDate || "",
    cycleEndDate: project.value.cycleEndDate || "",
    reportUpdateDate: project.value.reportUpdateDate || ""
  });
  budgetForm.budget = numberValue(project.value.budget);
  renewForm.cycleStartDate = project.value.cycleStartDate || "";
  renewForm.cycleEndDate = project.value.cycleEndDate || "";
}

function handleProjectChange(value: number) {
  selectedProjectId.value = value;
  activePipelineStage.value = "all";
  activeCooperation.value = null;
  router.replace({
    path: "/business/projects/detail",
    query: { id: String(value) }
  });
  loadDetail();
}

function goBack() {
  if (window.history.length > 1) {
    router.back();
    return;
  }
  router.push("/business/projects");
}

function openPost(post: any) {
  const url = String(post?.postUrl || "").trim();
  if (!url) {
    ElMessage.info("该内容暂未同步发布链接");
    return;
  }
  window.open(url, "_blank", "noopener,noreferrer");
}

function useRemotePostCover(event: Event, post: any) {
  const image = event.currentTarget as HTMLImageElement | null;
  if (!image) {
    return;
  }
  const fallbackURL = String(post?.coverRemoteUrl || "").trim();
  if (fallbackURL && image.dataset.remoteFallbackApplied !== "1") {
    image.dataset.remoteFallbackApplied = "1";
    image.src = fallbackURL;
    return;
  }
  post.coverUrl = "";
}

function selectCampaignTab(tab: CampaignTab) {
  campaignTab.value = tab;
  if (route.query.contentId) {
    const query = { ...route.query };
    delete query.contentId;
    router.replace({ path: route.path, query });
  }
}

function openContentDetail(post: any) {
  if (post?.id === undefined || post?.id === null) return;
  router.push({
    path: route.path,
    query: { ...route.query, contentId: String(post.id) }
  });
}

function closeContentDetail() {
  const query = { ...route.query };
  delete query.contentId;
  router.replace({ path: route.path, query });
}

function prepareContentEdit(post: any) {
  const cooperation = contentCooperation(post);
  if (!cooperation?.id) {
    ElMessage.warning("未找到该内容对应的合作记录，暂时无法编辑");
    return false;
  }
  Object.assign(contentEditForm, {
    contentId: String(post.id),
    cooperationId: Number(cooperation.id),
    resourceId: Number(post.resourceId),
    platform: normalizePlatformName(post.platform),
    postUrl: String(post.postUrl || "").trim()
  });
  editingContentPost.value = post;
  contentEditing.value = true;
  return true;
}

function editContentFromCard(post: any) {
  prepareContentEdit(post);
}

function cancelContentEdit() {
  contentEditing.value = false;
}

function resetContentEdit() {
  editingContentPost.value = null;
}

async function saveContentEdit() {
  if (!project.value || !editingContentPost.value) return;
  const postUrl = contentEditForm.postUrl.trim();
  try {
    const parsed = new URL(postUrl);
    if (!["http:", "https:"].includes(parsed.protocol)) throw new Error();
  } catch {
    ElMessage.warning("请输入以 http:// 或 https:// 开头的有效内容链接");
    return;
  }
  contentSaving.value = true;
  try {
    const res = await updateProjectContent({
      projectId: Number(project.value.id),
      ...contentEditForm,
      postUrl
    });
    if (res.code !== 0) {
      ElMessage.warning(res.message || "合作内容更新失败");
      return;
    }
    contentEditing.value = false;
    await loadDetail();
    if (res.data?.previewWarning) {
      ElMessage.warning(res.data.previewWarning);
    } else {
      ElMessage.success("内容链接与平台已更新");
    }
  } finally {
    contentSaving.value = false;
  }
}

async function ensureWebsiteScreenshot(post: any) {
  if (
    !project.value ||
    !post ||
    !usesPageScreenshot(post) ||
    post.coverUrl ||
    contentEditing.value
  ) {
    return;
  }
  const contentId = String(post.id || "");
  if (!contentId || attemptedWebsiteScreenshotIds.has(contentId)) return;
  const cooperation = contentCooperation(post);
  if (!cooperation?.id) return;
  attemptedWebsiteScreenshotIds.add(contentId);
  websiteScreenshotLoading.value = true;
  try {
    const res = await updateProjectContent({
      projectId: Number(project.value.id),
      contentId,
      cooperationId: Number(cooperation.id),
      resourceId: Number(post.resourceId),
      platform: normalizePlatformName(post.platform),
      postUrl: String(post.postUrl || "").trim()
    });
    if (res.code === 0 && res.data?.coverUrl) {
      await loadDetail();
    } else if (res.data?.previewWarning) {
      ElMessage.warning(res.data.previewWarning);
    }
  } finally {
    websiteScreenshotLoading.value = false;
  }
}

function contentOperationKey(post: any) {
  return String(contentCooperation(post)?.id || post?.id || "");
}

function clearContentSyncFields(post: any, cooperation: any) {
  const resourceId = Number(post?.resourceId || cooperation?.resourceId || 0);
  const records = [
    post,
    cooperation,
    ...cooperations.value.filter(
      item => Number(item.resourceId) === resourceId
    ),
    ...projectResources.value.filter(
      item => Number(item.resourceId) === resourceId
    ),
    ...contentPosts.value.filter(item => Number(item.resourceId) === resourceId)
  ];
  new Set(records.filter(Boolean)).forEach(item => {
    Object.assign(item, {
      platformUrl: "",
      resourceAvatarUrl: "",
      resourceAvatarRemoteUrl: "",
      contentCoverUrl: "",
      contentCoverLocalUrl: "",
      contentCoverRemoteUrl: "",
      coverUrl: "",
      coverLocalUrl: "",
      coverRemoteUrl: ""
    });
  });
}

async function syncContentFromCard(post: any) {
  if (!project.value) return;
  const cooperation = contentCooperation(post);
  if (!cooperation?.id) {
    ElMessage.warning("未找到该内容对应的合作记录，暂时无法同步");
    return;
  }
  const key = contentOperationKey(post);
  if (!key || syncingContentIds[key]) return;
  syncingContentIds[key] = true;
  clearContentSyncFields(post, cooperation);
  try {
    const res = isWebsiteContent(post)
      ? await updateProjectContent({
          projectId: Number(project.value.id),
          contentId: String(post.id),
          cooperationId: Number(cooperation.id),
          resourceId: Number(post.resourceId),
          platform: "Website",
          postUrl: String(post.postUrl || "").trim()
        })
      : await syncCooperation({ id: Number(cooperation.id) });
    if (res.code !== 0) {
      ElMessage.warning(res.message || "内容同步失败");
      return;
    }
    if (!isWebsiteContent(post) && !res.data?.synced) {
      ElMessage.warning(res.data?.message || "平台未返回可用的内容数据");
      return;
    }
    await loadDetail();
    if (res.data?.previewWarning) {
      ElMessage.warning(res.data.previewWarning);
    } else {
      ElMessage.success(res.data?.message || "内容数据已同步");
    }
  } finally {
    syncingContentIds[key] = false;
  }
}

async function deleteContentFromCard(post: any) {
  if (!project.value) return;
  const cooperation = contentCooperation(post);
  if (!cooperation?.id) {
    ElMessage.warning("未找到该内容对应的合作记录，暂时无法删除");
    return;
  }
  try {
    await ElMessageBox.confirm(
      `确认从当前项目删除「${contentDisplayTitle(post)}」吗？达人/媒体资料和平台内容库不会被删除。`,
      "删除合作内容",
      {
        type: "warning",
        confirmButtonText: "删除",
        cancelButtonText: "取消"
      }
    );
  } catch {
    return;
  }
  const res = await deleteProjectContent({
    projectId: Number(project.value.id),
    cooperationId: Number(cooperation.id),
    resourceId: Number(post.resourceId)
  });
  if (res.code !== 0) {
    ElMessage.warning(res.message || "内容删除失败");
    return;
  }
  await loadDetail();
  ElMessage.success("合作内容已删除");
}

function handleContentCardCommand(command: string, post: any) {
  if (command === "edit") {
    editContentFromCard(post);
    return;
  }
  if (command === "sync") {
    void syncContentFromCard(post);
    return;
  }
  if (command === "delete") void deleteContentFromCard(post);
}

function contentResource(post: any) {
  return projectCreators.value.find(
    item => Number(item.resourceId) === Number(post?.resourceId)
  );
}

function contentFollowers(post: any) {
  const resource = contentResource(post);
  return (
    numberValue(resource?.audienceSize) || numberValue(resource?.followers)
  );
}

function isWebsiteContent(post: any) {
  return normalizePlatformName(post?.platform) === "Website";
}

function usesPageScreenshot(post: any) {
  return ["Website", "X"].includes(normalizePlatformName(post?.platform));
}

function contentAudienceLabel(post: any) {
  if (!isMediaResource(contentResource(post))) return "粉丝";
  return isWebsiteContent(post) ? "月访问量" : "月独立访客（UMV）";
}

function contentExposureLabel(post: any) {
  return isWebsiteContent(post) ? "访问量" : "播放量";
}

function contentExposureHint(post: any) {
  return isWebsiteContent(post)
    ? "当前网页累计访问 / 曝光"
    : "当前内容累计播放 / 曝光";
}

function contentOpenLabel(post: any) {
  return isWebsiteContent(post) ? "打开网页" : "前往平台查看";
}

function contentAvatar(post: any) {
  const resource = contentResource(post);
  const localURL = String(
    post?.resourceAvatarUrl || resource?.resourceAvatarUrl || ""
  ).trim();
  const remoteURL = String(
    post?.resourceAvatarRemoteUrl || resource?.resourceAvatarRemoteUrl || ""
  ).trim();
  return (
    (localURL && !failedContentAvatarUrls.has(localURL) ? localURL : "") ||
    remoteURL ||
    localURL
  );
}

function useRemoteContentAvatar(post: any) {
  const resource = contentResource(post);
  const localURL = String(
    post?.resourceAvatarUrl || resource?.resourceAvatarUrl || ""
  ).trim();
  if (localURL) failedContentAvatarUrls.add(localURL);
}

function contentCooperation(post: any) {
  const cooperationId = Number(post?.cooperationId || 0);
  if (cooperationId) {
    const matched = cooperations.value.find(
      item => Number(item.id) === cooperationId
    );
    if (matched) return matched;
  }
  const postURL = normalizedContentUrl(post?.postUrl);
  if (!postURL) return null;
  return (
    cooperations.value.find(item => {
      if (Number(item.resourceId) !== Number(post?.resourceId)) return false;
      return [item.finalLink, item.deliverableLinks].some(value => {
        const raw = String(value || "");
        const links = raw.match(/https?:\/\/[^\s,，;；]+/g) || [raw];
        return links.some(link => normalizedContentUrl(link) === postURL);
      });
    }) || null
  );
}

function contentCooperationType(post: any) {
  return String(
    post?.cooperationType || contentCooperation(post)?.cooperationType || ""
  ).trim();
}

function contentTypeTag(post: any) {
  return String(
    post?.contentType || contentCooperation(post)?.contentType || ""
  ).trim();
}

function contentDisplayTitle(post: any) {
  return (
    String(post?.title || post?.description || "").trim() ||
    contentTitleFromUrl(post?.postUrl) ||
    "标题获取中"
  );
}

function isViralContent(post: any) {
  return postExposure(post) > 10_000_000;
}

function isMediaResource(row: any) {
  return /媒体|media/i.test(String(row?.resourceType || ""));
}

function normalizedContentUrl(value: unknown) {
  return String(value || "")
    .trim()
    .replace(/[?#].*$/, "")
    .replace(/\/$/, "")
    .toLowerCase();
}

function creatorNameKey(row: any) {
  return String(row?.resourceName || row?.name || "")
    .trim()
    .replace(/\s+/g, " ")
    .toLocaleLowerCase();
}

function creatorResourceIds(row: any) {
  const ids = Array.isArray(row?.resourceIds)
    ? row.resourceIds
    : [row?.resourceId];
  return new Set(ids.map(Number).filter(Boolean));
}

function projectCooperationsForCreator(row: any) {
  const name = creatorNameKey(row);
  const resourceIds = creatorResourceIds(row);
  return cooperations.value.filter(item => {
    if (name) return creatorNameKey(item) === name;
    return resourceIds.has(Number(item.resourceId));
  });
}

function creatorPlatforms(row: any) {
  const platforms = new Set<string>();
  const addPlatform = (value: unknown) => {
    if (!String(value || "").trim()) return;
    const platform = normalizePlatformName(value);
    if (platform) platforms.add(platform);
  };
  if (Array.isArray(row?.platforms)) {
    row.platforms.forEach(addPlatform);
  }
  (Array.isArray(row?.resources) ? row.resources : [row]).forEach(item =>
    addPlatform(item?.platform)
  );
  projectCooperationsForCreator(row).forEach(item =>
    addPlatform(item.contentPlatform || item.platform)
  );
  const resourceIds = creatorResourceIds(row);
  projectContentPosts.value
    .filter(post => resourceIds.has(Number(post.resourceId)))
    .forEach(post => addPlatform(post.platform));
  return Array.from(platforms);
}

function cooperationContentUrls(row: any) {
  const urls = new Set<string>();
  projectCooperationsForCreator(row).forEach(item => {
    [item.finalLink, item.deliverableLinks].forEach(value => {
      const raw = String(value || "").trim();
      const matches = raw.match(/https?:\/\/[^\s,，;；]+/g) || [];
      (matches.length ? matches : raw ? [raw] : []).forEach(url => {
        const normalized = normalizedContentUrl(url);
        if (normalized) urls.add(normalized);
      });
    });
  });
  return urls;
}

function projectPostsForResource(row: any) {
  const resourceIds = creatorResourceIds(row);
  const projectLinks = cooperationContentUrls(row);
  if (!projectLinks.size) return [];
  return projectContentPosts.value.filter(post => {
    if (!resourceIds.has(Number(post.resourceId))) return false;
    return projectLinks.has(normalizedContentUrl(post.postUrl));
  });
}

function projectContentCount(row: any) {
  return projectPostsForResource(row).length;
}

function projectAudience(row: any) {
  const resources = Array.isArray(row?.resources) ? row.resources : [row];
  return resources.reduce((total, resource) => {
    if (isMediaResource(resource)) {
      return total + numberValue(resource.audienceSize);
    }
    return (
      total +
      (numberValue(resource.followers) || numberValue(resource.audienceSize))
    );
  }, 0);
}

function projectExposure(row: any) {
  const total = projectPostsForResource(row).reduce(
    (sum, post) => sum + postExposure(post),
    0
  );
  return (
    total ||
    projectCooperationsForCreator(row).reduce(
      (sum, item) => sum + numberValue(item.views),
      0
    )
  );
}

function projectEngagement(row: any) {
  const total = projectPostsForResource(row).reduce(
    (sum, post) => sum + postEngagement(post),
    0
  );
  return (
    total ||
    projectCooperationsForCreator(row).reduce(
      (sum, item) => sum + numberValue(item.engagementCount),
      0
    )
  );
}

function projectCPM(row: any) {
  if (Object.prototype.hasOwnProperty.call(row || {}, "projectCpm")) {
    return numberValue(row.projectCpm);
  }
  const totals = projectCooperationsForCreator(row).reduce(
    (result, item) => {
      result.cost += numberValue(item.quoteAmount);
      result.views += numberValue(item.views);
      return result;
    },
    { cost: 0, views: 0 }
  );
  return totals.views > 0 ? (totals.cost / totals.views) * 1000 : 0;
}

function latestProjectPost(row: any) {
  return [...projectPostsForResource(row)].sort(
    (left, right) => contentDateRank(right) - contentDateRank(left)
  )[0];
}

function contentDateRank(post: any) {
  const value = post?.publishedAt || post?.releaseDate || post?.updatedAt;
  const date = new Date(Number(value) || String(value || ""));
  return Number.isNaN(date.getTime()) ? 0 : date.getTime();
}

function openResourceProfile(row: any) {
  const resourceId = Number(row?.resourceId || 0);
  if (!resourceId) return;
  router.push({
    path: "/business/resources",
    query: { resourceId: String(resourceId) }
  });
}

function sortByContentCount(left: any, right: any) {
  return projectContentCount(left) - projectContentCount(right);
}

function sortByAudience(left: any, right: any) {
  return projectAudience(left) - projectAudience(right);
}

function sortByExposure(left: any, right: any) {
  return projectExposure(left) - projectExposure(right);
}

function sortByEngagement(left: any, right: any) {
  return projectEngagement(left) - projectEngagement(right);
}

function sortByCPM(left: any, right: any) {
  return projectCPM(left) - projectCPM(right);
}

function resetCreatorForm() {
  Object.assign(creatorForm, {
    resourceId: null,
    resourceName: "",
    resourceType: "KOL",
    category: "",
    market: "",
    platform: "YouTube",
    platformUrl: "",
    primaryContact: "",
    followers: 0,
    audienceSize: 0,
    collaboratorTier: ""
  });
  creatorLibraryKeyword.value = "";
  onlineSearchExpanded.value = false;
  onlineSearchResult.value = null;
  Object.assign(onlineSearchForm, {
    platform: "Instagram",
    query: "",
    resourceType: "KOL"
  });
}

async function openCreateProjectResource() {
  if (!project.value) return;
  resetCreatorForm();
  creatorDialogMode.value = "create";
  creatorDialog.value = true;
  creatorOptionsLoading.value = true;
  try {
    const res = await getProjectResourceOptions({
      projectId: project.value.id
    });
    creatorOptions.value =
      res.code === 0 && Array.isArray(res.data) ? res.data : [];
  } finally {
    creatorOptionsLoading.value = false;
  }
}

function openEditProjectResource(row: any) {
  resetCreatorForm();
  creatorDialogMode.value = "edit";
  Object.assign(creatorForm, {
    resourceId: Number(row.resourceId || 0) || null,
    resourceName: row.resourceName || "",
    resourceType: row.resourceType || "KOL",
    category: row.category || "",
    market: row.market || row.country || "",
    platform: row.platform || "YouTube",
    platformUrl: row.platformUrl || "",
    primaryContact: row.primaryContact || "",
    followers: numberValue(row.followers),
    audienceSize: numberValue(row.audienceSize),
    collaboratorTier: row.collaboratorTier || ""
  });
  creatorDialog.value = true;
}

function creatorOptionLabel(row: any) {
  return [row.resourceName, row.resourceType, row.platform]
    .filter(Boolean)
    .join(" · ");
}

function filterCreatorOptions(value: string) {
  creatorLibraryKeyword.value = value;
}

function handleCreatorOptionChange(value: number | null) {
  if (value !== onlineSearchOptionValue) {
    onlineSearchExpanded.value = false;
    onlineSearchResult.value = null;
    return;
  }
  creatorForm.resourceId = null;
  onlineSearchForm.query = creatorLibraryKeyword.value.trim();
  onlineSearchExpanded.value = true;
}

async function runOnlineCreatorSearch() {
  if (!project.value) return;
  if (!onlineSearchForm.platform || !onlineSearchForm.query.trim()) {
    ElMessage.warning("请选择平台并输入主页链接、@handle 或账号");
    return;
  }
  onlineSearchLoading.value = true;
  onlineSearchResult.value = null;
  try {
    const res = await searchOnlineProjectResource({
      projectId: project.value.id,
      ...onlineSearchForm
    });
    if (res.code !== 0) {
      ElMessage.warning(res.message || "全网搜索失败");
      return;
    }
    const resource = res.data?.resource;
    if (!resource?.resourceId) {
      ElMessage.warning("平台接口未返回有效账号");
      return;
    }
    if (
      !creatorOptions.value.some(
        item => Number(item.resourceId) === Number(resource.resourceId)
      )
    ) {
      creatorOptions.value.unshift(resource);
    }
    creatorForm.resourceId = Number(resource.resourceId);
    onlineSearchResult.value = resource;
    ElMessage.success(
      res.data?.created
        ? "已从平台找到账号并同步到全球资源库"
        : "已在全球资源库中找到该账号"
    );
  } finally {
    onlineSearchLoading.value = false;
  }
}

async function submitProjectResource() {
  if (!project.value) return;
  if (!creatorForm.resourceId) {
    ElMessage.warning("请选择达人或媒体");
    return;
  }
  if (
    creatorDialogMode.value === "edit" &&
    (!creatorForm.resourceName.trim() || !creatorForm.resourceType)
  ) {
    ElMessage.warning("请填写达人/媒体名称和类型");
    return;
  }
  submitting.value = true;
  try {
    const res =
      creatorDialogMode.value === "create"
        ? await addProjectResource({
            projectId: project.value.id,
            resourceId: creatorForm.resourceId,
            source: "项目手动添加",
            status: "已关联"
          })
        : await updateProjectResource({
            projectId: project.value.id,
            ...creatorForm
          });
    if (res.code !== 0) {
      ElMessage.warning(res.message || "保存失败");
      return;
    }
    ElMessage.success(
      creatorDialogMode.value === "create"
        ? "达人/媒体已添加"
        : "达人/媒体已更新"
    );
    creatorDialog.value = false;
    await loadDetail();
  } finally {
    submitting.value = false;
  }
}

async function removeProjectResource(row: any) {
  if (!project.value) return;
  try {
    await ElMessageBox.confirm(
      `确认从当前项目删除「${row.resourceName || "该达人/媒体"}」吗？其项目合作记录和关联内容也会一并删除，但不会从全球资源库删除。`,
      "删除达人/媒体",
      {
        type: "warning",
        confirmButtonText: "删除",
        cancelButtonText: "取消"
      }
    );
  } catch {
    return;
  }
  const res = await deleteProjectResource({
    projectId: project.value.id,
    resourceId: row.resourceId
  });
  if (res.code !== 0) {
    ElMessage.warning(res.message || "删除失败");
    return;
  }
  ElMessage.success("已从项目删除达人/媒体");
  await loadDetail();
}

function setSection(section: SectionKey) {
  activeSection.value = section;
  if (section === "report") nextTick(renderReportChart);
}

function numberValue(value: unknown) {
  const number = Number(value || 0);
  return Number.isFinite(number) ? number : 0;
}

function formatCount(value: unknown) {
  const number = numberValue(value);
  if (number <= 0) return "-";
  if (number >= 1000000) return `${(number / 1000000).toFixed(1)}M`;
  if (number >= 1000) return `${(number / 1000).toFixed(1)}K`;
  return number.toLocaleString(locale.value === "en" ? "en-US" : "zh-CN");
}

function moneyText(
  value: unknown,
  currency = project.value?.currency || "USD"
) {
  const number = numberValue(value);
  if (number <= 0) return "-";
  const formatted = number.toLocaleString("en-US", {
    maximumFractionDigits: number >= 1000 ? 0 : 2
  });
  return `${currency === "USD" ? "$" : `${currency} `}${formatted}`;
}

function dateText(value: unknown) {
  if (!value) return "-";
  const raw = String(value);
  if (/^\d{4}-\d{2}-\d{2}/.test(raw)) return raw.slice(0, 10);
  const date = new Date(Number(value) || raw);
  if (Number.isNaN(date.getTime())) return "-";
  return [
    date.getFullYear(),
    String(date.getMonth() + 1).padStart(2, "0"),
    String(date.getDate()).padStart(2, "0")
  ].join("-");
}

function isToday(value: unknown) {
  if (!value) return false;
  const date = new Date(Number(value) || String(value));
  if (Number.isNaN(date.getTime())) return false;
  const today = new Date();
  return date.toDateString() === today.toDateString();
}

function primaryReach(row: any) {
  return numberValue(row.impressions) || numberValue(row.views);
}

function postExposure(row: any) {
  return (
    numberValue(row.viewCount) ||
    numberValue(row.impressions) ||
    numberValue(row.views)
  );
}

function postEngagement(row: any) {
  const totalEngagement = numberValue(row.engagementCount);
  if (totalEngagement > 0) {
    return totalEngagement + numberValue(row.commentCount);
  }
  return [
    "likeCount",
    "commentCount",
    "shareCount",
    "saveCount",
    "favoriteCount"
  ].reduce((sum, key) => sum + numberValue(row[key]), 0);
}

function isPaidKOLCooperation(row: any) {
  const resourceType = String(row.resourceType || "");
  return numberValue(row.quoteAmount) > 0 && !/媒体|media/i.test(resourceType);
}

function normalizePlatformName(value: unknown) {
  const platform = String(value || "Website").trim();
  const normalized = platform.toLowerCase();
  if (/tik\s?tok/.test(normalized)) return "TikTok";
  if (/you\s?tube/.test(normalized)) return "YouTube";
  if (/instagram|\big\b/.test(normalized)) return "Instagram";
  if (/facebook|\bfb\b/.test(normalized)) return "Facebook";
  if (/twitter|^x$/.test(normalized)) return "X";
  if (/linkedin/.test(normalized)) return "LinkedIn";
  if (/reddit/.test(normalized)) return "Reddit";
  if (/podcast|播客/.test(normalized)) return "Podcast";
  if (/website|web|网站/.test(normalized)) return "Website";
  return platform || "Website";
}

function platformColor(platform: string) {
  const colors: Record<string, string> = {
    TikTok: "#111827",
    YouTube: "#ef4444",
    Instagram: "#c026d3",
    Facebook: "#2563eb",
    X: "#64748b",
    LinkedIn: "#0a66c2",
    Reddit: "#f97316",
    Podcast: "#7c3aed",
    Website: "#0f9f8f"
  };
  return colors[platform] || "#5b8def";
}

function ratioPercent(numerator: unknown, denominator: unknown) {
  const top = numberValue(numerator);
  const bottom = numberValue(denominator);
  if (top <= 0 || bottom <= 0) return "-";
  const percent = (top / bottom) * 100;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

function cpmValue(cost: unknown, reach: unknown) {
  const costNumber = numberValue(cost);
  const reachNumber = numberValue(reach);
  if (costNumber <= 0 || reachNumber <= 0) return 0;
  return (costNumber / reachNumber) * 1000;
}

function cpcValue(cost: unknown, clicks: unknown) {
  const costNumber = numberValue(cost);
  const clicksNumber = numberValue(clicks);
  if (costNumber <= 0 || clicksNumber <= 0) return 0;
  return costNumber / clicksNumber;
}

function cooperationStage(row: any) {
  const status = `${row.status || ""} ${row.deliverableStatus || ""}`;
  if (
    /已发布|已完成|完成发布|数据回收|completed|approved|ads published/i.test(
      status
    )
  )
    return "published";
  if (/待发布|排期|发布中|pending publish/i.test(status))
    return "pending_publish";
  if (/制作|脚本|稿件|审核|修改|交付中|production|review/i.test(status))
    return "production";
  if (/确认合作|已确认|合作建立|待启动|confirmed/i.test(status))
    return "confirmed";
  return "inviting";
}

function cooperationStageLabel(row: any) {
  return (
    pipelineStages.value.find(item => item.key === cooperationStage(row))
      ?.label || "Review campaign"
  );
}

function cooperationStageTag(row: any) {
  const stage = cooperationStage(row);
  if (stage === "published") return "success";
  if (stage === "pending_publish") return "warning";
  if (stage === "production") return "primary";
  if (stage === "confirmed") return "info";
  return "warning";
}

function cooperationAction(row: any) {
  const stage = cooperationStage(row);
  if (stage === "inviting") return "确认报价与合作意向";
  if (stage === "confirmed") return "启动内容制作与交付";
  if (stage === "production") return "审核内容或跟进修改";
  if (stage === "pending_publish") return "确认发布排期与链接";
  if (
    stage === "published" &&
    primaryReach(row) <= 0 &&
    numberValue(row.clicks) <= 0
  )
    return "回收发布效果数据";
  return "";
}

function updatedTimeText(row: any) {
  const value = row.updatedAt || row.createdAt || row.releaseDate;
  if (!value) return "等待跟进";
  const timestamp = Number(value) || new Date(value).getTime();
  if (!Number.isFinite(timestamp)) return "等待跟进";
  const days = Math.max(
    0,
    Math.floor((Date.now() - timestamp) / (24 * 60 * 60 * 1000))
  );
  if (days === 0) return "今天有更新";
  return `已等待 ${days} 天`;
}

function cleanDisplayText(value: unknown) {
  const blockedBrandPattern = new RegExp(
    String.fromCharCode(65, 104, 97, 67, 114, 101, 97, 116, 111, 114),
    "gi"
  );
  return String(value || "")
    .replace(blockedBrandPattern, "系统")
    .replace(/Campaign/gi, "项目")
    .replace(/\s+项目/g, "项目");
}

function deliverableTitleText(item: any) {
  const key = String(item.stageKey || "").toLowerCase();
  const title = String(item.title || "").toLowerCase();
  if (key === "final_link" || title.includes("final link"))
    return "最终发布链接";
  if (title.includes("video draft")) return "视频草稿";
  if (title.includes("idea") || title.includes("script")) return "创意/脚本";
  if (title.includes("kickoff")) return "启动制作";
  if (title.includes("deal confirmed")) return "合作确认";
  if (title.includes("influencer applied")) return "达人申请/加入";
  return cleanDisplayText(item.title || "交付节点");
}

function deliverableStatusText(status: unknown) {
  const value = String(status || "").toLowerCase();
  if (value === "completed") return "已完成";
  if (value === "approved") return "已通过";
  if (value === "skipped") return "已跳过";
  if (value === "submitted") return "已提交";
  if (value === "rejected") return "已驳回";
  if (value === "pending") return "待处理";
  return cleanDisplayText(status || "已提交");
}

function openExecutionDetail(row: any) {
  activeCooperation.value = row;
}

function executionRowClassName({ row }: any) {
  return Number(row.id) === Number(focusedCooperation.value?.id)
    ? "execution-selected-row"
    : "";
}

function uniqueOptions(values: any[]) {
  return Array.from(
    new Set(values.map(value => String(value || "").trim()).filter(Boolean))
  );
}

function buildReportSummary(rows: any[]) {
  const summary = rows.reduce(
    (acc, row) => {
      acc.forecastViews += numberValue(row.forecastViews);
      acc.actualViews += numberValue(row.actualViews);
      acc.forecastClicks += numberValue(row.forecastClicks);
      acc.actualClicks += numberValue(row.actualClicks);
      acc.forecastCost += numberValue(row.forecastCost);
      acc.actualCost += numberValue(row.actualCost);
      return acc;
    },
    {
      forecastViews: 0,
      actualViews: 0,
      forecastClicks: 0,
      actualClicks: 0,
      forecastCost: 0,
      actualCost: 0
    }
  );
  return {
    ...summary,
    forecastCPM: cpmValue(summary.forecastCost, summary.forecastViews),
    actualCPM: cpmValue(summary.actualCost, summary.actualViews),
    forecastCPC: cpcValue(summary.forecastCost, summary.forecastClicks),
    actualCPC: cpcValue(summary.actualCost, summary.actualClicks)
  };
}

function metricPair(row: any) {
  if (reportMetric.value === "views")
    return [numberValue(row.forecastViews), numberValue(row.actualViews)];
  if (reportMetric.value === "clicks")
    return [numberValue(row.forecastClicks), numberValue(row.actualClicks)];
  if (reportMetric.value === "cpc") {
    return [
      cpcValue(row.forecastCost, row.forecastClicks),
      cpcValue(row.actualCost, row.actualClicks)
    ];
  }
  return [
    cpmValue(row.forecastCost, row.forecastViews),
    cpmValue(row.actualCost, row.actualViews)
  ];
}

function segmentName(row: any) {
  if (reportViewBy.value === "platform") return row.platform || "全部平台";
  if (reportViewBy.value === "creative") return row.creativeName || "全部创意";
  return row.audienceSegment || "全部受众";
}

function chartRows() {
  const grouped = new Map<string, any>();
  filteredSegments.value.forEach(row => {
    const name = segmentName(row);
    const current = grouped.get(name) || {
      name,
      forecastViews: 0,
      actualViews: 0,
      forecastClicks: 0,
      actualClicks: 0,
      forecastCost: 0,
      actualCost: 0
    };
    [
      "forecastViews",
      "actualViews",
      "forecastClicks",
      "actualClicks",
      "forecastCost",
      "actualCost"
    ].forEach(key => {
      current[key] += numberValue(row[key]);
    });
    grouped.set(name, current);
  });
  return Array.from(grouped.values()).slice(0, 28);
}

function renderReportChart() {
  if (
    activeSection.value !== "report" ||
    reportScope.value !== "campaign" ||
    !chartRef.value
  )
    return;
  reportChart ||= echarts.init(chartRef.value, undefined, { renderer: "svg" });
  const rows = chartRows();
  reportChart.setOption({
    color: ["#ffc9ad", "#a6dceb"],
    grid: { left: 56, right: 28, top: 30, bottom: 110 },
    tooltip: { trigger: "axis" },
    legend: { bottom: 0, data: ["预测", "实际"] },
    xAxis: {
      type: "category",
      data: rows.map(row => row.name),
      axisLabel: { rotate: 45, color: "#8a919c", interval: 0 }
    },
    yAxis: {
      type: "value",
      axisLabel: {
        color: "#8a919c",
        formatter: value =>
          reportMetric.value === "views" || reportMetric.value === "clicks"
            ? formatCount(value)
            : `$${value}`
      },
      splitLine: { lineStyle: { type: "dashed", color: "#e5e7eb" } }
    },
    series: [
      {
        name: "预测",
        type: "bar",
        data: rows.map(row => metricPair(row)[0]),
        barMaxWidth: 18
      },
      {
        name: "实际",
        type: "bar",
        data: rows.map(row => metricPair(row)[1]),
        barMaxWidth: 18
      }
    ]
  });
}

function platformTooltip(name: string) {
  const row = platformPerformance.value.find(item => item.platform === name);
  if (!row) return name;
  const exposureShare = ratioPercent(
    row.exposure,
    platformTotals.value.exposure
  );
  const engagementShare = ratioPercent(
    row.engagement,
    platformTotals.value.engagement
  );
  return [
    `<strong>${escapeHTML(row.platform)}</strong>`,
    `内容：${row.contentCount} 条`,
    `曝光 / 播放：${formatCount(row.exposure)}（${exposureShare}）`,
    `互动：${formatCount(row.engagement)}（${engagementShare}）`
  ].join("<br/>");
}

function emptyPlatformChartOption(message: string) {
  return {
    animation: false,
    tooltip: { show: false },
    series: [],
    graphic: [
      {
        type: "text",
        left: "center",
        top: "middle",
        style: {
          text: message,
          fill: "#9aa0aa",
          fontSize: 13,
          textAlign: "center"
        }
      }
    ]
  };
}

function renderPlatformDistributionChart() {
  if (!platformDistributionChartRef.value) return;
  if (
    !platformDistributionChart ||
    platformDistributionChart.getDom() !== platformDistributionChartRef.value
  ) {
    platformDistributionChart?.dispose();
    platformDistributionChart = echarts.init(
      platformDistributionChartRef.value,
      undefined,
      { renderer: "svg" }
    );
  }
  const rows = platformPerformance.value.filter(item => item.contentCount > 0);
  platformDistributionChart.clear();
  if (!rows.length) {
    platformDistributionChart.setOption(
      emptyPlatformChartOption("暂无可统计的发布内容")
    );
    return;
  }
  platformDistributionChart.setOption({
    color: rows.map(item => item.color),
    aria: { enabled: true, description: "各平台合作内容数量分布" },
    tooltip: {
      trigger: "item",
      confine: true,
      formatter: (params: any) => platformTooltip(String(params.name || ""))
    },
    graphic: [
      {
        type: "text",
        left: "center",
        top: "42%",
        style: {
          text: "内容总数",
          fill: "#7a808a",
          fontSize: 12,
          textAlign: "center"
        }
      },
      {
        type: "text",
        left: "center",
        top: "51%",
        style: {
          text: String(platformTotals.value.content),
          fill: "#20242a",
          fontSize: 24,
          fontWeight: 700,
          textAlign: "center"
        }
      }
    ],
    series: [
      {
        name: "平台内容分布",
        type: "pie",
        radius: ["56%", "78%"],
        center: ["50%", "50%"],
        minAngle: 4,
        avoidLabelOverlap: true,
        itemStyle: { borderColor: "#fff", borderWidth: 3 },
        label: { show: false },
        emphasis: {
          scale: true,
          scaleSize: 8,
          itemStyle: { shadowBlur: 14, shadowColor: "rgba(15, 23, 42, .18)" }
        },
        data: rows.map(item => ({
          name: item.platform,
          value: item.contentCount,
          itemStyle: { color: item.color }
        }))
      }
    ]
  });
}

function renderPlatformPerformanceChart() {
  if (!platformPerformanceChartRef.value) return;
  if (
    !platformPerformanceChart ||
    platformPerformanceChart.getDom() !== platformPerformanceChartRef.value
  ) {
    platformPerformanceChart?.dispose();
    platformPerformanceChart = echarts.init(
      platformPerformanceChartRef.value,
      undefined,
      { renderer: "svg" }
    );
  }
  const metric = platformMetric.value;
  const rows = platformPerformance.value.filter(item => item[metric] > 0);
  platformPerformanceChart.clear();
  if (!rows.length) {
    platformPerformanceChart.setOption(
      emptyPlatformChartOption(
        metric === "exposure" ? "暂无平台曝光数据" : "暂无平台互动数据"
      )
    );
    return;
  }
  const total =
    metric === "exposure"
      ? platformTotals.value.exposure
      : platformTotals.value.engagement;
  platformPerformanceChart.setOption({
    color: rows.map(item => item.color),
    aria: {
      enabled: true,
      description:
        metric === "exposure" ? "各平台曝光量占比" : "各平台互动量占比"
    },
    tooltip: {
      trigger: "item",
      confine: true,
      formatter: (params: any) => platformTooltip(String(params.name || ""))
    },
    graphic: [
      {
        type: "text",
        left: "center",
        top: "42%",
        style: {
          text: metric === "exposure" ? "总曝光 / 播放" : "总互动",
          fill: "#7a808a",
          fontSize: 12,
          textAlign: "center"
        }
      },
      {
        type: "text",
        left: "center",
        top: "51%",
        style: {
          text: formatCount(total),
          fill: "#20242a",
          fontSize: 24,
          fontWeight: 700,
          textAlign: "center"
        }
      }
    ],
    series: [
      {
        name: metric === "exposure" ? "平台曝光" : "平台互动",
        type: "pie",
        radius: ["56%", "78%"],
        center: ["50%", "50%"],
        minAngle: 4,
        itemStyle: { borderColor: "#fff", borderWidth: 3 },
        label: { show: false },
        emphasis: {
          scale: true,
          scaleSize: 8,
          itemStyle: { shadowBlur: 14, shadowColor: "rgba(15, 23, 42, .18)" }
        },
        data: rows.map(item => ({
          name: item.platform,
          value: item[metric],
          itemStyle: { color: item.color }
        }))
      }
    ]
  });
}

function renderOverviewCharts() {
  if (campaignTab.value !== "overview") return;
  renderPlatformDistributionChart();
  renderPlatformPerformanceChart();
}

function escapeHTML(value: unknown) {
  return String(value || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function openProjectDialog() {
  syncFormsFromProject();
  projectDialog.value = true;
}

async function submitProject() {
  if (!project.value) return;
  if (
    projectForm.cycleStartDate &&
    projectForm.cycleEndDate &&
    projectForm.cycleStartDate > projectForm.cycleEndDate
  ) {
    ElMessage.warning("项目周期的结束日期不能早于开始日期");
    return;
  }
  const { targetMarkets, ...formValues } = projectForm;
  submitting.value = true;
  try {
    const res = await updateProject({
      id: project.value.id,
      ...formValues,
      targetMarket: serializeProjectTargetMarkets(targetMarkets)
    });
    if (res.code === 0) {
      ElMessage.success("项目已更新");
      projectDialog.value = false;
      await loadDetail();
    }
  } finally {
    submitting.value = false;
  }
}

async function toggleProjectStatus() {
  if (!project.value) return;
  const action = isPaused.value ? "resume" : "pause";
  const res = await updateProjectStatus({ id: project.value.id, action });
  if (res.code === 0) {
    ElMessage.success(isPaused.value ? "项目已恢复" : "项目已暂停");
    await loadDetail();
  }
}

function openBudgetDialog() {
  budgetForm.budget = projectBudget.value;
  budgetDialog.value = true;
}

async function submitBudget() {
  if (!project.value) return;
  const res = await updateProjectBudget({
    id: project.value.id,
    budget: budgetForm.budget
  });
  if (res.code === 0) {
    ElMessage.success("预算已更新");
    budgetDialog.value = false;
    await loadDetail();
  }
}

async function submitRenew() {
  if (!project.value) return;
  const res = await renewProject({ id: project.value.id, ...renewForm });
  if (res.code === 0) {
    ElMessage.success("项目周期已更新");
    renewDialog.value = false;
    await loadDetail();
  }
}

function openInfluencerReport() {
  if (!focusedCooperation.value) return;
  influencerReportForm.reason = "内容质量或数据异常";
  influencerReportForm.detail = "";
  reportDialog.value = true;
}

async function submitInfluencerReport() {
  if (!focusedCooperation.value || !project.value) return;
  const res = await reportProjectInfluencer({
    projectId: project.value.id,
    cooperationId: focusedCooperation.value.id,
    resourceId: focusedCooperation.value.resourceId,
    ...influencerReportForm
  });
  if (res.code === 0) {
    ElMessage.success("已提交达人异常反馈");
    reportDialog.value = false;
  }
}

async function handleExportProjectData() {
  if (!project.value) return;
  try {
    const blob = await downloadProjectData({
      projectId: project.value.id,
      scope: "project"
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    const safeName = String(project.value.name || `project-${project.value.id}`)
      .replace(/[\\/:*?"<>|]/g, "_")
      .trim();
    link.download = `${safeName || `project-${project.value.id}`}_标准项目数据.xlsx`;
    link.click();
    URL.revokeObjectURL(url);
    ElMessage.success("项目标准数据已导出");
  } catch {
    ElMessage.error("项目数据导出失败，请稍后重试");
  }
}

function showBillingHistory() {
  const lines = billingEvents.value
    .slice(0, 10)
    .map(
      item =>
        `${item.occurredAt || "-"}  ${item.description || item.eventType}: ${moneyText(item.amount, item.currency)}`
    )
    .join("<br/>");
  ElMessageBox.alert(lines || "暂无账单流水", "Billing history", {
    dangerouslyUseHTMLString: true,
    confirmButtonText: "关闭"
  });
}

watch(
  [activeSection, reportScope, reportMetric, reportViewBy, filteredSegments],
  () => nextTick(renderReportChart)
);

watch(
  [campaignTab, platformMetric, platformPerformance],
  () => nextTick(renderOverviewCharts),
  { deep: true }
);

watch(selectedContentPost, post => {
  if (post) void ensureWebsiteScreenshot(post);
});

onMounted(async () => {
  await loadProjects();
  await loadDetail();
  window.addEventListener("resize", renderReportChart);
  window.addEventListener("resize", renderOverviewCharts);
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", renderReportChart);
  window.removeEventListener("resize", renderOverviewCharts);
  contentTitleRefreshTimers.forEach(timer => window.clearTimeout(timer));
  contentTitleRefreshTimers.clear();
  reportChart?.dispose();
  platformDistributionChart?.dispose();
  platformPerformanceChart?.dispose();
});
</script>

<template>
  <div v-loading="loading" class="campaign-workspace">
    <header class="campaign-header">
      <div class="campaign-heading">
        <el-button circle class="back-button" @click="goBack">
          <IconifyIconOnline icon="ri:arrow-left-line" />
        </el-button>
        <div class="campaign-mark">
          <IconifyIconOnline icon="ri:folder-user-line" />
        </div>
        <div class="campaign-name">
          <h1>{{ project?.name || "项目" }}</h1>
          <div class="campaign-meta">
            <el-tag size="small" effect="plain">{{
              project?.campaignType || "合作项目"
            }}</el-tag>
            <span
              ><IconifyIconOnline icon="ri:checkbox-circle-fill" />
              数据已同步</span
            >
          </div>
        </div>
      </div>

      <div class="campaign-actions">
        <el-select
          :model-value="selectedProjectId"
          class="project-switcher"
          size="default"
          @change="handleProjectChange"
        >
          <el-option
            v-for="item in projects"
            :key="item.id"
            :label="item.name"
            :value="item.id"
          />
        </el-select>
        <span class="cycle-label"
          >{{ fieldLabel("创建于") }} {{ createdDateLabel }}</span
        >
        <el-button circle text aria-label="编辑项目" @click="openProjectDialog">
          <IconifyIconOnline icon="ri:edit-line" />
        </el-button>
        <el-dropdown>
          <el-button circle text aria-label="更多项目操作"
            ><IconifyIconOnline icon="ri:more-2-fill"
          /></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="toggleProjectStatus">
                {{ isPaused ? "恢复项目" : "暂停项目" }}
              </el-dropdown-item>
              <el-dropdown-item @click="handleExportProjectData"
                >导出项目数据</el-dropdown-item
              >
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <nav class="campaign-tabs" aria-label="项目详情导航">
      <button
        type="button"
        :class="{ active: campaignTab === 'overview' }"
        @click="selectCampaignTab('overview')"
      >
        概览
      </button>
      <button
        type="button"
        :class="{ active: campaignTab === 'creators' }"
        @click="selectCampaignTab('creators')"
      >
        达人 ({{ projectCreators.length }})
      </button>
      <button
        type="button"
        :class="{ active: campaignTab === 'content' }"
        @click="selectCampaignTab('content')"
      >
        内容 ({{ projectContentPosts.length }})
      </button>
    </nav>

    <main class="campaign-main">
      <template v-if="campaignTab === 'content' && contentDetailView">
        <section class="content-detail-page">
          <div class="content-detail-toolbar">
            <button
              type="button"
              class="content-detail-back"
              @click="closeContentDetail"
            >
              <IconifyIconOnline icon="ri:arrow-left-line" />
              返回合作内容
            </button>
          </div>

          <div class="content-detail-hero">
            <button
              type="button"
              class="content-detail-cover"
              :aria-label="contentOpenLabel(contentDetailView)"
              @click="openPost(contentDetailView)"
            >
              <img
                v-if="contentDetailView.coverUrl"
                :src="contentDetailView.coverUrl"
                :alt="contentDetailView.title || contentDetailView.resourceName"
                @error="useRemotePostCover($event, contentDetailView)"
              />
              <span
                v-else
                class="content-detail-cover-empty"
                :class="{
                  'website-preview-empty': isWebsiteContent(contentDetailView)
                }"
              >
                <span
                  v-if="isWebsiteContent(contentDetailView)"
                  class="website-preview-window"
                >
                  <i /><i /><i />
                  <strong>{{ contentDetailView.postUrl || "Website" }}</strong>
                </span>
                <PlatformIconBadge
                  v-else
                  :platform="contentDetailView.platform"
                />
                <strong>{{
                  isWebsiteContent(contentDetailView)
                    ? websiteScreenshotLoading
                      ? "正在生成网页截图…"
                      : "该网页暂未生成截图"
                    : "该内容暂未同步封面"
                }}</strong>
                <small
                  >{{ fieldLabel("点击")
                  }}{{ contentOpenLabel(contentDetailView) }}</small
                >
              </span>
              <span
                class="content-detail-play"
                :class="{
                  'website-open-icon': isWebsiteContent(contentDetailView)
                }"
              >
                <IconifyIconOnline
                  :icon="
                    isWebsiteContent(contentDetailView)
                      ? 'ri:external-link-line'
                      : 'ri:play-fill'
                  "
                />
              </span>
              <span
                v-if="isViralContent(contentDetailView)"
                class="viral-badge detail-viral-badge"
                >爆 🔥</span
              >
            </button>

            <div class="content-detail-summary">
              <div class="content-detail-title-row">
                <div>
                  <span class="content-detail-platform">
                    <PlatformIconBadge :platform="contentDetailView.platform" />
                    {{ contentDetailView.platform || "合作内容" }}
                  </span>
                  <CooperationTypeTags
                    v-if="contentCooperationType(contentDetailView)"
                    class="content-detail-cooperation-types"
                    :value="contentCooperationType(contentDetailView)"
                  />
                  <el-tag
                    v-if="contentTypeTag(contentDetailView)"
                    class="content-type-tag content-detail-type-tag"
                    type="primary"
                    effect="plain"
                    size="small"
                    :title="fieldLabel('内容类型')"
                  >
                    {{ contentTypeTag(contentDetailView) }}
                  </el-tag>
                </div>
                <el-button type="primary" @click="openPost(contentDetailView)">
                  {{ contentOpenLabel(contentDetailView) }}
                  <IconifyIconOnline icon="ri:external-link-line" />
                </el-button>
              </div>
              <div class="content-detail-author">
                <el-avatar
                  :src="contentAvatar(contentDetailView)"
                  :size="48"
                  @error="useRemoteContentAvatar(contentDetailView)"
                  >{{
                    String(contentDetailView.resourceName || "R").slice(0, 1)
                  }}</el-avatar
                >
                <div>
                  <strong>{{
                    contentDetailView.resourceName || "未知合作方"
                  }}</strong>
                  <span>
                    {{ formatCount(contentFollowers(contentDetailView)) }}
                    {{ contentAudienceLabel(contentDetailView) }} ·
                    {{ dateText(contentDetailView.publishedAt) }} 发布
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="content-detail-metrics">
            <article>
              <span>{{ contentExposureLabel(contentDetailView) }}</span>
              <strong>{{
                formatCount(postExposure(contentDetailView))
              }}</strong>
              <small>{{ contentExposureHint(contentDetailView) }}</small>
            </article>
            <article>
              <span>{{ fieldLabel("互动量") }}</span>
              <strong>{{
                formatCount(postEngagement(contentDetailView))
              }}</strong>
              <small>{{ fieldLabel("点赞、评论、分享与收藏") }}</small>
            </article>
          </div>

          <div class="content-detail-link-row">
            <span>{{ fieldLabel("内容链接") }}</span>
            <el-link
              :href="contentDetailView.postUrl"
              target="_blank"
              type="primary"
            >
              {{ contentDetailView.postUrl || "暂无链接" }}
            </el-link>
          </div>
        </section>
      </template>

      <template v-else-if="campaignTab === 'overview'">
        <section class="overview-section">
          <div class="overview-section-heading">
            <div>
              <h2>{{ fieldLabel("内容概览") }}</h2>
              <p>
                {{ fieldLabel("合作方按名称去重，内容按发布链接去重统计。") }}
              </p>
            </div>
            <span
              >{{ fieldLabel("今日新增") }} {{ campaignOverview.today }}
              {{ fieldLabel("条") }}</span
            >
          </div>
          <div class="overview-metric-grid content-overview-grid">
            <article class="overview-metric-card primary-metric-card">
              <span>{{ fieldLabel("内容总数") }}</span>
              <strong>{{ campaignOverview.posts }}</strong>
              <small>{{ fieldLabel("当前项目已识别的全部合作内容") }}</small>
            </article>
            <article class="overview-metric-card">
              <span>{{ fieldLabel("合作达人 / 媒体总数") }}</span>
              <strong>{{ campaignOverview.collaborators }}</strong>
              <small>{{ fieldLabel("同名合作方仅计算一次") }}</small>
            </article>
          </div>
        </section>

        <section class="overview-section">
          <div class="overview-section-heading">
            <div>
              <h2>{{ fieldLabel("曝光与互动") }}</h2>
              <p>
                {{
                  fieldLabel(
                    "互动量包含点赞、评论、分享与收藏，以平台同步结果为准。"
                  )
                }}
              </p>
            </div>
          </div>
          <div class="overview-metric-grid performance-overview-grid">
            <article class="overview-metric-card primary-metric-card">
              <span>{{ fieldLabel("总曝光 / 播放量") }}</span>
              <strong>{{ formatCount(campaignOverview.views) }}</strong>
              <small>{{ fieldLabel("所有合作内容累计数据") }}</small>
            </article>
            <article class="overview-metric-card">
              <span>{{ fieldLabel("总互动量") }}</span>
              <strong>{{ formatCount(campaignOverview.engagements) }}</strong>
              <small>{{ fieldLabel("点赞 + 评论 + 分享 + 收藏") }}</small>
            </article>
            <article class="overview-metric-card">
              <span>{{ fieldLabel("平均互动率") }}</span>
              <strong>{{ campaignOverview.engagementRate }}</strong>
              <small>{{ fieldLabel("总互动量 / 总曝光量") }}</small>
            </article>
            <article class="overview-metric-card">
              <span>{{ fieldLabel("付费 KOL 成本") }}</span>
              <strong>{{ moneyText(campaignOverview.paidCost) }}</strong>
              <small>{{
                fieldLabel("仅统计有实际合作费用的非媒体资源")
              }}</small>
            </article>
            <article class="overview-metric-card">
              <span>CPM</span>
              <strong>{{ moneyText(campaignOverview.cpm) }}</strong>
              <small>{{ fieldLabel("付费成本 / 曝光 × 1000") }}</small>
            </article>
            <article class="overview-metric-card">
              <span>CPE</span>
              <strong>{{ moneyText(campaignOverview.cpe) }}</strong>
              <small>{{ fieldLabel("付费成本 / 互动量") }}</small>
            </article>
          </div>
        </section>

        <section class="overview-section platform-overview-section">
          <div class="overview-section-heading">
            <div>
              <h2>{{ fieldLabel("平台表现") }}</h2>
              <p>
                {{
                  fieldLabel("悬停图表可查看对应平台的内容、曝光、互动及占比。")
                }}
              </p>
            </div>
            <span>{{ platformPerformance.length }} 个平台</span>
          </div>
          <div class="platform-chart-grid">
            <article class="platform-chart-card">
              <header>
                <div>
                  <h3>{{ fieldLabel("平台内容分布") }}</h3>
                  <p>{{ fieldLabel("各平台发布内容数量及占比") }}</p>
                </div>
              </header>
              <div class="platform-chart-body">
                <div
                  ref="platformDistributionChartRef"
                  class="platform-pie-chart"
                  role="img"
                  aria-label="各平台合作内容数量分布饼图"
                />
                <div class="platform-legend" aria-label="平台内容图例">
                  <div
                    v-for="item in platformPerformance"
                    :key="`content-${item.platform}`"
                    class="platform-legend-row"
                  >
                    <span
                      class="legend-color"
                      :style="{ backgroundColor: item.color }"
                    />
                    <PlatformIconBadge :platform="item.platform" />
                    <strong>{{ item.platform }}</strong>
                    <span>{{ item.contentCount }} 条</span>
                    <em>{{
                      ratioPercent(item.contentCount, platformTotals.content)
                    }}</em>
                  </div>
                  <el-empty
                    v-if="!platformPerformance.length"
                    :description="fieldLabel('暂无平台内容')"
                    :image-size="52"
                  />
                </div>
              </div>
            </article>

            <article class="platform-chart-card">
              <header>
                <div>
                  <h3>{{ fieldLabel("各平台效果占比") }}</h3>
                  <p>{{ fieldLabel("切换查看曝光量或互动量构成") }}</p>
                </div>
                <div class="platform-metric-switch" aria-label="平台效果指标">
                  <button
                    type="button"
                    :class="{ active: platformMetric === 'exposure' }"
                    @click="platformMetric = 'exposure'"
                  >
                    曝光量
                  </button>
                  <button
                    type="button"
                    :class="{ active: platformMetric === 'engagement' }"
                    @click="platformMetric = 'engagement'"
                  >
                    互动量
                  </button>
                </div>
              </header>
              <div class="platform-chart-body">
                <div
                  ref="platformPerformanceChartRef"
                  class="platform-pie-chart"
                  role="img"
                  :aria-label="
                    platformMetric === 'exposure'
                      ? '各平台曝光量占比饼图'
                      : '各平台互动量占比饼图'
                  "
                />
                <div class="platform-legend" aria-label="平台效果图例">
                  <div
                    v-for="item in platformPerformance"
                    :key="`performance-${item.platform}`"
                    class="platform-legend-row performance-legend-row"
                  >
                    <span
                      class="legend-color"
                      :style="{ backgroundColor: item.color }"
                    />
                    <PlatformIconBadge :platform="item.platform" />
                    <strong>{{ item.platform }}</strong>
                    <span>{{
                      formatCount(
                        platformMetric === "exposure"
                          ? item.exposure
                          : item.engagement
                      )
                    }}</span>
                    <em>{{
                      ratioPercent(
                        platformMetric === "exposure"
                          ? item.exposure
                          : item.engagement,
                        platformMetric === "exposure"
                          ? platformTotals.exposure
                          : platformTotals.engagement
                      )
                    }}</em>
                  </div>
                  <el-empty
                    v-if="!platformPerformance.length"
                    :description="fieldLabel('暂无平台效果数据')"
                    :image-size="52"
                  />
                </div>
              </div>
            </article>
          </div>
        </section>
      </template>

      <section v-else-if="campaignTab === 'creators'" class="creator-page">
        <div class="toolbar creator-toolbar">
          <el-input
            v-model="creatorSearch"
            clearable
            :placeholder="fieldLabel('搜索达人或媒体名称')"
            class="search-field"
          >
            <template #prefix
              ><IconifyIconOnline icon="ri:search-line"
            /></template>
          </el-input>
          <el-select v-model="creatorCategory" class="creator-filter">
            <el-option :label="fieldLabel('全部领域')" value="all" />
            <el-option
              v-for="category in creatorCategoryOptions"
              :key="category"
              :label="category"
              :value="category"
            />
          </el-select>
          <el-select v-model="creatorPlatform" class="creator-filter">
            <el-option :label="fieldLabel('全部平台')" value="all" />
            <el-option
              v-for="platform in creatorPlatformOptions"
              :key="platform"
              :label="platform"
              :value="platform"
            />
          </el-select>
          <el-select v-model="creatorTier" class="creator-filter">
            <el-option :label="fieldLabel('全部层级')" value="all" />
            <el-option
              v-for="tier in creatorTierOptions"
              :key="tier"
              :label="tier"
              :value="tier"
            />
          </el-select>
          <div class="toolbar-actions">
            <el-button type="primary" @click="openCreateProjectResource">
              <IconifyIconOnline icon="ri:add-line" />
              添加达人 / 媒体
            </el-button>
          </div>
        </div>

        <div class="creator-part-heading">
          <div>
            <h2>{{ fieldLabel("达人") }}</h2>
            <p>
              {{
                fieldLabel(
                  "粉丝量采用达人账号数据，曝光与互动仅统计当前项目合作内容。"
                )
              }}
            </p>
          </div>
          <span>{{ influencerRows.length }} 位达人</span>
        </div>
        <el-table :data="influencerRows" class="creator-table">
          <el-table-column :label="fieldLabel('达人')" min-width="250" sortable>
            <template #default="{ row }">
              <button
                type="button"
                class="creator-cell creator-profile-link"
                @click="openResourceProfile(row)"
              >
                <el-avatar :src="row.resourceAvatarUrl" :size="34">{{
                  String(row.resourceName || "R").slice(0, 1)
                }}</el-avatar>
                <div>
                  <span class="creator-name-line">
                    <strong>{{ row.resourceName || "未命名达人" }}</strong>
                    <span class="creator-platform-icons">
                      <PlatformIconBadge
                        v-for="platform in creatorPlatforms(row)"
                        :key="platform"
                        :platform="platform"
                      />
                    </span>
                  </span>
                  <span>{{ fieldLabel("查看资源档案") }}</span>
                </div>
              </button>
            </template>
          </el-table-column>
          <el-table-column
            prop="category"
            :label="fieldLabel('领域')"
            min-width="120"
          />
          <el-table-column
            :label="fieldLabel('内容数量')"
            width="110"
            align="right"
            sortable
            :sort-method="sortByContentCount"
          >
            <template #default="{ row }">
              {{ projectContentCount(row) }}
            </template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('粉丝量')"
            width="125"
            align="right"
            sortable
            :sort-method="sortByAudience"
          >
            <template #default="{ row }">
              <span class="umv-value">
                {{ formatCount(projectAudience(row)) }}
                <small v-if="row.umvMonth">{{ row.umvMonth }}</small>
              </span>
            </template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('曝光量')"
            width="125"
            align="right"
            sortable
            :sort-method="sortByExposure"
          >
            <template #default="{ row }">{{
              formatCount(projectExposure(row))
            }}</template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('互动量')"
            width="125"
            align="right"
            sortable
            :sort-method="sortByEngagement"
          >
            <template #default="{ row }">{{
              formatCount(projectEngagement(row))
            }}</template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('CPM')"
            width="120"
            align="right"
            sortable
            :sort-method="sortByCPM"
          >
            <template #default="{ row }">
              <span
                :title="
                  fieldLabel('项目内该达人全部平台总成本 / 总播放量 × 1000')
                "
              >
                {{ moneyText(projectCPM(row)) }}
              </span>
            </template>
          </el-table-column>
          <el-table-column
            prop="collaboratorTier"
            :label="fieldLabel('层级')"
            width="100"
          >
            <template #default="{ row }">
              <el-tag effect="plain">{{
                row.collaboratorTier || "待同步"
              }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('最新合作内容')"
            width="140"
            align="center"
          >
            <template #default="{ row }">
              <button
                v-if="latestProjectPost(row)"
                type="button"
                class="latest-content-thumb"
                :aria-label="`打开 ${row.resourceName} 的最新合作内容`"
                @click="openPost(latestProjectPost(row))"
              >
                <img
                  v-if="latestProjectPost(row)?.coverUrl"
                  :src="latestProjectPost(row).coverUrl"
                  :alt="latestProjectPost(row).title || row.resourceName"
                  @error="useRemotePostCover($event, latestProjectPost(row))"
                />
                <span v-else>
                  <PlatformIconBadge
                    :platform="latestProjectPost(row).platform"
                  />
                  <small>{{ fieldLabel("查看内容") }}</small>
                </span>
              </button>
              <span v-else class="latest-content-empty">{{
                fieldLabel("暂无内容")
              }}</span>
            </template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('操作')"
            width="120"
            fixed="right"
          >
            <template #default="{ row }">
              <el-button
                link
                type="primary"
                @click.stop="openEditProjectResource(row)"
                >编辑</el-button
              >
              <el-button
                link
                type="danger"
                @click.stop="removeProjectResource(row)"
                >删除</el-button
              >
            </template>
          </el-table-column>
        </el-table>

        <div class="creator-part-heading media-part-heading">
          <div>
            <h2>{{ fieldLabel("媒体") }}</h2>
            <p>
              Website 媒体量级采用 Traffic.cv
              月访问量；其他媒体采用月独立访客（UMV）。
            </p>
          </div>
          <span>{{ mediaRows.length }} 家媒体</span>
        </div>
        <el-table :data="mediaRows" class="creator-table media-table">
          <el-table-column :label="fieldLabel('媒体')" min-width="250" sortable>
            <template #default="{ row }">
              <button
                type="button"
                class="creator-cell creator-profile-link"
                @click="openResourceProfile(row)"
              >
                <el-avatar :src="row.resourceAvatarUrl" :size="34">{{
                  String(row.resourceName || "M").slice(0, 1)
                }}</el-avatar>
                <div>
                  <span class="creator-name-line">
                    <strong>{{ row.resourceName || "未命名媒体" }}</strong>
                    <span class="creator-platform-icons">
                      <PlatformIconBadge
                        v-for="platform in creatorPlatforms(row)"
                        :key="platform"
                        :platform="platform"
                      />
                    </span>
                  </span>
                  <span>{{ fieldLabel("查看资源档案") }}</span>
                </div>
              </button>
            </template>
          </el-table-column>
          <el-table-column
            prop="category"
            :label="fieldLabel('领域')"
            min-width="120"
          />
          <el-table-column
            :label="fieldLabel('内容数量')"
            width="110"
            align="right"
            sortable
            :sort-method="sortByContentCount"
          >
            <template #default="{ row }">{{
              projectContentCount(row)
            }}</template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('月独立访客（UMV）')"
            width="170"
            align="right"
            sortable
            :sort-method="sortByAudience"
          >
            <template #default="{ row }">{{
              formatCount(projectAudience(row))
            }}</template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('播放量')"
            width="125"
            align="right"
            sortable
            :sort-method="sortByExposure"
          >
            <template #default="{ row }">{{
              formatCount(projectExposure(row))
            }}</template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('互动量')"
            width="125"
            align="right"
            sortable
            :sort-method="sortByEngagement"
          >
            <template #default="{ row }">{{
              formatCount(projectEngagement(row))
            }}</template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('CPM')"
            width="120"
            align="right"
            sortable
            :sort-method="sortByCPM"
          >
            <template #default="{ row }">
              <span
                :title="
                  fieldLabel('项目内该媒体全部平台总成本 / 总播放量 × 1000')
                "
              >
                {{ moneyText(projectCPM(row)) }}
              </span>
            </template>
          </el-table-column>
          <el-table-column
            prop="collaboratorTier"
            :label="fieldLabel('层级')"
            width="100"
          >
            <template #default="{ row }"
              ><el-tag effect="plain">{{
                row.collaboratorTier || "待同步"
              }}</el-tag></template
            >
          </el-table-column>
          <el-table-column
            :label="fieldLabel('最新合作内容')"
            width="140"
            align="center"
          >
            <template #default="{ row }">
              <button
                v-if="latestProjectPost(row)"
                type="button"
                class="latest-content-thumb"
                :aria-label="`打开 ${row.resourceName} 的最新合作内容`"
                @click="openPost(latestProjectPost(row))"
              >
                <img
                  v-if="latestProjectPost(row)?.coverUrl"
                  :src="latestProjectPost(row).coverUrl"
                  :alt="latestProjectPost(row).title || row.resourceName"
                  @error="useRemotePostCover($event, latestProjectPost(row))"
                />
                <span v-else
                  ><PlatformIconBadge
                    :platform="latestProjectPost(row).platform"
                  /><small>查看内容</small></span
                >
              </button>
              <span v-else class="latest-content-empty">{{
                fieldLabel("暂无内容")
              }}</span>
            </template>
          </el-table-column>
          <el-table-column
            :label="fieldLabel('操作')"
            width="120"
            fixed="right"
          >
            <template #default="{ row }">
              <el-button
                link
                type="primary"
                @click.stop="openEditProjectResource(row)"
                >编辑</el-button
              >
              <el-button
                link
                type="danger"
                @click.stop="removeProjectResource(row)"
                >删除</el-button
              >
            </template>
          </el-table-column>
        </el-table>
      </section>

      <section v-else class="content-page">
        <div class="toolbar content-toolbar">
          <el-input
            v-model="contentSearch"
            clearable
            :placeholder="fieldLabel('搜索达人或内容关键词')"
            class="search-field"
          >
            <template #prefix
              ><IconifyIconOnline icon="ri:search-line"
            /></template>
          </el-input>
          <el-select v-model="contentPlatform" class="platform-filter">
            <el-option :label="fieldLabel('全部平台')" value="all" />
            <el-option
              v-for="platform in contentPlatforms"
              :key="platform"
              :label="platform"
              :value="platform"
            />
          </el-select>
          <el-select v-model="contentSort" class="content-sort-filter">
            <el-option :label="fieldLabel('最新发布')" value="latest" />
            <el-option :label="fieldLabel('播放量从高到低')" value="views" />
            <el-option
              :label="fieldLabel('互动量从高到低')"
              value="engagement"
            />
          </el-select>
          <span class="content-count"
            >共 {{ filteredContentPosts.length }} 条内容</span
          >
        </div>
        <el-empty
          v-if="!filteredContentPosts.length"
          :description="fieldLabel('暂未同步到可展示的内容')"
        />
        <div v-else class="content-grid">
          <article
            v-for="post in filteredContentPosts"
            :key="post.id"
            class="content-card"
            @click="openContentDetail(post)"
          >
            <div class="content-author">
              <el-avatar
                :src="contentAvatar(post)"
                :size="34"
                @error="useRemoteContentAvatar(post)"
                >{{ String(post.resourceName || "R").slice(0, 1) }}</el-avatar
              >
              <div class="content-author-copy">
                <strong>{{ post.resourceName || "未知达人 / 媒体" }}</strong>
                <span>
                  {{ formatCount(contentFollowers(post)) }}
                  {{ contentAudienceLabel(post) }}
                </span>
              </div>
              <div class="content-card-actions" @click.stop>
                <PlatformIconBadge
                  class="content-platform-badge"
                  :platform="post.platform"
                />
                <el-dropdown
                  trigger="click"
                  @command="command => handleContentCardCommand(command, post)"
                >
                  <el-button
                    circle
                    text
                    size="small"
                    aria-label="内容操作"
                    :loading="syncingContentIds[contentOperationKey(post)]"
                    @click.stop
                  >
                    <IconifyIconOnline icon="ri:menu-line" />
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="edit">
                        <IconifyIconOnline icon="ri:edit-line" />
                        编辑
                      </el-dropdown-item>
                      <el-dropdown-item command="sync">
                        <IconifyIconOnline icon="ri:refresh-line" />
                        同步
                      </el-dropdown-item>
                      <el-dropdown-item command="delete" divided>
                        <IconifyIconOnline icon="ri:delete-bin-line" />
                        删除
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </div>
            <div class="content-cover-wrap">
              <img
                v-if="post.coverUrl"
                :src="post.coverUrl"
                :alt="post.title || post.resourceName"
                class="content-cover"
                @error="useRemotePostCover($event, post)"
              />
              <div v-else class="content-cover empty-cover">
                <PlatformIconBadge :platform="post.platform" />
              </div>
              <span v-if="isViralContent(post)" class="viral-badge"
                >{{ fieldLabel("爆") }} 🔥</span
              >
            </div>
            <div class="content-info">
              <CooperationTypeTags
                v-if="contentCooperationType(post)"
                class="content-card-cooperation-types"
                :value="contentCooperationType(post)"
              />
              <el-tag
                v-if="contentTypeTag(post)"
                class="content-type-tag content-card-type-tag"
                type="primary"
                effect="plain"
                size="small"
                :title="fieldLabel('内容类型')"
              >
                {{ contentTypeTag(post) }}
              </el-tag>
              <div class="content-card-metrics">
                <span>
                  <small>{{ contentExposureLabel(post) }}</small>
                  <strong>{{ formatCount(postExposure(post)) }}</strong>
                </span>
                <span>
                  <small>{{ fieldLabel("互动量") }}</small>
                  <strong>{{ formatCount(postEngagement(post)) }}</strong>
                </span>
              </div>
            </div>
          </article>
        </div>
      </section>
    </main>
  </div>

  <el-dialog
    v-model="contentEditing"
    :title="fieldLabel('编辑内容')"
    width="520px"
    destroy-on-close
    @closed="resetContentEdit"
  >
    <el-form
      :model="contentEditForm"
      label-position="top"
      @submit.prevent="saveContentEdit"
    >
      <el-form-item :label="fieldLabel('所属平台')" required>
        <el-select
          v-model="contentEditForm.platform"
          filterable
          class="w-full!"
        >
          <el-option
            v-for="platform in editableContentPlatformOptions"
            :key="platform"
            :label="platform"
            :value="platform"
          >
            <div class="content-platform-option">
              <PlatformIconBadge :platform="platform" />
              <span>{{ platform }}</span>
            </div>
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item :label="fieldLabel('内容链接')" required>
        <el-input
          v-model="contentEditForm.postUrl"
          clearable
          placeholder="https://..."
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="cancelContentEdit">{{ fieldLabel("取消") }}</el-button>
      <el-button
        type="primary"
        :loading="contentSaving"
        @click="saveContentEdit"
      >
        保存修改
      </el-button>
    </template>
  </el-dialog>

  <el-dialog
    v-model="creatorDialog"
    :title="
      creatorDialogMode === 'create' ? '添加达人 / 媒体' : '编辑达人 / 媒体'
    "
    width="620px"
  >
    <el-form :model="creatorForm" label-position="top">
      <template v-if="creatorDialogMode === 'create'">
        <el-form-item :label="fieldLabel('从全球资源库选择')" required>
          <el-select
            v-model="creatorForm.resourceId"
            filterable
            :filter-method="filterCreatorOptions"
            :loading="creatorOptionsLoading"
            :placeholder="fieldLabel('搜索达人、媒体或平台')"
            class="w-full!"
            @change="handleCreatorOptionChange"
          >
            <el-option
              v-for="item in visibleCreatorOptions"
              :key="item.resourceId"
              :label="creatorOptionLabel(item)"
              :value="item.resourceId"
            />
            <el-option
              v-if="creatorLibraryKeyword.trim()"
              :value="onlineSearchOptionValue"
              :label="`未找到目标？全网搜索“${creatorLibraryKeyword.trim()}”`"
              class="online-search-option"
            >
              <div class="online-search-option-content">
                <IconifyIconOnline icon="ri:global-line" />
                <span>{{ fieldLabel("未找到目标？全网搜索") }}</span>
                <small>{{ creatorLibraryKeyword.trim() }}</small>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <section v-if="onlineSearchExpanded" class="online-search-panel">
          <header>
            <div>
              <strong>{{ fieldLabel("从指定平台查询") }}</strong>
              <span>{{ fieldLabel("查询成功后会同步进入全球资源库") }}</span>
            </div>
            <IconifyIconOnline icon="ri:global-line" />
          </header>
          <div class="online-search-grid">
            <el-form-item :label="fieldLabel('平台')" required>
              <el-select v-model="onlineSearchForm.platform" class="w-full!">
                <el-option label="Instagram" value="Instagram" />
                <el-option label="TikTok" value="TikTok" />
                <el-option label="YouTube" value="YouTube" />
                <el-option label="X" value="X" />
                <el-option
                  :label="fieldLabel('Facebook（TikHub 暂未开放接口）')"
                  value="Facebook"
                  disabled
                />
                <el-option label="LinkedIn" value="LinkedIn" />
                <el-option label="Reddit" value="Reddit" />
                <el-option
                  v-if="onlineSearchForm.resourceType === '媒体'"
                  label="Website"
                  value="Website"
                />
              </el-select>
            </el-form-item>
            <el-form-item :label="fieldLabel('类型')">
              <el-select
                v-model="onlineSearchForm.resourceType"
                class="w-full!"
              >
                <el-option :label="fieldLabel('达人（KOL）')" value="KOL" />
                <el-option :label="fieldLabel('媒体')" value="媒体" />
                <el-option :label="fieldLabel('艺术家')" value="艺术家" />
              </el-select>
            </el-form-item>
          </div>
          <el-form-item
            :label="fieldLabel('主页链接 / @handle / 平台账号')"
            required
          >
            <el-input
              v-model="onlineSearchForm.query"
              clearable
              :placeholder="fieldLabel('例如 @username 或完整主页链接')"
              @keyup.enter="runOnlineCreatorSearch"
            >
              <template #append>
                <el-button
                  :loading="onlineSearchLoading"
                  @click="runOnlineCreatorSearch"
                  >查询</el-button
                >
              </template>
            </el-input>
          </el-form-item>
          <div v-if="onlineSearchResult" class="online-search-result">
            <el-avatar :src="onlineSearchResult.resourceAvatarUrl" :size="42">
              {{ String(onlineSearchResult.resourceName || "R").slice(0, 1) }}
            </el-avatar>
            <div>
              <strong>{{ onlineSearchResult.resourceName }}</strong>
              <span>
                {{ onlineSearchResult.platform }}
                {{
                  onlineSearchResult.platformHandle
                    ? `@${onlineSearchResult.platformHandle}`
                    : ""
                }}
                ·
                {{
                  onlineSearchResult.resourceType === "媒体"
                    ? `${formatCount(onlineSearchResult.audienceSize)} UMV`
                    : `${formatCount(onlineSearchResult.followers)} 粉丝`
                }}
              </span>
            </div>
            <el-tag type="success" effect="light">{{
              fieldLabel("已选中")
            }}</el-tag>
          </div>
        </section>
        <el-alert
          v-if="!creatorOptionsLoading && creatorOptions.length === 0"
          :title="
            fieldLabel(
              '全球资源库暂无可添加账号，可在上方输入账号后选择“全网搜索”。'
            )
          "
          type="info"
          :closable="false"
          show-icon
        />
      </template>
      <template v-else>
        <el-alert
          :title="fieldLabel('修改会同步更新全球资源库中的该达人/媒体资料。')"
          type="info"
          :closable="false"
          show-icon
          class="creator-edit-alert"
        />
        <div class="creator-form-grid">
          <el-form-item :label="fieldLabel('名称')" required>
            <el-input v-model="creatorForm.resourceName" />
          </el-form-item>
          <el-form-item :label="fieldLabel('类型')" required>
            <el-select v-model="creatorForm.resourceType" class="w-full!">
              <el-option
                v-for="item in ['KOL', '媒体', '艺术家']"
                :key="item"
                :label="item"
                :value="item"
              />
            </el-select>
          </el-form-item>
          <el-form-item :label="fieldLabel('领域')">
            <el-input v-model="creatorForm.category" />
          </el-form-item>
          <el-form-item :label="fieldLabel('市场')">
            <el-input v-model="creatorForm.market" />
          </el-form-item>
          <el-form-item :label="fieldLabel('平台')">
            <el-input v-model="creatorForm.platform" />
          </el-form-item>
          <el-form-item
            :label="
              creatorForm.resourceType === '媒体'
                ? normalizePlatformName(creatorForm.platform) === 'Website'
                  ? '月访问量（Monthly Visits）'
                  : '月独立访客（UMV）'
                : '本平台粉丝数'
            "
          >
            <el-input-number
              v-if="creatorForm.resourceType === '媒体'"
              v-model="creatorForm.audienceSize"
              :min="0"
              class="w-full!"
            />
            <el-input-number
              v-else
              v-model="creatorForm.followers"
              :min="0"
              class="w-full!"
            />
          </el-form-item>
          <el-form-item
            :label="fieldLabel('主页链接')"
            class="creator-form-grid__wide"
          >
            <el-input v-model="creatorForm.platformUrl" />
          </el-form-item>
          <el-form-item
            :label="fieldLabel('联系方式')"
            class="creator-form-grid__wide"
          >
            <el-input v-model="creatorForm.primaryContact" />
          </el-form-item>
          <el-form-item
            :label="fieldLabel('层级（系统自动）')"
            class="creator-form-grid__wide"
          >
            <el-input
              :model-value="creatorForm.collaboratorTier || '保存后自动计算'"
              disabled
            />
          </el-form-item>
        </div>
      </template>
    </el-form>
    <template #footer>
      <el-button @click="creatorDialog = false">{{
        fieldLabel("取消")
      }}</el-button>
      <el-button
        type="primary"
        :loading="submitting"
        @click="submitProjectResource"
      >
        {{ creatorDialogMode === "create" ? "添加" : "保存" }}
      </el-button>
    </template>
  </el-dialog>

  <div v-if="false" v-loading="loading" class="campaign-detail-page">
    <header class="detail-topbar">
      <div class="title-cluster">
        <el-button circle @click="goBack">
          <IconifyIconOnline icon="ri:arrow-left-line" />
        </el-button>
        <div class="campaign-logo">
          <IconifyIconOnline icon="ri:megaphone-line" />
        </div>
        <div>
          <el-select
            :model-value="selectedProjectId"
            filterable
            :placeholder="fieldLabel('选择营销项目')"
            @change="handleProjectChange"
          >
            <el-option
              v-for="item in projects"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            />
          </el-select>
          <h1>{{ project?.name || "项目执行页" }}</h1>
          <p>
            目标：{{ project?.campaignType || "未设置合作目标" }}
            <span />
            当前周期：{{ cycleLabel }}
          </p>
        </div>
      </div>
      <div class="top-actions">
        <el-tag :type="activeStatusType" round>{{ activeStatusLabel }}</el-tag>
        <el-button @click="openProjectDialog">
          <IconifyIconOnline icon="ri:edit-line" />
          编辑项目
        </el-button>
        <el-button @click="toggleProjectStatus">
          <IconifyIconOnline
            :icon="isPaused ? 'ri:play-line' : 'ri:pause-line'"
          />
          {{ isPaused ? "恢复" : "暂停" }}
        </el-button>
        <el-button circle>
          <IconifyIconOnline icon="ri:more-line" />
        </el-button>
      </div>
    </header>

    <div class="detail-body">
      <aside class="detail-side-nav">
        <button
          v-for="item in navItems"
          :key="item.key"
          type="button"
          :class="{ active: activeSection === item.key }"
          @click="setSection(item.key)"
        >
          <IconifyIconOnline :icon="item.icon" />
          <span>{{ fieldLabel(item.label) }}</span>
        </button>
        <div class="nav-fold">{{ fieldLabel("收起") }} &laquo;</div>
      </aside>

      <main class="detail-main">
        <template v-if="activeSection === 'collaboration'">
          <section class="progress-panel">
            <button
              v-for="(stage, index) in pipelineStages"
              :key="stage.key"
              type="button"
              :class="{
                active: activePipelineStage === stage.key,
                done: stage.count > 0 || index < 2
              }"
              @click="activePipelineStage = stage.key"
            >
              <span>
                <IconifyIconOnline
                  :icon="
                    stage.count > 0 || index < 2 ? 'ri:check-line' : stage.icon
                  "
                />
              </span>
              <div>
                <strong>{{ fieldLabel(stage.label) }}</strong>
                <p>{{ stage.count }} 位资源 · {{ stage.description }}</p>
              </div>
            </button>
          </section>

          <section class="collaboration-panel">
            <div class="collaboration-heading">
              <div>
                <h2>{{ fieldLabel("协作执行") }}</h2>
                <p>
                  {{ project?.platform || "全平台" }} ·
                  {{ stats.cooperationCount || 0 }} 条合作执行记录
                </p>
              </div>
              <el-tag effect="plain"
                >{{ fieldLabel("全部") }} {{ cooperations.length }}</el-tag
              >
            </div>

            <div class="assurance-strip">
              <div>
                <IconifyIconOnline icon="ri:user-star-line" />
                <strong>{{ fieldLabel("真实内容流量") }}</strong>
                <span>{{ fieldLabel("真实达人内容沉淀为可复盘数据") }}</span>
              </div>
              <div>
                <IconifyIconOnline icon="ri:shield-check-line" />
                <strong>{{ fieldLabel("发布保障") }}</strong>
                <span>{{ stats.completionRate || 0 }}% 发布完成率</span>
              </div>
              <div>
                <IconifyIconOnline icon="ri:line-chart-line" />
                <strong>{{ fieldLabel("当前 CPM") }}</strong>
                <span
                  >{{
                    moneyText(cpmValue(stats.totalCost, stats.totalReach))
                  }}
                  CPM</span
                >
              </div>
            </div>

            <section class="tip-bar">
              <strong>{{ fieldLabel("提示") }}</strong>
              <span
                >建议优先处理待确认报价、内容审核和发布链接回收，避免项目节点堆积。</span
              >
            </section>

            <section class="pending-section">
              <div class="section-heading">
                <div>
                  <strong>{{ fieldLabel("待处理事项") }}</strong>
                  <span>{{ pendingActions.length }} 项需要人工确认</span>
                </div>
              </div>
              <div v-if="pendingActions.length" class="pending-card-row">
                <button
                  v-for="item in pendingActions"
                  :key="item.id"
                  type="button"
                  :class="{
                    active: Number(item.id) === Number(focusedCooperation?.id)
                  }"
                  @click="openExecutionDetail(item)"
                >
                  <div class="creator-avatar">
                    {{ String(item.resourceName || "R").slice(0, 1) }}
                  </div>
                  <div>
                    <span>{{ item.action }}</span>
                    <strong>{{ item.resourceName || "未命名资源" }}</strong>
                    <p>{{ updatedTimeText(item) }} · 等待确认</p>
                  </div>
                  <IconifyIconOnline icon="ri:arrow-right-s-line" />
                </button>
              </div>
              <el-empty
                v-else
                :description="fieldLabel('当前没有需要人工处理的动作')"
              />
            </section>

            <section class="workspace-grid">
              <div class="table-panel">
                <div class="section-heading">
                  <div>
                    <strong>{{ fieldLabel("合作资源") }}</strong>
                    <span>{{ fieldLabel("点击行即可查看内容交付详情") }}</span>
                  </div>
                </div>
                <div class="stage-filter">
                  <button
                    type="button"
                    :class="{ active: activePipelineStage === 'all' }"
                    @click="activePipelineStage = 'all'"
                  >
                    全部 {{ cooperations.length }}
                  </button>
                  <button
                    v-for="stage in pipelineStages"
                    :key="`filter-${stage.key}`"
                    type="button"
                    :class="{ active: activePipelineStage === stage.key }"
                    @click="activePipelineStage = stage.key"
                  >
                    {{ fieldLabel(stage.label) }} {{ stage.count }}
                  </button>
                </div>
                <el-table
                  :data="pipelineRows"
                  stripe
                  class="influencer-table"
                  :row-class-name="executionRowClassName"
                  @row-click="openExecutionDetail"
                >
                  <el-table-column
                    prop="resourceName"
                    :label="fieldLabel('资源')"
                    min-width="170"
                  />
                  <el-table-column :label="fieldLabel('状态')" width="160">
                    <template #default="{ row }">
                      <el-tag :type="cooperationStageTag(row)" effect="light">{{
                        cooperationStageLabel(row)
                      }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column :label="fieldLabel('报价')" width="130">
                    <template #default="{ row }">{{
                      moneyText(row.quoteAmount, row.currency)
                    }}</template>
                  </el-table-column>
                  <el-table-column label="CPM" width="130">
                    <template #default="{ row }">{{
                      moneyText(
                        cpmValue(row.quoteAmount, primaryReach(row)),
                        row.currency
                      )
                    }}</template>
                  </el-table-column>
                </el-table>
              </div>

              <article v-if="focusedCooperation" class="creator-detail-panel">
                <header class="creator-header">
                  <el-avatar
                    :src="focusedCooperation.resourceAvatarUrl"
                    :size="64"
                  >
                    {{
                      String(focusedCooperation.resourceName || "R").slice(0, 1)
                    }}
                  </el-avatar>
                  <div>
                    <h3>
                      {{ focusedCooperation.resourceName || "未命名资源" }}
                      <el-tag
                        :type="cooperationStageTag(focusedCooperation)"
                        round
                        >{{ cooperationStageLabel(focusedCooperation) }}</el-tag
                      >
                    </h3>
                    <p>
                      <span v-for="star in 5" :key="star" class="rating-star"
                        ><IconifyIconOnline icon="ri:star-fill"
                      /></span>
                      {{
                        focusedCooperation.teamRating ||
                        focusedCooperation.level ||
                        5
                      }}
                      <span />
                      {{
                        focusedCooperation.platformHandle
                          ? `@${focusedCooperation.platformHandle}`
                          : focusedCooperation.platform || "全平台"
                      }}
                      <span />
                      {{
                        focusedCooperation.language ||
                        project?.language ||
                        "全部语言"
                      }}
                      <span />
                      <CooperationTypeTags
                        :value="focusedCooperation.cooperationType"
                        :empty-text="fieldLabel('未设置合作形式')"
                      />
                    </p>
                  </div>
                  <div class="creator-actions">
                    <el-button circle
                      ><IconifyIconOnline icon="ri:star-line"
                    /></el-button>
                    <el-button @click="openInfluencerReport"
                      >反馈异常</el-button
                    >
                  </div>
                </header>

                <div class="quality-strip">
                  <strong>
                    <IconifyIconOnline icon="ri:verified-badge-line" />
                    系统根据活跃度、互动率、履约记录和数据真实性辅助评估合作质量
                  </strong>
                  <div>
                    <span>{{ fieldLabel("近期活跃") }}</span>
                    <span>{{ fieldLabel("互动表现较好") }}</span>
                    <span>{{ fieldLabel("数据可信") }}</span>
                    <span>{{ fieldLabel("履约记录良好") }}</span>
                  </div>
                </div>

                <section
                  v-if="cooperationStage(focusedCooperation) === 'published'"
                  class="success-banner"
                >
                  <strong>{{ fieldLabel("合作已完成") }}</strong>
                  <p>
                    {{
                      fieldLabel(
                        "如发现内容质量、异常播放或互动问题，可提交异常反馈。"
                      )
                    }}
                  </p>
                  <el-button link type="primary" @click="openInfluencerReport"
                    >提交异常反馈</el-button
                  >
                </section>

                <div class="detail-tabs">
                  <button
                    type="button"
                    :class="{ active: detailTab === 'overview' }"
                    @click="detailTab = 'overview'"
                  >
                    概览
                  </button>
                  <button
                    type="button"
                    :class="{ active: detailTab === 'content' }"
                    @click="detailTab = 'content'"
                  >
                    内容交付
                  </button>
                  <button
                    type="button"
                    :class="{ active: detailTab === 'reviews' }"
                    @click="detailTab = 'reviews'"
                  >
                    评价
                  </button>
                </div>

                <section v-if="detailTab === 'overview'" class="overview-grid">
                  <div>
                    <span>{{ fieldLabel("粉丝数") }}</span
                    ><strong>{{
                      formatCount(focusedCooperation.followers)
                    }}</strong>
                  </div>
                  <div>
                    <span>{{ fieldLabel("互动率") }}</span
                    ><strong>{{
                      ratioPercent(focusedCooperation.engagementRate, 1)
                    }}</strong>
                  </div>
                  <div>
                    <span>{{ fieldLabel("播放/曝光") }}</span
                    ><strong>{{
                      formatCount(primaryReach(focusedCooperation))
                    }}</strong>
                  </div>
                  <div>
                    <span>{{ fieldLabel("点击") }}</span
                    ><strong>{{
                      formatCount(focusedCooperation.clicks)
                    }}</strong>
                  </div>
                </section>

                <template v-if="detailTab === 'content'">
                  <section class="content-block">
                    <h3>{{ fieldLabel("内容信息") }}</h3>
                    <div class="tracking-link">
                      <span
                        >为该资源生成的专属追踪链接，用于发布内容中追踪点击和效果。</span
                      >
                      <el-link
                        type="primary"
                        :href="
                          focusedCooperation.trackingLink ||
                          focusedCooperation.finalLink
                        "
                        target="_blank"
                      >
                        {{
                          focusedCooperation.trackingLink ||
                          focusedCooperation.finalLink
                            ? "打开追踪链接"
                            : "等待达人提交内容链接"
                        }}
                      </el-link>
                    </div>
                  </section>

                  <section class="delivery-timeline">
                    <div
                      v-for="item in currentDeliverables"
                      :key="item.id"
                      class="completed"
                    >
                      <span />
                      <article>
                        <div>
                          <strong>{{ deliverableTitleText(item) }}</strong>
                          <el-tag size="small" effect="plain">{{
                            deliverableStatusText(item.status)
                          }}</el-tag>
                        </div>
                        <p>
                          {{
                            item.submittedAt ||
                            cleanDisplayText(item.note) ||
                            "等待提交"
                          }}
                        </p>
                        <div class="timeline-note">
                          <el-link
                            v-if="item.link"
                            type="primary"
                            :href="item.link"
                            target="_blank"
                            >打开交付链接</el-link
                          >
                          <p v-if="item.caption">
                            文案：{{ cleanDisplayText(item.caption) }}
                          </p>
                          <p v-if="item.note">
                            {{ cleanDisplayText(item.note) }}
                          </p>
                          <p v-if="item.rejectionReason">
                            驳回原因：{{
                              cleanDisplayText(item.rejectionReason)
                            }}
                          </p>
                          <p
                            v-if="
                              item.stageKey === 'final_link' &&
                              focusedCooperation.adAuthorizationCode
                            "
                          >
                            广告授权码：{{
                              focusedCooperation.adAuthorizationCode
                            }}
                          </p>
                        </div>
                      </article>
                    </div>
                  </section>
                </template>

                <section v-if="detailTab === 'reviews'" class="content-block">
                  <h3>{{ fieldLabel("评价") }}</h3>
                  <div class="tracking-link">
                    <span>{{ fieldLabel("团队评分") }}</span>
                    <strong>{{ focusedCooperation.teamRating || "-" }}</strong>
                    <p>{{ focusedCooperation.notes || "暂无备注" }}</p>
                  </div>
                </section>
              </article>
            </section>
          </section>
        </template>

        <section
          v-if="activeSection === 'report'"
          class="section-card report-panel"
        >
          <div class="section-headline">
            <h2>{{ fieldLabel("效果报告") }}</h2>
            <div class="headline-actions">
              <el-segmented
                v-model="reportScope"
                :options="['campaign', 'influencer']"
              />
              <el-date-picker
                :model-value="[project?.cycleStartDate, project?.cycleEndDate]"
                type="daterange"
                disabled
                range-separator="-"
              />
              <el-button @click="handleExportProjectData">
                <IconifyIconOnline icon="ri:download-line" />
                下载
              </el-button>
            </div>
          </div>

          <div class="report-meta">
            <span
              >项目周期 <strong>{{ cycleLabel }}</strong></span
            >
            <span
              >项目目标
              <strong>{{ project?.campaignType || "-" }}</strong></span
            >
            <span
              >报告更新时间
              <strong>{{ project?.reportUpdateDate || "-" }}</strong></span
            >
            <span
              >已发生费用 <strong>{{ moneyText(costToDate) }}</strong></span
            >
          </div>

          <template v-if="reportScope === 'campaign'">
            <div class="report-filters">
              <el-select v-model="reportAudience">
                <el-option :label="fieldLabel('全部受众')" value="all" />
                <el-option
                  v-for="item in audienceOptions"
                  :key="item"
                  :label="item"
                  :value="item"
                />
              </el-select>
              <el-select v-model="reportPlatform">
                <el-option :label="fieldLabel('全部平台')" value="all" />
                <el-option
                  v-for="item in platformOptions"
                  :key="item"
                  :label="item"
                  :value="item"
                />
              </el-select>
              <el-select v-model="reportCreative">
                <el-option :label="fieldLabel('全部创意')" value="all" />
                <el-option
                  v-for="item in creativeOptions"
                  :key="item"
                  :label="item"
                  :value="item"
                />
              </el-select>
            </div>

            <div class="forecast-table">
              <div />
              <strong>{{ fieldLabel("播放") }}</strong>
              <strong>{{ fieldLabel("点击") }}</strong>
              <strong>CPM</strong>
              <strong>CPC</strong>
              <span>{{ fieldLabel("预测") }}</span>
              <b>{{ formatCount(visibleReportSummary.forecastViews) }}</b>
              <b>{{ formatCount(visibleReportSummary.forecastClicks) }}</b>
              <b>{{ moneyText(visibleReportSummary.forecastCPM) }}</b>
              <b>{{ moneyText(visibleReportSummary.forecastCPC) }}</b>
              <span>{{ fieldLabel("实际") }}</span>
              <b>{{ formatCount(visibleReportSummary.actualViews) }}</b>
              <b>{{ formatCount(visibleReportSummary.actualClicks) }}</b>
              <b>{{ moneyText(visibleReportSummary.actualCPM) }}</b>
              <b>{{ moneyText(visibleReportSummary.actualCPC) }}</b>
            </div>

            <div class="chart-toolbar">
              <div class="metric-tabs">
                <button
                  type="button"
                  :class="{ active: reportMetric === 'views' }"
                  @click="reportMetric = 'views'"
                >
                  播放
                </button>
                <button
                  type="button"
                  :class="{ active: reportMetric === 'clicks' }"
                  @click="reportMetric = 'clicks'"
                >
                  点击
                </button>
                <button
                  type="button"
                  :class="{ active: reportMetric === 'cpm' }"
                  @click="reportMetric = 'cpm'"
                >
                  CPM
                </button>
                <button
                  type="button"
                  :class="{ active: reportMetric === 'cpc' }"
                  @click="reportMetric = 'cpc'"
                >
                  CPC
                </button>
              </div>
              <div class="view-by">
                <span>{{ fieldLabel("查看维度") }}</span>
                <el-select v-model="reportViewBy">
                  <el-option :label="fieldLabel('受众')" value="audience" />
                  <el-option :label="fieldLabel('平台')" value="platform" />
                  <el-option :label="fieldLabel('创意')" value="creative" />
                </el-select>
              </div>
            </div>
            <div ref="chartRef" class="report-chart" />
          </template>

          <el-table v-else :data="cooperations" border class="report-table">
            <el-table-column
              :label="fieldLabel('资源')"
              min-width="220"
              sortable
            >
              <template #default="{ row }">
                <div class="influencer-cell">
                  <el-avatar :src="row.resourceAvatarUrl">{{
                    String(row.resourceName || "R").slice(0, 1)
                  }}</el-avatar>
                  <div>
                    <strong>{{ row.resourceName || "未命名资源" }}</strong>
                    <span>{{
                      row.platformHandle
                        ? `@${row.platformHandle}`
                        : row.platform || "-"
                    }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="fieldLabel('最终链接')" min-width="220">
              <template #default="{ row }">
                <el-link
                  v-if="row.finalLink || row.deliverableLinks"
                  type="primary"
                  :href="row.finalLink || row.deliverableLinks"
                  target="_blank"
                >
                  打开链接
                </el-link>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column
              :label="fieldLabel('订单价格')"
              width="140"
              sortable
            >
              <template #default="{ row }">{{
                moneyText(row.quoteAmount, row.currency)
              }}</template>
            </el-table-column>
            <el-table-column
              prop="language"
              :label="fieldLabel('语言')"
              width="130"
              sortable
            />
            <el-table-column
              prop="topGeographies"
              :label="fieldLabel('主要地区')"
              min-width="220"
              sortable
            />
            <el-table-column
              prop="publishTime"
              :label="fieldLabel('发布时间')"
              width="180"
              sortable
            />
          </el-table>
        </section>

        <section
          v-if="activeSection === 'budget'"
          class="section-card budget-panel"
        >
          <div class="section-headline">
            <h2>{{ fieldLabel("预算") }}</h2>
            <div class="headline-actions">
              <el-date-picker
                :model-value="[project?.cycleStartDate, project?.cycleEndDate]"
                type="daterange"
                disabled
                range-separator="-"
              />
              <el-button type="primary" @click="renewDialog = true"
                >Renew campaign</el-button
              >
            </div>
          </div>
          <div class="budget-card">
            <div>
              <h3>{{ fieldLabel("达人营销预算") }}</h3>
              <span>{{ fieldLabel("已发生费用") }}</span>
              <strong>{{ moneyText(costToDate) }}</strong>
            </div>
            <div>
              <span>{{ fieldLabel("总预算") }}</span>
              <strong>{{ moneyText(projectBudget) }}</strong>
              <el-button circle text @click="openBudgetDialog"
                ><IconifyIconOnline icon="ri:edit-line"
              /></el-button>
            </div>
            <el-button link type="primary" @click="showBillingHistory"
              >查看账单流水</el-button
            >
          </div>
        </section>

        <section
          v-if="activeSection === 'campaignInfo'"
          class="section-card info-panel"
        >
          <div class="section-headline">
            <h2>{{ fieldLabel("项目信息") }}</h2>
            <el-button @click="openProjectDialog">
              <IconifyIconOnline icon="ri:edit-line" />
              编辑项目
            </el-button>
          </div>
          <dl class="info-list">
            <div>
              <dt>{{ fieldLabel("名称") }}</dt>
              <dd>{{ project?.name || "-" }}</dd>
            </div>
            <div>
              <dt>{{ fieldLabel("目标") }}</dt>
              <dd>{{ project?.campaignType || "-" }}</dd>
            </div>
            <div>
              <dt>{{ fieldLabel("周期") }}</dt>
              <dd>{{ cycleLabel }}</dd>
            </div>
            <div>
              <dt>{{ fieldLabel("市场") }}</dt>
              <dd>{{ project?.targetMarket || "-" }}</dd>
            </div>
            <div>
              <dt>{{ fieldLabel("语言") }}</dt>
              <dd>{{ project?.language || "-" }}</dd>
            </div>
            <div>
              <dt>{{ fieldLabel("平台") }}</dt>
              <dd>{{ project?.platform || "-" }}</dd>
            </div>
            <div>
              <dt>{{ fieldLabel("负责人") }}</dt>
              <dd>{{ project?.owner || "-" }}</dd>
            </div>
            <div>
              <dt>{{ fieldLabel("状态") }}</dt>
              <dd>{{ activeStatusLabel }}</dd>
            </div>
            <div>
              <dt>{{ fieldLabel("预算") }}</dt>
              <dd>{{ moneyText(projectBudget) }}</dd>
            </div>
            <div class="wide">
              <dt>{{ fieldLabel("需求摘要") }}</dt>
              <dd>{{ cleanDisplayText(project?.brief) || "暂无需求摘要" }}</dd>
            </div>
          </dl>
        </section>
      </main>
    </div>

    <el-dialog
      v-model="projectDialog"
      :title="fieldLabel('编辑项目')"
      width="680px"
    >
      <el-form :model="projectForm" label-width="120px">
        <el-form-item :label="fieldLabel('项目名称')"
          ><el-input v-model="projectForm.name"
        /></el-form-item>
        <el-form-item :label="fieldLabel('目标市场')"
          ><el-select
            v-model="projectForm.targetMarkets"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            :max-collapse-tags="3"
            :placeholder="fieldLabel('搜索中文、英文或国家代码')"
            class="w-full!"
          >
            <el-option
              v-for="country in projectCountryOptions"
              :key="country.code || country.name"
              :label="country.label"
              :value="country.name"
            >
              <span>{{ country.name }}</span>
              <small class="country-option-meta">
                {{ country.englishName }}
                {{ country.code ? `· ${country.code}` : "" }}
              </small>
            </el-option>
          </el-select></el-form-item
        >
        <el-form-item :label="fieldLabel('语言')"
          ><el-input v-model="projectForm.language"
        /></el-form-item>
        <el-form-item :label="fieldLabel('目标')"
          ><el-input v-model="projectForm.campaignType"
        /></el-form-item>
        <el-form-item :label="fieldLabel('项目周期')">
          <el-date-picker
            v-model="projectCycleRange"
            type="daterange"
            unlink-panels
            range-separator="至"
            :start-placeholder="fieldLabel('开始日期')"
            :end-placeholder="fieldLabel('结束日期')"
            format="YYYY-MM-DD"
            value-format="YYYY-MM-DD"
            class="w-full!"
          />
        </el-form-item>
        <el-form-item :label="fieldLabel('预算')"
          ><el-input-number
            v-model="projectForm.budget"
            :min="0"
            class="w-full!"
        /></el-form-item>
        <el-form-item :label="fieldLabel('负责人')"
          ><el-input v-model="projectForm.owner"
        /></el-form-item>
        <el-form-item label="Brief"
          ><el-input v-model="projectForm.brief" type="textarea" :rows="4"
        /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="projectDialog = false">{{
          fieldLabel("取消")
        }}</el-button>
        <el-button type="primary" :loading="submitting" @click="submitProject"
          >保存</el-button
        >
      </template>
    </el-dialog>

    <el-dialog
      v-model="budgetDialog"
      :title="fieldLabel('编辑预算')"
      width="420px"
    >
      <el-input-number v-model="budgetForm.budget" :min="0" class="w-full!" />
      <template #footer>
        <el-button @click="budgetDialog = false">{{
          fieldLabel("取消")
        }}</el-button>
        <el-button type="primary" @click="submitBudget">{{
          fieldLabel("保存")
        }}</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="renewDialog"
      :title="fieldLabel('续期项目')"
      width="480px"
    >
      <el-form :model="renewForm" label-width="96px">
        <el-form-item :label="fieldLabel('开始日期')"
          ><el-date-picker
            v-model="renewForm.cycleStartDate"
            value-format="YYYY-MM-DD"
            type="date"
        /></el-form-item>
        <el-form-item :label="fieldLabel('结束日期')"
          ><el-date-picker
            v-model="renewForm.cycleEndDate"
            value-format="YYYY-MM-DD"
            type="date"
        /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renewDialog = false">{{
          fieldLabel("取消")
        }}</el-button>
        <el-button type="primary" @click="submitRenew">{{
          fieldLabel("续期")
        }}</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="reportDialog"
      :title="fieldLabel('提交异常反馈')"
      width="520px"
    >
      <el-form :model="influencerReportForm" label-width="96px">
        <el-form-item :label="fieldLabel('原因')"
          ><el-input v-model="influencerReportForm.reason"
        /></el-form-item>
        <el-form-item :label="fieldLabel('说明')"
          ><el-input
            v-model="influencerReportForm.detail"
            type="textarea"
            :rows="4"
        /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reportDialog = false">{{
          fieldLabel("取消")
        }}</el-button>
        <el-button type="primary" @click="submitInfluencerReport"
          >提交</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.campaign-detail-page {
  min-height: 100vh;
  color: #20242a;
  background: #f7f7f8;
}

.detail-topbar {
  position: sticky;
  top: 0;
  z-index: 5;
  display: flex;
  gap: 18px;
  align-items: center;
  justify-content: space-between;
  min-height: 82px;
  padding: 12px 24px;
  background: #fff;
  border-bottom: 1px solid #ebeef2;
}

.detail-topbar > .title-cluster,
.detail-topbar > .top-actions {
  max-width: 1280px;
}

.title-cluster,
.top-actions,
.creator-header,
.creator-actions,
.headline-actions,
.chart-toolbar,
.view-by {
  display: flex;
  gap: 12px;
  align-items: center;
}

.title-cluster {
  min-width: 0;
}

.title-cluster h1 {
  margin: 7px 0 4px;
  overflow: hidden;
  font-size: 18px;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: 0;
}

.title-cluster p,
.collaboration-heading p,
.pending-card-row p,
.creator-header p,
.tracking-link span,
.delivery-timeline p,
.report-meta,
.section-heading span {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: #7a828f;
}

.title-cluster p span,
.creator-header p > span:not(.rating-star) {
  display: inline-block;
  width: 1px;
  height: 12px;
  margin: 0 10px;
  vertical-align: -2px;
  background: #d7dce3;
}

.campaign-logo {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 42px;
  height: 42px;
  color: #f26522;
  background: #fff7ed;
  border: 1px solid #fed7aa;
  border-radius: 8px;
}

.detail-body {
  display: grid;
  grid-template-columns: 168px minmax(0, 1fr);
  gap: 16px;
  max-width: 1280px;
  padding: 16px 24px 28px;
  margin: 0 auto;
}

.detail-side-nav {
  position: sticky;
  top: 100px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-self: start;
  min-height: 350px;
  padding: 10px 8px;
  background: #fff;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.detail-side-nav button,
.progress-panel button,
.pending-card-row button,
.stage-filter button,
.detail-tabs button,
.metric-tabs button {
  font: inherit;
  cursor: pointer;
  border: 0;
}

.detail-side-nav button {
  display: flex;
  gap: 9px;
  align-items: center;
  padding: 10px 11px;
  color: #334155;
  text-align: left;
  background: transparent;
  border-radius: 6px;
}

.detail-side-nav button.active,
.detail-side-nav button:hover {
  color: #9a4b2f;
  background: #fff4e8;
}

.nav-fold {
  margin-top: auto;
  padding: 8px 12px;
  font-size: 12px;
  color: #69717d;
  text-align: right;
}

.detail-main,
.collaboration-panel,
.pending-section,
.table-panel,
.creator-detail-panel,
.content-block,
.section-card {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.progress-panel,
.collaboration-panel,
.section-card {
  background: #fff;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.progress-panel {
  position: relative;
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0;
  padding: 34px 26px 28px;
}

.progress-panel::before {
  display: none;
}

.progress-panel button {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
  align-content: start;
  min-width: 0;
  padding: 0 24px 0 0;
  text-align: left;
  background: transparent;
}

.progress-panel button::after {
  position: absolute;
  top: 14px;
  right: 16px;
  left: 44px;
  z-index: 0;
  display: block;
  height: 3px;
  content: "";
  background: #e6ebf2;
  border-radius: 999px;
}

.progress-panel button.done::after {
  background: #ff6422;
}

.progress-panel button:last-child::after {
  display: none;
}

.progress-panel button > span {
  z-index: 1;
  box-sizing: border-box;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  color: #94a3b8;
  background: #fff;
  border: 2px solid #d8dde6;
  border-radius: 50%;
  box-shadow: 0 0 0 8px #fff;
}

.progress-panel button > span svg {
  font-size: 15px;
}

.progress-panel button.done > span {
  color: #fff;
  background: #ff6422;
  border-color: #ff6422;
}

.progress-panel button.active > span {
  color: #ff6422;
  background: #fff;
  border: 3px solid #ff6422;
  box-shadow:
    0 0 0 7px #fff,
    0 0 0 9px rgb(255 100 34 / 18%);
}

.progress-panel strong,
.pending-card-row strong,
.section-heading strong {
  color: #20242a;
}

.progress-panel p {
  margin: 6px 0 0;
  overflow: hidden;
  font-size: 12px;
  line-height: 1.5;
  color: #8a919c;
  text-overflow: ellipsis;
  white-space: normal;
}

.collaboration-panel,
.section-card {
  padding: 18px;
}

.collaboration-heading,
.section-heading,
.section-headline {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.collaboration-heading h2,
.section-headline h2,
.creator-header h3,
.content-block h3,
.budget-card h3 {
  margin: 0;
  color: #20242a;
  letter-spacing: 0;
}

.collaboration-heading h2,
.section-headline h2 {
  font-size: 22px;
}

.assurance-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.assurance-strip > div {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 3px 10px;
  align-items: center;
  min-height: 62px;
  padding: 12px 16px;
  border-right: 1px solid #e9edf2;
}

.assurance-strip > div:last-child {
  border-right: 0;
}

.assurance-strip svg {
  grid-row: span 2;
  font-size: 20px;
  color: #48a763;
}

.assurance-strip span,
.pending-card-row span {
  overflow: hidden;
  font-size: 12px;
  color: #8a919c;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tip-bar,
.success-banner {
  display: grid;
  gap: 8px;
  padding: 12px 14px;
  border-radius: 8px;
}

.tip-bar {
  grid-template-columns: auto minmax(0, 1fr);
  align-items: start;
  background: #eef6ff;
}

.tip-bar strong {
  padding: 2px 8px;
  color: #fff;
  background: #5b8def;
  border-radius: 999px;
}

.success-banner {
  background: #eafaef;
}

.success-banner p {
  margin: 0;
  color: #5b6470;
}

.pending-card-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(220px, 1fr));
  gap: 12px;
  overflow-x: auto;
  padding-bottom: 4px;
}

.pending-card-row button {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  min-height: 92px;
  padding: 14px;
  text-align: left;
  background: #fff;
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.pending-card-row button:hover,
.pending-card-row button.active {
  border-color: #f26522;
  box-shadow: 0 10px 24px rgb(242 101 34 / 12%);
}

.creator-avatar {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  font-weight: 700;
  color: #f26522;
  background: #fff1e8;
  border-radius: 50%;
}

.workspace-grid {
  display: grid;
  grid-template-columns: minmax(430px, 0.9fr) minmax(520px, 1.1fr);
  gap: 16px;
  align-items: start;
}

.stage-filter,
.detail-tabs,
.metric-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.stage-filter button,
.metric-tabs button {
  padding: 6px 10px;
  font-size: 12px;
  color: #475569;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
}

.stage-filter button.active {
  color: #fff;
  background: #30343a;
  border-color: #30343a;
}

.metric-tabs button.active {
  color: #9a4b2f;
  background: #ffd0a8;
  border-color: #ffd0a8;
}

.creator-detail-panel {
  padding: 16px;
  border: 1px solid #edf0f4;
  border-radius: 8px;
}

.creator-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
}

.creator-header h3 {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  margin: 0;
  font-size: 20px;
}

.rating-star {
  color: #111827;
}

.quality-strip {
  display: grid;
  gap: 10px;
  padding: 12px 14px;
  background: #f8fafc;
  border: 1px solid #eef2f7;
  border-radius: 8px;
}

.quality-strip strong {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 12px;
  color: #48a763;
}

.quality-strip > div,
.overview-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.quality-strip span,
.overview-grid > div {
  padding: 8px 10px;
  overflow: hidden;
  font-size: 12px;
  color: #4b5563;
  text-overflow: ellipsis;
  white-space: nowrap;
  background: #f8fafc;
  border-radius: 6px;
}

.overview-grid strong {
  display: block;
  margin-top: 5px;
  font-size: 18px;
  color: #20242a;
}

.detail-tabs {
  gap: 28px;
  border-bottom: 1px solid #e5e7eb;
}

.detail-tabs button {
  padding: 0 0 10px;
  color: #4b5563;
  background: transparent;
  border-bottom: 2px solid transparent;
}

.detail-tabs button.active {
  color: #f26522;
  border-bottom-color: #f26522;
}

.tracking-link,
.timeline-note {
  display: grid;
  gap: 6px;
  padding: 12px;
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.timeline-note {
  max-width: 720px;
  background: #fffbea;
  border: 0;
}

.delivery-timeline {
  display: grid;
  padding-top: 8px;
}

.delivery-timeline > div {
  position: relative;
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr);
  gap: 12px;
  padding-bottom: 22px;
}

.delivery-timeline > div::before {
  position: absolute;
  top: 17px;
  bottom: -1px;
  left: 8px;
  width: 1px;
  content: "";
  background: #ffd1bb;
}

.delivery-timeline > div:last-child::before {
  display: none;
}

.delivery-timeline > div > span {
  z-index: 1;
  width: 9px;
  height: 9px;
  margin: 5px auto 0;
  background: #fff;
  border: 2px solid #ff6422;
  border-radius: 50%;
  box-shadow: 0 0 0 4px #fff;
}

.delivery-timeline > div.completed > span {
  background: #ff6422;
}

.delivery-timeline article {
  display: grid;
  gap: 8px;
}

.delivery-timeline article > div:first-child {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.report-meta,
.report-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  align-items: center;
}

.report-filters .el-select {
  width: 220px;
}

.forecast-table {
  display: grid;
  grid-template-columns: 150px repeat(4, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid #e3e7ec;
  border-radius: 8px;
}

.forecast-table > * {
  padding: 16px 18px;
  border-bottom: 1px solid #e9edf2;
}

.forecast-table > *:nth-last-child(-n + 5) {
  border-bottom: 0;
}

.forecast-table strong {
  color: #747b86;
}

.forecast-table b {
  font-size: 18px;
}

.chart-toolbar {
  justify-content: space-between;
}

.view-by .el-select {
  width: 140px;
}

.report-chart {
  width: 100%;
  height: 420px;
}

.influencer-cell {
  display: flex;
  gap: 12px;
  align-items: center;
}

.influencer-cell > div {
  display: grid;
  gap: 4px;
}

.influencer-cell span {
  color: #3b82f6;
}

.budget-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto;
  gap: 24px;
  align-items: center;
  min-height: 160px;
  padding: 34px 40px;
  border: 1px solid #d9dde5;
  border-radius: 8px;
}

.budget-card span,
.info-list dt {
  color: #5f6672;
}

.budget-card strong {
  display: inline-block;
  margin-top: 16px;
  font-size: 30px;
}

.info-list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin: 0;
}

.info-list > div {
  display: grid;
  gap: 6px;
  padding: 14px;
  background: #f8fafc;
  border-radius: 8px;
}

.info-list .wide {
  grid-column: 1 / -1;
}

.info-list dd {
  margin: 0;
  color: #20242a;
}

.date-separator {
  margin: 0 8px;
  color: #8a919c;
}

:deep(.influencer-table),
:deep(.report-table) {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

:deep(.influencer-table th.el-table__cell),
:deep(.report-table th.el-table__cell) {
  color: #747b86;
  background: #f8fafc;
}

:deep(.influencer-table .el-table__row) {
  cursor: pointer;
}

:deep(.execution-selected-row td.el-table__cell) {
  background: #fff7ed !important;
}

@media (width <= 980px) {
  .detail-topbar,
  .collaboration-heading,
  .section-headline,
  .chart-toolbar,
  .budget-card {
    display: grid;
    align-items: start;
  }

  .detail-body,
  .progress-panel,
  .assurance-strip,
  .workspace-grid,
  .quality-strip > div,
  .overview-grid,
  .forecast-table,
  .info-list {
    grid-template-columns: 1fr;
  }

  .detail-side-nav {
    position: static;
    min-height: 0;
  }

  .progress-panel button::after {
    display: none;
  }

  .assurance-strip > div {
    border-right: 0;
    border-bottom: 1px solid #e9edf2;
  }

  .creator-header {
    grid-template-columns: 1fr;
  }
}
/* Simplified campaign workspace, aligned with the three-view campaign layout. */
.campaign-workspace {
  min-height: 100vh;
  color: #24252b;
  background: #fff;
}
.campaign-header {
  display: flex;
  gap: 18px;
  align-items: center;
  justify-content: space-between;
  min-height: 66px;
  padding: 10px 22px;
  border-bottom: 1px solid #e8e8eb;
}
.campaign-heading,
.campaign-actions,
.campaign-meta,
.toolbar,
.toolbar-actions,
.creator-cell,
.handle-cell,
.content-author,
.content-info > div,
.section-title-row {
  display: flex;
  align-items: center;
}
.campaign-heading {
  gap: 14px;
  min-width: 0;
}
.campaign-actions {
  gap: 10px;
  flex: 0 0 auto;
}
.back-button {
  color: #53565d;
  border-color: #e4e5e8;
}
.campaign-mark {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  color: #f4a42a;
  background: #fff;
  border: 1px solid #e7e7ea;
  border-radius: 10px;
}
.campaign-mark svg {
  font-size: 20px;
}
.campaign-name {
  min-width: 0;
}
.campaign-name h1 {
  max-width: 580px;
  margin: 0;
  overflow: hidden;
  font-size: 18px;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: -0.02em;
}
.campaign-meta {
  gap: 8px;
  margin-top: 4px;
  color: #a0a2a8;
  font-size: 12px;
}
.campaign-meta > span {
  display: inline-flex;
  gap: 5px;
  align-items: center;
}
.campaign-meta svg {
  color: #b7bac0;
}
.project-switcher {
  width: 156px;
}
.cycle-label {
  color: #34353a;
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
}
.campaign-tabs {
  display: flex;
  gap: 24px;
  padding: 0 22px;
  border-bottom: 1px solid #e8e8eb;
}
.campaign-tabs button {
  padding: 13px 0 10px;
  color: #777a81;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  background: transparent;
  border: 0;
  border-bottom: 2px solid transparent;
}
.campaign-tabs button:hover {
  color: #24252b;
}
.campaign-tabs button.active {
  color: #24252b;
  border-bottom-color: #24252b;
}
.campaign-main {
  max-width: 1720px;
  padding: 24px 22px 36px;
  margin: 0 auto;
}
.metric-section + .metric-section {
  margin-top: 24px;
}
.section-title {
  margin-bottom: 12px;
}
.section-title h2 {
  margin: 0;
  font-size: 16px;
  line-height: 1.3;
  letter-spacing: -0.01em;
}
.section-title-row {
  justify-content: space-between;
}
.metric-grid {
  display: grid;
  gap: 12px;
}
.content-metrics {
  grid-template-columns: minmax(200px, 1.5fr) minmax(320px, 1.5fr) repeat(
      3,
      minmax(160px, 0.85fr)
    );
}
.engagement-metrics {
  grid-template-columns:
    repeat(2, minmax(210px, 0.8fr)) minmax(360px, 1.45fr)
    minmax(210px, 0.8fr);
}
.performance-metrics {
  grid-template-columns: minmax(280px, 0.9fr) minmax(380px, 1.35fr) minmax(
      280px,
      0.9fr
    );
}
.metric-card {
  min-width: 0;
  min-height: 82px;
  padding: 15px;
  background: #fff;
  border: 1px solid #e3e4e8;
  border-radius: 12px;
}
.metric-card > span,
.split-card span {
  display: block;
  color: #85878e;
  font-size: 14px;
  line-height: 1.25;
  text-decoration: underline dotted #b8bac0 2px;
  text-underline-offset: 5px;
}
.metric-card strong {
  display: block;
  margin-top: 9px;
  color: #24252b;
  font-size: 24px;
  line-height: 1;
  letter-spacing: -0.035em;
}
.metric-card em {
  color: #565960;
  font-size: 18px;
  font-style: normal;
}
.split-card {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  padding: 0;
  overflow: hidden;
}
.split-card > div {
  min-width: 0;
  padding: 15px;
}
.split-card > div + div {
  border-left: 1px solid #e6e7ea;
}
.split-card strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.overview-section + .overview-section {
  margin-top: 26px;
}
.overview-section-heading,
.platform-chart-card > header,
.platform-metric-switch,
.platform-legend-row {
  display: flex;
  align-items: center;
}
.overview-section-heading {
  gap: 18px;
  justify-content: space-between;
  margin-bottom: 13px;
}
.overview-section-heading h2,
.platform-chart-card h3 {
  margin: 0;
  color: #24252b;
  letter-spacing: -0.015em;
}
.overview-section-heading h2 {
  font-size: 17px;
}
.overview-section-heading p,
.platform-chart-card p {
  margin: 5px 0 0;
  color: #858991;
  font-size: 12px;
  line-height: 1.45;
}
.overview-section-heading > span {
  flex: 0 0 auto;
  padding: 5px 9px;
  color: #5f6672;
  font-size: 12px;
  background: #f5f6f8;
  border-radius: 6px;
}
.overview-metric-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 10px;
}
.overview-metric-card {
  box-sizing: border-box;
  min-width: 0;
  min-height: 116px;
  padding: 16px;
  background: #fff;
  border: 1px solid #e3e6eb;
  border-radius: 11px;
  box-shadow: 0 1px 2px rgb(15 23 42 / 3%);
}
.overview-metric-card.primary-metric-card {
  background: #f7f9ff;
  border-color: #cedafb;
}
.overview-metric-card > span {
  display: block;
  overflow: hidden;
  color: #737983;
  font-size: 13px;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.overview-metric-card strong {
  display: block;
  margin-top: 13px;
  overflow: hidden;
  color: #20242a;
  font-size: 27px;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: -0.035em;
}
.overview-metric-card.primary-metric-card strong {
  color: #2f63e7;
}
.overview-metric-card em {
  color: #747a85;
  font-size: 18px;
  font-style: normal;
  font-weight: 600;
}
.overview-metric-card small {
  display: block;
  margin-top: 10px;
  overflow: hidden;
  color: #9a9ea6;
  font-size: 11px;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.platform-overview-section {
  padding-top: 2px;
}
.platform-chart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}
.platform-chart-card {
  min-width: 0;
  padding: 18px;
  background: #fff;
  border: 1px solid #e3e6eb;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgb(15 23 42 / 4%);
}
.platform-chart-card > header {
  gap: 14px;
  justify-content: space-between;
  min-height: 40px;
}
.platform-chart-card h3 {
  font-size: 15px;
}
.platform-chart-body {
  display: grid;
  grid-template-columns: minmax(220px, 0.9fr) minmax(250px, 1.1fr);
  gap: 16px;
  align-items: center;
  margin-top: 14px;
}
.platform-pie-chart {
  width: 100%;
  height: 270px;
}
.platform-legend {
  display: grid;
  gap: 3px;
  align-content: center;
  min-width: 0;
  max-height: 270px;
  overflow: auto;
}
.platform-legend-row {
  display: grid;
  grid-template-columns: 8px 30px minmax(72px, 1fr) auto 44px;
  gap: 7px;
  min-height: 38px;
  padding: 3px 5px;
  color: #555b65;
  font-size: 12px;
  border-bottom: 1px solid #f0f1f3;
}
.platform-legend-row:last-child {
  border-bottom: 0;
}
.legend-color {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.platform-legend-row :deep(.platform-icon-badge) {
  width: 30px;
  height: 30px;
}
.platform-legend-row :deep(.platform-icon-badge img) {
  width: 20px;
  height: 20px;
}
.platform-legend-row :deep(.el-tag) {
  max-width: 30px;
  overflow: hidden;
  padding: 0 4px;
  text-overflow: ellipsis;
}
.platform-legend-row strong {
  overflow: hidden;
  color: #30343a;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.platform-legend-row > span:nth-last-child(2) {
  color: #32363d;
  font-weight: 600;
  white-space: nowrap;
}
.platform-legend-row em {
  color: #8a9099;
  font-style: normal;
  text-align: right;
  white-space: nowrap;
}
.platform-metric-switch {
  flex: 0 0 auto;
  gap: 2px;
  padding: 3px;
  background: #f1f3f6;
  border-radius: 8px;
}
.platform-metric-switch button {
  padding: 6px 10px;
  color: #6c727c;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  background: transparent;
  border: 0;
  border-radius: 6px;
}
.platform-metric-switch button.active {
  color: #2f63e7;
  font-weight: 600;
  background: #fff;
  box-shadow: 0 1px 3px rgb(15 23 42 / 10%);
}
.creator-page,
.content-page {
  min-width: 0;
}
.toolbar {
  gap: 12px;
  justify-content: space-between;
  padding-bottom: 14px;
  border-bottom: 1px solid #e8e8eb;
}
.search-field {
  width: min(440px, 100%);
}
.toolbar-actions {
  gap: 10px;
  color: #2e3036;
  font-size: 14px;
  white-space: nowrap;
}
.creator-table {
  width: 100%;
  margin-top: 0;
}
.creator-toolbar {
  flex-wrap: wrap;
  justify-content: flex-start;
  padding-bottom: 18px;
}
.creator-toolbar .toolbar-actions {
  margin-left: auto;
}
.creator-filter {
  width: 148px;
}
.creator-part-heading {
  display: flex;
  gap: 16px;
  align-items: flex-end;
  justify-content: space-between;
  padding: 20px 2px 12px;
}
.creator-part-heading h2 {
  margin: 0;
  font-size: 17px;
}
.creator-part-heading p {
  margin: 5px 0 0;
  color: #858991;
  font-size: 12px;
}
.creator-part-heading > span {
  color: #747981;
  font-size: 13px;
  white-space: nowrap;
}
.media-part-heading {
  padding-top: 30px;
  margin-top: 14px;
  border-top: 1px solid #e8e8eb;
}
.creator-cell {
  gap: 10px;
}
.creator-profile-link {
  padding: 0;
  font: inherit;
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: 0;
}
.creator-profile-link:hover strong {
  color: #2f63e7;
}
.creator-cell > div {
  display: grid;
  gap: 3px;
}
.creator-cell strong {
  color: #292b31;
  font-size: 14px;
}
.creator-cell span {
  color: #898b91;
  font-size: 12px;
}
.creator-cell .creator-name-line {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  color: inherit;
}
.creator-platform-icons {
  display: inline-flex;
  gap: 2px;
  align-items: center;
}
.creator-platform-icons :deep(.platform-icon-badge) {
  width: 22px;
  height: 22px;
}
.creator-platform-icons :deep(.platform-icon-badge img) {
  width: 18px;
  height: 18px;
}
.creator-platform-icons :deep(.platform-icon-badge--x img) {
  width: 14px;
  height: 14px;
}
.creator-platform-icons :deep(.el-tag) {
  height: 20px;
  padding: 0 5px;
  font-size: 10px;
}
.creator-platform {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  color: #484d55;
}
.latest-content-thumb {
  width: 92px;
  height: 58px;
  padding: 0;
  overflow: hidden;
  cursor: pointer;
  background: #f1f4f8;
  border: 1px solid #e1e6ec;
  border-radius: 8px;
}
.latest-content-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.latest-content-thumb > span {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #69717c;
}
.latest-content-thumb small {
  font-size: 10px;
}
.latest-content-thumb:hover {
  border-color: #9eb7ef;
}
.latest-content-empty {
  color: #a2a6ad;
  font-size: 12px;
}
.creator-type-cell {
  display: flex;
  gap: 8px;
  align-items: center;
}
.creator-type-cell span {
  color: #73767c;
  font-size: 12px;
}
.handle-cell {
  gap: 6px;
  color: #45474d;
}
.handle-cell svg {
  color: #24252b;
}
.creator-edit-alert {
  margin-bottom: 18px;
}
.online-search-option-content {
  display: flex;
  gap: 8px;
  align-items: center;
  color: var(--el-color-primary);
}
.online-search-option-content small {
  max-width: 220px;
  margin-left: auto;
  overflow: hidden;
  color: #8a8d94;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.online-search-panel {
  padding: 16px;
  margin-bottom: 18px;
  background: #f7f9fc;
  border: 1px solid #dde6f4;
  border-radius: 12px;
}
.online-search-panel > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
  color: #3568d4;
}
.online-search-panel > header div {
  display: grid;
  gap: 3px;
}
.online-search-panel > header span {
  color: #858a94;
  font-size: 12px;
}
.online-search-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 12px;
}
.online-search-result {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 11px;
  background: #fff;
  border: 1px solid #dce7dc;
  border-radius: 10px;
}
.online-search-result > div {
  display: grid;
  flex: 1;
  gap: 3px;
}
.online-search-result span {
  color: #7d8189;
  font-size: 12px;
}
.creator-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}
.creator-form-grid__wide {
  grid-column: 1 / -1;
}
.platform-filter {
  width: 155px;
}
.content-sort-filter {
  width: 190px;
}
.content-toolbar {
  justify-content: flex-start;
}
.content-count {
  margin-left: auto;
  color: #85878e;
  font-size: 13px;
}
.content-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 18px;
  margin-top: 18px;
}
.content-card {
  overflow: hidden;
  cursor: pointer;
  background: #fff;
  border: 1px solid #ececef;
  border-radius: 14px;
  transition:
    transform 160ms ease,
    box-shadow 160ms ease;
}
.content-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 26px rgb(31 35 42 / 10%);
}
.content-author {
  gap: 8px;
  min-width: 0;
  padding: 12px 13px;
}
.content-author-copy {
  display: grid !important;
  gap: 2px !important;
  min-width: 0;
  margin-top: 0 !important;
}
.content-author-copy strong {
  min-width: 0;
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.content-author-copy span {
  color: #8a8f97;
  font-size: 11px;
}
.content-card-actions {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  margin-left: auto;
}
.content-card-actions .el-button {
  color: #747983;
}
.content-cover-wrap {
  position: relative;
  overflow: hidden;
  background: #f5f5f6;
}
.content-cover {
  display: block;
  width: 100%;
  aspect-ratio: 16 / 9;
  object-fit: cover;
  background: #f5f5f6;
}
.empty-cover {
  display: grid;
  place-items: center;
  color: #a5a7ad;
  font-size: 32px;
}
.viral-badge {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 1;
  padding: 4px 8px;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  background: rgb(220 38 38 / 92%);
  border-radius: 999px;
  box-shadow: 0 3px 10px rgb(127 29 29 / 22%);
}
.content-info {
  padding: 12px 13px 14px;
}
.content-card-cooperation-types {
  margin-bottom: 9px;
}
.content-type-tag {
  font-weight: 600;
  border-radius: 0;
}
.content-card-type-tag {
  margin-bottom: 9px;
  margin-left: 6px;
}
.content-card-type-tag:first-child {
  margin-left: 0;
}
.content-post-link {
  display: block;
  max-width: 100%;
  margin-top: 8px;
  overflow: hidden;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.content-info > div {
  gap: 12px;
  margin-top: 11px;
  color: #777a81;
  font-size: 12px;
}
.content-info > .content-card-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}
.content-info .content-card-metrics > span {
  display: grid;
  gap: 3px;
  min-width: 0;
  padding: 8px 9px;
  background: #f7f8fa;
}
.content-card-metrics small {
  color: #8a8f97;
  font-size: 10px;
}
.content-card-metrics strong {
  overflow: hidden;
  color: #30343a;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.content-info span {
  display: inline-flex;
  gap: 4px;
  align-items: center;
}
.content-detail-page {
  max-width: 1180px;
  margin: 0 auto;
}
.content-detail-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}
.content-detail-back {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  padding: 0;
  margin-bottom: 0;
  color: #555c66;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  background: transparent;
  border: 0;
}
.content-detail-back:hover {
  color: #2f63e7;
}
.content-detail-hero {
  overflow: hidden;
  background: #fff;
  border: 1px solid #e3e6eb;
  border-radius: 14px;
  box-shadow: 0 8px 30px rgb(15 23 42 / 6%);
}
.content-detail-cover {
  position: relative;
  display: block;
  width: 100%;
  padding: 0;
  overflow: hidden;
  cursor: pointer;
  background: #15181d;
  border: 0;
}
.content-detail-cover > img,
.content-detail-cover-empty {
  display: flex;
  width: 100%;
  aspect-ratio: 16 / 8;
  object-fit: cover;
}
.content-detail-cover-empty {
  flex-direction: column;
  gap: 10px;
  align-items: center;
  justify-content: center;
  color: #d8dde5;
  background: linear-gradient(145deg, #252c36, #11151b);
}
.content-detail-cover-empty strong {
  font-size: 16px;
}
.content-detail-cover-empty small {
  color: #9ca6b4;
}
.website-preview-empty {
  color: #445468;
  background:
    radial-gradient(circle at 20% 0%, rgb(59 130 246 / 12%), transparent 35%),
    linear-gradient(145deg, #eef4fa, #dce7f1);
}
.website-preview-empty small {
  color: #738195;
}
.website-preview-window {
  display: grid;
  grid-template-columns: repeat(3, 7px) minmax(0, 1fr);
  gap: 7px;
  align-items: center;
  width: min(540px, 72%);
  padding: 13px 16px;
  color: #3e4b5d;
  background: rgb(255 255 255 / 88%);
  border: 1px solid rgb(148 163 184 / 35%);
  border-radius: 10px;
  box-shadow: 0 18px 45px rgb(51 65 85 / 14%);
}
.website-preview-window i {
  width: 7px;
  height: 7px;
  background: #b8c3d0;
  border-radius: 50%;
}
.website-preview-window strong {
  min-width: 0;
  margin-left: 6px;
  overflow: hidden;
  font-size: 12px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.content-detail-play {
  position: absolute;
  top: 50%;
  left: 50%;
  display: grid;
  width: 64px;
  height: 48px;
  color: #fff;
  font-size: 28px;
  background: #f52345;
  border-radius: 14px;
  box-shadow: 0 8px 24px rgb(0 0 0 / 24%);
  transform: translate(-50%, -50%);
  place-items: center;
}
.content-detail-play.website-open-icon {
  background: #176b87;
}
.detail-viral-badge {
  top: 18px;
  right: 18px;
  padding: 7px 11px;
  font-size: 12px;
}
.content-detail-summary {
  padding: 22px 24px;
}
.content-detail-title-row {
  display: flex;
  gap: 24px;
  align-items: flex-start;
  justify-content: space-between;
}
.content-detail-title-row > div > span {
  color: #e34b45;
  font-size: 12px;
  font-weight: 700;
}
.content-detail-platform {
  display: inline-flex;
  gap: 7px;
  align-items: center;
}
.content-detail-platform :deep(.platform-icon-badge) {
  width: 22px;
  height: 22px;
}
.content-detail-platform :deep(.platform-icon-badge img) {
  width: 15px;
  height: 15px;
}
.content-detail-cooperation-types {
  margin-top: 10px;
}
.content-detail-type-tag {
  margin-top: 10px;
  margin-left: 6px;
  vertical-align: middle;
}
.content-detail-author {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 20px;
}
.content-detail-author > div {
  display: grid;
  gap: 4px;
}
.content-detail-author strong {
  color: #2e333a;
  font-size: 15px;
}
.content-detail-author span {
  color: #858b94;
  font-size: 12px;
}
.content-detail-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-top: 18px;
  overflow: hidden;
  background: #fff;
  border: 1px solid #e3e6eb;
  border-radius: 12px;
}
.content-detail-metrics article {
  min-width: 0;
  padding: 22px;
  border-right: 1px solid #e7e9ed;
}
.content-detail-metrics article:last-child {
  border-right: 0;
}
.content-detail-metrics span,
.content-detail-metrics small {
  display: block;
  color: #838993;
  font-size: 12px;
}
.content-detail-metrics strong {
  display: block;
  margin: 10px 0 8px;
  color: #242931;
  font-size: 24px;
  line-height: 1;
}
.content-detail-metrics small {
  overflow: hidden;
  color: #a0a5ad;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.content-detail-link-row {
  display: grid;
  grid-template-columns: 90px minmax(0, 1fr);
  gap: 12px;
  padding: 18px 20px;
  margin-top: 18px;
  background: #f8fafc;
  border: 1px solid #e7eaf0;
  border-radius: 10px;
}
.content-platform-option {
  display: flex;
  gap: 8px;
  align-items: center;
}
.content-platform-option :deep(.platform-icon-badge) {
  width: 24px;
  height: 24px;
}
.content-platform-option :deep(.platform-icon-badge img) {
  width: 16px;
  height: 16px;
}
.content-detail-link-row > span {
  color: #737983;
  font-size: 13px;
}
.content-detail-link-row .el-link {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:deep(.campaign-workspace .el-button--primary) {
  --el-button-bg-color: #2f63e7;
  --el-button-border-color: #2f63e7;
  --el-button-hover-bg-color: #2558d7;
  --el-button-hover-border-color: #2558d7;
  border-radius: 9px;
}
:deep(.campaign-workspace .el-input__wrapper),
:deep(.campaign-workspace .el-select__wrapper) {
  min-height: 40px;
  border-radius: 10px;
  box-shadow: 0 0 0 1px #e1e2e6 inset;
}
:deep(.campaign-workspace .el-table) {
  --el-table-border-color: #e7e8eb;
  --el-table-header-bg-color: #fff;
  --el-table-row-hover-bg-color: #f8f9fb;
  font-size: 14px;
}
:deep(.campaign-workspace .el-table th.el-table__cell) {
  height: 66px;
  color: #282a30;
  font-weight: 650;
}
:deep(.campaign-workspace .el-table td.el-table__cell) {
  height: 68px;
}
@media (max-width: 1180px) {
  .overview-metric-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .platform-chart-grid {
    grid-template-columns: 1fr;
  }
  .content-metrics,
  .engagement-metrics,
  .performance-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .wide-card {
    grid-column: span 2;
  }
  .campaign-header {
    align-items: flex-start;
  }
  .campaign-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }
}
@media (max-width: 720px) {
  .campaign-header,
  .campaign-main {
    padding-right: 16px;
    padding-left: 16px;
  }
  .campaign-header {
    flex-direction: column;
  }
  .campaign-actions {
    width: 100%;
    justify-content: flex-start;
  }
  .project-switcher {
    width: 130px;
  }
  .campaign-tabs {
    gap: 20px;
    padding: 0 16px;
    overflow-x: auto;
  }
  .metric-grid,
  .content-metrics,
  .engagement-metrics,
  .performance-metrics {
    grid-template-columns: 1fr;
  }
  .overview-metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .overview-section-heading {
    align-items: flex-start;
    flex-direction: column;
  }
  .platform-chart-card {
    padding: 14px;
  }
  .platform-chart-card > header {
    align-items: flex-start;
    flex-direction: column;
  }
  .platform-chart-body {
    grid-template-columns: 1fr;
  }
  .platform-pie-chart {
    height: 240px;
  }
  .platform-legend {
    max-height: none;
  }
  .wide-card {
    grid-column: auto;
  }
  .toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
  .content-detail-title-row {
    flex-direction: column;
  }
  .content-detail-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .content-detail-metrics article:nth-child(2) {
    border-right: 0;
  }
  .toolbar-actions {
    width: 100%;
  }
  .content-count {
    margin-left: 0;
  }
  .search-field {
    width: 100%;
  }
  .creator-form-grid {
    grid-template-columns: 1fr;
  }
  .creator-form-grid__wide {
    grid-column: auto;
  }
}

.umv-value {
  display: inline-flex;
  flex-direction: column;
  align-items: flex-end;
  line-height: 1.2;
}

.umv-value small {
  margin-top: 3px;
  color: #98a2b3;
  font-size: 11px;
  font-weight: 500;
}

@media (max-width: 460px) {
  .overview-metric-grid,
  .content-detail-metrics {
    grid-template-columns: 1fr;
  }
  .content-detail-metrics article,
  .content-detail-metrics article:nth-child(2) {
    border-right: 0;
    border-bottom: 1px solid #e7e9ed;
  }
  .content-detail-metrics article:last-child {
    border-bottom: 0;
  }
}
</style>
