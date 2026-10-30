import { http } from "@/utils/http";

export type TrendTab = "tag" | "video";
export type TrendPoint = { timestamp: string; value: number };
export type TrendCreator = {
  ttUID?: string;
  nickname: string;
  handleName?: string;
  avatarURL: string;
  followedCnt?: string;
};
export type TrendTag = {
  hashtagID: string;
  hashtagName: string;
  rankIndex?: string;
  publishCnt: string;
  vv: string;
  popularityCurve: TrendPoint[];
  topCreators?: TrendCreator[];
};
export type TrendVideo = {
  itemID: string;
  title: string;
  coverURL: string;
  createTime?: string;
  videoURL?: string;
  author?: {
    name: string;
    handle?: string;
    avatarURL: string;
    followers: string;
  };
  metrics?: Record<string, number | string>;
  tags?: { contentLabelName: string }[];
};
export type TrendTagDetail = TrendTag & {
  ageProfile?: { ageLevel: string; vvPercent: string }[];
  representativeCountryProfile?: {
    countryCode: string;
    countryTgiScore: string;
  }[];
  videoList?: TrendRelatedVideo[];
};
export type TrendRelatedVideo = {
  itemID: string;
  coverURL: string;
  vid?: string;
  videoURL?: string | Record<string, string>;
};
export type TrendMeta = {
  source: "archive" | "live";
  capturedAt: string;
  curveEnd: string;
  coverage: "complete" | "limited";
  pageMismatch?: boolean;
  sourceURL: string;
};
export type TrendQuery = {
  tab: TrendTab;
  region: string;
  period: number;
  orderByMetric: number;
  currentPage: number;
  pageSize: number;
  live?: number;
  refresh?: number;
};
export type TrendOptions = {
  collectionMode?: "local" | "browser";
  tagRegions: string[];
  videoRegions: string[];
  tagPeriods: number[];
  videoPeriods: number[];
};
type Result<T> = { code: number; message: string; data: T };
export const getTrendOptions = () =>
  http.request<Result<TrendOptions>>("get", "/business/trends/options");
export const getTrendList = (params: TrendQuery) =>
  http.request<
    Result<{
      list: (TrendTag | TrendVideo)[];
      total: number;
      currentPage: number;
      pageSize: number;
      hasMore: boolean;
      meta: TrendMeta;
    }>
  >("get", "/business/trends/list", { params }, { timeout: 25000 });
export const getTrendDetail = (params: TrendQuery & { id: string }) =>
  http.request<
    Result<{ detail: TrendTagDetail | TrendVideo; meta: TrendMeta }>
  >("get", "/business/trends/detail", { params }, { timeout: 25000 });

export function safeTrendImageURL(value: string): string {
  try {
    const url = new URL(value);
    if (
      url.protocol === "https:" &&
      !url.username &&
      !url.password &&
      ["tiktokcdn.com", "tiktokcdn-us.com", "ibytedtos.com"].some(
        domain => url.hostname === domain || url.hostname.endsWith(`.${domain}`)
      )
    )
      return url.href;
  } catch {
    /* Keep invalid image URLs out of image elements. */
  }
  return "";
}

// Both image and video assets must use the observed TikTok CDN HTTPS domains.
export const safeTrendVideoURL = safeTrendImageURL;

export type TrendCollection = {
  desktopAvailable?: boolean;
  loginMode?: "external";
  id: string;
  state:
    | "starting"
    | "collecting"
    | "needs_login"
    | "complete"
    | "failed"
    | "cancelled";
  message: string;
  completed: number;
  total: number;
  cache: string;
  expiresAt: string;
};
export type TrendLoginScreen = {
  image: string;
  target: string;
  origin: string;
  tabs: { id: string; origin: string }[];
  width: number;
  height: number;
};
export type TrendLoginInput = {
  target: string;
  kind: "click" | "wheel" | "text" | "key";
  x?: number;
  y?: number;
  delta?: number;
  text?: string;
  key?: string;
};
export const startTrendCollection = (params: TrendQuery) =>
  http.request<Result<TrendCollection>>("post", "/business/trends/collect", {
    params
  });
export const getTrendCollection = (job: string) =>
  http.request<Result<TrendCollection>>(
    "get",
    `/business/trends/collect/${encodeURIComponent(job)}`
  );
export const cancelTrendCollection = (job: string) =>
  http.request<Result<{ cancelled: boolean }>>(
    "post",
    `/business/trends/collect/${encodeURIComponent(job)}/cancel`
  );
export const getTrendLoginScreen = (job: string, target = "") =>
  http.request<Result<TrendLoginScreen>>(
    "get",
    `/business/trends/collect/${encodeURIComponent(job)}/screen`,
    { params: { target } },
    { timeout: 12000 }
  );
export const sendTrendLoginInput = (job: string, data: TrendLoginInput) =>
  http.request<Result<{ accepted: boolean }>>(
    "post",
    `/business/trends/collect/${encodeURIComponent(job)}/input`,
    { data },
    { timeout: 12000 }
  );

export const restartTrendLogin = (job: string) =>
  http.request<Result<{ opened: boolean }>>(
    "post",
    `/business/trends/collect/${encodeURIComponent(job)}/login`,
    {},
    { timeout: 40000 }
  );

export const resumeTrendCollection = (job: string) =>
  http.request<Result<TrendCollection>>(
    "post",
    `/business/trends/collect/${encodeURIComponent(job)}/resume`
  );

export const openTrendDesktop = (job: string) =>
  http.request<Result<{ url: string }>>(
    "post",
    `/business/trends/collect/${encodeURIComponent(job)}/desktop`
  );

export type TrendLocalJob = {
  id: string;
  state:
    | "waiting_upload"
    | "processing"
    | "complete"
    | "partial"
    | "failed"
    | "cancelled";
  message: string;
  expiresAt: string;
  imagesSaved: number;
  imagesFailed: number;
  regions: {
    region: string;
    state: string;
    message: string;
    imagesSaved: number;
    imagesFailed: number;
  }[];
};
export const createTrendLocalJob = (
  params: TrendQuery & { regions?: string }
) =>
  http.request<Result<TrendLocalJob & { token: string }>>(
    "post",
    "/business/trends/local-jobs",
    { params }
  );
export const getTrendLocalJob = (id: string) =>
  http.request<Result<TrendLocalJob>>(
    "get",
    `/business/trends/local-jobs/${encodeURIComponent(id)}`
  );
export const cancelTrendLocalJob = (id: string) =>
  http.request<Result<TrendLocalJob>>(
    "post",
    `/business/trends/local-jobs/${encodeURIComponent(id)}/cancel`
  );
export const downloadTrendLocalClient = (platform: string) =>
  http.request<Blob>(
    "get",
    `/business/trends/local-client/${encodeURIComponent(platform)}`,
    { responseType: "blob" },
    { timeout: 180000 }
  );

export type MonthlyTrend = {
  id: string;
  month: string;
  title: string;
  sourceDate: string;
  publishedAt: string;
  dateConflict: boolean;
  summary: string;
  audioLabel: string;
  audioURL: string;
  videoURL: string;
  videoID: string;
  coverURL: string;
};
export type MonthlyTrendResult = {
  months: { month: string; count: number }[];
  month: string;
  list: MonthlyTrend[];
  total: number;
  sourceURL: string;
  sourceUpdatedAt: string;
  capturedAt: string;
};
export const getMonthlyTrends = (month = "") =>
  http.request<Result<MonthlyTrendResult>>("get", "/business/trends/monthly", {
    params: { month }
  });
export const refreshMonthlyTrends = (month = "") =>
  http.request<
    Result<{
      capturedAt: string;
      sourceUpdatedAt: string;
      mediaMode: "tiktok-embed";
    }>
  >(
    "post",
    "/business/trends/monthly/refresh",
    { params: { month } },
    { timeout: 25000 }
  );
