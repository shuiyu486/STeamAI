# Verification and completion criteria

完成前运行与改动和风险直接匹配的最短验证。

- 文档或模板：检查引用、预算、格式、source/snapshot 路径和 `git diff --check`。
- 声明式 YAML/JSON：检查语法、引用文件存在性和 snapshot closure。
- 代码或合同：运行 focused tests，再按影响面运行 canonical suite 与静态检查。
- Claude Code 原生行为：区分 synthetic contract、自动 probe 和真实独立 session acceptance；不得相互冒充。
- Verified learning：V1 deterministic、V2 replay、V3 calibrated matched comparison、V4 multiple explicit opt-in field outcomes 分层验证；`accepted`、用户确认、Apply 或 Git staging 不提升等级。
- UI/交互：实际走受影响的用户路径。

完成意味着相关验证通过、没有自己引入的 orphan 文件或旧 runtime fallback。未执行、失败或跳过的验证必须如实说明原因和剩余风险。
