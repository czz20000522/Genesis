package kernel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUnapprovedMemoryCandidateIsNotRecalled(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	_, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-source",
		Text:      "我偏好中文回答",
		SourceRef: "turn:memory-source",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "memory-consumer",
		InputItems: []InputItem{{Type: "text", Text: "你记得我的回答偏好吗？"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if strings.Contains(resp.Final.Text, "我偏好中文回答") {
		t.Fatalf("unapproved memory was recalled in final text: %q", resp.Final.Text)
	}
}

func TestCreateMemoryCandidateRequiresSourceRef(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))

	_, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-source",
		Text:      "我偏好中文回答",
	})
	if err == nil {
		t.Fatal("CreateMemoryCandidate returned nil error without source_ref")
	}
}

func TestHTTPCreateMemoryCandidateRejectsInvalidControlRefs(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	cases := map[string]MemoryCandidateRequest{
		"invalid source ref": {
			SessionID: "bad-memory-source",
			Text:      "memory",
			SourceRef: "free text",
		},
		"secret session id": {
			SessionID: "api_key=sk-memory-secret",
			Text:      "memory",
			SourceRef: "turn:bad-memory-secret-session",
		},
		"secret source ref": {
			SessionID: "bad-memory-secret-source",
			Text:      "memory",
			SourceRef: "turn:api_key=sk-memory-secret",
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			payload, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			resp, err := postJSONWithAuth(server.URL+"/memory/candidates", payload)
			if err != nil {
				t.Fatalf("POST candidate failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestApprovedMemoryCandidatePersistsAcrossSessionsAfterRestartWithoutAutoRecall(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-source",
		Text:      "我偏好中文回答",
		SourceRef: "turn:memory-source",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}

	restarted := newTestKernel(t, ledgerPath)
	approved, err := restarted.ApproveMemoryCandidate(candidate.CandidateID, testApprovalRequest("approval:memory-source"))
	if err != nil {
		t.Fatalf("ApproveMemoryCandidate returned error: %v", err)
	}
	if approved.Status != MemoryCandidateApproved {
		t.Fatalf("approved status = %q, want approved", approved.Status)
	}

	consumer := newTestKernel(t, ledgerPath)
	resp, err := consumer.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "memory-consumer",
		InputItems: []InputItem{{Type: "text", Text: "你记得我的回答偏好吗？"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if strings.Contains(resp.Final.Text, "我偏好中文回答") {
		t.Fatalf("final text = %q, want approved memory not auto recalled", resp.Final.Text)
	}

	sourceProjection, err := consumer.Session("memory-source")
	if err != nil {
		t.Fatalf("source Session returned error: %v", err)
	}
	if len(sourceProjection.MemoryCandidates) != 1 {
		t.Fatalf("len(MemoryCandidates) = %d, want 1", len(sourceProjection.MemoryCandidates))
	}
	if sourceProjection.MemoryCandidates[0].Status != MemoryCandidateApproved {
		t.Fatalf("candidate status = %q, want approved", sourceProjection.MemoryCandidates[0].Status)
	}
	if sourceProjection.MemoryCandidates[0].SourceRef != "turn:memory-source" {
		t.Fatalf("candidate source ref = %q, want turn:memory-source", sourceProjection.MemoryCandidates[0].SourceRef)
	}
	if sourceProjection.MemoryCandidates[0].ApprovalEvidenceRef != "approval:memory-source" {
		t.Fatalf("approval evidence ref = %q, want approval:memory-source", sourceProjection.MemoryCandidates[0].ApprovalEvidenceRef)
	}

	consumerProjection, err := consumer.Session("memory-consumer")
	if err != nil {
		t.Fatalf("consumer Session returned error: %v", err)
	}
	if len(consumerProjection.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(consumerProjection.Turns))
	}
}

func TestHTTPMemoryCandidateApproveDoesNotAutoRecall(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	candidatePayload, err := json.Marshal(MemoryCandidateRequest{
		SessionID: "http-memory-source",
		Text:      "我偏好中文回答",
		SourceRef: "turn:http-memory-source",
	})
	if err != nil {
		t.Fatalf("marshal candidate request: %v", err)
	}
	candidateResp, err := postJSONWithAuth(server.URL+"/memory/candidates", candidatePayload)
	if err != nil {
		t.Fatalf("POST /memory/candidates failed: %v", err)
	}
	defer candidateResp.Body.Close()
	if candidateResp.StatusCode != http.StatusOK {
		t.Fatalf("candidate status = %d, want 200", candidateResp.StatusCode)
	}
	var candidate MemoryCandidateProjection
	if err := json.NewDecoder(candidateResp.Body).Decode(&candidate); err != nil {
		t.Fatalf("decode candidate response: %v", err)
	}

	approvalPayload, err := json.Marshal(testApprovalRequest("approval:http-memory-source"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveResp.StatusCode)
	}
	var approved MemoryCandidateProjection
	if err := json.NewDecoder(approveResp.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approved response: %v", err)
	}
	if approved.Status != MemoryCandidateApproved {
		t.Fatalf("approved status = %q, want approved", approved.Status)
	}

	turnPayload := []byte(`{"session_id":"http-memory-consumer","input_items":[{"type":"text","text":"你记得我的回答偏好吗？"}]}`)
	turnResp, err := postJSONWithAuth(server.URL+"/turn", turnPayload)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	if turnResp.StatusCode != http.StatusOK {
		t.Fatalf("turn status = %d, want 200", turnResp.StatusCode)
	}
	var turn TurnResponse
	if err := json.NewDecoder(turnResp.Body).Decode(&turn); err != nil {
		t.Fatalf("decode turn response: %v", err)
	}
	if strings.Contains(turn.Final.Text, "我偏好中文回答") {
		t.Fatalf("final text = %q, want approved memory not auto recalled", turn.Final.Text)
	}
}

func TestHTTPMemoryCandidateListAndReadAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))

	firstCandidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-source-one",
		Text:      "我偏好中文回答",
		SourceRef: "turn:http-memory-source-one",
	})
	secondCandidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-source-two",
		Text:      "我偏好短回答",
		SourceRef: "turn:http-memory-source-two",
	})
	approvalPayload, err := json.Marshal(testApprovalRequest("approval:http-memory-source-one"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+firstCandidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveResp.StatusCode)
	}
	server.Close()

	restarted := newTestKernel(t, ledgerPath)
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	pendingResp, err := getWithAuth(restartedServer.URL + "/memory/candidates?status=pending")
	if err != nil {
		t.Fatalf("GET pending candidates failed: %v", err)
	}
	defer pendingResp.Body.Close()
	if pendingResp.StatusCode != http.StatusOK {
		t.Fatalf("pending status = %d, want 200", pendingResp.StatusCode)
	}
	var pending MemoryCandidateListResponse
	if err := json.NewDecoder(pendingResp.Body).Decode(&pending); err != nil {
		t.Fatalf("decode pending candidates: %v", err)
	}
	if len(pending.Items) != 1 || pending.Items[0].CandidateID != secondCandidate.CandidateID {
		t.Fatalf("pending candidates = %+v, want second candidate only", pending.Items)
	}
	if pending.Items[0].SourceRef != "turn:http-memory-source-two" {
		t.Fatalf("pending source ref = %q, want turn:http-memory-source-two", pending.Items[0].SourceRef)
	}

	readResp, err := getWithAuth(restartedServer.URL + "/memory/candidates/" + firstCandidate.CandidateID)
	if err != nil {
		t.Fatalf("GET memory candidate failed: %v", err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d, want 200", readResp.StatusCode)
	}
	var approved MemoryCandidateProjection
	if err := json.NewDecoder(readResp.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approved candidate: %v", err)
	}
	if approved.Status != MemoryCandidateApproved {
		t.Fatalf("approved status = %q, want approved", approved.Status)
	}
	if approved.ApprovalEvidenceRef != "approval:http-memory-source-one" {
		t.Fatalf("approval evidence ref = %q, want approval:http-memory-source-one", approved.ApprovalEvidenceRef)
	}

	badStatusResp, err := getWithAuth(restartedServer.URL + "/memory/candidates?status=unknown")
	if err != nil {
		t.Fatalf("GET bad status failed: %v", err)
	}
	defer badStatusResp.Body.Close()
	if badStatusResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad status response = %d, want 400", badStatusResp.StatusCode)
	}
}

func TestHTTPMemoryCandidateRejectAndReadAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))

	candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-reject-source",
		Text:      "红色雨伞",
		SourceRef: "turn:http-memory-reject-source",
	})
	rejectResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/reject", []byte(`{"rejection_authority":"runtime:test","rejection_reason":"not true","rejection_evidence_ref":"review:reject-memory"}`))
	if err != nil {
		t.Fatalf("POST reject failed: %v", err)
	}
	defer rejectResp.Body.Close()
	if rejectResp.StatusCode != http.StatusOK {
		t.Fatalf("reject status = %d, want 200", rejectResp.StatusCode)
	}
	var rejected map[string]interface{}
	if err := json.NewDecoder(rejectResp.Body).Decode(&rejected); err != nil {
		t.Fatalf("decode rejected candidate: %v", err)
	}
	if rejected["status"] != "rejected" || rejected["rejection_evidence_ref"] != "review:reject-memory" {
		t.Fatalf("rejected candidate = %#v, want rejected status and evidence", rejected)
	}
	server.Close()

	restarted := newTestKernel(t, ledgerPath)
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	rejectedListResp, err := getWithAuth(restartedServer.URL + "/memory/candidates?status=rejected")
	if err != nil {
		t.Fatalf("GET rejected candidates failed: %v", err)
	}
	defer rejectedListResp.Body.Close()
	if rejectedListResp.StatusCode != http.StatusOK {
		t.Fatalf("rejected list status = %d, want 200", rejectedListResp.StatusCode)
	}
	var rejectedList MemoryCandidateListResponse
	if err := json.NewDecoder(rejectedListResp.Body).Decode(&rejectedList); err != nil {
		t.Fatalf("decode rejected candidates: %v", err)
	}
	if len(rejectedList.Items) != 1 || rejectedList.Items[0].CandidateID != candidate.CandidateID || rejectedList.Items[0].Status != "rejected" {
		t.Fatalf("rejected candidates = %+v, want rejected candidate", rejectedList.Items)
	}

	readResp, err := getWithAuth(restartedServer.URL + "/memory/candidates/" + candidate.CandidateID)
	if err != nil {
		t.Fatalf("GET rejected candidate failed: %v", err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("rejected read status = %d, want 200", readResp.StatusCode)
	}
	var readBack map[string]interface{}
	if err := json.NewDecoder(readResp.Body).Decode(&readBack); err != nil {
		t.Fatalf("decode rejected candidate read: %v", err)
	}
	if readBack["status"] != "rejected" || readBack["rejection_evidence_ref"] != "review:reject-memory" {
		t.Fatalf("rejected candidate read = %#v, want rejected status and evidence", readBack)
	}

	pendingResp, err := getWithAuth(restartedServer.URL + "/memory/candidates?status=pending")
	if err != nil {
		t.Fatalf("GET pending candidates failed: %v", err)
	}
	defer pendingResp.Body.Close()
	var pending MemoryCandidateListResponse
	if err := json.NewDecoder(pendingResp.Body).Decode(&pending); err != nil {
		t.Fatalf("decode pending candidates: %v", err)
	}
	if len(pending.Items) != 0 {
		t.Fatalf("pending candidates = %+v, want none after rejection", pending.Items)
	}

	turnPayload := []byte(`{"session_id":"http-memory-reject-consumer","input_items":[{"type":"text","text":"你记得雨伞偏好吗？"}]}`)
	turnResp, err := postJSONWithAuth(restartedServer.URL+"/turn", turnPayload)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	if turnResp.StatusCode != http.StatusOK {
		t.Fatalf("turn status = %d, want 200", turnResp.StatusCode)
	}
	var turn TurnResponse
	if err := json.NewDecoder(turnResp.Body).Decode(&turn); err != nil {
		t.Fatalf("decode turn response: %v", err)
	}
	if strings.Contains(turn.Final.Text, "红色雨伞") {
		t.Fatalf("rejected memory was recalled in final text: %q", turn.Final.Text)
	}

	sourceProjectionResp, err := getWithAuth(restartedServer.URL + "/sessions/http-memory-reject-source")
	if err != nil {
		t.Fatalf("GET rejected source session failed: %v", err)
	}
	defer sourceProjectionResp.Body.Close()
	if sourceProjectionResp.StatusCode != http.StatusOK {
		t.Fatalf("source session status = %d, want 200", sourceProjectionResp.StatusCode)
	}
	var sourceProjection SessionProjection
	if err := json.NewDecoder(sourceProjectionResp.Body).Decode(&sourceProjection); err != nil {
		t.Fatalf("decode rejected source session: %v", err)
	}
	if len(sourceProjection.MemoryCandidates) != 1 {
		t.Fatalf("len(MemoryCandidates) = %d, want one rejected candidate", len(sourceProjection.MemoryCandidates))
	}
	if sourceProjection.MemoryCandidates[0].Status != MemoryCandidateRejected ||
		sourceProjection.MemoryCandidates[0].RejectionEvidenceRef != "review:reject-memory" {
		t.Fatalf("session rejected candidate = %+v, want rejected evidence projection", sourceProjection.MemoryCandidates[0])
	}
}

func TestHTTPRejectedMemoryCandidateCannotBeApproved(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-reject-then-approve",
		Text:      "rejected memory should stay rejected",
		SourceRef: "turn:http-memory-reject-then-approve",
	})
	rejectResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/reject", []byte(`{"rejection_authority":"runtime:test","rejection_reason":"not true","rejection_evidence_ref":"review:reject-then-approve"}`))
	if err != nil {
		t.Fatalf("POST reject failed: %v", err)
	}
	rejectResp.Body.Close()
	if rejectResp.StatusCode != http.StatusOK {
		t.Fatalf("reject status = %d, want 200", rejectResp.StatusCode)
	}

	approvalPayload, err := json.Marshal(testApprovalRequest("approval:rejected-candidate"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("approve status = %d, want 400", approveResp.StatusCode)
	}

	readResp, err := getWithAuth(server.URL + "/memory/candidates/" + candidate.CandidateID)
	if err != nil {
		t.Fatalf("GET memory candidate failed: %v", err)
	}
	defer readResp.Body.Close()
	var readBack map[string]interface{}
	if err := json.NewDecoder(readResp.Body).Decode(&readBack); err != nil {
		t.Fatalf("decode memory candidate: %v", err)
	}
	if readBack["status"] != "rejected" {
		t.Fatalf("candidate status after rejected approval = %#v, want rejected", readBack["status"])
	}
}

func TestHTTPApprovedMemoryCandidateCannotBeRejected(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-approve-then-reject",
		Text:      "approved memory should stay approved",
		SourceRef: "turn:http-memory-approve-then-reject",
	})
	approvalPayload, err := json.Marshal(testApprovalRequest("approval:approve-then-reject"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveResp.StatusCode)
	}

	rejectResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/reject", []byte(`{"rejection_authority":"runtime:test","rejection_reason":"not true","rejection_evidence_ref":"review:approve-then-reject"}`))
	if err != nil {
		t.Fatalf("POST reject failed: %v", err)
	}
	defer rejectResp.Body.Close()
	if rejectResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("reject status = %d, want 400", rejectResp.StatusCode)
	}

	readResp, err := getWithAuth(server.URL + "/memory/candidates/" + candidate.CandidateID)
	if err != nil {
		t.Fatalf("GET memory candidate failed: %v", err)
	}
	defer readResp.Body.Close()
	var readBack MemoryCandidateProjection
	if err := json.NewDecoder(readResp.Body).Decode(&readBack); err != nil {
		t.Fatalf("decode memory candidate: %v", err)
	}
	if readBack.Status != MemoryCandidateApproved || readBack.ApprovalEvidenceRef != "approval:approve-then-reject" {
		t.Fatalf("candidate after rejected rejection = %+v, want approved evidence", readBack)
	}
}

func TestHTTPMemoryCandidateSupersedeCreatesPendingReplacementAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))

	candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-supersede-source",
		Text:      "我偏好中文回答",
		SourceRef: "turn:http-memory-supersede-source",
	})
	approvalPayload, err := json.Marshal(testApprovalRequest("approval:supersede-source"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveResp.StatusCode)
	}

	supersedeResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/supersede", []byte(`{"replacement_text":"我偏好英文回答","replacement_source_ref":"review:supersede-replacement","supersession_authority":"runtime:test","supersession_reason":"user corrected preference","supersession_evidence_ref":"review:supersede-memory"}`))
	if err != nil {
		t.Fatalf("POST supersede failed: %v", err)
	}
	defer supersedeResp.Body.Close()
	if supersedeResp.StatusCode != http.StatusOK {
		t.Fatalf("supersede status = %d, want 200", supersedeResp.StatusCode)
	}
	var supersession MemorySupersessionProjection
	if err := json.NewDecoder(supersedeResp.Body).Decode(&supersession); err != nil {
		t.Fatalf("decode supersession response: %v", err)
	}
	if supersession.Superseded.Status != MemoryCandidateSuperseded ||
		supersession.Superseded.ReplacementCandidateID == "" ||
		supersession.Superseded.SupersessionEvidenceRef != "review:supersede-memory" {
		t.Fatalf("superseded candidate = %+v, want superseded evidence and replacement id", supersession.Superseded)
	}
	if supersession.Replacement.Status != MemoryCandidatePending ||
		supersession.Replacement.CandidateID == candidate.CandidateID ||
		supersession.Replacement.Text != "我偏好英文回答" ||
		supersession.Replacement.SourceRef != "review:supersede-replacement" {
		t.Fatalf("replacement candidate = %+v, want pending replacement candidate", supersession.Replacement)
	}
	server.Close()

	restarted := newTestKernel(t, ledgerPath)
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	readOriginalResp, err := getWithAuth(restartedServer.URL + "/memory/candidates/" + candidate.CandidateID)
	if err != nil {
		t.Fatalf("GET original candidate failed: %v", err)
	}
	defer readOriginalResp.Body.Close()
	if readOriginalResp.StatusCode != http.StatusOK {
		t.Fatalf("read original status = %d, want 200", readOriginalResp.StatusCode)
	}
	var original MemoryCandidateProjection
	if err := json.NewDecoder(readOriginalResp.Body).Decode(&original); err != nil {
		t.Fatalf("decode original: %v", err)
	}
	if original.Status != MemoryCandidateSuperseded || original.ReplacementCandidateID != supersession.Replacement.CandidateID {
		t.Fatalf("original after restart = %+v, want superseded with replacement id", original)
	}

	readReplacementResp, err := getWithAuth(restartedServer.URL + "/memory/candidates/" + supersession.Replacement.CandidateID)
	if err != nil {
		t.Fatalf("GET replacement candidate failed: %v", err)
	}
	defer readReplacementResp.Body.Close()
	if readReplacementResp.StatusCode != http.StatusOK {
		t.Fatalf("read replacement status = %d, want 200", readReplacementResp.StatusCode)
	}
	var replacement MemoryCandidateProjection
	if err := json.NewDecoder(readReplacementResp.Body).Decode(&replacement); err != nil {
		t.Fatalf("decode replacement: %v", err)
	}
	if replacement.Status != MemoryCandidatePending {
		t.Fatalf("replacement status = %q, want pending", replacement.Status)
	}

	oldTurn, err := restarted.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "http-memory-supersede-old-consumer",
		InputItems: []InputItem{{Type: "text", Text: "你记得我的中文回答偏好吗？"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn old query returned error: %v", err)
	}
	if strings.Contains(oldTurn.Final.Text, "我偏好中文回答") || strings.Contains(oldTurn.Final.Text, "我偏好英文回答") {
		t.Fatalf("final text = %q, want no superseded or pending replacement recall", oldTurn.Final.Text)
	}
	oldProjection, err := restarted.Session("http-memory-supersede-old-consumer")
	if err != nil {
		t.Fatalf("old consumer Session returned error: %v", err)
	}
	if len(oldProjection.Turns) != 1 {
		t.Fatalf("old turns = %+v, want one turn", oldProjection.Turns)
	}

	approveReplacementPayload, err := json.Marshal(testApprovalRequest("approval:supersede-replacement"))
	if err != nil {
		t.Fatalf("marshal replacement approval: %v", err)
	}
	approveReplacementResp, err := postJSONWithAuth(restartedServer.URL+"/memory/candidates/"+replacement.CandidateID+"/approve", approveReplacementPayload)
	if err != nil {
		t.Fatalf("POST replacement approve failed: %v", err)
	}
	approveReplacementResp.Body.Close()
	if approveReplacementResp.StatusCode != http.StatusOK {
		t.Fatalf("replacement approve status = %d, want 200", approveReplacementResp.StatusCode)
	}

	newTurn, err := restarted.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "http-memory-supersede-new-consumer",
		InputItems: []InputItem{{Type: "text", Text: "你记得我的英文回答偏好吗？"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn new query returned error: %v", err)
	}
	if strings.Contains(newTurn.Final.Text, "我偏好英文回答") || strings.Contains(newTurn.Final.Text, "我偏好中文回答") {
		t.Fatalf("final text = %q, want no automatic recall for approved replacement", newTurn.Final.Text)
	}
}

func TestSupersedeMemoryCandidateIsIdempotentWithoutAppendingDuplicateReplacement(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-supersede-idempotent",
		Text:      "old candidate",
		SourceRef: "turn:memory-supersede-idempotent",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}
	first, err := k.SupersedeMemoryCandidate(candidate.CandidateID, MemorySupersessionRequest{
		ReplacementText:         "replacement candidate",
		ReplacementSourceRef:    "review:first-supersede-source",
		SupersessionAuthority:   "runtime:test",
		SupersessionReason:      "first supersede",
		SupersessionEvidenceRef: "review:first-supersede",
	})
	if err != nil {
		t.Fatalf("first SupersedeMemoryCandidate returned error: %v", err)
	}
	second, err := k.SupersedeMemoryCandidate(candidate.CandidateID, MemorySupersessionRequest{
		ReplacementText:         "different replacement must not replace",
		ReplacementSourceRef:    "review:second-supersede-source",
		SupersessionAuthority:   "runtime:test",
		SupersessionReason:      "second supersede",
		SupersessionEvidenceRef: "review:second-supersede",
	})
	if err != nil {
		t.Fatalf("second SupersedeMemoryCandidate returned error: %v", err)
	}
	if second.Superseded.SupersessionEvidenceRef != first.Superseded.SupersessionEvidenceRef ||
		second.Replacement.CandidateID != first.Replacement.CandidateID ||
		second.Replacement.Text != first.Replacement.Text {
		t.Fatalf("second supersession = %+v, want original %+v", second, first)
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	supersedeEvents := 0
	for _, event := range events {
		if event.Type == "memory.candidate.superseded" && event.CandidateID == candidate.CandidateID {
			supersedeEvents++
		}
	}
	if supersedeEvents != 1 {
		t.Fatalf("supersede event count = %d, want 1", supersedeEvents)
	}
}

func TestHTTPMemoryCandidateSupersedeRejectsMissingEvidence(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/memory/candidates/anything/supersede", []byte(`{"supersession_authority":"runtime:test"}`))
	if err != nil {
		t.Fatalf("POST supersede failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestMemoryReplayRejectsReviewAfterSupersede(t *testing.T) {
	createdAt := time.Date(2026, 6, 22, 3, 0, 0, 0, time.UTC)
	supersededAt := createdAt.Add(time.Minute)
	approvedAt := createdAt.Add(2 * time.Minute)
	original := MemoryCandidateProjection{
		CandidateID: "mem-review-after-supersede",
		SessionID:   "memory-review-after-supersede",
		Text:        "old memory",
		SourceRef:   "turn:memory-review-after-supersede",
		Status:      MemoryCandidatePending,
		CreatedAt:   createdAt,
	}
	replacement := MemoryCandidateProjection{
		CandidateID: "mem-review-after-supersede-replacement",
		SessionID:   original.SessionID,
		Text:        "new memory",
		SourceRef:   "review:memory-review-after-supersede",
		Status:      MemoryCandidatePending,
		CreatedAt:   supersededAt,
	}
	superseded := original
	superseded.Status = MemoryCandidateSuperseded
	superseded.SupersessionAuthority = "runtime:test"
	superseded.SupersessionReason = "replaced"
	superseded.SupersessionEvidenceRef = "review:supersede-before-approve"
	superseded.ReplacementCandidateID = replacement.CandidateID
	superseded.SupersededAt = &supersededAt
	approved := original
	approved.Status = MemoryCandidateApproved
	approved.ApprovalAuthority = "runtime:test"
	approved.ApprovalReason = "late approval"
	approved.ApprovalEvidenceRef = "approval:after-supersede"
	approved.ApprovedAt = &approvedAt

	k := &Kernel{
		ledger: newStaticLedger(
			StoredEvent{
				EventID:     "evt-memory-review-after-supersede-created",
				SessionID:   original.SessionID,
				CandidateID: original.CandidateID,
				Type:        "memory.candidate.created",
				CreatedAt:   createdAt,
				Data:        EventData{MemoryCandidate: &original},
			},
			StoredEvent{
				EventID:     "evt-memory-review-after-supersede-superseded",
				SessionID:   original.SessionID,
				CandidateID: original.CandidateID,
				Type:        "memory.candidate.superseded",
				CreatedAt:   supersededAt,
				Data: EventData{
					MemoryCandidate:            &superseded,
					ReplacementMemoryCandidate: &replacement,
				},
			},
			StoredEvent{
				EventID:     "evt-memory-review-after-supersede-approved",
				SessionID:   original.SessionID,
				CandidateID: original.CandidateID,
				Type:        "memory.candidate.approved",
				CreatedAt:   approvedAt,
				Data:        EventData{MemoryCandidate: &approved},
			},
		),
		provider:     FakeProvider{},
		runtimeToken: testRuntimeToken,
		toolPolicy:   normalizedToolPolicy(ToolPolicy{}),
		clock:        time.Now,
	}

	if _, err := k.MemoryCandidate(original.CandidateID); err == nil || !strings.Contains(err.Error(), "competing memory review evidence") {
		t.Fatalf("MemoryCandidate error = %v, want competing memory review evidence", err)
	}
	if _, err := k.Session(original.SessionID); err == nil || !strings.Contains(err.Error(), "competing memory review evidence") {
		t.Fatalf("Session error = %v, want competing memory review evidence", err)
	}
}

func TestMemoryReplayRejectsDuplicateSupersedeWithModifiedReplacement(t *testing.T) {
	createdAt := time.Date(2026, 6, 22, 3, 30, 0, 0, time.UTC)
	supersededAt := createdAt.Add(time.Minute)
	original := MemoryCandidateProjection{
		CandidateID: "mem-duplicate-supersede-original",
		SessionID:   "memory-duplicate-supersede",
		Text:        "old memory",
		SourceRef:   "turn:memory-duplicate-supersede",
		Status:      MemoryCandidatePending,
		CreatedAt:   createdAt,
	}
	replacement := MemoryCandidateProjection{
		CandidateID: "mem-duplicate-supersede-replacement",
		SessionID:   original.SessionID,
		Text:        "new memory",
		SourceRef:   "review:duplicate-supersede-source",
		Status:      MemoryCandidatePending,
		CreatedAt:   supersededAt,
	}
	superseded := original
	superseded.Status = MemoryCandidateSuperseded
	superseded.SupersessionAuthority = "runtime:test"
	superseded.SupersessionReason = "replace old memory"
	superseded.SupersessionEvidenceRef = "review:duplicate-supersede"
	superseded.ReplacementCandidateID = replacement.CandidateID
	superseded.SupersededAt = &supersededAt
	mutatedReplacement := replacement
	mutatedReplacement.Text = "silently mutated replacement"

	k := &Kernel{
		ledger: newStaticLedger(
			StoredEvent{
				EventID:     "evt-duplicate-supersede-created",
				SessionID:   original.SessionID,
				CandidateID: original.CandidateID,
				Type:        "memory.candidate.created",
				CreatedAt:   createdAt,
				Data:        EventData{MemoryCandidate: &original},
			},
			StoredEvent{
				EventID:     "evt-duplicate-supersede-first",
				SessionID:   original.SessionID,
				CandidateID: original.CandidateID,
				Type:        "memory.candidate.superseded",
				CreatedAt:   supersededAt,
				Data: EventData{
					MemoryCandidate:            &superseded,
					ReplacementMemoryCandidate: &replacement,
				},
			},
			StoredEvent{
				EventID:     "evt-duplicate-supersede-mutated",
				SessionID:   original.SessionID,
				CandidateID: original.CandidateID,
				Type:        "memory.candidate.superseded",
				CreatedAt:   supersededAt.Add(time.Minute),
				Data: EventData{
					MemoryCandidate:            &superseded,
					ReplacementMemoryCandidate: &mutatedReplacement,
				},
			},
		),
		provider:     FakeProvider{},
		runtimeToken: testRuntimeToken,
		toolPolicy:   normalizedToolPolicy(ToolPolicy{}),
		clock:        time.Now,
	}

	if _, err := k.MemoryCandidate(replacement.CandidateID); err == nil || !strings.Contains(err.Error(), "competing memory review evidence") {
		t.Fatalf("MemoryCandidate error = %v, want competing memory review evidence", err)
	}
	if _, err := k.Session(original.SessionID); err == nil || !strings.Contains(err.Error(), "competing memory review evidence") {
		t.Fatalf("Session error = %v, want competing memory review evidence", err)
	}
}

func TestHTTPSupersededMemoryCandidateCannotBeApprovedOrRejected(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-superseded-terminal",
		Text:      "old terminal candidate",
		SourceRef: "turn:http-memory-superseded-terminal",
	})
	supersedeResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/supersede", []byte(`{"replacement_text":"new terminal candidate","replacement_source_ref":"review:terminal-supersede-source","supersession_authority":"runtime:test","supersession_reason":"replace terminal candidate","supersession_evidence_ref":"review:terminal-supersede"}`))
	if err != nil {
		t.Fatalf("POST supersede failed: %v", err)
	}
	supersedeResp.Body.Close()
	if supersedeResp.StatusCode != http.StatusOK {
		t.Fatalf("supersede status = %d, want 200", supersedeResp.StatusCode)
	}

	approvalPayload, err := json.Marshal(testApprovalRequest("approval:superseded-candidate"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve superseded failed: %v", err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("approve superseded status = %d, want 400", approveResp.StatusCode)
	}

	rejectResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/reject", []byte(`{"rejection_authority":"runtime:test","rejection_reason":"not true","rejection_evidence_ref":"review:reject-superseded"}`))
	if err != nil {
		t.Fatalf("POST reject superseded failed: %v", err)
	}
	defer rejectResp.Body.Close()
	if rejectResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("reject superseded status = %d, want 400", rejectResp.StatusCode)
	}
}

func TestHTTPMemoryCandidateSupersedeRejectsInvalidControlRefs(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "http-memory-supersede-bad-audit",
		Text:      "old memory",
		SourceRef: "turn:http-memory-supersede-bad-audit",
	})
	cases := map[string][]byte{
		"invalid replacement source ref": []byte(`{"replacement_text":"new memory","replacement_source_ref":"free text","supersession_authority":"runtime:test","supersession_reason":"replace","supersession_evidence_ref":"review:valid-supersede"}`),
		"invalid authority":              []byte(`{"replacement_text":"new memory","replacement_source_ref":"review:valid-replacement","supersession_authority":"root","supersession_reason":"replace","supersession_evidence_ref":"review:valid-supersede"}`),
		"invalid evidence ref":           []byte(`{"replacement_text":"new memory","replacement_source_ref":"review:valid-replacement","supersession_authority":"runtime:test","supersession_reason":"replace","supersession_evidence_ref":"free text"}`),
		"secret replacement source ref":  []byte(`{"replacement_text":"new memory","replacement_source_ref":"review:api_key=sk-memory-secret","supersession_authority":"runtime:test","supersession_reason":"replace","supersession_evidence_ref":"review:valid-supersede"}`),
		"secret authority":               []byte(`{"replacement_text":"new memory","replacement_source_ref":"review:valid-replacement","supersession_authority":"runtime:api_key=sk-memory-secret","supersession_reason":"replace","supersession_evidence_ref":"review:valid-supersede"}`),
		"secret evidence ref":            []byte(`{"replacement_text":"new memory","replacement_source_ref":"review:valid-replacement","supersession_authority":"runtime:test","supersession_reason":"replace","supersession_evidence_ref":"review:api_key=sk-memory-secret"}`),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			resp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/supersede", body)
			if err != nil {
				t.Fatalf("POST supersede failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestRejectMemoryCandidateIsIdempotentWithoutAppendingDuplicateEvent(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "memory-duplicate-reject",
		Text:      "duplicate rejection should not append",
		SourceRef: "turn:memory-duplicate-reject",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}

	first, err := k.RejectMemoryCandidate(candidate.CandidateID, MemoryRejectionRequest{
		RejectionAuthority:   "runtime:test",
		RejectionReason:      "not true",
		RejectionEvidenceRef: "review:first-reject",
	})
	if err != nil {
		t.Fatalf("first RejectMemoryCandidate returned error: %v", err)
	}
	second, err := k.RejectMemoryCandidate(candidate.CandidateID, MemoryRejectionRequest{
		RejectionAuthority:   "runtime:test",
		RejectionReason:      "different reason must not overwrite",
		RejectionEvidenceRef: "review:second-reject",
	})
	if err != nil {
		t.Fatalf("second RejectMemoryCandidate returned error: %v", err)
	}
	if second.RejectionEvidenceRef != first.RejectionEvidenceRef {
		t.Fatalf("second rejection evidence = %q, want original %q", second.RejectionEvidenceRef, first.RejectionEvidenceRef)
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	rejectionEvents := 0
	for _, event := range events {
		if event.Type == "memory.candidate.rejected" && event.CandidateID == candidate.CandidateID {
			rejectionEvents++
		}
	}
	if rejectionEvents != 1 {
		t.Fatalf("rejection event count = %d, want 1", rejectionEvents)
	}
}

func TestConcurrentMemoryReviewWritesOnlyOneTerminalDecision(t *testing.T) {
	createdAt := time.Date(2026, 6, 22, 1, 0, 0, 0, time.UTC)
	candidate := MemoryCandidateProjection{
		CandidateID: "mem-review-race",
		SessionID:   "memory-review-race",
		Text:        "race-sensitive memory",
		SourceRef:   "turn:memory-review-race",
		Status:      MemoryCandidatePending,
		CreatedAt:   createdAt,
	}
	ledger := newReviewRaceLedger(StoredEvent{
		EventID:     "evt-review-race-created",
		SessionID:   candidate.SessionID,
		CandidateID: candidate.CandidateID,
		Type:        "memory.candidate.created",
		CreatedAt:   createdAt,
		Data:        EventData{MemoryCandidate: &candidate},
	})
	k := &Kernel{
		ledger:       ledger,
		provider:     FakeProvider{},
		runtimeToken: testRuntimeToken,
		toolPolicy:   normalizedToolPolicy(ToolPolicy{}),
		clock: func() time.Time {
			return time.Date(2026, 6, 22, 1, 1, 0, 0, time.UTC)
		},
	}

	results := make(chan error, 2)
	go func() {
		_, err := k.ApproveMemoryCandidate(candidate.CandidateID, testApprovalRequest("approval:race"))
		results <- err
	}()
	<-ledger.firstTerminalAppendStarted
	go func() {
		_, err := k.RejectMemoryCandidate(candidate.CandidateID, MemoryRejectionRequest{
			RejectionAuthority:   "runtime:test",
			RejectionReason:      "not true",
			RejectionEvidenceRef: "review:race",
		})
		results <- err
	}()

	successes := 0
	for range 2 {
		if err := <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful terminal review decisions = %d, want 1", successes)
	}
	if terminalEvents := ledger.terminalReviewEvents(candidate.CandidateID); len(terminalEvents) != 1 {
		t.Fatalf("terminal review events = %+v, want exactly one terminal event", terminalEvents)
	}
}

func TestConcurrentMemorySupersedeWritesOnlyOneTerminalDecision(t *testing.T) {
	createdAt := time.Date(2026, 6, 22, 1, 10, 0, 0, time.UTC)
	candidate := MemoryCandidateProjection{
		CandidateID: "mem-supersede-race",
		SessionID:   "memory-supersede-race",
		Text:        "race-sensitive memory",
		SourceRef:   "turn:memory-supersede-race",
		Status:      MemoryCandidatePending,
		CreatedAt:   createdAt,
	}
	ledger := newReviewRaceLedger(StoredEvent{
		EventID:     "evt-supersede-race-created",
		SessionID:   candidate.SessionID,
		CandidateID: candidate.CandidateID,
		Type:        "memory.candidate.created",
		CreatedAt:   createdAt,
		Data:        EventData{MemoryCandidate: &candidate},
	})
	k := &Kernel{
		ledger:       ledger,
		provider:     FakeProvider{},
		runtimeToken: testRuntimeToken,
		toolPolicy:   normalizedToolPolicy(ToolPolicy{}),
		clock: func() time.Time {
			return time.Date(2026, 6, 22, 1, 11, 0, 0, time.UTC)
		},
	}

	results := make(chan error, 2)
	go func() {
		_, err := k.SupersedeMemoryCandidate(candidate.CandidateID, MemorySupersessionRequest{
			ReplacementText:         "replacement memory",
			ReplacementSourceRef:    "review:supersede-race-source",
			SupersessionAuthority:   "runtime:test",
			SupersessionReason:      "replace in race",
			SupersessionEvidenceRef: "review:supersede-race",
		})
		results <- err
	}()
	<-ledger.firstTerminalAppendStarted
	go func() {
		_, err := k.ApproveMemoryCandidate(candidate.CandidateID, testApprovalRequest("approval:supersede-race"))
		results <- err
	}()

	successes := 0
	for range 2 {
		if err := <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful terminal review decisions = %d, want 1", successes)
	}
	if terminalEvents := ledger.terminalReviewEvents(candidate.CandidateID); len(terminalEvents) != 1 {
		t.Fatalf("terminal review events = %+v, want exactly one terminal event", terminalEvents)
	}
}

func TestHTTPRejectMemoryCandidateRejectsMissingEvidence(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/memory/candidates/anything/reject", []byte(`{"rejection_authority":"runtime"}`))
	if err != nil {
		t.Fatalf("POST reject failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHTTPRejectMemoryCandidateRejectsInvalidControlRefs(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	cases := map[string]MemoryRejectionRequest{
		"invalid authority": {
			RejectionAuthority:   "runtime",
			RejectionReason:      "reject",
			RejectionEvidenceRef: "review:valid-memory-rejection",
		},
		"invalid evidence ref": {
			RejectionAuthority:   "runtime:test",
			RejectionReason:      "reject",
			RejectionEvidenceRef: "free text",
		},
		"secret authority": {
			RejectionAuthority:   "runtime:api_key=sk-memory-secret",
			RejectionReason:      "reject",
			RejectionEvidenceRef: "review:valid-memory-rejection",
		},
		"secret evidence ref": {
			RejectionAuthority:   "runtime:test",
			RejectionReason:      "reject",
			RejectionEvidenceRef: "review:api_key=sk-memory-secret",
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
				SessionID: "http-memory-reject-bad-audit-" + strings.ReplaceAll(name, " ", "-"),
				Text:      "memory",
				SourceRef: "turn:http-memory-reject-bad-audit",
			})
			payload, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			resp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/reject", payload)
			if err != nil {
				t.Fatalf("POST reject failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestHTTPApproveUnknownMemoryCandidateReturnsNotFound(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	approvalPayload, err := json.Marshal(testApprovalRequest("approval:missing"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	resp, err := postJSONWithAuth(server.URL+"/memory/candidates/missing/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHTTPApproveMemoryCandidateRejectsMissingEvidence(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/memory/candidates/anything/approve", []byte(`{"approval_authority":"runtime"}`))
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHTTPApproveMemoryCandidateRejectsInvalidControlRefs(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	cases := map[string]MemoryApprovalRequest{
		"invalid authority": {
			ApprovalAuthority:   "runtime",
			ApprovalReason:      "approve",
			ApprovalEvidenceRef: "approval:valid-memory-approval",
		},
		"invalid evidence ref": {
			ApprovalAuthority:   "runtime:test",
			ApprovalReason:      "approve",
			ApprovalEvidenceRef: "free text",
		},
		"secret authority": {
			ApprovalAuthority:   "runtime:api_key=sk-memory-secret",
			ApprovalReason:      "approve",
			ApprovalEvidenceRef: "approval:valid-memory-approval",
		},
		"secret evidence ref": {
			ApprovalAuthority:   "runtime:test",
			ApprovalReason:      "approve",
			ApprovalEvidenceRef: "approval:api_key=sk-memory-secret",
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
				SessionID: "http-memory-approve-bad-audit-" + strings.ReplaceAll(name, " ", "-"),
				Text:      "memory",
				SourceRef: "turn:http-memory-approve-bad-audit",
			})
			payload, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			resp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/approve", payload)
			if err != nil {
				t.Fatalf("POST approve failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestHTTPMemoryCandidateAccumulationMetadataPersistsWithoutAutoRecall(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))

	candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID:   "accumulation-source",
		Text:        "用户通常偏好先复用成熟应用层依赖",
		SourceRef:   "turn:accumulation-source",
		Kind:        "preference",
		Scope:       "global",
		AppliesWhen: "building application-layer UI or adapters",
		YieldsTo:    "current task instruction or project contract",
		Strength:    "preference",
	})
	approvalPayload, err := json.Marshal(testApprovalRequest("approval:accumulation-source"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveResp.StatusCode)
	}
	server.Close()

	restarted := newTestKernel(t, ledgerPath)
	stored, err := restarted.MemoryCandidate(candidate.CandidateID)
	if err != nil {
		t.Fatalf("MemoryCandidate returned error: %v", err)
	}
	if stored.Kind != "preference" ||
		stored.Scope != "global" ||
		stored.AppliesWhen != "building application-layer UI or adapters" ||
		stored.YieldsTo != "current task instruction or project contract" ||
		stored.Strength != "preference" {
		t.Fatalf("stored accumulation metadata = %+v", stored)
	}

	resp, err := restarted.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "accumulation-consumer",
		InputItems: []InputItem{{Type: "text", Text: "我们现在应用层该怎么选依赖？"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if strings.Contains(resp.Final.Text, "用户通常偏好先复用成熟应用层依赖") {
		t.Fatalf("accumulation was auto recalled in final text: %q", resp.Final.Text)
	}
}

func TestCreateMemoryCandidateRejectsUnknownAccumulationMetadata(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))

	cases := map[string]MemoryCandidateRequest{
		"unknown kind": {
			SessionID: "accumulation-bad-kind",
			Text:      "memory",
			SourceRef: "turn:accumulation-bad-kind",
			Kind:      "kernel_override",
		},
		"unknown scope": {
			SessionID: "accumulation-bad-scope",
			Text:      "memory",
			SourceRef: "turn:accumulation-bad-scope",
			Scope:     "everywhere",
		},
		"unknown strength": {
			SessionID: "accumulation-bad-strength",
			Text:      "memory",
			SourceRef: "turn:accumulation-bad-strength",
			Strength:  "must_obey",
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := k.CreateMemoryCandidate(req); err == nil {
				t.Fatal("CreateMemoryCandidate returned nil error for unknown accumulation metadata")
			}
		})
	}
}

func TestHTTPMemoryCandidateForgetIsDurableTerminalAndIdempotent(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))

	candidate := createMemoryCandidateOverHTTP(t, server.URL, MemoryCandidateRequest{
		SessionID: "forget-source",
		Text:      "outdated user preference",
		SourceRef: "turn:forget-source",
		Kind:      "preference",
		Scope:     "global",
		Strength:  "preference",
	})
	approvalPayload, err := json.Marshal(testApprovalRequest("approval:forget-source"))
	if err != nil {
		t.Fatalf("marshal approval request: %v", err)
	}
	approveResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/approve", approvalPayload)
	if err != nil {
		t.Fatalf("POST approve failed: %v", err)
	}
	approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveResp.StatusCode)
	}

	for i := 0; i < 2; i++ {
		forgetPayload, err := json.Marshal(testForgetRequest("review:forget-source"))
		if err != nil {
			t.Fatalf("marshal forget request: %v", err)
		}
		forgetResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/forget", forgetPayload)
		if err != nil {
			t.Fatalf("POST forget failed: %v", err)
		}
		defer forgetResp.Body.Close()
		if forgetResp.StatusCode != http.StatusOK {
			t.Fatalf("forget status = %d, want 200", forgetResp.StatusCode)
		}
		var forgotten MemoryCandidateProjection
		if err := json.NewDecoder(forgetResp.Body).Decode(&forgotten); err != nil {
			t.Fatalf("decode forgotten response: %v", err)
		}
		if forgotten.Status != MemoryCandidateForgotten || forgotten.ForgetEvidenceRef != "review:forget-source" {
			t.Fatalf("forgotten candidate = %+v", forgotten)
		}
	}

	approvedResp, err := getWithAuth(server.URL + "/memory/candidates?status=approved")
	if err != nil {
		t.Fatalf("GET approved failed: %v", err)
	}
	defer approvedResp.Body.Close()
	var approved MemoryCandidateListResponse
	if err := json.NewDecoder(approvedResp.Body).Decode(&approved); err != nil {
		t.Fatalf("decode approved candidates: %v", err)
	}
	for _, item := range approved.Items {
		if item.CandidateID == candidate.CandidateID {
			t.Fatalf("forgotten candidate appeared as approved: %+v", approved.Items)
		}
	}

	forgottenResp, err := getWithAuth(server.URL + "/memory/candidates?status=forgotten")
	if err != nil {
		t.Fatalf("GET forgotten failed: %v", err)
	}
	defer forgottenResp.Body.Close()
	var forgotten MemoryCandidateListResponse
	if err := json.NewDecoder(forgottenResp.Body).Decode(&forgotten); err != nil {
		t.Fatalf("decode forgotten candidates: %v", err)
	}
	if len(forgotten.Items) != 1 || forgotten.Items[0].CandidateID != candidate.CandidateID {
		t.Fatalf("forgotten candidates = %+v, want only forgotten candidate", forgotten.Items)
	}

	rejectPayload, err := json.Marshal(MemoryRejectionRequest{
		RejectionAuthority:   "runtime:test",
		RejectionReason:      "late reject",
		RejectionEvidenceRef: "review:forget-source-late",
	})
	if err != nil {
		t.Fatalf("marshal rejection request: %v", err)
	}
	rejectResp, err := postJSONWithAuth(server.URL+"/memory/candidates/"+candidate.CandidateID+"/reject", rejectPayload)
	if err != nil {
		t.Fatalf("POST reject failed: %v", err)
	}
	defer rejectResp.Body.Close()
	if rejectResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("reject-after-forget status = %d, want 400", rejectResp.StatusCode)
	}
	server.Close()

	restarted := newTestKernel(t, ledgerPath)
	stored, err := restarted.MemoryCandidate(candidate.CandidateID)
	if err != nil {
		t.Fatalf("MemoryCandidate after restart returned error: %v", err)
	}
	if stored.Status != MemoryCandidateForgotten {
		t.Fatalf("stored status = %q, want forgotten", stored.Status)
	}
	events, err := restarted.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	forgetEvents := 0
	for _, event := range events {
		if event.Type == "memory.candidate.forgotten" && event.CandidateID == candidate.CandidateID {
			forgetEvents++
		}
	}
	if forgetEvents != 1 {
		t.Fatalf("forget event count = %d, want 1", forgetEvents)
	}
}

func TestMemoryCandidateReplayRejectsApprovalAfterForget(t *testing.T) {
	createdAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	forgottenAt := createdAt.Add(time.Minute)
	approvedAt := createdAt.Add(2 * time.Minute)
	candidate := MemoryCandidateProjection{
		CandidateID: "mem-replay-forgotten",
		SessionID:   "replay-forget-source",
		Text:        "forgotten claim",
		SourceRef:   "turn:replay-forget-source",
		Status:      MemoryCandidatePending,
		CreatedAt:   createdAt,
		Kind:        "memory_fact",
		Scope:       "global",
		Strength:    "weak_hint",
	}
	forgotten := candidate
	forgotten.Status = MemoryCandidateForgotten
	forgotten.ForgetAuthority = "runtime:test"
	forgotten.ForgetReason = "forget"
	forgotten.ForgetEvidenceRef = "review:replay-forget-source"
	forgotten.ForgottenAt = &forgottenAt
	approved := candidate
	approved.Status = MemoryCandidateApproved
	approved.ApprovalAuthority = "runtime:test"
	approved.ApprovalReason = "approve"
	approved.ApprovalEvidenceRef = "approval:replay-forget-source"
	approved.ApprovedAt = &approvedAt

	k := &Kernel{ledger: newStaticLedger(
		StoredEvent{
			EventID:     "evt-create",
			SessionID:   candidate.SessionID,
			CandidateID: candidate.CandidateID,
			Type:        "memory.candidate.created",
			CreatedAt:   createdAt,
			Data:        EventData{MemoryCandidate: &candidate},
		},
		StoredEvent{
			EventID:     "evt-forget",
			SessionID:   candidate.SessionID,
			CandidateID: candidate.CandidateID,
			Type:        "memory.candidate.forgotten",
			CreatedAt:   forgottenAt,
			Data:        EventData{MemoryCandidate: &forgotten},
		},
		StoredEvent{
			EventID:     "evt-approve",
			SessionID:   candidate.SessionID,
			CandidateID: candidate.CandidateID,
			Type:        "memory.candidate.approved",
			CreatedAt:   approvedAt,
			Data:        EventData{MemoryCandidate: &approved},
		},
	)}

	if _, err := k.MemoryCandidate(candidate.CandidateID); err == nil || !strings.Contains(err.Error(), "competing memory review evidence") {
		t.Fatalf("MemoryCandidate error = %v, want competing memory review evidence", err)
	}
}

func testForgetRequest(evidenceRef string) MemoryForgetRequest {
	return MemoryForgetRequest{
		ForgetAuthority:   "runtime:test",
		ForgetReason:      "forgotten in test",
		ForgetEvidenceRef: evidenceRef,
	}
}
