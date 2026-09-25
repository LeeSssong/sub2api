# 账号准入功能本地验证记录

## 范围与来源
- 本地候选分支：codex/account-import-admission。未推送、未部署、未访问生产或测试站，未发送真实 OAuth / 上游模型请求。
- 最新获取的远端基线：50c1376e754bf81517b7c1e4fadd9073decf16c6。承接根目录本地 main 的既有 18 个依赖提交至 189d2efb8aae55909ae2bbb1efaf0ab4c5748863；这些依赖不属于本功能新增内容。
- 规格与计划：同目录上级 specs/2026-09-26-account-import-admission-design.md、plans/2026-09-26-account-import-admission.md。

## 用户可见行为
- 授权第一步仅新增“导入前降智检测”复选框与勾选后必选的“临时检测分组”。原生“分组”就是通过后的分组。第二步、页数和原按钮保留。
- JSON 导入默认不启用检测；启用后检测组默认“未分配”，要求选择有效的通过后分组。
- 后台立即排队执行原生糖果与鹈鹕检测各一次，同轮双通过才加入正式组。未通过账号保持隔离，不删除账号。每 10 分钟复检；传输故障不当作降智。
- 人工分组修改使当前自动结果失效；明确使用原生管理员调度开关可接管账号，普通自动恢复不能绕过待检门禁。

## 前端验证
- 实现提交 6d517400024997f944fa0bc0d1af970038c9a9b3，集成为 37aaaf4715；直接相关 6 个 Vitest 文件 45/45 通过，vue-tsc --noEmit 通过，相关独立文件 ESLint 通过。
- 新增 admission 测试共 32 项，覆盖原生授权各入口、JSON、关闭开关旧请求、平台匹配与重合、必填校验、授权码消费前校验。
- JSON 启用后原有“分组需手工绑定”提示调整为自动准入提示（9b4e3a6320）；仅重跑受影响 import + locale completeness 两文件，10/10 通过。
- 复测命令（frontend 目录）：node node_modules/vitest/vitest.mjs run src/components/admin/account/__tests__/ImportDataModal.admission.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts。
- 既有 CreateAccountModal.spec.ts 在修改前后均为 9 失败 / 32 通过：metadata sync/preview 4 项，OpenCode/Kimi/MiniMax endpoint 5 项。本次不扩修无关缺陷，不宣称该全文件全绿。

## 实际界面验证
使用真实 Vue 组件与本地 Vite browser-test 模式；Axios adapter 只返回演示数据。Playwright 独立浏览器：
1. 默认关闭检测，仅新增一个复选框。
2. 勾选但没有检测组时，不能进入授权第二步，auth-url/exchange 请求数为 0。
3. 选好检测组及原生正式组后，进入未改动的 Claude 授权第二步（生成 URL、粘贴授权码、返回/完成授权）。
4. JSON 勾选后检测组为“未分配”，展示原生通过后分组选择。
5. 校验后关闭专用浏览器与本地服务，移除临时 node_modules 软链接。

演示截图（本地忽略制品）：
- upstream/sub2api/frontend/output/playwright/authorization-admission-controls.png
- upstream/sub2api/frontend/output/playwright/json-admission-controls.png

## 后端验证
- 后端实现提交：6c433b25a9ceae9ae78a40f56206549d8aaa9b95；集成代码提交：76c0b1f2e89bec40def77115dd01c588f4b8c92c。
- 集成后对整个 backend 子树执行 git diff --exit-code，结果为 0；无冲突、代码与已测候选完全一致，因此复用有效证据。
- Service 定向测试 1.663s、handler 定向测试 1.378s、真实 PostgreSQL 准入集成套件 4.300s、原生 repository 相关 unit 测试 3.818s 均通过。Production build、Wire 生成、git diff --check 均退出 0。
- PostgreSQL 测试实际执行全部 migration，覆盖事务内初始隔离、双通过转组、结果不明保留原状态、租约接管/旧结果丢弃、人工分组/暂停/接管、重复账号保护、正常 token 刷新与人工凭据更新、空绑定缓存通知、成员锁竞争、旧对象回写保护。
- 独立审查发现并修复：membership/account 锁序、空绑定不触发取消、expires_in 刷新误失效；根代理补查并修复人工接管永久阻断与旧对象撤销晋级。对应实际 PostgreSQL 回归已通过。
- 完整 service unit 包有既有编译问题（重复 ptrFloat、旧 resolve 函数签名、缺失 context 导入）。本次未扩修；新增 service/handler 测试使用生产 GoFiles 加指定测试文件运行，未宣称全包 unit 全绿。
- 未单独生成测试日志文件；原始结果保存在实现任务的工具输出。

复现命令（在候选 worktree 的 upstream/sub2api/backend 起步，Go 自动使用模块指定工具链；需要本地 Docker）：
```bash
backend_dir="$PWD"
cd "$backend_dir/internal/service"
go test $(go list -f '{{join .GoFiles " "}}') account_admission_test.go -run '^TestAdmission' -count=1
cd "$backend_dir/internal/handler/admin"
go test $(go list -f '{{join .GoFiles " "}}') account_admission_test.go -run '^TestAdmission' -count=1
export DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock
cd "$backend_dir/internal/repository"
go test $(go list -f '{{join .GoFiles " "}}') integration_harness_test.go account_admission_integration_test.go -run '^TestAdmission' -count=1
cd "$backend_dir"
go test -tags unit ./internal/repository -run 'Test(LockAndMerge|AccountRepo|AccountRepository)' -count=1
go build ./cmd/server ./internal/service ./internal/repository ./internal/handler/admin
go run github.com/google/wire/cmd/wire ./cmd/server
git diff --check
```


## 部署边界
包含数据库 migration 255。当前仅本地开发候选；主站/测试站未部署，实际授权与模型调用、线上运行状态未验证。
