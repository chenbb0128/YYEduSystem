import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from './app'

export const pinia = createPinia()

setActivePinia(pinia)

export { useAppStore }
export { syncStoreToPage } from './bindPageStore'
export type { StoreSyncOptions } from './bindPageStore'
export { clearPersistedStore, persistStore } from './persist'
export type { PersistStoreOptions } from './persist'
