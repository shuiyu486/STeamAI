# STeamAI verified learning v1 路线

## 读取指南

- 路线 ID：`steamai-verified-learning-v1`
- 当前状态：`mechanical implementation and independent review passed; live calibration pending`
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
| 2 | strict runner、预注册独立 control patches、SuiteSpec prepare/run/finalize、contract/runtime binding、case-state 外 sibling arms、Windows suspended→Job→resume、salted blind commitments + exact reveal、failure bundle、actual cost capture | fake-Claude production lifecycle + Windows native test binary | implemented；真实 timeout/process-tree 与 calibration `pending` |
| 3 | candidate claim floor、behavioral V3 fail-closed、blind-decision exact file、attestation/run/reveal path closure、final patch binding、TOCTOU rebuild | learningbatch focused tests | implemented；真实 comparative journey `pending` |
| 4 | per-case explicit opt-in outcome、negative append-only、no cross-case discovery、V4 provenance path | contract/template tests | implemented；多个真实后续 case evidence `pending` |

## Calibration controls

真实 calibration suite 在 candidate 外冻结，至少包含：clear improvement、neutral、clear regression、authorization regression、prettier but weaker evidence。每个 behavioral scenario 初始 2 matched pairs，只在预注册接近区间内增加，最多 6；deterministic hard gate 先行，不能由偏好覆盖。所有 expected slots 和所有成功/失败 bundle 必须进入 calibration attestation。suite、model family、tool profile 或核心 pack contract 实质变化后重新 calibration。

只有所有授权/明显回归被拒绝、中性项不被过度宣称、预注册阈值通过时为 `go`；否则 `no-go` 或 `inconclusive`。`no-go` 不禁用研究 runner，但禁止 behavioral candidate 获得 V3 或进入正式 promotion。

## Gate 3 机械闭包

任一 candidate 声明 `behavioral→V3` 时，完整 thematic patch 必须同时绑定：

- calibration attestation raw SHA，literal decision=`go`，并绑定闭合所有预注册 slots 的 immutable suite manifest；
- promotion attestation raw SHA，literal hard safety=`pass`、comparative=`improved`、maturity=`V3`；
- candidate blind run bundle manifest raw SHA、semantic identity，以及 promotion attestation 中 exact sibling `reveal.json` path/SHA；calibration attestation 的 reveal path/SHA 为 literal `none`；
- 解盲前 Reviewer immutable blind-decision path/SHA、blind identity 与 preferred opaque arm，解盲后该 arm 必须等于 candidate arm；
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

## 下一步

1. 先落地固定开关 `STEAMAI_VERIFIED_LEARNING_LIVE_CALIBRATION=1` 的 focused live test，覆盖认证/预算/controls fail-closed、真实 timeout 与 Windows process-tree cleanup。
2. 再在 case 外 synthetic fixture 中执行显式真实 Claude calibration；如实记录 `go`、`no-go` 或 `inconclusive`。
3. calibration 非 `go` 时保持 Gate 3 behavioral promotion fail-closed。
4. 真实 V4 只能等待多个后续 case 自愿产生证据，不为完成路线制造或模拟。
