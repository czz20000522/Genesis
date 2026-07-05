package kernel

import "time"

type MemoryCandidateRequest struct {
	SessionID   string `json:"session_id"`
	Text        string `json:"text"`
	SourceRef   string `json:"source_ref"`
	Kind        string `json:"kind,omitempty"`
	Scope       string `json:"scope,omitempty"`
	AppliesWhen string `json:"applies_when,omitempty"`
	YieldsTo    string `json:"yields_to,omitempty"`
	Strength    string `json:"strength,omitempty"`
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

type MemoryForgetRequest struct {
	ForgetAuthority   string `json:"forget_authority"`
	ForgetReason      string `json:"forget_reason"`
	ForgetEvidenceRef string `json:"forget_evidence_ref"`
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
	Kind                    string     `json:"kind"`
	Scope                   string     `json:"scope"`
	AppliesWhen             string     `json:"applies_when,omitempty"`
	YieldsTo                string     `json:"yields_to,omitempty"`
	Strength                string     `json:"strength"`
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
	ForgetAuthority         string     `json:"forget_authority,omitempty"`
	ForgetReason            string     `json:"forget_reason,omitempty"`
	ForgetEvidenceRef       string     `json:"forget_evidence_ref,omitempty"`
	ForgottenAt             *time.Time `json:"forgotten_at,omitempty"`
}
