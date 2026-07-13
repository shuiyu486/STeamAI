# re-context-kits vision

## 读取指南

- 如果你只想使用当前仓库初始化一个安全 case（当前成熟示例是 `vmp-re` RE case），先读 `README.md` 的使用方式；本文件用于理解长期方向与阶段路线。
- 如果你要维护或迭代本仓库，先读本文件顶部的实施摘要、执行清单、验证标准；若要让新会话长期自主推进几十轮，先读 `docs/autonomous-goal.md`，再按阶段读取细节。
- 本文件是路线图，不代表所有能力已经实现；当前已经落地的是 `/rekit`、case 绑定、工作线、handoff、sync/promote、首个成熟领域 pack `vmp-re` 和 tooling 文档底座。
- 需要具体执行时，优先选择当前阶段的最小可验证切片，不跨阶段提前重构 runtime。

## 实施摘要

`re-context-kits` 的长期定位是：**面向网络安全研究与安全工程任务的 Claude Code Agent Team 框架**。

它不是单个脱壳工具、IDA 插件、漏洞扫描器或自动化脚本集合，而是把多 Agent 协作、领域工具链、证据账本、工作线管理、验证门禁和可复用安全领域 pack 组织成可持续迭代的 case workspace。`vmp-re` 是当前首个成熟 pack 和验证场，不是最终边界；长期目标是逐步支持逆向工程、恶意样本分析、漏洞研究、Web/API 安全评估、授权测试/靶场/CTF、Android native、OLLVM 等多类安全任务。

当前项目的合理边界是：

- Agent 负责决策、拆解、复核和调度。
- 外部领域工具负责执行静态/动态/trace/仿真、安全测试、分析或验证任务。
- `rekit` runtime 负责 case 状态、工作线、sync/promote、handoff 和可审计流程。
- `packs/<pack>` 负责领域知识、工具路由、验证规则和可复用模板。
- case-local 目录保存目标样本/系统信息、trace、dump、当前进度和私有结论。

## 执行清单

- [ ] Phase 0：定位与文档收敛。
- [ ] Phase 1：Agent Team 工作流固化。
- [ ] Phase 2：`vmp-re` 专项能力深化。
- [ ] Phase 3：网络安全多领域 pack 扩展（Web/API 安全、恶意样本分析、漏洞研究、CTF/靶场、通用 PE unpacking、Android native、OLLVM、通用二进制分析等；`web-security`、`malware-analysis`、`vuln-research`、`ctf`、`unpack-pe`、`ollvm`、`android-native` 与 `generic-binary-re` 已作为首批安全领域 pack 骨架启动）。
- [ ] Phase 4：工具适配与候选工具路由。
- [ ] Phase 5：证据账本与 intervention 模型增强。
- [ ] Phase 6：半自动 Agent Team runtime / orchestration。
- [ ] Agent Team rollout（选项 C：契约 dry-run 优先）：见 `docs/agent-team-rollout-plan.md`，先压测契约再按真实缺口决定 Phase 5/6 顺序。
- [ ] Go-first 收束 / release readiness：见 `docs/go-first-convergence-plan.md` 与 `docs/autonomous-goal.md`，Batch 101 后优先让 Go backend 成为 deterministic runtime owner，收缩 PowerShell 编排扩张，建立 Agent Team 真实 dry-run 闭环，并把长期自主推进约束写成可复制 goal。

每个阶段都应按“小步可验证”落地：先文档契约，再 case-local 试用，再 runtime 自动化，最后才抽象为跨 pack 能力。

## 批次执行协议

用户已授权后续可按本文件路线分批实施，不必每个小批次都先停下来确认。为了避免上下文压缩后执行偏差，后续会话应按以下规则推进：

1. **先读计划区**：开始新批次前先读本文件顶部的读取指南、实施摘要、执行清单、验证标准、风险与注意事项，以及当前阶段的小节。
2. **分批实施**：每批只做一个可验证切片，优先保持现有模块化；不跨阶段提前重构 runtime。
3. **批次自审**：每批完成后主动 review 自己的 diff，评估架构是否清晰、是否保持模块边界、是否引入维护风险或同步副作用。
4. **可自行调整**：若自审发现低风险文档职责重复、链接缺失、manifest 漏同步、CHANGELOG 漏记录等问题，可直接修正并再次验证。
5. **必须停下询问**：遇到产品方向变化、破坏性动作、外部副作用、动态调试/注入/patch/dump、写入 confirmed/authority、runtime schema 迁移、难以判断的架构取舍时，先问用户。
6. **计划写回文档**：后续阶段的具体实施计划应持续沉淀到本文件或相邻设计文档中，不能只留在聊天上下文里。
7. **每批都验证**：至少运行 `git diff --check`；能运行时再执行 `./rekit/rekit.ps1 status` 和 `./rekit/rekit.ps1 doctor`，失败要区分本批问题和既有阻塞。

## 验证标准

文档阶段至少验证：

1. README、CLAUDE.md、vision 文档对项目定位没有互相冲突。
2. README 仍能指导用户完成 `/rekit init` / `/rekit attach` 的现有流程。
3. `docs/vision.md` 不承诺尚未实现的自动能力为已完成能力。
4. `git diff --check` 无空白错误。
5. 可运行时执行 `./rekit/rekit.ps1 status` 与 `./rekit/rekit.ps1 doctor`；若失败，必须区分是本次改动导致还是既有 runtime 问题。

runtime 阶段还应增加：

- 临时 case 的 init/attach/sync/promote smoke test。
- pack manifest containment 与 managed/local 边界验证。
- 关键状态文件的 forward/backward compatibility 检查。
- 失败时可定位到具体 phase、lane、packet、tool adapter 或 ledger event。

## 风险与注意事项

- 不要把“长期目标是安全 Agent Team 框架”误写成“当前已能全自动脱壳、逆向、漏洞挖掘、恶意样本分析或渗透测试”。
- 不把真实样本、RVA/VA、trace、dump、artifact、客户信息、目标系统信息或本机绝对路径写入模板仓库。
- 不把外部工具变成硬依赖；优先以 tooling recipe、capability card、adapter contract 的方式接入。
- 不让 runtime 直接包含具体工具的业务逻辑；工具细节应留在 pack tooling 或 case-local adapter。
- 动态调试、注入、patch、dump、脱壳写文件等高风险动作必须有显式用户确认和可回溯记录。
- 自动化先做 review-first / dry-run / packet 化，再做 apply；不要绕过现有 sync/promote 边界。

## 1. 项目定位

`re-context-kits` 是安全研究 / 安全工程 Agent Team 的上下文、流程和工具编排框架。

它服务的对象有两类：

1. **安全 case 使用者**：在某个安全研究或安全工程 case 中使用 `/rekit overview`、`/rekit continue`、`/rekit start`、`/rekit handoff` 组织长期分析；当前最成熟示例是 `vmp-re` RE case。
2. **框架维护者**：在本仓库中迭代 runtime、pack、policy、tooling recipe、agent workflow 和文档。

最终形态不是一个“大而全脚本”，而是一个模块化 team system：每个 agent、工具、证据、候选结论和验证动作都有边界、输入输出和留痕。

## 2. 非目标

当前阶段不追求：

- 自研替代 IDA、x64dbg、Frida、unidbg、Unicorn、Triton 等成熟工具。
- 把所有 pack 都塞进 `vmp-re`。
- 让单个主 agent 读取全部 trace、反汇编、反编译输出。
- 未经用户确认自动执行破坏性动作、外部副作用或大规模动态调试。
- 在没有 case 证据的情况下把经验直接提升为 confirmed 模板规则。

## 3. 总体架构

推荐长期架构分为七层：

| 层 | 职责 | 当前映射 |
|---|---|---|
| Skill UI 层 | 给 Claude Code 暴露 `/rekit` 入口和用户语义 | `.claude/skills/rekit/SKILL.md`、case shim |
| Case runtime 层 | 管理 case 绑定、工作线、状态、handoff、sync/promote | `rekit/rekit.ps1`、`rekit/lib/*.ps1` |
| Agent Team 层 | 定义主 agent、功能支线、reviewer、工具 agent 的职责和 packet | `common/prompts/**`、`packs/*/prompts/**`、manifest `subagentRoutes` |
| Pack 领域层 | 保存某类安全任务的领域知识、流程、验证标准 | `packs/vmp-re/**`（当前成熟示例） |
| Tooling / adapter 层 | 描述外部工具能力、用法、止损条件和未来 adapter contract | `packs/<pack>/tooling/**`，当前以 `packs/vmp-re/tooling/**` 为主 |
| Evidence ledger 层 | 保存 observation、request、candidate、publication、decision、intervention | `.rekit/facts/*.jsonl`、`.rekit/lanes/**` |
| Verification gate 层 | 决定什么能进入 confirmed / authority，什么必须人工确认 | `common/policies/**`、pack overlays、runtime policy gate |

架构原则：

- runtime 只理解通用 case / lane / packet / manifest，不内嵌具体安全领域工具细节。
- pack 只声明领域流程和工具契约，不写 case 私有状态。
- case-local 只保存当前样本事实、产物、证据和私有脚本。
- confirmed/authority 写入必须比 candidate 写入更严格，并有验证和回滚线索。

## 4. 现有模块如何映射到目标架构

```text
re-context-kits/
  .claude/skills/rekit/        # canonical Claude Code 入口
  rekit/                       # deterministic runtime backend
  common/                      # 跨 pack 的 policy 与 prompt
  packs/vmp-re/                # 当前首个成熟领域 pack / RE 验证场
    references/vmp-re/         # 下发到 case 的 managed docs
    policies/                  # pack-specific policy overlay
    prompts/                   # pack-specific agent prompt
    tooling/                   # 工具 catalog、recipe、patch、candidate
    manifest.yml               # pack 单一事实源
  docs/                        # 设计、路线、迁移、sync/promote 说明
```

短期不要打破这个结构。新增能力优先放在最贴近职责的位置：

- 通用 agent/team 规则：`common/policies` 或 `common/prompts`。
- `vmp-re` 领域流程：`packs/vmp-re/references/vmp-re` 或 `packs/vmp-re/policies`；新增领域应放入对应 `packs/<pack>/`。
- 外部工具经验：`packs/<pack>/tooling`；当前成熟内容主要在 `packs/vmp-re/tooling`。
- runtime 自动化：`rekit/lib`，并保持 Core / State / Policy / Lane / Auto / Commands 分层。
- 长期路线与架构说明：`docs`。

## 5. 阶段实施路线

### Phase 0：定位与文档收敛

**目标：** 让 README、CLAUDE.md、docs/vision.md 对项目定位达成一致，避免维护者把本仓库误解成普通模板仓库，也避免用户误以为当前已具备全自动脱壳、自动逆向、自动漏洞挖掘或通用自动渗透能力。

**实施切片：**

1. README 顶部改为面向网络安全研究与安全工程任务的 Agent Team 框架定位，并说明 `vmp-re` 是首个成熟 pack。
2. 新增本文件，承载长期愿景和阶段路线。
3. CLAUDE.md 补充维护者视角：当前是 Agent Team 的 context/workflow/tooling 底座。
4. CHANGELOG 记录定位调整。

**产物：** `README.md`、`docs/vision.md`、`CLAUDE.md`、`CHANGELOG.md`。

**验证：** 文档互相不冲突；`git diff --check` 通过；尽量运行 `rekit status/doctor`。

### Phase 1：Agent Team 工作流固化

**目标：** 把“主 agent 决策、功能支线探索、review agent 复核、人工确认 confirmed”的流程变成明确的可执行契约。

**建议模块：**

- `common/policies/agent-team.md`：通用 agent team 边界。
- `common/prompts/*`：主 agent、feature agent、review agent 的通用 prompt。
- `packs/vmp-re/references/vmp-re/agent-driven-re.md`：VMP case 中的 agent team 操作手册。
- `packs/vmp-re/manifest.yml`：补充或细化 `subagentRoutes`。

**实施步骤：**

1. 定义 agent role：main、feature、tooling、reviewer、verifier、handoff curator。
2. 定义 packet：task packet、evidence packet、review packet、stuck-point packet、handoff packet。
3. 规定每类 packet 的字段、读写者和生命周期。
4. 将现有 `lane-main-session`、`lane-feature-session`、`lane-merge-review` 与 packet 对齐。
5. 在 case-local `.rekit/lanes/**` 中保持 agent 短命、工作线持久。

**验证标准：**

- 新 case 可以按文档创建主线和功能支线。
- 功能支线能提交 request/candidate，不写 confirmed。
- 主线能消费 candidate 并生成 review-first 结论。
- handoff 能说明当前工作线、开放问题、下一步和验证状态。

**调试维护点：** packet 必须小、结构化、可 diff；不要让 agent 依赖长篇自然语言历史。

### Phase 2：`vmp-re` 专项能力深化

**目标：** 把当前 `vmp-re` 从“模板/经验”推进到更清晰的 VMP 逆向工作流 pack，尤其是轻到重路线、trace/value-flow/focused review、confirmed CSV 和 routine IR 的闭环。

**建议模块：**

- `packs/vmp-re/references/vmp-re/workflow-template.md`
- `packs/vmp-re/references/vmp-re/toolchain-router.md`
- `packs/vmp-re/references/vmp-re/progressive-disclosure.md`
- `packs/vmp-re/tooling/recipes/*.md`
- `packs/vmp-re/policies/verification.overlay.md`

**实施步骤：**

1. 明确轻到重路线：静态 triage → VMEnter/context → focused trace → value-flow → candidate semantics → confirmed CSV → routine IR → superinstruction。
2. 给 full trace、动态调试、x64dbg、Frida、dump/patch 设置 escalation gate：必须有原因、预算、止损条件。
3. 把 value-flow / handler lowering 的证据字段固定化，避免低样本或 alias-heavy 直接进入 confirmed。
4. 为 routine IR 重建和 superinstruction mining 写成固定验证步骤。
5. 将外部文章和工具项目中的通用经验转成 tooling recipe，而不是写进 runtime。

**验证标准：**

- 一个临时 VMP-like case 可以从 `/rekit start` 到 request/candidate/handoff 走通。
- confirmed CSV 变更后有明确重建命令和检查项。
- 大 trace 不进入 Markdown；只保存 sidecar 和摘要。
- 任何 heavy step 都能解释“为什么轻路径走不通”。

**调试维护点：** 每个阶段产物要有确定文件位置、输入输出和失败原因；不要用“看情况”作为唯一流程说明。

### Phase 3：网络安全多领域 pack 扩展

**目标：** 在不污染 `vmp-re` 的前提下，逐步支持 Web/API 安全评估、恶意样本分析、漏洞研究、授权测试/靶场/CTF、通用脱壳、Android native、OLLVM、通用二进制功能分析等领域。

**候选 pack：**

```text
packs/web-security/
packs/malware-analysis/
packs/vuln-research/
packs/ctf/
packs/unpack-pe/
packs/android-native/
packs/ollvm/
packs/generic-binary-re/
```

**pack 最小结构：**

```text
packs/<pack>/
  manifest.yml
  CLAUDE.local.snippet.md
  references/<pack>/README.md
  references/<pack>/workflow-template.md
  references/<pack>/toolchain-router.md
  policies/*.overlay.md
  prompts/*.md
  tooling/catalog.yml
  tooling/recipes/*.md
```

**实施步骤：**

1. 先定义 pack contract：manifest 必填字段、managedFiles、templateFiles、localNeverOverwrite、toolingFiles、budgets。
2. 新增一个 pack skeleton 生成或手工模板，不急着接 runtime 新命令。
3. 每个新 pack 先只有文档、policy、tooling recipe，不引入自动分析逻辑。
4. 用临时 case 验证 `init/attach/sync/promote` 能处理多 pack。
5. 只有当两个以上 pack 重复出现相同需求时，再把共性提到 `common/` 或 runtime。

**验证标准：**

- `-Pack <pack>` 初始化不会越界写文件。
- pack-local managed docs 可以 sync，case-local 文件不会被覆盖。
- promote 不会把 case-specific 数据带回 pack。

**调试维护点：** 新 pack 不复制 runtime；只通过 manifest 和 managed docs 接入。

### Phase 4：工具适配与候选工具路由

**目标：** 将外部工具接入方式标准化，让 agent 知道何时用、怎么用、何时止损、输出保存在哪里，而不是把工具 README 整段塞进上下文。

**候选工具：** IDA / ida-agent-bridge、x64dbg MCP、Frida、unidbg、Unicorn、Triton、Ghidra headless、radare2、公开 unpack/import fixer。

**建议模块：**

- `packs/<pack>/tooling/catalog.yml`：工具能力卡和状态。
- `packs/<pack>/tooling/recipes/*.md`：按任务阶段写使用方法。
- `packs/<pack>/tooling/scripts/README.md`：可模板化脚本接口。
- 未来可新增 `common/policies/tool-adapters.md`：通用 adapter contract。

**实施步骤：**

1. 为每个工具写 capability card：用途、输入、输出、风险、止损、是否可 headless、是否会外部副作用。
2. 统一工具状态：mainline、auxiliary、candidate、cautious、stoploss、deprecated。
3. 把工具输出规范成摘要 + sidecar：Markdown 只放结论、命令、关键证据路径。
4. 对可自动化工具定义 adapter contract，但先不强制实现。
5. 对高风险工具设置确认门禁：debug、inject、patch、dump、network、long trace。

**验证标准：**

- agent 能根据 toolchain-router 选择候选工具，而不是盲试。
- 每个工具都有明确 stop condition。
- 工具失败时能给出下一步，不会无界重试。

**调试维护点：** 工具接入先 recipe 化，再 adapter 化；不要一开始做复杂统一接口。

### Phase 5：证据账本与 intervention 模型增强

**目标：** 将 candidate、confirmed、rejected、superseded、intervention、rollback、batch decision 变成更清晰的 append-only 账本模型，降低长程 RE 中的漂移和返工。

**建议模块：**

- `.rekit/facts/*.jsonl`：继续作为默认 append-only 存储。
- `rekit/lib/B3.State.ps1`：读写和聚合。
- `rekit/lib/B3.Policy.ps1`：确认门禁。
- `rekit/lib/B3.Auto.ps1`：低风险自动整理。
- `common/policies/evidence.md`、`review-first.md`：状态和证据规则。

**实施步骤：**

1. 定义统一事件类型：observation、hypothesis、candidate、verification、decision、intervention、rollback、publication、request。
2. 为事件加字段：id、actor、source lane、subject、evidence refs、confidence/verdict、status、related ids、createdAt。
3. 引入 batch id：一轮自动整理或 review 的候选可整体接受、拒绝或回滚。
4. `overview` 展示 stuck statistics：未决 request、冲突 candidate、需要人工确认、heavy gate 等。
5. 先使用 JSONL，只有查询复杂度确实压垮 runtime 时再考虑 SQLite。

**验证标准：**

- 任何 confirmed 写入都能追溯到 candidate、evidence 和 verifier。
- rejected/superseded 不会消失，后续 agent 能知道为什么不再走旧路。
- rollback 不删除历史，只追加反向事件。

**调试维护点：** 账本必须可人工打开、可 grep、可 diff；不要为了“智能检索”过早引入复杂数据库。

### Phase 6：半自动 Agent Team runtime / orchestration

**目标：** 在前面契约稳定后，让 `/rekit` 能更主动地生成任务包、分派只读复核、收敛结论、提示人工确认和生成 handoff。

**建议模块：**

- `rekit/lib/B3.Commands.ps1`：用户级命令入口。
- `rekit/lib/B3.Auto.ps1`：自动整理和低风险动作。
- `rekit/lib/B3.Policy.ps1`：高风险门禁。
- `packs/<pack>/manifest.yml`：agent routes、tool routes、budgets。
- `.rekit/runs/<run-id>/`：每轮 orchestration digest 和 packet。

**实施步骤：**

1. 先增强 `plan-subagents`：固定分片、输出契约、packet 生成。
2. 再实现 bounded dispatch：只读 review agent 可以消费 packet，不能写 authority。
3. 增加 heavy-tool gate：需要动态调试、full trace、patch、dump 时生成确认问题和预算说明。
4. 增加 replay/debug：每轮自动动作写 run digest、输入 packet、输出 verdict。
5. 最后才考虑跨工具 adapter 的实际调用；不要让 orchestration 一开始就控制所有工具。

**验证标准：**

- dry-run 能完整展示将要分派什么、读取什么、写什么。
- 任一 agent/task 失败不会污染 confirmed 状态。
- 用户可以从 `.rekit/runs/<run-id>/digest.md` 理解本轮发生了什么。
- 默认不执行外部副作用；写入动作仍 review-first。

**调试维护点：** orchestration 必须可暂停、可重放、可解释；每个自动动作要有来源、理由和边界。

## 6. 模块化原则

1. **职责单向依赖**：runtime 读 manifest 和 case state；pack 不反向依赖 case；case shim 不复制 runtime 逻辑。
2. **先契约后自动化**：先把 packet/schema/policy 写清楚，再让脚本生成和消费。
3. **工具可替换**：工具 recipe 描述能力，不把某个外部工具做成唯一道路。
4. **case 私有数据不回流**：promote 必须脱敏、review-first，并受 deny pattern 约束。
5. **confirmed 更难写**：candidate 可以多，confirmed 必须少、准、可追溯。
6. **长输出外置**：trace、反汇编、反编译、tool log 保存在 sidecar，Markdown 只保留摘要和定位。

## 7. 调试与维护原则

- 每个 runtime 命令都要能回答：目标 case 是谁、pack 是谁、会读哪些文件、会写哪些文件、如何回滚。
- 每个自动流程都要生成 digest，方便新会话接手。
- 每个 pack 都要能独立 doctor，避免一个领域的规则破坏另一个领域。
- 每个工具 adapter 都要有 timeout、stop condition、输出目录和失败摘要。
- 每个阶段都保留人工确认点，不把“继续”解释成扩大授权。

## 8. 后续任务候选清单

近期优先级建议：

1. 已完成：写 `packs/vmp-re/references/vmp-re/agent-driven-re.md`，把主 agent / feature lane / reviewer / human gate 的工作方式讲清楚。
2. 已完成：扩写 `workflow-template.md`，加入 VMP 轻到重路线和 heavy trace 升级条件。
3. 已完成：扩写 `toolchain-router.md` 与 `tooling/recipes/ida-x64dbg-mcp.md`，将 `ida-agent-bridge` 作为候选/辅助工具记录。
4. 已完成草案：细化 `.rekit/facts/*.jsonl` 事件类型，见 `docs/evidence-ledger.md`。
5. 已完成草案：为未来 pack 写 `docs/pack-authoring.md`，降低新增安全领域 pack 的成本。
6. 已完成草案：写 `docs/orchestration-plan.md`，定义半自动 Agent Team runtime 的实施边界。
7. `packs/_template/` pack 骨架与 `B3.Lane.ps1` PowerShell 解析问题已完成；后续按 `docs/batch-plan.md` 与 `docs/agent-team-rollout-plan.md` §4-§5 推进。
8. Agent Team rollout 按 `docs/agent-team-rollout-plan.md` 推进：先 R0-R2 契约 dry-run 压测，再在 R3 决策门决定 ledger runtime（Phase 5）与 bounded dispatch（Phase 6）顺序。
9. 定位纠偏完成后，后续新增 pack 规划应以网络安全多领域框架为边界：`vmp-re` 继续作为首个成熟 pack，但不要把 RE-only 路线写成项目最终目标。
