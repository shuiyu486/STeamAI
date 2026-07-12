# Reference absorption map

## 读取指南

本文件用于回答：本项目从外部参考文章和项目中吸收了什么、已经落地了哪些能力、哪些仍是后续计划。

如果你是在另一台电脑接手维护本项目，建议先读：

1. `CLAUDE.md`
2. `README.md`
3. `docs/agent-team-usage.md`
4. 本文件
5. `docs/vision.md`
6. `docs/batch-plan.md`

本文件只记录可复用设计和落地映射，不记录真实样本名、RVA/VA、trace/dump、artifact 路径、客户信息或 case-specific 进度。

## 实施摘要

本项目从参考资料中吸收的不是“某个现成脱壳器”，而是三类能力：

1. **Agent Team 工作方式**：多 Agent 分工、上下文隔离、工作线、handoff、candidate -> review -> confirmed。
2. **工具 adapter 策略**：IDA/调试器/trace 等工具先 recipe 化、candidate 化，再逐步 adapter 化，避免成为硬依赖或大输出源。
3. **证据与门禁模型**：evidence ledger、batch/intervention、heavy-tool gate、人工确认和可回滚的审查流程。

当前已经落地的是安全 Agent Team 框架底座、文档契约、`/rekit` 工作线 runtime、review-first sync/promote、首个成熟 pack `vmp-re` 扩展、非 RE pack 骨架 `web-security`、`malware-analysis` 与 `vuln-research`、tooling candidate、`ida-agent-bridge` 只读 packet contract，以及 evidence ledger runtime（`/rekit note` 手动 append 9 种 kind 事件 + overview/handoff/note-List 读层 + auto 流程 decision 字段对齐 `docs/evidence-ledger.md` 草案）；尚未落地的是 runtime 强制 heavy-tool 执行闭环、真实工具 bridge adapter 实现、自动多 Agent dispatch（R5 判定 runtime 不自动 spawn，由主会话用 Agent 工具完成），也不能宣称已具备自动脱壳/逆向引擎、自动漏洞挖掘器、自动恶意样本分析平台或通用自动渗透平台。

## 执行清单

- [x] 将项目定位从 context kit 扩展为面向网络安全研究与安全工程任务的 Agent Team 框架，`vmp-re` 作为首个成熟 pack。
- [x] 增加 Agent Team roles、packet、candidate/review/confirmed 流程。
- [x] 增加主线/功能支线日常入口与 handoff 说明。
- [x] 增加 heavy-tool gate 和轻到重分析路线。
- [x] 将 `ida-agent-bridge` 作为 candidate tooling 记录，而非硬依赖。
- [x] 增加 evidence/intervention ledger 草案。
- [x] 增加 orchestration 计划和 pack authoring template。
- [x] 将 evidence ledger 从文档推进为 runtime append-only JSONL（`/rekit note` + `Add-RekitFactEvent` 9 种 kind + overview/handoff/note-List 读层 + auto 流程 decision 字段对齐 `docs/evidence-ledger.md`，见 `docs/agent-team-rollout-plan.md` §4-§5）。
- [ ] 将 heavy-tool gate 从文档推进为 runtime packet / confirmation flow（R6 已落 `packs/vmp-re/policies/verification.overlay.md` 用 `note -Kind request -Status pending-gate` 登记 gate 事件，runtime 不强制 gate，属 Phase 6 后段）。
- [x] 将 `ida-agent-bridge` 从 candidate tooling 推进到只读 packet contract / capability card（仍不安装、不连接、不实现 runtime-level adapter）。
- [ ] 将 bounded dispatch 从计划推进为可验证 runtime 功能（R5 已判定 runtime 不扩，spawn 是主会话职责；`plan-subagents` + `note -Kind decision` 构成支撑）。
- [ ] 扩展 `web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 等安全领域 pack（`web-security`、`malware-analysis` 与 `vuln-research` 已有最小骨架，其它仍待推进）。

## 验证标准

维护本文件或相关映射后，至少执行：

```powershell
.\rekit\rekit.ps1 status
.\rekit\rekit.ps1 doctor
```

并执行：

```bash
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
- `clark-utov` 风格的 batch/ledger/intervention 已从文档推进到 runtime ledger（`/rekit note` 9 种 kind + auto decision 字段对齐 `docs/evidence-ledger.md`），但 batch 模型（`batchId`/整体接受/回滚）与 intervention runtime 强制门禁仍未实现，不能误导为 runtime 已完整实现。
- heavy trace、dynamic debug、inject、patch、dump、symex、网络/外部副作用仍必须用户确认。
- confirmed / authority 写入仍必须经 evidence、verifier、schema、backup、diff、无冲突等 gate。

## 1. 总览表

| 参考来源 | 吸收的核心思想 | 当前落地文件 | 当前状态 |
|---|---|---|---|
| 微信文章：Agent 化逆向经验 | 多 Agent 分工、上下文管理、人工确认、handoff、证据先行 | `docs/vision.md`、`docs/agent-team-usage.md`、`common/policies/agent-team.md`、`packs/vmp-re/references/vmp-re/agent-driven-re.md` | 已落地为工作方式和 policy；自动编排仍在计划中 |
| `TsingShui/ida-agent-bridge` | IDA sidecar/bridge、短连接查询、function index、strings/imports/xref、避免全量输出 | `packs/vmp-re/tooling/catalog.yml`、`packs/vmp-re/tooling/recipes/ida-x64dbg-mcp.md`、`packs/vmp-re/tooling/recipes/ida-agent-bridge-readonly.md`、`common/policies/tool-adapters.md` | 已作为 candidate tooling，并补只读 packet contract；未成为硬依赖 |
| `clarkluoluo/clark-utov` | batch/ledger/intervention、agent-as-judge、轻到重门禁、可回滚记录 | `docs/evidence-ledger.md`、`docs/orchestration-plan.md`、`packs/vmp-re/references/vmp-re/toolchain-router.md`、`workflow-template.md`、`rekit/lib/B3.State.ps1`（`Add-RekitFactEvent`）、`rekit/lib/B3.Auto.ps1`（`New-RekitDecision`） | 设计契约已落地；runtime ledger 9 种 kind + decision 字段已对齐草案，batch 模型与 intervention 强制门禁待实现 |

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

### 3.3 当前还没落地

- 没有把 `ida-agent-bridge` 安装或绑定成硬依赖。
- 没有实现 runtime-level IDA adapter（当前只定义只读 packet contract / recipe）。
- 没有在 `/rekit` 中直接调用 IDA bridge。
- 没有允许自动 rename/comment/patch。

这是刻意保守的：工具先 recipe 化，再 adapter 化，避免把不稳定外部工具变成模板硬依赖。

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
| evidence / intervention ledger 草案 | `docs/evidence-ledger.md`、`rekit/lib/B3.State.ps1`、`rekit/lib/B3.Auto.ps1` | 草案定义 9 种事件；runtime `Add-RekitFactEvent`/`New-RekitDecision` 已对齐草案字段（`schemaVersion`/`actor`/`risk`/`related`/`decision`/`confirmedBy`/`writes` + verification/intervention 扩展字段） |
| orchestration 计划 | `docs/orchestration-plan.md` | 定义 Planner、Dispatcher、Gate、Digest、Ledger 分阶段实现 |
| 轻到重路线 | `packs/vmp-re/references/vmp-re/workflow-template.md` | static triage -> I/O shape -> focused trace -> value-flow -> verifier -> confirmed |
| heavy-tool gate | `toolchain-router.md`、`common/policies/tool-adapters.md` | 重型动作必须记录原因、预算、输出、止损和确认 |
| batch 计划固化 | `docs/batch-plan.md` | 防止上下文压缩后偏离路线 |

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
| pack 作者骨架 | `packs/_template/` | 后续创建新 pack 的最小模板；`web-security`、`malware-analysis` 与 `vuln-research` 提供非 RE skeleton 参考 |
| case smoke 验证过的 runtime | `rekit/rekit.ps1`、`rekit/lib/*.ps1` | `init/attach/sync/promote` 边界已验证 |

### 5.2 部分可用 / 设计已就绪

| 能力 | 当前状态 | 下一步 |
|---|---|---|
| evidence ledger | runtime 已落地（`/rekit note` 9 种 kind + overview/handoff/note-List 读层 + auto decision 字段对齐草案） | 索引优化（SQLite 仅在查询压垮 runtime 时） |
| orchestration | `plan-subagents` 只读计划器 + route/shard/review-loop observability + `note -Kind decision` verdict 写回（R5 判定 runtime 不自动 spawn） | 跨工具 adapter 实际调用属 Phase 6 后段 |
| heavy-tool gate runtime | overlay 契约 + PowerShell `note -Kind request -Status pending-gate` + Go `gate -WhatIf/-Apply` preview/request；PowerShell `overview`/`handoff`/`note -List` 已展示 Go request 的 actor/risk/target/batch/gate 详情；不执行 heavy-tool、不写 confirmed/authority、不默认接入 façade | Phase 6 后段 runtime 强制 gate 与受控执行闭环 |
| tool adapter | policy + candidate + `ida-agent-bridge` 只读 packet contract | 后续多个真实 case 验证后，再考虑 runtime-level adapter 或其它工具 adapter |
| 多 pack 扩展 | `_template` + `packs/web-security/` + `packs/malware-analysis/` + `packs/vuln-research/` | `web-security`、`malware-analysis` 与 `vuln-research` 已有 skeleton；继续新增 `ctf` / `unpack-pe` / `ollvm` / `android-native` pack |

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

```powershell
git clone https://github.com/shuiyu486/re-context-kits
cd re-context-kits
claude
```

给 Claude 的接手提示：

```text
你正在接手 re-context-kits 项目。
请先读 CLAUDE.md、README.md、docs/agent-team-usage.md、docs/reference-absorption.md、docs/vision.md、docs/batch-plan.md。
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
