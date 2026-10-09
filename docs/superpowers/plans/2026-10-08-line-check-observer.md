# 线路检查误判修复与测试站发布

目标：将原生账户测试发送的内容与完成事件通知监控观察器，恢复成功判定、首字计时及首字超时取消。用户已授权修复和测试站部署。

实现：仅在 `AccountTestService.sendEvent` 中恢复观察器通知，放在完成事件抑制判断之后；不放宽异常流判定。覆盖 Chat Completions、Responses、空流及完成事件抑制边界。

- [x] 增加完整 `ProbeAccountConnection` 调用回归测试，先运行并确认成功流被误判。
- [x] 补充事件通知，运行直接相关 unit 测试。
- [x] 发布脚本仅允许本次账户测试文件及其测试进入现有发布链，并要求同步更新单例 worker；验证范围门禁。
- [x] 检查差异，提交功能分支；合入根目录 main 并推送，核对远端 commit/tree 及干净工作树。
- [x] 从合规 main 构建一次，蓝绿发布 API、串行替换相关 worker；公网健康和真实线路检查验证，失败回滚，成功排空旧实例。
- [x] 留存一份发布记录，包含来源、digest、验证、耗时和回滚入口；功能分支标为历史证据。

验收与当前生效版本见 `docs/superpowers/reports/2026-10-08-line-check-observer-test-station-release.md`。原账户和原模型已验证；未单独重跑原 GPT-Pro20x 分组入口。
