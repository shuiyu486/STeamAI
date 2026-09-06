# 单次 curl 读取取证

把一个明确的 Web/API 读取问题落实为一次可复核操作。研究判断仍使用 [单一请求假设与有界复核](request-replay.md)；本页不提供 executor，不把命令成功等同于权限验证，也不授权扫描或批量重放。

## 使用前

- 已获准一个 exact loopback origin、对象、身份和 GET；本次请求为何能改变判断、允许输出、时间/空间预算和停止条件已说明。授权与 Claude Code 工具权限必须同时成立。
- 使用已核对的原生 `curl`，至少支持 8.4.0 起对未知长度下载生效的 `--max-filesize`；交付时以实际测试过的版本为准，不自动安装或回退。
- URL 使用明确的 `127.0.0.1` 或 `[::1]` 与已确认端口，不允许用户信息、凭据、片段、URL 列表或未经核对的路径/查询。方法名 GET 不保证没有副作用。
- 输出目录已存在、属于当前 case 允许的 artifact 目录且无 symlink/reparse 重定向；选择一个尚不存在的文件。先检查目标，不覆盖、不自动创建目录或删除已有材料。
- 身份对照是两次分别获准的单次操作，不放进循环；每次使用新的输出文件，并核对同一资源、版本、归属和状态。预设 owner/non-owner 是合法对照变量，不把非预期会话失效当权限差异。

## 原生命令

将占位符替换为本次已确认值。`--disable` 必须是第一个参数；下面是一条命令，不是 PowerShell/.cmd/.bat 产品包装。

```text
curl --disable --globoff --noproxy "*" --proto "=http,https" --no-location --max-redirs 0 --retry 0 --connect-timeout 5 --max-time 15 --max-filesize 262144 --header "Accept-Encoding: identity" --no-clobber --output "<CASE_ARTIFACT_FILE>" --silent --show-error --write-out "status=%{response_code};peer=%{remote_ip};redirects=%{num_redirects};body-bytes=%{size_download};file=%{filename_effective}\n" --url "http://127.0.0.1:<PORT>/<EXACT_PATH>"
```

- `--disable` 不加载默认 curlrc；`--noproxy "*"` 禁用环境代理，literal loopback 不依赖 DNS。不得附加 `--config`、`--next`、第二 URL、`--location`、自动认证协商或 retry 参数。
- 不使用 `--compressed`，只要求 identity 编码；服务器不遵守时保留原始压缩 bytes，不自动解压扩大输出。curl 8.4.0～8.19.x 的大小限制不能据此宣称覆盖自动解压后的大小。
- 没有 `--fail`：保留 4xx/5xx 的有界 body，HTTP 拒绝和传输失败分别解释。不打印全部响应头，不使用 verbose/trace 避免认证、cookie 或敏感 header 落盘。
- `--max-filesize` 限制响应 body，不是 header、协议流量或整个进程内存的总上限；`--max-time` 是 curl 传输时限，不是外部进程树的硬隔离。若任务要求更严格总资源上限，而当前工具环境不能保证，就停止，不声称已满足。
- `--no-clobber` 会给冲突文件另加数字后缀，**不是遇冲突零动作拒绝**。因此前检目标不存在仍然必要；结束后核对 `filename_effective` 等于 exact 输出。若发生冲突改名，保留实际路径并报告异常，不自动删除或重试；请求可能已经送达。
- 需要身份材料时，仅增加一个经用户核对的 `--header "@<CASE_PRIVATE_HEADER_FILE>"`。该文件只包含本次允许的认证 header，不能包含 Host、代理、传输/连接控制或 CRLF 注入；由受控渠道供应，不把值放入 argv、提示词、共享记录或 pack。命令不自动管理凭据、不调用 ambient cookie/netrc/SSO；不知道如何安全供应身份则停。验收使用无真实秘密的合成身份。

## 执行后先判断材料，再判断业务

1. 保存退出码和上方有限元数据；对实际 body 文件计算 SHA-256/bytes 并登记现有 artifact index，记录本次 target/operation/identity 引用及观察时间。不要复制真实地址、身份或响应到 pack。
2. 核对 peer 为已批准 loopback、没有跟随 redirect、actual output 路径正确，响应是否完整、是否命中时限/大小上限。收到 3xx 只记录原响应，不自动访问 Location。
3. 退出码非零时，不从该码独自推断请求未送达。仅有明确的本地发出前证据才记 `failed-before-delivery`；否则为 `delivery-uncertain` 或有送达依据的 `aborted-after-delivery`，保留可能的副作用，禁止自动重试。
4. 限额/中断留下的文件只是部分材料，不声称完整 body hash、完整权限结果或“没有敏感字段”。未生成文件与服务端未处理也不是同一事实。
5. 从允许的有界响应中定位与假设有关的业务字段，结合规则和对象身份形成 E/F。只有状态码、长度或摘要不同，不足以得出漏洞或修复结论。
6. 需要第二观察时先说明它能区分什么，再获得该具体动作的确认；复用现有 [请求序列方法](../../references/web-security/request-sequence-review.md)。发送不确定、版本/身份漂移、敏感输出或意外副作用时停止并告知 Commander。

## 输出与复核

原 body 留在 case artifact，E 记录完整性、定位及来源，F 说明可支持、被否定和未知的范围，Reviewer 独立检查。需要实际业务终态时另取获准的最小观察；本命令只证明这一次读取，不证明服务端内部实现。

[request v1](../schemas/bounded-replay-request-v1.schema.json) / [result v1](../schemas/bounded-replay-result-v1.schema.json) 继续只描述原窄合同；本操作不解析它们，也不自动输出符合 result schema 的结果。多个独立 GET 不得冒充一条 schema request；这里没有新增 POST/body、多步 runner、重试器或数据库。
