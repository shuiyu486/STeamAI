# 真实使用加固路线图

## 读取指南

本文件是 `real-usage-hardening-v1` 的 active source，只内联当前批次卡。先由 `docs/context-routing.md` 路由到本文件；不要默认读取未来批次，只有当前批完成后才按解锁条件读取 `docs/real-usage-hardening-backlog.md` 中的下一张卡。

## 实施摘要

真实 Claude member/reviewer host 已存在，当前路线负责把固定验收链收敛为普通 public 产品路线，再依序降低用户操作、补失败恢复并扩大真实能力面。批次数不是目标；没有用户可感知断点时不得新增批次，也不得临时创造字段、summary、projection 或 receipt 微批次。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `real-usage-hardening-v1` |
| 当前批次 | `RH-09` Windows 日常试用与稳定性门槛 |
| 状态 | `completed` |
| 唯一允许领取 | 无；当前路线已按用户指定完成 |
| 下一批 | 无；`RH-10` 已按用户决定保持 `deferred`，当前不实施 Linux/macOS product path |
| 未来卡 | `docs/real-usage-hardening-backlog.md`，仅保留 deferred 指针，日常不要默认读取 |
| 最近完成 | `RH-09`；retry-aware 最新 fresh soak 首次尝试即 3/3、100%、recovery 通过、7/7 disposable case 真实创建并删除 |

## 执行清单

1. 从源码、public command、临时 case 或真实进程重验当前断点；文档陈述不算证据。
2. 只实现当前卡范围；支撑字段并入闭环，不独立立批。
3. deterministic/path/schema 测试可用明确 fixture；凡声称 member output、`ReviewerResult` 或 LLM 成功的验收必须启动真实 `claude.exe`。
4. 保留 Go-owned currentness、WhatIf/hash-bound Apply、strict intake、Human-in-the-Lane 和无未授权 heavy action 边界。
5. 运行当前卡要求的 focused/live gate，再运行 Windows 本机 release minimum；环境不可用时如实 `blocked`/`failed`。
6. 所有门槛通过后才标记 completed；只有存在明确解锁且未 deferred 的下一张卡时才从 future backlog 提升，并同步 `docs/batch-plan.md` 短投影。

## 当前批次卡

### RH-09：Windows 日常试用与稳定性门槛

**用户断点**：单次 gate 通过不等于可连续日常使用，尤其是长路径、清理、Claude 配额波动、重复请求和多个 case 接力。

**范围内**：在 Windows 连续运行至少 3 个 bounded、无敏感真实任务，覆盖 fresh case、existing case、人工纠偏、一次故障恢复和 terminal replay；输出一份仓库外机器 receipt 汇总成功率、人工底层输入数、耗时、durable member replacement、process replacement 和 cleanup。

**范围外**：不为了提高成功率放宽 strict intake 或预先生成 LLM 结果；不把 receipt 中的绝对临时路径提交进仓库；不实施已 deferred 的 RH-10 Linux/macOS product path。

**focused 验收**：完整 Windows release minimum；显式真实 soak gate。

**真实证据**：所有 case cleanup；`manualResultWrites=0`；用户仍不填写 ID/时间/路径/SHA；失败必须分类而非从统计中删除。

**停止/升级条件**：若 3 次中任一产品链失败，回到对应已完成批次修复并记录 reopen 理由；RH-10 保持 `deferred`。

**当前结果**：retry-aware 最新 fresh Windows soak 首次尝试即通过：task 3/3、task success=100%、attempt 3/3、attempt success=100%、`retriedTasks=0`、recovery 五个 cut point 通过、`cleanupExpected=cleanupCreated=cleanupRemoved=7`、`manualPlaceholders=0`、`manualResultWrites=0`、provider failure=`not-observed`。只有 `reviewer-semantic-or-lineage` 失败允许一次 fresh-case retry，首次失败仍进入 attempt 统计和全部历史；provider/contract/cleanup/timeout 等失败不自动 retry。此前 0/3 聚合假阴性、修复后 3/3、final2/final3 的 2/3 Reviewer reject 与单 pack 诊断均已保留在 `docs/batch-history.md`，没有用成功结果覆盖失败。RH-09 现为 `completed`，RH-10 继续 `deferred`。

## 验证标准

- 主链：自然语言目标 → public 产品入口 → 真实 Claude member → 自动收取真实结果 → 人工纠偏 → replacement member → 独立真实 Reviewer → evidence-bound completion。
- 测试代码不得手工生成、改写或伪装 member output、`ReviewerResult` 或其它 LLM 内容；`manualPlaceholders=0`、`manualResultWrites=0` 是 live success 必要条件。
- 每批都要有用户可感知 before/after、focused 自动测试、必要真实 gate、完整本机验证和仓库外 receipt；缺一项保持 `in_progress` 或 `blocked`。
- 路线图与 `docs/batch-plan.md` 的 route/current/state/next 必须一致；冲突时 fail-closed。

## 风险与注意事项

- 不新增 PowerShell runtime logic；默认产品路径继续 Go-native。
- 不把真实样本、trace、dump、capture、payload、flag、客户信息、绝对 case 路径或 case-specific artifact 写入模板仓库；临时 case 必须清理。
- heavy/debug/patch/dump/hook/network/exploit replay 只在 strict durable autonomy profile 与 `authorized-gate` 完整覆盖时执行。
- 本文件只保留当前卡；未来批次不回填到 active 入口，完成卡的长结果进入 `docs/batch-history.md`。
- 路线不授权 commit/push 或其它外部副作用。

## 路线变更记录

- 2026-08-09：RH-09 在 bounded semantic retry 与 selected-pack prompt 稳定性修复后完成；最新 fresh final4 首次尝试即 task/attempt 3/3、五阶段 recovery 与 7/7 cleanup 通过，未消费 retry。历史 0/3 与两轮 2/3 失败仍保留，RH-10 继续 `deferred`，当前路线到此结束。
- 2026-08-09：RH-09 在 cleanup/provider truth修复后的末轮 fresh soak 出现新的真实 `web-security` replacement Reviewer rejection，结果2/3且已保留；路线按停止条件 reopen 为`in_progress`。五阶段 recovery、cleanup truth与零手写结果仍通过，RH-10继续`deferred`。
- 2026-08-09：RH-08 完成跨 pack 真实 Claude session 兼容并提升 RH-09；完整证据见 `docs/batch-history.md`。
- RH-01～RH-07 的完整实现、真实验收和验证历史按需读取 `docs/batch-history.md`；active 入口不重复保存。
