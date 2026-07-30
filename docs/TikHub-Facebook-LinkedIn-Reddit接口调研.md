# TikHub Facebook、LinkedIn、Reddit 接口调研

调研日期：2026-07-30

## 结论

| 平台 | TikHub 官方核心 API 当前状态 | 账号搜索 | 用户/主页资料 | 帖子列表 | 接入结论 |
| --- | --- | --- | --- | --- | --- |
| Facebook | 当前官方接口目录与生产 OpenAPI 均未提供 Facebook 路由 | 无 | 无 | 无 | **暂时不能声称“从 TikHub 获取 Facebook 数据”** |
| LinkedIn | 已提供 LinkedIn Web V2 API | 有，按名和姓搜索 | 有，支持个人主页与公司主页 | 有，支持个人帖子与公司帖子 | 可接入，建议优先采用 V2 |
| Reddit | 已提供 Reddit APP API | 有，动态搜索的 `people` 类型 | 有 | 有 | 可接入 |

TikHub 的所有受保护接口都使用：

```http
Authorization: Bearer <TIKHUB_API_KEY>
```

基础地址为 `https://api.tikhub.io`。成功响应外层统一包含 `code`、`request_id`、`message`、`data` 等字段。官方 OpenAPI 将业务 `data` 定义为通用数据，没有提供稳定的细粒度响应 Schema，因此业务侧解析必须容忍字段缺失和层级变化，并保留原始响应用于排查。

## Facebook

截至调研日期：

- [TikHub 官方接口总目录](https://docs.tikhub.io/)中没有 Facebook API 分组。
- [TikHub 生产 OpenAPI](https://api.tikhub.io/openapi.json)中没有任何 `/api/v1/facebook/...` 路由。
- 文档中 LinkedIn 的 “Search ads (Ad Library)” 属于 LinkedIn API，不等同于 Facebook 页面、账号或帖子接口。

因此，Facebook 目前无法按本需求通过 TikHub 完成以下能力：

- 搜索 Facebook 达人或媒体主页；
- 读取主页头像、名称、粉丝数等资料；
- 读取主页帖子及互动数据。

### 接入建议

1. 本轮可以把 Facebook 平台选项和本地图标接入前端，但“全网搜索”不能伪造为 TikHub 数据。
2. 在 TikHub 发布正式 Facebook 路由前，服务端应明确返回“TikHub 暂未提供 Facebook 数据接口”，不要静默返回空搜索结果。
3. 若必须近期上线，需要另选 Meta Graph API 或其他有授权的数据服务；这属于新的数据源，不应复用 TikHub 的来源标识。

## LinkedIn

TikHub 同时存在旧版 `LinkedIn-Web-API` 文档和新版 `LinkedIn-Web-V2-API`。当前[生产 OpenAPI](https://api.tikhub.io/openapi.json)只列出 V2 路由，因此新代码建议优先接 V2。

### 1. 按姓名搜索个人账号

```http
GET /api/v1/linkedin/web_v2/search_users
```

请求参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `first_name` | 是 | 名，例如 `Bill` |
| `last_name` | 是 | 姓，例如 `Gates` |

用途：按名和姓搜索 LinkedIn 个人主页。

限制：V2 不是任意关键词搜索，两个参数都必填。产品若只提供一个搜索框，不能可靠地自动判断复姓、单名或昵称。可以在界面上拆成“名/姓”，或者要求用户直接输入主页 URL。

旧版文档还提供更灵活的[搜索用户/Search people](https://docs.tikhub.io/384533474e0)：

```http
GET /api/v1/linkedin/web/search_people
```

它支持 `name`、`first_name`、`last_name`、`title`、`company`、`school`、`page`、`geocode_location`、`current_company`、`profile_language`、`industry`、`service_category`。但该旧路由没有出现在当前生产 OpenAPI 中，正式接入前应使用实际 TikHub Token 做一次可用性验证，不建议把它作为唯一实现。

### 2. 获取个人主页资料

官方文档：[获取用户主页信息/Get user profile](https://docs.tikhub.io/452347395e0)

```http
GET /api/v1/linkedin/web_v2/get_user_profile?url=<profile-url-or-slug>
```

请求参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `url` | 是 | LinkedIn 个人主页 URL 或 slug，例如 `https://www.linkedin.com/in/williamhgates/` |

官方明确说明可返回的关键资料包括：

- 姓名；
- 当前公司；
- 简介；
- 所在地；
- 粉丝数；
- 工作经历；
- 教育经历。

部分字段会受目标账号隐私设置影响而缺失。官方简介没有承诺固定头像字段名，因此头像解析应兼容多种字段并允许为空，不能因为头像缺失而判定整条资料失败。

### 3. 获取个人帖子

官方文档：[获取用户帖子/Get user's posts](https://docs.tikhub.io/452347396e0)

```http
GET /api/v1/linkedin/web_v2/get_user_posts
```

请求参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `url` | 是 | LinkedIn 个人主页 URL 或 slug |
| `start_date` | 否 | ISO-8601 起始时间 |
| `end_date` | 否 | ISO-8601 结束时间 |

接口按主页 URL 获取个人帖子，并可按时间区间过滤。官方未公布稳定的帖子响应 Schema；业务侧至少应尝试归一化：

- 帖子 URL/ID；
- 文本；
- 发布时间；
- 图片或视频封面；
- 点赞、评论、转发等互动数。

若帖子列表只返回摘要，可再按帖子 URL 调用：

```http
GET /api/v1/linkedin/web_v2/get_post_detail?url=<post-or-article-url>
```

该接口用于获取单个 LinkedIn 帖子或文章详情。

### 4. 公司主页和公司帖子

媒体账号可能是 LinkedIn Company，而不是个人账号，需要同时支持两类 URL。

公司资料：

```http
GET /api/v1/linkedin/web_v2/get_company_profile?url=<company-url-or-slug>
```

官方文档：[获取公司主页资料/Get company profile](https://docs.tikhub.io/452347418e0)

公司帖子：

```http
GET /api/v1/linkedin/web_v2/get_company_posts
```

参数为：

- `url`：必填，公司主页 URL 或 slug；
- `start_date`：可选，ISO-8601 起始时间；
- `end_date`：可选，ISO-8601 结束时间。

官方文档：[获取公司帖子/Get company's posts](https://docs.tikhub.io/452347420e0)

### LinkedIn 业务字段映射建议

| 业务字段 | TikHub/LinkedIn 来源 |
| --- | --- |
| 名称 | 个人姓名或公司名称 |
| 头像 | 主页公开头像/公司 Logo；字段缺失时使用本地平台占位图 |
| 账号链接 | LinkedIn 主页 URL |
| 粉丝量 | 个人粉丝数或公司 followers，缺失时为未知，不以连接数强行替代 |
| 内容数量 | 本项目内已导入/同步的帖子数量 |
| 曝光量/播放量 | LinkedIn 公共数据未必返回，缺失时不能用点赞数代替 |
| 互动量 | 点赞 + 评论 + 转发等官方返回的互动指标之和 |
| 内容链接/缩略图 | 帖子 URL 和媒体字段 |

## Reddit

### 1. 搜索用户

官方文档：[获取 Reddit APP 动态搜索结果](https://docs.tikhub.io/369454687e0)

```http
GET /api/v1/reddit/app/fetch_dynamic_search
```

搜索用户时建议参数：

| 参数 | 值/说明 |
| --- | --- |
| `query` | 必填，搜索关键词 |
| `search_type` | 固定为 `people` |
| `safe_search` | 建议 `strict` 或按产品策略使用 `unset` |
| `allow_nsfw` | 建议固定 `0` |
| `after` | 下一页游标，首次为空字符串 |
| `need_format` | 可选；默认 `false` |

`people` 类型不支持 `sort` 和 `time_range`，不要在请求中传入这两个筛选条件。返回内容包含匹配用户列表和分页信息。

### 2. 获取用户资料

官方文档：[获取 Reddit APP 用户资料信息](https://docs.tikhub.io/369454690e0)

```http
GET /api/v1/reddit/app/fetch_user_profile
```

请求参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `username` | 是 | Reddit 用户名，不带 `u/` 前缀 |
| `need_format` | 否 | 是否清洗格式化数据，默认 `false` |

官方明确列出的关键返回信息包括：

- 用户名和 ID；
- 账号创建时间；
- 帖子 Karma 和评论 Karma；
- 头像和横幅图片；
- 个人简介；
- 是否验证账号；
- 徽章和奖励；
- 关注者数量。

### 3. 获取用户帖子

官方文档：[获取用户发布的帖子列表/Fetch User Posts](https://docs.tikhub.io/369454693e0)

```http
GET /api/v1/reddit/app/fetch_user_posts
```

请求参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `username` | 是 | Reddit 用户名 |
| `sort` | 否 | `NEW`、`TOP`、`HOT`、`CONTROVERSIAL`，默认 `NEW` |
| `after` | 否 | 下一页游标 |
| `need_format` | 否 | 是否清洗格式化数据，默认 `false` |

官方明确列出的关键返回信息包括：

- 帖子标题和正文；
- 发布时间；
- 所属 Subreddit；
- 点赞数和评论数；
- 帖子类型：文本、图片、视频或链接；
- 媒体内容；
- 分页信息。

### Reddit 业务字段映射建议

| 业务字段 | TikHub/Reddit 来源 |
| --- | --- |
| 名称 | Reddit username |
| 头像 | 用户资料的头像 |
| 账号链接 | 规范化为 `https://www.reddit.com/user/<username>/` |
| 粉丝量 | follower count；缺失时为未知 |
| 内容数量 | 本项目内已导入/同步的帖子数量 |
| 曝光量/播放量 | Reddit 接口未承诺提供，不能使用 Karma 或 upvotes 替代 |
| 互动量 | 帖子 upvotes + comments；如响应存在其他互动指标再累加 |
| 领域 | 可参考帖子所属 Subreddit 或活跃社区，但应作为推断字段标注来源 |
| 内容链接/缩略图 | 帖子 permalink 和媒体内容 |

Karma 是账号累计信誉值，不等于粉丝量、播放量或单帖互动量，数据库映射时应独立保留或放入原始指标 JSON。

## 实现注意事项

1. **保留远程素材 URL**：TikHub 返回的头像和帖子媒体 URL 应写入远程 URL 字段；即使下载了本地缓存，也不要覆盖远程来源。
2. **平台链接识别**：
   - LinkedIn 个人：`linkedin.com/in/...`
   - LinkedIn 公司：`linkedin.com/company/...`
   - Reddit：`reddit.com/user/...`、`reddit.com/u/...` 或裸用户名
3. **缺失指标保持未知**：LinkedIn、Reddit 不保证提供内容曝光/播放量，不能用互动量或粉丝量估算并当作真实值入库。
4. **响应兼容**：由于 TikHub OpenAPI 的 `data` 为通用对象，解析器应支持多候选路径、数字字符串转换、空值和数组嵌套变化。
5. **调用费用**：官方成功响应文案标明请求可能计费；上述接口文档没有统一稳定的单价说明，不应在代码中写死价格。
6. **安全搜索**：Reddit 用户搜索建议 `allow_nsfw=0`；是否使用 `safe_search=strict` 由产品策略决定。

## 官方来源

- [TikHub API 文档总目录](https://docs.tikhub.io/)
- [TikHub 生产 Swagger UI](https://api.tikhub.io/)
- [TikHub 生产 OpenAPI JSON](https://api.tikhub.io/openapi.json)
- [LinkedIn V2 用户资料](https://docs.tikhub.io/452347395e0)
- [LinkedIn V2 用户帖子](https://docs.tikhub.io/452347396e0)
- [LinkedIn V2 公司资料](https://docs.tikhub.io/452347418e0)
- [LinkedIn V2 公司帖子](https://docs.tikhub.io/452347420e0)
- [LinkedIn 旧版用户搜索](https://docs.tikhub.io/384533474e0)
- [Reddit 动态搜索](https://docs.tikhub.io/369454687e0)
- [Reddit 用户资料](https://docs.tikhub.io/369454690e0)
- [Reddit 用户帖子](https://docs.tikhub.io/369454693e0)
