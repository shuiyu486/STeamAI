# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志，也不是第二份 roadmap。先由 `docs/context-routing.md` 选场景；当前状态以 `docs/verified-learning-roadmap.md` 为准。已完成 Windows 产品基线见 `docs/windows-native-product-roadmap.md`，历史 thin-core 验收见 `docs/real-usage-hardening-roadmap.md`。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-verified-learning-v1` |
| source | `docs/verified-learning-roadmap.md` |
| 当前批次 | `VL-05 live gate 与真实 calibration` |
| 状态 | `mechanical verification and independent full review passed；live gates pending` |
| 已实现 | `Gate 0 integrity`、`Gate 1 replay contract`、`Gate 2 bounded runner`、`Gate 3 attestation-aware promotion`、`Gate 4 explicit opt-in outcome contract` |
| 已验证 | full suite、vet、Windows/Linux build、Windows test cross-compile 与 native evaluation runtime、diff check |
| 待验证 | 固定开关的 live evaluator test；Windows process-tree 与 locked-file update recovery live paths |
| live pending | actual Claude calibration；真实 candidate comparative journey；多个后续 case field outcomes |
| 下一批 | calibration 为 `go` 后执行真实 behavioral promotion journey；若为 `no-go/inconclusive` 则先修 evaluator，保持 promotion fail-closed |

当前判断：mechanical implementation 已闭合到 production API/CLI 和 case-pinned contracts；默认 synthetic tests 不调用模型。未完成的 live evidence 不由模板、fake Claude、cross-build 或旧 v1.0.4 证据替代。

## 验证标准

- `go test -count=1 -p=2 -timeout=30m ./...`
- `go vet ./...`
- Windows/Linux build 与 `git diff --check`
- evaluator/calibration/comparative/field evidence 按 `vnext/acceptance.md` 独立记录
- no-go/inconclusive 必须保留并阻止 behavioral V3 promotion

## 风险与注意事项

- native runner 不是 control plane、自动 judge、遥测或跨 case aggregator。
- 产品路径不使用 PowerShell、`.cmd` 或 `.bat`。
- `accepted`、eligible、用户 confirmation、Apply 或 Git staging 不提升 V0–V4 maturity。
- 真实 V4 依赖未来多个独立后续 case 的逐份 opt-in evidence，不能为关闭路线而模拟。
