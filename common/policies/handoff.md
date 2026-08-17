# Handoff and session continuity

目标：让下一次会话从短入口恢复，而不是重新读取长历史。

## 接手文档应包含

- 当前目标和边界。
- 已完成事项。
- 当前状态指标。
- 待处理队列。
- 下一步命令或检查清单。
- 权威数据文件路径。

## Apply currentness

- `status` 或 generic runbook 中未绑定 publication plan SHA/stamp 的 handoff Apply route 只是 `review-guidance`，必须保持 non-executable。
- 只有 fresh、同 scope 的 `handoff -WhatIf` 返回 exact `publicationPlanSha256`、`publicationStamp` 和 Apply request 后，才能执行该 Apply；不要手工重建参数。
- project publication 只能绑定 project Apply，lane publication 只能绑定 exact lane Apply；二者的 SHA/stamp 不得交叉复用。
- 持久化 Markdown、takeover JSON、typed driver request、expected receipt 和 run-loop 必须表达同一个 command/executable 状态。

## 不应包含

- round-by-round 长历史。
- 完整工具输出。
- 大段归档内容。
- 已可从代码、git 或机器数据重建的信息。

## 子 agent 结果沉淀

主 agent 只把合并后的结论、deferred 列表和下一步写入 handoff。子 agent 原始长输出不进入 handoff。

## workspace packet 引用

工作线 handoff 应列出当前 open candidate / defer 的 packet 文件路径（扫描 lane workspace 内 packet `.md`），让新会话只读 handoff 即可定位已产出证据，不必重跑。packet 文件路径相对 case root。

## decision event 引用

工作线 handoff `## decision` 区段应列本 lane latest N 条 decision event 摘要（`subject + decision + by + reason`），从 `.rekit/facts/decisions.jsonl` 读取。decision event schema 见 `docs/evidence-ledger.md`（`decision: accept|reject|defer|supersede`、`confirmedBy`、`writes`）。`by` 字段展示 `confirmedBy` 优先、fallback `actor`。`## pending-gate` 区段列本 lane `status=pending-gate` 的 request，供 heavy-tool gate 可见。
