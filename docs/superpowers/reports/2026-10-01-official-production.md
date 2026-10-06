# 2026-10-01 官方 2.9.6 主站发布

- 目标：`sub2api-prod` / `64.83.10.67`；用户明确授权推送、部署及本次不停机迁移例外。官方来源 `ranxi2001/sub2api@a20c0334a6bccc6e7d082bfba9a4f1cb075ef50c`。
- 结果：蓝色实例接流，2.9.6；API、Go worker、独立重登 worker 健康，无回滚。旧绿色实例自然排空 95 秒，未强制终止。PostgreSQL、Redis、Caddy、检测器未重建或重启。测试站未查询、未同步。
- 来源：干净、非 detached、已推送且远端核实一致的根 main `589341be96dc940da9beea0f8aa2234be37b8aa8`；tree `b3d8754f82352f77ec65f05f3e88baf49e39f0eb`。本记录为后续文档提交，不改变运行制品。
- API / Go worker image digest：`sha256:c77052e11433f536df82030da33df15e4dce398e79af7240229cfb35a7947cb0`；独立重登 worker：`sha256:2689933c40e1414b4b79d2b9868e7114f92ac148a056ba615acc0465d265bfab`。均从上述已推送 main 构建，来源标签核实；独立 worker 的本机 Bash 空数组问题通过显式传入 builder 解决，失败发生在构建启动前。
- 迁移：仅新增官方 `262_openai_oauth_reauth_engine.sql`，表预检为 32 行。迁移集合哈希从 `d5339ae8cc23d83fcb14727a248cd0e2e077d21741ed597f81edbec76bfafffe` 到 `6019a46ac500e6a669c8d6b26cc001f3cc96a093f199fece56405df926cb6768`。精确源/目标白名单，事务执行，100ms 锁等待、2s 语句限制；旧 API 和 worker 在迁移期间继续运行。新增字段保留默认值并兼容旧代码，回滚应用时保留新增 schema 和新写入。
- 复用验证：本地合并记录中的前后端构建、187 项前端测试、相关 Go 测试、46 项 Python 测试；12 条在合并前复现的后端历史失败仍未解决，未伪称全量通过。新增验证：真实一次性 PostgreSQL 18 的锁超时事务回滚、旧写入、新值约束和重复执行；本次发布控制器与宿主模拟测试覆盖成功、错误目标、迁移不完整、worker 失败和切流回滚。镜像构建再执行原生构建检查。
- 线上最小验证：公网 health、readyz、认证版本 2.9.6、质量分组规则和 Serverless 查询均通过；基础发布链模型列表验证通过。独立重登 worker 完成不领取任务的 `--check`，支持密码/TOTP 与邮件 OTP，健康状态通过。未合成真实登录/MFA 或上游推理任务。
- 可用性采样：北京时间 17:26:29–17:36:54，两域名共 182 次 `/health` 检查，全部 HTTP 200；这仅表明采样未发现中断，不代表所有用户请求零波动。
- 耗时：发布控制器 17:26:07–17:35:14，约 547 秒；构建、归档、传输及准备约 412 秒；宿主发布 17:32:59 开始，约 135 秒，17:33:36 平滑切流，排空记录 95 秒。独立重登 worker 于 17:35:21 切换并完成配置检查，随后健康及公网检查通过。
- 发布记录：`/var/lib/sub2api/release-records/20261001T093259Z-production-1217088.json`，`downtime_required=false`、`result=succeeded`。本地日志 `/private/tmp/oct01-release.log`、`/private/tmp/oct01-health.log`。
- 回滚入口：使用受保护生产环境调用 `/usr/local/libexec/deploy-sub2api-blue-green-host.sh --rollback --record /var/lib/sub2api/release-records/20261001T093259Z-production-1217088.json`。旧 API `a7884896e2` / `sha256:32e237634e14f7e4449464d2364d51d309479940097dc7bcc11b95877e34ed4b` 和旧 Go worker `fd6dc3601f` / `sha256:6792478460ad90a216510ebdda6bc129ade3eba3f36e1e2de7896a0eb23bcaf0` 均保留；不还原旧数据库覆盖新写入。独立重登 worker 回滚时先 pause 排空，再恢复 `/opt/sub2api/production/reauth-worker/runtime.env.before-oct01-589341be96`，resume 重新绑定健康 Go worker 网络空间。旧独立 worker 镜像和版本化源码保留。
