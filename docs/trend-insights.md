# 趋势洞察

入口：**资源运营 → 趋势洞察**，路由 `/business/trends`。菜单组件为 `business/trends/index`。

Tag 与 Video 分别对应 TikTok Creative Center 的 Hashtag、Video 页。地区清单采用已核对的原页范围；Tag 支持近 7/30/90 天，Video 支持近 7/30 天。Video 提供播放量、互动率、6 秒观看率排序。筛选值保存在 URL 中，可回退、刷新与分享。

## 数据与交互

- 默认美国、近 30 天，使用本次完整采集的 100 个标签、100 份标签详情和 100 条播放量榜单视频。Tag 展示发帖量、播放量、趋势和代表创作者，进入分析抽屉可查看趋势、年龄、地区指数和关联视频。Video 提供封面、文案、作者、粉丝、排序指标及详情。
- 切换其他地区、时间、视频排序，或点击“读取最新”，由后端读取对应的原站接口。不会把美国快照改标签后当成其他地区数据。
- 匿名原站访问受限：实测 Tag 可能只有 3 条、Video 只有 4 条；原站还可能忽略请求页码。页面明确标注当前可访问范围，检查实际返回页码，拒绝把重复首页当成新页。
- 美国 30 天完整采集数据会显示采集时间和数据截止日。Tag 曲线截至 **2026-10-15**；Video 使用接口提供的周期末时间。个别标签不足 30 个趋势点，按原样展示。
- Tag 详情关联视频为原站精选样本，不等于标签下全部历史视频。前端硬编码的相关兴趣占位值不作为业务数据展示。

原站接口：

| 用途 | 方法及路径 |
| --- | --- |
| 标签榜单 | `POST https://ads.tiktok.com/CreativeOne/KnowledgeAPI/GetHashtagList` |
| 标签详情 | `POST https://ads.tiktok.com/CreativeOne/KnowledgeAPI/GetHashtagDetail` |
| 视频周期末日 | `GET /CreativeOne/Report/GetTopContentsOverview` |
| 视频榜单 | `GET /CreativeOne/Report/CreativeCenterGetTopContentsList` |
| 视频详情 | `GET /CreativeOne/Report/CreativeCenterGetTopContentsItemDetail` |

视频接口按地区选择原页对应的固定 TikTok 域名。应用登录凭据不会发送到 TikTok。启用 `trend_browser` 后，“读取最新”使用下文的独立浏览器采集流程；Ads Cookie 仅由专用 Chrome 管理，不导出到应用或 Redis。未启用时保留旧的匿名读取接口，但“读取最新”按钮会明确提示管理员配置采集浏览器。

## 部署

1. 执行 `backend/migrations/058_trend_insights.sql`。新库的 `all.sql` 已包含相同迁移。新菜单默认授予已有全球资源库权限的角色，之后可在角色管理中调整。
2. 构建更新后的前后端。后端工作目录为 `backend` 时，会从 `data/trends` 读取快照和本地缩略图；部署时一起复制整个目录。也可通过 `TREND_DATA_DIR` 指定该目录的绝对路径。
3. 重启后端并重新登录或刷新动态菜单。无需新增依赖。

本机于 2026-10-29 已完成以上步骤，实际入口为现有后台 `/business/trends`。验收覆盖筛选、已采集详情、内部播放及真实 Redis 命中；日本实时视频详情样本被上游拒绝，具体边界见 `docs/qa/trend-insights-20261029.md`。此记录不代表远程生产环境已部署。

维护命令在 `backend` 工作目录执行：`go run ./cmd/server trends-status` 只报告连接、菜单和缓存状态，不输出凭据；`go run ./cmd/server trends-migrate` 检查目标数据库及菜单冲突，先备份相关菜单/授权，再以事务执行迁移。备份目录为 `outputs/operations/`。默认 Redis 账号可将 `username` 留空，仅配置密码；命名 ACL 账号才需要用户名。

快照位于 `backend/data/trends/snapshot.json`。`media/` 包含 1,185 张由已校验原图生成的 JPEG 缩略图，限制长边并移除 EXIF；原始导出保持不变。这样已采集页面不依赖过期的 CDN 签名。

所有 `/business/trends/*` 接口，包括图片接口，都在服务端校验登录和 `/business/trends` 菜单权限。前端通过现有认证客户端读取本地图片 Blob，不在 URL 中放访问令牌。远程媒体只接受明确的 HTTPS TikTok CDN 域名，媒体文件名采用固定白名单，原站请求有超时、体积限制和重定向限制。

## 验证

`go test ./cmd/server -run '^TestTrend'` 覆盖五页唯一 ID、快照隔离、详情完整性、非法筛选、无登录/无权限、媒体路径与符号链接、上游失败/重定向、大整数 ID 精度。测试不连接生产数据库。

不提供独立入口或独立验收页面。业务组件只由系统动态菜单挂载，继承现有后台侧栏、顶部导航和标签页布局。界面沿用现有标题、Element Plus 控件与抽屉，具体排版见下方页面更新说明。没有“查看 TikTok 原页”或关联视频外跳入口，视频在系统内预览；已过期或不可用的视频地址显示不可播放，不用外跳代替。临时验收页及专用预览服务器已移除。

## 页面排版更新（2026-10-29）

趋势页按已选设计整合：Video 使用四列媒体网格、4:5 封面、排名、作者信息及实际接口提供的三项指标；Tag 在宽屏采用左侧榜单、右侧标签分析，默认展示当前页首个标签，点击后更新曲线、受众和关联视频。小屏继续使用分析抽屉，视频网格调整为两列。保留现有过滤、分页、来源与数据截止日期披露及系统内播放。

页面沿用黑色导航、米白底色与橄榄绿选中态；仅移除趋势洞察内容区的网格背景。没有增加依赖。设计依据与截图位于 `docs/design/trend-insights/`，视觉和交互验收见项目根目录 `design-qa.md`。这次 UI 调整不代表远程环境已经更新。

## Redis 热点查询缓存

列表、详情和筛选项查询先执行系统登录及菜单权限校验，再读 Redis。只有 HTTP 200 且业务 code=0 的成功 JSON 结果会写入；接口错误不缓存。图片继续使用本地缩略图，Redis 缓存图片地址及业务结果，不重复存放图片字节。

缓存键隔离接口类型、Tag/Video、地区、时间范围、排序、页码、页大小、详情 ID、采集/实时来源、语言和时间节点。命中时不延长 TTL；“读取最新”绕过旧值，成功后覆盖对应缓存。不同地区或周期不会复用其他筛选结果。

默认按照北京时间每天 00:00 的节点过期，通过 Redis `SET ... PXAT` 原子写入绝对过期时间，不使用滑动过期。可配置日内更新时间；键的日期分桶与过期节点一致，旧键不会跨节点复用。Redis 需 6.2 或以上版本，参见 [SET 官方文档](https://redis.io/docs/latest/commands/set/)。

在运行配置中加入以下项，或使用对应环境变量覆盖，例如 `REDIS_ADDR`、`REDIS_PASSWORD`。示例不含实际凭据：

```yaml
redis:
  addr: 127.0.0.1:6379
  username: ""
  password: ""
  database: 0
  tls: false
  key_prefix: "kol:trends:v1:"
  node_hour: 0
  node_minute: 0
```

非回环 Redis 必须启用 TLS，使用系统 CA 和证书校验。仅实现本模块需要的 RESP2 `GET`、`SET PXAT` 及可选认证、数据库选择，不新增依赖。读写都有 800ms 超时及 8MiB 响应限制，不记录认证错误正文或密码。Redis 不可用时仍返回本次有效数据，记录固定错误信息，并通过响应头 `X-Trend-Cache: UNAVAILABLE` 明确标记，不声称已缓存。

`X-Trend-Cache` 的其他值为 `HIT`、`MISS`、`BYPASS`；`X-Trend-Cache-Expires-At` 返回绝对过期节点。缓存仅含查询结果，不含 Chrome Cookie 或系统访问令牌。

测试额外覆盖命中、条件隔离、强制刷新、非滑动过期、午夜边界、缓存不可用、过期值拒绝、失败结果不入缓存、RESP 格式/大小限制，以及权限校验早于缓存。设置 `TREND_REDIS_TEST_ADDR` 可在隔离 Redis 实例运行真实写入、读取及 PXAT 到期测试；不执行 FLUSHDB。

## REA 连接修复（2026-10-29 实测）

当前 Chrome 156 的 `http://127.0.0.1:9222/json/version` 返回 404，但 `ws://127.0.0.1:9222/devtools/browser` 握手成功。REA 6.7.0 对显式 WebSocket 输入要求 `/devtools/browser/ID`，会在连接前拒绝 Chrome 当前的无 ID 地址。这是端点兼容问题，不能将 404 等同于 Chrome 未开放调试。

`scripts/rea-cdp-bridge.cjs` 提供只绑定 `127.0.0.1:9223` 的兼容入口：`ws://127.0.0.1:9223/devtools/browser/rea-ads`。它复用已有 Chrome，不复制配置文件或登录凭据。使用 REA 已安装的 `ws` 模块，通过 `REA_WS_MODULE` 指定该模块的绝对路径；无需安装新依赖。运行 `node scripts/rea-cdp-bridge.cjs`，用相同环境变量执行 `node scripts/rea-cdp-bridge-check.cjs` 检查连接与拒绝路径。桥进程退出后需重新启动，未配置系统自启动。

桥仅允许 Ads 页面发现、附加及有限的只读观察命令；禁止 Cookie/Storage 读取、任意脚本执行、导航和关闭浏览器。网络响应正文读取仅限本连接已观察到的指定趋势业务接口，请求认证头、请求体和控制台信息不转发。该桥是本机诊断工具，不是面向用户的采集服务；不要向公网开放端口。

已通过真实 REA `list_browser_targets`、榜单及详情 `inspect_web_page`。详情读取返回 1,148 个 DOM 节点及 540 个辅助树节点。REA 证据 ID：`ev_017f63390b824ff8fd0ad7039b88ac614446a1ce45500a9ada9113ed17c13fd0`、`ev_31c3b81ee4de32b6745023f045c5ab20a2ecab25e85b45852a9845a61636bff6`、`ev_05b4521d4d2ed4e63394a73f1c9eb6e16ba7fdbe5f8e7308b4ef65a3940d39a5`。这是连接与页面读取成功的证据，不代表已完成全部认证接口采集。

参考：[Chrome DevTools WebSocket 连接配置](https://github.com/ChromeDevTools/chrome-devtools-mcp/blob/main/docs/configuration.md)。

## 独立采集浏览器与按需登录

已实现到现有后台“读取最新”流程，不另建业务页面。当前按用户要求在本机运行；Linux 提供同一实现及服务模板，未操作堡垒机或远程服务器。

### 运行方式

后端复用已有 `chromedp v0.13.7`，按后台用户创建独立 Chrome 进程及持久化用户目录。无需新增应用依赖，没有公开 CDP 或远程桌面端口。后台进程通过本机 Chrome 调试通道控制浏览器；客户端通过原有后台登录及菜单权限访问有限的画面/鼠标/键盘接口。

本机从仓库根目录运行 `scripts/start-trend-browser-backend.sh`；先停止同端口的旧后端。可通过 `TREND_BACKEND_BINARY` 指定已经构建的后端绝对路径，否则执行现有 Go 启动方式。脚本不安装软件。

配置项：`trend_browser.enabled / executable / profile_root / max_sessions`，对应环境变量 `TREND_BROWSER_ENABLED / TREND_BROWSER_EXECUTABLE / TREND_BROWSER_PROFILE_ROOT / TREND_BROWSER_MAX_SESSIONS`。默认关闭，最大并发默认 2 个用户浏览器，硬限制 8。仅设置配置不会安装 Chrome。

本地启动脚本把专用目录放在 Git 忽略的 `.runtime/trend-browser-profiles`；不复制日常 Chrome 配置。目录权限必须为 0700，子目录按用户数字 ID 隔离。不要查看、打包、备份到代码仓库或手动转发目录中的会话文件。

Linux 使用已审批安装的 Chrome、Xvfb、xauth，以普通服务用户运行。无 DISPLAY 时启动脚本用 `xvfb-run` 创建带授权的虚拟显示，关闭 X11 TCP 监听。保留 Chrome sandbox 和 TLS 验证，不提供绕过 Google 验证的参数。`deploy/trend-browser/kol-admin-trend-browser.service.example` 为 systemd 模板；部署时核对用户和路径，通过当前 HTTPS 反向代理提供后台。控制接口只接受 TLS 连接或本机回环连接，不信任客户端伪造的 X-Forwarded-Proto。

### 查询与登录

1. 用户点击“读取最新”，创建当前 Tag/Video、地区、周期、排序的异步任务。一个用户同一时间只有一个任务；重复点击同筛选返回原任务，不同筛选提示先完成或取消。
2. 浏览器打开对应 Ads 页面，以正常浏览器请求读取固定榜单接口。明确 HTTP 401/403，或观察到匿名榜单只有 3/4 条时，任务进入 `needs_login`。未知错误、限流、网络异常不会被伪装成登录成功。
3. 趋势页对话框展示该用户采集浏览器的实时 JPEG 画面。用户在 Ads 原页面选择 Google 登录并处理必要验证。画面支持点击、滚动、键盘、粘贴和登录弹窗自动切换；仅显示允许的 Ads/Google 登录来源。
4. 等待期间每 8 秒检测一次只读业务接口。获得超过匿名范围的结果后自动恢复整次采集，不跳转或刷新正在操作的 Google 登录页。
5. 校验 5 页、100 个唯一 ID 和 100 份对应详情，全部成功后一次 Redis SET PXAT 发布数据集。UI 自动关闭进度对话框并重新读缓存。详情或分页失败、用户取消、超时、Redis 写入失败均不会发布不完整结果。

状态包括 starting、collecting、needs_login、complete、failed、cancelled。任务限时 30 分钟；关闭对话框可继续任务，按钮改为“查看采集进度”；离开页面会请求取消，服务端超时为兜底。已结束且闲置 30 分钟的浏览器会关闭，持久化目录保留用于下次登录复用。后端重启会中断内存任务，用户需重新点击；已经发布的 Redis 结果和浏览器配置仍在。

### 缓存与边界

浏览器实时结果按后台用户、Tag/Video、地区、周期、排序和时间节点隔离。整个榜单及详情使用一个原子数据集，列表分页、详情从相同数据集返回；不采用旧的逐页实时缓存，避免刷新后新旧页面混用。过期仍遵循现有北京时间配置节点，命中不续期。完整采集成功但 Redis 失败时任务明确失败，不声称数据已更新。

原有本地历史快照继续可读。“读取最新”不会把匿名 3 条覆盖成完整榜单，也不会将旧快照冒充新数据。当前以目标榜单 100 条作为发布门槛；若某筛选原站实际不足 100 条，会保留旧结果并报未完整完成，需后续根据原站可验证的总量契约调整。

媒体使用原站业务结果中的 URL，页面既有媒体域名白名单继续生效。本次不新增全量图片下载；原有本地快照图片保持不变。跨域视频接口须遵守浏览器原站 CORS，不能通过导出 Cookie 绕过；若上游拒绝，任务安全失败。

远程画面只以 no-store 响应返回给当前用户；输入端点在现有请求日志采集前整段省略，不持久化文字或画面。接口不接收 URL、JS、CDP 指令或 Cookie。业务响应递归白名单过滤后才写 Redis。密码由用户操作登录页输入，应用不建立 Google 密码库，亦不自动代填密码。

### 验证范围

新增测试覆盖会话失效后的自动恢复、100 条和详情原子发布、跨用户数据隔离、任务归属、取消/失败保留旧缓存、过期、输入日志屏蔽、非法命令/来源拒绝、会话容量及传输要求。可使用 `TREND_BROWSER_TEST_EXECUTABLE` 显式指定已安装 Chrome 运行隔离的合成页面启动测试，不访问个人浏览器或真实登录凭据。

真实 Google 登录及原站完整采集验收需要用户在新采集浏览器中完成登录；状态机测试中的自动恢复为模拟上游认证结果，不等于已经验证 Google 接受该运行环境。

### 独立启动模式的历史验证与阻塞

本机已实际从现有后台点击“读取最新”，独立 Chrome 启动成功；匿名结果进入登录等待，后台远程画面可点击 Ads 的 Log in 和 Log in with Google，并切换到 accounts.google.com。最初网络连接关闭，随后用户在 Google 登录时收到“此浏览器或应用可能不安全”的拒绝。

Google 官方说明自动化控制的浏览器可能被拒绝登录：[支持的浏览器](https://support.google.com/accounts/answer/7675428?hl=en)。因此本次 **没有完成真实 Google 登录后的 100 条榜单及详情采集验收**，不得把模拟上游状态的集成测试称为真实 Google SSO 成功。用户已明确不采用浏览器扩展替代方案；没有新增或修改扩展权限，也没有通过隐去自动化标记、关闭安全保护、复制凭据来绕过 Google 限制。若账号已有 Ads 支持的邮箱/手机登录方式，可单独验证，但本次未代用户切换认证方式。

## 当前主方案：本地程序采集上传

当前“读取最新”默认创建本地采集授权，用户下载采集程序与任务文件，在自己的 Chrome 登录并采集。服务器接收完整业务数据，自行下载图片、保存归档并更新 Redis。服务器浏览器及远程桌面入口由新启动脚本明确关闭。部署与使用见 [本地采集上传说明](../deploy/trend-browser/LOCAL-UPLOAD.md)。

授权绑定后台用户和筛选条件，30 分钟有效；用户任务文件只含上传授权，不含 Google / Ads 登录凭据。新的上传接收接口使用该授权，不接收或导出浏览器 Cookie。图片下载限制 CDN、公网地址、类型、大小与并发，失败数量在后台显示。Redis 过期规则保持不变。

## 已验证的基础能力：普通 Chrome 登录后连接

Linux 服务器接入文件与操作说明见 [服务器部署说明](../deploy/trend-browser/README.md)。`desktop_enabled` 开启后，登录等待或失败状态提供“打开服务器浏览器”，先断开采集控制再打开独立认证的远程桌面；尚未完成网关和系统依赖配置时保持关闭。目标服务器的 Google 登录由部署方实测。

Chrome 144+ 官方支持在 `chrome://inspect/#remote-debugging` 启用并逐次授权连接已打开的浏览器：[Chrome 官方说明](https://developer.chrome.com/blog/chrome-devtools-mcp-debug-your-browser-session)。用户先在普通 Chrome 完成 Ads 的 Google 登录，后台随后通过 CDP 新建专用采集标签页，复用同一浏览器上下文。无需扩展，不复制 profile、不导出 Cookie、不读取 Google 凭据。

本地使用 `scripts/start-trend-attach-backend.sh`，必须设置 `TREND_BROWSER_ATTACHED_USER_ID` 为被授权的后台用户 ID。默认连接 `ws://127.0.0.1:9222/devtools/browser`；也可通过 `TREND_BROWSER_ENDPOINT` 指定回环地址。配置对应 `trend_browser.mode: attach`、`endpoint`、`attached_user_id`。端点不由前端输入，禁止外部网络地址，其他后台用户无法使用该 Chrome。脚本不会启动或重启 Chrome。

一次采集完成或普通业务错误后保留连接，后续查询复用；只有需要重新登录、闲置 30 分钟清理或后端退出时主动断开。断开时保留 Ads 采集标签页，不关闭用户的 Chrome 或其他标签页；同一任务登录后优先重新连接原来的采集标签页，若用户已关闭该页才新建。若 Chrome 自己关闭连接，下一次查询尝试重新连接，Chrome 可能再次要求授权；调试连接授权不等于 Google 重新登录。

检测到登录受限结果时，先断开调试连接，后台提示在普通 Chrome 手动登录 Ads。此时后台不会监看 Google 登录画面或操作账号；用户登录后点击“登录完成，继续采集”，再连接并继续原筛选条件的完整采集，校验通过后原子更新 Redis。完全断开期间无法自动感知登录成功，因此保留一次明确的继续操作。此模式禁用远程截图和输入接口。

这是本地电脑和本地后台的连接方式；服务器不能通过自己的 127.0.0.1 访问用户电脑的 Chrome。本次不开放远程调试公网端口，也未部署 Linux 环境。真实登录后采集结果以 QA 记录为准。

已通过隔离 Chrome 合成页面、断开保留标签页和原标签页重连、趋势模块竞态检查、Vue 类型检查、ESLint，以及隔离 Redis 的真实 GET/SET/PXAT 测试。普通 Chrome 连接模式已在本机真实完成 US / 30 天 Tag 和 Video（最高播放量）各 100 条榜单及全部详情采集，并成功写入 Redis；重新打开最新数据页可读取完整数据，视频缓存详情的封面、头像、正文及指标正常显示。两次采集复用同一登录会话，无需重新登录 Google。独立启动模式此前的 Google 登录阻塞不代表连接已登录 Chrome 的方案不可用。Linux 运行尚未验收。


## 每月趋势（Later）

新增“每月趋势”Tab，采用已选择的三列卡片设计，按月份显示 Later 编辑精选。默认选择最新已采集月份；月份写入 URL，刷新、回退和分享保留筛选。

- 已保存 2025-11 至 2026-10 的 12 个月、63 条内容、54 个原声音频引用，61 条视频可直接解析为站内嵌入 ID。2026-10 的 6 个真实封面已保存为本地 JPEG，移除 EXIF；其他缺失封面明确显示待解析。2 条短链接视频尚未解析为播放器 ID。
- 卡片显示真实标题、原文日期、玩法摘要、视频和原声状态。10 月提供中文摘要，其余月份保留源站英文摘要。9 月有 3 条日期与月份冲突，展示原日期及核对标记；缺失音频不补造。
- 点击视频在系统弹窗中加载 TikTok 原生嵌入播放器，来源为严格校验的数字视频 ID。原声音频通过“打开原声”在新标签页跳转 TikTok 原始链接，不解析或下载独立音频文件；缺少链接时不显示跳转按钮。样本播放器已在真实登录后的应用中成功播放，不承诺每条来源都能播放。
- `GET /business/trends/monthly?month=YYYY-MM` 返回月份和内容；`POST /business/trends/monthly/refresh` 抓取固定的公开 Gatsby JSON，不转发系统凭据。两个接口均由现有登录和趋势菜单权限保护。请求有超时、4MiB 限额、拒绝重定向和结构验证；失败保留旧快照。更新按月替换源站当前月分组，保留已退出源站文章的历史月份。没有创建定时任务。
- 数据位于 `backend/data/trends/monthly.json`，封面位于现有 `media/monthly_cover_<videoID>.jpg`，图片接口仍需登录及菜单权限。没有新增依赖。

本机按用户要求保留原 8080 的浏览器附着服务，月度请求暂由回环地址 8086 提供。执行 `scripts/start-monthly-trends-backend.sh` 可重启该独立后端；可通过 `MONTHLY_BACKEND_PORT` 更换端口。前端开发服务器使用 `VITE_MONTHLY_PROXY_TARGET=http://127.0.0.1:8086 pnpm dev --host 127.0.0.1 --port 8848`启用仅月度 API 和月度封面的分流。没有设置该变量时，使用既有统一后端代理；生产环境应部署包含新接口的完整后端，不依赖此开发分流。

月度后端测试：`go test ./cmd/server -run '^TestMonthly'`，覆盖权限负向、源站结构、日期格式、日期冲突、恶意来源、超限、HTTP 错误、月份隔离和归档合并。设计与截图见 `docs/design/trend-insights/monthly/`。

### 月度封面更新（2026-10-29）

“更新内容”同步 Later 文字与链接后，会补抓当前所选月份的缺失封面。封面从 TikTok 公开嵌入页结构化数据提取，仅下载明确允许的公共 CDN 节点图片；不读取浏览器凭据。图片经过尺寸及格式校验后重新编码 JPEG，原子保存到本地缓存，已有封面跳过。三路并发、单节点超时与备用节点保证更新有界；上游拒绝或资源缺失保留占位，下次更新重试。页面显示新增、就绪及待重试数量。没有定时更新。

### 首次部署出现 50301 / 更新后出现 50001（2026-10-30）

线上页面已观察到读取月度内容返回 50301，更新接口返回 HTTP 500 / 50001（月度内容保存失败）。后者表明该次固定上游抓取已成功，失败发生在本地缓存保存阶段；服务器目录缺失和写入权限不足均需检查，不能仅凭浏览器确认实际文件系统原因。

运行时 `data/trends/` 被 Git 忽略，首次部署不会自动带上本机缓存。现保存快照前使用 `os.MkdirAll(..., 0750)` 创建缺失目录，不修改已有目录权限；保存仍采用临时文件及原子替换。前端区分尚无缓存、上游失败与服务器保存失败，不再在无缓存时宣称保留了旧内容。

现有服务器二进制未升级时，可由管理员以**后端服务运行用户**进入后端工作目录，执行：

```sh
mkdir -p data/trends/media
```

若服务显式设置了 `TREND_DATA_DIR`，应在该变量实际指向的目录创建 `media` 子目录。不要通过开放所有人写权限解决。确认服务用户可写后，回到页面点击“更新内容”，应先成功保存 `monthly.json`，随后读取月份列表。此步骤无需重启；若目录已存在仍报 50001，应检查实际工作目录、服务账号权限、只读挂载及存储空间，再由管理员处理。

自动创建目录的代码修复需按正常审核发布流程部署后生效；未由本轮自动部署到生产环境。

### 月度媒体改为浏览器官方嵌入（2026-10-30，当前方案）

按用户确认，本节取代此前“更新内容补抓封面”的行为说明：月度卡片直接使用 Later 条目的数字视频 ID 构造 TikTok 官方 `player/v1/<id>` iframe。浏览器直接向 TikTok 加载预览及播放内容，不使用本地封面或后端图片代理。采用懒加载、默认不自动播放、有标题的 sandbox iframe；无法嵌入的短链接保留明确提示及“打开原视频”入口。每条卡片都提供原视频新标签页链接，原声继续通过“打开原声”跳转。

后端 `POST /business/trends/monthly/refresh` 只请求固定 Later JSON 并保存文章快照，返回 `mediaMode: "tiktok-embed"`，不再调用封面同步或下载 TikTok 图片。月度回归 `TestMonthlyRefreshOnlyFetchesLater` 断言只有一次固定 Later 请求，且不创建 media 目录。此前已缓存的图片没有删除，但当前前端不再使用它们。

播放器使用方式参照 TikTok 官方文档：<https://developers.tiktok.com/docs/en/embed-player>。预览能否加载仍取决于用户浏览器对 TikTok 的可达性；服务器访问 TikTok 的 429 不再影响文章更新。

发布时需同时更新前后端：前端重新构建并发布 `dist`，后端按现有流程构建及重启。只上传封面文件不是本方案的发布步骤。本轮未直接部署到生产服务器。
