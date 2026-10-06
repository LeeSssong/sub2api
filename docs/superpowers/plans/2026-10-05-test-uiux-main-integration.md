# 测试站 UI/UX 与主站功能整合 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development to implement sequentially with one writer. Do not deploy or update shared main.

**Goal:** 保留测试站现有视觉和交互，接入主站全部业务能力，提供可核验代码备份和恢复入口。

**Architecture:** origin/main 为基线，三方合并测试站来源；展示层按测试站保护，业务与数据契约逐项适配主站。数据库和运行环境独立，不复制生产数据。

**Tech Stack:** Go / PostgreSQL / Vue / TypeScript / Vitest / pnpm。

**Spec:** docs/superpowers/specs/2026-10-05-test-uiux-main-integration.md

## Global Constraints
- 测试站视觉和现有交互优先，主站核心业务优先；禁止整个前端或后端无差别覆盖。
- 不复活主站已退休调度器，不伪造日志或监控数据。
- 不改已应用迁移校验和，不访问或复制生产数据库和凭据。
- 一次只有一个 worktree 写入者；不部署、不推送共享 main。
- 新增可见入口单独提供可视对比并获用户确认后实现。
- 备份分支固定为 backup/pre-main-integration-20261005-96c0818277；恢复提交必须具有 tree 9924ce15d50a43525b0c726b1b7f22663cd10afd。

### Task 1: 合并后端与前端契约并保留测试站页面

**Files:**
- Resolve: `git diff --name-only --diff-filter=U` 的 37 个冲突文件，当前 merge 已在独立 worktree 开始，双方 SHA 见规格。
- Inspect merged: upstream/sub2api/backend/internal/{repository,service,server,web}/ 与 frontend/src/{router,components,views,features,api,types}/ 中双方修改文件。
- Test: 对应已有 *_test.go 和 *.spec.ts；新增仅针对实际发现的回归。

**Interfaces:** 保留主站全部 API 与鉴权，中间兼容层保留测试站 groups/available-models、monitor-v4/timeline、monitor-v4/check 和 group tool mappings。页面模板/样式以测试站为基准，适配主站数据类型与逻辑。

- [ ] 阅读 frontend/AGENTS.md；记录每个冲突的取舍及自动合并中的语义风险。
- [ ] 在修改业务逻辑前先运行直接相关已有测试，记录失败；补充揭示真实问题的最小回归测试。
- [ ] 解决后端 5 个文本冲突；核查 current_operational 与主站查询/快照服务兼容、P50、分组检查范围、权限和支付方式逻辑。
- [ ] 解决前端文本冲突；核查测试站样式与组件逻辑，保留主站新增页面/路由，暂不增加未经视觉确认的导航入口。
- [ ] 对旧调度日志给出真实可行的兼容处理；若不可等价保留，保留证据并明确上报，不能用空壳通过验收。
- [ ] 运行 pnpm typecheck、直接相关 Vitest、pnpm build；Go 对受影响服务/路由/仓库/迁移运行相关测试并编译入口。环境依赖失败先诊断，不降低门禁。
- [ ] 记录未验证项、实际命令和结果；无冲突标记且候选可编译后提交合并候选，不推送共享 main。

### Task 2: 代码回退工具

**Files:** Create ops/restore-test-station-uiux-main.sh; tests/operations/restore_test_station_uiux_main_test.sh。

**Interfaces:** 默认只读 dry run；--apply-code 显式执行。在临时 worktree/临时 index 创建以最新目标 main 为 parent、备份 tree 为内容的恢复提交，普通 push fast-forward；拒绝错误仓库、备份 SHA/tree 不匹配和并发远端变更。不执行部署或数据库操作，不把恢复脚本自删除视为失败（脚本保存在候选版本，执行中已读入）。

- [ ] 用临时 bare 仓库测试备份校验、dry run 不写远端、恢复后 tree 一致、并发更新导致安全拒绝。
- [ ] 实现脚本并运行上述测试，文档给出一条命令用法及数据库恢复边界。
- [ ] 提交工具和验证结果。

### Task 3: 候选核验与视觉确认

**Files:** docs/superpowers/reports/2026-10-05-test-uiux-main-integration.md（开发记录，非正式发布证据）。

- [ ] 审查相对两边的差异及功能映射，复用 Task 1/2 测试，不为换任务重复跑。
- [ ] 对测试站现有页面进行实际截图和交互核验；若认证/环境无法建立，明确具体阻碍。
- [ ] 主站新增导航入口提供可视对比并请求确认；没有确认不擅自改变测试站导航。
- [ ] 核验备份远端仍匹配、候选 head/tree、剩余风险和数据库迁移路径；不部署。

2026-10-05 用户追加明确指令：去掉调度日志页。删除旧调度日志页面、路由、导航和 API 依赖，保留主站原生调度。此指令覆盖旧页面保留要求；不实施历史归档 UI。主站既有 240_remove_custom_scheduler_artifacts.sql 保持不可变，未来部署执行前须确认历史日志删除范围与数据备份恢复边界。本次不部署。
