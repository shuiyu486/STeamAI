# Windows native product roadmap

## 路线

- ID：`steamai-windows-native-product-v1`
- 平台：Windows 10/11 x64
- 状态：`completed`
- 产品目标：从 GitHub Release 获取原生 `steamai.exe`，在任意明确授权的项目目录用一个 `steamai` 命令创建或继续 self-contained case；Commander 自动打开可见成员窗口；本机经验经两级 Reviewer 与用户 exact confirmation 以主题 batch 回流 canonical working tree。

## 不变量

- 普通 `claude` 项目完全不加载 STeamAI；不使用全局 plugin。
- 正式路径不使用 PowerShell、`.cmd` 或 `.bat`。
- native shell 只做 setup/update/uninstall、卸载后的窄自清理、Fresh/exact learning文件机械操作、Windows可见启动和瞬时单 Commander 互斥；不管理 task/session/message/roster/finding/review/learning decision。
- Fresh/current only；冲突 fail-closed；无 importer、迁移、dual-read/write 或兼容 runtime。
- 一个项目目录对应一个授权 case；不同 case 不扫描、不汇总，只通过审查、脱敏、用户确认的 pack experience共享。
- canonical working-tree current bytes 是 Fresh authority；HEAD 是 provenance anchor；case snapshot 建立后不漂移。
- learning apply 不 stage/commit/push；Git 外部操作需用户另行明确授权。

## 工作包

### WNP-01 — Native setup and launcher

状态：`implemented-and-automated-verified`

- `steamai setup` 绑定可见 ordinary Git checkout，并为当前用户安装 exe/PATH。
- `steamai` 自动在 case cwd 启动 `/steamai`；Fresh 只追加 canonical source 为 `--add-dir`。
- `CREATE_NEW_CONSOLE` 打开普通可见成员窗口；成员 cwd 为专属目录，并 `--add-dir <CASE_ROOT>`。
- Windows named mutex + volume/file identity 阻止同一物理 case 的第二 Commander。

### WNP-02 — Reliable Fresh

状态：`implemented-and-automated-verified`

- production `casebootstrap` 生成 zero-write exact preview 与 exact confirmation。
- stage-0 current tracked closure + working-tree bytes；拒绝 unmerged/intent-to-add/untracked closure/reparse/path escape。
- sibling staging、project-local skill no-replace first、current marker last、完整 current validation。

### WNP-03 — Thematic learning batch

状态：`implemented-and-automated-verified`

- candidate immutable、单 destination、逐条 eligibility；active roster Reviewer绑定。
- batch 可多 candidate、多 existing Markdown target；独立 exact batch review。
- preview 显示 source finding/review chain、Reviewer、pre/postimage 和完整 patch。
- Apply 使用已确认 patch bytes，重验 HEAD/index/case snapshot/targets，失败只恢复 batch targets。

### WNP-04 — Update, Release and live acceptance

状态：`completed`

已实现并通过独立代码终审：

- 显式 `steamai update`：从 canonical checkout 外运行，从 latest release manifest 取得 exact version/revision，执行 clean checkout、local refs/ignored 路径 gate、exe SHA/`--version` 兼容检查、tag/revision/canonical validation 与 source/exe 切换；从 source 根或子目录调用会在联网前拒绝；source 替换后保留并输出旧 sibling checkout 路径，不自动递归删除；不 merge/rebase/stash/reset/clean。
- `steamai uninstall`：删除安装入口、setup拥有的PATH和定位信息，保留 checkout/case；Windows锁定中的当前exe先重命名，再由同字节临时原生 helper 在父进程退出后删除；helper 仅保留在安装目录作为普通用户无脚本自清理的已知最小残留，命令输出其精确路径供进程退出后手工删除。

完成证据：

- GitHub Release `v1.0.3` workflow 成功，下载后的 `steamai-windows-amd64.exe`、`steamai-release.json` 与 `SHA256SUMS` 完整核验通过；anonymous latest URL 可用，manifest 绑定 exact tag revision 和 exe SHA-256。
- Windows 真实 setup/PATH、Fresh zero-write/exact apply、Fresh drift、三成员 visible windows、物理同 case 的 duplicate Commander、3 candidates/3 reviews/2 targets learning batch、保守 uninstall 与 helper residual 旅程通过。
- 真实 `v1.0.1` → `v1.0.2` source+exe update 通过，旧 checkout 保留；从 canonical source 内调用在联网前明确拒绝。
- Claude Code native context/file access、persistent session/direct correction、`HOLD_STALE_TASK` 与 Commander/member `ListAgents`/`SendMessage` 定向跨会话协作通过。
- automated/focused/full suite、vet、Windows/Linux build、diff check 与独立 correctness/security 终审通过。

## 完成标准

只有以下各层按自身范围通过，路线才能标为 completed：

1. automated contract/unit tests；
2. real Windows native filesystem/registry/process product-path tests；
3. actual Claude Code visible multi-session manual acceptance；
4. GitHub Release job实际生成并校验资产；
5. README、skill、vnext、router 与当前路线无矛盾。

fake platform、synthetic fixture、workflow definition、cross-compile 或零匹配 test 不能代替上述其它层。无法在当前环境执行的门必须保持 pending，不得以文档宣称完成。

## 后续边界

完成本路线后没有自动推进批次。只有真实使用反馈、明确新目标或授权边界变化时立项；不得为“可能需要”增加 GUI/TUI、daemon、Hub、经验数据库、session迁移器或通用控制面。
