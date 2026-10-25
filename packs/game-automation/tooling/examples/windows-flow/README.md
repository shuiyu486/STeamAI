# windows-flow 参考实现

合成窗口占位符的自动化脚本参考架构：决策核心（纯逻辑）与执行框架（Airtest）解耦。

## 结构

- `workflow.py`：状态枚举、`DecisionCore`、`DeviceAdapter`/`FakeDevice`、`Runner`（控制文件暂停/停止、ledger、时间线）。无第三方依赖。
- `config.example.json`：目标绑定、识别条件、动作/等待上限、非幂等清单的配置形状。
- `requirements.txt`：真实窗口执行时的依赖 pin（安装需用户批准；fake 单测不需要）。

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

**停止边界**：参考 Runner 在同步 `observe/act` 返回后才再次读取控制文件，不会中断长调用。接入耗时视觉模型等适配器时，必须另行实现可取消等待、发输入前复核停止与最新状态，并测试“调用期间发 stop 不再输入”；现有 selftest 和等待段急停记录不证明这一点。未满足前只用于有人看护的有界流程，不宣称全时段 ≤2 秒停止或无人值守安全。
