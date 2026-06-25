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

func TestUITimelineProjectionMergesToolEventsWithoutAuditFields(t *testing.T) {
	workspace := testTempDir(t)
	args, err := json.Marshal(map[string]string{
		"cwd":     workspace,
		"command": "echo timeline",
	})
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}
	provider := &toolFeedbackProvider{
		calls: []ModelToolCall{{
			ToolCallID: "call_timeline_shell",
			Name:       "shell_exec",
			Arguments:  json.RawMessage(args),
		}},
		final: "timeline final",
	}
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModePlan,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if _, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "timeline-session",
		InputItems: []InputItem{{Type: "text", Text: "show timeline api_key=sk-timeline-secret"}},
	}); err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}

	restarted := newTestKernelWithRuntimeTokenAndPolicy(t, ledgerPath, testRuntimeToken, ToolPolicy{
		PermissionMode: PermissionModePlan,
		WorkspaceRoot:  workspace,
	})
	timeline, err := restarted.UITimeline("timeline-session")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	if timeline.Status != "ok" || len(timeline.Items) != 1 {
		t.Fatalf("timeline = %+v, want one turn item", timeline)
	}
	turn := timeline.Items[0]
	if turn.Kind != "turn" {
		t.Fatalf("turn item = %+v, want turn projection", turn)
	}
	user := requireTimelineChild(t, turn, "user_message")
	if !strings.Contains(user.Text, "show timeline api_key=sk-timeline-secret") {
		t.Fatalf("user item = %+v, want local user content preserved", user)
	}
	processing := requireTimelineChild(t, turn, "processing_group")
	if processing.DefaultOpen || processing.ToolCount != 1 {
		t.Fatalf("processing group = %+v, want settled collapsed group with one tool", processing)
	}
	operation := requireNestedTimelineChild(t, processing, "operation_detail")
	if operation.Tool != "shell_exec" || operation.Status != "permission_denied" {
		t.Fatalf("operation detail = %+v, want merged permission_denied shell tool", operation)
	}
	if !operation.FullOutputAvailable || operation.OutputSource != "error" || !strings.Contains(operation.OutputPreview, "blocked") {
		t.Fatalf("operation output = %+v, want preview metadata", operation)
	}
	assistant := requireTimelineChild(t, turn, "assistant_message")
	if assistant.Text != "timeline final" {
		t.Fatalf("assistant item = %+v, want final answer", assistant)
	}
	timelineJSON, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	for _, forbidden := range []string{
		"tool_call_event_id",
		"provider_tool_call_id",
		"operation_id",
		"for_event_id",
		"tool.call",
		"tool.result",
	} {
		if strings.Contains(string(timelineJSON), forbidden) {
			t.Fatalf("timeline leaked %q: %s", forbidden, string(timelineJSON))
		}
	}
	if !strings.Contains(string(timelineJSON), "sk-timeline-secret") || strings.Contains(string(timelineJSON), "[REDACTED]") {
		t.Fatalf("timeline content fidelity broken: %s", string(timelineJSON))
	}

	server := httptest.NewServer(Handler(restarted))
	defer server.Close()
	resp, err := getWithAuth(server.URL + "/sessions/timeline-session/timeline")
	if err != nil {
		t.Fatalf("GET /sessions/{id}/timeline failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("timeline status = %d, want 200", resp.StatusCode)
	}
	var httpTimeline UITimelineResponse
	if err := json.NewDecoder(resp.Body).Decode(&httpTimeline); err != nil {
		t.Fatalf("decode timeline: %v", err)
	}
	if len(httpTimeline.Items) != len(timeline.Items) {
		t.Fatalf("HTTP timeline items = %+v, want %d items", httpTimeline.Items, len(timeline.Items))
	}
}

func TestObservabilityProjectionsSeparateRawAuditAndProviderContext(t *testing.T) {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	exitCode := 0
	toolResultContent := `{"status":"completed","executed":true,"exit_code":0,"stdout":"ok","stdout_truncated":true,"stdout_original_bytes":120,"stdout_omitted_bytes":80,"output_truncation":"head_tail"}`
	operation := OperationProjection{
		OperationID:         "op_observe",
		SessionID:           "observe-session",
		TurnID:              "turn_observe",
		Tool:                "shell_exec",
		Status:              "completed",
		PermissionMode:      PermissionModeDefault,
		CWD:                 "C:/workspace",
		Command:             "echo api_key=sk-observe-secret",
		ExitCode:            &exitCode,
		Stdout:              "api_key=sk-observe-secret\nok",
		StdoutTruncated:     true,
		StdoutOriginalBytes: 120,
		StdoutOmittedBytes:  80,
		OutputTruncation:    "head_tail",
		StartedAt:           now,
		EndedAt:             now.Add(time.Second),
	}
	k := &Kernel{ledger: newStaticLedger(
		StoredEvent{
			EventID:   "evt_submitted",
			SessionID: "observe-session",
			TurnID:    "turn_observe",
			Type:      "turn.submitted",
			CreatedAt: now,
			Data: EventData{
				InputItems:      []InputItem{{Type: "text", Text: "hello observability"}},
				ModelInputKinds: []string{ModelInputKindUserText},
				ToolManifest: []ToolSpec{{
					Name:            "shell_exec",
					Description:     "Execute a governed command.",
					InputSchema:     map[string]interface{}{"type": "object"},
					SideEffectLevel: "write",
					ExecutionKind:   "sandboxed_process",
				}},
				RuntimeContext: &ContextRuntimeSnapshot{
					Provider: ProviderStatus{Name: "test-provider", Readiness: ReadinessReady},
					Permission: PermissionInspection{
						PermissionMode:  PermissionModeDefault,
						AuthorityPolicy: AuthorityPolicyWorkspaceWrite,
						SandboxProfile:  SandboxProfileControlledWorkspace,
						ApprovalPolicy:  ApprovalPolicyNever,
					},
				},
			},
		},
		StoredEvent{
			EventID:   "evt_tool_call",
			SessionID: "observe-session",
			TurnID:    "turn_observe",
			Type:      "tool.call",
			CreatedAt: now.Add(time.Second),
			Data: EventData{ToolCall: &ToolCallProjection{
				ToolCallEventID:    "evt_tool_call",
				ProviderToolCallID: "call_provider_visible",
				Tool:               "shell_exec",
				Arguments:          `{"command":"echo api_key=sk-observe-secret"}`,
			}},
		},
		StoredEvent{
			EventID:     "evt_operation_completed",
			SessionID:   "observe-session",
			TurnID:      "turn_observe",
			OperationID: "op_observe",
			Type:        "operation.completed",
			CreatedAt:   now.Add(2 * time.Second),
			Data:        EventData{Operation: &operation},
		},
		StoredEvent{
			EventID:   "evt_tool_result",
			SessionID: "observe-session",
			TurnID:    "turn_observe",
			Type:      "tool.result",
			CreatedAt: now.Add(3 * time.Second),
			Data: EventData{ToolResult: &ToolResultProjection{
				ToolCallEventID:    "evt_tool_call",
				ProviderToolCallID: "call_provider_visible",
				Tool:               "shell_exec",
				ForEventID:         "evt_tool_call",
				Status:             "completed",
				Content:            toolResultContent,
			}},
		},
		StoredEvent{
			EventID:   "evt_final",
			SessionID: "observe-session",
			TurnID:    "turn_observe",
			Type:      "model.final",
			CreatedAt: now.Add(4 * time.Second),
			Data: EventData{Final: &FinalMessage{
				Text:  "done api_key=sk-observe-secret",
				Model: "test-model",
				Usage: &TokenUsage{InputTokens: 3, OutputTokens: 2, TotalTokens: 5},
			}},
		},
	)}

	timeline, err := k.UITimeline("observe-session")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	timelineJSON, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	for _, forbidden := range []string{"tool_call_event_id", "operation_id", "for_event_id", "tool.call"} {
		if strings.Contains(string(timelineJSON), forbidden) {
			t.Fatalf("timeline leaked %q: %s", forbidden, string(timelineJSON))
		}
	}
	if !strings.Contains(string(timelineJSON), "sk-observe-secret") || strings.Contains(string(timelineJSON), "[REDACTED]") {
		t.Fatalf("timeline should preserve local assistant/tool content without lossy redaction: %s", string(timelineJSON))
	}

	rawEvents, err := k.TurnEvents("turn_observe")
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	rawJSON, err := json.Marshal(rawEvents)
	if err != nil {
		t.Fatalf("marshal raw events: %v", err)
	}
	if !strings.Contains(string(rawJSON), "tool_call_event_id") || !strings.Contains(string(rawJSON), "operation_id") {
		t.Fatalf("raw event inspection = %s, want typed ids for debugging", string(rawJSON))
	}
	if !strings.Contains(string(rawJSON), "sk-observe-secret") || strings.Contains(string(rawJSON), "[REDACTED]") {
		t.Fatalf("raw event inspection should preserve local event content without lossy redaction: %s", string(rawJSON))
	}

	audit, err := k.AuditReplay("turn_observe")
	if err != nil {
		t.Fatalf("AuditReplay returned error: %v", err)
	}
	auditJSON, err := json.Marshal(audit)
	if err != nil {
		t.Fatalf("marshal audit: %v", err)
	}
	for _, want := range []string{"operation.completed", "stdout_original_bytes", "stdout_omitted_bytes", "head_tail"} {
		if !strings.Contains(string(auditJSON), want) {
			t.Fatalf("audit replay = %s, want %q", string(auditJSON), want)
		}
	}
	if !strings.Contains(string(auditJSON), "sk-observe-secret") || strings.Contains(string(auditJSON), "[REDACTED]") {
		t.Fatalf("audit replay should preserve local event content without lossy redaction: %s", string(auditJSON))
	}

	providerContext, err := k.ProviderContextProjection("turn_observe")
	if err != nil {
		t.Fatalf("ProviderContextProjection returned error: %v", err)
	}
	contextJSON, err := json.Marshal(providerContext.ModelRequest())
	if err != nil {
		t.Fatalf("marshal provider context: %v", err)
	}
	for _, forbidden := range []string{"tool_call_event_id", "operation_id", "permission_mode", "for_event_id", "op_observe"} {
		if strings.Contains(string(contextJSON), forbidden) {
			t.Fatalf("provider context leaked %q: %s", forbidden, string(contextJSON))
		}
	}
	if !strings.Contains(string(contextJSON), "call_provider_visible") || !strings.Contains(string(contextJSON), "hello observability") {
		t.Fatalf("provider context = %s, want provider-visible input and tool correlation", string(contextJSON))
	}

	server := httptest.NewServer(Handler(&Kernel{ledger: k.ledger, runtimeToken: testRuntimeToken}))
	defer server.Close()
	resp, err := getWithAuth(server.URL + "/turns/turn_observe/audit")
	if err != nil {
		t.Fatalf("GET /turns/{id}/audit failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit status = %d, want 200", resp.StatusCode)
	}
}

func TestSessionProjectionPreservesUserOwnedLocalContent(t *testing.T) {
	now := time.Date(2026, 6, 22, 7, 0, 0, 0, time.UTC)
	secret := "sk-proj-sessionsecret123456"
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abcdefghijklmnopqrstuvwx0123456789.ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	exitCode := 0
	k := &Kernel{ledger: newStaticLedger(
		StoredEvent{
			EventID:   "evt_session_submitted",
			SessionID: "session-content",
			TurnID:    "turn_session_content",
			Type:      "turn.submitted",
			CreatedAt: now,
			Data: EventData{
				InputItems:       []InputItem{{Type: "text", Text: "user text " + secret}},
				RecalledMemories: []MemoryRecall{{CandidateID: "mem_session", Text: "memory " + jwt, Source: "turn:source"}},
			},
		},
		StoredEvent{
			EventID:   "evt_session_final",
			SessionID: "session-content",
			TurnID:    "turn_session_content",
			Type:      "model.final",
			CreatedAt: now.Add(time.Second),
			Data: EventData{Final: &FinalMessage{
				Text:  "final " + secret,
				Model: "test-model",
			}},
		},
		StoredEvent{
			EventID:     "evt_session_operation",
			SessionID:   "session-content",
			TurnID:      "turn_session_content",
			OperationID: "op_session_content",
			Type:        "operation.completed",
			CreatedAt:   now.Add(2 * time.Second),
			Data: EventData{Operation: &OperationProjection{
				OperationID:    "op_session_content",
				SessionID:      "session-content",
				TurnID:         "turn_session_content",
				Tool:           "shell_exec",
				Status:         "completed",
				PermissionMode: PermissionModeYolo,
				CWD:            "C:/workspace",
				Command:        "echo " + secret,
				ExitCode:       &exitCode,
				Stdout:         "stdout " + jwt,
				StartedAt:      now,
				EndedAt:        now.Add(2 * time.Second),
			}},
		},
		StoredEvent{
			EventID:   "evt_session_work",
			SessionID: "session-content",
			WorkID:    "work_session_content",
			Type:      "work.submitted",
			CreatedAt: now.Add(3 * time.Second),
			Data: EventData{Work: &WorkProjection{
				WorkID:    "work_session_content",
				SessionID: "session-content",
				Title:     "work " + secret,
				SourceRef: "turn:source",
				Status:    "open",
				CreatedAt: now.Add(3 * time.Second),
			}},
		},
		StoredEvent{
			EventID:     "evt_session_memory",
			SessionID:   "session-content",
			CandidateID: "mem_session",
			Type:        "memory.candidate.created",
			CreatedAt:   now.Add(4 * time.Second),
			Data: EventData{MemoryCandidate: &MemoryCandidateProjection{
				CandidateID: "mem_session",
				SessionID:   "session-content",
				Text:        "candidate " + jwt,
				SourceRef:   "turn:source",
				Status:      MemoryCandidatePending,
				CreatedAt:   now.Add(4 * time.Second),
			}},
		},
	)}

	projection, err := k.Session("session-content")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	projectionJSON, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal session projection: %v", err)
	}
	for _, want := range []string{secret, jwt} {
		if !strings.Contains(string(projectionJSON), want) {
			t.Fatalf("session projection lost local content %q: %s", want, string(projectionJSON))
		}
	}
	if strings.Contains(string(projectionJSON), "[REDACTED]") {
		t.Fatalf("session projection should not use lossy redaction for local content: %s", string(projectionJSON))
	}
}

func TestContextInspectionProjectionPersistsProviderVisibleSnapshot(t *testing.T) {
	root := testTempDir(t)
	skillPath := writeSkillForTest(t, root, "lark-im", "lark-im", "Send chat messages through installed CLI", "FULL SKILL BODY MUST NOT BE PROJECTED")
	provider := &capturingProvider{text: "context inspected"}
	ledgerPath := filepath.Join(testTempDir(t), "events.jsonl")
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
		SkillRoots:   []string{root},
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  testTempDir(t),
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	candidate, err := k.CreateMemoryCandidate(MemoryCandidateRequest{
		SessionID: "context-inspection-source",
		Text:      "prefer concise replies",
		SourceRef: "turn:context-inspection-source",
	})
	if err != nil {
		t.Fatalf("CreateMemoryCandidate returned error: %v", err)
	}
	if _, err := k.ApproveMemoryCandidate(candidate.CandidateID, testApprovalRequest("approval:context-inspection-source")); err != nil {
		t.Fatalf("ApproveMemoryCandidate returned error: %v", err)
	}
	turn, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "context-inspection-consumer",
		InputItems: []InputItem{{Type: "text", Text: "Do you remember prefer concise replies? Authorization: Bearer tokentest123456"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	if got := strings.Join(provider.InputKinds(), ","); got != strings.Join([]string{ModelInputKindSkillIndexContext, ModelInputKindApprovedMemoryContext, ModelInputKindUserText}, ",") {
		t.Fatalf("provider input kinds = %v", provider.InputKinds())
	}

	restarted, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     FakeProvider{},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModePlan,
		},
	})
	if err != nil {
		t.Fatalf("restart New returned error: %v", err)
	}
	inspection, err := restarted.ContextInspection(turn.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	if inspection.Status != "ok" || inspection.SessionID != "context-inspection-consumer" {
		t.Fatalf("inspection = %+v, want ok for submitted turn", inspection)
	}
	if len(inspection.InputItems) != 1 || !strings.Contains(inspection.InputItems[0].Text, "Authorization: Bearer tokentest123456") {
		t.Fatalf("input items = %+v, want local user input preserved", inspection.InputItems)
	}
	if got := strings.Join(inspection.ModelInputKinds, ","); got != strings.Join([]string{ModelInputKindSkillIndexContext, ModelInputKindApprovedMemoryContext, ModelInputKindUserText}, ",") {
		t.Fatalf("model input kinds = %v", inspection.ModelInputKinds)
	}
	toolNames := toolSpecNames(inspection.ToolManifest)
	for _, want := range []string{"shell_exec", "job_status", "job_cancel"} {
		if !containsString(toolNames, want) {
			t.Fatalf("tool manifest = %+v, want %s", inspection.ToolManifest, want)
		}
	}
	if containsString(toolNames, "job_terminate") {
		t.Fatalf("tool manifest = %+v, must not expose process-level terminate tool", inspection.ToolManifest)
	}
	if len(inspection.SkillCatalog) != 1 || inspection.SkillCatalog[0].Name != "lark-im" {
		t.Fatalf("skill catalog = %+v, want persisted lark-im summary", inspection.SkillCatalog)
	}
	if len(inspection.RecalledMemories) != 1 || inspection.RecalledMemories[0].Source != "turn:context-inspection-source" {
		t.Fatalf("recalled memories = %+v, want source refs", inspection.RecalledMemories)
	}
	if inspection.Runtime == nil || inspection.Runtime.Provider.Name != "capturing" {
		t.Fatalf("runtime snapshot = %+v, want original provider", inspection.Runtime)
	}
	if inspection.Runtime.Permission.PermissionMode != PermissionModeDefault ||
		inspection.Runtime.Permission.AuthorityPolicy != AuthorityPolicyWorkspaceWrite ||
		inspection.Runtime.Permission.SandboxProfile != SandboxProfileControlledWorkspace ||
		inspection.Runtime.Permission.ApprovalPolicy != ApprovalPolicyNever {
		t.Fatalf("runtime permission = %+v, want resolved default policy profile", inspection.Runtime.Permission)
	}
	inspectionJSON, err := json.Marshal(inspection)
	if err != nil {
		t.Fatalf("marshal inspection: %v", err)
	}
	for _, forbidden := range []string{skillPath, "FULL SKILL BODY"} {
		if strings.Contains(string(inspectionJSON), forbidden) {
			t.Fatalf("context inspection leaked %q: %s", forbidden, string(inspectionJSON))
		}
	}
	if !strings.Contains(string(inspectionJSON), "tokentest123456") || strings.Contains(string(inspectionJSON), "[REDACTED]") {
		t.Fatalf("context inspection should preserve local user content without lossy redaction: %s", string(inspectionJSON))
	}

	server := httptest.NewServer(Handler(restarted))
	defer server.Close()
	resp, err := getWithAuth(server.URL + "/turns/" + turn.TurnID + "/context")
	if err != nil {
		t.Fatalf("GET /turns/{id}/context failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("context status = %d, want 200", resp.StatusCode)
	}
	var httpInspection ContextInspectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&httpInspection); err != nil {
		t.Fatalf("decode context inspection: %v", err)
	}
	if httpInspection.Status != "ok" || httpInspection.Runtime == nil || httpInspection.Runtime.Permission.PermissionMode != PermissionModeDefault {
		t.Fatalf("HTTP inspection = %+v, want persisted snapshot", httpInspection)
	}
}

func TestInspectionRedactsUnsafeProviderToolCallID(t *testing.T) {
	workspace := testTempDir(t)
	providerCallID := `C:\secrets\sk-providersecret123`
	k, err := New(Config{
		LedgerPath: filepath.Join(testTempDir(t), "events.jsonl"),
		Provider: &toolFeedbackProvider{
			calls: []ModelToolCall{{
				ToolCallID: providerCallID,
				Name:       "shell_exec",
				Arguments:  json.RawMessage(`{"command":"` + echoCommand("hello") + `"}`),
			}},
			final: "unsafe provider id stayed out of inspection",
		},
		RuntimeToken: testRuntimeToken,
		ToolPolicy: ToolPolicy{
			PermissionMode: PermissionModeDefault,
			WorkspaceRoot:  workspace,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID:  "provider-id-redaction",
		InputItems: []InputItem{{Type: "text", Text: "try unsafe provider id"}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}
	providerContext, err := k.ProviderContextProjection(resp.TurnID)
	if err != nil {
		t.Fatalf("ProviderContextProjection returned error: %v", err)
	}
	modelRequest := providerContext.ModelRequest()
	if len(modelRequest.ToolRounds) != 1 || len(modelRequest.ToolRounds[0].Calls) != 1 || len(modelRequest.ToolRounds[0].Results) != 1 {
		t.Fatalf("provider context tool rounds = %+v, want one call/result", modelRequest.ToolRounds)
	}
	if modelRequest.ToolRounds[0].Calls[0].ToolCallID != providerCallID || modelRequest.ToolRounds[0].Results[0].ToolCallID != providerCallID {
		t.Fatalf("provider context tool ids = %+v / %+v, want raw provider correlation id", modelRequest.ToolRounds[0].Calls[0], modelRequest.ToolRounds[0].Results[0])
	}

	session, err := k.Session("provider-id-redaction")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	events, err := k.TurnEvents(resp.TurnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	audit, err := k.AuditReplay(resp.TurnID)
	if err != nil {
		t.Fatalf("AuditReplay returned error: %v", err)
	}
	for _, inspected := range []struct {
		name       string
		payload    interface{}
		wantMarker bool
	}{
		{name: "session", payload: session, wantMarker: true},
		{name: "turn events", payload: events, wantMarker: true},
		{name: "audit", payload: audit},
	} {
		encoded, err := json.Marshal(inspected.payload)
		if err != nil {
			t.Fatalf("marshal %s: %v", inspected.name, err)
		}
		if strings.Contains(string(encoded), providerCallID) || strings.Contains(string(encoded), "sk-providersecret123") {
			t.Fatalf("%s leaked provider tool call id: %s", inspected.name, string(encoded))
		}
		if inspected.wantMarker && !strings.Contains(string(encoded), "provider_tool_call_id_unavailable") {
			t.Fatalf("%s = %s, want redacted provider tool call id marker", inspected.name, string(encoded))
		}
	}
}
