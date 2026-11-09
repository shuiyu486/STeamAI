# windows-flow 参考实现

合成窗口占位符的自动化脚本参考架构：决策核心（纯逻辑）与执行框架（Airtest）解耦。

## 结构

- `workflow.py`：状态枚举、`DecisionCore`、`DeviceAdapter`/`FakeDevice`、`Runner`（控制文件暂停/停止、可取消准备、ledger、时间线）。无第三方依赖。
- `localize.py`：定位蒸馏（`DistilledLocator`）——授权定位回调返回 bbox → 校验（越界/过小/低纹理拒绝）→ 裁剪模板仅存内存 → 同 key 本地全图模板匹配；低分失配回退回调并更新模板，歧义安全失败。图像操作经注入式 `matcher`（唯一真实实现 `CvMatcher` 用 numpy+OpenCV 的 `cv2.matchTemplate`——与 aircv/airtest `find_template` 同一 OpenCV 原语，不假称走 Airtest 官方 API；注入点仅是单测接缝，不是插件框架）。
- `synth_target.py`：纯合成无窗口目标（numpy 内存屏幕、带纹理 start/reward 按钮可位移、可关闭弹窗）。无真实窗口、无系统输入。
- `synth_device.py`：`SynthDevice` adapter + `ScriptedLocateCallback`（授权定位回调的脚本化替身，不调用任何模型）；直接运行脚本执行 demo。复用 `workflow.py` 的 Runner/DecisionCore/Ledger/ControlFile，不建第二套执行框架。
- `config.example.json`：目标绑定、识别条件、动作/等待上限、非幂等清单的配置形状。
- `requirements.txt`：真实窗口执行时的依赖 pin（安装需用户批准）。

**adapter 接口（prepare 与输入分离）**：`observe(checkpoint)`、`prepare(action, checkpoint)`（耗时截图/定位在此完成，期间反复调用 `checkpoint` 响应停止；返回的 `Prepared` 携带定位所依据的状态与目标 revision）、`act(action, prepared)`（只发短输入）。**act 合同：返回 True 表示目标的预期结果已确认，不只是"输入已发出且未抛异常"**；其余返回值或异常都是未确认（`InputUnconfirmed`）——非幂等动作保留 pending、不 commit、不重试，上层暂停等待确认。`SynthDevice.act` 在发输入前先复核目标状态与 revision：定位过期即未确认、不点击——不靠点击落点事后判定零误点。合成目标是单线程的，该复核精确；**真实 UI 中"复核通过"与"点击落地"之间仍非原子事务**，最终以目标端确认为准。Runner 在准备返回后与 pending 落盘后各复核一次控制文件（统一走 checkpoint），晚到的停止/pause/resume 不消费过期定位；期间观察到 resume 即丢弃定位结果重新观察。

## 自检与测试

仓库根目录 cwd：

```text
python -m unittest tests.pack_tooling.test_windows_flow          # Runner 合同（零依赖）
python -m unittest tests.pack_tooling.test_windows_flow_distill  # 定位蒸馏策略（零依赖）+ 图像/端到端
```

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
