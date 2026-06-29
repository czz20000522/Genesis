import type { SessionDebugExport } from './api/kernelApi'

export function debugSummary(debug: SessionDebugExport) {
  return [
    String(debug.readiness ?? debug.status ?? 'unknown'),
    String(Array.isArray(debug.steps) ? debug.steps.length : debug.step_count ?? 0),
    countSummary(debug.input_kind_counts),
    countSummary(debug.model_counts),
  ]
}

export function debugExportText(debug: SessionDebugExport) {
  return JSON.stringify(debug, null, 2)
}

function countSummary(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return 'none'
  const entries = Object.entries(value as Record<string, unknown>)
  if (entries.length === 0) return 'none'
  return entries.map(([key, count]) => `${key}: ${String(count)}`).join(', ')
}
