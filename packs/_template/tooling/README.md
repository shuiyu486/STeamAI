# Template tooling

本目录保存新 pack 的工具 catalog、recipes、脚本接口和候选工具经验。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具 capability card、状态和止损条件。 |
| `recipes/*.md` | 按任务阶段记录工具用法。 |
| `candidates/` | 从 case 回流的候选工具经验。 |

## Harmless read-only 验收 adapter

`catalog.yml` 中的 `rekit-readonly-inspector` 是框架维护用的最小 Go-owned adapter：它只接受 `_template` 的 `inspect` action、strict durable preauthorization、current lane owner 和已落盘 immutable dispatch；读取 exact case-local bounded regular text fixture，并只在授权 output path 独占写入 `inspection.json` 与 `adapter-report.json`。它不访问网络，不执行 debug、patch、dump、hook 或目标修改，也不是领域分析工具。

显式维护验收先构建 `rekit-adapter-host`，再运行 `rekit-adapter-acceptance` 并把 receipt 写到 disposable case 和模板仓库之外。Windows 验收器持有 adapter executable handle，`CREATE_SUSPENDED` 后验证 actual mapped image，再加入 kill-on-close Job Object并resume；timeout先关闭Job再有界回收进程树。验收链走 dispatch → adapter process → receipt → validation → observation → acknowledgement → Mission Commander resume；失败输出只按exact handle删除owned object，disposable case先no-replace quarantine并拒绝replacement/reparse后再清理。普通 `/rekit gate` 仍只记录授权、dispatch、receipt 或 observation evidence，不启动 adapter。

```text
go build -o <temp>\rekit-adapter-host.exe ./cmd/rekit-adapter-host
go run ./cmd/rekit-adapter-acceptance -repo . -adapter <temp>\rekit-adapter-host.exe -receipt <outside-case-receipt.json>
```

receipt 路径与临时 executable 由维护者管理；已有 receipt 不覆盖，验收失败也返回 `passed=false` 的机器结果。普通测试不会自动运行此 live gate。

## 原则

- 工具经验先 recipe 化，再考虑 adapter 化。
- 不硬编码本机路径；使用 `<caseRoot>`、`<toolsRoot>`、`<target>`。
- adapter 必须先记录 immutable dispatch，并绑定 exact owner、session、catalog、budget、path 与 `authorized-gate`。
- 不保存真实样本、trace、dump 或完整工具输出。
