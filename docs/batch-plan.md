# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志；本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`。四个实现阶段 Batch 830～833 已严格串行完成：core product closure → 唯一 `binary-re` → durable pause/resume/stop → ordinary `binary-re` actual analysis。当前只允许路线级完整验证与清理；source-clone-first、Go + Claude Code 预期依赖和无 installer 是稳定边界。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `路线收口` full validation and cleanup |
| 状态 | `in_progress` |
| 唯一允许领取 | `路线收口` |
| 上一批 | `Batch 833` binary-re actual analysis 已完成 |
| 下一批 | `无（路线完成后等待明确新路线）` |

### Current batch state

Batch 833 已完成：ordinary goal/resume/correction在real member前复用同一project-local VMP/IDA actual adapter lifecycle；strict gate、actual embedded parent/child、report/packet/receipt/observation、independent evidence review、public acknowledgement与accepted-review后task binding形成durable闭环。terminal result、缺closure或缺binding可exact恢复，不重启child/Reviewer、不重复acknowledgement；完整terminal replay零写入，stale control fail-closed。focused tests使用harmless synthetic indexes和deterministic strict Reviewer hook，真实Claude acceptance仍由当前路线收口执行。

### 路线收口：full validation and cleanup

状态：进行中。

目标：用完整fresh tests、vet、module verify、public release/status/packs/doctor、移动/复制E2E、真实Claude member→Reviewer→correction→completion和harmless synthetic actual adapter E2E验证四阶段组合行为；核查README/CLAUDE/reference/config/examples，删除两份临时计划与评估文件并完成最终commit/push和Git-local inspection。

当前边界：不新增Batch 834或新产品功能；不把focused hook、cross-compile、本地receipt、tracking ref或Markdown claim冒充remote CI green、跨平台runtime E2E或formal release。

### Locked sequence

| 工作 | 解锁条件 |
|---|---|
| 路线级完整验证与清理 | Batch 833 完成（已满足） |
| 新路线或新批次 | 当前路线完成后由用户明确批准 |

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan只保留一个compact最近完成批次摘要；更早批次只在`docs/batch-history.md`。
- 完整fresh gates、移动/复制E2E、真实Claude acceptance、临时文件清理与Git-local post-push inspection全部通过后，路线才能标记completed。
- 不声称未读取的remote CI green，不用cross-compile代替非Windows runtime E2E。

## 风险与注意事项

- Batch 833代码闭环已完成，但整条路线尚在验证与清理；不得把两种状态混为一谈。
- 不全局替换兼容 `rekit` identity，不新增PowerShell runtime logic，不引入installer或PATH fallback。
- authority/confirmed、heavy action、sync/promote和schema migration继续遵守exact review/gate边界。
