# 质量运维：原生模型移出与恢复候选

- 状态：实现及直接相关本地验证通过；未提交、未推送、未部署，不标记线上 DONE。
- 分支：`codex/quality-model-removal`。
- 基线：`1cbef989ed9c9e3c953b02e548586f5d7f79985e`（创建时最新 `origin/main`）。
- 工作区：`/Users/gongtengxinwen/.codex/worktrees/quality-model-removal/sub2api搭建`。
- 根目录 main 保持干净；未访问主站、备用站或测试站。

## 行为与复用

质量规则新增 `remove_models` 动作和 `remove_model_ids` 多选，最多 100 个明确模型 ID。明确答错时，从账户原生 `credentials.model_mapping` 原子移出所选条目；其他模型、认证字段、分组和调度开关不变。沿用原来的判题、整轮判定、租约、事务、操作记录及 scheduler outbox。

恢复记录仍保存在原有 `account_quality_states.state` JSON 内，只新增实际移出条目的原映射；没有新增表、迁移或封控名单。整轮通过且自动恢复开启时，只补回该规则实际移出的条目。无关模型修改被保留；已被人工重新配置的目标条目引发恢复冲突，绝不覆盖。账户级动作原有恢复保护保留。

为避免别名移出后无法探测原上游模型，下次领取探测任务时，从已有恢复记录加载模型映射，仅在这次直接探测的账户副本中应用。该字段不接受/输出 API JSON，不写回账户，不影响业务调度。

网关原有模型支持函数提取为可复用函数，原调度入口直接委托调用，判断分支不变。移出前复用该判断确认模型已不再受支持，覆盖 Bedrock 默认回退、Anthropic 别名归一化和 Antigravity 前缀处理。

## 边界

- 不支持无限制列表、通配符、自动透传或移出后模型列表为空。
- 平台默认映射/别名仍然允许目标模型时，整次动作拒绝并记录 `model_removal_blocked`，不部分修改。
- 批量配置显示所选账户共同的明确模型；编辑已移出的规则时仍可见原选择。
- 仍是一条规则的检测结果控制指定的一组模型，不是各模型独立检测/恢复。
- 原生调度缓存通过现有 outbox 更新；不新增缓存或调度流程。

## 已通过的验证

后端目录 `upstream/sub2api/backend`：

```sh
GOMAXPROCS=4 go test -p 2 ./internal/service ./internal/repository -run '^(TestQuality|TestPelican|TestGatewayServiceIsModelSupportedByAccount)' -count=1
DOCKER_HOST=unix:///Users/gongtengxinwen/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock SUB2API_TEST_POSTGRES_IMAGE=postgres:15-alpine SUB2API_TEST_POSTGRES_TMPFS=1 GOMAXPROCS=4 go test -p 2 -tags integration ./internal/repository -run '^(TestQuality|TestPelican.*Claim)' -count=1
```

数据库测试覆盖多模型移出/原映射恢复、原生模型支持状态、认证与调度保持、重复动作、原生配置边界、无关修改保留、人工目标冲突、调度事件以及后续探测租约加载。单测验证别名探测且账户原对象不被恢复，及三类平台默认回退拒绝。既有账号级质量测试通过。

前端目录 `upstream/sub2api/frontend`：

```sh
node node_modules/vitest/vitest.mjs run src/views/admin/__tests__/AccountQualityView.spec.ts src/stores/__tests__/accountQuality.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts
node node_modules/vue-tsc/bin/vue-tsc.js --noEmit
NODE_PATH="$PWD/node_modules/.pnpm/node_modules" node node_modules/eslint/bin/eslint.js src/views/admin/AccountQualityView.vue src/i18n/locales/zh/qualityOps.ts src/i18n/locales/en/qualityOps.ts
```

16 项前端测试、类型检查、上述定向 ESLint 和 `git diff --check` 均通过。只读审查发现的三类原生解析回退已修复并补回归测试。

本机 Colima 虚拟磁盘已满，默认 PostgreSQL 测试容器无法初始化；使用现成的 PostgreSQL 15 + tmpfs 测试选项后数据库测试通过，未清理其他任务容器。复用根目录已有前端依赖；验证后移除本任务创建的 node_modules 符号链接。未新增依赖。现有 Node localStorage/Browserslist 提示不影响测试通过。

未验证：完整生产镜像构建、真实上游在线检测和部署后用户流程。发布需后续授权及按项目 SOP 执行；本候选无数据库结构变更。
