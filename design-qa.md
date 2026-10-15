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

## 库外联网搜索 · 2026-10-14

- 在原框架与白色合作工作区中增加输入框联网模式和“库外”页签；结果以普通来源列表展示，保留单色左侧菜单，不新增独立子页面。
- Chrome / 真实 8848 与 Go 服务：搜索关键词输入、模式选中、禁用附件、库外空状态可用；无供应商配置时显示真实错误，保留输入并提供重试，未写入新业务会话消息。
- 手机 390 × 844：联网按钮和发送按钮正常排列，四个工作区页签可见；页面宽度与滚动宽度均为 390，无横向溢出。测试后恢复桌面视口。
- 证据：`docs/qa/ip-ai-web-search-desktop-2026-10-14.jpg`、`docs/qa/ip-ai-web-search-mobile-2026-10-14.jpg`。最后恢复会话的浏览器操作被 Chrome 扩展面板阻止；此前的搜索错误与手机布局检查已完成。
- 供应商模拟测试：Tavily basic 请求、Brave、SearXNG JSON；去重、HTML 清理、HTTPS 来源校验、额度与异常、取消、不转发密钥、模型不能伪造来源或资源 ID、空缓存数组兼容。
- 真实本地数据库验收：使用测试独占且清理的记录、模拟搜索供应商，验证同一会话保留两轮搜索来源，已有条件与候选不变，重复搜索命中缓存，失败不保存假回答；未使用第三方密钥或实际搜索服务。
- 检查：后端完整测试、相关竞态测试、本地数据库烟雾测试、前端六项流解析测试、Vue 类型检查、ESLint、Stylelint、设计扫描和 Vite 生产构建通过。

## Tavily env 与参考来源策略 · 2026-10-15

- 按用户要求预留后端 Tavily env：本地 `.env` 空密钥位置、Git 忽略规则、可提交的 `.env.example`；启动自动加载，部署环境变量优先，支持 `ENV_FILE`，异常消息不包含解析出的密钥。
- 参考目录：用户提供的授权机构、资源目录及影响力来源，补充官方跳转域名和稳定目录入口。Tavily 单次 basic 搜索使用优先域名模式，不逐站追加请求；实际返回资料按 P0/P1/P2/其他来源组织。
- 结果包含机构、层级、用途与可得发布时间，普通文本展示；提示词区分行业背景、品牌估值、商标核查与真实授权事实，不以来源优先级代替资料核实。
- 浏览器检查受 Chrome 打开的扩展面板阻止，本次未完成新元信息的真实浏览器视觉验收；保留此前原框架、白色等高容器及单色菜单布局，只新增文本信息。
- 自动验证：env 读取与部署变量优先、可选/必需文件、错误脱敏；参考域名齐全、域名冒充不匹配、来源排序、旧报告日期保留、未返回机构不会成为证据；后端完整测试与相关竞态测试、Vue 类型检查、ESLint、Stylelint、生产构建通过。
- 未读取用户密钥或进行真实 Tavily 搜索；用户自行填入 env 并重启后端后启用。

## Tavily 配置位置更正 · 2026-10-15

- 用户更正为 `config.yaml`。增加 typed `web_search.provider/tavily_api_key` 配置，真实配置与示例模板预留空密钥，搜索接口读取当前应用 Config；旧 env 仅作空值时兼容回退。
- 复用已有 YAML 热加载，密钥轮换后使用新的搜索缓存标识；真实配置文件继续 Git 忽略。未修改原有数据库、模型、SSO 或平台配置值。
- 验证：YAML 配置优先于旧 env、空密钥错误、兼容回退、重新读取 YAML 后经应用配置更新使用新密钥、响应不含密钥；后端完整测试和相关竞态测试通过。
- 本地服务继续使用 `/tmp/kol-admin-ui-debug-config.yaml`，默认部署文件为 `backend/config.yaml`，两处均已预留空密钥位置。未索取或调用用户的 Tavily 密钥。

## 项目会话三点菜单与切换请求 · 2026-10-15

- 原因：内容容器以完整 URL 为组件 key，切换 `subscriptionId` 重建推荐页；卸载又清空共享列表，同时选择事件和路由监听各自加载候选。线上 Chrome 只读复现：一次已保存会话切换产生 1 次订阅列表、2 次候选结果请求，之后恢复原项目，未写入或删除线上数据。
- 修正：推荐页忽略会话/结果选择参数造成的组件重建，保留其他提需上下文边界；路由监听统一读取候选。Pinia 列表增加 60 秒缓存及并发合并，过期更新时保留菜单，退出或更换登录会话时清空。删除与开关周检使用本地状态更新，不重新请求整张菜单。
- 本地真实 8848 / Chrome：悬停前三点 opacity=0、悬停后=1；点击出现“删除会话”，确认窗口包含实际项目名、历史删除范围和不可恢复说明。点击取消，已有会话保留。证据：`docs/qa/ip-session-menu-20261015.jpg`。侧栏继续透明背景、普通列表，复用原框架。
- 请求验证：新研究切回已有项目，CDP 捕获 0 次订阅列表、1 次候选结果。补充一个独占、暂停周检的临时本地订阅，通过 Chrome 原生界面切换两个已保存会话；本地系统请求日志按操作前的日志 ID 核对，单次切换 1 次候选、0 次列表，来回切换共 2 次候选、0 次列表。页面显示各自项目名、条件及候选，已有多轮对话恢复。随后只清理测试自建记录及临时测试脚本。
- 每周功能有真实调度：周一北京时间 09:00 后运行，已暂停订阅跳过，当周去重并写入新匹配系统通知。保留功能，文案明确为“库内新匹配”。调度时间与开关测试、本地数据库烟测通过；未声称观察到线上周检执行。
- 回归：原页面卸载探针修正前失败、修正后通过；15 项前端测试覆盖列表复用、并发/过期/失败、用户隔离、旧响应失效、删除导航、页面 key 与流式解析。Vue 类型检查、ESLint、Stylelint、设计扫描、生产构建和 diff 检查通过。
- 验收限制：后续 DOM/CDP 与本次手机检查持续受 Chrome 扩展弹窗阻挡；使用原生 Chrome 界面及本地请求日志完成已保存会话切换核对。本次未重新完成手机视觉验收；触屏三点入口由 44px 常显媒体规则提供。

## 联网搜索选中状态 · 2026-10-15

- 用户反馈原选中态的加粗、下划线不明显。改为沿用工作区荧光绿的实底按钮、深色文字及 Remix 勾选图标；输入框使用更清楚的绿色边界，提示明确显示“联网搜索已开启”。保持白色输入区与原菜单结构。
- 保留 `aria-pressed` 和稳定的按钮名称，补充开启/关闭操作提示及状态播报；关闭后恢复普通态。触屏入口至少 44px，窄容器工具栏可换行以容纳选中图标。
- 真实 Chrome 原生界面验证：点击后切换按钮 Value=1、操作提示为“关闭联网搜索”，出现开启状态说明，添加资料及空关键词发送禁用。没有调用搜索或发送资料。
- ESLint、Stylelint、Vue 类型检查及生产构建通过。DOM/CDP 继续受到扩展弹窗阻挡，原生截图返回空白，已丢弃；本次桌面像素及手机视觉验收未完成，未以空白截图作为证据。

## 联网按钮轻量描边调整 · 2026-10-15

- 按用户最新要求替代上一版强选中样式：撤掉绿色底色、勾选图标、选中字重和额外内边距，只在联网按钮四周绘制 1px 圆角轮廓。SVG 绝对定位，不参与按钮及工具栏宽度计算。
- 开启时沿轮廓平滑描绘 480ms，结束后保持静止；关闭时 120ms 淡出。连续切换可反向过渡，不循环闪动；减少动态效果设置下直接显示/隐藏完整轮廓。
- 输入框边界、提示样式和工具栏布局恢复到加强选中效果之前；保留原有联网关键词提示、输入限制、资料禁用等业务行为。
- 本地 Chrome 原生界面完成开启、关闭检查并取得可用截图：`docs/qa/ip-web-search-outline-20261015.png`。最终开启/关闭截图中按钮图标、文字的像素边界一致，按钮无底色，聊天容器的白底、边界和布局一致。没有发送查询或调用搜索。
- ESLint、Stylelint、Vue 类型检查、生产构建、diff 检查通过。DOM/CDP 仍被 Chrome 扩展弹窗阻挡，本次手机视口与减少动态效果未在真实浏览器中复验。

## 发送前校验文本框内容 · 2026-10-15

- 修正已有项目条件或附件令空文本框仍可发送的问题。普通对话与联网模式统一使用 `canSendMessage`：去掉首尾空白后必须有文字，且当前未生成、未读取资料。
- 发送按钮的禁用状态与 `sendMessage` 入口共用校验，Ctrl/⌘ + Enter 同样经过入口保护；已有条件或附件不再单独放开发送。
- Chrome 原生可访问性状态验证：纯空格为 disabled，填入测试文字后可用，清空后重新 disabled；测试文字已清除，未实际发送。像素截图仍显示缓存画面，未用作此次禁用状态证据。
- ESLint、Vue 类型检查、生产构建、diff 检查通过。改动未调整按钮或输入区样式。

## 库外推荐改为具体 IP 研究 · 2026-10-15

- 根据两轮 GrillMe 访谈落实：模糊请求先追问市场/用途，沿用公开研究条件；允许追问后的明确探索选择，预算可待定。电竞默认游戏、赛事、战队，选手须明确需要。
- 新流程执行规范化检索、主题过滤、具体实体及身份原文校验、聚焦补查。最多 3 轮、6 次基础搜索、60 秒；正文补读最多 5 个返回页面。失败停止额外补查，超时保留已核实部分与缺口。内部项目、预算、型号、附件和普通历史不进入搜索关键词。
- 截图的农业/WIPO Lex/代理广告负例修复前均放行、修复后被拒绝。WIPO 普通新闻不再标成商标数据库；相关性及身份准入优先于机构层级。
- 右侧原“库外”页签统计候选，提供身份、理由、切入点、待核实事项、来源展开、逐项补查及 2–3 个候选对比。正式意向和提需仍要求核实入库；未新增路由或独立页面。
- Chrome / 真实 8848 与本地 Go/MySQL：模糊电竞需求返回两个问题；快捷条件继续调用联网流程，空密钥如实提示。使用明确标注的临时模拟候选验证来源展开、两项对比和刷新恢复；这些记录仅供界面验收，不是真实搜索结果，随后由测试脚本清理。
- 桌面 1512px：白色左右面板实际高度均为 1432.73px，间距沿用原布局；页面无横向溢出。手机 390×844：页面滚动宽度为 390px，对比容器 292px、表格 480px，仅表格内部横向滚动。来源展开正常，检查时无浏览器控制台警告或错误。测试后恢复桌面视口；最后回到原项目的重载被 Chrome 扩展弹窗阻挡，未继续干扰用户扩展。
- 证据：`docs/qa/ip-external-candidates-desktop-20261015.png`、`docs/qa/ip-external-candidates-mobile-20261015.png`，均明确展示本地模拟数据。
- 自动检查：Go 全量测试、相关竞态测试、本地数据库烟测；15 项现有前端流式/会话/路由测试、ESLint、Stylelint、Vue 类型检查与生产构建。补充覆盖公开条件记忆、地域子串消歧、候选身份/引用/实体类型/授权声明负例、研究与请求限制、失败和取消、缓存用户隔离及密钥轮换。
- 未读取用户配置密钥，未调用真实 Tavily；真实供应商推荐质量须在已配置搜索及研究模型的环境中验证。

## 电竞 IP 真实联网验收 · 2026-10-15

- 按用户授权使用真实 Tavily、用户指定的 gpt-5 和既有密钥；本地服务切到已配置的 `backend/config.yaml`，保留 8082 监听。缺失的模型元数据按用户提供值补齐，未输出或提交密钥。
- 模糊需求真实返回市场与用途两个问题。修复后完整研究 38.52 秒、3 轮、5 次基础搜索、1 次正文补读、3 次模型调用，保留 3 个候选；MLBB 定向补查 21.44 秒，仅检索/返回指定 IP。
- 实测修复点名补查范围扩散及历史候选遗忘、Tavily 空域名偏好参数、固定 12 秒模型超时。回归测试均先红后绿；补查改为不施加行业站偏好，模型接收当前研究日期。
- 质量结论保留缺口：最终 MLBB 引用社交转帖，FFWS Spring 2026 具体届次已结束仍入选，RRQ 仅有行业合作背景；不能将结果称为可靠商业合作推荐。官方证据与时效性仍需在候选准入层加强。
- 完整记录与真实来源见 `docs/qa/ip-live-esports-20261015.md`。会话 43 保留供用户复查，未创建正式意向或回填提需。Chrome 原生界面确认追问及会话 43 的真实 3 个候选；DOM 受扩展面板阻挡，原生页面加载后取得有效截图 `docs/qa/ip-live-esports-20261015.png`，早期空白截图未保存。

## IP需求工作日截止时间与完成提醒 · 2026-10-15

- 在原 IP 需求管理、IP 反馈、营销意见页面接入期限。详情显示两个环节；列表显示当前环节或所在意见菜单对应环节，操作列固定在右侧。
- IP 意见 10 个工作日、营销意见 5 个工作日，次日起算、北京时间截止日全天有效；节假日排除、周末补班计入。内置已核验 2025/2026 年数据，后续完整年度可通过 config.yaml 热更新；缺少年份和历史阶段时间明确提示。
- 阶段时间独立持久化，草稿不启动下一阶段。全部意见补齐后，事务内生成发起人完成通知；顶部消息新增 IP需求，已读保存，弹窗支持原需求详情跳转。
- 浏览器在 localhost:8848 原框架验证四种状态、完整意见弹窗、查看详情跳转、刷新后未重弹、顶部待办具体日期。截图位于 docs/qa/ip-deadline-list-20261015.png 与 ip-deadline-detail-20261015.png，均为明确标记的临时验收样例。
- Chrome 扩展界面反复阻止 DOM 和 viewport 自动化；最终通过原窗口 AX 验证通知菜单与刷新状态，窄屏实测未完成，记录此限制。新增响应式布局已做静态检查。
- Go 全套、针对性 race、真实 MySQL 工作流与并发 race、前端 typecheck / ESLint / Stylelint / build，以及现有 15 项 Node 回归测试通过。测试禁用飞书发送，测试记录清理。
- 部署需先执行 migration 054。规则和年度维护说明见 docs/ip-request-deadlines.md。

## 营销意见按职能独立提交，提醒使用飞书 · 2026-10-15

- 根据用户补充，撤回上一版新增的站内完成弹窗、需求消息标签和轮询，停止写入站内工作流通知。反馈截止时间继续展示；阶段提醒与完成提醒沿用飞书应用消息，五个职能全部提交后仅提醒需求发起人。
- 营销意见按 IMC、品牌运营、用户运营、媒介投放、PR 分组，只展示、校验和提交本次选择的职能。每个职能有独立草稿、版本、提交人和时间；已提交只读，总进度可见。媒介 CPM 仍为选填，本职能补充意见为必填。五个职能共用原营销 5 个工作日期限。
- 后端以需求行锁读取最新 JSON，再合并所选职能字段；拒绝跨职能字段、未知 IP 和同职能旧版本，避免并发时覆盖伙伴意见。旧的已完成需求和历史共享意见保留。此次复用原 JSON 字段，无新迁移，不删除旧通知表。
- 所有需求/意见表单必填星号统一红色 `rgb(208,48,48)`；为 flex 表单把文字和星号归为同一行，浏览器测得基线相差 0.5px。
- 真实本地 MySQL + race 使用五个不同测试账号：IMC 独立草稿与提交，品牌/用户/媒介并发提交，PR 最后提交；验证前四个不完成、最后一个完成、重复/旧版本返回 409、不产生站内通知。测试关闭真实飞书，模拟发送器验证完成消息只发送发起人邮箱。
- 原 localhost:8848 框架浏览器实测：品牌仅填写自己的两项内容即可提交，进度 1/5 → 2/5，其余职能仍可继续；IMC 原意见保留且只读，星号红色同行。640px 下五个职能按钮自动换行，无该组横向溢出；已恢复原窗口尺寸。截图 docs/qa/ip-marketing-function-form-20261015.png 为明确标记的临时验收样例，验证后清理。
- 验证包含 Go 全套、针对性 race、真实数据库并发 race、前端 typecheck / ESLint / Stylelint / build。规则已同步 docs/ip-request-deadlines.md 和 backend/README.md。
