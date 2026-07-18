# Android native Agent Team routes

## 1. 角色边界

- `main`：确认授权 app / library / device 范围、静态/动态边界、隔离要求、case-local sidecar 位置和 gate 状态；合并 reviewer verdict，写 ledger / handoff，并在确认后更新 authority 文档。
- `native-analysis`：在自己的 workspace 中分析单个 app alias、native library alias、JNI symbol、ABI、component、hook candidate 或 emulator summary，产出 observation / request / candidate。
- `reviewer`：只读复核 bounded JNI hypotheses、native library notes、hook candidates、emulator summaries 或 tooling notes，输出 verdict，不连接设备、不 attach Frida、不执行 hook、不写文件。
- `tooling`：描述工具能力、输入输出、sidecar、预算、隔离要求和止损；默认不执行动态动作或自动写回 app/device 状态。

## 2. 默认 routes

| route | 适用任务 | 分片 | 权限 | 输出 |
|---|---|---|---|---|
| `android-native:bounded-review` | finding / evidence / JNI / hook / tooling review | library-or-finding | read-only | reviewer verdict |
| `android-native:native-analysis` | APK native / SO / JNI / Frida hook / emulator sidecar 分析 | app-library-or-symbol | read-only-or-workspace-only | observation / request / candidate |

`plan-subagents` 只生成 review packet 与 observability，不自动 spawn agent。主会话负责启动 Agent 工具、收集输出，并用 `/rekit note` 写回 verification / decision。

## 3. Packet 输出契约

所有子 agent 输出都必须包含：

```text
item, decision, confidence, evidence, risk, next_action, tier_used, tool_scope, defer_reason
```

Android native route 可追加：

```text
app_ref, library_ref, jni_symbol_ref, abi, component_ref, hook_candidate_ref, candidate_path
```

`app_ref`、`library_ref`、`jni_symbol_ref`、`component_ref` 与 `hook_candidate_ref` 应是 case-local 脱敏引用或 sidecar id，不是 APK 路径、包名、hash、真实 device id、hook script、endpoint、token、traffic capture、dump 路径或绝对路径。`decision` 是 reviewer output decision，不等同于 ledger canonical decision；main 合并后再写 `/rekit note -Kind verification` 与 `/rekit note -Kind decision`。

## 4. Review-first 门禁

- accepted JNI hypothesis / hook candidate / emulator finding 只能进入 main 合并队列，不能直接写 confirmed / authority / report。
- 证据不足时使用 `defer` 或 `needs-more-evidence`，并给出下一步轻量验证。
- 需要设备连接、emulator run、Frida attach、hook 执行、网络请求、动态 trace、dump、patch、重签名、安装/卸载应用或外部副作用时，先经 `/rekit gate` preflight；只有本次显式用户确认，或 strict validated durable autonomy profile + 覆盖本次边界的 `authorized-gate`，才允许 executor 执行。`gate -Apply` 本身只记录 request decision，不执行 heavy action。
- 每个 shard 的失败只影响本 shard；不要阻塞无关 app、library、JNI symbol、component 或 hook candidate。

## 5. 证据与 sidecar

- evidence 应引用 case-local sidecar 路径、app alias、library alias、JNI symbol alias、ABI、component alias、tool summary、时间窗口和脱敏 row id。
- 不在 pack reference 中保存 APK/AAB/DEX/SO、hash、包名、真实端点、device/emulator id、hook script、traffic/capture、dump、trace、patch、keystore、token、账号凭据、客户上下文或绝对路径。
- 任何可复用经验进入 pack 前必须清理样本特征、hash、package name、endpoint、device id、hook/traffic/trace/dump/patch 细节和 case-specific dynamic result。
