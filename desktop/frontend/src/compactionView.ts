import type { ContextCompactionResponse } from './api/kernelApi'

export function compactionSummary(result: ContextCompactionResponse) {
  return [
    String(result.admission_result ?? 'unknown'),
    String(result.reason_class ?? result.refusal_reason_class ?? ''),
  ]
}
