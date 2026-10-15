# IP AI 推荐工作台

用户选择的视觉参考：`design/ip-prototypes/ai-recommendation-workspace-reference.png`。参考图用于内容区的对话、合作工作区结构和荧光绿主操作；用户后续调整为统一白色容器，左右以细线分区。页面必须嵌入现有系统框架，复用全局菜单、顶部栏、页签和页脚；不得复制参考图的独立导航或用全屏固定定位覆盖框架。生产图片来自现有 IP 资源资料；`design/ip-prototypes/assets` 下的生成图片只用于示例预览。

项目会话属于原系统左侧菜单下的普通列表，必须保持黑白灰，不使用彩色高亮、卡片底色、圆角或装饰色条。项目名称单行展示，超长省略并可悬停查看全名；当前会话仅通过文字加粗和明暗区分，研究阶段统一在主内容进度栏展示。AI 推荐页的原菜单选中项也不使用绿色底块。保留会话图标与新建入口。侧栏收起时可点击会话图标展开；手机、顶部菜单布局或隐藏侧栏时保留内容区选择器。手机从菜单选择后自动收起侧栏。新建会话会清除旧的会话和结果路由参数。布局根据内容容器宽度切换两列或单列，兼容原系统侧栏展开、收起及手机布局。主菜单 Infinix 字标以 SVG URL 导入，避免 SVG 组件对象被用于图片地址。

页面与菜单共享同一份 Pinia 会话状态，列表在内存中缓存 60 秒，并合并同一登录会话的并发请求。切换项目只加载目标项目的候选结果，不刷新菜单列表；离开推荐页仅释放临时操作状态，返回时复用有效缓存。过期刷新期间保留现有列表，失败时可重试。缓存按用户名和登录会话有效期隔离，退出登录立即清空；完整刷新浏览器后重新读取服务端。无需新增 Redis，也不把对话历史写入浏览器持久存储。数据库中的多轮记忆与持久化方式保持不变。

原内容容器仅对推荐页的 `subscriptionId/resultId` 切换保持组件实例，其他路由以及 `requestId/selectionKey/selected` 等提需上下文仍使用各自页面实例。路由监听统一加载候选，避免选择事件和页面重建产生重复请求。新增、改名、删除和当前阶段由已有会话数据同步；生成、读取资料或保存期间禁用菜单切换。

会话列表只在 AI 推荐页显示。每行右侧三点在悬停、键盘聚焦或菜单展开时出现，触屏始终展示 44px 操作入口；点击展开“删除会话”。删除确认明确显示项目名及不可恢复的对话、推荐历史范围。成功后立即移除本地列表项；删除当前项目切换到剩余项目，没有剩余项目时展示新研究，删除其他项目则保留当前内容。

每周检查是已有真实后台功能：服务在北京时间周一 09:00 后检查已开启订阅的库内新匹配，当周运行记录去重，并将新匹配写入系统通知；暂停订阅不执行周检。界面明确标为“每周检查库内新匹配”，与用户主动发起的库外联网搜索区分。此说明基于调度与通知实现及测试，不表示本次已观察到线上定时任务运行。

桌面两栏以较长内容决定共同高度，顶部与底部对齐。对话正文与输入区分组，输入区贴底；合作工作区的初步意向与确认操作也贴底。布局不限制正文高度或截断长回答，窄屏单列时各区域按内容自然展开。

两栏共享一个白色容器与外边框，桌面使用垂直分隔线，窄屏改用水平分隔线。资料说明位于输入框上方，使输入框和右侧确认操作底边精确对齐；输入框用细边框区分于白色内容背景。

## 实际回答如何对应界面

模型返回结构化 JSON，由前端排版。提示词不负责生成 HTML 或界面代码。入口为 `POST /business/recommendations/ip/conversation`，继续使用当前推荐模型配置和现有候选检索、AI 排序流程。

| 字段 | 界面位置 |
| --- | --- |
| `presentation.headline` | 主要判断标题 |
| `presentation.explanation` | 判断解释 |
| `presentation.basis[].label/value` | 产品适配、合作延展、区域条件等依据 |
| `presentation.adjustments` | 本轮相对已保存条件的实际变化，由服务端计算 |
| `presentation.questions` | 本轮待确认的问题 |
| `presentation.suggestedPrompts` | 可继续追问的快捷操作 |
| `primaryResourceId / alternativeResourceId` | 库内候选资料入口与主候选选择 |
| 候选 `assessment.fit / entryPoints / pending` | 对比表的需求契合、合作切入、待确认 |

候选判断由 AI 排序返回的 `assessment` 生成，包括 `verdict`、`fit`、`basis`、`entryPoints`、`pending`。生成新推荐后以实际排序结果生成左侧结论，不沿用检索前候选的旧判断。比较和解释操作使用已存在的真实候选与历史对话。

## 四个阶段

1. **需求理解**：描述需求或添加资料，新会话使用 `clarify` 理解需求。
2. **条件确认**：检查市场、产品、预算、偏好和排除类型，可以继续对话或编辑条件。确认后执行 `recommend`。
3. **候选研究**：查看真实 IP 档案、授权资料、逐个合作方向；继续偏好调整会更新寻找条件并重新推荐。
4. **比较决策**：选择 2 至 3 个当前候选执行 `compare`。`discuss` 用于解释推荐依据；比较和解释保持已有寻找条件。意向可选择最多 5 个 IP，确认后回填原提需草稿或进入新提需。

## 内容与持久化约束

- 模型接收已保存条件、最近 12 条历史消息及最多 8 个当前候选；候选对比 ID 在调用模型前校验。资源资料和上传正文作为资料处理，不具有系统指令权限。
- 只使用当前用户的 IP 寻找会话。模型返回的候选 ID 必须存在于当前候选集合中。不得编造授权、报价、保证销量或成功率。
- 明确排除的 IP 类型参与数据库检索及实时评分校验，避免把“不要游戏”推断成寻找游戏。规则降级保留已确认市场及预算。
- 会话阶段和最近 24 条消息保存在已有 `filters` JSON 中；结构化候选建议使用已有 `advice` 字段的版本化封装。旧文本建议仍可读，无新增数据库迁移。
- 模型不可用时返回并标识规则整理/判断，用户可手动编辑条件继续寻找。网络失败保留输入，重试沿用失败操作。
- 上传支持 PDF、TXT、Markdown、CSV、XLSX、XLS，最大 15MB。PDF 提取最多 12 页，表格最多 3 个工作表，正文最多取 3800 字进入当前推荐请求；扫描 PDF 无可提取文本时明确提示。文件本身不上传到资源库。
- 预览中的 ORBIT DESIGN、MOON STUDIO 与光域艺术计划为示例内容，不写入真实资源库。实际界面显示数据库候选与资料。

## 流式调用

协议参考：[TranAI 接入文档](https://transsioner.feishu.cn/wiki/CQmwwlIh5ibGclkKDjPcKwI3nfh)，读取版本 3616。模型沿用业务环境中配置的 Base URL、模型名和服务端密钥，使用 ChatCompletion `stream: true`，读取 `choices[0].delta.content`；密钥不发送到浏览器。文档中的使用人工号、姓名和部门请求头为可选，本次沿用现有鉴权头。

- 前端调用原接口 `POST /business/recommendations/ip/conversation` 并发送 `stream: true`，保留普通 JSON 请求的兼容性。请求使用 Axios fetch adapter 和 ReadableStream，复用登录与 token 刷新流程。
- 后端先校验权限、会话归属及比较候选，再开启 `text/event-stream`。`status` 表示实际处理阶段，`presentation` 为经过校验的结构化快照；标题和解释随模型片段逐步更新，其他字段完整后显示。前端不显示原始 JSON 或模型推理过程。
- `session` 提前提供已保存的会话 ID，断流重试复用该会话。`done` 只在完整回答保存成功后发送，并携带原有的会话、候选和最终判断；`error` 表示处理或保存失败。断流不能被当作成功，输入与已收到的片段保留供重试。
- 新推荐先理解条件并检索、校验候选，再根据本轮实际候选流式生成最终结论，避免提前展示基于旧候选的判断。候选排序继续使用现有流程；模型不可用时明确显示规则结果，不用延时动画模拟流式输出。
- 支持停止生成与离开页面取消，取消传递到上游 HTTP 请求和数据库调用；取消前已保存的条件与候选运行仍可能保留，未完成的助手回答不会写入历史。
- 设置即时 Flush、10 秒心跳、`Cache-Control: no-cache, no-transform` 和 `X-Accel-Buffering: no`；请求日志包装器支持解包刷新与正确记录最终成功或错误。流式前端总超时 420 秒，单次上游调用遵守模型配置且不超过 120 秒。

部署时需确保外层代理对该路径不缓存 SSE，且允许长连接；应用返回的禁用缓冲响应头需要代理保留。

## 库外联网搜索

2026-10-15 升级为具体候选研究，完整约定见 [库外 IP 推荐实现约定](ip-external-recommendation-design.md)。原接口继续使用 `action: "search"`，输入最多 200 字。模糊请求先确认市场、合作用途和 IP 方向，每轮最多两个问题；预算可以待定，被追问后可明确选择“先探索”。已有公开研究条件沿用，不默认项目产品或合作形式。

检索词由规范化公开方向、市场、用途及已验证 IP 名称构造，不外发预算、产品型号、附件或普通对话历史。最多 3 轮、6 次基础搜索，研究总时限 60 秒；Tavily 可补读最多 5 个已返回页面。模型提取候选后，由服务端验证名称、类型、身份原文和来源下标。不能通过校验的网页不作为推荐展示，默认约 5 个、证据不足时少给。

右侧“库外”按候选组织身份、理由、合作切入、待核实事项和来源，支持 2–3 个候选对比与逐项补查。核实入库后才能进入正式意向或提需回填。历史网页记录仍可查看，网页条数不再充当候选数。研究条件和候选保存在原会话 JSON，支持刷新与多轮恢复，不改动库内结果。

检索页按用户、服务配置、密钥散列和公开关键词缓存 15 分钟，最多 512 项，失败不缓存；正文补读与模型核验仍会执行。SSE 状态对应实际阶段，超时或阶段失败保留已核实部分与缺口，用户取消不保存未完成回答，配置错误直接提示。

用户已选 Tavily，并更正配置位置为 `config.yaml`。实际文件 `backend/config.yaml` 已预留以下内容；可提交模板为 `backend/config.example.yaml`，真实文件仍由 Git 忽略。

```yaml
web_search:
  provider: tavily
  tavily_api_key: ""
```

只需填写 `web_search.tavily_api_key`。服务端搜索每次从应用当前 Config 读取设置；运行新版本后，保存 YAML 沿用现有热加载，密钥轮换会改变缓存标识。若通过 `CONFIG_FILE` 指定其他 YAML，则应编辑实际加载的文件。本地调试服务当前使用 `/tmp/kol-admin-ui-debug-config.yaml`，该文件同样预留空密钥块；默认部署使用 `backend/config.yaml`。Viper 标准 `WEB_SEARCH_PROVIDER` / `WEB_SEARCH_TAVILY_API_KEY` 部署环境变量可覆盖 YAML。

以下旧环境变量仅作 YAML 对应值为空时的兼容回退；env 修改需重启进程。已配置 YAML 时无需 `.env`，旧 dotenv 加载与 `ENV_FILE` 保留兼容，不再作为用户的推荐配置入口。

| 服务 | 配置 |
| --- | --- |
| Tavily | `IP_WEB_SEARCH_PROVIDER=tavily`、`TAVILY_API_KEY` |
| SearXNG | `IP_WEB_SEARCH_PROVIDER=searxng`、`SEARXNG_BASE_URL`，实例必须开放 JSON 格式 |
| Brave | `IP_WEB_SEARCH_PROVIDER=brave`、`BRAVE_SEARCH_API_KEY`、已确认存储权益时设置 `BRAVE_SEARCH_STORAGE_ALLOWED=1` |

未指定 provider 或指定 `auto` 时，仅按已经配置的 Tavily、SearXNG、Brave 顺序选择一次，不在失败后自动切换服务或启用收费计划。Tavily 固定 `basic`，关闭自动参数、搜索接口完整正文及额外回答。结果 URL 使用既有 HTTPS 来源校验；正文补读仅交给固定的 Tavily 端点，应用不直接访问任意结果主机。搜索请求超时 15 秒，补读超时 10 秒，均不跟随重定向。

### 用户指定的参考来源

使用 [Tavily 的优先域名参数](https://docs.tavily.com/documentation/api-reference/endpoint/search) `include_domains` 与 `include_domains_mode: "prefer"`，按 IP 方向选取相关参考站点，保留其他相关网页。实际资料按相关性组织，P0/P1/P2 不代替候选准入，也不强制优先排序。WIPO 新闻不再标成商标数据库，WIPO Lex 不参与候选发现。用户的“第二优先级”榜单类以 P2 标识；优先级不代表真实性评级或合作可用性。

| 层级 | 机构与官方入口 | 证据用途 |
| --- | --- | --- |
| P0 | [Licensing Expo Brand Directory](https://www.brandlicensingexpo.com/exhibitor-list/brand-directory/) | 品牌、授权方与参展主体线索；保留旧 `licensingexpo.com` 域名匹配 |
| P0 | [License Global Rankings & Lists](https://www.licenseglobal.com/licensing-resources/rankings-lists) | 授权企业与行业榜单；原 Top 150 链接跳转至此目录 |
| P0 | [Licensing International Survey](https://licensinginternational.org/get-survey/) | 行业分类、规模与趋势；不证明具体 IP 可授权，完整报告有会员或付费要求 |
| P1 | [Brand Licensing Europe](https://www.brandlicensing.eu/) | 欧洲品牌、授权机构与合作主体发现 |
| P1 | [WIPO Global Brand Database](https://www.wipo.int/zh/web/global-brand-database) | 商标主体与注册地域核查线索；不推断已取得 IP 合作授权 |
| P2 | [Kantar BrandZ](https://www.kantar.com/Campaigns/BrandZ/Global) | 品牌估值与影响力背景，不作为授权报价 |
| P2 | [YouGov Rankings](https://yougov.com/reports/rankings) | 消费者认知与市场差异；用户提供的 [2025 报告](https://yougov.com/reports/51444-yougov-best-global-brands-2025) 保留为历史参考 |
| P2 | [SportsPro 50MM](https://50mm.sportspro.com/) | 运动员商业价值背景；兼容榜单跳转到 `sportspro.com` |
| P2 | [Parrot Analytics](https://www.parrotanalytics.com/) | 公开影视需求与地区影响力资料 |
| P2 | [Billboard Charts](https://www.billboard.com/charts/) / [Luminate](https://luminatedata.com/) | 公开音乐榜单、消费研究与趋势 |
| P2 | [Esports Charts](https://escharts.com/) | 公开赛事、战队和观众数据 |

2026-10-15 核对来源入口，实际访问记录只用于修正目录和分类，不写入业务会话证据。Billboard Charts 此次公开读取返回 402，未读取其榜单正文；Licensing International 明示完整报告受会员或付费访问限制。模型只使用供应商实际返回的摘要或补读正文，不宣称读取付费报告、数据库完整注册记录或实时专有指标。

结果增加 `sourceName/sourcePriority/sourceCategory/evidenceUse`，供界面和模型区分机构、层级与用途；保留供应商返回的 `publishedAt`。界面使用普通文本展示，沿用白色容器和单色菜单。榜单年份、数据观察区间、网页发布/更新日期与检索时间分别看待，不将旧榜单称为最新。提示词同样约束后续多轮回答，不将行业规模、品牌价值、商标登记或影响力排名推断为授权、报价或效果保证。

2026-10-14 核对的免费方案：[Tavily](https://www.tavily.com/pricing) 每月 1,000 积分、无需信用卡；[Brave](https://brave.com/search/api/) 每月 $5 额度，Search 为 $5/1,000 请求，需要信用卡验证；[SearXNG](https://docs.searxng.org/) 为可自建的免费软件，服务器运维另计。原有 Wikidata 免费查询继续用于核实库内 IP 身份背景，不等同于广泛的库外网页搜索。

本地尚未配置搜索密钥或 SearXNG 地址；真实浏览器验证了入口与未配置提示，成功检索、来源过滤、缓存及持久化由模拟搜索供应商和真实本地数据库验证，未宣称已启用真实第三方搜索。

## 验证范围

详见根目录 `design-qa.md`。服务端测试覆盖排除条件、规则筛选、历史建议兼容、模型 JSON 契约、候选 ID 白名单和未登录访问。早期隔离 API 示例验证了四阶段、对比、追问、资料读取、错误重试和多 IP 意向回填，但未覆盖现有框架，因此不能作为框架适配的验收依据。修正后使用 Chrome 访问 `http://localhost:8848/#/business/ip/recommendations`，连接实际 Go 服务和本地 MySQL，验证登录、资源与推荐结果读取、原系统菜单切换、Logo 加载以及桌面和手机布局。

未连接线上模型。当前真实本地服务读取数据库中已有的 Mock 验收记录，结果明确标为规则合作建议。为恢复本地登录和数据读取，补齐已有迁移中的会话表、推荐建议字段和市场渗透率字段；未修改线上数据库。线上生成质量和模型供应商对 JSON 格式的支持需要在已配置的业务环境继续验收。
