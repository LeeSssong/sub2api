# 2026-10-09 智商监测与官方 2.10.1 主站发布

- 目标：主站 `64.83.10.67`。用户明确授权本次官方更新及不停机部署；未停主站 API。测试站未查询、未同步。
- 改动：逻辑题公共区域下紧凑展示 `gpt-6-astra`、`gpt-6.1-sol`；复用质量运维最近完整时段的随机账号结果，固定快照、保留原判定和回复，不重复调用、判题或计费。绘图仍每半小时一次，画作位于绘图区域右侧。包含旧绘图计划冲突保护。
- 官方来源：`ranxi2001/sub2api` production `11be589504b482d77827ab383b4b53f777241238`（2.10.1），基线 `bc83ff9c367883e5b7d0140e6bb42e2e7cc5239c`（2.9.7）。保留本地定制，逐文件来源见同日期 official-source 交接；没有把整条外部 Git 历史并入项目。
- 发布来源：根目录干净、非 detached 的 main `cc40b46df6713a11a0f417cda78868cfc77a20fe`，tree `7345514bda61507b1fe97342ce603ee33135ea0a`；构建前已推送并重新 fetch，commit/tree 与 origin/main 一致。本记录的后续文档提交不改变应用内容。
- 制品：`ghcr.io/leesssong/xingqiao-sub2api:release-cc40b46df6713a11a0f417cda78868cfc77a20fe-89b5d14b6f787243551560d63ce045e985d10e80a856568e74f39bf9ec68d119`，manifest/image digest `sha256:89b5d14b6f787243551560d63ce045e985d10e80a856568e74f39bf9ec68d119`。构建输入来自该 main 的 Git archive；缓存 Node/Go 平台 digest 已对照源码固定官方 index 核验，流式上下文按既有忽略意图排除开发机 node_modules 链接。
- 数据库：仅增加官方 9 个迁移，旧 SQL 字节/checksum 未改；迁移集从 `94e7d3f18b82168089015e41e69b3f4d9f2f6b3d4fe9f493c5eb1b67dd87e44d` 到 `10a94ee6d7eb6ecfef05053c99571604230edba42780257eb73cf105d378073a`。本次精确在线门禁、100ms 锁等待和 2s 语句上限已验证；旧通知队列不存在 threshold 任务的前置条件通过。受影响配置/告警元数据备份位于 `/root/sub2api-backups/intelligence-quality-official-20261009/affected-metadata.dump`，校验和 `0b8cbf04369ec81a6be99aa6696175e9135e3d29a6e36c991e371224c8fa54a0`，已验证可被 pg_restore 读取；未做普通发布的全量数据库备份。
- 本地验证：复用后端 7 组真实 PostgreSQL 智商监测集成测试及相关单元证据；最终整合后的 service、handler、admin、routes 相关检查通过。精确 pnpm 9 frozen install、相关前端用例及包含 locale/类型检查的生产构建通过。官方升级真实 PostgreSQL 验证涵盖旧 reader、旧校验和、旧告警读写、锁冲突退出与重试。发布模拟覆盖精确版本对、retain、通知安全门禁、错误目标拒绝、回退和连接排空。最终审查发现的 Astra 多实例调度写入问题已修复并定向复审通过。
- 发布结果：北京时间 14:21:07 平滑切到 green，新 API、worker 健康。旧 blue 连接排空后收到正常结束信号，14:23:39 退出码 0；记录排空 149 秒、`forced=false`，未强杀长连接。Caddy、PostgreSQL、Redis、探测器和 Codex2API 容器 ID、启动时间及重启计数保持原状。
- 线上专项：14:30 正常周期两个分组各生成两模型糖果快照和一条成功绘图（共四条糖果、两条绘图）。糖果原结果关联、模型及回复匹配，新增费用均为 0；其中一条原始异常如实保留。两条绘图于 14:32:46／14:32:47 完成，计划租约清空、下一轮为 15:00。未为验收额外触发模型调用。管理员版本接口为 2.10.1，新智商页面与资源返回 200，紧凑逻辑区和独立绘图区资源已核对。
- 连续检查：14:15:38—14:35:34，服务器本地经 Caddy 的 393 次健康检查全部 200，覆盖迁移、切流与排空。本机公网探针有连接超时，切流前也存在，不能据此声称所有公网路径零错误；后续公网页面和健康检查返回 200。
- 阶段耗时：成功构建内前端 90.4s、后端编译 32.8s、镜像导出 4.0s；宿主发布开始至切流约 45s；排空 149s。上传未独立计时。此前镜像仓库直连及上下文链接问题均在切流前退出，解决后复用缓存，没有重复推广不同制品。
- 回滚：未回滚。保留旧 API/worker 镜像及 `/var/lib/sub2api/release-records/20261009T062022Z-production-3476787.json`；使用宿主发布执行器的 `--rollback --record` 入口。兼容回退保留新增 schema，不以旧镜像切换冒充数据库数据还原；存在新阈值待发任务时旧 worker 回退会被兼容门禁拦截。
- 限制：自动登录返回 400，登录态页面的线上交互未验；布局已做本地桌面/手机浏览器验证，数据已做线上数据库专项核对。首次历史回填仅复制仍留存的源结果且有 5 秒预算，不补造缺失历史。
- 归档：`codex/intelligence-legacy-dedup`、`codex/intelligence-quality-ui`、`codex/official-october09`、`codex/intelligence-official-release` 均已合入 main，仅保留为历史证据，不重复合并发布。发布前既有 CPA 回滚文档按原内容恢复。
