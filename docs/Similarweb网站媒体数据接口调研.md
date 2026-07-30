# Similarweb Website 媒体数据接口调研

> 调研日期：2026-07-30
>
> 调研范围：Website 类型媒体档案及量级数据
>
> 资料范围：仅使用 Similarweb 官方 API 文档与官方 Knowledge Center

## 结论摘要

1. Similarweb 适合作为 Website 媒体的流量画像数据源，可提供月独立访客、访问量、浏览量、访问时长、跳出率、地域、渠道、排名、受众和热门页面等数据。最新接口属于 Similarweb API V5；官方文档仍标注 V5 为 beta，因此新接入应封装供应商适配层，避免业务代码直接依赖具体版本。[V5 总览](https://docs.similarweb.com/api-v5) [Web Intelligence API](https://docs.similarweb.com/api-v5/api-reference/website-analysis-api)
2. Website 媒体量级不应使用“粉丝数”。产品中的“月独立访客（UMV）”建议取最新完整自然月、全球范围（`country=ww`）、全设备（`web_source=total`）的 `unique_visitors`。
3. `unique_visitors` 在 Similarweb V5 中明确为“跨设备未去重”的独立访客估算；若业务要求同一人在桌面端和移动端只计算一次，应使用 `deduplicated-audience` 的 `total_deduplicated_audience`。两个指标必须分别标注，不能混用。[Traffic & Engagement](https://docs.similarweb.com/api-v5/similarweb-api/website-analysis-api/website-performance/traffic-and-engagement) [Deduplicated Audience](https://docs.similarweb.com/api-v5/similarweb-api/website-analysis-api/website-audience/deduplicated-audience) [Unique Visitors 定义](https://support.similarweb.com/hc/en-us/articles/4558942036113-Unique-Visitors)
4. Similarweb 官方 Website 数据字段清单没有提供网站 Logo、头像或 favicon 字段。因此媒体头像仍需从目标网站自身的 `icon`、`apple-touch-icon` 或 `og:image` 抓取，并保留远程 URL；Similarweb 只负责流量和受众画像。此结论是根据官方完整数据清单和 Website 数据集字段列表作出的推断。[V5 Available Data](https://docs.similarweb.com/api-v5/guides/available-data) [V4 Website Dataset](https://developers.similarweb.com/docs/websites-dataset)
5. 单个媒体在下拉搜索中被选中后，可用 REST API 即时补全；媒体库批量回填和周期更新使用 Batch API。官方说明 REST 适合即时、精确查询，Batch 适合大规模和历史数据提取，单批最多可处理 100 万个域名。[Getting Started](https://developers.similarweb.com/docs/getting-started)

## UMV 口径

### 推荐产品口径

| 产品字段 | Similarweb 指标 | 推荐查询条件 | 说明 |
| --- | --- | --- | --- |
| 月独立访客（UMV） | `unique_visitors` | 最新完整月、`monthly`、`total`、`ww` | Similarweb 标准 UMV；桌面和移动 Web 跨设备未去重 |
| 跨设备去重月受众 | `total_deduplicated_audience` | 最新完整月、`monthly`、`ww` | 同一用户跨桌面与移动 Web 去重；属于独立端点，权限可能不同 |
| 月访问量 | `visits` | 最新完整月、`monthly`、`total`、`ww` | 会话次数；同一用户可产生多次访问，不能作为 UMV |

Similarweb 对 Monthly Unique Visitors 的定义是：同一用户在一个月内访问多次仍只算一个月独立访客。其官方说明同时指出，全流量独立访客是桌面端与移动 Web 独立访客之和，并非跨设备去重。[Unique Visitors](https://support.similarweb.com/hc/en-us/articles/4558942036113-Unique-Visitors)

因此建议落库时至少保存：

- `umv`、`umv_month`、`umv_country`、`umv_web_source`
- `umv_is_cross_device_deduplicated`
- `data_provider=similarweb`
- `provider_updated_at`、`fetched_at`

若订阅包含 Deduplicated Audience，建议项目中的媒体层级计算优先使用 `total_deduplicated_audience`；否则使用 `unique_visitors`，同时明确记录 `umv_is_cross_device_deduplicated=false`，避免后续将两种口径直接比较。

## 可获取的数据

### 1. 核心流量与互动

V5 `GET /v5/website-analysis/websites/traffic-and-engagement` 可返回：

- `visits`
- `unique_visitors`
- `page_views`
- `average_visit_duration`
- `pages_per_visit`
- `bounce_rate`
- `new_users`
- `returning_users`

接口支持 `daily`、`weekly`、`monthly` 参数，但部分指标存在粒度限制；例如新用户和回访用户只支持月粒度与全设备口径。该端点按返回结果和指标消耗 data credits，官方示例为每个结果每项指标 1 credit，全部 8 项最多 8 credits。[Traffic & Engagement](https://docs.similarweb.com/api-v5/similarweb-api/website-analysis-api/website-performance/traffic-and-engagement)

### 2. 跨设备去重受众

V5 `GET /v5/website-analysis/websites/deduplicated-audience` 可返回：

- `total_deduplicated_audience`
- 仅桌面端受众数量与占比
- 仅移动 Web 受众数量与占比
- 同时使用桌面端和移动 Web 的受众数量与占比

该端点只支持月粒度，并消耗 data credits。[Deduplicated Audience](https://docs.similarweb.com/api-v5/similarweb-api/website-analysis-api/website-audience/deduplicated-audience)

### 3. 地域分布

V5 `GET /v5/website-analysis/websites/geography/aggregated` 按国家返回：

- 国家代码和国家名称
- 国家排名
- 访问量及流量占比
- 平均访问时长
- 每次访问页数
- 跳出率

该端点仅支持月粒度，可用于判断媒体核心市场和受众地域分布。[Traffic Geography](https://docs.similarweb.com/api-v5/similarweb-api/website-analysis-api/website-performance/traffic-geography)

### 4. 流量渠道

V5 `GET /v5/website-analysis/websites/traffic-sources` 可按以下渠道返回访问量和互动指标：

- Direct
- Display Ads
- Mail
- Organic Search
- Paid Search
- Referrals
- Social

全设备和移动 Web 口径只支持月粒度；平均访问时长、每次访问页数和跳出率等部分互动指标仅支持桌面端。[Marketing Channel Sources](https://docs.similarweb.com/api-v5/api-reference/website-analysis-api/website-performance/marketing-channel-sources)

### 5. 排名与类目

V5 `GET /v5/website-analysis/websites/website-rank` 可返回：

- `country_rank`
- `category`
- `category_rank`

该端点仅支持全设备口径；支持月粒度，以及最近 28 天的日粒度。[Website Ranking](https://docs.similarweb.com/api-v5/api-reference/website-analysis-api/website-performance/website-ranking)

### 6. 受众画像

Website Audience 系列端点还可提供：

- 年龄、性别分布
- 指定年龄和性别人群的流量占比与互动指标
- 受众兴趣与关联网站
- 2 至 5 个网站的受众重叠

相关端点包括 `demographics/aggregated`、`traffic-by-demographics/aggregated`、`audience-interests/aggregated` 和 `audience-overlap/aggregated`。Audience & Demographics 数据的历史范围取决于订阅，旧版 Batch 文档标注最多 37 个月。[Website Audience API](https://docs.similarweb.com/api-v5/api-reference/website-analysis-api/website-audience) [Audience & Demographics Dataset](https://developers.similarweb.com/docs/websites-audience-analysis-dataset)

### 7. 可选增强数据

V5 还提供：

- Referrals：引荐来源及外链流量
- Similar Sites：相似网站和亲和度
- Popular Pages：热门页面、流量占比和趋势分类
- Subdomains / Leading Folders：子域名和目录流量
- Technologies：网站使用的技术栈
- PPC Spend：付费搜索投放估算

这些字段适合用于媒体档案详情或后续的媒体筛选，不建议在首期列表接口中全部实时查询。[V5 Available Data](https://docs.similarweb.com/api-v5/guides/available-data)

## 历史范围与数据粒度

- V5 Web Intelligence 官方页面标注最多 61 个月历史数据，取决于订阅。[Web Intelligence API](https://docs.similarweb.com/api-v5/api-reference/website-analysis-api)
- V5 总览描述所有数据类型最长可达 7 年，但这不是每个 Website 端点都能获得的保证。[V5 总览](https://docs.similarweb.com/api-v5)
- 旧版 REST 的多数流量接口标注最多 37 个月；旧版 Batch Website Dataset 当前标注最多 61 个月。[REST Visits](https://developers.similarweb.com/reference/visits) [Batch Website Dataset](https://developers.similarweb.com/docs/websites-dataset)
- 实际可访问的国家、日期、指标和粒度由订阅决定。V5 大多数端点提供 `/describe`，例如 `GET /v5/website-analysis/websites/traffic-and-engagement/describe`；应在开发和运行期以该结果为准。[Describe endpoints](https://docs.similarweb.com/api-v5/guides/available-data)

## API 版本与 Batch 数据集

- V5 REST 是新接入的优先选择，核心端点统一位于 `/v5/website-analysis/...`，并支持一次请求多个指标。
- V4 Batch 仍通过 `POST https://api.similarweb.com/batch/v4/request-report` 创建报告。Website 相关 vtable 包括 `traffic_and_engagement`、`traffic_sources`、`referrals`、`desktop_top_geo`、`demographics`、`audience_interests` 和 `website`。[Batch Website Dataset](https://developers.similarweb.com/docs/websites-dataset) [Batch Audience Dataset](https://developers.similarweb.com/docs/websites-audience-analysis-dataset)
- 截至调研日期，营销渠道正处于新旧数据并行期。新 Batch vtable 为 `website_marketing_channels`，旧 `marketing_channels` 将在 2026-11-30 完全停用；新开发不应依赖旧表。新表提供全设备、桌面端和移动 Web 的 visits/share，并新增 Gen AI、Affiliates、Paid Social、Organic Social 等渠道能力。[Marketing Channels Data Upgrade](https://support.similarweb.com/hc/en-us/articles/36081690111517-Marketing-Channels-Data-Upgrade-API)

## 鉴权、额度与套餐限制

1. 使用 API Key 鉴权，V5 示例通过请求头 `api-key: YOUR_API_KEY` 发送。只有账户管理员可以生成 API Key，并且 Key 必须处于激活状态。[Authentication](https://developers.similarweb.com/docs/authentication)
2. Similarweb API 属于付费能力。官方旧版页面称 API 是 subscription add-on，V5 页面则称付费订阅包含 API；由于目前处于版本与套餐迁移期，应由 Similarweb Account Manager 确认本账户的 V5、Batch 和 Deduplicated Audience 权限。[API 产品说明](https://developers.similarweb.com/docs/similarweb-web-traffic-api) [V5 总览](https://docs.similarweb.com/api-v5)
3. `GET https://api.similarweb.com/capabilities?api_key=...` 可免费查看剩余 credits、可用国家和历史范围；V5 各端点的 `/describe` 应作为更细粒度的权限依据。[Check Capabilities](https://developers.similarweb.com/docs/check-capabilities)
4. REST API 官方限制为 10 requests/second，超限返回 HTTP 429；批量数据应改用 Batch API。[Rate Limit](https://developers.similarweb.com/docs/rate-limit)
5. Data credits 的消耗受域名数、端点价格、粒度、国家数、历史区间和返回条数影响。只请求页面实际需要的指标，并使用缓存，可显著降低消耗。[Data Credits](https://developers.similarweb.com/docs/data-credits-unpublished-whats-new-in-v40)

## 推荐接入方案

### 首期

1. 用户将媒体类型选为 Website，并输入网址。
2. 后端把网址规范化为主域名：去掉协议、`www.`、路径和参数。Similarweb 接口要求传域名而不是完整 URL。[Website Dataset](https://developers.similarweb.com/docs/websites-dataset)
3. 首次接入时调用 `/capabilities` 和相关 `/describe`，保存账户实际支持的国家、历史范围、粒度和指标。
4. 用 Traffic & Engagement 查询最新完整月的 `unique_visitors`、`visits`、`page_views`、`average_visit_duration`、`pages_per_visit`、`bounce_rate`。
5. 若账户支持 Deduplicated Audience，再查询 `total_deduplicated_audience`，将其作为媒体层级划分的优先口径。
6. 从网站自身抓取头像，优先级建议为 `apple-touch-icon`、高分辨率 `icon`、`og:image`、默认站点图标；保存远程 URL，并可另存本地缓存。
7. 数据按“域名 + 月份 + 国家 + 设备口径 + 指标口径”缓存。列表只读缓存，不在每次打开列表时请求 Similarweb。

### 后续增强

- 定时使用 Batch API 更新媒体库全部 Website 数据。
- 增加地域、渠道、排名、受众画像和热门页面。
- 对 API Key、credits、429、无数据和权限不足设置独立监控。
- 保留 Similarweb 原始响应快照或关键元数据，以便指标版本变化时追溯。

## 建议的数据状态提示

前端应区分以下状态，避免把缺少权限或无数据误显示为 `0`：

- 已更新：展示数值和统计月份
- 暂无估算：Similarweb 对该域名无可用结果
- 权限不足：当前套餐不包含该国家、历史范围或指标
- 待更新：已有域名但尚未完成抓取
- 数据过期：超过设定刷新周期

错误返回、没有 credits 和速率限制可分别对应 Similarweb 的权限/额度错误及 HTTP 429。[REST Error Codes](https://developers.similarweb.com/docs/error-codes-rest)
