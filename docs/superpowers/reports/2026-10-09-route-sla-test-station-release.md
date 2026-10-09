# 线路详情成功率 SLA：测试站发布记录

目标：独立测试站 `43.133.75.82`，用户明确授权“部署到测试站”。每条线路按对应 group_id 使用后台原始 SLA；详情卡片累计与曲线共用 timeline 计数。缓存、首字 P50、疑似降智率和其他旧监控口径不变，无数据库迁移。

- 功能提交 `99ff42ca`；合入当前 origin/main 后，发布范围适配提交 `15b0c3a5`。分支 `codex/route-sla` 已合入根 main 并推送，保留为历史证据。
- 部署源码 commit `15b0c3a57b6467c8d7cc42e123d00c324e326db4`，tree `e959059f21fd7f73fbd60f73fbbccfb04a6530fa`。根 main 干净、非 detached；发布脚本成功 fetch 并核实 commit/tree 与 origin/main 一致。外层一次 fetch 因 SSL 连接失败，脚本自身重试成功，未豁免来源门禁。
- 镜像 digest `sha256:aa64d00f1fbf6e0fe2bb6eb91d2d2c3d86416448e3bffb662750f95d7feb5065`；二进制 SHA256 `171fe8ae0efc3e0c5847f9c070d32d1eb2f6d1d2aa2b9edfbb2ec5d1a1cc575e`。
- 复用本地 PostgreSQL 逐组逐桶比较、后端 repository/service/handler、前端66项及类型检查证据。合并后涉及共用 monitor service 编译，定向 timeline/service 测试再验通过。新增发布范围测试后30项发布脚本测试、shell语法和 diff check 通过。正式 root main 前端 build（含3项i18n）、Go embed linux/amd64 构建通过。
- API-only 蓝绿：新蓝 API 就绪、平滑切流、公网 health=ok / readyz=ready、页面验收、旧绿 API 排空后停止。无关 worker、探测器、数据库、Redis未重建。服务器构建及就绪21.13秒，最终排空停止0.57秒，残留连接0，允许上限300秒；前端构建17.45秒，其他本地阶段未独立计时。

实际页面验收：Codex详情近24小时小时/天计数相同；近7天小时168桶/天7桶计数相同；近1小时12个5分钟桶零样本显示暂无数据；切换粒度时显示—。现有登录浏览器会话正常。只读数据库以快照同一时间窗、后台SLA条件复算，与页面四组累计完全一致：

| group_id | 24h 成功/总数 | 7d 成功/总数 |
| --- | --- | --- |
| 25 | 0/0 | 1623/1642 |
| 6 | 6440/6467 | 137371/138270 |
| 2 | 19603/19788 | 117974/118320 |
| 19 | 8/8 | 28/67 |

证据位于本地 `.release/route-sla-deploy.log`、`.release/route-sla-db-verification.json`、`.release/route-sla-test-station.jpg`。未注入业务数据、未发起上游请求。受保护 env 中登记的管理员登录凭据返回401（容器ADMIN和release登记值均尝试）；故未声称独立管理员API探针成功，使用现有会话页面与只读数据库核验，未修改凭据。

结果：`succeeded`，`rolled_back=false`。恢复目录 `/opt/sub2api-test-station/releases/15b0c3a57b6467c8d7cc42e123d00c324e326db4` 保留 previous-state.json、previous-Caddyfile 和旧镜像；恢复命令 `sudo python3 /var/tmp/sub2api-test-station-api.t0hy9K/deploy.py rollback /opt/sub2api-test-station/releases/15b0c3a57b6467c8d7cc42e123d00c324e326db4`。未执行回滚。未访问或部署主站。
