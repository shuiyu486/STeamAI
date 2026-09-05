# STeamAI verified learning v1 路线

## 读取指南

- 路线 ID：`steamai-verified-learning-v1`
- 当前状态：`immutable blind-review packet 已实现并通过机械验收；V3 有界 Reviewer calibration 10/10 pass；正式 behavioral V3 仍需 go attestation 与 candidate comparison`
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
| 0 | canonical mutation mutex、update rollback recovery、stable latest、完整 source review | focused production tests；late executable/Registry fault seams + recovery-path checks；真实 Windows locked-file path 仍待 live 验收 | implemented；mechanical verification passed |
| 1 | proof-carrying replay，V0–V2，negative/inconclusive first-class | contract/templates + deterministic checks | implemented；真实 replay 按 case 执行 |
| 2 | strict runner、预注册独立 control patches、SuiteSpec prepare/run/finalize、contract/runtime binding、case-state 外 sibling arms、Windows suspended→Job→resume、salted blind commitments + exact reveal、immutable blind-review packet、failure bundle、actual cost capture | fake-Claude production lifecycle；packet content/entry/output-SHA 防篡改 tests；Windows 原生 helper timeout/process-tree；V3 有界 Reviewer 10/10 | implemented；calibrated for frozen V3 synthetic suite/runtime |
| 3 | candidate claim floor、behavioral V3 fail-closed、packet-bound blind-decision exact file、entry→arm→output SHA→reveal closure、final patch binding、TOCTOU rebuild | learningbatch focused tests，包括外层 SHA 全部重算后的 semantic mismatch rejection | implemented；真实 comparative journey `pending` |
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

## 下一步

1. 模型选择与主执行身份证据、Reviewer 内容—标签对应协议均已修复，新 ID 的 V3 有界验收已 10/10 `pass`；旧 V2 `no-go` 保留不改。下一步由独立 Reviewer 将该 frozen suite 的完整证据闭合为 exact calibration `go` attestation，不能把 test-local `pass` 直接改名为 `go`。
2. 只有 current calibration attestation 真正为 `go` 后，才执行最终完整 candidate patch 的真实 comparative journey；产品 Gate 状态或 synthetic calibration `pass` 不直接给任何 learning 授予 V3。
3. 真实 V4 只能等待多个后续 case 自愿产生证据，不为完成路线制造或模拟。
