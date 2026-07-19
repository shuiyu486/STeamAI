# Agent Team 使用与兼容指南

## 读取指南

如果你只是维护本仓库，先读根目录 `CLAUDE.md`、`README.md` 和本文件即可；需要理解长期路线时再读 `docs/vision.md`。

如果你正在具体安全 case 中工作（当前成熟示例是 `vmp-re` RE case），先在 case 目录用 `/rekit status` 确认绑定，再按本文件选择新 case、旧 case、主线或功能支线流程。

本文件只说明通用使用方式和兼容策略，不记录真实样本名、RVA/VA、trace/dump、artifact 路径或 case-specific 进度。

## 实施摘要

新方案不是替换旧 case 的大迁移，而是在现有 `/rekit`、pack、case shim、工作线机制上增加 Agent Team 的组织方式：

- 用户入口仍然是 `/rekit`。
- 旧 case 可继续使用 `.re-template.yml`，也可以通过 `/rekit attach` / `/rekit repair` 补齐 `.rekit/instance.yml`。
- 主线和功能支线仍然保留，而且是新架构的核心协作单元。
- `sync` / `promote` 仍然 review-first，写入前需要确认具体范围。
- Agent Team 当前主要是 context、workflow、tooling、ledger、gate 的底座，不代表已经全自动脱壳、全自动逆向、自动漏洞挖掘、自动恶意样本分析或通用自动渗透。

推荐心智模型：

```text
kit 仓库 = runtime + packs + common policies + tooling 经验
case 目录 = 具体目标/样本/项目状态 + 工作线 + 证据 + 候选结论
主线 = 收敛、确认、长期 handoff
功能支线 = 专项探索、证据收集、候选结论
```

## 执行清单

### 新安全 case（当前以 `vmp-re` 为例）

1. 在 kit 仓库启动 Claude Code。
2. 使用 `/rekit init -Target <caseRoot> -Pack vmp-re -ProjectName <caseName> -Apply`。
3. 进入 case 目录启动 Claude Code。
4. 执行 `/rekit status` 和 `/rekit overview`。
5. 用 `/rekit continue main` 接手主线。
6. 需要专项分析时，用 `/rekit start <name>` 创建功能支线。
7. 每轮结束用 `/rekit handoff` 或 `/rekit handoff <name>` 生成接手文档。

### 旧 case

1. 在 kit 仓库或 case 目录确认当前绑定：`/rekit status`。
2. 如果还没有 `.rekit/instance.yml`，用 `/rekit attach -Target <caseRoot> -Pack vmp-re -Apply` 补齐 metadata 和 case-local shim。
3. 执行 `/rekit sync` 生成同步审查包。
4. Claude 复核冲突、收益和覆盖范围后，再由用户确认是否执行写入型 `sync -Apply`。
5. 执行 `/rekit doctor` 验证结构。
6. 执行 `/rekit overview`，再选择 `/rekit continue main` 或 `/rekit start <name>`。

### 日常工作

优先用自然语言让主 Agent 统筹 mission，而不是让用户手动记命令：

```text
继续推进当前 mission。
总体怎么样？哪些 lane 卡住了？
让我进入 verifier lane 帮它纠错。
这个 lane 上下文污染了，生成接手包，让新会话接手。
把这次可复用经验整理成 promote 候选。
```

主 Agent 会按需组合底层 runtime API：

```text
/rekit overview              # 看项目总览，不选择工作线
/rekit continue main         # 接手主线
/rekit start unpacking       # 创建/进入功能支线
/rekit continue unpacking    # 继续功能支线；若存在 open intervention 会要求先 reconcile
/rekit reconcile unpacking   # 将用户干预显式 reconcile 到 durable lane state
/rekit handoff               # 生成项目级接手索引
/rekit handoff main          # 生成主线接手文档
/rekit handoff unpacking     # 生成功能支线接手文档
/rekit sync                  # kit -> case，默认只生成 review
/rekit promote               # case -> kit，默认只生成 review
```

## 验证标准

- `/rekit status` 能正确显示 kit/case 绑定。
- `/rekit doctor` 通过，且 managed docs、policy、tooling 文件预算未超限。
- 旧 case 同步前先看到 `.rekit/reviews/<timestamp>-sync/summary.md`、`packet.json` 和 bounded diff。
- `overview` 能显示主线、功能支线、共享事实统计和 Mission Control brief；brief 必须让主 Agent 不读完整 ledger 也能看到 ready/blocked lanes、pending gates、authorized gates、open decisions、interventions、next agent actions 与 escalations。overview JSON/text 还应暴露逐 lane `laneExecutorActions[]`、`missionCommanderActions[]` action index（state/primary/follow-up/boundary）、顶层 `executionEvidenceReview[]` 与主 Agent 可直接按序消费的 `missionCommanderNextActions[]`（source/blocked/requiresReview/reasons/boundary/command），并让 next steps 与 next actions 先处理 execution evidence review、reconcile、pending gate、open decision，只为 ready lane 建议 continue，避免 blocker summary、evidence review queue 与继续建议冲突。overview、start/continue/handoff/gate/reconcile JSON envelope、continue run artifacts、project handoff、lane handoff、lane `RESUME.md` 与 typed `checkpoints/latest.json` 应使用同一 Go mission snapshot / blocker 语义：只有 pending gate、effective open intervention、open candidate/decision 会让对应 lane blocked；start/gate/reconcile JSON envelope/text、continue JSON envelope/run artifacts、overview/project/lane handoff JSON/Markdown 与 lane-local resume/checkpoint 还应持久化或投影同一 lane Mission Control brief 与 executor action snapshot，包含 blocked/ready、基于 typed facts 的 pending gate / open intervention / open decision counts、blocker reasons、reconcile/pending-gate/open-decision requirements、resume/handoff command、next actions、escalations 与 `missionCommanderAction` state/prompt/primary/follow-up/boundary；project handoff 文本、lane handoff 与 `RESUME.md` 必须让主 Agent或替换 executor 不回查 JSON 也能直接看到 commander primary/follow-up/boundary；`gate -WhatIf` 应同时暴露当前 executor action 与写入 request 后的 would executor action；`authorized-gate` 只是已记录的 durable autonomy authorization decision，应在 `missionBrief.authorizedGates`、overview、handoff、continue digest/status、gate executor action 与 lane-local resume/checkpoint gate snapshot 中可见，并直接携带 execution boundaries、gate `eventId` 与可复制 report contract handoff，让替换 executor 不重扫 request ledger 即可读取 `gate -ExecutionReportContract`；`gate -ExecutionReportContract` 与 `gate -ValidateExecutionReport` JSON 还应暴露 adapter sidecar `missionCommanderAction`（validation/record/repair state、primary/follow-up commands、read-only/no-heavy/no-authority boundary）与 `missionCommanderNextActions[]`（read-only validate、valid=true 后 record、handoff、repair hints、rerun validation 的 source/reasons/boundary/command），让主 Agent 先确认 `valid=true` 再记录 bounded observation evidence，contract 阶段不得把 blocked record handoff 当作可立即执行，invalid/missing sidecar validation 不得推荐 `-Apply` record；`gate -Apply -GateEventId ...` 的 execution evidence record result 也应暴露 evidence-specific `missionCommanderAction`，区分 `ready-for-evidence-review`、`evidence-already-recorded` 与 `needs-main-escalation`，并同步输出 `executionEvidenceReview[]` 与 `missionCommanderNextActions[]` / CLI text `mission commander next action` lines，让主 Agent 在刚记录 bounded observation evidence 后不用再跑 overview/handoff/continue/resume/checkpoint 就能消费 evidence-first handoff/overview/continue ordering；duplicate execution evidence 必须把 `evidence-already-recorded` 与 `duplicate record did not append observation evidence` boundary 投影到 review queue 和 next actions，只保留 review handoff/overview，不推荐 `/rekit continue ... -WhatIf` 或 lane-level `missionCommanderActions` continue；且先 review output/evidence refs、保持 no-heavy/no-authority/confirmed boundary；但它不作为 pending-gate blocker；project/lane handoff JSON、project/lane handoff Markdown、lane `RESUME.md` 与 typed `checkpoints/latest.json` 还应投影已记录 authorized execution observation evidence 的 `executionEvidenceReview[]`/review checklist，包含 gate eventId、status、action、outputRefs/evidenceRefs、review/handoff command、`missionCommanderAction` state/primary/follow-up、no-replay/no-authority boundary，并让 boundary-hit/escalated evidence 明确进入 main review 且不推荐 autonomous continue；`promote -CreateCandidates` 的 `reviewPlan` 还应暴露 pack-memory candidate `missionCommanderAction`、`decisionChecklist[]`、`mainAgentExecutionPlan[]` 与 `missionCommanderNextActions[]`，让主 Agent 可区分 WhatIf preview、actual candidate review 与 blocked/no-op plan，并按 per-candidate review/accept/reject/cleanup/verification checklist 处理；`cleanupTargets[]` 应携带 candidate cleanup action 与 `indexPath` 处理，`mainAgentExecutionPlan[]` 应把 materialize、review decisions、cleanup、pack doctor、fresh-case reconsume 与 attached-case reconsume 收口为 bounded commands/expected/evidence/boundary 且明确 runtime 不执行 merge/cleanup/init/doctor，`missionCommanderNextActions[]` 应把 decisionChecklist review、candidate cleanup、pack doctor、fresh-case init/doctor 与 attached-case doctor 投影为 source/reasons/boundary/command 的可消费 next-action list，CLI text 也应输出 promote candidates cleanup/reconsume 和 `mission commander next action` lines；`reconsume.verificationChecklist[]` 应在 accepted tooling merge 后列出 doctor、fresh case 与 attached case reconsume 的 commands/expected/evidence/boundary；被 `resolvesEventId` resolution 关闭的 intervention 不应继续阻塞。
- overview 与 project/lane handoff 的 top-level `nextSteps[]` 应直接提升 `executionEvidenceReview[]` 的 commander guidance：先 review gateEventId 对应 outputRefs/evidenceRefs，再执行 evidence handoff；如果 review queue 中存在 boundary-hit/escalated/escalation evidence，则必须显示 main-review stop guidance，并抑制 autonomous `continue` 建议，即使 authorized-gate 本身仍是非阻塞且 lane executor action 为 ready。overview text/JSON、project/lane handoff JSON/Markdown、`/rekit continue` JSON/run artifacts、lane `RESUME.md` 与 typed `checkpoints/latest.json` 的 `missionCommanderNextActions[]` / `Mission Commander next actions` 投影还应把 evidence review commander primary/follow-up 与 lane commander primary action 收口到单一 next-action list：evidence review source 排在前面，blocked mission 或 evidence main-review 时过滤 `continue` follow-up，保留 review/reconcile command、reason 与 no-replay/no-authority boundary，避免主 Agent 或替换 executor 从 `executionEvidenceReview[]`、`missionCommanderActions[]`、`laneExecutorActions[]`、`executorAction` 和 `nextSteps[]` 多处手工拼接执行顺序。project handoff Markdown 的逐 lane 行、lane handoff 新会话开场、continue run digest 与 lane-local resume/checkpoint 也必须同步这一优先级：先列 evidence next action，把 ready lane continue 降级为 review 后候选，并明确当前不要执行 autonomous continue。
- `note -List` 与 note append strict duplicate eventId 去重 / lane guard、gate lane guard、doctor JSONL validation、workstream board snapshot、shared facts 写入/读取路径、facts path/read/append、continue facts promotion 与 duplicate eventId scan、handoff/continue facts snapshot、gate duplicate eventId、note ledger kind 顺序、workstream lane/workspace local JSONL 文件集合与 path helpers、facts JSONL / fact-file mapping、JSONL append、board 读取、open-lane 过滤和 known-lane 诊断应复用同一 Go helper source，避免 note、overview、doctor、handoff、start/continue/gate 对 ledger kind/file/path、workstream local JSONL file/path、board JSON、JSONL/facts reader/append 或 lane guard 维护并行实现。note 写路径还应返回 shared typed action delta：`-WhatIf` 输出当前 `executorAction` 与内存模拟 append 后的 `wouldExecutorAction`，actual append 输出 post action，duplicate eventId 只输出未改变的 current action；candidate、defer/open decision、open intervention 与 pending-gate request 应分别投影 open-decision、intervention 与 pending-gate blocker，非 blocker kind 保持 readiness 不变，malformed ledger 必须在投影或写入前 fail-closed。
- `continue main` 与 `continue <name>` 明确接手不同工作线；无参数 `continue` 不应在多工作线时盲猜。`start` / `handoff` / `continue` / `reconcile` apply 后的 `nextSteps`、CLI text 与 handoff Markdown 必须使用 lane-local executor actions：blocked lane 不得推荐 continue 或泄漏其它 ready lane 的 continue，ready lane 才建议自己的 continue，paused/closed/unready lane回到 handoff/read-only。
- 功能支线只写自己的 workspace、outbox、candidate/request，不直接写 confirmed CSV、routine IR 或长期 handoff。
- 长期成员身份绑定 lane，不绑定旧 session；旧会话上下文污染或用户希望重开时，新会话应读取 handoff / packet / evidence 接手同一 lane。需要显式登记或接管当前 lane executor 时，主 Agent使用 `/rekit start <name> -Apply -Executor <session-id> -Actor <actor> -Reason <reason>` 写入 `currentExecutor`、`executorGeneration` 与 takeover metadata；当 `<name>` 解析到已有工作线（例如 `main`、`feature-login` 或 `login`）时，start 进入该 durable lane 而不是新建并行 lane，因此 replacement session 可用 `/rekit start main -Apply -Executor <new-session> ...` 接手主线并刷新 resume/checkpoint/events；runtime 只记录和投影该声明，不自动 spawn、停止或监控 session。
- 用户可随时进入 lane 打断、纠错、改向或硬切模型；lane 继续时要用 `/rekit reconcile <name> -InterventionId <eventId> -Apply` 将干预写成 append-only resolution event，并刷新 durable lane executor/resume/checkpoint/board state。
- `plan-subagents` planning mode 只写 review artifacts，不自动 spawn reviewer；`packet.json` / `summary.md` 的 `ownerBinding`、`shardHandoffs[]` 与 `reviewerOrchestration` 提供 target lane current executor / generation / last takeover snapshot、read-only dispatch prompt、strict `reviewerResultContract`、decision/conflict mapping、`reviewerIntakeCommands`、writeback sequence、多 reviewer dispatch/result root、lifecycle 与 result/intake completion criteria。主 Agent调度短命 reviewer后负责把包含 `reviewerSession` 的单个 JSON object 放入生成的 `reviewerResultPath`，再调用 reviewer-intake `-WhatIf/-Apply`；runtime 严格绑定 packet/route/shard/items，验证 lane owner binding、case-local evidence 与 route output contract，若 packet 要求的 executor generation 已被 takeover 则写 facts 前 fail-closed，按 verification-before-decision 顺序写 facts，并返回 `orchestrationSnapshot`、overview/handoff/doctor post-validation。verification / decision events 与 lane handoff 会记录 reviewer session provenance 与 owner binding snapshot。reviewer 不写文件或 ledger；intake 不写 authority/confirmed、不执行 heavy-tool，最终 merge decision 仍由主 Agent拥有。
- confirmed / authority 写入仍需要更严格 gate；lane 文档/packet 只表达授权意图，动态调试、注入、patch、dump、hook、full trace、网络、exploit replay 等外部副作用只有在 strict durable autonomy profile + `authorized-gate` decision 完全覆盖 action、exact target、typed budget、stop conditions、output paths、record/notify 和 grant/expiry 时才可由 executor 执行，否则必须升级。

## 风险与注意事项

- 不要把 `docs/vision.md` 中的长期目标理解为当前已经具备全自动脱壳、全自动逆向、自动漏洞挖掘、自动恶意样本分析或通用自动渗透能力。
- 不要在 kit 仓库里创建真实 case state；验证 `init/attach/sync/promote` 时只用临时 case。
- 不要将真实样本、客户信息、RVA/VA、trace/dump、artifact 路径或绝对 case 路径写回 pack 模板。
- `attach` 只绑定 metadata 和 shim，不会直接同步 managed docs；旧 case 接入后还需要 `/rekit sync` review。
- `sync -Apply` 会写 managed docs / managed block，必须先 review 再确认。
- `promote -Apply` 会写回 pack，风险更高；优先使用 `promote` review 或 `-CreateCandidates`。
- 多 Agent 可以并行读和产出候选，但不要并发写同一个 IDB、confirmed CSV、handoff 或 authority 文件。

## 1. 新架构如何使用

### 1.1 维护 kit 仓库

维护者主要改五层：

| 层 | 路径 | 什么时候改 |
|---|---|---|
| Skill UI | `.claude/skills/rekit/SKILL.md` | 调整用户可见 `/rekit` 语义和确认规则 |
| Runtime | `cmd/rekit/**`、`internal/rekit/**`、retained façade `rekit/rekit.ps1` | 调整确定性命令、状态、review、sync/promote 行为；不新增 PowerShell runtime logic |
| Pack | `packs/<pack>/**` | 新增领域流程、tooling、reference、manifest 规则 |
| Common | `common/**` | 多 pack 共享 policy / prompt |
| Docs | `README.md`、`docs/**` | 使用说明、设计、路线、迁移和验证 |

维护本仓库时不需要运行 `/rekit init`。只有需要验证 case 行为时，才创建临时 case。

### 1.2 接入新安全 case（当前以 `vmp-re` 为例）

从 kit 仓库启动 Claude Code，然后：

```text
/rekit init -Target <workspaceRoot>\cases\<caseName> -Pack vmp-re -ProjectName <caseName> -Apply
```

`init` 会创建：

```text
<caseRoot>\.rekit\instance.yml
<caseRoot>\.rekit\state.json
<caseRoot>\.claude\skills\rekit\SKILL.md
<caseRoot>\references\vmp-re\...
<caseRoot>\CLAUDE.local.md 中的 managed router block
```

之后进入 case 目录，每天只用 `/rekit`：

```text
/rekit overview
/rekit continue main
/rekit start <feature>
/rekit handoff
```

### 1.3 接入已有 case

已有 case 不建议直接 `init` 覆盖。先用：

```text
/rekit attach -Target <caseRoot> -Pack vmp-re -Apply
```

`attach -Apply` 只做四件事：

1. 写 `.rekit/instance.yml`。
2. 写 legacy `.re-template.yml`。
3. 写/更新 `.rekit/state.json`。
4. 写 case-local `/rekit` thin shim。

它不会覆盖已有 references、handoff 或工具链文档。随后用：

```text
/rekit sync
```

生成 review 包。确认无误后，再让 Claude 执行写入型 sync；Batch 106 起 `sync -Apply` 默认由 Go backend 处理，Batch 228 起 `sync` / `update` PowerShell fallback 已退休。

## 2. 主线和功能支线是否还能用

能，而且新架构更依赖它们。

| 工作线 | 典型命令 | 主要职责 | 默认可写 |
|---|---|---|---|
| 主线 | `/rekit continue main` | 收敛结论、验证 candidate、维护长期 handoff、处理 authority 写入；JSON envelope/run artifacts 暴露 apply 后 `missionBrief`，含 pending gates 与非阻塞 authorized gates | canonical 文件、主线 workspace、`.rekit/**` |
| 功能支线 | `/rekit start <name>`、`/rekit continue <name>` | 围绕一个功能点/阻塞点做探索、收集 evidence、提出 candidate/request；start/continue preview/apply `missionBrief` 让 lane executor 看到全局 ready/blocked 状态、pending gates 与非阻塞 authorized gates | 自己的 lane workspace、outbox、candidate/request |
| 项目级索引 | `/rekit handoff` | 生成跨工作线接手索引，并在顶部 Markdown 与 Go JSON `missionBrief` 汇总 ready/blocked lanes、pending gates、authorized gates、open decisions、interventions、next agent actions 与 escalations | `.rekit/handovers/latest.md` |

推荐流程：

```text
/rekit overview
/rekit continue main
# 主线判断需要专项探索
/rekit start vm-entry
# 在 vm-entry 支线收集证据和候选
/rekit continue vm-entry
# 回主线复核并决定是否确认
/rekit continue main
/rekit handoff
```

功能支线不是“低级 agent”，而是隔离写入和上下文的单位。它可以由同一个 Claude 会话推进，也可以由后续子 agent 或新会话接手。

## 3. 旧 case 如何兼容

### 3.1 旧 metadata

旧 case 可能只有 `.re-template.yml`。新 runtime 会优先读 `.rekit/instance.yml`，缺失时回退 `.re-template.yml`。

建议逐步补齐：

```text
/rekit status
/rekit attach -Target <caseRoot> -Pack vmp-re -Apply
/rekit sync
/rekit doctor
```

如果 case 被移动过：

```text
/rekit status
/rekit repair
确认修复，执行 repair -Apply
/rekit doctor
```

### 3.2 旧文档和 handoff

- `references/vmp-re/task-handoff.md` 是 local file，不会被 sync 覆盖。
- `CLAUDE.local.md` 的 managed router block 会被 sync 管理，但 block 外私有内容保留。
- `tools.local.yml`、`captures/**`、`artifacts/**` 不会被 promote。
- 旧 handoff 可继续保留；新 handoff 会优先写 `.rekit/handovers/**`。

### 3.3 旧命令习惯

旧 wrapper 仍保留，但日常不推荐直接使用脚本：

| 旧习惯 | 新入口 |
|---|---|
| 手动跑 `bootstrap.ps1` | `/rekit init` |
| 手动跑 `update.ps1` | `/rekit sync` |
| 手动跑 `validate.ps1` | `/rekit doctor` |
| 手动维护单一 task handoff | `/rekit handoff` 与 `/rekit handoff <name>` |
| 直接把 case 经验复制回 pack | `/rekit promote` review-first |

## 4. 这套架构的后续优化空间

### 4.1 短期优化

- 补 `docs/agent-team-usage.md` 到 case managed docs 或 reference 路由中，让 case 内也能直接看到本指南。
- 让 `/rekit overview` 更清楚地区分“未接手工作线”和“已接手工作线”。
- 增加旧 case 检测摘要：缺 `.rekit/instance.yml`、只有 `.re-template.yml`、缺 managed docs、缺 handoff 索引时给出明确下一步。
- smoke test 已有维护指南 `rekit/tests/README.md` 和 pack smoke matrix；后续可继续推进 CI 检查，避免只靠人工临时命令。

### 4.2 中期优化

- 将 evidence ledger 从文档草案推进到 runtime 支持的 append-only JSONL。
- 将 heavy-tool gate / lane autonomy profile 做成可复用 packet、授权和记录流程，支持预授权范围内自主执行与越界升级。
- 继续完善 `plan-subagents` 的 tactical reviewer dispatch、多 shard intake 与跨会话接手收敛；当前已支持 planning review artifacts，以及单个 contract-compliant reviewer result 的显式 WhatIf/Apply verification + main-decision writeback，后续重点是 bounded orchestration，而不是重复实现单 reviewer intake 或恢复手动 note 落账。
- 为 `packs/_template` 增加最小验证命令，降低新 pack 作者出错概率。

### 4.3 长期优化

- 拆出更多安全领域 pack；`web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re` 已有 skeleton，后续按真实需求继续推进其它领域 pack。
- 引入工具 adapter 层，把 IDA/x64dbg/trace/unicorn/symex、Web/API 测试、样本分析等能力先 recipe 化，再稳定成 adapter。
- 建立 candidate -> review -> confirmed 的机器可验证 gate。
- 将多 Agent 编排与证据账本结合，支持可回放、可审计、可回滚的安全研究工作流。

## 5. 推荐使用决策表

| 你现在的情况 | 推荐动作 |
|---|---|
| 只维护本仓库 | 读 `CLAUDE.md`、`docs/vision.md`、本文件；不要 init case |
| 新建安全 case（当前成熟示例：`vmp-re` RE case） | 在 kit 仓库用 `/rekit init -Target ... -Pack vmp-re -Apply` |
| 已有 case 接入新架构 | 用 `/rekit attach`，再 `/rekit sync` review |
| 旧 case 移动了目录 | `/rekit status` -> `/rekit repair` -> 确认后 `repair -Apply` -> `/rekit doctor` |
| 想看项目全局状态 | `/rekit overview` |
| 想继续主线 | `/rekit continue main`；自动化可用 `-WhatIf/-Apply -Format json` 读取 `missionBrief` |
| 想做专项探索 | `/rekit start <name>`，之后 `/rekit continue <name>`；start JSON/text 与 continue run status/digest 会记录 Mission Control brief / executor action snapshot |
| 想换会话 | `/rekit handoff` 或 `/rekit handoff <name>` |
| 想把 kit 更新同步到 case | `/rekit sync`，确认后才 apply |
| 想把 case 经验回流到 kit | `/rekit promote`，优先生成 candidate |
