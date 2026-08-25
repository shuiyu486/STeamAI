# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志；本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-product-optimization-v1` 的获批 P0～P3 residual closure。`P0P3-C1` 已完成，当前只推进 `P0P3-C2`；随后按 `C3`、`C4`、retired pack identity migration、binary-re 专项验收校准和完整 P0～P3 验证收口。Batch 830～833 与 validation repair 仍只是各自局部证据；不创建 Batch 834、不自选新功能。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `P0P3-C2 orchestration non-authorization` |
| 状态 | `in_progress` |
| 唯一允许领取 | `P0P3-C2` |
| 上一批 | `P0P3-C1 capability control sinks` 已完成并通过 fresh local validation |
| 下一批 | `P0P3-C3 DTO/receipt/session/skill` |

### Current batch state

### P0P3-C2：orchestration non-authorization

状态：in_progress；完成旧 task #37，不创建新 numbered batch。

目标：统一 custom tools、MCP、tactical subagents、Reviewer 与 current model-tool handoff 的 durable typed command、owner/generation/currentness/capability/gate约束；transport、endpoint、delivery/provider acknowledgement、hooks、SessionStore、Agent Teams或tool presence不得构造 authority、confirmed、`authorized-gate` 或 heavy-tool grant。

验证结果：待当前代码审计、直接越权复现、focused/product/full validation；不得把 C1 的 sink currentness、字段存在、`release-check.ready=true` 或 synthetic transport fixture预写为 C2 完成。

### Batch 833：binary-re actual analysis

状态：已完成 ordinary `binary-re` actual adapter lifecycle、直接相关 fresh validation 与 implementation commit/push；它是 latest numbered implementation identity，不代表随后全部 P0～P3 residual work完成。

目标：以 `Batch 833` 作为 latest numbered receipt identity；P0P3 residual milestones由 active route 独立表达，不冒充 Batch 834。

验证结果：focused fresh lifecycle tests实际启动 embedded authorized parent/fixed child，并覆盖strict gate、independent evidence review、accepted-only binding、terminal recovery与stale control fail-closed；随后validation repair的最终`release-run -Format json`以7/7通过，direct commit `870d908`与tracking ref/post-push receipt已闭合。这些证据只证明相应实现和validation，不关闭后续 residual work。

### Locked sequence

| 工作 | 解锁条件 |
|---|---|
| `P0P3-C1` capability control sinks | 已完成并解锁 C2 |
| `P0P3-C2` orchestration non-authorization | 当前唯一允许领取 |
| `P0P3-C3` DTO/receipt/session/skill | C2完成并验证 |
| `P0P3-C4` runtime ownership | C3完成并验证 |
| retired pack identity migration | C4完成并验证 |
| binary-re 专项验收校准 | retired pack identity migration完成并验证 |
| 路线总体完成 | 上述闭环及原P0～P3逐项验收全部通过 |

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan只保留一个 current residual milestone和一个latest numbered batch handoff；更早批次只在`docs/batch-history.md`。
- route completion不能由Git-local validation receipt替代；machine validation也不能由Markdown claim替代。
- `Batch 833`不自动解锁下一 numbered batch；当前路线不创建Batch 834。
- 不声称未读取的remote CI green，不用cross-compile代替非Windows runtime E2E。

## 风险与注意事项

- 不再把局部batch完成、提交推送、测试全绿或stop hook收尾误判为长期获批方案完成。
- 不全局替换兼容`rekit` identity，不新增PowerShell runtime logic，不引入installer或PATH fallback。
- authority/confirmed、heavy action、sync/promote和schema migration继续遵守exact review/gate边界。
