# 线路缓存命中率：测试站发布记录

目标：独立测试站 `43.133.75.82`；用户授权“部署到测试站”。线路详情缓存曲线使用后台同线路＋流式的原始用量日志，不额外筛选完整状态、不做请求去重，保留线路时间窗口。其他指标与推荐快照口径不变，无数据库迁移。

- 功能提交 `346aca9f`，精确发布范围适配提交 `d8b24ab9`。`codex/route-cache-stream` 已合入根 main 并推送，分支保留为已合并历史证据。
- 部署 main commit `d8b24ab9a0f895dbfc4f7b8bb372e14d0792e8b2`，tree `3363f5fa7172adb68efeada39d9fbe4a96c12bb8`。构建前根 main 干净、非 detached，fetch 后 commit/tree 与 origin/main 一致。
- 镜像 digest `sha256:453a788a88cb6dee37ae31b03c87d9597dad4415be0e8c14dc2e63d4142b3112`，二进制 SHA256 `8e7c5575dec5f716fc2ef6246521824b9695bda9b5ce184b58cd084c5c495847`。
- 复用本地 PostgreSQL 五种窗口／粒度逐组逐桶对后台真实趋势查询对账、SLA 回归、repository 与 service 定向测试证据。新增精确 API 发布路径测试先失败再通过；API 发布脚本测试31项、独立测试站发布/主机执行器契约、shell语法与 diff check通过。正式前端 build（包含3项i18n检查）及 Go embed linux/amd64 构建通过。
- API-only 蓝绿：新绿 API 就绪，Caddy平滑切流，公网 `/health` 为 `ok`、`/readyz` 为 `ready`。运行 commit/tree、镜像和二进制身份与发布制品一致。worker、探测器、PostgreSQL、Redis未更新或重启。
- 使用现有登录浏览器会话刷新 Codex 线路详情，近24小时小时曲线正常。GPT-Pro5x（group_id=6）与 GPT-Pro20x（group_id=2）的26个有数据小时桶，与同一时间范围的只读数据库流式日志 Token 汇总一致（按页面1位小数）；例：10/08 06:00 GPT-Pro5x=76.4%，10/08 10:00 GPT-Pro20x=85%。仅读取展示数据及聚合统计，未写入业务数据、未发起上游请求、未重新尝试已失效的env登录凭据。
- 阶段耗时：前端构建19.08秒，服务器构建与就绪20.18秒，旧蓝 API 连接排空及停止0.37秒、残留连接0，最大排空上限300秒；其他阶段未独立计时。
- 结果：`succeeded`，`rolled_back=false`。发布日志：`.release/route-cache-stream-deploy.log`。本次未访问或发布主站。
- 回滚入口：`sudo python3 /var/tmp/sub2api-test-station-api.iTPL6a/deploy.py rollback /opt/sub2api-test-station/releases/d8b24ab9a0f895dbfc4f7b8bb372e14d0792e8b2`。旧版本镜像、previous-state.json及previous-Caddyfile均保留；未执行回滚。
- 未解决问题：无本次功能阻塞。线上只验证近24小时小时曲线；其他窗口／粒度复用本地真实 PostgreSQL验证证据。
