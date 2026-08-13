# 真实使用加固与日常产品收口路线图

## 读取指南

本文件是当前已批准路线的唯一 active source，只内联当前批次卡。先由 `docs/context-routing.md` 路由到本文件；当前批涉及 adapter execution-time authorization 时，再按需读取 `internal/rekit/autonomy/**`、`internal/rekit/gate/**`、`internal/rekit/adapterhost/**` 与相邻 execution symbols。`docs/batch-plan.md` 只做短投影，历史批次按 ID 查询 `docs/batch-history.md`。

## 实施摘要

`real-usage-hardening-v1`、`daily-product-closure-v1`、`remote-control-reviewer-transport-v1` 与 `windows-mission-control-usability-v1` 均已完成。Windows 路线已依次关闭 canonical text/trusted generation、intelligent terminal correction、typed ReKit invocation、execution-time authorization currentness、maintenance hotspot decomposition 和完整产品验收。

本路线保持单向依赖和唯一状态机：自然语言入口只消费 fresh typed state，executable action 使用 typed invocation，adapter 在执行/发布边界重算 strict authorization currentness，四个维护热点按原 package 职责拆分。远程 CI、Linux/macOS、三平台兼容和安装包不属于本路线完成口径。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `windows-mission-control-usability-v1` |
| 当前批次 | `Batch 827` maintenance hotspot decomposition and full Windows acceptance |
| 状态 | `completed` |
| 唯一允许领取 | 无；路线已完成 |
| 上一批 | `Batch 826` execution-time authorization currentness 已完成并关闭 |
| 下一批 | 无；当前路线已按用户指定完成 |
| canonical 代码入口 | `internal/rekit/releasecheck/**`、`internal/rekit/cli/**`、`internal/rekit/sessionhost/**` |
| 最近完成 | Batch 827：热点拆分与完整 Windows 产品验收已完成 |

## 执行清单

1. 将 latest-batch handoff、CLI next-batch、session current-step 与 daily Reviewer correction 按既有职责移入同 package 专属文件；不改变 public command、durable schema、JSON/text contract 或状态机 owner。
2. 对拆分后的调用链运行相邻 package 回归，清理移动造成的重复声明、unused import 与旧测试 fixture，不顺手重写相邻代码。
3. 关闭组合验收暴露的高置信回归：Reviewer Apply 后 fresh blocked status、explicit rejection replay、historical committed reopen、typed selector 歧义、VMP publication-time ownership 与 authorization-drift terminal evidence。
4. 独立终审 typed invocation/reopen、VMP execution/cleanup 和 Batch 823～827 组合行为；只修复可复现的 Critical/Important finding。
5. 运行真实 Windows `rekit-host -live-acceptance`、完整 `go test ./...`、`go vet ./...`、`go mod verify`、公开 `release-check/status/packs/doctor` 与 `git diff --check`；任一失败保持 `in_progress`。
6. 验证全部通过后同步路线、短投影、历史、CHANGELOG 与 Windows-only 可用性复评，再统一 commit/push 到 `main`。

## 当前批次卡

### Batch 827：maintenance hotspot decomposition and full Windows acceptance

目标：在不改变状态机、public command或durable schema的前提下，把四个高频维护热点按现有职责边界拆成同package文件，并用完整Windows产品验收确认Batch 823～827组合行为。

验证结果：已完成；四个热点拆分、focused/full 回归、真实 Windows 产品路径、公开入口、独立终审和路线总验收均已关闭。

**用户断点**：连续四批已改善Windows文本、自然语言纠偏、typed invocation和执行时授权，但latest-batch handoff、CLI next-batch、session current-step与daily reviewer correction仍是AI维护时容易牵连相邻职责的热点；最后还缺整条路线的Windows组合验收。

**范围内**：

- 机械拆分`release_handoff_latest_batch.go`、`cli/next_batch.go`、`sessionhost/host_current_step.go`与`sessionhost/daily_reviewer_correction.go`；
- 只移动既有symbols及其私有helpers，保持package、调用方向、JSON/text contract、错误文本与测试行为；
- 每次拆分后运行对应focused package，最后运行完整Windows minimum与适用真实产品验收；
- 独立终审只接受高置信correctness/security/maintenance regression。

**范围外**：

- 不重写Mission Commander、correction/reopen、authorization或reviewer状态机；
- 不新增字段、命令、PowerShell runtime logic、heavy action或authority/confirmed写入；
- 不把机械拆分扩展成跨package abstraction或未来平台产品化。

**当前结果**：`completed`。四个热点已按原 package 职责拆分；selector/reopen/Reviewer replay、VMP publication ownership、authorization drift、orphan terminal closure 与 Windows identity-bound cleanup 回归均已关闭。真实 Windows live acceptance 通过，member/Reviewer 各 3 次完成，VMP adapter/contained child、纠偏替换、attached recovery、terminal replay 与 cleanup 全部成功；完整 Go tests、vet 和 module verify 通过。

**完成门槛**：已满足。Windows 本机 focused/full tests、真实产品路径、公开 release-check/status/packs/doctor、独立终审与 diff 检查全部通过后，路线关闭并统一 commit/push 到 `main`。

## 验证标准

- Windows 本机 `go test ./...`、`go vet ./...`、公开 `release-check/status/packs/doctor` 与 `git diff --check` 全部通过。
- profile/gate currentness 在三个时点可重算且结果一致；不存在缓存授权、transport 授权或 request-SHA 授权旁路。
- route/current/state/claim/next 与 `docs/batch-plan.md` 完全一致；冲突时 fail-closed。
- 路线后续保持 gate 决策 owner、共享 validator、adapter executor 与 publication owner 的单向依赖；失败清理不得删除非本进程 exact-owned object。

## 风险与注意事项

- validator 只判断当前授权是否仍成立，不创建、续期或迁移授权。
- 不为共享而吞掉 adapter-specific dispatch/output identity；共享层只接收已经解析的 bounded inputs并返回 typed result。
- actual heavy-tool、authority/confirmed、sync/promote和外部 transport 边界保持不变。
- 用户已授权本路线实施与必要本机验证；commit/push只在全部 Batch 823～827 完成并总验收后执行。

## 路线变更记录

- 2026-08-13：Batch 827 与 `windows-mission-control-usability-v1` 完成并关闭；四个维护热点按原 package owner 拆分，组合验收关闭 typed selector、historical reopen、Reviewer post-mutation、VMP orphan terminal closure 与 Windows cleanup identity 竞态。真实 Windows live acceptance、完整 Go tests/vet/module verify、公开入口和 release gate 通过；路线无可领取下一批，等待用户明确新路线。
- 2026-08-13：Batch 826完成并关闭；gate-owned共享只读authorization currentness validator接通generic adapter三阶段与VMP execution/publication/seal，profile/decision/owner/dispatch/output漂移均fail-closed；Windows exact-handle cleanup和Linux/Darwin no-replace canonical isolation关闭replacement误删与VMP terminal lifecycle断点，Windows真实adapter产品链、全仓Go tests/vet/diff、跨平台compile-only和独立终审通过，唯一领取推进到Batch 827。
- 2026-08-13：Batch 825完成并关闭；bounded `PublicInvocation` 成为 executable ReKit action identity，Mission Commander queue/driver request、daily/status/handoff/takeover和current-step/current-loop统一消费typed invocation与exact selected lane，多lane、unknown/blocked/stale/rebind和双selector均fail-closed；Windows临时case正反路径、全仓Go tests/vet/diff、公开入口和独立终审通过，唯一领取推进到Batch 826。
- 2026-08-13：Batch 824完成并关闭；fresh typed correction router复用既有Reviewer correction与reopen owner，terminal correction支持pending exact recovery、committed mutation-free replay、compound target currentness和多closed-lane typed choices，真实Windows公开进程零Claude launch；全仓Go tests、vet、文档/发布invariants和独立终审通过，唯一领取推进到Batch 825。
- 2026-08-12：Batch 823完成并关闭；Windows CRLF/LF source representation现在生成同一canonical new-case write set，attached/mission raw路径与durable exact-byte identity保持，fresh `vmp-re` onboarding和完整Windows本机minimum通过，独立复核无surviving Critical/Important；唯一领取推进到Batch 824。
- 2026-08-12：用户明确批准`windows-mission-control-usability-v1`，要求在Windows真实可用基础上继续改善好用度、智能化、模块清晰度和AI维护成本；路线固定为Batch 823～827的有序闭环，当前只激活Batch 823。
- 2026-08-12：`remote-control-reviewer-transport-v1` / Batch 822完成并关闭；explicit read-only Reviewer transport companion保留existing durable external-session、uncertain fencing和strict intake边界。
- 2026-08-11：`daily-product-closure-v1`完成；Windows本机真实Claude member/Reviewer、恢复、soak与四个日常产品闭环通过。
- `RH-01`～`RH-09`和旧Batch的完整实现/验证历史按ID查询`docs/batch-history.md`，active入口不重复保存。
