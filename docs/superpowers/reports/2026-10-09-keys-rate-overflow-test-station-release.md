# API 密钥线路倍率标签：测试站发布

2026-10-09（Asia/Shanghai），目标 `43.133.75.82` / `sub2api-test-station`。结果：API 蓝绿发布及已登录线上布局验收成功，未回滚。主站未访问、未部署。

- 改动：线路徽标允许随按钮收缩，倍率标签禁止收缩，名称空间不足时显示省略号，保留箭头及边缘留白；不改变倍率值或绑定逻辑。
- 来源：`codex/keys-rate-overflow` 的 `7377acc4` 合入根目录 main，已推送并获取 origin/main。部署前分支为 main、工作树干净、commit/tree 完全一致。功能分支已合入，仅作历史证据。
- 发布 commit：`cedbd21b83230705338a3933b472fd8b2753de54`；tree：`e4e6b1b9e431345e858a9323d93a1033e090546d`。
- 镜像 ID：`sha256:b7d4f3da0998a7570b6e78a0511f82da2dfac981717eb451aea145f1b6627e34`；嵌入式二进制 SHA256：`fe942823975297ac471bfa1d8eab24ee261bba1abe78045ed40213c1f55b8ac9`。
- 公网 `KeysView-C8C0FHGA.css` 与本地正式构建逐字节一致，SHA256：`4078c1062a1a2b9f962fbfd38b86c4de6a0f5bbb8622f263faef4d5e0492ba74`。

复用本次本地 63 项相关测试及 9 个布局场景验证证据（190/220/280px 单元格、长名称、桌面及 375px 窄屏）。原有 Vue 属性警告在修改前已存在。正式前端构建与 Linux amd64 嵌入式服务构建成功；无合并冲突或新增业务代码，不重复全量测试。

新增线上验证：使用 Chrome 已有测试站会话打开 `/keys`，确认加载新 CSS；3 条现有密钥的倍率标签均完整位于按钮内，右侧至边框约 36.7px，箭头至边框约 12.8px。375px 窄屏中两条较长名称出现省略号，倍率及箭头仍完整；不创建或改写密钥。公网 `/health` 返回 `ok`、`/readyz` 返回 `ready`。API green 健康，worker 保持原容器及原镜像；首页、detector、PostgreSQL、Redis 未重启。

阶段耗时：前端 Vite 构建 16.80 秒；宿主镜像构建与就绪 21.41 秒；排空并停止旧 API 0.44 秒，剩余连接 0（上限 300 秒）。从成功重试发布命令启动至首次确认 promoted 约 296 秒，包含来源核验、构建、传输和状态检查间隔；后端构建及传输未单独计时。首次尝试在 origin fetch 网络失败时结束，尚未修改服务器，随后通过系统已配置本地代理完成正常来源核验和发布，未跳过门禁。

回滚入口：`sudo python3 /var/tmp/sub2api-test-station-api.oVngJN/deploy.py rollback /opt/sub2api-test-station/releases/cedbd21b83230705338a3933b472fd8b2753de54`。旧 API 镜像、配置及恢复入口保留。本次无数据库迁移、无 worker 更新；未在线执行回滚演练。发布执行器现有 Python 转义及 tar 扩展属性警告未影响结果，不属于本次修复范围。

截图：根目录 `.release/keys-rate-overflow-evidence/desktop.jpg` 与 `mobile.jpg`（忽略目录内，仅用于本次本地复核）。原截图中的 0.3x/1.5x 两条密钥不在当前登录账号列表，已用本地对应模拟倍率验证；线上直接检查当前三条真实密钥。无未解决的本次布局缺陷。
