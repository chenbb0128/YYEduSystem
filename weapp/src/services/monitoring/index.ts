import { appEnv } from '@/config/env'

export interface ErrorContext {
  source: string
  metadata?: Record<string, unknown>
}

export interface ReportedError {
  error: unknown
  context: ErrorContext
  timestamp: number
}

export type ErrorReporter = (event: ReportedError) => void | Promise<void>

let customReporter: ErrorReporter | undefined

function logReporterFailure(error: unknown) {
  if (!appEnv.enableLog) {
    return
  }

  // eslint-disable-next-line no-console
  console.error('[monitoring:reporter-error]', error)
}

export function configureErrorReporter(reporter?: ErrorReporter) {
  customReporter = reporter
}

export function reportError(error: unknown, context: ErrorContext) {
  if (appEnv.enableLog) {
    // eslint-disable-next-line no-console
    console.error(`[${context.source}:error]`, error)
  }

  if (!customReporter) {
    return
  }

  try {
    const result = customReporter({
      error,
      context,
      timestamp: Date.now(),
    })

    if (result instanceof Promise) {
      void result.catch(logReporterFailure)
    }
  }
  catch (reporterError) {
    logReporterFailure(reporterError)
  }
}
