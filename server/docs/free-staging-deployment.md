# 豆芽成长助手免费内测部署

这套方案不购买云数据库、OSS、短信或商业 HTTPS 证书，使用已有服务器、Docker、MySQL、Redis、本地图片卷和 Let's Encrypt。适用于前期自用和小规模内测，不等同于高可用正式生产环境。

## 1. 方案边界

- MySQL、Redis：运行在现有服务器的 Docker 容器中。
- 图片：保存到 `staging_uploads` 数据卷，服务重启不会丢失。
- API：只绑定服务器本机 `127.0.0.1:8080`。
- HTTPS：由 Caddy 自动申请和续期 Let's Encrypt 证书。
- 登录：家长使用微信授权；教师和管理员使用账号密码。
- 短信、OSS/COS、云 OCR：内测阶段关闭。

本方案使用 `app.env=staging`。当前后端会在 `production` 环境强制要求 S3/OSS，因此不要把本地图片存储配置标成 `production`。后续切换 OSS 时，再改用生产配置模板。

## 2. 准备服务器

服务器需要安装 Docker 和 Docker Compose，并确认：

- 域名 `xy.pdurl.cn` 的 A 记录指向服务器公网 IP；
- 80、443 端口对外开放；
- 3306、6379 不对公网开放；
- 8080 只允许本机访问。

如果 `https://xy.pdurl.cn/health` 已经可以访问，说明 API 域名和 HTTPS 入口已有基础配置，但仍需确认反向代理是否指向新的 API 容器。

## 3. 创建内测环境变量

在 `server` 目录执行：

```powershell
Copy-Item .env.staging.example .env.staging
```

然后填写 `.env.staging`：

- 三个数据库/Redis 密码；
- 两个不同的随机应用密钥；
- 首次启动管理员和平台管理员密码；
- 微信配置准备好后，再填写 AppID、AppSecret 和模板 ID。

真实 AppSecret、密码和 Token 只能放在服务器的 `.env.staging`，不能提交 GitHub。

## 4. 启动免费内测环境

在 `server` 目录执行：

```powershell
docker compose --env-file .env.staging -f compose.staging.yaml config
docker compose --env-file .env.staging -f compose.staging.yaml up -d --build
docker compose --env-file .env.staging -f compose.staging.yaml ps
```

检查 API：

```powershell
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/ready
```

`ready` 必须在 MySQL、Redis 和迁移都成功后才会通过。停止服务时不要加 `-v`，否则会删除数据卷：

```powershell
docker compose --env-file .env.staging -f compose.staging.yaml down
```

首次成功登录后，把两个 `TUOGUAN_SYSTEM_AUTH_BOOTSTRAP_*_ENABLED` 改为 `false`，再执行：

```powershell
docker compose --env-file .env.staging -f compose.staging.yaml up -d
```

## 5. 配置 HTTPS

安装 Caddy 后，将 `deployments/Caddyfile.staging.example` 复制为 Caddy 配置并按实际管理端域名修改。Caddy 负责：

- 自动申请 Let's Encrypt 证书；
- HTTP 跳转 HTTPS；
- 将 API 转发到 `127.0.0.1:8080`；
- 托管管理端静态文件。

证书申请成功后测试：

```powershell
curl https://xy.pdurl.cn/health
curl https://xy.pdurl.cn/ready
```

## 6. 配置两个前端

小程序生产构建环境写入：

```env
VITE_APP_NAME=豆芽成长助手
VITE_API_BASE_URL=https://xy.pdurl.cn/api/v1
```

管理端 `apps/web-ele/.env.production` 写入：

```env
VITE_TUOGUAN_API_URL=https://xy.pdurl.cn/api/v1
```

`VITE_*` 会进入前端构建产物，只能放公开地址或模板 ID，不能放 AppSecret、数据库密码或 Redis 密码。

## 7. 维护和备份

免费方案只有单机保障，必须至少每天备份 MySQL。停止服务、重启服务器和更新镜像时不要删除 `staging_mysql_data`、`staging_redis_data`、`staging_uploads` 数据卷。图片也需要单独备份；否则服务器硬盘损坏会导致图片丢失。

## 8. 后续升级

当用户量或图片量增加时，按以下顺序升级：

1. 本地图片切换到私有 OSS/COS；
2. 图片和 MySQL 做异地备份；
3. MySQL/Redis 改为托管服务或独立机器；
4. 将 `app.env` 切换为 `production` 并完成上线验收。
