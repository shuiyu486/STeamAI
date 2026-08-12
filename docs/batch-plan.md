# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source，再按需读取 `docs/agent-team-usage.md` 的 “Remote Control Reviewer transport companion” 小节。`docs/daily-product-closure-plan.md` 只保留上一条已关闭路线的历史证据；完整历史已拆到 `docs/batch-history.md`。

## 实施摘要

`daily-product-closure-v1` 已完成并关闭。当前已批准路线 `remote-control-reviewer-transport-v1` / Batch 822 也已完成：Claude Code Remote Control 已作为 durable external-session 的 optional read-only Reviewer transport companion 收口；没有下一批，等待用户明确批准新的产品路线。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `remote-control-reviewer-transport-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `Batch 822` optional Remote Control read-only Reviewer transport companion |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 822` |
| 上一批 | `daily-product-closure-v1` / `DPC-04` 已完成 |
| 下一批 | 无；Batch 822 完成后等待用户明确批准新的产品路线 |

### Current batch state

Batch 822 已完成生产实现、focused 全链、受影响 package 与全仓回归、完整 Windows 本机 release minimum、canonical 文档写回和两轮独立复核。路线保持 terminal completed、下一批为无；不启动真实 Remote Control、不修改全局 provider/settings、不 commit/push，也不自行选择下一路线。

### Batch 822：optional Remote Control read-only Reviewer transport companion

状态：已完成。

目标：让显式durable Reviewer dispatch可使用Claude Code Remote Control跨会话通信，同时保留lane-centric identity、existing external-session state machine、self-contained evidence、truthful delivery、generation fencing和strict intake；local`claude-code-cli`继续作为Windows默认provider。

验证结果：focused Remote Control 产品链及 `memberexecution`、`externalsession`、`cli`、`sessionhost` 四包完整回归通过；全仓 `go test ./... -count=1`、`go vet ./...`、`git diff --check`、公开 `status`、10-pack `packs`、`doctor` 与完成态 `release-check -Format json`（`ready=true`）全部通过。独立代码终审无surviving finding；独立文档复核发现的当前入口误指 DPC 与 release 状态陈述漂移均已修复。该验收只证明 Windows deterministic product path，不声称原生 Windows live cross-machine Remote Control E2E。

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan 最多保留一个 compact latest `### Batch N` 摘要且保持短投影；完整历史只在 `docs/batch-history.md`。
- 当前批只有focused tests、truthful Windows deterministic acceptance、完整Windows本机minimum和canonical文档写回全部通过后才可完成；原生Windows live cross-machine Remote Control E2E不属于可伪造门槛。

## 风险与注意事项

- 本文件不是第二份 roadmap；阶段方向或批次顺序只在路线图记录，再同步本投影。
- 不把完整 receipt、逐轮日志、未来批次卡或生成 inventory 塞回本文件。
- 路线与文档不授予 commit/push；只有当前用户明确授权时才执行。
