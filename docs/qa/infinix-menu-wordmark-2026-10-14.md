# Infinix 菜单字标替换

- 使用已提交的 `vue-pure-admin/src/assets/infinix-wordmark.svg`，与 AI 推荐界面共用字标资产。
- 主导航 `useNav.getLogo()` 已切换为字标；展开/折叠侧栏及横向菜单替换旧图标与旁边标题，链接仍保留系统名称及原首页目标。
- 深色菜单显示白色字标，保持图片比例；展开宽度 114px，54px 折叠菜单内宽度 42px，横向菜单宽度 114px。
- 浏览器挂载实际 SidebarLogo 组件，检查展开/折叠两种宽度：图片加载成功，无越界、变形或控制台错误。验证页使用导航依赖桩，未连接业务服务。
- 截图：`infinix-menu-wordmark-2026-10-14.jpg`。Vue 类型检查、ESLint、Stylelint、生产构建及差异格式检查通过。
- 设计扫描仅发现原有菜单宽度/边距动画警告，本次未新增该类动画。
