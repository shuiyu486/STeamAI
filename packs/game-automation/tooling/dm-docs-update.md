# 大漠资料版本导入与对比

`tooling/scripts/dm_docs.py` 是维护者显式调用的离线资料工具。它复用已有经审节选，支持下一份 CHM 的对比与新目录导入；不是 SDK 安装器、版本推荐器或自动升级服务。**不加载/注册 SDK，不打开帮助浏览器，不执行 HTML，不修改已有资料或 case snapshot。**

## 输入与固定选择

- Windows 10/11 x64、Python 3.9+，只用标准库和系统 `hh.exe -decompile`。无网络、无后台常驻、无新增第三方包。
- `scripts/dm_docs.selection.json` 固定当前 7.2520 `SOURCE.md` 中的选择：SDK 标签、CHM bytes/SHA-256、CHM 内路径、原页 SHA-256、经审 Markdown SHA-256、`review` 状态。测试逐项核对 SOURCE 和实际 bytes，不用固定页数充当验收。
- SDK 标签由调用者提供，只是归档标识，不是厂商版本签名或实际加载 SDK 的版本证明；运行版本另用 `Ver` 核对。
- `compare` 同时要求基线 CHM、下一版 CHM 和基线经审目录。基线必须精确匹配 selection；没有旧 CHM 时不会伪造原文差异。用户自行保留原 CHM，仓库不保存。
- 工具先读取 CHM 的 ITSF v3 / ITSP v1 目录索引，拒绝路径穿越、Windows 路径别名、重复项与异常结构，再交系统 HTML Help 解压。工具不实现 CHM 解压器。

## 1. 对比下一版

以下在仓库根目录执行，PowerShell 仅是维护者手工命令，不是产品入口。将两个占位路径改为用户自己的 CHM。父目录必须已存在，`$work` 必须不存在；不要选择任何 case 内目录。

```powershell
$oldChm = "OLD_CHM_PATH"
$newChm = "NEW_CHM_PATH"
$work = Join-Path $env:TEMP "dm-docs-review-7.2642"
python -B packs/game-automation/tooling/scripts/dm_docs.py compare --baseline-chm "$oldChm" --chm "$newChm" --sdk-label 7.2642 --selection packs/game-automation/tooling/scripts/dm_docs.selection.json --reviewed-dir packs/game-automation/upstream/dm-7.2520 --output "$work"
```

输出 stdout 为 `unchanged / changed / added / missing` 统计，详细内容在新目录：

| 文件 | 用途 |
|---|---|
| `report.json` | 旧/新 SDK 标签、CHM hash/bytes、HTML 页数、逐个选择的状态、原页/节选 hash、生成文件 hash；不写本机绝对路径 |
| `selection.json` | 下一版选择草案；未变项继承已审状态，其它项没有可复用的输出 hash |
| `reviewed/*.md` | **仅原页 bytes 完全未变**的已审节选，逐字复用，不重跑旧行号 |
| `review/*.txt` | 变更/显式新增的普通接口正文，纯文本，标记为待审；去掉脚本、样式、嵌入对象和首个“示例”段及其之后内容 |
| `review/*.diff.txt` | 同一普通接口的旧/新候选纯文本差异；HTML 改变但所选文本未变也明确提示待审 |

`changed` 一律重新审查，不能因 diff 文本相同直接接受：被省略的示例/尾部注意事项或 HTML 结构仍可能改变。`missing` 不搬旧页冒充新版本；先判断更名、移除或输入异常，再显式调整 selection。

绑定模式、许可、错误码及常见问题页属于受限来源；原页未变可复用既有经审节选，改变时**只记录旧/新 hash 和待审状态，不导出完整正文**。需要维护者对本地来源独立复核，不能按旧行号截取。工具不根据文件名搜索并复制未知接口、驱动/保护绕过接口或整个 CHM。

## 2. 需要新增普通接口时

默认只比较 selection 内的页，不宣称检查过整个 SDK 的新增/删除内容。`added` 表示“本次显式新加入选择”，不一定表示厂商本版新加 API。新旧 HTML 总页数仅用于辨认输入，不等于所选资料的变化数。

例如额外选择普通鼠标位置查询：把输出改为另一个不存在的目录，并追加 `--add-interface GetCursorPos`：

```powershell
python -B packs/game-automation/tooling/scripts/dm_docs.py compare --baseline-chm "$oldChm" --chm "$newChm" --sdk-label 7.2642 --selection packs/game-automation/tooling/scripts/dm_docs.selection.json --reviewed-dir packs/game-automation/upstream/dm-7.2520 --add-interface GetCursorPos --output "$work-with-cursor"
```

参数可重复，但同一接口重复、已选择接口、未批准接口均拒绝。允许范围在脚本 `ORDINARY` 中，是有界的文本导出面，不是全部 SDK 清单；`--help` 可列出当前项。接纳新的普通接口只需审查并新增对应白名单项，不必逐版本重写现有选择路径；不得用通配符/全量目录替代白名单。受限来源不能用此参数绕过节选审查。

## 3. 审查与导入

1. `unchanged` 可以复用已有审查；`changed/added` 先核对原文，检查参数/返回值、缺失注意事项、是否混入不应保留的指引或标识。不让工具自动判断“安全”“兼容”或代签审查。
2. 对确认纳入的页，在 review 工作目录的 `reviewed/` 中写经审 UTF-8 LF Markdown，沿用选定文件名。不要把候选文本直接当作已审文档；不要沿用旧页行号，不保留真实目标、许可证、注册码或本机路径。
3. 在该工作目录 `selection.json` 对应项中保留工具计算的 `source_sha256`，填入经审 Markdown 的 SHA-256 并把 `review` 改为 `reviewed`。计算单个节选 hash 的可重复命令：

   ```powershell
   python -B -c "import hashlib,pathlib,sys; print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())" "$work/reviewed/Capture.md"
   ```

4. `missing` 项必须保留待审，或经复核明确从本版选择中移除。不得伪造当前原页 hash。review bundle 的原始 `report.json` 保留为对比证据；它绑定生成时草案，人工审查后的 selection/节选与它不同是预期的，不能倒改 report 把人工修改伪称成工具输出。
5. 所有所选页均为 `reviewed` 后，导入另一个不存在的新目录：

   ```powershell
   $imported = Join-Path $env:TEMP "dm-docs-import-7.2642"
   python -B packs/game-automation/tooling/scripts/dm_docs.py import --chm "$newChm" --selection "$work/selection.json" --reviewed-dir "$work/reviewed" --output "$imported"
   ```

`import` 重新验证 CHM、每个原页和节选 hash；未审、缺页、编码异常、hash 漂移都会拒绝，目标目录不发布。输出为选择的 Markdown、五列表 `SOURCE.md` 和可继续使用的 `selection.json`。这是资料节选集，**不自动生成/搬运维护者 README、C++/MFC/颜色指南，不保证这些手写页的相对链接齐全**；需要正式入 pack 时，维护者另核查路由、指南、引用链接、manifest 与合同测试。工具不修改这些入口，不自动 `git add/commit/push`。

以后比较下一版，使用导入目录的 `selection.json` 和该目录本身作 `--reviewed-dir`，用其绑定的 CHM 作 `--baseline-chm`；仅改变候选 CHM、标签和新输出目录。无需重新填写未变接口的路径或旧行号。

## 写入与解码边界

- 输出只能是父目录已存在的全新目录；拒绝 symlink、junction/reparse、路径穿越、Windows 非法/保留名、网络/设备路径、现有 case 内目录、经审输入目录内路径。Windows 下 sibling staging 完成后用不覆盖目标的 rename 发布；失败清理本次 staging，不删除别人的目标。
- CHM 至多 64 MiB，索引声明的展开总量/实际展开总量也受限，单个读取的 HTML 至多 2 MiB，经审节选至多 16 KiB。原始 CHM 的临时副本和全部提取树只用于本次检查，结束即清理，不保留全量解包。
- HTML charset 必须唯一、明确、受支持；GB2312/GBK/GB18030 使用 GB18030 严格解码，UTF-8 使用 UTF-8 严格解码。不猜编码、不替换坏字节，重复 JSON key/选择项、重复或冲突 CHM 路径均失败关闭。
- 文本提取不是浏览器/CSS 排版引擎，也不是对任意恶意 CHM 的沙箱；只处理显式提供的文档。未知 CHM 格式拒绝，不降级调用浏览器或第三方解压器。
- selection 的 `reviewed` 是维护者的审查声明，SHA 只证明材料一致，不证明人的判断正确、SDK 运行成功或新版适合当前 case。

## 验证

```text
python -B -m unittest discover -s tests/pack_tooling -p test_dm_docs.py -v
```

自动测试不依赖本机 CHM、不启动模型或帮助浏览器。覆盖选择与 SOURCE 一致性、严格解码、纯文本/示例剔除、原页/节选漂移、changed/added/missing 状态、人工审查后导入、下一轮选择复用、路径/Windows junction/可用时 symlink、CHM 目录重复/别名、已存在目标/发布竞争和 CLI 失败退出；OS 解压通道使用明确 fixture 替身，不能替代真实 CHM 实测。

本次真实 Windows 路径：两份用户提供的 CHM 经系统 `hh.exe -decompile` 无窗口提取，7.2520 → 7.2642 的既有选择结果为 **unchanged 46、changed 0、missing 0、added 0**；两次独立 compare 输出逐字相同，import 的 46 页与旧经审节选逐字相同。CHM HTML 页数为 474 → 506，不等于全 SDK 变化审查结论。

7.2642 输入为 `1907278` bytes，SHA-256 `3d9e702417b5fdfb3bb8e772c1747cb6a9024e10ac9e1350d2d4eb747718df49`。显式追加 `GetCursorPos` 得到 added 1、189 bytes 普通候选纯文本；未审 import 返回 2 且未创建目标。旧 upstream 未改变，未新增重复的 7.2642 pack 目录，临时解包与导入结果已清理。该实测只证明资料工作流，不证明 SDK、界面、游戏输入或绑定能力。
