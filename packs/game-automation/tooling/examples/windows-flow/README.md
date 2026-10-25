# windows-flow 参考实现

合成窗口占位符的自动化脚本参考架构：决策核心（纯逻辑）与执行框架（Airtest）解耦。

## 结构

- `workflow.py`：状态枚举、`DecisionCore`、`DeviceAdapter`/`FakeDevice`、`Runner`（控制文件暂停/停止、可取消准备、ledger、时间线）。无第三方依赖。
- `config.example.json`：目标绑定、识别条件、动作/等待上限、非幂等清单的配置形状。
- `requirements.txt`：真实窗口执行时的依赖 pin（安装需用户批准；fake 单测不需要）。

**adapter 接口（prepare 与输入分离）**：`observe(checkpoint)`、`prepare(action, checkpoint)`（耗时截图/定位在此完成，期间反复调用 `checkpoint` 响应停止）、`act(action, prepared)`（只发短输入）。Runner 在准备返回后与 pending 落盘后各复核一次控制文件，晚到的停止不消费过期定位；暂停期间保持观察但不发输入。

## 自检

```text
python workflow.py --selftest
```

用 FakeDevice 跑一遍正常流程与停止路径，验证决策核心可独立执行。

## 接真实目标（示例→实际）

1. 复制本目录到任务分配的项目路径，按 config 绑定真实批准目标。
2. 实现 `AirtestDevice`（observe/act 走 `airtest`，参考 `../../integrations/airtest.md`），决策核心不改。
3. fake 单测迁移进项目测试；端到端按 `../../recipes/automation-regression.md` 六场景验收。
4. 本示例不绑定任何真实窗口；占位符只指向合成窗口。

**停止边界**：Runner 的停止检查点是控制文件轮询（`poll_interval` ≤0.2s）；`prepare` 内的耗时步骤（视觉模型调用、外部进程）必须反复调用传入的 `checkpoint`，让 stop/estop 在长调用期间立即生效并丢弃未完成的定位；不合作的同步 adapter 只能靠准备后的复核兜底，其长调用期间无法中断。验收须覆盖"识别期间发 stop"与"输入前发 stop"两类负例（见 `tests/pack_tooling/test_windows_flow.py`）；等待段与复核点的实测时延不外推到不合作 adapter 的阻塞段。
