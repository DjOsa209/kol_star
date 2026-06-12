// 模拟后端动态生成路由
import { defineFakeRoute } from "vite-plugin-fake-server/client";
import { system, monitor } from "@/router/enums";

/**
 * roles：页面级别权限，这里模拟二种 "admin"、"common"
 * admin：管理员角色
 * common：普通角色
 */

const systemManagementRouter = {
  path: "/system",
  meta: {
    icon: "ri:settings-3-line",
    title: "menus.pureSysManagement",
    rank: system
  },
  children: [
    {
      path: "/system/user/index",
      name: "SystemUser",
      meta: {
        icon: "ri:admin-line",
        title: "menus.pureUser",
        roles: ["admin"]
      }
    },
    {
      path: "/system/role/index",
      name: "SystemRole",
      meta: {
        icon: "ri:admin-fill",
        title: "menus.pureRole",
        roles: ["admin"]
      }
    },
    {
      path: "/system/menu/index",
      name: "SystemMenu",
      meta: {
        icon: "ep:menu",
        title: "menus.pureSystemMenu",
        roles: ["admin"]
      }
    },
    {
      path: "/system/dept/index",
      name: "SystemDept",
      meta: {
        icon: "ri:git-branch-line",
        title: "menus.pureDept",
        roles: ["admin"]
      }
    }
  ]
};

const businessRouter = {
  path: "/business",
  meta: {
    icon: "ri:global-line",
    title: "资源运营",
    rank: 2
  },
  children: [
    {
      path: "/business/assistant",
      component: "business/assistant/index",
      name: "BusinessAssistant",
      meta: {
        icon: "ri:chat-search-line",
        title: "智能资源助手",
        roles: ["admin"]
      }
    },
    {
      path: "/business/resources",
      component: "business/resources/index",
      name: "BusinessResources",
      meta: {
        icon: "ri:contacts-book-3-line",
        title: "全球资源库",
        roles: ["admin"]
      }
    },
    {
      path: "/business/tags",
      component: "business/tags/index",
      name: "BusinessTags",
      meta: {
        icon: "ri:price-tag-3-line",
        title: "标签体系",
        roles: ["admin"]
      }
    },
    {
      path: "/business/projects",
      component: "business/projects/index",
      name: "BusinessProjects",
      meta: {
        icon: "ri:briefcase-4-line",
        title: "项目合作",
        roles: ["admin"]
      }
    },
    {
      path: "/business/briefs",
      component: "business/briefs/index",
      name: "BusinessBriefs",
      meta: {
        icon: "ri:file-list-3-line",
        title: "Brief模板库",
        roles: ["admin"]
      }
    },
    {
      path: "/business/dashboard",
      component: "business/dashboard/index",
      name: "BusinessDashboard",
      meta: {
        icon: "ri:bar-chart-box-line",
        title: "数据看板",
        roles: ["admin"]
      }
    },
    {
      path: "/business/governance",
      component: "business/governance/index",
      name: "BusinessGovernance",
      meta: {
        icon: "ri:settings-4-line",
        title: "治理规则",
        roles: ["admin"]
      }
    }
  ]
};

const systemMonitorRouter = {
  path: "/monitor",
  meta: {
    icon: "ep:monitor",
    title: "menus.pureSysMonitor",
    rank: monitor
  },
  children: [
    {
      path: "/monitor/online-user",
      component: "monitor/online/index",
      name: "OnlineUser",
      meta: {
        icon: "ri:user-voice-line",
        title: "menus.pureOnlineUser",
        roles: ["admin"]
      }
    },
    {
      path: "/monitor/login-logs",
      component: "monitor/logs/login/index",
      name: "LoginLog",
      meta: {
        icon: "ri:window-line",
        title: "menus.pureLoginLog",
        roles: ["admin"]
      }
    },
    {
      path: "/monitor/operation-logs",
      component: "monitor/logs/operation/index",
      name: "OperationLog",
      meta: {
        icon: "ri:history-fill",
        title: "menus.pureOperationLog",
        roles: ["admin"]
      }
    },
    {
      path: "/monitor/system-logs",
      component: "monitor/logs/system/index",
      name: "SystemLog",
      meta: {
        icon: "ri:file-search-line",
        title: "menus.pureSystemLog",
        roles: ["admin"]
      }
    }
  ]
};

export default defineFakeRoute([
  {
    url: "/get-async-routes",
    method: "get",
    response: () => {
      return {
        code: 0,
        message: "操作成功",
        data: [businessRouter, systemManagementRouter, systemMonitorRouter]
      };
    }
  }
]);
