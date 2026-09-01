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
- Evidence/generalization：`{{EVIDENCE_GENERALIZATION_CHECK}}`
- Applicability/counterexamples：`{{APPLICABILITY_CHECK}}`
- Dedup/conflict：`{{DEDUP_CONFLICT_CHECK}}`
- Redaction/denyPatterns：`{{REDACTION_CHECK}}`
- Target allowlist/currentness：`{{TARGET_ELIGIBILITY_CHECK}}`

只有 `eligible` 才能生成无权威 proposal patch。`needs-evidence` 返回原 finding owner；`disputed`、`superseded` 或 `ineligible` 不生成 proposal。

## Checkpoint B — Exact proposal patch

- Canonical target：`{{PACK_RELATIVE_PATH}}`
- Base revision：`{{BASE_REVISION}}`
- Manifest base blob：`{{MANIFEST_BASE_BLOB}}`
- Target base blob：`{{TARGET_BASE_BLOB}}`
- Patch path：`{{PATCH_REF}}`
- Patch SHA-256：`{{PATCH_SHA256}}`
- Patch target count：`1`
- Added-lines deny result：`{{PATCH_DENY_RESULT}}`
- `git apply --check` result：`{{PATCH_APPLY_CHECK_RESULT}}`
- Patch decision：`{{PATCH_DECISION}}`

只有 Checkpoint A 为 `eligible`、Checkpoint B 为 `accepted`，且上方所有 identities current，Commander 才能向用户展示完整 exact patch 并请求确认。candidate、review、patch、manifest、target、source evidence 或 snapshot 任一漂移都使该 review stale；不得修改旧字段伪装 current，应重新生成 proposal 并重新审查。
