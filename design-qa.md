# Design QA Records

## Global Resource Identity

- Source visual truth: `/var/folders/2b/z81n4myx7k765ws80j1fgvc00000gn/T/codex-clipboard-b9622cdd-b424-4847-a63a-e31f083c4193.png`
- Browser-rendered implementation: `/Users/rui.ma1/Documents/kol_admin/docs/global-resource-identity-fix-implementation.jpg`
- Focused comparison: `/Users/rui.ma1/Documents/kol_admin/docs/global-resource-identity-fix-comparison.png`
- Browser viewport: 950 × 782 CSS px, device scale factor 1
- Source pixels: 372 × 254; implementation pixels: 950 × 782
- Density normalization: the reported and implemented identity regions were cropped and aspect-fit into equal comparison panels.
- State: authenticated Chinese-language Global Resource Library, default unfiltered list, first resource row visible.

### Findings

- No actionable P0, P1, or P2 visual differences remain.
- The long homepage URL is replaced by the extracted account `@techreviewdaily`.
- The identity/domain/tier row now renders `达人 / 科技测评 / 腰部` without clipping or compressing the tier tag.
- Existing non-media values such as `KOL`, `YouTuber`, `IP`, and other creator variants are normalized to `达人`; media values are normalized to `媒体`.

### Required Fidelity Surfaces

- Fonts and typography: resource name retains the established weight and size; the account is a smaller muted secondary line.
- Spacing and layout rhythm: the identity column minimum width was increased and the three tags are non-shrinking, keeping the tier immediately after the domain.
- Colors and visual tokens: existing neutral text and semantic tag colors are preserved.
- Image quality: avatar and platform icon behavior is unchanged; no assets were replaced.
- Copy and content: the type vocabulary is limited to `达人` and `媒体`, and homepage URLs are converted to account handles or hostnames.
- Interaction and responsiveness: both filter options were exercised in-browser. `达人` returned 35 resources and `媒体` returned 13 resources; the default list was restored afterward.

### Comparison History

1. Initial state — P2: the raw homepage URL occupied the secondary line, `YouTuber` introduced an unsupported third classification, and the tier tag was visibly clipped.
2. Fix — added account extraction, two-type normalization, wider identity grid sizing, and non-shrinking tag layout.
3. Post-fix comparison — account, type, domain, and tier are all fully visible with no overlap or clipping.

### Implementation Checklist

- [x] Limit displayed and editable resource types to `达人` and `媒体`.
- [x] Make both type filters work across legacy stored values.
- [x] Extract social handles from homepage URLs.
- [x] Keep media hostnames readable when no social handle exists.
- [x] Place the tier tag after the domain without squeezing.
- [x] Verify default, creator-filtered, and media-filtered states in the browser.

Final result: passed

---

## 达人档案视觉验收

- 验收页面：http://localhost:8848/#/business/resources/profile?id=1268
- 参考图：`/Users/rui.ma1/.codex/generated_images/01a0c1b2-c66a-7091-9d87-a030209d061c/exec-596010dc-e359-4b1a-946e-bf8f1e52577e.png`
- 实现截图：`design-qa-assets/profile-page-implementation.jpg`
- 对比图：`design-qa-assets/profile-page-comparison.jpg`
- 参考图像素：1680 × 944；实现截图像素：950 × 782。
- CSS 视口：950 × 782，device scale factor 2；对比图将参考图等比缩放至 950 像素宽，不改变纵横比。
- 状态：独立达人档案页，TechSpurt，YouTube 平台，含一条合作明细。

## 覆盖范围

- 资源抬头：头像、名称、账号、资源类型、领域、市场、可跳转的平台图标、分级标签。
- 达人基础表现：平台切换、六项指标、周环比、播放量波动指数、联系方式。
- 达人合作表现：费用、曝光、互动、次数、平均曝光、波动指数、互动率、CPM、备注。
- 达人合作明细：八列明细、年月日发布日期、可跳转作品封面。

## 五项视觉核对

- 字体与层级：标题、指标名、数值和辅助说明均按参考图的紧凑层级缩小，无大字号或异常换行。
- 间距与节奏：独立页面固定六列基础指标、四列合作指标，卡片高度、间距、圆角和区块留白接近参考图。
- 颜色与视觉令牌：白色卡片、浅蓝背景、蓝色功能图标、绿色增长/稳定状态与参考图一致。
- 图片质量：头像、平台 Logo 和合作封面均使用真实资源，封面保持比例裁切且可点击。
- 文案与内容：三段标题、指标字段和八列合作明细完整；缺少周快照或足够样本时显示 `--`、`/` 或“待评估”，不伪造数据。

## 对比与迭代历史

1. 初版 P1：档案位于 Dialog 内，950 像素视口触发三列/两列响应式换行，卡片和文字比例明显大于参考图。
2. 修复：改为独立路由页面，缩小整体字号、间距、卡片高度和圆角，并恢复六列/四列信息密度。
3. 第二轮 P2：合作明细表总列宽超过视口，合作供应商列被裁切。
4. 修复：压缩八列宽度；最终截图中表头、封面和合作供应商完整可见。
5. 全图对比和密集区域（基础指标、合作指标、明细表）均已在 `profile-page-comparison.jpg` 中核对，无剩余 P0/P1/P2。

## 交互与质量

- 多平台达人已验证 YouTube/Instagram 标签切换，平台粉丝量和播放量随平台独立更新。
- 已验证从资源库进入独立档案页，以及“返回资源库”导航。
- 最近合作作品仅读取合作记录，封面和平台主页保留跳转能力。
- 浏览器控制台无 error/warning。
- P0/P1/P2 视觉或功能问题：无。

final result: passed

---

## IP运营原型实现验收

- Source visual truth: `/Users/rui.ma1/Documents/kol_admin/design/ip-prototypes/01-ip-resource-library-v2.png`, `01b-ip-detail-v2.png`, `02-ip-request-v2.png`, `03-ip-initial-feedback.png`, `04-marketing-opinion.png`, `05-add-ip-manual.png`, `06-add-ip-bulk.png`.
- Source pixels: 1671–1672 × 941. CSS viewport: 1672 × 941; implementation screenshot: 1672 × 941 JPEG, density 1.
- Implementation: `http://localhost:8850/`，后端 `http://localhost:8080/`。
- Implementation screenshot path: CUA browser capture in this task's tool output; the browser tool did not expose a persistent local file path.
- State: 已登录的管理员账号。当前数据库无 IP 和需求记录，因此列表是实际空状态，详情抽屉及反馈/营销详情无可打开的记录。

### 已完成的代码对照

- 资源库补齐缩略图、统计图标、价格列和右侧档案；详情页改为图文抬头及双栏资料区。
- 需求页补齐意向 IP 缩略图、详情跳转、库外推荐及授权预算；反馈和营销阶段拆成各自的重点页面。
- 新增 IP 保留单条与批量双入口，将选填的档案扩展字段折叠，优先呈现原型中的基础信息、授权及附件。

### 五项视觉核对

- 字体与排版：资源库、单条录入、批量导入和需求页的标题、表单标签、辅助文案与原型层级相近；沿用现有应用字体。
- 间距与布局：1672 × 941 下核对了四张原型与浏览器页面。资源库统计与筛选、单条录入双栏、批量导入三步、需求页双栏均正常。现有系统顶部页签增加了垂直占用，附件与意向 IP 的内容比原型更靠下，需要滚动查看。
- 色彩与视觉令牌：沿用现有黑色导航、白色卡片和荧光绿强调色，按钮与选中态一致。
- 图片与资源：IP 封面读取上传文件；空数据库无法核对原型中的封面裁切和抽屉资料图片。未把原型中的虚构内容写入业务库。
- 文案与内容：授权预算、参考权益价格、版权文件、历史结案、库外推荐、批量模板规则及两阶段菜单在浏览器中可见。空状态无异常换行。

### 全图与聚焦对比记录

1. 代码初检发现新增 IP 的受众字段位于商业与授权之前，导致原型首屏结构偏移；已移至附件之后的可展开区。
2. 浏览器对照发现资源库统计卡缺少图标、筛选项换行；已补图标并把完成度和更新时间移入高级筛选。同一视口复查后，统计与主要筛选恢复原型的一行结构。
3. 浏览器对照发现单条录入和需求页的选填字段把核心内容推至首屏以下；已折叠选填字段，压缩卡片和阶段条间距。同一视口复查后，需求页可见意向 IP 区域，单条录入首屏可见基础信息与商业授权。
4. 批量导入初版的下载区和上传区过于空；已加入模板字段清单与 Excel/PDF 双上传框，同一视口复查后对应原型的三步结构。
5. 参考图与实现截图在同一浏览器工具输出中逐张对照。重点放大核对了资源库统计/筛选、单条录入基础信息、批量导入模板与上传、需求摘要与意向 IP 区域。参考图中的样例记录与实际空数据状态不一致，因此没有把样例封面、抽屉、反馈卡和营销详情当作已验收。

### 交互与质量

- 已验证 IP 运营的四个子菜单、单条/批量切换、资源库高级筛选展开、需求页库入口链接地址和空数据状态。
- 模板下载按钮执行后，内置浏览器没有报告下载事件；需要在常规浏览器再验一次下载。
- 资源库、需求、反馈及营销页面的浏览器控制台无 error/warning。
- ESLint、Vue TypeScript、生产构建、Go 测试和 `git diff --check` 均通过。

### 下一步

1. 在隔离的测试数据中提供至少一条含封面/PDF/结案的 IP 和一条需求，核对资源抽屉、详情、IP 反馈与营销意见的有数据状态。
2. 用可保存截图的浏览器工具记录原型与实现的持久化对比图，补齐截图路径。
3. 在常规浏览器确认 Excel 模板下载与新标签页跳转。

final result: blocked

### 2026-10-08 Mock 上传回归

- 在本地数据库创建并清理了标记为“【自动验证】”的 IP；上传了封面 PNG、版权介绍 PDF 和历史结案 PDF。资源库列表、封面、参考价格、右侧档案抽屉及详情页均显示对应测试资料。
- 通过实际 HTTP 接口核对上传文件的字节和类型、无效 PDF 拒绝、文件删除；批量导入预览识别可导入/重复行，导入后关联版权和结案 PDF。`xlsx` 对模板字段进行了一次内存生成与解析回环。
- 有效 PDF 在内置浏览器的原生 iframe 中为空白；改用项目已有的 `vue-pdf-embed` 组件后，页面显示了测试 PDF 内容。Vue 类型检查和生产构建通过。
- 内置浏览器没有捕获文件选择器或下载事件，因此这两项仍需在常规浏览器走一次人工验收。测试记录和上传文件已清理，资源库恢复为 0 条。

---

## 达人内容数据视觉验收

- 验收页面：http://localhost:8848/#/business/resources/profile?id=1268
- 参考图：用户提供的三张达人信息、内容作品和内容分析截图。
- 参考图拼板：`design-qa-assets/profile-content-reference-board.jpg`
- 实现拼板：`design-qa-assets/profile-content-implementation-board.jpg`
- 同屏对比：`design-qa-assets/profile-content-comparison.jpg`
- 桌面验收视口：1440 × 1000 CSS px。
- 状态：TechSpurt / YouTube，展示 10 条真实同步作品及真实作品指标。

## 覆盖范围

- 达人信息：五项平台维度指标、联系方式、周环比提示，以及右上角平台切换角标。
- 内容数据：总曝光、点赞、评论、分享、收藏汇总；五列作品分栏；封面、发布日期、视频时长和五项指标。
- 内容分析：词云和按作品标签占比排序的 TOP5 内容主题标签。
- 数据边界：切换到无作品的平台时展示空状态，不混入其他平台作品；标签优先读取标题/描述中的 hashtag，无 hashtag 时才回退资源标签。

## 五项视觉核对

- 字体与层级：三段均使用一致的小标题层级，指标标签、数值和辅助说明保持紧凑。
- 间距与节奏：摘要横排、五列作品网格和双列分析区在 1440 像素视口完整对齐，无水平裁切。
- 颜色与视觉令牌：延续档案页白色卡片、浅蓝背景与蓝/橙/绿/紫/红指标色，未引入冲突主题。
- 图片质量：使用实际同步作品封面，统一 4:3 裁切，日期与时长叠加层保持可读。
- 文案与内容：字段与需求一致；无数据时统一显示 `-` 或空状态，不伪造指标与标签。

## 对比与迭代历史

1. 初版沿用独立档案页的紧凑比例，加入达人信息、内容数据和内容分析三段。
2. P2：平台无匹配作品时曾回退展示其他平台内容，会破坏平台维度口径。
3. 修复：严格按当前平台筛选；无匹配作品时显示空状态，内容分析同步为空。
4. 与参考图同屏复核后，卡片密度、平台切换位置、内容画廊和分析区均无剩余 P0/P1/P2。

## 交互与质量

- 点击作品卡可打开原作品链接；平台主页 Logo 保持可跳转。
- 平台切换同时更新达人信息、内容汇总、作品列表和内容分析。
- 浏览器控制台无 error/warning。
- ESLint、Vue TypeScript 检查和生产构建均通过。

final result: passed

---

# IP AI 推荐工作台设计验收 · 2026-10-14

历史记录：此轮通过仅针对隔离示例中的内容布局和交互，不满足用户后续明确的原系统框架要求。最终框架验收见文末修正记录。

## 比较对象与证据

- Source visual truth：`design/ip-prototypes/ai-recommendation-workspace-reference.png`，用户选定的 Image Gen 设计。
- Implementation：实际 `vue-pure-admin/src/views/business/ip/recommendations/index.vue`，隔离示例 API，`http://127.0.0.1:5173/`。
- 最终截图：`docs/qa/ip-ai-workspace-2026-10-14/ip-ai-workspace-desktop.jpg`。
- 状态：GT Opus 项目，候选研究阶段；工作区对比 2 个候选、意向 1 个，与参考图的主要状态一致。
- CSS viewport、参考图片和实现图片均为 **1499 × 1049**，1:1 像素比较，无缩放或密度转换。源图和最终截图在同一次图像比较输入中一起打开检查。
- 手机：**390 × 844**，`ip-ai-workspace-mobile.jpg` 与滚动后的 `ip-ai-workspace-mobile-workbench.jpg`。中等屏幕：**1024 × 900**，`ip-ai-workspace-tablet.jpg`。文件均位于上述 QA 目录。
- 完整图在 1:1 下可读：重点检查左侧标题、依据表、输入区以及右侧对比表、缩略图和确认按钮，无需额外缩放裁切。手机采用上下两张实际视口截图验证内部滚动区域。

## 已修复的发现与比较历史

1. **[P2] 首屏的主要操作被纵向间距挤出。** 初版截图 `ip-ai-workspace-desktop-before.jpg` 的合作区从 y≈296 延伸至 y≈1268；参考图从 y≈246 至 y≈1027。首屏看不到确认意向按钮。修复：侧栏改为 207px，压缩阶段栏、条件栏及重复留白，把 AI 比较操作移入工作区工具栏，合并确认区排版；移除对比表中的重复通用待确认说明，保留全局版权核对提示。最终截图的确认按钮和完整输入区均在首屏内，结论与对比图保持左右主次布局。初版与参考、最终与参考均在同一图像输入中进行了比较。
2. **[P2] 手机“新建会话”挤成竖排。** 证据 `ip-ai-workspace-mobile-before.jpg`。修复：降低继承字体规则的选择器优先级，文字包裹并在窄屏隐藏；保留按钮可访问名称，增加窄屏项目会话选择器。最终手机截图侧栏只显示图标，无竖排文字或水平溢出。
3. **[P2] 候选比较中出现重复的授权提示，增加密度。** 初版对比表同时包含候选待确认内容及完整通用声明。修复：表格呈现候选特有待确认事项，全局说明承担通用声明。最终两列均可读，手机也可完整读取。
4. 修复样式之后再次采集最终桌面截图；检查样式格式化未改变布局。新浏览器标签页控制台无错误或警告。早期示例 API 缺少常量导出导致的启动错误已在预览配置中修正。

## 五个视觉面

| 维度 | 结果 |
| --- | --- |
| 字体与层级 | 系统可用的 Helvetica Neue / PingFang SC 回退字体；46px 页面标题、最多 36px 结论标题、24px 工作区标题。保留大结论与紧凑依据的层级，较参考图部分正文更紧凑，为 P3。无截断主要判断。 |
| 留白与布局 | 207px 深色侧栏；左右研究/合作区间隔 24px；暖白主底、浅石色工作区。主区比例与参考接近，两个主要操作在桌面首屏内。390、1024、1499px 均无横向页面溢出。 |
| 色彩与状态 | 深色 `#18201c`、主底 `#fafaf7`、工作区 `#f0f2ec`、主操作 `#caff00`。高亮只用于当前阶段、已选意向和主要操作；按键有焦点样式及加载禁用状态。 |
| 图片与图标 | 示例资产均为单独生成的栅格图，沿用暖灰金属雕塑及冷色沉浸空间方向，清晰无拉伸。生产使用库内视觉资料，缺失时明确显示暂无视觉资料。图标使用 Remix Icon；Infinix 字标使用其官网现有 SVG 资产，保留实际字标而非生成图中的斜体近似。 |
| 内容 | 结论、依据、本轮调整、候选契合/合作切入/待确认与后续追问都由结构化字段或实际候选映射。已标识本地示例数据，移除不存在的用户身份和示例 PDF 资料入口；实际资料缺失不会伪造文件。 |

字标来源：[Infinix 官网](https://id.infinixmobility.com/)。图片资产放在 `design/ip-prototypes/assets/`，不作为生产候选的占位内容。

## 实际交互与代码验证

- 新建会话 → 描述需求 → 条件确认 → 确认并推荐 → 候选 → AI 对比 → 解释追问，阶段、消息及历史入口均更新。
- 候选资料抽屉可打开和关闭；对比支持当前已选候选，初步意向支持移除。
- PDF 正文解析并传入示例 API（83 字符，含文件名），XLSX 内容解析并传入（94 字符）。模型/网络失败提示、保留原输入、重试及后续恢复已验证。
- 两个 IP 确认意向后进入实际提需页，两个 ID 和选择理由均回填。证据：`ip-ai-intent-handoff.jpg`，并读取 DOM 表单值确认理由。
- 空白会话显示需求提示和空候选区；未生成候选时比较阶段禁用。加载期间输入和重复请求按钮禁用。
- 手机首屏、向下滚动后的对比表和确认区均检查；中等屏幕测量主容器宽度等于滚动宽度。最终交付标签页控制台 `error/warn` 列表为空。
- `go test ./cmd/server`、Vue 类型检查、ESLint、Stylelint、生产 Vite 构建、`git diff --check` 均通过。
- Impeccable 扫描只报告通用字体警告；本次依据选定参考采用既有系统无衬线字体，作为已评估的 P3 保留。

## 保留差异与测试范围

无未解决的 P0/P1/P2。参考为生成式设计，因此文字、字标与图像并非可直接用于实际业务的完整资产集：官方字标、真实资源图片、真实文件入口和可执行的 AI 比较操作为明确的产品约束。结论自动换行和较小正文是剩余 P3。

浏览器验证使用隔离示例 API，模型契约测试使用本地 HTTP 模型模拟，未调用线上模型或写业务数据库；视觉通过不等同于线上 AI 推荐质量已验收。PDF 扫描件需用户提供可提取的文本，未加入 OCR。

## 实现检查清单

- [x] 对应选定视觉方向，并在相同视口/状态比较。
- [x] 主要阶段、表单、对话、候选比较与意向回填可操作。
- [x] 模型 JSON 契约对应前端呈现，已有建议文本兼容。
- [x] 桌面、手机、中等屏幕和主要错误/空状态通过。
- [x] 保存参考图、最终截图、验证记录。

---

# IP AI 推荐原系统框架修正 · 2026-10-14

final result: passed（框架适配与真实本地服务验证）

- 视觉与结构依据：用户明确要求在原框架内实现；Chrome 检查线上现有菜单、顶部栏及页签。生成参考图只约束内容区。
- 验收入口：`http://localhost:8848/#/business/ip/recommendations`。实际 Vite 前端连接本地 Go 服务 `127.0.0.1:8082` 和原生 MySQL，使用正常登录及数据库内已有的 Mock 验收会话与两个候选；未通过独立 HTML 页面或隔离 API 验收。
- 修正：移除重复 IP 导航、内部面包屑和全屏固定定位；会话选择与新建操作移入内容区；内容按容器宽度适配；新建清理旧会话路由参数；主菜单 Logo 使用 `svg?url`，实际图片加载成功。
- 桌面 1512 × 771：一个全局侧栏、零个重复侧栏；推荐页面定位为 `relative`；页面滚动宽度 1512，无水平溢出。菜单、顶部栏和页签完整可见。
- 手机 390 × 844：原系统收起侧栏；推荐内容单列，页面滚动宽度 390。检查首屏及向下滚动后的候选、比较、初步意向操作与页脚。测试后恢复桌面视口。
- 证据：`docs/qa/ip-ai-original-framework-2026-10-14-desktop.jpg`、`docs/qa/ip-ai-original-framework-2026-10-14-mobile.jpg`、`docs/qa/ip-ai-original-framework-2026-10-14-mobile-workbench.jpg`。
- 真实服务问题：登录因缺少 `sys_sessions` 表失败；资源接口因缺少 `market_penetrations` 字段返回 500；推荐结果接口因缺少 `advice` 等建议字段返回 500。仅在本地执行已有迁移 051 的建表语句、049 和 053 的增列语句，未执行删除语句。复验登录成功，资源列表返回 200 并显示两个已有记录，推荐结果正常显示两个候选。
- 验证：Vue 类型检查、ESLint、Stylelint、Vite 生产构建通过。保持原有结构化回答、候选对比和意向功能。
- 范围：未调用线上模型，当前本地候选缺少图片时显示“暂无视觉资料”，旧建议显示“规则合作建议”；这次验收不代表线上 AI 推荐质量通过。未部署线上服务。

## 左右等高与操作区对齐 · 2026-10-14

- 用户补充要求：内容区更接近已选视觉图，左右两栏高度对齐；继续复用原系统框架。
- 发现：网格使用 `align-items: start`，真实本地候选状态右栏比左栏长约 324px，输入区提前结束。
- 修正：网格拉伸两栏；对话正文和输入区分组，输入区利用弹性留白贴底；工作区使用纵向弹性布局，意向确认区贴底。高度随内容增长，不限制或裁切回答和候选。
- Chrome / 真实 8848 服务：候选状态两栏同高 881.55px；对比状态同高 700.64px，上下坐标完全一致；输入框底部与右侧操作区域底部相差 3.6px（各自说明与内边距形成的光学差异）。空白会话两栏同高 527.78px。
- 桌面截图视口 1512 × 1140，保留原框架并完整显示底部操作：`docs/qa/ip-ai-aligned-columns-2026-10-14-desktop.jpg`。手机 390 × 844 下为 326px 单列，两栏恢复各自自然高度，滚动宽度 390：`docs/qa/ip-ai-aligned-columns-2026-10-14-mobile.jpg`。
- 机械布局扫描无发现；Vue 类型检查、ESLint、Stylelint、Vite 生产构建通过。测试后恢复原浏览器视口，未调用模型或写业务记录。

## 统一白色容器 · 2026-10-14

- 用户调整：左右两栏放入同一个框，背景使用白色。
- 实现：两栏共享白色底、1px 外框和 12px 圆角；取消右侧独立底色与圆角，桌面用 1px 竖线分区，单列使用横线。统一两侧内边距；输入框改为细边框，资料提示放到输入框上方。
- 真实 8848 服务桌面 1512 × 1140：容器背景 `rgb(255,255,255)`，左右区域背景透明；两栏高度均为 700.64px；输入框与工作区底部均为 y=1074.03，精确对齐。证据：`docs/qa/ip-ai-white-workspace-2026-10-14-desktop.jpg`。
- 窄屏 390 × 844：单列宽 324px，页面宽 390，无横向溢出；合作区上边框 1px、左边框 0。后续截图、刷新和视口恢复受到 Chrome 已打开的扩展面板阻挡，未保存手机截图；需在扩展面板关闭后恢复视口。
- Vue 类型检查、ESLint、Stylelint、设计扫描、Vite 生产构建和 `git diff --check` 通过；未调用模型或写业务记录。

## IP 推荐流式调用 · 2026-10-14

- 依据：用户指定的 TranAI 飞书文档（版本 3616），ChatCompletion 流式请求与增量内容格式。
- 后端：真实上游流式消费；校验后的标题/解释逐步呈现；最终结构化回答保存后结束。保留候选 ID 白名单、权限、排除条件与规则降级。新增即时刷新、心跳、取消传播和日志包装器刷新支持。
- 前端：原框架和白色等高容器中更新回答，显示真实处理阶段；支持停止生成、错误重试、保存后会话 ID 复用及离开页面取消。Axios fetch adapter 沿用现有登录与刷新机制。
- 单元验证：模型尚未结束时已收到标题片段；转义、Unicode 与中文分片正确；模型断流/错误/截断均失败；取消关闭上游；日志包装器未缓存首个事件且最终审计结果正确。前端六项测试覆盖提前展示、逐字节 UTF-8、CRLF/心跳/多行事件、断流、保存失败、取消及会话复用。
- Chrome 真实服务：在 `http://localhost:8848/#/business/ip/recommendations` 的已有 Mock 验收会话点击追问，原接口返回 HTTP 200 / `text/event-stream`，完成后更新用户消息、规则回答和历史条数。证据：`docs/qa/ip-ai-streaming-2026-10-14.jpg`。未创建独立预览页，测试后恢复原视口。
- 范围：本地调试配置未载入模型密钥，数据库中的模型名和网关地址为空；真实浏览器验证为规则降级，模型流式输出用本地 HTTP 模拟服务验证，未调用线上网关。当前未修改线上配置或部署。
- 代码验证：后端完整测试与流式相关竞态测试、前端六项流解析测试、Vue 类型检查、ESLint、Stylelint、设计扫描、Vite 生产构建及 `git diff --check` 全部通过。

## 原菜单项目会话列表 · 2026-10-14

- 用户要求：参考图中的项目会话更明显，放到左侧菜单中。
- 实现：直接在原系统 `NavVertical` 侧栏中增加项目会话分区，复用真实会话列表；显示名称、阶段、当前会话高亮与新建入口。未创建第二套侧栏或独立子页面。页面和菜单共用 Pinia 状态，卸载时清理，生成/读取资料/保存期间禁用会话操作。
- 真实 8848 / Chrome：仅一个 `.sidebar-container`，一个项目会话分区，当前项 `aria-current=true`；展开菜单时无重复的内容区选择器。通过菜单新建后显示空白研究；点击已有会话恢复消息、条件和候选，URL 更新为相应 `subscriptionId`。收起菜单后显示展开会话按钮与内容区选择器；点击按钮恢复完整列表。
- 页面边界：离开到 IP 资源库时项目会话分区消失，返回推荐页重新恢复真实列表。
- 手机 390 × 844：内容区选择器可用，无横向溢出；展开原菜单后能看到项目会话和新建入口，选择会话后自动收起遮罩。测试后恢复原桌面视口和展开菜单。
- 证据：`docs/qa/ip-ai-menu-sessions-2026-10-14-desktop.jpg`、`docs/qa/ip-ai-menu-sessions-2026-10-14-mobile.jpg`、`docs/qa/ip-ai-menu-sessions-2026-10-14-mobile-menu.jpg`。
- 范围：浏览器使用已有 Mock 验收会话，只做菜单与页面切换，未调用模型或创建业务记录。本次验证没有完整枚举所有菜单主题和顶部菜单布局；其降级入口由现有布局状态控制。
- 检查：Vue 类型检查、ESLint、Stylelint、设计扫描、Vite 生产构建及 `git diff --check` 通过。

## 项目会话菜单视觉简化 · 2026-10-14

- 用户追加要求：左侧不要有颜色，不要像卡片，会话属于菜单下面的普通列表。
- 修正：移除会话选中底色、绿色标记、圆角和阶段副标题；项目名称单行省略，完整名称通过悬停标题查看。当前会话仅加粗文字；新建入口与键盘焦点保持单色。AI 推荐页原菜单的绿色选中底块和收起时的彩色标记一并移除，样式按当前路由限定。
- Chrome / 真实 8848：原 IP 菜单展开，只有一个系统侧栏。会话背景透明、圆角 0px、左边框 0px、当前文字字重 600；原菜单选中伪元素为 `display: none`。没有新增业务记录或调用模型。
- 证据：`docs/qa/ip-ai-plain-menu-2026-10-14.jpg`。
- 检查：Vue 类型检查、ESLint、Stylelint、设计扫描、Vite 生产构建通过。本次仅修改展示样式，会话切换与多轮记忆继续复用原实现。
