# 真实使用加固与日常产品收口路线图

## 读取指南

本文件是当前已批准路线的唯一 active source，只内联当前批次卡。先由 `docs/context-routing.md` 路由到本文件；当前批的详细设计再按需读取 `docs/daily-product-closure-plan.md` 的共同架构边界与对应 `DPC-*` 章节，不预读其余批次全文。

## 实施摘要

`real-usage-hardening-v1` 已完成 Windows 本机真实 Claude member、Reviewer、恢复和 soak 门槛。`daily-product-closure-v1` 也已用四个中型闭环把既有底层能力收成可试日用的产品路径，并完成最终源码真实 Claude/adapter 验收与独立终审；当前路线已关闭，不自动选择 installer、GUI、第二个成熟 pack 或其它后续路线。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `daily-product-closure-v1` |
| 当前批次 | `DPC-04` IDA 索引只读 adapter 与四闭环整体验收已完成 |
| 状态 | `completed` |
| 唯一允许领取 | `DPC-04` |
| 上一批 | `DPC-03` 已完成；ordinary-directory admission、hash-bound create-only init 与 Windows exact rollback 已通过 |
| 下一批 | 无；等待用户明确批准新的产品路线 |
| 详细设计 | `docs/daily-product-closure-plan.md` 顶部完成证据；不作为新路线入口 |
| 最近完成 | 最终源码真实 `vmp-re` acceptance `passed=true`；contained adapter、独立 evidence review、member/Reviewer lineage、terminal replay、attached recovery、missing-Claude evidence stop 与独立终审全部通过 |

## 执行清单

1. 从源码、public command、临时 case 或真实进程重验当前断点；文档陈述不算证据。
2. 只实现当前卡及其必要支撑；不提前实现后续 `DPC-*`，不顺手修测试性能或相邻产品问题。
3. 保留 Go-owned currentness、WhatIf/hash-bound Apply、strict intake、Human-in-the-Lane 和无未授权 heavy action 边界。
4. 人话 action 只从 fresh typed result/status 派生，不持久化，不反向决定 runtime 路由。
5. 运行当前卡 focused/真实入口验收，再运行 Windows 本机 release minimum；任何失败都保持 `in_progress` 或标记 `blocked`。
6. 当前卡全部通过后才更新本指针并解锁下一卡，同时同步 `docs/batch-plan.md` 的短投影。

## 当前批次卡

### DPC-04：IDA index 只读 adapter 终审修复与整体验收

**用户断点**：成功 live receipt 已证明固定 IDA TSV 查询能进入日常 completion/correction 链，但终审发现 child 失败终态、public profile capability、rejected evidence review 和 observation 后 takeover 仍有 fail-closed 缺口；不关闭这些边界就不能把 DPC-04 标记完成。

**范围内**：

- child timeout/nonzero/invalid stdout 写 dispatch-bound failed/aborted report、零 artifact receipt、failed observation并 exact revoke；同一 dispatch terminal replay 零 child；
- public profile provision 只允许 `vmp-re` + `inspect` + 内容寻址 request + fixed budget/output/stop contract；
- rejected evidence review 保持可见 blocker，只有 accepted/superseded closure 才允许消费；
- TaskBinding 必须绑定 dispatch owner executor/generation，observation 后 takeover 不能把旧 evidence 重绑到新 generation；
- focused tests与真实 DPC-04 acceptance 全部通过后，再进入四闭环整体验收与发布收口。

**范围外**：

- 不新增 workflow engine、durable schema、Go package、public command 或 generic profile provision；
- 不启动 IDA、不执行 catalog entry、不联网、不打开 IDB、不 rename/comment/patch/debug/dump；
- 不把 installer、GUI、第二个成熟 pack、远程三平台 CI 或 OS 级网络隔离混入本批。

**完成结果**：`completed`。Child 失败终态、profile capability、rejected evidence blocker、owner-generation binding、evidence-review currentness 与 missing-Claude host/daily stop 均有对抗回归；最终源码相关包和全仓 tests/vet、真实 `vmp-re` acceptance、公开 inventory 与独立终审通过。真实 receipt 为 `passed=true`，profile revoke、terminal replay、attached recovery 和 cleanup 均闭合。

**关闭边界**：本卡完成不授权新的产品路线。Installer、GUI、第二个成熟 pack、远程跨平台专项和 OS 级网络隔离继续等待用户明确选择；不得从本卡自行领取。

## 验证标准

- 用户自然语言 → thin canonical skill → fresh typed status/daily result → 单一人话 action；展示层不成为 durable truth。
- 查询只读；开始/继续只进入现有 daily owner；纠偏只进入现有 correction owner；sync/promote 和 heavy action 继续分别受 review-first 与 strict authorized-gate 约束。
- route、current、state、claim、next 与 `docs/batch-plan.md` 完全一致；冲突时 fail-closed。
- focused tests、真实自然语言验收和项目约定的 Windows 本机 release minimum 全部通过后，当前卡才可标记 completed。

## 风险与注意事项

- 不根据文件是否存在、`FinalState` 文本或错误字符串重建 mission 路由；稳定 action 只摘要 typed result，typed failure 原样保留。
- 自动进入 skill 不等于写入授权；“继续”不能扩展为 sync/promote、profile provision、gate、authority/confirmed 或 heavy action 授权。
- 不新增 PowerShell runtime logic；默认产品路径继续 Go-native。
- 不把真实样本、trace、dump、capture、payload、客户信息、绝对 case 路径或 case-specific artifact 写入仓库。
- 路线批准只授权实施当前闭环，不自动授权 commit、push 或其它外部副作用。

## 路线变更记录

- 2026-08-11：`daily-product-closure-v1` 完成。最终源码真实 `vmp-re` live acceptance 为 `passed=true`，3 个真实 member 与 3 个独立 Reviewer 完成，fixed adapter child、profile revoke、evidence acknowledgement、exact-generation binding、terminal replay、attached recovery 和 cleanup 全部通过；missing-Claude direct/daily evidence stop、currentness 竞态与失败/漂移边界回归通过，独立终审无高置信 Critical/Important。Active route 关闭为 `completed` 且无下一批，等待用户明确选择新路线。
- 2026-08-10：`DPC-03` 完成 ordinary-directory 五类只读 admission、`directory-adoption-required`、manifest/source/target 绑定的 stable init hash、Windows create-only exact rollback 和 cleanup truthful outcome；focused、分组全仓 tests、vet、公开命令、真实 sentinel/doctor 及两轮只读复审通过，解锁 `DPC-04` 为唯一 active batch。
- 2026-08-10：`DPC-02` 的真实 member/Reviewer/completion-correction、attached 两个 cutpoint 恢复、同 goal 零启动 completion recovery 和 terminal replay 全部通过；解锁 `DPC-03` 为唯一 active batch。
- 2026-08-09：用户批准 `daily-product-closure-v1` 四闭环方案，`DPC-01` 成为唯一 active batch；`RH-09` 保持历史 completed，`RH-10` 保持 deferred。
- 2026-08-09：`RH-09` retry-aware Windows fresh soak 首次尝试 3/3、recovery 与 7/7 cleanup 通过；完整成功和失败历史见 `docs/batch-history.md`。
- `RH-01`～`RH-09` 的完整实现、真实验收和验证历史按 RH ID 查询 `docs/batch-history.md`，active 入口不重复保存。
