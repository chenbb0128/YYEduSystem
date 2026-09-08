export interface LoadGuardOptions {
  force?: boolean
  ttl?: number
}

/**
 * 合并相邻的页面加载请求，并在短时间内复用最近一次成功结果。
 * 页面重新显示时可以调用 run；写操作成功后调用 markDirty 强制刷新。
 */
export function createLoadGuard() {
  let inFlight: Promise<void> | undefined
  let lastLoadedAt = 0
  let dirty = true

  return {
    async run(loader: () => Promise<void>, options: LoadGuardOptions = {}) {
      const ttl = Math.max(0, options.ttl ?? 30_000)
      const isFresh = !dirty && Date.now() - lastLoadedAt < ttl
      if (!options.force && isFresh) {
        return
      }
      if (inFlight) {
        return inFlight
      }

      inFlight = loader()
        .then(() => {
          lastLoadedAt = Date.now()
          dirty = false
        })
        .catch((error) => {
          dirty = true
          throw error
        })
        .finally(() => {
          inFlight = undefined
        })

      return inFlight
    },
    markDirty() {
      dirty = true
    },
    reset() {
      dirty = true
      lastLoadedAt = 0
    },
  }
}
