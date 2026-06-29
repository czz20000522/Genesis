import type { MaterialIntakeProjection } from './api/kernelApi'

export function materialIntakeSummary(projection: MaterialIntakeProjection): string[] {
  return [
    String(projection.admission_result ?? '').trim(),
    String(projection.source_snapshot_ref ?? projection.refusal_reason_class ?? '').trim(),
    (projection.available_operations ?? []).join(', '),
  ]
}
