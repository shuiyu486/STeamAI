# `{{REPLAY_RESULT_ID}}` — Result for `{{REPLAY_SPEC_ID}}`

- Verifier：`{{VERIFIER}}`
- Replay spec：`{{REPLAY_SPEC_REF}}`
- Replay spec SHA-256：`{{REPLAY_SPEC_SHA256}}`
- Finding：`{{FINDING_REF}}`
- Finding SHA-256：`{{FINDING_SHA256}}`
- Environment observed：`{{ENVIRONMENT_OBSERVED}}`
- Started/finished：`{{TIME_WINDOW}}`
- Budget used：`{{BUDGET_USED}}`
- Result：`{{RESULT}}`
- Output artifact alias：`{{OUTPUT_ARTIFACT_ALIAS_OR_NONE}}`
- Output artifact path：`{{OUTPUT_ARTIFACT_CASE_RELATIVE_PATH_OR_NONE}}`
- Output artifact SHA-256：`{{OUTPUT_ARTIFACT_SHA256_OR_NONE}}`
- Output artifact bytes：`{{OUTPUT_ARTIFACT_BYTES_OR_NONE}}`
- Output artifact authorized use：`{{OUTPUT_ARTIFACT_AUTHORIZED_USE_OR_NONE}}`
- Attempts：`{{ATTEMPTS_WITH_RESULT_AND_BUDGET}}`

## Observation

`{{OBSERVATION}}`

## Variance and deviations

`{{VARIANCE_AND_DEVIATIONS}}`

## Limitations

`{{LIMITATIONS}}`

`Result` 只能是 `reproduced`、`not-reproduced`、`blocked`、`invalidated` 或 `inconclusive`。结果必须来自不依赖 owner session memory 的 verifier；同一模型的新 session 只提供上下文隔离，不等于独立认知来源。所有实际 attempt、预算、偏差和负向结果都必须保留，不得重试到出现预期答案。原始输出留在 case-local artifact，本文件只保存可复查摘要与 alias/path/SHA-256/bytes/authorized-use exact binding。
