# APK native triage recipe

## 目标

在不安装应用、不连接设备、不执行 hook、不抓包、不 dump、不 patch 的前提下，建立 app/library/JNI symbol 别名、manifest / ABI / native library / exported component 摘要、JNI bridge hypothesis 和 open questions 的最小索引。

## 输入

- 授权范围摘要。
- case-local app alias、library alias 或 JNI symbol id。
- 已脱敏的 manifest summary、native library inventory、ABI summary、JNI symbol summary、component summary、string/import summary 或 static tool sidecar。

## 输出

```text
app_ref, library_ref, jni_symbol_ref, abi, component_ref, native_hypothesis, evidence_ref, open_questions
```

输出写入 case-local sidecar 或 lane workspace；聊天和 Markdown 只引用摘要与路径。

## 止损

- 发现 APK/package/hash、真实 endpoint、device/emulator id、hook script、traffic/capture、token、keystore、账号凭据、客户上下文或绝对路径时停止提升到 pack，保留在 case-local 并标记 redaction needed。
- APK、DEX、SO、logcat、capture、trace、dump 或工具 raw output 过大时先按 app / library / symbol / evidence row 分片，不把完整内容粘入聊天或 Markdown。
- 证据不足以判断 JNI/native 路线时产出 request，不直接升级为 confirmed。
