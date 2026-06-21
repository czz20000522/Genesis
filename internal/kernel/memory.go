package kernel

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

const (
	MemoryCandidatePending  = "pending"
	MemoryCandidateApproved = "approved"
)

var ErrMemoryCandidateNotFound = errors.New("memory candidate not found")

func (k *Kernel) CreateMemoryCandidate(req MemoryCandidateRequest) (MemoryCandidateProjection, error) {
	if err := validateMemoryCandidateRequest(req); err != nil {
		return MemoryCandidateProjection{}, err
	}
	now := k.clock()
	candidate := MemoryCandidateProjection{
		CandidateID: newID("mem", now),
		SessionID:   strings.TrimSpace(req.SessionID),
		Text:        strings.TrimSpace(req.Text),
		SourceRef:   strings.TrimSpace(req.SourceRef),
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
	if err := k.ledger.Append(event); err != nil {
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
	if err := k.ledger.Append(event); err != nil {
		return MemoryCandidateProjection{}, err
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
	return nil
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
	return nil
}

func (k *Kernel) memoryCandidates() (map[string]MemoryCandidateProjection, error) {
	_, candidates, err := k.memoryCandidateList()
	return candidates, err
}

func (k *Kernel) memoryCandidateList() ([]MemoryCandidateProjection, map[string]MemoryCandidateProjection, error) {
	events, err := k.ledger.Load()
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
		case "memory.candidate.created", "memory.candidate.approved":
			candidate := *event.Data.MemoryCandidate
			if candidate.CandidateID == "" {
				candidate.CandidateID = event.CandidateID
			}
			if candidate.CandidateID == "" {
				return nil, nil, fmt.Errorf("%s event missing candidate id", event.Type)
			}
			if _, exists := candidates[candidate.CandidateID]; !exists {
				order = append(order, candidate.CandidateID)
			}
			candidates[candidate.CandidateID] = candidate
		}
	}
	ordered := make([]MemoryCandidateProjection, 0, len(order))
	for _, candidateID := range order {
		ordered = append(ordered, candidates[candidateID])
	}
	return ordered, candidates, nil
}

func (k *Kernel) recallMemories(items []InputItem) ([]MemoryRecall, error) {
	candidates, _, err := k.memoryCandidateList()
	if err != nil {
		return nil, err
	}
	query := inputText(items)
	var recalls []MemoryRecall
	for _, candidate := range candidates {
		if candidate.Status != MemoryCandidateApproved {
			continue
		}
		if memoryMatchesTurn(candidate.Text, query) {
			recalls = append(recalls, MemoryRecall{
				CandidateID: candidate.CandidateID,
				Text:        candidate.Text,
				Source:      candidate.SourceRef,
			})
		}
	}
	return recalls, nil
}

func inputText(items []InputItem) string {
	var parts []string
	for _, item := range items {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func memoryMatchesTurn(memoryText string, query string) bool {
	memoryNorm := normalizeSearchText(memoryText)
	queryNorm := normalizeSearchText(query)
	if memoryNorm == "" || queryNorm == "" {
		return false
	}
	if strings.Contains(queryNorm, memoryNorm) || strings.Contains(memoryNorm, queryNorm) {
		return true
	}
	memoryBigrams := cjkBigrams(memoryNorm)
	for _, bigram := range memoryBigrams {
		if strings.Contains(queryNorm, bigram) {
			return true
		}
	}
	return false
}

func normalizeSearchText(text string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func cjkBigrams(text string) []string {
	runes := []rune(text)
	if len(runes) < 2 {
		return nil
	}
	var bigrams []string
	for i := 0; i < len(runes)-1; i++ {
		if isCJK(runes[i]) || isCJK(runes[i+1]) {
			bigrams = append(bigrams, string(runes[i:i+2]))
		}
	}
	return bigrams
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}
