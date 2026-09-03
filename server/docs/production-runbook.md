# 豆芽成长助手上线运行手册

这份手册只描述可执行的上线步骤。真实微信 AppSecret、订阅模板 ID、HTTPS 域名、MySQL/Redis/S3 凭据必须由部署负责人通过 Secret 注入，不能写入仓库。

## 1. 上线前必须准备

- 微信公众平台：小程序 AppID/AppSecret、服务器域名、业务域名和五类订阅消息模板（接送、餐食、作业、请假、每日总结）。
- API HTTPS 域名，例如 `https://api.example.com`；小程序开发设置中将其加入 request 合法域名。
- 管理端 HTTPS 域名，例如 `https://admin.example.com`，并加入后端 CORS 白名单。
- MySQL 8.4 或兼容版本，创建只用于本系统的数据库账号；不使用 root 运行 API。
- 私有 S3/OSS/MinIO 桶，API 使用最小权限的读写账号；不要把桶设为公共读。
- Redis 7 或兼容版本，用于 worker 队列；设置密码和独立 key 前缀。
- 腾讯云短信：申请短信签名和验证码模板，准备 `SecretId`、`SecretKey`、`SmsSdkAppId`、签名和模板 ID。

## 2. 配置顺序

1. 复制 `configs/config.production.example.yaml`，或使用同名环境变量覆盖它。
2. 填写 `auth.secret` 和 `storage.url_signing_secret` 两个不同的随机密钥（至少 32 字符）。关闭 `auth.bootstrap_admin_enabled`。
3. 填写 MySQL DSN，先运行迁移：

   ```powershell
   go run ./cmd/migrate -command up -dsn "$env:TUOGUAN_SYSTEM_DATABASE_DSN"
   ```

4. 配置 S3/OSS 的 HTTPS endpoint、私有 bucket 和最小权限密钥。生产配置会拒绝 local 存储、占位符和 HTTP endpoint。
5. 填写微信 AppID、AppSecret 和五类模板 ID；按公众平台实际 keyword 修改 `subscribe_template_fields`。同时在 `D:\xilin\weapp` 的生产构建环境中填写同一组模板 ID（模板 ID 不是 Secret，但必须与公众平台审核通过的模板一致），否则小程序无法发起授权弹窗。
6. 如启用手机号验证码登录，设置 `sms.enabled=true`、`sms.provider=tencent`，并通过环境变量注入 `sms.secret_id`、`sms.secret_key`、`sms.sdk_app_id`、`sms.sign_name`、`sms.template_id` 和独立的 `sms.code_secret`。生产环境禁止 `sms.provider=local`，API 也不会返回 `debug_code`。
7. 启动 API 和 worker。worker 负责订阅消息投递重试和按周期排班生成当天待确认任务。
8. 检查 `/health` 和 `/ready`，再登录管理端创建首个管理员、教师账号及教师负责班级。

## 3. 关键业务验收

- 教师创建周一至周日排班后，worker 或管理端“生成当天任务”只生成 `draft`，不会绕过教师确认。
- 教师确认任务后，家长站内通知立即可见；已授权的家长才会异步收到微信订阅消息。
- 微信发送失败只记录 `failed` 和投递日志，不回滚接送、作业、餐食或总结业务；worker 会按配置重试。
- 短信验证码只保存 HMAC 摘要，Redis 启用时使用带前缀的 Redis key；验证码有 5 分钟有效期、60 秒重发冷却和错误次数上限。
- 家长端通过 `wx.login` 登录，服务端只信任 code2Session 返回的 OpenID；客户端提交的 OpenID 在生产环境会被忽略。
- 照片仅保存到私有对象存储，接口返回的图片地址是短时签名 URL；不能直接访问 `/uploads/*`。

## 4. 备份和恢复

Windows PowerShell：

```powershell
$env:TUOGUAN_BACKUP_MYSQL_PASSWORD = '<备份账号密码>'
.\scripts\backup-mysql.ps1 -OutputDirectory 'D:\backups\douya'
```

备份应由计划任务或云数据库自动备份每日执行，并至少保留一份异地副本。恢复前先在隔离数据库演练：

```powershell
$env:TUOGUAN_BACKUP_MYSQL_PASSWORD = '<恢复账号密码>'
.\scripts\restore-mysql.ps1 -BackupFile 'D:\backups\douya\tuoguan_system-20260903-030000.sql'
```

恢复脚本有二次确认，但仍必须先确认目标数据库；恢复后运行迁移状态、登录、学生档案、接送照片和通知中心检查。

## 5. 必须保留的安全证据

- 每次发布的 Git commit、迁移版本和配置版本。
- 管理员、教师、家长三类账号的最小权限测试记录。
- 儿童信息查看、修改、审核和照片访问的审计记录。
- 备份成功日志、SHA-256 校验文件和最近一次恢复演练结果。
- 微信订阅消息的成功、跳过、失败、重试投递日志。

## 6. 当前不能在仓库内代替完成的事项

真实凭据、域名备案/HTTPS 证书、微信公众平台模板审核、对象存储购买、真机扫码登录和微信弹窗授权都依赖外部账号或设备。没有这些条件时只能完成代码、配置校验和本地联调，不能宣称已经上线或真机通过。
