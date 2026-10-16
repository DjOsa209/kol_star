# IP English terminology

Use short, action-led button labels and sentence case. Keep **AI IP** as the agreed menu name. Interface wording is separate from stored project content, names, IDs, enum values, and chat history.

The shared shell follows the same conventions: **Sign in / Sign out**, **Settings**, **Administration**, **View more**, and **Column settings**. These changes apply to common navigation, settings, notifications, and table controls; other modules’ business content is outside this pass.

| Concept | Preferred wording | Usage |
| --- | --- | --- |
| IP资源库 | IP library | Navigation and profiles |
| 初步意向IP反馈 / IP组评估 | IP review | Review stage; submitting feedback does not imply approval |
| 营销意见补充 | Marketing review | Covers IMC, Brand, User engagement, Media buying, and PR |
| 意见 | Feedback | Individual responses and submit actions |
| 项目会话 | Project chats | Conversation list; “project chat” for one item |
| 合作工作区 | Workspace | Candidate research and comparison |
| 意向清单 | Shortlist | “Add to shortlist” / “Confirm shortlist” |
| 版权方 | Rights holder | Does not imply that a license has been granted |
| 授权费用 / 预算 | Licensing fee / Licensing budget | Pricing and budget fields |
| 基础权益价格 | Indicative licensing fee | An estimate, not a binding quote |
| 授权区域 | Licensed territories | Actual licensing scope; distinct from audience market coverage |
| 合作切入 | Partnership opportunities | Potential uses and collaboration ideas |
| 历史合作 / 结案资料 | Case studies / Final reports | Past work and supporting reports |
| 待补充 / 待询价 | Not provided / Quote needed | Missing information, not workflow approval states |
| 资料完成度 | Profile completeness | Record completion indicator |

Different Chinese labels that mean the same action should use the same English label. Avoid literal phrases such as “initial interest IP”, “pending supplement”, “base rights price”, or “backfill the request”. Use complete messages when available and preserve interpolation tokens.

## Reference basis

- [Atlassian button content guidelines](https://atlassian.design/components/button/usage): short labels, action verbs, sentence case.
- [Microsoft capitalization guidance](https://learn.microsoft.com/en-us/style-guide/capitalization): sentence case for interface labels; preserve proper nouns and acronyms.
- [Licensing International: What is licensing?](https://licensinginternational.org/education/what-is-licensing/): licensing, licensors / IP owners, territories, and licensing agents. The exact labels above are choices for this product, based on those concepts.

## Import compatibility

The English workbook uses the revised terminology. `ip.en.legacy.json` retains earlier headers and option labels for importing existing spreadsheets. Display labels can change without modifying canonical Chinese values or breaking old templates. Frontend and server dictionaries must stay identical.

Checked with `pnpm check:ip-locales`: both workbook versions map to canonical fields, legacy and current category labels map to the same enum, and placeholders remain intact. Browser evidence is stored under `docs/qa/ip-english-copy-20261016-*.png`.
