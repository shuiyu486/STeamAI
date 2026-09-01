# Binary RE case handoff template

> 复制到 case-local 目录后填写；本模板不得保存真实样本名、地址、trace 路径或 case 进度。

## Scope

- Case alias：`<CASE_ALIAS>`
- Authorization summary：`<AUTHORIZED_SCOPE>`
- Prohibited actions：`<PROHIBITED>`
- Stop conditions：`<STOP_CONDITIONS>`

## Team

| Member | Current task | Allowed writes | Status |
|---|---|---|---|
| `<member>` | `<bounded task>` | `<case-relative scope>` | pending |

## Findings and review

- Accepted finding refs：`<FINDING_REFS>`
- Open evidence gaps：`<GAPS>`
- Reviewer decisions：`<REVIEW_REFS>`

## Next action

- Owner：`<OWNER>`
- Action：`<ONE_NEXT_ACTION>`
- Inputs：`<BOUNDED_REFS>`
- Expected output：`<OUTPUT>`
- Stop conditions：`<TASK_STOP>`

## Boundary

大 binary、trace、dump、decompile、disasm 与 raw tool output 留在 case-local artifact/sidecar；本文只记录脱敏摘要和相对 evidence refs。
