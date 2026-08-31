# TikHub 标准项目模板平台接口调研

调研日期：2026-08-31

范围：标准项目模板中的 `Website`、`播客`、`电视`、`报刊`、`YouTube`、`TikTok`、`Instagram`、`Facebook`、`X`、`LinkedIn`、`Reddit`，以及本次特别要求的 `小红书 / RedNote`。
来源约束：仅使用 TikHub 官方 [Apifox 文档](https://docs.tikhub.io/)、[Swagger UI](https://api.tikhub.io/) 和[生产 OpenAPI](https://api.tikhub.io/openapi.json)，仓库部分仅做只读代码对照。

## 结论摘要

| 模板平台 | TikHub 公开接口 | 账号/主页资料 | 作品列表 | 单作品详情或 URL 输入 | 当前仓库状态 |
| --- | --- | --- | --- | --- | --- |
| Website | 无通用网站接口 | — | — | — | 已使用 Similarweb/网页抓取，不依赖 TikHub |
| 播客 | 无通用播客接口 | — | — | — | 无平台同步 |
| 电视 | 无通用电视接口 | — | — | — | 无平台同步 |
| 报刊 | 无通用报刊接口 | — | — | — | 无平台同步 |
| YouTube | 有 | 有，频道资料 | 有，视频/Shorts/直播 | 有，支持视频 ID；V2 也支持视频 URL | 已用 Google YouTube Data API 完成资料、作品和单视频同步 |
| TikTok | 有 | 有 | 有 | 有，支持作品 ID；App V3 支持分享链接 | 已用 TikHub 完成资料、作品、单作品同步 |
| Instagram | 有 | 有 | 有，帖子和 Reels | 有，V3 可直接传帖子 URL | 已用 TikHub V1 完成资料、作品和单作品同步；可升级 V3 |
| Facebook | 无 | — | — | — | 明确返回 TikHub 不支持 |
| X | 有，官方分组名为 Twitter | 有 | 有 | 有，传 tweet ID | 已用 TikHub 完成资料、作品、单推文同步 |
| LinkedIn | 有，Web V2 | 有，个人和公司 | 有，个人和公司 | 有，帖子/文章详情直接传 URL | 已完成资料和作品列表；单内容链接尚未接入 |
| Reddit | 有，APP | 有 | 有 | 有，传带 `t3_` 前缀的帖子 ID | 已完成资料和作品列表；单内容链接尚未接入 |
| 小红书 / RedNote | 有，官方推荐 App V2 | 有 | 有，笔记列表 | 有，`share_text` 可直接传长链或短链 | 当前工作区已新增资料、首页笔记和图文详情实现；分页与视频详情仍需补齐 |

生产 OpenAPI 当前没有任何路径包含 `facebook`、`podcast`、`television`、`newspaper`、`magazine` 或通用 `website`。TikHub 虽提供今日头条、微信公众号、网易云音乐等特定产品接口，但它们不能等价替代通用“报刊”“播客”或“Website”。此结论是 2026-08-31 的接口快照，不代表 TikHub 以后不会新增。

## 通用调用约束

- 鉴权：`Authorization: Bearer <token>`。
- 基础地址：中国大陆使用 `https://api.tikhub.dev`，其他地区使用 `https://api.tikhub.io`；路径和参数相同。[官方 README](https://docs.tikhub.io/)
- 官方生产 OpenAPI 声明：QPS 上限 `10/秒`、错误最多重试 `3` 次、请求超时建议 `30–60 秒`。[生产 OpenAPI](https://api.tikhub.io/openapi.json)
- 成功响应统一是信封结构，包含 `code`、`request_id`、`message`、`message_zh`、`data` 等字段；具体业务数据在 `data`。OpenAPI 的 200 响应统一引用通用 `ResponseModel`，很多接口没有稳定的细粒度响应 Schema，因此解析器应保留多候选字段、容忍空值，并保留原始 JSON 用于排查。[生产 OpenAPI](https://api.tikhub.io/openapi.json)
- 官方响应文案会提示请求计费。除文档明确给出价格的接口外，不应在代码里写死费用。

## 各平台接口

### 小红书 / RedNote

TikHub 官方把小红书接口优先级标为 `App V2 > App > Web V3 > Web V2 > Web`。本项目应直接采用 App V2。

#### 用户资料

```http
GET /api/v1/xiaohongshu/app_v2/get_user_info
```

- 参数：`user_id` 或 `share_text` 二选一；两者同时传时优先 `user_id`。
- `share_text` 支持 `xiaohongshu.com` 长链接和 `xhslink.com`、`xhslink.cn` 短链接，因此无需自行解析短链才能请求。
- 返回：昵称、头像、简介、粉丝数、关注数、笔记数等用户资料。
- 限制：无效用户 ID 仍可能 HTTP/业务请求成功，但 `data` 返回上游“服务异常”，而且仍计费，不能只看外层 `code` 判成功。

来源：[获取用户信息](https://docs.tikhub.io/420136395e0)

#### 用户笔记列表

```http
GET /api/v1/xiaohongshu/app_v2/get_user_posted_notes
```

- 参数：`user_id` 或 `share_text` 二选一；`cursor` 首次为空。
- 分页：下一页传上次响应的 cursor；官方给出的路径是 `$.data.data.notes[-1].cursor`，通常为列表最后一条笔记的 `note_id`。
- 返回：笔记基本信息和分页信息。
- 限制：无效用户或分享链接解析失败仍可能计费；列表接口没有 `count` 参数，达到业务 `post_limit` 前需按 cursor 拉取并按 `note_id` 去重。

来源：[获取用户笔记列表](https://docs.tikhub.io/420136396e0)

#### 单笔记详情和 URL 解析

```http
GET /api/v1/xiaohongshu/app_v2/get_image_note_detail
GET /api/v1/xiaohongshu/app_v2/get_video_note_detail
```

- 两个接口参数相同：`note_id` 或 `share_text` 二选一；`share_text` 同样支持上述长短链接。
- 推荐流程：先调 `get_image_note_detail`。它可识别图文和视频笔记，并返回正文、图片/封面、作者和互动数据；若响应类型是 `video` 且需要播放地址，再调 `get_video_note_detail`。
- `get_video_note_detail` 只能获取视频笔记；图文笔记不能用该接口。图文接口遇到视频时只给封面，不给视频播放链接。
- 无效 note ID 或分享链接解析失败也可能正常计费，必须验证业务数据而非只验证 HTTP 200。

来源：[图文笔记详情](https://docs.tikhub.io/420136391e0)、[视频笔记详情](https://docs.tikhub.io/420136392e0)

建议归一化：`note_id`、标题/正文、作者 ID/昵称、笔记类型、发布时间、封面/图片/视频 URL、点赞、评论、收藏、分享；播放量只有上游确实返回时才写入，不能由互动数推算。

### TikTok

```http
GET /api/v1/tiktok/web/fetch_user_profile
GET /api/v1/tiktok/web/fetch_user_post
GET /api/v1/tiktok/web/fetch_post_detail
GET /api/v1/tiktok/app/v3/fetch_one_video_by_share_url_v2
```

- 资料：`uniqueId` 或 `secUid` 至少一个，优先 `uniqueId`。[用户资料](https://docs.tikhub.io/186826057e0)
- 作品列表：必填 `secUid`；`cursor=0` 为第一页，后续用返回的 `cursor`；`count` 默认和最大都是 15；用 `hasMore` 判断结束，相邻页可能重复，必须按作品 `id` 去重。[用户作品](https://docs.tikhub.io/186826058e0)
- 列表返回的视频 CDN 直链可能需要响应中的 `tt_chain_token`；如果只需要指标和封面，不应依赖该临时播放链。
- 单作品：Web 详情传 `itemId`，可选 `region`，默认 `US`；V1 首个播放链接无需 Cookie。[单作品详情](https://docs.tikhub.io/186826056e0)
- 分享链接：App V3 `fetch_one_video_by_share_url_v2` 直接传 `share_url`，适合短链或不能可靠本地解析的链接。[分享链接详情](https://docs.tikhub.io/402414076e0)

仓库已覆盖资料、列表、单作品。当前列表请求发送 `count=20`，但官方最大值为 15，服务端会按 15 处理；应改为 15，并在需要更多作品时实现 cursor 翻页。仓库 TikTok HTTP client 超时为 15 秒，也低于 TikHub 官方建议的 30–60 秒。

### Instagram

建议新实现优先 V3：

```http
GET /api/v1/instagram/v3/get_user_profile
GET /api/v1/instagram/v3/get_user_posts
GET /api/v1/instagram/v3/get_user_reels
GET /api/v1/instagram/v3/get_post_info
GET /api/v1/instagram/v3/get_post_info_by_code
GET /api/v1/instagram/v3/extract_shortcode
```

- 资料：必填 `username`。返回路径包括 `data.user.id`、`username`、`full_name`、`biography`、头像、认证/隐私状态、粉丝数、关注数、帖子总数和 Reels 数；价格标为 0.008 USD/请求。[用户资料](https://docs.tikhub.io/419083059e0)
- 帖子：必填 `username`；`first` 默认 12、最大 50；后翻页使用 `after = data.page_info.end_cursor`，前翻页使用 `last + before`；返回 `data.edges` 和 `data.page_info`；价格 0.008 USD/请求。[用户帖子](https://docs.tikhub.io/419083061e0)
- Reels：必填 `username`；`first` 默认 12、最大 50，分页同上；返回 `code`、`pk`、点赞、评论、播放数、文本、媒体和封面；价格 0.008 USD/请求。[用户 Reels](https://docs.tikhub.io/419083063e0)
- 单帖 URL：`get_post_info` 接受 `media_id` 或完整 `url`，支持 `/p/`、`/reel/`、`/reels/`、`/tv/`；返回 `data.items`，含 ID、短码、媒体类型、点赞、评论、文本、作者、媒体版本和发布时间；价格 0.008 USD/请求。[帖子详情](https://docs.tikhub.io/419083069e0)
- 若已从 URL 取出 shortcode，可调用 `get_post_info_by_code?code=...`；也可用 `extract_shortcode?url=...` 专门解析 URL。[短码详情](https://docs.tikhub.io/419083070e0)

仓库当前仍用 V1 的用户名/ID资料、用户帖子、Reels 和 URL 帖子详情接口；这些路由仍在当前 OpenAPI 中，不是失效接口。升级 V3 的价值是字段和分页契约更明确、单帖可直接传 URL，但需要同步调整归一化字段。当前 Instagram HTTP client 同样只有 15 秒超时。

### X（Twitter）

```http
GET /api/v1/twitter/web/fetch_user_profile
GET /api/v1/twitter/web/fetch_user_post_tweet
GET /api/v1/twitter/web/fetch_tweet_detail
```

- 资料：`screen_name` 或 `rest_id` 二选一；使用 `rest_id` 时忽略用户名。[用户资料](https://docs.tikhub.io/191321710e0)
- 作品列表：同样用 `screen_name` 或 `rest_id`；`cursor` 从上一次响应取得。[用户发帖](https://docs.tikhub.io/191321711e0)
- 单推文：必填 `tweet_id`，从 `/status/<id>` 链接本地提取即可。[单推文](https://docs.tikhub.io/191321709e0)

仓库已覆盖这三类能力；目前作品列表只取第一页。如业务 `post_limit` 超过首页数量，应补 cursor 翻页。

### LinkedIn

个人账号：

```http
GET /api/v1/linkedin/web_v2/get_user_profile?url=<完整主页URL>
GET /api/v1/linkedin/web_v2/get_user_posts?url=<完整主页URL>
```

- 资料返回姓名、当前公司、简介、所在地、粉丝数、经历和教育等公开字段。[个人资料](https://docs.tikhub.io/452347395e0)
- 作品列表：`type` 可为 `posts`、`comments`、`reactions`，默认 `posts`；分页用 `start`（0、50、100…）和上次 `paging.pagination_token`；响应含 `data` 列表和 `paging`。[个人帖子](https://docs.tikhub.io/452347396e0)

公司账号：

```http
GET /api/v1/linkedin/web_v2/get_company_profile?url=<完整公司URL>
GET /api/v1/linkedin/web_v2/get_company_posts?url=<完整公司URL>
```

- 公司资料返回名称、简介、规模、行业、总部、粉丝数等。[公司资料](https://docs.tikhub.io/452347418e0)
- 公司帖子分页同样使用 `start + pagination_token`，另有 `sort_by=top|recent`。[公司帖子](https://docs.tikhub.io/452347420e0)

单内容：

```http
GET /api/v1/linkedin/web_v2/get_post_detail?url=<帖子或文章URL>
```

支持 `/posts/...` 与 `/pulse/...`，返回正文、作者、发布时间、标签、图片/视频和互动数。[生产 OpenAPI](https://api.tikhub.io/openapi.json)

限制：官方特别说明个人/公司帖子接口来自有限资源池，高并发或资源紧张时可能间歇性返回 400，建议 `retry=3`。仓库通用请求器只会对网络错误、429 和 5xx 做域名 fallback，不会重试 400，因此 LinkedIn 需要单独的受限重试策略。仓库已完成资料和首页作品，但尚未在内容链接同步分支接 `get_post_detail`，也未使用分页参数。

### Reddit

```http
GET /api/v1/reddit/app/fetch_user_profile
GET /api/v1/reddit/app/fetch_user_posts
GET /api/v1/reddit/app/fetch_post_details
```

- 资料：必填 `username`（不带 `u/`）；可选 `need_format`。返回用户名/ID、创建时间、帖子与评论 Karma、头像/横幅、简介、认证、徽章、关注者数。[用户资料](https://docs.tikhub.io/369454690e0)
- 作品列表：必填 `username`；`sort` 可为 `NEW`、`TOP`、`HOT`、`CONTROVERSIAL`；`after` 用于下一页；返回标题/正文、时间、Subreddit、upvotes、评论数、内容类型、媒体和分页信息。[用户帖子](https://docs.tikhub.io/369454693e0)
- 单帖详情：`post_id` 必须是 APP 格式，带 `t3_` 前缀；可选 `include_comment_id` 和带 `t1_` 前缀的 `comment_id`；返回正文、标题、作者、统计、版块、媒体等。[帖子详情](https://docs.tikhub.io/369454680e0)
- URL 解析：详情接口不直接接受 Reddit URL；业务侧需从 `/comments/<id>/...` 提取 ID 并规范成 `t3_<id>`。

仓库已完成资料和首页作品，但没有把 Reddit 内容链接接入单内容实时抓取，也没有传 `after` 翻页。Karma 不能映射为粉丝数、播放量或单帖互动量。

### YouTube

TikHub 可提供完整替代路径：

```http
GET /api/v1/youtube/web/get_channel_id_v2?channel_url=...
GET /api/v1/youtube/web/get_channel_info?channel_id=...
GET /api/v1/youtube/web/get_channel_videos_v2
GET /api/v1/youtube/web/get_video_info
GET /api/v1/youtube/web_v2/get_video_info_v2
```

- `get_channel_id_v2` 支持 `@handle`、`/channel/`、`/c/`、`/user/` URL，返回标准 channel ID、规范 URL 和解析来源。[频道 URL 转 ID](https://docs.tikhub.io/413417981e0)
- 频道资料必填 `channel_id`。[频道资料](https://docs.tikhub.io/413417983e0)
- 视频列表的 `channel_id` 也可传 `@handle`；`sortBy=newest|oldest|mostPopular`，`contentType=videos|shorts|live`，用 `nextToken` 翻页；返回视频 ID、标题、缩略图、观看、点赞、评论和时长。[频道视频](https://docs.tikhub.io/413417985e0)
- V1 单视频必填 `video_id`，元数据和下载信息较全，价格 0.002 USD/请求。[视频详情 V1](https://docs.tikhub.io/413417969e0)
- Web V2 `get_video_info_v2` 接受 `video_id` 或 `video_url`，`need_format=true` 时返回稳定的核心字段；官方说明适合作为 V1 标题/作者为空时的 fallback。[视频详情 V2](https://docs.tikhub.io/461775522e0)

仓库已用 Google YouTube Data API 完成频道资料、上传列表和单视频同步，并非当前缺口。除非要统一计费方或为 Google API 失败增加 fallback，否则无需优先迁移到 TikHub。

### Facebook、Website、播客、电视、报刊

- Facebook：当前官方总目录没有 Facebook 分组，生产 OpenAPI 没有 `/api/v1/facebook/...`。仓库返回“TikHub 当前公开接口未提供 Facebook 账号及帖子数据”是正确行为。[官方目录](https://docs.tikhub.io/)、[生产 OpenAPI](https://api.tikhub.io/openapi.json)
- Website：TikHub 没有通用网站受众/流量接口；仓库现有 Similarweb/网页元数据实现应保留，不应伪装成 TikHub 数据。[生产 OpenAPI](https://api.tikhub.io/openapi.json)
- 播客：没有通用播客平台资料、节目列表或单集接口。网易云音乐等特定服务不能覆盖任意播客。[官方目录](https://docs.tikhub.io/)
- 电视、报刊：没有通用电视台、报刊媒体资料或内容抓取接口。今日头条是独立内容平台，不能作为报刊通用数据源。[官方目录](https://docs.tikhub.io/)

这五类平台应允许模板导入和人工维护，但抓取控制必须显示“不支持/无数据源”，不能以空成功状态误导用户。

## 与仓库现状的实现优先级

### P0：完成小红书闭环

当前工作区已新增 `xiaohongshu_sync.go`、平台别名、模板选项、迁移以及内容链接分支，已调用 App V2 用户资料、用户笔记列表和图文详情。仍需补齐：

1. 内容链接同步：图文详情响应为 `video` 且业务需要播放地址时，再请求 `get_video_note_detail`；当前只请求图文详情。
2. 分页、去重、限量：当前用户笔记只请求 `cursor=""` 的第一页；应按 cursor 拉取到 `post_limit`，跨页按 `note_id` 去重，并遵守全局 QPS。
3. 业务失败判定：显式识别外层成功但 `data` 内“服务异常”的响应，避免把无效 ID 记为同步成功。
4. 回归覆盖：增加短链主页、图文笔记、视频笔记、空/服务异常数据、cursor 翻页和重复笔记测试。

### P1：已有平台的确定性缺口

1. LinkedIn 单帖子/文章 URL：接 `get_post_detail`；个人和公司作品列表对瞬时 400 最多重试 3 次。
2. Reddit 单帖 URL：本地解析 `/comments/<id>`，补 `t3_` 后调用 `fetch_post_details`。
3. TikTok 列表把 `count=20` 改为官方最大 15；需要 15 条以上时用 cursor 翻页并去重。
4. TikHub 请求超时统一到官方建议的 30–60 秒；目前 TikTok/Instagram 使用 15 秒。

### P2：质量与演进

1. Instagram 评估从 V1 升级 V3，获得明确分页契约和直接 URL 详情；需配套回归现有归一化字段。
2. X、LinkedIn、Reddit 的作品列表按业务 `post_limit` 补齐分页，而不是固定只取首页。
3. YouTube 保持 Google API 为主；如真实运行中遇到配额或网络问题，再增加 TikHub fallback。

### 明确不做

- 不用 TikHub “模拟实现” Facebook、播客、电视、报刊或通用 Website 抓取。
- 不用 Karma、点赞、粉丝等可见指标推算并伪装成播放量/曝光量。
- 不把 TikHub 返回外层 HTTP 200 等同于业务成功。
