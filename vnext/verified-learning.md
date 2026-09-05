# Verified learning 与 proof-carrying finding

本合同在现有 evidence → finding → review → learning batch 之上增加可独立复查的成熟度层级。它不让 Go 或单一模型判定研究真伪，也不建立跨 case 数据库、自动遥测或自动 promotion。

## 成熟度

- **V0 Reviewed**：current accepted evidence chain，尚无效果验证。
- **V1 Mechanically verified**：deterministic contract/regression 通过。
- **V2 Replay-backed**：独立 verifier 在绑定输入、环境、授权与预算内重现观察。
- **V3 Comparative**：通过校准的 evaluator 在冻结的 matched suite 中证明最终 candidate batch 优于 baseline，且 hard safety gates 无回归。
- **V4 Field-observed**：多个后续真实 case 在逐份用户 opt-in、脱敏和 accepted review 后支持同一效果边界。

成熟度只陈述已证明范围。产品 Gate 的实现状态与某个 artifact 的 V0–V4 maturity 是两条轴；`accepted`、`eligible`、batch `accepted`、用户 confirmation、Apply 与 Git staging 都不自行提升 maturity。相同模型的新 session 提供上下文隔离，但不构成独立认知来源；有限 synthetic suite 不证明全局或真实世界改善。

## Gate 1 — Proof-carrying finding

1. 只有重要、disputed、准备成为 learning source 或用户要求独立复现的 finding 才晋级；普通 finding 路径不变。
2. owner 创建 immutable `replay-spec`，绑定 finding SHA、输入、环境、授权、禁止动作、预算、预期观察、允许差异和停止条件。
3. replay class 为 `deterministic-readonly` 或 `sandboxed-local` 时，可由不依赖 owner session memory 的 verifier 执行；其它 class 只能人工执行或标记 `non-replayable`。
4. replay 原始输出登记为 case-local artifact，摘要写入 `replay-result`，再形成新的 evidence 与 append-only review round。
5. `not-reproduced`、`blocked`、`invalidated` 与 `inconclusive` 都是正常结果；不得重试到出现期望答案。

## Gate 2 — Evaluator calibration

calibration 不是“评估 candidate”，而是先证明评估器本身能区分已知类别。suite 至少包含：明确改善、应保持中性、明确回归、授权/安全回归，以及“表达更漂亮但证据更差”的控制项。

- scenario、rubric、expected class 与 hard safety gates 在 candidate 之外冻结并绑定 SHA；calibration control 使用逐 slot 独立 control patch，不把它误称为正式 candidate，也不得与最终 behavioral patch 共享 path/SHA。`steamai __evaluation-suite-prepare` 从 strict JSON 验证这些现有 bytes 后 no-replace 发布预注册 SuiteSpec；suite manifest 是 `evaluations/runs/<suite>.json` 的 immutable JSON，由 `steamai __evaluation-suite-finalize` 从 SuiteSpec、全部 slot run 与 Reviewer 冻结的 observed class机械生成，绑定 frozen scenario/rubric、verified-learning contract、model、Claude Code version、platform、tool profile，并列出每个唯一 `slotId`、expected class、run manifest path/raw SHA 与 bundle identity；每个 calibration request/manifest/record 也必须在执行前 exact 声明同一 `runId=slotId`、expected class 与 control patch，suite 不能事后给任意 run 贴类别；五类 controls 每类至少覆盖预注册的 2 个 initial pair slots。每个 bundle 的 scenario/rubric/runtime/tool profile 必须与 suite exact 一致；缺项、重复、额外同-suite run 或 bundle 漂移即无效。
- arms 使用相同模型、工具、权限和预算，在 case state 之外的 sibling 临时目录与无持久 session 中运行；`--safe-mode` 禁止加载 case/global customizations。blind manifest 与 arm records 不含 baseline/candidate path、patch SHA 或可反查 tree SHA，只保存随机 salted pack commitments；nonce、arm→tree SHA 与 patch SHA mapping 只写入独立 `reveal.json`。manifest 绑定 reveal SHA，Reviewer 冻结 opaque-arm 裁决后才由 Commander解盲并验证 commitments。
- deterministic assertions 先执行；任一 hard gate 失败直接 `no-go`，不交给偏好裁决覆盖。
- 初始每个 behavioral scenario 使用 2 个 matched pairs；只有结果接近且仍在预算内时增加，最多 6 pairs。不得无限重试。
- calibration 只有在所有授权回归被拒绝、所有明显回归被拒绝、中性项不过度宣称改善，并达到 suite 预注册阈值时为 `go`；否则为 `no-go` 或 `inconclusive`。
- calibration attestation 绑定 frozen suite manifest；suite manifest 必须闭合预注册 expected slots（control × pair），逐项列出全部成功/失败 run bundles、模型与工具环境及 observed class。`__evaluation-suite-finalize` 对结构完整的 `no-go`/`inconclusive` 仍发布 immutable structural closure；只有独立 Gate 3 validator 才判断 go eligibility。机械 validator 只在 improvement→improved、neutral→neutral、regression/prettier-weaker→regressed或rejected、authorization-regression→rejected 时允许 `go`；任何 inconclusive 或不匹配都 fail-closed。缺 slot、重复 slot 或未列出的 run 都使 calibration 不完整。suite、model family、tool permission profile 或核心 pack contract 实质变化后必须重新 calibration。

`no-go` 的含义是：evaluation subsystem 仍可用于研究和收集证据，但没有资格把行为 candidate 标记为 V3，也不能作为正式 learning promotion 条件。

## Gate 3 — Comparative promotion

1. 机械经验走 deterministic V1；分析方法通常至少 V2；声称改善团队/LLM 行为的经验必须 V3。
2. candidate 先冻结，再选择未被 candidate 修改的 scenario/rubric。baseline 与 candidate arms matched、并行、限预算运行。
3. Reviewer 先只看 opaque arms，再由 Commander 解盲；分歧时最多增加一名 tie-breaker，不重新生成无限 runs。
4. 只有 calibration current=`go`、hard gates=`pass`、comparative=`improved`，才能形成 promotion attestation。
5. 多 candidate thematic patch 必须评估最终完整 patch；单项分别通过不能替代组合回归。
6. `learning-batch-review` 必须绑定 promotion attestation path/SHA、calibration attestation path/SHA、suite/run bundle identity、independent reveal SHA 和被评估最终 patch SHA；promotion attestation 还必须把 reveal path exact 绑定为同一 run manifest 的 sibling `reveal.json`。原生 preview/apply 重验这些 exact bytes。

## Gate 4 — Field outcomes

- 只在后续 case 中由用户明确 opt-in 创建 field outcome；不自动监控、回传或扫描。
- outcome 仍通过 artifact/evidence/finding/review 链，记录改善、无变化、回归、竞争解释和适用边界。
- 单个 case 不升级为 V4。多个独立 current accepted outcomes 经过逐份脱敏和用户确认后，才能提出 canonical provenance 更新。
- 负向 outcome 不得删除；它使既有 V3 的适用范围收窄，必要时提出回滚或 superseding learning。

## Ownership 与 durable paths

- owner/verifier 在 `evaluations/specs/` 创建 immutable replay/evaluation specs；runner 只在 `evaluations/runs/` no-replace 发布 bundles，并仅使用 `evaluations/work/baseline` 作为显式冻结输入。
- Reviewer 只在任务指定的 exact `evaluations/attestations/<id>.md` 写 calibration/promotion attestation，不运行 arms、不修改 spec/run/candidate/patch。
- field outcome 由 Commander 在用户对该份脱敏内容 explicit opt-in 后写入 `evaluations/outcomes/`，再走普通 evidence/finding/review 链。

## Run bundle 与本地 runner 边界

runner 是一次性、无状态的机械执行器，只负责：验证 strict JSON request、在 case state 之外创建 sibling 隔离目录、冻结输入、为每次 run 生成不可预测 opaque arm labels、并行启动 matched Claude Code `--print --safe-mode --no-session-persistence` arms、施加 tools/permissions/model/time/USD budget、捕获成功、失败、超时、正常中断取消、CLI `is_error`、reported cost 超预算和无效输出状态、计算 SHA、no-replace 发布 immutable blind bundle 与独立 reveal。Windows arm 以 suspended process 启动，先加入 `KILL_ON_JOB_CLOSE` Job Object，再以 `PROCESS_SUSPEND_RESUME` 权限恢复，避免进程在加入 Job 前派生未受控子进程。`__evaluation-run` 遇到非 completed/pass arm 时必须先发布失败 bundle，再返回 typed nonzero outcome；调用方不能把非零误解为“没有证据”。正常 cancel/timeout 以终止整个 arm process tree 为目标，但 Windows 的无逃逸 process-tree 行为仍须 live gate 证明；进程硬杀/断电无法保证发布 cancellation bundle。失败结果也是证据，必须随 stdout/stderr 和状态发布，不能靠丢弃失败 run 改写结论。它不做 research judgment、candidate 选择、Reviewer 决策、自动 Apply、commit 或 push。

自动 runner 只接受 synthetic、无凭据、无网络、无真实目标且仅允许 `Read` 的 scenario。需要 Bash/Edit/外部服务/heavy action 的验证保持人工或 environment-bound，不能通过扩大 runner 权限绕过用户确认。
