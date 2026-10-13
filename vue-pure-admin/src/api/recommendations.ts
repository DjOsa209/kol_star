import { http } from "@/utils/http";

export type RecommendationDomain = "creator" | "ip";
export type RecommendationFeedback = "interested" | "not_interested";

export interface RecommendationSubscriptionInput {
  id?: number;
  domain: RecommendationDomain;
  name: string;
  query: string;
  markets: string[];
  platforms?: string[];
  resourceTypes?: string[];
  ipTypes?: string[];
  audience?: string;
  goal?: string;
  brand?: string;
  product?: string;
  requestId?: number;
  budgetCurrency?: string;
  budgetMin?: number | null;
  budgetMax?: number | null;
  active?: boolean;
}

export interface RecommendationSubscription extends RecommendationSubscriptionInput {
  id: number;
  active: boolean;
  lastRunAt?: string | null;
  createdAt?: string;
}

export interface RecommendationResult {
  id: number;
  subscriptionId: number;
  resourceId: number;
  resourceName: string;
  score: number;
  reason: string;
  risks?: string[] | string;
  advice?: string;
  adviceSource?: "ai" | "rule";
  scenarios?: string[];
  evidence?: RecommendationEvidence[];
  evidenceStatus?: "available" | "none" | "unavailable" | "not_searched";
  source: "ai" | "rule" | string;
  isNew: boolean;
  feedback?: RecommendationFeedback | null;
  createdAt: string;
  resource: Record<string, any>;
}

export interface RecommendationEvidence {
  title: string;
  url: string;
  source: string;
  publishedAt?: string;
  summary?: string;
}

export interface RecommendationNotification {
  id: number;
  domain: RecommendationDomain;
  title: string;
  description: string;
  createdAt: string;
  readAt?: string | null;
  targetPath: string;
}

type Result<T> = { code: number; message: string; data: T };
type RecommendationPage = { list: RecommendationResult[]; total: number };
const prefix = "/business/recommendations";

export const listRecommendationSubscriptions = (domain: RecommendationDomain) =>
  http.request<Result<{ list: RecommendationSubscription[] }>>(
    "post",
    `${prefix}/subscriptions/list`,
    { data: { domain } }
  );

export const saveRecommendationSubscription = (
  input: RecommendationSubscriptionInput
) =>
  http.request<Result<{ id: number; warning?: string }>>(
    "post",
    `${prefix}/subscriptions/save`,
    { data: input },
    { timeout: 130000 }
  );

export const deleteRecommendationSubscription = (
  domain: RecommendationDomain,
  id: number
) =>
  http.request<Result<null>>("post", `${prefix}/subscriptions/delete`, {
    data: { domain, id }
  });

export const runRecommendationSubscription = (
  domain: RecommendationDomain,
  subscriptionId: number
) =>
  http.request<Result<{ results: RecommendationResult[] }>>(
    "post",
    `${prefix}/run`,
    { data: { domain, subscriptionId } },
    { timeout: 130000 }
  );

export const listRecommendationResults = (
  domain: RecommendationDomain,
  subscriptionId?: number,
  view: "new" | "all" = "all"
) =>
  http.request<Result<RecommendationPage>>("post", `${prefix}/results/list`, {
    data: { domain, subscriptionId, view }
  });

export const sendRecommendationFeedback = (
  domain: RecommendationDomain,
  id: number,
  feedback: RecommendationFeedback
) =>
  http.request<Result<null>>("post", `${prefix}/results/feedback`, {
    data: { domain, id, feedback }
  });

export const listRecommendationNotifications = () =>
  http.request<Result<{ list: RecommendationNotification[]; unread: number }>>(
    "get",
    `${prefix}/notifications`
  );

export const markRecommendationNotificationsRead = (ids: number[]) =>
  http.request<Result<null>>("post", `${prefix}/notifications/read`, {
    data: { ids }
  });
