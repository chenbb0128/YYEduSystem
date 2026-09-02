import type { StateTree, StoreGeneric } from 'pinia'
import { reportError } from '@/services/monitoring'
import { getStorage, removeStorage, setStorage } from '@/utils/storage'

interface PersistedStoreSnapshot {
  version: number
  state: StateTree
}

export interface PersistStoreOptions {
  key: string
  version?: number
  pick?: readonly string[]
  migrate?: (state: StateTree, fromVersion: number) => StateTree
}

function isPersistedStoreSnapshot(value: unknown): value is PersistedStoreSnapshot {
  return Boolean(
    value
    && typeof value === 'object'
    && typeof Reflect.get(value, 'version') === 'number'
    && Reflect.get(value, 'state')
    && typeof Reflect.get(value, 'state') === 'object',
  )
}

function selectState(state: StateTree, pick?: readonly string[]) {
  if (!pick?.length) {
    return { ...state }
  }

  return pick.reduce<StateTree>((result, key) => {
    if (Object.hasOwn(state, key)) {
      result[key] = state[key]
    }
    return result
  }, {})
}

function toSerializableState(state: StateTree) {
  return JSON.parse(JSON.stringify(state)) as StateTree
}

/**
 * 为原生小程序场景显式启用 Pinia 持久化。
 * 调用方应保存返回的取消函数，并在不再需要时调用。
 */
export function persistStore(store: StoreGeneric, options: PersistStoreOptions) {
  const version = options.version ?? 1
  const snapshot = getStorage<unknown>(options.key)

  if (isPersistedStoreSnapshot(snapshot)) {
    try {
      if (snapshot.version === version) {
        store.$patch(snapshot.state)
      }
      else if (options.migrate) {
        store.$patch(options.migrate(snapshot.state, snapshot.version))
      }
    }
    catch (error) {
      reportError(error, {
        source: 'store-persist',
        metadata: { operation: 'hydrate', storeId: store.$id },
      })
    }
  }

  return store.$subscribe((_mutation, state) => {
    try {
      const persistedState = toSerializableState(selectState(state, options.pick))
      setStorage<PersistedStoreSnapshot>(options.key, {
        version,
        state: persistedState,
      })
    }
    catch (error) {
      reportError(error, {
        source: 'store-persist',
        metadata: { operation: 'save', storeId: store.$id },
      })
    }
  }, {
    detached: true,
    flush: 'sync',
  })
}

export function clearPersistedStore(key: string) {
  return removeStorage(key)
}
