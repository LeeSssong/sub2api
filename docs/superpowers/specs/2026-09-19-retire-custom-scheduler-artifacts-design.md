# 自定义调度遗留制品删除设计

日期：2026-09-19。状态：已批准实施；候选完成前不得合并、推送或部署。

## 1. 目标

永久删除已经退出运行控制的 OpenAI 自定义调度配置、持久化日志及管理页面，避免管理员继续看到或依赖不再生效的数据。普通 OpenAI 文本请求继续使用当前 Sub 原生调度链路，不改变账号资格、会话粘性、订阅优先、Top-K、原生权重、并发槽、利润终检、重放安全、用量或计费。

## 2. 删除边界

数据库迁移删除以下 `settings.key` 行：

- `openai_advanced_scheduler_candidate_pool_mode`
- `openai_advanced_scheduler_exploration_ratio`
- `openai_advanced_scheduler_starvation_threshold_seconds`
- `openai_advanced_scheduler_fairness_weight`
- `openai_advanced_scheduler_group_overrides`
- `openai_advanced_scheduler_group_policies`
- `openai_advanced_scheduler_custom_presets`

数据库迁移同时执行 `DROP TABLE IF EXISTS openai_scheduler_logs`。迁移必须幂等，不修改既有迁移文件，不触碰 usage、错误日志、账号监控、审计、余额或计费表。

代码删除自定义日志模型、repository、sink、清理循环、依赖注入、启动生命周期、列表/详情 handler 和 `/api/v1/admin/scheduler/logs` 路由。删除运营面板 `/api/v1/admin/ops/openai-scheduler-experience` 及其只服务于退休卡片的聚合模型。

前端删除 `/admin/scheduler-logs` 路由、侧栏入口、页面、API client、文案和相关测试；删除运营面板调度体验卡片。设置 API 和设置页不再返回、接收或渲染上述七项字段。

## 3. 明确保留

保留以下原生有效设置及其 API/UI：

- `openai_advanced_scheduler_enabled`
- `openai_advanced_scheduler_sticky_weighted_enabled`
- `openai_advanced_scheduler_subscription_priority_enabled`
- `openai_advanced_scheduler_lb_top_k`
- `openai_advanced_scheduler_weight_*`

删除退休公平性字段后，运行时不得再读取它们或生成兼容默认值。图片、WebSocket、Grok 和其他平台既有路径不得因本任务改变。

## 4. 接口与兼容

旧调度日志和调度体验接口从路由表中移除，访问结果为标准 `404`，不保留退休占位响应。设置 GET 响应不再包含七个退休字段；设置 PUT 中出现未知旧字段时按现有 JSON 绑定兼容策略处理，但服务端不得存储或重新创建对应 settings 行。

历史日志与旧配置不可逆删除。回滚应用版本时必须同时恢复迁移前数据库备份，不能只切回旧镜像。

## 5. 验证

- 迁移契约测试证明七个 key 被精确删除、日志表使用 `IF EXISTS` 删除，并且原生设置 key 不在删除集合。
- 后端路由测试证明两个旧接口族不再注册，设置响应不含退休字段，原生调度设置仍可读写。
- 编译和定向测试证明 repository、sink、handler 依赖已全部移除，server 可构建。
- 前端测试证明路由、侧栏、运营卡片和旧设置控件不存在；typecheck 和生产构建通过。
- `git diff --check` 和代码搜索证明没有运行时引用残留。

## 6. 发布边界

本任务只形成独立分支候选。由于包含数据删除和结构迁移，未来发布必须采用项目 SOP 的停机数据库发布：停写、备份、迁移、启动单套新版本、验证后恢复流量。不得按静态文件直接更新或普通蓝绿无停机发布。
