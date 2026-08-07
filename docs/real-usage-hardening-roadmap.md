# 真实使用加固路线图

## 读取指南

本文件是 `real-usage-hardening-v1` 的 active source，只内联当前批次卡。先由 `docs/context-routing.md` 路由到本文件；不要默认读取未来批次，只有当前批完成后才按解锁条件读取 `docs/real-usage-hardening-backlog.md` 中的下一张卡。

## 实施摘要

真实 Claude member/reviewer host 已存在，当前路线负责把固定验收链收敛为普通 public 产品路线，再依序降低用户操作、补失败恢复并扩大真实能力面。批次数不是目标；没有用户可感知断点时不得新增批次，也不得临时创造字段、summary、projection 或 receipt 微批次。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `real-usage-hardening-v1` |
| 当前批次 | `RH-03` Claude 不可用与结果失败的可操作诊断 |
| 状态 | `in_progress` |
| 唯一允许领取 | `RH-03` |
| 下一批 | `RH-04`，仅在 RH-03 全部验收与完整本机验证通过后解锁 |
| 未来卡 | `docs/real-usage-hardening-backlog.md`，日常不要默认读取 |
| 最近完成 | `RH-02`；target + goal/correction 日常前门、真实 signed-Claude 链与完整 Windows 本机 minimum 通过 |

## 执行清单

1. 从源码、public command、临时 case 或真实进程重验当前断点；文档陈述不算证据。
2. 只实现当前卡范围；支撑字段并入闭环，不独立立批。
3. deterministic/path/schema 测试可用明确 fixture；凡声称 member output、`ReviewerResult` 或 LLM 成功的验收必须启动真实 `claude.exe`。
4. 保留 Go-owned currentness、WhatIf/hash-bound Apply、strict intake、Human-in-the-Lane 和无未授权 heavy action 边界。
5. 运行当前卡要求的 focused/live gate，再运行 Windows 本机 release minimum；环境不可用时如实 `blocked`/`failed`。
6. 所有门槛通过后才标记 completed、从 future backlog 提升明确解锁的下一张卡，并同步 `docs/batch-plan.md` 短投影。

## 当前批次卡

### RH-03：Claude 不可用与结果失败的可操作诊断

**用户断点**：当前 host 能失败/replacement，但用户面对的主要是进程错误和有界 diagnostics，尚无稳定的“发生了什么、是否已写入、下一步怎么做”故障矩阵。

**范围内**：覆盖 executable 缺失、未登录/鉴权失败、配额或模型不可用、spawn 失败、timeout、permission denial、nonzero exit、invalid envelope、session ID mismatch、invalid structured output、submission/intake 失败；返回 typed terminal/replaceable/recoverable 状态和唯一安全 next action。

**范围外**：不模拟 LLM success，不实现 provider fallback，不绕过权限。

**focused 验收**：纯进程/解析失败可用 deterministic failure fixture；任何成功分支仍用真实 Claude；至少一轮 baseline live success 证明失败分类未破坏正常链。

**真实证据**：每个故障都有 stable code、truthful mutation boundary、replacement/retry 判断和用户可执行恢复；attempt 上限后不会循环。

**停止/升级条件**：环境本身无法制造某个 provider-side failure 时记录 `not-observed`，不能把预期当实测；不得用假的 LLM JSON 覆盖该缺口。

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

- 2026-08-07：RH-02 完成 target + natural-language goal/correction 的 Go-owned 日常前门。最终 signed `claude.exe` gate 真实覆盖 fresh、attached、generation 1→2 correction replacement、独立 Reviewer、evidence-bound completion、terminal/exact-goal zero-launch replay和 cleanup；`manualPlaceholders=0`、`manualResultWrites=0`、`packageMutations=0`。pending completion、receipt publication truthfulness、symlink/junction/ancestor replacement 和 archived fail-closed finding 均由回归与独立终复核关闭，完整 Windows minimum通过后按既定顺序提升 RH-03。
- 2026-08-07：RH-01 将 trusted launch 修复为 handle-bound WinTrust / PE version 与 direct native `NtOpenFile` actual-image binding；mismatched path/handle、suspended mismatch、native `SameFile` 回归与独立安全终审关闭原 Critical。最终 fresh public-route real-Claude gate和完整 Windows minimum 通过后，按既定顺序提升 RH-02。
- 2026-08-07：按文档渐进式披露不变量，将 RH-02～RH-10 移至单一按需 backlog；顺序、验收和授权边界不变。
- 2026-08-06：建立 `real-usage-hardening-v1`，当前只解锁 RH-01。
