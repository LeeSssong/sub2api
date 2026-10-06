# PR #84 融合与部署评估

日期：2026-09-25。目标分支：`codex/ranxi-v2813-fusion`。本次仅在本地融合候选中合并 PR #84，未推送、未修改根目录 `main`、未部署生产。

## 来源

- PR：[ranxi2001/sub2api#84](https://github.com/ranxi2001/sub2api/pull/84)
- 标题：`feat: 为 Excel / BPS 增加 403 时自动关闭选项`
- 上游 PR head：`7a97283dd8db71a3ffd5543c7050901868ce4028`
- 上游 PR merge base：`3e345632fd66aea7724c5929e9415a828a4ae83b`
- 融合基线：已完成的 Xingqiao × ranxi v2.8.13 候选，当前仍保留本项目的账务、发布、turn-state、蓝绿和界面定制。

## 融合内容

PR #84 已适配现有分支差异并纳入候选，共涉及后端仓储/服务/调度快照、前端账号编辑弹窗、中文/英文文案和专项测试 17 个文件。

- 新增账号 extra：`openai_excel_bps_auto_disable_on_403`，默认关闭。
- 只有管理员在 OAuth 账号的 Excel/BPS 设置中主动勾选后，真实上游 HTTP 403 才会自动关闭该账号的 Excel/BPS 开关。
- 只修改 BPS 开关；不修改账号状态、可调度状态、凭据、模型范围，也不重试当前请求。
- 模型权限专用错误、其他 HTTP 状态、传输失败和 HTTP 200 SSE 内部错误不会触发自动关闭。
- 更新采用凭据和 opt-in 条件的条件写入，随后通过既有 scheduler outbox 刷新调度快照；`updated_at` 使用项目现有单调递增写法，避免并发版本回退。
- 批量关闭 BPS 时会清理该选项；旧账号缺少字段时按关闭处理。普通用户页面和既有导航不变，新增选项仅出现在管理员账号编辑弹窗的 BPS 区域。

## 验证

- `go test ./internal/service ./internal/repository -run 'ExcelBPS|AutoDisable' -count=1`：通过。
- `go build ./cmd/server`：通过。
- `pnpm exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts`：66/66 通过。
- `pnpm typecheck`：通过。
- `pnpm build`：通过；仅保留既有 chunk 大小和动态导入提示。
- `git diff --cached --check`：通过。

## 是否需要停机

### 仅 PR #84

不需要停机。PR #84 不新增数据库迁移、不改变表结构、不要求重建索引。它只读写现有 `accounts.extra` JSONB，并在同一事务中写入已有 scheduler outbox。新旧 API 可以在蓝绿切换期间共存：旧版本忽略新字段，新版本对缺失字段按 false 处理。前端字段是增量兼容的，未勾选时行为与旧版本相同。

### 合并到当前完整融合候选

仍可按不停机蓝绿路径发布，但前提是现有在线迁移和连接保留门禁全部满足：

- 迁移集合哈希与已审查目标一致，在线迁移锁等待/执行超时和 schema receipt 校验通过；
- 生产 live Caddy 及实际候选配置已具备至少 24 小时 `stream_close_delay`；
- BPS blue/green 精确回调路由在 public 和内部 `:8081` 均已存在且不改写路径；
- 使用 retain 排空模式，旧 API 在 300 秒后仍有连接时保留，不强杀；worker 使用 60 秒优雅停止窗口。

这些条件不满足时，发布链应在迁移或切流前拒绝继续；不能把“PR #84 无迁移”当作整个融合候选已经具备无停机上线条件。当前尚未对生产执行这些前置检查，因此本报告不宣称生产已具备无停机发布资格，也未执行任何线上操作。

即使条件满足，“不停机”表示蓝绿切流和旧连接排空，不表示无限期连接保留；当前代理对长连接保留上限为 24 小时。
