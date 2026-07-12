# Android native tooling

本目录保存 android-native pack 的工具 catalog、recipes、脚本接口和候选工具经验。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具 capability card、状态、sidecar、预算和止损条件。 |
| `recipes/*.md` | 按任务阶段记录工具用法。 |
| `candidates/` | 从 case 回流的候选工具经验。 |

## 原则

- 工具经验先 recipe 化，再考虑 adapter 化。
- 不硬编码本机路径；使用 `<caseRoot>`、`<toolsRoot>`、`<appRef>`、`<libraryRef>`、`<sidecar>`。
- 不保存 APK/AAB/DEX/SO、hash、包名、真实端点、device/emulator id、hook script、traffic/capture、dump、trace、patch、keystore、token、账号凭据、客户上下文或绝对路径。
- 设备连接、emulator run、Frida attach、hook 执行、网络请求、动态 trace、dump、patch、重签名、安装/卸载应用和外部副作用默认是 gated action。
