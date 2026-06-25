package kernel

const (
	RuntimePhaseRunning    = "running"
	RuntimePhaseWaiting    = "waiting"
	RuntimePhaseEnded      = "ended"
	RuntimePhaseFinalizing = "finalizing"

	WaitReasonApprovalRequired = "approval_required"
	WaitReasonBudgetPause      = "budget_pause"
	WaitReasonUserInput        = "user_input_required"

	TerminalOutcomeSucceeded   = "succeeded"
	TerminalOutcomeFailed      = "failed"
	TerminalOutcomeCancelled   = "cancelled"
	TerminalOutcomeInterrupted = "interrupted"

	TerminalCauseTimeout                 = "timeout"
	TerminalCauseBudgetExhausted         = "budget_exhausted"
	TerminalCauseRuntimeError            = "runtime_error"
	TerminalCausePermissionDenied        = "permission_denied"
	TerminalCauseDependencyUnavailable   = "dependency_unavailable"
	TerminalCauseUserCancelled           = "user_cancelled"
	TerminalCauseCommandFailed           = "command_failed"
	TerminalCauseValidationFailed        = "validation_failed"
	TerminalCauseApprovalRequired        = "approval_required"
	TerminalCauseToolInfrastructureError = "tool_infrastructure_failed"
)

type runtimeStateAxes struct {
	Phase           string
	WaitReason      string
	TerminalOutcome string
	TerminalCause   string
}

func runtimeAxesFromOwnerOutcome(status string) runtimeStateAxes {
	switch status {
	case "", RuntimePhaseRunning, "ok":
		return runtimeStateAxes{Phase: RuntimePhaseRunning}
	case "completed", "complete", "delivered":
		return runtimeStateAxes{Phase: RuntimePhaseEnded, TerminalOutcome: TerminalOutcomeSucceeded}
	case "failed", "tool_request_invalid":
		return runtimeStateAxes{Phase: RuntimePhaseEnded, TerminalOutcome: TerminalOutcomeFailed, TerminalCause: TerminalCauseRuntimeError}
	case "blocked", "permission_denied":
		return runtimeStateAxes{Phase: RuntimePhaseEnded, TerminalOutcome: TerminalOutcomeFailed, TerminalCause: TerminalCausePermissionDenied}
	case "approval_required":
		return runtimeStateAxes{Phase: RuntimePhaseWaiting, WaitReason: WaitReasonApprovalRequired}
	case "paused", "deferred":
		return runtimeStateAxes{Phase: RuntimePhaseWaiting, WaitReason: WaitReasonBudgetPause}
	case "waiting_for_user":
		return runtimeStateAxes{Phase: RuntimePhaseWaiting, WaitReason: WaitReasonUserInput}
	case "interrupted", "interrupt_requested":
		return runtimeStateAxes{Phase: RuntimePhaseEnded, TerminalOutcome: TerminalOutcomeInterrupted, TerminalCause: TerminalCauseUserCancelled}
	case "cancelled", "canceled":
		return runtimeStateAxes{Phase: RuntimePhaseEnded, TerminalOutcome: TerminalOutcomeCancelled, TerminalCause: TerminalCauseUserCancelled}
	case "cancel_requested", "running_cancel_requested":
		return runtimeStateAxes{Phase: RuntimePhaseWaiting, WaitReason: WaitReasonUserInput}
	default:
		return runtimeStateAxes{Phase: RuntimePhaseEnded, TerminalOutcome: TerminalOutcomeFailed, TerminalCause: status}
	}
}
