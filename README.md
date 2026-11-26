# STeamAI

STeamAI 是面向已授权安全研究与自动化开发的、人在环的 Claude Code 多会话团队协作与经验学习层。一个真实项目目录对应一个明确授权的研究或开发 case：研究类任务交付可复核结论，开发类任务（工具链、修复补丁、自动化脚本、项目代码等）交付可运行代码；用户主要指挥 Commander，也可以随时观察、暂停和纠偏屏幕上独立可见的正式成员会话。

Commander 与正式成员都是运行在 Claude Code 中的 LLM 智能 agent。STeamAI 按结果组织协作：讲清目标、权限和责任，让成员在边界内自主选择研究方法，而不是逐步遥控；简单问题不强行多人参与，方法按需参考，关键结论按现有合同复核。自主性不取消具体动作确认、文件保护和证据要求。后续设计与迭代以根 [CLAUDE.md](CLAUDE.md#智能成员与自主协作基准) 中的长期基准为准。

当前源码按此基准调整任务与角色合同：当前任务内可自主换方法，已满足且边界未变的同一确认不重复索要，有界协作不逐次请示；最终综合审查仍须回答原问题。新合同只由后续 Fresh 固定，已有 case 不升级。逆向、Web 安全与游戏自动化开发三线已有受控任务交付；专业工具（IDA/idalib、Playwright、Airtest 等）从 pack 入口按具体用途接纳。[专业交付验收记录](docs/professional-delivery-results.md) 保存最终证据与未解决问题：小样本效果对照质量平局（tie）为主，不宣称研究增益；游戏线同步定位期间停止缺口尚未修复，不作为无人值守或实时控制保证。执行计划已移除，日常使用不需要读旧路线。

Canonical repository：[`shuiyu486/STeamAI`](https://github.com/shuiyu486/STeamAI)。当前正式 Release 为 [`v1.0.4`](https://github.com/shuiyu486/STeamAI/releases/tag/v1.0.4)；v1 正式产品路径支持 Windows 10/11 x64。**版本区别：本文的三线交付、新合同及后续主辅包能力描述当前源码，不表示下载 `v1.0.4` 已包含这些改动；这轮源码尚未发布新 Release。**

## 安装

前提：本机已经安装并登录 Claude Code（成员 `--name` 基础启动要求 2.1.76 或更高版本；Windows 原生跨会话消息要求 2.1.248 或更高版本），且有原生 Git for Windows。

1. 从 GitHub Release 下载 `steamai-windows-amd64.exe`、`steamai-release.json` 与 `SHA256SUMS`，先按 `SHA256SUMS` 核验两个文件，再确认 manifest 中的 exe SHA-256 与实际 exe 一致。
2. 将 exe 放到临时目录并运行：

```text
steamai-windows-amd64.exe setup
```

setup 默认把 canonical checkout 放到 `%LOCALAPPDATA%\STeamAI\source`，把原生 `steamai.exe` 安装到 `%LOCALAPPDATA%\STeamAI\bin`，并只为当前用户加入 PATH。也可以绑定已经 clone 的普通 Git checkout：

```text
steamai-windows-amd64.exe setup --source <SOURCE_CLONE>
```

重新打开终端后即可使用。STeamAI 不安装 Claude Code、不管理登录、不启用全局 plugin，也不使用 PowerShell、`.cmd` 或 `.bat` 产品脚本。

## Quickstart

在明确授权的目标项目目录运行：

```text
cd <CASE_ROOT>
steamai
```

`steamai` 会启动当前目录中的 Commander Claude Code，并自动进入 `/steamai`：

- `.steamai-vnext/` 完全不存在：连续完成目标与授权澄清、初始组队、零写入 exact preview、用户确认、Fresh 创建和可见成员窗口启动；
- 完整有效的 `.steamai-vnext/`：继续当前 case，复用仍运行的 active 成员，按需重开未完成成员；
- partial、来源不明或冲突：fail-closed，不自动 repair、迁移、覆盖或删除。

**接着上次聊**：回到原项目目录，在终端里选一条命令，不用先打开聊天窗口。

| 想做什么 | 输入 |
|---|---|
| 从历史列表里选一段对话 | `steamai -r`（`steamai --resume`） |
| 直接接着最近一次聊 | `steamai -c`（`steamai --continue`） |
| 忘了命令，看帮助 | `steamai -h` |

如果原 Commander 窗口还开着，直接回到那个窗口。历史列表由 Claude Code 提供，可能也有同目录的普通对话；它不是所有成员的聊天汇总。

**做到一半，想换个新上下文继续**：在旧聊天里说 **「交接到新会话」**。等 Commander 回复「交接已保存」并给出位置，再像平时一样关闭旧标签，在原项目目录运行 `steamai`。不需要 `/exit`、`/clear`，也不用把摘要复制给新会话。

旧 Commander 会把刚讨论但还没写进文件的决定、已排除的方向、工作断点和下一步，补进 `.steamai-vnext/CLAUDE.md` 末尾的「Commander 交接」；没有验证的事情不会写成完成。新 Commander 从这里找到负责人和对应产物，不需要继承整段聊天。

**如果已经直接关了**：也能运行 `steamai`，但这是从已保存的项目状态恢复，不会自动带上旧聊天。交接段缺失或过时，就核对成员任务和实际文件，不能确认的事不猜、不重做。若确实需要只在旧聊天里的决定，且原生历史还在，可用 `steamai -r` 选回那段对话，再说「交接到新会话」。

**关掉哪个会话，就清理哪个会话的工具。** 新版用 Windows 进程组为 Commander 和每名正式成员分开收尾。在 Windows 10 上实际验证过：真实 Claude Code 用 Git Bash 的 `tail` 等候日志时，结束该 Claude，`tail` 也退出；另用两个临时项目、真实 Windows 伪终端和后台 CLI 替身验证：直接关闭第一个 Commander 的终端，无需 `/exit` 就能在原目录立即重开，独立成员和第二个项目仍在运行。

STeamAI 不会关闭你原本开着的外部应用，也不接管由系统服务或其它宿主另行启动的程序。如果你关的是装着多个标签的整扇终端窗口，其中的标签仍会一起关闭。此修复只对新版入口启动的会话生效；旧工具若仍占锁，不要删项目文件或批量结束同名进程。

Fresh 来源是 setup 绑定的 canonical checkout 当前 working-tree bytes；stage-0 index 定义 current tracked path/mode，HEAD 只是历史 anchor。因此已经审查、用户确认并应用但尚未 commit 的本机经验，也能供后续新 case 使用。preview 绑定 case facts、source records、目标 pre-state 和全部 writes；只有 `CONFIRM STEAMAI FRESH <identity>` 才能 Apply。

Apply 在同卷 sibling staging 中生成并验证完整 state tree，先 no-replace 发布并重验 project-local skill，最后发布包含 marker 的 `.steamai-vnext/`。case 建立后固定读取自己的主 pack、可选辅助 pack + 完整 `common/**` snapshot，不随 mutable canonical checkout 漂移。

普通项目只运行 `claude` 时，不会加载或携带 STeamAI；另行明确启用的 user-scope MCP 属于 Claude Code 用户配置，会在同一配置域的其它项目可见。

### 终端、模型和桌面操作

- **终端**：成员优先开在主会话所在 WezTerm 窗口的新标签页，不另起一堆 console；不在 WezTerm 内但已安装它时开新 WezTerm 窗口，未安装才保留原生 console。启动失败如实报告，不向已有标签发输入、不重复启动。标签仍是独立、可输入和纠偏的正式成员。
- **模型**：本机两档配置，只对新启动的 STeamAI 会话生效，已有会话不强切：

  | 操作 | 命令 |
  |---|---|
  | 查看当前设置 | `steamai models` |
  | 高档用 Sonnet、常规档用 Haiku | `steamai models --high sonnet --standard haiku` |
  | 恢复默认 | `steamai models --high opus --standard sonnet` |

  两项各选 `opus`、`sonnet` 或 `haiku`，也可以选同一个。

  Commander 与关键研究/审查用高档，普通成员和短子 agent 默认常规档；高/常规是选档不是计费硬上限。别名仍由 Claude Code 按用户 `ANTHROPIC_DEFAULT_*_MODEL` 配置解析为实际模型，STeamAI 不重映射这些变量，不改变本机其它普通 Claude 会话或已有 case 的固定 snapshot；旧 case 文本里的 `--model opus` / `--model sonnet` 按高档/常规档的角色意图解析。完整合同见 [`vnext/capabilities.md`](vnext/capabilities.md)。
- **子 agent**：各角色鼓励用显式常规档子 agent 分担短任务与大文件筛选（在原生 Agent(model=...) 传已解析的别名，不把 `high` / `standard` 当模型名），每批最多 5 个且不嵌套；主角色保留判断和整合，不把大段搜索结果搬回上下文。
- **桌面 MCP**：跨 pack 的基础入口在 [`common/tooling/windows-desktop-mcp.md`](common/tooling/windows-desktop-mcp.md)。固定 Windows-MCP 0.8.9，关闭遥测，只暴露截图、键鼠等 9 项必要工具；通过 Claude Code 原生 user scope 一次接入，而非为每个软件造脚本。首次下载/注册与前台测试分别需要具体批准；仅复制入口不代表已安装或可控。它不能保证操作所有游戏、提权窗口或 UAC 界面，也不取代大漠、Airtest、Playwright。

WezTerm 新标签继承 server 的环境，启动器会保留 Claude 配置域并清理嵌套会话标记，但不传递全部临时 shell 环境或把 token 放入命令行。模型别名到实际模型的映射仍由 Claude Code 用户配置（`ANTHROPIC_DEFAULT_*_MODEL`）解析；STeamAI 的两档选择（`steamai models`）只决定各角色用哪个别名，不重映射这些变量。这轮包含原生 Go 改动，需要新版 `steamai.exe`，仅更新文档不会改变已安装旧入口；已有 case 的固定 snapshot 不自动更新。新版入口用无参 `steamai` 进入已有项目时，会附带一句短导航，让 Commander 先读项目内已有的 `.steamai-vnext/CLAUDE.md` 及其指向的入口；`-c` / `-r` 续聊和成员会话不带这句。协作与接班规则仍只维护在 canonical skill 和 case 模板里：已有项目的固定内容不迁移、不改写，也不会因此获得新规则。已打开的 Claude Code 会话不保证热发现新 MCP；需要时先正常退出，再从原目录 resume 同一会话，不在旧会话仍运行时另开同目录 writer。

本机已通过 WezTerm 同窗新标签的真实两轮交互，以及 Windows-MCP 对自建窗口的裁剪截图、一次点击和短文本键入；测试窗口均正常关闭。这些历史验收对应当时的固定 `opus`/`sonnet` 参数路径；本轮两档映射（`steamai models`、`--tier` 启动）的真实网关与前台验收尚未执行。环境、首次失败原因及未验范围见[终端与桌面验收记录](docs/windows-desktop-results.md)，不代表任意游戏或所有 DPI 配置已通过。

### 游戏自动化怎么选工具

大漠与 Airtest 都保留，按任务选择，不是二选一淘汰：Airtest 适合沿用 Python 图像自动化与现有参考工程；明确选择大漠或已有大漠字库/脚本时，先评估复用，不要求重做成 Airtest。普通任务不默认两套工具同时操作同一窗口。

在独立项目的 Commander 对话中，可以直接说：

> 这个项目使用大漠＋C++/MFC，不再依赖按键精灵。从内置基础工程开始，先核对我的大漠版本和编译环境，再开发具体功能；不自动升级大漠或运行旧脚本。Airtest 继续保留。

大漠入口见 `packs/game-automation/tooling/integrations/dm.md`：

- **离线查接口**：`upstream/dm-7.2520/README.md` 收录 42 个常用接口和线程/字库限制，不需要安装 `dm-api`。
- **换新版文档**：`tooling/dm-docs-update.md` 提供可运行的 CHM 比较/导入命令；新旧版本分开存，变更页需检查，不会自动替换 DLL 或旧项目资料。
- **脱离按键精灵**：`tooling/examples/dm-cpp-mfc/README.md` 提供实际 C++/MFC 源码、CMake、开始/暂停/停止、日志、SDK 版本探针、注册码许可（密码框、private stdin）与自建目标测试，交付 `dm_console` / `dm_worker` / `dm_demo_target` 三个 Win32 EXE。真实链路（MFC 开始按钮 → `Ver` → `Reg` → 绑定 → 取色 → 点击/按键 → 解绑 → 目标关闭）已在非活动桌面的自建目标上实测通过，界面/执行进程正常退出，用户输入桌面不切换；准确边界见工程 README。具体游戏业务不在基础工程内，也不迁移既有游戏。

以上相对路径均位于 `packs/game-automation/`。新 Fresh 会带上资料和源码，已有 case 不自动补装。接入不捆绑 DLL、不自动注册/安装；真实脚本、字库、凭据和绝对 case 路径不进入源码仓库。

## 研究方法与主辅 pack

当前源码路线同时深化两类方法：Binary RE 的低样本 final-effect/证据复核，以及 Web/API 的单一请求假设、有界验证、差异归因与修复证据。重要研究分岔点先选能改变判断的检查；反证出现后停止、收窄或改派，最终按用户问题说明已证明、被否定与未知。它们不新建成员记忆或调度系统。该方法与主辅包路线已完成四组、32 轮真实静态对照并独立复查，质量评分全部平局；方法已落地，但不能据此宣称研究更准或稳定更快。

Fresh 默认选一个主 pack，确需另一套独立维护的方法时，可在同次 preview 加入最多一个不同的辅助 pack。两个包分别固定、完整 common 只复制一次；成员按任务读取指定入口，不默认把两包全部加载。辅助不扩大授权或团队，不覆盖主 case 规则，经验仍只写回主 pack。主辅不是两个 case 合并，也不自动发现其他项目。

已有 current case 保持自己的单包内容、skill 和合同，不补装辅助或迁移；这些源码改动不会自动进入既有 case。新版原生入口沿同一 snapshot parser 接受无辅助字段的单包 case；旧 executable 不承诺读取新双包 snapshot。这轮方法与主辅能力的交付证据见 `docs/research-capability-roadmap.md`，不能用已发布 `v1.0.4` 的历史验收证明这些新行为。

## 专业实操与联合研究

`steamai-research-execution-v1` 路线在上述基础上交付三项相互配合的能力：A 将专业方法落实为实际取证步骤；B 核查处理步骤之间的连接，并用尚未查看的分支检验预测；C 在同一授权 case 内对应客户端字段、API 请求/业务结果与客户端处理，形成整体复核结论。已验事实与原对照未完成结论见 `docs/research-execution-roadmap.md`，不沿用旧平局对照证明新能力。该轮研究效果与 Reviewer 判别能力的验证结果见 `docs/research-validity-roadmap.md`；验证结果不自动授予 learning 晋级资格，也不证明当前自主协作优化的收益。

Binary 路径提供一个显式 IDAPython 定点导出工具，读取已打开的授权 Windows x64 稳定数据库副本，只写指定 case artifact；需要用户已有可用 IDA/IDAPython，不自动安装、不执行目标或修改 analysis DB，也不默认要求反编译器。Web 路径直接使用本机原生 curl，仍是具体确认后的单次 loopback GET，不新增 HTTP 执行器、POST、批量或 retry。Fresh 固定这些文件不会自动运行工具；脚本不进入 learningTargets。

跨步骤和联合方法只在相应问题上按需读取。综合 finding 直接引用全部支撑连接的 evidence，并整体审查；两个局部 accepted 不能自动合成联合 accepted。已有 case 不更新固定内容，真实 IDA/HTTP、可见协作和研究效果分别验收。

## 团队模型

- **Commander**：理解目标与授权、按需组队、解决冲突、组织审查、集成交付和发起经验回流。分工看要交付什么，不猜工期。已有团队时，报告、实现、修复等能独立交付的工作优先交给已有负责人，上一项任务结束的成员可以重新派工。例如，Commander 可以检查集成结果；发现问题需要查因、修改和回归时，就在动手前明确执行成员。没有团队时，简单问题可以自己解决，不为每个动作组队。
- **正式成员**：每名成员拥有 `.steamai-vnext/members/<name>/CLAUDE.md`，身份与当前任务属于该目录，不属于 session ID。Commander 通过原生 launcher 打开用户可见、可输入的普通 Claude Code 会话（WezTerm 同窗新标签优先），并以成员名设置原生 session 的可读寻址提示；发送前在同一 Claude Code 配置域中取得 `claude agents --json` 的 exact cwd→name 映射，并与 `ListAgents` 的当前可达 name 唯一相交，不能靠任一单独来源认定成员。
- **Reviewer**：只读 artifact/evidence/finding/spec/run/candidate/patch，只写 `reviews/` 与任务指定的 exact evaluation attestation，不执行 heavy action或运行 arms。
- active team 默认最多 3 名执行成员 + 1 名 Reviewer；每个问题一名 owner、最多一名 verifier。
- Claude Code 原生 session 是工作记忆，`ListAgents` / `SendMessage` 是协作通道，原生 logs/attach/resume/respawn 只作观察与恢复。独立 Commander/成员启动会移除继承的 nested/child 会话标记，避免被 Claude Code 当成不保存 transcript 的嵌套会话；不改用户全局配置。Windows 原生跨会话消息 live 验收使用明确支持该能力的 Claude Code 2.1.248 或更高版本，并同时验证发送结果与接收记录。STeamAI 不自建 task/session/message registry、队列或 supervisor。
- 用户在成员窗口里的直接输入优先；跨会话消息不能冒充用户纠偏、改派正式任务或扩大 case 授权。
- 同一 case 同时只允许一个 Commander；重复启动会拒绝第二个。

## 研究产物

```text
.steamai-vnext/
  CLAUDE.md
  pack-snapshot/
  contracts/
  members/<member>/CLAUDE.md
  artifacts/index.md
  evidence/E-*.md
  findings/F-*.md
  reviews/R-*.md
  reviews/R-L-*.md
  reviews/R-LB-*.md
  learnings/candidates/L-*.md
  learnings/patches/LB-*.patch
  evaluations/specs/*.md
  evaluations/specs/<suite-spec>.json
  evaluations/runs/<run-id>/manifest.json
  evaluations/runs/<suite-manifest>.json
  evaluations/attestations/*.md
  evaluations/outcomes/*.md
```

重要 finding 可按需升级为 proof-carrying replay；accepted review 只证明 V0。V1/V2/V3/V4 分别表示 mechanical、replay-backed、calibrated comparative 与 multiple field-observed 的已证明范围，不是自动状态机。

artifact index 记录 case-relative path、SHA-256、bytes、来源和授权范围；evidence 绑定 artifact；finding 引用 evidence；append-only review round 绑定 finding/evidence exact SHA。临时思考、聊天和长工具输出留在原生 session，不持久化为控制面状态。

## 经验批次回流

在现有 Commander 窗口说“整理并回流本 case 的经验”。需要比较 canonical checkout 时，用 Claude Code 原生 `/add-dir <CANONICAL_CHECKOUT>` 为同一 session 增加目录访问；用户不需要手工编辑 pack。

流程是：

1. 从 current accepted evidence chain 提炼任意多条 immutable candidate；每条只提出 selected pack 中一个 destination，并声明 `mechanical→V1`、`analysis-method→V2` 或 `behavioral→V3` 的最低 maturity。
2. Reviewer 逐条只做 eligibility，检查证据、通用性、反例、重复/冲突、脱敏与目标资格。
3. 将同主题 eligible candidates 组成一个或多个 batch；一个 batch 可修改同一 pack 中多个现有 Markdown targets。
4. Reviewer 独立绑定完整未截断 patch、candidate/review SHA、canonical HEAD、target pre/postimage；behavioral batch 还必须绑定经 native prepare→逐 slot run→finalize 闭合所有预注册独立 control patches 的 current `go` calibration、解盲前一次读取 immutable `blind-review.json` 后形成的 exact blind decision（packet path/SHA、preferred entry/output SHA）、`pass`/`improved`/`V3` promotion、matched runtime/contract run bundle，以及该 run 对应的 exact `reveal.json` path/SHA，且被评估的是最终完整 patch，才可给出 accepted batch review。
5. 原生 helper 生成零写入 exact preview；只有 `CONFIRM STEAMAI LEARNING BATCH <identity>` 才会 Apply。
6. Apply 只改变 canonical working-tree targets；HEAD、index、当前 case snapshot 不变；失败只恢复本 batch targets；不自动 `git add`、commit 或 push。

未确认 candidate 留在来源 case，不扫描或汇总其它 case。verified-learning 引入前创建的 case 仍可作为 current case 继续研究，但不支持新版 learning preview/apply；helper 会在解析旧 artifact 前明确拒绝，不迁移或猜测旧字段。后续 field outcome 只由对应 case 用户逐份 opt-in，negative/inconclusive 也永久保留；单个 case 不足以 V4，也不建立 Hub、inbox 或经验数据库。已确认并应用的 working-tree bytes 可立即供本机后续 Fresh 使用；Git history 最终自然汇聚用户另行授权 commit/push 的通用经验。

## 更新与卸载

普通 `steamai` 不联网。显式更新到 GitHub 上的最新正式 release：

```text
steamai update
```

update 从 latest release manifest 取得 exact version/revision。请从 canonical checkout 之外的目录运行该命令；Windows 会阻止替换被当前终端占用的 source 目录。切换前要求 canonical checkout 无 staged、unstaged、untracked 或 ignored 本机内容，拒绝 release 未包含的任何本地 branch/stash commit，校验 exe SHA 与 `--version`，并验证 exact tag/revision 和 canonical identity。最终切换前再次绑定 HEAD、working tree 与本地 refs；任一漂移都停止。source 发生替换后，旧 checkout 以 sibling backup 保留并输出路径，不自动递归删除。它不自动 merge/rebase/stash/reset/restore/clean。需要保留的本地经验先做 local commit；若该 commit 尚未进入 release，则 update 会保守停止，push 仍不是本机消费经验的前提。

保守卸载：

```text
steamai uninstall
```

只删除已安装入口、setup 自己添加的 PATH 项和最小安装定位信息；默认保留 canonical Git checkout、未 push commits 和所有 case。当前 exe 先原子重命名，再由同字节的窄职责临时原生 helper 等待卸载命令退出后删除；helper 不执行其它产品职责，普通用户不依赖管理员权限或重启。Windows 锁语义会让该 helper 自身留在原安装目录，命令会显示其精确路径，进程退出后可手工删除。

v1 不支持 active case 跨电脑迁移、case import/export、云同步或 session 迁移器。新电脑只从 Release/Git 获取产品和已经 commit/push 的通用经验，用于新的 case。

## 维护与验收

- 产品入口：`cmd/steamai`、`internal/steamai/**`
- canonical/project-local skill：`.claude/skills/steamai/SKILL.md`
- case/研究模板与合同：`vnext/**`
- pack/common：`packs/<pack>/**`、`common/**`
- 文档路由：`docs/context-routing.md`
- 三线交付验收与未解决问题：`docs/professional-delivery-results.md`
- 历史路线与学习待验项：按文档路由定向读取，不作为日常使用前置
- 自动与人工验收分层：`vnext/acceptance.md`

维护验证：

```text
python -m unittest discover -s tests/pack_tooling -p "test_*.py"
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

fake process、synthetic fixture、cross-compile 或 workflow definition 都不能冒充真实 Windows setup/PATH、可见成员窗口、用户纠偏、跨会话消息、evaluator calibration/comparative result、field outcomes、learning apply 或 formal release 验收。Windows native test binary 可证明 suspended→Job→resume 的普通执行路径；原生 helper 的真实 timeout/process-tree 测试与 Commander/成员会话 Job 的无窗口进程链测试都只证明受控 Windows 父子进程清理。真实 Claude Code 会话退出后工具进程被收走、成员会话保留，须按 `vnext/acceptance.md` 的 live gate 另行验收，不等于 Claude 整体行为验收。真实模型调用的 token smoke 也不能替代 Reviewer calibration；维护/live 测试默认显式选当前配置的 Sonnet 档（`--model sonnet`）；仅极简探针或 headless 且 Sonnet 缺命令工具时降级 Haiku，不继承高档主模型；未经本次明确批准不用 Opus/顶级模型，不限定模型供应商；主会话实际消息模型须与冻结选择一致，不能把总用量中的额外模型直接视为主模型漂移。live 运行方式见 `vnext/acceptance.md`，结果按文档路由读取对应验收记录。默认测试不调用模型、不占前台；窗口/真实鼠标验收先取得用户同意的时间窗。

旧 Go control plane、mega CLI、project-local runtime、PowerShell façade、adapter host、legacy `/rekit` 与旧项目 importer 均已删除。STeamAI 不是自动逆向/漏洞挖掘或渗透引擎；危险动作仍受明确 case 授权、具体用户确认与 Claude Code 工具权限约束，`CLAUDE.md` 只提供上下文，不授予权限。
