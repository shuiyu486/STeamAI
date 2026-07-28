# re-context-kits 真实代码复审与长期推进建议（2026-07-28）

## 读取指南

本报告基于当前 `main` 分支的真实 Go 源码、测试、pack、skill、workflow、近期提交与本机命令执行结果，不以 roadmap 或 batch 文案替代实现证据。后续自主推进只需先读本页顶部执行区；需要具体依据时再读“代码证据”和对应源码，不要把本报告加入每轮默认 read-first 清单。

## 实施摘要

`re-context-kits` 已经具备质量较高的 **deterministic Mission Control substrate**：21 个 Go-owned public commands、durable lane/executor generation、handoff/checkpoint、append-only facts、strict WhatIf→Apply、reviewer packet/intake/recovery、authorized-gate 与 adapter report validation、pack-memory candidate lifecycle 都是真实代码，不是纯文档设计。

但它目前仍不是“可实际托管多会话 Agent Team 的 Mission Control”。Mission Commander 主要是 action queue/first-screen 投影；executor 是 lane 上的字符串与 generation，不是有 heartbeat/exit/result provenance 的真实 session；reviewer 与 adapter 也主要依赖主 Agent或外部 executor 手工执行，再把 JSON/sidecar 交给 runtime。最近大量 batch 又集中在 first-screen、handoff、summary、replay 文案与投影闭合，边际收益已明显下降。

当前最优路线不是继续补字段，而是：

1. 先修复当前 `main` 已存在的 release/status 真值漂移和 reviewer/gate 正确性缺口。
2. 建立 session/reviewer/adapter 的统一 lifecycle receipt 与 Mission Commander run loop，形成真实可运行闭环。
3. 在闭环内收敛巨型 CLI、字符串 action source、重复 queue/projection 与 repo-mutating tests。
4. 再把 pack-memory 做成跨 case 可消费的 review UX，而不只是五段式证明链。

## 执行清单

### 推荐优先顺序

- [x] **P0：恢复 main 的真实 green**：修复 Batch 702 release handoff 解析漂移，使 focused regression 与仓库级测试恢复通过；本批同步保留分段 `release-check ready=true` evidence 的真值。
- [x] **P0：收紧 reviewer/gate 正确性**：校验 reviewer `decision` 枚举、严格读取 dispatch packet、保留 acknowledged escalation；reviewer latest ordering 与 recovery exact-move 风险仍列为后续 correctness slice。
- [ ] **P1：实现 session/reviewer lifecycle vertical slice**：引入 durable dispatch/session/completion/intake receipts，把 lane owner generation、prompt hash、result hash、session identity 和 takeover 串成真实产品路径。
- [ ] **P1：实现 adapter execution provenance vertical slice**：在不让 `/rekit` 直接执行 heavy tool 的前提下，绑定 gate、adapter catalog、executor/session、预算计量、exit status、output hash 与 sidecar hash。
- [ ] **P1：收敛 CLI 与 Mission Commander 内核**：按 command 拆分 parser/handler，类型化 action source/state，统一生成一次 mission snapshot，再由 status/start/continue/handoff 渲染。
- [x] **P1：隔离测试 fixture**：8 个会修改真实 `_template` 的 CLI pack-memory/promote E2E 已迁移到独立临时 kit repo，并通过双进程并发验证；只读真实仓库 contract tests 保留。
- [ ] **P2：重做 pack-memory review UX**：把 review→cleanup→provision→verify→retire 表达为机器可读阶段图和一个可继续的主 Agent流程，并增加真实跨 case reconsume 场景。
- [ ] **P2：压缩 active state 文档**：让 `docs/batch-plan.md` 真正只保留 current/latest/next，旧批次归档；CHANGELOG 只保留可发布的用户可见摘要。

## 验证标准

每个中大型 slice 都应至少证明：

- 一个真实用户/Mission Commander 断点被端到端消除，而不是只新增字段或文本。
- package tests、CLI product-path、临时 case 或 installed entrypoint 中至少一条真实路径通过。
- `go test ./...`、`go vet ./...`、`git diff --check` 通过；`release-check ready=true` 仍只称为 inventory ready。
- session/reviewer/adapter 相关变更必须验证 takeover、幂等、stale result、partial failure 与 resume。
- 不把远程 `steps=[]` 当成 CI green；不因外部 runner blocker停止 Windows 本机可验证的产品闭环推进。

## 风险与注意事项

- 不建议直接把 Go runtime 变成 Claude Code 进程管理器。更稳妥的边界是：主 Agent/Claude Code harness 负责真正 spawn，Go runtime 负责 durable request/receipt、哈希绑定、状态机、恢复和审计。
- 不建议单独做“大重构 CLI”批次。应先选 reviewer/session 或 adapter lifecycle vertical slice，在该 slice 内抽出 command registry、typed source 和 shared snapshot。
- 不建议用更多 boundary strings、summary DTO 或 first-screen package代替真实生命周期；这些只能作为闭环产物。
- PowerShell façade 删除不是当前最高价值工作。真实 installed `/rekit` 入口、Windows 本机闭环和 session provenance 更优先；远程三平台 green 恢复后再执行最终删除门禁。

---

## 一、总体评估

| 维度 | 评价 | 结论 |
|---|---:|---|
| Go deterministic substrate | 8/10 | 路径、哈希、WhatIf/Apply、ledger、receipt、fail-closed 边界扎实。 |
| Durable lane protocol | 7/10 | lane ownership/generation、handoff/checkpoint、stale guard 是真实可用代码。 |
| Reviewer/adapter contract | 7/10 | strict JSON、packet/intake/recovery 和 sidecar validation 很深入。 |
| 真实 Mission Commander orchestration | 3/10 | 主要是 queue/projection；没有统一 run loop、session registry、heartbeat/exit/result lifecycle。 |
| 用户产品 UX | 4/10 | `/rekit` skill 是薄入口，但底层流程和 pack-memory/reviewer 操作仍需要大量命令拼接。 |
| 可维护性 | 4/10 | 多个超大文件、字符串路由、重复投影和超长测试/active docs 已成为明显阻力。 |
| Release truthfulness | 5/10 | 文档知道 inventory≠green，但当前 main 的 status handoff 与测试已经漂移。 |
| Pack-based team memory | 5/10 | primitives 完整，真实跨 case、多 pack、人工 review UX 尚未闭合。 |

### 已经做得好的部分

1. **public command ownership 真实存在**：`internal/rekit/commands/commands.go:109-155` 当前实际定义 21 个 public commands 和 mutation profiles。
2. **replaceable executor 基础是真的**：`internal/rekit/workstream/start.go:523-584` 会持久化 executor claim/generation，`continue` 使用 generation owner guard，不只是展示文案。
3. **reviewer contract 边界严格**：`internal/rekit/reviewerresult/reviewer_result.go:58-117` 使用 required fields、`DisallowUnknownFields`、single-object、confidence enum 等验证。
4. **authorized execution 不冒充实际执行**：`internal/rekit/workstream/authorized_gate_handoff.go:253-260` 明确只投影 contract/validate/record，禁止 heavy-tool replay 和 authority/confirmed 写入。
5. **测试体量和产品路径覆盖可观**：CLI tests 已覆盖 lane takeover、reviewer intake、gate report、promote candidate lifecycle、installed shim status/refresh 等真实 temp-case 路径。

## 二、当前真实问题

### P0-1：当前 main 的完整 Go 测试失败，release/status 真值漂移

实际执行：

```text
go run ./cmd/rekit -- -Command release-check -Format json   # exit 0, ready=true
go run ./cmd/rekit -- -Command status                       # exit 0
go run ./cmd/rekit -- -Command packs                        # exit 0
go run ./cmd/rekit -- -Command doctor                       # exit 0
go vet ./...                                                # exit 0
git diff --check                                            # exit 0
go test ./...                                               # exit 1
```

独占复现：

```text
go test ./internal/rekit/cli -run '^TestRunStatusJsonKit$' -count=1
```

稳定失败于 `internal/rekit/cli/cli_test.go:838-850`：Batch 702 的 release inspection cadence 已为 `complete`，但 `status.projectHandoff` 仍给出：

```text
latestLocalValidationReady=false
latestNextAction=run the full local release minimum and update docs/batch-plan.md
currentAction.source=releaseHandoffLatestBatch
nextBatchSelectionPackage=null
```

而测试要求 completed cadence 转向 `releaseHandoffNextBatch / next-batch-selection`。根因证据在：

- `internal/rekit/releasecheck/release_handoff.go:3943-3957`：从 batch 文本派生 handoff。
- `internal/rekit/releasecheck/release_handoff.go:4010-4034`：local validation 要求识别完整命令集合。
- `internal/rekit/releasecheck/release_handoff.go:4605-4623`：`LocalValidationReady=false` 时优先回退本地验证。
- `docs/batch-plan.md:29`：Batch 702 完成记录中的 release-check 复跑采用缩写叙述，和 parser 的精确证据规则发生漂移。

这说明 `release-check ready=true` 只证明 inventory 自洽，不能证明 release handoff/current action 自洽，更不能证明测试 green。

### P0-2：public command 数量文档陈旧

真实代码是 21 个 public commands，包含 `release-run`：`internal/rekit/commands/commands.go:109-155`；实际 `release-check` 也输出 `commandProfileSummary.total=21`。但 `docs/release-readiness.md:13,95,111` 和项目说明仍多处声称 20 个。这种硬编码计数会继续造成 release 文档与 runtime 漂移。

### P0-3：reviewer decision contract 声明了枚举，但 Decode 未执行枚举校验

- `internal/rekit/reviewerresult/reviewer_result.go:42` 声明 `accept/reject/defer/abandon/needs-more-evidence`。
- `internal/rekit/reviewerresult/reviewer_result.go:97` 只 trim decision。
- `internal/rekit/reviewerresult/reviewer_result.go:114-116` 只验证 confidence。

因此基础 contract 可能接受任意非约束 decision 字符串。应在最底层 `Decode` 校验，不能依赖后续 route/intake 偶然拦截。

### P1-1：acknowledgement 可能隐藏仍需主 Agent升级的 adapter report

`internal/rekit/workstream/authorized_gate_handoff.go:174-199` 在 report 为 `evidence-already-recorded` **或** `RequiresMainEscalation` 时，只要 event ID 被 acknowledged，就清空 Mission Commander actions/current action。普通 evidence acknowledgement 不应自动关闭 escalation；应要求独立 escalation resolution receipt 或继续保留升级动作。

### P1-2：reviewer packet 主体读取弱于 integrity sidecar

- `internal/rekit/workstream/reviewer_dispatch_intake.go:1080-1089` 对 `packet.json` 使用普通 `os.ReadFile + json.Unmarshal`。
- `internal/rekit/workstream/reviewer_dispatch_intake.go:1092-1148` 对 integrity 使用 stable artifact read、size/type/path检查、`DisallowUnknownFields` 和 trailing-object guard。

不可变 packet 主体不应比 sidecar 更宽松。应统一为 stable strict read，并覆盖 unknown field、trailing JSON、symlink 与 changed-while-reading。

### P1-3：reviewer writeback 的 `Latest*` 不是全局时间顺序

`internal/rekit/workstream/reviewer_writeback.go:67-74` 先 append 所有 verifications，再 append 所有 decisions；`reviewer_writeback.go:107-120` 直接把最后一项称为 latest。若较新的 verification 晚于较旧的 decision，summary 仍会把 decision 当 latest。应按 durable sequence/createdAt/event order 合并，或者停止使用 `Latest*` 语义。

### P1-4：regular-file recovery 存在 move 后验证窗口

`internal/rekit/subagents/reviewer_result_recovery.go:524-562` 对 regular file 先 `os.Rename`，再读取 quarantine 验证 bytes；相邻 obstruction path 在 `:477-521` 已使用 handle-validated exact move。regular file 也应绑定 source identity，否则并发替换可能在报错前移动错误对象。

### P1-5：测试会修改真实 pack，跨进程不可隔离

例如：

- `internal/rekit/cli/cli_test.go:1973-1985` 直接在真实 `packs/_template/promote-candidates` 和 `tooling/candidates` 写 fixture。
- `internal/rekit/cli/cli_test.go:11083-11128` 的完整 pack-memory E2E 同样修改真实 pack target，再通过 cleanup 恢复。

单进程通常可恢复，但两个 `go test`/审计进程并发时会相互看到候选、receipt 和 pack target 中间态。本次审核最初就实际触发了这种干扰。应将 repo-mutating tests 迁到临时 repo clone/fixture root，不能靠 `t.Cleanup` 保护真实仓库全局状态。

## 三、架构层面的主要瓶颈

### 1. Mission Commander 是投影器，不是统一 orchestrator

真实 action queue 位于 `internal/rekit/mission/brief.go`，并被 start/handoff/continue/status/overview/reviewer staging 多处重新构造。它可以解释状态，却不会：

- 注册或监控 Claude Code session；
- 记录 heartbeat、exit code、stdout/result provenance；
- 自动把 dispatch receipt 与 reviewer result/session 对上；
- 在 partial failure、timeout、stale owner 后统一恢复任务；
- 运行一个“读状态→选择动作→dispatch→收集→intake→重算”的 durable mission loop。

因此当前准确定位应是 **Mission Control protocol/runtime substrate**，不是完整 Mission Control 产品。

### 2. action source/state 依赖字符串约定

Mission Commander queue 的优先级和来源识别大量依赖 `Source` 字符串前缀。随着 `missionCommanderActions`、`executionEvidenceReview`、`reviewerDispatch...`、`packMemoryCandidates...`、`releaseHandoffNextBatch...` 持续增加，拼写或投影 drift 很难由编译器发现。应引入 typed source/category/lifecycle state，并让 renderer 只消费统一 snapshot。

### 3. CLI 和测试已经超过合理单文件边界

当前规模：

| 文件 | 行数 | 大小 |
|---|---:|---:|
| `internal/rekit/cli/cli.go` | 11,110 | 559 KB |
| `internal/rekit/cli/cli_test.go` | 20,307 | 1.45 MB |
| `internal/rekit/releasecheck/release_handoff.go` | 4,713 | 236 KB |
| `internal/rekit/gate/gate.go` | 4,099 | 214 KB |
| `internal/rekit/workstream/reviewer_dispatch_intake.go` | 2,627 | 160 KB |

`cli.Options` 已容纳 promote proof、reviewer recovery、gate、note、start/continue 等多领域 flags；`Parse` 和 `Run` 是全局开关。建议在不改变 public ABI 的前提下建立 command registry，每个 command/模式拥有 parser validation、handler 和 renderer。

### 4. active docs 与渐进披露目标矛盾

- `docs/batch-plan.md` 约 698 KB / 2,546 行，仍保留大量旧批次。
- `CHANGELOG.md` 约 673 KB / 827 行，每个 batch 写入超长实现与验证日志。
- `docs/batch-history.md` 约 1.5 MB。

这会让后续几十上百轮 autonomous goal 花大量上下文解析历史，并继续诱导“找下一个投影断点”。应把 active batch state 压缩为 current/latest/next，release evidence 进入机器可读 receipt 或归档，CHANGELOG 只保留用户可见摘要。

## 四、推荐的中大型路线

### 路线 1：Release truth + reviewer correctness hardening（先做）

一个完整 batch 可同时解决：

- 当前 main 的 `TestRunStatusJsonKit` 与 local validation parser drift；
- 21-command 文档/invariant；
- reviewer decision enum；
- packet strict read；
- escalation acknowledgement guard；
- focused negative tests 与全量 gate。

这是中型 correctness closure，不是微字段批次，也是后续大改前恢复可信基线的必要步骤。

### 路线 2：Durable reviewer session lifecycle（核心主线）

新增统一生命周期：

```text
planned → dispatched → running/unknown → completed/failed → collected → intaken → consumed/retired
```

每个 dispatch/completion receipt 至少绑定：

- packet/route/shard；
- prompt SHA-256、agent type；
- lane owner executor/generation；
- harness/session ID；
- dispatched/completed timestamp；
- result path/hash、exit status；
- takeover/adoption provenance。

真正 spawn 仍由主 Agent/Claude Code harness 完成；Go runtime 提供 request/receipt/intake/recovery。做完后，`reviewerSession` 不再只是任意字符串，Mission Commander 能判定 waiting、stale、failed、retry、ready intake。

### 路线 3：Mission Commander autonomous run loop

在 route 2 的 receipts 上实现一轮可恢复 loop：

1. 读取统一 mission snapshot。
2. 选择 bounded current actions，按 lane/priority/max parallel 生成 dispatch requests。
3. 由主 Agent执行 Agent tool/外部 executor。
4. 记录 completion receipts，执行 strict collection/intake。
5. 重算 queue，持久化 run digest；partial failure 可由新 session 继续。

先只支持 read-only reviewer，不直接执行 heavy tool。这样能够首次真实证明“用户只对主 Agent说继续，主 Agent推进多个 reviewer shard”。

### 路线 4：Adapter execution provenance

复用 session lifecycle 模型，增加 adapter execution receipt：

```text
adapterId + gateEventId + executor/session + command/input hash + measured budget + exit status + output hashes + sidecar hash
```

`gate -Apply -ExecutionReportPath` 必须同时校验 receipt、catalog adapter、授权预算、output path 和 sidecar。heavy tool 仍由 lane executor/tool adapter执行，runtime 不越界，但 evidence 不再完全依赖 self-reported JSON。

### 路线 5：Pack-memory review experience

当前流程实际是 review→cleanup→provision→verify→retire 五段状态机。应：

- 用 typed stage graph 替代大量 runbook string；
- 让主 Agent提供单一“继续 pack-memory review”体验；
- 把 tooling candidate 的 manual merge proof 纳入明确阶段；
- 增加两个独立 case 对同一 promoted recipe 的真实 reconsume E2E；
- 为 `mature` 定义能力门槛，而不只是配置密度标签。

### 路线 6：架构与测试收敛（嵌入上述路线）

- command registry 拆分 `cli.go`；
- typed action source/state；
- shared mission snapshot，避免每个命令重算/重投影；
- temp-repo test harness，禁止改真实 pack；
- 按 subsystem 拆分 20k 行 CLI tests；
- active docs 减压。

## 五、长期 goal 设计

### 推荐 goal

```text
在 C:\AI\m_projects\RE\re-context-kits 的 main 分支长期自主、连续推进项目成为可实际运行的 Lane-centric Agent Team Mission Control。每轮选择一个中大型、端到端、可验证的产品闭环，优先把现有 durable contract 接到真实 session、reviewer 和 adapter 生命周期，并同步收敛阻碍闭环的架构复杂度；完成验证、文档、提交和推送后继续下一轮。除非遇到必须由我决策的不可逆事项，否则不要停止，也不要把单批完成、工作树干净或 inventory ready 当作 goal 完成。
```

### 为什么这版更适合几十上百轮

- 只固定产品北极星、批次粒度、近期真实缺口和继续推进原则。
- 不枚举几十条命令、边界、测试和停止条件；这些继续由仓库 `CLAUDE.md` 与路由文档承载。
- “优先”而不是“只允许”，模型可以根据代码证据调整顺序。
- 明确要求端到端闭环，能抑制 first-screen/summary/metadata 微批次。
- 只有一个停止原则，避免模型在单批完成后自行宣布 goal 完成。

### 不建议继续使用的写法

不建议在 goal 中复制 `docs/autonomous-goal.md:94-100` 的全部细项。现有版本虽然方向正确，但把读取步骤、八类近期重点、PowerShell、CI blocker、文档、push、停止条件都塞入 goal，容易让模型把“逐项满足文本约束”误当任务，而不是主动发现真正产品断点。

## 六、结论

项目不是“没实现”，而是实现重心失衡：确定性 contract、receipt、handoff 和投影已经很深，真实 orchestration/session lifecycle 与可维护性相对落后。近期不应再连续做字段/first-screen/summary closure。最有价值的连续主线是：

```text
恢复真实 green
→ reviewer/session receipt
→ Mission Commander run loop
→ adapter execution provenance
→ pack-memory multi-case UX
→ 在每条 vertical slice 内持续拆分 CLI、类型化状态并隔离测试
```

这条路线足够支撑几十到上百轮中大型自主推进，同时不会被一份过长 goal 过度束缚。