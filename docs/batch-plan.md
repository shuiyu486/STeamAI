# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志；本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-product-optimization-v1` 的获批 P0～P3 residual closure。`P0P3-C1`、`P0P3-C2`与`P0P3-C3`已完成，当前只推进`P0P3-C4`；随后按 retired pack identity migration、binary-re专项验收校准和完整P0～P3验证收口。Batch 830～833与validation repair仍只是各自局部证据；不创建Batch 834、不自选新功能。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `P0P3-C4 runtime ownership` |
| 状态 | `in_progress` |
| 唯一允许领取 | `P0P3-C4` |
| 上一批 | `P0P3-C3 DTO/receipt/session/skill` 已完成并通过 focused/product validation |
| 下一批 | retired pack identity migration |

### Current batch state

### P0P3-C3：DTO/receipt/session/skill

状态：已完成；完成旧task #52，不创建新numbered batch。

结果：`skillcontractgen -check` 已零写入并进入三平台 CI；bundle/init/current-sync 使用同一份 exact validated skill bytes，late pair drift fail-closed；handoff stamped publication 使用 immutable preflight 与 atomic no-replace exact replay，旧 stamp 不能覆盖历史 identity。DTO/public boundary 与 session owner 只做有界证据审计，未发现可复现缺陷；没有新增 schema、session 状态机或 provider。

验证结果：skillcontract、runtimebundle、sync、fs、workstream、releasecheck、CLI handoff/release-check focused tests及受影响完整 package tests通过；完整 P0～P3 验证仍未完成。

### P0P3-C4：runtime ownership

状态：in_progress；完成旧task #38，不创建新numbered batch。

目标：让 scoped descriptor 与 typed reducer 真正拥有 continue/control/gate 目标 runtime 的 binder、validator、handler、profile、policy 和 publication owner，消除平行 dispatch 与共享可变投影；只修复可复现缺口，不重写 durable state、gate、receipt 或权限模型。

验证结果：待 C4 focused/product/full validation；C4 完成并验证后才解锁 retired pack identity migration。

### Batch 833：binary-re actual analysis

状态：已完成 ordinary `binary-re` actual adapter lifecycle、直接相关 fresh validation 与 implementation commit/push；它是 latest numbered implementation identity，不代表随后全部 P0～P3 residual work完成。

目标：以 `Batch 833` 作为 latest numbered receipt identity；P0P3 residual milestones由 active route 独立表达，不冒充 Batch 834。

验证结果：focused fresh lifecycle tests实际启动 embedded authorized parent/fixed child，并覆盖strict gate、independent evidence review、accepted-only binding、terminal recovery与stale control fail-closed；随后validation repair的最终`release-run -Format json`以7/7通过，direct commit `870d908`与tracking ref/post-push receipt已闭合。这些证据只证明相应实现和validation，不关闭后续 residual work。

### Locked sequence

| 工作 | 解锁条件 |
|---|---|
| `P0P3-C1` capability control sinks | 已完成 |
| `P0P3-C2` orchestration non-authorization | 已完成并解锁C3 |
| `P0P3-C3` DTO/receipt/session/skill | 当前唯一允许领取 |
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
