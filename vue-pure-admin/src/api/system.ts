import { http } from "@/utils/http";

type Result = {
  code: number;
  message: string;
  data?: Array<any>;
};

type ResultTable = {
  code: number;
  message: string;
  data?: {
    /** 列表数据 */
    list: Array<any>;
    /** 总条目数 */
    total?: number;
    /** 每页显示条目个数 */
    pageSize?: number;
    /** 当前页数 */
    currentPage?: number;
  };
};

/** 获取系统管理-用户管理列表 */
export const getUserList = (data?: object) => {
  return http.request<ResultTable>("post", "/user", { data });
};

export const createUser = (data?: object) => {
  return http.request<Result>("post", "/user/create", { data });
};

export const updateUser = (data?: object) => {
  return http.request<Result>("post", "/user/update", { data });
};

export const deleteUser = (data?: object) => {
  return http.request<Result>("post", "/user/delete", { data });
};

export const updateUserStatus = (data?: object) => {
  return http.request<Result>("post", "/user/status", { data });
};

export const resetUserPassword = (data?: object) => {
  return http.request<Result>("post", "/user/reset-password", { data });
};

export const updateUserRoles = (data?: object) => {
  return http.request<Result>("post", "/user/roles", { data });
};

/** 系统管理-用户管理-获取所有角色列表 */
export const getAllRoleList = () => {
  return http.request<Result>("get", "/list-all-role");
};

/** 系统管理-用户管理-根据userId，获取对应角色id列表（userId：用户id） */
export const getRoleIds = (data?: object) => {
  return http.request<Result>("post", "/list-role-ids", { data });
};

/** 获取系统管理-角色管理列表 */
export const getRoleList = (data?: object) => {
  return http.request<ResultTable>("post", "/role", { data });
};

export const createRole = (data?: object) => {
  return http.request<Result>("post", "/role/create", { data });
};

export const updateRole = (data?: object) => {
  return http.request<Result>("post", "/role/update", { data });
};

export const deleteRole = (data?: object) => {
  return http.request<Result>("post", "/role/delete", { data });
};

export const updateRoleStatus = (data?: object) => {
  return http.request<Result>("post", "/role/status", { data });
};

/** 获取系统管理-菜单管理列表 */
export const getMenuList = (data?: object) => {
  return http.request<Result>("post", "/menu", { data });
};

export const createMenu = (data?: object) => {
  return http.request<Result>("post", "/menu/create", { data });
};

export const updateMenu = (data?: object) => {
  return http.request<Result>("post", "/menu/update", { data });
};

export const deleteMenu = (data?: object) => {
  return http.request<Result>("post", "/menu/delete", { data });
};

/** 获取系统管理-部门管理列表 */
export const getDeptList = (data?: object) => {
  return http.request<Result>("post", "/dept", { data });
};

export const createDept = (data?: object) => {
  return http.request<Result>("post", "/dept/create", { data });
};

export const updateDept = (data?: object) => {
  return http.request<Result>("post", "/dept/update", { data });
};

export const deleteDept = (data?: object) => {
  return http.request<Result>("post", "/dept/delete", { data });
};

/** 获取系统监控-在线用户列表 */
export const getOnlineLogsList = (data?: object) => {
  return http.request<ResultTable>("post", "/online-logs", { data });
};

/** 获取系统监控-登录日志列表 */
export const getLoginLogsList = (data?: object) => {
  return http.request<ResultTable>("post", "/login-logs", { data });
};

/** 获取系统监控-操作日志列表 */
export const getOperationLogsList = (data?: object) => {
  return http.request<ResultTable>("post", "/operation-logs", { data });
};

/** 获取系统监控-系统日志列表 */
export const getSystemLogsList = (data?: object) => {
  return http.request<ResultTable>("post", "/system-logs", { data });
};

/** 获取系统监控-系统日志-根据 id 查日志详情 */
export const getSystemLogsDetail = (data?: object) => {
  return http.request<Result>("post", "/system-logs-detail", { data });
};

/** 获取角色管理-权限-菜单权限 */
export const getRoleMenu = (data?: object) => {
  return http.request<Result>("post", "/role-menu", { data });
};

/** 获取角色管理-权限-菜单权限-根据角色 id 查对应菜单 */
export const getRoleMenuIds = (data?: object) => {
  return http.request<Result>("post", "/role-menu-ids", { data });
};

export const updateRoleMenus = (data?: object) => {
  return http.request<Result>("post", "/role/menus", { data });
};

export const getPlatformSyncControl = () => {
  return http.request<any>("get", "/system/platform-sync-control");
};

export const savePlatformSyncControl = (data?: object) => {
  return http.request<any>("post", "/system/platform-sync-control/save", {
    data
  });
};
