# 官方 2FA、凭证守护与重登恢复计划

用户要求：删除上述模块的本地业务改造，以 ranxi2001/sub2api production 为准；SSH 核实 ai8 缓存计费。
官方来源：4b5381fcbc138b1698b66a792c1e4a58e779322a（2026-09-29 实时 fetch）。

- [x] 从 origin/main 创建独立 codex/official-account-guard 分支。
- [x] SSH 只读核实 ai8-plus 设置、用量与计费。
- [x] 逐字恢复官方业务文件、前端和测试；删除本地 native、安全恢复、邮件及 Redis 守护运行态实现。
- [x] 检查共享调用点，保留站点通用 API/worker 单例启动边界，移除本地业务依赖。
- [x] 执行官方守护/2FA 测试、API Key 缓存测试、前端相关测试与类型检查、服务构建。
- [x] 核对官方文件一致性、记录变更与上线配置需求。

授权更新：用户明确要求不停机部署主站，包含合并、推送、蓝绿发布、官方配置衔接和线上验证；不改历史账单。已经执行的数据库迁移保留为不可变账本，不删除表或修改历史迁移校验和；不再有运行代码使用本地 ownership 表。主站当前旧配置 probe/relogin 地址为空，恢复官方代码后仍须在部署时迁移为官方接口配置；不得声称代码恢复等于真实导入已验证。

## 验证结果

- 官方 28 个独立业务/前端/测试/文档文件逐字一致；共享 CreateAccountModal 的 importTwoFACredential 回调逐字一致。
- 删除 12 个本地专用文件；共享注入、路由与运行态测试中的本地守护分支同步移除。原有 OAuth 重授权实现与官方一致，无需另改。
- 保留的适配仅为 service/wire.go 中站点通用 shouldStartSingleton(cfg) 启动边界，避免 API/worker 重复巡检；未保留自定义探活、重登、预绑定或通知业务。
- go test -tags unit ./internal/service ./internal/repository ./internal/handler/admin -run 'Test(AccountTokenGuard|TokenGuard|OpenAITwoFA|TwoFA|APIKeyCacheCreationAsInput|HarvestWorkerRuntimeVisibleOnAPI)' -count=1：通过。repository/handler 在该过滤条件下无用例，不算功能覆盖。
- go build ./cmd/server：通过。
- pnpm exec vue-tsc --noEmit：通过。
- 官方前端 accountTokenGuard API 9 项、OpenAITwoFAImport 4 项、TokenGuardV2View 7 项、i18n 3 项：通过。
- CreateAccountModal 的 2FA 首次导入及已导入身份重试注册两项：通过。
- CreateAccountModal 整文件：34 项通过、9 项失败；在根目录未修改 main 上复核，同样 9 项失败（33 项通过，少一项本次引入的官方重试测试）。失败属于既有模型元数据同步及 OpenCode/Kimi/MiniMax 协议预期，不扩大本任务修改范围。
- git diff --check：通过。
- 未执行：真实第三方 2FA 登录、数据库集成测试、主站部署和线上新请求缓存验证。

## SSH 核查 ai8-plus

2026-09-29 主站 64.83.10.67 只读查询：账号 158，为 OpenAI API Key；创建缓存按输入开关为 true。最新用量仍是 2026-09-28 14:33:20（北京时间），没有更新后可用于线上验证的新请求。

截图四笔创建缓存记录对应 432380、432382、432388、432391。创建 token 合计 330,973，四笔实际总扣费 0.1616504220 美元。按照各笔已记录的普通输入单价、创建单价及 0.18 倍率重算，若创建部分应按普通输入计费，差额合计 0.0297875700 美元。这是条件重算结果，不是已退款或已证明开关当时状态的结论。账号 updated_at 不足以证明开关启用时间。

## 上线前配置衔接

主站保存的 V1 守护配置为明文 JSON，relogin_accounts 数量为 0，probe_endpoint/relogin_endpoint 为空。官方代码不会自动覆盖数据库中已保存的空字段。上线时需备份现有设置，并通过官方配置恢复官方探活/重登接口及请求头，再验证真实登录。当前未更改该配置、未发送真实登录凭据、未删除生产数据或调整历史扣费。

本次没有数据库结构变更；历史迁移 250/254/255 保持原有字节，已存在的本地 ownership 表不再被运行代码读取。历史表仅留作旧版回滚与迁移账本，不作为新的业务实现。

## API Key 缓存按钮修复
API Key 使用独立 openai_apikey_cache_creation_as_input，显式 false 优先，未迁移账号回退旧键；创建和编辑弹窗保存独立开关，并镜像旧键兼容回滚。
新增红绿测试验证独立设置、旧键兼容、关闭优先、保存、余额/订阅/流式计费；定向 Go 测试和 vue-tsc 通过，前端 cache/2FA/operations enrollment 8 项通过。
待完成：合并推送、主站蓝绿发布、官方配置衔接和线上核实。
