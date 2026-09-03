# Windows 原生产品验收

本文件定义 `steamai-windows-native-product-v1` 的证据分层。所有自动 fixture 使用临时目录和无真实样本内容；结果不写入模板仓库，不把 session ID、绝对 case path 或 artifact bytes 保存为产品状态。

## 证据分层

以下层级互不替代：

1. **Default automated tests**：生产 `casebootstrap`、`learningbatch`、native shell 的 deterministic filesystem/Git/command contract。
2. **Windows native live**：真实 `steamai.exe`、HKCU Registry/PATH、NTFS publication、named mutex 与 `CREATE_NEW_CONSOLE`。
3. **Claude Code live**：真实 context/file access、visible independent sessions、用户直接纠偏与原生跨会话消息。
4. **Release live**：tag workflow实际生成、上传、下载和校验 release assets。

fake platform、synthetic fixture、remote CI definition、cross-compile 或 `go test -run` 零匹配不能冒充其它层。

## 1. Default automated gates

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

focused：

```text
go test -count=1 ./internal/steamai/casebootstrap
go test -count=1 ./internal/steamai/learningbatch
go test -count=1 ./internal/steamai ./internal/steamai/vnextcontract
```

必须覆盖：Fresh zero-write/exact confirmation/source+target drift/current deep validation；learning 多 candidate/多 target、Reviewer binding、mutable patch TOCTOU、HEAD/index/snapshot不漂移、target-only rollback；setup/update/uninstall command边界；禁止产品脚本和旧 control plane。

## 2. Claude Code capability probe

显式 opt-in：

```text
STEAMAI_VNEXT_LIVE_ACCEPTANCE=1 go test -count=1 -run TestLiveNativeContextAndFileAccess ./internal/steamai/vnextcontract
```

该 probe 只证明成员 cwd 自动上下文与 `--add-dir` case file access。缺少 `claude`、认证或 CLI capability 必须失败；默认 suite 不发起模型调用。它不能证明用户看见窗口、直接纠偏或跨会话协作。

保留的自动 persistent probe（仍不等于人工可见验收）：

```text
STEAMAI_VNEXT_PERSISTENT_MULTISESSION_ACCEPTANCE=1 go test -count=1 -run TestLivePersistentMemberContextAndCorrection ./internal/steamai/vnextcontract
```

## 3. Windows native product journey

在一次性本机测试账户或明确可恢复的临时安装边界中，用真正构建的 `steamai-windows-amd64.exe` 完成：

1. **setup**：默认路径与 `--source <TEMP_CHECKOUT>` 各一次；检查 installed exe、HKCU source/version/PATH ownership；新终端可解析 `steamai`。不使用 PowerShell/.cmd/.bat 产品脚本。
2. **Fresh**：从外部普通临时项目运行 `steamai`；同一 Commander窗口看到 exact preview；确认前零写；输入 exact synthetic confirmation 后 project-local skill、contracts、pack/common snapshot与marker current。
3. **Fresh drift**：改变 source或target后，旧确认失效且`.steamai-vnext/`不发布。
4. **Visible member**：Commander调用 `steamai __open-member <name>`；屏幕上立即出现普通交互 Claude Code窗口，cwd是成员目录，case通过`--add-dir`可读。
5. **Duplicate Commander**：第一个仍运行时再次在物理同一case（包括path alias）运行`steamai`，第二个明确拒绝。
6. **Learning batch**：至少3 candidates→3 eligibility reviews→2 targets→1 accepted batch review；preview完整显示source chain/Reviewer/pre-postimage/patch；exact confirmation后只修改targets，HEAD/index/case snapshot不变。
7. **Update**：clean checkout从已发布测试tag更新；manifest/hash/tag/revision均匹配；source需要变化时与exe一起切换，HEAD已等于release时走exe-only路径。再分别制造dirty/untracked/ignored、本地其他branch或stash commit、错误hash、错误revision、已存在staging/backup、文件锁、准备期间source漂移与网络失败，确认旧可用版本保留且无自动Git修复；source替换成功后旧checkout作为sibling backup保留，命令输出路径且不自动递归删除。
8. **Uninstall**：只移除installed exe、setup拥有的PATH和定位信息；checkout、local commits和case保留；运行中exe先重命名，同字节临时原生 helper 在卸载命令退出后删除已安装入口；普通用户不需要管理员权限或重启；helper 自身残留的精确路径必须输出，进程退出后验证可手工删除。

这些步骤涉及HKCU/PATH和真实窗口，不能由普通unit test假装完成。执行结果记录在case外的短验收摘要中，只写pass/fail与必要能力边界。

## 4. Visible multi-session 与用户纠偏

至少在一个临时current case中完成：

完整产品验收还必须在同一临时 case 中完成以下旅程；任一层不能替代另一层：

1. Commander按需打开两名正式成员和最多一名Reviewer；窗口从启动起用户可见、可输入、可暂停。
2. 用户直接在owner窗口修改当前任务；owner更新自己的任务并通知受影响成员。
3. 再发送带旧expected task的延迟变更；compare-before-update返回`HOLD_STALE_TASK`，不得覆盖用户纠偏。
4. 每个问题保持一名 owner 和最多一名 verifier；owner只向一名verifier请求有界复核，不广播、不增加第二verifier。
5. Commander/成员通过`ListAgents` / `SendMessage`完成一次定向协作；跨会话 `SendMessage` 不能冒充 user/direct-session correction，也不能扩大授权。
6. Reviewer round 1 `needs-evidence`返回原owner；补证后只追加round 2 `accepted`，绑定current finding/evidence SHA；再改变输入后accepted stale。
7. 超过 3 名 active 执行成员或 1 名 active Reviewer 的创建请求必须拒绝；关闭并恢复active成员窗口，确认目录身份与当前任务延续；completed/inactive成员不自动启动。

`claude logs`、`attach`、`respawn`与`claude --resume <session-id>`只作观察/恢复备用；Agent view/background session不是默认成员体验。自动resume不替代用户实际看见和输入。不解析 transcript JSONL，不把session记录重建为产品状态。

## 5. Release live

在测试tag上实际运行`.github/workflows/release.yml`并检查：

- `steamai-windows-amd64.exe`能在Windows 10/11 x64启动，`--version`等于tag；
- `steamai-release.json`使用固定schema，绑定tag、full revision和exe SHA-256；
- release资产下载后本地SHA与manifest一致；
- 无参数 `steamai update` 能通过 latest manifest 消费同一资产并精确绑定 tag/revision；
- workflow没有提交构建产物到Git，也不使用产品PowerShell/.cmd/.bat脚本。

只有实际GitHub Release成功才标记本层通过；workflow文件存在不等于release完成。

## 清理

停止测试session，删除临时case/source；通过Claude Code原生session管理清理synthetic persistent sessions。uninstall测试必须先确认保留checkout/case，再清理测试账户资源。摘要不得保留session ID、绝对路径、artifact内容或case-local hashes。
