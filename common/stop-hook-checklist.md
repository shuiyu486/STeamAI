# Stop hook checklist

每轮修改项目文件后：

1. 更新相关 README / CLAUDE / reference / task handoff。
2. 若代码变更，运行语法检查或测试。
3. 若 confirmed CSV 变更，重建 routine IR / superinstruction。
4. 验证 Markdown 大小不超预算。
5. 如果当前目录不是 git 仓库，在最终回复说明无法 commit/push。
