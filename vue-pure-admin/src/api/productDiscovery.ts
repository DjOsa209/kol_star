import { http } from "@/utils/http";

type Result<T> = { code: number; message: string; data: T };
export type DiscoveryMode = "resource" | "brand" | "product";
export interface DiscoveryPage<T = Record<string, any>> {
  list: T[];
  total: number;
  currentPage: number;
  pageSize: number;
}

export interface DiscoveryProduct {
  id: number;
  name: string;
  brandId: number;
  brandName: string;
  resourceCount: number;
  contentCount: number;
  totalViews: number;
  coverUrl?: string;
}

export interface ProductCooperation {
  cooperationId: number;
  projectId: number;
  projectName: string;
  resourceId: number;
  resourceName: string;
  platform: string;
  contentUrl?: string;
  coverUrl?: string;
  views?: number;
  impressions?: number;
  engagementCount?: number;
  publishTime?: string;
  sourceKind?: string;
  sourceField?: string;
  sourceText?: string;
  reviewedEvidenceUrl?: string;
  confirmedBy?: number;
  confirmedByName?: string;
  confirmedAt?: string;
}

export interface ProductCandidate {
  id: number;
  cooperationId: number;
  projectName: string;
  resourceName: string;
  suggestedProduct: string;
  sourceField: string;
  sourceText?: string;
  evidenceUrl?: string;
  status: string;
}

export const searchProductDiscovery = (data: {
  mode: DiscoveryMode;
  keyword: string;
  currentPage: number;
  pageSize: number;
}) =>
  http.request<Result<DiscoveryPage>>("post", "/business/discovery/search", {
    data
  });

export const getDiscoveryProductDetail = (productId: number) =>
  http.request<
    Result<{ product: DiscoveryProduct; cooperations: ProductCooperation[] }>
  >("post", "/business/discovery/product/detail", {
    data: { productId }
  });

export const refreshProductCandidates = () =>
  http.request<Result<{ scanned: number; created: number }>>(
    "post",
    "/business/discovery/candidates/refresh",
    { data: {} },
    { timeout: 60000 }
  );

export const listProductCandidates = (data: {
  status: string;
  currentPage: number;
  pageSize: number;
}) =>
  http.request<Result<DiscoveryPage<ProductCandidate>>>(
    "post",
    "/business/discovery/candidates/list",
    { data }
  );

export const decideProductCandidate = (data: {
  candidateId: number;
  decision: "confirmed" | "rejected";
  brandName?: string;
  productName?: string;
}) =>
  http.request<Result<{ updated: boolean }>>(
    "post",
    "/business/discovery/candidates/decision",
    { data }
  );
