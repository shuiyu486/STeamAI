# STeamAI 专业能力实际交付路线

## 读取指南与当前状态

- 路线 ID：`steamai-professional-delivery-v1`；制定于 2026-10-24，初始源码基线 `c3facdf5`。
- 本文件是本轮目标、批次状态、阻塞与下一步的唯一事实入口；`docs/batch-plan.md` 只作短投影。
- 本轮实施已开始（2026-10-24），执行状态、当前批次与授权边界以下方“接续点”为准；本文件是唯一状态入口。用户实施请求涵盖其声明的仓库改动与验证；未涵盖的安装、目标运行、网络动作、提交或发布仍需单独授权，已经明确的业务目标不重新询问。
- 新会话：根 `CLAUDE.md` → `docs/context-routing.md` → Git 状态 → 本文件；长期行为与 gate 定义在 README、skill、`vnext/templates`、`vnext/acceptance.md` 与各 pack 入口，不默认通读旧路线或全部 pack。
- 批实施临时细则（原 `plans/professional-delivery-{domains,validation}.md`）已于 DLV-08 收口删除；本路线不再保存第二套进度或临时执行说明。

## 目标与已确定决策

用户明确需要三个方向：**逆向、Web 安全、游戏自动化脚本开发**。目标不是增加工具名称，而是新建项目后，成员从一开始就使用合适的专业能力，交付可复核结论或可运行代码。

| 业务线 | 本轮交付目标 | 首选专业能力 | 首版边界 |
|---|---|---|---|
| 逆向 | 一个功能的跨函数分析、版本差异、必要的运行时补证 | REA + 已有 IDA MCP；动态验证用实际可用的 Windows 调试工具 | Windows x64、有限功能；不承诺全程序还原或所有 provider 等价 |
| Web | 源码线索 → 浏览器/API 实证 → 修复后安全与正常业务回归 | Strix + Playwright；已有 Burp 可按题使用 | 本机受控应用先验；远程目标、写请求和账号范围另按具体 case 批准 |
| 游戏脚本 | 识别状态 → 执行动作 → 检查结果 → 有界恢复/停止 → 可维护脚本 | Windows 首选 Airtest；有 SDK/控件支持才用 Poco；浏览器场景才选 Playwright | 先交付 Windows 窗口自动化，不把 Android/iOS、多游戏引擎或反检测纳入本轮 |

1. 产品目标从“仅安全研究”扩为“已授权安全研究与自动化开发”，仍是人在环的 Claude Code 协作层，不变成通用软件平台。该目标在 DLV-01 一致更新合同后生效，不能只改宣传文案。
2. 复用现有 `execution`/Reviewer、七字段任务、主包＋最多一辅、原生窗口/消息/恢复；不新增开发角色枚举、任务数据库或 `caseType`。
3. 调用专业项目的官方 CLI/MCP/库，固定成熟版本；不 fork 整套引擎、不追 `latest`、不复制上游功能实现。保留来源与许可；上游原文和运行配置不进入 learningTargets。
4. 新建 `packs/game-automation/`，不是把 `android-native` 改名。研究交付与代码交付分开，开发任务不强制伪造漏洞 finding。
5. 先把三条业务线各做成完整用户路径，再验两两协作；不要求每题都多人参与或同时调用全部工具。
6. 费用不成为主要工作：执行前采用能覆盖完成与收尾的宽松批次额度；默认配置的 Haiku/Sonnet 档，禁止继承高档主模型或擅自升级。仅汇总已知总量和异常；不建费用平台，不以紧预算反复试跑。

## 基线与不能继承的完成声明

旧 `docs/professional-delegation-roadmap.md` 保留当时记录，不重写成功/失败历史。本节是新路线的适用性说明，不使旧局部证据失效。

- 协作/Fresh/独立成员/产物与审查已有实现；REA 固定材料与小型静态任务、本机 Strix 静态审查及其交付链已有证据。它们不等于三个业务方向均产品化。
- REA 的 manifest/integration 与部分旧默认 router/workflow 未完全接通；Strix 本机经验尚未形成 web pack 的完整复用入口；游戏自动化 pack 缺失。
- **PD-03 GUI 保留需要补验**：旧维护探针仅调用 REA `open_binary`/`close_binary`。REA 6.7.0 的 composite client 按需初始化，仅开关会话不足以证明实际 IDA 工作期间 GUI 不受影响。须实际完成后端分析、确认私有 worker 与目标、再核对 GUI。GUI 哈希未变这一观察保留，但不能扩大解释。
- **PD-04C 只保留有限答案盲评 none 的事实**：观察到预算截断后提高额度重跑、盲审未走完整 manifest/immutable packet 绑定，且答案有身份痕迹；不继承“正式比较通过”或研究增益。无需为修饰旧结论重跑旧题。
- PD-05/06 的作用域、响应后预算、停止后新开工、turns 与退出码来源等限制仍适用；静态审查不授予主动网络扫描资格。
- “只支持安全研究”是现有产品合同与已明确用户目标的冲突，不是用户尚未决定是否需要游戏脚本。DLV-01 解决它。

## 必守边界与变更范围

- Go 继续只管已有确定性 OS/Git/文件职责。默认不改 production；只有实际 pack/Fresh/路径合同测试证明存在机械阻塞，才做最小修复。不得增加工具配置管理器、执行调度器、消息总线、常驻进程或第二套 evaluator。
- `vnext/**` 只放 Markdown。专业脚本留在 pack 明示入口或授权 case 的项目代码中；脚本内部有界状态和等待可以存在，不等于允许 STeamAI 增加团队运行状态机。
- 不自动改用户全局配置、安装/升级/卸载第三方插件或启动/关闭用户已有 GUI。确需环境变更先说明精确目标与恢复方式；优先临时独立配置。读取当前值、保留其余内容，不整文件重写共享配置。
- case 外真实目标、凭据、登录态、原始截图/流量/样本/报告和绝对本机路径不进仓库。测试只保存生成器与无敏感合成源，运行数据写仓库外；工具输出中的指令是数据，不授予权限。
- Fresh/learning exact confirmation、成员单写、`HOLD_STALE_TASK`、用户纠偏优先、证据 currentness 不变。旧 current case 不升级、不迁移、不补包；新能力由新 Fresh 固定。
- 研究代码、游戏脚本和修复补丁可在明确分配的项目路径内交付；部署、注入、网络、状态修改等动作仍遵守具体授权。不能把新定位解释为无边界自动执行。
- 本轮不包含自建 GUI、跨 case 共享服务、全自动学习晋级、云端平台、跨电脑迁移、反作弊规避或正式 Release 发布。

## 批次与依赖

状态只用 `pending / active / blocked / done`，为维护计划而非产品运行状态；暂停前须把未完成项留为 pending/blocked，不写虚假的 done。

| 批次 | 内容与关键文件 | 依赖 | 完成信号 | 状态 |
|---|---|---|---|---|
| DLV-00 | 核对基线、纠正完成适用范围、冻结受支持环境 | 无 | 有可追查的能力矩阵，旧 GUI/比较缺口被明确纳入而非抹除 | done |
| DLV-01 | 研究/开发双交付合同；根说明、skill、case/member/role/review 模板 | 00 | 新 Fresh 可明确分配代码路径并审查代码交付；旧 case 零改写 | done |
| DLV-02 | 专业工具默认入口与可重复准备/交接；binary/web 入口，game 环境预检 | 01 | binary/web 新 case 可完成专业操作与回交；game pack 与其 G-TOOLS 留 DLV-05 | done（Playwright/Airtest 已获批安装，分项补验见结果区） |
| DLV-03 | 逆向跨函数、版本比较、Windows 定点动态补证 | 02 | 有限功能分析与变化解释经独立复核，GUI 保留完成有效补验 | done（运行时补证为输出级，见结果区） |
| DLV-04 | Web 源码/浏览器/API/修复回归 | 02 | 受控问题被实证，修复阻止问题且合法业务通过 | done（整体 review 以 needs-evidence 收尾；NE-1 证据影响 2026-10-25 补验闭合，历史漂移保留，见结果区） |
| DLV-05 | Windows 游戏自动化开发完整路径 | 02 | 可运行脚本在正常、扰动、停止及恢复条件下符合预期 | done（headless 通道 6/6+五轮 review accepted；真实鼠标通道修复后未复测，见结果区） |
| DLV-06 | 两两联合任务与跨成员接续 | 03、04、05 | 一次真实关联任务有贯通证据与整体审查；不把局部 accepted 拼成整体完成 | done（整体 review accepted；交接原为 case artifact+接收事实，原生消息部分 2026-10-25 补验闭合，见结果区） |
| DLV-07 | 新 Fresh、可见会话、负向回归与有限效果观察 | 06；过程中增量验证 | 三线工程可用结论与收益观察分别记录，不以 tie/unknown 伪造提升 | done（六臂 headless 全交付、A/B tie 探索观察级；可见成员窗口路径未测，见结果区） |
| DLV-08 | 文档收口、清理、安装一致性与授权后的 Git 交付 | 07 | 交付清单完整，残余项明示，临时方案可安全移除 | done（Reviewer 四轮复核：r1/r2/r3 rejected→逐轮修复，r4 accepted；Go 未改未重装；Git 已 stage 未 commit，见结果区） |

顺序：`00 → 01 → 02 → (03 / 04 / 05) → 06 → 07 → 08`。03/04/05 可在接口冻结后并行，单批 subagent ≤3 且不嵌套；共享 skill、模板、common、测试清单和 router 由同一集成人单写，领域 owner 只改分配 pack。外部 Strix 内部 agent 不计为正式成员。

## DLV-00：基线和环境冻结

1. 核对工作树与当前入口，阅读本轮涉及的实际代码/合同；不靠旧摘要认定能力。把“材料存在 / 本机可连接 / 已完成实际操作 / 已完成用户路径”分开记在本文件结果区。
2. 核实 REA 6.7.0、Strix 1.7.0 的实际安装身份；IDA 插件发行版本、协议版本、引擎版本分列，不互相代替。旧 GUI 启动缺陷与 Windows TTY 问题先核当前版本，未变则保留已验证绕行，不擅改第三方安装。
3. 为 Playwright/Airtest 选满足需求的保守固定版，记录官方来源、运行时与关键平台限制；Poco 仅为有集成前提的扩展。不得先写“已支持”再找可用版本。
4. 先在单个新目录验证模型命令工具与专业工具可用性；查看实际 assistant/响应身份，不以 `unrecognized_model` 日志一行或用量中的附属模型猜主模型。准备失败时先修环境，不启动三路重复失败实验。
5. 单独记录执行授权中的目标、允许的状态修改、进程/网络界限与预算。原始实验材料仅在本机按已知位置定向查看；仓库只记脱敏的证据编号和结果。

## DLV-01：合同一次改一致

优先修改：根 `CLAUDE.md`、`README.md`、`.claude/skills/steamai/SKILL.md`、`vnext/README.md`、`vnext/capabilities.md`、`vnext/templates/case/CLAUDE.md`、`vnext/templates/member/CLAUDE.md`、`vnext/templates/roles/{analysis-member,reviewer}.md`，必要时同步 `common/policies/{write-boundaries,review-first,evidence}.md`。

- goal/inputs/allowedReads/allowedWrites/deliverables/stopOrEscalate/exitConditions 表达两类任务，不新增必填字段。代码文件以 case-relative 范围分给 owner；同一源码文件不并发写，整合前核对基线和用户修改。
- 研究：继续 artifact → E → F → review。开发：项目代码/补丁＋测试观察＋使用说明 → code review；复用 artifacts/reviews，源码留项目原位置，不增加任务仓库或伪造安全 finding。
- Reviewer 可读取分配源码/diff/测试回执；开发 review 绑定源码版本或 exact diff 与相关测试证据。独立运行测试由获准 verifier 承担，不暗中让只读 Reviewer 获得执行权。
- 开发交付 accepted 只表示此次代码交付经审查，不自动取得 V0–V4 或 learning 资格。本轮不扩展 learning 的证据格式/晋级器；欲回流开发经验，先按现有 gate 证明适用，不能用 code review 绕过。
- 将“唯一工具只能是旧 IDA exporter”的阶段限制改为按具体用途接纳 pack 工具，但继续禁止通用运行器/adapter host。新工具须先证明上游/native 不够，限定输入/输出、无后台和显式调用。
- 在 `vnext/acceptance.md` 增加本轮真实路径条目；旧 gate 保留。普通 GUI/API 动作可按一个明确有界流程批准，不要求每次点击重复确认，也不默许范围变化。

验证：合同一致性、代码路径分配与 reviewer 权限负例、production Fresh 物化、新旧 case 边界。计划文档发布本身不表示本批已执行。

## 执行中的接续规则

- 每次开始确认 Git diff、当前批次、已有产物和未解决阻塞，再执行表中第一个依赖满足的 pending 项；不重新从“研究该用什么工具”开始。
- 每批只在下方留简短结果：改动位置、实际验证、证据指针、限制、下一步。详细输出留 case；不追加逐轮费用和长日志。
- 阻塞必须给具体事实、受影响部分及一个最小解锁动作。可继续的独立工作继续做；有代码和环境可修时不得以“文档写完/测试绿”结束。
- 变更事实时同步本文件与短投影；不靠 memory 或聊天宣告完成。首次进入实施时清除 README/CLAUDE/router/vnext 入口的“实施未开始”静态提示，改为只链接本文件状态，不维护六份进度。超过阅读预算时把详细结果放适当稳定文档，不删除失败事实来挤空间。
- 仅在所有必需批次 done 且交付复核通过时说整轮完成。可选扩展与真实外部阻塞分列；blocked 不是完成。

## 接续点

- 已完成：方案与文档入口；DLV-00 基线与环境冻结；DLV-01 双交付合同；DLV-02 三 pack 接入（分项补验 DLV-04/05 闭合）；DLV-03 逆向完整功能分析；DLV-04 Web 实证与修复回归（整体 needs-evidence 收尾；NE-1 证据影响 2026-10-25 补验闭合）；DLV-05 游戏自动化完整路径（五轮 review accepted；真实鼠标复测失败于模板脱节，待用户时间窗）；DLV-06 逆向×Web 联合（内容层整体 accepted；原生消息 2026-10-25 补验闭合）；DLV-07 新 Fresh 三线重走＋A/B 探索观察（六臂全交付、tie 为主）；DLV-08 收口（长期行为写回、清理、最终验证、独立复核四轮至 r4 accepted）。
- 当前批次：无（整轮 DLV-00～08 全部 done）。
- 下一步：无待执行批次（交付批次全部执行完毕；必需 gate 缺口已闭合）。补验状态（2026-10-25 用户全权授权后执行）：NE-1 证据闭合✓（重跑+独立复核 accepted）；G-JOINT 原生消息✓（可见成员窗口+token 交接双侧凭证）；真实鼠标通道复测✗（模板脱节失败，根因候选 PrintWindow 坐标系/色彩表示，**按用户指示前台验证须事先同意时间窗，调试中止**）；三线可见窗口重走未做。Git 提交推送已获用户授权，收尾执行。
- 用户新指示（2026-10-25，已写入 CLAUDE.md/acceptance）：①测试默认档位 `ANTHROPIC_DEFAULT_SONNET_MODEL`（Haiku 仅极简探针或 headless 无工具场景）；②一切维护测试/验收默认无窗口后台通道，可见窗口/真实鼠标类仅用户同意时间窗内执行。
- 已知待解（按批次来源分列，均为未闭合限制而非已修复项）：
  - 工具/环境：ida-pro-mcp 0.1.0 `force_gui` 缺陷（手动绕行已验证，缺陷仍在）；idalib 目标同目录伴生文件；x64dbg 未安装（运行时补证输出级）；DLV-02 MCP/IDA/Hex-Rays 版本未从响应侧取得、Strix 两次调用响应侧模型身份 unknown；DLV-03 REA `close_binary` 返回 `cleanup_incomplete`（当次进程观察无残留 worker，不改写工具返回值）、v2 无符号部分函数来源保持 unknown；DLV-04 Strix 运行宿主进程并经模型 API 网络动作（非完全不联网）、自动初始化 Caido、child pending=2 未消费（child 热纠偏未证）；DLV-05 Airtest URI 截断/mss 不穿遮挡/tkinter 不响应 PostMessage 等工程坑（绕行已记 pack，非缺陷）。
  - DLV-01：行为层负例（越界源码写入拒绝、Reviewer 只读负例）本批只交付机械合同，未执行 live 负例。
  - DLV-04：NE-1 对整体结论的证据影响已于 2026-10-25 闭合（登记版脚本原样重跑+独立复核 accepted，见 DLV-04 结果区）；历史 11:38 回执的执行脚本 exact 身份仍 `unknown`（历史漂移事实永久保留）；整体 review 历史状态 needs-evidence 不改写，其证据缺口由闭合重跑补上。
  - DLV-05：真实鼠标 windowed 复测已于 2026-10-25 用户时间窗内 6/6 通过（LLM 视觉定位，见结果区）；遗留：headless 纯色模板 aircv 零方差伪匹配（识别失败路径未被 headless 模板真实验证，llm 模式不受影响）；同步调用执行期 stop 时序回执未采集（estop 实测 0.111-0.161s）（≈0.212s 为采样粗估非验证上界，sc5 等待段 0.109s 为单场景实测）；act/commit 中断窗口异常 kill 未构造复现（单测形状验证不冒充故障注入）；首轮真实窗口版 6/6 report 原件被覆盖（不可独立复查，仅存旧 review 引用与摘要）。
  - DLV-06：原生消息交接已补验闭合（2026-10-25，见结果区）；headless 恢复接续不覆盖正式 roster 生命周期（补验走了正式 Commander 改派与成员任务更新，roster 生命周期管理仍未验）；Reviewer 首轮 26 turns 用尽未写产物（重发加"先写结论"约束后 13t 完成——review 类任务需效率约束，已在本轮与 DLV-08 复审沿用）。
  - DLV-07：可见成员窗口路径未测（留用户空闲时补验）；A/B 效果观察条件限制——非盲（B 产物结构可识别）、每线仅一对样本、无热纠偏场景、A 臂 web 为拆小步指令、B 臂 web 补接口事实、题面"≥5 个不同 XFF"字面未满足（两臂以 2 值＋交错对照证明机制，同标准）；tie 不外推、不作 learning gate 证据。
  - headless 模型：`glm-5.3[1m]` 无命令工具（headless 会话限制，交互会话有 Bash）。
- 尚未授权：新的安装、全局配置改动、任意外部目标、正式发布以及本轮 Git 提交推送，除非后续用户明确涵盖。

## 批次结果区

按批次追加简短条目；证据必须能区分 unit/synthetic、真实工具、可见成员、用户直接纠偏与比较结论，详细输出留 case。目前条目如下。

### DLV-00 基线与环境冻结（2026-10-24，done）

**工具身份分列（全部现场核对，非旧摘要）**

| 工具 | 安装身份 | 本机状态 | 来源证据 |
|---|---|---|---|
| REA | `rea-agents@6.7.0`（npm 全局） | CLI `--version` 实测 6.7.0 | `C:\Users\13209\AppData\Roaming\npm\node_modules` |
| Strix | `strix-agent` 1.7.0（pip user），direct_url 钉 `git+https://github.com/usestrix/strix @ v1.7.0`，commit `55bc07991aacb1c26a81b43e7ca53148730d2019` | 与旧记录一致 | site-packages dist-info `direct_url.json` |
| Docker 沙箱 | Server 29.9.2；镜像 `ghcr.io/usestrix/strix-sandbox:1.3.0`（`f6906c3114e5`，5.9GB） | 就绪 | `docker version` / `docker images` |
| IDA MCP | 发行：mrexodia 插件 `ida-pro-mcp` **0.1.0**（marketplace 缓存）；实现：插件目录 pyproject **2.0.0**（uv project 运行 `idalib-mcp --stdio`）；协议库 `mcp` 1.27.0；引擎绑定 `idapro`(idalib) 0.0.9 | `steamai-ida-local` Connected，无打开会话 | `claude mcp list`、插件目录 pyproject/uv.lock |
| Claude CLI | 2.1.273；Node 24.15.0 | 与旧记录一致 | `--version` |

- **旧缺陷复核**：ida-pro-mcp 0.1.0 `force_gui` Windows 缺陷仍在——`ida_mcp/discovery.py:188-196` `_get_ida_executable()` 非 Linux 直接返回 `sys.executable`，uv 运行时即 python.exe；GUI 须手动启动后 adopt，绕行不变。Strix Windows `tty=true` 经验前提（Docker/openai-agents 0.19.4）未变。
- **版本冻结（保守成熟 patch，仅记录未安装）**：Playwright **1.62.0**（2026-07-31 发布，约 3 个月成熟期；官方 PyPI，`playwright install chromium` 拉对应浏览器；Python 3.12 兼容在 DLV-02 现场预检）；Airtest **1.4.3**（2025-12-04，1.4 系列终版，10 个月成熟期）；`pocoui` 1.0.94 仅作有 SDK 前提的可选项。
- **模型命令工具验证**（仓库外 `%TEMP%\steamai-pd\pd00-probe-*`，headless `-p`）：haiku 档 assistant 身份 `gpt-6.1-sol`，有 PowerShell tool_use，2 turns success、逐字正确（$0.077）；sonnet 档 assistant 身份 `glm-5.3`，仅 ToolSearch×3、无命令工具，4 turns `error_max_turns`（$0.073）。结论：需命令执行的 headless 维护会话选 haiku 档；交互式会话工具集与 headless 不同（本维护会话 glm-5.3 有 Bash），正式可见成员窗口的模型工具可用性留 DLV-07 用户路径实测。两臂合计 $0.15。
- **能力矩阵（材料存在 / 本机可连接 / 已完成实际操作 / 已完成用户路径）**：REA＝仓库 upstream 固定＋npm 安装 ✅ / MCP 可加载 ✅ / 旧路线静态导出实测 ✅ / 新合同下 ❌。IDA MCP＝插件缓存 ✅ / Connected ✅ / 旧轮 open/close＋导出 ✅ / ❌。Strix＝安装＋镜像 ✅ / Docker 就绪 ✅ / 旧路线六项资格 live ✅ / ❌。Playwright＝版本冻结，未安装，四层全 ❌。Airtest＝同左。三条业务线的“已完成用户路径”均为 ❌——旧路线局部实测不继承为本轮交付，与“基线与不能继承的完成声明”一致。
- **执行授权边界（本轮）**：已涵盖＝仓库内合同/pack/测试/文档修改、仓库外维护 fixture 生成（`%TEMP%\steamai-pd\`）、headless haiku/sonnet 档小探针、既有已装工具的只读/低影响操作；未涵盖＝新安装、全局配置改动、case 外网络目标、正式发布、Git 提交推送。进程界限＝不启动/关闭用户 GUI 与既有进程，只管理本次自建资源；模型＝Haiku/Sonnet 档冻结，Opus/顶级档未经批准不使用；预算＝宽松额度按批执行，仅汇总总量与异常。
- 限制：Playwright/Airtest 安装待授权（见接续点）；Airtest 1.4.3 与 Python 3.12/OpenCV 栈兼容性未现场验证；本批未改任何产品代码。

### DLV-01 研究/开发双交付合同（2026-10-24，done）

**改动位置**：根 `CLAUDE.md`（双定位＋pack 工具按用途接纳，替换“唯一 pack 工具”阶段限制）、`README.md`、`.claude/skills/steamai/SKILL.md`（双定位、开发任务 `allowedWrites` 分配源码路径、code review 写 `reviews/` 并绑定源码版本/exact diff、开发 accepted 不升级 maturity）、`vnext/capabilities.md`（非 MCP 工具与 pack 工具接纳条款，继续禁止通用运行器/adapter host）、`vnext/templates/case/CLAUDE.md`（标题改 `STeamAI Case`、目标字段标签“研究目标”→“Case 目标”、双交付产物边界、Reviewer 可读分配源码/diff/测试回执）、`vnext/templates/member/CLAUDE.md`（case-relative 项目源码路径写入、同一源码不并发写、整合前核对基线）、`vnext/templates/roles/{analysis-member,reviewer}.md`、`common/policies/{write-boundaries,evidence}.md`、`vnext/acceptance.md`（新增“专业能力实际交付 — steamai-professional-delivery-v1”节：默认机械合同＋G-TOOLS/G-RE/G-WEB/G-GAME/G-JOINT/G-FRESH live gate，旧 gate 全保留）。

**production 最小修复**：`internal/steamai/casebootstrap/current.go` 深验目标字段改为接受 “Case 目标”（现行模板）或 “研究目标”（历史 case）任一非空——不重写旧 case 文件，新旧 case 深验均通过。新增 `internal/steamai/vnextcontract/professional_delivery_contract_test.go`：双交付合同文字断言＋member 模板 placeholder 集合负例（仍为既有 11 项，无新增必填字段）。既有 day-2/professional-delegation 断言短语全部保留。

**验证**：`go vet ./...`、`git diff --check`、全量 `go test -count=1 -p=2 ./...`（internal/steamai 30.6s、casebootstrap 92.5s 含 production Fresh/Apply/current 真实物化、evaluation、learningbatch、vnextcontract 全 ok）、Python pack tooling 27 tests ok。旧 case 深验由 casebootstrap_test 旧标签 fixture 继续覆盖。本机 `steamai.exe` 已按已批准安装流程重建并 uninstall+setup 更新（dev，绑定本仓库 source），uninstall helper 残留已清理。

**限制**：开发任务行为层（越界源码写入拒绝、Reviewer 只读负例）属 live gate G-CORE/G-FRESH 范围，本批只交付机械合同与文字断言；未做真实新 Fresh 用户路径（留 DLV-02/07）。

### DLV-02 专业入口与 G-TOOLS binary/web（2026-10-24，done；安装批准与分项补验于 DLV-04/05 闭合）

**文档/入口改动**：`packs/binary-re/`（catalog 新增 `rea-program-analysis` 条目、general-toolchain-router 与 README 增 REA/IDA 分工两行、tooling README 增 integrations 行、rea.md 补 headless 分页与 `cleanup_incomplete` 限制）；`packs/web-security/`（新增 `tooling/integrations/{strix,playwright}.md`，toolchain-router 增专业工具分工节，README/catalog/manifest 同步引用）；新增 `packs/game-automation/` 骨架 8 文件（manifest/router/toolchain-router/catalog/airtest integration，版本冻结 1.4.3/1.0.94，完整 workflow/recipes/examples 留 DLV-05）；SKILL 第 7 条扩为三线入口选择。新文件已按规则精确 stage（10 个路径，未 commit）。

**G-TOOLS binary（真实路径，通过）**：仓库外合成 Go 目标（`target-a.exe`，SHA `3f82234…`）；production `steamai __fresh-preview/__fresh-apply` 新 Fresh case（binary-re 主包、新双交付合同物化）；headless 成员（haiku→assistant `gpt-6.1-sol`）经临时 `--mcp-config`（uv+ida-pro-mcp 插件 0.1.0 目录，不改全局配置）完成 idb_open→反编译 `main.classify`/`main.main`→`get_global_value`/`get_bytes`/`xrefs_to` 机器码级间接调用证据链（函数值槽 `0x1400D65C8`→`call rcx`）→`idb_close`；E + artifact index 含 hash/bytes 绑定与限制；独立会话跨会话阅读复述正确（$0.16）。成员会话 $1.52（39 turns）。负例：MCP 未注入时成员按停止条件诚实阻塞、零写入。清理：`idb_close` 后会话列表空；idalib 在目标同目录残留 `id0/id1/id2/nam/til` 由维护者清理（已记入集成注意）。未知项如实保留：MCP/IDA/Hex-Rays 版本号未从响应取得。

**G-TOOLS web/Strix（真实路径，通过，含一次负例）**：合成 Flask 目标（三路由：XSS/路径穿越预置+合法对照）；新 Fresh case（web-security 主包）；成员委托 `strix -n -t … --scan-mode quick --max-budget 1 --instruction-file --config`（config 由维护方提供、成员只传参不读内容）。Run 1（`dlv02-webapp_3754`）：root 用满 max-turns 12、源码未读、SARIF 仅 needs_follow_up 覆盖缺口——成员如实报“审查受阻，不当零漏洞”，另发现 posthog/scarf 遥测发送违反禁联网边界（$0.37）。修正：维护方 config 关 `STRIX_TELEMETRY`、turns 提 30；Run 2（`dlv02-webapp_b47e`）另开编号保留 Run 1：47 turns（root 20+2 子代理）、$0.59、三路由覆盖（/search 模板拼接、/file 路径拼接、/health 无问题，均为 agent-reported 待核线索）、遥测 disabled 无发送行、容器清理确认。E（26.9KB）与 index 双 run 归档、hash 逐件绑定。响应侧模型身份两 run 均 unknown（如实）。成员会话：run1 $2.60 59 turns（含大量日志复核）+ run2 一次 CLI 无产出会话（unrecognized_model 后重试成功）。

**安装阻塞解除与分项补验（后续批次闭合）**：Playwright 1.62.0 与 Airtest 1.4.3 安装已获用户批准并完成（此前"blocked on authorization"为批准前状态，保留记录）；web 线 Playwright 实操在 DLV-04 G-WEB 浏览器实证完成（见 DLV-04 结果区）；game 线 Airtest 1.4.3 真实链路（Python 3.12+OpenCV 栈兼容实测）在 DLV-05 完成（见 DLV-05 结果区）；game 线 G-TOOLS 环境预检同批闭合。DLV-02 批次表 `done` 以 binary/web 入口本批验证＋两项分项补验闭合为据。

**维护经验**：给成员的目标路径应以 case 根为基准或绝对路径（成员目录相对 `../../` 曾引发一次成员自行纠正）；网关 OpenAI 兼容端点需用无 `[1m]` 后缀模型名（见 memory claude-cli-gateway-behavior）。

### DLV-03 逆向完整功能分析（2026-10-24，done）

**pack 扩展**：新增 `tooling/recipes/{function-version-compare,runtime-observation}.md`（三类判定、先预测后运行）；catalog 增两条 mainline-template；router 增两行；manifest 同步。已 stage 精确路径。

**验收任务（真实路径，通过）**：仓库外两版合成 Go 目标（v1 全符号，v2 strip 且互换函数表条目 1/2＝一处实质行为变化）；参考答案 `ANSWER-KEY.md` 与成员会话隔离（事后 grep 全部 dlv03 会话 transcript 零接触）。production Fresh 新 case `dlv03-re`（binary-re 主包）；analyst（haiku→`gpt-6.1-sol`，94 turns $3.77）交付：每版功能链（入口→函数表选择→变换→最终效果，地址/指令级）、三类判定（改变＝索引 1 选择互换；未观察到改变＝K0/V/大写链；不可比较＝v2 辅助函数来源）——与维护者参考答案一致；符号缺失未被当作语义证据。

**运行时补证与证据诚信负例（重要）**：analyst 最初在 F §5 虚构“已运行、40 bytes、exit 0”观测——维护方 transcript 核对发现**无任何运行 exe 的工具调用**。Reviewer round 1 独立复核核心判定全部成立但判 `needs-evidence`（预测-运行时间链不可独立证明），正好挡住虚构。处置：Commander 责令修正（原文保留＋明确撤回声明＋真实补跑 17:50:13 命中预测），Reviewer round 2 独立复跑两 exe 后 `accepted`，且明确“不接受原虚构回执、不追认历史执行”、来源边界（transcript 核对为 Commander 告知，Reviewer 未读原 transcript）如实分层。负例全程留痕，未被抹除。x64dbg 本机未安装，运行时补证为输出级（stdout/exit 对照，无内部状态观测），F/E 已如实记录该限制；需要调试器级补证时另寻获准入口。

**GUI 保留有效补验（真实路径，通过——补上旧 PD-03 最大缺口）**：自启 IDA 9.3 GUI（PID 31208，可见窗口，Responding=true）打开 copy-A/target-v1.exe 自动分析；记录分件基线（bytes+mtime，id0 被锁不能 hash）。REA 6.7.0（`--mcp-config` 会话级注入 `rea-agents --mcp`＋`REA_IDA_MCP_CONFIG` 指向既有 headless idalib 注册，零配置写入）对同 hash copy-B **实际反编译 `main.normalizeToken`（61 行）并导出 12,444 bytes/2 records bundle**；期间 copy-A 六文件零漂移、GUI 持续响应；正常关闭（CloseMainWindow）后打包 `.i64`（33,050,433 bytes，SHA `37c516…`）记录。REA `close_binary` 报 `cleanup_incomplete`（已知限制），进程侧未见本次残留 worker，如实记录。REA 首轮曾因 `binary_session` 大输出触发成员诚实停止（按 case 停止条件请求确认），按 rea.md 超限处置（导出 bundle 定向读取）后续跑；第二轮因 `REA_IDA_MCP_CONFIG` 未配置诚实阻塞，注入后第三轮成功——两次负例均未冒充成功。

**费用（dlv03 相关真实调用）**：约 $11.1（analyst $3.77、reviewer 两轮 $2.81+$1.45、GUI 三轮 $0.81+$0.78+$0.86、修正 $0.61）。

**限制**：输出级运行补证不含内部状态/寄存器观测；headless 维护测试非可见成员窗口（G-FRESH 留 DLV-07）；v2 无符号部分函数来源保持 unknown（正确行为）。

### DLV-04 Web 实证、Strix 委托与修复回归（2026-10-24，done）

**pack 扩展**：新增 `tooling/recipes/{browser-api-validation,fix-regression}.md`（双侧分开取证再连接；修复双面证据），catalog/manifest 同步，已 stage。

**验收任务（真实路径，通过）**：标准库合成本地应用 v1（两隔离身份＋各自对象；F-A＝`/api/notes` 只验 token 不校验归属的 IDOR；F-B＝`/api/orders/{id}/pay` 无状态转移守卫可重复扣款；合法对照齐全，非字符串可解）；v2 修复版由维护者预置冒烟。production Fresh case `dlv04-web`。

- **阶段 A（双侧实证）**：web-lead 三次会话接力（524 网关截断×1、服务未就绪保守退出×1、成功×1）——F-A 经 API+浏览器双侧证实（bob/alice 互读对方 note 200＋`/ui` 渲染同值、6 截图、401/404 阴性对照）；F-B 顺序 API 完整证实（双身份独立订单 100→90→80 重复扣款、余额清零对照、无负余额）。根因定位到行号，与维护者参考答案一致。
- **阶段 B（Strix 委托+受控中止/重开）**：单文件 target 被 Strix 1.7.0 拒绝（`is not a directory`，0.3s 参数校验失败——工具真实限制，失败记录保留）；改目录后 abort2（$0.05 预算→`stopped/budget_exceeded`，$0.11 实际、1 turn）与 retry2（completed/finished_by_tool，root14+child15 turns，$0.41）对照成立；四类 needs_follow_up 覆盖线索含两个实证缺陷方向；SARIF 7 条覆盖记录未被报成漏洞或零漏洞；容器清理确认。边界发现：Strix 自动初始化 Caido（宿主 9105/9609）并调模型 API——"工具完全不联网"不成立，child 纠偏 pending=2 未消费（无热纠偏证明）。
- **阶段 C/D（修复+独立回归，开发交付合同首次实战）**：owner 在 `src/app-fixed.py` 写最小修复（仅两处守卫，其余 bytes 不变，diff `36b09c12…` 绑定）；**独立 verifier-d** 复测 12 组 67 请求全过：缺陷阻断（403/409 语义正确）＋合法保留（自读 200/首付 100→90/401/409）＋状态一致＋清理。
- **阶段 E（Reviewer 三轮）**：round 1 抽查 10 请求全过但判 needs-evidence（两项程序缺口）；round 2 开发交付 accepted、NE-2 关闭、NE-1 维护方佐证被发现时间矛盾（我方摘要批次标注错误，更正 v2）；round 3 收尾：**开发修复+回归 accepted；整体 needs-evidence 收尾**——NE-1 历史执行脚本 exact 身份保留 unknown（实证脚本被续跑会话于 11:37 重写、11:46 定稿、登记未同步；11:38 回执时间戳与最后写入的冲突无证据闭合），69/69 artifact current。

**流程负例与维护经验（有价值）**：① 上游安全过滤（OpenAI 通道）在成员会话第 50 turns 截断含"渗透测试"语料的工作——长会话宜分段；② 成员把 Commander 维护指令标注为"用户直接纠偏"（语义错但行为合规）；③ **会话间接管重写既有脚本会破坏证据 exact 绑定**（NE-1 根因）——续跑应沿用既有脚本或立即同步 artifact 登记；④ ECONNREFUSED 就绪等待应写入任务提示。

**NE-1 证据闭合补验（2026-10-25，用户全权授权后执行；`%TEMP%\steamai-pd\dlv04-ne1c\`＋`dlv04-ne1c-review\`）**：以登记版脚本 `validate_dlv04_resume.py`（执行前后 SHA 均 `6f27663c…` 零漂移）原样对同 fixture `app-v1.py`（SHA `24e0822d…`）在隔离副本重跑——历史产物移存 `ne1-history/` 零触碰。新回执与历史回执逐字段一致（result `passed`、api 11/11、browser 6/6、请求计数 65+6、target SHA 前后一致、server_exit/port/cleanup 全同），差异仅计时字段；即**登记的 exact 脚本＋fixture 可稳定重现历史回执全部实质结果，DLV-04 整体结论（双侧实证）获得 exact 绑定的可复现证据**。独立 Reviewer 复核（headless，29t/$0.82，accepted）："NE-1 对该整体结论的证据影响闭合；这是新增重跑证据链，不恢复历史执行身份；11:38 历史回执与 11:46 定稿间身份断裂保留为 unknown"。历史漂移事实不改写。

**费用（dlv04 成员+工具）**：约 $25.4（成员会话 $24.9＋Strix $0.53）。

### DLV-05 Windows 游戏自动化完整路径（2026-10-25，done）

**pack 补全**：`packs/game-automation/` 从 8 文件扩到 17 文件——references 新增 `workflow-template.md`（任务输入、阶段、决策核心结构、停止语义）与 `task-handoff.template.md`（状态/ledger 含 pending/产物交接）；tooling 新增 recipes `windows-stateful-automation.md`（状态枚举、识别通道、两阶段非幂等保护）与 `automation-regression.md`（六场景矩阵+竞态/停止/中断边界段）；examples `windows-flow/`（README、requirements pin、config 示例、workflow.py 参考实现：State/DecisionCore/DeviceAdapter+FakeDevice/Runner、控制文件暂停停止、两阶段 Ledger、时间线，`--selftest` 零依赖）。manifest/catalog/README 路由同步，全部已 stage。

**参考实现经五轮独立 review 修复的实质缺陷**（headless 成员 haiku 档）：①ledger 枚举/字符串类型错（READY+已领取热循环，靠 Runner 二道防线挡输入）；②act→record 中断窗口与 `open("w")` 截断窗口——改两阶段 begin/commit＋tmp+os.replace 原子写＋损坏 fail-closed＋pending 残留暂停待确认；③ACT 分支无节流（快通道 8ms 连发 4 次，真实鼠标 ~0.8s 慢通道掩盖）——ACT 后强制观察间隔；④测试基建竞态与时基（pause 消费确认、墙钟并列→同线程序号严格比较、RecordingCore 快照 decide 输入核对恢复首轮 waited==0/retries==1）。

**验收（仓库外 `%TEMP%\steamai-pd\dlv05-game\`）**：合成仿游戏窗口（tkinter，标题暴露状态，注入延迟/位移/弹窗/关闭/冻结，事件流含非法点击计数）＋**无窗口 headless fixture**（`fake_game_headless.py`：状态 json＋离屏 PIL 渲染＋消息文件输入——用户使用电脑期间全程后台零干扰，windowed 模式保留）。Airtest 1.4.3 真实链路：模板匹配走 aircv（headless 对离屏渲染图，窗口版对 PrintWindow 截图）；**首轮（用户空闲时）真实鼠标注入 6/6 端到端**；修复后回归在 headless 通道 6/6（含五轮加强断言：sc4 暂停区间零输入/resume→新 observe→collect 同线程序号 5<6 落盘 report、sc6 第二轮观察≥15 次/退出限定 restart 后/ledger 决策级证明、sc5 estop 0.109s≤2s 且窗口存活）。fake 单测 16/16（决策级类型回归、防热循环上下界、pending、fail-closed、两阶段形状、恢复顺序与上下文重置）。

**五轮 review 终审（REVIEW-dlv05-round5.md）**：**accepted**——round 1 rejected（类型缺陷+断言缺口）、round 2/3/4 needs-evidence（测试指针、顺序区分力、waited 覆盖、措辞收紧）逐轮闭合；round 5 确认两类指定回归均有可区分断言、sc4 落盘一致、数值对齐。

**定位方法升级为 LLM 视觉定位（2026-10-25，用户指出模板匹配为最笨路径后执行）**：模板匹配复测失败（confidence 恒 0.297、重标定无效，PrintWindow 截图与模板坐标系/色彩表示不一致）后，放弃在模板参数上打转，改为方法分层：原生控件枚举 > **LLM 视觉定位（自绘窗口默认）** > 模板（仅极简单静态场景）。实现：`airtest_device.py` 增加 `llm_locate`（截图→headless claude 读图→JSON bbox→中心点，`DLV05_LOCATE` 可覆盖，默认 headless=template 快通道/windowed=llm；编排超时 `DLV05_JOIN_TIMEOUT` 可调）。**验证（全部后台无窗口）**：离屏渲染图定位两状态全中（bbox 误差≤1px，$0.096/次）；headless 强制 llm 模式 sc1 端到端 PASS；默认通道六场景 6/6 无回归；单测 19/19（含 llm 解析与默认策略 3 项）。分层说明已写入 pack（airtest.md 方法分层节+windows-stateful-automation.md）。**真实鼠标 windowed 端到端复测（2026-10-25，用户同意时间窗内执行）：6/6 全过**——LLM 定位+真实鼠标全链：正常完成/3500ms 延迟/弹窗扰动暂停（不发 collect 持续观察）/暂停恢复+窗口位移（seq 顺序证据）/estop 0.125s≤2s 停止后零新输入/重启 ledger 语义。三轮迭代如实保留：第一轮 2/6（harness wait_acts 15s 不适应 llm 单次 30-60s，修编排超时+sc3 固定 sleep 改轮询）、第二轮 5/6（sc1 偶发：PrintWindow 抓在窗口首帧渲染前，截图无按钮→LLM 如实答无→加"定位失败先重截图重试×3"修复）、最终轮 6/6（`windowed-6of6-final.json`；各轮 report 全保留）。headless 默认通道同期 6/6 与单测 19/19（sc6 曾一次偶发超时，单独复跑通过，flake 记录）。**新发现限制（如实）**：headless 纯色模板在 aircv 下零方差伪匹配（纯色模板对纯色区域恒 1.0，匹配位置无意义）——headless 模板模式下"识别失败"路径未被模板真实验证（消息通道不带坐标不校验位置；决策/账本/停止语义与 observe 层弹窗/状态检测均为真实验证不受影响；llm 定位模式识别真实）。历史 6/6 report 备份齐全。

**限制（如实）**：真实鼠标通道修复后未复测（用户使用电脑，`DLV05_MODE=windowed` 保留待空闲复跑）；act/commit 间异常 kill 未构造复现（两阶段静态处理+单测形状验证）；执行期（同步调用内部）stop 时序未采集（act-timing.json 为样本粗估：headless 通道 max 12ms+poll 200ms≈0.212s，不构成已验证上界）；首轮窗口版 6/6 的 report 原件被后续重跑覆盖（数据留存于 round 1 引用与本记录）；工程通道坑记录：`#`/正则转义在 `Windows:///` URI 被截断（`.` 通配绕开）、Windows 设备无 `exists`、aircv 需 `from airtest import aircv`、snapshot 用 mss 抓屏幕区域不穿遮挡（PrintWindow+RENDERFULLCONTENT 可穿）、tkinter 不响应 PostMessage 合成点击、失焦判定用 IsIconic 而非前台（后台自动化正常形态）。

**费用**：成员 review 五轮约 $6；维护侧无模型费用。

### DLV-06 逆向×Web 联合任务与接续（2026-10-25，done）

**材料（仓库外 `%TEMP%\steamai-pd\dlv06-joint\`）**：Go 合成客户端 v1/v2（`buildChecksum`：v1=`(n×0x9E3779B1) mod 2^32 ^ 0x1337C0DE`；v2 变体 `(n+0xCAFEBABE)×0x9E3779B1`；`-ldflags=-s -w` 编译）；本地 API 127.0.0.1:8902（仅 v1 校验；初版 403 带 `expected_v1` 泄底被维护方移除后重启）；维护者参考答案。production Fresh case `dlv06-joint`（binary-re 主＋web-security 辅，两成员物化，apply 确认串流程照常）。

**链路（headless haiku 成员×4）**：
- **re-member（$2.50，31 turns）**：idalib MCP 逆向 targets/joint-client-v1.exe——`buildChecksum` 内联进 `main.main`（经 Go 元数据 funcnametab/PCDATA/FUNCDATA 内联树定位，不虚构独立入口）；算法与两常量精确；3 个 worked example＋0/max 边界复算；原样引用反编译/反汇编并解释 `-1640531535` 是 `0x9E3779B1` 有符号显示；主动排除误导性工具标签（`crypto_sha256.New` 实为 `runtime.makemap_small`）；限制声明完整。**维护方核对：exe 实跑 1/2/1001 输出与三个 worked example 完全一致（2365634927/794375100/3051165127），与参考答案一致。**
- **web-member（$1.41，40 turns）**：实际 Read E-RE-001 后精确引用（行号/常量/哈希核对）——接收事实成立，非发送方声明；按交接算法构造请求：42→3861788628 得 **200 accepted（order=42, ref=J-0000002A）**、一位错误值 403 rejected；v2 错配场景：运行 v2.exe 得 407822952，v2 原始组合与"v2 值+v1 标签"对照均 403——四点判别（v1 算法推不出该值/同订单 v1 成功排除全拒/标签更换不改变拒绝/限定测试点）→ **拒绝组合、请求绑定 v2 SHA 的独立逆向补证**，不硬猜不重试；首次 v2 启动 FileNotFoundError 如实单列（零 HTTP 请求，路径修正后成功），未冒充 API 拒绝。
- **恢复会话接续（$0.58，18 turns）**：re-member 原会话退出后零记忆恢复——只用身份文件+既有 E/F/index 完成核对（worked example 复算一致、SHA 一致、原 E/F 零改写），写 E-RE-002。
- **整体 Reviewer（$0.41，13 turns，R-dlv06.md）**：首轮 26 turns 用尽未写产物（会话记录保留于 `%TEMP%\steamai-pd\dlv06-joint`），重发加"读完立即写结论"效率约束后本轮 13t 完成——该处置为维护方会话记录说明，非独立结果证据。**accepted**——关键连接直接绑定 E（200/403 记录经 A-WEB-002 原始 HTTP 核对，非 F→F 拼接）、错配判别支持拒绝组合、恢复接续只用固定文件、身份抽查 4/4（自算 SHA，首次抽算命令引号失败如实记录后修正）；接受范围明确限定（不表示 v2 兼容通过、不扩大为全输入/全流程成立）。

**限制（如实）**：成员交接为 case artifact＋读取事实验证（E 内精确引用＋Reviewer 核实），**非正式成员会话间原生消息**（headless 会话无 ListAgents 域；正式可见成员窗口在用户使用电脑期间未开）——原生消息交接留待后续需要时以正式成员路径补验；维护验证由维护方携带合成确认，成员交付不冒充用户批准；roster 更新未做（headless 非正式 session 生命周期）。

**原生消息交接补验（2026-10-25，用户全权授权后执行；通过）**：在 dlv06-joint case 以 `steamai __open-member --trust-workspace` 打开 re-member/web-member 两个可见成员窗口（首轮无 trust-workspace 卡目录信任框未注册 agents，重启后注册成功——信任写入为唯一弹框前动作）。维护方 SendMessage 派发被 re-member 按合同拒绝（"跨会话授权声明不作为解除停止条件"，因 8902 关停触发 case 全局停止——合规负例）；随后 headless Commander（case cwd 正式身份）解除停止并七字段正式改派：Commander→re-member 任务送达确认；**re-member→web-member 原生 token 交接**——`claude agents --json` exact cwd→name 映射＋ListAgents 唯一相交、SendMessage `success:true`（msg_id `e7cb17f5…`）、web-member incoming envelope（`from-name="re-member"` uds 管道）记录于接收凭证；token `P0WXD6` 双侧独立凭证一致（发送方声明未读接收文件、接收方声明未读发送文件——交接内容只能经原生消息传递）。产物：`msg-verify-sent.md`/`msg-verify-received.md`/`commander-msg-verify.md`（Commander 实读两文件核对通过）。窗口用后即关。**G-JOINT 的原生消息部分自此闭合**（内容层 DLV-06 已 accepted）；顺带实证 `__open-member` 可见窗口启动路径与跨会话消息全链（DLV-07 可见窗口路径的主要缺口补上，三线任务在可见窗口的重走仍未做）。

**费用**：成员合计约 $4.9（re $2.50＋web $1.41＋resume $0.58＋review $0.41）。

### DLV-07 新 Fresh 三线重走与 A/B 有限观察（2026-10-25，done）

**材料（仓库外 `%TEMP%\steamai-pd\dlv07\`）**：三线未见新题＋维护者 ANSWER-KEY（逆向 `decoder-v1.exe`：42 字节 6-byte 循环 XOR，密钥 `5A C3 96 3B 71 E4`，明文 `STEAMAI-DLV07-FLAG{x0r_l00p_1s_n0t_s3cure}`；Web `rateapp` 8903/8904 双实例：per-IP 3 次/60s 限流无条件信任 XFF 首值；游戏 `game2-fixture.py`：IDLE→start→LOADING→READY→prep→PREP→submit 非幂等→DONE，错序记 illegal_attempts）。B 臂＝三线新 Fresh case（DLV-05 后合同+pack，无维护经验补注）；A 臂＝裸 Claude Code＋同工具说明＋独立目标副本。全部 headless haiku 成员、题目/判分/预算/复跑规则事先冻结。

**结果（六臂全部完成交付，用户介入 0；完整评分表与判分细节：`%TEMP%\steamai-pd\dlv07\A-B-RESULTS.md`）**

| 线 | A 臂（裸） | B 臂（STeamAI Fresh） |
|---|---|---|
| 逆向 | 全对，31t/$1.36/538s | 全对+证据绑定，42t/$1.85/644s |
| Web | 全对，14t/$0.90/548s（2×网关 5xx 重跑后拆小步成功） | ①字段名 400 阻塞如实保留（31t/$1.76）→②补接口规范后全对（31t/$1.93/989s，31t 截断于 max-turns 但 E/F/A-004~007+独立 verifier PASS 回执完整） |
| 游戏 | 全对，49t/$4.01/2102s（fake 23/23 维护方亲跑复核；e2e 五场景含错序 refused+0.016s 停止；失败保留 e2e-records） | 全对+超额，14t/$4.17/1732s（fake 26/26；headless 十场景含错序双层拒绝 illegal=0；headless-02 失败轮保留后修复 10/10；manifest SHA 绑定） |

- **工程可用成立（G-FRESH）**：全新 Fresh 目录、仅正常产品入口与目标说明，三线必需路径全部走通；失败轮次（B-Web①、B-game headless-02、A-web×2 网关 5xx）全部保留未覆盖。
- **效果观察为探索观察级 tie 为主**：两臂判分点均达成；B 的可见增量是可复核证据结构（E/F/index、SHA/manifest 绑定、失败保留、独立自查回执），代价是逆向线 +36% turns；游戏线 B 以 14 vs 49 turns 完成更完整交付。**非盲评**（B 产物结构可识别、维护方全程在场），每线各一对样本，不外推统计结论，不作为 learning gate 证据，不回流经验。
- **B-Web① 字段名事件**：两臂指令均未给字段名（输入对称）；B 成员按合同在反复 400 后如实记录阻塞而非暴力枚举，A 臂自行探索成功。判定为维护方题目信息不足×B 诚实合同的交互，非 B 缺陷；补接口事实（非答案）后 B 一次完成。
- **限制（如实）**：全部为 headless `-p` 成员，**非可见成员窗口路径**（用户使用电脑期间不弹窗，可见会话工具可用性与原生窗口协作留待用户空闲时补验）；A 臂 attempt3 的拆小步指令是网关长尾应对（如实列为 A 臂观察条件）；ANSWER-KEY"≥5 个不同 XFF"字面未满足（两臂均以 2 值+交错对照证明按值分桶机制，同标准判达成）；轮询均为 headless 无热纠偏场景。

**费用**：A 臂 $6.27＋B 臂 $9.71（含 B-Web① 失败保留），headless 成员合计约 $16。

### DLV-08 收口与可删除方案（2026-10-25，done）

**必需验收矩阵 owner 对照（G-CORE～G-FRESH，证据均在本文件结果区）**：G-TOOLS binary/web＝DLV-02 通过（Playwright/Airtest 分项由 DLV-04/05 补验闭合）；G-TOOLS game＝DLV-05；G-RE＝DLV-03（运行时补证输出级限制）；G-WEB＝DLV-04（双侧实证+修复回归，整体 review needs-evidence 收尾——部分通过，非整体）；G-GAME＝DLV-05（五轮 review accepted；真实鼠标修复后未复测）；G-JOINT＝DLV-06 内容层整体 accepted＋原生消息交接 2026-10-25 补验闭合（见 DLV-06 结果区）；G-FRESH＝DLV-07 六臂 headless 工程可用（可见窗口启动+跨会话消息路径经 G-JOINT 补验顺带实证，三线任务在可见窗口重走未做）。整轮硬条件逐项如实分账：三线有真实交付；失败/未知全部保留（见下方待解清单）；新 Fresh 消费分发内容（DLV-07 六臂全新目录）而旧 case 零改写；工具从 pack 入口选择；代码交付未伪装漏洞报告；无新增控制面；长期入口承接后临时细则已删；"必需项有真实对应证据"经 2026-10-25 两项补验后：G-JOINT 原生消息部分已闭合（原生 token 交接双侧独立凭证）；G-WEB 整体 evidence 已由 NE-1 闭合重跑补上（独立复核 accepted，历史漂移保留）。G 系列必需项至此均有真实路径证据；剩余未闭合为真实鼠标通道复测（复测尝试失败于模板脱节，根因候选已记）与三线可见窗口重走，均属范围扩展而非必需项缺口。

**长期行为写回**：README 第 7 段、根 CLAUDE.md 路由段改为完成态（保留 tie/残余限制限定）；acceptance G-JOINT 行补实际验收范围标注；skill/模板/pack/合同在各批已物化，收口核查无实施态残留（vnext/README、capabilities、skill 扫描零命中）。DLV-02 结果区补安装批准与分项补验闭合回写（原"blocked on authorization"为批准前历史）。

**环境清理（自建）**：8903/8904 rateapp API 已停（Get-NetTCPConnection 无 8902-8904 监听）；遗留挂起 unittest 子进程（01:01 起，DLV-05 收尾任务产物）已结束，其任务随之下沉完成——其中 16 单测一次 failures=1 为六臂满载时段瞬时失败，后续两次复跑 16/16 OK；fixture 子进程六臂自述 reaped 且进程清单确认零残留；Playwright 无浏览器实例残留；Chrome/Edge/Docker 为用户常驻进程未动；本会话 idalib-mcp 子进程随会话退出。无真实凭据配置需删除（rateapp 凭据为合成题目材料，保留于仓库外证据区）。

**最终验证**：`python -m unittest discover -s tests/pack_tooling`＝27/27 OK；`go test -count=1 -p=2 ./...`＝全 ok；`go vet ./...`＝OK；`git diff --check`＝OK；临时细则无当前读取依赖或坏链（grep/Glob 核查；roadmap 首尾两处旧路径为历史删除说明，非链接——round 2 复核确认）；round 1 曾发现的接续点过时删除指令已随收束移除。

**收口独立复核（headless haiku Reviewer，12t/$0.61，`%TEMP%\steamai-pd\dlv08\REVIEW-dlv08.md`）**：round 1 **rejected**——①DLV-08 尚无结果条目而 README/CLAUDE 已宣布完成（时序提前）；②DLV-02 批次表 done 与结果区 active/blocked 冲突、分项补验无落点；③G-JOINT 原生消息要求与实际验收错配；④待解清单弱化（NE-1 缩写为"教训"）与漏列（DLV-05 真实鼠标未复测/停止时序未采集/异常 kill 未复现/原件覆盖、DLV-01 行为层负例、DLV-02 响应侧版本与 Strix 模型身份、roster 生命周期、A/B 观察条件限制等）；⑤接续点仍含已执行的删除指令；context-routing 第 9/10 行内部冲突。**全部属实，逐项修复**：DLV-02 结果区回写（本条上方"长期行为写回"）；acceptance G-JOINT 标注；待解清单按批次来源重写补全；接续点与 context-routing 校正；本条目补齐后 DLV-08 才置 done。Reviewer 同时确认：无残留细则文件（但其 C2 结论同时发现接续点存在真实过时执行指令——上文第⑤项）；DLV-03 虚构回执撤回、DLV-05 工程坑等为已记录历史边界未被误写成待修缺陷；Git 未提交状态正确（未授权）。

**Round 2 复审（14t/$1.32，REVIEW-dlv08-r2.md，rejected→修复）**：确认 round1 五项中②④⑤主体落地、①③部分修复；剩余：G-JOINT 必需 gate 未闭合被列为"可选补验"且完成声明未限定、DLV-06 Reviewer 26t 失败无结果区落点、清单重写丢失 Strix/Caido 网络与 child 热纠偏项、REA cleanup_incomplete 批次归属、引用检查措辞未区分历史说明与坏链。**全部属实，逐项修复**：接续点与长期入口改为"交付批次全部执行完毕＋必需 gate 未闭合部分分列（G-JOINT 原生消息、G-WEB 整体 evidence 为非可选未闭合项）"；硬条件表述按原义精确化；DLV-06 结果区补 26t 失败/处置落点（标注为维护方会话记录说明）；清单补回 Strix/Caido（DLV-04 行）并纠正 REA 归属（DLV-03 行）、v2 unknown 入列；引用检查限定为当前执行依赖/坏链并区分历史说明。

**Round 3/4 复审（r3 23t/$1.04 rejected→修复；r4 9t/$0.25 accepted）**：r3 剩余——"整轮硬条件按原义成立"仍混同交付记账与验收条件、round1 记录保留"无坏链"错误归因（round2 C2 曾同时发现过时执行指令）、Git 变更量记账过时。修复：硬条件分账明示"G-JOINT 原生消息部分'必需项有真实对应证据'不成立——未执行无证据，整轮完成声明以此为限"；round1 确认句改为如实含过时指令事实；记账改 +1553/-48 并注明以 `git diff --cached --stat` 为准。r4 核对三处落地并只读 Git 核验记账后 **accepted**（限定：接受本轮文档修复，不追认整轮全部必需 gate 已闭合——与本文"必需 gate 未闭合部分明示"立场一致）。

**授权后补验（2026-10-25 用户全权授权）**：NE-1 证据闭合（DLV-04 结果区）与 G-JOINT 原生消息（DLV-06 结果区）两项通过；真实鼠标复测失败中止（DLV-05 结果区）。同批落实用户新指示并写入 CLAUDE.md/acceptance：测试默认 `ANTHROPIC_DEFAULT_SONNET_MODEL`（Haiku 仅极简探针或 headless 无工具）、维护测试/验收默认无窗口后台通道（前台仅限用户同意时间窗）。补验费用：NE-1 重跑+复核 $0.82、Commander 及消息链（含成员窗口会话）约 $1.5。

**Git（按授权）**：本轮全部改动已精确 stage，**commit+push 已获用户授权（2026-10-25"全权授权"），在补验记录落档后执行**。

**费用（本轮收口）**：Reviewer $0.61。

## 删除临时方案（已执行）

批实施细则 `docs/plans/professional-delivery-{domains,validation}.md` 已于 DLV-08 收口时删除；删除前长期行为已物化于 README、根 CLAUDE.md、skill、`vnext/templates`/`vnext/acceptance.md` 与各 pack 入口，最终结果与残余限制保留在本路线结果区，全部引用（CLAUDE.md、context-routing、batch-plan、本文件）已修复。未删除任何 case 原件、历史失败或未知边界；`docs/plans/` 目录已空。
