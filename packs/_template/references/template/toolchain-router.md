# Template toolchain router

> 复制后按领域填写工具、状态、输入输出和止损条件。

## 工具状态

| 状态 | 含义 |
|---|---|
| `mainline-template` | 多个授权 case 中重复验证，可作为推荐模板。 |
| `auxiliary` | 只适合特定阶段或辅助查询。 |
| `candidate` | 值得有界短测，尚未充分验证。 |
| `cautious` | 有执行、写入或外联风险，必须具体确认。 |
| `deprecated` | 不再推荐，仅保留历史结论。 |

## Capability card

```yaml
id: <tool-id>
status: candidate
purpose: <why it is used>
inputs: [<bounded input>]
outputs: [<case-relative output>]
sideEffects: [filesystem-read]
risks: [<risk>]
requiresUserConfirmation: false
stopConditions: [timeout, scope-drift, output-limit]
```

只读工具若会生成 sidecar，仍应限制输出路径和大小。执行、debug、inject、patch、dump、network、安装或共享数据库写入必须展示 exact action/target、隔离、预算、输出、回滚与止损，并取得用户具体确认和 Claude Code 工具权限。

## 维护规则

- catalog 只描述能力，不能作为可解释执行命令。
- 新工具先进入 `candidate`；短测必须有 timeout 和输出上限。
- 不把完整 README、日志、trace、反编译、凭据或 case 数据写入 pack。
