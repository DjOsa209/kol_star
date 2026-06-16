import { http } from "@/utils/http";

type Result<T = any> = {
  code: number;
  message: string;
  data: T;
};

type ResultTable<T = any> = Result<{
  list: T[];
  total: number;
  pageSize: number;
  currentPage: number;
}>;

export const getResourceList = (data?: object) => {
  return http.request<ResultTable>("post", "/business/resources", { data });
};

export const createResource = (data?: object) => {
  return http.request<Result>("post", "/business/resources/create", { data });
};

export const updateResource = (data?: object) => {
  return http.request<Result>("post", "/business/resources/update", { data });
};

export const deleteResource = (data?: object) => {
  return http.request<Result>("post", "/business/resources/delete", { data });
};

export const syncResource = (data?: object) => {
  return http.request<Result>("post", "/business/resources/sync", { data });
};

export const syncAllResources = (data?: { platforms?: string[] }) => {
  return http.request<Result>("post", "/business/resources/sync-all", { data });
};

export const getResourceSyncStatus = () => {
  return http.request<Result>("get", "/business/resources/sync-status");
};

export const getResourceExtraFields = () => {
  return http.request<Result<any[]>>("get", "/business/resources/extra-fields");
};

export const getResourcePosts = (data?: object) => {
  return http.request<Result>("post", "/business/resource-posts", { data });
};

export const importResources = (data?: object) => {
  return http.request<Result>("post", "/business/resources/import", { data });
};

export const recommendResources = (data?: object) => {
  return http.request<Result>(
    "post",
    "/business/assistant/recommend",
    { data },
    { timeout: 130000 }
  );
};

export const addProjectResource = (data?: object) => {
  return http.request<Result>("post", "/business/project-resources/create", {
    data
  });
};

export const getMarketOptions = () => {
  return http.request<Result<any[]>>("get", "/business/markets");
};

export const createMarketOption = (data?: object) => {
  return http.request<Result>("post", "/business/markets/create", { data });
};

export const deleteMarketOption = (data?: object) => {
  return http.request<Result>("post", "/business/markets/delete", { data });
};

export const getTagList = () => {
  return http.request<Result<any[]>>("get", "/business/tags");
};

export const createTag = (data?: object) => {
  return http.request<Result>("post", "/business/tags/create", { data });
};

export const getProjectList = (data?: object) => {
  return http.request<ResultTable>("post", "/business/projects", { data });
};

export const createProject = (data?: object) => {
  return http.request<Result>("post", "/business/projects/create", { data });
};

export const updateProject = (data?: object) => {
  return http.request<Result>("post", "/business/projects/update", { data });
};

export const getProjectDetail = (params?: object) => {
  return http.request<Result>("get", "/business/projects/detail", { params });
};

export const updateProjectStatus = (data?: object) => {
  return http.request<Result>("post", "/business/projects/status", { data });
};

export const renewProject = (data?: object) => {
  return http.request<Result>("post", "/business/projects/renew", { data });
};

export const updateProjectBudget = (data?: object) => {
  return http.request<Result>("post", "/business/projects/budget", { data });
};

export const reportProjectInfluencer = (data?: object) => {
  return http.request<Result>("post", "/business/projects/influencer-report", {
    data
  });
};

export const downloadProjectReport = (params?: object) => {
  return http.request<Blob>(
    "get",
    "/business/projects/report/download",
    { params, responseType: "blob" },
    {
      beforeResponseCallback: response => response.data
    } as any
  );
};

export const getCooperationList = (data?: object) => {
  return http.request<ResultTable>("post", "/business/cooperations", { data });
};

export const createCooperation = (data?: object) => {
  return http.request<Result>(
    "post",
    "/business/cooperations/create",
    { data },
    { timeout: 30000 }
  );
};

export const updateCooperation = (data?: object) => {
  return http.request<Result>(
    "post",
    "/business/cooperations/update",
    { data },
    { timeout: 30000 }
  );
};

export const syncCooperation = (data?: object) => {
  return http.request<Result>(
    "post",
    "/business/cooperations/sync",
    { data },
    { timeout: 30000 }
  );
};

export const importCooperations = (data?: object) => {
  return http.request<Result>("post", "/business/cooperations/import", {
    data
  });
};

export const getBriefTemplateList = (data?: object) => {
  return http.request<ResultTable>("post", "/business/brief-templates", {
    data
  });
};

export const createBriefTemplate = (data?: object) => {
  return http.request<Result>("post", "/business/brief-templates/create", {
    data
  });
};

export const getBusinessDashboard = (params?: object) => {
  return http.request<Result>("get", "/business/dashboard", { params });
};

export const getGovernanceRules = () => {
  return http.request<Result<any[]>>("get", "/business/governance");
};

export const saveGovernanceRule = (data?: object) => {
  return http.request<Result>("post", "/business/governance/save", {
    data
  });
};

export const getAIModelConfig = () => {
  return http.request<Result>("get", "/business/ai-model");
};

export const saveAIModelConfig = (data?: object) => {
  return http.request<Result>("post", "/business/ai-model/save", {
    data
  });
};

export const testAIModelConfig = (data?: object) => {
  return http.request<Result>("post", "/business/ai-model/test", {
    data
  });
};
