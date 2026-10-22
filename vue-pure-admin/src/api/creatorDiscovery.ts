import { http } from "@/utils/http";

export type DiscoveryStatus =
  | "queued"
  | "planning"
  | "searching"
  | "collecting"
  | "recommending"
  | "completed"
  | "partial"
  | "failed"
  | "cancelled";

export interface DiscoveryPost {
  platformPostId?: string;
  postId?: string;
  title?: string;
  description?: string;
  url?: string;
  postUrl?: string;
  coverUrl?: string;
  publishedAt?: string;
  viewCount?: number | null;
  likeCount?: number | null;
  commentCount?: number | null;
}

export interface DiscoverySnapshot {
  channelId: string;
  name: string;
  handle: string;
  profileUrl: string;
  avatarUrl?: string;
  avatarRemoteUrl?: string;
  description?: string;
  country?: string;
  language?: string;
  followers: number | null;
  totalViews?: number | null;
  videoCount?: number | null;
  recentPosts: DiscoveryPost[];
  evidence: { title: string; url: string; query?: string; summary?: string }[];
  collectedAt?: string;
  warning?: string;
}

export interface DiscoveryCandidate {
  id: number;
  jobId: number;
  channelId: string;
  status: "pending" | "approved" | "rejected";
  resourceId?: number;
  country?: string;
  language?: string;
  note?: string;
  evidenceUrl?: string;
  industry?: string;
  category?: string;
  tagIds?: number[];
  snapshot: DiscoverySnapshot;
}

export interface DiscoveryRecommendation {
  source: "ai";
  modelInvoked: boolean;
  model: string;
  recommendations: Record<string, any>[];
  message: string;
  demandText?: string;
  generatedAt?: string;
}

export interface DiscoveryJob {
  id: number;
  market: string;
  query: string;
  queries: string[];
  maxCandidates: number;
  status: DiscoveryStatus;
  message: string;
  discoveredCount: number;
  reviewedCount: number;
  importedCount: number;
  createdAt: string;
  updatedAt: string;
  recommendation?: DiscoveryRecommendation;
}

export interface DiscoveryInput {
  query: string;
  market?: string;
  queries?: string[];
  maxCandidates?: number;
}

interface Result<T> {
  code: number;
  message: string;
  data: T;
}

const prefix = "/business/creator-discovery";
export const createDiscoveryJob = (data: DiscoveryInput) =>
  http.request<Result<{ jobId: number }>>("post", `${prefix}/jobs/create`, {
    data
  });
export const listDiscoveryJobs = () =>
  http.request<Result<{ jobs: DiscoveryJob[] }>>(
    "post",
    `${prefix}/jobs/list`,
    {
      data: {}
    }
  );
export const getDiscoveryJob = (id: number) =>
  http.request<Result<{ job: DiscoveryJob; candidates: DiscoveryCandidate[] }>>(
    "post",
    `${prefix}/jobs/detail`,
    { data: { id } }
  );
export const cancelDiscoveryJob = (id: number) =>
  http.request<Result<unknown>>("post", `${prefix}/jobs/cancel`, {
    data: { id }
  });
export const retryDiscoveryJob = (id: number) =>
  http.request<Result<{ jobId: number }>>("post", `${prefix}/jobs/retry`, {
    data: { id }
  });
export const reviewDiscoveryCandidate = (data: {
  id: number;
  action: "approve" | "reject";
  country: string;
  language: string;
  note: string;
  evidenceUrl: string;
  topicConfirmed: boolean;
  industry?: string;
  category?: string;
  tagIds?: number[];
}) =>
  http.request<Result<{ resourceId?: number; status: string }>>(
    "post",
    `${prefix}/candidates/review`,
    { data }
  );
export const recommendDiscoveryJob = (id: number, demandText: string) =>
  http.request<Result<DiscoveryRecommendation>>(
    "post",
    `${prefix}/jobs/recommend`,
    { data: { id, demandText } },
    { timeout: 135000 }
  );
