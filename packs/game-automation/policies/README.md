# game-automation policy overlays

本 pack 的领域规则已并入 `common/policies/*` 与 `references/game-automation/*.md`：开发写入边界（分配的项目源码路径）、测试回执作为 evidence、code review 绑定源码版本或 exact diff。当前没有额外的 overlay 文件；出现 game-automation 特有且 common 不覆盖的执行规则时，才在此目录新增最小 overlay，不复制 common 全文。
