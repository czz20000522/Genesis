package kernel

import (
	"errors"
	"fmt"
	"strings"
)

type sessionProjectionBuilder struct {
	projection    SessionProjection
	turnByID      map[string]int
	jobByID       map[string]int
	approvalByID  map[string]int
	sandboxByID   map[string]int
	workByID      map[string]int
	candidateByID map[string]int
}

func projectSessionProjection(sessionID string, events []StoredEvent) (SessionProjection, error) {
	builder := newSessionProjectionBuilder(sessionID)
	for _, event := range events {
		if event.SessionID != sessionID {
			continue
		}
		builder.appendRawEvent(event)
		if err := builder.applyOwnerEvent(event); err != nil {
			return builder.projection, err
		}
	}
	if len(builder.projection.Events) == 0 {
		return SessionProjection{}, ErrSessionNotFound
	}
	return builder.projection, nil
}

func newSessionProjectionBuilder(sessionID string) *sessionProjectionBuilder {
	return &sessionProjectionBuilder{
		projection: SessionProjection{
			SessionID:        sessionID,
			Turns:            []TurnProjection{},
			Operations:       []OperationProjection{},
			Jobs:             []JobProjection{},
			Approvals:        []ApprovalProjection{},
			SandboxReadiness: []SandboxReadinessProjection{},
			Works:            []WorkProjection{},
			MemoryCandidates: []MemoryCandidateProjection{},
			Events:           []EventProjection{},
		},
		turnByID:      map[string]int{},
		jobByID:       map[string]int{},
		approvalByID:  map[string]int{},
		sandboxByID:   map[string]int{},
		workByID:      map[string]int{},
		candidateByID: map[string]int{},
	}
}

func (b *sessionProjectionBuilder) appendRawEvent(event StoredEvent) {
	b.projection.Events = append(b.projection.Events, EventProjection{
		EventID:            event.EventID,
		TurnID:             event.TurnID,
		OperationID:        event.OperationID,
		JobID:              event.JobID,
		WorkID:             event.WorkID,
		CandidateID:        event.CandidateID,
		ApprovalID:         event.ApprovalID,
		SandboxReadinessID: event.SandboxReadinessID,
		Type:               event.Type,
		CreatedAt:          event.CreatedAt,
		Data:               inspectionEventData(event.Data),
	})
}

func (b *sessionProjectionBuilder) applyOwnerEvent(event StoredEvent) error {
	switch event.Type {
	case "turn.submitted", "model.final", "turn.paused", "turn.failed", "assistant.interrupted":
		b.applyTurnEvent(event)
	case "operation.running", "operation.completed", "operation.failed", "operation.blocked", "operation.interrupted", "operation.tool_infrastructure_failed":
		b.applyOperationEvent(event)
	case "job.started", "job.output", "job.cancel_requested", "job.completed", "job.failed", "job.cancelled":
		b.applyJobEvent(event)
	case "approval.requested", "approval.approved", "approval.denied", "approval.expired":
		b.applyApprovalEvent(event)
	case "sandbox.ready", "sandbox.unavailable":
		b.applySandboxReadinessEvent(event)
	case "work.submitted", "work.canceled":
		return b.applyWorkEvent(event)
	case "memory.candidate.created", "memory.candidate.approved", "memory.candidate.rejected", "memory.candidate.superseded":
		return b.applyMemoryCandidateEvent(event)
	}
	return nil
}

func (b *sessionProjectionBuilder) applyApprovalEvent(event StoredEvent) {
	if event.Data.Approval == nil {
		return
	}
	approval := *event.Data.Approval
	if approval.ApprovalID == "" {
		approval.ApprovalID = event.ApprovalID
	}
	if approval.ApprovalID == "" {
		return
	}
	idx, ok := b.approvalByID[approval.ApprovalID]
	if ok {
		b.projection.Approvals[idx] = approval
		return
	}
	b.approvalByID[approval.ApprovalID] = len(b.projection.Approvals)
	b.projection.Approvals = append(b.projection.Approvals, approval)
}

func (b *sessionProjectionBuilder) applySandboxReadinessEvent(event StoredEvent) {
	if event.Data.SandboxReadiness == nil {
		return
	}
	readiness := *event.Data.SandboxReadiness
	if readiness.SandboxReadinessID == "" {
		readiness.SandboxReadinessID = event.SandboxReadinessID
	}
	if readiness.SandboxReadinessID == "" {
		return
	}
	idx, ok := b.sandboxByID[readiness.SandboxReadinessID]
	if ok {
		b.projection.SandboxReadiness[idx] = readiness
		return
	}
	b.sandboxByID[readiness.SandboxReadinessID] = len(b.projection.SandboxReadiness)
	b.projection.SandboxReadiness = append(b.projection.SandboxReadiness, readiness)
}

func (b *sessionProjectionBuilder) applyTurnEvent(event StoredEvent) {
	switch event.Type {
	case "turn.submitted":
		b.turnByID[event.TurnID] = len(b.projection.Turns)
		b.projection.Turns = append(b.projection.Turns, TurnProjection{
			TurnID:           event.TurnID,
			IdempotencyKey:   event.Data.IdempotencyKey,
			Phase:            RuntimePhaseRunning,
			InputItems:       event.Data.InputItems,
			IngressRisks:     event.Data.IngressRisks,
			ModelInputKinds:  event.Data.ModelInputKinds,
			RecalledMemories: event.Data.RecalledMemories,
			StartedAt:        event.CreatedAt,
		})
	case "model.final":
		idx, ok := b.turnByID[event.TurnID]
		if !ok {
			return
		}
		b.projection.Turns[idx].Phase = RuntimePhaseEnded
		b.projection.Turns[idx].TerminalOutcome = TerminalOutcomeSucceeded
		b.projection.Turns[idx].TerminalCause = ""
		if event.Data.Final != nil {
			b.projection.Turns[idx].FinalMessage = *event.Data.Final
		}
		b.projection.Turns[idx].CompletedAt = event.CreatedAt
	case "turn.paused":
		idx, ok := b.turnByID[event.TurnID]
		if !ok {
			return
		}
		b.projection.Turns[idx].Phase = RuntimePhaseWaiting
		b.projection.Turns[idx].WaitReason = WaitReasonBudgetPause
		if event.Data.TurnPause != nil {
			b.projection.Turns[idx].Pause = event.Data.TurnPause
			if strings.TrimSpace(event.Data.TurnPause.WaitReason) != "" {
				b.projection.Turns[idx].WaitReason = event.Data.TurnPause.WaitReason
			}
		}
		b.projection.Turns[idx].CompletedAt = event.CreatedAt
	case "turn.failed":
		idx, ok := b.turnByID[event.TurnID]
		if !ok {
			return
		}
		b.projection.Turns[idx].Phase = RuntimePhaseEnded
		b.projection.Turns[idx].TerminalOutcome = TerminalOutcomeFailed
		if event.Data.TurnError != nil {
			b.projection.Turns[idx].Error = event.Data.TurnError
			b.projection.Turns[idx].TerminalCause = event.Data.TurnError.Code
		}
		if b.projection.Turns[idx].TerminalCause == "" {
			b.projection.Turns[idx].TerminalCause = TerminalCauseRuntimeError
		}
		b.projection.Turns[idx].CompletedAt = event.CreatedAt
	case "assistant.interrupted":
		idx, ok := b.turnByID[event.TurnID]
		if !ok {
			return
		}
		b.projection.Turns[idx].Phase = RuntimePhaseEnded
		b.projection.Turns[idx].TerminalOutcome = TerminalOutcomeInterrupted
		b.projection.Turns[idx].TerminalCause = TerminalCauseUserCancelled
		if event.Data.TurnInterruption != nil {
			b.projection.Turns[idx].Interruption = event.Data.TurnInterruption
			b.projection.Turns[idx].Error = &TurnError{Code: "turn_interrupted", Message: "turn was interrupted"}
		}
		b.projection.Turns[idx].CompletedAt = event.CreatedAt
	}
}

func (b *sessionProjectionBuilder) applyOperationEvent(event StoredEvent) {
	if event.Data.Operation == nil {
		return
	}
	operation := *event.Data.Operation
	for i := range b.projection.Operations {
		if b.projection.Operations[i].OperationID == operation.OperationID {
			b.projection.Operations[i] = operation
			return
		}
	}
	b.projection.Operations = append(b.projection.Operations, operation)
}

func (b *sessionProjectionBuilder) applyJobEvent(event StoredEvent) {
	if event.Data.Job == nil {
		return
	}
	job := *event.Data.Job
	if job.JobID == "" {
		job.JobID = event.JobID
	}
	if job.JobID == "" {
		job.JobID = event.EventID
	}
	idx, ok := b.jobByID[job.JobID]
	if ok {
		b.projection.Jobs[idx] = job
		return
	}
	b.jobByID[job.JobID] = len(b.projection.Jobs)
	b.projection.Jobs = append(b.projection.Jobs, job)
}

func (b *sessionProjectionBuilder) applyWorkEvent(event StoredEvent) error {
	if event.Data.Work == nil {
		return nil
	}
	work := *event.Data.Work
	if work.WorkID == "" {
		work.WorkID = event.WorkID
	}
	if work.WorkID == "" {
		return fmt.Errorf("%s event missing work id", event.Type)
	}
	idx, ok := b.workByID[work.WorkID]
	if ok {
		merged, err := mergeWorkProjection(b.projection.Works[idx], work, true)
		if err != nil {
			return err
		}
		b.projection.Works[idx] = merged
		return nil
	}
	b.workByID[work.WorkID] = len(b.projection.Works)
	b.projection.Works = append(b.projection.Works, work)
	return nil
}

func (b *sessionProjectionBuilder) applyMemoryCandidateEvent(event StoredEvent) error {
	if event.Data.MemoryCandidate == nil {
		return nil
	}
	candidate := *event.Data.MemoryCandidate
	if candidate.CandidateID == "" {
		candidate.CandidateID = event.CandidateID
	}
	if candidate.CandidateID == "" {
		return fmt.Errorf("%s event missing candidate id", event.Type)
	}
	if err := b.upsertMemoryCandidate(candidate); err != nil {
		return err
	}
	if event.Type != "memory.candidate.superseded" {
		return nil
	}
	if event.Data.ReplacementMemoryCandidate == nil {
		return errors.New("superseded memory candidate missing replacement candidate")
	}
	replacement := *event.Data.ReplacementMemoryCandidate
	if replacement.CandidateID == "" {
		return fmt.Errorf("%s event missing replacement candidate id", event.Type)
	}
	return b.upsertMemoryCandidate(replacement)
}

func (b *sessionProjectionBuilder) upsertMemoryCandidate(candidate MemoryCandidateProjection) error {
	idx, ok := b.candidateByID[candidate.CandidateID]
	if ok {
		merged, err := mergeMemoryCandidateProjection(b.projection.MemoryCandidates[idx], candidate, true)
		if err != nil {
			return err
		}
		b.projection.MemoryCandidates[idx] = merged
		return nil
	}
	b.candidateByID[candidate.CandidateID] = len(b.projection.MemoryCandidates)
	b.projection.MemoryCandidates = append(b.projection.MemoryCandidates, candidate)
	return nil
}
