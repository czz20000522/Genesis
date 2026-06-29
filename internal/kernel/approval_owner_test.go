package kernel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApprovalRequiredCreatesPendingApprovalWithoutEffect(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "approval-pending-should-not-run.txt")
	k, provider := newApprovalRequiredTurnKernel(t, workspace, target)

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-owner-pending",
		InputItems: []InputItem{{Type: "text", Text: "write after approval"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if resp.Final.Text != "approval owner feedback received" {
		t.Fatalf("final text = %q, want approval owner feedback received", resp.Final.Text)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("approval-required command created %q; stat err=%v", target, err)
	}
	requests := provider.Requests()
	if len(requests) != 2 {
		t.Fatalf("provider requests = %d, want tool feedback round", len(requests))
	}
	payload := decodeJSONMap(t, requests[1].ToolRounds[0].Results[0].Content)
	if payload["status"] != "approval_required" || payload["executed"] != false {
		t.Fatalf("tool result payload = %+v, want approval_required without execution", payload)
	}
	for _, forbidden := range []string{"approval_id", "permission_mode", "authority_policy", "sandbox_profile", "approval_policy", "policy_snapshot", "cwd", "command"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("tool result payload = %+v, must not expose control-plane field %q", payload, forbidden)
		}
	}

	projection, err := k.Session("approval-owner-pending")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Approvals) != 1 {
		t.Fatalf("approvals = %+v, want one pending approval", projection.Approvals)
	}
	approval := projection.Approvals[0]
	if approval.Status != ApprovalStatusPending {
		t.Fatalf("approval status = %q, want pending", approval.Status)
	}
	if approval.OperationID == "" || approval.SessionID != "approval-owner-pending" || approval.PolicySnapshot.ApprovalPolicy != ApprovalPolicyOnRequest {
		t.Fatalf("approval = %+v, want bound operation/session/policy snapshot", approval)
	}
	if approval.Effect.Tool != "shell_exec" || approval.Effect.CommandPreview == "" {
		t.Fatalf("approval effect = %+v, want shell_exec command summary", approval.Effect)
	}
}

func TestApprovalRequiredValidatesControlledWorkspaceBeforeApproval(t *testing.T) {
	workspace := testTempDir(t)
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": "unsupported-write-command target.txt",
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{ToolCallID: "call_approval_preflight", Name: "shell_exec", Arguments: json.RawMessage(arguments)}},
		final: "controlled workspace preflight feedback received",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
			ApprovalPolicy: ApprovalPolicyOnRequest,
			SandboxProfile: SandboxProfileControlledWorkspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-controlled-preflight",
		InputItems: []InputItem{{Type: "text", Text: "try unsupported write"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	projection, err := k.Session("approval-controlled-preflight")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Approvals) != 0 {
		t.Fatalf("approvals = %+v, want no approval before controlled workspace preflight passes", projection.Approvals)
	}
	if len(projection.SandboxReadiness) != 0 {
		t.Fatalf("sandbox readiness = %+v, want no ready evidence for unsupported command", projection.SandboxReadiness)
	}
	if len(projection.Operations) != 1 || projection.Operations[0].BlockedReason != "unsupported_default_command" {
		t.Fatalf("operations = %+v, want unsupported_default_command pre-approval block", projection.Operations)
	}
}

func TestApprovalApproveExecutesFrozenEffectAfterApprovedFact(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "approval-approved-runs.txt")
	k, _ := newApprovalRequiredTurnKernel(t, workspace, target)
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-owner-approve",
		InputItems: []InputItem{{Type: "text", Text: "write after approval"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	approval := requireSinglePendingApproval(t, k, "approval-owner-approve")

	decided, err := k.DecideApproval(context.Background(), ApprovalDecisionRequest{
		ApprovalID:          approval.ApprovalID,
		Decision:            ApprovalDecisionApproved,
		DecisionAuthority:   "operator:test",
		DecisionReason:      "approve exact frozen write",
		DecisionEvidenceRef: "approval:approve-frozen-effect",
	})
	if err != nil {
		t.Fatalf("DecideApproval approve returned error: %v", err)
	}
	if decided.Status != ApprovalStatusApproved {
		t.Fatalf("approved status = %q, want approved", decided.Status)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("approved command did not create %q: %v", target, err)
	}
	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	approvedIndex := eventIndex(events, "approval.approved")
	runningIndex := eventIndexAfter(events, "operation.running", approvedIndex)
	if approvedIndex < 0 || runningIndex < 0 || approvedIndex > runningIndex {
		t.Fatalf("event order missing approval.approved before execution: approved=%d running=%d", approvedIndex, runningIndex)
	}
	projection, err := k.Session("approval-owner-approve")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if projection.Approvals[0].Status != ApprovalStatusApproved {
		t.Fatalf("projection approvals = %+v, want approved", projection.Approvals)
	}
	if lastOperationStatus(projection.Operations) != "completed" {
		t.Fatalf("operations = %+v, want completed approved execution", projection.Operations)
	}

	reuse, err := k.toolGateway().ExecShell(context.Background(), ShellExecRequest{
		SessionID:      "approval-owner-approve",
		CWD:            approval.Effect.CWD,
		Command:        approval.Effect.CommandPreview,
		IdempotencyKey: "manual-reuse",
		approvedByID:   approval.ApprovalID,
	}, approval.TurnID)
	if err != nil {
		t.Fatalf("reused approval ExecShell returned error: %v", err)
	}
	if reuse.Status != "blocked" || reuse.BlockedReason != "approval_effect_mismatch" {
		t.Fatalf("reused approval operation = %+v, want approval_effect_mismatch block", reuse)
	}
}

func TestApprovalApprovedCrashWindowReplayExecutesFrozenEffectOnce(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "approval-approved-crash-window-runs.txt")
	k, _ := newApprovalRequiredTurnKernel(t, workspace, target)
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-approved-crash-window",
		InputItems: []InputItem{{Type: "text", Text: "write after approval"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	approval := requireSinglePendingApproval(t, k, "approval-approved-crash-window")
	decision := ApprovalDecisionRequest{
		ApprovalID:          approval.ApprovalID,
		Decision:            ApprovalDecisionApproved,
		DecisionAuthority:   "operator:test",
		DecisionReason:      "approve before crash window replay",
		DecisionEvidenceRef: "approval:approved-crash-window",
	}
	approved := decideApprovalProjection(approval, decision, ApprovalStatusApproved, "", k.clock())
	if err := k.appendApprovalEvent("approval.approved", approved); err != nil {
		t.Fatalf("append approval.approved returned error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("seeded approval.approved unexpectedly created %q; stat err=%v", target, err)
	}

	decided, err := k.DecideApproval(context.Background(), decision)
	if err != nil {
		t.Fatalf("DecideApproval replay returned error: %v", err)
	}
	if decided.Status != ApprovalStatusApproved {
		t.Fatalf("replayed approval status = %q, want approved", decided.Status)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("crash-window replay did not create %q: %v", target, err)
	}
	if _, err := k.DecideApproval(context.Background(), decision); err != nil {
		t.Fatalf("DecideApproval second replay returned error: %v", err)
	}
	projection, err := k.Session("approval-approved-crash-window")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	completed := 0
	for _, operation := range projection.Operations {
		if operation.Status == "completed" {
			completed++
		}
	}
	if completed != 1 {
		t.Fatalf("completed operations = %d in %+v, want exactly one recovered approved effect", completed, projection.Operations)
	}
}

func TestApprovalApprovedCrashWindowReplayFailsClosedOnPolicyMismatch(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "approval-approved-crash-window-policy-mismatch.txt")
	k, _ := newApprovalRequiredTurnKernel(t, workspace, target)
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-approved-crash-window-policy-mismatch",
		InputItems: []InputItem{{Type: "text", Text: "write after approval"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	approval := requireSinglePendingApproval(t, k, "approval-approved-crash-window-policy-mismatch")
	decision := ApprovalDecisionRequest{
		ApprovalID:          approval.ApprovalID,
		Decision:            ApprovalDecisionApproved,
		DecisionAuthority:   "operator:test",
		DecisionReason:      "approve before policy drift",
		DecisionEvidenceRef: "approval:approved-crash-window-policy-mismatch",
	}
	approved := decideApprovalProjection(approval, decision, ApprovalStatusApproved, "", k.clock())
	if err := k.appendApprovalEvent("approval.approved", approved); err != nil {
		t.Fatalf("append approval.approved returned error: %v", err)
	}
	k.toolPolicy = normalizedToolPolicy(ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
		ApprovalPolicy: ApprovalPolicyOnRequest,
	})

	if _, err := k.DecideApproval(context.Background(), decision); err != nil {
		t.Fatalf("DecideApproval replay returned error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("policy-mismatched replay created %q; stat err=%v", target, err)
	}
	projection, err := k.Session("approval-approved-crash-window-policy-mismatch")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if lastOperationBlockedReason(projection.Operations) != "policy_snapshot_mismatch" {
		t.Fatalf("operations = %+v, want policy_snapshot_mismatch block", projection.Operations)
	}
}

func TestApprovalDenyRecordsTerminalBlockedOutcomeWithoutEffect(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "approval-denied-should-not-run.txt")
	k, _ := newApprovalRequiredTurnKernel(t, workspace, target)
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-owner-deny",
		InputItems: []InputItem{{Type: "text", Text: "write after approval"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	approval := requireSinglePendingApproval(t, k, "approval-owner-deny")

	denied, err := k.DecideApproval(context.Background(), ApprovalDecisionRequest{
		ApprovalID:          approval.ApprovalID,
		Decision:            ApprovalDecisionDenied,
		DecisionAuthority:   "operator:test",
		DecisionReason:      "deny exact frozen write",
		DecisionEvidenceRef: "approval:deny-frozen-effect",
	})
	if err != nil {
		t.Fatalf("DecideApproval deny returned error: %v", err)
	}
	if denied.Status != ApprovalStatusDenied {
		t.Fatalf("denied status = %q, want denied", denied.Status)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("denied command created %q; stat err=%v", target, err)
	}
	projection, err := k.Session("approval-owner-deny")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if projection.Approvals[0].Status != ApprovalStatusDenied || lastOperationBlockedReason(projection.Operations) != "approval_denied" {
		t.Fatalf("projection = %+v, want denied approval and terminal blocked operation", projection)
	}
}

func TestApprovalDecisionRejectsUnknownExpiredMismatchedAndStaleRequests(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "approval-invalid-should-not-run.txt")
	k, _ := newApprovalRequiredTurnKernel(t, workspace, target)
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-owner-invalid",
		InputItems: []InputItem{{Type: "text", Text: "write after approval"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	approval := requireSinglePendingApproval(t, k, "approval-owner-invalid")

	if _, err := k.DecideApproval(context.Background(), ApprovalDecisionRequest{
		ApprovalID:          "approval_missing",
		Decision:            ApprovalDecisionApproved,
		DecisionAuthority:   "operator:test",
		DecisionReason:      "must fail",
		DecisionEvidenceRef: "approval:missing",
	}); err == nil {
		t.Fatal("DecideApproval returned nil for unknown approval")
	}

	k.toolPolicy = normalizedToolPolicy(ToolPolicy{
		PermissionMode: PermissionModeDefault,
		WorkspaceRoot:  workspace,
		ApprovalPolicy: ApprovalPolicyOnRequest,
	})
	if _, err := k.DecideApproval(context.Background(), ApprovalDecisionRequest{
		ApprovalID:          approval.ApprovalID,
		Decision:            ApprovalDecisionApproved,
		DecisionAuthority:   "operator:test",
		DecisionReason:      "changed policy must fail",
		DecisionEvidenceRef: "approval:mismatch",
	}); err == nil {
		t.Fatal("DecideApproval returned nil for policy snapshot mismatch")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("mismatched approval created %q; stat err=%v", target, err)
	}

	expiringWorkspace := testTempDir(t)
	expiringTarget := filepath.Join(expiringWorkspace, "approval-expired-should-not-run.txt")
	now := time.Date(2026, 6, 25, 8, 0, 0, 0, time.UTC)
	expiringKernel, _ := newApprovalRequiredTurnKernelWithClock(t, expiringWorkspace, expiringTarget, func() time.Time { return now })
	if _, err := expiringKernel.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-owner-expired",
		InputItems: []InputItem{{Type: "text", Text: "write after approval"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	expired := requireSinglePendingApproval(t, expiringKernel, "approval-owner-expired")
	now = now.Add(defaultApprovalTTL + time.Second)
	if _, err := expiringKernel.DecideApproval(context.Background(), ApprovalDecisionRequest{
		ApprovalID:          expired.ApprovalID,
		Decision:            ApprovalDecisionApproved,
		DecisionAuthority:   "operator:test",
		DecisionReason:      "expired must fail",
		DecisionEvidenceRef: "approval:expired",
	}); err == nil {
		t.Fatal("DecideApproval returned nil for expired approval")
	}
	expiredProjection, err := expiringKernel.Session("approval-owner-expired")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if expiredProjection.Approvals[0].Status != ApprovalStatusExpired {
		t.Fatalf("approval status = %+v, want expired", expiredProjection.Approvals)
	}
	if _, err := os.Stat(expiringTarget); !os.IsNotExist(err) {
		t.Fatalf("expired approval created %q; stat err=%v", expiringTarget, err)
	}

	denyWorkspace := testTempDir(t)
	denyTarget := filepath.Join(denyWorkspace, "approval-stale-should-not-run.txt")
	staleKernel, _ := newApprovalRequiredTurnKernel(t, denyWorkspace, denyTarget)
	if _, err := staleKernel.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-owner-stale",
		InputItems: []InputItem{{Type: "text", Text: "write after approval"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	stale := requireSinglePendingApproval(t, staleKernel, "approval-owner-stale")
	if _, err := staleKernel.DecideApproval(context.Background(), ApprovalDecisionRequest{
		ApprovalID:          stale.ApprovalID,
		Decision:            ApprovalDecisionDenied,
		DecisionAuthority:   "operator:test",
		DecisionReason:      "deny once",
		DecisionEvidenceRef: "approval:stale-deny",
	}); err != nil {
		t.Fatalf("DecideApproval deny returned error: %v", err)
	}
	if _, err := staleKernel.DecideApproval(context.Background(), ApprovalDecisionRequest{
		ApprovalID:          stale.ApprovalID,
		Decision:            ApprovalDecisionApproved,
		DecisionAuthority:   "operator:test",
		DecisionReason:      "stale approve must fail",
		DecisionEvidenceRef: "approval:stale-approve",
	}); err == nil {
		t.Fatal("DecideApproval returned nil for stale terminal approval")
	}
	if _, err := os.Stat(denyTarget); !os.IsNotExist(err) {
		t.Fatalf("stale approval created %q; stat err=%v", denyTarget, err)
	}
}

func TestUnavailableSandboxRecordsReadinessAndCannotBeApprovedIntoHost(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "sandbox-unavailable-should-not-run.txt")
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand(filepath.Base(target), "blocked"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{ToolCallID: "call_unavailable_sandbox_approval", Name: "shell_exec", Arguments: json.RawMessage(arguments)}},
		final: "sandbox unavailable feedback received",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
			SandboxProfile: SandboxProfileOSWorkspace,
			ApprovalPolicy: ApprovalPolicyOnRequest,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "sandbox-readiness-before-approval",
		InputItems: []InputItem{{Type: "text", Text: "try unavailable sandbox"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("unavailable sandbox command created %q; stat err=%v", target, err)
	}
	projection, err := k.Session("sandbox-readiness-before-approval")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.SandboxReadiness) != 1 || projection.SandboxReadiness[0].Status != SandboxReadinessUnavailable {
		t.Fatalf("sandbox readiness = %+v, want one unavailable evidence", projection.SandboxReadiness)
	}
	if len(projection.Approvals) != 0 {
		t.Fatalf("approvals = %+v, want no approval when sandbox is unavailable", projection.Approvals)
	}
}

func TestApprovalHTTPSurfaceListsPendingAndSubmitsDecisionCommand(t *testing.T) {
	workspace := testTempDir(t)
	target := filepath.Join(workspace, "approval-http-runs.txt")
	k, _ := newApprovalRequiredTurnKernel(t, workspace, target)
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "approval-http-surface",
		InputItems: []InputItem{{Type: "text", Text: "write after approval"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	t.Cleanup(server.Close)

	listResp, err := getWithAuth(server.URL + "/approvals?status=pending")
	if err != nil {
		t.Fatalf("GET /approvals returned error: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /approvals status = %d, want 200", listResp.StatusCode)
	}
	var list ApprovalListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode approval list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("approval list = %+v, want one pending item", list.Items)
	}
	approvalID := list.Items[0].ApprovalID

	badResp, err := postJSONWithAuth(server.URL+"/approvals/"+approvalID+"/decision", []byte(`{"decision":"approved","decision_authority":"operator:test","decision_reason":"approve","decision_evidence_ref":"approval:http-bad","permission_mode":"yolo"}`))
	if err != nil {
		t.Fatalf("POST bad decision returned error: %v", err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad decision status = %d, want 400 for unknown control field", badResp.StatusCode)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("bad HTTP decision created %q; stat err=%v", target, err)
	}

	mismatchedResp, err := postJSONWithAuth(server.URL+"/approvals/"+approvalID+"/decision", []byte(`{"approval_id":"approval_different","decision":"approved","decision_authority":"operator:test","decision_reason":"approve","decision_evidence_ref":"approval:http-mismatch"}`))
	if err != nil {
		t.Fatalf("POST mismatched decision returned error: %v", err)
	}
	defer mismatchedResp.Body.Close()
	if mismatchedResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("mismatched decision status = %d, want 400", mismatchedResp.StatusCode)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("mismatched HTTP decision created %q; stat err=%v", target, err)
	}

	approveResp, err := postJSONWithAuth(server.URL+"/approvals/"+approvalID+"/decision", []byte(`{"decision":"approved","decision_authority":"operator:test","decision_reason":"approve exact frozen request","decision_evidence_ref":"approval:http-approve"}`))
	if err != nil {
		t.Fatalf("POST approve decision returned error: %v", err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveResp.StatusCode)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("HTTP-approved command did not create %q: %v", target, err)
	}

	retryResp, err := postJSONWithAuth(server.URL+"/approvals/"+approvalID+"/decision", []byte(`{"decision":"approved","decision_authority":"operator:test","decision_reason":"retry approve exact frozen request","decision_evidence_ref":"approval:http-approve-retry"}`))
	if err != nil {
		t.Fatalf("POST retry approve decision returned error: %v", err)
	}
	defer retryResp.Body.Close()
	if retryResp.StatusCode != http.StatusOK {
		t.Fatalf("retry approve status = %d, want 200 idempotent replay", retryResp.StatusCode)
	}
	projection, err := k.Session("approval-http-surface")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	completed := 0
	for _, operation := range projection.Operations {
		if operation.Status == "completed" {
			completed++
		}
	}
	if completed != 1 {
		t.Fatalf("completed operations = %d in %+v, want one approved effect after retry", completed, projection.Operations)
	}
}

func TestApprovalHTTPListRejectsUnknownStatusFilter(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	t.Cleanup(server.Close)

	resp, err := getWithAuth(server.URL + "/approvals?status=maybe")
	if err != nil {
		t.Fatalf("GET /approvals returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("GET /approvals unknown status = %d, want 400", resp.StatusCode)
	}
}

func newApprovalRequiredTurnKernel(t *testing.T, workspace string, target string) (*Kernel, *toolFeedbackProvider) {
	t.Helper()
	return newApprovalRequiredTurnKernelWithClock(t, workspace, target, func() time.Time {
		return time.Date(2026, 6, 25, 8, 0, 0, 0, time.UTC)
	})
}

func newApprovalRequiredTurnKernelWithClock(t *testing.T, workspace string, target string, clock func() time.Time) (*Kernel, *toolFeedbackProvider) {
	t.Helper()
	arguments, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": writeFileCommand(filepath.Base(target), "approved"),
	})
	if err != nil {
		t.Fatalf("marshal shell args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{ToolCallID: "call_approval_owner", Name: "shell_exec", Arguments: json.RawMessage(arguments)}},
		final: "approval owner feedback received",
	}
	k, err := New(Config{
		LedgerPath:   filepath.Join(testTempDir(t), "events.sqlite"),
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeYolo,
			WorkspaceRoot:  workspace,
			ApprovalPolicy: ApprovalPolicyOnRequest,
		},
		Clock: clock,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return k, provider
}

func requireSinglePendingApproval(t *testing.T, k *Kernel, sessionID string) ApprovalProjection {
	t.Helper()
	projection, err := k.Session(sessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Approvals) != 1 {
		t.Fatalf("approvals = %+v, want one pending approval", projection.Approvals)
	}
	if projection.Approvals[0].Status != ApprovalStatusPending {
		t.Fatalf("approval status = %q, want pending", projection.Approvals[0].Status)
	}
	return projection.Approvals[0]
}

func eventIndex(events []StoredEvent, eventType string) int {
	return eventIndexAfter(events, eventType, -1)
}

func eventIndexAfter(events []StoredEvent, eventType string, after int) int {
	for i := after + 1; i < len(events); i++ {
		if events[i].Type == eventType {
			return i
		}
	}
	return -1
}

func lastOperationStatus(operations []OperationProjection) string {
	if len(operations) == 0 {
		return ""
	}
	return operations[len(operations)-1].Status
}

func lastOperationBlockedReason(operations []OperationProjection) string {
	if len(operations) == 0 {
		return ""
	}
	return operations[len(operations)-1].BlockedReason
}
