# tuoguan-system

`tuoguan-system` 是一套面向生产实践的 Go 后端基础开发模板。它的目标不是堆功能，而是先把“长期可维护的工程底座”搭稳：启动、配置、日志、请求规范、响应规范、数据库、Redis、队列、迁移、OpenAPI、Metrics、Tracing、Docker 和测试验证都已经有基础闭环。

当前状态：第一版基础档案中心、账号认证、教师负责班级授权、接送状态闭环、请假、作业图片和家长端纵向切片已接入；微信订阅消息可通过配置启用，照片使用短时签名 URL 访问。

## 已具备能力

- API 进程：Gin + 标准库 `http.Server`，支持优雅启动/关闭。
- Worker 进程：Asynq + Redis，支持队列优先级和优雅关闭。
- 配置系统：Viper 独立实例、强类型配置、默认值、环境变量覆盖、启动前校验、敏感信息脱敏。
- 请求规范：`X-Request-ID` 校验/生成/透传、访问日志、Panic Recovery、CORS、安全响应头、请求体大小限制。
- 响应规范：统一 success/error envelope、统一业务错误码、404/405 兜底。
- 健康检查：`/health` 活性检查、`/ready` 依赖就绪检查。
- MySQL：连接池、DSN 规范化、启动 ping、readiness ping。
- Migration：Goose migration 文件和独立 `cmd/migrate` 命令。
- SQL 生成：sqlc 查询与生成代码目录。
- Redis：连接池、启动 ping、readiness ping、统一 key prefix builder。
- OpenAPI：`api/openapi.yaml` 描述当前业务接口和统一响应/错误结构。
- 基础档案：以学生档案为日常唯一入口；填写学校、年级、班级和可选托管班名称即可自动归类，支持查询、筛选和编辑。学校、学期、学校班级和托管班仍保留为高级设置数据。
- 每日接送：按学校班级创建当天任务，自动生成在托学生名单，支持出发、完成和按学生点名。
- 接送状态：支持校门口接到、到班核对、未到班异常、自行到托管班、家长临时接走、请假、未找到和其他异常；未完成到班核对的名单不能完成任务。
- 接送留痕：校门口接到必须关联照片；每次状态变化都会追加事件历史，并生成待推送家长通知记录。
- 照片上传：本地开发支持 JPEG、PNG、WebP 图片上传，单张最大 5 MiB；照片通过短时签名 URL 访问，不能直接枚举上传目录。
- 家长端：家长使用微信登录后可扫描教师分享的官方班级小程序码，自动带入班级并只填写孩子姓名；教师审核通过后自动建档并绑定，同时支持申请状态、接送事件、现场照片和通知查询。
- 账号认证：管理员/教师使用账号密码登录，家长使用微信登录；业务路由按家长与教职工角色隔离，Token 支持刷新。
- 用户管理：管理员可在后台创建、编辑、停用教师账号并设置角色。
- 教师班级授权：管理员/编辑者可配置教师负责的学校班级；教师登录后只能查看和操作已分配班级。
- 作业管理：按学校班级批量布置作业，也可只选部分学生；教师逐项填写完成、订正或未交结果，家长端按孩子查看。
- 作业图片：教师端可为一条班级作业上传最多 9 张图片，家长端随作业查看；单张图片最大 5 MiB。
- 请假流程：家长提交请假申请，老师可在接送卡片中登记口头请假；家长申请由管理端/老师审核，老师代记记录直接生效，审核通过的日期会自动从当天接送名单排除。
- 本地开发：未启用 MySQL 时自动使用内存存储，配置 MySQL 后自动切换到 sqlc + MySQL 持久化。
- Metrics：Prometheus `/metrics`，记录 HTTP 请求数、耗时、状态码、响应大小、in-flight 请求。
- Tracing：OpenTelemetry TraceContext 传播，支持 stdout 和 OTLP gRPC exporter，日志关联 `trace_id` / `span_id`。
- Docker：API / Worker / Migrate 多目标镜像，MySQL + Redis + API + Worker compose 环境。
- 测试验证：已有单元测试，`go test ./...`、`go build ./...`、`go vet ./...` 已通过。

## 技术栈

| 类别 | 选型 |
|---|---|
| Go | 1.24.3 |
| HTTP | Gin + `net/http` |
| 配置 | Viper + mapstructure |
| 日志 | `log/slog` JSON |
| 数据库 | MySQL 8.4+ / `database/sql` |
| SQL 生成 | sqlc |
| 数据库迁移 | Goose |
| Redis | go-redis |
| 异步任务 | Asynq |
| OpenAPI | OpenAPI 3.0.3 |
| Metrics | Prometheus client_golang |
| Tracing | OpenTelemetry |
| 交付 | Docker / Docker Compose |

Go 工具通过 `go.mod` 的 `tool` 指令固定：

- `sqlc` v1.30.0
- `goose` v3.24.3

## 作为新项目模板使用

拿这个仓库开新项目时，建议先做这几步：

1. 修改 module path。

   ```bash
   go mod edit -module your-company/your-project
   ```

2. 全局替换示例 import path。

   ```text
   github.com/chenbb0128/tuoguan-system-server -> your-company/your-project
   ```

3. 修改项目名和命名空间。

   - `configs/config.example.yaml` 里的 `app.name`
   - `redis.key_prefix`
   - `observability.metrics.namespace`
   - `compose.yaml` 里的数据库名、用户名、Redis key prefix
   - README 中的项目名

4. 初始化 Git。

   ```bash
   git init
   git add .
   git commit -m "init go backend template"
   ```

5. 按业务方案继续扩展模块。

当前基础档案、接送和家长接口使用默认组织（ID 为 1）。本地无 MySQL 时默认管理员为 `admin / 123456`，首次启动后请立即修改或停用该密码。手机号验证码在 local/dev/test 环境默认使用随机本地测试码（接口响应的 `debug_code` 仅用于本地开发），不会再接受固定验证码；生产环境需配置腾讯云短信并设置 `sms.enabled=true`、`sms.provider=tencent`，或使用微信登录。

## 目录结构

```text
tuoguan-system/
├── api/                         # OpenAPI 等 API 契约文件
│   └── openapi.yaml
├── cmd/                         # 进程入口，只负责启动
│   ├── api/
│   ├── migrate/
│   └── worker/
├── configs/                     # 示例配置，不放敏感配置
│   └── config.example.yaml
├── database/
│   ├── migrations/              # Goose migration
│   └── queries/                 # sqlc SQL 查询
├── deployments/                 # Dockerfile 等部署构建文件
├── internal/
│   ├── app/                     # 应用组装和生命周期管理
│   ├── config/                  # 强类型配置、加载、校验
│   ├── modules/                 # identity、masterdata、assignment、pickup、homework、parent
│   ├── platform/                # 基础设施适配器
│   │   ├── database/
│   │   ├── logging/
│   │   ├── metrics/
│   │   ├── queue/
│   │   ├── redis/
│   │   ├── requestid/
│   │   └── telemetry/
│   ├── transport/               # HTTP 协议层
│   │   └── httpapi/
│   └── workers/                 # Asynq task handler 注册入口
├── compose.yaml
├── Makefile
├── sqlc.yaml
├── go.mod
└── README.md
```

## 快速启动

默认配置不启用 MySQL、Redis、Worker；Metrics 默认启用，所以可以直接运行档案、接送、作业和家长 API：

```bash
go run ./cmd/api
```

检查基础端点：

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/metrics
curl -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"123456"}' http://localhost:8080/api/v1/auth/login
# 将登录响应中的 data.accessToken 放入 Authorization: Bearer <token>
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/summary
curl -H "Authorization: Bearer <token>" "http://localhost:8080/api/v1/pickup-operations?date=2026-09-01"
curl -X POST -H "Content-Type: application/json" -d '{"code":"local-parent-demo"}' http://localhost:8080/api/v1/auth/parent/wechat
# 本地手机号验证码：先请求验证码，从响应 data.debug_code 读取随机测试码，再调用 phone-login
curl -X POST -H "Content-Type: application/json" -d '{"phone":"13800000000"}' http://localhost:8080/api/v1/auth/phone-code
curl -X POST -H "Content-Type: application/json" -d '{"phone":"13800000000","code":"<debug_code>"}' http://localhost:8080/api/v1/auth/phone-login
```

未启用 MySQL 时，档案、接送任务、事件和通知都保存在内存中，服务重启后会清空；上传照片写入 `storage.upload_dir`（默认 `data/uploads`），Compose 使用独立卷持久化。

如果要启动完整本地依赖：

```bash
docker compose up -d --build
```

compose 会启动：

- MySQL
- Redis
- migrate
- api
- worker

查看日志：

```bash
make docker-logs
```

关闭环境：

```bash
make docker-down
```

## 常用命令

```bash
make run              # 启动 API
make worker           # 启动 Worker
make migrate          # 执行 migration up
make migrate-status   # 查看 migration 状态
make sqlc             # 生成 sqlc 代码
make test             # 单元测试
make build            # 编译所有包
make vet              # go vet
make verify           # fmt + sqlc + test + build + vet
```

如果没有 Make，也可以直接使用 Go 命令：

```bash
go run ./cmd/api
go run ./cmd/worker
go run ./cmd/migrate -command up
go tool sqlc generate
go test ./...
go build ./...
go vet ./...
```

## 配置说明

配置默认值在代码中，示例文件在：

```text
configs/config.example.yaml
```

启动时指定配置文件：

```bash
go run ./cmd/api -config ./configs/config.example.yaml
```

环境变量统一使用 `TUOGUAN_SYSTEM_` 前缀。点号配置路径会转换为下划线，例如：

```bash
TUOGUAN_SYSTEM_HTTP_ADDR=:9090
TUOGUAN_SYSTEM_LOG_LEVEL=debug
TUOGUAN_SYSTEM_DATABASE_ENABLED=true
TUOGUAN_SYSTEM_DATABASE_DSN='tuoguan_system:tuoguan_system_pass@tcp(127.0.0.1:3306)/tuoguan_system'
TUOGUAN_SYSTEM_REDIS_ENABLED=true
TUOGUAN_SYSTEM_REDIS_ADDR=127.0.0.1:6379
TUOGUAN_SYSTEM_OBSERVABILITY_TRACING_ENABLED=true
```

本地私有配置建议创建：

```text
configs/config.local.yaml
```

该文件已被 `.gitignore` 忽略，不要提交真实密码、Token、生产 DSN。

## HTTP 规范

### Request ID

所有请求都会带有 `X-Request-ID`：

- 如果客户端传入合法值，则继续透传。
- 如果缺失或非法，服务端自动生成。
- 响应头会返回最终使用的 `X-Request-ID`。
- 日志、错误排查、Trace 关联都使用它。

### 统一成功响应

除 `/metrics` 和 `204 No Content` 外，JSON 成功响应统一为：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

### 统一错误响应

```json
{
  "code": 10001,
  "message": "请求不合法",
  "details": [
    {
      "field": "username",
      "reason": "必填"
    }
  ]
}
```

当前预置错误码：

| Code | 含义 |
|---:|---|
| 0 | 成功 |
| 10001 | Bad Request |
| 10002 | Unsupported Media Type |
| 10003 | Not Acceptable |
| 10004 | Payload Too Large |
| 10005 | Not Found |
| 10006 | Method Not Allowed |
| 10007 | Validation Failed |
| 20001 | Unauthorized |
| 50000 | Internal Error |
| 50001 | Dependency Unavailable |

## 基础端点

| Method | Path | 说明 |
|---|---|---|
| GET | `/health` | 活性检查，不访问外部依赖 |
| GET | `/ready` | 就绪检查，会检查已启用的 MySQL / Redis |
| GET | `/metrics` | Prometheus 指标，不走 JSON envelope |
| GET/POST | `/api/v1/schools` | 学校档案 |
| GET/POST | `/api/v1/academic-terms` | 学年学期档案 |
| GET/POST | `/api/v1/school-classes` | 学校班级档案 |
| GET/POST | `/api/v1/care-classes` | 托管班档案 |
| GET/POST/PUT | `/api/v1/students` | 学生档案 |
| POST | `/api/v1/students/profile` | 通过名称填写学生档案并自动创建/复用分类 |
| PUT | `/api/v1/students/:id/profile` | 通过名称编辑学生档案并自动归类 |
| GET/POST | `/api/v1/pickup-operations` | 查询/创建每日接送任务 |
| GET/POST/PUT | `/api/v1/pickup-schedules` | 查询、新增、编辑周期接送排班 |
| POST | `/api/v1/pickup-schedules/generate` | 按排班生成当天待确认任务 |
| GET | `/api/v1/pickup-operations/:id/students` | 查询接送名单 |
| POST | `/api/v1/pickup-operations/:id/start` | 开始接送任务 |
| POST | `/api/v1/pickup-operations/:id/finish` | 完成接送任务 |
| POST | `/api/v1/pickup-operations/:id/students/:student_id/status` | 登记学生状态 |
| GET | `/api/v1/pickup-operations/:id/events` | 查询接送事件历史 |
| POST | `/api/v1/pickup-operations/:id/handover` | 途中交接给同班已授权教师 |
| GET | `/api/v1/pickup-operations/:id/handoff-teachers` | 查询可接手教师 |
| GET | `/api/v1/pickup-operations/:id/handoffs` | 查询接送交接历史 |
| GET | `/api/v1/notifications` | 查询待推送通知记录 |
| POST | `/api/v1/uploads/pickup` | 上传接送照片 |
| POST | `/api/v1/uploads/homework` | 上传作业图片 |
| POST | `/api/v1/parent/bindings` | 旧版家长绑定兼容接口（新流程不使用） |
| GET | `/api/v1/class-invites/qrcode?school_class_id=:id` | 教师生成自己负责班级的官方微信小程序码 |
| GET | `/api/v1/parent/class-invites/:token` | 家长扫码后查询邀请班级信息 |
| GET | `/api/v1/parent/me` | 查询当前家长和孩子 |
| GET | `/api/v1/parent/students/:student_id/pickup-events` | 家长查询孩子接送动态 |
| GET | `/api/v1/parent/notifications` | 家长查询自己的通知 |
| GET | `/api/v1/parent/leave-requests` | 查询家长请假记录 |
| POST | `/api/v1/parent/students/:student_id/leave-requests` | 提交家长请假 |
| GET | `/api/v1/leave-requests` | 管理端查询请假申请 |
| POST | `/api/v1/leave-requests/:id/review` | 审核请假申请 |
| POST | `/api/v1/leave-requests/teacher` | 老师代记口头请假 |
| GET/POST | `/api/v1/teacher-assignments` | 查询/新增教师负责班级 |
| PUT | `/api/v1/teacher-assignments/:id` | 启用或停用教师班级分配 |
| GET/POST | `/api/v1/homework-tasks` | 查询/布置作业 |
| GET | `/api/v1/homework-tasks/:id/students` | 查询学生作业完成情况 |
| POST | `/api/v1/homework-tasks/:id/students/:student_id/review` | 批改学生作业 |
| GET | `/api/v1/parent/students/:student_id/homework` | 家长查询孩子作业和批改结果 |

## 每日接送闭环

典型流程如下：

1. 工作人员直接在学生档案中填写学校、年级、班级和可选托管班，系统自动复用或创建分类；管理端不再要求先逐项维护分类。
2. 管理端或教师端按学生档案中的学校班级、托管班和在托状态创建当天任务；也可以通过周期排班自动生成待确认任务，或通过 `student_ids` 指定当天实际接送学生。
3. 老师出发前核对名单、执行教师和预计出发时间，确认任务后家长会收到今日接送安排通知，再调用 `start`。
4. 老师逐个登记学生状态：校门口接到必须先上传照片；接到后还要确认到班，未到班和异常必须补充说明；自行到托管班、家长临时接走、请假和未找到可以直接登记。到班后可以登记已离班、中途离班或异常，并填写说明。
5. 每次登记会保留当前状态、追加不可覆盖的接送事件，并产生通知记录；接送途中可交接给同班已授权教师，交接人、时间和说明会单独留痕。
6. 所有学生都已处理后调用 `finish`；仍有 `planned`、`picked_up`、`not_arrived` 或 `abnormal` 学生时服务端会拒绝完成。

接送和作业通知默认先落库到消息中心；配置 `wechat.subscribe_templates` 后，系统会异步尝试发送微信订阅消息，模板字段按实际模板配置，结果回写为 `sent`、`skipped` 或 `failed`，失败会按 worker 配置重试，且不会回滚业务。照片上传接口返回的原始路径仅供内部保存，查询接送动态、餐食或作业时会返回 15 分钟有效的签名 URL。直接访问 `/uploads/*path`、过期或篡改签名都会被拒绝。家长端新流程通过微信登录后扫描教师分享的班级小程序码，提交孩子姓名即可申请入班，不需要学生档案编号；审核通过后自动建档、绑定并通知家长，旧绑定接口和旧邀请链接仅保留兼容。教师班级授权通过 `/api/v1/teacher-assignments` 配置，教师接送、请假、作业和排班接口会自动按授权班级收敛范围。

生产配置和验收材料：

- `configs/config.production.example.yaml`
- `docs/production-runbook.md`
- `docs/privacy-policy.md`
- `docs/real-device-test-checklist.md`

## OpenAPI

OpenAPI 文件：

```text
api/openapi.yaml
```

当前描述基础设施、基础档案、教师授权、每日接送、作业和家长接口，以及统一响应/错误结构。后续新增业务接口时，要求同步更新 OpenAPI。

如果本机安装了 Redocly，可以检查规范：

```bash
make openapi-check
```

## Metrics

Metrics 默认启用：

```yaml
observability:
  metrics:
    enabled: true
    path: /metrics
    namespace: tuoguan_system
```

核心指标：

- `tuoguan_system_http_requests_total`
- `tuoguan_system_http_request_duration_seconds`
- `tuoguan_system_http_response_size_bytes`
- `tuoguan_system_http_in_flight_requests`
- Go runtime / process 默认指标

注意：指标 label 使用 Gin route template，例如 `/api/v1/users/:id`，不要把用户 ID、订单号、手机号等高基数字段写进 label。

## Tracing

Tracing 默认关闭，避免本地输出噪音或依赖外部 collector。

本地 stdout 调试：

```bash
TUOGUAN_SYSTEM_OBSERVABILITY_TRACING_ENABLED=true \
TUOGUAN_SYSTEM_OBSERVABILITY_TRACING_EXPORTER=stdout \
go run ./cmd/api
```

接 OpenTelemetry Collector：

```bash
TUOGUAN_SYSTEM_OBSERVABILITY_TRACING_ENABLED=true \
TUOGUAN_SYSTEM_OBSERVABILITY_TRACING_EXPORTER=otlp \
TUOGUAN_SYSTEM_OBSERVABILITY_TRACING_ENDPOINT=localhost:4317 \
TUOGUAN_SYSTEM_OBSERVABILITY_TRACING_INSECURE=true \
go run ./cmd/api
```

HTTP 中间件会读取并传播 W3C TraceContext：

- `traceparent`
- `tracestate`

访问日志会在 tracing 启用时输出：

- `request_id`
- `trace_id`
- `span_id`

## 数据库与迁移

Migration 文件目录：

```text
database/migrations
```

SQL 查询目录：

```text
database/queries
```

sqlc 生成目录：

```text
internal/platform/database/sqlc
```

常用命令：

```bash
go run ./cmd/migrate -command status
go run ./cmd/migrate -command up
go run ./cmd/migrate -command down
go run ./cmd/migrate -command redo
go tool sqlc generate
```

也可以临时覆盖 DSN：

```bash
go run ./cmd/migrate -command up -dsn 'tuoguan_system:tuoguan_system_pass@tcp(127.0.0.1:3306)/tuoguan_system'
```

约定：

- migration 文件只追加，不随意修改已发布 migration。
- SQL 查询写在 `database/queries`，生成代码不手改。
- MySQL DSN 会自动规范化为 `parseTime=true`、`charset=utf8mb4`、UTC 时间。

## Redis 与队列

Redis 用途：

- 缓存
- 会话/临时状态
- 限流状态
- Asynq 任务队列

Redis key 必须通过 `internal/platform/redis.KeyBuilder` 构建，避免 key 命名散落。

Worker 使用 Asynq。当前 `internal/workers/mux.go` 不注册任何业务任务；后续业务任务统一在 `internal/workers` 下注册。

Worker 默认关闭：

```yaml
worker:
  enabled: false
```

启用 Worker 时必须同时启用 Redis。

## 业务模块接入规范

新增业务模块建议放在：

```text
internal/modules/<module-name>/
```

推荐结构：

```text
internal/modules/example/
├── model.go          # 领域模型
├── errors.go         # 领域错误
├── ports.go          # 模块需要的小接口
├── service.go        # 业务服务
├── api/              # HTTP handler / DTO / routes
└── mysqlrepo/        # MySQL adapter
```

依赖规则：

- Handler 只做协议转换，不写业务逻辑。
- Service 不依赖 Gin、sqlc、`*sql.DB`、`*sql.Tx`、go-redis、Asynq。
- 模块定义自己需要的 Port，由基础设施 adapter 实现。
- sqlc 生成类型不能直接返回到 HTTP response。
- 事务边界放在 service 层，由 transaction helper 协调。
- 队列任务通过业务 Port 发布，不在 service 中直接依赖 Asynq client。

## 测试规范

单元测试默认不依赖外部 MySQL / Redis。

常规验证：

```bash
go test ./...
go build ./...
go vet ./...
```

提交前建议跑：

```bash
make verify
```

后续如果增加集成测试，建议使用 build tag：

```bash
go test -tags=integration ./...
```

## 安全与提交规范

不要提交：

- `.env`
- `configs/config.local.yaml`
- 生产数据库 DSN
- Redis 密码
- Token 密钥
- TLS 私钥
- 本地日志、缓存、构建产物

日志里不要记录：

- 明文密码
- 完整 Token
- Cookie Secret
- 数据库密码
- Redis 密码
- 身份证、手机号等敏感信息

## Docker

构建 API 镜像：

```bash
docker build -f deployments/Dockerfile --build-arg TARGET=api -t tuoguan-system-api .
```

构建 Worker 镜像：

```bash
docker build -f deployments/Dockerfile --build-arg TARGET=worker -t tuoguan-system-worker .
```

构建 Migrate 镜像：

```bash
docker build -f deployments/Dockerfile --build-arg TARGET=migrate -t tuoguan-system-migrate .
```

本地完整环境：

```bash
docker compose up -d --build
```

## 当前仍依赖外部条件的内容

这些不是遗漏，而是刻意等待真实业务边界或外部配置后再做：

- 真实微信 AppSecret、模板审核、HTTPS 合法域名和真机授权
- 真实云对象存储、MySQL/Redis 账号及备份恢复演练
- 运营主体补全隐私政策的联系人、保存期限和客服渠道
- CI/CD 流水线、Testcontainers 集成测试和独立部署平台

## 后续建议

如果继续把它打磨成团队级模板，我建议下一步做：

1. 一键重命名脚本：自动替换 module path、项目名、配置前缀。
2. GitHub Actions / GitLab CI：自动跑 `go test`、`go vet`、`go build`、OpenAPI lint。
3. Swagger UI：开发环境可视化查看 OpenAPI。
4. 集成测试：用 Testcontainers 跑真实 MySQL / Redis。
5. 在真实微信、MySQL、对象存储和 HTTPS 环境完成发布前验收。
