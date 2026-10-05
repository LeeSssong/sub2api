### Task 2: 代码回退工具

**Files:** Create ops/restore-test-station-uiux-main.sh; tests/operations/restore_test_station_uiux_main_test.sh。

**Interfaces:** 默认只读 dry run；--apply-code 显式执行。在临时 worktree/临时 index 创建以最新目标 main 为 parent、备份 tree 为内容的恢复提交，普通 push fast-forward；拒绝错误仓库、备份 SHA/tree 不匹配和并发远端变更。不执行部署或数据库操作，不把恢复脚本自删除视为失败（脚本保存在候选版本，执行中已读入）。

- [ ] 用临时 bare 仓库测试备份校验、dry run 不写远端、恢复后 tree 一致、并发更新导致安全拒绝。
- [ ] 实现脚本并运行上述测试，文档给出一条命令用法及数据库恢复边界。
- [ ] 提交工具和验证结果。

