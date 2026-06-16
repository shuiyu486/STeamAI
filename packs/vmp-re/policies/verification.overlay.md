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
