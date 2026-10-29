# 趋势洞察验收记录 · 2026-10-29

结论：本机现有后台已完成菜单迁移、Redis 配置、服务重启和认证后集成验收。资源运营菜单内可见“趋势洞察”，Tag / Video、地区、周期、排序、分页和美国已采集详情正常；Redis 列表及详情均实测 MISS → HIT。实时上游仍有访问限制，不能据此宣称所有地区详情全部可用。此次为本机运行环境验收，未部署远程生产环境。

## 既有视觉体系对照

- 主要参照 `vue-pure-admin/src/views/business/resources/index.vue`：趋势页根节点使用 `business-page`，内容区内边距同为 `18px 14px 14px`，沿用白色内容面。`resource-toolbar` 标题同为 `30px`、字重 `780`、行高 `1.2`，说明文字同为 `14px`、标题下间距 `9px`。页面留白和文字层级对齐现有资源库。
- Tab、Select、Button、Table、Pagination、Dialog、Drawer 继续使用 Element Plus，背景、文本、边框和选中态沿用现有 `--el-*` 变量。Tag 数据表、移动列表、曲线及 Video 媒体卡片是业务局部扩展，未建立替代全局主题。
- 同时参考既有 `resource-posts/index.vue` 的媒体卡片与 `dashboard/index.vue` 的数据表达；dashboard 自身的米白、黑色、荧光绿及字体风格没有被推广为全站规则。
- 已移除“查看 TikTok 原页”等原站按钮和锚点。视频使用系统 Dialog / Drawer 中的 `TrendVideoPlayer` 原生播放器，地址缺失、失效或播放失败时显示“视频暂不可播放”，不提供外跳替代。
- 临时独立预览 HTML / JS 与专用预览服务器已移除。业务入口为资源运营下的动态菜单，继承现有系统布局；没有新增独立验收页面。

既有视觉体系保留；本次未创建 `PRODUCT.md`、`DESIGN.md` 或设计 token。

## 数据及 Redis 契约

默认美国近 30 天采用真实采集的 100 个 Tag、100 份详情及 100 条 Video，本地缩略图随快照提供。其他筛选和“读取最新”读取实际原站接口，界面披露匿名访问限制，不能将有限实时结果解释为完整榜单。具体数据截止日、部署和缓存配置见 `docs/trend-insights.md`。

`backend/cmd/server/main.go` 在 options / list / detail 缓存包装器外执行菜单权限校验。`trend_cache.go` 按接口、Tab、地区、周期、排序、分页、详情 ID、采集/实时来源、语言及时间节点隔离键；仅缓存成功业务 JSON，错误结果不入缓存，缓存不含认证数据。

缓存默认在北京时间每日 00:00 统一到期，配置可调整小时和分钟；以原子 `SET PXAT` 写入绝对节点，命中不续期。“读取最新”绕过旧值并在成功后更新。Redis 不可用时降级返回本次有效结果，通过 `X-Trend-Cache: UNAVAILABLE` 标记，固定错误日志不泄漏上游认证错误正文。

## 已完成验证与边界

以下工程结果由实现任务报告，文档人员未重复运行：

- 加入 Redis 后的后端完整测试套件通过；`go vet` 通过。
- 最新 `TestTrend` 定向测试使用本机隔离 Redis 6397 通过，包括真实 GET / SET PXAT、约 200ms 到期，以及缓存条件隔离、权限、失败路径和新增认证信息脱敏负向用例；最新 `go vet` 亦通过。
- 前端类型检查、包含安全规则的 ESLint、Vite 生产构建通过；构建耗时 22.89 秒。

最终源码扫描未发现趋势组件中的 `window.open`、`href`、`target=_blank` 或原站 CTA，且无独立预览文件残留。已在真实认证后的主应用重新验收，页面无 TikTok 外跳锚点、无视口横向溢出，本地缩略图正常加载。

## 本机实际运行验收

- MySQL 菜单迁移成功：菜单 ID 910、父菜单 900、组件 `business/trends/index`，授予 2 个既有资源库角色；迁移前菜单及授权备份保存在 `outputs/operations/`，未备份凭据。
- 后端 8080、前端 8848 已重启并返回 HTTP 200。实际 Redis 6379 使用默认账号认证，`username` 留空；认证状态与数据库连接均正常。配置文件权限为 0600，凭据未进入验收产物。
- 美国近 30 天 Tag / Video 各显示已采集 100 条；Tag 五页分页可用，标签详情显示曲线、年龄、热门地区及 10 个关联视频。视频详情显示文案、封面、累计/期间播放量、互动率及 6 秒观看率。
- 美国视频样本在主应用内部成功播放，播放器 readyState=4、paused=false，播放时间持续推进；未打开外部页面。
- 日本近 30 天与近 7 天标签返回不同榜单；日本近 7 天 Video 及最高互动率排序返回不同视频。匿名接口列表为有限结果，界面明确提示限制。
- 列表、详情与筛选项响应实测 MISS / HIT；读取最新携带 `refresh=1` 重新查询并写入，后续命中。过期节点均为 `2026-10-30T00:00:00+08:00`，命中不续期。后端重启后详情仍为 HIT。
- 日本实时视频详情样本返回 HTTP 502，界面显示重试提示，响应 `X-Trend-Cache: BYPASS`，失败未缓存。这是当前实时上游边界，不能描述为全量详情成功。
- 本轮后端完整测试、定向趋势测试、`go vet` 通过；隔离 Redis 的默认 ACL 账号认证读写用例通过，测试实例已停止。

真实主应用证据位于 `.impeccable/review/`：`trends-integrated-video.png`、`trends-integrated-video-detail.png`、`trends-integrated-tag-detail.png`。`trends-runtime-cache-checks.json` 仅保存趋势接口路径、筛选参数、状态码及缓存响应头，不含请求认证头。

## 历史截图

`.impeccable/review/` 中以下截图曾逐张检查，但来自修正前的独立预览，**仅作历史交互证据，不是当前集成界面的最终证明**：

| 文件 | 尺寸 | 历史范围 |
| --- | --- | --- |
| `desktop.png` | 1280×800 | Tag 筛选、榜单、曲线及头像 |
| `video-desktop.png` | 1280×800 | Video 排序及封面卡片 |
| `tag-detail.png` | 1280×800 | Tag 趋势、年龄分布与地区指数 |
| `mobile.png` | 390×844 | Video 移动布局 |
| `tag-mobile.png` | 390×844 | Tag 移动列表 |

此前浏览器验证包括 Tag 详情、本地图片、美国完整分页、互动率排序实时四条记录和日本近 7 天数据变化，截图未见视口横向溢出；Detector 当次结果为 `[]`。这些历史结果不覆盖后续布局、内嵌播放和 Redis 更新，也不构成完整可访问性认证。

本机已执行 `backend/migrations/058_trend_insights.sql`、配置 Redis，并通过既有动态菜单验收。远程环境如需部署，仍需按部署文档单独执行迁移并复制采集数据和缩略图。

源码评审核对 `trends/index.vue`、`trends/TrendVideoPlayer.vue`、资源库页面、`backend/cmd/server/trend_cache.go`、路由注册及 `docs/trend-insights.md`；此前直接检查 `trends/TrendChart.vue`、`trends/TrendImage.vue` 与 `vue-pure-admin/src/api/trends.ts`。前端短路径均相对 `vue-pure-admin/src/views/business/`。
