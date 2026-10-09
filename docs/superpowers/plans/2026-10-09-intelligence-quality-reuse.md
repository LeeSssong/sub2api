# 智商监测复用质量运维 Implementation Plan

**Goal:** 每组一个逻辑题区域包含 gpt-6-astra/gpt-6.1-sol 两行；绘图每半小时一次，画作位于绘图区域右侧。糖果只复用质量运维，发布主站不停机。
**Architecture:** 复用 quality_rule_templates → quality_rule_template_accounts → scheduled_test_results，按真实测试分组及模型筛选。用现有 pelican_group_test_results 和 JSON 配置保存展示快照，不新增 SQL 迁移，不重复请求、判题或计费。
**Tech Stack:** Go, PostgreSQL, Vue/TypeScript, Vitest。
**Spec:** 本会话用户确认的方案以及最后的两区域布局修订；新增官方 ranxi2001/sub2api production 最新代码更新一起发布。

## Global Constraints
- 保持现有风格；逻辑题公共标题/来源/图例只出现一次，模型各有紧凑指标与时间线。绘图区右侧仅显示绘图预览。
- 糖果按每30分钟展示时段，取该时刻最近一个已结束的质量规则时段；每模型等概率抽账号，再在账号最近完成轮次抽一条；抽样固定，缺失不补测、不无限回退旧记录。
- 原判定含正确、错误、异常、未知；未执行/skipped 不作答题样本。不混用模型、不重新判题。
- 记录来源规则/结果ID/原时间，详情脱敏并遵守既有分组可见性；历史源数据保留不足时不伪造。
- 线上已有分组2→质量模板10（*/2），分组6→模板9（*/5），均含两个模型；只读证据，运行时按当前真实配置确认。
- 无数据库迁移优先；官方若包含迁移须单独评估能否满足用户本次明确不停机要求。
- 每worktree一个写入者。主线程唯一发布写入者，主站64.83.10.67。保留根main已有未追踪文件。

## Interface contract
- IntelligenceRuleConfig 新增 `quality_template_id: number`、`candy_models: string[]`，每组各自绑定。旧 Candy 字段兼容读取，但执行不再发送糖果请求。
- IntelligenceRuleInput 新增 `quality_sources: {group_id:number,template_id:number}[]`、`candy_models:string[]`。旧 candy_prompt/expected_answer/judge 可选兼容，当前表单不再写入。
- IntelligenceGroup 新增 `candy_model_ids:string[]`、`quality_template_id?:number`、`quality_source_status?:string`；仍一个绘图模型卡片，所有糖果模型归入相同分组卡片，不分裂卡片。
- IntelligenceResult 继续使用现有 id，额外 `source?:"quality_ops"`、`source_result_id?:number`、`source_template_id?:number`、`source_started_at?:string`、`source_finished_at?:string`。started_at 为展示时段，原时间单独保留；id 全局唯一仍使用原公共序列。无样本则不伪造结果。
- 默认两个 candy_models: ["gpt-6-astra","gpt-6.1-sol"]。后台可选择规则来源，歧义时要求指定，唯一候选可自动匹配旧规则。

### Task 1: backend（独立worktree intelligence-legacy-dedup，唯一写入者backend agent）
- [ ] 直接相关测试先复现：糖果不调用上游、模型隔离、稳定快照、源规则分组隔离、失败保留、时段/过期、详情权限、原保留策略清理后仍可读。
- [ ] 实现来源验证和最近完整时段查询，抽样+快照复用原结果存储；重启/刷新不重复抽；租约仍只允许每计划一个执行者。
- [ ] 调整dashboard归组、原结果来源DTO及配置验证；绘图cron固定半小时（兼容已有半小时规则）。源缺失不能阻塞绘图。
- [ ] 测试使用 `GOMAXPROCS=2 GOMEMLIMIT=2GiB go test -p 1 -tags unit ./internal/service -run 'Intelligence|PelicanGroup' -count=1` 和真实Postgres相关集成测试；Colima DOCKER_HOST 与 TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock，勿使用旧TMPFS选项。
- [ ] 提交后返回SHA、具体测试、兼容/线上配置注意事项。不推送/部署。

### Task 2: frontend（独立worktree intelligence-quality-ui，唯一写入者frontend agent）
- [ ] 修改API类型、后台规则表单和默认值：复用现有 account-quality-templates API，按所选分组选择来源，展示两个模型，移除重复糖果编辑。
- [ ] IntelligenceGroupCard 改为上方逻辑题区域（两模型紧凑行）、下方绘图左右布局。模型与推理强度按各结果自身显示，卡片不再误标全卡单模型。
- [ ] timeline 可复用紧凑模式，统计按展示样本；总糖果通过率合并两模型；状态齐全/时效正确；详情展示来源与原时间。
- [ ] Vitest覆盖两模型不混用、布局归属、缺失/过期状态、表单payload、详情；类型与生产构建；浏览器桌面/手机布局检查。
- [ ] 提交后返回SHA和验证。不得操作backend、生产或推送。

### Task 3: official update + integration（官方独立worktree，主线程整合与部署）
- [ ] 官方production锁定当前remote SHA，按既有引入来源做三方内容合并，保留本地定制，不机械并入外部历史；迁移逐项分类。
- [ ] 合并前检查与本次文件冲突，补受影响测试；一次最终审查，处理明确问题，不重复全量审查。
- [ ] 核实来源main干净且推送一致，沿用本地主机发布脚本与已有制品链；新槽就绪/内部冒烟后切流。
- [ ] 旧API连接自然排空，使用retain模式保留超时SSE而不是强杀；worker按调度租约排空安全替换；无关服务保留。
- [ ] 公网验证双糖果和绘图定位、版本、健康、源结果关联、不重复糖果请求，保留一份简短发布记录和回滚入口。

## Progress
- 已提交旧重复计划修复975e42ea8f并合并最新origin/main到后端工作区，根main未动。
- 主站旧计划1/2已暂停；现有质量模板9/10已核实；运行API与worker版本不同，发布前须核验迁移兼容。
