# 线路检查误判修复：测试站发布记录

用户授权：修复并部署到测试站。目标为 `43.133.75.82`（`sub2api-test-station`）；本任务未部署或查询主站。

## 改动与来源

账户测试的 SSE 发送函数未通知监控观察器，成功响应因此被误判为 `malformed_stream`，首字耗时为空。恢复 `AccountTestService.sendEvent` 的观察器通知，并保留完成事件抑制和异常流判定。新增完整 Probe 回归覆盖 Responses、Chat Completions、合法无文本完成、空流和完成抑制边界；修正已有测试中的字面换行。发布门禁精确允许本次四个账户测试文件，要求同步更新相关 worker。

- 已合入并推送的源码 commit：`2d6e427240f2fd65f8f59be96ba5b92a151e3bfc`。
- tree：`7fca3e009d18c83fed3f4a32a17162577babc851`。
- 构建前根目录为干净、非 detached 的 `main`，与获取后的 `origin/main` commit/tree 一致。
- 镜像 ID：`sha256:916b8d738aef5e982ea99a9ae03961dd8bba00af57a00869437b004371d73b35`。
- 二进制 SHA256：`f3b49b3d9ecc6200c540b41a662a7d213f5a36c2098dd046ab262a3ddbf2c1ba`。
- 功能分支 `codex/line-check-observer` 已合入，作为历史证据保留。

## 实际验证

修复前回归测试复现成功流误判，修复后以下验证通过，发布阶段复用同一代码的有效证据：

```sh
go test -tags unit ./internal/service -run '^(TestAccountMonitorProbe|TestBuildAccountMonitorProbeResult|TestManualProbeFirstTokenDeadline|TestMonitorV4Check|TestAccountProbe|TestAccountTestService.*OpenAI|TestProcessOpenAI)' -count=1
python3 -m unittest discover -s tests/operations -p test_test_station_api_release.py
bash -n ops/release-sub2api-test-station-api.sh
git diff --check
```

Go 测试通过（3.152 秒）；发布门禁 23 项通过。未使用 unit tag 的服务测试因既有测试 helper 缺失无法编译，未将该运行称为通过。只读代码审查无阻塞问题。发布构建中的 i18n 测试（3 项）、类型检查、Vite 和 Go embed 构建均成功。

真实页面和数据库探测结果：

| 验收入口 | 账户 / 模型 | 结果 | 首字耗时 | checked_at（UTC） |
| --- | --- | --- | --- | --- |
| 我的 AI 线路检查，GPT-Pro5x（group 6） | 420 / gpt-5.6-terra | success，空 error_code | 1804.34 ms，页面显示 1.80 s | 2026-10-08 12:17:15.791070 |
| 账号监控，刷新原账户状态（无 group_id） | 420 / gpt-5.6-sol | success，空 error_code | 2786.25 ms | 2026-10-08 12:32:09.135399 |
| 修复前原失败记录，GPT-Pro20x（group 2） | 420 / gpt-5.6-sol | failed / malformed_stream | 空 | 2026-10-08 10:59:40.290821 |

原账户和原模型的成功判定、首字计时均恢复。原 GPT-Pro20x 分组的手动检查入口未单独重跑：受保护配置的登录凭据返回 401，后续使用既有已登录浏览器完成上述验收，未修改凭据或用户线路配置。截图保存在本地 `.release/line-check-evidence/account420-success.jpg`。

同次页面检查的生图（account 334 / gpt-5.6）仍返回 `http_error`，grok heavy（account 509 / grok-4.7）仍返回 `account_test_error`。未把这些独立错误判成成功，也未在本次任务中定位其根因。

## 发布结果、耗时与当前状态

本次 API 蓝绿切流、单例 worker 串行替换成功；公网 `/health` 和 `/readyz` 均通过。服务器发布记录 `result=succeeded`、`rolled_back=false`。服务端构建与就绪 20.23 秒，worker 替换 7.34 秒，最终排空检查及停止 0.55 秒，旧实例剩余连接为 0，排空上限 300 秒。客户端从启动发布至取得 promoted 状态约 174 秒（包含本地构建、上传及查询间隔，不能视为纯服务端耗时）；专项验收与文档耗时未独立计时。

收尾时发现后续独立任务已发布 `077d98a63f602325c300fb1ea43a5717a4a117d2`（tree `74349d95d7a066feba33e4e3886fb408ea35de48`，image `sha256:06fa9f22f09966ea6871bbf0479c9dabe318948f923dc01dbc32e7bd5a28d9b7`）。核对确认它是本次修复提交的后代，本次四个服务/测试文件内容完全相同；当前 API 和 worker 使用该镜像且 healthy，公网健康与就绪检查再次通过。上述真实探测发生在本次镜像上，未宣称在后续镜像上重新运行探测。本次 finalize 重复调用因已不处于当前 promoted 状态被门禁拒绝，未改变后续发布；本次和后续部署记录均已 succeeded。

## 回滚入口与未解决项

本次发布记录与配置留在 `/opt/sub2api-test-station/releases/2d6e427240f2fd65f8f59be96ba5b92a151e3bfc`；镜像仍保留。后续发布的旧蓝实例 `sub2api-test-station-test-station-api-blue-1` 已停止，使用本次镜像，作为后续发布的恢复制品保留。当前恢复入口应使用当前发布目录 `/opt/sub2api-test-station/releases/077d98a63f602325c300fb1ea43a5717a4a117d2` 的 `previous-state.json`、Caddy 和 worker 配置，以及对应 `deploy-sub2api-test-station-api.py rollback` 流程；本任务未执行回滚，也未验证后续独立发布的数据库恢复。不要调用本次旧发布的 rollback 去覆盖当前版本，其上一版容器已不存在。

未解决项：原 group 2 分组入口未单独重跑；受保护管理员登录凭据返回 401；生图与 grok heavy 的独立错误尚未定位。修复代码已部署并实测有效，不代表所有线路都可用。
