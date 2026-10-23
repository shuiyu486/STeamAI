# STeamAI 专业负责人接入路线

## 读取指南

- 路线 ID：`steamai-professional-delegation-v1`。
- 长期基准见根 `CLAUDE.md` 的"智能成员与自主协作基准"；本文只记录本轮范围与验收状态，完整路由见 `docs/context-routing.md`。
- 承接已完成的低干预启动路线事实（Fresh 机器确认、成员工作区自动信任、七字段纠偏、有限 A/B tie），不改判任何历史对照或 `inconclusive` 结论。

## 用户画面与范围

用户只向 Commander 说明研究或交付目标；Commander 在派单时即把专业问题交给配置了固定专业能力的普通 `execution` 负责人（不新增成员等级、任务字段或 Go 调度状态）：

- **专业工具负责人**：成员直接使用固定版本专业工具（当前为 REA 与既有 IDA MCP 的按路线选择；后续 Burp/Playwright/Airtest 同模式），从任务开始按所选入口执行，不先运行较弱的默认流程。
- **外部 agent 团队**（Strix 等）：由一名对接负责人经其官方接口委托单一有界任务；内部 agent 不是 durable member 也不是 tactical subagent；未通过资格验收不自主运行。
- 成果进入现有 artifact→evidence→finding→review 链；外部工具的 `pass`/完整性校验不自动等于 accepted；用户纠偏、learning exact confirmation、heavy action 边界全部不变。

## 批次

| 批次 | 交付 | 状态 |
|---|---|---|
| PD-01 | 新路线、合同与路由收口：本文、SKILL.md 路由条款、analysis-member 角色条款、`vnext/capabilities.md` 外部工具接入节、`vnext/acceptance.md` 专业负责人 gate、context-routing/batch-plan/README/根 `CLAUDE.md` 指针 | 完成（机械合同；未安装任何外部工具） |
| PD-02 | binary-re 固定 REA 上游材料与接入说明：`upstream/rea/**`（6.7.0 / commit `27dc42d1`，拼接 SHA-256 机械校验）、`tooling/integrations/rea.md`、manifest 声明、不进入 learningTargets | 完成（材料固定与合同测试；未注册 MCP、未调用模型/工具） |
| PD-03 | 真实可见单成员逆向任务端到端：成员 local-scope MCP 可见与正确性、A→B→A 切换、正例命中、导出交接、GUI IDB 保留、不可用路径报告缺口 | 完成（2026-10-23；JS 段与 native 段均过 gate，reviewer 独立复核局部 accepted；详细发现与费用见下节"PD-03 实测记录"；GUI IDB 保留未验，attached 模式未启用） |
| PD-04 | 有限对照：A=直接 Claude Code+REA vs B=STeamAI 负责人+同一 REA；同模型/材料/预算/未见题 | 完成（2026-10-23，两轮：修正前 B 更差、修正后 tie 且成本更高；合同修正已提交 `3167dfaa`；详见下节"PD-04 对照记录"） |
| PD-05 | Strix `v1.7.0` 有界外部执行资格核查：范围、可确认停止（含内部 agent）、含子代理费用、纠偏生效、产物取回、实际模型六项 | 完成（2026-10-23，接口核查+真实运行实测；结论**有条件通过**：范围/停止终态/产物/模型四项过，费用为软上限（实测超申报 39%）、headless 无热纠偏；详见下节"PD-05 运行实测记录"） |
| PD-06 | 首个外部团队任务交付（一名负责人、单一有界任务、可停止、成果可复核） | 未执行 |
| 后续 | 游戏脚本负责人单平台试点（Airtest/Playwright，不自建运行器） | 未排期；纯开发任务是否属于安全研究 case 需先收口范围 |

## PD-03 实测记录（2026-10-23，合成目标，全部 live）

环境：成员 `analyst`（execution，逆向专业负责人）+ `reviewer`，case 位于本机临时目录；REA 以 local-scope MCP 注册在成员 exact cwd（`rea-agents@6.7.0`，全局 npm 安装、直接 `node` 启动避开 npx 冷启动超时；运行时导出 `MCP_TIMEOUT=180000`/`MCP_TOOL_TIMEOUT=900000`）。

通过项：

- 成员 local-scope MCP 注册、连接与工具可见；版本核对：CLI `6.7.0` + stdio `serverInfo re/6.7.0` 双证据。
- JS 应用分析 complete，导出 bundle（91957 bytes/2 records）+ index 登记 + 12 项 unknowns 保留。
- native 段 A→B→A：`main.alphaHandler` 两次 pseudocode（8 行）与 assembly（19 项）逐字一致、beta 内容不同；`search_strings` 经导出文件确认命中 `alpha-marker:` @ `0x1400ccfb1`；最终 bundle 670655 bytes/5 records，全部 evidence_id 落盘。
- 导出 bundle 由 reviewer 独立读取复核（`R-PD03.md`）：哈希、records 集合、pseudocode 逐字一致、字符串命中全部通过，局部 accepted；reviewer 同时正确指出 bundle 存储顺序不证明调用时间顺序、历史"读取 0%"为过程陈述不可独立核验。
- 不可用路径 fail-closed 共真实触发六次（连接超时、`binary_session` 截断、目标哈希不匹配、注册配置损坏、adapter 参数不兼容、关闭清理未确认），成员每次按合同停止并如实记录，无一次绕过或猜测根因；双可见会话 hold 与跨会话身份确认纪律保持。

实测发现的集成边界（均已写入接入理解，不改判 REA 审计结论）：

- **adapter 版本配对**：REA 6.7.0 modern headless 需要 ida-pro-mcp 上游 main 构建分页工具（`decompile` 带 `max_lines/offset/database`）；Claude 插件缓存 `0.1.0` 的 `decompile` 无分页，`analyze_function` 报 `unexpected parameters`。已按 REA 文档验证快照安装 `ida-pro-mcp==2.0.0`（git commit `c133c385`，独立 venv，不动 0.1.0 插件）。新构建 `serverInfo.version` 仍报 `1.0.0`，与发行版号不符，版本钉靠安装记录而非该字段。
- **无界输出**：REA `search_strings`/`search_procedures` schema 无 `limit` 参数，literal 结果代理全量清单；`inspect_analysis_view` 不支持 search 父证据分页。唯一有界路径是把 evidence bundle 导出到 case 内再读文件——成员实测两轮均这样闭合。
- **worker 生命周期**：`close_binary` 可返回不可重试 `cleanup_incomplete`（"IDA worker remains registered after close"），worker 可超越 supervisor 生命周期；本轮由维护方确认进程清零并删除 `%TEMP%\rea-ida-*` 私有工作区（36MB）后解除。
- **目标设计**：Go 单行小函数会被内联，IDA 无独立函数可解析；合成探针需 `//go:noinline` 才能做 A→B→A（目标问题，非工具缺陷）。
- export schema 仅 `path/overwrite`，不支持按 operation 过滤导出。
- 未验证项：attached/GUI 模式与 GUI IDB 保留（本轮仅 headless）。

费用（Claude Code 实测 `total_cost_usd` 之和）：10 次成员/复核运行合计 **$6.53**（0.34+0.40+0.70+0.62+1.03+0.49+0.77+0.54+0.96+0.68）。执行前申报预算 $2，实超 227%：native 段五次迭代均为真实阻塞（工具超时→响应截断→目标内联→配置转义损坏→adapter 版本错配），非重试刷通过；据此后续批次预算按实际迭代深度重估并继续逐项记录。

## PD-04 对照记录（2026-10-23，合成目标，三组运行）

设计：新合成 Go 探针 `pd04-probe.exe`（`//go:noinline` 的 `main.gammaFn`/`main.deltaFn` + rodata 字符串 `pd04-marker-xyz`，双方均未见）；三道客观题（算法语义/字符串 VA 与引用函数/分段函数语义），标准答案在运行前由维护方用 adapter stdio 机械查询固定（字符串 VA=`0x1400cd796`）；同模型、同 REA 6.7.0、同 ida-pro-mcp `c133c385` adapter、同预算上限 $1.5、同 allowedTools。

| 组 | 配置 | 结果 | 费用 | 产物 |
|---|---|---|---|---|
| A | 裸 Claude Code+REA（无合同、无方法材料） | 三题全对（含 FNV-1a vs FNV-1 区分、字符串描述符间接引用链） | $0.49 | 仅 stdout 答案 |
| B | STeamAI 负责人（Fresh case+七字段任务+pack 材料，**修正前合同**） | 三题 unknown：首次 `binary_session` 版本探测 67KB 超限触发任务级停止条件，忠实停止 | $0.48 | E + 空 bundle（0 记录）+ index |
| B' | 同 B，**修正后合同**（`3167dfaa`：版本核对走 CLI/serverInfo，输出超限分级处置） | 三题全对，答案与 A 等价 | $1.01 | E（含 evidence_id 链）+ bundle（8 条证据）+ index |

结论（按"tie/更差为一等结果"口径）：

- **修正前 B 更差**：同题同预算下 A 完成而 B 失败，根因是接入合同把非必需的大响应版本探测设为任务级停止条件——STeamAI 层在该配置下对任务结果造成净负值。这本身是对照的有效产出：合同缺陷被真实暴露并已修复。
- **修正后 B' 与 A tie**：答案等价正确，但 B' 成本为 A 的 2.06 倍；额外产出是可交接的证据链（evidence_id、bundle、index）与边界披露（清理未确认等），该价值未在本对照中定价。
- 不据此宣称 STeamAI 负责人模式带来研究质量增益；本对照只支持"合同修正消除了自伤性停止"与"证据链产物以约 2 倍成本获得"两个事实。
- 两组均如实报告 `close_binary` `cleanup_incomplete`（PD-03 已知 worker 生命周期边界，组间一致，非差异项）。

费用合计：$1.98（A $0.49 + B $0.48 + B' $1.01），预算 ≤$4 未超。

## PD-05 接口核查记录（2026-10-23，CLI 层，未运行）

安装：`pip install strix-agent @ git+...@v1.7.0`（包名 `strix-agent`，非 `strix`）；本地模式依赖 Docker（首次运行自动拉 sandbox 镜像）与 LLM key（LiteLLM 格式，`STRIX_LLM`/`LLM_API_KEY`/`LLM_API_BASE` 或 `~/.strix/cli-config.json`）。

六项接口核查结果（`strix --help` v1.7.0 实测）：

1. **范围**：`-t/--target`（本地目录挂载进 sandbox 可写）、`--scope-mode {auto,diff,full}`、`--instruction`（任务前注入范围约束）——接口存在。
2. **可确认停止（含内部 agent）**：`--max-turns N` 每 agent 到限强停+渐进 wrap-up 警告；`--max-budget` 到限 clean stop——接口存在；headless 运行中的外部停止确认接口未见，运行终态可确认性待实测。
3. **含子代理费用**：`--max-budget USD` 覆盖全部 agent，接近时向所有 agent 发渐进警告——接口与文档一致。
4. **纠偏生效**：任务前 `--instruction` 存在；**headless（`-n`）模式无运行中热纠偏接口**，仅交互 TUI 模式可中途干预——资格核查重要发现：非交互委托期间无法热纠偏。
5. **产物取回**：报告写入 `strix_runs/<run-name>`（README 声明），`--fail-on` 控制 headless 退出码——待实测。
6. **实际模型**：`STRIX_LLM` 为 LiteLLM 格式模型名，实际调用模型需运行日志/报告核验——待实测。

环境进展：Docker Desktop 29.9.2 已装（winget 静默）+ `com.docker.service` 已起；新版 WSL 2.7.13.0 已装；WSL 可选组件已启用但**需重启系统生效**（`Wsl/WSL_E_WSL_OPTIONAL_COMPONENT_REQUIRED` → `wsl --install --no-distribution` → "直到重新启动系统前更改将不会生效"）。Docker Linux 引擎当前 500（backend not running），预期重启后恢复。

## PD-05 运行实测记录（2026-10-23，重启后）

环境：Docker Desktop 29.9.2（winget）+ WSL 2.7.13.0，用户重启系统后引擎就绪；沙箱镜像 `ghcr.io/usestrix/strix-sandbox:1.3.0` 首次运行自动拉取成功。LLM 经用户网关（LiteLLM `anthropic/gpt-6-astra`；注意 `[1m]` 后缀是 Claude Code 内部标记，裸 Messages API 不认，网关 404 实测）。配置用 `--config` 临时 JSON（key 不入仓库）。

运行：`strix -n -t ./pd05-target --scan-mode quick --max-budget 2 --max-turns 30 --instruction <范围限定> --config <临时>`；目标为合成 Flask 应用（教科书级 SQL 拼接与 XSS 回显，仅本地授权静态审查）。运行正常完成（exit 0），产物落盘 `strix_runs/pd05-target_241c`。

六项核查实测结论：

1. **范围** ✓：本地目录挂载沙箱、`--instruction` 生效，日志未见越界动作。
2. **可确认停止（含内部 agent）** ✓：预算触顶时日志逐 agent 记录 `agent.status <id>=stopped`、child 停止确认、sandbox teardown、scan done、进程退出 0；`.state/agents.json` 留终态。
3. **含子代理费用** ✗→实测缺陷：`--max-budget 2` 最终 **$2.7855（超 39%）**——预算为响应后检查，两个并发 agent 各自越过后才被拦（$2.2192 与 $2.7855 两级触发）。预算是软上限，申报时须按 1.5–2 倍上浮并配合 `--max-turns` 双保险。
4. **纠偏生效** △：headless（`-n`）无运行中热纠偏接口（仅交互 TUI 可中途干预）；任务前 `--instruction` 约束生效。委托期间不可热纠偏须作为已知边界接受。
5. **产物取回** ✓：run.json（指标+逐请求用量）/coverage.json/findings.sarif/strix.log/.state 完整落盘；SARIF 如实标注 `stopped` 与覆盖限制，不冒称完整完成。
6. **实际模型** ✓（需 DEBUG 日志）：`strix.log` 每请求记录 `provider=anthropic model=gpt-6-astra host=<网关>`，requested 与实际调用一致、逐请求可审计。

附带观察（不属资格项）：该次 quick 模式在 14 请求/$2.79 内未报出合成目标中的 SQL 拼接与 XSS（SARIF 空 results，扫描因预算停止且覆盖受限）——不据此下能力结论，仅记录为本次有界运行的事实。

判定：**有条件通过**——PD-06 委托合同必须包含：预算按实测上浮系数申报、`--max-turns` 双保险、单一有界目标、非交互接受无热纠偏、产物与模型日志取回归档。

## 预算与授权边界

- 本轮机械交付（PD-01/PD-02）不安装外部工具、不注册 MCP、不调用模型；live gate（PD-03 起）逐项取得用户批准，预算在执行前单独给出，旧实验预算不沿用。
- REA/Strix 等第三方项目自带的使用约束与许可（含"仅授权目标"要求）始终适用；它们的 agent 不因由 Claude Code 启动而自动继承 Claude Code 逐工具权限。
- 不为接入新增通用 adapter host、后台 supervisor、任务数据库或 `steamai.exe` 配置管理。

## 不变的硬边界

- Fresh 授权、learning exact confirmation、成员单写、`HOLD_STALE_TASK`、用户纠偏优先、evidence currentness 全部不变。
- `upstream/**` 固定材料不进入 learningTargets；经验只回主包既有合格 Markdown。
- 已有 current case 不升级、不迁移；新合同只进入新 Fresh。
- 已完成路线的历史结论不改判：低干预 A/B tie、自主协作 inconclusive、研究有效性 pass-limited/tie 等保留原文。

## 已知事实与不声称

- REA 能力与限制最初来自对 `rea-agents-6.7.0`（commit `27dc42d1`）的源码/文档审计；PD-03 已在本机 live 运行，审计中的 attached 仅 legacy 工具目录、headless 不继承已有 IDB、IDA 结果为 live 等限制全部得到实测印证，并新增上节所列五项集成边界。
- Strix 控制面结论：`v1.7.0` 官方文档结论已由本机 CLI 接口核查部分印证（范围/预算/turns 限制接口存在；headless 无热纠偏）；运行中行为（停止终态、产物、实际模型）仍待实测，不据接口存在声称资格通过。
- 不声称：接入即研究增益、外部团队可自主运行、上游材料等同本机已安装版本、PD-02 机械验收等同 PD-03 live 验收、PD-03 合成目标上的通过等同真实研究目标质量。

## 验证与完成口径

机械部分（本轮）：

```text
go test -count=1 ./internal/steamai/vnextcontract
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

live 部分按 `vnext/acceptance.md` 专业负责人接入节逐 gate 执行并在此表更新状态；每项有结果或有证据的阻塞，缺证不降标、不重试刷通过。
