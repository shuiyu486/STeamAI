# 渐进式披露与上下文预算

## 路由

| 需求 | 入口 |
|---|---|
| 通用分析 | `general-analysis.md`、`general-workflow.md` |
| 团队协同 | `general-agent-team.md`、`lane-collaboration.md` |
| VMP workflow | `workflow-template.md` |
| 工具选择 | `general-toolchain-router.md` 或 `toolchain-router.md` |
| focused handler review | `singleton-handler-review.md` |
| policy overlays | `../../policies/README.md` |

## 快速规则

- 新成员先读 case/member `CLAUDE.md`，再只读一个与当前任务匹配的 pack 入口。
- 查 coverage 用 summary 或脚本统计，不整读大 CSV。
- 查 handler 按 alias/RVA 过滤 sidecar，不读全部 round 文档。
- 批量复核必须固定分片、限定文件/行范围和输出契约；每个问题一名 owner、最多一名 verifier。
- tool 大输出只返回摘要和 case-relative evidence ref。
- learning 只从 accepted finding/review 提炼，经 Reviewer 和用户确认 exact patch 后回流。

## 禁止模式

- 不把完整 CSV、disasm、decompile、trace 或历史归档粘进 Markdown。
- 不在成员 `CLAUDE.md` 追加 round-by-round 历史。
- 不启动无界成员或 subagent 去探索整批大文件。
- 不用 pack 推断当前 case 进度、成员任务或授权。

## 收尾

1. 运行与修改直接相关的最短验证。
2. 修改 shared confirmed table 后重建派生产物并检查 coverage/unknown。
3. 更新 case-local finding/review 或 handoff，不更新 pack 中的 case 进度。
4. 检查 Markdown 预算、链接与 `git diff --check`。
5. 可复用经验只进入 learning candidate，不直接写 canonical pack。
