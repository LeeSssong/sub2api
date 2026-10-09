# 原生分组扣费标准：测试站发布

- 目标：独立测试站 `43.133.75.82`，SSH `sub2api-test-station`。用户明确授权“部署到测试站”。主站未访问、未同步。
- 改动：扣费表读取原生分组、当前工具授权分组的模型目录和用户倍率；倍率与扣费公式使用同一个值。打开、手动刷新和可见页面每 60 秒更新；隐藏暂停、关闭取消，失败显示重试。模型目录复用普通 `/v1/models` 的原生候选、白名单、禁用模型和固定账号目录规则。无数据库、依赖或账务变更。
- 来源：`codex/native-group-pricing` 对齐最新主线后快进合入并推送根目录 main。发布源 commit `a7e7415928c8debecb72b06128b5e8d764d19f2c`，tree `98aabf5ab199c91f54d21ce420e085512cce2574`；构建前后 main 非 detached、工作树干净，commit/tree 与获取后的 origin/main 一致。本记录单独归档，不改变运行制品；功能分支和工作区保留为已合入的历史证据，不再作为发布来源。
- 制品：内嵌 Linux amd64 Go binary SHA256 `7da06abf21c387b55b982be53fe78a9c1944013bd053d9db78b2555de5c87761`；镜像 ID `sha256:180fc4bdf56718fb4043ee320cfce1279edd9c2f327a38f4e286f73b1dd22036`。发布目录 `/opt/sub2api-test-station/releases/a7e7415928c8debecb72b06128b5e8d764d19f2c`。运行容器内 binary、镜像 ID 与部署 commit 均与发布清单一致。
- 路径：API 蓝绿 `test-station-api-blue` → `test-station-api-green`。发布门禁仅补充本次八个后端文件的精确范围；依赖、迁移和未审查后端路径仍拒绝，monitor 改动仍要求 worker 更新。本次不更新 worker；worker `a98548b4174b`、探测器 `595856eabbd1`、DB `3cb0cd97a12d`、Redis `17fab82c3e5e`、Caddy `d339db76e851` 保持原实例，Caddy 仅平滑 reload。
- 验证复用：63 项相关前端测试、typecheck、build、九个文件 ESLint、Go 原生分组/模型端点与 service unit 定向测试、Go build、桌面/手机真实组件本地 mock 交互。新增发布门禁测试先失败后通过，发布控制测试共 18 项通过；对齐主线后补跑受影响首页 31 项测试通过，shell 语法和 diff 检查通过。发布构建同时通过 3 项 locale 检查及 Vue 类型构建。
- 线上已验证：公网 `/health` 为 ok、`/readyz` 为 ready；`/dashboard` 使用本次入口 JS；公网入口包和含新价格表的 DashboardView 功能包 SHA256 均与本地构建文件一致。API 运行容器 `2cf7ee16ae82` 健康，宿主发布状态 succeeded。
- 未验证：登录后的原生三接口、同组 `/v1/models` 列表对照和实际价格表交互。受保护 `/opt/sub2api-test-station/.env` 实际使用 `ADMIN_LAB_ADMIN_EMAIL/PASSWORD`，Compose 映射为运行态 `ADMIN_EMAIL/PASSWORD`；与文档中的 `ACCEPTANCE_ADMIN_*` 不同。按实际字段读取后，新旧 API 用相同管理员凭据均返回 `401 INVALID_CREDENTIALS`，属于既有凭据问题。已请求用户更新受保护凭据；未改密码、绕过认证或创建临时 key，未将本地 mock 当作线上验收。既有公告和七天统计问题不属于本次验证范围。
- 阶段耗时：前端构建 15.69 秒；宿主镜像准备和新 API 就绪 22.46 秒；finalize 连接排空 0.34 秒，残留 0，上限从切流时间起 300 秒。编译、传输及认证诊断没有独立计时，不推算。
- 结果：部署成功，功能认证验收待补，未回滚。旧 API 已排空停止，旧镜像、Compose、路由和 env 保留。根目录原有两份环境规则修改在发布阶段暂存，归档后原样恢复，stash 保留为恢复证据。
- 回滚入口：`sudo -n python3 /var/tmp/sub2api-test-station-api.k6X4SC/deploy.py rollback /opt/sub2api-test-station/releases/a7e7415928c8debecb72b06128b5e8d764d19f2c`。旧 API `bfda2271c71c`，旧版本 `612b67cd93286563737fb72bc5a4d6c4481f2b30`；无数据库迁移，回滚不覆盖业务数据库。
- 证据：`.release/native-group-pricing-release.log`、本次 staging 的 `manifest.json`、`.release/native-group-pricing-public-assets.json` 和宿主 `deployment.json`。凭据、token 与 env 全文未进入记录或仓库。
