# 真实使用加固与日常产品收口路线图

## 读取指南

本文件是当前已批准路线的唯一 active source，只内联当前批次卡。先由 `docs/context-routing.md` 路由到本文件；当前批的 transport contract 与操作边界再按需读取 `docs/agent-team-usage.md` 的 “Remote Control Reviewer transport companion” 小节。`docs/daily-product-closure-plan.md` 只保留上一条已关闭路线的历史完成证据，不是当前批入口。

## 实施摘要

`real-usage-hardening-v1` 与 `daily-product-closure-v1` 已完成 Windows 本机真实 Claude member、Reviewer、恢复、soak 和四个日常产品闭环。用户随后明确批准把 Claude Code Remote Control 多会话通信吸收到架构中；当前唯一 active route 是 `remote-control-reviewer-transport-v1`，以一个中型闭环把它实现为 durable external-session 的可选 read-only Reviewer transport companion，而不是默认 provider、第二套 session runtime或新的 durable identity。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `remote-control-reviewer-transport-v1` |
| 当前批次 | `Batch 822` optional Remote Control read-only Reviewer transport companion |
| 状态 | `completed` |
| 唯一允许领取 | `Batch 822` |
| 上一批 | `daily-product-closure-v1` / `DPC-04` 已完成并关闭 |
| 下一批 | 无；Batch 822 完成后等待用户明确批准新的产品路线 |
| canonical 使用说明 | `docs/agent-team-usage.md` 的 “Remote Control Reviewer transport companion” 小节 |
| 最近完成 | Batch 822：explicit durable Reviewer opt-in、self-contained evidence bundle、endpoint/delivery/return lineage、uncertain fencing、relay→strict intake全链、完整 Windows 本机 release minimum 与独立高强度复审均已通过 |

## 执行清单

1. 从源码、public command、临时 case 或真实进程重验当前断点；文档陈述不算证据。
2. 只实现当前卡及其必要支撑；不顺手扩展 member transport、自动 Remote Control 登录/配置、跨平台 product path、测试性能或相邻产品问题。
3. 保留 Go-owned currentness、WhatIf/hash-bound Apply、strict intake、Human-in-the-Lane 和无未授权 heavy action 边界。
4. 人话 action 只从 fresh typed result/status 派生，不持久化，不反向决定 runtime 路由。
5. 运行当前卡 focused/真实入口验收，再运行 Windows 本机 release minimum；任何失败都保持 `in_progress` 或标记 `blocked`。
6. 当前卡全部通过后才更新本指针并解锁下一卡，同时同步 `docs/batch-plan.md` 的短投影。

## 当前批次卡

### Batch 822：optional Remote Control read-only Reviewer transport companion

**用户断点**：Claude Code 已提供跨会话 `ListAgents` / `SendMessage`，但 ReKit 不能直接把临时聊天 endpoint当成长久成员、把一次工具返回当作可靠launch/completion，或要求另一台机器读取本机case路径。若只写评估而不进入durable external-session、evidence和strict intake闭环，这项能力对Mission Commander不可真实使用。

**范围内**：

- 只允许durable Reviewer dispatch显式使用`ReviewerHarness=claude-code-remote-control`与caller-generated durable `ReviewerSession` opt-in；local Windows `sessionhost`继续默认`claude-code-cli`；
- 从canonical current member evidence manifest生成deterministic、content-addressed、完整内联的UTF-8 evidence closure；missing/extra/drift、duplicate/case-fold collision、path escape、symlink/reparse、oversize、binary和本机case root泄漏全部fail-closed；
- 将`ListAgents name [ref]`记录为immutable opaque endpoint snapshot，将一次exact `SendMessage`记录为`accepted|rejected|uncertain` delivery observation；accepted/rejected只能精确派生既有launch receipt，uncertain禁止自动重发和same-job replacement；
- inbound `ReviewerResult`使用current generation destination，按result first→transport-return receipt second→submission last发布；relay独立重验bundle/endpoint/delivery/launch/source/result lineage，并进入既有Reviewer completion/source/stage/collect/strict intake/writeback路径；
- typed Mission Commander discovery/delivery/launch/return/new-dispatch handoff、对抗回归、canonical文档和Windows本机deterministic acceptance闭合。

**范围外**：

- 不把Remote Control设为默认provider，不支持member/heavy-tool transport，不把chat session或`name [ref]`写入lane owner；
- 不自动登录claude.ai、修改全局provider/settings、安装WSL、启动真实Remote Control session、调用真实跨机器`ListAgents`/`SendMessage`或发送消息；
- 不承诺exactly-once、FIFO、offline queue、stable endpoint TTL、reconnect或Remote Control process supervision；
- 不伪造原生Windows live cross-machine E2E，不扩大authority/confirmed、heavy action、sync/promote或外部副作用授权。

**当前结果**：`completed`。生产实现、focused Remote Control 全链、`memberexecution` / `externalsession` / `cli` / `sessionhost` 四包完整回归、全仓 `go test ./... -count=1`、`go vet ./...`、`git diff --check`、公开 `release-check` / `status` / `packs` / `doctor` 与 canonical 文档写回均通过；完成态 `release-check -Format json` 返回 `ready=true`，10 个 pack inventory有效。独立代码终审无surviving finding，独立文档复核的两项Important路由漂移已修复。

**关闭边界**：本卡完成只证明ReKit transport契约和Windows deterministic product path闭合，不证明Claude Code/claude.ai外部服务的delivery guarantee或原生Windows live跨机器支持；不自动解锁其它产品路线。

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

- 2026-08-12：用户明确批准吸收Claude Code Remote Control多会话能力，`remote-control-reviewer-transport-v1` / Batch 822成为唯一active route；实现限定为explicit read-only Reviewer transport companion，保留durable lane/session identity、local provider默认值、uncertain fencing、existing relay/intake与Windows truthful acceptance边界。
- 2026-08-11：`daily-product-closure-v1` 完成。最终源码真实 `vmp-re` live acceptance 为 `passed=true`，3 个真实 member 与 3 个独立 Reviewer 完成，fixed adapter child、profile revoke、evidence acknowledgement、exact-generation binding、terminal replay、attached recovery 和 cleanup 全部通过；missing-Claude direct/daily evidence stop、currentness 竞态与失败/漂移边界回归通过，独立终审无高置信 Critical/Important。Active route 关闭为 `completed` 且无下一批，等待用户明确选择新路线。
- 2026-08-10：`DPC-03` 完成 ordinary-directory 五类只读 admission、`directory-adoption-required`、manifest/source/target 绑定的 stable init hash、Windows create-only exact rollback 和 cleanup truthful outcome；focused、分组全仓 tests、vet、公开命令、真实 sentinel/doctor 及两轮只读复审通过，解锁 `DPC-04` 为唯一 active batch。
- 2026-08-10：`DPC-02` 的真实 member/Reviewer/completion-correction、attached 两个 cutpoint 恢复、同 goal 零启动 completion recovery 和 terminal replay 全部通过；解锁 `DPC-03` 为唯一 active batch。
- 2026-08-09：用户批准 `daily-product-closure-v1` 四闭环方案，`DPC-01` 成为唯一 active batch；`RH-09` 保持历史 completed，`RH-10` 保持 deferred。
- 2026-08-09：`RH-09` retry-aware Windows fresh soak 首次尝试 3/3、recovery 与 7/7 cleanup 通过；完整成功和失败历史见 `docs/batch-history.md`。
- `RH-01`～`RH-09` 的完整实现、真实验收和验证历史按 RH ID 查询 `docs/batch-history.md`，active 入口不重复保存。
