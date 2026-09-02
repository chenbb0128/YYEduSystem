# weapp-template

面向多项目复用的原生微信小程序母版。它提供稳定的工程基础设施，不预设具体业务协议。

仓库地址：[chenbb0128/weapp-template](https://github.com/chenbb0128/weapp-template)

项目使用 `WXML + TypeScript + CSS` 编写页面，基于 `weapp-vite` 构建，同时支持微信小程序和 Web 预览。Vue 只作为 Pinia 的响应式运行时，本项目不是 Vue SFC 项目。

## 内置能力

| 类别 | 方案 |
| --- | --- |
| 构建 | weapp-vite、TypeScript、微信开发者工具联动、Web 预览 |
| 样式与组件 | Tailwind CSS 4、weapp-tailwindcss、TDesign Miniprogram |
| 状态 | Pinia、原生 Page 状态同步、带版本迁移的显式持久化 |
| 请求 | Axios、`wx.request` adapter、统一错误和可配置 hooks |
| 配置 | 类型化环境变量、项目级存储命名空间、一键初始化脚本 |
| 可观测性 | App 全局错误入口、Promise rejection、请求和存储错误上报入口 |
| 工程质量 | ESLint、Stylelint、TypeScript、双端构建、GitHub Actions |
| 验收 | 微信开发者工具日志、运行时截图、截图对比 |

母版只保留一个轻量首页、`AppPage` 和 `StateView` 两个通用组件。布局演示、图标集、Dashboard 和具体业务代码均未内置。

## 环境要求

- Node.js 20 或更高版本；CI 使用 Node.js 22。
- pnpm 11，仓库通过 `packageManager` 固定为 `11.21.0`。
- 微信开发者工具。使用 `open`、截图、对比或日志桥接前，需登录并开启服务端口。

## 快速开始

新项目建议从仓库复制一份，再用初始化脚本写入项目名、标题、AppID、API 地址和本地配置：

```bash
git clone https://github.com/chenbb0128/weapp-template.git mall-weapp
cd mall-weapp
pnpm init:project -- --name mall-weapp --title 商城小程序 --appid wx0000000000000000 --api https://api.example.com
pnpm install
pnpm dev
```

需要先预览初始化结果时：

```bash
pnpm init:project -- --name mall-weapp --title 商城小程序 --appid wx0000000000000000 --api https://api.example.com --dry-run
```

## 创建业务项目

如果已经复制好了母版目录，直接初始化项目参数：

```bash
pnpm init:project -- --name mall-weapp --title 商城小程序 --appid wx0000000000000000 --api https://api.example.com
```

然后安装依赖并启动：

```bash
pnpm install
pnpm dev
```

初始化脚本会修改：

- `package.json`：包名、版本、描述和可选作者；
- `project.config.json`：项目名、描述和 AppID；
- `.env.local`：应用名、API 地址、超时、日志开关和存储前缀；
- `project.private.config.json`：本地开发者工具配置。

常用选项：

| 选项 | 作用 |
| --- | --- |
| `--name` | 必填，npm package name |
| `--title` | 小程序展示名称，默认使用 package name |
| `--appid` | 微信 AppID，默认 `touristappid` |
| `--api` | API 基础地址，默认 `https://api.example.com` |
| `--storage-prefix` | 本地存储前缀，默认由 package name 生成 |
| `--author` | 写入新项目的 `package.json` 作者 |
| `--dry-run` | 只预览，不写文件 |
| `--force` | 覆盖已有 `.env.local` 和 `project.private.config.json` |

建议先加 `--dry-run` 检查结果。初始化完成后不要把 `.env.local` 或 `project.private.config.json` 提交到仓库。

## 目录结构

```text
.
├─ .github/workflows/ci.yml       # CI 质量门禁
├─ docs/
│  ├─ architecture.md             # 依赖方向与架构约定
│  └─ release-checklist.md        # 发布检查清单
├─ scripts/init-project.mjs       # 复制母版后的初始化脚本
├─ src/
│  ├─ components/                 # 跨业务通用原生组件
│  ├─ config/                     # 环境与项目配置
│  ├─ pages/                      # 页面编排
│  ├─ services/                   # 请求、监控等基础设施
│  ├─ stores/                     # Pinia 与原生 Page 桥接
│  ├─ types/                      # 全局和环境类型
│  ├─ utils/                      # 无业务语义工具
│  ├─ app.json
│  ├─ app.ts                      # 应用组合入口
│  └─ app.css                     # Tailwind CSS 入口与主题 token
├─ project.config.json            # 微信开发者工具公共配置
└─ weapp-vite.config.ts           # 构建、生成器和组件自动导入配置
```

业务增长后新增 `src/features/<domain>`，按领域维护接口、类型、Store 和常量：

```text
src/features/user/
├─ api.ts
├─ store.ts
├─ types.ts
└─ constants.ts
```

依赖方向保持为：`pages/components → features → stores/services → config/utils`。页面只做数据装配和交互，不直接拼请求头、存储 key 或后端错误协议。详细规则见 [架构约定](docs/architecture.md)。

## 环境变量

`.env.example` 是可提交的示例，项目实际值写入 `.env.local`。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VITE_APP_NAME` | `微信小程序` | 页面展示名称 |
| `VITE_STORAGE_PREFIX` | `weapp-template` | 存储 key 命名空间，多项目必须唯一 |
| `VITE_API_BASE_URL` | 空 | Axios API 基础地址 |
| `VITE_API_TIMEOUT` | `10000` | 请求超时，单位毫秒 |
| `VITE_ENABLE_LOG` | `true` | 是否在控制台输出基础设施错误 |

所有 `VITE_*` 值都会进入客户端构建产物，禁止放入密钥、私钥或其他服务端秘密。业务代码统一从 `src/config/env.ts` 读取配置。

## Axios 请求层

`src/services/request` 在微信环境使用 `wx.request` adapter，在 Web 环境使用 Axios 默认 adapter，并统一处理：

- API base URL 和超时；
- Bearer access token，可通过 `withAuth: false` 跳过；
- HTTP、401、网络、超时、取消和未知错误；
- `RequestError`、登录失效 hook 和错误上报 hook；
- `AbortSignal` 请求取消。

业务接口写在领域模块中：

```ts
import { request } from '@/services/request'

export interface UserProfile {
  id: string
  nickname: string
}

export function getUserProfile() {
  return request<UserProfile>({
    url: '/users/me',
    method: 'GET',
  })
}
```

`request<T>` 直接返回 `response.data`；需要状态码或响应头时使用 `requestRaw<T>`。登录失效跳转、监控平台和 Token 来源通过 `configureRequestHooks` 接入。

后端常见的 `{ code, data, message }`、刷新 Token 和并发重放策略因项目而异，不在母版中写死。文件上传、下载和云开发能力也应按目标运行端单独封装。

## Pinia 与原生 Page

Pinia 在 `src/stores/index.ts` 创建并激活。原生 WXML 不会自动响应 Pinia 状态，页面应使用 `syncStoreToPage` 选择必要字段，并在 `onUnload` 中调用返回的取消函数。首页提供了完整示例。

需要跨启动保存的状态显式启用持久化：

```ts
import { persistStore, useAppStore } from '@/stores'

const appStore = useAppStore()

const stopPersist = persistStore(appStore, {
  key: 'store.app',
  version: 1,
  pick: ['hasStarted'],
  migrate: (state, fromVersion) => state,
})
```

存储层会自动添加 `VITE_STORAGE_PREFIX`。不要持久化页面实例、请求实例、函数或其他不可序列化对象。

## 监控与通用状态

`src/services/monitoring` 默认按 `VITE_ENABLE_LOG` 输出错误。正式项目通过 `configureErrorReporter` 接入 Sentry、云开发或内部平台，并在上报前移除 Token、手机号等敏感信息。

通用组件：

- `AppPage`：统一页面最外层和基础背景；
- `StateView`：统一 `loading`、`empty`、`error` 状态及重试事件。

## 常用命令

| 命令 | 作用 |
| --- | --- |
| `pnpm dev` | 启动微信小程序开发构建 |
| `pnpm dev:open` | 启动并打开微信开发者工具 |
| `pnpm dev:web` | 启动 Web 预览 |
| `pnpm build` | 构建微信小程序到 `dist/` |
| `pnpm build:web` | 构建 Web 版本到 `dist/web/` |
| `pnpm open` | 打开微信开发者工具 |
| `pnpm g user/profile --page` | 在 `src/pages` 生成页面 |
| `pnpm g UserCard` | 在 `src/components` 生成组件 |
| `pnpm prepare:weapp` | 刷新 `.weapp-vite/` 支持文件 |
| `pnpm ide:logs` | 打开开发者工具并桥接终端日志 |
| `pnpm screenshot -- <参数>` | 采集真实小程序运行时截图 |
| `pnpm compare -- <参数>` | 执行 baseline 截图对比 |
| `pnpm typecheck` | TypeScript 检查 |
| `pnpm lint` | ESLint 检查 |
| `pnpm stylelint` | Stylelint 检查 |
| `pnpm check` | 类型、代码、样式和双端构建全量检查 |

截图验收示例：

```bash
pnpm screenshot -- --project ./dist --page pages/index/index --output .tmp/acceptance.png --json
```

截图对比示例：

```bash
pnpm compare -- --project ./dist --page pages/index/index --baseline .screenshots/baseline/index.png --diff-output .tmp/index.diff.png --max-diff-pixels 100 --json
```

## 新项目必须补齐的业务能力

以下内容会改变业务行为，母版有意只保留接入点：

1. 后端业务响应结构、错误码和提示策略；
2. 登录、刷新 Token、退出登录及 401 并发处理；
3. 页面路由、tabBar、权限和分享策略；
4. 业务分包、独立分包和预下载规则；
5. 上传下载、支付、订阅消息、地图或云开发等平台能力；
6. 与业务复杂度匹配的单元测试、Mock 和 E2E 用例；
7. 正式监控、隐私协议和数据脱敏策略。

这些能力应先在第一个真实项目中验证，再把稳定且无业务语义的部分反哺母版。

## 维护方式

- 通用基础设施修复先回到母版，再同步到各业务项目；
- 业务代码留在 `features`、`pages` 和分包中，不下沉到通用层；
- 新依赖必须有至少一个真实使用点，删除演示后同步删除依赖和脚本；
- 每次升级依赖或构建配置后执行 `pnpm check`；
- 发布前按 [发布检查清单](docs/release-checklist.md) 完成真机与隐私检查。

## 相关文档

- [weapp-vite](https://vite.icebreaker.top/)
- [weapp-tailwindcss](https://tw.icebreaker.top/)
- [TDesign Miniprogram](https://tdesign.tencent.com/miniprogram/overview)
- [Pinia](https://pinia.vuejs.org/)
- [Axios](https://axios-http.com/)
