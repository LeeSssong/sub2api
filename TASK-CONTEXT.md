# 调度恢复原生：本地候选交接（2026-09-19）

**本地实施、只读审查及直接相关验收完成。未推送、未合并、未部署，不是生产 DONE。**

## 工作区与授权

- 唯一 worktree：`/Users/gongtengxinwen/Documents/sub2api搭建/.worktrees/scheduler-native-convergence`
- 分支：`codex/scheduler-native-convergence`
- 实施基线：`30abf237e25ce21ca406030f95c7fab3ff92d8af`
- 固定官方恢复契约：v0.2.7 / `aea725f2ea644d5592d0bbb1d63b607efa7e200a`
- 最新候选身份：本任务本地提交，使用 `git log -1` / `git rev-parse HEAD^{tree}` 核实。
- 用户授权本地实施、审查、验收及计划内任务提交；明确暂不推送、合并或部署。后续不得自动扩大授权。
- 续接已保留原 worktree 全部改动，没有 reset/clean/重建工作区，也没有访问服务器或读全量队列/总账。
- 前端 `node_modules` 仍是复用根仓库依赖的未跟踪 symlink，禁止提交；本地构建产物只作验证，不是合规发布制品。

## 最终行为

普通 OpenAI HTTP 文本退出统一质量选择、请求质量 SQL 刷新、组策略评分及完整事件采集；恢复 v0.2.7 粘性/负载/Top-K/账号级 error/TTFT，账号统计可收到真实分组请求上报。普通文本默认换号3次、按正配置覆盖，退出额外5秒/故障域准入预算。图片、WS、其他平台的特殊算法/预算保持，计费、安全重放、权限、取消和利润终检保留。

全池无利润合格账号的显式 bypass 保留；合格池满载不 bypass，不复活禁用/模型不匹配/排除账号。删除不可达 unified 预算状态和分支。

事件入口 no-op、无 ledger/新日志写入；历史查询保留，清理启动/分钟循环、2秒、1000×最多10批，Stop 取消并等待。退休指标/告警返回 unavailable，保留 enabled；显式写退休参数拒绝，普通保存不重写 raw 原值。

旧设置面板本来就隐藏，本次维持隐藏。原生参数 API 仍可写；退休提示移到可见区，说明旧策略在专用路径仍可能使用。没有新设计或新管理入口。

## 验收事实与限制

- 默认 Go 定向四包通过；unit handler/admin 通过（实际取消、利润终检、全部 mock 场景）。
- 安全专项、JSON/SSE 容量终态与等待后释放、历史清理边界/SQL 契约通过。
- 前端5文件86通过/11既有skip，typecheck通过，i18n3通过；Go、Vite和Go embed构建通过。
- 本地假上游矩阵1/32/128：非流每组1000，SSE单并发20、多并发各1000；空响应/缺终态0，成功请求额外尝试0，goroutine回落。不是线上RPM或真实Redis饱和测试。
- 4096事件微基准：旧 ledger overlay 7225401 B/op，退休入口0 B/op；仅说明事件开销，不代表全请求分配。
- 只读复审确认4个发现已修复。剩余TTFT debug标记读legacy容量字段只用于no-op观测，不影响转发。
- service 全unit仍有交接已知编译失败；两个admin AccountMonitor旧失败及一个WS文案旧失败已在原始HEAD overlay复现。未篡改业务迁就旧断言。
- 浏览器只验收实际退休体验组件；未登录后台作全页验收。清理未跑真实PostgreSQL。未检查主站/测试站运行态。

## 权威证据与下一步

1. `docs/superpowers/reports/2026-09-19-scheduler-convergence-evidence.md`：最终变更/命令/范围/限制。
2. `docs/handoffs/scheduler-native-convergence-evidence/`：通过日志、红测、审查、基线对照、截图；`initial-task-context.md`保留续接前原始交接。
3. 原 spec/plan 已同步候选状态，不要再按初始待办重复实现。

当前只保留候选，等待用户进一步指令。未来整合或部署需明确授权，并按届时根目录新SOP、干净已推送main来源执行。不自动追随根main、不修改全量队列/总账。
