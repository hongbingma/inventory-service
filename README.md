# inventory-service

基于 **Kratos + gRPC + HTTP + sqlc + PostgreSQL** 的库存管理微服务。接口先在 `api/inventory/v1/inventory.proto` 中定义，再由 Kratos 生成 HTTP/gRPC transport 代码，业务实现放在 `internal/service`、`internal/biz`、`internal/data`，符合 Kratos 推荐的 API / Service / Biz / Data 分层。

实现参考了阿里云文章《库存合并扣减：一种基于分布式缓存的强一致性热点库存扣减方案》中的核心原则：库存扣减必须有明细单据、请求幂等、数据库条件更新兜底防超卖、编辑链路用版本号保护一致性。

## 生产可用性设计

- **防超卖**：扣减使用单条 SQL 条件更新：`available >= quantity` 时才将 `available` 转入 `locked`，依赖 PostgreSQL 行锁与原子更新保证并发安全。
- **幂等**：下单扣减以 `request_id` 唯一约束去重；取消返还/支付确认会锁定原扣减单据并按状态流转，重复调用不会重复加库存或重复计入已售。
- **一致性**：库存表约束 `total = available + locked + sold`；编辑接口要求 `expected_version`，且新总库存必须大于等于 `locked + sold`。
- **明细单据**：`inventory_deductions` 记录扣减快照与生命周期，支持订单取消返还库存、支付后确认已售。
- **双协议**：同一套 protobuf 服务同时注册到 Kratos HTTP 和 gRPC server，对外提供 REST/JSON 与 gRPC 两种访问方式。
- **可观测/部署**：提供 `/healthz`，数据库连接池参数可通过环境变量配置，包含 Dockerfile、docker-compose 和迁移 SQL。

## 目录结构

```text
api/inventory/v1/          # protobuf IDL 以及 Kratos HTTP/gRPC 生成代码
cmd/inventory-service/     # 服务启动入口
internal/service/          # Kratos service 实现，负责协议 DTO 与业务模型转换
internal/biz/              # 领域用例和领域错误
internal/data/             # PostgreSQL 仓储、事务与 sqlc 查询封装
internal/server/           # Kratos HTTP/gRPC server 注册
```

## HTTP 接口

### 初始化或重置库存

```bash
curl -X POST http://localhost:8000/v1/inventories \
  -H 'Content-Type: application/json' \
  -d '{"sku_id":1001,"total":100}'
```

### 1. 库存扣减

用户下单时调用。成功后库存从 `available` 转入 `locked`，等待后续支付确认或取消返还。

```bash
curl -X POST http://localhost:8000/v1/inventories/deduct \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"order-10001","sku_id":1001,"quantity":2}'
```

### 2. 库存增加（订单取消/支付失败返还）

```bash
curl -X POST http://localhost:8000/v1/inventories/increase \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"cancel-10001","deduction_request_id":"order-10001"}'
```

`/v1/inventories/release` 也作为同义路径保留。

### 支付确认（将锁定库存转为已售）

```bash
curl -X POST http://localhost:8000/v1/inventories/confirm \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"pay-10001","deduction_request_id":"order-10001"}'
```

### 3. 库存编辑

商家后台编辑总库存，需要携带当前版本号，避免覆盖并发扣减或其他编辑。

```bash
curl -X PUT http://localhost:8000/v1/inventories/1001 \
  -H 'Content-Type: application/json' \
  -d '{"total":120,"expected_version":3}'
```

### 4. 获取库存和已售数量

```bash
curl http://localhost:8000/v1/inventories/1001
```

## gRPC 接口

服务名：`inventory.v1.Inventory`，默认监听 `:9000`。

可使用 `grpcurl` 调用（需要本地有 proto include 路径）：

```bash
grpcurl -plaintext \
  -import-path api \
  -import-path "$(go env GOPATH)/pkg/mod/github.com/go-kratos/kratos/v2@v2.8.4/third_party" \
  -proto inventory/v1/inventory.proto \
  -d '{"request_id":"order-10001","sku_id":1001,"quantity":2}' \
  localhost:9000 inventory.v1.Inventory/DeductInventory
```

## 本地启动

```bash
docker compose up --build
```

或连接已有 PostgreSQL：

```bash
psql "$DATABASE_DSN" -f migrations/001_init.sql
HTTP_ADDR=:8000 GRPC_ADDR=:9000 DATABASE_DSN="$DATABASE_DSN" go run ./cmd/inventory-service
```

## 代码生成

### protobuf / Kratos transport

```bash
make api
```

### sqlc

`db/schema.sql` 和 `db/query.sql` 是 sqlc 输入，`internal/data/sqlc` 是生成代码。安装 sqlc 后可运行：

```bash
make sqlc
```
