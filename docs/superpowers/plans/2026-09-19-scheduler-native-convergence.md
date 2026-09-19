# 调度恢复原生与现有改造收敛实施方案

> **For agentic workers:** REQUIRED SUB-SKILL: 使用 executing-plans 按任务在同一隔离 worktree 顺序实施。本方案不要求子代理或并行写入。步骤以复选框追踪；本地候选验收完成，未推送、合并或部署。具体证据、既有失败及范围调整以本任务 evidence 为准。

**Goal:** 恢复 v0.2.7 原生普通 OpenAI 文本调度，退出自定义质量聚合与高成本完整观测，修复保留模块的正确性和数据生命周期问题。

**Architecture:** 按功能比较固定官方源码并移植恢复，不整文件覆盖。保留本站业务门、协议兼容和安全计费保护；运行链路、管理展示和告警同时收敛。

**Tech Stack:** Go、Gin、Redis、PostgreSQL、Vue/TypeScript、Vitest。

**Spec:** `docs/superpowers/specs/2026-09-19-scheduler-native-convergence-design.md`。

## Global Constraints

- 用户已于 2026-09-19 授权实施；允许本地开发、测试及任务提交，不推送、不发布。
- 固定官方版本 v0.2.7 / aea725f2ea644d5592d0bbb1d63b607efa7e200a；方案起点 aeebf865f453e3dca131119e3c67b7dfb0498153。
- 不引入对原生的新优化，不整文件覆盖，不新增评分、指标或路由控制面。
- 不改变余额、计费事实、售价、账号组关系，不删除表/设置，不新增 schema migration。
- 保留权限、能力、利润既有语义及安全重放、用量完整性、取消释放保护。
- 使用新鲜 origin/main 的独立 codex 分支和 worktree；同一 worktree 一个写入者。
- 中间提交不单独上线；任务完成并通过直接测试才是候选，不标记生产 DONE。
- 文档路径均相对该 worktree；下文 B=upstream/sub2api/backend，F=upstream/sub2api/frontend。

## 执行顺序

T0 原生差异契约 → T1 退出统一评分与刷新 → T2 恢复准入/重试 → T3 退出完整事件与日志写入 → T4 收敛消费者和兼容 → T5 定向验收 → 独立发布授权。

T1—T4 组成一个最终候选；任一未完成不得将其他部分当作完成方案部署。

### T0：锁定恢复边界与现状证据

**文件：** 新增 `docs/superpowers/reports/2026-09-19-scheduler-convergence-evidence.md`（实施时创建）。读取 XINGQIAO_UPSTREAM.md、当前 AGENTS.md 及下列直接代码，不读总账/历史队列。

- [x] 前一窗口已从当时最新 origin/main 建执行 worktree；续接核对原分支、HEAD、dirty files 与 worktree，未重新创建或重置。
- [x] 使用固定官方对象逐个比对 scheduler、gateway handler、chat completions、concurrency helper、配置默认值；原生对象缺失时获取该精确 commit。
- [x] 记录普通 Responses、Chat、Messages、Embeddings、WS、Images 是否进入统一评分/共享预算/事件记录，不以函数名字推断调用关系。
- [x] 在 evidence 中填入每个差异块：原生行为、本站目的、恢复/保留决定、调用点、验证用例。重点列出平台转发、模型映射、利润、取消、usage、OAuth 429、安全重放。
- [x] 记录原生开关有效路径和默认尝试次数；恢复原生调度不等于强制关闭整个高级 scheduler。

读取基线示例：
```sh
git show aea725f2ea644d5592d0bbb1d63b607efa7e200a:backend/internal/service/openai_account_scheduler.go
git show aea725f2ea644d5592d0bbb1d63b607efa7e200a:backend/internal/handler/openai_gateway_handler.go
```

**产出契约：** 固定版本的调度/等待/重试行为表，供 T1/T2 使用。若导入版本已变更，仅重新比对受影响部分。

### T1：恢复原生选号，完全退出请求驱动质量聚合

**修改：** B/internal/service/openai_account_scheduler.go、openai_gateway_service.go、openai_gateway_usage.go、openai_account_scheduler_projection.go；B/internal/handler/openai_gateway_handler.go、openai_chat_completions.go；其余 opt-in 调用按 T0 清单处理。

**退役候选：** B/internal/service/openai_unified_quality_scheduler.go、openai_account_quality.go、openai_quality_score.go。仅在全部生产调用和展示消费者退出后删除文件；不能为让旧测试继续通过保留运行入口。

**测试：** 新增 B/internal/service/openai_native_convergence_test.go，更新 openai_unified_quality_scheduler_test.go 和 openai_account_quality_test.go 中已不适用的契约。

- [x] 建立行为测试：同一正常会话保持账号；首选满载可使用其他有容量的合格账号；禁用/权限/模型不匹配账号不可入选；previous_response 和 guardian 绑定遵循原生。
- [x] 建立拒绝调用的质量仓库 stub，跑实际成功用量完成入口，断言零次 ListOpenAIAccountQuality；不能只测试 RequestRefresh 被删除。
- [x] 按 T0 差异块恢复官方选择流程，移除普通文本 WithOpenAIUnifiedQualityScheduling 和统一评分控制，不改变 WS/图片的原生路径。
- [x] 移除质量 provider 初始化、requestQualityRefreshAfterUsage 调用及管理投影的隐式刷新；T4 将去掉的字段标记不可用。
- [x] 删除额外冷启动/日常加分、统一评分资源池硬切分和重复设置读取；保留原生优先级、资格与负载。
- [x] 将本站利润资格/终检接回同一既有接口，并用利润合格池、全池越线 bypass、排队后价格变化用例证明既有语义保持。
- [x] 运行该任务定向测试，记录受影响差异；执行授权后提交此任务，暂不部署。

测试契约示例（复用现有仓库 fake，实际入口由 T0 锁定）：
```go
if qualityQueryCalls != 0 {
    t.Fatalf("forward/usage path queried historical quality %d times", qualityQueryCalls)
}
if got.Account.ID != stickyAccountID {
    t.Fatalf("healthy sticky binding moved to account %d", got.Account.ID)
}
```

**产出：** 普通文本选原生路径；质量聚合不再由流量或管理投影启动。原有业务接口尽可能不变。

### T2：恢复原生并发等待和尝试流程，保留安全否决

**修改：** B/internal/handler/openai_gateway_handler.go、openai_chat_completions.go、openai_embeddings.go、openai_retry_budget.go；直接相关调用使用 T0 清单。

**测试：** 新增 B/internal/handler/openai_native_convergence_test.go；复用 gateway_helper_hotpath_test.go、openai_retry_budget_test.go、openai_profit_slot_recheck_test.go、openai_responses_failover_cancel_test.go 的 fake 和场景。

- [x] 写失败复现：可控时钟推进六秒代表本地等待，首次 Forward 必须可达；不要真实睡六秒。
- [x] 用 httptest 走真实 handler，模拟队列满、Redis 获取失败、等待超时；验证状态码和结构化错误；SSE 检查错误终态，禁止只断言 helper 返回枚举。
- [x] 恢复原生等待与错误分支，移除只在 unified 路径出现的 RetryNext 状态，所有调用方穷尽成功、利润重选、已响应失败三类结果。
- [x] 按原生配置恢复最大切换和同账号重试，不再强制覆盖原生容量；去掉 ConsumeAttempt 对第一次转发的五秒拦截、故障域二次次数限制及由共享健康降级额外缩减次数。
- [x] 对 openai_retry_budget.go 的工具函数逐项处理：安全重放判断/必要 OAuth 冷却适配保留；仅用于退役 unified 预算的状态和注释删除；特殊协议仍运行的 legacy 预算保留。不允许直接删除文件导致安全保护丢失。
- [x] 验证取消后不启动新 Forward、已输出/已有工具副作用/已知用量不得被不安全重放；槽释放且计费不重复。保留本站明确安全例外，记录与原生差异。
- [x] 跑 T2 定向测试，记录结果并在执行授权后提交。

端到端错误断言：
```go
if recorder.Code == http.StatusOK && recorder.Body.Len() == 0 {
    t.Fatal("capacity failure returned an empty success")
}
if forwardsAfterCancel != 0 {
    t.Fatalf("started %d forwards after cancellation", forwardsAfterCancel)
}
```

**产出：** 原生有限尝试和有限等待；没有丢失返回状态或首次请求被额外预算拒绝。安全否决保持独立。

### T3：移除完整事件账本和新日志写入，修正历史清理

**修改：** B/internal/service/openai_resilience_observability.go、openai_scheduler_log_sink.go、wire.go；B/internal/repository/openai_scheduler_log_repo.go；相应 wire 生成文件遵循项目生成方式。消费者与 T4 同一候选交付。

**测试：** openai_scheduler_log_sink_test.go；新增 B/internal/service/openai_observability_retirement_test.go；现有日志仓库测试增加清理界限场景。

- [x] 写测试：批量模拟完成事件，不再增加 ledger/新日志写入计数；保留原生错误和 usage 的记录。
- [x] 停止全量 selection/outcome sink 装配，移除进程内完整 ledger 及事件关联计算；不采用“4096 改成更小”或默认关闭但仍每请求复制的伪修复。
- [x] 删除只服务旧账本的生产者；被共享健康/安全代码调用的符号先拆清副作用，不能删健康状态变化或业务审计记录。
- [x] 为历史日志清理保留现有 DeleteOpenAISchedulerLogsBefore 接口，清理生命周期独立于退役写入 sink。启动及每 60 秒触发，单次 context 2 秒，每批 1000、每轮最多 10000；不足一批或取消即结束，不固定等待。
- [x] 清理只按 event_at 早于 7 天删除；避免 worker/探测器重复启动；停止时取消并等待任务退出。
- [x] 验证写入失败不再存在该 sink 路径；历史查询仍能读取；清理遇到数据库故障不阻塞业务、不紧密重试。
- [x] 本地复跑原审计形状的 benchmark：4096 项满载前后观察 B/op，不再随历史条数增长；不以 M5 耗时作为服务器容量承诺。

清理核心边界：
```go
for deleted := int64(0); deleted < 10000 && ctx.Err() == nil; {
    n, err := cleaner.DeleteOpenAISchedulerLogsBefore(ctx, cutoff, 1000)
    if err != nil { return err }
    deleted += n
    if n < 1000 { return nil }
}
```

**产出：** 没有完整事件列表热路径；旧表保留、只读查询和有界保留期清理可用。本轮不建立新采样系统。

### T4：管理界面、设置和告警随运行态收敛

**修改：** B/internal/service/openai_account_scheduler_projection.go、ops_openai_scheduler_experience.go、ops_alert_evaluator_service.go、openai_scheduler_log.go；相关 admin handler 按 rg 调用定位。F/src/views/admin/AccountMonitorView.vue、SchedulerLogsView.vue、scheduler/schedulerPolicy.ts、ops/components/OpsOpenAISchedulerExperienceCard.vue；F/src/api/admin/accountMonitor.ts、schedulerLogs.ts 及相关现有测试。

- [x] 列明仍有效的原生设置与已退役的自定义设置；旧值保留，不改数据库。退役设置写入返回明确可识别的错误，禁止假保存。
- [x] 在现有投影/体验 DTO 添加可选 availability 字段；值限定 active/retired，retired 原因固定说明“已恢复原生调度，该自定义统计停止采集”。缺失数值为 null/省略，不填 0。
- [x] 历史日志列表添加可选 collection_status=retired；前端保留历史筛选，注明时间来源为日志 event_at，不把最新历史时间称为当前实时状态。
- [x] 旧关联告警指标返回不支持，显示规则不可用；核对 evaluator 的 false 语义，确保不被当作正常零值或恢复通知。其余原生告警照常运行。
- [x] 账号页保留成本、用量、实际状态；移除“质量榜等于实时选择顺序”的承诺。不能为了展示再调用退役质量仓库；不另建排名算法。
- [x] 界面变化仅限停用提示和失效控件处理，不进行新页面设计；本次续接修复已实施提示位于隐藏区域的问题。旧设置面板维持仓库原有隐藏状态，未重新开放。
- [x] 用 Vitest 验证 retired 显示、旧响应兼容、保存不带退休键、监控成本字段不受影响；原生参数编辑通过已有后端 API 验证，旧隐藏面板不重开；后端验证 raw 旧值保留与告警不可用。

**产出接口：** 在现有响应上添加 availability（active/retired）和 reason，日志响应添加 collection_status（active/retired），均为可选字段；不新增管理入口。

### T5：候选验收与交接

**文件：** T0 evidence，更新本方案复选框。不得修改总账/队列/生产记录。

- [x] 用 rg 核对生产调用闭环：无 WithOpenAIUnifiedQualityScheduling、RequestOpenAIAccountQualityRefresh 的有效入口；无 ledger 全表复制；无未处理 RetryNext。允许文档/历史测试记录保留旧名，不能把文本搜索本身当运行证明。
- [x] 运行受影响 Go 用例及前端相关 Vitest、typecheck。先运行任务级测试，最终只补跨任务接口和编译验证，复用未变化证据。
- [x] 受控上游模拟器验证：快速成功、首选繁忙后回退、长 SSE 取消、所有槽饱和、等待后成功、首次失败后原生重试。仅测试本次链路，无真实账号消费。
- [x] 分别验证空 200 为0、计费幂等/部分用量保护、取消后不新 Forward、槽释放、质量查询为0、自定义日志写入为0；不伪称全部来自同一真实 Redis/数据库压力场景。
- [x] 本地并发批次 1/32/128，模拟 10ms 响应与每 50ms 一条共 20 条 SSE，非流各1000请求，SSE workers32/128各1000、workers1缩减为20；记录端到端延迟和 goroutine 回落，分配用独立事件微基准，禁止外推主站最大 RPM。
- [x] 检查 git diff：无数据库迁移、无旧表/设置删除、无无关平台/计费改写。若有合并冲突，补冲突影响用例。
- [x] 输出候选 commit/tree、恢复原生清单、保留差异、测试命令与实际结果、未验证项目及旧镜像/路由回滚要求。发布另获明确授权后执行。

直接检查命令（工作目录 B；新用例创建后执行）：
```sh
go test ./internal/service ./internal/handler ./internal/repository -run 'Test.*(NativeConvergence|ObservabilityRetirement|ProfitRecheck|FailoverCancel|SchedulerLog)' -count=1
go test ./internal/service ./internal/handler ./internal/repository -run '^$'
```
前端在 F：
```sh
pnpm exec vitest run src/views/admin/__tests__/AccountMonitorView.spec.ts src/views/admin/__tests__/SchedulerLogsView.spec.ts src/views/admin/ops/components/__tests__/OpsOpenAISchedulerExperienceCard.spec.ts src/views/admin/scheduler/__tests__/schedulerPolicy.spec.ts
pnpm run typecheck
```

## 验收矩阵与发布条件

| 设计要求 | 实施任务 | 必须有的证据 |
|---|---|---|
| 固定版本、避免回退新平台修复 | T0 | 差异契约及保留清单 |
| 原生选择、粘性、资源池回退 | T1 | 行为用例 |
| 零请求驱动质量聚合 | T1/T4 | 请求完成及页面查询 spy |
| 队列明确响应、首次等待可转发 | T2 | httptest 和可控时钟 |
| 安全/计费保护不退化 | T2/T5 | 取消、已输出、副作用、用量幂等用例 |
| 事件固定开销退出、日志停止采集 | T3 | 调用计数及微基准 |
| 旧日志保留清理 | T3 | 条数/时间/取消/未过期不删除 |
| 失效指标不装作零值 | T4 | API、UI 和告警消费者测试 |
| 无 schema 与业务数据迁移 | T5 | diff 检查 |

不以高级开关关闭、健康检查成功或静态字符串测试单独判定完成。实现完成后仅称“候选完成”；发布版本和线上专项验证完成后再称上线完成。


## 2026-09-19 续接验收说明

- 复选框表示在原方案基础上完成本地候选范围；不表示线上部署或全仓回归通过。
- T0—T4 已实施部分复用交接及无变化证据；本轮只修复审查发现并补足直接用例。中间提交收敛为一个本地候选，未按任务重复推送或发布。
- T3 的“未过期不删”验证 worker 传入7天截止及 repository 发出严格 `<` SQL；没有运行真实 PostgreSQL 删除。
- T5 完整应用构建通过；本地 UI 仅实际体验组件截图与相关 DOM/保存测试，非登录后台全页验收。
- service unit 编译基线限制、两个 admin 旧失败、一个 WS 旧文案失败均明确写入 evidence；不为本任务扩大修复。
- 详细命令、性能样本、审查闭环及候选交付边界见 `../reports/2026-09-19-scheduler-convergence-evidence.md`。
