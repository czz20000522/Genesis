import type { ContextCompactionResponse } from './api/kernelApi'

export function compactionSummary(result: ContextCompactionResponse) {
  return [
    admissionLabel(result.admission_result),
    String(result.reason_class ?? result.refusal_reason_class ?? ''),
  ]
}

function admissionLabel(value: unknown) {
  const text = String(value ?? '').trim()
  if (text === 'admitted') return '已开始'
  if (text === 'refused') return '未执行'
  return '未知'
}
