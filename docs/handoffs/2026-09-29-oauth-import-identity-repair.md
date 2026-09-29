# OAuth 导入身份与 2FA 错绑修复

## 候选代码

- 工作分支：codex/oauth-import-identity；基线：7bab5c3913eb8023a7de88994d89ee7e539ec6d1。
- 用户授权实施已诊断的导入身份/2FA 错绑修复；本次未获应用发布授权，未合并、推送或部署。
- 导入索引从存量 token 补取缺失的用户身份；过期 token 仍可用于身份比较，不用于授权，不修改索引来源对象。
- 2FA 使用的 skip_existing 路径不再仅凭共享 workspace 在身份不明时认定同一用户；通用旧导入兼容行为与 Agent Identity 语义保留。
- 支持 access-token profile 命名空间中的 email。保存重登配置拒绝邮箱不符/无法确认身份的写入；回写 OAuth 凭据时也校验旧 token 中的用户身份；对历史配置或已在途任务，仅有 workspace 而不能确认成员身份时拒绝回写。

## 已运行的相关验证

新增测试先复现了不同成员误跳过、profile 邮箱漏读、错邮箱配置覆盖、旧 token 身份未用于重登回写检查，以及历史配置仅凭 workspace 通过回写的问题。

通过：

- go test ./internal/handler/admin -run 'Test.*Codex.*|Test.*AgentIdentity.*' -count=1
- go test -tags unit ./internal/service -run 'TestOpenAIOAuthReauth|TestValidateReauthToken|TestAccountTokenGuardV2' -count=1
- go test ./internal/pkg/openai -count=1
- git diff --check

service 包原本不带 unit tag 时存在 rawChatCompletionsTestAccount 未定义的测试编译问题；本次按其相关测试的 unit tag 运行，没有修改无关测试。未进行应用构建、前端交互测试或修复版本线上导入验收。

## 已完成的主站定点数据纠正

- 目标：sub2api-prod / 64.83.10.67。
- 排查期间原 #445 被删除重建；实时确认 #447 为同一用户的有效账号，#444 为同 workspace 的不同成员，#445 已软删除。
- 2026-09-29 23:21:05 +08：短事务内补齐 #444/#447 的 email 和 chatgpt_user_id；将已成功重登验证过的 #445 配置迁至 #447；移除 #444 的错误配置和错误 guard 关联；启用 #447 的原生 2FA guard。#444 的业务账号仍在，若需其自动重登须另行配置它本人的 2FA。
- 未恢复已删除账号，未更改 OAuth token、账号调度/分组或计费数据；原加密密码/TOTP 原样保留。
- 执行前进行了身份校验和回滚式演练；执行时使用行锁、配置表锁、短超时、源快照比对、目标不存在与无在途任务检查，防止覆盖并发修改。
- 受保护的服务器备份：/root/sub2api-repair-backups/oauth-identity-20260929T152105Z.json（0600，仅服务器保存，不进 Git）。SHA-256：f39e64b21c7f9d5c35481a1d926bff1cd5425e050d73d4b30e66b185db2659bb。
- 恢复入口为以上快照；如需恢复须先核对修复后变更，不得直接用旧快照覆盖新的用户操作。
- 23:21:07 +08：#447 原生 guard probe_state=ok；23:21:26 查询时 #447 已有 52 条调用、12,300 output tokens，无错误记录。
- 测试站未查询、未同步。数据已纠正不等于预防复发的代码已上线。
