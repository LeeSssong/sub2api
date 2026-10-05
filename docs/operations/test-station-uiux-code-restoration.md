# 测试站 UI/UX 整合代码回退

本工具只恢复 `git@github.com:LeeSssong/sub2api-test-station.git` 的远端 `main` 代码。执行目录必须是配置了该精确 `test-station` fetch/push URL 的仓库。不会部署、SSH、修改本地工作树、索引或分支；本地 `main` 有未提交改动也不受影响。

默认只读校验和预览：

```bash
bash ops/restore-test-station-uiux-main.sh
```

获准执行代码回退后，一条命令创建并普通推送恢复提交：

```bash
bash ops/restore-test-station-uiux-main.sh --apply-code
```

恢复源固定为 `backup/pre-main-integration-20261005-96c0818277`，commit `96c08182779b12c5a39ffc0ffdda9ba89ad38bde`，tree `9924ce15d50a43525b0c726b1b7f22663cd10afd`。任一不符即退出。临时 bare 仓库内的新提交以获取的远端 main 为唯一 parent，内容精确等于备份 tree，保留后续历史。普通 fast-forward push 前的 pre-push 校验拒绝远端前进、分叉或回退；push 广告后的并发写入由服务器 Git 更新锁/旧 SHA 校验拒绝。失败后重新预览、确认并发变更再重试，不使用 force。

恢复 tree 中可能没有本工具，这是正常现象；执行中的脚本和临时 Git 数据不会依赖恢复后的代码文件。本地脚本仍保留在执行 checkout，直到操作者另行更新本地代码。成功输出恢复 commit/tree；若推送成功后并发更新使即时核对失败，工具明确报告已生成的恢复 commit，须先核查远端，不能把错误退出理解为从未推送。

数据库恢复独立进行。旧代码和旧镜像不能恢复数据，也不能撤销已经应用的迁移。整合涉及 `240_remove_custom_scheduler_artifacts.sql` 时，须先确认历史日志删除范围及数据库备份/恢复演练；不得修改已应用迁移来规避校验。代码恢复不代表数据库兼容或站点可运行。实际部署须另行授权并遵守发布来源、数据库策略与恢复要求。

`--fixture-repo`、`--fixture-backup-commit`、`--fixture-backup-tree` 仅用于本地自动测试，必须同时指定绝对路径的现存 bare 仓库及完整 SHA；不能把网络 URL 或 fixture SHA 注入默认目标。生产用法不传这些参数。工具隔离 Git 环境变量、全局/系统配置，操作临时仓库并使用固定 URL，避免调用者索引、replace refs 或 URL rewrite 改变目标。

验证命令：`bash tests/operations/restore_test_station_uiux_main_test.sh`。真实远端写入、SSH、部署和数据库恢复没有在此验证中执行。
