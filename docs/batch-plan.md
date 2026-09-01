# Batch implementation plan

## 读取指南

本文件只是已完成路线的短投影，不选题、不保存完整实施日志。本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；验收事实以 `docs/real-usage-hardening-roadmap.md` 的完成卡为唯一 source。完整历史只在 Git history 或 `CHANGELOG.md` 按 ID 查询。

## 实施摘要

`steamai-vnext-thin-core-v1` 已完成；当前无待领取批次。后续只根据真实使用反馈、新授权边界或明确产品目标立项。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-vnext-thin-core-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `VNT-06 收尾发布验证` |
| 状态 | `completed` |
| 唯一允许领取 | 无 |
| 前置 | `VNT-05` 旧控制面、旧文档入口与 current pack/common 旧语义已删除 |
| 下一批 | 无；等待真实使用反馈 |

最新结果：VNT-01～VNT-06 与后续 RGH-00～RGH-03 硬化均已完成。canonical `/steamai`、case-local contracts、schema v2、Fresh staged publication、pack+common complete payload digest、Day-2 hash-bound review、accepted-only exact patch learning 与原生 Claude Code 多会话合同均已收口；旧项目 importer 因无兼容需求已删除。RGH 收口的本地 default suite、vet、diff 与终审通过；Windows live context/file-access probe 和三平台 remote contract CI 是重构基线证据，不代表 RGH 改动的 product-path/persistent/manual live 或 remote green。

历史 numbered batch 只从 Git history/`CHANGELOG.md` 按需查询，不在当前完成态投影中固化旧 identity。

## 验证标准

- 本文件与路线图的 route/current/state/next 必须一致；冲突时 fail-closed。
- 本文件只保留 current projection、一句最新结果和 release handoff 所需的 latest numbered identity；未来阶段细节只在按需 backlog。
- route completion 不能由 Markdown claim、cross-compile、workflow definition 或 synthetic fixture 单独证明。

## 风险与注意事项

- 当前产品只支持 fresh/current；旧项目 importer 与兼容路径已删除。
- 真实 heavy action 仍要求明确 case 授权、具体用户确认与 Claude Code 工具权限。
- 不因未来需求复活旧 runtime、双写、adapter 或任务状态机。
