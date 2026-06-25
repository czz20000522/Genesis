package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrApprovalRejected = errors.New("approval rejected")

func (k *Kernel) requestApprovalForBlockedShellOperation(operation OperationProjection) error {
	if strings.TrimSpace(operation.BlockedReason) != "approval_required" {
		return nil
	}
	k.approvalMu.Lock()
	defer k.approvalMu.Unlock()

	existing, ok, err := k.approvalByOperationID(operation.SessionID, operation.OperationID)
	if err != nil {
		return err
	}
	if ok && existing.Status == ApprovalStatusPending {
		return nil
	}
	now := k.clock()
	policy := resolveToolPolicy(k.toolPolicy)
	approval := ApprovalProjection{
		ApprovalID:     newID("approval", now),
		SessionID:      operation.SessionID,
		TurnID:         operation.TurnID,
		OperationID:    operation.OperationID,
		Status:         ApprovalStatusPending,
		Tool:           operation.Tool,
		PolicySnapshot: approvalPolicySnapshot(policy),
		Effect:         approvalEffectSummaryFromOperation(operation),
		RequestedAt:    now,
		ExpiresAt:      now.Add(defaultApprovalTTL),
	}
	return k.appendApprovalEvent("approval.requested", approval)
}

func (k *Kernel) DecideApproval(ctx context.Context, req ApprovalDecisionRequest) (ApprovalProjection, error) {
	if err := validateApprovalDecisionRequest(req); err != nil {
		return ApprovalProjection{}, err
	}
	k.approvalMu.Lock()
	approval, ok, err := k.approvalByID(req.ApprovalID)
	if err != nil {
		k.approvalMu.Unlock()
		return ApprovalProjection{}, err
	}
	if !ok {
		k.approvalMu.Unlock()
		return ApprovalProjection{}, fmt.Errorf("%w: approval not found", ErrApprovalRejected)
	}
	if approval.Status != ApprovalStatusPending {
		if approval.Status == ApprovalStatusApproved && strings.TrimSpace(req.Decision) == ApprovalDecisionApproved {
			operation, ok, err := k.rawOperationByID(approval.SessionID, approval.OperationID)
			if err != nil {
				k.approvalMu.Unlock()
				return ApprovalProjection{}, err
			}
			if !ok {
				k.approvalMu.Unlock()
				return ApprovalProjection{}, fmt.Errorf("%w: approval operation evidence is missing", ErrApprovalRejected)
			}
			k.approvalMu.Unlock()
			if err := k.ensureApprovedApprovalEffect(ctx, approval, operation); err != nil {
				return cloneApprovalProjection(approval), err
			}
			return cloneApprovalProjection(approval), nil
		}
		k.approvalMu.Unlock()
		return ApprovalProjection{}, fmt.Errorf("%w: approval is already %s", ErrApprovalRejected, approval.Status)
	}
	operation, ok, err := k.rawOperationByID(approval.SessionID, approval.OperationID)
	if err != nil {
		k.approvalMu.Unlock()
		return ApprovalProjection{}, err
	}
	if !ok {
		k.approvalMu.Unlock()
		return ApprovalProjection{}, fmt.Errorf("%w: approval operation evidence is missing", ErrApprovalRejected)
	}
	now := k.clock()
	if !approval.ExpiresAt.IsZero() && now.After(approval.ExpiresAt) {
		expired := decideApprovalProjection(approval, req, ApprovalStatusExpired, "approval_expired", now)
		if err := k.appendApprovalEvent("approval.expired", expired); err != nil {
			k.approvalMu.Unlock()
			return ApprovalProjection{}, err
		}
		if err := k.appendApprovalTerminalBlockedOperation(operation, "approval_expired", now); err != nil {
			k.approvalMu.Unlock()
			return ApprovalProjection{}, err
		}
		k.approvalMu.Unlock()
		return cloneApprovalProjection(expired), fmt.Errorf("%w: approval expired", ErrApprovalRejected)
	}
	if !sameApprovalPolicySnapshot(approval.PolicySnapshot, approvalPolicySnapshot(resolveToolPolicy(k.toolPolicy))) {
		denied := decideApprovalProjection(approval, req, ApprovalStatusDenied, "policy_snapshot_mismatch", now)
		if err := k.appendApprovalEvent("approval.denied", denied); err != nil {
			k.approvalMu.Unlock()
			return ApprovalProjection{}, err
		}
		if err := k.appendApprovalTerminalBlockedOperation(operation, "policy_snapshot_mismatch", now); err != nil {
			k.approvalMu.Unlock()
			return ApprovalProjection{}, err
		}
		k.approvalMu.Unlock()
		return cloneApprovalProjection(denied), fmt.Errorf("%w: policy snapshot mismatch", ErrApprovalRejected)
	}
	if operation.Status != "blocked" || operation.BlockedReason != "approval_required" {
		denied := decideApprovalProjection(approval, req, ApprovalStatusDenied, "approval_operation_stale", now)
		if err := k.appendApprovalEvent("approval.denied", denied); err != nil {
			k.approvalMu.Unlock()
			return ApprovalProjection{}, err
		}
		if err := k.appendApprovalTerminalBlockedOperation(operation, "approval_operation_stale", now); err != nil {
			k.approvalMu.Unlock()
			return ApprovalProjection{}, err
		}
		k.approvalMu.Unlock()
		return cloneApprovalProjection(denied), fmt.Errorf("%w: approval operation is stale", ErrApprovalRejected)
	}

	switch strings.TrimSpace(req.Decision) {
	case ApprovalDecisionDenied:
		denied := decideApprovalProjection(approval, req, ApprovalStatusDenied, "approval_denied", now)
		if err := k.appendApprovalEvent("approval.denied", denied); err != nil {
			k.approvalMu.Unlock()
			return ApprovalProjection{}, err
		}
		if err := k.appendApprovalTerminalBlockedOperation(operation, "approval_denied", now); err != nil {
			k.approvalMu.Unlock()
			return ApprovalProjection{}, err
		}
		k.approvalMu.Unlock()
		return cloneApprovalProjection(denied), nil
	case ApprovalDecisionApproved:
		approved := decideApprovalProjection(approval, req, ApprovalStatusApproved, "", now)
		if err := k.appendApprovalEvent("approval.approved", approved); err != nil {
			k.approvalMu.Unlock()
			return ApprovalProjection{}, err
		}
		k.approvalMu.Unlock()
		if err := k.ensureApprovedApprovalEffect(ctx, approved, operation); err != nil {
			return cloneApprovalProjection(approved), err
		}
		return cloneApprovalProjection(approved), nil
	default:
		k.approvalMu.Unlock()
		return ApprovalProjection{}, fmt.Errorf("%w: unknown approval decision", ErrApprovalRejected)
	}
}

func (k *Kernel) ensureApprovedApprovalEffect(ctx context.Context, approval ApprovalProjection, operation OperationProjection) error {
	execReq := ShellExecRequest{
		SessionID:      operation.SessionID,
		CWD:            operation.CWD,
		Command:        operation.Command,
		TimeoutSec:     operation.TimeoutSec,
		IdempotencyKey: "approval:" + approval.ApprovalID,
		approvedByID:   approval.ApprovalID,
	}
	if k.shellTimeoutExceedsForeground(k.normalizedShellTimeoutSec(operation.TimeoutSec)) {
		_, err := k.toolGateway().InvokeShell(ctx, execReq, operation.TurnID, "", true)
		return err
	}
	_, err := k.toolGateway().ExecShell(ctx, execReq, operation.TurnID)
	return err
}

func (k *Kernel) Approvals(status string) ([]ApprovalProjection, error) {
	status = strings.TrimSpace(status)
	events, err := k.loadEvents()
	if err != nil {
		return nil, err
	}
	latest := map[string]ApprovalProjection{}
	order := []string{}
	for _, event := range events {
		if event.Data.Approval == nil {
			continue
		}
		approval := *event.Data.Approval
		if approval.ApprovalID == "" {
			approval.ApprovalID = event.ApprovalID
		}
		if approval.ApprovalID == "" {
			continue
		}
		if _, exists := latest[approval.ApprovalID]; !exists {
			order = append(order, approval.ApprovalID)
		}
		latest[approval.ApprovalID] = approval
	}
	items := make([]ApprovalProjection, 0, len(order))
	for _, id := range order {
		approval := latest[id]
		if status != "" && approval.Status != status {
			continue
		}
		items = append(items, cloneApprovalProjection(approval))
	}
	return items, nil
}

func (k *Kernel) approvalAuthorizesShellExecution(approvalID string, req ShellExecRequest, turnID string, resolved ResolvedToolPolicy) (bool, string, error) {
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return false, "approval_required", nil
	}
	approval, ok, err := k.approvalByID(approvalID)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, "approval_not_found", nil
	}
	if approval.Status != ApprovalStatusApproved {
		return false, "approval_not_approved", nil
	}
	if strings.TrimSpace(req.IdempotencyKey) != "approval:"+approvalID {
		return false, "approval_effect_mismatch", nil
	}
	if !sameApprovalPolicySnapshot(approval.PolicySnapshot, approvalPolicySnapshot(resolved)) {
		return false, "policy_snapshot_mismatch", nil
	}
	operation, ok, err := k.rawOperationByID(approval.SessionID, approval.OperationID)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, "approval_operation_missing", nil
	}
	if operation.Tool != "shell_exec" ||
		operation.SessionID != strings.TrimSpace(req.SessionID) ||
		operation.TurnID != strings.TrimSpace(turnID) ||
		operation.CWD != strings.TrimSpace(req.CWD) ||
		operation.Command != strings.TrimSpace(req.Command) ||
		k.normalizedShellTimeoutSec(operation.TimeoutSec) != k.normalizedShellTimeoutSec(req.TimeoutSec) {
		return false, "approval_effect_mismatch", nil
	}
	return true, "", nil
}

func validateApprovalDecisionRequest(req ApprovalDecisionRequest) error {
	if err := validateKernelControlToken("approval_id", strings.TrimSpace(req.ApprovalID)); err != nil || strings.TrimSpace(req.ApprovalID) == "" {
		if err != nil {
			return err
		}
		return errors.New("approval_id is required")
	}
	switch strings.TrimSpace(req.Decision) {
	case ApprovalDecisionApproved, ApprovalDecisionDenied:
	default:
		return errors.New("decision must be approved or denied")
	}
	if err := validateKernelAuthority("decision_authority", req.DecisionAuthority); err != nil {
		return err
	}
	if strings.TrimSpace(req.DecisionReason) == "" {
		return errors.New("decision_reason is required")
	}
	if err := validateKernelTextNotSecret("decision_reason", req.DecisionReason); err != nil {
		return err
	}
	if err := validateKernelRef("decision_evidence_ref", req.DecisionEvidenceRef); err != nil {
		return err
	}
	return nil
}

func decideApprovalProjection(approval ApprovalProjection, req ApprovalDecisionRequest, status string, blockedReason string, now time.Time) ApprovalProjection {
	approval.Status = status
	approval.DecidedAt = now
	approval.DecisionAuthority = strings.TrimSpace(req.DecisionAuthority)
	approval.DecisionReason = strings.TrimSpace(req.DecisionReason)
	approval.DecisionEvidenceRef = strings.TrimSpace(req.DecisionEvidenceRef)
	approval.BlockedReason = strings.TrimSpace(blockedReason)
	return approval
}

func (k *Kernel) appendApprovalEvent(eventType string, approval ApprovalProjection) error {
	createdAt := approval.RequestedAt
	if strings.TrimSpace(approval.Status) != ApprovalStatusPending && !approval.DecidedAt.IsZero() {
		createdAt = approval.DecidedAt
	}
	return k.appendEvent(StoredEvent{
		EventID:     newID("evt", createdAt),
		SessionID:   approval.SessionID,
		TurnID:      approval.TurnID,
		OperationID: approval.OperationID,
		ApprovalID:  approval.ApprovalID,
		Type:        eventType,
		CreatedAt:   createdAt,
		Data: EventData{
			Approval: &approval,
		},
	})
}

func (k *Kernel) appendApprovalTerminalBlockedOperation(operation OperationProjection, reason string, now time.Time) error {
	operation.Status = "blocked"
	operation.BlockedReason = strings.TrimSpace(reason)
	operation.EndedAt = now
	operation.ElapsedMs = operationElapsedMs(operation.StartedAt, operation.EndedAt)
	return k.appendOperationEvent(operation)
}

func (k *Kernel) approvalByID(approvalID string) (ApprovalProjection, bool, error) {
	approvalID = strings.TrimSpace(approvalID)
	events, err := k.loadEvents()
	if err != nil {
		return ApprovalProjection{}, false, err
	}
	var latest ApprovalProjection
	found := false
	for _, event := range events {
		if event.Data.Approval == nil {
			continue
		}
		approval := *event.Data.Approval
		if approval.ApprovalID == "" {
			approval.ApprovalID = event.ApprovalID
		}
		if approval.ApprovalID != approvalID {
			continue
		}
		latest = approval
		found = true
	}
	return latest, found, nil
}

func (k *Kernel) approvalByOperationID(sessionID string, operationID string) (ApprovalProjection, bool, error) {
	events, err := k.loadEvents()
	if err != nil {
		return ApprovalProjection{}, false, err
	}
	var latest ApprovalProjection
	found := false
	for _, event := range events {
		if event.SessionID != sessionID || event.Data.Approval == nil {
			continue
		}
		approval := *event.Data.Approval
		if approval.OperationID != operationID {
			continue
		}
		latest = approval
		found = true
	}
	return latest, found, nil
}

func (k *Kernel) rawOperationByID(sessionID string, operationID string) (OperationProjection, bool, error) {
	events, err := k.loadEvents()
	if err != nil {
		return OperationProjection{}, false, err
	}
	var latest OperationProjection
	found := false
	for _, event := range events {
		if event.SessionID != sessionID || event.Data.Operation == nil {
			continue
		}
		operation := *event.Data.Operation
		if operation.OperationID != operationID {
			continue
		}
		latest = operation
		found = true
	}
	return latest, found, nil
}

func approvalPolicySnapshot(policy ResolvedToolPolicy) ApprovalPolicySnapshot {
	return ApprovalPolicySnapshot{
		PermissionMode:  strings.TrimSpace(policy.PermissionMode),
		AuthorityPolicy: strings.TrimSpace(policy.AuthorityPolicy),
		SandboxProfile:  strings.TrimSpace(policy.SandboxProfile),
		ApprovalPolicy:  strings.TrimSpace(policy.ApprovalPolicy),
		WorkspaceRoot:   strings.TrimSpace(policy.WorkspaceRoot),
		ExecutorAdapter: "local_shell",
	}
}

func sameApprovalPolicySnapshot(left ApprovalPolicySnapshot, right ApprovalPolicySnapshot) bool {
	return left.PermissionMode == right.PermissionMode &&
		left.AuthorityPolicy == right.AuthorityPolicy &&
		left.SandboxProfile == right.SandboxProfile &&
		left.ApprovalPolicy == right.ApprovalPolicy &&
		left.WorkspaceRoot == right.WorkspaceRoot &&
		left.ExecutorAdapter == right.ExecutorAdapter
}

func approvalEffectSummaryFromOperation(operation OperationProjection) ApprovalEffectSummary {
	return ApprovalEffectSummary{
		Tool:           operation.Tool,
		ExecutionKind:  ToolExecutionKindSandboxedProcess,
		SideEffect:     ToolSideEffectWrite,
		CWD:            operation.CWD,
		CommandPreview: operation.Command,
		TimeoutSec:     operation.TimeoutSec,
	}
}
