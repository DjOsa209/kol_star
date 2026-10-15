package main

import "strings"

// Only state-changing, user-authenticated routes are operation-log entries.
// Request tracing still records other POSTs in sys_system_logs.
var operationActions = map[string]string{
	"/mine/password":                                 "修改个人密码",
	"/upload/images":                                 "上传图片",
	"/user/create":                                   "新增用户",
	"/user/update":                                   "更新用户",
	"/user/delete":                                   "删除用户",
	"/user/status":                                   "变更用户状态",
	"/user/reset-password":                           "重置用户密码",
	"/user/roles":                                    "分配用户角色",
	"/role/create":                                   "新增角色",
	"/role/update":                                   "更新角色",
	"/role/delete":                                   "删除角色",
	"/role/status":                                   "变更角色状态",
	"/role/menus":                                    "配置角色菜单",
	"/menu/create":                                   "新增菜单",
	"/menu/update":                                   "更新菜单",
	"/menu/delete":                                   "删除菜单",
	"/dept/create":                                   "新增部门",
	"/dept/update":                                   "更新部门",
	"/dept/delete":                                   "删除部门",
	"/online-logs/offline":                           "强制会话下线",
	"/system/platform-sync-control/save":             "更新平台同步控制",
	"/system/standard-import-options/create":         "新增标准导入选项",
	"/system/standard-import-options/update":         "更新标准导入选项",
	"/system/standard-import-options/delete":         "删除标准导入选项",
	"/business/discovery/candidates/refresh":         "刷新产品合作候选",
	"/business/discovery/candidates/decision":        "确认产品合作候选",
	"/business/ip/resources/save":                    "保存 IP 资源",
	"/business/ip/resources/case":                    "保存 IP 合作案例",
	"/business/ip/resources/file":                    "上传 IP 文件",
	"/business/ip/resources/file/rename":             "重命名 IP 文件",
	"/business/ip/resources/file/delete":             "删除 IP 文件",
	"/business/ip/resources/import":                  "批量导入 IP 资源",
	"/business/ip/requests/save":                     "保存 IP 需求",
	"/business/ip/requests/delete":                   "删除 IP 需求",
	"/business/ip/requests/feedback":                 "提交 IP 反馈",
	"/business/ip/requests/marketing":                "补充营销意见",
	"/business/resources/create":                     "新增资源",
	"/business/resources/update":                     "更新资源",
	"/business/resources/delete":                     "删除资源",
	"/business/resources/sync":                       "同步资源",
	"/business/resources/traffic-cv-html":            "导入资源履历",
	"/business/resources/sync-all":                   "批量同步资源",
	"/business/resources/import":                     "批量导入资源",
	"/business/resources/translate":                  "翻译资源资料",
	"/business/recommendations/subscriptions/save":   "保存推荐条件",
	"/business/recommendations/subscriptions/delete": "删除推荐条件",
	"/business/recommendations/run":                  "运行推荐",
	"/business/recommendations/results/feedback":     "提交推荐反馈",
	"/business/recommendations/notifications/read":   "标记推荐通知已读",
	"/business/project-resources/create":             "新增项目资源",
	"/business/project-resources/update":             "更新项目资源",
	"/business/project-resources/delete":             "删除项目资源",
	"/business/markets/create":                       "新增市场",
	"/business/markets/delete":                       "删除市场",
	"/business/tags/create":                          "新增标签",
	"/business/projects/content/sync-all":            "同步项目内容",
	"/business/projects/content/update":              "更新项目内容",
	"/business/projects/content/delete":              "删除项目内容",
	"/business/projects/status":                      "变更项目状态",
	"/business/projects/renew":                       "续签项目",
	"/business/projects/budget":                      "更新项目预算",
	"/business/projects/influencer-report":           "生成项目达人报告",
	"/business/projects/create":                      "新增项目",
	"/business/projects/import":                      "批量导入项目",
	"/business/projects/update":                      "更新项目",
	"/business/projects/delete":                      "删除项目",
	"/business/cooperations/create":                  "新增合作",
	"/business/cooperations/update":                  "更新合作",
	"/business/cooperations/sync":                    "同步合作",
	"/business/cooperations/import":                  "批量导入合作",
	"/business/brief-templates/create":               "新增 Brief 模板",
	"/business/ai-model/save":                        "更新 AI 模型配置",
	"/business/governance/save":                      "更新治理规则",
}

func operationAction(path string) string {
	return operationActions[canonicalAPIPath(path)]
}

func operationModule(path string) string {
	path = canonicalAPIPath(path)
	switch {
	case strings.HasPrefix(path, "/business/ip/"):
		return "IP 运营"
	case strings.HasPrefix(path, "/business/projects/") || strings.HasPrefix(path, "/business/cooperations/"):
		return "项目管理"
	case strings.HasPrefix(path, "/business/recommendations/"):
		return "智能推荐"
	case strings.HasPrefix(path, "/business/"):
		return "资源运营"
	case strings.HasPrefix(path, "/online-logs/"):
		return "在线用户"
	case strings.HasPrefix(path, "/mine/"):
		return "账户设置"
	case strings.HasPrefix(path, "/upload/"):
		return "文件上传"
	default:
		return "系统管理"
	}
}

func canonicalAPIPath(path string) string {
	if path == "/api" {
		return "/"
	}
	if strings.HasPrefix(path, "/api/") {
		return strings.TrimPrefix(path, "/api")
	}
	return path
}
