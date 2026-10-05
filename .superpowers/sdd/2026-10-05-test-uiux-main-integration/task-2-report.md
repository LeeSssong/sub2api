# Task 2：代码回退工具

状态：工具实现及直接相关本地验证完成；未执行真实代码回退、推送或部署。

新增：
- `ops/restore-test-station-uiux-main.sh`：默认 dry-run；显式 `--apply-code` 使用临时 bare 仓库，校验精确目标 URL、备份 SHA/tree，新建以远端当前 main 为 parent、备份 tree 为内容的提交并普通 fast-forward push。
- `tests/operations/restore_test_station_uiux_main_test.sh`：所有最终测试使用临时本地 bare 仓库。
- `docs/operations/test-station-uiux-code-restoration.md`：一条命令、并发失败处理、代码/部署/数据库恢复边界。

测试证据：
- 测试先于工具，首次执行预期失败 `FAIL: rollback tool is missing`。
- `bash -n ops/restore-test-station-uiux-main.sh tests/operations/restore_test_station_uiux_main_test.sh`：通过。
- `bash tests/operations/restore_test_station_uiux_main_test.sh`：退出 0；PASS 覆盖默认 dry-run 不写远端、备份 commit/tree 不符拒绝、fixture 仅接受本地 bare 路径、默认 fetch/push 身份拒绝、恢复精确 tree/父提交、dirty checkout/未跟踪文件/HEAD 不变、推送广告前远端回退拒绝、广告后并发更新由接收端 CAS 拒绝。
- `git diff --check`：通过。

并发保护使用临时 pre-push hook 校验服务器广告 SHA，与 fetch 时的 main 完全一致；正常 push 的 receive-pack 继续以广告旧 SHA 原子更新。没有 force 或 force-with-lease。测试的接收 hook 只为模拟另一个写入者，并清除隔离区环境后修改已存在的 fixture commit。

限制：真实远端推送权限、远端 hooks、服务端策略、部署、数据库恢复、迁移兼容及真实业务行为未验证。测试开发中曾误从实际 checkout 调用默认 dry-run，允许一次只读 fetch 尝试，约 10 秒后中止；未使用 apply、未推送、未操作 SSH/服务/数据库。该用例已修正为仅在临时 fixture checkout 运行，最终验证全部隔离。本工具不会更新真实 checkout；远端恢复 tree 不含工具不影响已启动脚本。推送后的其他写入者可能再次更新 main，工具会在即时核验失败时报告已推送 commit；不能据此推断推送未发生。

Task1 应用代码及控制器 progress.md 未修改或暂存。本次只提交工具、测试、runbook 和本报告。
