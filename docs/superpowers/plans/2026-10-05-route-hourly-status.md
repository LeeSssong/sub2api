# 线路四状态实现计划

目标：近一小时真实计数判定四状态，工具卡片按计数加权汇总，删除旧可用字段。
执行：当前会话单写入者，使用 executing-plans 与 TDD；不提交、推送或部署。

- [x] 运行模型、Dashboard、Monitor API 和密钥线路测试作为基线。
- [x] 先添加90/70阈值、零请求、仅探测、加权汇总、过期快照、详情窗口独立测试；运行并确认失败。
- [x] 实现统一判定，接入卡片、详情和密钥线路；沿用success/warning/danger/muted颜色，修正brand-theme。
- [x] 移除CurrentOperational类型、DTO、SQL、快照读写及判定；新增241删除列migration，保持历史迁移。
- [x] 更新旧字段依赖测试；运行相关Vitest、类型检查/build和后端service/repository/handler测试。
- [x] 本地桌面和窄屏核验并截图，运行界面机械检查；审查diff、残留引用和回退约束。

验证命令：frontend/node_modules/.bin/vitest run（直接相关测试）；vue-tsc -b；vite build；go test ./internal/service ./internal/repository ./internal/handler -run 'MonitorV4|HybridMonitor' -count=1。
发布后续预检：DROP列需要确认旧进程停止，回退旧版本前重建current_operational BOOLEAN NOT NULL DEFAULT FALSE；本轮不执行线上操作。

独立审查补充已完成：密钥选择器响应式过期、详情 1h 失败失效与恢复、统计错误及重试、真实 PostgreSQL 脚本删除旧字段依赖。浏览器发现的新快照早于下一时钟 tick 的问题已通过失败／通过回归测试修复。最终证据见 artifacts/route-hourly-status-validation-2026-10-05.md。

卡片多状态计数已实现并验证：按工具关联线路分别显示正常运行、波动、异常、暂无数据的数量，数量为 0 的状态隐藏；13 个相关测试文件共 131 项通过，桌面模拟页面已核验。
