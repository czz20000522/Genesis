import type { ApprovalProjection } from './api/kernelApi'

export function approvalSummary(approval: ApprovalProjection) {
  return [
    String(approval.status ?? '').trim() || 'unknown',
    String(approval.effect?.tool ?? '').trim() || 'unknown tool',
    String(approval.effect?.command_preview ?? approval.blocked_reason ?? '').trim() || 'no effect summary',
  ]
}
