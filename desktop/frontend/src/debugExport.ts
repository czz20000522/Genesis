import type { SessionDebugExport } from './api/kernelApi'

export function debugSummary(debug: SessionDebugExport) {
  return [
    readinessLabel(String(debug.readiness ?? debug.status ?? '')),
    String(Array.isArray(debug.steps) ? debug.steps.length : debug.step_count ?? 0),
    countSummary(debug.input_kind_counts),
    countSummary(debug.model_counts),
  ]
}

function readinessLabel(readiness: unknown) {
  const value = String(readiness || '').trim().toLowerCase()
  return value === 'ready' || value === 'serving-ready' || value === 'ok' ? '已连接' : '未连接'
}

export function debugExportText(debug: SessionDebugExport) {
  return JSON.stringify(debug, null, 2)
}

function countSummary(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return '无'
  const entries = Object.entries(value as Record<string, unknown>)
  if (entries.length === 0) return '无'
  return entries.map(([key, count]) => `${key}: ${String(count)}`).join(', ')
}
