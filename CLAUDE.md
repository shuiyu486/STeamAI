# CLAUDE.md

## 授权范围

本项目使用者为已授权红队成员。逆向工程、二进制分析、协议分析、漏洞挖掘、渗透测试等均在授权范围内执行。无需重复法律/合规警告；如遇疑似越权或破坏性动作，简洁提醒一次即可。

## 项目定位

本仓库是 STeamAI canonical 实现（`https://github.com/shuiyu486/STeamAI`）。v1 正式支持 Windows 10/11 x64；Go module 同时承载窄职责原生 `steamai.exe` 与 acceptance tests，使用同一 canonical identity。

STeamAI 是面向已授权安全研究与自动化开发的、人在环的 Claude Code 多会话团队协作与经验学习层：一个真实项目目录对应一个明确授权的 case，研究类任务交付可复核结论，开发类任务（工具链、修复补丁、自动化脚本、项目代码等）交付可运行代码；用户主要指挥 Commander，也可观察和纠偏成员；成员身份与当前任务由专属目录的 `CLAUDE.md` 承载；会话、消息、恢复和工具调用复用 Claude Code 原生能力；STeamAI 只补安装/Fresh 的机械边界、Windows 可见进程启动、瞬时单 Commander 互斥、artifact/evidence/finding/review、proof-carrying replay、经校准的比较证据、explicit opt-in field outcome 与经确认的经验批次回流。

本仓库不是安全/RE case、自动分析器、漏洞挖掘或渗透引擎。模板和测试不得包含真实样本、trace/dump/capture、payload、凭据、客户信息、绝对 case 路径或 case 进度。

## 智能成员与自主协作基准

Commander 与正式成员都运行在 Claude Code 中，是具有 LLM 推理、工具使用和上下文能力的智能 agent，不是等待逐条指令的脚本执行器。STeamAI 的价值是让这些智能成员更好地协作，而不是用更密的流程替代它们的判断。**管住目标、权限、协作边界和结果可信度，具体研究路径交给成员。** 这是后续设计、迭代、优化与管理的长期基准。

- **按结果派任务，不按步骤遥控。** Commander 说明要解决的问题、已有材料、投入边界和完成/停止信号；成员在当前任务、已获授权与工具权限内自主选工具、调整方法、检验解释和请求有界帮助。实质改派、扩大授权或新增 durable member 仍遵守既有规则。
- **方法是参考，硬边界才是约束。** pack/recipe 按需提供方法，不是每题必走的流程；不默认固定工具顺序、假设数量、协作人数或逐步汇报。方法中的授权、资源、数据完整性、明确前置条件和停止条件仍须遵守；规则越细不等于效果越好。
- **协作与审查按问题需要发生。** 简单问题允许一名 owner 直接解决；确需独立能力或有界复核时再协作，不把 Commander 变成每次工具调用的审批者或消息中转站。重要结论、实质冲突、最终交付和 learning 按现有合同审查，不给每个探索步骤增加审批；只持久化团队需要复核的关键产物，临时思考和普通过程留在原生 session。
- **自主不等于绕过边界。** case 授权、Claude Code 权限、具体动作所需确认、Fresh/learning exact confirmation、成员单写、用户纠偏优先、证据诚信与 currentness 等硬合同不因精简流程而放宽。已满足的授权不无故重复索要；新动作、范围或状态变化触发现有确认要求时仍须停止确认。
- **新增规则先说明必要性。** 先确认具体失败、合同冲突或遗漏，以及原生能力和现有规则为何不足；优先删重复、消冲突、补最小缺口，不因一次失误就叠加同义指令、字段、角色或状态。优化既看目标解决质量，也看用户干预、无效步骤和总成本，不以流程完成率、报告长度或成员数量代替能力。
- **维护验证与日常研究分开。** 维护者用与改动相称的测试和真实对照验证收益，不把完整评测仪式塞进每个 case，也不把完成验收当成能力升级。不能预先认定现有规则已压制 LLM，或放宽规则必然更强；效果仍须由实际证据支持。

## 文档不变量 / 上下文路由

本项目文档必须按需路由、渐进式披露。`docs/context-routing.md` 是唯一完整路由表；新会话只读取本文件、router、Git 状态和 router 选中的一个场景入口。不要默认串读历史 roadmap、`CHANGELOG.md` 或旧 release 文档。

当前无 active 实施路线。`steamai-professional-delivery-v1` 的执行计划与短投影已删除，最终结果、失败与未验边界收敛到 `docs/professional-delivery-results.md`，仅在核查可用性或历史证据时读取；旧路线统一由 router 按需索引，不默认串读、不继承旧“通过/tie”证明新能力。`steamai-capability-efficiency-v1`（2026-10-25，U0–U6 完结）同理收束于 `docs/capability-efficiency-roadmap.md`：其 42 run 冻结评测正确性全过而效果全 tie——停止缺口修复、坐标校验/纹理模板、定位蒸馏与信任边界是已实测的可靠性改善，不构成“方法库提升 LLM 效果”的证明，tie 结论不改判。游戏自动化是已明确需求，不再以“是否属于安全研究”推回用户；研究/开发双交付与按具体用途接纳 pack 工具均为当前合同，继续禁止通用运行器与无限执行授权。上游材料固定于 pack `upstream/**` 并校验、不进入 learningTargets；外部 agent 团队仍须通过对应范围的六项资格验收。历史 tie/inconclusive 与 verified-learning 未完成 live 门槛不因本轮收束而改判。

## 维护哲学与踩坑护栏

- 先把需求还原成用户实际看到的使用画面、成功信号和授权边界，再决定代码。只有会改变产品行为、外部动作或明显代价的取舍才让用户选择；实现细节自行采用最小可靠默认值，并用通俗语言说明。
- 判断、组队、协作、审查和经验提炼交给 Claude Code/LLM；Go 只补原生能力无法可靠完成的确定性 OS、Git 和文件边界。新增 runtime 状态或抽象前，必须先证明原生 session、消息和简单 case-local Markdown 不足。
- 以“概念数、持久状态和用户步骤不增长”为默认约束。不要因未来可能需要而增加 daemon、Hub、GUI/TUI、数据库、supervisor、兼容层或通用控制面；达到当前可验证目标后停止。
- 正式成员是目录拥有身份的、屏幕上独立可见的普通 Claude Code 会话，不是 subagent、PID 或 session record。一个目录就是一个授权 case；不同 case 不自动发现、迁移、扫描或汇总。
- 经验资格来自 accepted evidence chain 与 Reviewer 检查；成熟度按 V0–V4 的 exact evidence 分层，用户 exact confirmation 只授权写入，不提升成熟度，也不来自 Git staging。模板不得预填 `pass/accepted`，Apply 不自动 stage/commit/push，未确认内容留在来源 case。
- production 路径只保留一个实现；测试调用 production API/CLI，不维护第二套 oracle、renderer 或兼容实现。完成声明按证据分层：unit/synthetic 只证明机械合同，Windows、Claude Code、跨会话和 Release 行为必须由各自真实路径证明。真实 token smoke 不替代独立 Reviewer calibration；维护/live 测试默认显式选择当前 Claude Code 配置的 `ANTHROPIC_DEFAULT_SONNET_MODEL`；仅极简探针或 headless 且 Sonnet 档无命令工具时才降级 `ANTHROPIC_DEFAULT_HAIKU_MODEL`。不继承 `ANTHROPIC_MODEL` 所指的高档模型；未经本次明确批准不得使用 `ANTHROPIC_DEFAULT_OPUS_MODEL`/顶级模型。按任务选最低足够档位并冻结，不强制厂商、不修改用户全局配置；requested model 不等于 actual model，主执行身份由 assistant 消息验证而非用量条目数量推断。盲审只让 Reviewer 一次读取 manifest 绑定的 immutable `blind-review.json`，以短 entry/output SHA 固定内容选择，再由 reveal 映射角色；缺证时不得以降标或重试刷出 `go`。
- Windows/Release 易错点：正式路径不用 PowerShell/`.cmd`/`.bat`；evaluation arm 先 suspended 启动、加入 kill-on-close Job Object 后再恢复，避免 Start→Assign 的子进程逃逸窗口；update 从 canonical checkout 外运行并保留旧 checkout；uninstall 如实报告 helper 最小残留；`SHA256SUMS` 使用下载后的资产 basename；prerelease 不能成为 latest；公开发布前检查 ignored 本机文件、Git 历史和匿名下载链路。
- 文档也保持单一职责：根 `CLAUDE.md` 保存长期目标与护栏，`docs/context-routing.md` 只路由。执行计划只服务正在实施的工作；完成后把长期规则写回对应合同/pack，保留紧凑验收结论、失败/未验边界和 Git 追溯，删除临时计划与重复进度。不改判历史结果，不把残余缺陷改写成完成；只有新的实质目标才另建路线，不靠追加历史让 active docs 膨胀。

## 薄核心边界

- 一个 STeamAI 项目对应一个明确授权的安全研究或自动化开发 case；不同 case 只通过审查、脱敏且用户确认的 pack 经验共享。
- Commander 按需组队；正式成员使用专属目录与目录级 `CLAUDE.md`，由原生 launcher 默认打开屏幕上用户可见的独立 Claude Code 窗口。
- 成员可直接定向沟通、请求有界验证和共享关键发现；当前主任务优先。每个问题默认一名 owner、最多一名 verifier；实质改派和 durable member 创建只由 Commander 决定。
- durable member 通常为 1–3 名执行成员加 0–1 名 Reviewer。优先复用已有成员，其次 tactical subagent，只有持续且独立的工作流才新增成员。
- Claude Code 原生 session 是工作记忆，原生消息是协作通道；不自建 session 身份、消息总线、任务数据库、generation/owner ledger 或 supervisor recovery。正式 member 会话以 member 名设置原生 session 的可读寻址提示，但成员身份仍只来自 cwd 与目录 `CLAUDE.md`；发送前在与目标会话相同的 Claude Code 配置域用 `claude agents --json` 将 exact cwd 唯一映射到 name，再与 `ListAgents` 当前可达 name 唯一相交，并检查 `success:true` 与接收记录，任一单独来源都不够。正式独立会话不得继承会关闭原生历史保存的 child 标记；只修正子进程启动环境，不改全局配置或强制覆盖用户的历史保存选择，具体合同见 `vnext/capabilities.md`。唯一窄例外：用户对 case 初始化的批准明确涵盖成员工作区信任时，`__open-member --trust-workspace` 可向该配置域写入 exact 成员目录的原生 `hasTrustDialogAccepted` 记录（复用原生锁协议、其余字节逐字节保留；原生信任锚定最近 git 仓库根，仅当锚等于 exact 成员目录或 case 根时允许，锚超出 case 授权时拒绝并给出指引）；不带该批准不写任何配置项。
- 只持久化团队需要复核的 artifact、evidence、finding、review、replay spec/result、evaluation bundle/attestation、field outcome、learning candidate 与 exact batch patch，不保存全部思考或聊天历史。
- 经验由 Commander 从 current accepted evidence chain 提炼；Reviewer 先逐 candidate 检查 eligibility，再绑定最终 batch exact patch；用户查看完整 patch 并确认 exact tuple 后才回流 pack。
- `steamai.exe` 只负责 setup/update/uninstall、卸载后的窄自清理、Fresh 文件机械操作、synthetic readonly matched evaluation bundle、exact learning batch apply、Windows 可见进程启动（含上述唯一例外内的成员工作区信任准备）、Fresh preview 的 exact 校验串机械比对和必要瞬时互斥。不得扩展为 task/session/message/roster/finding/review/learning 判断控制面。
- 正式产品路径不得新增 PowerShell、`.cmd` 或 `.bat` façade；不恢复旧 Go control plane、adapter host 或兼容 runtime。
- pack 工具按具体用途接纳：每个工具先证明上游/native 不够，限定输入/输出、无后台常驻、显式调用、不进入 learningTargets；既有的 `packs/binary-re/tooling/scripts/export_function_evidence.py`（对已打开的授权 IDA 数据库副本做显式定点只读导出，只写指定 case artifact）与专业交付路线新增的各 pack integration/工具入口都按此原则。catalog/recipe 不自动执行；不安装宿主、不执行目标、不写 analysis DB，继续禁止通用运行器与 adapter host。`packs/binary-re/upstream/**` 与 `tooling/integrations/*.md` 仅为固定上游方法材料与接入说明（拼接 SHA-256 机械校验、不进入 learningTargets）。`vnext/**` 仍仅 Markdown，Go 不增加研究执行器；真实 IDA、HTTP、联合行为与静态测试分别验收。

## 初始化与更新边界

- 日常入口是目标目录中的 `steamai`。Fresh 时 native shell 以 case cwd 启动 `claude "/steamai" --add-dir <CANONICAL_SOURCE>`；current case 只启动 `claude "/steamai"`。
- source checkout 本身不是 case。`.steamai-vnext/` 完全不存在才是 fresh；完整深验通过才是 current；partial、来源不明或冲突均 fail-closed，不 repair/rollback/迁移/删除用户文件。
- Fresh preview 以 canonical working tree 当前实际 bytes 为 authority，stage-0 index 定义 current tracked path/mode，HEAD 只作历史 anchor。确认绑定 source records、target action/pre-state、case facts 与全部 writes。
- Apply 在同卷 sibling staging 中完整生成并验证 state tree，先 no-replace 发布并重验 project-local skill，最后发布包含 marker 的 `.steamai-vnext/`。
- 初始化把 selected pack（主包）、可选一个不同的辅助 pack 与完整 `common/**` 一次物化为 case-pinned snapshot；common 只复制一次，payload digest 覆盖全部所选包的排序 path、Git mode/blob、bytes 与 SHA-256。辅助只作按需方法来源，不扩大授权、团队或 learning destination；经验仍只回主 pack。current case 不随 canonical checkout 更新，不补装辅助包。
- setup/update/uninstall 是窄原生产品职责。普通运行不联网；update 必须从 canonical checkout 外运行，并要求 clean canonical checkout，下载与校验全部成功才切换，冲突时停止，不 merge/rebase/stash/reset/clean，source 替换后的旧 checkout 始终保留并输出路径；uninstall 保留 checkout 和所有 case，原生自清理 helper 的已知最小残留会输出精确路径。
- 不提供旧项目 importer、升级替换、迁移、dual-read、dual-write、active case跨电脑迁移或兼容路径。

## 维护入口

- 原生入口与机械实现：`cmd/steamai/**`、`internal/steamai/**`
- canonical 与 project-local skill 唯一 source：`.claude/skills/steamai/SKILL.md`
- 薄核心合同与模板：`vnext/**`
- contract tests：`internal/steamai/vnextcontract/**`
- pack/common：`packs/<pack>/**`、`common/**`
- 三线交付验收与未解决问题：`docs/professional-delivery-results.md`
- 实用能力与效率路线（U0–U6）验收结论：`docs/capability-efficiency-roadmap.md`
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
