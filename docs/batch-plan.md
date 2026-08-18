# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志；本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`。四个阶段严格串行：core product closure → 唯一 `binary-re` → durable pause/resume/stop → `binary-re` actual analysis。Batch 830 与 Batch 831 已完成，当前只允许领取 Batch 832；source-clone-first、Go + Claude Code 预期依赖和无 installer 是稳定边界。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `Batch 831` binary-re convergence |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 832` |
| 上一批 | `Batch 830` core product closure 已归档 |
| 下一批 | `Batch 832` |

### Current batch state

Batch 831 已完成：唯一 active `binary-re` 已吸收成熟 VMP/IDA 与通用 static/function behavior 能力，retired pack identity 在 public/attached/bundle 入口 typed fail-closed，unknown pack 保持普通 missing 语义；source-clone-first 与 current project-local verified runtime 边界已同步到 active 文档、schema、façade和 smoke。Batch 832 是唯一允许领取的下一批。

### Batch 831：binary-re convergence

状态：已完成。

目标：以成熟 `vmp-re` 为能力主体、吸收 `generic-binary-re` 的通用分析能力，发布唯一 active `packs/binary-re`；旧 identity只返回typed `pack-migration-required`，不提供alias、双写或自动迁移，并固定canonical source-clone-first产品入口。

验证结果：受影响 `packidentity`、manifest、instance、runtime/runtimebundle、adapterhost、hostcmd、onboarding、workstream、CLI与defaultdocs fresh tests通过；catalog、9-pack inventory、7个skeleton discovery、compatibility façade和current `binary-re` hash-bound真实dry-run smoke通过。最终本批只运行直接相关的公开status/packs/doctor、release-check inventory与diff gate；路线级完整fresh tests、vet、module verify、移动/复制E2E和真实Claude acceptance留到四阶段收口，不声称remote CI green。

### Locked sequence

| Batch | 目标 | 解锁条件 |
|---|---|---|
| 832 | durable pause/resume/stop 与 late-result held ledger | Batch 831 完成 |
| 833 | `binary-re` VMP/IDA actual adapter 与真实分析闭环 | Batch 832 完成 |

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan 只保留一个 compact 当前批次摘要；旧批次只在 `docs/batch-history.md`。
- 当前批次 focused tests、独立审查和临时项目 E2E 通过后才移动指针。
- 每批完成 focused/fresh local validation 后提交、推送并做 Git-local post-push inspection，再继续唯一解锁批次；四阶段结束后另做 route-level full validation和临时文档清理。不声称未读取的 remote CI green。

## 风险与注意事项

- Batch 832 已解锁为唯一下一批；Batch 833 仍 locked。不得把后续设计蓝图或 partial code 写成完成事实。
- 不全局替换兼容 `rekit` identity，不新增 PowerShell runtime logic，不引入 installer 或 PATH fallback。
- authority/confirmed、heavy action、sync/promote 和 schema migration 继续遵守 exact review/gate 边界。
