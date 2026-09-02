import { reportError } from '@/services/monitoring'
import { useAppStore } from '@/stores'

App({
  onLaunch() {
    useAppStore().initialize()
  },
  onError(error) {
    reportError(error, { source: 'app' })
  },
  onUnhandledRejection(event) {
    reportError(event.reason, { source: 'app-unhandled-rejection' })
  },
})
