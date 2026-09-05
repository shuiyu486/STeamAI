# `{{SCENARIO_ID}}` — `{{TITLE}}`

- Owner：`{{OWNER}}`
- Suite：`{{SUITE_ID}}`
- Calibration slot ID：`{{SLOT_ID_OR_NONE}}`
- Expected control class：`{{EXPECTED_CLASS_OR_NONE}}`
- Scenario class：`{{SCENARIO_CLASS}}`
- Replay class：`{{REPLAY_CLASS}}`
- Synthetic fixture：`required`
- Credentials：`forbidden`
- Task fixture：`{{TASK_FIXTURE_BINDING}}`
- Input SHA-256：`{{INPUT_SHA256}}`
- Rubric version：`{{RUBRIC_VERSION}}`
- Rubric SHA-256：`{{RUBRIC_SHA256}}`
- Tool profile：`{{TOOL_PROFILE}}`
- Tool network：`forbidden`
- Real targets：`forbidden`
- Claude API call：`expected`
- Arm budget：`{{ARM_BUDGET}}`
- Initial pairs：`{{INITIAL_PAIRS}}`
- Maximum pairs：`{{MAXIMUM_PAIRS}}`

## Preregistered success criteria

`{{SUCCESS_CRITERIA}}`

## Hard safety gates

`{{HARD_SAFETY_GATES}}`

## Inconclusive conditions

`{{INCONCLUSIVE_CONDITIONS}}`

## Holdout and leakage boundary

`{{HOLDOUT_BOUNDARY}}`

calibration scenario 必须在执行前给每个 control/pair 指定唯一 `Calibration slot ID` 与 `Expected control class`，且 request/run bundle exact 复制这两个值；candidate scenario 两项填 `none`。自动 runner 要求 `Synthetic fixture=required`、`Credentials=forbidden`，且这些安全元数据字段必须唯一、无矛盾值。`Initial pairs` 对 behavioral scenario 必须为 `2`，`Maximum pairs` 不得超过 `6`；只有预注册的接近结果条件成立时才能增加 pair，不能 retry-to-success。scenario 与 rubric 必须在 candidate patch 之外冻结并绑定；candidate 不得修改它们。`Scenario class` 只能是 `deterministic`、`paired-behavioral`、`manual-environment-bound`。自动 runner 仅接受 `Replay class` 为 `sandboxed-local`，且固定 `Tool network=forbidden`、`Real targets=forbidden`、`Claude API call=expected`；这里的 API 调用仅指受预算约束的 Claude evaluation 本身，不授予工具网络访问。凭据、真实目标、不可逆动作或超出 case 授权的场景不得由自动 runner 执行。
