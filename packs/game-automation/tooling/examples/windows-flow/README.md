# windows-flow 参考实现

合成窗口占位符的自动化脚本参考架构：决策核心（纯逻辑）与执行框架（Airtest）解耦。

## 结构

- `workflow.py`：状态枚举、`DecisionCore`、`DeviceAdapter`/`FakeDevice`、`Runner`（控制文件暂停/停止、可取消准备、ledger、时间线）。无第三方依赖。
- `localize.py`：定位蒸馏（`DistilledLocator`）——授权定位回调返回 bbox → 校验（越界/过小/低纹理拒绝）→ 裁剪模板仅存内存 → 同 key 本地全图模板匹配；低分失配回退回调并更新模板，歧义安全失败。图像操作经注入式 `matcher`（唯一真实实现 `CvMatcher` 用 numpy+OpenCV 的 `cv2.matchTemplate`——与 aircv/airtest `find_template` 同一 OpenCV 原语，不假称走 Airtest 官方 API；注入点仅是单测接缝，不是插件框架）。
- `synth_target.py`：纯合成无窗口目标（numpy 内存屏幕、带纹理 start/reward 按钮可位移、可关闭弹窗）。无真实窗口、无系统输入。
- `synth_device.py`：`SynthDevice` adapter + `ScriptedLocateCallback`（授权定位回调的脚本化替身，不调用任何模型）；直接运行脚本执行 demo。复用 `workflow.py` 的 Runner/DecisionCore/Ledger/ControlFile，不建第二套执行框架。
- `config.example.json`：目标绑定、识别条件、动作/等待上限、非幂等清单的配置形状。
- `requirements.txt`：真实窗口执行时的依赖 pin（安装需用户批准）。
- `refdiff.py`：参照/候选两份观察的首差比对（纯逻辑）：只比外部证据，缺证据即未知，实现差异单列，慢步骤归因，暂停/位移时的最后确认边界；步骤表定义不同即不可比；人类输出在首差处列出两份的原始事实、定位点-落点关系和可选的下一次检查。
- `win_target.py`：自建 `dm_demo_target --pages N` 的 Windows 设备：`--exe-sha256` 信任闸门、只操作自己启动的那一个窗口（显示到所有显示器之外，不占前台）、屏外窗口画面读取、只向该 hwnd `PostMessage`。
- `workbench.py`：参照-候选工作台，`run` / `observe` / `compare` / `matrix` 四个命令（见下节）。
- `scenario.example.json`（自建窗口切页）、`scenario.synth.example.json`（合成打开/领取）、`plan.example.json`、`plan.synth.example.json`：场景只写"做什么"，variant 写"这次自动化怎么做"，产品不按 case 写死。

**adapter 接口（prepare 与输入分离）**：`observe(checkpoint)`、`prepare(action, checkpoint)`（耗时截图/定位在此完成，期间反复调用 `checkpoint` 响应停止；返回的 `Prepared` 携带定位所依据的状态与目标 revision）、`act(action, prepared)`（只发短输入）。**act 合同：返回 True 表示目标的预期结果已确认，不只是"输入已发出且未抛异常"**；其余返回值或异常都是未确认（`InputUnconfirmed`）——非幂等动作保留 pending、不 commit、不重试，上层暂停等待确认。`SynthDevice.act` 在发输入前先复核目标状态与 revision：定位过期即未确认、不点击——不靠点击落点事后判定零误点。合成目标是单线程的，该复核精确；**真实 UI 中"复核通过"与"点击落地"之间仍非原子事务**，最终以目标端确认为准。Runner 在准备返回后与 pending 落盘后各复核一次控制文件（统一走 checkpoint），晚到的停止/pause/resume 不消费过期定位；期间观察到 resume 即丢弃定位结果重新观察。

## 自检与测试

仓库根目录 cwd：

```text
python -m unittest tests.pack_tooling.test_windows_flow          # Runner 合同（零依赖）
python -m unittest tests.pack_tooling.test_windows_flow_distill  # 定位蒸馏策略（零依赖）+ 图像/端到端
python -m unittest tests.pack_tooling.test_windows_flow_workbench  # 首差比对反例 + 安全拒绝 + 合成端到端；真实窗口需显式环境变量
```

`test_windows_flow_workbench` 的真实窗口用例只有同时给出 `STEAMAI_DEMO_TARGET`（自建 `dm_demo_target.exe`）与 `STEAMAI_DEMO_TARGET_SHA256`（调用者独立核对的哈希）才运行，默认 skip；不读其它窗口、不占前台。

## 参照-候选工作台（workbench.py）

以示范/旧版为参照，回答四个问题：首次差异在哪一步哪一相、是哪种可区分的故障、下一步查什么、修后用同一输入和终端状态复验是否保留外部行为。不是通用执行器：场景文件只描述步骤（when/expect/控件/画面区域），执行仍走 `workflow.Runner`/`Ledger`/`ControlFile`，只换成按步骤表决策的 `ScenarioCore` 和一层记录设备。

```text
python workbench.py run     --scenario scenario.synth.example.json --out T.json [--variant V]     # 合成目标
python workbench.py observe --scenario scenario.example.json --exe <dm_demo_target.exe> --exe-sha256 <HEX> --out T.json [--variant V] [--placement offscreen|hidden]
python workbench.py compare A.json B.json [--out R.json]   # 只读已有记录，不需要 NumPy/OpenCV
python workbench.py matrix  --plan plan.example.json --out DIR [--exe ... --exe-sha256 ...]
```

退出码分开两件事：`run`/`observe` 0 = 工作流达成（每步有外部证据）；`compare` 0 = 外部行为相同**且两份都确有目标达成、声明的保留项有证据且无冲突**（同一份坏轨迹自比是 `not-achieved`，缺终态/关键证据是 `unknown`，都不是 0）；`matrix` 0 = 每项实际结果都符合 plan 的声明，**不表示工作流成功**（坏候选按声明失败也记 0）；1 = 未达成/有差异/证据缺失/保留项冲突/不符合声明；2 = 用法或基础设施错误。

证据规则（`refdiff.py`）：
- 每次尝试的外部证据顺序：pre.state → input.sent → input.discarded → input.delivered → post.state → post.picture → effect → result；任一项缺失即 `unknown`，既不算相同也不算不同。
- 每步都是非幂等动作。准备时记录的前态和发输入前的当前状态都要符合声明的 `when`（没声明就没有这项限制）；不满足或缺证就不发。这不是把复核和输入变成原子操作。比较已有记录也核对该次前态：不满足为未达成，缺少要求的字段为未知，不能因为最后到了正确页面而通过。
- 最后一次尝试必须未丢弃、确已发出并送达，后态满足 `expect`，同名区域的有效三通道像素也确实变化。残缺画面不是变化；设备自报成功但证据不足时不会提交这步或执行下一步。前面的尝试若明确未发（如暂停）可正常恢复；非幂等步骤已重发或发送情况不明，不能拿末次成功抵消。末次没回执不借上次的，最终退回了错误页面也不算达成。布尔值不是页码，`null` 不是已观察到的值。
- 点击坐标、定位 bbox/来源/分数、输入种类、各相时长、窗口位置是可换实现，单列"实现/环境差异"不影响结论；但 variant 里 `behaviors.keep` 声明必须保留的证据（含坐标路径）不一致时判 `keep-violated`。
- 确认输入送达后，场景关注的状态没变是 `delivered-no-effect`，变了却不是期望的状态是 `wrong-state`；状态符合但画面不支持，仍不能算达成。
- 尝试/步骤个数差异是后验现象，排在公共前缀里的真实差异之后。
- 慢步骤按 wait/locate/effect 三相归因；暂停/位移给出最后一次确认到哪、有无已发未确认的输入。
- 每份记录先核对 `scenario.steps`：实际记录必须按序覆盖声明的全部步骤，两份同时少了同一步也不算达成。不把两份记录的一致当作场景原件的真实性证明。
- 可比前提是同一份步骤表：两份逐步的 `label`/`target`/`when`/`expect` 任一不同即 `incomparable`（退出码 2）。标签相同但目标或期待不同时，即使观察到的行为一致也不是 `same`。

`compare` 的默认输出给负责人直接选检查（不用再翻 JSON 或实现）：首差之后列出该步两份的原始事实——步骤目标、期待、定位点/bbox/来源、落点、送达（已发/已到、未发原因、丢弃）、前态→后态（是否满足期待、画面是否变化）——再给定位点-落点的**已证关系**：`定位本身不同`（两份定位点不同）/ `定位后输入位置变了`（定位点相同、落点不同）/ `位置一致`（两者都相同；效果缺失时明说"效果未知/尚未可证"）；没发输入时明说"无落点可比"。`下一步` 是按位置关系和故障类别列出的**检查项**（选一项、改一处、同一输入复验），不是根因判定，也不给具体坐标或偏移当修复答案；JSON 报告里对应 `focus`（原始事实摘录）与 `position` 两个字段。

信任与安全边界（`win_target.py` / `matrix`）：`--exe` 启动前按调用者独立给出的 SHA-256 核对文件字节，缺失/不一致不创建进程（路径、文件名不能证明是哪份程序，也不从 exe 旁边的文件读哈希）；只使用子进程 stdout 报告的 hwnd 并核对归属；`matrix` 在启动任何目标前预检 plan id（短、唯一、无路径分隔）、产物不出 `--out`、不覆盖已有文件；`compare --out` 不能指回输入。不截屏、不取桌面 DC、不 `SendInput`、不装钩子、不调用模型。

真实窗口的画面通道（本机实测，Win10 144 DPI）：屏外窗口可见区为空、收不到有效 `WM_PAINT`，呈现表面一直空白（`GetDC`+`BitBlt` 读到全白）；`capture()` 先 `PrintWindow(PW_CLIENTONLY)` 让目标按 `WM_PRINTCLIENT` 自绘到 DIB，再从同一 hwnd 的窗口 DC `BitBlt`，两路像素一致才算有画面，否则记 unknown。`--hidden` 窗口两路都读不到像素（全黑），画面一律 unknown，不伪造颜色；不处理 `WM_PRINTCLIENT` 的窗口会给空帧。DPI：Python 默认 DPI-unaware，从 unaware 线程看到的几何被缩小（200→133）、`PostMessage` 的鼠标坐标被放大（99→148，实测页签切错），所以几何、截图、输入全部在目标窗口的 DPI 感知上下文里完成。

实测记录（2026-11-27，自建 `dm_demo_target --pages 3` 屏外窗口 + 合成目标，产物在仓库外）：`matrix` 六项全部符合声明——baseline 切页 confirmed；wrong-point（点击偏移 60px）消息送达、目标计数 +1 但页签不变 → `delivered-no-effect`，首差 `to-page-2/post.state`；fixed 与 baseline `same`（两份都达成）；slow-locate 外部行为相同、`locate` 相 30ms→180ms → `slow-only`；stale-move 定位后窗口位移 → 发送前复核拒绝 `stale-binding`，不发输入；pause-resume 第 0 次尝试被丢弃、恢复后蒸馏模板定位、最终 confirmed，首差 `discarded-by-pause`。合成目标另验证 `effect-late`（预算 1ms 放弃后效果才到，终态满足 expect，与"没效果"分开）。未验：MFC 控制台作为第二控制通道、现成 Windows 应用、真实大漠路径、隐藏放置的画面。

本示例目录 cwd（`packs/game-automation/tooling/examples/windows-flow`）：

```text
python workflow.py --selftest   # 零依赖：决策核心 + Runner 正常/停止路径
python synth_device.py          # 合成通路 demo：弹窗自动关闭 + 显式 resume（需 numpy+opencv）
```

依赖边界：`test_windows_flow_distill` 的策略单测零第三方依赖；图像管线与端到端测试需要 `numpy` 与提供 `cv2` 的 OpenCV 包（复用 airtest 的既有依赖，无需并装另一 OpenCV 包；pack 不代装），缺失时整类 skip 并注明原因，不伪造第二套 oracle。

## 定位蒸馏（localize.py）

- key = `(实例 token, action, 目标名, 状态)`：目标实例、动作或状态变更都不跨用缓存（共用 locator 的两个同名目标实例不串缓存）；截图帧尺寸变化同样失效重建。缓存仅内存 dict，无任何文件 I/O（进程重启即失效，符合"重启不带状态"）。
- 失配（本地匹配低分）→ 回退授权回调并更新模板；**歧义（全图存在近分第二峰值，对称近峰邻域抑制后取次峰）、低纹理（逐通道空间 std——纯色块不论颜色一律拒绝）、越界、回调失败一律安全失败**（`LocateFailed`），不猜坐标、不缓存坏模板、不用回退刷结果。回调 bbox 必须是 4 个有限实数（bool/字符串/NaN/inf/长度≠4 一律 malformed），严格校验在任何缓存写入之前完成，不留半成品。
- 蒸馏命中只是"目标看起来还在"：坐标校验与点击后状态复核不因此豁免；目标端测试校验（`SynthTarget.click` 的命中判定）是测试 oracle，**不构成真实点击可靠性证明**。
- 性能边界如实声明：加速只发生在"同一运行内、同 key、外观未变"的重复定位；本仓库合成通路当前实测参考值（480x320 截图、44x36 模板）单次同步匹配约 8ms、首次回调路径取决于回调实现。`../../integrations/airtest.md` 中"首次 28.2s / 蒸馏 0.013s"是旧评测环境的历史报称，不是本仓库实测，**不外推为"全任务 2000×"**。真实场景的首次定位走授权回调（如 headless 视觉模型，见 airtest.md），本示例用脚本化替身，不强制任何模型调用。

## 合成弹窗（synth_target.py）

修复历史评测 fixture 的缺陷（弹窗无关闭路径，popup 后永久 DIALOG、任务不可达）：

- `popup_close_after_ms` 契约：**None/0/省略 = 永久不自动关闭；正数 = 到时自动关闭；负数/非数值/bool 构造即 `ValueError`**。
- 弹窗显隐与帧内容一致（显隐即时重渲染，有帧级像素测试），自报 state 不掩盖图像错误。
- 弹窗关闭（自动或 `close_popup()` 显式）只是目标侧事实变化，**不触发任何恢复**——Runner 仍暂停，恢复只来自显式 `resume`。
- workflow 遇 DIALOG 一律暂停，不盲点；`popup_trigger_state` 参数按状态确定性注入弹窗（每 run 一次）。
- 按钮位移用 `set_target_pos`：整按钮必须落在画幅内，出画直接 `ValueError`（不产生残缺渲染，不复现历史"位移出截图"缺陷题）。
- 目标的任何可观察变化（状态/弹窗/位移）推进 `revision`，供 adapter 发输入前复核。
- 合成边界：state 由合成目标自报（真实游戏也知道自己状态），代表状态机输入来源，**不代表识别能力**；识别/定位链在被测的 adapter 侧。UNFOCUSED/GONE 等窗口事件不在本 fixture 范围内。

## 接真实目标（示例→实际）

1. 复制本目录到任务分配的项目路径，按 config 绑定真实批准目标。
2. 实现 `AirtestDevice`（observe/act 走 `airtest`，定位分层参考 `../../integrations/airtest.md`），决策核心与 `localize.py` 的蒸馏纪律不改。
3. fake/合成单测迁移进项目测试；端到端按 `../../recipes/automation-regression.md` 六场景验收。
4. 本示例不绑定任何真实窗口；占位符只指向合成窗口。

**停止边界**：Runner 的停止检查点是控制文件轮询（`poll_interval` ≤0.2s）；`prepare` 内的耗时步骤（授权定位回调、外部进程）必须反复调用传入的 `checkpoint`，让 stop/estop 在长调用期间立即生效并丢弃未完成的定位。本地模板匹配是单次同步 OpenCV 调用：停止只在调用前后的 checkpoint 生效，**不承诺 native 调用期间可取消**；调用时长受截图尺寸约束（输入尺寸有界；本机 480x320 实测约 9ms，最坏停止时延 ≈ 一次匹配时长 + poll 间隔）。验收须覆盖"识别期间发 stop"与"输入前发 stop"两类负例（见 `tests/pack_tooling/`）；等待段与复核点的实测时延不外推到不合作 adapter 的阻塞段。
