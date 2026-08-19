# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志；本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-product-optimization-v1`。四个阶段严格串行：core product closure → 唯一 `binary-re` → durable pause/resume/stop → `binary-re` actual analysis。Batch 830～832 已完成，当前只允许领取 Batch 833；source-clone-first、Go + Claude Code 预期依赖和无 installer 是稳定边界。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `Batch 832` durable execution control |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 833` |
| 上一批 | `Batch 831` binary-re convergence 已归档 |
| 下一批 | `Batch 833` binary-re actual analysis |

### Current batch state

Batch 832 已完成：public `control` 以per-lane append-only generation和exact preview stamp/hash发布durable pause/resume/stop；result birth、relay/claim/writer progression与local supervisor均重验同一binding。pause不做OS suspend，stop先durable提交、只由exact local supervisor owner关闭自己持有的containment；actuation失败不回滚stopped，process termination不是durable成功判据，opaque Remote Control session不受本路径管理。Batch 833是唯一允许领取的下一批。

### Batch 832：durable execution control

状态：已完成。

目标：从自然语言或`/steamai`安全控制exact lane，让paused/stopped/旧generation结果保留raw truth与held/late receipt但不能推进live output、Reviewer、completion或checkpoint；control不授予authority/confirmed、gate或heavy action。

验证结果：executioncontrol、CLI、externalsession、member/reviewer progression和sessionhost focused fresh tests覆盖状态机、current/legacy单写、dual-root拒绝、sticky held/late、三层consumer竞争、replacement project lease与durable-first Windows Job actuation；façade smoke和32-command release/default-doc inventory通过。路线级完整fresh tests、vet、module verify、移动/复制E2E与真实Claude acceptance留到四阶段收口，不声称remote CI green。

### Locked sequence

| Batch | 目标 | 解锁条件 |
|---|---|---|
| 833 | `binary-re` VMP/IDA actual adapter 与真实分析闭环 | Batch 832 完成（已满足） |

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan只保留一个compact最近完成批次摘要；更早批次只在`docs/batch-history.md`。
- Batch 833 focused tests、临时项目E2E与真实分析证据通过后才移动指针。
- 每批完成 focused/fresh local validation 后提交、推送并做 Git-local post-push inspection，再继续唯一解锁批次；四阶段结束后另做 route-level full validation和临时文档清理。不声称未读取的 remote CI green。

## 风险与注意事项

- Batch 833 已解锁为唯一下一批；不得把其设计蓝图或partial code写成完成事实。
- 不全局替换兼容 `rekit` identity，不新增PowerShell runtime logic，不引入installer或PATH fallback。
- authority/confirmed、heavy action、sync/promote和schema migration继续遵守exact review/gate边界。
