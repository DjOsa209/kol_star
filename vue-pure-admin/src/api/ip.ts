import { http } from "@/utils/http";

export interface IPResourceInput {
  id?: number;
  name: string;
  ipType: string;
  rightsOwner: string;
  contact: string;
  markets: string[];
  audience: string;
  summary: string;
  cooperationStatus: string;
  currency: string;
  priceMin: number | null;
  priceMax: number | null;
  licenseNotes: string;
  profile: Record<string, string>;
}

export interface IPRequestInput {
  id?: number;
  projectName: string;
  department: string;
  markets: string[];
  expectedLaunch: string;
  goal: string;
  description: string;
  budgetCurrency: string;
  budgetMin: number | null;
  budgetMax: number | null;
  externalRecommendation: string;
  candidateIds: number[];
  submit: boolean;
  brief: Record<string, any>;
}

type Result<T> = { code: number; message: string; data: T };
type Page<T> = {
  list: T[];
  total: number;
  pageSize: number;
  currentPage: number;
  stats?: {
    total: number;
    cooperable: number;
    pendingFiles: number;
    recent: number;
  };
};

export const listIPResources = (data: object) =>
  http.request<Result<Page<any>>>("post", "/business/ip/resources/list", {
    data
  });
export const saveIPResource = (data: IPResourceInput) =>
  http.request<Result<{ id: number }>>("post", "/business/ip/resources/save", {
    data
  });
export const getIPResource = (id: number) =>
  http.request<Result<{ resource: any; cases: any[]; files: any[] }>>(
    "get",
    "/business/ip/resources/detail",
    { params: { id } }
  );
export const saveIPCase = (data: {
  ipId: number;
  title: string;
  summary: string;
}) =>
  http.request<Result<{ id: number }>>("post", "/business/ip/resources/case", {
    data
  });
export const uploadIPFile = (
  ipId: number,
  fileKind: string,
  file: File,
  caseId?: number
) => {
  const data = new FormData();
  data.append("ipId", String(ipId));
  data.append("fileKind", fileKind);
  if (caseId) data.append("caseId", String(caseId));
  data.append("file", file);
  return http.request<Result<{ id: number }>>(
    "post",
    "/business/ip/resources/file",
    { data },
    { timeout: 60000, headers: { "Content-Type": false } }
  );
};
export const readIPFile = (id: number) =>
  http.request<Blob>(
    "get",
    "/business/ip/resources/file",
    { params: { id }, responseType: "blob" },
    { beforeResponseCallback: response => response.data } as any
  );
export const deleteIPFile = (id: number) =>
  http.request<Result<null>>("post", "/business/ip/resources/file/delete", {
    data: { id }
  });
export const previewIPImport = (rows: IPResourceInput[]) =>
  http.request<Result<any[]>>("post", "/business/ip/resources/import/preview", {
    data: { rows }
  });
export const importIPResources = (rows: IPResourceInput[]) =>
  http.request<
    Result<{
      imported: number;
      results: any[];
      created: { row: number; id: number }[];
    }>
  >("post", "/business/ip/resources/import", { data: { rows } });

export const listIPRequests = (data: object) =>
  http.request<Result<Page<any>>>("post", "/business/ip/requests/list", {
    data
  });
export const saveIPRequest = (data: IPRequestInput) =>
  http.request<Result<{ id: number; status: string }>>(
    "post",
    "/business/ip/requests/save",
    { data }
  );
export const getIPRequest = (id: number) =>
  http.request<Result<{ request: any; candidates: any[] }>>(
    "get",
    "/business/ip/requests/detail",
    { params: { id } }
  );
export const submitIPFeedback = (data: object) =>
  http.request<Result<null>>("post", "/business/ip/requests/feedback", {
    data
  });
export const submitIPMarketing = (data: object) =>
  http.request<Result<null>>("post", "/business/ip/requests/marketing", {
    data
  });
