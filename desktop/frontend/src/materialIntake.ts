import type { MaterialIntakeProjection } from './api/kernelApi'

export function materialIntakeSummary(projection: MaterialIntakeProjection): string[] {
  return [
    admissionLabel(projection.admission_result),
    String(projection.source_snapshot_ref ?? projection.refusal_reason_class ?? '').trim(),
    (projection.available_operations ?? []).map(operationLabel).join(', '),
  ]
}

function admissionLabel(value: unknown) {
  const text = String(value ?? '').trim()
  if (text === 'admitted') return '已添加'
  if (text === 'refused') return '未添加'
  return '未知'
}

function operationLabel(value: string) {
  if (value === 'source_tree') return '查看目录'
  if (value === 'source_read') return '读取文件'
  return value
}
