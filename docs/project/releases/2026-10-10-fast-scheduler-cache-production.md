# Fast 调度缓存修复主站发布（2026-10-10）

- 目标：主站 `64.83.10.67` / `api.xingqiaolab.top`。用户授权合并、归档删除工作区、推送 main 和不停机部署；测试站未查询、未同步。
- 改动：保留调度轻量缓存的 `openai_fast_supported`、`openai_fast_models`，保留显式 null 模型范围的拒绝语义。修复提交 `9565d0efa`，由 `codex/fix-fast-scheduler-cache` 合入；工作区已由 Codex 原生工具归档删除，临时分支已删除。
- 发布来源：根目录干净、非 detached `main`，推送并 fetch 后本地/远端 commit `dcbf1d5c98be20dffa47c2a39c09d8dc040d2cc6`、tree `aed3d59947b785c1d51b5866b293818d5d2b06e6` 完全一致。本记录为发布后文档提交，不改变已部署代码。
- 制品：`ghcr.io/leesssong/xingqiao-sub2api:release-dcbf1d5c98be20dffa47c2a39c09d8dc040d2cc6-5d2a5481c348a4528792e0daae66ee0a3f85ae6705c313a5e9a3dea0657d6274`；image/manifest digest `sha256:5d2a5481c348a4528792e0daae66ee0a3f85ae6705c313a5e9a3dea0657d6274`。源码来自已推送 main 的 Git archive；依赖与上一生产版本没有差异，复用已核验 runtime，重新编译 API 二进制及嵌入资源，制品只构建一次。
- 复用验证：缓存序列化到真实网关选号回归及相关缓存字段测试、fast 路由/策略定向测试通过，diff 检查及只读审查通过。覆盖显式/分组强制 fast、批量/单个选号、模型映射、null/空/无效范围及缓存更新。四项旧缓存 stale-write/delete-fencing 测试失败已在原 main 复现，未扩展本次修复；未声称全量回归通过。
- 部署：无数据库或依赖变化。沿用已审查宿主蓝绿脚本；旧 blue 持续服务，新 green 就绪后平滑切流。调度快照由 singleton worker 写入，因此同一制品串行更新 worker 并排空原有任务，启动时重建缓存，避免旧 writer 再次丢字段。脚本联动暂停/恢复独立 reauth worker：容器重新创建，镜像保持 `sha256:d281e2cdfc012cf93e8bbb6f6091f9c6f15e5b3ced33a9e7c891dc0609d05d91`，恢复后健康。PostgreSQL、Redis、Caddy、model-detector 的容器 ID、镜像、启动时间和重启数均未改变，Caddy 仅平滑 reload。
- 新增线上验证：API/worker 来源标签、实际镜像与发布状态匹配且健康；公网 `/health`、`/readyz` 成功。旧 API 停止后 `sched:meta:420` 的 fast 标记仍为 true。一次公网 `gpt-6.1-sol` / `service_tier=priority` 小请求 HTTP 200、流正常完成、输出 `OK`，用时 2.28 秒；Request ID `2e03fd91-6033-44f0-81ea-9f6a9f14c5ec`。使用记录确认选中账号 420 / 分组 25。
- 时间：北京时间约 23:14:14 切到 green；前端资源生成 21 秒，后端编译 5 秒，宿主候选准备至切流约 27 秒，旧 blue 排空 260 秒提前结束，`forced=false`，停止后保留镜像与回滚制品。镜像装配/压缩上传未独立计时。
- 结果：宿主最终发布记录 `result=succeeded`、`rolled_back=false`、`drain.status=completed`。本地 SSH 复用连接在远端进程结束后仍挂起；核对无发布进程/锁后关闭该客户端连接，本地入口因未收到 stdout 最终退出 1（`host executor output is invalid`）。发布成功依据是宿主完整最终记录及实际容器、路由、请求验证，不能把本地入口的异常退出称作正常成功。
- 回滚入口：宿主 `/usr/local/libexec/deploy-sub2api-blue-green-host.sh --rollback --record /var/lib/sub2api/release-records/20261010T151347Z-production-1947186.json`；保留旧 `69b74ae33` 镜像、配置与状态。未回滚。
- 未解决/验证限制：上游响应及 usage 的 service tier 为 `auto/default`，因此已证明本站 fast 选号误拒绝修复，但不能证明上游实际提供 priority 服务等级。Python urllib 公网探针先收到无本站 Request ID 的 403；改用发布链 curl 后成功，未改 Cloudflare 配置。SSH 回传挂起尚未修复，本次仅清理已结束发布的客户端连接。
- 受保护构建/诊断证据：本机 `~/.config/sub2api/release-evidence/fast-cache-20261010/`；宿主最终记录见上述回滚入口。
