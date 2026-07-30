# Similarweb 免费与开源平替调研

> 调研日期：2026-07-30
>
> 适用场景：项目中的 Website 媒体档案、月独立访客（UMV）与第三方网站流量画像
>
> 资料范围：仅使用各产品官方或项目第一方资料

## 结论

本次核查未发现一个同时满足以下条件的产品：

- 免费且允许商业使用；
- 可通过 API 查询任意第三方网站；
- 返回可靠的绝对月独立访客（UMV）；
- 不需要网站所有者安装统计代码或授权数据。

原因是绝对 UMV 必须依赖跨站流量样本、运营商/浏览器/插件面板数据，或网站自身的一方统计数据。免费公开数据通常只提供相对排名、排名区间、DNS 热度或真实用户性能分布，不公开绝对访客数。网站分析工具则能提供准确的一方访客数据，但只能查询自己或已授权的网站。

因此本项目可采用“双轨方案”：

1. 对愿意授权数据的媒体，接入 GA4 或由媒体提供报表，形成“已验证 UMV”。
2. 对未授权的第三方 Website，免费方案只能使用 CrUX 热度区间、Cloudflare Radar/Tranco 排名等替代指标；若产品必须显示绝对 UMV，仍需 Similarweb 等付费估算数据源。

## 总体对比

| 方案 | 任意第三方网站 | 绝对 UMV | 免费情况 | API | 商用结论 | 本项目用途 |
| --- | --- | --- | --- | --- | --- | --- |
| Google Analytics 4 | 否，必须获得 Property 权限 | 是，一方数据 | 标准版免费 | 有 | 可商用，但第三方数据必须取得授权 | 媒体授权后的“已验证 UMV” |
| Cloudflare Web Analytics | 否，必须部署 Beacon 或控制 Cloudflare Zone | 否；主要提供 Visits、Page views 和性能 | 免费 | GraphQL，权限限定账户/Zone | 可用于自己的站点 | 媒体自有数据补充，不适合第三方估算 |
| Cloudflare Radar | 是，覆盖进入其数据集的域名 | 否 | API 免费 | 有 | 数据为 CC BY-NC 4.0，不适合直接用于商业产品 | 非商用调研或获得授权后的热度/地域代理 |
| Matomo On-Premise | 否，必须自行采集或获得实例权限 | 是，一方数据 | 核心版永久免费，自付服务器 | 有 | GPLv3，官方明确兼容商业使用 | 自建的一方统计或媒体授权数据 |
| Plausible CE / Cloud | 否，必须安装脚本并拥有站点权限 | 不等同严格月 UMV；跨天/跨设备会重复 | CE 免费自托管；Cloud 仅试用 | 有，但 Cloud Stats API 为 Business 功能 | CE 为 AGPLv3，闭源集成需法务评估 | 隐私友好的一方日活/访问量，不适合作为第三方 UMV |
| Tranco | 是，可查进入榜单的域名 | 否 | 榜单与 API 可访问 | 有 | 项目定位为研究，且来源包含 CC BY-NC 数据；商业使用需单独确认 | 排名代理、开发测试 |
| Google CrUX | 是，但仅覆盖公开且达到样本门槛的 Origin | 否 | API 免费 150 次/分钟；BigQuery 有免费层 | 有 | 数据 CC BY 4.0，可在署名条件下商用 | 推荐的免费第三方“热度区间 + 性能”代理 |

## 1. Google Analytics 4

### 能提供什么

GA4 Data API 可查询 `activeUsers`、`totalUsers`、`sessions`、页面浏览、互动与地域等一方数据。`totalUsers` 是至少记录过一个事件的不同用户数，`activeUsers` 是访问过站点或应用的不同活跃用户数。[GA4 API Dimensions & Metrics](https://developers.google.com/analytics/devguides/reporting/data/v1/api-schema)

标准 GA4 服务免费；Google 官方页面明确称其提供免费工具，服务条款也写明 GA4 Properties 免费提供。[Google Analytics 产品页](https://marketingplatform.google.com/about/analytics/) [Google Analytics Terms](https://marketingplatform.google.com/about/analytics/terms/us/)

### 为什么不能替代 Similarweb

Data API 请求必须指定 `properties/{PROPERTY_ID}`，调用者必须先在 GA 界面获得该 Property 的访问权限。因此它只能读取自己或合作媒体授权的站点，不能输入任意域名查询第三方网站 UMV。[Data API Quickstart](https://developers.google.com/analytics/devguides/reporting/data/v1/quickstart)

Google 条款允许在自己的 Property 或取得授权的 Third Party Property 上使用；代表第三方使用时必须有权代表对方，且未经第三方同意不得向其他方披露其 Customer Data。[Google Analytics Terms](https://marketingplatform.google.com/about/analytics/terms/us/)

### 推荐用法

- 在媒体档案中增加“数据已验证”状态。
- 支持媒体通过 OAuth 授权指定 GA4 Property，读取最新完整月的 `totalUsers` 或约定的用户指标。
- 保留 Property、日期、时区、指标定义和授权主体。
- GA4 数据可作为媒体主动提供的高可信一方数据，但不能承担“全网搜索媒体”的默认数据源。

## 2. Cloudflare Web Analytics 与 Cloudflare Radar

### Cloudflare Web Analytics

Cloudflare Web Analytics 是免费的隐私优先一方分析工具。网站需加入 Cloudflare 账户并安装 JavaScript Beacon；即使网站不通过 Cloudflare 代理，也必须在页面加入脚本后才会产生数据。[Cloudflare Web Analytics](https://developers.cloudflare.com/web-analytics/about/) [启用方式](https://developers.cloudflare.com/web-analytics/get-started/)

其官方高层指标主要是 Visits、Page views、Page load time 和 Core Web Vitals，并未提供可直接替代 Similarweb UMV 的严格月独立访客字段。[High-level metrics](https://developers.cloudflare.com/web-analytics/data-metrics/high-level-metrics/)

GraphQL Analytics API 只允许访问 Token 获得授权的账户和 Zone；Token 可明确限制账户与 Zone 资源，所以不能查询任意第三方域名。[GraphQL Authentication](https://developers.cloudflare.com/analytics/graphql-api/getting-started/authentication/)

Cloudflare Zone Analytics 虽有 Unique Visitors，但其口径基于请求 IP，且会包含爬虫、威胁或未完整加载页面的请求，官方明确提示数值可能高于浏览器端分析工具，不能直接当作人级别 UMV。[Cloudflare Analytics FAQ](https://developers.cloudflare.com/analytics/faq/about-analytics/)

### Cloudflare Radar

Radar API 可以查询任意已覆盖域名的：

- 全球或国家 Top 100 的精确顺序；
- Top 200、500、1,000 至 1,000,000 的排名桶；
- 基于 1.1.1.1 DNS 查询的地域占比和热度趋势。

排名来自 Cloudflare 的公共 DNS 查询，并不返回网站绝对访客数或 UMV。[Radar Domain Ranking](https://developers.cloudflare.com/radar/investigate/domain-ranking-datasets/) [Radar DNS](https://developers.cloudflare.com/radar/investigate/dns/)

Radar API 免费，但官方说明其数据按 CC BY-NC 4.0 提供；`NC` 限制意味着未经额外授权不应直接用于本商业项目的生产功能。[Cloudflare Radar Overview](https://developers.cloudflare.com/radar/)

### 推荐用法

- Cloudflare Web Analytics：仅作为合作媒体自行安装后的真实流量补充。
- Cloudflare Radar：可用于内部调研和开发验证；生产商用前必须取得 Cloudflare 授权。
- 不把 Radar 排名、DNS 请求量或 Zone 的 IP-based uniques 标记为 UMV。

## 3. Matomo

Matomo Reporting API 可返回：

- `nb_uniq_visitors`
- `nb_visits`
- 页面浏览与动作数
- 停留时长、跳出、转化等

接口通过 `idSite` 查询 Matomo 实例中已配置并采集的网站，因此不能用域名查询任意第三方网站。[Matomo Reporting API](https://developer.matomo.org/guides/reporting-api)

Matomo Core On-Premise 可永久免费自托管，官方明确说明没有软件许可费；Cloud 版本收费并提供限时试用。[Matomo 免费说明](https://matomo.org/faq/log-analytics-tool/is-matomo-truly-free-to-use-what-are-the-costs-or-requirements/) [Matomo Pricing](https://matomo.org/pricing/)

Matomo 使用 GPLv3，官方功能页明确称其兼容商业使用；但服务器、数据库、维护和合规仍由本项目承担。[Matomo Licences](https://matomo.org/licences/) [Matomo Features](https://matomo.org/features/)

### 推荐用法

Matomo 适合将来为自有站点或愿意安装统计代码的媒体提供一方统计能力。它可以生成真实月独立访客，但无法替代 Similarweb 的第三方流量估算。

## 4. Plausible

Plausible Stats API 可返回 `visitors`、`visits`、`pageviews`、`bounce_rate` 和 `visit_duration`，但请求中的 `site_id` 必须是已经添加到调用者 Plausible 账户中的网站。Cloud Stats API 属于 Business 付费方案，默认限制为每个 API Key 每小时 600 次请求。[Plausible Stats API](https://plausible.io/docs/stats-api) [Subscription Plans](https://plausible.io/docs/subscription-plans)

Plausible Community Edition 可免费自托管，采用 AGPLv3。官方说明在网络服务中使用 AGPL 代码涉及向网络用户提供相应源代码；在闭源商业产品中进行修改、派生或深度嵌入前需法务确认。[Plausible CE](https://plausible.io/self-hosted-web-analytics) [AGPL 说明](https://plausible.io/blog/open-source-licenses)

Plausible 不使用持久 Cookie。同一个人从不同设备访问或在不同日期访问会被计为不同访客，所以其“Unique Visitors”不等同跨整月、跨设备去重的严格 UMV。[Metrics Definitions](https://plausible.io/docs/metrics-definitions) [Security Practices](https://plausible.io/security)

### 推荐用法

可用于隐私友好的一方日活、访问量和内容表现，但不适合本项目的第三方 Website UMV，也不应将其长周期 `visitors` 与 Similarweb 的月独立访客直接比较。

## 5. Tranco

Tranco 提供每日更新的 Top 100 万域名榜单、单域名历史排名查询、下载和 API。其排名将多个供应商的榜单在过去 30 天内聚合，目标是提高研究的稳定性和抗操纵性，而不是估算绝对访问量。[Tranco 首页](https://tranco-list.eu/) [Tranco Methodology](https://tranco-list.eu/methodology)

它只能回答“域名大致有多热门、排名多少”，不能返回访问人数、访问量或 UMV。

Tranco 第一方页面说明其数据源包含 Cloudflare Radar（CC BY-NC 4.0）等不同许可证的数据，且服务本身定位为 research-oriented。页面没有给出可直接支撑本商业产品使用聚合榜单的统一商业许可，因此生产使用前需要单独确认；在未确认前仅建议用于内部测试或研究。[Tranco 首页及来源署名](https://tranco-list.eu/)

## 6. Google Chrome UX Report（CrUX）

CrUX 可通过免费 REST API 查询公开且样本充足的第三方 Origin 或 URL。API 每个 Google Cloud 项目免费提供 150 queries/minute，不能付费提高该额度。[CrUX API](https://developer.chrome.com/docs/crux/api)

可获得：

- LCP、INP、CLS、TTFB 等真实用户性能分布；
- 桌面、手机和平板占比；
- BigQuery 中的国家维度；
- `rank` 热度区间。

CrUX 的 `rank` 是按 Origin 导航次数计算的相对热度区间，例如 1,000、5,000、10,000、50,000 等，并非精确排名，更不是 UMV。[CrUX Metrics](https://developer.chrome.com/docs/crux/methodology/metrics) [CrUX Release Notes](https://developer.chrome.com/docs/crux/release-notes)

CrUX 只覆盖公开可索引且达到未公开最低访客样本量的网站，且数据来自符合条件并选择同步使用情况统计的 Chrome 用户；Google不会公布符合条件用户所占比例，因此不能用其样本反推绝对访客数。[CrUX Methodology](https://developer.chrome.com/docs/crux/methodology)

CrUX 数据采用 CC BY 4.0，可在遵守署名要求的情况下用于商业用途。BigQuery 公共数据从 2017 年开始，免费层每月包含前 1 TiB 查询处理量；超出后按 BigQuery 用量计费。[CrUX BigQuery](https://developer.chrome.com/docs/crux/bigquery) [BigQuery Pricing](https://cloud.google.com/bigquery/pricing)

### 推荐用法

CrUX 是本项目最适合的免费第三方补充源，但只能作为：

- Website 是否达到一定公共热度门槛；
- 粗粒度流量排名区间；
- 真实用户体验质量；
- 设备和国家分布代理。

建议字段命名为“CrUX 热度区间”或“网站热度等级”，绝不能显示成“月独立访客”。

## 本项目建议

### 可立即落地的免费能力

1. 用 CrUX API 查询域名是否被覆盖，并保存热度区间、Core Web Vitals、设备分布和数据月份。
2. 无 CrUX 数据时显示“暂无公开热度数据”，不能显示 `0 UMV`。
3. 可将 CrUX 热度区间用于低成本的候选媒体排序，但不参与现有以 UMV 为阈值的头部/腰部/尾部分层。
4. 媒体若提供 GA4 或 Matomo 数据授权，则展示“已验证 UMV”并可参与准确分层。

### 分层建议

在缺少 Similarweb 时，不应把排名区间硬换算为访客人数。建议把 Website 分层拆成两套状态：

- `UMV 分层`：仅对 Similarweb 或经媒体授权的一方 UMV 生效；
- `公开热度等级`：依据 CrUX/经授权可商用的排名数据展示“高/中/低/无公开样本”，不与 100 万、10 万阈值混算。

### 数据可信度展示

| 数据来源 | 建议可信度标签 | 是否可用于 UMV 分层 |
| --- | --- | --- |
| 媒体授权 GA4/Matomo | 已验证一方数据 | 是 |
| Similarweb 去重或标准 UMV | 第三方估算 | 是，但需标注估算口径 |
| CrUX | 公开热度代理 | 否 |
| Cloudflare Radar/Tranco | 排名代理 | 否；另需解决商业许可 |
| 网站自行填写且无证据 | 媒体自报 | 可人工确认后使用 |

## 最终判断

- GA4、Matomo、Plausible 和 Cloudflare Web Analytics 是“自有/授权网站分析工具”，不是 Similarweb 的第三方查询平替。
- Cloudflare Radar、Tranco 和 CrUX 是“第三方公开热度代理”，不能提供绝对 UMV。
- CrUX 是免费、API 可用且商业许可最清晰的补充方案，但只能用于热度区间和性能，不可伪装成 UMV。
- 若业务坚持自动查询任意 Website 的绝对月独立访客，则不存在已核实的免费可靠方案，需要采购 Similarweb 类商业数据，或要求媒体授权/上传一方数据。
