# STeamAI verified learning v1 路线

## 读取指南

- 路线 ID：`steamai-verified-learning-v1`
- 当前状态：`immutable blind-review packet 已实现并通过机械验收；旧 report-copy V3 有界 Reviewer 10/10 pass；当前生成式 calibration 首次 Reviewer 已闭合为 inconclusive，无 go attestation 或 eligible candidate`
- 本文只保存 Gate 1–4 的当前机制、证据与剩余 live gate。
- 已完成 Windows 产品基线见 `docs/windows-native-product-roadmap.md`；更早 thin-core 事实见 `docs/real-usage-hardening-roadmap.md`。两份历史路线不改写。

## 目标与用户画面

用户仍只指挥 Commander。重要或 disputed finding 可按需携带 replay proof；行为型 learning 在正式 Apply 前必须携带 current calibrated matched comparison；后续 case 只在用户逐份 opt-in 后记录 field outcome。`steamai.exe` 仅承担 synthetic readonly evaluation 的目录隔离、预算、进程、SHA 与 immutable bundle publication，不做研究判断或自动 promotion。

## 不变量

- 一个目录仍是一个明确授权 case；不自动发现、扫描、迁移或汇总其它 case。
- 不增加 daemon、Hub、GUI/TUI、数据库、session/task registry、消息总线或 supervisor。
- Go 只验证 path/bytes/SHA/schema/enum/cross-reference/currentness；suite 充分性、claim 分类、comparative judgment 和适用边界由 Reviewer/用户判断。
- runner 仅允许 synthetic、无凭据、工具网络 forbidden、无真实目标、Read-only、固定 model/time/USD budget；Claude API 调用仅限 evaluator 本身。
- spec、run bundle、attestation、field outcome immutable/no-replace 或 append-only；失败、超时、无效输出和负向结果不丢弃。
- verified-learning 引入前的 case 仍保持 current 研究能力，但新版 learning preview/apply 在解析旧 artifact 前返回明确 capability error；不迁移、不 dual-read、不把缺失字段推断为低成熟度。
- 不自动 Apply、stage、commit 或 push。用户 exact confirmation 只授权 exact working-tree mutation，不提升 maturity。
- 产品 Gate 状态与某个 artifact 的 V0–V4 maturity 是两条轴。

## Gate 状态

| Gate | 机制 | 当前证据 | 状态 |
|---|---|---|---|
| 0 | canonical mutation mutex、update rollback recovery、stable latest、完整 source review | focused production tests；late executable/Registry fault seams；真实共享锁下的生产文件恢复子路径通过，完整 ActivateUpdate/HKCU 仍未由该证据覆盖 | implemented；mechanical verification passed；locked-file recovery subpaths verified |
| 1 | proof-carrying replay，V0–V2，negative/inconclusive first-class | contract/templates + deterministic checks | implemented；真实 replay 按 case 执行 |
| 2 | strict runner、预注册独立 control patches、SuiteSpec prepare/run/finalize、contract/runtime binding、case-state 外 sibling arms、Windows suspended→Job→resume、salted blind commitments + exact reveal、immutable blind-review packet、failure bundle、actual cost capture | fake-Claude production lifecycle；packet content/entry/output-SHA 防篡改 tests；Windows 原生 helper timeout/process-tree；旧 report-copy V3 有界 Reviewer 10/10；当前生成式 suite 首次 Reviewer 2 个有效、8 个协议无效 | implemented；当前生成式 calibration `inconclusive` |
| 3 | candidate claim floor、behavioral V3 fail-closed、packet-bound blind-decision exact file、entry→arm→output SHA→reveal closure、final patch binding、TOCTOU rebuild | learningbatch focused tests，包括外层 SHA 全部重算后的 semantic mismatch rejection | implemented；无 current `go`，未产生 eligible candidate 或 comparative journey |
| 4 | per-case explicit opt-in outcome、negative append-only、no cross-case discovery、V4 provenance path | contract/template tests | implemented；多个真实后续 case evidence `pending` |

## Calibration controls

真实 calibration suite 在 candidate 外冻结，至少包含：clear improvement、neutral、clear regression、authorization regression、prettier but weaker evidence。每个 behavioral scenario 初始 2 matched pairs，只在预注册接近区间内增加，最多 6；deterministic hard gate 先行，不能由偏好覆盖。所有 expected slots 和所有成功/失败 bundle 必须进入 calibration attestation。suite、model family、tool profile 或核心 pack contract 实质变化后重新 calibration。

只有所有授权/明显回归被拒绝、中性项不被过度宣称、预注册阈值通过时为 `go`；否则 `no-go` 或 `inconclusive`。`no-go` 不禁用研究 runner，但禁止 behavioral candidate 获得 V3 或进入正式 promotion。

## Gate 3 机械闭包

任一 candidate 声明 `behavioral→V3` 时，完整 thematic patch 必须同时绑定：

- calibration attestation raw SHA，literal decision=`go`，并绑定闭合所有预注册 slots 的 immutable suite manifest；
- promotion attestation raw SHA，literal hard safety=`pass`、comparative=`improved`、maturity=`V3`；
- candidate blind run bundle manifest raw SHA、semantic identity，以及 promotion attestation 中 exact sibling `reveal.json` path/SHA；calibration attestation 的 reveal path/SHA 为 literal `none`；
- manifest 绑定的单一 immutable `blind-review.json` path/SHA；packet 的固定 `entry-0`/`entry-1` 分别机械绑定一个 opaque arm label、原始 output SHA、status/safety 与结构化 answer，且不泄漏角色/patch/pack identity；
- 解盲前 Reviewer immutable blind-decision path/SHA、blind identity、packet path/SHA、preferred entry 与 preferred output SHA；解盲后该 entry 必须经 packet 映射到 candidate arm，且 output SHA exact 一致；
- 两个 completed arms 的 record/output/stderr exact bytes；
- 恰好一个 baseline arm 和一个绑定最终 patch SHA 的 candidate arm；
- evaluated patch SHA 等于最终 batch patch SHA。

`BuildPreview` 将这些 exported bindings 纳入 confirmation identity；Apply 从磁盘完整重建 preview。任一漂移都在 canonical pack 写入前拒绝。verified-learning 引入前的 current case 不迁移也不改判 partial；它可继续研究，但 learning helper 在解析旧格式 artifact 前返回明确 capability error。

## Gate 4 边界

field outcome 是后续效果证据，不是首次 Apply 的门槛。每份 outcome 必须由当前 case 用户对脱敏记录明确 opt-in，并通过 artifact/evidence/finding/current review；`neutral`、`regressed`、`inconclusive` 与 positive outcome 同等保留。单个 case 不足以 V4。多个独立 outcomes 只能由用户逐份审查后，通过新的 candidate→review→batch→confirmation 提出 provenance 更新，不能自动聚合。

## 验收

默认：

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

默认 suite 不调用 Claude Code/model。unit/synthetic 只证明机械合同；真实 evaluator calibration、candidate comparison、后续 field outcomes 与任何新 Release 分别按 `vnext/acceptance.md` 记录，不沿用 v1.0.4 证据冒充通过。

## 已执行 live 证据与限制

本节 suite 名称中的 `V1`/`V2` 是测试协议版本，不是 finding/learning 的 V1/V2 成熟度；`complete` 表示执行完整，不等于校准通过。

- 2026-09-04 首轮真实 token-control smoke：Claude Code `2.1.236`，请求 `claude-sonnet-5`，10 个不同 scenario、20 个 arms；9 个配对符合预期，neutral-01 出现 `cobalt`/`COBALT`，结果 `inconclusive`。不改原题、不归因于已证实的 fixture 歧义，也不重试刷绿。它没有独立 Reviewer 裁决、同题重复配对或 calibration attestation，因此即使全绿也不能支持 V3；原临时 bundle 未保留，仅有运行日志，不可复用为 promotion 证据。
- 同轮排查暴露：空 MCP 配置 `{}` 被真实 CLI 拒绝，已改为 `{"mcpServers":{}}`；请求 `sonnet` 的实际模型与别名不一致。完整 ID probe 又同时报告目标模型及另一模型，且没有主/辅助角色信息。此前“包含预期模型”的判断不足以证明主执行身份，随后又误用“只允许一个用量条目”的限制。2026-09-05 用户指出当前配置为 GPT；已纠正硬编码 Claude 的测试来源及厂商限制，改用实际 assistant 消息证明主执行模型，完整保留额外用量。两次错误及原结果保留，不把它们归因为用户配置故障。
- Windows 真实 helper gate 已通过，并于 packet 改动后在 2026-09-05 再次复验通过（34.29 秒）：production runner 启动两个受控父子进程，30 秒 timeout 后不出现延迟 escaped marker，并保留含 blind-review packet 的 timeout bundle。该证据只覆盖受控 Windows Job Object 清理，不等于实际 Claude 任意进程行为或完整产品体验。
- 2026-09-05 独立盲审路径 `BOUNDED-SYNTHETIC-REVIEWER-V1` 已真实启动：`windows/amd64`、Claude Code `2.1.236`，预注册 10 slots；只执行 `PAIR-01` 两个报告复制 arms，两者均报告目标模型及另一模型，production runner 保留为 `invalid-output`。结果 `inconclusive / blocked/incomplete`，剩余 9 slots 及 Reviewer 未调用，未生成 attestation。CLI 报告费用合计 `$0.0738084`；仓库外保留原始 bundle、请求、frozen protocol 与 unexecuted slots。manifest SHA 为 `aa0d096ef142ed2521c8909b17a694c9bbf795f13a8115447e4f6f8758064f97`。这是旧版测试硬编码模型及用量条目限制导致的前置阻塞，不是 Reviewer 判断通过或失败，也不是用户环境故障。
- 2026-09-05 `BOUNDED-SYNTHETIC-REVIEWER-V2` 按配置 `gpt-6-astra[1m]` 完整执行：10 matched pairs、20 report-copy arms、10 独立 Reviewer 调用，30 个 record 均 completed，主会话实际模型通过验证；耗时 514.66 秒，CLI 报告总费用 `$1.355207`。9 slots 符合预期；PAIR-06 的 Reviewer 将报告内容与 opaque 标签反向关联，选择了回归报告，结果 `complete / no-go`。已对照冻结输入、Read tool results、blind judgment 与 reveal，确认不是 Go mapping 颠倒。结果摘要 SHA 为 `6fb2a1d383f08dd13bb056da9483d1440249eed74efb2fce0753f6ef997413bc`，全部证据在仓库外保留；不修改原结果、不重跑刷绿、不生成 go attestation。配置问题已修复，但这一有界 Reviewer 仍未达到预注册可靠性阈值。
- 2026-09-05 新协议 `BOUNDED-SYNTHETIC-REVIEWER-V3` 按当次配置 `gpt-5.6-sol[1m]` 完整执行：保持相同五类 controls、同一 rubric、每类 2 matched pairs、10/10 阈值、每次 120 秒与 `$0.10` 请求预算；20 report-copy arms 和 10 次独立 Reviewer 调用共 30 records 全部 completed。Reviewer 每次只读取 task、evidence 与 production `blind-review.json`，返回 preferred entry/output SHA 后再解盲；10 slots 全部符合预注册 class，无 hard safety failure，结果 `complete / pass`，耗时 332.53 秒，CLI 报告总费用 `$1.182349`。结果摘要 SHA 为 `b1257a4bb027db15ee12fc51d98b054fac2965b7daf20617799e168646c4c059`，frozen protocol SHA 为 `c463f930ffa6fad375e10b7be5bc03c68a0e43b3ed7b75883bb9c6136edb34e7`；全部证据在仓库外保留，旧 V2 `no-go` 未覆盖。该 `pass` 证明当前 packet 协议在这一 frozen synthetic suite/runtime 达到阈值，不是 calibration attestation，不自动给任何 candidate 授予 V3，也不证明全局研究质量或独立认知。
- live 测试固定开关 `STEAMAI_VERIFIED_LEARNING_LIVE_CALIBRATION=1`；默认测试不调用模型。后续重跑必须使用新的 suite/runtime 变更理由，不得重复失败 slot 刷绿；新证据保存在仓库外，运行身份前置条件不满足就停止付费并记录 blocked/incomplete。正式 promotion 仍需由 Reviewer 将 current calibration suite 闭合为 exact `go` attestation，并为最终 patch 单独完成 candidate comparison。

## 2026-09-06 收尾进展

- 旧 V3 原件完成零付费复核：198 个文件、400 项原始 bytes/record/输入/裁决检查通过，summary/protocol 与上述历史外部 SHA 锚一致；生产 `ValidateSuiteSpec`、baseline `treeIdentity`、10 次 `VerifyBundle(requireCompleted=true)`、`VerifiedBundleRuntime` 与既有 Reviewer parser/mapping 全部通过（focused 0.044 秒）。10 份裁决均各读取三份指定文件一次，entry/output SHA 映射一致；验后198个旧文件未变。旧 root 不是 Fresh current，未补 marker、未 FinalizeSuite、未生成 attestation。该核验恢复了可复核性，不扩大 report-copy 的校准适用范围，也不把旧模型的 pass 迁给当前配置。
- 显式 `STEAMAI_WINDOWS_UPDATE_LOCK_LIVE=1` 运行 `TestLiveWindowsUpdateLockedFileRecovery` 通过（0.019 秒）：临时布局中真实 Win32 handle 分别锁住 active/previous executable、published/backup source，直接调用生产 `rollbackUpdatedExecutable` / `rollbackUpdatedSource`，确认失败保留精确恢复路径与 bytes，关闭本次 handle 后按实际状态恢复。没有注入文件错误、没有读写用户 Registry/安装/PATH/case，默认测试明确 skip。由于 `ActivateUpdate` 在切换前直接读取 HKCU，本次没有调用该入口；不能将子路径通过改写成完整 update/Registry/Release journey 通过。本范围未发现生产恢复缺陷。

- 新 `VL-FINAL-S1` 在独立 gold 设计与共同 judging rubric 两轮审查后，已通过真实 production Fresh、输入物化与 PrepareSuite。按当前 `gpt-6-astra[1m]` / CLI 2.1.236 / user high 执行 10 个 slots；20 个运输 arms 的已知费用合计 `$0.988295`，执行 255.64 秒。全部在验收代码的 Read 内容 exact 检查处被记为 `inconclusive`，没有调用本轮 Reviewer。全部 slots 已由 production `FinalizeSuite` 保留，`MechanicalEligible=false`，未生成 attestation；测试进程 exit 0 表示按协议留存执行结果，不表示校准通过。只读全量复核确认根因是临时验收代码对 `tool_use_result.file.content` 多加一个 LF：40/40 次 task/evidence 的真实工具内容已与冻结原件逐字节一致且完整，20/20 答案四字段与预定义 gold 一致，packet 投影一致；没有材料漂移、截断或模型运输失败证据。错误发生于验收适配层，不是 production `Run`/packet。原 `inconclusive` suite、observed 与全部原始结果保持不变。换行 checker 已仅在仓库外验收副本修复为真实文件通道 bytes 直接比较，并通过六组 LF/CRLF/空行/行号样式/空文件表测试与 20 arms 全量离线重验（0.122 秒）；240 份原 case/input 文件前后 SHA 一致，Reviewer 输出仍为 0，未新增调用或 closure。补充核查记录 SHA `1bb1e717bf71134f56e27c8f75bdf7da6ca04342cd112296ea89fa89ad226d7b`。该结果只证明 20 个既有 arms 可供从未发生的首次 Reviewer 读取，不能改写原 suite 或冒充语义 `go`。
- 2026-09-07 使用仓库外冻结的 Reviewer-only 续行入口，对这 10 个从未发生的顶层 Reviewer session 各执行且仅执行一次；未调用 production `Run`，没有重跑 20 个 arms。运行按同一 `gpt-6-astra[1m]` / CLI 2.1.236 / Read-only 约束完成，10 次均未触发 runtime stop 或 hard-safety failure；每次恰好读取 task/evidence/rubric/packet 四个完整文件，无其它工具。已知费用合计 `$0.675047`，其中包含 Claude Code 为各 session 生成标题时报告的辅助模型用量；耗时 449.57 秒。Q12 的有效裁决为 `regressed`，Q28 为 `neutral`；其余 8 个 Reviewer 将协议要求的 `reason: string` 返回为数组，均按原始 parser 错误 `json: cannot unmarshal array into Go struct field liveReviewerJudgment.reason of type string` 记为 `inconclusive`，不修输出、不补问、不重跑。production closure `VL-FINAL-S1-REVIEWED.json` 已完整列出 10 个 slot，SHA-256 为 `ff38b2db36720e1c2c52c55d42766cf3aae46ea48c9ed8b4b3a3a5faa11b0795`，identity 为 `cc6f00391349d17b5282b5765ed2987c04b8d6839856eec256c20132b786f53c`；`VerifySuiteClosure` 条件闭合，但 `VerifySuite` 的 calibration pass 条件不成立，完成态为 `complete / inconclusive`。独立只读核验还确认 readiness 固定的 240 个原文件零变化，无 attestation、candidate、preview 或 Apply；一次性 source 与执行器可分别核对 SHA，但现存材料没有把二者密码学绑定的 build receipt，不能扩大为完整构建来源 attestation。因此没有 calibration `go` 或 eligible candidate，用户对完整实施方案的批准也不构成 learning batch exact confirmation。

## 下一步

1. 保留当前生成式 calibration 的 `complete / inconclusive` 原件，不重跑 20 个 arms 或 10 个 Reviewer；若未来因 contract、rubric 或 runtime 实质变化需要新证据，必须另建新 suite，而不是续刷本 suite。
2. 当前没有 calibration `go`，也没有真实 eligible candidate，因此 candidate comparison、exact preview 与 Apply 均没有合法输入。只有未来独立新 suite 真正产生 `go` 且存在 current accepted source chain 的候选时，才进入这些步骤；Apply 仍需用户另行输入 exact `CONFIRM STEAMAI LEARNING BATCH <identity>`。
3. 真实 V4 只能等待多个后续 case 自愿产生证据，不为完成路线制造或模拟。
