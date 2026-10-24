# STeamAI 专业负责人接入路线

## 读取指南

- 路线 ID：`steamai-professional-delegation-v1`；长期基准见根 `CLAUDE.md`，路由见 `docs/context-routing.md`。
- 继承已验 Fresh 确认、成员信任、七字段纠偏；历史 tie/inconclusive 不改判。

## 用户画面与范围

用户向 Commander 说明目标；Commander 在派单时即交给配置了固定专业能力的普通 `execution` 负责人，不新增成员等级、任务字段或 Go 调度状态：

- **专业工具负责人**：直接使用固定专业工具（当前为 REA 与既有 IDA MCP 的按路线选择；后续 Burp/Playwright/Airtest 同模式），不先走较弱流程。
- **外部 agent 团队**：一名负责人经官方接口委托单一有界任务；外部内部 agent 不是 durable member 或 tactical subagent，未通过资格验收不自主运行。
- 成果进入 artifact→evidence→finding→review；外部工具自报成功不等于 accepted。用户纠偏、learning exact confirmation、heavy action 边界不变。

## 批次

| 批次 | 交付 | 状态 |
|---|---|---|
| PD-01 | 角色、skill、capabilities、acceptance 与路线指针 | 完成：机械合同；未安装外部工具 |
| PD-02 | REA 固定上游材料、接入说明、manifest 与 SHA 校验；不进 learningTargets | 完成：材料与合同测试；本批不注册 MCP/调用模型 |
| PD-03 | 成员工具可见、目标切换、正例、导出交接、GUI IDB 保留与不可用路径 | 通过（限定条件）：JS/native 静态、独立交接与 GUI IDB 保留均实测；可见成员全程未验；撤回早先“全过” |
| PD-04 | 直接 Claude Code+REA 与 STeamAI 负责人的同模型/材料/预算/未见题比较 | 正式匹配盲测（2026-10-24）tie：两臂新未见题三问全对，盲审 preferred=none；探索记录见下 |
| PD-05 | Strix `v1.7.0` 范围、停止、费用、纠偏、产物、模型六项资格 | 通过（限定条件）：六项均有 live 实证；范围为指令约束非机械隔离、预算为响应后检查、纠偏机制为停止后新开工而非热消息 |
| PD-06 | 一名负责人委托首个有界外部团队任务，成果可复核 | 已交付：Strix 静态扫描两项发现均独立 accepted，E/F/index/review 链经 R-PD06-R3 终审 accepted；turns 口径缺口等限制如实保留 |
| 后续 | 游戏脚本单平台试点（Airtest/Playwright，不自建运行器） | 未排期；纯开发任务是否属于安全研究 case 需先收口 |

## PD-03 实测记录（2026-10-23）

固定 REA `6.7.0`/`27dc42d1`，exact 成员 cwd 注册 local-scope MCP，CLI 与 serverInfo 分别核对。JS 导出 91957 bytes/2 records；native A→B→A alpha 相同、beta 不同，bundle 670655 bytes/5 records。`R-PD03.md` 局部 accepted（不证明原调用时序/阅读比例）。超限、哈希不匹配、配置损坏、清理未确认均有停止记录；可见会话与 headless 分别验收。

集成边界：

- headless 需分页 decompile；本机插件 `0.1.0` 无 `max_lines/offset`，独立 venv ida-pro-mcp `2.0.0`/`c133c385` 可用（serverInfo 仍报 `1.0.0`，不可互代）。
- 搜索无 limit/分页，须导出 case artifact 定向读取；导出不等于已读。`cleanup_incomplete` 不证明 worker 退出。内联小函数缺独立符号，`//go:noinline` 后出现。

10 份客户端报告合计 **$6.534376**（申报 $2，超 227%）。预算失守、副本未同步、JSON 转义损坏均为维护错误；全权授权不使超支合理，不以付费重跑刷通过。

### GUI IDB 保留实测（2026-10-24）

用户 IDA 9.3 装临时 ida-pro-mcp 插件后打开 `pd04-probe.exe.i64`；REA 6.7.0 headless 打开同一 exe 并 close（SHA-256 与上轮一致）。GUI 进程/注册/adapter 会话完好且仍可查询，`.i64` SHA-256 与 mtime 不变——**REA headless 不保存也不关闭用户 GUI 数据库**，与 rea.md 声明一致。验证后已还原（测试 IDA 关闭、插件卸载）。附带发现：Claude 插件 ida-pro-mcp 0.1.0 的 force_gui 在 Windows 把 `sys.executable` 当 IDA 可执行，无法启动 GUI，本验证改手动启动后 adopt。

## PD-04 对照记录（2026-10-23）

合成 Go 探针三题；维护方从源码/adapter 取参考答案，未形成盲审包。三组 transcript 主 assistant 均记录 `gpt-6-astra`；REA/adapter/$1.5 阈值/allowedTools 相同，但材料/交付不匹配，B' 复用同题。

| 组 | 配置 | 结果 | 费用 | 产物 |
|---|---|---|---|---|
| A | 裸 Claude Code+REA，无合同/方法材料 | 三题全对，含 FNV-1a/FNV-1 与字符串描述符间接引用链 | $0.49 | stdout |
| B | STeamAI Fresh+任务+pack，修正前合同 | 首次 binary_session 67KB 超限，按任务停止条件退出，三题 unknown | $0.48 | E、空 bundle、index |
| B' | 同 B，修正后合同 `3167dfaa` | 三题全对，与 A 答案等价 | $1.01 | E、8 记录 bundle、index |

- 修正前 B 更差（合同把非必需大响应设为任务级停止条件，净负值，已修正）；B' 同题回归非新题比较，费用 A $0.487240 / B' $1.008449（A 无相同产物要求，差额不全是证据链成本）。
- 撤回“有效 tie、约 2 倍成本买证据链”推断；保留正确/unknown/费用事实，不声称质量增益。两组均有 `cleanup_incomplete`，非差异项。

### 正式匹配盲测（2026-10-24）

新探针 `pd04c-probe.exe` 新三题；两臂匹配 haiku、budget $2、max-turns 40、answer-only、同 REA 6.7.0。A（裸）$1.233/25 turns、B（Fresh case 成员）$1.218/29 turns，三问均全对（参考答案由维护者源码+独立 headless IDA 核实）；盲审（固定随机映射、reviewer 不知臂身份）preferred=none。**结论 tie，不据此宣称增益或劣势。** 观测：$1 试跑 A 完成、B 触预算截断无收尾（合同上下文每轮开销略大，预算为响应后检查）；B 按合同核验 REA/Node 版本、超限按处置导出，A 无版本核验；B 答案含合同痕迹、盲度有限。记录 `%TEMP%\steamai-pd\pd04c-result.md`。正式 $2.60 + pilot $1.29（A $0.28：local-scope 注册 key 需正斜杠否则 MCP 不加载；B $1.01 预算截断）= **$3.89**。

## PD-05 资格核查（2026-10-23，修订结论）

重启后 WSL 2、Docker Server 29.9.2 就绪，镜像 `ghcr.io/usestrix/strix-sandbox:1.3.0` 可用；`strix-agent` 1.7.0 钉到 commit `55bc07991aacb1c26a81b43e7ca53148730d2019`，不以 tag 代替安装身份。

首轮仅授权合成本地 Flask 静态审查（`-n --scan-mode quick --max-budget 2 --max-turns 30`）：exit 0 但 run `stopped`、coverage `budget_exceeded`、findings 空、SQLite 无 app.py 成功返回证据——不是审查后零漏洞，不判模型漏检。

| 资格项 | 已核实事实 | 判定与限制 |
|---|---|---|
| 范围 | 控制实验仅一 child、执行标记命令与读 scope.txt；持久调用未见网络/目标执行/其它读取 | 指令约束非机械隔离（writable mount）；不证明扫描范围资格 |
| 可确认停止 | child 回执 Process running 且输出 marker 后发 Ctrl+C；interrupted/exit 1、容器前有后无、无强杀 | 主动停机与子命令终止已验；维护信号不冒称用户操作；快照不当存活判断 |
| 费用 | 网关账单逐分验证 zai 参考单价（倍率 1）；控制实验 $0.981564/$3、重启轮 $0.2224/$1.2，单事件 cost_usd 全非空 | 通过：--max-budget 真实生效但仍为响应后检查，请求/turns 上限兜底；不用其它价目或无关用量冒充 |
| 纠偏 | 停止（Ctrl+C）与全新目录带新指令重开（`target_45f5`）均 live 实证：旧队伍 ID 未恢复，纠偏任务逐字执行并 completed | 通过（机制=停止后新开工）：`--resume` 先恢复旧 child 的窗口与 headless 无热消息接口为已记录边界 |
| 产物 | run/coverage/SARIF/log/原始 SQLite 可取回；控制任务 completed，内容与 scope 一致 | 只证明控制任务；无 findings 不代表安全；快照 WAL 缺口见下节 |
| 实际模型 | 12 个成功响应的 response.model 均 glm-5.3；1 个取消事件无响应身份 | 证明 SDK 规范化响应身份，不穿透网关证明底层模型；不从请求 model 或事件总数推断 |

**判定：通过（限定条件）。** 六项均有 live 实证；范围是指令约束非机械隔离、预算是响应后检查、纠偏机制是停止后新开工而非热消息、模型身份是 SDK 响应报告不穿透网关。过早"有条件通过"已撤回；2026-10-24 补齐计价核验与新开工实测后六项才成立，PD-06 据此开放首次委托。

### Windows 空输出与控制接口

同镜像、network none、无宿主挂载/模型的离线对照：Docker 直接 exec 正常，openai-agents `0.19.4`+docker-py `7.2.0` 的 `tty=false` exit 0/空输出，`tty=true` 完整返回。根因：SDK `agents/sandbox/sandboxes/docker.py:973–977` shutdown(SHUT_WR) 遇 docker-py `npipesocket.py:201–203` 直接 close，SDK `docker.py:1100–1121` 吞读异常。Sonnet live 的 tty=true 读取亦有内容；未改第三方安装。

源码位置（相对 Strix 安装包，均已人工核读）：`interface/cli.py:155–163` 信号；`core/agents.py:518–550`；`runtime/session_manager.py:381–409` 吞清理失败；`core/runner.py:537–569` 先 respawn 再注入；`core/execution.py:407–480` 恢复无纠偏屏障；`llm/request_log.py:654,713–734,896–924` 默认日志不证响应身份；`runtime/docker_client.py:54–66,223–235`；TUI controller 可控、standalone viewer 不控 headless。

### 旧 Astra 控制实验

一次性维护驱动、不进产品。`target_7294`（阈值 $0.75）：child 等待命令启动前即触顶（$1.519478），未发 Ctrl+C；官方 resume 注入新指令后 root stop_agent 返回 child 已 stopped，随后 MaxTurnsExceeded(1)/exit 1；累计 $2.078808（恢复增量 $0.559330）。容器清理、无强杀；7 响应 gpt-6-astra、cost_usd 全 null（空费用合计为 0 的早期错误已纠正）。旧报告累计 **$4.864274/$5** 后停止；跨模型重跑不作能力对照。

### Sonnet 重跑与原始证据复核

本轮冻结 `ANTHROPIC_DEFAULT_SONNET_MODEL=glm-5.3[1m]`，已验 Messages API 映射为 `glm-5.3`；不继承主模型、不改全局设置。独立申报 $3，分阶段阈值 $1.2/$2.5，另设每 agent 15 turns、每阶段 20 个 agent 事件及 360 秒观察限额。凭据临时配置已删除。

`target_2f2e` 两阶段：stop 128 秒（child 回执 Process running 且输出 marker 后 Ctrl+C，interrupted/exit 1、容器消失、无强杀）；resume 53 秒（新指令到 root 前旧 child 已恢复，write_stdin 报 PTY not found；root 随后 stop_agent、读 scope、finish_scan，completed、无重跑 sleep）。

- 13 事件中 12 success（含 2 次 preflight）、1 cancelled；成功响应均 glm-5.3；root 7 + child 3 请求，共输入 594399/输出 5804 tokens。cost_usd 全 null、run cost 0：completion_cost 对该别名报 ModelNotMappedError，`report/usage.py:215–277` 吞估价异常、账本保留初始 0。新轮费用当时 unknown（2026-10-24 经计价核验解除，见下节）。
- 维护快照曾漏 WAL 致两阶段 DB 副本无表；原始 DB+WAL 完整（31 条消息逐核），SQLite backup 最终副本 SHA-256 `1075053b…56d1dd`。快照函数已改用 backup 并离线回归。

### 零推理费用的后续核查

- 只读接口无逐请求账单/模型单价（billing 仅倍率、usage 混合汇总、`/api/v1/*` 需登录态）；纠偏替代与配置边界（persist_current 写回、cleanup 吞错须以准确 container ID 核实）由离线源码核查确认，后经 2026-10-24 计价核验与实测闭环，见上节。

### 计价核验与停止后新开工（2026-10-24）

- 计价：网关 `/v1/usage` 汇总账单（4465 请求、$219.86087336）与 litellm zai/glm-5.3 参考单价（输入 $1.4e-6、输出 $4.4e-6、缓存读 $2.6e-7，倍率 1、无高峰）逐分一致，差值 -0.0；据此核算既有控制实验停止阶段 $0.383961、全周期 $0.981564（$3 内）。账户侧不再有前置依赖；无关请求用量不归因。
- 单价注册：维护入口把已验证单价注册进 litellm 成本表（公共 API，不改安装）。重启实测每事件 cost_usd 非空，Strix 汇总与事件合计一致（差额即 preflight），`--max-budget` 由恒 0 变真实生效（仍为响应后检查）。
- 停止后新开工（`target_45f5`，$0.2224/$1.2，76 秒）：全新空目录、不带 `--resume`、带纠偏指令；旧队伍 ID 未恢复，child 逐字读 scope.txt（tty=true）、root finish_scan 收尾；completed、容器清理、无强杀。早前“费用 unknown”阻塞解除；`--resume` 恢复旧 child 的窗口与无热消息接口为已记录边界。

## PD-06 尝试与离线修复

两次负责人 headless 尝试分别止于目标子目录缺失、`STRIX_LLM is not set`，无产物。布局已补；第二次为 PowerShell→bash -lc 丢内层引号致 instruction/config 丢失，离线 argv 复现并验证直接调用修法。

诊断和原始 case 不入产品；临时密钥配置已删，用户配置未改。维护方不覆盖成员 E/身份；"用户直接纠偏"的维护驱动文字不作真实用户操作证据。当前无 artifact/E/F/review 全链。

### PD-06 成员会话环境限制（2026-10-24）

- headless 成员会话须清除 `CLAUDE_CODE_CHILD_SESSION` 等全套嵌套标记（旧 helper 只清 `CLAUDECODE`）；CLI 2.1.273 嵌套会话无命令工具，清除后恢复。
- CLI 2.1.273 对不识别主模型（`glm-5.3[1m]`）不给命令工具，`--tools` 点名不恢复；`gpt-6.1-sol[1m]` 可识别且工具完整。需命令执行的成员会话选 haiku 档；Strix 走 litellm 直连不受影响。两次诚实阻塞（$0.34/$1.08）与探针 $0.53 留档。

### PD-06 交付（2026-10-24）

- 按 PD-05 修正调用执行（单层 bash、`--instruction-file`、临时 `--config`、真实预算）：run `pd06-target_298c` quick 静态，**$0.66911920/$1.50**，19 响应全部 `glm-5.3`，completed、沙箱清理；宿主退出码 2 原因未查明（绑定 `PD06-298c-host-session`）。
- 纯静态找到两个预置漏洞（`/file` 路径穿越 CWE-22、`/search` 反射 XSS CWE-79，F-PD06-001/002）；原件按 bytes/SHA-256 登记 index，SARIF 合成位置未当源码定位。独立 Reviewer（haiku、含哈希复核）逐行核对后两项 F 均 accepted，经三轮补证循环 R-PD06-R3 终审 accepted、传递绑定闭合。残余限制：root 17 turns 对 `--max-turns 12` 的口径缺口、退出码仅有会话文字记录、无完整沙箱命令/网络审计。
- 费用：本轮 PD-06 **$6.12**（Strix $0.67 + 成员/审查会话 $5.45），超申报 $4 约 53%（主因两次环境阻塞 $1.43 与三轮补证）；另有诊断探针 $0.53、PD-05 重启 $0.22。旧尝试 $1.055888 保留。

报告核算：PD-03 $6.534376；PD-04 $5.87（旧 $1.977 + 正式盲测 $3.89）；PD-05 $6.068（Astra $4.864274 + Sonnet $0.981564 + 重启 $0.222384）；PD-06 $7.172。合计 **$25.64**，另有诊断探针 $0.53、主会话与维护审计未计；总实付以网关账单为准。

## 授权与硬边界

- 用户已批准完整方案、重启恢复、低成本重跑与继续完成；不授权降标或把维护驱动冒称用户纠偏。每批单列预算。
- REA/Strix 许可约束适用；外部 agent 不自动继承逐工具权限。不新增 adapter host/任务数据库，不改 Go production。
- Fresh、learning exact confirmation、成员单写、用户纠偏优先、evidence currentness 不变；upstream 不进 learningTargets；旧 current 不升级。
- headless 不覆盖 GUI/可见全程；机械/合成验收不证明真实质量。

## 验证与完成口径

```text
go test -count=1 ./internal/steamai/vnextcontract
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

本轮 focused tests、Go 全量与 vet、Python pack tooling 27 tests、diff check 均通过；维护快照 WAL/句柄回归亦通过。无 Go production 或固定 upstream 改动；默认测试不调用模型。

live 按 `vnext/acceptance.md` 逐项执行；每项有实证结果或具体阻塞，不以缺证降标、不重试刷通过。维护模型政策见根 CLAUDE.md 与 acceptance。
