package kernel

import "genesis/internal/kernel/authority"

const (
	ApprovalStatusPending  = authority.ApprovalStatusPending
	ApprovalStatusApproved = authority.ApprovalStatusApproved
	ApprovalStatusDenied   = authority.ApprovalStatusDenied
	ApprovalStatusExpired  = authority.ApprovalStatusExpired

	ApprovalDecisionApproved = authority.ApprovalDecisionApproved
	ApprovalDecisionDenied   = authority.ApprovalDecisionDenied

	SandboxReadinessReady       = authority.SandboxReadinessReady
	SandboxReadinessUnavailable = authority.SandboxReadinessUnavailable

	defaultApprovalTTL = authority.DefaultApprovalTTL
)

type ApprovalListResponse = authority.ApprovalListResponse
type ApprovalDecisionRequest = authority.ApprovalDecisionRequest
type ApprovalProjection = authority.ApprovalProjection
type ApprovalPolicySnapshot = authority.ApprovalPolicySnapshot
type ApprovalEffectSummary = authority.ApprovalEffectSummary
type SandboxReadinessProjection = authority.SandboxReadinessProjection
