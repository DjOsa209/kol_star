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
