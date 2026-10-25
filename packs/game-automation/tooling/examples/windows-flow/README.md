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
