# 全部本地分支整合与主站蓝绿发布计划

目标：用户授权本地非 main 改动整合、推送 origin/main、不停机部署 64.83.10.67。

约束：保留原工作区；等价补丁只连接历史；无共同基线或整条外部历史保留归档；不搬运测试站数据；不新增数据库迁移；仅根目录干净 main 且与 origin/main commit/tree 一致可发布；旧连接最多排空 300 秒，回滚制品保留。

- [x] 盘点全部本地分支、现存 worktree 与未提交内容，核对主站当前健康和来源。
- [x] 合入删除质量快照后的测试站功能；git cherry 全部为负号的四条历史分支用 ours 合并连接。
- [x] 固定并复制 fast-capability-routing 工作区补丁和五个新文件，原工作区不修改。
- [x] 验证 Fast 选号、模型范围、WS、续接、计费及账号表单，验证线路限流/统计/图标；相关前端、后端、PostgreSQL 和首页测试；必要独立代码审查。
- [x] 保存分支例外及来源清单，核对候选迁移目录与主站相同。
- [ ] 根目录 main 快进整合并推送，重新 fetch 核对 commit/tree/清洁状态。
- [ ] 从合规 main 构建一次，复用已核验 runtime/依赖；新 API 就绪，必要 worker 交接，平滑切流，线上专项和排空。
- [ ] 保留一份发布结果记录、实际制品 digest、验证、耗时、回滚入口与未验证项。

整合清单：93/100 条非 main 分支原已合入；测试站删除快照分支新增正常合并；四条全部补丁等价的历史分支只连接祖先；快速路由未提交内容固定 patch SHA256 dfe7bade00919e87b1e6c18812257ff96b3a5bf8abc074025381ec9142309c4a 加五个新文件整合。normal-session-proxy 包含 7886 条外部历史、quality-ops-upstream-pr 无共同基线，按 AGENTS.md 保留，不机械并入。原快速路由 worktree 不写入，两个 node_modules 未跟踪链接保留原样。

验证：前端 284 项、首页相关 24 项、类型检查、相关 Go tests、隔离 PostgreSQL 线路缓存/SLA 与用量兼容、发布 Python 流程测试通过；独立审查未发现 critical/important。较宽的 Go repository 测试中四条 upstream billing mock 参数失败在未修改 main 复现；首页全量旧教程/升级弹窗 8 项失败涉及文件与 main 相同，不扩展本次范围。
