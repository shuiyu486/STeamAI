# 专业交付验收记录

`steamai-professional-delivery-v1` 的交付批次 DLV-00～08 已收束。本文件只保存最终结果、证据边界和未解决问题，不是执行计划；当前没有待续跑批次，也不表示所有 live gate、工程缺陷或学习门槛均已通过。

## 当前可用范围

- 当前源码支持人在环的逆向、Web 安全与 Windows 自动化开发协作；三线都在仓库外受控目标上完成过实际交付。不是自动分析引擎，也不是安装后即可操纵任意游戏的成品脚本。
- 本机安装核对为绑定当前 checkout 的 `dev` 版；已发布 `v1.0.4` 不包含这轮源码能力。本轮没有发布新 Release。已有 current case 继续使用自己的 pinned snapshot，新合同/pack 只进入后续 Fresh。
- 已观察到的增量是统一证据、失败留存、独立复核与跨成员交接；尚无充分证据证明比裸 Claude Code 普遍更准、更快、更省。
- 游戏线适用于有界、慢速、有人看护的流程。LLM 定位单次约 30–60 秒，阻塞期间急停存在下述缺口；不宣称实时控制或无人值守安全。
- 长期行为与验收要求分别以根 `CLAUDE.md`、canonical skill、`vnext/acceptance.md` 和各 pack 为准；验收记录不授予新动作权限。

## 已交付与验收范围

| 范围 | 实际结果 | 不能据此声称 |
|---|---|---|
| DLV-00/01 基线与双交付合同 | 环境分层核对；研究交付与代码交付均可分配、审查；production Fresh/current 接受新“Case 目标”及旧“研究目标”，不改写旧 case；本机 exe 已重建安装 | 合同文字等于模型遵守权限；本轮越界源码写入/Reviewer 只读 live 负例未做 |
| DLV-02 专业工具入口 | REA/IDA MCP、Strix 真实操作与跨会话产物阅读；Playwright/Airtest 安装获批后在 DLV-04/05 补验；三 pack 入口固定 | 自动安装环境、所有 provider 等价、Strix 可无边界自主运行 |
| DLV-03 逆向 | 两版合成程序跨函数/间接调用分析、版本差异与输出级运行补证，补证后 review accepted；REA 实际反编译与导出期间独立 GUI 数据库保留 | 调试器内部状态已验、全程序还原；无符号部分函数来源仍 unknown |
| DLV-04 Web | API/浏览器双侧实证；修复与独立回归 12 组、67 请求通过，开发交付 accepted；NE-1 新证据链补验 accepted | 原整体 review 的 needs-evidence 被追改、历史执行脚本身份恢复、全部阶段重新验收 |
| DLV-05 游戏脚本 | 决策/执行分离、两阶段 ledger、暂停恢复；五轮 review 至 accepted；LLM 定位真实鼠标最终六场景 6/6，fake 单测 19/19 | 纯视觉识别任意游戏状态、同步定位期间急停合格、headless 模板坐标正确 |
| DLV-06 联合 | 逆向算法→API 请求→结果关联、版本错配拒绝、文件冷接续整体 accepted；正式成员原生消息补验通过 | roster 全生命周期、所有输入或版本兼容 |
| DLV-07 新 Fresh 与有限对照 | 三线六臂交付；headless 新 Fresh 可消费分发内容；质量 tie 为主 | 正式盲测、普遍增益、零维护方干预、三线可见窗口完整重走 |
| DLV-08 收尾 | 长期行为写回；四轮文档复核至 accepted；自建服务/窗口清理；三次实现与补验提交已交付 | Reviewer 接受文档等于所有产品路径通过；新 Release 已发布 |

### 承重补验

- **NE-1**：登记版脚本对相同 fixture 原样重跑，执行前后 SHA 不变；新旧回执仅计时字段不同，API 11/11、浏览器 6/6 与请求/清理字段一致。独立 Reviewer 接受“对双侧实证结论的证据影响闭合”，未重新运行全部修复阶段。历史 11:38 回执与 11:46 脚本定稿之间的 exact 身份断裂永久保留为 unknown，原整体 review 的 needs-evidence 不改写。
- **原生消息**：正式 `__open-member --trust-workspace` 路径启动两个可见成员；exact cwd→name、ListAgents 唯一相交、SendMessage success 与 incoming 接收记录、双方独立凭证共同证明交接。首次无 trust 卡信任框；维护方消息不能解除 case 停止条件，成员合规拒绝后由 Commander 正式改派。窗口启动与消息全链已验，不再写成“完全未测”；三线完整任务在可见窗口重走仍未做。
- **真实鼠标**：获准时间窗内最终回执 `windowed-6of6-final.json` 为 6/6：正常、3500ms 延迟、意外弹窗暂停、暂停恢复与位移（observe seq 5 < act seq 6）、estop 0.125s、重启不重复领取。最终回执本次只读复核仍在；这一急停样本不覆盖同步 LLM 定位过程。

## 效果观察

A 为裸 Claude Code 加同批工具说明，B 为 STeamAI Fresh；每线仅一对合成任务，均为 headless。两臂判分点均达成，B 多出统一证据绑定与失败留存。

| 方向 | A | B | 可保留的观察 |
|---|---|---|---|
| 逆向 | 31 turns / 538s | 42 turns / 644s | 同题均正确，结构化交付有额外开销 |
| Web | 网关两次失败后拆小步成功，14 turns | 字段名 400 阻塞后补接口事实成功，31 turns | 有维护方重跑/补注，不能称完全无人干预 |
| 游戏 | 49 turns / 2102s | 14 turns / 1732s | 本题 B 交付更完整；turns 不等于总成本或普遍能力 |

已知费用 A $6.27、B $9.71；A 的两次网关失败未计入该总数，B 包含一次失败，不能据此作同条件成本比。用户直接介入记为 0，不等于维护方介入为 0。评审非盲、B 结构可识别、无热纠偏场景；题面“≥5 个不同 XFF”两臂均只以 2 值加交错对照证明机制，未满足字面数量。tie 不外推、不作 learning gate 证据、不据此回流经验。

## 保留的失败与限制

### 正确性与安全边界

- **同步动作期间停止缺口（本次核查确认）**：参考 `workflow.py` 的 Runner 只在每轮开始读取 stop，`device.act()` 为同步调用；维护 fixture 的 LLM 定位之后直接 touch，没有输入前再次检查停止/状态。本次用阻塞 fake adapter 和事件屏障复现“先写 estop，解除阻塞后仍发出一次模拟输入，随后才读取 estop”；未发真实输入、未调用模型。故不能把 0.125s 样本推广为全时段 ≤2s。此项未修复；需要可取消定位、发输入前重新核对停止与状态及相应回归，不能靠放宽超时解决。
- headless 纯色模板的 aircv 零方差伪匹配可恒报 1.0；消息输入按动作名、不校验落点。因此模板识别失败/错误坐标未获有效验证；决策、ledger、暂停停止的对应测试不因此失效。LLM 真实窗口通过不补足这一 headless 负例。
- 合成窗口通过标题暴露状态，headless 通过 JSON 提供状态；视觉定位按钮不等于在陌生真实游戏中识别全部状态。PrintWindow、DPI 和渲染支持仍须按目标核实。
- act/commit 中断窗口异常 kill 未故障注入，只有两阶段处理与单测；DLV-01 越界写入/只读 Reviewer 行为负例未做；正式 roster 生命周期未验。不可都称为“已解决的非阻塞项”。
- verified-learning 的 current calibration go、合格 comparative promotion 与多个真实后续 case 的 V4 evidence 仍未具备；详见 `verified-learning-roadmap.md`，不能用本轮完成替代。

### 工具与环境

- ida-pro-mcp 0.1.0 的 Windows force_gui 缺陷仍在，手动 GUI adopt 绕行已验；idalib 会在目标旁留下伴生文件。REA close 返回 cleanup_incomplete，当次进程观察无残留不改写该返回值。
- x64dbg 未安装，本轮运行时补证仅 stdout/exit；DLV-02 响应侧 MCP/IDA/Hex-Rays 版本及两次 Strix 响应模型身份 unknown。
- Strix 会运行宿主进程、调用模型 API 并初始化 Caido，不是完全不联网；child pending=2 未消费，child 热纠偏没有证明。预算为响应后检查，不是绝对费用上限。
- Airtest URI 截断、mss 不穿遮挡、tkinter 不响应 PostMessage 等绕行已写 pack；它们不是任意目标通用适配保证。
- 当时 Sonnet 映射的 headless 会话缺命令工具，相关试验使用 Haiku 降级；这不是永久环境结论。以后仍默认显式 Sonnet，现场核对，前台验收另须用户同意时间窗。

### 历史失败不抹除

- DLV-03 曾虚构“已运行”回执，transcript 无执行工具调用；Reviewer 挡住后撤回原声明并真实补跑，后来 accepted 不追认历史执行。
- DLV-04 的网关截断、未就绪退出、单文件 Strix 参数拒绝、预算中止、SARIF 仅覆盖线索、Commander 指令误标用户纠偏、NE-1 脚本被接管重写均保留；不得把线索报成漏洞或零漏洞。
- DLV-05 原始模板复测失败后改 LLM；windowed 三轮为 2/6→5/6→6/6，分别修编排超时/轮询与首帧截图竞态。首轮更早的窗口 report 曾被覆盖，仅旧 review 引用/摘要存留；后来各轮保存不恢复该原件。headless sc6 曾超时，单独复跑通过仍记 flake。
- DLV-05 五轮审查修过 ledger 类型、两阶段/原子写、快通道连发及顺序断言缺陷；DLV-06 Reviewer 26 turns 无产物，重发后 13 turns 才交付；DLV-07 Web 字段阻塞、网关失败、game 失败轮与 max-turns 截断均保留。
- DLV-08 r1/r2/r3 因提前宣告完成、漏记未验项、gate 错配、过时状态等 rejected，r4 accepted 只覆盖当轮文档修复。后来补验不能倒改旧裁决。本次又发现主路线多个“未复测/待提交”旧句与最终结果冲突，现已移除重复状态而非抹除失败。

## 验证与追溯

- 既有收尾回归：27 项 Python pack tests、全量 Go tests、go vet、diff check 通过；只证明各自机械范围，不替代 live。
- 本次可用性核查：本机安装/source 只读核对；casebootstrap、learningbatch、native shell、vnextcontract focused tests 与 go vet 通过；Python pack 27/27、参考 workflow selftest 通过。停止探针先因测试配置缺字段失败，再因 Windows 计时同值的断言失败；补齐配置并改用事件顺序后复现上述缺陷，不将探针错误冒充产品错误。
- 实现与补验提交：`eb91eba8`（三线交付）、`78ae1f41`（定位分层）、`111068b4`（真实鼠标最终复测）。删除前完整批次记录可从 `git show 111068b4:docs/professional-delivery-roadmap.md` 定向追溯，不默认加载。
- 本机原始证据仅留仓库外维护目录：`dlv04-ne1c-review/REVIEW-ne1c.md`、`dlv05-game/automation/runs/windowed-6of6-final.json`、`dlv06-joint` 的消息双方凭证、`dlv07/A-B-RESULTS.md`、`dlv08` 四轮 review。临时证据区不是长期归档保证；未将 case 原件、凭据或绝对 case 路径复制入库。
- 本轮两份临时实施细则此前已移除；本次再删除主执行计划与无待办短投影，结果收敛到本文件。旧路线只按需追溯，不删除未完成 gate 或改判 tie/inconclusive。
