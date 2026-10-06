# 2026-10-07 测试站充值与兑换布局发布

- 目标：独立测试站 `43.133.75.82`，入口 `/redeem`；主站未查询、未发布。
- 改动：最近活动移入左侧兑换列，与兑换卡片同宽；云猫店铺独占右列。保留既有桌面 2:3 比例、24px 间距和窄屏单列，内层网格限制最小轨道宽度以防内容撑宽。
- 合入分支：`codex/redeem-activity-column`（已合入，作为历史证据保留）；提交 `e17f7950`、`e022b155`。
- 发布时根目录 `main` 与已获取的 `origin/main` commit/tree 完全一致，非 detached HEAD、工作树干净；原有两份规则文档修改暂存保留，发布结束后恢复。
- 发布源 main commit：`e022b1557657cb8b60aa7e2aaf3606ad0c05e216`。
- 发布源 tree：`bec43419f1da3e6faf3556f73343187aa6707503`。
- 镜像 ID/digest：`sha256:f917c6e49b5c71613e51a5d47d8eb4b25a9c0bad7f0876c28cb64fb99009cf9c`；内嵌二进制 SHA-256：`acbb4f67ee30d323fcce9b80ff786171dbeed560c80dd70af08bf1216df81f26`。
- 复用验证：兑换页及充值导航 20 项测试通过，独立测试站发布控制器 19 项测试通过；无兑换交易或支付操作。
- 新增验证：最终构建含 3 项 i18n 完整性检查、Vue/TypeScript 检查及 Vite 构建成功；公网 `/health` 返回 ok、`/readyz` 返回 ready。
- 线上布局：1512px 桌面，兑换与最近活动同为 469.59px，店铺位于右侧；390px 窄屏，兑换、最近活动和店铺均为 278px，单列且无横向溢出。截图只保存在本地忽略目录，未提交账户内容。
- 阶段耗时：最终 Vite 构建 16.10 秒；宿主镜像构建及就绪 19.32 秒；旧实例排空 33.68 秒，剩余连接为 0；其余阶段未单独计时。
- 结果：API 蓝绿发布成功，未回滚；worker、探测器、数据库及 Redis 未重建。窄屏检查发现的约 6px 内层网格撑宽已在最终提交中修正。
- 回滚入口：当前发布包 `deploy.py rollback /opt/sub2api-test-station/releases/e022b1557657cb8b60aa7e2aaf3606ad0c05e216`，发布包路径记录在本地 `.release/test-station-api-e022b1557657cb8b60aa7e2aaf3606ad0c05e216/remote-staging-path`；使用该包恢复 previous-state.json 和 previous-Caddyfile。旧 release `/opt/sub2api-test-station/releases/e17f79501f20cd0c2f32c4c224daa7ec31020836` 和旧镜像保留。
- 未解决问题：无本次布局问题；真实充值、支付和兑换交易未验证。
