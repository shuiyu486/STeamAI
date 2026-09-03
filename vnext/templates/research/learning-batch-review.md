# `{{LEARNING_BATCH_REVIEW_ID}}` — `{{BATCH_TITLE}}`

- Reviewer 单写者：`{{REVIEWER}}`
- Selected pack：`{{PACK_NAME}}`
- Case revision：`{{PACK_REVISION}}`
- Canonical HEAD：`{{CANONICAL_HEAD}}`
- Snapshot digest：`{{SNAPSHOT_DIGEST}}`
- Patch：`{{PATCH_REF}}`
- Patch SHA-256：`{{PATCH_SHA256}}`
- Added-lines deny result：`{{ADDED_LINES_DENY_RESULT}}`
- `git apply --check` result：`{{GIT_APPLY_CHECK_RESULT}}`
- Candidate mapping/theme：`{{CANDIDATE_MAPPING_THEME_RESULT}}`
- Dedup/conflict/counterexamples：`{{DEDUP_CONFLICT_COUNTEREXAMPLES_RESULT}}`
- Redaction：`{{REDACTION_RESULT}}`

## Candidates

按 candidate path 排序；每项必须有独立且 current 的 `eligible` review。

{{CANDIDATE_BINDINGS}}

`CANDIDATE_BINDINGS` 必须替换为连续的实际记录；每项依次填写 Candidate、Candidate SHA-256、Eligibility review、Eligibility review SHA-256 与 Destination，不保留示例或占位符。

## Targets

按 target path 排序；preimage 绑定 canonical working-tree 当前 bytes，postimage 绑定完整 patch 应用后的 exact bytes。

{{TARGET_BINDINGS}}

`TARGET_BINDINGS` 必须替换为连续的实际记录；每项依次填写 Target、Preimage SHA-256/bytes 与 Postimage SHA-256/bytes，不保留示例或占位符。

## Final decision

- Decision：`{{BATCH_DECISION}}`

只有 `accepted` 才能请求用户确认。Reviewer 必须完整阅读 candidate、eligibility review 和未截断 patch，确认每个 candidate 只映射到自己的 destination、所有 targets 都由至少一个 eligible candidate 支持、batch 主题一致、无重复/冲突/敏感内容，且 patch 不 create/delete/rename/copy/change mode。任一 binding 漂移后重新生成 batch review，不修改旧字段伪装 current。
