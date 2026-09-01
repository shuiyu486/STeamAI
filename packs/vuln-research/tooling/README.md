# Authorized vulnerability research tooling

本目录保存声明式 tool catalog 与 recipe，不包含 executable，也不自动执行任何动作。

- `catalog.yml`：能力、输入输出、风险与停止条件。
- `recipes/`：按场景读取的有界操作说明。
- `candidates/`：尚未回流的脱敏候选；只有 accepted finding/review 才能产生。
- `schemas/`：如存在，仅描述 sidecar 数据结构，不授予执行权限。

工具使用必须符合 case 授权。heavy action 需要针对具体动作的用户确认和 Claude Code 工具权限；recipe 不能充当授权。
