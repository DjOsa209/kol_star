# Design QA

- Source visual truth: the earlier overview/import references plus `/var/folders/2b/z81n4myx7k765ws80j1fgvc00000gn/T/codex-clipboard-b677ab31-c88c-4717-b2e0-197b1b857e34.png`, `/var/folders/2b/z81n4myx7k765ws80j1fgvc00000gn/T/codex-clipboard-37db8791-c2a2-49bf-90d5-01fb6aaaf4b9.png`, `/var/folders/2b/z81n4myx7k765ws80j1fgvc00000gn/T/codex-clipboard-cd4bcd29-fd8d-4de1-96d3-9c8da3b44dbd.png`, `/var/folders/2b/z81n4myx7k765ws80j1fgvc00000gn/T/codex-clipboard-78932f01-a583-417f-9610-eae9fed7b0fa.png`, `/var/folders/2b/z81n4myx7k765ws80j1fgvc00000gn/T/codex-clipboard-883c0105-dd1e-4b45-b2c0-5fccb13aa981.png`, `/var/folders/2b/z81n4myx7k765ws80j1fgvc00000gn/T/codex-clipboard-df841b3a-f5f0-439f-851d-f402c49d8408.png`, `/var/folders/2b/z81n4myx7k765ws80j1fgvc00000gn/T/codex-clipboard-6689be11-8b70-4db2-8627-140852882e93.png`, and `/var/folders/2b/z81n4myx7k765ws80j1fgvc00000gn/T/codex-clipboard-03e11059-ae8f-49ec-815b-bc55a234a54f.png`
- Implementation: `http://localhost:8848/#/business/projects/detail?id=24` and `http://localhost:8848/#/business/projects`
- Implementation screenshot: Codex in-app Browser tab 3 capture with the Upload Project Content dialog open (inline browser evidence; the browser backend did not expose a filesystem path)
- Viewport: 842 × 782 CSS px for the latest upload-dialog copy check; 842 × 750 for the metric-tooltip interaction check; 1280 × 720 for the project-table header check; 795 × 782 for the top-navigation contrast and sidebar-brand checks; 795 × 750 for the Tier-column check; 1518 × 1000 for the earlier KOL / Media comparison.
- Latest source pixels: 1220 × 424; latest implementation capture: 842 × 782 CSS px at device scale factor 1, with Replace mode selected and the project selector visible.
- State: English locale, light theme, authenticated local admin session, existing project id 24.

## Findings

- No actionable P0/P1/P2 differences remain for the requested changes.
- The longer Average Engagement Rate and Cost cards now receive larger grid tracks while the short CPM/CPE cards stay compact.
- Ellipsized metric descriptions expose their full localized copy through the native hover title. The Cost description was checked in the browser and its `title` value matched the full visible copy.
- Platform Performance displays `platforms:4` in the verified data state.
- Project-list action headings display `Discover More`; the previously narrow action columns were widened to 160px.
- Import-preview columns with long English labels or values now use larger minimum widths. Niche, Market, Platform, Collaboration Type, Content URL, Content Type, Contact, and Status were covered; overflow tooltips are enabled for truncatable cells.
- KOL and Media totals now use `KOL: 2` and `Media: 4`. The Monthly Unique Visitors (UMV) column uses a 280px minimum width and renders its full English heading at the reference width.
- KOL/media names, niche cells, UMV cells, and expanded collaboration-content titles expose full values on hover. Browser checks confirmed `imparkerburton` and the full expanded Instagram title.
- KOL and Media Tier columns now allocate 150px. In the English Media state, each `Pending Sync` tag measured about 99px inside a 150px cell, with equal `scrollWidth` and `clientWidth`, so no text is clipped.
- The project campaign-type tag above the content was removed; the synced-state indicator remains.
- In English mode, the configured product name now renders as `Infinix Global Resource Operations` in the sidebar logo, document title, and footer. Chinese mode continues to use the configured Chinese brand name.
- The first-level breadcrumb is now `#e4e4e7` at weight 500 against the `#111116` header. Before the fix it computed to `rgb(22,22,26)`, nearly matching the background; the second/current level retains its muted styling for hierarchy.
- The project-list `Impressions / Views` and `Engagement Rate` columns now measure 210px and 180px respectively, up from 150px and 120px. Their English labels and sort affordances render independently without colliding with adjacent headings.
- Every secondary description in the Exposure & Engagement metric cards now uses an Element Plus tooltip. Hovering an ellipsized description immediately exposes the complete localized copy instead of relying on a delayed native browser title.
- The Replace/Add upload flow now labels the selector `Filed Project`; its search placeholder and empty-state copy use the same terminology.

## Required fidelity surfaces

- Fonts and typography: existing product font stack, weights, sizes, truncation, and hierarchy were preserved. Long labels no longer change optical hierarchy.
- Spacing and layout rhythm: the six-card row remains aligned; only the relative column allocation changed. No clipping or page-level overflow was visible at 1280px.
- Colors and visual tokens: unchanged from the existing design system.
- Image quality and asset fidelity: no image assets were introduced or changed.
- Copy and content: `Actions` became `Discover More`, and the platform total uses the requested `platforms:xx` format.

## Full-view and focused comparison evidence

- Full-view browser capture confirmed the overview remains visually balanced and the platform section stays aligned below the metric row.
- A single comparison input contained the first source crop and the updated browser capture. The long-label cards are visibly wider in the implementation.
- Focused DOM/browser checks confirmed the full hover title, `platforms:4`, and the English `Discover More` header.
- A second same-input comparison paired the 1518px KOL / Media source with the updated 1518px browser capture; the UMV heading is fully visible and both counters use the requested label-first format.
- The latest focused comparison checked the supplied cropped Tier reference against the horizontally scrolled English Media table. `Pending Sync` and `Mid Tier` are fully visible with balanced horizontal padding, and the header no longer shows the campaign-type tag.
- The sidebar-brand comparison used the supplied expanded-sidebar crop and the refreshed English project-list view. The logo asset, typography, spacing, and ellipsis behavior are unchanged; only the localized title changed from mixed Chinese/English to English.
- The latest focused comparison checked the supplied dark-header crop against the updated project-list header. The first-level `Resource Operations` breadcrumb is now immediately legible while the separator and current page remain visually secondary; spacing, icons, and header height are unchanged.
- The newest focused comparison paired the supplied project-table crop with the horizontally positioned 1280 × 720 browser render. `Impressions / Views` and `Engagement Rate` are fully separated, and Content, Contact, values, and the fixed `Discover More` column retain their alignment. A focused region was required because the source is a cropped, horizontally scrolled table state.
- The latest focused comparison paired the supplied metric-card crop with the browser-rendered Exposure & Engagement section. The card layout, typography, color, and truncation remain unchanged; the hovered description now displays a dark tooltip containing the complete text. A focused interaction capture was required because tooltip behavior cannot be judged from a static full-page view.
- The latest focused comparison paired the supplied Upload Project Content crop with the rendered Replace-mode dialog. The requested copy now reads `Filed Project`, the placeholder reads `Search and Select a Filed Project`, and the form spacing, radio state, selector width, and surrounding alerts remain unchanged.
- The import preview's populated state was not opened because doing so requires selecting a local spreadsheet; its widths and Element Plus overflow-tooltip behavior were validated from the rendered column configuration and type/build checks.

## Interaction and console checks

- Tested locale switching, project navigation, and project-detail loading.
- Checked browser console errors after rendering: none.
- `pnpm typecheck`: passed.
- `pnpm build`: passed. The build emitted only existing third-party Rolldown pure-annotation warnings.

## Comparison history

- Initial issue: equal-width metric tracks squeezed long English labels and descriptions; import-preview English columns reused narrow widths; action labels read `Actions`.
- Fix: introduced weighted metric tracks, hover titles, wider import/action columns, overflow tooltips, and updated copy.
- Post-fix evidence: browser capture at 1280 × 720, full Cost tooltip title, `platforms:4`, `Discover More`, no console errors.
- Latest issue: KOL/Media totals used value-first copy, the UMV column was too narrow, and custom truncated cells lacked hover text.
- Latest fix: changed the counters to `KOL: xx` / `Media: xx`, widened UMV and identity columns, enabled overflow tooltips, and added native titles to custom name/content cells.
- Latest post-fix evidence: 1518 × 1000 English browser capture, full UMV heading, both requested counters, verified name/content titles, and no console errors.
- Latest issue: the 100px Tier column clipped the English status tag, and the project header still displayed a campaign-type/content-import tag.
- Latest fix: widened both Tier columns to 150px and removed the campaign-type tag from the project header.
- Latest post-fix evidence: focused 795 × 750 English browser capture, measured 150px Tier cells with non-overflowing 99px `Pending Sync` tags, no campaign-type tag, and no console errors.
- Latest issue: the English sidebar retained the Chinese product-name suffix from the static platform configuration.
- Latest fix: routed the product title through the existing field translation dictionary in the sidebar, document title, and footer.
- Latest post-fix evidence: expanded English sidebar capture showing `Infinix Global R…`, accessibility title `Infinix Global Resource Operations`, English document title/footer, and no console errors.
- Latest issue: the first-level breadcrumb inherited an anchor color of `rgb(22,22,26)` on the `#111116` navbar, making it less legible than the second level.
- Latest fix: added a navbar-scoped breadcrumb rule using `#e4e4e7` and font-weight 500 for non-final breadcrumb links, while retaining a muted final level.
- Latest post-fix evidence: computed first-level color `rgb(228,228,231)`, weight 500, refreshed browser capture, and unchanged breadcrumb layout.
- Latest issue: fixed widths of 150px and 120px caused the English `Impressions / Views` and `Engagement Rate` headings to crowd each other and their sort icon.
- Latest fix: widened the two project-list columns to 210px and 180px while preserving the table's horizontal scrolling and fixed action column.
- Latest post-fix evidence: browser-measured header widths of 210px and 180px, focused 1280 × 720 capture with clear inter-column spacing, zero console errors, and passing typecheck/build.
- Latest issue: ellipsized metric descriptions used native `title` attributes, which did not provide a reliable visible hover response.
- Latest fix: replaced the six native titles with consistent `el-tooltip` triggers while preserving the existing ellipsis and card dimensions.
- Latest post-fix evidence: CDP mouse hover over the first and second metric descriptions rendered the full tooltip copy (`所有合作内容累计数据` and `点赞 + 评论 + 分享 + 收藏`), with zero console errors and passing typecheck/build.
- Latest issue: the upload dialog used `Existing Project` terminology in the field label, search placeholder, and empty state.
- Latest fix: updated the three English localization strings to use `Filed Project` consistently.
- Latest post-fix evidence: browser-rendered Replace-mode dialog shows `Filed Project` and `Search and Select a Filed Project`, with unchanged layout, zero console errors, and passing typecheck/build.

## Follow-up polish

- P3 test gap: exercise the populated import-preview dialog with a representative spreadsheet in a future regression pass.

final result: passed
