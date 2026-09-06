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

仅在复合问题中，于现有方法/观察/限制中说明本 E 支持哪条连接、两端对象/版本/状态及表示转换、实际发生与静态可达的区别。预测验证须指向查看新观察前已记录的预期和条件，并保留实际结果；同源再次导出不构成独立观察。无需增加 artifact tuple 或新的状态文件。
