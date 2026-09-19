# 调度恢复原生：实施中交接（2026-09-19）

> 新窗口首先读本文件，再读本任务 spec/plan 与直接涉及的 diff。不要读取项目全量队列、总账和其他历史交接。
> **任务尚未完成；全部代码改动尚未提交，未合并、未推送、未部署。不要把本交接当作候选完成证明。**

## 1. 用户目标与授权

用户先要求审计本站大量调度改造，重点找高并发/高 RPM 下比 Sub 原生更差的内容，排除上游账号数量不足。随后接受恢复原生或修正现有改造，要求制作实施方案，最后明确说 **“开始实施”**。

最新指令是因上下文过长转到新窗口，要求 handoff。它是交接，不是取消实施目标。当前窗口已停止业务代码工作，仅整理交接。

已授权：本地实施已写方案、直接相关测试和计划内任务提交。未授权：生产部署、推送、合并根 main、线上压力测试。不要重复索取已授权开发的确认。

本轮范围：恢复原生普通 OpenAI 文本调度/准入/重试，移除自定义质量聚合、完整事件账本与全量调度日志采集；保留计费、安全重放、权限、协议兼容和既有利润回退语义。不新增原生优化、指标平台或质量榜，不改数据库 schema、余额、售价、已有设置值。

## 2. 唯一执行工作区

- 仓库根：`/Users/gongtengxinwen/Documents/sub2api搭建`
- **执行 worktree：`/Users/gongtengxinwen/Documents/sub2api搭建/.worktrees/scheduler-native-convergence`**
- **分支：`codex/scheduler-native-convergence`**
- 当前 HEAD / 开始实施基线：`30abf237e25ce21ca406030f95c7fab3ff92d8af`
- 固定原生恢复基线：官方 **v0.2.7**，commit `aea725f2ea644d5592d0bbb1d63b607efa7e200a`，Git 对象已下载。
- 初次审计用 v0.2.4；实施不能按那个旧版本整文件回退。
- 早期文档 worktree `.worktrees/scheduler-convergence-plan` 只保存旧方案，不在其中继续实现。
- 当前 execution worktree 有 44 个 tracked 修改/删除文件，还有新增测试/文档。不要 reset、clean、覆盖或再次另建分支丢掉改动。
- 根 main 可能被其他任务推进；不要去根目录改代码，也不要自动把本 worktree 重置到它。
- 一个写入者；实现一直由主代理写。只读审查代理曾启动但被用户中断，见第 9 节。

## 3. 任务文档

执行 worktree 内：

1. `docs/superpowers/specs/2026-09-19-scheduler-native-convergence-design.md`
2. `docs/superpowers/plans/2026-09-19-scheduler-native-convergence.md`
3. `docs/superpowers/reports/2026-09-19-scheduler-convergence-evidence.md`
4. `docs/handoffs/scheduler-native-convergence-evidence/`：已复制主要通过日志和基线失败摘要，避免依赖 /tmp。

spec/plan 是从文档 worktree 复制过来的未跟踪文件。已更新其中开发授权，但其标题状态、复选框和 evidence 还没有反映所有进度，**本 handoff 对当前状态更准确**。最终需同步文档，不能仅按旧复选框全部重做。

## 4. 已实现的业务改动（都尚未提交）

### A. 普通文本退出统一质量选号及 SQL 刷新

主要文件 `backend/internal/service/openai_account_scheduler.go`（下文路径都在 `upstream/sub2api/` 下）：

- 删除 `Select` 中优先于 sticky 的 unified-quality 分支。
- 移除各 HTTP handler 的 `WithOpenAIUnifiedQualityScheduling` opt-in，以及该 context helper/Responses wrapper。
- 删除 `openai_unified_quality_scheduler.go`、`openai_quality_score.go` 及对应退役算法测试。
- 删除 Gateway 构造器的质量 provider 装配、`openaiQuality` 字段、Snapshot/RequestRefresh 方法，以及成功用量后的刷新入口。
- `openai_account_quality.go` 目前只留历史 repository 所需 DTO/接口；`repository/usage_log_quality.go` 没删，但生产已没有调用它的聚合入口。
- 新 `usesNativeOpenAITextSelection(req)`：平台 OpenAI、无图片能力、transport 为 Any 或 HTTPSSE 才恢复原生；WS/图片/其他平台保持既有特殊策略。
- 普通文本使用原生 account priority、account runtime stats、全局原生 weights/TopK；不应用公平性加分、动态 TopK、质量门、分组 quality sticky escape。
- 专用协议仍保留共享实现中的相关机制。注意：`resolveOpenAIAccountSchedulerPolicy` 和 `buildOpenAISelectionOrder` 里仍可能读取/编译旧策略后被 native 分支忽略，最终审查应评估能否顺手避免这些无效读取，但不要扩大成新算法。
- 保留运行健康、共享冷却、半开、权限、协议能力和利润保护。

### B. 保留利润全池无合格账号的既有 bypass

原 unified selector 存在“无利润合格候选时可用性优先”语义。直接回原生会错误变成拒绝，所以新实现：

- `selectByLoadBalance` 在 `len(filtered)==0` 且存在 profit exclusion、普通文本、当前未 bypass 时，以 bypass context 重入一次。
- 成功 selection 带 `profitBypass=true`，供 handler 终检重放。
- 有利润合格候选但容量满时**不能** bypass。
- 已有一个“全池越线仍保留明确 bypass”行为回归通过。
- **还应补“利润合格候选满载不得绕过、禁用/模型不支持不因 bypass 被放行”的回归，审查原先资格过滤顺序是否引入语义偏差。**

### C. 原生等待/重试与安全保护

`handler/openai_gateway_handler.go`：

- 删除 `openAISlotAcquireRetryNext` 及三个不写响应的返回分支；现在使用既有明确错误响应。
- handler 默认最大换号恢复为原生 3，配置 >0 时尊重配置，不再 clamp 到本站 4。

`handler/openai_retry_budget.go`：

- 目前使用兼容方案：`openAIRetryBudgetConfig.NativeHTTP` + state `nativeHTTP`，运行时 `openAIRetryBudgetConfigFromConfig` 返回 true。
- nativeHTTP 下 `ConsumeAttempt` 只记录次数、不因 5 秒期限/额外上限拒绝；`CanSwitch` 仍拒绝已输出/副作用；`ObserveDomain` 只记录；共享健康降级不缩减预算；重试等待调用原生 `sameAccountRetryDelayFor`。
- 真实次数仍由 handler 原生 `switchCount/maxAccountSwitches`、同账号 retry limit 控制。
- 旧 unified/non-native 预算辅助逻辑及测试仍保留，部分仅兼容用途。**这是收尾审查重点：不要宣称整份预算代码已删除；检查仍有无运行入口绕过 NativeHTTP 或使用 cfg.MaxAttempts 做第二次拦截。**
- 安全重放、取消检测、用量完整性/计费幂等没有删除。

### D. 退出完整观测，保留历史清理

- `openai_resilience_observability.go` 删除 4096 项全局 ledger 及 readers/counters。
- `RecordOpenAIResilienceOutcome` 保留为 no-op 兼容入口，原来健康/安全操作自身副作用继续存在。事件 wrapper/部分事件构造调用尚留，可做有针对性的清理，不要误删健康状态更新。
- `openai_scheduler_log_sink.go` 的 run 只做历史清理：启动一次，以后每分钟；每轮 context 2 秒、每批 1000、最多 10000，取消或不足一批即结束。
- 运行时默认 sink 和 ConfigureDefault sink 不再分配请求事件队列；wire 非请求角色返回空 sink，不启动清理。
- 旧 Enqueue/Flush/Writer 接口与仓库写方法仍在，为兼容及已有测试保留，但请求生产者已 no-op，运行 worker 不 flush。
- 历史表不删，查询继续。清理不依赖写入 tick。
- 需补清理的 10000 上限、context 取消、错误不忙重试、未过期不删的直接测试；目前仅 2500 分三批已测。

### E. 页面/API/设置/告警

- 普通 OpenAI 文本 `Project` 返回 `availability=retired`、reason、空 candidates；不查询质量历史。其他协议投影保留。
- `AccountMonitorService.attachSchedulerProjection` 对 retired 设置 SchedulerUnavailable、清空 scheduler rank/score，保留监控成本/状态等。
- 调度体验 API 返回 availability/reason，指标 value 为 null；前端 retired 时不显示指标栅格/样本和窗口。
- 调度日志 list API 返回 `collection_status=retired`；前端显示历史记录提示。
- 五种依赖旧事件的告警 metric `computeRuleMetric` 返回 `(0,false)`；核查过 evaluator false 分支只 reset evaluation state 并 continue，在恢复通知逻辑之前，不会把它当健康零值发恢复。
- `OpsService.ListAlertRules` 给这五类 rule 的副本添加 Availability/AvailabilityReason，不重写 Enabled；前端规则卡显示不可用原因。
- admin UpdateSettings 在读/写 service 前拒绝请求中显式出现的七项退休参数：candidate_pool_mode / exploration_ratio / starvation_threshold_seconds / fairness_weight / group_overrides / group_policies / custom_presets。
- SettingsView 保留原生开关/TopK/weights，退休区域放进 disabled fieldset，保存 payload 不再带退休项；删除无用 serializeSchedulerPolicies、validateSchedulerPolicyDraft（**这两个删除晚于上一轮 typecheck，需最终再跑一次 typecheck**）。
- 账号监控页加普通文本恢复原生且排名不代表下一次选择的提示。未做新页面设计；还没做浏览器视觉检查。

## 5. 新增测试与改动测试说明

新增：

- `backend/internal/service/openai_native_convergence_test.go`：健康 sticky、实际 RecordUsage 零质量查询、事件零新写入、2500历史清理、多种 retired 告警、native TopK、利润 bypass、投影 retired、列表告警不可用。
- `.../openai_native_convergence_fixture_test.go`：复用账号 fixture。
- `backend/internal/handler/openai_native_convergence_test.go`：可控时钟六秒后首次 Forward 预算准入与已输出安全否决。
- `.../openai_native_capacity_test.go`：真实 Gin HTTP 边界调用槽位 handler，队列满/Redis 失败/等待超时必须返回非空错误。它不是完整 Responses 入口的饱和压测；不要夸大。
- `.../openai_native_load_test.go`（unit tag）：实际 Responses handler + 本地假 HTTPUpstream，无网络/无付费。
- `.../admin/scheduler_retirement_test.go`：退休参数显式写入拒绝。
- 前端 OpsOpenAISchedulerExperienceCard 测试增加 retired 不显示旧成功率。

旧测试：退役统一评分/事件列表/普通HTTP排名的旧契约测试删除；保留 Grok projection、pure helper 和安全/计费测试。不能把删除测试数量当作实现充分性，最终审查需确认替代行为覆盖。

两个已有 fixture 修复：

1. `grok_media_selection_test.go` 的 escaped 已是 string，True/False 改为 NotEmpty/Empty，否则默认 service 包无法编译。
2. `openai_account_scheduler_test.go` 的 weighted previous-response fixture 显式补 PreviousResponse weight=5（之前设了其他权重却没有这个值，断言偏好原账号不成立）。

保留计费测试中的事件断言改为真实 billing command/usage 元数据断言；不是把计费测试整体删除。

## 6. 已运行的验证与新鲜度

持久证据目录：`docs/handoffs/scheduler-native-convergence-evidence/`。

### 通过

1. `backend-targeted-pass.txt`（原 /tmp/scheduler-convergence-tests6.log）：四包都通过：

```sh
# cwd upstream/sub2api/backend
go test ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository \
  -run 'Test.*(NativeConvergence|SchedulerRetirement|OpenAI.*Scheduler|OpenAI.*RetryBudget|OpenAI.*RecordUsage|OpenAI.*Failover|GrokVideoSticky)' -count=1
```

2. `handler-admin-unit-targeted-pass.txt`：handler/admin 带 unit 的直接集合通过：

```sh
go test -tags unit ./internal/handler ./internal/handler/admin \
  -run 'Test(NativeConvergence|AcquireResponsesAccountSlotProfitRecheck|OpenAIGatewayHandlerResponses_FailoverStops|OpsSchedulerExperience|SchedulerRetirement)' -count=1
```

注意正则 `FailoverStops` **没有匹配真实取消测试名**。真实取消测试名为 `TestOpenAIGatewayHandlerResponses_FailoverAbortsWhenClientDisconnected`，下一步显式跑它，不要把以上日志声称覆盖该用例。

3. `frontend-targeted-pass.txt`：四文件 47 测试通过，包括 AccountMonitorView(36)、SchedulerLogsView(2)、OpsExperienceCard(6)、schedulerPolicy(3)。
4. `frontend-typecheck-prior.txt`：空日志表示当次 typecheck 成功，但晚于此有少量 SettingsView/告警 UI 清理，最终需补跑。
5. `mock-load-smoke-summary.txt`：实际 Responses handler 单并发非流请求 1000 次成功，P99 约 15.5ms（假上游10ms），goroutines 14→14，无额外重试。
6. 新增 retirement rule list/容量错误等定向测试已运行通过，未全部输出到持久日志；最终定向总集合补跑一次覆盖最新内容。
7. 交接时 `git diff --check` 无输出。

### 尚未跑完或不能宣称通过

- Mock burst 的 workers=32/128 以及全部 stream=true 子场景尚未执行。
- 代码审查未完成。
- 无完整 service `-tags unit` 通过证据；基线就无法编译，详见下一节。
- 无生产压测/上线/测试站检查。
- 无最终整应用构建与候选 commit/tree；当前没有新 commit。
- 无“恢复后事件 B/op”的新微基准结果；只验证 no-op 无写入和代码删除。
- 没有最终 UI 浏览器检查。

## 7. 基线问题，避免无关扩大修复

### Go 依赖下载

默认 proxy.golang.org IPv6 超时，已用以下命令获取**相同版本**依赖，没改 go.mod/go.sum：

```sh
GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct go mod download
```

### service -tags unit 原有编译失败

`unit-suite-known-failures-summary.txt`：

- payment_config_plans_validation_test / account_monitor_quality_fusion_test 重复 ptrFloat。
- account_stats_pricing_test 旧函数签名缺时间参数。
- gateway_forward_as_chat_completions_test 缺 context import。
- openai_images_tool_cooldown_test 引用已不存在的字段/函数。

不应为本任务修复整个支付/图片测试体系。默认 service 定向集合可编译运行；handler 带 unit 单独运行不编译 service 测试，因此能验证所需利润/取消。

### 已在根 main 原样复现的 handler 旧失败

`baseline-handler-failures.txt`，根 main 只读测试结果：

- TestOpenAICapacityFailoverExhaustionPreservesMessageAsServerError：旧断言期待英文 overload，当前错误清洗返回中文通用错误。
- TestOpenAIGatewayHandlerResponses_FailoverContinuesForConnectedClient：旧断言期待 [1,2]，实际原生已有同账号重试，返回 [1,1,2,2]。

这两个失败不是本分支引入，不要篡改业务来满足旧断言。取消用例需单独验证。

### admin 带 unit 扩展集合

两个 AccountMonitorHandler 测试失败（字段/保留分数口径），**尚未在根 main 对照，因此不能直接归为基线**。失败名：

- TestAccountMonitorHandlerReturnsCompleteWindowTimelineAndGlobalRanking
- TestAccountMonitorHandlerReturnsUnavailableAndStaleRowsWithRetainedNativeScores

应对照基线/新退休契约后判定，而不是忽略。OpsSchedulerExperienceHandler 的旧数据断言已改为 retired/null 并通过。

### 前端依赖

系统 fallback pnpm 与仓库 lockfile overrides 配置不兼容，frozen install 失败。为避免改锁文件，execution worktree 的 `upstream/sub2api/frontend/node_modules` 是指向根仓库同版本 node_modules 的 symlink：

`/Users/gongtengxinwen/Documents/sub2api搭建/upstream/sub2api/frontend/node_modules`

**此 symlink 当前显示 untracked，绝不能 git add 提交！** 所有 npm 测试通过 node 直接调用现有 bin；没有改 package.json/pnpm-lock.yaml。

## 8. 下一步（按顺序，勿重头开始）

1. 读当前 AGENTS、此 handoff、spec/plan；进入指定 execution worktree，确认 HEAD/dirty files。不要重新创建 implementation worktree。
2. 做一次只读 review（前个 reviewer 被中断，没有结果）。重点：
   - 普通文本 helper 的 transport/ImageIntent 边界与 WS 特殊路径是否保持。
   - 原生评分是否还有本站额外优先级排序/组策略残留（不仅关 unified）；检查 `isOpenAIAccountCandidateBetter`、LB plan、runtime settings。
   - nativeHTTP retry state 是否仍有第二层次数/时间判断，响应已输出/副作用保护是否完整。
   - 利润 bypass 仅无合格池时触发，不能因为容量不足或权限失败错误扩大；补负向用例。
   - 退休设置被拒绝但旧设置读取仍参与 WS/特殊协议：文档和 UI 描述应准确，不能声称全平台都不使用。
   - 清理生命周期、限额、错误/取消；旧 sink 接口残留不得被运行路径使用。
   - 退休告警不可用与 Enabled 保留；设置普通保存不误写退休参数。
   - 大量删旧测试之后剩余直接行为证据是否足够。
3. 清理无效注释和符号（例如 request 中 unifiedQuality bool/残留说明、OpenAIUnifiedExtraRetryCount、旧预算分支等），仅在确认无生产依赖后删除。不要为了测试保留仅测试使用的生产字段。
4. 补必要测试：利润合格池满载不 bypass、不可调度账号不被 bypass 复活、清理10000上限/取消、disabled settings不覆盖存量值、实际取消用例。
5. 执行已写 mock burst 剩余子场景；全矩阵约 70 秒上下。测试为 false:1000每组；true:workers1仅20请求（原计划1000个1秒串行需16分钟，已收敛），workers32/128各1000请求，每请求20×50ms。记录此样本缩减，不能伪称原计划全量都测了。
6. 最新默认 Go定向测试、相关 unit handler/admin、前端typecheck/Vitest、必要应用编译。依据变化补测试，不重复全量无关回归。
7. 更新 evidence 和 plan：当前大部分复选框没更新，明确有验证的完成项、基线限制、未验证项。不要宣称整包unit全部通过。
8. 完成直接审查后按方案做任务提交（避免 git add . 包含 node_modules）；保留一个完整候选，仍不推送、不合main、不部署。
9. 最终交付候选 commit/tree、恢复/保留清单、测试事实与限制、部署仍需明确授权。

## 9. 运行中的工具/代理

交接检查时没有仍运行的 go test/vitest/vue-tsc 进程。

只读代理 `/root/review_convergence` 已被用户中断，状态 interrupted，**没有可用审查结论**。新窗口无法依赖它的内存状态；可以重新发起一次有边界的只读审查。其依据是已读取的 requesting-code-review skill 明确要求 reviewer；没有实现子代理写代码。

主代理此前已加载 executing-plans、test-driven-development、writing-good-tests、requesting-code-review。项目要求最小验证、单写入者优先；用户实施授权覆盖计划内正常步骤。

## 10. 可直接使用的命令

```sh
cd '/Users/gongtengxinwen/Documents/sub2api搭建/.worktrees/scheduler-native-convergence'
git status --short
git diff --check
```

后台测试须给每个 exec 指定正确 workdir，不依赖上次 cd。Go目录为 worktree/upstream/sub2api/backend；前端为 worktree/upstream/sub2api/frontend。

```sh
# cwd backend：已完成单并发fast；运行其余mock场景
go test -tags unit ./internal/handler \
 -run '^TestNativeConvergenceHTTPMockBurst/(stream_false_workers_(32|128)|stream_true_workers_(1|32|128))$' \
 -count=1 -v > /tmp/scheduler-convergence-load-remaining.log 2>&1

# 真正的取消测试 + 利润槽终检
go test -tags unit ./internal/handler \
 -run 'TestOpenAIGatewayHandlerResponses_FailoverAbortsWhenClientDisconnected|TestAcquireResponsesAccountSlotProfitRecheck|TestNativeConvergence' -count=1

# cwd frontend
node node_modules/vue-tsc/bin/vue-tsc.js --noEmit
node node_modules/vitest/vitest.mjs run \
 src/views/admin/__tests__/AccountMonitorView.spec.ts \
 src/views/admin/__tests__/SchedulerLogsView.spec.ts \
 src/views/admin/ops/components/__tests__/OpsOpenAISchedulerExperienceCard.spec.ts \
 src/views/admin/scheduler/__tests__/schedulerPolicy.spec.ts
```

本轮未访问生产进行任何写操作，没有修改数据库或真实账号凭据。前次审计的主站状态不应当作现在运行态。
