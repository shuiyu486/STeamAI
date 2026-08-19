# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志；本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`，且已完成：四个实现阶段 Batch 830～833 严格串行落地，并通过路线级真实验收收口。当前没有已批准的新路线；source-clone-first、Go + Claude Code 预期依赖和无 installer 是稳定边界。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `路线收口` full validation and cleanup |
| 状态 | `completed` |
| 唯一允许领取 | `无` |
| 上一批 | `Batch 833` binary-re actual analysis 已完成 |
| 下一批 | `无（等待明确新路线）` |

### Current batch state

### Batch 833：binary-re actual analysis

状态：已完成 ordinary `binary-re` actual adapter lifecycle、直接相关 fresh validation 与 implementation commit/push；路线级完整验证与清理是当前独立收口工作，不创建 Batch 834。

目标：以 `Batch 833` 作为 latest numbered implementation identity，供 Git-local machine receipt 绑定随后同批路线收口修复；`路线收口` 继续由 active route 独立表达，不冒充新产品批次。

验证结果：Batch 833 focused fresh lifecycle tests 已实际启动 embedded authorized parent/fixed child，并覆盖 strict gate、independent evidence review、accepted-only binding、terminal recovery 与 stale control fail-closed；路线级 frozen-tree release minimum、真实 Claude acceptance 和最终 Git-local receipt 仍由当前收口工作完成，本段不宣称它们已通过。

### 路线收口：full validation and cleanup

状态：已完成；当前无已批准下一批。

完成结果：默认 `binary-re` 真实 Claude gate 从 fresh 与 attached 自包含项目走通 member→独立 Reviewer rejection→zero-launch replay→人工纠偏→actual embedded adapter parent/fixed child→independent evidence review→replacement member→独立 Reviewer acceptance→completion→terminal/attached recovery；3 个 member 与 3 个 Reviewer 全部来自真实 Claude Code envelope，`manualPlaceholders=0`、`manualResultWrites=0`，fresh/attached case cleanup 均为 `removed`。README、CLAUDE、reference、config/example 与测试漂移已同步收敛；复制/移动 E2E 由 canonical full suite 覆盖。

完成证据：本段保存真实 gate 边界；frozen 7-step local minimum、exact changed artifacts、唯一 direct implementation commit 与 Git-local tracking ref 只由 v2 machine receipt/post-push inspection 判定，不由 Markdown 预写。未读取 remote CI，因此不声称 remote CI green、跨平台 runtime E2E 或 formal release。

### Locked sequence

| 工作 | 解锁条件 |
|---|---|
| 路线级完整验证与清理 | Batch 833 完成（已满足） |
| 新路线或新批次 | 由用户明确批准（当前无） |

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan 只保留一个 compact 最近完成 numbered batch handoff；更早批次只在 `docs/batch-history.md`。
- 路线 tracked completion 只是候选终态；只有 v2 machine receipt 验证 frozen fresh gates、exact artifacts、direct commit 与本地 tracking ref 后，local completion 才可信。
- `Batch 833` 只作为 latest numbered receipt identity；路线收口不创建 Batch 834，machine receipt 而非本段 prose 决定 local validation readiness。
- 不声称未读取的 remote CI green，不用 cross-compile 代替非 Windows runtime E2E。

## 风险与注意事项

- Batch 833 代码闭环已完成，但整条路线尚在验证与清理；不得把两种状态混为一谈。
- 不全局替换兼容 `rekit` identity，不新增 PowerShell runtime logic，不引入 installer 或 PATH fallback。
- authority/confirmed、heavy action、sync/promote 和 schema migration 继续遵守 exact review/gate 边界。
