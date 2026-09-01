# VMP 工具链路由

## 状态

| 状态 | 含义 |
|---|---|
| 主线模板 | 多个授权 case 中重复验证，后续优先使用。 |
| 辅助 | 只在特定阶段使用。 |
| 慎用 | 容易触发反调试、卡死或大输出，必须有具体确认和止损。 |
| 止损短测 | 新目标可有界短测，无明确命中立即停止。 |
| 候选 | 尚未充分验证，只记录能力和风险。 |

## 按任务路由

| 任务 | 首选 | 备用 | 边界 |
|---|---|---|---|
| 公开 unpack/import 工具判断 | 有 timeout 的短测 | 静态识别 | 无明确命中立即止损。 |
| VMEnter/source stubs | 静态扫描与已有 sidecar | IDA/x64dbg 静态辅助 | 不把大反汇编带入上下文。 |
| 真实 context | in-process probe | suspended launcher | 属于执行动作，必须具体确认。 |
| 离线 dispatch trace | Unicorn + bounded context | memory augmenter | 输出留 case-local sidecar。 |
| focused handler review | focused trace + value-flow | IDA 静态精读 | 低样本/alias-heavy 必须 Reviewer 复核。 |
| 动态调试 | 隔离环境中的 bounded debugger action | ScyllaHide | 避免裸启动，严格 stop conditions。 |

## Heavy action

full trace、动态调试、注入、patch、dump、长时间符号执行或 network 前，向用户展示 exact action/target、轻路径为何不足、隔离与网络策略、运行/磁盘/request 预算、case-relative outputs、rollback 与 stop conditions。只有用户确认该具体动作并满足 Claude Code 工具权限后才能执行；scope、预算、输出或风险变化时停止并重新确认。

## 维护规则

- candidate 经有界短测后才可提升为辅助或主线模板；不稳定、噪声大或不匹配时标记止损。
- 不贴完整 README/build log；只记录能力、输入输出、适用阶段、风险和止损。
- 对单个 case 有效但不可泛化的工具结论只留 case finding/review。
