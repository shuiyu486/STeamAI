# `{{LEARNING_REVIEW_ID}}` — Review of `{{LEARNING_ID}}`

- Reviewer 单写者：`{{REVIEWER}}`
- Candidate：`{{CANDIDATE_REF}}`
- Candidate SHA-256：`{{CANDIDATE_SHA256}}`
- Source finding：`{{SOURCE_FINDING_REF}}`
- Source finding SHA-256：`{{SOURCE_FINDING_SHA256}}`
- Source accepted review：`{{SOURCE_REVIEW_REF}}`
- Source review SHA-256：`{{SOURCE_REVIEW_SHA256}}`
- Selected pack：`{{PACK_NAME}}`
- Source revision：`{{PACK_REVISION}}`
- Pack tree：`{{PACK_TREE}}`
- Common tree：`{{COMMON_TREE}}`
- Snapshot digest：`{{SNAPSHOT_DIGEST}}`
- Proposed destination：`{{PACK_RELATIVE_PATH}}`

## Checkpoint A — Eligibility

- Decision：`{{ELIGIBILITY_DECISION}}`
- Evidence/generalization：`{{EVIDENCE_GENERALIZATION_RESULT}}`
- Applicability/counterexamples：`{{APPLICABILITY_COUNTEREXAMPLES_RESULT}}`
- Dedup/conflict：`{{DEDUP_CONFLICT_RESULT}}`
- Redaction/denyPatterns：`{{REDACTION_DENY_RESULT}}`
- Target allowlist/currentness：`{{TARGET_CURRENTNESS_RESULT}}`

只有 `eligible` 才能进入 thematic learning batch。`needs-evidence` 返回原 finding owner；`disputed`、`superseded` 或 `ineligible` 不进入 batch。本文件只判断 candidate eligibility；candidate review 不绑定或授权任何 patch，也不接受任何 patch；最终 exact patch 由独立的 learning batch review 绑定。candidate、source evidence 或 snapshot 任一漂移都使本 review stale。
