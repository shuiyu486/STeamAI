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

本节是当前正式任务。Commander 只在首次创建、尚未启动本成员 session 时生成初始文件；首次启动后，本成员是本文件的产品单写者。Commander 的正式任务变更消息必须列出 expected current task 的全部七项与 new current task 的全部七项；只有本节仍逐项匹配 expected 时才由本成员更新并执行。若已被用户纠偏或其它更新改变，返回 `HOLD_STALE_TASK`，零覆盖并通知 Commander。

Commander 不因本 session 不可达而代写本文件。重新激活时，成员先在 roster 仍为 `completed`/`inactive` 时 compare-before-update 写入新任务并报告 ready；Commander 收到 ready 后才把唯一 durable roster 改为 `active`。同一 member cwd 若当前观察到两个可写 session，所有 agent 发起的任务改写都 hold，直到用户直接选择一个 session；不创建 primary-session 状态。

团队 roster、成员容量、用户纠偏和研究产物规则由父目录 `.steamai-vnext/CLAUDE.md` 唯一拥有；本文件不复制 roster。角色特有例外：`{{ROLE_SPECIFIC_RULES}}`

## 完成

完成当前任务后通知 Commander。本任务完成不等于整个 case 完成；在 Commander 明确重新激活前，不自行领取新主任务。