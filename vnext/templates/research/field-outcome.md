# `{{OUTCOME_ID}}` — Field outcome for `{{LEARNING_ID}}`

- Owner：`{{OWNER}}`
- Observation mode：`explicit-user-opt-in`
- Learning/provenance：`{{LEARNING_REF}}`
- Learning/provenance SHA-256：`{{LEARNING_SHA256}}`
- Source finding：`{{SOURCE_FINDING_REF}}`
- Source finding SHA-256：`{{SOURCE_FINDING_SHA256}}`
- Relevant pack tree：`{{PACK_TREE}}`
- Environment class：`{{ENVIRONMENT_CLASS}}`
- Outcome：`{{improved_neutral_regressed_inconclusive}}`
- Artifact tuple refs：`{{ARTIFACT_REFS_WITH_PATH_SHA256_BYTES_AUTHORIZED_USE}}`
- Evidence refs：`{{EVIDENCE_REFS_WITH_SHA256}}`
- Source accepted review：`{{SOURCE_REVIEW_REF}}`
- Source review SHA-256：`{{SOURCE_REVIEW_SHA256}}`
- User disclosure confirmation：`{{CONFIRMATION_REF}}`

## Observed effect

`{{OBSERVED_EFFECT}}`

## Competing explanations

`{{COMPETING_EXPLANATIONS}}`

## Redaction and transfer boundary

`{{REDACTION_BOUNDARY}}`

field outcome 只能由当前 case 的用户对本份脱敏记录明确提出或确认创建；必须 exact 绑定当前 artifact alias/path/SHA-256/bytes/authorized-use tuple、evidence、finding 与 current accepted review。不得自动遥测、发现、扫描、拉取、比较或汇总其它 case；`neutral`、`regressed` 与 `inconclusive` 是一等结果，append-only 保留，不能删除或覆盖。单个 outcome 不能证明普遍有效；V4 需要多个独立后续 case 的 current accepted outcomes，并由用户逐份检查后，通过新的 candidate/review/exact batch/confirmation 形成脱敏、聚合的 provenance patch。
