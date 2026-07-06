# VMP routine IR verification overlay

Extends: `common/policies/verification.md`

## confirmed CSV 变更后

修改 confirmed opcode 或 handler role 表后，至少验证：

1. 重建 `routine_ir.events.csv`、`routine_ir.summary.csv`、`routine_ir.md`。
2. 重建或刷新 `routine_ir.superinstructions.csv`、`.md`。
3. 检查 coverage、unknown 数量和新增 lowering 是否符合预期。
4. 更新 `references/vmp-re/task-handoff.md` 的当前状态和下一步。

## handler lowering 完成标准

- 低样本、1-count、alias-heavy 或 pointer-heavy handler 不可只凭 formula 自动写 confirmed CSV。
- no-stable-VSP-payload 只保守降低为 bridge/control role，除非 focused instruction review 支持 opcode semantics。
- 子 agent 建议必须由主 agent 合并并验证后才落库。

## 文档和 policy 变更后

- 运行 `/rekit doctor` 或 backend `doctor`。
- 运行 `git diff --check`。
- 确认未把 case live state、路径、trace/dump、地址快照写入 common policy 或 pack overlay。

## heavy-tool gate

full trace、动态调试、Frida/注入、patch、dump、network 属于 heavy-tool 动作，触发前必须：

1. 先走轻路径并记录失败原因（静态 triage、focused review、value-flow、unicorn 局部 trace）。
2. 用 `/rekit note -Kind request -Lane <lane> -Subject <动作> -Summary <原因与目标> -Status pending-gate` 在账本记录 gate 事件，附 `tried_light_steps`、`budget`（runtime_s/disk_mb）、`stop_conditions`。
3. 向用户说明：将做什么、为什么轻路径走不通、预算、止损、输出保存位置。
4. 用户明确确认后执行；确认只覆盖列明 scope，不扩大授权。
5. 执行后用 `/rekit note -Kind observation -Lane <lane> -Subject <动作> -Summary <结果摘要> -Confidence <low|medium|high>` 记录结果。

runtime 当前不强制 gate；这是 agent 行为契约，违反应由 review 暴露。未来 Phase 6 才考虑 runtime 强制确认。
