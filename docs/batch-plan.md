# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志；本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`。四个阶段严格串行：core product closure → 唯一 `binary-re` → durable pause/resume/stop → `binary-re` actual analysis。Batch 830 已完成，当前只允许领取 Batch 831；source-clone-first、Go + Claude Code 预期依赖和无 installer 是稳定边界。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `Batch 830` core product closure |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 831` |
| 上一批 | `Batch 829` GitHub repository identity migration 已归档 |
| 下一批 | `Batch 831` |

### Current batch state

Batch 830 已完成：Identity v2 copy/move、project-local promote/no-reparse、public projection、ordinary-directory adoption、release truth、actual adapter root binding 和真实 Claude 产品链均已按本批范围验证；Batch 831 是唯一允许领取的下一批。Batch 830 不在本次提交前提前复制到 `docs/batch-history.md`。

### Batch 830：core product closure

状态：已完成。

目标：让自包含项目的 identity、复制/移动恢复、project-local promote、public projection、ordinary-directory adoption、release truth 和真实 Claude member→Reviewer→correction→completion 路径在真实调用链上闭合。

验证结果：Identity v2 copy/move 与旧 current v1 relocation fail-closed、project-local promote no-reparse、受影响 `promote`/`cli` 包 fresh 测试、真实 Claude 全链一次通过、第二次 gate 的 Reviewer 语义拒绝与 cleanup truth 均已如实验证；最终本机 release minimum 只以冻结工作树生成的 fresh machine receipt 判定。本批不声称 remote CI green，本地 receipt 和 tracking ref 仅证明 Git-local validation/publication truth。

### Locked sequence

| Batch | 目标 | 解锁条件 |
|---|---|---|
| 831 | 唯一 active `binary-re`，旧 pack typed migration-required | Batch 830 完成 |
| 832 | durable pause/resume/stop 与 late-result held ledger | Batch 831 完成 |
| 833 | `binary-re` VMP/IDA actual adapter 与真实分析闭环 | Batch 832 完成 |

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan 只保留一个 compact 当前批次摘要；旧批次只在 `docs/batch-history.md`。
- 当前批次 focused tests、独立审查和临时项目 E2E 通过后才移动指针。
- 每批完成 focused/fresh local validation 后提交、推送并做 Git-local post-push inspection，再继续唯一解锁批次；四阶段结束后另做 route-level full validation和临时文档清理。不声称未读取的 remote CI green。

## 风险与注意事项

- Batch 831 已解锁为唯一下一批；Batch 832/833 仍 locked。不得把后续设计蓝图或 partial code 写成完成事实。
- 不全局替换兼容 `rekit` identity，不新增 PowerShell runtime logic，不引入 installer 或 PATH fallback。
- authority/confirmed、heavy action、sync/promote 和 schema migration 继续遵守 exact review/gate 边界。
