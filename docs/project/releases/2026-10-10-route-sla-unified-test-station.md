# 请求成功率统一 SLA：测试站发布记录

用户授权：全部请求成功率按后台 SLA，排除业务限制及客户端取消；先部署测试站。业务类别 Optimization；类型 D。未部署主站。

## 来源与结果

- 实现分支 `codex/route-sla-unified@5d6a078a` 已合入并推送根 main；发布来源 commit `74cfb5415fc0a0ce227107d8816518fac57e3650` / tree `8a93f2acbcc110f4e19e5e10e1fc5c7b93615291`，发布前干净 main 与实时 origin/main 一致。
- 测试站 `http://43.133.75.82`，API-only 蓝绿，活动槽 blue；无迁移、worker 更新或上游测试请求。
- 镜像 ID `sha256:b92e64669a9d6938413231f6d25f9a6ac26373d27a68210f5240d1ba822fc7fb`；二进制 SHA256 `09e972a8d411c2e44b4ec0d4ee43b729b772843d665c723c4df4e0919d3727bd`。
- 宿主发布结果 `succeeded`，rolled_back=false。前端构建 15.61 秒；宿主 build_and_ready 22.7 秒；排空 1.79 秒，剩余连接 0，300 秒上限。

## 改动与验证

snapshot 增加独立 SLA 成功/总计数，复用后台 timeline SQL、授权分组及精确统计窗口。AI 工具健康、聚合、排序、最佳线路、我的 AI 线路、密钥下拉/换线路、混合性能卡片采用 SLA；详情状态、百分比和走势使用同一所选范围。旧统计字段保留兼容，缺失 SLA 不回退旧分母；零样本显示暂无数据。

前端直接相关 115 项及密钥页 62 项回归通过；vue-tsc、MonitorV4 handler 测试、独立 SLA service 测试、发布门禁 33 项、bash -n、diff-check 通过。前端和嵌入 Go server 发布构建成功。660/667 回归样例得到 98.95% / 正常运行。

线上 /health、/readyz 成功；release-state 与来源 commit/tree 一致；资源 `/assets/index-DHGutKsa.js`。浏览器核验我的 AI 线路 SLA 提示、Codex 详情 1h/24h/7d、创建线路密钥下拉及密钥页换线路入口。近一小时 0/0 显示暂无数据；七天历史 GPT-Pro5x 为 91200/91788=99.36%、GPT-Pro20x 为 106959/107292=99.69%，均正常运行，提示包含完整排除规则。未创建/修改密钥、未发上游检查请求。

截图：本机 `.release/route-sla-unified-live.jpg`。原始发布日志与 deployment.json 位于本机 `.release/`，宿主正式证据 `/opt/sub2api-test-station/releases/74cfb5415fc0a0ce227107d8816518fac57e3650/deployment.json`。

## 限制与恢复

完整 Go service 包测试受基线缺失 `rawChatCompletionsTestAccount` 阻断，已编译全部生产 service 文件并运行独立 SLA 测试。近一小时无真实样本，未在线复现用户的 660/667 数据；该样例通过自动化回归。兼容性能页当前路由到旧 V2，显示暂无可见分组，混合性能卡片仅组件/页面回归验证，未声称该线上页面完整验收通过。主站未发布。

旧 green 槽及制品保留。恢复入口：本次受保护 staging 中的 `deploy.py rollback /opt/sub2api-test-station/releases/74cfb5415fc0a0ce227107d8816518fac57e3650`（经 sudo 执行）；宿主 staging 路径由本机 `.release/test-station-api-74cfb5415fc0a0ce227107d8816518fac57e3650/remote-staging-path` 定位。无数据库回滚。

后续提交仅归档文档；运行二进制/source 身份仍为上述发布 commit，不因文档提交再次构建部署。已合入分支保留作历史证据，不重复合并。
