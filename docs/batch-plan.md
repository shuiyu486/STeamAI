# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史已拆到 `docs/batch-history.md`，旧批次只按 ID 查询。

## 实施摘要

`windows-mission-control-usability-v1` 已按 Batch 823～827 顺序完成并关闭。Windows canonical text、自然语言终态纠偏、typed ReKit invocation、执行时授权复核、维护热点拆分和完整产品验收均已通过；当前没有可领取下一批。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `windows-mission-control-usability-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `Batch 827` maintenance hotspot decomposition and full Windows acceptance |
| 状态 | `completed` |
| 唯一允许领取 | 无；路线已完成 |
| 上一批 | `Batch 826` execution-time authorization currentness 已完成 |
| 下一批 | 无；当前路线已按用户指定完成 |

### Current batch state

Batch 827 已完成：latest-batch handoff、CLI next-batch、session current-step 和 daily Reviewer correction 已按原 package owner 拆分；组合验收关闭 selector/reopen/Reviewer replay、VMP terminal closure 与 Windows cleanup identity 回归，真实 Windows live acceptance 和完整本机 minimum 通过。

### Batch 827：maintenance hotspot decomposition and full Windows acceptance

状态：已完成。

目标：行为不变地拆分四个维护热点，并完成 Batch 823～827 的 Windows 组合验收、独立终审和最终文档/commit/push 闭环。

验证结果：四项拆分、focused/full tests、真实 Windows 产品路径、公开入口、release gate、vet、module verify 与 diff 检查通过。

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan 最多保留一个 compact latest `### Batch N` 摘要；完整历史只在 `docs/batch-history.md`。
- 当前批只有focused tests、真实Windows产品路径、完整Windows本机minimum和canonical文档写回全部通过后才可完成。
- 远程CI、Linux/macOS、三平台兼容和安装包不参与当前完成判断。

## 风险与注意事项

- 本文件不是第二份 roadmap；阶段方向或批次顺序只在路线图记录，再同步本投影。
- 不把完整receipt、逐轮日志、未来批次长卡或生成inventory塞回本文件。
- 用户已授权当前路线实施；全部批次完整验收后再commit/push，不在失败状态提交完成声明。
