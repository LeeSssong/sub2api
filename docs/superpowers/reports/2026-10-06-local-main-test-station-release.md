# 本地 main 整合与测试站发布

目标：用户授权合入所有非 main 分支，不推送远端 main，直接部署独立测试站；目标经用户确认使用当前 SSH alias `sub2api-test-station`，实际为 `ubuntu@43.133.75.82:22`，Compose project `sub2api-test-station`。未操作主站或 `49.51.203.200`。

## 来源与改动

- 本地分支 `codex/route-degradation-rate`、`codex/best-route-score`、`codex/intelligence-brand`、`codex/route-status-box` 均已成为 main 祖先；保留原分支/worktree 为历史证据，未删除或清理已有内容。
- 整合降智率按有评级巡检轮次统计、最佳线路加权评分、品牌样式及既有线路状态展示；Dashboard 冲突保留线路健康分布和加权最佳线路。
- 部署来源为根目录非 detached、干净的本地 main：commit `42788429bd86f5a701aa35c9e143cd62dbd926cd`，tree `23c089c8399a865352bbd43d8c67a33b0e983c14`。
- 发布脚本增加显式固定本地 main 的测试站入口 `TEST_STATION_LOCAL_MAIN_COMMIT`，普通发布仍保留 origin/main 一致门禁。本次用户明确不推送，未执行任何 push；发布后实时查询远端 main 仍为 `aed5de647dfae7b7410f657a0a1a42c1c2ab1162`。
- 本记录后续提交只增加发布文档，不改变已部署应用制品。

## 制品

标准 Dockerfile 构建遇 GHCR/Docker Hub/Alpine 网络 EOF。最终在本地编译 server（embed、linux/amd64、Go 元数据指向上述干净 commit），将本地 main 的 server、v4.1.1 detector adapter、token-guard、入口脚本、resources 及构建来源文件上传到宿主构建；复用已核验上一版 d98 镜像的运行依赖，未将服务器代码作为新代码来源。

- 依赖底座：`sub2api-test-station-runtime:d98cb5279b6527262708c8d919867dfd7482d0be`，ID `sha256:aa08811fc53983ff221699ae533d1293c66fffbf987fd0661b612b5b19b0328a`；其 token-guard、adapter、入口脚本与候选对应源码无差异，curl_cffi 为 0.16.3。
- 本地构建输入包 SHA256：`d124cd94d71969e6f98ce20cb0dc1d19b2b0ca4ad4be940d03b99c0a62d3471c`。
- 最终镜像：`sub2api-test-station-runtime:42788429bd86f5a701aa35c9e143cd62dbd926cd`，linux/amd64。
- Image ID：`sha256:27a0d9546954b46e0f4ebbbb9ce0924bebe77b1b82f282967217775b1b560232`。
- 镜像归档 SHA256：`7811994b2cee0057c7086c2842fbe61157c87e78170ba0a8aa38f33faa9d359c`。
- Migration set SHA256：`4d3fad380f0535a6f7d39a78efb68317501141a833c32bdea94f0070f513e651`。
- 镜像内 server/adapter/入口脚本分别与本地输入 checksum 一致；独立 detector 健康探针返回 `status=ok, version=4.1.1`。
- 最终制品和构建输入保留于宿主 `/var/tmp/xingqiao-20261006-release.lLo0r0`。

## 验证与阶段耗时

复用本轮整合阶段通过的直接验证：前端相关 Vitest 66 项、locale 3 项、vue-tsc、生产构建、Go MonitorV4/质量时间线测试、独立 PostgreSQL 质量统计集成测试，以及测试站发布/备份/迁移门禁契约测试。合并后 `git diff --check` 通过。

北京时间 2026-10-06：

- 首次发布约 20:43 开始停服，临时制品误将 server 副本用于 detector；候选 API/worker 未启动，数据库仍为 365 个迁移，264 未执行。SSH 中途断开后只读重连核实，执行器失败于 compose_start，自动恢复记录 `rolled_back=false`。约 20:52 手动恢复保留的 d98 Compose，health/readyz 正常；首次中断约 9 分钟，未留精确停服/恢复秒级时间。失败目录归档为 `<commit>.failed-20261006T125118Z`，原失败记录未覆盖。
- 重试前根据正式 Dockerfile 修正运行 detector 为 Python adapter，并完整核验所有源码输入及运行依赖。临时 Caddy 单行块语法在维护前预检失败，修正多行格式后验证通过；该预检失败未停服。
- 最终维护开始 20:59:31，正式路由恢复 21:03:10，维护窗口 219 秒。先阻止公网业务写入，再停 API/worker/detector，备份后启动单套候选并迁移；PostgreSQL/Redis/Caddy 保持原实例。
- 发布状态在 21:02:22 写入：`result=succeeded`、`rolled_back=false`，commit/tree/image/hash 与最终制品一致。
- 新增迁移 `264_quality_rule_tested_group.sql` 已应用一次；schema_migrations 从 365 增至 366，`quality_rule_template_accounts.tested_group_id` 列存在。
- 公网 `http://43.133.75.82/health` 与 `/readyz` 返回正确 JSON；`/login` 使用新入口资源，公网入口 JS SHA256 与本地一致。
- 使用当前受保护凭据登录成功；Monitor V4 snapshot（24h）返回 7 个分组，timeline（24h/hour）返回 168 个点，graded_round_count/suspected_degraded_round_count 存在且计数边界有效。未触发真实探测。
- 两个 Dashboard chunk 和 IntelligenceTestView chunk 共 3 个功能资源均 HTTP 可读取且 SHA256 与本地构建一致。
- API/worker/detector/PostgreSQL/Redis healthy，Caddy running。发布 env 与根目录 env 的初始化管理员凭据无法登录（401），验收使用今日受保护的密码轮换凭据成功；未修改账号/密码，也未输出秘密。

## 结果与恢复入口

测试站最终发布成功，正式公网流量已恢复；远端 main 未推送。旧版与两次停写备份均保留：

- 新 release：`/opt/sub2api-test-station/releases/42788429bd86f5a701aa35c9e143cd62dbd926cd`。
- 旧 release/镜像：`/opt/sub2api-test-station/releases/d98cb5279b6527262708c8d919867dfd7482d0be` / `sub2api-test-station-runtime:d98cb5279b6527262708c8d919867dfd7482d0be`。
- 最终迁移前完整恢复点：`/opt/sub2api-test-station/backups/20261006T125936Z`，含 PostgreSQL dump、Redis RDB、app-data；备份 helper 已校验格式和 SHA256。首次恢复点为 `20261006T124339Z`。
- 回滚需先重新停止业务写入并评估恢复公网后新增数据，再使用上述旧 release 和兼容恢复方案；不能把切旧镜像当成数据库回滚，也不能盲目还原快照丢弃新数据。

未解决/未验证项：首次自动恢复未成功的具体原因尚未定位，实际旧服务恢复由人工执行 Compose 并验证；不能据契约测试宣称本次自动数据库恢复成功。未做逐页浏览器视觉验收、真实支付或真实上游调用。主站及其他测试站未访问、未同步。
