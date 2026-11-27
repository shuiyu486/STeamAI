# Claude Code 原生能力契约

## 必需能力

vNext 薄核心只假定：

- 用户能从指定成员目录启动一个普通、可见的 Claude Code 会话；
- 该会话自动加载启动目录及父目录的 `CLAUDE.md`；
- 用户能直接观察、输入、暂停和恢复该会话；
- 成员能读取被明确加入访问范围的 case 根目录。

成员启动目录是 `.steamai-vnext/members/<member>/`。Windows 10/11 x64 正式路径由 Commander 在 case 根调用：

```text
steamai __open-member [--trust-workspace] [--tier high|standard] <member-name>
```

原生入口验证成员目录与 `CLAUDE.md`，然后从该目录启动用户可见、可输入的普通交互式 Claude Code 会话。位于 WezTerm 时，按当前 `WEZTERM_PANE` 在同一窗口新开标签；标签承载独立正式成员，不是 subagent 或 split pane。不在 WezTerm 内但能找到它时打开新的 WezTerm 窗口；未安装时保留原生 console。已选 WezTerm 后失败就报告，不重试其它窗口或重复启动、不向已有标签发送输入。以 `--tier high|standard` 显式传档（成员默认 standard），Claude Code 实际收到该档位当前别名：`--model <所选别名>`，同时以 `--append-system-prompt` 携带本会话两档别名，供角色派发原生子 agent 时使用；以 `--name <member-name>` 给原生 session 一个可读的寻址提示，并以 `--add-dir <CASE_ROOT>` 加入 case 根访问范围；这条基础启动路径要求支持 `--name` 的 Claude Code 2.1.76 或更高版本。旧固定 case 的 skill 文本写的是 `--model opus` / `--model sonnet`：原生入口把它们当作历史的 high / standard 角色意图接受，仍解析到当前本机档位配置，绝不绕过用户的新选择。session name 不构成成员身份；发送前先在与目标会话相同的 Claude Code 配置域运行原生 `claude agents --json`，将 exact member cwd 唯一映射到实际 session name，再用 `ListAgents` 确认同名 peer 此刻可达，只有两边唯一相交才向该 name 发送。用户从窗口出现起即可观察、输入、暂停和纠偏；该入口不使用 `--bg`，不保存 PID/session ID，也不管理成员任务或生命周期。原生启动失败时才展示从成员目录执行 `claude --model <对应档位的当前别名> --name <member-name> --add-dir <CASE_ROOT>` 的手工 fallback。

`--trust-workspace` 是内部显式选项，只在用户对本次 case 初始化的批准明确涵盖成员工作区信任时由新版 canonical skill 传入：原生入口在全部成员校验通过后、打开窗口前，向当前配置域的 Claude Code 项目记录写入该 exact 成员目录的 `hasTrustDialogAccepted: true`，用户无需逐个点击信任框。该写入只发生在校验通过的 exact 成员目录上：复用与原生一致的 `.claude.json.lock` 目录锁和原子替换，其余 JSON 内容逐字节保留，已信任时零写入。原生对 git 仓库内目录的信任锚定最近 repo root，因此只有两种锚被允许——exact 成员目录与 case 根（case 建在仓库根，范围恰等于本次 case 授权）；case 位于仓库子目录（锚超出 case）或成员与 case 根之间存在嵌套仓库时，该成员启动以具体原因停止并列出可执行下一步，不降级弹框、不扩大信任、不静默重试。配置畸形、存在重复/大小写别名键或锁持续被持有时同样停止。不带该选项的调用保持原有行为，不为旧 case 静默新增信任。

正式 Commander/member 启动仅在新进程环境中移除 `CLAUDECODE` 与 `CLAUDE_CODE_CHILD_SESSION`：后者可能由 Commander 的 shell 工具继承，若带入独立交互会话会令 Claude Code 关闭 transcript 保存。不得用全局修改或强设 `CLAUDE_CODE_FORCE_SESSION_PERSISTENCE` 替代这条边界，也不覆盖用户其它明确的历史保存选择。

会话内打开成员窗口时，有既有 WezTerm mux 就由其 server 开标签；原 mux 定位丢失时明确报错，不退回会话进程组内开窗。没有 mux 时，由该会话自己的 STeamAI 启动器代开。此入口只在会话存续期间接受本机请求，只收项目物理标识、已声明的成员名及档位。启动器按自己绑定的项目复查成员，使用本机模型偏好与父侧配置域；请求不能指定命令、路径、环境、信任写入或新的授权。发出请求后没有收到回执，不自动再开一次。

WezTerm pane 由 server 派生，不能假定继承 Commander 的临时 shell 环境。两条可见路径统一启动内部 `__member-session --config-dir <目录或-> --tier <high|standard> --high <别名> --standard <别名> <成员>`，helper 在 pane 内复用成员校验、清理上述标记，再前台执行 Claude Code；不嵌套开终端，不写信任或其它配置，不接受任意 exe/env 载荷。档位与两档别名由父启动器验证并冻结后显式传入，pane 内不从 WezTerm server 环境或磁盘重新取值。`-` 明确清除 server 的 `CLAUDE_CONFIG_DIR`、使用默认配置域；非默认域传绝对目录，仅影响 Claude 子进程。不把 token、完整环境或设置文件内容放进命令行。用户在 Claude 配置中定义的模型槽位继续生效；临时 shell 的其它变量不保证跨 WezTerm server 继承。

STeamAI 用本机两档配置决定角色别名：`steamai models` 只读查看当前高档与常规档；`steamai models --high <别名> --standard <别名>` 配置本机（设置时两个参数必须同时提供，各项只接受 `opus`、`sonnet` 或 `haiku`，允许同档），写入 `%LOCALAPPDATA%\STeamAI\models.json`，只对新启动的 STeamAI 会话生效。没有该文件时默认 high=opus、standard=sonnet，可用显式命令恢复，或删除该文件回到默认；文件畸形时报错，不悄悄回退默认值。Commander 用 high 档，关键研究/审查优先 high，普通成员与短子 agent 默认 standard；成员由负责人按任务选档。别名仍由 Claude Code 按用户的 `ANTHROPIC_DEFAULT_OPUS_MODEL` / `ANTHROPIC_DEFAULT_SONNET_MODEL` 等配置解析为实际模型，CLI 选择优先于 `ANTHROPIC_MODEL`，不在 Go 中解析厂商 ID；STeamAI 不重映射这些变量、不修改全局模型配置，也不影响本机其它普通 Claude 会话或旧 case 的固定 snapshot。已有会话不被强切，用户选择、预算与服务限制优先；请求档位不等于网关实际返回模型，高/常规是选档不是计费硬上限。角色派发原生子 agent 时须在原生 Agent(model=...) 显式传已解析的别名（默认 standard 别名，复杂反驳可用 high 别名），不把 `high` / `standard` 当作 Claude 模型名，也不通过篡改子 agent 全局环境或自建 runner 强制档位；各角色短任务每批最多 5 个且不嵌套，维护/live 验收按独立预算选档。

`--add-dir` 只表达文件访问范围，不用于切换成员身份或加载兄弟目录规则。Fresh Commander 仍从目标 case cwd 启动；`claude --model <high 档当前别名> "/steamai" --add-dir <CANONICAL_SOURCE>` 会从 added directory 发现 `.claude/skills/steamai/SKILL.md`，且 positional `/steamai` 必须位于 variadic `--add-dir` 之前；current case 同样以 high 档当前别名启动 `/steamai`，不加入 mutable source clone。current 的无参启动在 `--append-system-prompt` 里除两档说明外只附一句导航：先读项目内已有的 `.steamai-vnext/CLAUDE.md` 及其指向的入口，按其中的协作与接班约定行事；分工与交接规则不由 Go 注入，`-c` / `-r` 续聊和成员启动也不带这句。已有项目交互启动（含 `-r` / `-c`）先做本地比较，有差异才问是否更新；确认绑定刚展示的这份预览，核验成功后按原入口打开。默认回车、非交互输入或成员仍占用时不更换框架。`steamai upgrade` 仍可单独手动预览、确认；`steamai check` 仅核验当前内容，二者都不启动会话。项目共享规则已位于成员启动目录的父层级。

从原项目根运行 `steamai -r` / `--resume` 打开 `claude --resume` 原生历史选择器；`steamai -c` / `--continue` 用 `claude --continue` 接最近一次对话，两组开关均不接额外参数。它们沿用普通 Commander 的 cwd、单实例保护和当前 high 档启动参数（含两档系统说明），不发送 `/steamai` 或其它用户消息。Fresh 仍加 `--add-dir <CANONICAL_SOURCE>`，current 不加，partial 拒绝。历史属于原生 Claude 配置域，可能包含同目录的普通对话；STeamAI 不扫描、汇总或保存会话列表。无参 `steamai` 开新上下文，不继承旧聊天。主动换班先对旧 Commander 说「交接到新会话」，等它确认把尚在上下文的关键信息写入项目后，直接关闭、从原目录重新运行；不必 `/exit` 或 `/clear`。未交接就关闭时，只能按已保存的状态恢复。写什么、读什么由项目内的 skill 和 case 说明约定；原生入口不保存或拼接聊天。Windows 启动器与直属 Commander 共同持有 Commander 单实例锁；工具子进程不继承，直属 Commander 退出即释放，成员会话不持有它。旧版遗留锁不能靠删文件解除，也不按进程名批量结束或强开第二个写者。不要选其它项目或成员的记录；无参 `steamai`、`-r`、`-c` 都不能从 `.steamai-vnext` 内启动 Commander，框架源码目录及其子目录也不能用于开新项目。恢复成员仍按下文从 exact 成员目录操作，并重新传入 `--add-dir <CASE_ROOT>`。

每个由 STeamAI 启动的 Commander 或正式成员都有独立的 Windows Job。直属 Claude 在运行前已进入该组；会话工具不能自行脱离。直属 Claude 退出或其启动器被关闭时，Windows 清理仍在该组里的进程；不扫描父进程树，也不按进程名结束程序。正式成员从组外启动，有自己的 Job；原已运行的外部应用不由 STeamAI 关闭。系统服务、计划任务、其它宿主另行创建的进程不在回收范围。用户自己关闭含多个标签的整扇终端窗口时，标签仍会随窗口关闭。Job 不增加工具权限，也不是动作授权。

Windows 10 实测：旧的允许脱离版本曾留下真实 Git Bash `tail`；严格 Job 版本中，真实 Claude Code 经 Bash 读取合成日志，强制结束直属 Claude 后，同次固定句柄的 `tail.exe` 连续三次均退出（本地模拟模型响应）。另在两个不同 pack 的临时项目中，用真实 Windows 伪终端与后台 CLI/终端替身验证：关闭一个 Commander 后自己的工具退出，同目录一次重开成功；经 `__open-member` 打开的独立成员和另一项目不受影响。未在本轮重验前台成员窗口的键盘输入，也不据此推断所有外部宿主的行为。

主辅 pack 不改变上述成员模型。每个成员仍从自己的目录启动，读取自己的 `CLAUDE.md` 与父级 case 规则；辅助只是任务指定的 pinned 方法文件，不是额外配置根、第二个成员身份或新的记忆系统。新增方法和模板随后续 Fresh 固定；已有项目可按预览更新原来选定的包，但不自动补装其它包。

## 可选能力

- `ListAgents` / `SendMessage`：发现和联系可达的独立 Claude Code 会话；不是 durable、exactly-once 消息队列。Windows 原生消息 live gate 使用明确发布该能力的 Claude Code 2.1.248 或更高版本；更早或未暴露 peer address/ListAgents 的版本按“无跨会话消息”降级。当前 `ListAgents` 输出不承诺包含 cwd，因此发送方先在与目标会话相同的 Claude Code 配置域用 `claude agents --json` 取得 exact cwd→name 唯一映射，再要求该 name 在 `ListAgents` 中恰好可达；inventory 单独不能冒充可达 peer，`ListAgents` 单独也不能证明成员目录身份。发送结果还必须明确 `success:true`，并在接收方 transcript 中出现对应 incoming record，任一缺失都不能声称送达。后台自动验收若需要无人值守接收跨会话消息，必须在临时 session settings 中显式设置 `crossSessionInbound: "accept"`；产品默认不全局修改用户设置。
- 本机 peer 的一次性空闲通知是**可选项**：以当前 `SendMessage` 是否暴露 `notify_when_idle` 为准，不由消息最低版本推断。主会话可发送时一并订阅，或省略 `message` 只订阅。对已经空闲的成员，通知也可能指向此前结束的同一轮，不代表刚有新进展；空闲、退出、订阅到期都不是任务回执。它不提供定时心跳、可靠离线投递或进程重启。通知可能需用户批准才可见；不为收通知放宽权限。如何避免死等、何时只提醒原成员一次，见项目固定 skill 的「等成员回报」。
- Agent view / `claude agents --json --all`：查看当前可观察的交互式或后台 session；`--cwd` 只限定后台列表，不能据此假定过滤全部 foreground session，发送方仍须核对每条 row 的 exact `cwd`。普通 foreground session 不保证一定出现在 Agent view；未观察到只能标记 `unknown`，不能推断 offline/completed。它只为上述即时发送提供临时身份交叉核对，不持久化为 session registry。
- `claude logs <id>`：查看后台成员最近终端输出；`claude attach <id>`：进入后台 session 并直接纠偏；`claude respawn <id>`、`claude --resume <session-id>`：恢复已有独立 session。resume 时必须重新传入当前 case 的 `--add-dir <CASE_ROOT>`，不能假定 launch-only access flag 自动恢复。只有用户直接输入或同一 session 的明确 resume/attach input 才验证 direct correction；跨会话 `SendMessage` 不能冒充用户纠偏或授予任务变更权限。同一 member cwd 若观察到两个可写 session，agent 任务改写必须 hold，等待用户直接选择。
- `claude --bg`：用户不需持续观察时的可选后台模式，不是默认。
- tactical subagent：正式成员内部的窄任务，不成为 durable member。

## 外部专业工具接入（按路线启用）

- 专业目标工具默认在 exact 成员目录注册 local-scope MCP（同一 Claude 配置域、固定版本、经用户逐项批准）。跨项目共用的 Windows 桌面能力有一个明确例外：用户批准具体包、版本、工具白名单和配置域后，可通过 Claude Code 原生 `claude mcp add --scope user` 注册，供该配置域所有项目使用；不由 Fresh 或启动器暗中安装/注册。基础入口见 `common/tooling/windows-desktop-mcp.md`。`--add-dir` 不会带入 MCP 配置，不能拿它替代 scope 选择。`steamai.exe` 不因此获得配置管理职责，Windows 正式路径也不新增 PowerShell、`.cmd` 或 `.bat` 包装。
- 非 MCP 形态的专业工具（Python 库、官方 CLI 等）以固定版本的 pack integration 说明接入：使用方在获准环境自行安装/调用，工具调用是任务里的显式动作；STeamAI 不代装、不托管进程、不维护工具配置管理器。
- pack 工具按具体用途接纳：新工具须先证明上游/native 能力不足，限定输入/输出、无后台常驻、显式调用，并作为 pack 明示入口进入 case；工具脚本不进入 learningTargets。继续禁止通用运行器、adapter host 或第二套执行控制面。
- local scope 只决定工具在该目录会话中加载，不是文件或网络隔离；具体动作仍受工具权限、针对该动作的用户确认与 case 授权约束。
- pack 固定的是方法材料与接入合同，不是运行环境；实际工具、provider 与目标身份须在执行前核对并记录到 evidence，自检通过不能代替真实连接成功。
- 上游工具的证据标识（如 REA `evidence_id`）只在单一 MCP 连接内有效；跨会话/成员只交接完整导出产物。
- 外部自带 agent 团队（如 Strix）不是 durable member，也不是 tactical subagent；未通过有界外部执行资格验收（范围、停止、费用、纠偏、产物、模型六项）前不自主运行。
- 降级规则不变：外部工具缺失或版本不符时如实报告具体缺口；不得回退旧 Go control plane，也不以较弱流程冒充同等交付。

## 降级

- 无跨会话消息：Commander 给出定向消息，用户在目标成员终端输入；成员目录与研究文件仍是共享面。
- 无后台 session：始终使用可见前台会话。
- 原会话不可恢复：从同一成员目录启动新会话，读取最新成员 `CLAUDE.md` 与共享研究产物。
- 无 Agent Team experimental 功能：不受影响；vNext 不依赖 teammate task list。
- 原生能力缺失时不得回退旧 Go control plane、外部 kit、PowerShell、`.cmd` 或 `.bat` runtime；Windows 原生 `steamai.exe` 只承担已声明的安装、确定性文件操作与可见会话启动边界。

## 验收入口

自动 capability/context/file-access probe 和真实独立 session live acceptance 的命令、场景与通过标准见 `vnext/acceptance.md`。自动 probe 不能证明用户直接观察/纠偏或成员直连，live acceptance 也不能替代可重复的 context/file-access 合同。

## 非承诺

- 不保证消息顺序、永久离线投递或 exactly-once；
- 不把 session ID、session name、PID 或 endpoint 当成员身份或授权；
- 不保证长期无人值守运行；
- 会话 Job 只覆盖该会话的普通子孙进程，不承诺收走所有外部服务；STeamAI 不因会话退出连带结束成员会话或已在运行的外部应用；
- case 说明里的交接短段只是导航：新上下文不自动带上旧聊天，没落到项目里的决定不保证接得上；原生历史仍由 Claude Code 管理，短段也不是授权或验收证明；
- 不解析 Claude Code transcript 内部 JSONL 作为产品数据库。
