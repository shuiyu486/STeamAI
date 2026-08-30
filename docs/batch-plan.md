# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志。本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-architecture-product-convergence-v1`，`APC-01`～`APC-04` 已完成。当前无 owner/next batch；等待新的明确批准路线。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-architecture-product-convergence-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `APC-04 真实产品验收与架构收口` |
| 状态 | `completed` |
| 唯一允许领取 | `none` |
| 上一批 | `APC-03` 已完成 |
| 下一批 | `none`；等待新的明确批准路线 |

### Current batch state

### APC-04：真实产品验收与架构收口

状态：completed；无当前 owner。

目标结果：APC-01～APC-03整体通过canonical/full public gates、source-clone/self-contained、copy/tamper/dual-root、两个mature pack、successor generation与显式real-Claude产品验收；owner、代码规模和证据真实性完成终审。

最终验证：last-finding actor-bound successor `applyArgs`修复后，compiled unified host原样执行回归、fresh full suite、vet、diff/contract gates通过，独立终审无Important/Blocker。Windows与real-Claude证据和synthetic/cross-compile/remote边界已分别记录；不创建新numbered batch。

### Batch 833：binary-re actual analysis

状态：已完成 ordinary `binary-re` actual adapter lifecycle、直接相关 fresh validation 与 implementation commit/push；它是 latest numbered implementation identity，不代表当前 active route 已完成。

目标：保留 `Batch 833` 作为 latest numbered receipt identity；`APC-01`～`APC-04` 由 active route 独立表达，不创建新的 numbered batch冒充路线进度。

验证结果：focused fresh lifecycle tests实际启动 embedded authorized parent/fixed child，并覆盖 strict gate、independent evidence review、accepted-only binding、terminal recovery与 stale control fail-closed；最终 `release-run -Format json` 7/7通过，direct commit与 tracking ref已闭合。该历史证据不能替代当前路线的 fresh validation。

### Locked sequence

| 工作 | 解锁条件 |
|---|---|
| `APC-01` correctness and hermetic validation | 当前唯一允许实施 |
| `APC-02` typed command / projection ownership | `APC-01` 完成并通过完整门禁 |
| `APC-03` mission lifecycle / public UX | `APC-02` 删除旧 owner并通过 projection矩阵 |
| `APC-04` real product acceptance / architecture closure | `APC-03` 真实临时项目闭环通过 |
| 路线总体完成 | 四阶段全部完成且 fresh machine publication truth成立 |

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- 每阶段必须形成可运行修复、删除旧 owner、真实用户闭环或可量化复杂度净减；只增加字段/summary/inventory不算完成。
- `APC-04` 完成前不提前声明路线收口，也不创建新的 numbered batch 冒充当前路线。
- route completion不能由Git-local validation receipt替代；machine validation也不能由Markdown claim替代。
- 不声称未读取的remote CI green，不用cross-compile代替非Windows runtime E2E。

## 风险与注意事项

- 不全局替换兼容`rekit` identity，不新增PowerShell runtime logic、installer或PATH fallback。
- 不为减行数合并workstream mission、external session、execution control或process lifecycle等不同安全状态机。
- authority/confirmed、heavy action、sync/promote和schema migration继续遵守exact review/gate边界。
