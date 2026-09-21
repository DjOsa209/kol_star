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

- 结果：PASS
- 验收页面：http://localhost:8848/#/business/resources
- 参考图：`/Users/rui.ma1/.codex/generated_images/01a0c1b2-c66a-7091-9d87-a030209d061c/exec-596010dc-e359-4b1a-946e-bf8f1e52577e.png`
- 实现截图：`design-qa-assets/profile-implementation.jpg`
- 对比图：`design-qa-assets/profile-comparison.jpg`

## 覆盖范围

- 资源抬头：头像、名称、账号、资源类型、领域、市场、可跳转的平台图标、分级标签。
- 达人基础表现：平台切换、六项指标、周环比、播放量波动指数、联系方式。
- 达人合作表现：费用、曝光、互动、次数、平均曝光、波动指数、互动率、CPM、备注。
- 达人合作明细：八列明细、年月日发布日期、可跳转作品封面。

## 视觉差异

- 参考图为 1680×944 桌面画布；当前应用验收视口为 950×782，因此基础表现由单行六卡响应式调整为三列两行，合作表现调整为两列，颜色、间距、卡片层级及信息顺序保持一致。
- 参考图使用演示数据；验收截图使用本地真实数据。无周快照、联系方式或足够样本时按产品规则显示 `--`、`/` 或“待评估”，不伪造增长率和稳定性。

## 交互与质量

- 多平台达人已验证 YouTube/Instagram 标签切换，平台粉丝量和播放量随平台独立更新。
- 最近合作作品仅读取合作记录，封面和平台主页保留跳转能力。
- 浏览器控制台无 error/warning。
- P0/P1/P2 视觉或功能问题：无。
