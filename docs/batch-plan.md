# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志；本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`，现处于获批 P0～P3 residual closure。Batch 830～833 与 validation repair 已完成各自局部闭环，但原始 P0～P3 task ledger 明确保留 #50、#37、#52、#38 为 pending，且没有用户取消记录；此前 `completed / 无下一批` 是假收口。现保持同一路线、不创建 Batch 834，按 `P0P3-C1`～`C4` 依赖顺序完成剩余架构闭环。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `P0P3-C1 capability control sinks` |
| 状态 | `in_progress` |
| 唯一允许领取 | `P0P3-C1` |
| 上一批 | `Batch 833` binary-re actual analysis 与随后 validation repair 已完成 |
| 下一批 | `P0P3-C2 orchestration non-authorization` |

### Current batch state

### P0P3-C1：capability control sinks

状态：in_progress；完成旧 task #50，不创建新 numbered batch。

目标：在 adapter parent/private child、Reviewer lifecycle、Remote transport/current model-tool effect sink 前消费 exact capability/control lineage；paused/stopped/stale 在副作用前 fail-closed，late result只保留 held truth。完成后只解锁 `P0P3-C2`。

验证结果：待当前代码审计、复现测试与 focused/product/full validation；不得用既有 shared capability DTO、`release-check.ready=true` 或 `870d908` validation receipt预写为完成。

### Batch 833：binary-re actual analysis

状态：已完成 ordinary `binary-re` actual adapter lifecycle、直接相关 fresh validation 与 implementation commit/push；它是 latest numbered implementation identity，不代表随后全部 P0～P3 residual work完成。

目标：以 `Batch 833` 作为 latest numbered receipt identity；P0P3 residual milestones由 active route 独立表达，不冒充 Batch 834。

验证结果：focused fresh lifecycle tests实际启动 embedded authorized parent/fixed child，并覆盖strict gate、independent evidence review、accepted-only binding、terminal recovery与stale control fail-closed；随后validation repair的最终`release-run -Format json`以7/7通过，direct commit `870d908`与tracking ref/post-push receipt已闭合。这些证据只证明相应实现和validation，不关闭 #50/#37/#52/#38。

### Locked sequence

| 工作 | 解锁条件 |
|---|---|
| `P0P3-C1` capability control sinks | 当前唯一允许领取 |
| `P0P3-C2` orchestration non-authorization | C1完成并验证 |
| `P0P3-C3` DTO/receipt/session/skill | C2完成并验证 |
| `P0P3-C4` runtime ownership | C3完成并验证 |
| 路线总体完成 | C1～C4及原P0～P3逐项验收全部通过 |

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan只保留一个 current residual milestone和一个latest numbered batch handoff；更早批次只在`docs/batch-history.md`。
- route completion不能由Git-local validation receipt替代；machine validation也不能由Markdown claim替代。
- `Batch 833`不自动解锁下一 numbered batch；P0P3-C1～C4不创建Batch 834。
- 不声称未读取的remote CI green，不用cross-compile代替非Windows runtime E2E。

## 风险与注意事项

- 不再把局部batch完成、提交推送、测试全绿或stop hook收尾误判为长期获批方案完成。
- 不全局替换兼容`rekit` identity，不新增PowerShell runtime logic，不引入installer或PATH fallback。
- authority/confirmed、heavy action、sync/promote和schema migration继续遵守exact review/gate边界。
