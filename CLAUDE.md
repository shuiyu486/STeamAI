# CLAUDE.md

## 授权范围

本项目使用者为已授权红队成员。逆向工程、二进制分析、协议分析、漏洞挖掘、渗透测试等均在授权范围内执行。无需重复法律/合规警告；如遇疑似越权或破坏性动作，简洁提醒一次即可。

## 项目定位

本仓库是 STeamAI canonical 实现（`https://github.com/shuiyu486/STeamAI`）。v1 正式支持 Windows 10/11 x64；Go module 同时承载窄职责原生 `steamai.exe` 与 acceptance tests，使用同一 canonical identity。

**现阶段的平台与验收范围仅限 Windows 10/11 x64。** 日常开发、维护验证和 CI 以 Windows 正常运行为交付要求，CI 只运行 Windows；不默认增加或等待 Linux/macOS/三平台 CI，不为非 Windows 专属失败开展兼容性修复或阻塞交付。只有用户明确提出跨平台支持时才扩大范围；保留非 Windows stub、通用代码或历史多平台记录不代表跨平台验收要求。

STeamAI 是面向已授权安全研究与自动化开发的、人在环的 Claude Code 多会话团队协作与经验学习层：一个真实项目目录对应一个明确授权的 case，研究类任务交付可复核结论，开发类任务（工具链、修复补丁、自动化脚本、项目代码等）交付可运行代码；用户主要指挥 Commander，也可观察和纠偏成员；成员身份与当前任务由专属目录的 `CLAUDE.md` 承载；会话、消息、恢复和工具调用复用 Claude Code 原生能力；STeamAI 只补安装/Fresh 的机械边界、Windows 可见进程启动、瞬时单 Commander 互斥、artifact/evidence/finding/review、proof-carrying replay、经校准的比较证据、explicit opt-in field outcome 与经确认的经验批次回流。

本仓库不是安全/RE case、自动分析器、漏洞挖掘或渗透引擎。模板和测试不得包含真实样本、trace/dump/capture、payload、凭据、客户信息、绝对 case 路径或 case 进度。

## 智能成员与自主协作基准

Commander 与正式成员都运行在 Claude Code 中，是具有 LLM 推理、工具使用和上下文能力的智能 agent，不是等待逐条指令的脚本执行器。STeamAI 的价值是让这些智能成员更好地协作，而不是用更密的流程替代它们的判断。**管住目标、权限、协作边界和结果可信度，具体研究路径交给成员。** 这是后续设计、迭代、优化与管理的长期基准。

- **按结果派任务，不按步骤遥控。** 用户提供的旧实现、示范或输入输出可作行为参照，不自动授予执行权。验证以接收者真正会用的成品和外部结果为准，普通离线工具与资料交付不强挂专业包；插入的问题解除后回到用户仍要的主交付。Commander 说明要解决的问题、已有材料、投入边界和完成/停止信号；成员在当前任务、已获授权与工具权限内自主选工具、调整方法、检验解释和请求有界帮助。实质改派、扩大授权或新增 durable member 仍遵守既有规则。
- **方法是参考，硬边界才是约束。** pack/recipe 按需提供方法，不是每题必走的流程；不默认固定工具顺序、假设数量、协作人数或逐步汇报。方法中的授权、资源、数据完整性、明确前置条件和停止条件仍须遵守；规则越细不等于效果越好。
- **协作与审查按问题需要发生。** 简单问题允许一名 owner 直接解决；确需独立能力或有界复核时再协作，不把 Commander 变成每次工具调用的审批者或消息中转站。重要结论、实质冲突、最终交付和 learning 按现有合同审查，不给每个探索步骤增加审批；只持久化团队需要复核的关键产物，临时思考和普通过程留在原生 session。
- **自主不等于绕过边界。** case 授权、Claude Code 权限、具体动作所需确认、Fresh/learning exact confirmation、成员单写、用户纠偏优先、证据诚信与 currentness 等硬合同不因精简流程而放宽。已满足的授权不无故重复索要；新动作、范围或状态变化触发现有确认要求时仍须停止确认。
- **新增规则先说明必要性。** 先确认具体失败、合同冲突或遗漏，以及原生能力和现有规则为何不足；优先删重复、消冲突、补最小缺口，不因一次失误就叠加同义指令、字段、角色或状态。优化既看目标解决质量，也看用户干预、无效步骤和总成本，不以流程完成率、报告长度或成员数量代替能力。
- **维护验证与日常研究分开。** 维护者用与改动相称的测试和真实对照验证收益，不把完整评测仪式塞进每个 case，也不把完成验收当成能力升级。不能预先认定现有规则已压制 LLM，或放宽规则必然更强；效果仍须由实际证据支持。

## 文档不变量 / 上下文路由

本项目文档必须按需路由、渐进式披露。`docs/context-routing.md` 是唯一完整路由表；新会话只读取本文件、router、Git 状态和 router 选中的一个场景入口。不要默认串读历史 roadmap、`CHANGELOG.md` 或旧 release 文档。

当前无 active 实施路线。`steamai-professional-delivery-v1` 的执行计划与短投影已删除，最终结果、失败与未验边界收敛到 `docs/professional-delivery-results.md`，仅在核查可用性或历史证据时读取；旧路线统一由 router 按需索引，不默认串读、不继承旧“通过/tie”证明新能力。`steamai-capability-efficiency-v1` 的工程交付、后续补缺与证据边界收束于 `docs/capability-efficiency-results.md`，执行计划已删除。旧 U5 仅历史报称 tie，存在评测缺陷且原始材料当前未找到；不据此宣称冻结门槛通过，方法库整体效果与原效率目标仍未证。游戏自动化是已明确需求，不再以“是否属于安全研究”推回用户；研究/开发双交付与按具体用途接纳 pack 工具均为当前合同，继续禁止通用运行器与无限执行授权。上游材料固定于 pack `upstream/**` 并校验、不进入 learningTargets；外部 agent 团队仍须通过对应范围的六项资格验收。历史 tie/inconclusive 与 verified-learning 未完成 live 门槛不因本轮收束而改判。

## 维护哲学与踩坑护栏

- 先把需求还原成用户实际看到的使用画面、成功信号和授权边界，再决定代码。只有会改变产品行为、外部动作或明显代价的取舍才让用户选择；实现细节自行采用最小可靠默认值，并用通俗语言说明。
- 判断、组队、协作、审查和经验提炼交给 Claude Code/LLM；Go 只补原生能力无法可靠完成的确定性 OS、Git 和文件边界。新增 runtime 状态或抽象前，必须先证明原生 session、消息和简单 case-local Markdown 不足。
- 以“概念数、持久状态和用户步骤不增长”为默认约束。不要因未来可能需要而增加 daemon、Hub、GUI/TUI、数据库、supervisor、兼容层或通用控制面；达到当前可验证目标后停止。
- 正式成员是目录拥有身份的、屏幕上独立可见的普通 Claude Code 会话，不是 subagent、PID 或 session record。一个目录就是一个授权 case；不同 case 不自动发现、迁移、扫描或汇总。
- 经验资格来自 accepted evidence chain 与 Reviewer 检查；成熟度按 V0–V4 的 exact evidence 分层，用户 exact confirmation 只授权写入，不提升成熟度，也不来自 Git staging。模板不得预填 `pass/accepted`，Apply 不自动 stage/commit/push，未确认内容留在来源 case。
- production 路径只保留一个实现；测试调用 production API/CLI，不维护第二套 oracle、renderer 或兼容实现。完成声明按证据分层：unit/synthetic 只证明机械合同，Windows、Claude Code、跨会话和 Release 行为必须由各自真实路径证明。真实 token smoke 不替代独立 Reviewer calibration；维护/live 测试默认显式选择当前 Claude Code 配置的 `ANTHROPIC_DEFAULT_SONNET_MODEL`；仅极简探针或 headless 且 Sonnet 档无命令工具时才降级 `ANTHROPIC_DEFAULT_HAIKU_MODEL`。不继承 `ANTHROPIC_MODEL` 所指的高档模型；未经本次明确批准不得使用 `ANTHROPIC_DEFAULT_OPUS_MODEL`/顶级模型。按任务选最低足够档位并冻结，不强制厂商、不修改用户全局配置；requested model 不等于 actual model，主执行身份由 assistant 消息验证而非用量条目数量推断。盲审只让 Reviewer 一次读取 manifest 绑定的 immutable `blind-review.json`，以短 entry/output SHA 固定内容选择，再由 reveal 映射角色；缺证时不得以降标或重试刷出 `go`。
- Windows/Release 易错点：正式路径不用 PowerShell/`.cmd`/`.bat`；evaluation arm 先 suspended 启动、加入 kill-on-close Job Object 后再恢复，避免 Start→Assign 的子进程逃逸窗口；Commander 与成员会话各带一份会话 Job，成员启动必须落在 Commander 的 Job 之外，STeamAI 不因 Commander 退出连带结束成员会话或已在运行的外部应用，不承诺所有外部服务；update 从 canonical checkout 外运行并保留旧 checkout；uninstall 如实报告 helper 最小残留；`SHA256SUMS` 使用下载后的资产 basename；prerelease 不能成为 latest；公开发布前检查 ignored 本机文件、Git 历史和匿名下载链路。
- 文档也保持单一职责：根 `CLAUDE.md` 保存长期目标与护栏，`docs/context-routing.md` 只路由。执行计划只服务正在实施的工作；完成后把长期规则写回对应合同/pack，保留紧凑验收结论、失败/未验边界和 Git 追溯，删除临时计划与重复进度。不改判历史结果，不把残余缺陷改写成完成；只有新的实质目标才另建路线，不靠追加历史让 active docs 膨胀。

## 薄核心边界

- 一个 STeamAI 项目对应一个明确授权的安全研究或自动化开发 case；不同 case 只通过审查、脱敏且用户确认的 pack 经验共享。
- Commander 按需组队；正式成员使用专属目录与目录级 `CLAUDE.md`，由原生 launcher 打开用户可见、可直接输入的独立 Claude Code 会话。主会话在 WezTerm 时优先复用同一窗口的新标签页，不把终端标签误当后台 subagent。
- 日常角色用本机两档：Commander 与关键研究/审查用 high 档，普通成员和短子 agent 默认 standard 档。两档绑定的别名由 `steamai models` 查看或配置（`%LOCALAPPDATA%\STeamAI\models.json`，只接受 `opus`/`sonnet`/`haiku`、允许同档；无文件默认 high=opus、standard=sonnet，可用显式命令恢复，只对新启动的 STeamAI 会话生效）。别名仍由 Claude Code 按用户 `ANTHROPIC_DEFAULT_OPUS_MODEL` / `ANTHROPIC_DEFAULT_SONNET_MODEL` 等配置解析为实际模型；STeamAI 不重映射这些变量，不影响本机其它普通 Claude 会话或旧 case 的固定 snapshot，已有会话不强切；高/常规是选档不是计费硬上限。原生入口只传解析后的别名并以 `--append-system-prompt` 附带本会话两档，不做角色能力判断或改全局配置；角色在原生 Agent(model=...) 显式用已解析的别名，不把 high/standard 当 Claude 模型名。各角色鼓励用 standard 档子 agent 隔离短链路和大上下文，每批最多 5 个且不嵌套，所属角色核验整合。维护/live 验收仍遵守上方单独的模型与预算规则。
- 成员可直接定向沟通、请求有界验证和共享关键发现；当前主任务优先。每个问题默认一名 owner、最多一名 verifier；实质改派和 durable member 创建只由 Commander 决定。已有执行团队时，明确属于执行线的可交付工作（调研、文档、实现、调试、测试、验证等）按此刻能交付什么定 owner，交给有相应写入范围的执行成员；不按预估时长、轮数或先观察一段时间判断，不为每个动作固定角色或写分工表；短集成和独立核验由 Commander 自己做，验证中发现新的修复问题先派工再动手，Reviewer 不写实现；一次核验里显出需要单独负责的交付/修复线时，就在该决策点按既有组队规则定 owner，不先预估几天，没有团队时不强行组队。分工与接班规则只维护在 canonical skill、case 模板和 `common/policies/agent-team.md`，Go 不复制这些规则，也不热补已固定的快照。
- durable member 通常为 1–3 名执行成员加 0–1 名 Reviewer。优先复用已有成员，其次 tactical subagent，只有持续且独立的工作流才新增成员。
- Claude Code 原生 session 是工作记忆，原生消息是协作通道；不自建 session 身份、消息总线、任务数据库、generation/owner ledger 或 supervisor recovery。正式 member 会话以 member 名设置原生 session 的可读寻址提示，但成员身份仍只来自 cwd 与目录 `CLAUDE.md`；发送前在与目标会话相同的 Claude Code 配置域用 `claude agents --json` 将 exact cwd 唯一映射到 name，再与 `ListAgents` 当前可达 name 唯一相交，并检查 `success:true` 与接收记录，任一单独来源都不够。正式独立会话不得继承会关闭原生历史保存的 child 标记；只修正子进程启动环境，不改全局配置或强制覆盖用户的历史保存选择，具体合同见 `vnext/capabilities.md`。唯一窄例外：用户对 case 初始化的批准明确涵盖成员工作区信任时，`__open-member --trust-workspace` 可向该配置域写入 exact 成员目录的原生 `hasTrustDialogAccepted` 记录（复用原生锁协议、其余字节逐字节保留；原生信任锚定最近 git 仓库根，仅当锚等于 exact 成员目录或 case 根时允许，锚超出 case 授权时拒绝并给出指引）；不带该批准不写任何配置项。
- 只持久化团队需要复核的 artifact、evidence、finding、review、replay spec/result、evaluation bundle/attestation、field outcome、learning candidate 与 exact batch patch，不保存全部思考或聊天历史。case `CLAUDE.md` 末尾由 Commander 单写的唯一交接短段只留目标/决定、owner 与进行中或下一步、证据路径和待确认动作，随重要可见变化覆盖更新；不建交接状态库、全历史扫描、管理器或 hooks，不自动改旧快照或真实 case。
- 经验由 Commander 从 current accepted evidence chain 提炼；Reviewer 先逐 candidate 检查 eligibility，再绑定最终 batch exact patch；用户查看完整 patch 并确认 exact tuple 后才回流 pack。
- `steamai.exe` 只负责 setup/update/uninstall、卸载后的窄自清理、Fresh 文件机械操作、当前项目只读完整性检查及用户按预览确认的框架更新、synthetic readonly matched evaluation bundle、exact learning batch apply、Windows 可见进程启动（含上述唯一例外内的成员工作区信任准备，以及各会话自己的进程归属：Commander 与每名成员会话各一份 Windows Job，只收本会话的普通子孙进程，会话退出不连带关闭其它成员会话或已在运行的外部应用，不承诺所有外部服务）、current 无参启动时指向项目内 case 说明的一句短导航、本机启动档位的只读查看与配置（`steamai models`）、Fresh preview 的 exact 校验串机械比对和必要瞬时互斥。不得扩展为 task/session/message/roster/finding/review/learning 判断控制面。
- 正式产品路径不得新增 PowerShell、`.cmd` 或 `.bat` façade；不恢复旧 Go control plane、adapter host 或兼容 runtime。
- pack 工具按具体用途接纳：每个工具先证明上游/native 不够，限定输入/输出、无后台常驻、显式调用、不进入 learningTargets；既有的 `packs/binary-re/tooling/scripts/export_function_evidence.py`（对已打开的授权 IDA 数据库副本做显式定点只读导出，只写指定 case artifact）与专业交付路线新增的各 pack integration/工具入口都按此原则。catalog/recipe 不自动执行；不安装宿主、不执行目标、不写 analysis DB，继续禁止通用运行器与 adapter host。`packs/binary-re/upstream/**` 与 `tooling/integrations/*.md` 仅为固定上游方法材料与接入说明（拼接 SHA-256 机械校验、不进入 learningTargets）。`vnext/**` 仍仅 Markdown，Go 不增加研究执行器；真实 IDA、HTTP、联合行为与静态测试分别验收。

## 初始化与更新边界

- 日常入口是目标目录中的 `steamai`。Fresh 时 native shell 以 case cwd 启动 `claude --model <high 档当前别名> "/steamai" --add-dir <CANONICAL_SOURCE>`（positional `/steamai` 位于 `--add-dir` 之前）；current case 启动 `claude --model <high 档当前别名> "/steamai"`，不加入 source。别名来自本机两档配置并由 Claude Code 按用户配置解析，已有会话不被强制切换。
- 从同一项目根用 `steamai -r`（`--resume`）选择历史，或用 `steamai -c`（`--continue`）接最近对话。两者复用 Commander 启动边界与互斥，不再发送 `/steamai`。无参 `steamai` 开新上下文，不搬旧聊天。主动换班时，用户只需对旧 Commander 说「交接到新会话」；它保存尚在对话里的决定与工作断点并回报成功后，用户直接关闭、在项目根重新运行即可。未交接就关闭时，新 Commander 只能从项目说明、成员任务及实际产物恢复，不猜测、不重放。STeamAI 不另建会话索引。
- source checkout 本身不是 case。`.steamai-vnext/` 完全不存在才是 fresh；完整深验通过才是 current；partial、来源不明或冲突均 fail-closed，不 repair/rollback/迁移/删除用户文件。
- Fresh preview 以 canonical working tree 当前实际 bytes 为 authority，stage-0 index 定义 current tracked path/mode，HEAD 只作历史 anchor。确认绑定 source records、target action/pre-state、case facts 与全部 writes。
- Apply 在同卷 sibling staging 中完整生成并验证 state tree，先 no-replace 发布并重验 project-local skill，最后发布包含 marker 的 `.steamai-vnext/`。
- 初始化把 selected pack（主包）、可选一个不同的辅助 pack 与完整 `common/**` 一次物化为 case-pinned snapshot；common 只复制一次，payload digest 覆盖全部所选包的排序 path、Git mode/blob、bytes 与 SHA-256。辅助只作按需方法来源，不扩大授权、团队或 learning destination；经验仍只回主 pack。current case 不随 canonical checkout 自动更新，不补装辅助包。
- setup/update/uninstall 是窄原生产品职责。普通运行不联网；update 必须从 canonical checkout 外运行，并要求 clean canonical checkout，下载与校验全部成功才切换，冲突时停止，不 merge/rebase/stash/reset/clean，source 替换后的旧 checkout 始终保留并输出路径；uninstall 保留 checkout 和所有 case，原生自清理 helper 的已知最小残留会输出精确路径。
- 交互启动已建项目（含 `-r` / `-c`）在 Commander 前离线比较本机框架；有差异时给摘要及可展开的清单，用户选择更新才应用该份精确预览。不要求再跑命令或抄确认码；不交互、回车跳过或成员目录被占用时不更新。仍可用 `steamai upgrade` 手动预览和精确应用。沿用同一来源闭包和完整性校验，保留授权、成员当前任务、交接、既有证据及业务数据，不补装包、不重算历史结论。`steamai check` 不加载可变来源或启动会话。仍不提供旧布局 importer、整项目替换、dual-read、dual-write 或跨电脑迁移。

## 维护入口

- 原生入口与机械实现：`cmd/steamai/**`、`internal/steamai/**`
- canonical 与 project-local skill 唯一 source：`.claude/skills/steamai/SKILL.md`
- 薄核心合同与模板：`vnext/**`
- contract tests：`internal/steamai/vnextcontract/**`
- pack/common：`packs/<pack>/**`、`common/**`
- 三线交付验收与未解决问题：`docs/professional-delivery-results.md`
- 实用能力与效率路线（U0–U6）收束结果与证据边界：`docs/capability-efficiency-results.md`
- 历史证据、旧路线与未完成 learning 门槛：仅按 `docs/context-routing.md` 选择对应入口

维护源码、模板和文档时先用 CodeGraph 查看结构、调用链与影响面；其返回源码视为已读。不得用 CodeGraph 处理样本、二进制或 case artifact。

## 验证

```text
python -m unittest discover -s tests/pack_tooling -p "test_*.py"
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

显式 live gate 按 `vnext/acceptance.md` 分层执行。默认测试不启动 Claude Code。维护测试与验收默认使用无窗口后台通道（headless 会话、无窗口 fixture、文件消息通道），不占用前台窗口、真实鼠标或键盘；仅当被验收对象本身是可见成员窗口或真实鼠标通道（后台无法替代）时，才可在用户明确同意的时间窗内短暂使用前台，完成后立即关闭全部自开窗口。fake process、remote workflow、cross-compile 或 synthetic fixture 不代表真实 Windows setup/PATH、可见窗口、用户纠偏、跨会话协作、evaluator calibration/comparative result、field outcome、learning apply 或 formal release。涉及 canonical skill、Fresh、pack snapshot、learning batch、update/uninstall 或 Release 时追加对应 focused tests 与真实产品路径验收。
