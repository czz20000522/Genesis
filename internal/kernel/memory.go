package kernel

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MemoryCandidatePending    = "pending"
	MemoryCandidateApproved   = "approved"
	MemoryCandidateRejected   = "rejected"
	MemoryCandidateSuperseded = "superseded"
	MemoryCandidateForgotten  = "forgotten"
)

const (
	MemoryKindPreference     = "preference"
	MemoryKindHeuristic      = "heuristic"
	MemoryKindMethod         = "method"
	MemoryKindLesson         = "lesson"
	MemoryKindProjectOverlay = "project_overlay"
	MemoryKindCapabilityHint = "capability_hint"
	MemoryKindMemoryFact     = "memory_fact"
)

const (
	MemoryScopeGlobal     = "global"
	MemoryScopeProject    = "project"
	MemoryScopeWorkspace  = "workspace"
	MemoryScopeCapability = "capability"
)

const (
	MemoryStrengthWeakHint     = "weak_hint"
	MemoryStrengthPreference   = "preference"
	MemoryStrengthStrongRule   = "strong_rule"
	MemoryStrengthContractHint = "contract_hint"
)

var ErrMemoryCandidateNotFound = errors.New("memory candidate not found")

func (k *Kernel) CreateMemoryCandidate(req MemoryCandidateRequest) (MemoryCandidateProjection, error) {
	if err := validateMemoryCandidateRequest(req); err != nil {
		return MemoryCandidateProjection{}, err
	}
	metadata, err := normalizedMemoryCandidateMetadata(req)
	if err != nil {
		return MemoryCandidateProjection{}, err
	}
	now := k.clock()
	candidate := MemoryCandidateProjection{
		CandidateID: newID("mem", now),
		SessionID:   strings.TrimSpace(req.SessionID),
		Text:        strings.TrimSpace(req.Text),
		SourceRef:   strings.TrimSpace(req.SourceRef),
		Kind:        metadata.kind,
		Scope:       metadata.scope,
		AppliesWhen: metadata.appliesWhen,
		YieldsTo:    metadata.yieldsTo,
		Strength:    metadata.strength,
		Status:      MemoryCandidatePending,
		CreatedAt:   now,
	}
	event := StoredEvent{
		EventID:     newID("evt", now),
		SessionID:   candidate.SessionID,
		CandidateID: candidate.CandidateID,
		Type:        "memory.candidate.created",
		CreatedAt:   now,
		Data: EventData{
			MemoryCandidate: &candidate,
		},
	}
	if err := k.appendEvent(event); err != nil {
		return MemoryCandidateProjection{}, err
	}
	return candidate, nil
}

func (k *Kernel) ApproveMemoryCandidate(candidateID string, req MemoryApprovalRequest) (MemoryCandidateProjection, error) {
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return MemoryCandidateProjection{}, errors.New("candidate id is required")
	}
	if err := validateMemoryApprovalRequest(req); err != nil {
		return MemoryCandidateProjection{}, err
	}
	k.memoryReviewMu.Lock()
	defer k.memoryReviewMu.Unlock()

	candidates, err := k.memoryCandidates()
	if err != nil {
		return MemoryCandidateProjection{}, err
	}
	candidate, ok := candidates[candidateID]
	if !ok {
		return MemoryCandidateProjection{}, ErrMemoryCandidateNotFound
	}
	if candidate.Status == MemoryCandidateApproved {
		return candidate, nil
	}
	if candidate.Status == MemoryCandidateRejected {
		return MemoryCandidateProjection{}, errors.New("rejected memory candidate cannot be approved")
	}
	if candidate.Status == MemoryCandidateSuperseded {
		return MemoryCandidateProjection{}, errors.New("superseded memory candidate cannot be approved")
	}
	if candidate.Status == MemoryCandidateForgotten {
		return MemoryCandidateProjection{}, errors.New("forgotten memory candidate cannot be approved")
	}
	now := k.clock()
	candidate.Status = MemoryCandidateApproved
	candidate.ApprovalAuthority = strings.TrimSpace(req.ApprovalAuthority)
	candidate.ApprovalReason = strings.TrimSpace(req.ApprovalReason)
	candidate.ApprovalEvidenceRef = strings.TrimSpace(req.ApprovalEvidenceRef)
	candidate.ApprovedAt = &now
	event := StoredEvent{
		EventID:     newID("evt", now),
		SessionID:   candidate.SessionID,
		CandidateID: candidate.CandidateID,
		Type:        "memory.candidate.approved",
		CreatedAt:   now,
		Data: EventData{
			MemoryCandidate: &candidate,
		},
	}
	if err := k.appendEvent(event); err != nil {
		return MemoryCandidateProjection{}, err
	}
	return candidate, nil
}

func (k *Kernel) RejectMemoryCandidate(candidateID string, req MemoryRejectionRequest) (MemoryCandidateProjection, error) {
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return MemoryCandidateProjection{}, errors.New("candidate id is required")
	}
	if err := validateMemoryRejectionRequest(req); err != nil {
		return MemoryCandidateProjection{}, err
	}
	k.memoryReviewMu.Lock()
	defer k.memoryReviewMu.Unlock()

	candidates, err := k.memoryCandidates()
	if err != nil {
		return MemoryCandidateProjection{}, err
	}
	candidate, ok := candidates[candidateID]
	if !ok {
		return MemoryCandidateProjection{}, ErrMemoryCandidateNotFound
	}
	if candidate.Status == MemoryCandidateRejected {
		return candidate, nil
	}
	if candidate.Status == MemoryCandidateApproved {
		return MemoryCandidateProjection{}, errors.New("approved memory candidate cannot be rejected")
	}
	if candidate.Status == MemoryCandidateSuperseded {
		return MemoryCandidateProjection{}, errors.New("superseded memory candidate cannot be rejected")
	}
	if candidate.Status == MemoryCandidateForgotten {
		return MemoryCandidateProjection{}, errors.New("forgotten memory candidate cannot be rejected")
	}
	now := k.clock()
	candidate.Status = MemoryCandidateRejected
	candidate.RejectionAuthority = strings.TrimSpace(req.RejectionAuthority)
	candidate.RejectionReason = strings.TrimSpace(req.RejectionReason)
	candidate.RejectionEvidenceRef = strings.TrimSpace(req.RejectionEvidenceRef)
	candidate.RejectedAt = &now
	event := StoredEvent{
		EventID:     newID("evt", now),
		SessionID:   candidate.SessionID,
		CandidateID: candidate.CandidateID,
		Type:        "memory.candidate.rejected",
		CreatedAt:   now,
		Data: EventData{
			MemoryCandidate: &candidate,
		},
	}
	if err := k.appendEvent(event); err != nil {
		return MemoryCandidateProjection{}, err
	}
	return candidate, nil
}

func (k *Kernel) SupersedeMemoryCandidate(candidateID string, req MemorySupersessionRequest) (MemorySupersessionProjection, error) {
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return MemorySupersessionProjection{}, errors.New("candidate id is required")
	}
	if err := validateMemorySupersessionRequest(req); err != nil {
		return MemorySupersessionProjection{}, err
	}
	k.memoryReviewMu.Lock()
	defer k.memoryReviewMu.Unlock()

	candidates, err := k.memoryCandidates()
	if err != nil {
		return MemorySupersessionProjection{}, err
	}
	candidate, ok := candidates[candidateID]
	if !ok {
		return MemorySupersessionProjection{}, ErrMemoryCandidateNotFound
	}
	if candidate.Status == MemoryCandidateSuperseded {
		return existingMemorySupersession(candidate, candidates)
	}
	if candidate.Status == MemoryCandidateForgotten {
		return MemorySupersessionProjection{}, errors.New("forgotten memory candidate cannot be superseded")
	}
	now := k.clock()
	replacement := MemoryCandidateProjection{
		CandidateID: newID("mem", now),
		SessionID:   candidate.SessionID,
		Text:        strings.TrimSpace(req.ReplacementText),
		SourceRef:   strings.TrimSpace(req.ReplacementSourceRef),
		Kind:        candidate.Kind,
		Scope:       candidate.Scope,
		AppliesWhen: candidate.AppliesWhen,
		YieldsTo:    candidate.YieldsTo,
		Strength:    candidate.Strength,
		Status:      MemoryCandidatePending,
		CreatedAt:   now,
	}
	candidate.Status = MemoryCandidateSuperseded
	candidate.SupersessionAuthority = strings.TrimSpace(req.SupersessionAuthority)
	candidate.SupersessionReason = strings.TrimSpace(req.SupersessionReason)
	candidate.SupersessionEvidenceRef = strings.TrimSpace(req.SupersessionEvidenceRef)
	candidate.ReplacementCandidateID = replacement.CandidateID
	candidate.SupersededAt = &now
	event := StoredEvent{
		EventID:     newID("evt", now),
		SessionID:   candidate.SessionID,
		CandidateID: candidate.CandidateID,
		Type:        "memory.candidate.superseded",
		CreatedAt:   now,
		Data: EventData{
			MemoryCandidate:            &candidate,
			ReplacementMemoryCandidate: &replacement,
		},
	}
	if err := k.appendEvent(event); err != nil {
		return MemorySupersessionProjection{}, err
	}
	return MemorySupersessionProjection{Superseded: candidate, Replacement: replacement}, nil
}

func (k *Kernel) ForgetMemoryCandidate(candidateID string, req MemoryForgetRequest) (MemoryCandidateProjection, error) {
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return MemoryCandidateProjection{}, errors.New("candidate id is required")
	}
	if err := validateMemoryForgetRequest(req); err != nil {
		return MemoryCandidateProjection{}, err
	}
	k.memoryReviewMu.Lock()
	defer k.memoryReviewMu.Unlock()

	candidates, err := k.memoryCandidates()
	if err != nil {
		return MemoryCandidateProjection{}, err
	}
	candidate, ok := candidates[candidateID]
	if !ok {
		return MemoryCandidateProjection{}, ErrMemoryCandidateNotFound
	}
	if candidate.Status == MemoryCandidateForgotten {
		return candidate, nil
	}
	now := k.clock()
	candidate.Status = MemoryCandidateForgotten
	candidate.ForgetAuthority = strings.TrimSpace(req.ForgetAuthority)
	candidate.ForgetReason = strings.TrimSpace(req.ForgetReason)
	candidate.ForgetEvidenceRef = strings.TrimSpace(req.ForgetEvidenceRef)
	candidate.ForgottenAt = &now
	event := StoredEvent{
		EventID:     newID("evt", now),
		SessionID:   candidate.SessionID,
		CandidateID: candidate.CandidateID,
		Type:        "memory.candidate.forgotten",
		CreatedAt:   now,
		Data: EventData{
			MemoryCandidate: &candidate,
		},
	}
	if err := k.appendEvent(event); err != nil {
		return MemoryCandidateProjection{}, err
	}
	return candidate, nil
}

func (k *Kernel) MemoryCandidates(status string) ([]MemoryCandidateProjection, error) {
	status = strings.TrimSpace(status)
	if status != "" && !validMemoryCandidateStatus(status) {
		return nil, fmt.Errorf("unsupported memory candidate status %q", status)
	}
	candidates, _, err := k.memoryCandidateList()
	if err != nil {
		return nil, err
	}
	if status == "" {
		return candidates, nil
	}
	filtered := make([]MemoryCandidateProjection, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Status == status {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, nil
}

func (k *Kernel) MemoryCandidate(candidateID string) (MemoryCandidateProjection, error) {
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return MemoryCandidateProjection{}, errors.New("candidate id is required")
	}
	candidates, err := k.memoryCandidates()
	if err != nil {
		return MemoryCandidateProjection{}, err
	}
	candidate, ok := candidates[candidateID]
	if !ok {
		return MemoryCandidateProjection{}, ErrMemoryCandidateNotFound
	}
	return candidate, nil
}

func validateMemoryCandidateRequest(req MemoryCandidateRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return errors.New("session_id is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return errors.New("text is required")
	}
	if strings.TrimSpace(req.SourceRef) == "" {
		return errors.New("source_ref is required")
	}
	if err := validateKernelControlToken("session_id", req.SessionID); err != nil {
		return err
	}
	if err := validateKernelRef("source_ref", req.SourceRef); err != nil {
		return err
	}
	if _, err := normalizedMemoryCandidateMetadata(req); err != nil {
		return err
	}
	return nil
}

type memoryCandidateMetadata struct {
	kind        string
	scope       string
	appliesWhen string
	yieldsTo    string
	strength    string
}

func normalizedMemoryCandidateMetadata(req MemoryCandidateRequest) (memoryCandidateMetadata, error) {
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = MemoryKindMemoryFact
	}
	if !validMemoryKind(kind) {
		return memoryCandidateMetadata{}, fmt.Errorf("unsupported memory candidate kind %q", kind)
	}
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = MemoryScopeGlobal
	}
	if !validMemoryScope(scope) {
		return memoryCandidateMetadata{}, fmt.Errorf("unsupported memory candidate scope %q", scope)
	}
	strength := strings.TrimSpace(req.Strength)
	if strength == "" {
		strength = MemoryStrengthWeakHint
	}
	if !validMemoryStrength(strength) {
		return memoryCandidateMetadata{}, fmt.Errorf("unsupported memory candidate strength %q", strength)
	}
	return memoryCandidateMetadata{
		kind:        kind,
		scope:       scope,
		appliesWhen: strings.TrimSpace(req.AppliesWhen),
		yieldsTo:    strings.TrimSpace(req.YieldsTo),
		strength:    strength,
	}, nil
}

func validateMemoryApprovalRequest(req MemoryApprovalRequest) error {
	if strings.TrimSpace(req.ApprovalAuthority) == "" {
		return errors.New("approval_authority is required")
	}
	if strings.TrimSpace(req.ApprovalReason) == "" {
		return errors.New("approval_reason is required")
	}
	if strings.TrimSpace(req.ApprovalEvidenceRef) == "" {
		return errors.New("approval_evidence_ref is required")
	}
	if err := validateKernelAuthority("approval_authority", req.ApprovalAuthority); err != nil {
		return err
	}
	if err := validateKernelRef("approval_evidence_ref", req.ApprovalEvidenceRef); err != nil {
		return err
	}
	return nil
}

func validateMemoryRejectionRequest(req MemoryRejectionRequest) error {
	if strings.TrimSpace(req.RejectionAuthority) == "" {
		return errors.New("rejection_authority is required")
	}
	if strings.TrimSpace(req.RejectionReason) == "" {
		return errors.New("rejection_reason is required")
	}
	if strings.TrimSpace(req.RejectionEvidenceRef) == "" {
		return errors.New("rejection_evidence_ref is required")
	}
	if err := validateKernelAuthority("rejection_authority", req.RejectionAuthority); err != nil {
		return err
	}
	if err := validateKernelRef("rejection_evidence_ref", req.RejectionEvidenceRef); err != nil {
		return err
	}
	return nil
}

func validateMemorySupersessionRequest(req MemorySupersessionRequest) error {
	if strings.TrimSpace(req.ReplacementText) == "" {
		return errors.New("replacement_text is required")
	}
	if strings.TrimSpace(req.ReplacementSourceRef) == "" {
		return errors.New("replacement_source_ref is required")
	}
	if strings.TrimSpace(req.SupersessionAuthority) == "" {
		return errors.New("supersession_authority is required")
	}
	if strings.TrimSpace(req.SupersessionReason) == "" {
		return errors.New("supersession_reason is required")
	}
	if strings.TrimSpace(req.SupersessionEvidenceRef) == "" {
		return errors.New("supersession_evidence_ref is required")
	}
	if err := validateKernelRef("replacement_source_ref", req.ReplacementSourceRef); err != nil {
		return err
	}
	if err := validateKernelAuthority("supersession_authority", req.SupersessionAuthority); err != nil {
		return err
	}
	if err := validateKernelRef("supersession_evidence_ref", req.SupersessionEvidenceRef); err != nil {
		return err
	}
	return nil
}

func validateMemoryForgetRequest(req MemoryForgetRequest) error {
	if strings.TrimSpace(req.ForgetAuthority) == "" {
		return errors.New("forget_authority is required")
	}
	if strings.TrimSpace(req.ForgetReason) == "" {
		return errors.New("forget_reason is required")
	}
	if strings.TrimSpace(req.ForgetEvidenceRef) == "" {
		return errors.New("forget_evidence_ref is required")
	}
	if err := validateKernelAuthority("forget_authority", req.ForgetAuthority); err != nil {
		return err
	}
	if err := validateKernelRef("forget_evidence_ref", req.ForgetEvidenceRef); err != nil {
		return err
	}
	return nil
}

func (k *Kernel) memoryCandidates() (map[string]MemoryCandidateProjection, error) {
	_, candidates, err := k.memoryCandidateList()
	return candidates, err
}

func (k *Kernel) memoryCandidateList() ([]MemoryCandidateProjection, map[string]MemoryCandidateProjection, error) {
	events, err := k.loadEvents()
	if err != nil {
		return nil, nil, err
	}
	candidates := map[string]MemoryCandidateProjection{}
	order := []string{}
	for _, event := range events {
		if event.Data.MemoryCandidate == nil {
			continue
		}
		switch event.Type {
		case "memory.candidate.created", "memory.candidate.approved", "memory.candidate.rejected", "memory.candidate.superseded", "memory.candidate.forgotten":
			candidate, err := candidateFromEvent(event, event.Data.MemoryCandidate, event.CandidateID)
			if err != nil {
				return nil, nil, err
			}
			if err := recordMemoryCandidate(candidate, candidates, &order); err != nil {
				return nil, nil, err
			}
			if event.Type == "memory.candidate.superseded" {
				if event.Data.ReplacementMemoryCandidate == nil {
					return nil, nil, errors.New("superseded memory candidate missing replacement candidate")
				}
				replacement, err := candidateFromEvent(event, event.Data.ReplacementMemoryCandidate, "")
				if err != nil {
					return nil, nil, err
				}
				if err := recordMemoryCandidate(replacement, candidates, &order); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	ordered := make([]MemoryCandidateProjection, 0, len(order))
	for _, candidateID := range order {
		ordered = append(ordered, candidates[candidateID])
	}
	return ordered, candidates, nil
}

func candidateFromEvent(event StoredEvent, candidateRef *MemoryCandidateProjection, fallbackCandidateID string) (MemoryCandidateProjection, error) {
	candidate := *candidateRef
	if candidate.CandidateID == "" {
		candidate.CandidateID = fallbackCandidateID
	}
	if candidate.CandidateID == "" {
		return MemoryCandidateProjection{}, fmt.Errorf("%s event missing candidate id", event.Type)
	}
	return candidate, nil
}

func recordMemoryCandidate(candidate MemoryCandidateProjection, candidates map[string]MemoryCandidateProjection, order *[]string) error {
	current, exists := candidates[candidate.CandidateID]
	merged, err := mergeMemoryCandidateProjection(current, candidate, exists)
	if err != nil {
		return err
	}
	if !exists {
		*order = append(*order, candidate.CandidateID)
	}
	candidates[candidate.CandidateID] = merged
	return nil
}

func mergeMemoryCandidateProjection(current MemoryCandidateProjection, incoming MemoryCandidateProjection, exists bool) (MemoryCandidateProjection, error) {
	if !exists {
		return incoming, nil
	}
	if !sameMemoryCandidateCore(current, incoming) {
		return MemoryCandidateProjection{}, fmt.Errorf("competing memory review evidence for %s", current.CandidateID)
	}
	if current.Status == MemoryCandidateForgotten {
		if incoming.Status == MemoryCandidateForgotten && sameMemoryForgetDecision(current, incoming) {
			return current, nil
		}
		return MemoryCandidateProjection{}, fmt.Errorf("competing memory review evidence for %s", current.CandidateID)
	}
	if current.Status == MemoryCandidateSuperseded {
		if incoming.Status == MemoryCandidateForgotten {
			return incoming, nil
		}
		if incoming.Status == MemoryCandidateSuperseded && sameMemorySupersessionDecision(current, incoming) {
			return current, nil
		}
		return MemoryCandidateProjection{}, fmt.Errorf("competing memory review evidence for %s", current.CandidateID)
	}
	switch incoming.Status {
	case MemoryCandidateForgotten:
		return incoming, nil
	case MemoryCandidateSuperseded:
		return incoming, nil
	case MemoryCandidateApproved:
		if current.Status == MemoryCandidateRejected || current.Status == MemoryCandidateSuperseded {
			return MemoryCandidateProjection{}, fmt.Errorf("competing memory review evidence for %s", current.CandidateID)
		}
		if current.Status == MemoryCandidateApproved && !sameMemoryApprovalDecision(current, incoming) {
			return MemoryCandidateProjection{}, fmt.Errorf("competing memory review evidence for %s", current.CandidateID)
		}
		if current.Status == MemoryCandidateApproved {
			return current, nil
		}
		return incoming, nil
	case MemoryCandidateRejected:
		if current.Status == MemoryCandidateApproved || current.Status == MemoryCandidateSuperseded {
			return MemoryCandidateProjection{}, fmt.Errorf("competing memory review evidence for %s", current.CandidateID)
		}
		if current.Status == MemoryCandidateRejected && !sameMemoryRejectionDecision(current, incoming) {
			return MemoryCandidateProjection{}, fmt.Errorf("competing memory review evidence for %s", current.CandidateID)
		}
		if current.Status == MemoryCandidateRejected {
			return current, nil
		}
		return incoming, nil
	case MemoryCandidatePending:
		if current.Status != MemoryCandidatePending {
			return MemoryCandidateProjection{}, fmt.Errorf("competing memory review evidence for %s", current.CandidateID)
		}
		return current, nil
	default:
		return MemoryCandidateProjection{}, fmt.Errorf("unsupported memory candidate status %q", incoming.Status)
	}
}

func sameMemoryCandidateCore(left MemoryCandidateProjection, right MemoryCandidateProjection) bool {
	return left.CandidateID == right.CandidateID &&
		left.SessionID == right.SessionID &&
		left.Text == right.Text &&
		left.SourceRef == right.SourceRef &&
		left.Kind == right.Kind &&
		left.Scope == right.Scope &&
		left.AppliesWhen == right.AppliesWhen &&
		left.YieldsTo == right.YieldsTo &&
		left.Strength == right.Strength &&
		left.CreatedAt.Equal(right.CreatedAt)
}

func sameMemoryApprovalDecision(left MemoryCandidateProjection, right MemoryCandidateProjection) bool {
	return left.ApprovalAuthority == right.ApprovalAuthority &&
		left.ApprovalReason == right.ApprovalReason &&
		left.ApprovalEvidenceRef == right.ApprovalEvidenceRef &&
		sameOptionalTime(left.ApprovedAt, right.ApprovedAt)
}

func sameMemoryRejectionDecision(left MemoryCandidateProjection, right MemoryCandidateProjection) bool {
	return left.RejectionAuthority == right.RejectionAuthority &&
		left.RejectionReason == right.RejectionReason &&
		left.RejectionEvidenceRef == right.RejectionEvidenceRef &&
		sameOptionalTime(left.RejectedAt, right.RejectedAt)
}

func sameMemorySupersessionDecision(left MemoryCandidateProjection, right MemoryCandidateProjection) bool {
	return left.SupersessionAuthority == right.SupersessionAuthority &&
		left.SupersessionReason == right.SupersessionReason &&
		left.SupersessionEvidenceRef == right.SupersessionEvidenceRef &&
		left.ReplacementCandidateID == right.ReplacementCandidateID &&
		sameOptionalTime(left.SupersededAt, right.SupersededAt)
}

func sameMemoryForgetDecision(left MemoryCandidateProjection, right MemoryCandidateProjection) bool {
	return left.ForgetAuthority == right.ForgetAuthority &&
		left.ForgetReason == right.ForgetReason &&
		left.ForgetEvidenceRef == right.ForgetEvidenceRef &&
		sameOptionalTime(left.ForgottenAt, right.ForgottenAt)
}

func validMemoryCandidateStatus(status string) bool {
	return status == MemoryCandidatePending ||
		status == MemoryCandidateApproved ||
		status == MemoryCandidateRejected ||
		status == MemoryCandidateSuperseded ||
		status == MemoryCandidateForgotten
}

func validMemoryKind(kind string) bool {
	switch kind {
	case MemoryKindPreference, MemoryKindHeuristic, MemoryKindMethod, MemoryKindLesson, MemoryKindProjectOverlay, MemoryKindCapabilityHint, MemoryKindMemoryFact:
		return true
	default:
		return false
	}
}

func validMemoryScope(scope string) bool {
	switch scope {
	case MemoryScopeGlobal, MemoryScopeProject, MemoryScopeWorkspace, MemoryScopeCapability:
		return true
	default:
		return false
	}
}

func validMemoryStrength(strength string) bool {
	switch strength {
	case MemoryStrengthWeakHint, MemoryStrengthPreference, MemoryStrengthStrongRule, MemoryStrengthContractHint:
		return true
	default:
		return false
	}
}

func sameOptionalTime(left *time.Time, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func existingMemorySupersession(candidate MemoryCandidateProjection, candidates map[string]MemoryCandidateProjection) (MemorySupersessionProjection, error) {
	if strings.TrimSpace(candidate.ReplacementCandidateID) == "" {
		return MemorySupersessionProjection{}, errors.New("superseded memory candidate missing replacement id")
	}
	replacement, ok := candidates[candidate.ReplacementCandidateID]
	if !ok {
		return MemorySupersessionProjection{}, errors.New("superseded memory candidate replacement not found")
	}
	return MemorySupersessionProjection{Superseded: candidate, Replacement: replacement}, nil
}
