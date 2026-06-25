package kernel

import (
	"strings"
)

func sandboxReadinessForShellOperation(operation OperationProjection, policy ResolvedToolPolicy, blocker string) (SandboxReadinessProjection, bool) {
	blocker = strings.TrimSpace(blocker)
	if blocker != "approval_required" && !strings.HasPrefix(blocker, "sandbox_profile_unavailable") {
		return SandboxReadinessProjection{}, false
	}
	now := operation.StartedAt
	status := SandboxReadinessReady
	unavailableReason := ""
	if strings.HasPrefix(blocker, "sandbox_profile_unavailable") {
		status = SandboxReadinessUnavailable
		unavailableReason = blocker
	}
	return SandboxReadinessProjection{
		SandboxReadinessID: newID("sandbox", now),
		SessionID:          operation.SessionID,
		TurnID:             operation.TurnID,
		OperationID:        operation.OperationID,
		SandboxProfile:     strings.TrimSpace(policy.SandboxProfile),
		WorkspaceRoot:      strings.TrimSpace(policy.WorkspaceRoot),
		ExecutorAdapter:    "local_shell",
		Status:             status,
		UnavailableReason:  unavailableReason,
		CreatedAt:          now,
	}, true
}

func (k *Kernel) appendSandboxReadinessEvent(readiness SandboxReadinessProjection) error {
	eventType := "sandbox.ready"
	if readiness.Status == SandboxReadinessUnavailable {
		eventType = "sandbox.unavailable"
	}
	return k.appendEvent(StoredEvent{
		EventID:            newID("evt", readiness.CreatedAt),
		SessionID:          readiness.SessionID,
		TurnID:             readiness.TurnID,
		OperationID:        readiness.OperationID,
		SandboxReadinessID: readiness.SandboxReadinessID,
		Type:               eventType,
		CreatedAt:          readiness.CreatedAt,
		Data: EventData{
			SandboxReadiness: &readiness,
		},
	})
}
