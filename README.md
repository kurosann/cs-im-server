# cs-im-server

CS IM 聚合客服系统的后端服务。当前处于可联调阶段，提供核心数据模型、业务渠道入口和转发渠道出口。

## 模块边界

服务按业务能力拆分模块，模块内部封装 `internal`，只通过模块级 `pkg` 暴露最小能力接口：

### `iam`：身份与访问控制

`iam` 只负责“谁在调用系统”，不承载业务规则。

- 初始化业务访问 Token、转发渠道 Token 和默认租户。
- 校验 Token 的状态、过期时间和撤销标记，并把 `business`、`relay` 两类身份、租户和 scope 放入请求上下文。
- 服务端随机生成动态 Token，数据库只保存 SHA-256 摘要；明文只在签发和轮换响应中返回一次。
- 提供 Token 签发、查询、禁用、启用、轮换能力，默认 scope 分别限制为 `channel:access` 和 `relay:access`。
- 提供 `AccountTokenMiddleware`、`RequirePrincipalKind` 和 `RequirePrincipalScope`，供各模块声明可访问的身份与权限。
- 持久化 `AccountToken` 模型。

管理路由挂在 `/admin/v1/iam/tokens` 下，要求 `business` 身份和 `iam:token:manage` scope。开发引导 Token 使用 `*`，仅用于初始化和联调，不要直接暴露给业务渠道。

### `gateway`：核心领域与数据所有权

`gateway` 是系统的领域核心，拥有主数据和核心流转规则，但不是对外协议入口。

- 管理业务 App、客户、会话和消息。
- 维护消息序列、消息历史和实时事件发布/订阅。
- 支持 `text`、`image`、`file`、`video` 消息；媒体只保存调用方提供的 URL 引用，不提供上传能力。
- 对客服聚合系统提供 `/admin/v1` 管理接口。
- 通过 `pkg/gateway.go` 暴露 `AppAuthorizer`、`ConversationIngress`、`MessageIngress`、`MessageEgress`、`RealtimeSubscriber` 等窄接口。

`gateway/internal/service` 只能被本模块访问。`channel` 和 `relay` 只能调用 `gateway/pkg` 中的能力门面。

### `channel`：业务渠道入口

`channel` 面向 WhatsApp、Telegram、网页客服等业务采集端，负责把客户消息从外部渠道引入系统。

- 实现 `cs-im-sdk` 中的 `ChannelGatewayService` gRPC 契约。
- 提供 `/api/v1/channels` HTTP 入口，包括建会话、发送客户消息、查询历史消息和订阅客服消息。
- 先校验业务身份和 App 授权，再调用 `gateway` 的 ingress 能力。
- 业务身份只能发送客户消息、接收客服消息，不能发送客服消息。

`channel` 不定义会话和消息持久化规则，也不保存核心领域数据。

### `relay`：转发渠道出口

`relay` 面向私有转发渠道和外部投递能力，负责把客服侧内容送出去。

- 实现 `cs-im-internal-sdk` 中的 `RelayChannelService` gRPC 契约。
- 管理转发渠道、Webhook、AI 配置和投递记录。
- 对管理侧提供 `/admin/v1` 配置接口。
- 对转发渠道提供 `/api/v1` 客服消息发送、客户消息订阅和历史消息能力。
- relay 身份只能发送客服消息、接收客户消息，不能冒充业务渠道发送客户消息。

`relay` 是出口编排层；最终的会话、客户、消息等核心数据仍归属 `gateway`。

### `system`：服务基础信息

`system` 提供服务版本、构建分支、提交号和构建时间等基础信息，不包含业务能力。

边界关系：

```text
业务渠道 -> channel -> gateway
客服聚合系统 -> gateway
relay 身份/转发渠道 -> relay -> gateway -> 外部转发渠道
```

`channel` 不直接访问 `gateway/internal/service`，只能使用 `gateway/pkg` 暴露的接口能力；`relay` 同理。

## 本地运行

```bash
make run
```

默认读取 `./config/config.yaml`。配置中 gRPC 监听 `127.0.0.1:19091`；若未显式配置 HTTP 端口，则沿用同一端口。开发时可显式覆盖：

```bash
SERVER_HOST=127.0.0.1 SERVER_PORT=8080 make run
```

数据库连接：

```bash
CS_IM_DB_DSN="user:password@tcp(host:port)/cs_im_dev?charset=utf8mb4&parseTime=true&loc=Local" \
  SERVER_PORT=8080 make run
```

通过注册入口创建租户主账号，随后通过 IAM 管理接口创建员工、角色和渠道凭证。

## 构建与测试

```bash
make test
make build
./bin/cs-im-server start -c ./config/config.yaml
./bin/cs-im-server version
```

版本接口：

```bash
curl http://127.0.0.1:8080/api/v1/version
```

健康检查、Prometheus 等遥测能力由 `cs-im-common` 内置，本工程不单独定义 `healthz` / `readyz` 路由。

## 配置

主要配置位于 `config/config.yaml`：

- `server`：gRPC、standalone 与遥测配置
- `http`：HTTP 监听地址和端口
- `log`：日志配置
- `database`：MySQL DSN

环境变量会在加载 YAML 后覆盖对应字段。

## 工程结构

```text
cmd/                          # 启动入口
config/                       # 默认配置
internal/cfg/                 # 配置加载和环境变量注入
internal/module/              # 模块注册
internal/module/<module>/internal/ # 模块私有 api、model、service
internal/module/<module>/pkg/      # 模块对外最小能力
pkg/consts/                   # 服务常量
```
## LLM gateway

Model routing and provider credentials live in the independent `config/llm.yaml` file. Leave its sample models disabled until you have set model IDs and the environment variables named by `api_key_env`. See [docs/llm-module.md](docs/llm-module.md) for providers, routes, MiniMax normalization, and configuration details.
