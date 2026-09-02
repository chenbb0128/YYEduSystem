interface ImportMetaEnv {
  readonly VITE_APP_NAME?: string
  readonly VITE_API_BASE_URL?: string
  readonly VITE_API_TIMEOUT?: string
  readonly VITE_STORAGE_PREFIX?: string
  readonly VITE_ENABLE_LOG?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
