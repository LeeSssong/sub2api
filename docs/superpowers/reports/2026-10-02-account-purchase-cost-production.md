# 2026-10-02 新 main 与账号购买成本主站发布

- 目标：`sub2api-prod` / `64.83.10.67`。用户明确要求在新 main 上实现、合并并不停机部署；本次包含该 main 的官方更新、OAuth 观测及其数据库迁移。
- 结果：绿色实例接流，API、Go worker 和独立重登 worker 健康；旧蓝色实例自然排空 18 秒，未强制终止，无回滚。PostgreSQL、Redis、Caddy 和检测器未重建。测试站未查询、未同步。
- 来源：根目录干净、非 detached、已推送且远端核实的 main `b7505c1f8744ad97442b058659e078085acbbaad`，tree `4cd94cf4c714bcb44e15148af9b47b967b2f70bf`。基于用户新 main `073ac15961` 整合。本记录是发布后的文档提交，不改变运行制品。
- API / Go worker 制品：`ghcr.io/leesssong/xingqiao-sub2api:release-b7505c1f8744ad97442b058659e078085acbbaad-7a7e6f9d29013bbd6ae3b02377169bcea0b41c12a66a21f635d67dc5906ec793`；preloaded 镜像 ID/digest `sha256:7a7e6f9d29013bbd6ae3b02377169bcea0b41c12a66a21f635d67dc5906ec793`。通过已推送 main 的 Git archive 构建，运行来源标签核实一致。
- 功能：JSON 与 2FA 导入可填写批次人民币购买成本，预先按结构有效账号数均分，创建失败或重复跳过不重分摊；复用 `procurement_cost_cny`，列表用量单元格显示购买成本，空值显示 `—`。
- 迁移：新增 `237_add_api_key_concurrency_limit.sql`、`263_oauth_observations.sql`。集合哈希从 `6019a46ac500e6a669c8d6b26cc001f3cc96a093f199fece56405df926cb6768` 到 `600a3160b811deeb1795a446e3ba2f325bd3532b874274c4228eee5d05e121ea`。本次精确白名单；旧 API/worker 保持运行，migrate-only 使用 100ms 锁等待、2s 语句限制及事务；应用回滚保留新增 schema 和数据。
- 验证：复用新 main 已有交接验证；整合后账号成本前端 74 项通过、前端生产构建通过，Go service/admin/dto 直接相关测试与 embedded server 构建通过。PostgreSQL 18 一次性数据库验证迁移、旧式写入、观测修正/保留/并发统计；控制器和宿主测试覆盖成功、错误迁移目标、部分迁移、worker 失败及回滚。源码门禁新增对官方 API Key lease ownership 的精确兼容，并验证仍拒绝自定义账号准入。
- 线上最小验证：health、readyz、认证版本、运行 commit/tree、lite 列表成本字段和两个导入接口负数校验均通过；公网前端制品包含三个新增界面节点。已在主站真实登录页面打开 JSON 和 2FA 弹窗确认字段，并确认列表展示。未使用真实凭据创建测试账号或执行上游登录。
- 可用性采样：北京时间 03:58:57–04:11:42，两域名共 224 次健康检查，全部 HTTP 200。采样未发现中断，不代表所有用户请求零波动。
- 耗时：从首轮预检至发布结束约 9 分 12 秒（含源码门禁适配重试、构建、归档和传输）；宿主发布 04:07:25–04:08:09，约 44 秒，04:07:50 切流，排空 18 秒。Python HTTP 验证客户端被边缘返回 403，改用发布链同款 curl 后所有检查通过。
- 发布记录：`/var/lib/sub2api/release-records/20261001T200725Z-production-1939196.json`。控制器结果 `downtime_required=false`、`result=succeeded`。本地日志 `/private/tmp/oct02-account-cost-release.log`、`/private/tmp/oct02-account-cost-health.log`；界面截图 `/private/tmp/oct02-json-import.png`、`/private/tmp/oct02-twofa-import.png`。
- 回滚入口：使用受保护生产环境调用 `/usr/local/libexec/deploy-sub2api-blue-green-host.sh --rollback --record /var/lib/sub2api/release-records/20261001T200725Z-production-1939196.json`。旧 API/worker `589341be96` / `sha256:c77052e11433f536df82030da33df15e4dce398e79af7240229cfb35a7947cb0` 已保留；不还原旧数据库覆盖新写入。
- 未解决项：新 main 交接记录及此前测试记录中的无关基线测试失败仍保留，本次没有声称全量测试通过。没有发现本次发布阻塞项。
