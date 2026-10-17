# vane-api-server AGENTS.md

面向在本仓库协作的 AI / 开发代理。改后端代码前先读完本文，再动 `internal/module/**`。

权威实现与文档在：

- 框架核心：`vane-common/`（`server` / `module` / `dbserver` / `pkg/dataaccess` / `pkg/taskmgr` / `module/statemgr` / `pkg/safe` / `publish` / `pkg/logger` / `trpc`）
- 持久化用法：`vane-common/pkg/dataaccess/USAGE.md`
- 分布式事务：`vane-common/pkg/taskmgr/README.md`（Task ≠ Job）
- 状态管理器：`vane-common/module/statemgr`（Push+Alive 成对；或 Pull 拉齐合并）
- 协程：`vane-common/pkg/safe`（禁止裸 `go` 不管）
- 本服务骨架：`vane-api-server/README.md`
- **范本模块**：`internal/module/user/`（分层、持久化、错误语义最完整）

---

## 1. 一句话

`vane-api-server` 是基于 `vane-common/server` 的 **模块化 gRPC 后端**。业务以 module 为单位注册；持久化只走 `dataaccess`；协议契约在 `vane-sdk`；跨模块调用走对方 **`pkg/` 门面** 或对方已暴露的 **gRPC 接口**，禁止互相 import `internal/`。

---

## 2. 技术栈与仓库边界

| 层级 | 位置 | 职责 |
|------|------|------|
| 公共框架 | `tlg-common` | 服务启动、模块、DB、dataaccess、日志、安全协程、gRPC/trpc、Job（定时）/ Task（分布式事务）、遥测 |
| 本服务 | `vane-api-server` | 业务模块、配置、命令入口 |
| 契约 | `github.com/kurosann/vane-sdk` | gRPC proto / 生成代码 |
| 客户端 | `vane-console` / 其它进程 | **不**在本仓实现业务裁决 |

不要在 `vane-api-server` 里复制一套 tlg-common 已有的基础设施（自建 repo、自管连接池、裸 `gorm.Open` 等）。

---

## 3. 启动链路（必须理解）

入口：`cmd/main.go`

```text
cmd.NewServerCmd
  └─ server.NewServer(...).RegModule(Modules()).RegExtendServer(dbserver.EnableDBServer(...))
       ├─ ModuleManager.Init
       │    ├─ 各 module.Init()
       │    ├─ GrpcRuntime：收集 Api() 注册到 gRPC
       │    ├─ CronRuntime：收集 Job()
       │    └─ TaskRuntime：收集 Task()
       └─ dbserver.Init → RegisterRelationDB
            └─ dbserver.InitDB
                 └─ 收集实现了 IModuleDb 的 Model()
                      └─ store.RelationDbAutoMigrate(...)   ← 表结构只在这里演进
```

要点：

1. **先** `module.Init`，**后** `InitDB` / AutoMigrate。Init 里若立刻打库，可能表尚未迁完；seed 类逻辑要像 `user` 一样等待 `store.IsRegistered` 并容忍短暂 schema 未就绪。
2. **表结构演进** = `IModuleDb.Model()` + GORM AutoMigrate。**不要**在 service 里再写 `AutoMigrate` / `Migrator().AddColumn` / `ensureSchema`。
3. AutoMigrate **只加不删**。Drop 表/列、拆表搬数据属于运维/独立迁移程序，**不属于**本服务启动逻辑。
4. **禁止使用外键约束**：模型一律不用 `gorm:"constraint:..."` / `foreignKey` 关联，表之间只通过业务字段（如 `account_key`、`id`）逻辑关联，不做 DB 级外键。约束只靠代码/索引保证，避免 AutoMigrate 与运维迁移被外键依赖拖累。

---

## 4. 模块规范

### 4.1 模块接口（`vane-common/module`）

模块用 **组合接口** 声明能力：必须实现 `IModule`；其余按需实现，框架通过 **类型断言** 发现。

源码：`vane-common/module/define.go`、`http.go`，以及 `statemgr/`、`monitor/`。

#### 必选：`IModule`

```go
type IModule interface {
    Init(ctx context.Context) error
    Api() []ApiRegInfo
    Job() []JobRegInfo
    Task() []TaskRegInfo
}
```

| 方法 | 谁消费 | 说明 |
|------|--------|------|
| `Init` | `ModuleManager.Init` | 组装 service、Register 跨模块门面；**此时 DB AutoMigrate 可能尚未完成** |
| `Api` | `GrpcRuntime` | `NewApiRegInfo(ServiceDesc, impl)`；无 gRPC 时返回 `nil` |
| `Job` | `CronRuntime` | **定时任务**（按 cron 周期触发）；与 Task **完全不同类** |
| `Task` | `TaskRuntime` | **分布式事务**：编排多步控制流、步骤幂等、失败可回滚；由业务触发，不是定时器 |

Job 选项：

- `WithJobGlobal()` — 集群内单实例跑（分布式锁）
- `WithJobLaunchRun()` — 进程启动时先跑一次

**Job ≠ Task**：Job 管「到点跑什么」；Task 管「一次业务流程怎么幂等走完 / 失败怎么补偿」。禁止用 Job 冒充长流程事务，也禁止用 Task 当 cron。

编译期建议：`var _ module.IModule = (*Module)(nil)`。

#### 可选扩展（按需实现）

| 接口 | 方法 | 谁消费 | 何时实现 |
|------|------|--------|----------|
| `IModuleDb` | `Model() []ModelRegInfo` | `dbserver.InitDB` → `RelationDbAutoMigrate` | 模块拥有关系/KV 表 |
| `IPublicModule` | `Public() bool` | `GrpcRuntime` | `true` 时把该模块 `Api()` 方法记入公开白名单（仍可在 handler 内校验 token） |
| `IHTTPModule` | `HttpApi() HttpApiInfo` | `httpserver`（若挂了 HTTP 扩展） | 需要 REST/下载等 HTTP 入口 |
| `IModuleStateMgr` | `StateMgr() []statemgr.StateSyncRegInfo` | `CronRuntime` | 状态管理：Push+Alive 成对，或 Pull 合并（见 **§11**） |
| `IMonitorModule` | `Monitor() []monitor.MonitorRegInfo` | `MonitorRuntime` → telemetry | 自定义采集 / metrics |
| `IStopModule` | `Stop(ctx) error` | `ModuleManager.Stop` | 释放连接、后台循环等模块持有资源 |

`ModelRegInfo` 选项：

- `WithModelCache(ttl)` — dataaccess 门面缓存
- `WithModelListen(listener)` — Create/Update/Delete 事件回调

StateMgr 注册助手（在 `module/statemgr`，详见 **§11**）：模块通过它们挂载**同步器**，而不是另写轮询。

- `NewPullStateSyncRegInfo` — Pull：拉目标数据，做远端与本地合并同步
- `NewPushStateSyncRegInfo` + `NewAliveCheckerRegInfo` — **必须成对**：远端 Push 定时推状态，本地 Alive 定时检查存活；少一侧则链路不完整

#### 注册

```go
module.NewModule("user", user.NewUserModule())
```

集中写在 `internal/module/mod_register.go` 的 `Modules()`。**Init 顺序 = 切片顺序**；有依赖时先注册被依赖方（例：`sync` 在 `official` 之前）。

#### 最小骨架示例

```go
type Module struct{ svc *service.Service }

var (
    _ module.IModule   = (*Module)(nil)
    _ module.IModuleDb = (*Module)(nil) // 有表才加
)

func (m *Module) Init(ctx context.Context) error {
    m.svc = service.New(dataaccess.Accessor[*model.Foo]("main"))
    return nil
}
func (m *Module) Api() []module.ApiRegInfo   { return nil }
func (m *Module) Job() []module.JobRegInfo   { return nil }
func (m *Module) Task() []module.TaskRegInfo { return nil }
func (m *Module) Model() []module.ModelRegInfo {
    return []module.ModelRegInfo{module.NewModelRegInfo(&model.Foo{})}
}
```

本服务当前实际用到的扩展：`IModuleDb`、`IPublicModule`、`IHTTPModule`（cloudprofile 本地下载）。`IModuleStateMgr` / `IMonitorModule` / `IStopModule` 框架已支持，本仓业务模块尚未普遍使用。

### 4.2 目录约定

```text
internal/module/<name>/
├── module.go                 # 仅注册与依赖注入（见下）
├── dto/                      # 可选：进程内 DTO / 注册门面
├── pkg/                      # 本模块对外提供的稳定 API（Register/Get + 接口）
└── internal/
    ├── api/                  # gRPC / HTTP 控制层：参数校验、调 service、映射响应
    ├── model/                # dataaccess 模型（实现 model.Table）
    ├── service/              # 本模块业务逻辑 + DataAccessor
    ├── depend/               # 本模块对「外部能力」的引用与适配
    ├── job/                  # 可选：定时 Job 实现
    ├── task/                 # 可选：分布式任务 Plan + step（见 §8）
    │   └── <plan_name>/
    │       ├── *_plan.go     # TaskWorker.Arrange
    │       └── step/         # 各 StepWorker
    ├── statemgr/             # 可选：StateMgr 下的同步器实现（见 §11）
    └── metric/               # 可选：IMonitor 实现（见 §10）
```

#### `module.go`：只做装配，业务尽量薄

`module.go` **仅**放注册与依赖注入相关代码，控制层 / 业务实现放到 `internal/`：

| 允许写在 `module.go` | 禁止写在 `module.go` |
|----------------------|----------------------|
| `Init`：构造 storage / `depend` / `service.New(...)`、`pkg.Register*` | HTTP/gRPC handler 实现、参数校验、响应拼装 |
| `Model` / `Api` / `HttpApi` / `Job` / `Task` / `Monitor` / `StateMgr`：**注册表**（把已实现的类型挂上去） | 业务规则、DB 访问、鉴权细节、下载/上传等处理逻辑 |
| `Stop`：释放本模块持有的连接 / 资源（委托给已注入组件即可） | 大段协议处理、中间件实现体 |

约定：

1. **控制层**（gRPC / HTTP）→ `internal/api/`；`Api()` / `HttpApi()` 只 `NewXxx` + 填 `RegInfo`。
2. **业务** → `internal/service/`；`Init` 里注入依赖后交给 service。
3. **Job / Task / Monitor / StateMgr 实现** → 对应 `internal/job|task|metric|statemgr/`；`module.go` 只注册。
4. 目标形态：打开 `module.go` 能一眼看清「本模块挂了什么、依赖怎么拼」，而不是业务细节。

范本：`cloudprofile/module.go`（HTTP 在 `internal/api/`）、`public/module.go`（gRPC 在 `internal/api/`）。

本仓参考：`user`、`public`、`official`、`cloudprofile`、`cloudproxy`、`sync`。  
`depend` 范本：`vane-common/server/internal/itnlmodule/depend/`。

#### `pkg/` vs `internal/depend/`

| 目录 | 方向 | 放什么 |
|------|------|--------|
| `pkg/` | **出站（本模块 → 别人）** | 本进程内给其它模块调用的稳定接口与 `RegisterXxx` |
| `internal/depend/` | **入站依赖（别人 → 本模块要用）** | 对其它模块 `pkg`、**其它模块 gRPC**、远端服务、中间件的引用与适配 |

`depend` 规则：

1. **「外部」= 其它业务模块**（以及 Center/Pilot 等非本模块能力），**不是** vane-sdk / proto。service **可以**使用 proto。
2. 凡要用外部能力：经本侧 **`internal/depend/`**（调对方 `pkg` 或对方 gRPC），**或**在提供方侧走对方已暴露的 **`pkg/`**；**禁止** service 直接 `import` 其它模块并散落调用。
3. **用 interface 隔离**：在本模块（通常 `depend` 或 service 同包）定义窄接口；`depend` 写实现去调对方 `pkg` / gRPC；`module.Init` 把实现注入 service。service **只依赖接口**，不依赖对方具体类型包。
4. `depend` **可以** import 其它模块的 `pkg/`、`tlg-common`、契约包；**不可以** import 其它模块的 `internal/`。
5. 纯本模块的 DataAccessor / 本地 storage **不算** 外部（仍放 `service` / `storage`）。

示例：

```text
official/internal/depend/accounts.go      # 实现 Accounts 窄接口，内调 user/pkg
xxx/internal/depend/user_grpc.go          # 实现窄接口，经 gRPC 调 Auth
official/internal/service/...             # 字段类型是接口，不 import user/pkg
```

```go
// depend 或 service 包内：窄接口
type AccountLookup interface {
    GetByUserName(ctx context.Context, name string) (*AccountView, error)
}

// service：只持有接口
type Service struct {
    accounts AccountLookup
    // ...
}

// depend：实现接口，内部调对方 pkg / gRPC
type accountDepend struct{}
func (d accountDepend) GetByUserName(...) (*AccountView, error) {
    return userpkg.Accounts().GetByUserName(...) // 仅此处碰对方 pkg
}
```

### 4.3 分层职责

| 层 | 允许 | 禁止 |
|----|------|------|
| `module.go` | 注册扩展点、`Init`/`Stop` 装配与注入 | handler、业务规则、DB、鉴权实现（见 **§4.2**） |
| `api` | 参数校验、调 service、映射 HTTP/`status.Error`；可用 proto | 业务规则、直接 DB、直连其它模块 |
| `service` | 业务、`DataAccessor`、领域错误；**可用 vane-sdk / proto** | `*gorm.DB`；**直接** import/调用其它模块（须经 depend/`pkg` + interface，见 **§4.2 / §4.4**） |
| `depend` | 实现窄接口：适配其它模块 `pkg` **或** 其 gRPC、远端/基础设施 | 本模块核心业务规则、本模块表 CRUD |
| `model` | 表结构、gorm tag、`TableName`/`DbSource`/`DbType` | 业务编排 |
| `pkg` | 给同进程其它模块的稳定接口（供对方 depend 调用） | 依赖其它模块的 `internal/`；不要塞入站适配 |

**范本注释**（照这个写）：

```go
// 持久化直接使用 vane-common/pkg/dataaccess,不再有独立 repo 层
// service 可以使用 vane-sdk / proto（如组装 SyncEvent）
// 其它模块能力：只依赖本侧窄接口，由 depend 调对方 pkg 或 gRPC 注入；禁止 service 直连其它模块
```

### 4.4 跨模块调用

两种合法触达方式（**二选一或组合**）；**使用方 service 始终只看见本侧 interface**：

| 方式 | 适用 | 做法 |
|------|------|------|
| **`pkg` 门面** | 同进程、同二进制内模块互调 | 提供方暴露 `pkg`；使用方 `depend` 实现窄接口并调对方 `pkg` |
| **gRPC** | 跨进程/跨服务，或希望走正式 RPC 边界 | 提供方 `Api()` 暴露服务；使用方 `depend` 里 `trpc.GetSvcConn` + 生成 Client，同样经窄接口注入 service |

硬约束：

- **禁止** import 其它模块的 `internal/`（无论同进程还是 gRPC）。
- **禁止** service 直接 `userpkg.Xxx()` / `GetSvcConn` / import 其它模块业务包；收敛到 `internal/depend/`（或等价适配），**中间必须有 interface**。
- `public` 等入口模块同样：经 depend + 接口，不拉对方 internal。
- 同进程优先 `pkg`（少一层序列化）；已有稳定 gRPC、或调用方不在本进程时用 gRPC。
- **proto / vane-sdk 不算「外部模块」**，service 与 api 均可使用。

---

## 5. 持久化规范（硬约束）

### 5.1 只用 DataAccessor

`DataAccessor` **直接作为 service 的字段**组装，不要再包 repo。推荐写法（与 `user` 的 `AuthService` 一致）：

```go
type AuthService struct {
	userVerifier         CredentialVerifier
	deviceAccount        dataaccess.DataAccessor[*model.DeviceAccount]
	tokenSession         dataaccess.DataAccessor[*model.TokenSession]
	maxAttempts          int
	lockoutSeconds       int
	accessTokenSeconds   int
	refreshTokenSeconds  int
	refreshWindowSeconds int
}
```

组装方式二选一即可：

1. **在 `New` 内组装**（本模块表，常见）：

```go
func New() *Service {
	return &Service{
		deviceAccount: dataaccess.Accessor[*model.DeviceAccount]("main"),
		tokenSession:  dataaccess.Accessor[*model.TokenSession]("main"),
	}
}
```

2. **由 `module.Init` 传入**（便于单测替换或共用 source 常量）：

```go
func NewAuthService(
	userVerifier CredentialVerifier,
	deviceAccount dataaccess.DataAccessor[*model.DeviceAccount],
	tokenSession dataaccess.DataAccessor[*model.TokenSession],
	// ...
) *AuthService
```

用法：

```go
item, err := s.deviceAccount.First(ctx, dataaccess.Eq("device_id", id))
if errors.Is(err, dataaccess.ErrDataNotFound) { /* 业务 not found */ }
err = s.tokenSession.Create(ctx, session)
err = s.account.Update(ctx, acc, dataaccess.OnlyFields("status", "update_at"))
err = s.account.Update(ctx, acc, dataaccess.IfRetry(func(ctx context.Context, fresh *model.Account) (*model.Account, error) {
	return fresh, nil
}))
```

条件：`Eq` / `Ne` / `In` / `IsNull` / `Or` / `And` / `Order` / `Page` / `Limit` 等（见 `condition.go` / `USAGE.md`）。

### 5.2 模型

- 嵌入 `base.BaseModel`（或 `BaseModelOnlyDate` + 自定义主键）。
- 实现 `TableName()`；按需 `DbSource()` / `DbType()`。
- 需要 TTL / 乐观锁时嵌入 `ExpireAtBaseModel` / `RevisionBaseModel`（`IfRetry` 对 `RevisionTable` 才真正生效）。
- 索引、唯一约束写在 **gorm tag** 上，交给 AutoMigrate；不要在 service 里 `CREATE INDEX`。

在 `Module.Model()` 注册：

```go
func (m *Module) Model() []module.ModelRegInfo {
    return []module.ModelRegInfo{
        module.NewModelRegInfo(&model.Foo{}),
    }
}
```

### 5.3 禁止事项（曾在 cloudprofile / cloudproxy 出现）

1. **禁止**业务路径使用 `*gorm.DB`、`db.Where`、`Clauses(OnConflict)`、`UpdateColumns`。
2. **禁止** `store.GetAllRelationDB()` / `relationDB()` 作为业务持久化入口。
3. **禁止** service 内 `ensureSchema` / 运行时 `AutoMigrate` / `Migrator().AddColumn`。
4. **禁止**为薄 CRUD 再包一层 repo；DataAccessor 就是访问层。
5. **禁止**把「旧表拆分 / Drop 废弃表列 / 全量数据搬家」放进 `module.Init` 后台 goroutine；交给外部迁移程序。
6. **禁止**在 api 层直接打库。

文件存储（如 cloudprofile payload）可以单独抽象 `storage.PayloadStore`；**元数据行**仍必须走 DataAccessor。

### 5.4 错误语义

对齐 `user` 模块：

- **业务失败**（密码错、锁定、不存在）→ 返回 outcome/reason，`err == nil`；由 api 或 service 填入响应（可用 proto 字段）。
- **系统失败**（DB 不可达）→ `err != nil`；api 映射 gRPC `Internal`。
- 查询空：`errors.Is(err, dataaccess.ErrDataNotFound)`，不要依赖 `gorm.ErrRecordNotFound`。

---

## 6. 基础设施与基础组件

优先复用 `tlg-common`，不要平行造轮子。

| 组件 | 包 | 用法 |
|------|-----|------|
| 服务编排 | `vane-common/server` | `NewServer` + `RegModule` + `RegExtendServer` |
| DB 注册 | `vane-common/dbserver` | `EnableDBServer(cfg.GetDbConfig)`；配置里 `db.main` DSN |
| MySQL | `vane-common/pkg/mysqldb` | 由 cfg → dbserver 注入，业务不直接 Open |
| 数据访问 | `vane-common/pkg/dataaccess` | 唯一业务持久化入口 |
| 日志 | `vane-common/pkg/logger` | `Infof/Warnf/Errorf(ctx, ...)`；启动时 `InitLogger` |
| 时间 | `vane-common/pkg/clock` | 可测时间；BaseModel 已用 |
| 安全协程 | `vane-common/pkg/safe` | 见 **§7**；禁止裸 `go` 不管 |
| gRPC/trpc | `vane-common/trpc` | 服务发现、公开方法注册、拦截器 |
| 定时任务（Job） | `module.Job()` + `module/job` | **仅** cron 周期任务；实现放 `internal/job/`；见 **§8.5** |
| 分布式事务（Task） | `module.Task()` + `pkg/taskmgr` | **控制流 + 幂等步骤 + 可回滚**；见 **§8**；实现放 `internal/task/` |
| 集群消息总线 | `vane-common/publish` | 见 **§9** |
| 业务 SyncPlane | `sync/pkg` | 见 **§12**（客户端下行；≠ publish、≠ StateMgr） |
| 状态管理器 | `IModuleStateMgr` + `module/statemgr` | 见 **§11**；同步器实现放 `internal/statemgr/` |
| Monitor | `IMonitorModule` + `module/monitor` | 见 **§10**（含健康检查与 **Revert 自愈**）；实现放 `internal/metric/` |
| 遥测底座 | `vane-common/telemetry` | health / Prometheus；Monitor 注册到其上 |
| 配置 | `internal/cfg` | YAML → `ConfigRoot`；`GetServerConfig` / `GetDbConfig` |

CLI：`vane-common/cmd` 的 `NewServerCmd`（start / version / 配置路径）。

---

## 7. 协程规范（`vane-common/pkg/safe`）

**硬约束：业务代码禁止裸写 `go func() { ... }()` 后不管。**  
启动协程必须：能取消、能收尾、panic 可恢复。优先用 `safe`；需要生命周期时绑定 `context`。

### 7.1 为什么不能裸 `go`

| 裸 `go` 的问题 | 要求 |
|----------------|------|
| panic 打挂进程或静默丢 | 用 `safe` 捕获并打日志 |
| 请求结束后仍跑、泄漏 | 绑定可取消的 `ctx`，循环里 `select` `ctx.Done()` |
| 错误无人收 | `GoE` / `GoCtxE` + `WaitAndRecover`，或明确文档化「仅日志」 |
| 关停时拖死进程 | `IStopModule.Stop` / `cancel` / `WaitGroup` 等停干净 |

框架 Job（定时）/ Task（分布式事务）/ StateMgr / Monitor 已由 runtime 调度，**不要**再在业务里为同类事情自建无限 `go` 循环。

### 7.2 选用哪一种

| 场景 | API | 说明 |
|------|-----|------|
| 一次后台活、需等结束 | `safe.NewWaitGroupContext(ctx)` + `GoCtx` / `GoCtxE` → `WaitAndRecover` | 推荐默认 |
| 限制并发度 | `safe.NewPoolContext(ctx, max)` + `GoCtx*` → `WaitAndRecover` | 批量扇出 |
| 并行跑完再汇总 | `safe.GoAndWait(f1, f2, ...)` | 同步等待多任务 |
| 必须常驻、可取消 | `safe.GoKeep(ctx, gap, fn)` 或 `WaitGroup` + 循环听 `ctx.Done()` | **必须**传入可取消 ctx |
| 仅 panic 防护的同步调用 | `safe.Try` / `safe.TryE` | 不启协程 |
| 周期业务（定时） | `module.Job()` / `StateMgr()` | 不要自建 ticker goroutine |
| 多步幂等事务（控制流） | `module.Task()` | 不要 `go` 裸跑长流程，更不要塞进 Job |

### 7.3 推荐写法

```go
// 带取消、等结束
wg := safe.NewWaitGroupContext(ctx)
wg.GoCtx(func(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// work；阻塞调用应尊重 ctx
		}
	}
})
if err := wg.WaitAndRecover(); err != nil {
	return err
}

// 限流扇出
p := safe.NewPoolContext(ctx, 8)
for _, item := range items {
	item := item
	p.GoCtxE(func(ctx context.Context) error {
		return do(ctx, item)
	})
}
return p.WaitAndRecover()

// 常驻循环（Init 里启动时：ctx 来自模块可取消根，Stop 时 cancel）
safe.GoKeep(runCtx, 10*time.Second, func() {
	_ = tickOnce()
})
```

`GoCtx` / `GoCtxE` 拿到的是 WaitGroup/Pool 绑定的 ctx；子任务里 **RPC / DB / sleep** 都要能被该 ctx 打断。

### 7.4 生命周期规范

1. **每个后台协程都有主人**：谁 `Start`，谁在 `Stop` / `defer cancel` / 请求结束时负责停。
2. **context 链**：请求路径用入参 `ctx`；模块级后台用 `context.WithCancel` 根 ctx，在 `IStopModule.Stop` 里 `cancel` 并 `Wait`。
3. **超时**：可能挂住的调用包 `context.WithTimeout`；禁止无超时的无限阻塞。
4. **循环退出**：`for { select { case <-ctx.Done(): return ... } }`；不要只靠 `time.Sleep` 假装可停。
5. **channel**：发送方或接收方退出时关闭/cancel 要明确，避免永久堵死；慢消费者要有背压策略（drop / 断开重连，见 SyncPlane Hub）。
6. **不要**在 `Init` 里 `go` 跑完就丢：seed 类也要用 `safe` + 可取消 ctx，并容忍 DB 尚未就绪。
7. **共享状态**加锁或用 `safe.Map` 等并发安全结构；禁止无同步的跨协程写。

### 7.5 与框架调度的边界

- 已有 **Job / Task / StateMgr / Monitor** 的能力，优先注册到模块接口，而不是手写 `go` + `time.Ticker`。
- gRPC stream 内的读循环：可用 `safe.Go`/`WaitGroup`，且必须随 `stream.Context()` 结束；`defer cancel`。
- 确需与请求同生共死的辅助协程：派生 `ctx`，`defer cancel()`，主路径 `WaitAndRecover` 或明确文档化取消点。

### 7.6 反例

```go
// ❌ 禁止
go func() {
	for {
		doWork() // 无法取消、panic 无恢复、错误丢弃
		time.Sleep(time.Second)
	}
}()

// ✅
wg := safe.NewWaitGroupContext(parentCtx)
wg.GoCtx(func(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := doWork(ctx); err != nil {
				logger.Errorf(ctx, "doWork: %v", err)
			}
		}
	}
})
```

源码：`vane-common/pkg/safe/goroutine.go`。

---

## 8. 分布式事务（Task / taskmgr）

**Task 与 Job 完全不是一类东西。**

| | **Job（定时任务）** | **Task（分布式事务）** |
|--|---------------------|------------------------|
| 本质 | 到点执行的周期回调 | 一次业务的**控制流程**（编排 / 推进 / 补偿） |
| 触发 | cron Spec（`CronRuntime`） | API / 其它业务调用 `AsyncRunTask` 等（`TaskRuntime`） |
| 目标 | 巡检、清理、周期性拉取 | 跨资源多步骤，**步骤幂等**，失败可 **Rollback** |
| 状态 | 一般无「流程实例」持久化编排 | 有任务记录 / 步骤状态，可恢复、可查询 |
| 失败 | 等下次定时再跑 | 已完成步骤逆序补偿，流程结束在失败/成功态 |

本框架的「分布式事务」指 **可编排、步骤幂等、可回滚、可恢复的多步骤控制流**（Saga 风格），不是 XA。  
包：`vane-common/pkg/taskmgr`（详细 API 见该包 `README.md`）。**不要**再用已废弃的 `pkg/task`。

### 8.1 注册

在 `Module.Task()` 返回：

```go
func (m *Module) Task() []module.TaskRegInfo {
	return []module.TaskRegInfo{
		module.NewTaskRegInfo("run_foo", run_foo.NewRunFooPlan(/* depend */)),
		module.NewTaskRegInfo("stop_foo", stop_foo.NewStopFooPlan()),
	}
}
```

- **task code** 用稳定字符串常量（可放 `pkg/consts` 或模块内 consts，避免魔法字符串散落）。
- `TaskRuntime` 会挂好 DataProvider / Reporter / Lock（框架 `depend`），业务只需提供 `TaskWorker`。

### 8.2 目录与职责

```text
internal/task/<plan>/
├── <plan>_plan.go      # 实现 taskmgr.TaskWorker：Arrange 编排步骤
└── step/
    ├── step_a.go       # 实现 taskmgr.StepWorker
    └── types.go        # 步骤间 payload / result DTO
```

API 细节见 `vane-common/pkg/taskmgr/README.md`。

### 8.3 Plan（TaskWorker）

`Arrange` 只负责**编排控制流顺序**，不写重业务：

```go
func (r *RunFooPlan) Arrange(ctx context.Context, payload []byte) []taskmgr.StepRegInfo {
	return []taskmgr.StepRegInfo{
		taskmgr.NewStepInfo("step1-check", step.NewCheckStep()),
		taskmgr.NewStepInfo("step2-do", step.NewDoStep(r.depend)),
		taskmgr.NewStepInfo("step3-notify", step.NewNotifyStep()),
	}
}
```

外部能力经 **depend 注入 Plan/Step**，不要在 step 里直连其它模块 internal。

### 8.4 Step（StepWorker）规范

| 方法 | 要求 |
|------|------|
| `Execute` | 解析 payload / 上一步 result；业务副作用；**必须幂等**；`StepExecCompleted` / `Failed` / `Pending`（异步） |
| `PollStatus` | 同步步骤直接 `StepPollCompleted`；异步步骤轮询直到完成/失败 |
| `Rollback` | **逆序补偿**；**必须幂等**；从 Context / 入参恢复要回滚的 ID |

约定：

1. 步骤间数据用 **JSON result** 或 `taskmgr.ContextSet/Get`，类型定义在 `step/types.go`。
2. 进度：`taskmgr.UpdatePosition(ctx, n)`。
3. 失败返回 `StepExecFailed(err)`，由引擎触发已完成步骤的 Rollback。
4. 需要暂停/停止时再实现 `StepWorkerPause` / `Stop` / `Cancel`（见 taskmgr README）。
5. Step 内持久化仍走 **DataAccessor**；调外部服务走 **depend**。

### 8.5 Job（定时任务）要点

Job 只回答「按什么节奏重复跑」：

```go
func (m *Module) Job() []module.JobRegInfo {
	return []module.JobRegInfo{
		module.NewJobRegInfo("cleanup", "@every 1h", job.NewCleanup(), module.WithJobGlobal()),
	}
}
```

- 实现放 `internal/job/`。
- 选项：`WithJobGlobal()`（集群单实例）、`WithJobLaunchRun()`（启动先跑一次）。
- **禁止**在 Job 里实现「多步创建/删除资源 + 失败要回滚」的事务流——那种必须用 **Task**。
- StateMgr 的同步器虽也由 `CronRuntime` 调度，但语义是**状态管理**（§11），不要和普通业务 Job、更不要和 Task 混为一谈。

### 8.6 选型（互不替代）

| 需求 | 用 |
|------|----|
| 每 N 秒/每天跑一次清理、巡检、统计 | **Job** |
| 一次「开通 / 下线 / 迁移」等多步，要幂等推进、失败补偿 | **Task** |
| 远程↔本地状态视图持续对齐 | **StateMgr**：Push+Alive 成对，或 Pull 合并 |

---

## 9. 消息总线（publish）

集群级 Pub/Sub：`vane-common/publish`。框架在非 standalone 模式下由内部 module 接好远端 transport；业务侧主要用发布/订阅 API。

### 9.1 API

```go
import "vane-common/publish"

// 发布到所有订阅者（含远端，带重试）
err := publish.Publish(ctx, topic, data)

// 仅本机订阅者
publish.PublishLocal(ctx, topic, data)

// 订阅；必须 defer cancel()
ch, cancel, err := publish.Subscribe(ctx, topic)
defer cancel()
for msg := range ch { /* ... */ }
```

Topic 用**具名常量**；本仓新增 topic 时集中定义，禁止散落字符串。

### 9.2 规范

1. **payload 用 JSON**（或约定好的 protobuf bytes），订阅方容忍解析失败（log + continue）。
2. 订阅循环用 `safe.Go` / `safe.GoKeep`，监听 `ctx.Done()`，退出时 `cancel()`。
3. 需要持久化订阅关系时，由中心侧 `SetSubscriberStore`；业务模块一般只 Publish/Subscribe。
4. **不要**在业务里自建另一套跨机广播；进程内 fan-out 另说（§9.3）。
5. 远端发送适配属于框架 `depend`（如 `PublishRemoteTransportService`），业务不手写 `GetSvcConn` 发 publish。

API 见 `vane-common/publish`。

### 9.3 与 SyncPlane / StateMgr 的区别

| | `vane-common/publish` | `sync/pkg`（§12） | `statemgr`（§11） |
|--|----------------------|-------------------|-------------------|
| 用途 | 集群服务发现、告警、跨服务通知 | 客户端 Sync 订阅的业务实体变更 | **状态管理**：同步器维护远程/本地状态视图 |
| 范围 | 多节点 topic | 当前进程 Hub → 已登录客户端 | CronRuntime 调度已注册的同步器 |
| 谁用 | 基础设施 / 跨服务 | official content 等写库后 live 下发 | 需与中枢/对端对齐状态的模块 |

「服务间事件」走 **publish**；「内容变更推客户端」走 **sync/pkg**；「模块状态对齐」走 **StateMgr + 同步器**。不要混用。

---

## 10. Monitor（健康、指标与自愈）

实现 `module.IMonitorModule`，在 `Monitor()` 注册采集器；`MonitorRuntime` 挂到 `telemetry.CollectCenter`。

Monitor **不只是打点**：还可挂健康检查，以及 **`IsOk()==false` 时触发的自愈（Revert）**。

### 10.1 注册

```go
var _ module.IMonitorModule = (*Module)(nil)

func (m *Module) Monitor() []monitor.MonitorRegInfo {
	return []monitor.MonitorRegInfo{
		monitor.NewMonitorRegInfo("db_status", metric.NewDBStatus()),
		monitor.NewMonitorRegInfo("discover_status", metric.NewDiscoverStatus(), monitor.WithSpec("@every 10s")),
	}
}
```

实现放 `internal/metric/`。  
框架范本：`vane-common/server/internal/itnlmodule/metric/discover_status.go`（健康检查 + 自愈）。

### 10.2 IMonitor 契约

```go
type IMonitor interface {
	Collect()                       // 周期被调用：探测并 Set 指标值
	Metrics() []*monitor.MetricRegInfo  // 声明要暴露的指标（可带健康/自愈）
}
```

| 指标包装 | 工厂 | 能力 |
|----------|------|------|
| 普通指标 | `NewGaugeRegInfo` 等 | 仅 Prometheus 采集 |
| 健康检查 | `NewHealthCheckerMetric(metric, HealthChecker)` | `IsOk()` → healthz；不自愈 |
| **健康 + 自愈** | `NewHealthCheckerAndRevertableMetric(metric, HealthCheckerAndRevertable)` | `IsOk()` 失败时异步调 `Revert()` |

```go
type HealthChecker interface {
	IsOk() (ok bool, info map[string]any)
}
type Revertable interface {
	Revert() error // 自愈动作：须幂等、尽快返回（重活可 Flush Job / 发信号）
}
type HealthCheckerAndRevertable interface {
	HealthChecker
	Revertable
}
```

### 10.3 自愈链路（框架已实现）

```text
Collect / health 周期 或 ?now=1
        │
        ▼
IsOk() ──true──► 仅上报健康
        │
       false（且为 HealthCheckerAndRevertableMetric）
        │
        ▼
telemetry 投递 revertCh（标记 recovering）
        │
        ▼
后台 GoKeep 调用 Revert()（safe.TryE）
        │
        ▼
成功清 recovering；失败记 recoverErr（出现在健康 info 里）
```

源码：`vane-common/telemetry/collection.go` 的 `checkHealthDataRevert` / `InitCollectCenter` 中 revert 循环。  
健康探针注册在 `gosundheit`；`/metrics?now=1` 会 `CollectData` + `CheckHealthAndRevert`。

业务范本（发现不一致则 Flush 同步 Job）：

```go
func (d *DiscoverStatus) Metrics() []*monitor.MetricRegInfo {
	return []*monitor.MetricRegInfo{
		monitor.NewHealthCheckerAndRevertableMetric(d.discoverCount, d.discoverStatusHealth),
	}
}
func (d *DiscoverStatusHealthAndRevertable) Revert() error {
	job.Flush(context.Background(), internalJob.SyncSvcmapJobName)
	return nil
}
```

### 10.4 规范

1. `Collect` 必须**短小、有超时**；`IsOk` / `Revert` 同样不要长时间堵死探测路径。
2. **需要自愈**用 `NewHealthCheckerAndRevertableMetric`；只要探活、不要自动修，用 `NewHealthCheckerMetric`。
3. `Revert` **必须幂等**；优先触发已有 Job / StateMgr `Flush`，不要在 Revert 里堆重逻辑或裸 `go`。
4. 自愈由框架异步执行；`IsOk` 里**不要**自己再调一遍 `Revert`（避免重复与阻塞探活）。
5. 指标名稳定、label 低基数；探测外部依赖时客户端放 **depend**。
6. 与 StateMgr 区分：Monitor 是**可观测 + 可选自愈**；StateMgr 是**状态视图**的 Push/Alive 或 Pull（§11）。

---

## 11. 状态管理器（StateMgr / statemgr）

**StateMgr = 状态管理器**，不是「数据同步器」本身。  
模块实现 `IModuleStateMgr`，在 `StateMgr()` 里**挂载同步器**；框架 `CronRuntime` 按 Spec 调度这些同步器，用来维护本模块关心的状态视图（本机 ↔ 对端/中枢）。

设计意图（实现时记住即可）：集群能力按**状态**组织——中枢侧掌握集群内相关状态后，功能编排才简单。业务模块通过 StateMgr 把「该对齐的状态」持续收敛到一致，而不是到处手写轮询。

包：`vane-common/module/statemgr`。默认 Spec 多为 `@every 10s`。

| 概念 | 是什么 |
|------|--------|
| **StateMgr** | 模块扩展点：`StateMgr() []StateSyncRegInfo` |
| **同步器** | 挂在 StateMgr 下的 Pull / Push / Alive 实现 |
| **非 StateMgr** | 客户端内容下行 → **§12 SyncPlane**；多步幂等控制流 → **§8 Task**；纯定时回调 → **Job** |

### 11.1 两种用法（先选模式）

| 模式 | 谁注册什么 | 关系 |
|------|------------|------|
| **Push + Alive（成对）** | **远端**用 `NewPushStateSyncRegInfo`；**本地**用 `NewAliveCheckerRegInfo` | 远端定时 **Push** 把状态推过去做数据同步；本地定时 **Alive** 检查远端推来的数据是否仍存活（TTL）。**少一边则运行不完整** |
| **Pull** | 需要跟齐目标侧的一方用 `NewPullStateSyncRegInfo` | **拉**目标数据过来，按 ID 做远端与本地的 **合并同步**（Create / Update / Delete） |

不要把 Push、Alive、Pull 当成三个可随意单挂的「同类定时器」。Push 与 Alive 是一条上下行存活链路的两端；Pull 是另一条「拉齐合并」路径。

### 11.2 注册示例

**远端（Push 端）**：

```go
func (m *Module) StateMgr() []statemgr.StateSyncRegInfo {
	return []statemgr.StateSyncRegInfo{
		statemgr.NewPushStateSyncRegInfo("foo_push", syncer.NewFooPush(m.depend),
			statemgr.WithPushLaunchRun(),
			statemgr.WithPushSpec("@every 10s"),
		),
	}
}
```

**本地（Alive 端，与上面对应同一类状态）**：

```go
func (m *Module) StateMgr() []statemgr.StateSyncRegInfo {
	return []statemgr.StateSyncRegInfo{
		statemgr.NewAliveCheckerRegInfo("foo_alive", syncer.NewFooAlive(m.depend),
			statemgr.WithAliveTtl(30*time.Second), // 须大于对端 Push 周期，留抖动余量
			statemgr.WithAliveSpec("@every 10s"),
			statemgr.WithAliveLaunchRun(),
		),
	}
}
```

**Pull（拉目标合并）**——可与上面模式并存，但是独立语义：

```go
func (m *Module) StateMgr() []statemgr.StateSyncRegInfo {
	return []statemgr.StateSyncRegInfo{
		statemgr.NewPullStateSyncRegInfo("foo_pull", syncer.NewFooPull(m.depend),
			statemgr.WithPullLaunchRun(),
			statemgr.WithPullSpec("@every 30s"),
		),
	}
}
```

同步器实现放 `internal/statemgr/`。  
框架内范本：`vane-common/server/internal/itnlmodule` 的 `service_sync`（Push 上报本机服务状态；对端需有对应 Alive 才构成完整存活链路）。

### 11.3 同步器契约

#### Push（远端）+ Alive（本地）——成对

```text
远端进程                         本地 / 中枢侧
────────                         ────────────
定时 Push 状态 ──────────────►  写入/刷新带 timestamp 的状态
                                 定时 Alive：timestamp 过期则 Delete
```

| | Push（远端） | Alive（本地） |
|--|--------------|---------------|
| 注册 | `NewPushStateSyncRegInfo` | `NewAliveCheckerRegInfo` |
| 行为 | `ListLocal` → 逐条 `Push` 到对端 | `List` 本地已收状态 → 超 TTL 则 `Delete` |
| 缺了会怎样 | 本地收不到刷新，Alive 会把状态当死亡清掉 | 远端一直推，本地过期垃圾堆着或永不收敛 |

```go
type IPushStateSyncer[L any] interface {
	IsReady(ctx context.Context) bool
	ListLocal(ctx context.Context) ([]L, error)
	Push(ctx context.Context, local L) error
}

type IAliveChecker[L AliveEntity] interface { // AliveEntity: GetTimestamp() int64
	IsReady(ctx context.Context) bool
	List(ctx context.Context) ([]L, error)
	Delete(ctx context.Context, item L) error
}
```

Alive 超时：`now - timestamp > ttl` → `Delete`。  
**TTL 必须大于 Push 周期**（加网络/调度抖动），否则误杀仍存活的远端。

默认：Push Spec `@every 10s`；Alive TTL `10s`、Spec `@every 10s`、`IsGlobal=true`。成对设计时两边 Spec/TTL 要一起算。

#### Pull——拉目标合并同步

把**目标侧**数据拉过来，与本地按 ID 合并：

1. `IsReady` 为 false → 整轮跳过。
2. `ListRemote` / `ListLocal` 建 ID map。
3. 远端有、本地无或 `CompareExist==false` → `Create`；否则 → `Update`。
4. 本地有、远端无 → `Delete`。

实体需实现 `StateEntity`（`GetID() string`）。

```go
type IPullStateSyncer[R, L StateEntity] interface {
	IsReady(ctx context.Context) bool
	ListRemote(ctx context.Context) ([]R, error)
	ListLocal(ctx context.Context) ([]L, error)
	CompareExist(ctx context.Context, remote R, local L) bool // false 视为需重建
	Create(ctx context.Context, remote R) error
	Update(ctx context.Context, remote R, local L) error
	Delete(ctx context.Context, local L) error
}
```

默认：`Spec=@every 10s`，`IsGlobal=true`。

### 11.4 选项与手动触发

| 选项 | 作用 |
|------|------|
| `With*LaunchRun()` | 进程启动后立刻跑一轮 |
| `With*Spec(cron)` | 覆盖默认 `@every 10s` |
| `WithAliveTtl(d)` | Alive 超时（仅 Alive；须配合对端 Push 周期） |

手动立刻跑一轮（与 Job 共用 Flush）：

```go
statemgr.Flush(ctx, "foo_push") // name 与 RegInfo.Name 一致
```

### 11.5 规范

1. **先定模式**：要「远端推 + 本地验活」→ **Push 与 Alive 成对部署**（可跨进程/模块）；要「拉目标合并」→ **Pull**。
2. **Push/Alive 禁止只挂一半**：只有 Push 没有 Alive，或只有 Alive 没有 Push，状态链路不完整。
3. **Alive TTL > Push 周期**，留足抖动；改 Push Spec 时同步改对端 TTL。
4. **同步器挂 StateMgr**：不要自建 ticker；也不要把状态同步塞进普通 `Job()` / `Task()`。
5. **幂等**：Create/Update/Delete/Push 必须可重复执行。
6. **IsReady**：依赖未就绪返回 false，禁止硬失败拖垮 cron。
7. **短小**：单轮尽快结束；大批量分页/限流。
8. **持久化仍走 DataAccessor**；远端经 **depend** 注入。
9. **Pull 的 GetID() 稳定**；`CompareExist`：true → Update，false → Create。
10. **与 SyncPlane 分工**：StateMgr 管服务/集群侧**状态视图**；SyncPlane 管对客户端的内容下行。

---

## 12. SyncPlane（业务内容下行）
本仓模块：`internal/module/sync`。协议在 `vane-sdk` 的 `SyncService`（Pull + Subscribe）。  
**服务端是 SoT**；客户端（如 vane-core）只持本地镜像。v1 **单实例**进程内 Hub；多实例前 revision 需改为全局发号。

### 12.1 职责切分

| 角色 | 做什么 |
|------|--------|
| `sync` 模块 | gRPC、鉴权→owner、Hub live fan-out、调度 DomainProvider |
| 业务模块（如 `official`） | `RegisterDomain` / `RegisterOwnerResolver`；写库后 `Publish` |
| 客户端 | 登录后 Pull / Subscribe；按 cursor 增量；DELETE 清本地 |

**Init 顺序**：`sync` 必须在接入方（如 `official`）之前，以便 `RegisterPublisher` 先完成。

### 12.2 业务接入（`sync/pkg`）

内容变更推客户端走 **`sync` 模块的 `pkg`**（同进程门面）。按 **§4.4**：业务 service **不**直接散落 `syncpkg.Publish`；本侧定义窄接口（如 `ContentPublisher`），由 `depend` 调 `sync/pkg` 注入——或接受在 `Init` 注册时注入已封装好的接口。

禁止 import `sync/internal`。

```go
// Init（模块装配 / depend）：注册 owner 映射 + domain 增量查询（可调 sync/pkg）
syncpkg.RegisterOwnerResolver(...)
syncpkg.RegisterDomain(syncpkg.DomainContentGroup, provider)

// service 写路径：经注入的窄接口发布（实现内部再调 sync/pkg.Publish）
// 事件可用 vanev1.SyncEvent（proto 允许）
publisher.Publish(ctx, &vanev1.SyncEvent{
	Domain: domain, EntityId: id, OwnerId: owner,
	Revision: rev, Op: vanev1.SyncOp_SYNC_OP_UPSERT, // 或 DELETE
	Payload: payloadJSON,
})
```

`DomainProvider.Pull` 约定：

| cursor | 行为 |
|--------|------|
| `0` | 只返回未软删活数据（首次镜像） |
| `>0` | `revision > cursor`；已软删 → `SYNC_OP_DELETE` |
| 过期 | 可返回 `resync=true`，客户端 ClearDomain 后全量 |

revision 用 `syncpkg.NextRevision()`（进程内单调，宜经 depend 封装）；事件必须带非空 `owner_id` + `domain`。

范本（存量仍有直调 `syncpkg`，新代码按 interface 收敛）：`official/internal/service/sync_providers.go`、`content_sync.go`。

### 12.3 协议要点（服务端）

1. **Pull**：token → owner → 各 domain since-cursor 批次。
2. **Subscribe**：Hello（可先 Ping）→ 鉴权 → **先挂 live 再 CatchUp**（避免间隙丢事件）→ Live。
3. Hub **非阻塞**；缓冲满 drop 并信号 overflow → 客户端应断流、带 cursor 重连补齐。
4. 软删必须进通道；不要物理删后还指望客户端靠「没推到」发现。

### 12.4 规范

1. 触达 SyncPlane：只经 **`sync/pkg`**（由本侧 `depend` + 窄接口注入 service），禁止 import `sync/internal`；service 不要散落直调 `syncpkg`。
2. 写路径：**先持久化成功，再 Publish**；Publish 失败不回滚写（客户端靠 Pull 自愈）。
3. 新增 domain：加常量 + Provider + 写路径 Publish；**不改** Sync 协议。
4. owner：子账号经 `OwnerResolver` 映射到主账号；空 owner **禁止**当广播。
5. 不要用 `vane-common/publish` 或 StateMgr 替代 SyncPlane 推客户端内容。
6. **可以用 proto** 组装 `SyncEvent`（见 **§4.3**）。

---

## 13. gRPC / 契约

- 服务定义与生成代码在 **vane-sdk**，本仓不手改 `*_grpc.pb.go`。
- `Api()` 使用 `module.NewApiRegInfo(vanev1.Xxx_ServiceDesc, impl)`。
- `IPublicModule.Public()==true` 的服务方法会进入公开白名单；仍要在实现里校验 token（如 Sync）。
- api 层保持「薄」：参考 `public/internal/api/auth_server.go`。

---

## 14. 配置

- 文件：`config/config.yaml`（示例见 `config.yaml.example`）。
- 结构：`internal/cfg.ConfigRoot`（server / log / db / auth / cloud_profile / translate…）。
- 新增配置：先加字段与默认值，再在业务 `Init` 读取；不要在业务里读环境变量散落魔法值（除非已有约定）。

---

## 15. 代码风格

1. **匹配周围文件**：命名、错误包装、`fmt.Errorf("%w")`、注释语言与邻域一致。
2. **`module.go` 尽量薄**：只注册扩展点与依赖注入；控制层 / 业务见 **§4.2**。
3. **注释写清「为什么」**：包头说明职责与非目标（见 `user`、`sync`、`public`）。
4. **Service 持有 DataAccessor 字段**：在 `New` 里 `dataaccess.Accessor[*model.X]("main")` 组装，或由 Init 注入；**不要**再包 repo，也**不要**在方法里临时 `GetAllRelationDB`。
5. **跨模块 / 外部能力**：经 `depend`（或对方 `pkg`）+ **interface** 注入；service **可以**用 proto，**禁止**直连其它模块。
6. **并发**：禁止裸 `go` 不管；用 `safe` + `context`，见 **§7**。
7. **Seed / 本地自举**：可放在 Init 后台（如 auth seed），但那是**灌默认数据**，不是 schema 迁移；启动仍走 `safe` + 可取消 ctx。
8. **不要**顺手重构无关模块；不要为「整洁」删除仍被登录兼容路径使用的懒迁移，除非产品确认可砍。

---

## 16. 测试

- 单测用 sqlite 内存库：`sqlitedb` + `store.RegisterRelationDB` + `RelationDbAutoMigrate`（仅测试）。
- 断言走 DataAccessor / service API，不要在测试里依赖业务 service 的私有 `relationDB`。
- 假依赖实现 `pkg` / `depend` 接口（如 `userpkg.AccountAPI`），注意接口演进后补齐方法。
- Task step 可单测 `Execute`/`Rollback`；Monitor 可单测 `Collect` / `IsOk` / `Revert`。
- StateMgr：对挂载的 `IPullStateSyncer` / `IPushStateSyncer` / `IAliveChecker` 做表驱动单测（可参考 `vane-common/module/statemgr/*_test.go`）。
- SyncPlane：hub overflow、DomainProvider cursor=0 / >0 / 软删 DELETE。

---

## 17. 改动检查清单

提交前自问：

- [ ] 新表是否已在 `Model()` 注册？索引是否在 gorm tag？
- [ ] 业务是否只用 `DataAccessor`，无裸 GORM？
- [ ] `module.go` 是否仅注册/注入，handler 与业务是否在 `internal/api`、`internal/service` 等？
- [ ] api 是否不碰 DB？service 是否未直连其它模块（经 depend/`pkg` + interface）？
- [ ] 跨模块：对方 `pkg` **或** 对方 gRPC（经本侧 `depend` + 窄接口），不碰对方 `internal/`？
- [ ] 多步幂等控制流 / 分布式事务是否用 `Task()`，而不是 Job 或裸 goroutine？
- [ ] 纯定时逻辑是否用 `Job()`，而不是 Task？
- [ ] 跨服务事件是否用 `publish` + 具名 Topic？内容推客户端是否用 `sync/pkg`？
- [ ] 模块状态：Push/Alive 是否成对（远端推、本地验活）？拉齐合并是否用 Pull？禁止自建轮询？
- [ ] 新健康/指标是否实现 `IMonitorModule`？需要自愈时是否用 `HealthCheckerAndRevertable` + 幂等 `Revert`？
- [ ] 后台协程是否用 `safe`（非裸 `go`），且绑定可取消 `context` / 有 Stop 收尾？
- [ ] 是否把「旧库搬家 / Drop」误加进启动路径？
- [ ] 业务错误 vs 系统错误是否分层正确？
- [ ] 日志是否带 `ctx`？

---

## 18. 常见错误对照

| 错误做法 | 正确做法 |
|----------|----------|
| service 里 `ensureSchema` + `AddColumn` | `Model()` + tag，交给 `InitDB` AutoMigrate |
| `GetAllRelationDB` + `db.Create` | service 字段上的 `DataAccessor` |
| Upsert 用 `OnConflict` 裸写 | `First` + `Create` 或 `Update`（必要时事务/`IfRetry`） |
| Init 里迁旧表并 Drop | 外部迁移程序；本服务只认新 schema |
| 新建 `internal/repo` | 直接 DataAccessor |
| 在 `module.go` 写 handler / 业务逻辑 | `module.go` 只注册与注入；实现放 `internal/api`、`service` 等 |
| service 直接 `import` 其它模块 / 散落 `userpkg.Xxx()` | 窄 interface + `depend`（内调对方 `pkg` 或 gRPC）注入 |
| 禁止 service 使用 proto | service **可以**用 vane-sdk / proto；「外部」指其它模块 |
| api import 其它模块 internal | 对方 `pkg` 或 gRPC；本侧经 `depend` |
| 裸 `go func(){...}()` 启动后不管 | `safe.NewWaitGroupContext` / `Pool` / `GoKeep` + `ctx` |
| 长流程当定时 Job 跑 / 无回滚裸 `go` | `module.Task()`：控制流 + 幂等 Step + Rollback |
| 把 Task 当 cron 用 | `module.Job()` 定时任务 |
| 自建 ticker 无限循环 | `Job()` / `StateMgr` / `safe.GoKeep(ctx, ...)` |
| 自建跨机广播 | `vane-common/publish` |
| 把 Sync 事件打到 publish topic | 内容变更用 `sync/pkg.Publish` |
| 自建远程↔本地状态对账循环 | `IModuleStateMgr`：Pull 合并，或 Push+Alive 成对 |
| 只挂 Push 或只挂 Alive | 远端 Push + 本地 Alive **成对**部署 |
| SyncPlane 写库前 Publish / 空 owner 广播 | 先落库再 Publish；owner 必填 |
| 在 service 里乱写 Prometheus | `IMonitorModule` + `internal/metric` |
| 健康失败只打日志 / 在 `IsOk` 里同步重修 | `NewHealthCheckerAndRevertableMetric`，由 telemetry 异步 `Revert` |
| 业务里 `fmt.Println` / 标准 log | `logger.*(ctx, ...)` |

---

## 19. 推荐阅读顺序（新人 / 新代理）

1. 本文
2. `vane-common/pkg/dataaccess/USAGE.md`
3. `vane-api-server/internal/module/user/`（module + service + model + pkg + public api）
4. `vane-common/module/define.go`、`vane-common/server/server.go`、`vane-common/dbserver/enabledb.go`
5. 协程：`vane-common/pkg/safe/goroutine.go`
6. 分布式事务（Task）：`vane-common/pkg/taskmgr/README.md`（勿与 Job 混淆）
7. 消息总线：`vane-common/publish`
8. Monitor（含自愈）：`vane-common/module/monitor` + `telemetry/collection.go`；范本 `itnlmodule/metric/discover_status.go`
9. 状态管理器：`vane-common/module/statemgr` + `vane-common/server/internal/itnlmodule`（`StateMgr`）
10. SyncPlane：`internal/module/sync` + `official/.../sync_providers.go`
11. 再改目标业务模块

当框架能力与业务需求冲突时：**先查 tlg-common 是否已有 primitive**，再考虑扩展；不要在业务模块里旁路框架。
