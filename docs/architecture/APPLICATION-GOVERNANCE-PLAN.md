# Application Governance — 可执行落地方案

> Document class: **DECISION**  
> Authority: 本项工作的目标、任务契约、依赖顺序和验收政策；不是已实现功能清单  
> Current status authority: [`../STATUS.md`](../STATUS.md)；未合并任务的精确进度与验证结果见对应 PR  
> Task prefix: **AG**（应用代码组织治理，不是新的 Runtime wave）  
> Decision date: 2026-09-07

## 1. 目标和范围

将“封闭 Application → 按用例收窄依赖 → 复用真实窄能力包装 → 精确检查 → 模块级与连续演化验证”变成可执行工作。先用真实小闭环证明，再推广到 Biz、IoT Delivery 和业务模板。每轮至少实施一个独立任务；不能只重复规划，也不能把未验证实现称为完成。

允许在 `hvritual/biz`、`hvritual/iot-delivery-system` 大范围重构手写应用。默认保持公开 API/Operation ID、认证授权、租户隔离、CAS、根事务/UoW、幂等、事件和数据语义。生产部署、正式数据迁移、破坏性清理不在本计划授权内。这里只归档用户明确要求的治理方案，不附带发布历史内部审计资料或整库消费者源码。

Yunka 的 `core.App`、Executor、ExecutionScope 和生成链仍只有一套。复用 context/ownership/ChangeSet/audit/DebtDelta/Attestation；不新增 LLM 运行依赖、反射 DI、通用领域推理器或另一份手工架构图。

## 2. 与已有材料和并行工作的关系

本计划取代讨论中的“大而全规则先行”实施顺序，不取消 v0.1 的五层原则、正反例、覆盖、反自证和结构迁移要求。历史附件中的 17 条规则和 42 个测试定义不是已部署机制；先挑选可证明案例实现，不按数量宣称完成。

原 `yunka-boundary-poc.zip` 只在 Go 1.23.2 上运行过六个独立机制实验，不是框架/消费者资格验证。AG-01 将相关机制转为仓库内、锁定工具链的永久回归；这些案例不是用于扫描任意项目的检查器。

启动时核对的 main：Yunka `7d7fc663f7086df9735ee9089c75f65d2fd100e4`；历史消费者案例 Biz `7d5afbe9cb4b849be462d9a7aed65877ed227700`、IoT Delivery `f190fe237fa1efb738b4866275804093a209de5b`。它们只是取证点；每个任务重新获取 main、任务分支、框架锁、工作区/子模块状态。

Service Boundary #161 已有并行 PR 链 #166 → #167 → #168 → #169；启动时未合并。AG 不重复其 Operation 创建证明，不依据旧对话宣称这部分尚无人实施。包封装试点不等待全部语义治理；涉及 ChangeSet schema/边界声明的任务必须先核对其集成状态并协调。普通结构迁移不得偷偷绕过 #160 来源门禁。未批准/未实现的命令参数不得写入产品帮助。

## 3. 架构决策

### 3.1 工具链能直接限制的结构优先

仓库级 `internal` 只隔离外部导入者。需要兄弟应用隔离时采用应用级嵌套 `internal`，隐藏实现只由所有者包构造。生成端口暂留规范生成位置；手写实现不与生成 wrapper 共享包级私有范围。

候选布局（仅为待资格验证目标，不授权移动生成文件）：

```text
internal/access/application/zz_yunka_*_gen.go
internal/access/tenantlifecycle/build.go
internal/access/tenantlifecycle/internal/usecase/
internal/access/tenantlifecycle/internal/persistence/
internal/access/tenantmember/build.go
internal/access/tenantmember/internal/usecase/
internal/access/domain/
internal/assembly/
```

全局 Assembly 通过所有者工厂装配。工厂是 composition-only 入口；业务代码不得自行调用别的应用工厂。所有者包不反向导入全局 Assembly，防止循环。共享领域规则必须确有共同语义，不强制一实体一包、一 RPC 一包、一应用一数据库或一应用一进程。

### 3.2 能力对象与接口声明同时收窄

小接口指向大实现仍可能经类型断言扩权；公共工厂返回 internal 类型、匿名嵌入或公开解包方法也可能泄露能力。复用现有 source-edge child wrapper 的独立类型、私有非嵌入字段、显式方法集合和 ExecuteChildTyped。不得以新增一层空接口转发替代真正的能力收窄。

封装是工程约束，不是对任意恶意同进程代码的沙箱；unsafe、反射攻击、任意运行时代码执行不在这批静态保证内。发现未知动态路径必须明确分析不完整，不伪造“已全部覆盖”。

### 3.3 封闭应用内部仍按用例组织依赖

按真实用例定义依赖和端口，避免同一个 Service 持有全量 Repository。可保留满足 PB 接口的薄门面，但门面只委派，不重新持有所有数据库能力。Application 端口允许使用 PB；领域规则/模型与具体 SQL/HTTP/PB 实现解耦。不要求所有值对象或配置都接口化。

### 3.4 五层规则

| 层次 | 约束 | 判定边界 |
| --- | --- | --- |
| L1 业务归属 | Operation 所属 Application 和增长决策有效 | 复用 #161；未知业务语义需要决策，不假装静态证明 |
| L2 包与分层 | 生产包完整清点、分层、应用级隐藏实现、测试资产隔离 | 工具链 + 一套通用依赖检查政策 |
| L3 类型与能力 | 工厂仅供装配、不可泄露原始实现、跨应用调用走子能力 | 类型身份和构造证据，不按变量名猜测 |
| L4 变更与迁移 | 已允许文件也不能新增越界职责；迁移限定 base/path/type | 复用 ChangeSet/Ownership，精确扩展迁移协议 |
| L5 演化与交付 | 不新增/扩大已证明债务，必需证据不能跳过，规则不能自我放宽 | 同一引擎政策比较 base/current，受信任集成回读 |

文件行数、RPC 数、依赖数量只是观察/审查触发，不是业务正确性规则。结果区分 PROVEN_VIOLATION、OBSERVATION、REVIEW_REQUIRED、INCOMPLETE。历史案例不自动等于过去违反当时规范。

## 4. GitHub 机制采用政策

| 来源 | 采用内容 | 明确不采用 |
| --- | --- | --- |
| Spring Modulith | 模块公开 API、隐藏实现、允许依赖、模块级测试 | Spring 运行时、所有跨模块调用改异步 |
| Go internal / 类型系统 | 导入封装、实际方法集合、类型身份 | 把包封装宣传成安全沙箱 |
| ThreeDotsLabs/wild-workouts-go-ddd-example | 窄端口、独立用例、渐进重构 | 整套 CQRS、云设施、替换 Yunka 事务 |
| fe3dback/go-arch-lint | 通用包依赖检查候选 | 将 deepScan 视为完整跨函数数据流分析 |
| ArchUnit FreezingArchRule | 历史债差量、架构测试思想 | 新建可任意重置的架构债真相文件 |
| Go analysistest / flyingmutant/rapid | 预期诊断、属性与演化序列测试 | 仅非零退出就记为成功检出 |

先用标准库实现 AG-01，不引入任何外部工具。后续工具入锁前登记精确版本/commit、许可证、校验和、Go/OS/构建矩阵、正反例验证；不使用 @latest。只有一个依赖政策权威，禁止多个 lint 各自维护矛盾规则。

核对来源（机制参考，不宣称已在 Yunka 获得资格）：

- https://pkg.go.dev/cmd/go#hdr-Internal_Directories
- https://go.dev/ref/spec#Method_sets
- https://go.dev/ref/spec#Type_assertions
- https://github.com/spring-projects/spring-modulith
- https://docs.spring.io/spring-modulith/reference/verification.html
- https://docs.spring.io/spring-modulith/reference/testing.html
- https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example
- https://github.com/fe3dback/go-arch-lint
- https://github.com/TNG/ArchUnit
- https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest
- https://github.com/flyingmutant/rapid

## 5. 独立任务及依赖

这里保存任务定义；执行状态只在 docs/STATUS.md 与精确 PR/验收证据中记录，不维护第二份活动状态表。

### AG-00 — 归档方案（文档任务）

输入：本次用户决策、当前治理文档、先前方案和 POC。输出：本文件、任务顺序、首轮范围与后续交接入口。验收：文档实际保存到任务分支并回读；本任务独立提交，先于实现提交。禁止把文档落盘当成实现任务。回滚：单独 revert 文档提交，保留其他任务。

### AG-01 — 锁定工具链的封装边界回归基座（首个实现任务）

允许：`pkg/architecturepolicy/ag_boundary*_test.go` 及本计划对应的证据记录。输出：可执行 Go 回归，不是 JSON 用例定义。复用现有 architecture-check/test 入口，不新增模块/依赖、普通 CI 或运行时功能。

最少覆盖：根 internal 的限制、嵌套 internal 的拒绝、所有者合法构造、小接口指向宽对象、显式包装、具体返回值泄露；增加同包私有字段访问与跨包拒绝、嵌入导致方法提升、变更别名。预期拒绝必须校验目标错误原因；解析失败/编译器故障不能冒充预期反例。先跑合法控制组。子进程仅处理临时目录，禁网、禁工作区继承、关闭测试缓存、设置超时；源码树只读。

正式验收：实际 Go 必须与 tools/toolchain.env 一致；定向回归无 skip、预期诊断准确；同一源码重复运行一致；make architecture-check 与 make verify（进入完整集成时仍遵守原 verify-production 门槛）；精确提交/树/结果回读。工具缺失为 INCOMPLETE，不降版本宣布合格。非锁版本的开发试运行只能单列为探索证据。回滚：删除/恢复本任务测试文件，不影响运行时。

### AG-01I — 回归完整性与主线集成（AG-02 前置收口）

输入：AG-01 已验证分支。目标：避免减少/重复/替换机制案例后仍报告原覆盖；补齐状态和持久决策入口，按用户授权将框架变动验证后同步 main。

允许：AG 边界测试、当前状态、持久决策、本计划；禁止普通 CI/工具链锁/Runtime/Executor/Authz/UoW/消费者语义变更。明确用例清单及正反例类别；测试删除、替换、缺少入口、覆盖 go.mod、路径逃逸等错误输入。通过变异副本证明旧 harness 接受减少后的案例，新 harness 对同样变异给出准确的 inventory 错误。完整候选运行 locked Go 定向 JSON 测试、原 make verify-production、生成/依赖再生成和 clean-tree 检查。

正式新增提交由本地 Git 或 Runner-local Git 创建，Connector 只用于隔离 control 分支暂存、读回和 PR 管理。main 仅在资格验证和实际审查处置后通过 non-force fast-forward 更新；并行 main 有变化就停下重新整合，不覆盖它。独立 review 的实际结果与测试资格分列，不伪造 APPROVE。每个完成的框架任务最终同步 main，不把已验证 Draft 当作最终交付。回滚：独立 revert 本任务提交；不改数据。

AG-01I 审查收口还包括进程树清理：Unix 命令使用独立进程组，取消和父进程退出后都清理后代；Linux 回归覆盖超时及父进程先退出但管道仍被后代持有。非 Unix 尚无合格 backend 时明确 INCOMPLETE，不静默退化为仅杀直接进程。详见 [AG-01I 验收记录](../waves/AG-01I-boundary-qualification.md)。

AG-01I.2 固定用例定义绑定：每个清单项除名称和正反类别外，绑定固定且经过审查的源文件路径/内容、预期输出和诊断模式摘要。摘要从已核对的原始定义计算后作为字面量提交；运行时不得从候选用例重建期望摘要。替换内容但保留名称/类别必须拒绝；调整 map 插入顺序或非控制用例执行顺序不应误拒绝。此策略防止静默覆盖漂移，不防御能同时改写检查器和基线的主体。

### AG-02 — Biz 封闭 Application 试点

依赖：AG-01。先重新读取 Biz main/框架锁；选一个租户生命周期 Application。允许改其手写实现、所有者工厂、装配适配、测试和消费者文档；生成物只有经规范生成器再生成才可更新。

将实现从生成 wrapper 同包范围移出，采用应用级 internal；复用 source-edge wrapper，业务对象不获得完整目标实现。保留根 UoW 子调用、租户隔离与原接口。验收：非法兄弟导入反例被拒绝，合法装配、现有生成零漂移与真实 MySQL/子 Operation/回滚通过。若 Ownership/ChangePlan 不支持新位置，保存最小失败并拆出框架任务，不手加宽泛豁免。回滚：应用代码/装配一起回退，不改 schema。

AG-02 的兼容落点采用 `internal/access/application/tenantlifecycle/internal/usecase`：移出生成 wrapper 的同包私有范围，同时保留现有 Change Plan/Audit 的 Application 根。前文 sibling 目录是候选示例，不是强制另建目录真相。试点产生的就绪证据竞争和消费者 owner 快照缺陷必须保留确定性回归；已验证的新测试同时接入对应常规工作流的测试选择器和路径触发范围，不能只存在于一次性交付脚本中。当前资格与集成结果见 STATUS 和 [AG-02 证据](../waves/AG-02-biz-encapsulation.md)。

### AG-03 — IoT Delivery 窄用例试点

依赖：AG-01；与 AG-02 可独立实现，模板推广需二者均验收。选边界较小用例；把全量 Repository 收窄到实际需要的方法，分离规则、SQLite 与兼容映射。允许相关手写业务、端口、持久化适配、特征测试。保持 API/Operation ID/CAS/授权/审计/Outbox。验收：依赖确实收窄，不只是拆文件；临时 SQLite、回归和适用 HTTP/gRPC 验证；生成无漂移。回滚：用例整批回退；schema 变化另开任务。

### AG-04 — 工厂/能力类型检查

依赖：两个试点的最小真实违规。允许工具/控制面分析与测试，不修改 Executor/Authz/UoW。采用 go/packages/go/analysis 或已锁等价基础；先限定可证明的类型、构造和调用关系。校验工厂仅装配使用、不得泄露完整实现/解包器/匿名嵌入额外能力；按实际类型而非 svc/operations 等名称判断。analysistest 正反例必须校验准确诊断。未知动态路径报告 INCOMPLETE。回滚：独立移除本规则，不放宽旧规则。

### AG-05 — 全源码清点与统一政策检查

依赖：AG-02/03。声明支持模块及构建矩阵，区分源码清点、适用规则、分析完成度。覆盖嵌套 go.mod/go.work/replace、构建标签、生成标记欺骗、生产依赖测试支持。比较通用工具后仅选择一个主要后端。验收：新包不漏检，工具错误/源码遗漏不返回 PASS；陌生领域名不影响结论。

### AG-06 — 模板默认封装与公开工作流

依赖：AG-02~05。先保留现有生成端口位置；固化所有者工厂、隐藏实现、窄用例、测试资产隔离、任务/ADR 模板。统一更新 scaffold、ownership、context、change plan 的派生路径，不能另造映射真相。验收：空目录生成不同于两库的业务；规范生成两次零差异；常规新增不用手补规则，原消费者兼容。

### AG-07 — 受控结构迁移 + 债务增长

依赖：AG-04/05；修改 ChangeSet 前先核对 #161 PR 链。只在现有协议上扩展，绑定 base、主体、旧/新路径与类型映射、允许语义和必需测试；不创建第二 Done 协议。base/current 使用同一检查器和政策；同一违规新增调用者也算增长，不能只数 ID。规则/豁免更新不能与违规功能自我批准。验收：允许合法迁移、拒绝扩大数组/跳过测试/过期证据/政策自改/假修复；兼容旧协议。

### AG-08 — 两库批量重构

依赖：AG-06/07。每批一个职责闭环并单独合入；不做一次性大爆炸。Biz 隔离压力资产、按用例收窄依赖；IoT Delivery 分离全量实现、SQLite、兼容层，兼容层有退出条件。旧 backend 冻结证据不盲删。必须验证 last-owner、即时撤权、租户隔离、CAS、职责分离、事务审计、Outbox、健康/关闭等受影响行为。数据/API/事件协议迁移必须独立任务与副本回读，禁止仅回退二进制称数据已回滚。

### AG-09 — 连续演化与陌生业务资格

依赖：AG-06~08。三层测试：精确诊断、模块/组合/完整运行、属性/变换序列。覆盖别名、分文件、宽对象赋窄接口、旧文件增长、嵌套模块、豁免放宽、过期回执；合法共享/复杂算法不过度拒绝。至少一个未参与规则编写的第三业务，连续新增和迁移；输出实际覆盖和每个必需用例结果，禁止按总数达标。

## 6. 每轮执行和完成规则

1. 读 AGENTS、PROJECT_MEMORY、STATUS、本计划以及当前任务 PR；核对并行工作与 Git/框架锁。上一任务若尚未通过其必需门槛，先收口，不跳到依赖它的下一任务。
2. 明确一个任务 ID、输入、可改/禁止路径、行为不变量、验收命令和回滚点，先保留最小反例及合法控制组，再实现。
3. 规范生成与完整原门禁不降低；缺工具、未执行、超时与报告不全均是 INCOMPLETE。安全/事务/运行时变更必须另行压力分类、资格验证与消费者反向验证。
4. 任务分支交付；不能自动覆盖 main、force push、丢弃用户变更或替自己作独立批准。产品主线只继承实际相同源码/树的证据。检查器更新由已批准基线的回归集约束。
5. 回执包含：任务 ID、base/head/tree、改动文件、实际工具链、命令、退出状态、预期反例原因、测试数量/skip、生成与工作树状态、未完成项、远端回读。实现提交、资格通过、已合并是不同状态。
6. 每轮更新当前状态权威并给出下一独立任务，不声称异步自动继续。即使本环境受限也交付真实文件和实现，但不能伪造完成或隐藏阻塞。

## 7. 总体退出标准

错误实现能被对应机制准确拒绝；合法复杂实现不被误判；两库行为保持且职责/能力实际收敛；从空项目开始默认受治理；连续增长与改名不能规避；必需检查完整，规则不能自改放行。没有第二 compiler/runtime/security/架构事实源。不承诺静态证明全部业务正确或提供同进程安全沙箱。
