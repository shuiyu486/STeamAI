---
name: steamai
description: 当前自包含 STeamAI 项目的 Mission Control 自然语言入口。
argument-hint: "[目标、继续请求、状态查询、纠偏或授权意图]"
---

# STeamAI 项目入口

本目录是一个自包含 STeamAI 项目。使用 `${CLAUDE_PROJECT_DIR}` 定位项目根，不读取或修改用户级 skill，不依赖全局 plugin 或外部 kit 仓库。

1. 只接受 `${CLAUDE_PROJECT_DIR}/.steamai/instance.yml` 作为当前项目 metadata。
2. 若 `.steamai` 与 `.rekit` 同时存在，立即停止并报告状态根冲突，不拼接状态。
3. 只运行 manifest 绑定的项目内 executable，不通过 PATH 或外部 kit 回退。Windows 确定命令：
   - compact status：`& "${CLAUDE_PROJECT_DIR}\.steamai\runtime\bin\steamai.exe" runtime -Command status -Target "${CLAUDE_PROJECT_DIR}" -Format compact-json`
   - 新目标：`& "${CLAUDE_PROJECT_DIR}\.steamai\runtime\bin\steamai.exe" host -daily -target "${CLAUDE_PROJECT_DIR}" -goal "<GOAL>"`
   - 继续：`& "${CLAUDE_PROJECT_DIR}\.steamai\runtime\bin\steamai.exe" host -daily -target "${CLAUDE_PROJECT_DIR}" -lane "<TYPED_LANE>"`
   - 纠偏：`& "${CLAUDE_PROJECT_DIR}\.steamai\runtime\bin\steamai.exe" host -daily -target "${CLAUDE_PROJECT_DIR}" -lane "<TYPED_LANE>" -correction "<CORRECTION>"`
4. 每次运行前由 executable 自身严格验证 `.steamai/runtime/manifest.json` 及全部 hash/size/role/layout；校验失败只报告修复动作。用户只查询时保持零写入、零 Claude launch；开始、继续、纠偏时才走 typed daily owner。
5. 默认只输出“现在、原因、下一步”；多 lane 只展示 runtime 返回的 typed choices。
6. full status、内部路径、SHA、lane/session ID 仅在用户明确要求维护诊断时读取。
7. heavy action 必须通过 strict durable profile 与 fresh `authorized-gate`；授权档位不扩大 authority/confirmed、sync/promote、schema migration、公开发布或项目外目标。
8. 用户要求“开启较高自治”时只走 `bounded-autonomous-v1`：从 fresh typed state确定单一 lane、manifest actions、exact targets、正数 budget、完整 stop、case-relative outputs和最长 15 分钟 expiry；缺项只问一个问题。先运行项目内 `runtime -Command gate -ProvisionProfile -ProfilePreset bounded-autonomous-v1 -ProfileExplicitOptIn ... -Format json` 的零写入 preview，用人话展示 exact 范围；用户确认后才原样追加 runtime 返回的 `-ExpectedProfilePlanSha256 <SHA> -Apply`。network 还必须传与 exact targets 相同的 `-ProfileExternalTargetScope`。撤销使用 `-RevokeProfile` 同样 preview 后 exact Apply。不要让用户记 SHA，也不要把 profile mutation说成已执行或已写 `authorized-gate`。
9. fresh status 或 daily 返回的 typed `invocation` 是唯一通用命令桥。仅当 `invocation.schemaVersion=1`、`commandExecutable=true`、`blocked=false`，且 request 的 `command` 与非空 `expectedReceipt.command` 完全一致、用户意图覆盖 exact request 时，才将 `["runtime", "-Command", invocation.command] + invocation.arguments` 作为 argv 数组逐项传给同一个项目内 executable。需要 review/confirmation 时只运行 request 自带 preview，不自行追加 `-Apply`。
10. 不从 `command`/`guidance` prose 重建参数，不拼接 shell command，不使用 `Invoke-Expression`，不增删改 `-Target`、`-Lane`、`-WhatIf`、`-Apply`、hash 或 receipt 参数。`commandExecutable=false` 的 guidance、model-tool handoff 和 `preview-command-template` 即使含 command 文本也绝不执行。执行后只消费 typed result，并重新读取 fresh compact status。
11. typed state 无法唯一选路时只问一个问题，不根据文件存在、错误字符串或模型文案自制下一步。
