package kernel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestSemanticTextFieldsAllowSecretShapedContent(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	secretTitle := "Investigate GENESIS_PROVIDER_API_KEY=sk-work-secret as quoted user text"
	createPayload, err := json.Marshal(WorkSubmitRequest{
		SessionID: "semantic-text-work",
		Title:     secretTitle,
		SourceRef: "turn:semantic-text-work",
	})
	if err != nil {
		t.Fatalf("marshal work request: %v", err)
	}
	createResp, err := postJSONWithAuth(server.URL+"/work", createPayload)
	if err != nil {
		t.Fatalf("POST /work failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d, want 200", createResp.StatusCode)
	}
	var work WorkProjection
	if err := json.NewDecoder(createResp.Body).Decode(&work); err != nil {
		t.Fatalf("decode work: %v", err)
	}
	if work.Title != secretTitle {
		t.Fatalf("work title = %q, want semantic text preserved", work.Title)
	}

	cancelReason := "User quoted Authorization: Bearer tokentest123456 while canceling"
	cancelPayload, err := json.Marshal(WorkCancelRequest{
		CancelAuthority:   "runtime:test",
		CancelReason:      cancelReason,
		CancelEvidenceRef: "review:semantic-text-work",
	})
	if err != nil {
		t.Fatalf("marshal cancel request: %v", err)
	}
	cancelResp, err := postJSONWithAuth(server.URL+"/work/"+work.WorkID+"/cancel", cancelPayload)
	if err != nil {
		t.Fatalf("POST /work cancel failed: %v", err)
	}
	defer cancelResp.Body.Close()
	if cancelResp.StatusCode != http.StatusOK {
		t.Fatalf("cancel status = %d, want 200", cancelResp.StatusCode)
	}
	var canceled WorkProjection
	if err := json.NewDecoder(cancelResp.Body).Decode(&canceled); err != nil {
		t.Fatalf("decode canceled work: %v", err)
	}
	if canceled.CancelReason != cancelReason {
		t.Fatalf("cancel reason = %q, want semantic text preserved", canceled.CancelReason)
	}

	approvalReason := "Reviewer quoted api_key=sk-memory-secret but approved the candidate"
	approvedCandidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "semantic-text-memory-approval",
		Text:      "approved memory",
		SourceRef: "turn:semantic-text-memory-approval",
	})
	approvalPayload, err := json.Marshal(MemoryApprovalRequest{
		ApprovalAuthority:   "runtime:test",
		ApprovalReason:      approvalReason,
		ApprovalEvidenceRef: "approval:semantic-text-memory",
	})
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approvalResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+approvedCandidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer approvalResp.Body.Close()
	if approvalResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approvalResp.StatusCode)
	}
	var approved MemoryCandidateProjection
	if err := json.NewDecoder(approvalResp.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approved candidate: %v", err)
	}
	if approved.ApprovalReason != approvalReason {
		t.Fatalf("approval reason = %q, want semantic text preserved", approved.ApprovalReason)
	}

	rejectionReason := "Rejected because the statement only quoted Authorization: Bearer tokentest123456"
	rejectedCandidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "semantic-text-memory-rejection",
		Text:      "rejected memory",
		SourceRef: "turn:semantic-text-memory-rejection",
	})
	rejectionPayload, err := json.Marshal(MemoryRejectionRequest{
		RejectionAuthority:   "runtime:test",
		RejectionReason:      rejectionReason,
		RejectionEvidenceRef: "review:semantic-text-memory",
	})
	if err != nil {
		t.Fatalf("marshal rejection request: %v", err)
	}
	rejectionResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+rejectedCandidate.CandidateID+"/reject", rejectionPayload)
	if err != nil {
		t.Fatalf("POST reject failed: %v", err)
	}
	defer rejectionResp.Body.Close()
	if rejectionResp.StatusCode != http.StatusOK {
		t.Fatalf("reject status = %d, want 200", rejectionResp.StatusCode)
	}
	var rejected MemoryCandidateProjection
	if err := json.NewDecoder(rejectionResp.Body).Decode(&rejected); err != nil {
		t.Fatalf("decode rejected candidate: %v", err)
	}
	if rejected.RejectionReason != rejectionReason {
		t.Fatalf("rejection reason = %q, want semantic text preserved", rejected.RejectionReason)
	}

	supersessionReason := "Superseded after reviewing GENESIS_PROVIDER_API_KEY=sk-memory-secret in source text"
	supersededCandidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "semantic-text-memory-supersession",
		Text:      "old memory",
		SourceRef: "turn:semantic-text-memory-supersession",
	})
	replacementText := "replacement mentions api_key=sk-replacement-secret as semantic content"
	supersessionPayload, err := json.Marshal(MemorySupersessionRequest{
		ReplacementText:         replacementText,
		ReplacementSourceRef:    "review:semantic-text-memory-replacement",
		SupersessionAuthority:   "runtime:test",
		SupersessionReason:      supersessionReason,
		SupersessionEvidenceRef: "review:semantic-text-memory",
	})
	if err != nil {
		t.Fatalf("marshal supersession request: %v", err)
	}
	supersessionResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+supersededCandidate.CandidateID+"/supersede", supersessionPayload)
	if err != nil {
		t.Fatalf("POST supersede failed: %v", err)
	}
	defer supersessionResp.Body.Close()
	if supersessionResp.StatusCode != http.StatusOK {
		t.Fatalf("supersede status = %d, want 200", supersessionResp.StatusCode)
	}
	var supersession MemorySupersessionProjection
	if err := json.NewDecoder(supersessionResp.Body).Decode(&supersession); err != nil {
		t.Fatalf("decode supersession: %v", err)
	}
	if supersession.Superseded.SupersessionReason != supersessionReason {
		t.Fatalf("supersession reason = %q, want semantic text preserved", supersession.Superseded.SupersessionReason)
	}
	if supersession.Replacement.Text != replacementText {
		t.Fatalf("replacement text = %q, want semantic text preserved", supersession.Replacement.Text)
	}
}
