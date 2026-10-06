# 测试站 UI/UX 与主站功能整合发布

目标：用户明确指定的测试站 `43.133.75.82:22`（ubuntu），`/opt/sub2api-test-station`，Compose project `sub2api-test-station`。本次未部署主站 `64.83.10.67`，未访问 `49.51.203.200`。

改动：保留测试站品牌样式、AI 工具/线路交互、密钥/用量/充值页面并整合主站业务能力；按确认新增智能运维、配置驱动的智商监测/商城入口、更多筛选、订阅购买；按明确指令移除旧调度日志页面及接口。发布脚本固定目标身份、维护备份/恢复边界，修复 fake Docker 条件语法。

## 来源与制品

- 来源：根目录非 detached、干净 `main`，HEAD 与 fetch 后的 origin/main commit/tree 一致，两边远端 main 均已核对。
- 整合候选：`884d82dda6`，审查修复 `ac3d87a60e`，批准 UI `3098101f51`，部署适配 `9fd70404ec`/`9928138083`，最终夹具修复 `d98cb5279b`。
- 实际部署 source commit：`d98cb5279b6527262708c8d919867dfd7482d0be`。
- 实际部署 source tree：`08b031f100b1c0a63df87c0162bb94c3cb6e7df4`。
- 镜像：`sub2api-test-station-runtime:d98cb5279b6527262708c8d919867dfd7482d0be`，linux/amd64。
- Image ID：`sha256:aa08811fc53983ff221699ae533d1293c66fffbf987fd0661b612b5b19b0328a`；API、worker、detector 运行 image 与此一致。
- 镜像归档 SHA256：`fee00aea8de893b1435491fda79e9aa36feeb810e3f554e713c91be0bad17729`。
- Migration set SHA256：`9e956f0b2cbfd2cdd98075a12d36d54b459f63e5512ce10029dedc74763b68b0`。
- 保留原分支：测试站远端 `backup/pre-main-integration-20261005-96c0818277`，commit `96c08182779b12c5a39ffc0ffdda9ba89ad38bde` / tree `9924ce15d50a43525b0c726b1b7f22663cd10afd`。

## 验证与阶段耗时

复用整合候选的相关前端测试、类型检查、构建、后端测试/编译、30 项真实 SQL 断言、空库迁移/幂等演练与测试站 321 个迁移→候选 44 个迁移的演练；UI 新增部分 53 项直接测试、类型检查/构建及独立审查通过。本地合成数据预览已核对导航和更多筛选；不能称为线上业务验证。

新增：三组发布门禁（宿主、备份、release contract）通过。停机前核实 321 个实际已应用迁移 checksum 全部匹配候选，实际候选 365 个、待应用 44 个。测试站实际 pg_dump 成功恢复到临时独立数据库，验证后清理；业务数据库未因演练修改，未复制生产数据。

2026-10-05 北京时间：
- 构建/首轮 SCP：21:07:20 开始，21:33:53 连接重置、上传失败；此阶段没有停服。镜像已成功构建，没有因网络失败重新构建。
- 复用制品/断点续传：21:43:28 启动 rsync，后续上传/校验在 22:04:25 维护开始前完成（约 21 分钟的传输与准备窗口）；继续执行经过测试的原宿主控制器，不跳过来源/校验/备份/就绪门禁。
- 公网维护：22:04:25 临时 Caddy 维护配置平滑加载，保留健康/就绪路径，阻止业务写入。
- 停服、备份、迁移、启动/内部就绪：控制器实测 51.17 秒；停 API/worker/detector，数据库/Redis/Caddy 保持原实例。旧数据库/Redis/app-data 备份经格式/校验和检查。新服务启动后验证就绪，确认 release-state 后恢复原 Caddy 路由。
- 22:05:16 恢复公网，维护窗口 51 秒。没有执行失败回滚。
- 公网 `/health` 与 `/readyz` 返回预期 JSON。服务器上 dashboard HTML、入口资源 `/assets/index-KHHawftl.js` HTTP 200；入口 JS SHA256 `386faf9912a243b710ac0fa8e435fd4b244f1cdac4263306285b3576eb1835d5`。
- 受保护管理员登录成功；groups/available、groups/available-models、monitor-v4（1h）、monitor-v4/timeline（24h）、keys、payment/checkout-info、admin/groups、admin/settings 均 HTTP 200/code 0，仅读取，无支付/真实上游探测/账号变更。
- 运行态：API/worker/detector/PostgreSQL/Redis healthy、Caddy running；数据库/Redis/Caddy StartedAt 保持原值。schema_migrations=365，current_operational 不存在，退休的 openai_scheduler_logs 不存在。

## 结果与恢复入口

部署成功：release-state `result=succeeded`、`rolled_back=false`，实际 commit/tree/image/归档 hash 与本次制品一致，公网流量已恢复。

- 新 release：`/opt/sub2api-test-station/releases/d98cb5279b6527262708c8d919867dfd7482d0be`。
- 旧 release/镜像保留：`/opt/sub2api-test-station/releases/b61b6431aa20eb49416838f4140e326f5f64f6bd` / `sub2api-test-station-runtime:b61b6431aa20eb49416838f4140e326f5f64f6bd`。
- 停服后的完整恢复点：`/opt/sub2api-test-station/backups/20261005T140428Z`（PostgreSQL、Redis、app-data）。
- 代码恢复：`bash ops/restore-test-station-uiux-main.sh --apply-code`；仅代码恢复，不等于数据库/线上实例回滚。数据库恢复只可在重新停止写入并评估恢复公网后的新数据后执行，不允许盲目还原本次快照丢弃新写入。
- 自动失败恢复路径：经过测试的宿主控制器停止候选，重建单个业务 DB/恢复 dump、恢复 Redis RDB 与精确 app-data named volume，然后启动保留的旧实例；此次未触发。

未验证项：本地浏览器访问公网 dashboard 报 ERR_BLOCKED_BY_CLIENT，未完成部署后的逐页截图/普通用户全流程视觉验收；公共大配置经本地网络读取超时/空响应，在服务器通过原服务入口完整读取正常。未执行真实支付、真实上游调用、全部管理操作或完整回归。以上不等同于功能全部线上验收。主站运行态未查询/未部署，其他测试站未查询/未同步。本记录提交只增加文档，不改变已部署应用制品。
