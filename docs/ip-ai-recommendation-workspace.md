# IP AI 推荐工作台

用户选择的视觉参考：`design/ip-prototypes/ai-recommendation-workspace-reference.png`。实现采用深色导航、暖白对话区、浅石色合作工作区和荧光绿主操作。生产图片来自现有 IP 资源资料；`design/ip-prototypes/assets` 下的生成图片只用于示例预览。

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

## 验证

详见根目录 `design-qa.md`。服务端测试覆盖排除条件、规则筛选、历史建议兼容、模型 JSON 契约、候选 ID 白名单和未登录访问。浏览器使用实际 Vue 组件与隔离 API 示例，验证四阶段、对比、追问、资料读取、错误重试和多 IP 意向回填。

本轮未连接线上模型或写入业务数据库。线上生成质量和模型供应商对 JSON 格式的支持需要在已配置的业务环境继续验收。
