# 固定上游方法材料来源（REA）

本目录保存从 REA 官方发布中按需选取的方法参考材料，供 binary-re 专业负责人按任务读取；它是阅读材料，不是执行许可，也不代表本机已安装 REA。

## 上游身份

- 上游项目：`morluto/rea`（MIT License，见本目录 `LICENSE`）。
- 固定版本：npm 正式发布 `rea-agents@6.7.0`，registry integrity `sha512-yhIx2ioJYYT1C/tONfYeitC6LhnKmCh+ZwRUOgYuhSc733lGhkkt73zhug9WmLgxoMdKuqIYGCoBfD4fXkOhFw==`，发布日期 2026-10-18。
- Git commit：`27dc42d10249f0410904a75c0997842eb03b931c`。
- 商业分析器（IDA/Hopper/Ghidra 等）的许可不随材料提供，须使用者自行合法持有。

## 文件清单与拼接校验

超过 16 KiB 阅读预算的上游文件按换行边界切分为两个部分文件；按表中顺序逐字节拼接后，长度与 SHA-256 必须等于上游原文。`internal/steamai/vnextcontract` 对此做机械校验。

| 本目录文件（按拼接顺序） | 上游路径 | 拼接 bytes | 拼接 SHA-256 |
|---|---|---|---|
| `skill-1of2.md` + `skill-2of2.md` | `.agents/skills/reverse-engineer-anything/SKILL.md` | 23510 | `976e400f028618926c0b658ee50df79996f5f32402d33b49a23bfdbc232b45e6` |
| `native-and-artifacts-1of2.md` + `native-and-artifacts-2of2.md` | `.agents/skills/reverse-engineer-anything/references/native-and-artifacts.md` | 18911 | `18b8828aec6476be03ca92bdb9506c47cc9ef8c1415c37d33e4af2f8c55cf8f9` |
| `evidence-workflows.md` | `.agents/skills/reverse-engineer-anything/references/evidence-workflows.md` | 4363 | `2bdee98a843d018cf850c706111bb74c459ba8640da00799e9208122e4d5ab80` |
| `LICENSE` | `LICENSE` | 1064 | `798f9e4d18e49b30bf2f12696c8ced73dc4e1413da375a333d01281543a1ea8b` |

## 纪律

- 材料内容为上游英文原文，未改写；描述的是 6.7.0 的工具合同与限制。
- 本机实际安装的 REA、Node、分析 provider 与目标绑定不随本 pack 固定；负责人使用前另行核对，并把运行身份记录到对应 evidence。
- 本目录不进入 `learningTargets`；经验回流只修改主包既有合格 Markdown，不更新上游材料、REA 版本或工具配置。
- 不自动追 `latest`，不自动安装或升级 REA；上游新版本进入本仓库必须重新审阅、替换并更新本表。
- 材料中的任何工具说明都不构成 heavy action 或网络/执行许可；具体动作仍须满足 case 授权、针对该动作的用户确认与 Claude Code 工具权限。
