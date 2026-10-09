# 检查线路每分钟限流：测试站发布记录

目标：独立测试站 `43.133.75.82`。沿用用户修复并部署测试站的授权，新增每登录用户任意连续 60 秒最多 30 次检查；一次批量检查计一次。服务端 Redis 原子滑动窗口，跨标签页和实例共享，管理员也适用；超限返回 429、专属错误码、Retry-After 和等待秒数。Redis 故障返回 503。前端展示中文等待提示、临时禁用并自动恢复按钮，保留既有结果；按钮标题显示限额。保留原 Global/Heavy 限流。本次未访问或部署主站，无数据库迁移。

## 来源与制品

- 分支 `codex/line-check-rate-limit` 已快进合入根目录 main 并推送，作为历史证据保留。
- 发布源码 commit：`3e5d1ae934b90902a1dbdf9434c1626f2ad884ea`。
- tree：`e485fdb2661ef0cee4837d05c6654c24ac30a6fa`。
- 构建前根目录 main 干净、非 detached，获取远端后 commit/tree 与 origin/main 一致。
- 镜像：`sha256:93eab5601777f8f9c536e519f01a8f83264daf22d05a08624f26236f3d07902f`。
- 二进制 SHA256：`5e57b19ae056bd1ccd27b885a8cdec8388347ffe753f70b0b833a09d90894122`。
- API 蓝绿发布，worker、探测器、数据库、Redis 未重启。精确扩展 API 发布文件门禁，没有放宽其他后端文件范围。

## 实际验证

旧实现回归复现第 31 次仍返回 200、并发 60 次全部执行；旧页面不显示限额提示。修复后通过：

- `go test -race -tags unit ./internal/server/routes ./internal/server/middleware ./internal/middleware -run 'Test(LineCheckRateLimit|PanelRate|RateLimit|MonitorV4)' -count=1`。覆盖前 30 次/第 31 次、身份校验、用户隔离、跨实例、60 并发、滑动窗口、TTL、Redis 故障。三个包通过，无竞争报告。
- `go test -tags unit ./internal/server/routes -count=1`，完整路由包通过（13.790 秒）。
- Dashboard parity 33 项通过，限流专项先红后绿；vue-tsc 类型检查通过。
- 发布脚本 28 项测试、bash 语法、git diff --check 通过；发布构建的 i18n 3 项、Vite 和 Go embed 构建成功。构建存在既有 pnpm/浏览器数据/包体积提示，未视为错误。

线上页面正常探测返回结果，GPT 首字耗时 2.89 秒。线上限流验收采用当前测试账号的临时 Redis 额度预填至 30，短时保持以便点击验证；未连续发送 30 次真实上游探测。实际 API 日志记录 `/api/v1/monitor-v4/check` 返回 429、处理耗时 2ms。页面显示“每分钟最多检查 30 次，请在 60 秒后重试”，按钮禁用，之前结果保留；等待结束后按钮可用、提示消失。临时额度自动过期，不写业务数据库。30 次放行的精确边界、并发与实例共享由本地真实路由/miniredis 测试验证，未混称线上压力测试。

截图：本地 `.release/line-check-rate-limit-evidence/limited.png`。公网 `/health` 和 `/readyz` 分别返回 ok/ready。

## 结果与恢复

服务器最终 `result=succeeded`、`rolled_back=false`；API 蓝槽就绪，旧绿槽已排空后停止并保留。服务端构建及就绪 22.61 秒，最终排空及停止 0.35 秒，排空上限 300 秒。GitHub 连接中断后重试恢复；本地构建、上传、人工页面验证和文档阶段未独立计时，不将等待间隔视为服务器执行耗时。首次 finalize 使用不存在的 release 内脚本路径被拒绝，后改用本次已上传 remote staging 中的原脚本完成，未绕过门禁。

恢复材料保留在 `/opt/sub2api-test-station/releases/3e5d1ae934b90902a1dbdf9434c1626f2ad884ea` 的 previous-state.json、previous-Caddyfile 和 compose.yaml；旧镜像 `sha256:06fa9f22f09966ea6871bbf0479c9dabe318948f923dc01dbc32e7bd5a28d9b7` 及旧绿 API 实例保留。恢复入口为本次上传的 `deploy.py rollback <release_dir>`（具体 staging 路径见本地 `.release/test-station-api-3e5d1ae934b90902a1dbdf9434c1626f2ad884ea/remote-staging-path`）。未执行回滚。

未验证项：没有在测试站执行真实 30 次上游探测的压力测试，没有人为关闭线上 Redis 做故障验收（本地已覆盖）。生图与 grok heavy 既有线路探测失败仍存在，不属于本次频控修改；不据此声称全部线路可用。
