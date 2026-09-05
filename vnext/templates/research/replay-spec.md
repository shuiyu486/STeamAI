# `{{REPLAY_SPEC_ID}}` — Replay of `{{FINDING_ID}}`

- Owner：`{{OWNER}}`
- Finding：`{{FINDING_REF}}`
- Finding SHA-256：`{{FINDING_SHA256}}`
- Replay class：`{{REPLAY_CLASS}}`
- Environment：`{{ENVIRONMENT}}`
- Inputs (path/SHA-256/bytes)：`{{INPUT_BINDINGS}}`
- Authorization basis：`{{AUTHORIZATION_BASIS}}`
- Allowed actions：`{{ALLOWED_ACTIONS}}`
- Prohibited actions：`{{PROHIBITED_ACTIONS}}`
- Budget：`{{TIME_TOKEN_TOOL_BUDGET}}`
- Expected observation：`{{EXPECTED_OBSERVATION}}`
- Allowed variance：`{{ALLOWED_VARIANCE}}`
- Stop conditions：`{{STOP_CONDITIONS}}`

## Procedure

`{{REPLAY_PROCEDURE}}`

## Limitations

`{{LIMITATIONS}}`

`Replay class` 只能是 `deterministic-readonly`、`sandboxed-local`、`environment-bound`、`manual-only` 或 `non-replayable`。前两类可由独立 verifier 执行，但自动 runner 只接受满足 synthetic、无凭据、工具网络 forbidden、无真实目标、Read-only 边界的 `sandboxed-local`；其余类别不得由自动 runner 擅自执行。spec 创建后保持 immutable；finding、输入、环境或授权变化时新建 spec，不覆盖旧文件。
