# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source，再按需读取 `docs/daily-product-closure-plan.md` 的共同边界与对应卡。完整历史已拆到 `docs/batch-history.md`。

## 实施摘要

`daily-product-closure-v1` 的 `DPC-01`～`DPC-04` 和四旅程整体验收均已完成；最终源码真实 Claude/adapter acceptance、missing-Claude evidence stop、全仓 tests/vet、公开 inventory 与独立终审通过。当前路线关闭且没有自动解锁的下一批；`RH-09` 保持历史 completed，`RH-10` 保持 deferred。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `daily-product-closure-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `DPC-04` IDA 索引只读 adapter 与四闭环整体验收已完成 |
| 状态 | `completed` |
| 唯一允许领取 | `DPC-04` |
| 上一批 | `DPC-03` 已完成；ordinary-directory admission 与 Windows exact rollback 已通过 |
| 下一批 | 无；等待用户明确批准新的产品路线 |

### Current batch state

`DPC-04` 与四闭环整体验收已完成：最终源码真实 acceptance `passed=true`，child terminal failure、profile provision、evidence blocker、owner-generation/currentness、missing-Claude stop 和 terminal replay 均有回归，独立终审无高置信 Critical/Important。当前只允许完成已授权的本次提交/推送与 post-push 验证，不自行选择下一路线。

### Batch 821：unified current-step external session campaign handoff and resume closure

状态：已完成。

目标：让 fresh Mission Commander / replacement executor 通过统一 `run-current-step` 消费 durable external member/reviewer session campaign，不再手工拼 nested modes。

验证结果：完成态 `release-check -Format json` 返回 `ready=true`；统一 `release-run -Format json` 以 7/7 通过；implementation commit `5e9c670` 已推送。完整记录已归档到 `docs/batch-history.md`。

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan 最多保留一个 compact latest `### Batch N` 摘要且保持短投影；完整历史只在 `docs/batch-history.md`。
- 当前批只有 focused tests、所需真实入口验收、完整 Windows 本机 minimum 和文档写回全部通过后才可完成。

## 风险与注意事项

- 本文件不是第二份 roadmap；阶段方向或批次顺序只在路线图记录，再同步本投影。
- 不把完整 receipt、逐轮日志、未来批次卡或生成 inventory 塞回本文件。
- 路线与文档不授予 commit/push；只有当前用户明确授权时才执行。
