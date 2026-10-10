# 官方 2.10.4 与 main 整合内容主站不停机发布（2026-10-10）

- 目标：主站 64.83.10.67 / api.xingqiaolab.top。用户明确授权官方最新合并及本次不停机发布。测试站未查询、未同步。
- 官方 production：ff9198947a70a83c59b8448f5ef13909119cfe7c（2.10.4），实时 ls-remote 二次确认；上次导入基线 11be589504b482d77827ab383b4b53f777241238（2.10.1）。510 文件增量导入，365 直接、115 自动、30 冲突逐项处理；不把外部历史合入项目。保留本地账务/配额、准入、SLA、监控、用户壳、singleton。生成 Ent/Wire。
- 包含此前 ef4a6d222 main 的测试站 SLA、订单/返利入口及工单关闭后隐藏修复。
- 发布来源：根目录干净、非 detached main 69b74ae33c5401b1283213e6c697569944d04563 / tree 1a2c2c50af72412a29e9a42652499a9be8ac3506，构建前推送并重新 fetch 核实一致。后续文档提交只归档证据，不改变部署内容。
- 应用镜像：ghcr.io/leesssong/xingqiao-sub2api:release-69b74ae33c5401b1283213e6c697569944d04563-6b211a9eeb2c9cae5f39294f01ab72fdd31f90865c0acc766343681ba3681374；Docker image/manifest ID sha256:6b211a9eeb2c9cae5f39294f01ab72fdd31f90865c0acc766343681ba3681374；源码 Git archive，pnpm9按现有lock恢复依赖，Go1.27.2编译，复用已核验依赖未变的150317生产运行镜像。一次前端构建，二进制首次链接元数据大小写错误已在部署前纠正，仅重新链接及更新镜像；最终版本探针返回2.10.4和完整commit。
- 独立reauth镜像 sha256:d281e2cdfc012cf93e8bbb6f6091f9c6f15e5b3ced33a9e7c891dc0609d05d91；复用已核验curl_cffi0.16/jsdom26.1及固定依赖源的旧镜像，更新main源码；TOTP恢复日志挂载持久0700目录，旧env和执行器位于 /opt/sub2api/production/reauth-worker/before-69b74ae33c5401b1283213e6c697569944d04563。
- 数据库：376份既有SQL字节不变。新增239_payment_order_subscription_renewal_mode、269_openai_totp_rotation、270_drop_platform_check_constraints、271_openai_excel_dual_oauth，官方SQL字节保持；本次精确在线pair 10a94ee6d7eb6ecfef05053c99571604230edba42780257eb73cf105d378073a → e5d79ffa00d518804211a877ede6c7cb54277ada8c76e975526dca622445a6de。100ms锁/2s语句门禁，旧API保持服务。受影响schema备份 /root/sub2api-backups/official-20261010/affected-schema.dump，SHA256 2e354ebaf2086f672dbcabc54a51716c648a6cc4628f0f74ea31c43f4a0938d0，pg_restore可读取；无全量业务备份。
- 验证：复用此前SLA和工单55项证据；新增前端相关246项通过、6项既有跳过，vue-tsc和locale完整性通过，生产构建成功；service/handler定向、basispoints/apicompat通过；PostgreSQL18真实升级涵盖锁冲突及时退出/重试、旧收据/列/告警读写、空新任务表及旧reader验证；真实rollback fence阻塞并发写且释放后恢复；本次精确发布模拟覆盖迁移、worker排空、错误目标、验收失败回退；TOTP13/日志6/lifecycle5/supervisor2通过。
- 宽筛选中的既有基线限制：TestAdmissionValidation未实际调用validateAdmission，payment测试夹具未设置必需quota_rule_snapshot（原main亦如此）；旧全量发布测试fixture在历史September26门禁与preloaded模拟失败。未以这些失败声称全量回归通过；本次定向门禁全部通过。
- 审查：Codex主agent及两个只读审查子agent（模型由会话继承，完整模型ID/推理档位工具未提供）分别检查官方业务合并和runtime/回退兼容。修复Gemini混合模型列表遗漏、合并重复定义/签名、测试断言以及切旧路由前的数据兼容竞态。
- 线上：API/worker与独立reauth健康；版本2.10.4；source commit/tree/镜像与制品一致；/health、/readyz与公网资源成功。登录页实际entry /assets/index-DkM6yE9k.js，资源SHA核对。公开support_ticket_enabled=false；发布前端浏览器以隔离用户/API fixtures渲染确认网站工单隐藏，非真实用户登录或真实业务端到端验收。未额外触发模型调用、2FA轮换、支付或创建订单。
- 发布时间：北京时间22:53:04左右平滑切到blue；旧green排空246秒，forced=false，退出码0。宿主经Caddy 267次健康检查全部200，覆盖准备、迁移、切流与排空；公网单次健康/就绪成功，不将宿主探针等同所有公网请求零错误。
- 阶段耗时：前端构建29.63s；Go交叉编译约3min（首次linux/amd64缓存）；部署启动到切流约33s；排空246s；部分阶段未单独计时。上传复用宿主已有相同镜像层，45MB增量组合后完整215486976字节归档SHA与本地相同，不使用服务器内容作为新源码。
- PostgreSQL/Redis/Caddy/model-detector/Codex2API容器ID、启动时间、重启数均保持；Caddy仅平滑reload。登录worker相关更新已按singleton串行排空。
- 回滚：未回滚。保留旧150317镜像及 /var/lib/sub2api/release-records/20261010T145233Z-production-1906259.json，宿主 /usr/local/libexec/deploy-sub2api-blue-green-host.sh --rollback --record 入口。回退保留新增schema/data；活跃TOTP、Excel任务/独立grant、未结算restart续费及新增平台数据阻止旧版回退。Oct10回退在持有SHARE写入fence期间查兼容、切路由、停止候选及恢复旧worker，避免竞态；独立reauth旧配置恢复入口如上。
- 临时工作区归档，功能分支保留为已合入历史证据；根目录main工作区唯一写者。
