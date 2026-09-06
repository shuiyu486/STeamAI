# Authorized Web and API security research route

`web-security` 是已授权 Web/API 安全评估、靶场与防御性验证的声明式研究 pack。它只提供 passive triage、request hypothesis、bounded replay 与 remediation evidence 的按需 reference、tooling recipe 与安全边界，不执行研究动作。

## 读取顺序

| 场景 | 读取 |
|---|---|
| 规划领域流程 | `workflow-template.md` |
| 组建成员或请求复核 | `agent-team.md` |
| 选择工具或判断 heavy action | `toolchain-router.md` |
| 单一请求假设、发送不确定、差异归因或修复对照 | `../../tooling/recipes/request-replay.md` |

## 常驻边界

- 先读 case `.steamai-vnext/CLAUDE.md`，再从 case-local pack snapshot 按需读取本文件；只选择当前任务需要的入口，不默认串读整个 pack。
- 研究恢复复用 native session 与现有 E/F/R，不要求新建 handoff 文件。请求方法和已有 JSON schema 只是声明，不提供 executor；真实请求另需具体确认，readonly evaluator 不因此获得网络权限。
- 真实对象、原始 artifact、敏感日志、凭据、客户信息、绝对路径与 case 进度只留在 case 内。
- target_ref、endpoint_ref、request_ref、finding_ref 使用 case-local 脱敏引用；finding 必须引用 evidence，review 必须引用 finding/evidence。
- heavy action 必须有明确 case 授权、针对该具体动作的用户确认与 Claude Code 工具权限；任何范围或预算漂移都停止。
- 可复用经验只从 accepted finding/review 提炼，经 Reviewer 检查并由用户确认 exact patch 后才回流。
