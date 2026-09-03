# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不保存完整实施日志，也不是第二份 roadmap。先由 `docs/context-routing.md` 选场景；当前状态以 `docs/windows-native-product-roadmap.md` 为准，历史 thin-core 验收以 `docs/real-usage-hardening-roadmap.md` 为准。

## Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-windows-native-product-v1` |
| source | `docs/windows-native-product-roadmap.md` |
| 当前批次 | `WNP-04 Release 与真实 Windows 产品验收` |
| 状态 | `in-progress` |
| 已完成 | `WNP-01 native setup/launcher`、`WNP-02 production Fresh`、`WNP-03 thematic learning batch` |
| 剩余 | 真实 Windows Fresh/visible member/duplicate Commander/learning/update product-path、Commander/member原生跨会话消息，以及实际 GitHub tag Release |
| 下一批 | 无；完成当前路线后等待真实使用反馈 |

最新结果：原生 `steamai.exe`、working-tree-authority Fresh、可见成员 launcher、单 Commander 瞬时互斥、多 candidate/多 target exact learning batch、latest-manifest update 与普通用户原生 uninstall 自清理已进入主工作树。default/full suite、vet、Windows/Linux build、diff check、local release asset、Claude Code native context/file access 与 persistent session/direct-correction live probe 已通过。当前尚不能宣称 formal GitHub Release 或完整 Windows 产品旅程完成。

## 验证标准

- 本文件与当前 roadmap 的 route/state/next 必须一致；冲突时 fail-closed。
- 不由 Markdown claim、workflow definition、fake process、cross-compile 或 synthetic fixture单独证明 live completion。
- 真实 setup/PATH、Fresh preview/apply、visible member、user correction、cross-session message、duplicate Commander、learning batch、update/uninstall 和 Release asset 必须逐项记录结果；隔离 setup/PATH/uninstall 已通过不代表其余 product journey 完成。

## 风险与注意事项

- native shell 不得演化为 control plane、session registry、消息总线、任务数据库或 compatibility runtime。
- 产品路径不使用 PowerShell、`.cmd` 或 `.bat`。
- v1 不支持 active case迁移或跨电脑 session/case同步。
