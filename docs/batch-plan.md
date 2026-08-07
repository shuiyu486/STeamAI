# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志。先由 `docs/context-routing.md` 选择场景；实施真实使用路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。旧批次只按 Batch ID 查询 `docs/batch-history.md`。

## 实施摘要

当前执行 `real-usage-hardening-v1`。RH-02 已完成并归档；当前唯一允许领取 RH-03，目标是把 Claude 不可用与结果失败收敛为 typed、可恢复、只有一个安全 next action 的诊断矩阵。本文件不复制路线规则、未来批次卡或旧验证长日志。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `real-usage-hardening-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `RH-03` Claude 不可用与结果失败的可操作诊断 |
| 状态 | `in_progress` |
| 唯一允许领取 | `RH-03` |
| 下一批 | `RH-04`，仅在 RH-03 全部验收与完整本机验证通过后解锁 |

### Current batch state

RH-03 已按既定路线解锁。当前要把 executable/auth/quota/model/spawn/timeout/permission/envelope/session/structured-output/submission/intake 等失败收敛为 stable code、truthful mutation boundary、typed terminal/replaceable/recoverable 状态与唯一安全 next action；成功分支仍只能由真实 Claude 证明。

### Batch 821：unified current-step external session campaign handoff and resume closure

状态：已完成。

目标：让 fresh Mission Commander / replacement executor 通过统一 `run-current-step` 消费 durable external member/reviewer session campaign，不再手工拼 nested modes。

验证结果：完成态 `release-check -Format json` 返回 `ready=true`；统一 `release-run -Format json` 以7/7通过；implementation commit `5e9c670` 已推送。完整记录已归档到 `docs/batch-history.md` 的 Batch 821。

## 验证标准

- 本文件与路线图的 route/current/state/next 必须一致；冲突时 fail-closed。
- active plan 最多保留一个 compact latest `### Batch N` 摘要，且文件保持短投影；完整历史已拆到 `docs/batch-history.md`。
- 当前批只有 focused tests、所需真实 gate、完整 Windows 本机 minimum 和文档写回全部通过后才可完成。

## 风险与注意事项

- 本文件不是第二份 roadmap；阶段方向或批次顺序变化只在路线图记录，再同步本投影。
- 不把完整 receipt、逐轮日志、未来批次卡或生成 inventory 塞回本文件。
- 路线与文档不授予 commit/push；只有当前用户明确授权时才执行。
