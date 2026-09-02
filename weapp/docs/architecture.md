# 架构约定

这份模板的目标是作为多个微信小程序的“母版”。复制项目时只替换项目参数和业务模块，基础设施保持一致。

## 依赖方向

```text
pages / components
        ↓
features（业务模块）
        ↓
stores / services
        ↓
config / utils
```

约束：

- `config` 只负责运行环境和项目参数，不依赖业务代码。
- `utils` 保持无业务语义，不直接操作页面。
- `services` 封装请求、监控等跨项目能力，不写具体业务接口。
- `stores` 只保存真正需要跨页面共享的状态。
- `pages` 负责页面编排，不直接拼装请求头、存储 key 或 HTTP 错误。
- 业务增长后按 `src/features/<domain>` 组织接口、类型和领域 Store，避免把所有接口堆进一个目录。
- `src/app.ts` 是应用组合入口，只负责初始化 Store、请求 hooks 和监控，不放页面业务。

推荐业务模块结构：

```text
src/features/user/
├─ api.ts
├─ store.ts
├─ types.ts
└─ constants.ts
```

## 创建新项目

先复制模板，再执行：

```bash
pnpm init:project -- --name mall-weapp --title 商城小程序 --appid wx0000000000000000 --api https://api.example.com
```

可先添加 `--dry-run` 查看变更。初始化脚本会统一修改：

- `package.json` 的项目名称和描述；
- `project.config.json` 的项目名与 AppID；
- `.env.local` 的展示名称、存储前缀和 API 地址；
- `project.private.config.json` 的本地项目名。

已有本地配置默认不会被覆盖，需要覆盖时显式使用 `--force`。

## 环境与存储

所有项目差异通过 `.env.local` 注入：

- `VITE_APP_NAME`：展示名称；
- `VITE_STORAGE_PREFIX`：本地存储命名空间，多项目必须保持唯一；
- `VITE_API_BASE_URL`：API 基础地址；
- `VITE_API_TIMEOUT`：请求超时；
- `VITE_ENABLE_LOG`：是否输出基础设施错误日志。

业务代码只使用 `src/utils/storage.ts`，不要直接散落 `wx.setStorageSync`。模板会自动给 key 添加项目命名空间。

所有 `VITE_*` 内容都会进入客户端产物，不得存放服务端密钥或其他秘密。

## 请求层

业务接口统一放在领域模块中，并调用 `request`：

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

请求层负责：

- 小程序 `wx.request` 与 Web Axios 适配；
- access token 请求头；
- 超时、网络、HTTP、401、取消请求的统一错误；
- 登录失效和错误上报 hooks。

项目启动时可以连接自己的登录页和监控平台：

```ts
import { configureErrorReporter } from '@/services/monitoring'
import { configureRequestHooks } from '@/services/request'
import { clearAccessToken } from '@/utils/storage'

configureErrorReporter(event => sendToMonitoring(event))

configureRequestHooks({
  onUnauthorized() {
    clearAccessToken()
    wx.reLaunch({ url: '/pages/login/index' })
  },
})
```

业务页面只处理 `RequestError`，不依赖 Axios 细节：

```ts
import { isRequestError } from '@/services/request'

try {
  await getUserProfile()
}
catch (error) {
  if (isRequestError(error)) {
    console.warn(error.code, error.message)
  }
}
```

后端响应包裹格式通常因项目不同，例如 `{ code, data, message }`。不要写死在模板请求层，应在各项目的 API 模块或统一业务响应转换器中处理。

## 状态管理

状态放置规则：

- 仅当前页面使用：保留在 Page `data`；
- 多页面共享、需要响应式：使用 Pinia；
- 需要跨启动保存：在 Pinia 基础上显式调用 `persistStore`；
- 服务端数据优先按需请求，不要无条件复制到全局 Store。

持久化示例：

```ts
import { persistStore, useAppStore } from '@/stores'

const appStore = useAppStore()

persistStore(appStore, {
  key: 'store.app',
  version: 1,
  pick: ['hasStarted'],
})
```

`version` 变化时可以提供 `migrate` 完成旧数据迁移。不要直接持久化不可序列化对象、请求实例或页面实例。

原生 WXML 不会自动订阅 Pinia。页面需要响应 Store 时使用字段级同步，并在卸载时取消：

```ts
import { syncStoreToPage, useAppStore } from '@/stores'

let stopSync: (() => void) | undefined

Page({
  onLoad() {
    stopSync = syncStoreToPage(this, useAppStore(), {
      dataKey: 'app',
      select: state => ({
        initialized: state.initialized,
      }),
    })
  },
  onUnload() {
    stopSync?.()
  },
})
```

## 页面与分包

- 主包只保留启动页、登录页和高频页面。
- 大型独立业务放入 `subPackages`，并在 `src/subpackages/<domain>` 下维护。
- 分包可以依赖主包公共基础设施，主包不要反向依赖分包业务代码。
- 路由和分包调整后同步更新 `src/app.json` 与 `src/sitemap.json`。

## 母版边界

以下能力保留接入点，但必须由业务项目确定后实现：

- 后端响应包裹、业务错误码和用户提示策略；
- 登录、刷新 Token、401 并发重放和页面权限；
- tabBar、分享、分包、预下载和独立分包；
- 上传下载、支付、订阅消息、地图与云开发；
- 单元测试、Mock 和 DevTools E2E 的具体覆盖范围。

只有经过真实项目验证、且不带业务语义的实现才应回流母版。

## 质量门禁

本地与 CI 使用同一命令：

```bash
pnpm check
```

它会执行类型检查、ESLint、Stylelint、微信小程序构建和 Web 构建。微信开发者工具内的运行时验收使用 `pnpm screenshot -- <参数>` 和 `pnpm compare -- <参数>`；终端日志使用 `pnpm ide:logs`。
