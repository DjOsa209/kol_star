import { http } from "@/utils/http";
import type { AxiosProgressEvent } from "axios";

export interface IPResourceInput {
  id?: number;
  name: string;
  ipType: string;
  rightsOwner: string;
  contact: string;
  markets: string[];
  marketPenetrations?: Record<string, number>;
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
export const IP_DOCUMENT_TIMEOUT_MS = 180000;
type IPFileReadOptions = {
  timeout?: number;
  onDownloadProgress?: (progress: AxiosProgressEvent) => void;
};
export const readIPFile = (id: number, options: IPFileReadOptions = {}) =>
  http.request<Blob>(
    "get",
    "/business/ip/resources/file",
    { params: { id }, responseType: "blob" },
    {
      ...options,
      beforeResponseCallback: response => response.data
    } as any
  );
export const deleteIPFile = (id: number) =>
  http.request<Result<null>>("post", "/business/ip/resources/file/delete", {
    data: { id }
  });
export const renameIPFile = (id: number, originalName: string) =>
  http.request<Result<null>>("post", "/business/ip/resources/file/rename", {
    data: { id, originalName }
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
export const deleteIPRequest = (id: number) =>
  http.request<Result<null>>("post", "/business/ip/requests/delete", {
    data: { id }
  });
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
  http.request<
    Result<{ status: string; functionKey: string; version: number }>
  >("post", "/business/ip/requests/marketing", {
    data
  });

export type IPColleague = {
  id: string | number;
  name: string;
  department: string;
  employeeNo?: string;
  notifiable?: boolean;
};

export const searchIPColleagues = (q: string, page = 1) =>
  http.request<
    Result<IPColleague[]> & {
      hasNext?: boolean;
      directorySource?: "uac" | "cache";
      directoryWarning?: string;
    }
  >("get", "/business/ip/colleagues", { params: { q, page } });

export interface IPStageDeadline {
  label: string;
  workingDays: number;
  startedAt?: string;
  dueAt?: string;
  completedAt?: string;
  state:
    | "waiting"
    | "unknown_start"
    | "calendar_pending"
    | "pending"
    | "due_today"
    | "overdue"
    | "completed";
  late: boolean;
  remainingWorkdays: number;
  overdueWorkdays: number;
  warning?: string;
}
