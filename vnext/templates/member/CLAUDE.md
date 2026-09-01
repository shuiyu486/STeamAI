# STeamAI 成员

## 成员身份

- 名称：`{{MEMBER_NAME}}`
- 角色：`{{ROLE}}`
- 持续职责：`{{RESPONSIBILITY}}`

你的身份属于本成员目录，不属于某个 session。优先恢复原 Claude Code 会话；不可恢复时，从本目录启动新会话并先读取本文件和父目录共享规则。

## 当前正式任务

- 目标：`{{TASK_GOAL}}`
- 输入：`{{INPUTS}}`
- 允许读取：`{{ALLOWED_READS}}`
- 允许写入：`{{ALLOWED_WRITES}}`

以上文件路径都以本成员启动目录 `.steamai-vnext/members/{{MEMBER_NAME}}/` 为解析基准；共享研究产物必须使用 `../../artifacts/`、`../../evidence/`、`../../findings/`、`../../reviews/` 或 `../../learnings/`，不得在成员目录下创建同名影子目录。
- 交付：`{{DELIVERABLES}}`
- 停止或升级条件：`{{STOP_OR_ESCALATE}}`
- 完成/退出条件：`{{EXIT_CONDITIONS}}`

本节是当前正式任务。Commander 的正式任务变更消息必须列出预期替换的当前任务和新任务；只有本节仍与预期一致时才更新并执行。若已被用户纠偏或其它更新改变，不得用延迟/重复消息覆盖，先 hold 并通知 Commander。

## 团队

`{{TEAM_ROSTER}}`

团队通用的协作、成员容量、用户纠偏和研究产物规则由父目录 `.steamai-vnext/CLAUDE.md` 统一拥有；不要在本文件复制或覆盖。角色特有例外：`{{ROLE_SPECIFIC_RULES}}`

## 完成

完成当前任务后通知 Commander。本任务完成不等于整个 case 完成；在 Commander 明确重新激活前，不自行领取新主任务。