package kernel

import "time"

type MemoryCandidateRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	SourceRef string `json:"source_ref"`
}

type MemoryCandidateListResponse struct {
	Items []MemoryCandidateProjection `json:"items"`
}

type MemoryApprovalRequest struct {
	ApprovalAuthority   string `json:"approval_authority"`
	ApprovalReason      string `json:"approval_reason"`
	ApprovalEvidenceRef string `json:"approval_evidence_ref"`
}

type MemoryRejectionRequest struct {
	RejectionAuthority   string `json:"rejection_authority"`
	RejectionReason      string `json:"rejection_reason"`
	RejectionEvidenceRef string `json:"rejection_evidence_ref"`
}

type MemorySupersessionRequest struct {
	ReplacementText         string `json:"replacement_text"`
	ReplacementSourceRef    string `json:"replacement_source_ref"`
	SupersessionAuthority   string `json:"supersession_authority"`
	SupersessionReason      string `json:"supersession_reason"`
	SupersessionEvidenceRef string `json:"supersession_evidence_ref"`
}

type MemorySupersessionProjection struct {
	Superseded  MemoryCandidateProjection `json:"superseded"`
	Replacement MemoryCandidateProjection `json:"replacement"`
}

type MemoryCandidateProjection struct {
	CandidateID             string     `json:"candidate_id"`
	SessionID               string     `json:"session_id"`
	Text                    string     `json:"text"`
	SourceRef               string     `json:"source_ref"`
	Status                  string     `json:"status"`
	CreatedAt               time.Time  `json:"created_at"`
	ApprovalAuthority       string     `json:"approval_authority,omitempty"`
	ApprovalReason          string     `json:"approval_reason,omitempty"`
	ApprovalEvidenceRef     string     `json:"approval_evidence_ref,omitempty"`
	ApprovedAt              *time.Time `json:"approved_at,omitempty"`
	RejectionAuthority      string     `json:"rejection_authority,omitempty"`
	RejectionReason         string     `json:"rejection_reason,omitempty"`
	RejectionEvidenceRef    string     `json:"rejection_evidence_ref,omitempty"`
	RejectedAt              *time.Time `json:"rejected_at,omitempty"`
	SupersessionAuthority   string     `json:"supersession_authority,omitempty"`
	SupersessionReason      string     `json:"supersession_reason,omitempty"`
	SupersessionEvidenceRef string     `json:"supersession_evidence_ref,omitempty"`
	ReplacementCandidateID  string     `json:"replacement_candidate_id,omitempty"`
	SupersededAt            *time.Time `json:"superseded_at,omitempty"`
}
