---
name: verify
description: Verify rekit CLI/runtime changes by driving the public Go CLI or PowerShell façade surface.
---

# verify

用于本仓库变更的运行时观察验证。不要把 `go test` / `go vet` 当作本 skill 的证据；这些属于后续 release validation。

## 常用运行面

- Manifest / pack inventory / schema contract：运行 `go run ./cmd/rekit -- -Command packs -Format json`，观察对应 pack 的 `schemaValid`、`error` 与 heavy-tool gate inventory。
- Gate 默认值或 heavy-tool gate contract：创建临时 case，再用 `go run ./cmd/rekit -- -Command gate -Target <caseRoot> -Pack <pack> -WhatIf -Format json -Action <action> -Lane <lane>` 观察 pending-gate preview。只跑 `-WhatIf`，或在需要 ledger 写入语义时跑 `-Apply` pending-gate request；绝不执行实际 heavy-tool。
- Kit release inventory：运行 `go run ./cmd/rekit -- -Command release-check -Format json`，只读观察 readiness envelope。
- PowerShell façade 行为：只在 diff 触及 façade 或 smoke contract 时运行 `./rekit/rekit.ps1` 对应命令；保持 dry-run / WhatIf / pending-request 边界。

## 临时 case 模式

```powershell
$caseRoot = Join-Path $env:TEMP ('rekit-verify-' + [guid]::NewGuid().ToString('N'))
go run ./cmd/rekit -- -Command init -Target $caseRoot -Pack _template -ProjectName verify -Apply -Format json
go run ./cmd/rekit -- -Command gate -Target $caseRoot -Pack _template -WhatIf -Format json -Action symex -Lane main -Subject verify-gate
```

验证结束后可删除自己创建的 `$caseRoot`。不要在 kit 仓库内伪造 case state。
