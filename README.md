# STeamAI

STeamAI 是面向网络安全研究与安全工程任务的 Claude Code **Agent Team Mission Control**。它把一个真实项目目录组织成自包含工作空间：用户主要指挥主 Agent / Mission Commander；durable member lanes、可替换 Claude Code session executors、短命 tactical subagents、领域 pack、证据账本和授权门禁在项目内协作。

Canonical GitHub repository：[`shuiyu486/STeamAI`](https://github.com/shuiyu486/STeamAI)。从源码维护或试用时使用 `git clone https://github.com/shuiyu486/STeamAI.git`；`github.com/shuiyu486/re-context-kits` 暂时只作为 Go module compatibility identity 保留，不是当前 repository clone 地址。

用户前提很简单：**本机已经能正常使用 Claude Code**。STeamAI 不安装 Claude Code、不管理登录、不要求全局 plugin，也不另造桌面启动器。

下面三步只适用于**已经完成一次 STeamAI 接入**的项目。未接入的普通目录在 init 前没有项目级 `/steamai`，Claude Code 不会凭空识别它；首次接入必须由可信的外部 STeamAI initializer / maintenance executable 先做只读分类并生成 hash-bound init preview，用户确认 exact writes 后才 Apply。Apply 会把 `/steamai` skill、`.steamai` 状态根、verified runtime 和 selected pack 发布进项目；此后日常使用不再依赖 initializer、机器 PATH、全局 plugin 或原中央 kit。

目前仓库尚未提供面向普通用户的独立安装包；从源码试用时，一次性 initializer 由维护者在 canonical clone 中构建 unified `cmd/rekit`，再用该 executable 对目标目录运行 init/directory-adoption 的只读 preview 与其返回的 exact Apply。不能把 `cd → claude → /steamai` 描述成一个从未接入目录的首次启动方式，也不能让新项目回退到机器 PATH、外部 kit 或中央源码 runtime。

```text
cd <已接入的 project>
claude
/steamai
```

项目内 verified executable 也提供不带内部 mode 的小型用户入口；普通用户不需要记 `runtime -Command`：

```text
.steamai/runtime/bin/steamai.exe help
.steamai/runtime/bin/steamai.exe status
.steamai/runtime/bin/steamai.exe continue
.steamai/runtime/bin/steamai.exe continue --lane <selector>
```

`status` 默认只输出“现在、原因、下一步”；`status --diagnostics`（或 `--format=json`）才输出完整 typed JSON。`continue` 永远只生成 fresh preview，不自动 Apply、不启动 Claude、不执行 heavy tool；确认后由主 Agent原样消费 preview 返回的 exact Apply action。该入口只适用于已经完成一次接入的项目，不能代替外部 initializer。

可信的外部 initializer／maintenance status 对显式的未接入目录只读返回可选 pack 及成熟度，不写目录，也不把选择写成 durable identity。若 onboarding intent 已持久化但 final publication 尚未提交，fresh status 不重新开放 pack 选择，只在 `missionControlRunbook.currentDriverRequest` 发布绑定原 identity、publication stamp 和 plan SHA 的唯一 exact Apply recovery；不得从 diagnostics 手工重建。onboarding 已提交但 Mission Commander board 尚缺失时仍走只读 `overview` bootstrap，不复用 onboarding Apply。

也可以直接说：

```text
开始这个项目，目标是……
继续推进。
现在到哪了？
按这条意见纠偏……
暂停 verifier lane。
恢复 verifier lane。
停止 verifier lane。
```

新项目使用项目级 `.claude/skills/steamai/SKILL.md`、唯一 current 状态根 `.steamai/`、项目内 verified runtime 和 selected pack。一个项目目录就是一个隔离的 STeamAI 项目；复制或移动后不能依赖旧绝对路径、机器 PATH 或原中央 kit。旧 `/rekit`、`.rekit` 和中央 kit/thin-shim 模型只在迁移期间兼容，不是新项目默认。

> 当前已批准路线是 `steamai-architecture-product-convergence-v1`，按 `APC-01 → APC-02 → APC-03 → APC-04` 顺序实施；不并行跳批，也不创建新的 numbered batch 冒充路线进度。完成态必须由当前路线文档、Git-local typed receipt、direct commit 与本地 tracking ref共同证明；局部 live acceptance、synthetic fixture、manifest maturity、测试或 inventory 不单独代表总体完成，本地 readiness 也不冒充 remote CI green。

STeamAI 不是全自动脱壳器、自动逆向引擎、自动漏洞挖掘器、自动恶意样本分析平台或通用自动渗透平台；它优先提供可审计、可交接、review-first 的 Agent Team 底座。heavy action 只在 strict durable profile 与 fresh `authorized-gate` 覆盖的 exact scope/budget/stop/output 内执行并留证；`bounded-autonomous-v1` 只是显式、短时、有界的免逐次询问，不是无限权限。

Claude Code Remote Control 仅作为显式 opt-in 的 read-only Reviewer transport companion；本机 Claude Code 仍是 Windows 日常默认 provider。opaque endpoint、delivery observation、request SHA 或自然语言都不能授予 heavy action、authority 或 confirmed；uncertain delivery 不自动重发。

**当前支持口径**：Windows 本机自包含产品路径（含 copied-directory + 中央 kit 不可用）已完成。远程 CI、Linux/macOS runtime E2E、三平台产品兼容、独立安装器和 GUI/TUI 不在本轮完成范围；cross-compile不能当作平台运行证据。依赖handle-bound exact mutation的current-sync Apply与current durable detached handoff当前只在Windows启用，非Windows会在任何持久化副作用前fail-closed；read-only/preview和legacy compatibility保持可用。

## 项目路线（按需文档索引）

以下是文档索引，不是默认必读清单。新会话、上下文压缩后接手或维护文档时，先在 `main` 分支确认 `main` 与 `origin/main` 同步且工作树干净，再读 `docs/context-routing.md`，并按场景只读对应顶部区。

- 新会话与维护文档的按需路由入口：`docs/context-routing.md`
- 新架构使用与旧 case 兼容：`docs/agent-team-usage.md`
- 参考资料吸收映射：`docs/reference-absorption.md`
- 长期愿景与阶段实施方案：`docs/vision.md`
- Mission Control 最终产品方向：`docs/mission-control-product-direction.md`
- 当前架构说明：`docs/design.md`
- 历史真实可用性与产品化程度评估快照：`docs/current-usability-assessment-2026-08-11.md`（仅用于追溯当时 Windows-only 评分与候选；其中“第二个成熟 pack 待做”已由 mature `web-security` vertical slice 超越，不是当前路线或默认 read-first；当前事实看 active route、release readiness 与 fresh machine inventory）
- 已实现的日常产品四闭环详细设计：`docs/daily-product-closure-plan.md`（`DPC-01`～`DPC-04` 均已完成实现与各自验收；其中binary-re live gate使用synthetic existing-index input，不能作为IDA export producer、真实目标工具或VMProtect trace/devirtualization实证；当前只按需读取历史完成证据与共同边界）
- 后续真实使用有序路线与压缩后接手协议：`docs/real-usage-hardening-roadmap.md`
- 当前批次和路线指针投影：`docs/batch-plan.md`
- 启动已批准路线的短 goal 与新会话接手指南：`docs/autonomous-goal.md`
- pack 编写指南：`docs/pack-authoring.md`（新 pack 可从 `packs/_template/` 复制；`packs/binary-re/` 是唯一成熟二进制逆向 pack，`packs/web-security/` 是 manifest-mature 的 Web/API production vertical slice；两者的统一 mature-pack admission 与 prompt/policy consumption receipt 仍由 release contract 独立验证；`packs/malware-analysis/`、`packs/vuln-research/`、`packs/ctf/`、`packs/unpack-pe/`、`packs/ollvm/` 与 `packs/android-native/` 仍是安全领域 pack 骨架）
- evidence / intervention 账本草案：`docs/evidence-ledger.md`
- 半自动 orchestration 计划：`docs/orchestration-plan.md`
- Agent Team rollout 计划：`docs/agent-team-rollout-plan.md`
- 二进制 RE Agent Team 工作方式：`packs/binary-re/references/binary-re/agent-driven-re.md`
- sync/promote 机制：`docs/promote-sync.md`
- case 迁移说明：`docs/case-migration.md`
- Go backend 渐进迁移：`docs/go-runtime-migration.md`
- Go-first 收束与 release readiness 阶段计划：`docs/go-first-convergence-plan.md`
- 发布门禁与当前 release readiness checklist：`docs/release-readiness.md`（机器可读 inventory 与 release handoff：`go run ./cmd/rekit -- -Command release-check -Format json`；三平台 Go-native workflow 定义：`.github/workflows/release-gate.yml`；inventory ready 不等于远程 jobs 已实际运行并通过；`commitRefs[]` 只记录 implementation commit refs，远程 jobs `steps=[]` / `steps 为空` 会通过 `remoteReleaseGateDetail` 记录 run/job/boundary，并按 runner/billing blocker 而不是 remote CI green 处理）
- PowerShell-free / Go-native convergence roadmap：`docs/powershell-deprecation.md`

## 如果你在维护本仓库

本仓库本身不是具体安全 case，也不是具体 RE case。维护时先看根目录 `CLAUDE.md` 与 `docs/context-routing.md`，再按需路由到对应顶部章节；不要默认串读或扩写全部 durable docs。

- canonical `/steamai` skill：`.claude/skills/steamai/SKILL.md`
- legacy `/rekit` compatibility skill：`.claude/skills/rekit/SKILL.md`
- deterministic runtime：`rekit/rekit.ps1` façade、`cmd/rekit/**`、`internal/rekit/**`；真实 Claude Code session host：`cmd/rekit-host/**`、`internal/rekit/sessionhost/**`；read-only adapter host 与显式验收门：`cmd/rekit-adapter-host/**`、`cmd/rekit-adapter-acceptance/**`、`internal/rekit/adapterhost/**`。显式 adapter acceptance 的 `-runtime` 必须指向单独构建的 unified `./cmd/rekit` image，不能复用 `rekit-adapter-host`、`rekit-host` 或 acceptance executable；该 image 只作为 disposable project-local runtime 的验证来源。legacy `rekit/lib/*.ps1` 已删除，历史语义以 Go runtime 为准。
- 领域 pack：`packs/<pack>/**`
- 通用 policy / prompt：`common/**`
- 设计与路线：`docs/**`

不要因为下面的 case 初始化示例而在本仓库内伪造 case state；只有验证 `init/attach/sync/promote` 行为时才创建临时 case。

## 使用方式

### 1. 新 STeamAI 项目

**首次接入（一次）**：未接入目录里还没有 `/steamai`。可信的外部 initializer 先只读分类目标目录并返回 exact init preview；用户确认具体新增文件后才 Apply。initializer 不覆盖普通项目文件，不把中央 kit 路径写成日常依赖；collision、partial state、wrong binding、dual root、symlink/junction/reparse 或 plan/source/target drift 都会停止。

**接入完成后（日常）**：直接在真实项目目录启动 Claude Code：

```text
cd <project>
claude
```

使用 `/steamai`，或直接用自然语言告诉主 Agent：

```text
开始这个 case，目标是还原核心逻辑；使用 binary-re pack，由当前 Mission Commander 会话接手主线。
```

主 Agent 使用项目内 deterministic daily front door；用户不填写 pack、lane、executor、session/event ID、generation、时间、路径或 SHA，也不需要记底层 executable 命令。`/steamai` 通过 `${CLAUDE_PROJECT_DIR}` 定位并验证项目内 runtime，再提交原始 goal/correction。需要直接查询时，项目内 executable 的 no-mode `help` / `status` / `continue` 是普通用户 surface；`runtime -Command ...` 与 `host -daily` 只作为 typed bridge 和维护 API。

ordinary public `continue` 只消费 fresh status 的 typed `-WhatIf -Format json`，再原样执行 preview 返回的 exact Apply。preview 顶层 `continuePlanSha256` 绑定完整 mutation snapshot，returned Apply 携带同值的 `-ExpectedContinuePlanSha256`；current action blocked 时不发布 Apply。Apply 后必须刷新 status，不能手工拼 phase、参数或复用旧 request。

fresh target 的 external initializer 只展示 schema-valid、非 template 的 pack choices；普通用户只能选择 mature pack（当前为 `binary-re` 与 `web-security`），骨架 pack可以在 inventory 中可见但不能作为普通接入选择。已通过 `attach/init` 绑定且 doctor-ready、尚无 Mission Control 状态的 existing case 会从 metadata 选择 pack，并只追加 immutable onboarding intent/mission/commit，不覆盖普通 case 文件。旧 `vmp-re` / `generic-binary-re` metadata 只返回 typed `pack-migration-required`，不作为 alias、不自动迁移或改写。相同 goal 在当前真实 member 已 intake-ready 后安全 replay，不重复启动 Claude；当前 mission 尚未完成时的冲突 goal 会明确拒绝。

已有 case 同时有多个可继续 lane 时，daily 先返回 typed choices，选择前不启动 Claude、也不写 case。主 Agent 使用 choice 的 canonical ID 重新调用 `-lane <lane-id>`；该 selector 只在本次调用生效，并贯穿 status、current-step/current-loop、纠偏和完成，所选 lane 不可继续时会停止，不会回退到其它 lane。lane 的人话 label 只用于展示，不能代替 canonical ID。

外部 initializer 对尚未接入的普通非空目录先做只读分类并返回 `directory-adoption-required`；选择 `initialize-in-place` 前零写入。接入使用 canonical init preview 的 exact plan SHA，再执行 hash-bound Apply；只允许新增或保留现有文件，managed collision、partial state、wrong binding、dual root、symlink/junction/reparse 和 plan/source/target drift 均 fail-closed。Apply 发布 `/steamai`、`.steamai` 和 verified project-local runtime bundle；从此才进入上面的项目内日常路径。bundle/copy、installed-skill typed-command bridge 与完整 Windows local minimum 均已通过，路线已按 Windows 本机口径完成。

`binary-re` 的 established-case status 会显式投影 `enabledSpecialties`；该集合只取当前 pack 的 executable owner、production contract 与 typed verified project-local catalog `supported` 集合完全一致的 exact adapter IDs。当前为 `static-binary-triage-sidecar` 与 `vmp-ida-index-inspector`；它不表示已授权、已执行或已有真实 target/tool receipt。后者只查询用户已经导出的 `function_index.tsv`（必需）以及可选 `strings.tsv` / `imports.tsv` / `xrefs.tsv`。主 Agent 会先预览内容寻址 request 和最长 15 分钟的 exact `inspect` profile；只有用户确认 profile 且 canonical `authorized-gate` current 时，独立 `rekit-adapter-host` 才运行 compiled-in inspector，随后写入 bounded packet/report/receipt/observation、恢复默认 manual profile，并交给独立 evidence review、member 和 Reviewer。该路径不生成 export、不安装或启动 IDA、不打开 IDB、不联网，也从不执行 tooling catalog 的 `entry`；VMProtect trace/devirtualization 仍是 recipe/template，不是已启用 producer。当前 `NoNetwork` 只表示固定 Go child 没有网络代码路径，不是 OS 级 socket 隔离。

人工纠偏也只提交文本；多个可纠偏 lane（包括已完成 lane）会先返回 typed choices，选择前零写入、零 Claude launch。用户只需告诉主 Agent“按这条意见纠偏：优先核对控制流证据，区分 observation 与 hypothesis”；主 Agent 选择同一个 canonical lane ID，并只调用 manifest 绑定的项目内 `steamai.exe host`。中央源码目录下的 direct host 仅作为 maintenance/internal API，不是新项目默认入口。

Reviewer rejection 仍由既有 correction/reconcile owner 记录 intervention、启动 replacement member 与独立 Reviewer，并在 evidence-bound 条件满足后完成 lane。若所选 lane 已 committed completion/closed，front door 会把当前 completion receipt 作为证据，消费 public zero-write `reopen` preview 与 owner 返回的 exact Apply；提交后只返回 `ready-to-continue`，不会自动接管 executor、恢复旧 session/current-loop budget或启动 Claude。中断恢复只接受同 actor、纠偏文本、lane 与 exact plan；成功响应丢失后的相同请求返回同 operation 的 mutation-free replay，并复核 compound targets 仍是 current reopen。

若整个当前 mission 已 `mission-complete`，用户提出不同新目标时不再报“冲突 goal”或复用 reopen。daily owner先返回绑定 predecessor mission intent、完整 closure、generation、publication stamp 与 write set 的零写入 successor preview；用户确认后主 Agent原样消费其 exact Apply。Apply commit-last、active-pointer-last，只激活新的 `.steamai/missions/gNNNNNN/` namespace，保留 predecessor audit tree，不写 authority/confirmed、不执行 heavy tool，也不自动启动 Claude；fresh status随后只发布该新 mission唯一的 initial `start` preview。普通用户不填写或记忆 SHA；相同请求只允许 committed replay，stale closure、legacy `.rekit`、dual root、partial/corrupt transition 或 pointer drift均 fail-closed。

trusted daily 路线还使用 host-owned durable supervisor：front host 在 Claude 启动、output 返回、result-first、submission 或 intake 后中断时，fresh host 会收取同一 attempt/session 的 exact result、从已提交边界继续或在 ownership 证据丢失时先 durable fence 再 replacement，不用 PID 单独声称 liveness，也不会重复启动成功 session。Claude 登录、配额、模型或进程不可用时会真实返回 blocked/failed，不会退化为伪造 member output 或 `ReviewerResult`。失败 JSON 的顶层 `failure` 返回 stable `code` / `stage`、`terminal|replaceable|recoverable`、真实 `mutationApplied` / `mutationBoundary`、attempt 计数和唯一 `nextAction`；达到上限后不会自动循环。完整故障矩阵与恢复语义按需见 `docs/agent-team-usage.md`。

内部 Go commands、legacy compatibility API 和 direct host 仍供维护、迁移与排障使用；项目内 ordinary host 必须消费 fresh status 发布的 typed current driver request 及其 exact SHA-256，多条可继续 lane 并存时还必须先消费 typed choice，request SHA 不能替代 lane 选择。日常不需要手工拼接底层步骤，也不应直接调用中央 kit source runtime。

### 2. 日常继续项目

进入真实项目目录启动 Claude Code：

```text
cd <project>
claude
```

日常优先直接用自然语言指挥主 Agent，例如：

```text
开始这个 case，目标是还原核心逻辑。
继续推进当前 mission。
总体怎么样？哪些 lane 卡住了？
让我进入 verifier lane 帮它纠错。
这个 lane 上下文污染了，生成接手包，让新会话接手。
把这次可复用经验整理成 promote 候选。
```

主 Agent 会把这些意图交给 `/steamai` 背后的 typed daily owner 和项目内 runtime；状态、继续、开工作线、接手、授权、记录、同步和回流都是确定性内部动作，不是用户需要记忆的命令。daily 只由一个 operation owner 在 resume/goal/correction/control/adoption 中选路，lane 只是 selector。runtime 只执行 fresh typed request：`commandExecutable=true`、未 blocked、typed invocation 与 expected receipt 一致，并且用户当前意图覆盖 exact request；guidance、model-tool handoff 和 command template 不会被当成 shell 命令执行。普通 continue 先执行 status 给出的 typed `-WhatIf -Format json`，确认后只消费 preview 结果返回的同 selector/owner/generation exact `-Apply`，Apply 后重新读取 fresh status；不能从 command 文本手工拼 phase，也不能复用刚执行的 Apply request。

暂停、恢复或停止同样只需自然语言。主 Agent先从fresh status确定exact lane并展示review-first preview；多lane先让用户选择。`pause`只提交durable paused状态，不做OS suspend；`resume`只允许新generation的未来结果继续，旧held结果不自动释放；`stop`先提交durable stopped receipt，再由exact local supervisor owner尝试关闭自己持有的containment。actuation失败不回滚stopped，process termination不是durable成功判据，opaque Remote Control session不受本地actuation管理；control也不授予authority/confirmed、gate或heavy action。

需要外部 member 或 Reviewer 时，Mission Commander 返回 bounded typed handoff，保存 durable checkpoint、attempt、owner 与 output hash lineage。accepted 只证明外部会话已被接收，不证明任务完成；迟到或旧代结果不能推进 current state，uncertain delivery 不自动重发。中断后由 fresh status 给出唯一恢复动作，不靠 PID、文件存在或 prose 猜测状态。详细 dispatcher、claim、launch、submission 与 result-first/submission-last 合同按需见 `docs/agent-team-usage.md`。

排障或新会话接手仍先使用 `/steamai`。默认 compact status 无法完整、安全地表达 typed request/choices 时，会返回 `details-required` 和 typed full-diagnostics route；普通用户不需要记内部命令。

## 目录模型

新项目的核心布局：

```text
<project>\
  .claude\skills\steamai\SKILL.md
  .steamai\instance.yml
  .steamai\state.json
  .steamai\runtime\manifest.json
  .steamai\runtime\bin\...
  .steamai\packs\<selected-pack>\...
  .steamai\common\...
```

项目资料、lane、facts、evidence、reviews 与 handoffs 都属于同一真实项目；项目内 bundle 是运行依赖边界。多个项目各自隔离，不共享 mutable state，也不要求放进固定 `kits/` / `cases/` sibling 布局。

### Legacy 中央 kit/thin-shim 模型

旧项目可能仍含 `.claude/skills/rekit/SKILL.md`、`.rekit/instance.yml`、`.rekit/state.json`，并通过 metadata 指回中央 kit。它们在迁移完成前继续兼容，但不再是新项目 quickstart。迁移必须显式 review-first；不得通过复制 runtime 逻辑、创建第二状态根或只改品牌展示来伪装完成。

## 内部与 legacy Runtime/API 参考

以下 `/rekit` 形式是主 Agent、维护者、迁移兼容和排障使用的确定性 API，不是新 STeamAI 项目的主要 UX；新项目默认从 `/steamai` 调用项目内 verified runtime。普通日常使用优先通过自然语言让主 Agent 选择和组合这些动作。Executable Mission Commander action/request同时携带bounded typed ReKit invocation；`command`只是其兼容投影，不接受任意shell/executable，也不能扩大target/pack/lane或授权范围。多lane未选择时不发布可执行request；选定后status/handoff/daily/current-step/current-loop与replacement takeover只使用唯一exact durable `-Lane`，普通人工命令仍兼容positional label。`currentDriverRequest.expectedReceipt`保留同源command与`refreshStatusCommand`；接手者执行driver后应直接运行该refresh command重建durable state，不从相邻文本手工拼接。Request SHA只校验exact消费，不代表authority/confirmed、authorized gate或heavy-action授权。

Adapter execution report lifecycle 的 contract、dispatch、scaffold、draft、validation、receipt、record 与 status/handoff 投影会输出 `runbookSteps[]` 或对应 text runbook 行；replacement executor 应优先按这些步骤确认 state/path/hash。durable lane 且当前 action 有 tooling catalog candidate 时，先按 `liveValidation.dispatchCommand` 记录 immutable dispatch，再等外部 adapter/harness 写出 bounded report；`_template` 的 `rekit-readonly-inspector` 可由独立 Go-owned `rekit-adapter-host` 消费 exact dispatch，在 strict preauthorization 下只读一个 bounded case-local text fixture并独占写入 report/artifact；显式 Windows acceptance 会在任何 adapter bytes 执行前验证 suspended actual image、用 kill-on-close Job Object约束进程树，并以 exact-object output cleanup 与 no-replace disposable-case quarantine 收尾；canonical `/rekit gate` 本身仍不启动进程。contract、validation、`status`、`overview`、project/lane `handoff` 与 durable Markdown 会共享 `currentRunLoopStepId` / `runLoop`，按 `inspect-contract → record-dispatch → run-external-adapter → draft-or-write-report → validate-report → record-receipt → record-observation → review-recorded-evidence` 显示当前步骤、命令、owner/provenance 与 boundary。随后用 validation 返回的 receipt preview 记录 current executor generation、external harness/session、catalog/report/artifact hashes 与 outcome/exit status，再使用 validation/status 返回的 report+receipt 双 hash-bound record Apply；record 后只进入 evidence review。已记录 execution evidence 且无需 main escalation 时，`status`/`handoff`/`continue` 的 Mission Commander current action 与 `currentDriverRequest.command` 会直接指向 `acknowledgementReviewCommand`（accepted `note -Kind verification ... -WhatIf -Format json`），review 后再执行该 WhatIf 返回的 hash-bound `recordCommand` 关闭 review queue；`/rekit handoff <lane>` 仍保留为 follow-up/provenance，而不是当前 primary。installed case-local `/rekit` 可从 nested lane/workspace cwd 完成同一路径；takeover 后旧 executor/generation/session receipt 会 fail-closed，acknowledgement 后的 handoff、`RESUME.md` 与 continue digest 仍保留 receipt/owner/harness/catalog/artifact lineage。Go runtime 不执行 adapter/heavy tool，也不从 contract/report/receipt 推断 authority/confirmed。

| 命令 | 方向 | 什么时候用 |
|---|---|---|
| `/rekit status` | 只读 | 看当前 case 绑定状态、case-local thin shim / canonical skill readiness，并在 case shim 后先显示 compact Mission Commander first-screen strip；`status`与case-local project `handoff`使用同一lightweight project handoff owner读取latest batch、known gaps、pack-memory与validation routes，不执行PowerShell/public façade全仓release audit，完整release inventory仍由`release-check`持有；kit-mode project handoff 会同步投影 latest-batch release-run transient retry evidence / validation warning；若目录被移动或 shim drift，只提示，不修复；case 模式会投影 `pendingGateHandoffs[]` 的 review / WhatIf / request-decision boundary、`authorizedGateHandoffs[]` 的 execution report contract boundary 及 compact adapter handoff（`defaultReportPath` / `reportPath`、`reportSummary`、`liveValidation` validate/record commands、authorized workspaces、adapter candidate / selected adapter detail（entry/purpose/sideEffects/reportGuidance/evidenceGuidance/stopConditionHints）/ sidecar guidance、contract error）、`openDecisionHandoffs[]` 的 source fact/list command 与 decision note WhatIf/hash-bound recordCommand boundary（terminal `note -Kind decision -Related <candidateEventId>` 会关闭对应 open candidate blocker）、`interventionHandoffs[]` 的 reconcile WhatIf/Apply boundary，以及已记录 authorized execution observation evidence 的 compact `executionEvidenceReviewSummary`（ready/main escalation/duplicate/ref/boundary/action queue summary、latest adapter context）和完整 `executionEvidenceReview[]`（含 `acknowledgementReviewCommand`、accepted/rejected acknowledgement previews、adapterContext entry/purpose/sideEffects/reportGuidance/evidenceGuidance/stopConditionHints；终结性的 related verification / decision notes 会关闭 review queue，并阻止 exact `evidence-already-recorded` current action 回流）；非 escalation 的 evidence review current driver request 会直接指向 acknowledgement note preview，不再把 `/rekit handoff <lane>` 当作当前 primary；`-Format json` 输出机器可读 status envelope 与 `caseShim` readiness。 |
| `/rekit packs` | 只读 | 维护者查看当前 kit 内所有 pack 的成熟度、schema、route、managed/tooling 和 authority lane 概览；`-Format json` 输出机器可读 inventory。 |
| `/rekit release-check` | 只读 | 维护者查看 release inventory、gateProfile、latest-batch handoff 与 CI truthfulness boundary；只枚举门禁，不执行测试。latest-batch handoff 会识别明确记录的 `release-run` 7/7 成功结果，并在 completed release cadence 后把 current action 交棒给 next-batch selection；目标、计划、待执行、失败或历史叙述不会被当作成功证据。已记录的 transient retry 仍作为 evidence / validation warning 暴露，但不替代完整本机 release minimum，也不代表远程 CI green。 |
| `/rekit release-run` | Git-local 本机验证 | 顺序执行 `release-check` 的 local release minimum（完整 Go tests 使用有限 `go test -count=1 -p=2 -timeout=30m ./...`；`-count=1`禁用 package test-result cache，确保本次 frozen bytes 被 fresh 执行但不禁用 Go build cache；`-p=2`限制跨 package并发；30分钟是逐 package test binary上限，整条多 package command另有45分钟硬上限，其它步骤及receipt/inspection Git命令为5分钟），汇总 exit code / duration / attempts / output tail。所有外部命令都在有界进程树中运行：Windows suspended start后加入non-breakaway kill-on-close Job Object再resume，Unix使用独立process group；根进程退出、context deadline或64 MiB合并输出上限都会终止 containment 内的剩余子孙并回收根进程；随后只做5秒有限pipe drain，逃逸writer未关闭时返回明确错误，不会永久等待stdout/stderr EOF。成功且存在待 direct commit 的 machine validation subject 时，会在 Git metadata 写 typed v3 receipt，显式区分 numbered-batch 与已 completed 的 exact active-route，并绑定 baseline HEAD、完整 gate profile、当前 exact implementation artifacts 的工作树 SHA-256/bytes，以及经 Git clean filter 得到的 blob OID；该 receipt 不进入 commit。旧 v2 receipt 只从独立 v2 路径只读兼容 numbered-batch，不能关闭 active route；v3 路径存在但无效时不回退 v2。提交后仅当 HEAD 是 baseline 后唯一 direct commit，且 changed set、mode 与 tree blob OID完全匹配，status 才恢复post-push cadence；numbered-batch 可在 receipt runtime 缺陷暴露后由新 typed receipt精确绑定一次同批repair，active-route 必须匹配 exact route/current/state/claim/next，不得伪装为 numbered repair。缺失、版本/路径/subject错配、篡改、验证后编辑或未绑定extra commit均fail-closed；不写tracked repo/case state、不查询远程CI、不执行heavy tool、不写authority/confirmed。 |
| `/rekit next-batch` | kit review-first planning receipt | 接受 `status` / `release-check` 的 next-batch guidance 时使用；`-WhatIf -Format json` 预览 `docs/batch-history.md`、`CHANGELOG.md` 与 `docs/batch-plan.md` 三份 exact writes，先归档更早 active batch，再返回 `expectedNextBatchPlanSha256`；`-Apply` 必须带同一 hash，并可从已完成的 exact write prefix 幂等恢复，随后要求刷新 `/rekit status -Format json`。不触碰 case state，不执行 reviewer/adapter/pack-memory/gate/sync/promote mutation，不 commit/push，也不声明 remote CI green。 |
| `/rekit onboard` | new-case review-first mission intent | 新 case 的 public Go-owned/no-fallback Mission Control 入口。主 Agent先把自然语言显式映射为 `Target` / `Pack` / `ProjectName` / opaque bounded `Goal` / `Actor` / `Executor` / `InitialLane`；`-WhatIf -Format json` 零写入返回 immutable mission intent、exact `publicationStamp` / `onboardingPlanSha256` 与机器可读 `applyArgs[]`。`-Apply` 必须消费同一 stamp/hash 和 identity，按 intent-first / commit-last 发布；partial publication 用 exact Apply 恢复，committed exact replay 幂等。提交后先 `status`，再 `overview`，最后按 committed lane/executor/actor `start`；后续继续使用 public `note` / `reconcile` / `handoff` / `complete` / `reopen`。Runtime 不解析自然语言、不创建 board/lane、不执行 heavy-tool、不写 authority/confirmed，也不 spawn/poll session。 |
| `/rekit migrate-state` | legacy state root review-first migration | legacy-only attached 项目迁移到 current `.steamai` 的唯一 owner。对 retired `vmp-re` / `generic-binary-re` 必须显式携带 exact source `-Pack`；`-WhatIf -Format json` 零写入并返回 canonical `binary-re` target、完整 root/onboarding projection、plan SHA 与 `applyArgs[]`，确认后只原样消费该 Apply。迁移 single-write 保留 durable/local bytes，receipt 前保持 fence；不把 retired identity 当 alias，不写 authority/confirmed，不执行 heavy tool，不触发 sync/promote。 |
| `/rekit run-current-step` | case-local unified review-first runner | 主 Agent/harness推进当前 focused Mission Commander request 的首选单步入口。每次 `-WhatIf -Format json` 都从 refreshed `missionControlRunbook.scope/currentDriverRequest` 自动选择 case 或 reviewer route，返回对应 nested runner plan；有 deterministic nested step 时同时返回绑定 route、current request 与 nested hash 的 `expectedCurrentStepPlanSha256`，复核后用 `-Apply -ExpectedCurrentStepPlanSha256 <hash>` 执行一步并读取 refreshed receipt。reviewer spawn/result wait 等外部动作只返回 typed handoff，不生成可 Apply hash；lane/reviewer各自原有lease、packet、artifact、candidate与intake锁保持不变。runtime不调用Agent tool、不管理session、不执行heavy-tool、不写authority/confirmed；必须显式`-Target`且只支持JSON。 |
| `/rekit run-current-loop` | case-local bounded review-first loop | 主 Agent/harness在同一初始 route/lane 上连续推进最多 `-MaxSteps 1..20` 个 deterministic current steps。先运行 `-WhatIf -Format json`，再以返回的 `expectedCurrentLoopPlanSha256` 和相同 `MaxSteps` 显式 Apply；每步都刷新 durable status、重建 exact nested plan并留下 receipt。route/lane漂移、fresh Human-in-the-Lane reconcile和external reviewer stop在预算尚有剩余时都会返回统一 `kind=current-loop-campaign-continuation`：`segmentRoute/segmentLane → expectedRoute/expectedLane`、按本段已执行步数扣减后的 `remainingMaxSteps` 与fresh WhatIf command；external reviewer另带单一typed `attempt`：稳定绑定packet/route/shard/prompt/owner/current executor与dispatch receipt，提供`attemptSnapshotSha256`、唯一`selectedAction`及checkpoint-bound `durableContinuationDriverRequest`。外部harness接受session后生成绑定immutable dispatch ID的新attempt；session accepted、managed result与failed observation必须携带fresh snapshot guard，stale值在preview前zero-write拒绝。direct result按typed target外部写入后刷新到successor attempt再intake，不复用predecessor snapshot；replacement executor只消费fresh status/handoff当前attempt。成功Apply还会把单段provenance按严格递增sequence写为immutable `.rekit/runs/current-loop-segments/<sequence>.json` checkpoint，并用前一个exact artifact SHA组成无gap/fork链；新会话`status.missionControlRunbook.currentLoopSegment`与`handoff.currentLoopSegment`仅在完整chain、canonical exact bytes、case/pack、重算的outer plan/request-receipt lineage及refreshed current request完全匹配时暴露typed continuation、remaining budget和可执行`resumeDriverRequest`，wall-clock回退不改变latest，tamper、chain gap/break、symlink、unknown entry、stale request、terminal或status refresh failure均fail-closed且不回退旧artifact。主Agent/harness可直接执行该request中的`-ResumeCurrentLoop -ExpectedCurrentLoopCheckpointSha256 <artifact>` fresh WhatIf；external member/reviewer alternative同时返回checkpoint/attempt-bound `observationEnvelopeTemplate`与统一`observationPathCommand`，harness将一次accepted/returned/failed observation写入case-local symlink-free bounded strict JSON后只传`-CurrentLoopObservationPath`。Preview返回exact file SHA，Apply命令只携带path、expected observation SHA、checkpoint SHA及outer/nested hashes；bytes/checkpoint/attempt/state/capability漂移或与legacy flags混用均fail-closed。Runtime从strict checkpoint派生remaining budget并复核expected route/lane/current request。Apply再次要求同一source artifact仍是latest ready checkpoint，并在任何nested mutation前durably one-shot claim该source；claim后source立即为`consumed`，并发、崩溃、nested/publication failure均不恢复其预算。成功后的新segment checkpoint把`resumeSourceSha256`绑定到immediate predecessor；旧source/plan hash重试zero-write失败。JSON中的旧unbound WhatIf仅以`legacyUnboundWhatIfCommand`保留诊断兼容，唯一可执行/推荐恢复入口是`resumeDriverRequest.command`。旧segment plan hash与receipts不能跨界复用或累计；没有ready durable checkpoint时不能声称恢复旧预算。fresh status还会从latest ready checkpoint派生统一typed `externalSessionJob`，固定job/checkpoint/attempt/owner或reviewer packet-route-shard身份、submission-last路径与允许outcome，并提供状态化`harnessPackage`。无attempt时只暴露review-first attempt request；running时`launch`给出exact member handoff或reviewer prompt path/SHA、tool/agent type、read-only边界和current generation owner，输入缺失或SHA漂移即`ready=false`；`return.templates[]`按outcome给出exact attempt-bound strict submission JSON与required result-first writes。Replacement刷新整包并撤销旧generation；submission-ready时`return.reviewRequest`指向一次reviewed external-result turn，`relayRecoveryRequest`仅用于partial恢复。外部harness先写member outputs或ReviewerResult，再最后写strict `submission.json`；status随后把`selectedDriverRequest`切换为一次reviewed external-result turn：WhatIf零写入绑定exact job/submission/source/destination/relay artifacts、observation、checkpoint与nested resume hashes；Apply以exclusive no-overwrite、exact-prefix recovery顺序生成member manifest+outputs或canonical reviewer relay source、publication receipt，最后发布Batch 813兼容inbox envelope，再从durable filesystem strict intake、one-shot claim并继续bounded loop。若后半段因Human intervention或其它currentness drift拒绝，已提交relay保持truthful，checkpoint不会被误消费，fresh status可路由reconcile或relay-only recovery。无submission时旧member/reviewer handoff继续可用；invalid submission或ambiguous/invalid inbox fail-closed。managed packet把`reviewerResultDropPath`标为canonical input destination，relay生成与drop/input/source分离的case-local immutable source后仍进入既有save/completion/capture/stage/collect/intake链；无managed input-save capability的direct packet则把drop path标为direct result destination。guidance/blocker、no-progress、step limit或nested error仍停止且不扩容预算；已成功步骤不回滚。runtime不自动跨lane/route Apply、不调用Agent tool、不管理session、不执行heavy-tool、不写authority/confirmed；必须显式`-Target`且只支持JSON。 |
| `/rekit run-driver-step` | case-local review-first runner | 主 Agent/harness 消费唯一 focused、case-scoped `missionControlRunbook.currentDriverRequest` 时使用。外层 `-WhatIf -Format json` 允许当前 `start`、`continue` 或 `reconcile` preview request，直接调用对应 Go preview handler并返回 typed Apply request与 `expectedDriverStepPlanSha256`，不写 case；复核后外层 `-Apply -ExpectedDriverStepPlanSha256 <hash> -Format json` 会重新构建同一 preview plan，hash drift 时 fail-closed，只调用一个 matching Go Apply handler，随后刷新 status并返回 typed runner receipt。三类写入都会在 lane mutation lease 内重验 preview currentness；必须显式 `-Target`。runner 不调用 shell、不递归调用 public runtime、不 spawn/poll/stop session、不执行 reviewer/adapter/heavy-tool、不写 authority/confirmed，也不接受 missing-board onboarding、gate、note、handoff、sync、promote、next-batch 等 request。 |
| `/rekit run-reviewer-step` | case-local reviewer review-first runner | 主 Agent/harness 消费 `caseMission.reviewerDispatchIntakeSummary.operatorPackage.currentDriverRequest` 时使用。spawn reviewer 与 ReviewerResult JSON 生成保持外部动作：缺少真实 harness/session 或结果路径时返回 typed `externalHandoff`；提供 `-ReviewerHarness/-ReviewerSession/-Actor` 或 `-ReviewerResultInputSourcePath/-Actor` 后，runner 直接调用现有 Go reviewer preview handler，返回 typed Apply request与 `expectedReviewerStepPlanSha256`。hash-bound `-Apply` 只执行当前 dispatch receipt、result-input save、completion receipt、source capture、staging、collection 或 intake 一步，随后刷新 reviewer status并返回 receipt；同一packet仍有running/open shard时优先继续该shard，不提前batch intake，collection还会在锁内复验WhatIf返回的candidate hash。replacement executor takeover 会让旧 packet owner路径 fail-closed，显式 adoption 后才可生成新 dispatch plan。runtime 不调用 Agent tool、不 spawn/poll/stop reviewer、不伪造 reviewer output、不执行 heavy-tool、不写 authority/confirmed；必须显式 `-Target` 且只支持 JSON。 |
| `/rekit overview` | case-local 状态 | 显示项目概览、主线/支线、共享事实统计、Mission Control brief、逐 lane `laneExecutorActions[]` 和 blocker-aware 下一步建议；文本/JSON 直接展示 blocked/ready、typed blocker counts、requirements、resume/handoff command 以及 current executor / generation / last takeover 摘要，只有 ready lane 才进入 continue 建议；已记录 authorized execution observation evidence 时，同时显示 compact `executionEvidenceReviewSummary` 与完整 `executionEvidenceReview[]`（含 recorded adapter context detail），让替换 executor 先确认 ready/main escalation、duplicate、refs、latest review/handoff/current action、adapter entry/guidance/stop conditions 与 no-replay/no-authority boundary；`authorized-gate` 作为 durable autonomy 已授权决策单独展示但不阻塞 lane，并在 Mission brief 中带出 `requestedBudget`、`outputPaths`、`stopConditions`、`eventId` 与可复制的 `reportContract=/rekit gate -ExecutionReportContract -GateEventId ... -Format json` handoff；overview JSON/text 还直接投影 compact adapter handoff（`defaultReportPath` / `reportPath`、`reportSummary`、`liveValidation` validate/record commands、authorized workspaces 与 adapter sidecar guidance），让替换 executor 不必切回 status 或完整 contract 才能定位 safe validation/record handoff；缺 `.rekit/board.json` 时由 Go 初始化 case-local board/facts/policy/default authority lane；只表示总览，不代表当前会话已选择工作线。 |
| `/rekit continue main` | case-local 自动整理 | 明确选择主线并整理相关状态；多工作线时不要用无参数 `continue` 盲猜。普通 executable action 先由 fresh status 投影唯一 typed `/rekit continue main -WhatIf -Format json` preview；确认后只执行该 preview 返回的同 selector/owner/generation exact `-Apply`，Apply 后重新读取 fresh status，不能手工拼 phase 或复用旧 Apply request。JSON envelope 含结构化 `missionBrief`，其中包含 pending gates 与非阻塞 authorized gates。 |
| `/rekit continue <name>` | case-local 自动整理 | 明确选择某条功能支线，只整理该支线的 workspace/outbox 并刷新接续提示；`-WhatIf -Format json` 是唯一 public continue preview owner，返回的 `currentDriverRequest.invocation` 是唯一 exact Apply 来源；Apply 后必须回到 fresh status 并重新发布 preview。JSON/status/digest 都保持同一 `missionBrief`、selector、owner/generation 与 no-replay/no-authority boundary；若该 lane 因 effective open intervention、pending-gate request 或 open candidate/decision 阻塞，blocked `continue` 继续只输出对应 reconcile/gate/note handoff 并保持 zero-write。 |
| `/rekit start <name>` | case-local 状态 | 创建或进入一个功能支线，例如 `/rekit start login`；支线只写自己的工作区；当 `<name>` 解析到已有工作线（如 `main` 或 `feature-login`）时，start 会进入该 durable lane 而不是新建并行 lane；维护自动化可用 `-WhatIf -Format json` 消费非写入 start 计划和结构化 `missionBrief`，显式 `-Apply` 输出含 apply 后 `missionBrief` 的 Go JSON envelope；start JSON/text 会投影 lane-local `pendingGateHandoffs[]` / `openDecisionHandoffs[]`（含 gate WhatIf/Apply、note WhatIf/hash-bound recordCommand、boundary 与 evidence）以及 compact authorized-gate adapter handoff（default/current report path、`reportSummary`、`liveValidation` validate/record commands、authorized workspace 与 no-heavy/no-authority boundary），让替换 executor 在 preview 或 takeover apply 第一屏即可定位 gate/decision 与 safe validation/record handoff；需要登记/接管当前会话时由主 Agent显式传 `-Executor <session>`、`-Actor <actor>`、`-Reason <reason>`，例如 `start main -Apply -Executor <new-session>` 可让替换会话接手主线并刷新 lane resume/checkpoint/events；runtime 只写 durable executor metadata，不自动 spawn 或管理 session。mission-complete 后普通 start 不会隐式重开 mission。 |
| `/rekit complete <name>` | case-local review-first completion | 审核 lane 的 case-local evidence 后先运行 `-Actor ... -Reason ... -EvidenceRefs ... -WhatIf -Format json`；preview hash 绑定 evidence exact bytes、blockers 与写集，Apply 必须使用返回的 `-ExpectedCompletePlanSha256`。未解决 intervention/gate/decision/task/reviewer/execution-review/adapter blocker 时拒绝，main 必须最后关闭；feature 完成后路由下一条 open lane，全部 lane 的 intent/receipt/board/resume/checkpoint 一致时 status 返回 typed `mission-complete`。completion 不写或推断 authority/confirmed，不执行 heavy-tool；closed/pending-completion lane 拒绝 start/continue/gate/note/reviewer mutation。 |
| `/rekit reopen <name>` | case-local review-first completion supersession | 误完成、补充证据或事后发现新工作时使用。`-WhatIf`把actor/reason、case-local evidence、被supersede的exact completion receipt、effective targets和写集绑定到`reopenPlanSha256`；Apply必须携带返回的`-ExpectedReopenPlanSha256`。terminal feature复开会在同一个compound operation显式纳入已失效的main aggregate completion，final operation commit是共同生效点；中断恢复只消费immutable per-lane intent，pending/invalid operation期间handoff与ordinary lane mutation均fail-closed。复开清空旧executor、递增generation且不恢复旧session/current-loop budget；历史completion/reopen artifacts保持append-only，不写authority/confirmed、不执行heavy-tool。 |
| `/rekit handoff` | case-local 状态 | 生成项目级接手索引 `.rekit/handovers/latest.md`；索引和 Go JSON envelope 都包含 Mission Control brief，汇总 ready/blocked lanes、pending gates、authorized gates、compact authorized-gate adapter handoff、open decisions、interventions、next agent actions 与 escalations。`-WhatIf -Format json` 零写入返回 `publicationPlanSha256`、`publicationStamp` 与 exact Apply request；Apply 必须携带同一 hash/stamp。发布完成后 `latest-generation.json` 才作为整组 RESUME/checkpoint/handoff/takeover 的最终 commit point；status 只在整组 exact bytes匹配时信任 durable takeover，`mixed-generation` 时必须刷新 handoff，不得拼用混合文件；不代表某个会话。 |
| `/rekit handoff <name>` | case-local 状态 | 生成指定工作线接手文档，例如 `/rekit handoff main` 或 `/rekit handoff login`；lane handoff 的 Markdown 与 Go JSON `missionBrief` 使用 overview 同一 blocker 语义，pending gate、open intervention、open candidate/decision 都会让该 lane 显示为 blocked；`authorized-gate` 单独展示授权 profile / decision 但不阻塞 lane，并带出 requested budget、authorized output paths、stop conditions、gate `eventId`、可复制 report contract command 与 compact adapter handoff，供替换 executor 接手 actual heavy action 前核对边界并读取 default/current report path、report summary、live validation validate/record commands 与 authorized workspace；已记录 execution observation evidence 时，lane handoff JSON/Markdown 同时输出 compact review summary 与完整 review item（含 recorded adapter context detail），避免逐条扫描 refs/follow-through/action queue 或回退 observations ledger/contract 才能确认 adapter entry/guidance/stop conditions。该命令与项目级 handoff 使用同一 hash/stamp Apply 和 lane-local generation commit；中途失败不会推进 latest generation，replacement executor 只能消费 status 标记为 fresh 的 committed takeover。 |
| `/rekit reconcile <name>` | case-local intervention | 显式处理 lane-local effective open intervention；`-WhatIf` 预览 resolution、lane event、executor takeover、resume/checkpoint/board refresh，`-Apply` 只写这些 case-local durable state，不执行 heavy-tool、不写 authority/confirmed。reconcile JSON/text 会和 start 一样投影 lane-local `pendingGateHandoffs[]` / `openDecisionHandoffs[]` 与 compact authorized-gate adapter handoff；当 intervention resolution 后仍有 gate/decision blocker 时，替换 executor 可在 reconcile 结果第一屏直接拿到下一条 gate/note handoff 与 safe validate/record handoff，而不必回查 status/handoff。 |
| `/rekit gate -WhatIf` / `/rekit gate -Apply` | case-local gate/evidence | Go backend 的 heavy-action authorization preflight；`-WhatIf -Format json` 输出 gate decision plan 和当前 `missionBrief`，不写 ledger、不执行 heavy-tool；`-Apply -Format json` 默认 append pending-gate 或 authorized-gate request ledger decision，并输出 apply 后 `missionBrief`；`missionBrief.authorizedGates` 会直接显示 requested budget、authorized output paths、stop conditions、gate event id 与可复制 report contract command；对已授权 gate 可先用 `-ExecutionReportContract -GateEventId <authorized-gate-event-id> -Format json` 读取只读 adapter execution report contract（在 case-local / authorized output workspace cwd 中可省略 `-Target`），供 lane executor / tool adapter 在执行前先看 compact `reportSummary` 判断 state、report/default path、validation/record readiness、current action、repair/main-escalation flags、allowed counts 与 no-heavy/no-authority boundary，再按需消费完整 action、budget、output paths、default report path、status、stop conditions、boundary/escalation requirements、validation failure taxonomy / `validationRepairHints[]`、`liveValidation` handoff（`authorizedWorkspaces[]`、`reportFileName`、`caseRelativeReportPath`、sidecar template、workspace-relative 与 case-relative validate/record command strings + args、从 pack `tooling/catalog.yml` 投影的 `adapterCandidates[]`、默认 `selectedAdapter` / sidecar `adapterId` guidance、replay behavior；managed adapter attempt 必须在外部执行前先运行 runtime 返回的 `-RecordAdapterExecutionDispatch` preview，再消费其 exact expected-binding-hash Apply command写入 immutable `dispatch.json`；sidecar template会携带dispatch ID/path/SHA，report-first事后补造dispatch会fail-closed；dispatch已记录但report缺失时，status/handoff会等待external harness，或仅在harness outcome已知时用`-DraftExecutionReport -ExecutionStatus failed|aborted`记录dispatch-bound terminal report；takeover后stale dispatch只能走distinct reauthorization，不能由新owner采用；record handoff 运行前需替换 `<executor-id>`，重复 `RecordArgs` / `CaseRelativeRecordArgs` 返回 `duplicate eventId` 且不追加 observations）和 sidecar 规则；adapter 写出 sidecar 后，可用 `-ValidateExecutionReport -GateEventId <authorized-gate-event-id> -ExecutionReportPath <path> -Format json` 做只读 strict validation preflight（在 case-local / authorized output workspace cwd 中可省略 `-Target`，report path 可相对当前 workspace），输出 `isMutation=false` / `applied=false` 且 `valid=true` 或 `valid=false` envelope；validation JSON/text 同样投影 compact `reportSummary`，让替换 executor 直接看到 valid/recordReady/recordBlocked、repair hints counts、report status/adapter id/actualBudget、refs/boundary hits、failure code/stage 与下一条安全命令，并在可用时携带 `adapterContext.candidates[]` / `adapterContext.selected` 且 text 输出 adapter candidate / selected adapter provenance（invalid sidecar 含 `error`、`errors[]`、`failureCode`、`failureStage`、带 `evidence[]` / `boundary[]` 的 `repairHints[]`、`reportPath`、可用时的 partial report 与 contract boundaries），且不写 observations ledger；传入 `-GateEventId <authorized-gate-event-id>` 与 `-ExecutionStatus` 时改为记录授权动作后的 observation execution evidence 到 `.rekit/facts/observations.jsonl`，包含 actual budget、output refs、evidence refs、boundary hits 与 escalation；也可传入 `-ExecutionReportPath` 读取 lane executor / tool adapter 写在 authorized output paths 下的 bounded `adapter-execution-report` JSON sidecar，并可在 case-local / authorized output workspace cwd 中省略 `-Target`、用当前 workspace 相对 sidecar path 记录 evidence，重复记录同一 sidecar 返回 `duplicate eventId` 且不重复 append observations，校验 action/status/gateEventId/budget/refs/boundary/escalation 后嵌入 evidence，并在 `execution.adapterContext` 中保留匹配到的 concrete tooling candidate；output refs、evidence refs 与 report path 必须落在 authorized gate 的 output paths 内；sidecar 若声明 `boundary-hit` / `escalated` 或实际预算越界，必须自带 `boundaryHits` 或 `escalation` marker，`boundaryHits` 必须被本次 authorized gate `stopConditions` 覆盖，`failed` / `boundary-hit` / `escalated` / `aborted` sidecar 必须包含 bounded summary；durable lane autonomy profile 完全覆盖时可记录 `authorized-gate`，并在 overview、handoff、continue digest/status 与 `missionBrief.authorizedGates` 中可见；否则 fail-closed 为 pending/denied decision；实际 heavy-tool 仍由 lane executor / tool adapter 在授权范围内执行，`/rekit` 只记录 request/evidence，不写 confirmed/authority。 |
| `/rekit sync` | kit -> case | 默认生成同步审查包；确认后才用 `-Apply` 写入 managed docs / managed block。当前显式 attached case 的 status/handoff 也会发现 completed verified pack-memory change；选择一个 `changeId` 后先审核 selected WhatIf 返回的 producer authority、单一 managed path、source/target/state hashes、backup/receipt和 exact plan hash，再执行原样 hash-bound Apply。成功后 case-local receipt 与 fresh status/handoff 证明已消费；runtime 不扫描其它 case、不自动 sync。 |
| `/rekit promote` | case -> kit | 默认生成回流审查包；确认后才用 `-CreateCandidates` 生成候选或用 `-Apply` 写回 pack。 |
| `/rekit doctor` | 只读 | 排障时详细验证结构；日常不必主动运行；维护自动化可用 `-Format json` 消费验证 rows。 |
| `/rekit repair` | case metadata | 迁移目录后先预览修复；确认后由 Claude 调用 backend `-Apply`。 |

Handoff currentness：`status` 或 generic runbook 中尚未绑定 publication plan SHA/stamp 的 Apply route 只是 non-executable review guidance。只有 fresh、同 scope 的 `/rekit handoff ... -WhatIf -Format json` 返回 exact `publicationPlanSha256`、`publicationStamp` 和 Apply request 后，才能执行该 Apply；project 与 lane publication identity 不得交叉复用，持久化 Markdown、takeover JSON、typed request、expected receipt 和 run-loop 必须保持一致。

`validate` 和 `plan-subagents` 仍是 backend/内部命令，不是日常主入口；`plan-subagents` planning mode 默认经 Go façade 生成 review artifacts，reviewer-intake mode 由主 Agent显式执行 strict WhatIf/Apply 与 post-validation，但 runtime 不自动 spawn agent；`packs` 是维护者/排障入口，用于多 pack 发现和矩阵验证；`note -List` 文本/table/tsv 与 `note -List -Format json` 默认经 Go façade 只读查询 ledger events；`note -WhatIf` JSON envelope 输出当前 `executorAction`、内存模拟 append 后的 `wouldExecutorAction`、`eventSha256` 与可重放时的 hash-bound `recordCommand`；该 command 带 `-CreatedAt`、`-EventId` 与 `-ExpectedNoteEventSha256`，record 时若 event body drift 会 fail-closed 且不写 ledger。实际 append 输出写入后的 `executorAction`，duplicate eventId 只返回未改变的当前 action 且不写 ledger；含 reviewer-intake 内部字段等不可 CLI 重放 event 时只输出 hash、不输出 misleading record command。note 仍只写 facts JSONL 或预览，不写 authority/confirmed。

## 日常工作流

普通用户只需在真实项目目录启动 `claude`，然后使用 `/steamai` 或自然语言；主 Agent 会从 fresh typed state 选择项目内 daily、状态、纠偏和接手 owner。下面的 `/rekit` 子命令、legacy `.rekit` 路径和 live gate 都是维护者、自动化、迁移兼容或按需排障参考，不是 current `.steamai` 项目的日常操作清单。

维护真实 session 产品链时，普通 `go test ./...` 不会启动 Claude。只有维护者显式运行以下 live gate，才会创建无敏感内容的临时 case，依次启动第一代真实 member、真实 Reviewer reject、证据绑定的人工 correction、replacement member 和独立新 Reviewer accept，验证旧 rejected manifest 不会自动复审、strict writeback、accepted-only feature-lane completion 与自动清理。省略 `-pack` 时保持 fresh 默认 `binary-re`；RH-08 跨 pack 维护验收只允许显式选择 `_template` 或 `web-security`，ordinary `-daily` 仍拒绝 `-pack` 并只从 fresh default 或 attached metadata 派生 pack：

```text
# 默认 binary-re：actual adapter + ordinary evidence/member/Reviewer lifecycle
go build -o "<outside-repository>/rekit-adapter-host.exe" ./cmd/rekit-adapter-host
go run ./cmd/rekit-host -live-acceptance -adapter "<outside-repository>/rekit-adapter-host.exe" -goal "<bounded-natural-language-goal>" -correction "<human-correction>" -receipt "<outside-case-receipt.json>"

# allowlisted cross-pack maintenance（不进入 binary-re adapter lifecycle）
go run ./cmd/rekit-host -live-acceptance -pack "<_template-or-web-security>" -goal "<bounded-natural-language-goal>" -correction "<human-correction>" -receipt "<outside-case-receipt.json>"
```

通过 receipt 必须同时满足 `passed=true`、exact `pack`、`manualPlaceholders=0`、`manualResultWrites=0`、两代 member 完成、独立 Reviewer 完成、completion fail-closed 边界成立且 `cleanup=removed`。默认 `binary-re` 的 `vmpIda.verified=true` 只表示 `evidenceScope.verifiedMeaning=bounded-existing-index-inspection-lifecycle`；同一 scope 还必须逐项写明 synthetic existing-IDA-TSV input、real contained compiled adapter child、real independent Claude Code sessions，以及 `sourceProducerObserved=false`、`realTargetToolReceiptObserved=false`，因此不能提升为 IDA export producer、真实目标工具验收或 VMProtect trace/devirtualization 实证。attached case 还必须证明 member packet cutpoint、accepted Reviewer intake cutpoint、同一 goal 的零 Claude completion recovery 和 terminal replay，且 `replayLaunches=0`。每个 member 记录从所选 pack manifest 派生的 `outputContract`（manifest path/SHA、task type、route ID 与 fields），Reviewer rejection/acceptance 必须绑定同一 exact route；completion 还会重验 packet shard 与 canonical `ReviewerResult.items` 完全一致，并都指向当前 member manifest。action-ready 路径继续要求 TaskContext 绑定当前 RESUME/checkpoint/owner/correction；终态 receipt 只把已完成 attempt 当作 immutable snapshot 验证其内部 artifact hashes、mission intent 与当前 exact pack contract，不能因 completion 合法刷新 lane 文档而误报历史快照漂移。receipt 将 durable owner、external attempt 与本次 host 启动顺序分别记录为 `ownerGeneration`、`attemptGeneration`、`hostRun` + `runLaunchOrdinal`，不再用一个含糊的 generation 字段混表示。

维护 RH-09 Windows 连续试用时，使用 Go-owned 聚合 gate；它顺序运行默认 `binary-re`、`_template`、`web-security` 三个真实任务，并追加既有真实进程中断恢复门槛，任一失败仍保留在最终仓库外 receipt 中：

```text
go run ./cmd/rekit-host -live-soak-acceptance -goal "<bounded-natural-language-goal>" -correction "<human-correction>" -receipt "<outside-repository-receipt.json>"
```

通过必须同时证明 3/3 任务成功、fresh/existing case、人工纠偏、rejection replay、replacement、独立 Reviewer、terminal replay、完整五阶段 recovery、所有 disposable case 真实创建且清理，以及 `manualResultWrites=0`。聚合 receipt 分开记录 task-level 最终成功率与 attempt-level 原始成功率，并记录总耗时、自然语言输入数、底层人工输入数、全部 member/Reviewer 启动与完成、durable member replacement、process replacement、`cleanupExpected/cleanupCreated/cleanupRemoved` 和失败分类；未创建的 case 不得计为 removed。只有 `reviewer-semantic-or-lineage` 失败允许一次 fresh-case retry，首次失败仍保留在 tasks、failure counts、session、duration 与 cleanup totals；provider/contract/cleanup/timeout 等失败不自动 retry。底层 typed provider diagnosis 即使被最外层 attempt-limit 文本遮蔽仍优先保留；provider auth/quota/model 未在现场真实触发时保持 `providerFailureObservation=not-observed`。普通 `go test ./...` 不启动该 gate。

维护跨 case pack-memory 复用链时，使用另一条显式 gate：

```text
go run ./cmd/rekit-host -live-pack-memory-acceptance -goal "<bounded-generic-pack-memory-goal>" -receipt "<outside-repository-receipt.json>"
```

该 gate 在 disposable isolated kit 中依次运行真实 Claude producer、packet-bound 独立 Reviewer 和第二 fresh case consumer，验证 strict raw-output → deterministic sanitize → candidate/promoted-source hash lineage、review-first promote、verification/retirement、completed catalog、selected sync/reconsume，以及只能引用 predecessor → accepted successor 新增内容的 exact accepted-delta quote/use proof；sanitize 只能重写 predecessor 中已知的两处 `capturesPath`，其它 replacement 或 deny violation 均 fail closed。Reviewer bytes 作为 outer reviewer plan-bound 的 hash/length/bytes in-memory snapshot 进入 canonical route；shard lock 内再次校验 packet、dispatch prompt exact bytes/SHA、current dispatch/currentness 和 snapshot identity，再直接写入 canonical input，不先发布 host relay source。consumer route 也只在当前 owner generation 的 `pack-memory-consumer` binding 与 current selected-sync receipt 的 `changeId`、source/receipt/plan SHA 全部一致时开放；case scope与pack-memory下钻都必须经过同一strict validator和repo+case lease，checkpoint绑定下钻后实际执行的case request而非全局pack-memory focus，真实Claude process启动前还会在该双lease内最终重验immutable task context、target/state/receipt、catalog与promoted source，drift时零process launch。durable member attempt ID 与 external-session attempt ID 属于不同命名空间；launch通过exact task-context path/SHA和current durable inspection绑定，不要求二者字符串相等。它不向当前仓库 pack 写入测试知识；默认清理 isolated kit/cases；receipt 必须位于仓库外，且不公开本机或 disposable case 绝对路径；失败receipt保留bounded typed phase/attempt诊断，并按Windows大小写与两种路径分隔符脱敏known local paths。普通 `go test ./...` 不运行该 gate。

维护 onboarding、status quickstart、continue/reconcile、handoff 或 replacement executor takeover 路线时，当前只在 Windows 本机运行以下 Go-native smoke，覆盖完整日常闭环；它不作为跨平台验收：

```text
go test ./internal/rekit/cli -run '^TestRunDailyMissionControlRouteSmokeProductPath$' -count=1
```

具体场景与安全边界见 `rekit/tests/README.md`；正常 case 用户不需要手动运行该维护命令。

### 1. 看当前项目状态

current 项目直接输入：

```text
/steamai
现在到哪了？下一步是什么？
```

主 Agent 会读取 compact status；只有 compact 结果明确要求 full diagnostics 时，才消费 runtime 返回的 typed full-diagnostics route。它会展示：

- 当前主线和功能支线；
- 共享事实、request、candidate、publication 统计；
- Mission Control brief：ready/blocked lanes、pending gates、authorized gates（含 execution boundaries、eventId 与 report contract handoff）、open decisions、interventions、next agent actions 与 escalations；
- 逐 lane executor action index：blocked/ready、pending gate / open intervention / open decision counts、requirements、resume/handoff command 与 current executor / generation / last takeover 摘要；
- blocker-aware 下一步：`continue` 的 apply JSON、text 与 handoff Markdown 使用 lane-local executor actions；blocked lane 只建议 reconcile / pending gate / open decision 处理，ready lane 才建议自己的 continue，paused/closed/unready lane 回到 handoff/read-only；
- 未决 candidate、pending-gate、authorized-gate、最近 verification / decision 等 review loop 摘要；
- 需要人工确认的事项；
- 推荐下一步。

### 2. 选择并继续某条工作线

```text
继续推进当前 mission。
继续主线。
继续 login 工作线。
```

主 Agent 从 fresh typed state 选择工作线；多条 open 工作线时先展示 typed choices，选择前零启动、零写入。明确选择后，主 Agent 先消费 runtime 返回的只读计划与结构化 `missionBrief`，仅在用户意图覆盖 exact request 时执行其 typed Apply。runtime 会整理对应工作线的 case-local 状态：收集 outbox/workspace 事件、发布低风险共享事实、路由 request、验证候选并刷新接续提示；Apply 后的 run status 与 digest 会记录 Mission Control brief，便于 lane executor 直接判断 open decision / pending gate / authorized gate / intervention 状态。

安全边界：candidate 同时满足 evidence、accepted verifier、confidence 阈值、CSV schema、无冲突、backup、diff、max rows 时，只代表可进入 authority review。`continue -Apply` 不写 authority/confirmed；authority 写入、覆盖/删除、冲突、schema change、外部副作用或破坏性动作必须经过独立 gate 和显式用户确认。

### 3. 开一个功能支线

```text
开一条 login 工作线，专项核对登录逻辑。
```

主 Agent 会预览创建或进入功能支线，例如 `feature-login`，并在需要写入时让用户确认具体动作；确认后只消费 runtime 返回的 exact typed Apply。若当前 Claude Code 会话要登记为该 lane 的 executor，主 Agent会提供 runtime 要求的 executor/actor/reason。runtime 只记录 `currentExecutor` / `executorGeneration` / takeover metadata 并刷新 RESUME、checkpoint、board、overview 和 handoff，不负责创建、停止或监控会话。功能支线用于专项分析、证据收集、候选结论和 request；它默认不能写 confirmed CSV、`routine_ir.*` 或 `references/binary-re/task-handoff.md`。

主线/支线不是级别高低，而是写入权限不同：

| 类型 | 职责 | 可写 |
|---|---|---|
| 主线 | 维护最终结论、验证和长期 handoff | canonical 文件 |
| 功能支线 | 分析某个功能、收集证据、提出候选和 request | 自己的 workspace |

### 4. 换新会话

直接告诉主 Agent：

```text
生成项目接手信息，让新会话接手。
生成 login 工作线的接手信息。
```

主 Agent 会先读 fresh status，并在你明确要求发布后才消费 scope-bound handoff preview 与其 exact typed Apply；current 项目的 durable handoff 写入 `<active-state-root>/handovers/`。legacy-only 项目仍使用自己的 legacy active root，但不会与 current 状态拼接。

新会话在同一项目目录启动 `claude` 后，使用 `/steamai` 或直接说“接手并继续主线”；无需手工寻找 handover 路径或拼底层 continue 命令。工作线接手文档会附带本工作线的 workspace packet、最近 verification、decision、pending-gate、authorized-gate、intervention 和 rollback 摘要，便于新会话看到 reviewer verdict、main decision 与 durable autonomy gate decision 的状态。

这些接手文档只引用 `references/binary-re/task-handoff.md`，不会覆盖它。

### 5. 同步模板更新到当前项目

直接说“审查并同步模板更新”，主 Agent 会先生成 review 包并展示具体范围和差异；用户确认后才消费 runtime 返回的 exact typed Apply。整个过程始终 review-first，review artifacts 位于 `<active-state-root>/reviews/`。

写入型同步会同步：

```text
references/binary-re/README.md
references/binary-re/agent-driven-re.md
references/binary-re/workflow-template.md
references/binary-re/progressive-disclosure.md
references/binary-re/toolchain-router.md
references/binary-re/singleton-handler-review.md
references/binary-re/lane-collaboration.md
CLAUDE.local.md 中的 managed router block
```

不会覆盖：

```text
references/binary-re/task-handoff.md
tools.local.yml
captures/**
artifacts/**
CLAUDE.local.md 中 block 外的 case 私有内容
```

默认 `sync -Apply` 不会覆盖已存在的本地 template files；只有显式 `-Force` 才会在写入前备份并覆盖 manifest 声明的本地模板目标。

`sync` 只允许作用于已经 `attach/init` 的 case。若目标目录拼错或还未绑定，会直接失败，不会静默创建假 case。

### 6. 回流可复用经验

直接说“把这次可复用经验整理成 promote 候选”，主 Agent 会先生成 bounded diff 和安全的脱敏 preview；review artifacts 位于 `<active-state-root>/reviews/`。整个过程始终 review-first，Claude 复核后，你再选择明确写入动作：

1. `-CreateCandidates`：生成 managed docs 候选或 tooling candidate。
2. `-Apply`：按已确认内容写回 pack。

`-CreateCandidates` 的 JSON `reviewPlan.reviewSummary` 与文本 `promote candidates review summary...` 会先给出 candidate/tooling/index、review/cleanup/reconsume artifact、Mission Commander next action、no-merge/no-cleanup/no-heavy/no-authority boundary，以及 terminal `proofSummary`：expected proof total/present/missing、decision/cleanup/reconsume missing counts、proof progress、current stage、next missing proof type/path/candidate/pack target、compact `nextMissingProof` detail（stage/proofType/path/candidatePath/packTarget/when/action/format/evidence/boundary）与 proof boundary。`status` / `release-check` 的 pack-memory handoff 也输出 compact review/proof summary、expected proof paths、present/missing counts 与同类 next-missing proof detail；当存在 next missing proof 时，还会投影 `currentRunLoopStepId` / `runLoop`，把 `inspect-proof-gap → bind-review-packet → draft-proof-whatif → apply-proof-with-expected-hash → refresh-pack-memory-status → continue-review-cleanup-reconsume` 作为只读 operator workflow。缺 case-local packet 时 current action 停在 `bind-review-packet`，case-local status 绑定 packet/evidence 后才进入 `draft-proof-whatif`，便于 replacement executor 刚预览或生成候选后不扫描完整候选列表、`reviewArtifacts[]` 或 proof 目录，就判断是否还需先补 packet、decision proof、cleanup proof 或 reconsume proof，以及该 proof 应记录什么 evidence / boundary。

`promote` 很保守：若 managed docs 含真实绝对路径、样本名、RVA/VA、ctx/round 快照、artifact/capture/trace/dump 路径，会阻止直接回流。工具链经验只有在脱敏后不再命中 deny pattern 时才写 sanitized preview；候选由你审查后合入正式 tooling 文档。合入 `tooling/catalog.yml` 或 `tooling/recipes/*` 后，后续 init/attached fresh case 通过 `templateRoot` + `templatePack` 读取同一 pack tooling 资产；`sync` 不把 tooling files 复制进 case managed docs，避免把候选经验静默混入 case 私有路由。

`promote` 只允许作用于已经 `attach/init` 的 case，避免从普通目录误回流到 pack。

## 内部状态模型

日常不用理解这些文件，但排障或 review 时可能会看到：

| 路径 | 内容 |
|---|---|
| `.steamai/board.json` | current 项目概览的机器状态。 |
| `<active-state-root>/lanes/<id>/` | 每条工作线的事件、任务、inbox/outbox、`prompts/RESUME.md`、`checkpoints/latest.json` 与 append-only completion lifecycle；current 项目的 active root 是 `.steamai`。 |
| `<active-state-root>/facts/*.jsonl` | append-only ledger：observation、hypothesis、candidate、verification、decision、intervention、rollback、publication、request；runtime 统一解析 active root，不由命令自行拼路径。 |
| `<active-state-root>/runs/<run-id>/digest.md` | typed continue 每轮摘要，记录 inputs、route、packet refs、Mission Control brief、outputs、decisions 与 open risks，供 replacement executor 接手。 |
| `<active-state-root>/handovers/latest.md` | 项目级接手索引。 |
| `<active-state-root>/handovers/<laneId>-latest.md` | 指定工作线接手文档。 |

字段名中仍保留 `lane`，这是内部 schema 名称；用户层统一称“工作线 / 主线 / 功能支线”。

## 高级/内部：子 agent 分片计划

`/rekit plan-subagents` 是内部 tactical reviewer planning/intake 入口。planning mode 按 manifest route 生成 `packet.json`、`summary.md`、read-only shard handoff、strict reviewer result contract、owner binding 与 writeback guidance；它不自动启动 reviewer。planning result / packet 的 `reviewerOrchestration.summary` 以及 terminal / `summary.md` lines 会直接给出 mode、target lane、reviewer/dispatch counts、owner binding、intakeAvailable/dispatchOnly、Mission Commander queue counts、first dispatch、current action、next actions 与 no-spawn/no-heavy/no-authority boundary，让 replacement executor 不必解析 nested dispatches / lifecycle / action queue 才能接续。主 Agent调用 shard 的 read-only Agent tool request 后，先运行 `-RecordReviewerDispatch -WhatIf`，仅在 harness 实际接受该 session 后消费返回的 hash-bound Apply；session 结束后运行 `-RecordReviewerCompletion -WhatIf` 并消费其 hash-bound Apply。immutable receipts strict 绑定 packet/route/shard/prompt SHA、harness/session、current lane owner generation、completion outcome 与 exact result input hash/bytes；只有 current owner 下的 successful completion 才可进入 source capture。reviewer 产出单个 contract-compliant JSON object（包含 dispatch receipt 绑定的 `reviewerSession`）后，主 Agent显式传入 `PacketPath`、`ReviewerResultPath`、`Lane` 与 `Actor`，先用 WhatIf 校验 packet/route/shard/items、receipt lineage、route output、evidence refs、conflicts、blocked actions 与 lane executor owner binding，再用 Apply 按 verification-before-decision 顺序写 case-local facts；reviewer intake JSON / text 的 `summary` 会直接压缩 status、dispatch progress、compact `orchestrationProgress`（dispatch index/total、completed/open、current/next/remaining shards）、blocked/repair counts、compact `repairGuidanceSummary`、postValidation totals、reviewer writeback count、compact `reviewerWritebackSummary`、current/next actions 与 no-heavy/no-authority boundary。写回后的 downstream status/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 还会输出 compact `reviewerWritebackSummary` / reviewer writeback summary lines，直接汇总 verification/decision counts、latest shard/reviewer session/result/packet/route、owner binding / risks / conflicts / route output flags、latest evidence refs 与 no-heavy/no-authority/no-spawn boundary，replacement executor 不必逐条扫描 `reviewerWritebacks[]` 才能复核 reviewer provenance。deterministic event IDs 支持相同 intake 的安全重试。写后返回完整 overview、lane handoff 与 doctor validation，并在 `postValidation.summary` / text summary lines 中直接给出 verification/decision totals、doctor row count、lane/executor action state、reviewer writeback count、compact `reviewerWritebackSummary`、current action、next actions 与 no-heavy/no-authority boundary。PowerShell fallback 已退休，即使设置 `REKIT_GO_DISABLE=1` 也不会回落到 PowerShell 业务实现。

在 reviewer result 写回前，case `status`、`handoff`、`continue`、continue run `status.json` / `digest.md`、lane `RESUME.md` 与 checkpoint 也会投影 open `reviewerDispatchIntakeHandoffs` / compact summary：直接显示每个 shard 的 reviewer result 目标路径、Agent tool request、dispatch/completion receipt command 与 provenance、staging/collection/WhatIf/Apply intake command、ready-for-dispatch / running-unknown / completion-preview / failed / stale-owner / source-capture-ready / staging-ready / collection-ready / intake-ready / dispatch-only state、packet-level progress（dispatchTotal / completed / open / nextOpen / remaining）、`nextActionRunbookSteps[]` / per-shard `runbookSteps[]` 与 no-spawn/no-heavy/no-authority boundary，便于 replacement executor 不打开完整 `packet.json` / `summary.md` 或扫描 reviewer writebacks 即可从 live command 或 durable artifact 接续。实际执行 `-CaptureReviewerResultSource`、`-StageReviewerResult` 或 `-CollectReviewerResult` 后，返回 JSON 的 `runbookSteps[]` 与 text 的 `reviewer result ... runbook` 行也会再次给出当前 command、hash-bound Apply 纪律、下一步 WhatIf 和 capture/staging/collection/packet-level intake 分离边界，避免会话切换时重新拼 reviewer writeback 顺序。若 collection preview 发现 canonical reviewer result 已被不同 bytes、empty-file 或 symlink obstruction 占据，会以 `status=recovery-required` 返回 canonical kind/hash/bytes 与独立 `-RecoverReviewerResult -WhatIf` handoff；collection Apply 仍拒绝覆盖，recovery 必须走自己的 WhatIf→hash-bound Apply；direct recovery JSON 的 `runbookSteps[]` 与 text 的 `reviewer result recovery runbook` 行会串起 WhatIf→hash-bound Apply→interrupted finalize→collection WhatIf。

生成的 `packet.json` / `summary.md` 会标出 route 选择原因、目标 lane 的 `ownerBinding`（current executor / generation / last takeover snapshot）、每个 shard 的初始 `planned` 状态、`shardHandoffs[]` read-only dispatch prompt、spawn/merge 责任、`reviewerResultContract`、`intakeChecklist[]`、`reviewerDecisionMappings[]`、`conflictHandling[]`、`reviewerIntakeCommands`、`writebackSequence[]` / `commandBindings[]` 和 post-review merge guidance。若 packet 要求 owner binding 且当前 lane executor/generation 已被 takeover，reviewer intake 会在写 facts 前 fail-closed；planning 阶段的 `packet.json` / `summary.md` / text handoff 会通过 `reviewerIntakeCommands.repairGuidance[]` 预先列出会导致 blocked intake 的原因、修复动作、证据字段与 no-apply / no-heavy / no-authority 边界；blocked / event-id-collision / post-validation failed intake 也会返回结构化 `repairGuidance[]`，top-level `summary.repairGuidanceSummary` 与 terminal text 会直接给出 total、primary reason/action、evidence、boundary 和下一条 safe command，让主 Agent 不必解析完整 JSON 或人工拼 blocked reason 才能修复后重跑 WhatIf。verification 与 decision events 会记录 `reviewerSession`、`ownerExecutor`、`ownerGeneration`、`ownerBindingMode`、`ownerBindingTarget`、reviewer `decision` / `recommendedVerdict`、`risks[]` / `conflicts[]` 与 normalized `routeOutput`；这些字段也会通过 downstream `reviewerWritebacks[]`、status/overview/handoff/continue、lane `RESUME.md`、checkpoint 与 digest 投影，replacement executor 不必重开 reviewer result JSON 即可复核 reviewer provenance。evidence-ref validation 只证明引用为 packet ID、已知 ledger event ID 或存在的 case-local 文件，证据内容是否足够仍由主 Agent审查。reviewer 本身不写文件或 ledger；runtime 不自动 spawn agent、不写 authority/confirmed、不执行 heavy-tool，也不修改 managed docs 或项目源文件；`sync`/`promote` 继续 review-first。

## 工具经验保存在哪里

工具经验不会只留在当前 case。现在分两层：

| 层级 | 路径 | 内容 |
|---|---|---|
| 通用 tooling 资产 | `packs/binary-re/tooling/` | 工具 catalog、recipes、脚本模板化清单、补丁/止损经验；fresh case 通过 pack reference/tooling 路径重新消费，不复制成 case-local managed docs。 |
| 当前 case 状态 | `<caseRoot>/references/binary-re/toolchain-router.md` | 当前样本具体脚本、路径、工具结论和状态。 |

通用 tooling 资产包括：

```text
packs/binary-re/tooling/catalog.yml
packs/binary-re/tooling/recipes/public-tool-triage.md
packs/binary-re/tooling/recipes/lane-collaboration.md
packs/binary-re/tooling/recipes/vmenter-context-probe.md
packs/binary-re/tooling/recipes/unicorn-trace.md
packs/binary-re/tooling/recipes/focused-handler-review.md
packs/binary-re/tooling/recipes/value-flow-mining.md
packs/binary-re/tooling/recipes/ida-x64dbg-mcp.md
packs/binary-re/tooling/scripts/README.md
packs/binary-re/tooling/patches/vmpimportfixer-timeout-and-quiet-log.md
```

原则：具体样本名、RVA、ctx、coverage 留在 case；可复用工具路线、脚本接口、短测/止损经验进 tooling。

## Legacy `.rekit` 项目迁移到新目录

本节只适用于仍使用 `.rekit`、`/rekit` 和中央 kit metadata 的旧项目；current `.steamai` 自包含项目不走这套 repair 流程。推荐顺序是：**先复制，确认修复 metadata，再验证新目录，最后归档旧目录**。

### 1. 复制 case 目录

先关闭正在使用该 case 的 Claude Code、IDA、x64dbg、trace 脚本等进程：

```text
robocopy <oldCaseRoot> <newCaseRoot> /E
```

### 2. 在新目录检查状态

```text
cd <newCaseRoot>
claude
```

然后：

```text
/rekit status
```

如果 `.rekit/instance.yml` 里的旧 `projectRoot` 和当前目录不一致，`status` 只会提示，不会静默修改。

### 3. 确认后修复 metadata

确认这是你预期的迁移后：

```text
/rekit repair
```

`repair` 默认只预览。需要写入时，直接告诉 Claude：

```text
确认修复，执行 repair -Apply
```

`repair -Apply` 会更新：

```text
.rekit/instance.yml
.re-template.yml
.claude/skills/rekit/SKILL.md
```

### 4. 排障验证

```text
/rekit doctor
```

### 5. 检查旧绝对路径

迁移后还要搜索只属于旧 case 根目录的绝对路径：

```text
CLAUDE.local.md
.re-template.yml
references/binary-re/task-handoff.md
自写脚本中的 PROJECT_ROOT / workdir / output path
```

目标样本路径如果没有变化，不需要改。

## 后端脚本什么时候用

正常情况下不用。

这些入口只是为了自动化、按需 CI、排障或旧流程兼容：

```text
cmd/rekit/main.go                  # Go-native backend CLI，CI workflow / 维护自动化入口
rekit/rekit.ps1                    # retained compatibility façade；无业务 runtime、无 PowerShell fallback，默认 CI 不依赖
rekit/tests/README.md              # smoke 维护指南与验证选择表
rekit/tests/catalog.json            # smoke 机器可读导航目录（非自动执行器）
rekit/tests/catalog-smoke.ps1       # smoke catalog 输出契约自测
rekit/tests/facade-smoke.ps1       # façade 委托回归 smoke
rekit/tests/pack-smoke-lib.ps1     # 多安全领域 pack smoke 共享 helper
rekit/tests/pack-smoke-matrix.ps1  # 多安全领域 pack smoke 串行矩阵 runner，支持 -Format json 与 -DiscoveryOnly
rekit/tests/pack-smoke-matrix-selftest.ps1 # pack smoke matrix 输出契约自测
packs/binary-re/scripts/bootstrap.ps1
packs/binary-re/scripts/update.ps1
packs/binary-re/scripts/validate.ps1
packs/binary-re/scripts/promote.ps1
```

面向新项目用户时，优先用自然语言或 `/steamai` 表达，不要让用户手动跑这些脚本；`/rekit` 只出现在明确标注的 legacy compatibility、内部 API 或维护诊断说明中。

## 架构边界

- `/steamai` 是新项目用户入口；`/rekit` 只用于 legacy compatibility、内部 API 和维护诊断。
- `rekit/rekit.ps1` 是 retained PowerShell compatibility façade，只负责旧参数兼容、Go delegation、no-fallback guard 与错误透传；它不承载业务 runtime，也不是新项目 fallback。
- Go backend 位于 `cmd/rekit/**` 与 `internal/rekit/**`；低风险只读命令 `status`、`packs`、`doctor/validate`，attached case 的 `overview` 文本/JSON 与缺 board 时的 case-local board/facts/policy/default authority lane 初始化，`note -List` 文本/table/tsv/JSON 只读查询，attached case 的 note append / `note -WhatIf` facts JSONL 写入或预览，`gate -WhatIf` 非写入 heavy-action authorization preflight，`gate -Apply` pending-gate / authorized-gate request ledger decision 写入，attached case 的 `start` / `handoff` JSON preview、explicit apply、文本 preview 与 bare/default 工作线 flow，`continue` JSON preview、explicit apply 与文本/default preview 的 case-local facts/routing/run digest/lane resume/checkpoint/board safe subset（存在 effective open intervention 时 fail-closed，需先 `reconcile`），边界清晰的 case lifecycle 命令 `attach`、`repair`、`init/bootstrap` 的预览与显式 `-Apply`，`/rekit sync` review、`sync -Apply` 实际写入和 `sync -Apply -WhatIf -Format json` 非写入预览，以及 `/rekit promote` review、review artifact 写入、promote `-CreateCandidates` 实际候选写入、promote `-CreateCandidates -WhatIf -Format json` 非写入预览、promote `-Apply` 实际 pack source 写入和 promote `-Apply -WhatIf -Format json` 非写入预览、`reconcile` 显式 resolution 写入与 lane executor/resume/checkpoint/board 刷新默认委托 Go。`release-check`、`status`、`packs`、`doctor/validate`、`attach/repair/init/bootstrap`、`sync/update`、`promote`、`overview`、`note`、`gate`、`start`、`handoff`、`continue`、`reconcile` 与 `plan-subagents` 已 no-fallback；`REKIT_GO_DISABLE=1` 不再让 Go-default command rows 回落到 PowerShell 业务实现。`plan-subagents` review artifacts 只写 review packet/summary/combined diff 路径、不自动 spawn agent；actual heavy-tool 执行、authority/confirmed 写入和非 note/gate/continue/reconcile apply 的其它 ledger 写入命令仍不由默认 Go façade执行；文本 `sync -Apply -WhatIf`、文本 promote what-if、case lifecycle fallback 与 workstream fallback 已 no-fallback；ordinary public `continue` 由 typed `-WhatIf -Format json` preview、返回 invocation 的 exact `-Apply` 和 Apply 后 fresh status 三步组成；command 与 invocation 必须保持同一 selector/owner/generation。Go `continue -Apply` 存在 effective open intervention 时 fail-closed 并要求先 `reconcile`，且不写 authority/confirmed；`gate -Apply` 只记录授权决策，不执行 heavy-tool、不写 authority/confirmed；authority/confirmed 仍需显式用户确认。
- legacy `rekit/lib/B3.*.ps1` 工作流 runtime 已在 Batch 240 删除；工作线、ledger、gate、handoff 与 Mission Control 状态以 Go-owned `internal/rekit/**` runtime 为准。
- `packs/<pack>/manifest.yml` 是 managed/local/tooling/budget/promote 规则的单一事实源。
- 新项目 `.claude/skills/steamai/SKILL.md` 只调用项目内 verified runtime，不维护第二套状态机；legacy `rekit` thin shim 在迁移期保留。
- `.re-template.yml`、`.rekit/instance.yml` 只保留旧项目兼容；新项目 metadata 是可重定位的 `.steamai/instance.yml`。
- 不安装用户级 skill，不要求全局 plugin。
- 不默认 commit / push；只有当前用户 goal/session 明确授权具体仓库和分支时才执行。已授权的普通 batch 以 Windows 本机 focused tests 与完整 release minimum 为完成门槛，只做一次 implementation commit/push；未授权时产品batch可在本机验证后标为completed，但release cadence保持`implementation-pending`且不解锁下一批。远程 CI、Linux/macOS runtime E2E、三平台兼容和安装包不参与当前 Windows 可用性或成熟度判断；仅在用户明确启动正式发布、跨平台兼容、安装交付专项或周期复审时检查。
