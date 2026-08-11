# Reference absorption map

## 读取指南

本文件用于回答：本项目从外部参考文章和项目中吸收了什么、已经落地了哪些能力、哪些仍是后续计划。

如果你是在另一台电脑接手维护本项目，先确认在 `main` 分支且 `main` 与 `origin/main` 同步，再读 `CLAUDE.md` 与 `docs/context-routing.md`，并按场景读取本文件顶部、`docs/mission-control-product-direction.md`、`docs/autonomous-goal.md`、`docs/agent-team-usage.md` 或 `docs/batch-plan.md`。不要把本文件当成新会话默认 read-first 清单；它只在需要追溯外部参考吸收关系时进入上下文。

本文件只记录可复用设计和落地映射，不记录真实样本名、RVA/VA、trace/dump、artifact 路径、客户信息或 case-specific 进度。

## 实施摘要

本项目从参考资料中吸收的不是“某个现成脱壳器”，而是三类能力：

1. **Agent Team 工作方式**：多 Agent 分工、上下文隔离、工作线、handoff、candidate -> review -> confirmed。
2. **工具 adapter 策略**：IDA/调试器/trace 等工具先 recipe 化、candidate 化，再逐步 adapter 化，避免成为硬依赖或大输出源。
3. **证据与门禁模型**：evidence ledger、batch/intervention、heavy-tool gate、人工确认和可回滚的审查流程。

当前已经落地的是安全 Agent Team 框架底座、文档契约、Go-owned/no-fallback `/rekit` runtime、durable lanes、显式 reconcile、typed autonomy preflight、Mission brief / executor action、review-first sync/promote、首个成熟 pack `vmp-re` 扩展、安全领域 pack 骨架 `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re`、tooling candidate、`ida-agent-bridge` 只读 packet contract、独立的 compiled-in `vmp-ida-index-inspector` fixed adapter、bounded reviewer dispatch/result/writeback 本机 E2E、deterministic runtime 外的 Go-owned `cmd/rekit-host` 真实 member/reviewer Claude Code session orchestration与自然语言→人工纠偏→replacement→独立 Reviewer→feature completion live gate、authorized execution observation evidence + bounded adapter execution report strict intake/contract projection/read-only validation preflight、pack-memory promote/reconsume package E2E，以及 evidence ledger runtime。固定 IDA inspector 已把已有 TSV 索引接入 exact request/profile/gate/contained child/receipt/observation/evidence/member/Reviewer 链，但不安装、不连接或控制 IDA，也不执行外部 bridge。尚未落地的是更多真实工具 bridge、更多 pack 的真实 session 产品场景、普通安装交付和跨平台 product-path E2E。

## 执行清单

- [x] 将项目定位从 context kit 扩展为面向网络安全研究与安全工程任务的 Agent Team 框架，`vmp-re` 作为首个成熟 pack。
- [x] 增加 Agent Team roles、packet、candidate/review/confirmed 流程。
- [x] 增加主线/功能支线日常入口与 handoff 说明。
- [x] 增加 heavy-tool gate 和轻到重分析路线。
- [x] 将 `ida-agent-bridge` 作为 candidate tooling 记录，而非硬依赖。
- [x] 增加 evidence/intervention ledger 草案。
- [x] 增加 orchestration 计划和 pack authoring template。
- [x] 将 evidence ledger 从文档推进为 Go-owned append-only JSONL（`internal/rekit/note/**` append/list + `internal/rekit/mission/**` typed facts/brief + `internal/rekit/workstream/**` consume/writeback，9 种 kind 与 decision 字段对齐 `docs/evidence-ledger.md`，见 `docs/agent-team-rollout-plan.md` §4-§5）。
- [x] 将 typed autonomy + `authorized-gate` 从 authorization preflight 推进为 executor/tool-adapter execution evidence closure：Go `gate -ExecutionReportContract -GateEventId ... -Format json` 只读投影 adapter execution report contract（含 `defaultReportPath`、`liveValidation.authorizedWorkspaces[]` / `reportFileName` / `caseRelativeReportPath`、workspace-relative 与 case-relative validate/record handoff，以及 validation taxonomy），Mission brief / overview / handoff / continue artifacts 直接显示 authorized-gate `eventId` 与可复制 `reportContract` command，`gate -ValidateExecutionReport -GateEventId ... -ExecutionReportPath ... -Format json` 以 `valid=true/false` non-mutating envelope 只读校验 bounded adapter sidecar，`gate -Apply -GateEventId ... -ExecutionStatus ... -ExecutionReportPath ...` 消费 authorized-gate、actual budget、output refs、evidence refs、boundary hits、escalation 与 strict validated bounded adapter execution report provenance，并写 observation evidence；真实工具业务逻辑仍留在 lane executor / tool adapter，不塞入 core runtime。
- [x] 将 `ida-agent-bridge` 保持为不安装、不连接的 candidate contract，并另行实现 compiled-in `vmp-ida-index-inspector`：只读已有固定 TSV，不控制 IDA，不执行 candidate `entry`。
- [x] 将 bounded dispatch 从完整 contract 推进为 Mission Commander 可验证 E2E：主 Agent实际 spawn 只读 reviewer，完成 result intake、WhatIf、ledger writeback 与 post-validation；`plan-subagents` 本身仍不自动 spawn。
- [x] 在 deterministic runtime 外增加 Go-owned `cmd/rekit-host`，自动消费 durable current step、启动真实 Claude Code member/reviewer session、收取真实 structured output、处理有界 replacement，并用显式 fresh `vmp-re` live gate 验证人工纠偏、新会话接手、canonical accepted Reviewer lineage 与 feature completion；普通 Go tests 不伪造或启动 LLM。
- [ ] 扩展 `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native`、`generic-binary-re` 等安全领域 pack（`web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re` 已有最小骨架；后续按真实需求继续扩展）。

## 验证标准

维护本文件或相关映射后，至少执行 Go-native release readiness 子集：

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command doctor
git diff --check
```

文档内容还应满足：

- 不声称当前已经具备完整自动脱壳、自动逆向、自动漏洞挖掘、自动恶意样本分析或通用自动渗透能力。
- 不把外部工具写成硬依赖。
- 不泄漏真实样本、路径、trace、dump、artifact 或 case 进度。
- 每个吸收点都能指向当前 repo 中的落地文件或后续计划文档。

## 风险与注意事项

- 外部参考只能作为设计来源，不能替代本项目的 manifest、runtime 和 pack 边界。
- `ida-agent-bridge` 当前是 candidate，不是必装工具。
- `clark-utov` 风格的 batch/ledger/intervention 已从文档推进到 runtime ledger（`/rekit note` 9 种 kind + auto decision 字段对齐 `docs/evidence-ledger.md`）；effective open intervention 的 durable blocker、显式 `reconcile`、reviewer-wave pause 与 stale mutation fail-closed 门禁已实现。尚未实现的是 batch 模型（`batchId`/整体接受/回滚）、由 `continue` 自动生成 intervention/rollback 以及完整 batch-level replay，不能误导为 runtime 已完整实现。
- lane 文档/packet 只表达授权意图；heavy trace、dynamic debug、inject、patch、dump、symex、网络/外部副作用只有在 strict durable autonomy profile + `authorized-gate` decision 完全覆盖时才可由 executor 执行；未授权、越界、出现新风险或需要 confirmed/authority/promote 时必须升级给用户。
- confirmed / authority 写入仍必须经 evidence、verifier、schema、backup、diff、无冲突等 gate。

## 1. 总览表

| 参考来源 | 吸收的核心思想 | 当前落地文件 | 当前状态 |
|---|---|---|---|
| 微信文章：Agent 化逆向经验 | 多 Agent 分工、上下文管理、人工确认、handoff、证据先行 | `docs/vision.md`、`docs/agent-team-usage.md`、`common/policies/agent-team.md`、`packs/vmp-re/references/vmp-re/agent-driven-re.md` | 已落地为工作方式和 policy；自动编排仍在计划中 |
| `TsingShui/ida-agent-bridge` | IDA sidecar/bridge、短连接查询、function index、strings/imports/xref、避免全量输出 | `packs/vmp-re/tooling/catalog.yml`、`packs/vmp-re/tooling/recipes/ida-agent-bridge-readonly.md`、`internal/rekit/adapterhost/vmp_ida_*.go` | 外部 bridge 仍是 candidate；其固定 TSV 子集已由 compiled-in `vmp-ida-index-inspector` 落地为非硬依赖的只读 authorized adapter |
| `clarkluoluo/clark-utov` | batch/ledger/intervention、agent-as-judge、轻到重门禁、可回滚记录 | `docs/evidence-ledger.md`、`docs/orchestration-plan.md`、`internal/rekit/note/**`、`internal/rekit/mission/**`、`internal/rekit/workstream/**`、`packs/vmp-re/references/vmp-re/toolchain-router.md`、`workflow-template.md` | 设计契约与 Go-owned ledger/read-model/runtime 已落地；9 种 kind + decision 字段已对齐草案，batch-level replay/resume 与 candidate → verified → confirmed 机器强制 gate 待实现 |

## 2. 微信文章方向：Agent 化逆向工作流

### 2.1 吸收点

主要吸收的是“让 LLM 像一个可交接的逆向团队工作”，而不是单轮聊天式分析：

- 主 agent 负责目标选择、任务拆分、上下文收敛和最终确认。
- 功能支线 agent 负责窄范围探索，不把临时结论直接写成 confirmed。
- reviewer / verifier 负责只读复核和门禁。
- 人类确认者负责重型动作、外部副作用、confirmed/authority 写入。
- 每轮工作要有 handoff / resume，避免上下文丢失后只能重新猜。

### 2.2 当前落地

| 能力 | 落地位置 | 说明 |
|---|---|---|
| Agent Team 定位 | `README.md`、`CLAUDE.md`、`docs/vision.md` | 项目定位已从 context kit 扩展为网络安全研究 / 安全工程 Agent Team 框架，`vmp-re` 是首个成熟 pack |
| 角色与 packet | `common/policies/agent-team.md`、`packs/vmp-re/references/vmp-re/agent-driven-re.md` | 定义主 agent、功能支线、tooling agent、reviewer、verifier、人类确认者 |
| 主线 / 功能支线 | `/rekit overview`、`continue`、`start`、`handoff`；说明见 `docs/agent-team-usage.md` | 工作线仍是核心，不是被新架构替代 |
| candidate -> review -> confirmed | `agent-driven-re.md`、`common/policies/agent-team.md` | 结论必须先进入 candidate/review，不直接写 confirmed |
| 人工确认边界 | `CLAUDE.md`、`toolchain-router.md`、`tool-adapters.md` | heavy action、confirmed/authority、外部副作用、schema 迁移需确认 |

### 2.3 当前还没落地

- 还没有自动调度多个子 agent 的完整 runtime（R5 判定 runtime 不自动 spawn，由主会话用 Agent 工具完成）。
- 还没有机器强制的 candidate -> verified -> confirmed gate（runtime 不强制，靠 policy 契约 + `note` 手动落账）。
- evidence ledger runtime 写入已落地（`/rekit note` 9 种 kind + auto 流程 decision 字段对齐草案 + `batchId` + intervention/rollback 展示闭环），但 candidate → verified → confirmed 的机器强制门禁尚未实现。

对应后续计划：`docs/orchestration-plan.md`、`docs/evidence-ledger.md`、`docs/batch-plan.md`。

## 3. `ida-agent-bridge`：工具 bridge 与 sidecar 策略

### 3.1 吸收点

吸收的是“不要把 IDA 输出当作聊天粘贴材料，而要把它变成可查询、可裁剪、可复用的工具接口”：

- 先读 function index、strings、imports 等小索引。
- 需要时窄范围查询 pseudocode、xref、hexdump。
- 默认避免全量 decompile/disasm 输出。
- rename/comment/patch 会修改共享 IDB，不能让多 Agent 并发写。
- 工具先以 candidate 身份进入 catalog，经过多个 case 验证后再上升为 mainline-template。

### 3.2 当前落地

| 能力 | 落地位置 | 说明 |
|---|---|---|
| candidate tooling | `packs/vmp-re/tooling/catalog.yml` | 新增 `ida-agent-bridge`，状态为 `candidate` |
| recipe | `packs/vmp-re/tooling/recipes/ida-x64dbg-mcp.md` | 说明 function index、strings、imports、窄范围查询、stoploss |
| read-only packet contract | `packs/vmp-re/tooling/recipes/ida-agent-bridge-readonly.md` | 定义只读 index adapter capability card、packet schema、sidecar/evidence refs、limits/truncation 与禁止项 |
| adapter 契约 | `common/policies/tool-adapters.md` | 定义 capability card、输出契约、side effects、stop conditions |
| 重型工具门禁 | `packs/vmp-re/references/vmp-re/toolchain-router.md` | full trace/debug/inject/patch/dump/symex 需要 reason、budget、outputs、stop conditions、confirmation |

### 3.3 已落地与仍未落地的边界

- 已落地 fixed-purpose runtime adapter `vmp-ida-index-inspector`：只读取已有 `function_index.tsv` 及可选 strings/imports/xrefs，以 literal query 生成有界 packet，并进入 exact profile、`authorized-gate`、dispatch、receipt/observation 和独立 evidence/member/Reviewer 链。
- 没有把外部 `ida-agent-bridge` 安装、启动或绑定成硬依赖，也没有在 `/rekit` 中调用它。
- fixed adapter 不打开 `.i64/.idb`，不生成导出，不联网，不执行 catalog `entry`。
- 没有允许自动 rename/comment/patch/debug/dump 或通用 IDA 控制。

这是刻意保守的：把经过验证的固定文本输入子集编译进显式 adapter ID，而不是把不稳定外部工具或动态 plugin 变成模板硬依赖。

## 4. `clark-utov`：batch、ledger、intervention 与 gate

### 4.1 吸收点

吸收的是“可审计、可回放、可回滚”的自动化逆向工程治理方式：

- 每批工作要有 batch / run / digest。
- 发现、候选、验证、决策、干预、回滚应是结构化事件。
- agent 可以提出判断，但 verifier / judge 要独立复核。
- 重型动作必须走轻到重升级：先静态 triage、窄 trace、value-flow，再考虑 full trace/debug/dump。
- 失败、阻塞、回滚不是聊天里的口头描述，应写成事件。

### 4.2 当前落地

| 能力 | 落地位置 | 说明 |
|---|---|---|
| evidence / intervention ledger 草案 | `docs/evidence-ledger.md`、`internal/rekit/note/**`、`internal/rekit/mission/**`、`internal/rekit/workstream/**` | 草案定义 9 种事件；Go runtime 的 note append/list、typed facts/brief 与 workstream consume/writeback 已对齐核心字段和 verification/intervention 扩展字段 |
| orchestration 计划 | `docs/orchestration-plan.md` | 定义 Planner、Dispatcher、Gate、Digest、Ledger 分阶段实现 |
| 轻到重路线 | `packs/vmp-re/references/vmp-re/workflow-template.md` | static triage -> I/O shape -> focused trace -> value-flow -> verifier -> confirmed |
| heavy-tool gate | `toolchain-router.md`、`common/policies/tool-adapters.md` | 重型动作必须记录原因、预算、输出、止损和确认 |
| batch / goal / PowerShell-free 路线固化 | `docs/batch-plan.md`、`docs/autonomous-goal.md`、`docs/powershell-deprecation.md` | 防止上下文压缩后偏离路线；详细路线、关键决策、验证结果和下一步必须写回 repo docs |

### 4.3 当前还没落地

- append-only ledger runtime 已落地（`/rekit note` 9 种 kind + auto 流程 decision + `batchId`），但 batch-level replay/resume 与整体接受/回滚自动化尚未实现。
- intervention / rollback event 可由 `/rekit note` 手动写入，并已进入 overview/handoff/note-List 展示闭环；尚未由 `/rekit continue` auto 流程自动写入（auto 仅写 observation/request/candidate/publication/decision）。
- agent-as-judge 尚未成为固定命令。
- batch-level replay / resume 还处于设计阶段。

## 5. 当前项目新增能力清单

### 5.1 已经可用

| 能力 | 使用方式 | 说明 |
|---|---|---|
| 新架构使用指南 | `docs/agent-team-usage.md` | 说明新 case、旧 case、主线/功能支线、后续优化 |
| 维护入口 | `CLAUDE.md` | Claude Code 维护本仓库时的入口与边界 |
| Agent Team policy | `common/policies/agent-team.md` | 跨 pack 的角色、packet、状态流、确认边界 |
| Tool adapter policy | `common/policies/tool-adapters.md` | 外部工具状态、capability card、side effects、stop conditions |
| VMP Agent Team reference | `packs/vmp-re/references/vmp-re/agent-driven-re.md` | case 内可同步的 VMP Agent Team 工作方式 |
| 轻到重 VMP 路线 | `workflow-template.md` | 限制先重型 trace/debug 的冲动 |
| heavy-tool gate | `toolchain-router.md` | 重型动作需要确认和止损 |
| `ida-agent-bridge` candidate | `tooling/catalog.yml`、`ida-x64dbg-mcp.md`、`ida-agent-bridge-readonly.md` | 外部工具候选，不是硬依赖；已定义只读 index packet contract |
| pack 作者骨架 | `packs/_template/` | 后续创建新 pack 的最小模板；`web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re` 提供安全领域 skeleton 参考 |
| case smoke 验证过的 runtime | `cmd/rekit/**`、`internal/rekit/**`；`rekit/rekit.ps1` 仅 retained compatibility façade | Go-owned/no-fallback case lifecycle、sync/promote 与 workstream 边界已有 package/smoke 覆盖；legacy `rekit/lib/*.ps1` 已删除 |

### 5.2 部分可用 / 设计已就绪

| 能力 | 当前状态 | 下一步 |
|---|---|---|
| evidence ledger | runtime 已落地（`/rekit note` 9 种 kind + overview/handoff/note-List 读层 + auto decision 字段对齐草案） | 索引优化（SQLite 仅在查询压垮 runtime 时） |
| orchestration | `plan-subagents` planning mode 生成 route/shard/review-loop observability 与 read-only reviewer contract；显式 reviewer intake 已支持 strict WhatIf/Apply、verification-before-decision facts 写回、幂等重试与 post-validation（runtime 仍不自动 spawn 或管理 reviewer/session） | 下一步是统一 session/reviewer orchestration 与跨工具 adapter 实际调用 |
| heavy-tool gate runtime | Go-owned/no-fallback `gate -WhatIf/-Apply` preview/request/evidence 与 `note`、`overview`、`handoff` 读写/投影链路；可写 pending/authorized gate decision，也可在授权动作后写 observation execution evidence（actual budget / output refs / evidence refs / boundary hits / escalation）；不执行 heavy-tool、不写 confirmed/authority | 后续由 lane executor/tool adapter 消费授权并承担真实工具调用、隔离、停止条件和 adapter-specific validation |
| tool adapter | `_template` fixture inspector + `vmp-re` fixed `vmp-ida-index-inspector` 已有真实 contained-child/receipt/Claude E2E；外部 `ida-agent-bridge` 仍是 candidate | 后续按真实需求增加其它固定低风险 adapter；不先建动态 plugin registry |
| 多 pack 扩展 | `_template` + `packs/web-security/` + `packs/malware-analysis/` + `packs/vuln-research/` + `packs/ctf/` + `packs/unpack-pe/` + `packs/ollvm/` + `packs/android-native/` + `packs/generic-binary-re/` | `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re` 已有 skeleton；后续按真实需求继续扩展领域 pack |

### 5.3 尚未实现，不能对外宣称

- 全自动脱壳。
- 全自动反虚拟化。
- 全自动算法还原。
- 全自动漏洞挖掘或漏洞利用链生成。
- 全自动恶意样本分析平台。
- 通用自动渗透执行平台。
- 自动 IDA/x64dbg 操作。
- 自动 patch / dump / inject。
- 无人工确认的 confirmed / authority 写入。

## 6. 另一台电脑接手建议

### 6.1 只接手维护 kit 仓库

```text
git clone https://github.com/shuiyu486/re-context-kits
cd re-context-kits
claude
```

给 Claude 的接手提示：

```text
你正在接手 re-context-kits 项目。
请先读 CLAUDE.md 与 docs/context-routing.md；再按当前任务只读 docs/batch-plan.md 顶部 current/next、CHANGELOG.md 顶部 Unreleased，以及 context-routing 指向的场景入口顶部。
本项目目标是逐步优化为网络安全研究 / 安全工程 Agent Team 框架；当前以 vmp-re 作为首个成熟 pack，不是完整自动脱壳器、自动逆向引擎、自动漏洞挖掘器、自动恶意样本分析平台或通用自动渗透平台。
不要在 kit 仓库里创建真实 case；验证 init/attach/sync/promote 时只用临时 case。
请总结当前已落地能力和待实现能力，然后按 docs/batch-plan.md 选择下一批最小可验证优化。
```

### 6.2 接手已有 case

推荐目录：

```text
<workspaceRoot>\
  kits\
    re-context-kits\
  cases\
    <caseName>\
```

绑定旧 case：

```text
/rekit attach -Target <workspaceRoot>\cases\<caseName> -Pack vmp-re
/rekit sync
/rekit doctor
```

如果目录迁移过：

```text
/rekit status
/rekit repair
确认修复，执行 repair -Apply
/rekit doctor
```

日常接手：

```text
/rekit overview
/rekit continue main
```

专项支线：

```text
/rekit start <feature>
/rekit continue <feature>
```

## 7. 后续优化建议

推荐下一批按最小可验证切片推进：

1. **固化 smoke test**
   - 将现有临时 `init/attach/sync/promote` 验证整理成脚本或 CI 文档。
   - 风险低，能保护后续 runtime 改动。

2. **ledger 最小实现**（已完成）
   - `.rekit/facts/*.jsonl` 已落地 9 种 kind 文件（observation/hypothesis/candidate/verification/decision/intervention/rollback/publication/request）。
   - `/rekit note` 手动 append + `/rekit continue` auto 流程写入；`overview`/`handoff`/`note -List` 读层聚合查询。

3. **heavy-tool gate packet**
   - 增加只读命令或 helper，生成 heavy action request packet。
   - 不直接运行工具，只把确认材料标准化。

4. **`ida-agent-bridge` 只读 adapter 草案**（已完成 contract）
   - 已定义 function index、strings、imports、xrefs、selected snippet 的只读 packet schema 与 sidecar/evidence ref 规则。
   - 不做 rename/comment/patch；后续实现 runtime-level adapter 前先用多个真实 case 验证 contract。

5. **新 pack 试点**
   - 从 `packs/_template/` 派生一个低风险 pack，例如 `web-security`、`ctf` 或 `generic-binary-re`。
   - 用临时 case 验证 init/sync/promote。
