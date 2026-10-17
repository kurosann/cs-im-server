# cs-im-server

聚合客服系统的核心后端服务。

当前阶段提供最小可运行骨架，用来承载产品规划中的对外 API：

- 租户访问控制
- 业务 App 注册与心跳
- 客户、会话、消息
- 路由分配
- 转发渠道配置
- 转发投递记录

## 运行

```bash
make run
```

默认监听 `127.0.0.1:8080`，开发 Token 为 `dev-access-token`，默认租户为 `dev-tenant`。

可用环境变量：

```bash
export SERVER_HOST=0.0.0.0
export SERVER_PORT=8080
export CS_IM_ACCESS_TOKEN=dev-access-token
export CS_IM_TENANT_ID=dev-tenant
```

服务启动方式与 `vane-api-server` 保持一致：

```bash
make build
./bin/cs-im-server start -c ./config/config.yaml
./bin/cs-im-server version
```

## 健康检查

```bash
curl http://127.0.0.1:8080/healthz
```

构建时会注入与 `vane-api-server` 一致的版本元数据：

```bash
make build
curl -H "Authorization: Bearer dev-access-token" \
  http://127.0.0.1:8080/api/v1/version
```

创建业务 App：

```bash
curl -H "Authorization: Bearer dev-access-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"web-portal","type":"web"}' \
  http://127.0.0.1:8080/api/v1/apps/register
```

## 当前边界

- 数据暂时保存在进程内存中，用于打通 API 和模块边界。
- AI、渠道适配、WebSocket、Webhook 投递为后续接入点。
- 当前只保留网关入口，后续在 `gateway` 内拆分 `ingress` 和 `egress`。

## 工程结构

```text
cmd/                 # 启动入口、版本信息
config/              # YAML 配置
internal/cfg/        # 配置加载和环境变量注入
internal/module/     # system 与 gateway 模块注册
internal/module/gateway/internal/api/ # 网关 HTTP 控制层
pkg/consts/          # 服务常量
```
