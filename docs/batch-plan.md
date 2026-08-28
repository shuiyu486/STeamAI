# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志；本文件不是第二份 roadmap。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-product-optimization-v1` 的获批 P0～P3 residual closure，现已完成：`P0P3-C1`～`P0P3-C4`、retired pack identity migration、binary-re专项验收校准与完整总验证均有对应实现和证据。当前没有已批准下一路线，不创建Batch 834、不自选新功能；Git-local v3 receipt、唯一direct commit与post-push ref独立证明machine publication truth。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-product-optimization-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `路线收口 完整 P0～P3 验证` |
| 状态 | `completed` |
| 唯一允许领取 | 无；当前路线已完成，等待显式用户路线变更 |
| 上一批 | 路线收口完整 P0～P3 验证已通过 |
| 下一批 | 无；当前路线已完成，等待显式用户路线变更 |

### Current batch state

### P0P3-C3：DTO/receipt/session/skill

状态：已完成；完成旧task #52，不创建新numbered batch。

结果：`skillcontractgen -check` 已零写入并进入三平台 CI；bundle/init/current-sync 使用同一份 exact validated skill bytes，late pair drift fail-closed；handoff stamped publication 使用 immutable preflight 与 atomic no-replace exact replay，旧 stamp 不能覆盖历史 identity。DTO/public boundary 与 session owner 只做有界证据审计，未发现可复现缺陷；没有新增 schema、session 状态机或 provider。

验证结果：skillcontract、runtimebundle、sync、fs、workstream、releasecheck、CLI handoff/release-check focused tests及受影响完整 package tests通过；其结论已纳入完成的完整 P0～P3总验证。

### binary-re 专项验收校准

状态：已完成；C1～C4与 retired migration均已完成，不创建新numbered batch。

结果：established status只投影同pack executable owner、production contract与typed verified catalog完全一致的exact IDs；VMP能力收窄到已有IDA TSV bounded inspection；repository inventory、synthetic input、real contained child/Claude与未观察的producer/target-tool receipt已typed分层。

验证结果：两条独立review发现并关闭跨pack归属与catalog结构缺口；focused、受影响完整package、文档与release门禁fresh通过。本结果只解锁总验证，不代表路线完成。

### 完整 P0～P3 验证

状态：completed；当前没有已批准下一路线。

目标：逐项复核全部未取消的原P0～P3要求，运行产品/focused/full gates，并核对真实用户路径；fresh machine receipt、direct commit与本地tracking ref由completed projection后的machine publication独立证明。

验证结果：原未取消项、focused/affected/full tests、vet、module verify、skill contract、public release/status/packs/doctor、diff/façade、fresh installed project-local real-Claude binary-re/web-security产品路径及最终定向反证均通过；未把remote workflow定义、synthetic fixture或cross-compile冒充更强证据。

### Batch 833：binary-re actual analysis

状态：已完成 ordinary `binary-re` actual adapter lifecycle、直接相关 fresh validation 与 implementation commit/push；它是 latest numbered implementation identity，不代表随后全部 P0～P3 residual work完成。

目标：以 `Batch 833` 作为 latest numbered receipt identity；P0P3 residual milestones由 active route 独立表达，不冒充 Batch 834。

验证结果：focused fresh lifecycle tests实际启动 embedded authorized parent/fixed child，并覆盖strict gate、independent evidence review、accepted-only binding、terminal recovery与stale control fail-closed；随后validation repair的最终`release-run -Format json`以7/7通过，direct commit `870d908`与tracking ref/post-push receipt已闭合。这些证据只证明相应实现和validation，不关闭后续 residual work。

### Locked sequence

| 工作 | 解锁条件 |
|---|---|
| `P0P3-C1` capability control sinks | 已完成 |
| `P0P3-C2` orchestration non-authorization | 已完成并解锁C3 |
| `P0P3-C3` DTO/receipt/session/skill | 已完成并解锁C4 |
| `P0P3-C4` runtime ownership | 已完成并解锁retired migration |
| retired pack identity migration | 已完成并解锁binary-re专项验收校准 |
| binary-re 专项验收校准 | 已完成并解锁总验证 |
| 完整P0～P3验证 | 已完成 |
| 路线总体完成 | 已完成；等待显式用户路线变更 |

## 验证标准

- 本文件与路线图的 route/current/state/claim/next 必须一致；冲突时 fail-closed。
- active plan只保留一个 current residual milestone和一个latest numbered batch handoff；更早批次只在`docs/batch-history.md`。
- route completion不能由Git-local validation receipt替代；machine validation也不能由Markdown claim替代。
- `Batch 833`不自动解锁下一 numbered batch；当前路线不创建Batch 834。
- 不声称未读取的remote CI green，不用cross-compile代替非Windows runtime E2E。

## 风险与注意事项

- 不再把局部batch完成、提交推送、测试全绿或stop hook收尾误判为长期获批方案完成。
- 不全局替换兼容`rekit` identity，不新增PowerShell runtime logic，不引入installer或PATH fallback。
- authority/confirmed、heavy action、sync/promote和schema migration继续遵守exact review/gate边界。
