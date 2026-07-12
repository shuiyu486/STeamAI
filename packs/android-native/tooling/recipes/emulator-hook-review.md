# Emulator hook review recipe

## 目标

只读复核已存在的 emulator summary、logcat summary、Frida summary、hook candidate 或 saved dynamic sidecar，用于确认 Android native candidate 或消除误报；本 recipe 不连接设备、不运行 emulator、不 attach Frida、不执行 hook、不抓包、不 dump、不 patch、不安装/卸载应用。

## Gate 前置

如果需要设备连接、emulator run、Frida attach、hook 执行、网络请求、动态 trace、dump、patch、重签名、安装/卸载应用或外部副作用，必须先登记 pending-gate request，至少包含：

```yaml
action: device-connect | emulator-run | frida-attach | hook-execute | network-capture | trace | dump | patch | resign | install-uninstall
scope: <authorized app/library/symbol aliases and authorization summary>
isolation: <emulator/device/lab/offline/network policy>
budget:
  runtime_s: <max seconds>
  apps: <max app aliases>
  libraries: <max library aliases>
  requests: <max network requests if any>
  output_mb: <max output size>
  network: <disabled | sinkholed | explicit allowlist>
tried_light_steps:
  - APK native triage
  - emulator/hook sidecar review
stop_conditions:
  - out-of-scope app, endpoint, device, or account
  - unexpected network egress
  - output size above budget
  - destructive patch or irreversible app/device state change
  - credential, token, package, endpoint, or customer data leakage
```

## 输出

- case-local app/library/symbol alias / hook candidate id。
- JNI bridge 摘要、trigger shape、precondition、observed behavior、diff 摘要。
- sidecar path。
- verifier verdict 或 open questions。

## 禁止

- 不主动连接设备、运行 emulator、attach Frida、执行 hook、联网、抓包、dump、patch、重签名、安装/卸载应用或修改设备/app 状态。
- 不把 APK/AAB/DEX/SO、hash、包名、真实 endpoint、device/emulator id、hook script、traffic/capture、dump、trace、patch、keystore、token、账号凭据、客户上下文或绝对路径写入 pack。
- 不绕过用户确认执行动态动作、设备/app 状态写入或破坏性动作。
