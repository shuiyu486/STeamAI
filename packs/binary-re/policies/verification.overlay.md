# VMP routine IR verification overlay

Extends: `common/policies/verification.md`

## confirmed CSV 变更后

修改 confirmed opcode 或 handler role 表后，至少验证：

1. 重建 `routine_ir.events.csv`、`routine_ir.summary.csv`、`routine_ir.md`。
2. 重建或刷新 `routine_ir.superinstructions.csv`、`.md`。
3. 检查 coverage、unknown 数量和新增 lowering 是否符合预期。
4. 更新 `references/binary-re/task-handoff.md` 的当前状态和下一步。

## handler lowering 完成标准

- 低样本、1-count、alias-heavy 或 pointer-heavy handler 不可只凭 formula 自动写 confirmed CSV。
- no-stable-VSP-payload 只保守降低为 bridge/control role，除非 focused instruction review 支持 opcode semantics。
- 子 agent 建议必须由主 agent 合并并验证后才落库。

## 文档和 policy 变更后

- 运行 `/steamai doctor` 或 backend `doctor`。
- 运行 `git diff --check`。
- 确认未把 case live state、路径、trace/dump、地址快照写入 common policy 或 pack overlay。

## heavy-tool gate

full trace、动态调试、Frida/注入、patch、dump、network 属于 heavy-tool 动作，触发前必须：

1. 先走轻路径并记录失败原因（静态 triage、focused review、value-flow、unicorn 局部 trace）。
2. 用 `/steamai gate -WhatIf` 预览 action、exact target、typed budget、output paths、stop conditions 与 autonomy evaluation；确认预览正确后再显式 `gate -Apply`，只向 request ledger 追加 `pending-gate` 或 `authorized-gate` decision，不执行 heavy-tool。
3. `pending-gate` 时向用户说明将做什么、为何轻路径无法闭合、预算、止损和输出位置，并取得本次显式确认；`authorized-gate` 只有在 strict validated durable autonomy profile 完整覆盖本次边界时才构成 deterministic grant。
4. lane executor / tool adapter 只在上述任一路径有效时执行，且不得扩大列明 scope；超出 action/target/budget/stop/output/record/notify 边界必须重新升级。
5. 执行后用 `/steamai note -Kind observation -Lane <lane> -Subject <动作> -Summary <结果摘要> -Confidence <low|medium|high>` 记录结果，并写回 evidence、预算消耗和 boundary-hit。

Go runtime 已强制 gate action/profile preflight 与 request decision 写入边界；它不执行实际 heavy-tool，实际执行和证据回写仍由 lane executor / tool adapter 负责。
