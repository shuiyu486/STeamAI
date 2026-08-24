# Agent Team orchestration plan

## 读取指南

本文件是 Agent Team 编排设计参考，不是当前批次默认必读清单。日常推进先读 `docs/context-routing.md`、`docs/batch-plan.md` 顶部和相关 Go symbols；只有需要复核 planner / dispatcher / gate / digest / ledger 的长期分层时，再读本文件顶部和对应阶段。

## 实施摘要

当前 runtime 已采用 `plan-subagents` 生成 packet、主 Agent 显式 dispatch reviewer、strict intake 写回 verification/decision 的模式；本文件保留长期 orchestration 设计，不代表 runtime 会自动 spawn reviewer 或执行 heavy-tool。实际产品路径和已完成闭环以 `docs/batch-plan.md` 当前批次和 CLI tests 为准。

## 执行清单

- reviewer / lane / gate product-path 问题先查 CodeGraph 与 `docs/agent-team-usage.md` 顶部。
- 需要设计取舍时只读取本文件相关层级或阶段，不默认读取全部历史计划。
- 自动化边界必须回到 gate / autonomy / authorized-gate contract，不从本设计文档推断授权。
- 变更后把用户可见行为记录到 `CHANGELOG.md`，批次细节放 `docs/batch-plan.md` 或归档。

## 验证标准

- `plan-subagents` 仍只生成 bounded packet，不自动 spawn/monitor reviewer。
- reviewer intake 仍保持 WhatIf-before-Apply、verification-before-decision、no authority/confirmed、no heavy-tool。
- 新会话能从 status/handoff/continue/durable artifacts 接手，不需要读完整 orchestration 设计。

## 风险与注意事项

- 本文件是设计参考，不是授权来源；heavy/debug/patch/dump/hook/network/exploit replay 必须由 strict durable autonomy profile + authorized-gate decision 覆盖。
- 不要把具体 case packet、reviewer result、trace、dump 或客户信息写入本仓库文档。

## 目的

定义未来 `/rekit` 半自动 Agent Team runtime 的实施计划。当前文件是设计计划，不表示 runtime 已实现自动分派。

目标是在保持 review-first、可解释、可暂停、可回放的前提下，让 `/rekit` 能为不同安全领域 pack 生成 packet、分派只读复核、收敛结论、完成 deterministic authorization preflight、在必要时提示人工确认并更新 handoff。历史上首个 mature `vmp-re` 已收敛为唯一 active 二进制逆向 identity `binary-re`；当前另有 mature `web-security` production vertical slice，两者都不是最终边界。

## 非目标

- 不让 runtime 直接控制所有外部安全领域工具。
- 不在缺少本次显式用户确认且缺少 strict durable autonomy profile + 对应 `authorized-gate` decision 时执行 debug、inject、patch、dump 或 network；confirmed/authority 写入仍必须显式升级。
- 不让子 agent 写 authority 文件。
- 不把 orchestration 做成不可解释的黑盒。

## 分层

| 层 | 职责 | 预期位置 |
|---|---|---|
| Planner | 根据 manifest route 和 case state 生成 packet | `internal/rekit/subagents/**` 与 CLI `plan-subagents` handler |
| Dispatcher | 有界分派只读 review 或 workspace-only task | main agent / future runtime hook |
| Gate | 判断是否允许写入、升级或询问用户 | `internal/rekit/gate/**` |
| Digest | 记录每轮输入、输出、决策和风险 | `.rekit/runs/<run-id>/digest.md` |
| Ledger | 追加 observation/candidate/decision/intervention | `.rekit/facts/*.jsonl` |

## 实施阶段

### O1：增强 plan-subagents

- 输入：items、route、taskType、manifest route。
- 输出：review packet、分片计划、输出契约。
- 不启动 agent，不写 managed docs。
- 验证：packet 可读、分片固定、无越界路径。

### O2：bounded dispatch 约定

> **状态注：** 本节描述的"runtime 自动 bounded dispatch"已被 `docs/agent-team-rollout-plan.md` R5 否决——spawn 子 agent 是主会话/Claude Code 职责，不是 PowerShell runtime 职责。runtime 侧由 `plan-subagents`（输出分片计划）+ `note -Kind decision`（verdict 写回）构成支撑，真启 reviewer 由主会话用 Agent 工具完成。本节保留作为 Phase 6 后段（跨工具 adapter 实际调用）的设计参考，当前不按此自动 spawn。

- 由主 agent 根据 packet 启动只读 reviewer。
- reviewer 输出 `decision, confidence, evidence, risk, next_action`。
- 主 agent 合并 accepted/rejected/deferred。
- 验证：子 agent 不能写 authority，失败不会污染 confirmed。

### O3：heavy-tool gate

- 对 full trace、debug、inject、patch、dump、symex、network 执行 deterministic authorization preflight：优先检查 strict validated durable autonomy profile 与覆盖本次 action、exact target、typed budget、stop/output/record/notify 边界的 `authorized-gate` decision；未完整覆盖时生成具体确认问题。
- 记录 reason、tried_light_steps、budget、outputs、stop_conditions、profile/grant 和 gate decision。
- 显式用户确认或 durable grant 都只覆盖列明 scope，不扩大授权；confirmed/authority/promote 仍需单独升级。
- 验证：既无本次显式用户确认、也无有效 deterministic grant 时，不执行外部副作用。

### O4：digest / replay

- 每轮自动流程写 `.rekit/runs/<run-id>/digest.md`。
- digest 包括 inputs、pack、route、packet refs、outputs、decisions、open risks。
- replay 不重做外部副作用，只重读 packet 和 sidecar。
- 验证：新会话只读 digest 能理解本轮发生了什么。

### O5：case-local trial

- 只在临时 case 中试运行。
- 先 dry-run，再 apply。
- 验证 init/attach/sync/promote 不回归。

## Debug checklist

每个 orchestration run 必须能回答：

```text
case root 是什么？
pack 是什么？
用了哪个 route？
读了哪些 packet / sidecar？
写了哪些 state / digest / facts？
哪些 candidate 被接受、拒绝、延期？
是否触发 heavy gate？
是否需要用户确认？
如何回滚或 supersede？
```

## 成功标准

- 主 agent 上下文不需要装入大 trace 或大反编译。
- 子 agent 只消费有界 packet。
- 所有自动写入都有 digest 和 ledger event。
- confirmed/authority 写入仍由 gate 控制。
- 任一 agent 失败时，主线可以继续或明确记录 deferred。