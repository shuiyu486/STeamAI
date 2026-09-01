# `{{EVIDENCE_ID}}` — `{{SUBJECT}}`

- Owner：`{{OWNER}}`
- Artifact alias：`{{ARTIFACT_ALIAS}}`
- Artifact path：`{{ARTIFACT_CASE_RELATIVE_PATH}}`
- Artifact SHA-256：`{{ARTIFACT_SHA256}}`
- Artifact bytes：`{{ARTIFACT_BYTES}}`
- Authorized use：`{{ARTIFACT_AUTHORIZED_USE}}`
- 方法：`{{METHOD}}`
- 证据定位：`{{EVIDENCE_REF}}`
- 观察：`{{OBSERVATION}}`
- Confidence：`{{CONFIDENCE}}`

## 限制与不确定性

`{{LIMITATIONS}}`

上方 artifact tuple 必须与当前 `artifacts/index.md` 的同 alias entry 以及当前 case-local artifact bytes 同时一致；alias 重绑、path/SHA/bytes/authorized-use 变化或 artifact bytes 漂移都会使本 evidence stale。本文只保存可复查摘要和证据定位；长反汇编、trace、dump、capture 或工具日志留在 case-local sidecar，不内联。
