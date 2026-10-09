# 连续检查误限流：测试站修复发布记录

目标为独立测试站 `43.133.75.82`。沿用本会话修复、每分钟最多 30 次及测试站部署授权。用户反馈第二次点击出现普通“检查过于频繁”提示；只读日志显示第一次 200 后，约 1 秒后的下一次 429。真实服务仍保留 `monitorV4CheckCooldown = 30 * time.Second`，与新 Redis 滑动窗口叠加，是本次根因。上轮路由替身未覆盖服务层冷却，遗漏了实际可点击频率。

## 改动、来源与制品

移除服务层每用户 30 秒固定冷却和 checkNext 状态；频率统一由已部署 Redis 每用户连续 60 秒 30 次控制。同用户不允许重叠检查、每进程最多 4 个活动批次、每批次最多 2 个探测 worker 均保留。完成时删除活动用户记录。新增真实服务连续检查和真实服务加路由限流回归，仅替换上游 IO。

- 分支 `codex/line-check-cooldown-fix` 已快进合入根 main 并推送，保留为历史证据。
- 源码 commit：`99cdf161569f806925d12a5abd2c6a1bc1e92505`；tree：`775dc2ccb0c61f3acf2c6a732e3d71951d79cb1d`。
- 构建前 root main 干净、非 detached，commit/tree 与获取后的 origin/main 完全一致。
- 镜像：`sha256:82ba8081f917a9947780ec0c7dad15e67b792c51500b2b972e80106d8d074326`。
- 二进制 SHA256：`893b887a93df8f5ebfd51dee00c89309d851afeb0fcae91a6b5c3b403d9ca6fc`。
- 本次改变仅影响 API 主动检查 admission，worker 不使用该路径；精确允许三个相关服务文件做 API-only 发布，其他监控 runtime 改动仍要求 worker 同步。没有数据库迁移，无关服务未重启。

## 实际验证

新增测试在修复前准确复现第二次请求 429 / LINE_CHECK_RATE_LIMIT；修复后：

- `go test -race -tags unit ./internal/service -run '^TestMonitorV4Check' -count=1` 通过（2.556 秒）：真实服务连续 30 次无旧冷却、同用户重叠和进程并发边界保留。
- `go test -race -tags unit ./internal/server/routes -run '^TestLineCheckRateLimit' -count=1` 通过（2.955 秒）：包含真实服务前 30 次成功、第 31 次专属 429、拒绝不执行上游，以及已有滑动窗口/多用户/并发/Redis 故障测试。
- 发布脚本 29 项通过，shell 语法、git diff --check 通过；正式前端、类型及 i18n、Go embed 构建成功。
- 扩大到 TestManualCheck 时，两项账号优先级测试失败；在改动前分支同命令复现同样失败（TestManualCheckPrefersGroupPriorityOverGlobalPriority、TestManualCheckRandomizesAccountsAtBestGroupPriority）。属于既有独立问题，本次未改动账号选择逻辑，不声称该扩展套件通过。

线上在同一浏览器操作中第一次完成后立即点击第二次，两次发起间隔约 15.38 秒（小于旧冷却 30 秒），均无限流 alert：

| 完成时间（北京时间） | 状态 | 请求耗时 |
| --- | --- | --- |
| 2026-10-09 01:23:55.771 | 200 | 15062 ms |
| 2026-10-09 01:24:11.146 | 200 | 15062 ms |

此次上游 GPT 探测返回响应超时，生图/grok 仍失败，是接口成功执行后的线路结果，不将它们标记为可用。截图 `.release/line-check-cooldown-evidence/two-checks.png` 显示无频控提示，检查按钮可用。没有注入额度或修改业务数据来验证两次点击。每分钟 30 次边界复用上轮线上验证，并由新增真实服务路由测试确认。

## 发布结果与恢复

API 绿槽切流、公网 health=ok/readyz=ready、最终连接排空成功；`result=succeeded`、`rolled_back=false`，旧蓝 API 排空后停止。最终排空停止 0.57 秒，最大排空 300 秒；服务器构建与就绪 23.68 秒，排空时残留连接 0。本地构建、GitHub 网络重试、页面验收阶段未独立计时。一次未推送时发布被来源门禁拒绝，重试推送成功后按正常链发布，未绕过验证。

恢复材料位于 `/opt/sub2api-test-station/releases/99cdf161569f806925d12a5abd2c6a1bc1e92505` 的 previous-state.json、previous-Caddyfile、compose.yaml；旧蓝 API 和旧镜像 `sha256:93eab5601777f8f9c536e519f01a8f83264daf22d05a08624f26236f3d07902f` 保留。恢复入口为本次上传 staging 中的 deploy.py rollback <release_dir>，staging 路径见本地 `.release/test-station-api-99cdf161569f806925d12a5abd2c6a1bc1e92505/remote-staging-path`。未执行回滚。

未解决项：两项既有账号优先级测试失败；本次观察到的上游超时及既有生图/grok 失败未定位。未访问或部署主站。
