# 调度收敛实施证据

状态：本地候选验收完成，未推送、未合并、未部署；不是生产 DONE。执行分支 codex/scheduler-native-convergence，起点 30abf237e2；官方 v0.2.7 aea725f2ea644d5592d0bbb1d63b607efa7e200a。

## T0 差异契约

| 路径 | 官方/当前差异 | 实施决定 |
|---|---|---|
| Responses / Chat / Messages | 当前显式 unified opt-in 优先于 session sticky | 恢复原生粘性和 LB；退出统一评分 |
| Embeddings | 使用相同选号接口；原本没有 unified opt-in | 保留原生负载选择，退出额外预算共享影响 |
| WS / Images / 其他平台 | 共用 scheduler；协议、能力和额度分支已在 v0.2.7 更新 | 不回退这些分支；只退出新增全量事件采集 |
| buildOpenAIAccountLoadPlan | 本站公平性、动态 Top-K、组策略覆盖原生评分 | 普通 OpenAI 文本使用原生评分与加权顺序；保留其他协议既有行为 |
| handler account slot | 本站 unified 返回 RetryNext 但调用方直接返回 | 恢复原生已写响应的失败分支 |
| handler retry | 本站 5s/故障域/硬上限覆盖原生重试 | 普通 HTTP 回到 handler 原生次数控制，保留取消与重放安全判断 |
| OAuth 429 / runtime health | 官方冷却＋本站共享健康保护 | 保留冷却和已验证安全恢复，不加独立次数或质量排名预算 |
| 利润及权限 | 官方原有利润资格/终检；本站添加 unified 无合格池 bypass | 保留既有利润保护和明确 bypass 语义，恢复时需回归 |
| 成本与计费 | 本站 request/attempt 元数据、未知用量、幂等证据 | 保留，不恢复为缺少这些字段的官方计费实现 |
| 事件观测 | 官方无完整 ledger/自定义全量 sink | 退出新增采集；原生 usage/ops errors 不动 |
| 配置/页面 | 本站保存的组策略、quality cap、探索及运行排名 | 退出参数保留存储但不可编辑，展示 retired 而非零值 |

原生高级开关仍保留。原生 handler 默认最大换号数为 3，配置 Gateway.MaxAccountSwitches > 0 时按配置；不能使用本站 openAIMaxAccountSwitches=4 作为原生事实。原生 Top-K/score weights 是仍有效参数，不能整体禁用高级 scheduler。

## 验证记录

- 首次基线编译因 proxy.golang.org IPv6 超时失败；使用 GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct 下载同版本依赖后重试，不修改 go.mod/go.sum。
- 最终结果见下文；日志保存在 `docs/handoffs/scheduler-native-convergence-evidence/`。


## 本轮续接与审查闭环

在原 worktree 原 HEAD 上继续，未重建分支、未重置已有改动，未读取全量队列/总账，未访问主站或测试站。候选代码身份由本报告所属任务提交及其 tree 标识；最终回复给出实际 SHA。

只读审查发现并修复：原生账号级统计与分组结果上报脱节、native retry 范围误扩到图片/其他平台、无效组策略编译和热缓存深拷贝、隐藏区域内的退休提示。补充测试发现并修复普通设置保存重新规范化退休原值。详见 `review-summary.md`。

普通 OpenAI 文本现在使用固定 v0.2.7 的账号优先级、账号级 error/TTFT、原生 Top-K/weights、粘性与负载回退。分组运行样本同步到账号桶，group 0 不重复累计。无利润合格池时保留明确 bypass；合格账号仅仅满载不能 bypass，其他资格仍生效。

HTTP 普通文本默认换号 3 次、配置为正时尊重配置，不再受额外 5 秒/总尝试/故障域预算拦截；实际换号、同账号重试、取消与安全否决仍由原有 handler 控制。图片/其他平台保留原预算和最多 4 次 clamp。删除仅旧统一预算使用的构造器、状态和不可达分支；没有把所有重试保护一起删除。

事件 wrapper 已成为不组装 metadata 的 no-op；没有运行态 ledger、质量 provider 或新增事件 sink 写入。历史 repository DTO、显式 writer/Flush 兼容接口仍保留，运行时不调用。历史清理仅在既有请求角色运行：启动一次、每分钟、每轮 2 秒、每批 1000、最多 10000、Stop 取消并等待结束。

退休参数拒绝显式写入；普通保存从更新集合排除退休键，保持 raw 存量值（包括 group_policies 映射的 group_overrides）。原生 Top-K/weights API 仍有效。**原仓库已经隐藏整个旧调度编辑面板，本次维持隐藏，不宣称原生控件在页面可编辑**；只在可见网关设置区域补充退休提示，明确专用路径仍可能使用旧策略。未新增页面设计或管理入口。

## 最终验证与证据

| 验证 | 结果与范围 | 证据文件 |
|---|---|---|
| 默认 Go 定向集合 | service / handler / admin / repository 四包通过；含 native、scheduler、retry、RecordUsage、failover、Grok sticky | backend-final-pass.txt |
| unit handler/admin | 两包通过；真实取消用例、利润终检、native mock 全矩阵、体验 API/退休设置 | handler-admin-unit-final-pass.txt |
| 安全专项 | 已输出不得重放、tools/function_call_output/tool_result 不安全重放、认证失败换号、配置 429 同账号重试、部分用量、Grok bounded retry 通过 | safety-final-pass-summary.txt |
| 容量与释放 | 队列满、Redis 错误、等待超时：JSON 明确错误；已开始 SSE 返回 response.failed；等待成功只释放一次槽 | capacity-final-pass-summary.txt |
| 利润边界 | 全池越线明确 bypass；合格池满载不 bypass；禁用/模型不匹配/排除不复活 | backend-final-pass.txt |
| 历史清理 | 多批、10000 上限、取消、错误只尝试一次、2 秒 deadline、启动/Stop；SQL 严格 event_at < cutoff + limit | cleanup-final-pass.txt |
| 分组原生统计 | 修复前方法 overlay 红测、修复后通过；group 0 不双记 | native-stats-baseline-red.txt、backend-final-pass.txt |
| 退休设置 | 显式字段含 null 拒绝；普通保存保留旧原始值，原生 Top-K 可写 | backend-final-pass.txt |
| 前端 | 5 文件 86 通过、11 个既有 skip；typecheck 通过；i18n 3 通过 | frontend-final-pass-summary.txt、frontend-final-typecheck.txt、frontend-i18n-pass-summary.txt |
| 构建 | Go server、Vite 前端、Go embed 完整应用均 exit 0 | build-summary.txt |
| 视觉检查 | Browser 技能启动本地实际体验卡，stub retired API；显示停止采集，无样本窗口/数值栅格，截图无遮挡 | retired-experience-local.png |
| 差异检查 | git diff --check 通过；无 schema/migration、依赖锁文件、余额/售价修改 | 本任务 commit diff |

后端主集合：
```sh
go test ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository \
 -run 'Test.*(NativeConvergence|SchedulerRetirement|OpenAI.*Scheduler|OpenAI.*RetryBudget|OpenAI.*RecordUsage|OpenAI.*Failover|GrokVideoSticky|GroupResultReporting)' -count=1
```
unit 主集合：
```sh
go test -tags unit ./internal/handler ./internal/handler/admin \
 -run 'TestOpenAIGatewayHandlerResponses_FailoverAbortsWhenClientDisconnected|TestAcquireResponsesAccountSlotProfitRecheck|TestNativeConvergence|TestOpsSchedulerExperience|TestSchedulerRetirement' -count=1
```
前端完整命令仍为本方案 T5 列出的四文件，另加 `src/views/admin/__tests__/SettingsView.spec.ts`；通过 node 调用已存在的 vitest/vue-tsc/vite，未更改依赖版本。构建产物仅用于本地验证，不作为合规发布制品。

### 模拟并发

真实 Responses handler + 本地假 HTTPUpstream，无网络和付费账号。非流每组 1000 请求，10ms 假上游；SSE 20 条×50ms，workers 32/128 各1000请求，workers 1 使用20请求（明确缩减串行样本）。全矩阵共5020请求，最终 unit 集合复验通过；以下延迟来自首次补齐矩阵的日志，不伪称最终树的独立性能保证。

| stream | workers | requests | 端到端 P99 | goroutine 前→后 |
|---|---:|---:|---:|---:|
| false | 1 | 1000 | ~15.5ms（复用先前烟测） | 14→14 |
| false | 32 | 1000 | 16.816ms | 14→14 |
| false | 128 | 1000 | 31.722ms | 15→14 |
| true | 1 | 20 | 1.025s | 15→15 |
| true | 32 | 1000 | 1.027s | 16→14 |
| true | 128 | 1000 | 1.035s | 15→14 |

断言：空/错误/缺终态响应0，成功上游额外尝试0，goroutine 回落。此 mock 不接真实 Redis/Postgres，不代表真实槽饱和压测；容量/释放、零质量查询、零事件写入、取消后无新 Forward、计费幂等分别由直接用例验证，不把它们伪报为同一压力测试观测。

### 事件微基准

同一本地 M5、预热4096事件，`go test ./internal/service -run '^$' -bench '^BenchmarkNativeConvergenceRetiredEvent$' -benchmem -benchtime=100ms`。

- overlay 基线 HEAD 的旧 event ledger 文件，使用同一测试夹具和兼容 sink：703676 ns/op，7225401 B/op，4 allocs/op。
- 候选退休入口：1.095 ns/op，0 B/op，0 allocs/op；无副作用 no-op 可被编译器消去，这是退休入口的预期行为。

只证明旧完整 ledger 分配已退出，不把微基准或 mock 吞吐外推为主站 RPM。

## 基线限制与未验证项

1. 默认定向集通过，不宣称 `go test ./...` 或全 service `-tags unit` 全绿。交接记录的 service unit 编译失败仍未扩修（重复 ptrFloat、旧签名、缺 import、图片旧符号）。
2. 两项 admin AccountMonitor 测试已用原始 HEAD 全部修改 Go 源恢复 overlay 复现同样失败：`ReturnsCompleteWindowTimelineAndGlobalRanking`、`ReturnsUnavailableAndStaleRowsWithRetainedNativeScores`。当前改动只在非 nil 的 retired projection 新分支；这两个夹具传 nil projection。
3. 扩展安全集仅 WS `TestResponsesWebSocketCredentialFailoverLoop/provider_configuration_stops` 旧错误文案断言失败；原始 HEAD overlay 同样返回中文清洗错误而非期待英文，已记录。其余安全用例单独通过。交接中两个已复现的旧 handler 失败保持原记录。
4. 视觉验证仅实际退休体验组件与单元 DOM/保存流程，不是登录管理后台后的全页面验收。设置旧面板维持原有隐藏状态；原生编辑能力仅验证服务端 API。
5. 日志清理真实 PostgreSQL 删除未执行；测试验证 worker 生命周期/限额/取消以及真实 repository 发出的严格截止 SQL。未访问任何环境数据。
6. 未执行生产压测、测试站验收、推送、合并或部署；两站运行态未查询，不推断一致性。

## 候选保留与后续边界

保留 `codex/scheduler-native-convergence` 和该 worktree，node_modules symlink 不提交。只进行本地任务提交。未来发布需另有明确授权，从届时干净、已推送根 main 按新 SOP 构建/推广；旧版本制品/路由用于恢复，配置旧值与历史表未删除。超过保留期已清理日志不可承诺回滚恢复。
