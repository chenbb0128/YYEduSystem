import type { StateTree, StoreGeneric } from 'pinia'

interface NativePage {
  setData: (data: Record<string, unknown>) => void
}

export interface StoreSyncOptions {
  /**
   * 将状态挂载到页面 data 的指定字段下；不设置时直接展开到页面 data。
   */
  dataKey?: string
  /**
   * 仅选择页面真正需要的字段，避免无效 setData。
   */
  select?: (state: StateTree) => Record<string, unknown>
}

/**
 * 将 Pinia store 的状态同步到原生 Page 的 data。
 * 原生 WXML 不会像 Vue 模板一样自动响应 Pinia 状态，页面卸载时应调用返回的取消函数。
 */
export function syncStoreToPage(
  page: NativePage,
  store: StoreGeneric,
  options: StoreSyncOptions = {},
) {
  const select = options.select ?? (state => ({ ...state }))

  const sync = (state: StateTree) => {
    const selectedState = select(state)
    page.setData(
      options.dataKey
        ? { [options.dataKey]: selectedState }
        : selectedState,
    )
  }

  sync(store.$state)

  return store.$subscribe((_mutation, state) => {
    sync(state)
  }, {
    detached: true,
    flush: 'sync',
  })
}
