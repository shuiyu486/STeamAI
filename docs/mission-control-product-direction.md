# Mission Control 产品方向

## 读取指南

本文件是 `re-context-kits` 的最终产品方向锚点，用于防止后续自主推进时偏回“命令大全”、短命 subagent 或用户手动盯多个会话的路线。

如果你是新会话或上下文压缩后的 AI，先读本文件顶部，再读 `docs/autonomous-goal.md`、`docs/go-first-convergence-plan.md` 与 `docs/batch-plan.md` 最新批次。实现时以本文件的产品方向为北极星，以现有 Go-first / release readiness 文档为落地路线。

本文档描述目标形态，不代表所有能力都已实现。新增能力必须按可验证 vertical slice 落地，并持续写回 `docs/batch-plan.md` 或相邻设计文档。

## 实施摘要

`re-context-kits` 的最终产品方向是 **Lane-centric Agent Team Mission Control**：一个面向安全研究、逆向和安全工程任务的 Claude Code 多会话 Agent Team 工作系统。

用户主要和一个 **主 Agent / Mission Commander** 会话交互。主 Agent 通过 ReKit deterministic runtime 调度长期 **member lane**，而不是依赖某个旧聊天窗口。每个 lane 可以由当前 Claude Code 会话执行，也可以在上下文污染、模型切换、会话损坏或用户希望重开时，由新的 Claude Code 会话读取 handoff / packet / evidence 后接手。主 Agent 也可以启动短命 tactical subagent 处理搜索、验证、review、小修、小调研等局部任务。

`/rekit` 与 Go backend 是底层确定性 runtime / API，`rekit.ps1` 只作为 retained compatibility façade；它们都不是主要用户体验。用户体验应尽量表现为自然语言 mission control：开始任务、继续推进、查看总体状态、进入某个成员 lane、接手 lane、沉淀经验。

Batch 359 后，Go-owned public surface、durable lane state、显式 `reconcile`、typed autonomy preflight、Mission brief / executor action snapshot、Go-native bounded reviewer strict intake/writeback、pack-memory promote/reconsume package E2E，以及 authorized execution observation evidence + bounded adapter execution report strict intake/contract projection（含 sidecar boundary/escalation marker fail-closed validation）已形成底座；主 Agent已在本机真实 spawn 一个 read-only reviewer，并跑通 packet/result binding、evidence-ref validation、WhatIf/Apply verification-before-decision 幂等写回与 overview/handoff/doctor post-validation。runtime 仍不自动 spawn、注册或管理 reviewer/member session，也不执行 heavy-tool；当前未完成的是统一 session/reviewer orchestration、真实 lane executor / tool-adapter live validation、pack-memory product UX 与跨平台 product-path E2E。

## 执行清单

后续自主推进围绕这些方向做中大型、可验证切片：

- [ ] **Mission Control UX**：README、skill、case 使用文档从命令枚举转向“主 Agent 统筹 mission”的日常使用模型；底层命令保留为 runtime API。
- [ ] **Lane protocol**：定义 durable member lane 的 packet、status、outbox、handoff、intervention、authority profile 与 reconcile 流程。
- [ ] **Replaceable session executor**：让新 Claude Code 会话能接手旧 lane，避免长期旧会话上下文污染；成员身份绑定 lane，而不是绑定 session。
- [ ] **Human-in-the-Lane**：用户可随时进入任意成员 lane 纠错、补充、改向、打断或硬切模型；当前 `continue` 对 open intervention fail-closed 并要求显式 `reconcile`，目标是让 Mission Commander 自动发现、解释并准备安全 resolution。
- [ ] **Tactical subagents**：主 Agent可按需启动短命 subagent 做只读搜索、复核、反驳、小修、文档一致性检查或 bounded implementation；Batch 353 已由主 Agent在本机真实 spawn read-only reviewer，并通过显式 reviewer intake WhatIf/Apply 完成 verification-before-decision 写回与 post-validation，但 runtime / `plan-subagents` 仍不自动 spawn 或管理 reviewer session。
- [ ] **Pre-authorized lane autonomy**：lane 文档/packet 可提出 heavy-tool、动态调试、patch、dump、hook、网络、exploit replay 等授权意图；只有 strict validated durable `autonomy.json` 加 `authorized-gate` decision 才构成 executor 的确定性预授权依据。
- [ ] **Pack-based team memory**：把 case 中复用价值高的 recipe、checklist、prompt、policy、tool adapter 与 workflow 经 review/promote 沉淀回 pack/common。
- [ ] **Go-first deterministic substrate**：Go backend 已是 public command surface 的确定性 owner；继续完成 PowerShell-free default/product path、retained compatibility façade 收束和跨平台 product-path E2E，禁止新增 PowerShell runtime logic。

## 验证标准

每个实现批次至少验证：

1. 用户是否更少接触底层命令，更多通过主 Agent 意图推进 mission。
2. 主 Agent 能否从 case-local state 判断 lane 状态、阻塞、下一步与是否需要用户决策。
3. lane 是否可以被新会话接手，且接手依赖 handoff / packet / evidence，而不是旧聊天上下文。
4. 用户在 lane 内手动纠错、改向或硬切模型后，干预是否被 reconcile 到 durable state。
5. 预授权 heavy action 是否被限制在 lane packet / autonomy profile 明确的 target、budget、stop conditions 和 output path 内，并写入 ledger / evidence。
6. 短命 subagent 的输出是否结构化、可合并、可验证，不把大段日志或无界探索灌回主上下文。
7. 可复用经验是否通过 review/promote 进入 pack/common，而不是把 case 私有样本、trace、dump、目标信息或绝对路径写入模板仓库。

## 风险与注意事项

- 不要把“长期成员”实现成“长期旧聊天窗口不可替换”。长期的是 lane、状态和职责；session 只是当前 executor。
- 不要把主 Agent 做成命令帮助器。它应统筹 mission、解释状态、调度 lane、合并结果和上报关键决策。
- 不要让自动化只靠 LLM 聊天记忆。事实、证据、干预、请求、handoff 和经验沉淀必须进入 case-local source of truth。
- “悄悄执行”只能理解为 **在预授权范围内不逐步打断用户**，不能理解为无记录、无边界、无目标、无预算或无止损地执行。
- confirmed / authority 写入、sync/promote 写回模板、runtime schema 迁移、破坏性项目改动或超出当前 lane 授权范围的外部副作用，仍必须提升到主 Agent / 用户决策。

## 1. 产品定位

最终产品不是一组让用户记忆的 `/rekit` / `rekit.ps1` 命令，而是 Claude Code 中的 Agent Team Mission Control：

```text
用户
  ↓
主 Agent / Mission Commander
  ↓
ReKit deterministic runtime
  ↓
长期 member lanes + 可替换 Claude Code session executors + 短命 tactical subagents
```

推荐一句话定位：

> `re-context-kits` 是一个面向安全研究 / 逆向 / 安全工程任务的 Claude Code 多会话 Agent Team 操作系统：用户主要指挥主 Agent，主 Agent 调度长期 member lanes 与短命 tactical subagents，runtime 负责可靠状态与验证，pack 负责领域能力沉淀，团队能力会随着使用持续增强。

## 2. 核心抽象

### 2.1 Mission

用户的大目标，例如“分析这个样本的核心逻辑”、“完成 Web/API 授权测试 triage”、“复核一个漏洞假设”。

### 2.2 Member lane

长期职责线，例如：

- `triage`
- `reverse`
- `verifier`
- `toolsmith`
- `reporter`
- `exploit-repro`
- `network-assessment`
- `pack-maintainer`

lane 持久存在，保存 packet、status、outbox、evidence、handoff、intervention、decision 与 open risks。

### 2.3 Session executor

当前执行某个 lane 的 Claude Code 会话。session 可以长期工作，也可以被废弃、重开、换模型或由新会话接手。成员身份不绑定 session。

### 2.4 Tactical subagent

主 Agent 临时启动的小型 agent，用于搜索、验证、review、反驳、摘要、小修或 bounded implementation。短命 subagent 不替代长期 lane。

### 2.5 Runtime source of truth

`.rekit/` 下的 board、facts、lanes、runs、handovers、reviews、pack metadata 是协作真相源。聊天上下文可以辅助执行，但不能成为唯一状态。

## 3. 日常使用模型

理想日常交互应接近：

```text
用户：开始这个 case，目标是还原核心逻辑。
主 Agent：选择 pack，初始化/附着 case，规划 mission lanes，生成首轮 packet。

用户：继续推进。
主 Agent：读取 status/outbox/ledger/handoff，判断哪些 lane 继续、哪些卡住、哪些需要验证，然后自动调度。

用户：总体怎么样？
主 Agent：输出 mission brief：已完成、证据、阻塞、风险、下一步、需要用户决策的事项。

用户：我进去帮 verifier。
主 Agent：告诉用户接手哪个 lane、当前 packet、应读哪些 evidence、怎样把干预写回状态。

用户：这个 lane 上下文污染了，换新会话接手。
主 Agent：生成/刷新 handoff，新会话读取 packet + evidence + open risks 后接手同一 lane。

用户：沉淀这次经验。
主 Agent：整理 recipe/checklist/prompt/tool adapter，经 review/promote 写回 pack/common。
```

## 4. 长期 member lane 与接手

设计原则：

```text
长期的是 lane，不是旧 session。
```

旧会话上下文污染、模型风格变化、任务方向偏移或用户想重开时，应支持：

1. 主 Agent 刷新 lane handoff。
2. 新 Claude Code 会话读取 lane packet / status / evidence / decisions / interventions / open risks。
3. 新会话声明接手同一 lane。
4. 新会话先 reconcile，再继续推进。
5. 旧会话可以废弃，不影响 lane 身份。

接手时不应要求复制旧聊天全文。接手包应包含最小恢复上下文、证据引用、当前假设、废弃假设、下一步、授权边界和 stop conditions。

## 5. Human-in-the-Lane

用户可以随时进入任意 member lane：

- 打断错误执行；
- 纠正错误假设；
- 注入新上下文；
- 改变下一步；
- 硬切 `/model` 后继续；
- 要求 lane 暂停、转向或由新会话接手。

产品稳定性不依赖“用户不碰成员”。目标形态是 lane 在继续时自动 reconcile；当前实现会 fail-closed 并要求 Mission Commander / executor 显式执行 `reconcile`：

```text
1. 读取当前 packet / status / evidence / intervention / outbox。
2. 判断用户是否刚刚纠错、改向、切模型或改变授权。
3. 总结干预，标记 superseded assumptions / invalidated candidates。
4. 更新 status / outbox / intervention event。
5. 若影响全局计划，通知主 Agent。
6. 再继续推进当前 lane。
```

用户不需要在切模型前手写 checkpoint。模型硬切后，同一 Claude Code 会话仍保留上下文；但 lane 应在下一次继续时把重要变化落到 durable state，便于主 Agent 统筹和新会话接手。

## 6. 预授权 lane autonomy

lane 文档与 task packet 可以表达或提议较重动作的授权意图，但不能单独作为 deterministic executor grant。当前 runtime 的确定性依据是 strict validated `.rekit/lanes/<lane>/autonomy.json`，并由 `gate -WhatIf/-Apply` 评估 action、exact target、typed budget、stop conditions、output paths、record/notify 与 grant/expiry；只有 `authorized-gate` decision 才允许 executor 在覆盖范围内不逐步打断用户确认。该能力用于提高已明确授权 sandbox、CTF、靶场、样本分析或内部安全评估 case 的自主性。

当前 durable profile field shape 可用一个 strict-valid、默认 fail-closed 的 `manual-gate` 文件表示：

```json
{
  "schemaVersion": 1,
  "profileId": "main-manual-gate",
  "lane": "main",
  "mode": "manual-gate",
  "allowedActions": [],
  "deniedActions": [],
  "targetScope": [],
  "budget": {"runtimeSeconds": 0, "diskMB": 0, "requests": 0},
  "stopConditions": [],
  "outputPaths": [],
  "recordRequired": true,
  "notifyMainOn": []
}
```

实际文件中的 `lane` 必须与 lane ID 一致。切换为 `preauthorized` / `autonomous` 时，`allowedActions` 必须来自所选 pack 的 `heavyToolGates`，`targetScope` 只支持 `exact`，三项 budget 必须为正数，并提供非空、合法的 `stopConditions`、case-relative `outputPaths`、`notifyMainOn`、`grantedBy` 以及 RFC3339 `grantedAt` / `expiresAt`；不要把占位符或 union 字符串直接保存到 `autonomy.json`。

解释：

- `manual-gate`：每次 heavy action 都经 gate / 用户确认；budget 可以为零。
- `preauthorized`：在明确范围内不逐步询问，但必须记录、止损、可回放；要求非空 `allowedActions`、exact `targetScope`、正数 budget、`stopConditions`、`outputPaths`、`notifyMainOn`、`grantedBy`、`grantedAt` 与 `expiresAt`。
- `autonomous`：只适合完全 sandbox / mock / CTF / 明确授权的本地环境；使用与 `preauthorized` 相同的 strict validation 要求。

重要边界：

- 预授权不是无边界授权。超出 durable profile 的 `targetScope`、`budget`、`stopConditions`、`outputPaths`、grant/expiry 或 `authorized-gate` decision 时必须暂停并升级。
- 所有 heavy action 必须产生 summary / sidecar / evidence ref，不把大输出直接写入 Markdown。
- confirmed / authority 写入和 pack promote 仍比 lane 内工具执行更严格，不能因为工具执行被预授权就自动发布最终结论。

## 7. 主 Agent 调度策略

主 Agent 每轮应优先做：

1. 读取全局 mission state。
2. 汇总 lane status / outbox / pending requests / open risks。
3. 判断哪些 lane 可继续、哪些需要接手、哪些需要短命 subagent。
4. 根据 lane autonomy profile 决定是否需要询问用户。
5. 合并结果，写 decision / intervention / handoff。
6. 只把关键阻塞和产品级决策上报用户。

主 Agent 不应要求用户实时盯多个成员会话。

## 8. 短命 tactical subagent 策略

主 Agent 可以启动短命 subagent 处理：

- 代码/文档搜索；
- 只读复核；
- adversarial verification；
- bounded patch；
- 小工具生成；
- 文档一致性检查；
- release invariant review；
- 对某个 candidate 的独立反驳。

短命 subagent 输出必须短、结构化、可合并。它们不替代长期 member lane，也不应无界探索。

## 9. 经验沉淀闭环

case 中产生的复用价值应进入 pack/common，而不是留在一次性聊天：

```text
case observation / recipe / checklist / prompt / adapter candidate
  → review packet
  → sanitize / deny-pattern check
  → promote candidate
  → pack/common recipe / policy / prompt / tooling catalog
```

沉淀对象包括：

- 工具 recipe；
- 失败模式；
- review checklist；
- lane prompt；
- tool adapter；
- stop condition；
- autonomy profile 模板；
- pack-specific workflow。

## 10. 推荐给新会话的接手话术

在发正式 goal 前，可先把这段发给新会话：

```text
在 C:\AI\m_projects\RE\re-context-kits 继续维护项目。先读取 CLAUDE.md、docs/mission-control-product-direction.md、docs/autonomous-goal.md、docs/go-first-convergence-plan.md、docs/release-readiness.md 和 docs/batch-plan.md 的 current/最新区，并检查 git 与本地/远程 release gate 的实际状态。当前产品方向已经确认：re-context-kits 要收敛为 Lane-centric Agent Team Mission Control，而不是命令大全；主 Agent 统筹 durable member lanes，lane 由可替换 Claude Code session executor 执行，也可用短命 tactical subagents；用户可随时介入 lane，当前通过显式 reconcile 把干预写回 durable state；lane 文档/packet 只表达授权意图，实际 heavy action 必须由 strict durable autonomy profile + authorized-gate decision 覆盖 target、budget、stop conditions、output paths、记录和升级边界。先只读取并确认接手，不要立刻改文件，等我发正式 goal。
```

## 11. 推荐长期 goal 语句

```text
在 C:\AI\m_projects\RE\re-context-kits 中，长期自主推进 re-context-kits 向 Lane-centric Agent Team Mission Control 收敛。每轮选择一个 coherent、中大型、可验证、能降低真实维护风险或提高实际可用性的 vertical slice，不要只做一两行或逐字段 metadata 微批次；完成后自行审查、评估效果、做必要低风险调整，然后执行真实验证，更新 docs/batch-plan.md 的 current/active/next 区或相关设计文档，并只记录必要的 CHANGELOG。仅在当前用户 goal/session 明确授权时提交并推送指定分支；每批后重新校准运行事实和风险，长期目标未完成且无升级条件时再继续下一批。

产品北极星：用户主要和主 Agent / Mission Commander 交互；主 Agent 调度 durable member lanes，而不是绑定旧聊天窗口；每个 lane 可由长期 Claude Code 会话执行，也可由新会话接手；主 Agent 可启动短命 tactical subagents 做搜索、验证、review、小修和 bounded implementation；用户可随时进入任意 lane 纠错、改向、硬切模型或要求新会话接手，当前 `continue` 对 open intervention fail-closed 并要求显式 `reconcile` 写入 durable state，目标是 Mission Commander 自动准备安全 resolution；lane 文档/packet 只表达授权意图，只有 strict durable autonomy profile 加 `authorized-gate` decision 完全覆盖 action、exact target、typed budget、stop conditions、output paths、record/notify 和 grant/expiry 时，成员 lane 才可不逐步打断用户地执行 heavy-tool、动态调试、patch、dump、hook、exploit replay、网络扫描/请求回放；超出范围、出现新风险、需要 confirmed/authority 或 pack promote 时必须升级。

实施主线围绕七类：1) Mission Control UX：减少用户面对的命令，把 /rekit 作为 runtime API 而不是主要 UX；2) lane protocol：把已有 packet/status/outbox/handoff/intervention/autonomy/reconcile 底座推进为实际接手闭环；3) replaceable session executor：长期成员身份绑定 lane，并明确 session registration/takeover/ownership 的 operational contract；4) tactical subagents：实际验证主 Agent dispatch、intake、WhatIf、writeback 与 post-validation；5) pre-authorized lane autonomy：让 executor/tool adapter 消费 durable profile + authorized-gate 并回写 evidence/budget/boundary-hit；6) pack-based team memory：把复用经验 review/promote 后在 fresh case 重新消费；7) Go-first deterministic substrate：完成 PowerShell-free default/product path、retained façade 收束与跨平台 product-path E2E，禁止新增 PowerShell runtime logic。

每批开始前先读 CLAUDE.md、docs/mission-control-product-direction.md、docs/autonomous-goal.md、docs/release-readiness.md、docs/go-first-convergence-plan.md 和 docs/batch-plan.md 的 current/最新区，并检查 git、本地 gate 与远程 CI 实际状态；优先做 coherent 中大型 vertical slice，并自审是否偏离 Mission Control 北极星。除新的产品方向变化、破坏性仓库操作、未授权外部副作用、runtime schema 迁移、公共入口删除门禁不完整、confirmed/authority 写入策略变化或难以判断的架构取舍外，自主判断并持续推进。
```
