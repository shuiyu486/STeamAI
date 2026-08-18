# Unicorn dispatch trace

## 目标

基于真实 VMEnter context 离线执行 VM dispatcher，输出 dispatch events、native instruction trace、VMExit 片段和 handler 输入输出。

## 输入

- rebuilt/dumped PE
- VMEnter context JSON
- augmented memory ranges
- VMEnter RVA/VA
- dispatch limit / focus handler filter

## 输出

- trace CSV
- trace summary
- focused handler occurrence summary
- VMExit traces
- routine IR events/summary 的输入数据

## 规则

- 长 trace 用于 coverage 和 top unknown。
- 自然 VMExit trace 用于识别 epilogue、bridge、native continuation。
- focused trace 用下一 dispatch event 作为 exit state。
- 不把完整 trace、完整 disasm、大 CSV 粘进 Markdown。

## 常见失败处理

- 缺 TEB/PEB/GS 或 DLL 页：先补 memory ranges。
- 无法自然 VMExit：保留长 trace 和 focused trace，不强行等待。
- handler alias-heavy：进入 value-flow + manual review。
