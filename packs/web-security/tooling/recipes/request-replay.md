# Bounded request replay recipe

## 目标

对一个已由 `openapi-v3-json-inventory` 绑定的授权操作执行一次确定性回放，用 digest diff 确认 candidate finding 或消除误报。

## 输入合同

1. 先生成并绑定 canonical OpenAPI inventory；source 只读，inventory 不联网、不持久化 secret。
2. replay request 必须符合 `../schemas/bounded-replay-request-v1.schema.json`，并以 canonical bytes 的 SHA-256 发布到：

```text
inputs/bounded-replay-requests/<request-sha256>.json
```

3. request 只允许：
   - exact `http|https` scheme、loopback IP literal（`127.0.0.0/8` 或 `::1`）、port 与 basePath；
   - inventory 中唯一匹配的 `GET`、`HEAD` 或 `OPTIONS` operation；
   - one request、zero redirects、zero retries、no request body、no ambient proxy；
   - 最多 30 秒 runtime 与 1 MiB response body；
   - 可选认证只写 `authRef: STEAMAI_AUTH_<NAME>`，secret value 仅在 child environment 中提供，绝不写入 request/result/dispatch/report/receipt/observation。

非 content-addressed 路径、scheme/host/port/basePath 漂移、inventory binding 漂移、unsupported method、未知 `authRef` 或边界缺失都必须在 child launch 前 fail-closed。

## Gate 与执行

执行前先经 project-local gate preflight。Gate Apply 只记录 request decision，不执行 HTTP；actual replay 只有在 strict durable autonomy profile 与 exact `authorized-gate` 同时覆盖 target、budget、output path、stop conditions 和 current owner generation 时，才由 fixed compiled-in `bounded-http-replay` child 执行。

Catalog `entry` 只作 provenance，永远不作为命令执行。Transport/endpoint/delivery observation 不授予权限；`delivery-uncertain` 是 terminal result，不重试，也不做 same-job replacement。

## 输出合同

result 必须符合 `../schemas/bounded-replay-result-v1.schema.json`，只持久化：

- exact request / inventory file binding；
- exact target 与 operation；
- terminal status：`matched`、`different`、`failed-before-delivery`、`delivery-uncertain` 或 `aborted-after-delivery`；
- delivery attempts/certainty 与 bounded error code；
- actual status code、body SHA-256/bytes，以及最多一个 `content-type` SHA-256/bytes；
- deterministic `match/statusMatch/bodyMatch/headersMatch`；
- limits 与 fixed boundary flags。

不持久化 response body、raw headers、timing trace、request body、credential、token、cookie、JWT、API key 或 payload。Result 完成后仍需独立 evidence Reviewer 接受，才能发布 verification acknowledgement、closure 与 owner-generation-bound member task。

## 禁止

- 不访问公网、域名或非 loopback IP；不自动扫描、枚举、fuzz、bruteforce、credential stuffing、exploit replay 或 DoS。
- 不使用 ambient proxy、redirect、retry、same-job replacement 或 catalog entry execution。
- 不把 secret 或完整 request/response 写入 durable artifact、pack、fixture、日志或聊天。
- 不把 gate/profile/transport/currentness 当成 authority、confirmed 或无限权限。
