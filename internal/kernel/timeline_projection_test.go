package kernel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUITimelineRunningTurnProjectsOpenProcessingGroup(t *testing.T) {
	startedAt := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	k := &Kernel{
		ledger: newStaticLedger(
			StoredEvent{
				EventID:   "evt_running_submitted",
				SessionID: "timeline-running-session",
				TurnID:    "turn_running",
				Type:      "turn.submitted",
				CreatedAt: startedAt,
				Data: EventData{InputItems: []InputItem{{
					Type: "text",
					Text: "run something",
				}}},
			},
			StoredEvent{
				EventID:   "evt_running_tool_call",
				SessionID: "timeline-running-session",
				TurnID:    "turn_running",
				Type:      "tool.call",
				CreatedAt: startedAt.Add(time.Second),
				Data: EventData{ToolCall: &ToolCallProjection{
					ToolCallEventID: "evt_running_tool_call",
					Tool:            "shell_exec",
					Arguments:       `{"command":"sleep"}`,
				}},
			},
		),
		clock: func() time.Time {
			return startedAt.Add(45 * time.Second)
		},
	}

	timeline, err := k.UITimeline("timeline-running-session")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	turn := requireSingleTimelineTurn(t, timeline, "turn_running")
	processing := requireTimelineChild(t, turn, "processing_group")
	if processing.Phase != RuntimePhaseRunning || !processing.DefaultOpen {
		t.Fatalf("processing group = %+v, want running and default open", processing)
	}
	if processing.Text != "正在处理 45s" {
		t.Fatalf("processing text = %q, want computed live elapsed", processing.Text)
	}
	if child := timelineChild(turn, "assistant_message"); child != nil {
		t.Fatalf("assistant child = %+v, want none for running turn", *child)
	}
}

func TestUITimelineSettledTurnCollapsesProcessingGroupWithFixedDuration(t *testing.T) {
	startedAt := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	toolResultContent := `{"status":"failed","executed":true,"exit_code":2,"stderr":"missing argument","stdout_truncated":false,"stderr_truncated":false}`
	k := &Kernel{
		ledger: newStaticLedger(
			StoredEvent{
				EventID:   "evt_settled_submitted",
				SessionID: "timeline-settled-session",
				TurnID:    "turn_settled",
				Type:      "turn.submitted",
				CreatedAt: startedAt,
				Data: EventData{InputItems: []InputItem{{
					Type: "text",
					Text: "run bad command api_key=sk-timeline-settled",
				}}},
			},
			StoredEvent{
				EventID:   "evt_settled_tool_call",
				SessionID: "timeline-settled-session",
				TurnID:    "turn_settled",
				Type:      "tool.call",
				CreatedAt: startedAt.Add(2 * time.Second),
				Data: EventData{ToolCall: &ToolCallProjection{
					ToolCallEventID: "evt_settled_tool_call",
					Tool:            "shell_exec",
					Arguments:       `{"command":"bad --missing"}`,
				}},
			},
			StoredEvent{
				EventID:   "evt_settled_tool_result",
				SessionID: "timeline-settled-session",
				TurnID:    "turn_settled",
				Type:      "tool.result",
				CreatedAt: startedAt.Add(3 * time.Second),
				Data: EventData{ToolResult: &ToolResultProjection{
					ToolCallEventID: "evt_settled_tool_call",
					Tool:            "shell_exec",
					ForEventID:      "evt_settled_tool_call",
					Status:          "failed",
					Content:         toolResultContent,
				}},
			},
			StoredEvent{
				EventID:   "evt_settled_final",
				SessionID: "timeline-settled-session",
				TurnID:    "turn_settled",
				Type:      "model.final",
				CreatedAt: startedAt.Add(65 * time.Second),
				Data: EventData{Final: &FinalMessage{
					Text: "final answer",
				}},
			},
		),
		clock: func() time.Time {
			return startedAt.Add(10 * time.Minute)
		},
	}

	timeline, err := k.UITimeline("timeline-settled-session")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	turn := requireSingleTimelineTurn(t, timeline, "turn_settled")
	processing := requireTimelineChild(t, turn, "processing_group")
	if processing.Phase != RuntimePhaseEnded || processing.TerminalOutcome != TerminalOutcomeSucceeded || processing.DefaultOpen {
		t.Fatalf("processing group = %+v, want settled and collapsed", processing)
	}
	if processing.Text != "已处理 1m 5s" {
		t.Fatalf("processing text = %q, want fixed terminal duration", processing.Text)
	}
	if processing.ToolCount != 1 {
		t.Fatalf("processing tool count = %d, want 1", processing.ToolCount)
	}
	if timelineChild(turn, "tool") != nil {
		t.Fatalf("turn children = %+v, want no ordinary tool row", turn.Children)
	}
	operation := requireNestedTimelineChild(t, processing, "operation_detail")
	if operation.Phase != RuntimePhaseEnded || operation.TerminalOutcome != TerminalOutcomeFailed || operation.OutputSource != "stderr" || !strings.Contains(operation.OutputPreview, "missing argument") {
		t.Fatalf("operation detail = %+v, want failed command stderr in detail", operation)
	}
	assistant := requireTimelineChild(t, turn, "assistant_message")
	if assistant.Text != "final answer" {
		t.Fatalf("assistant = %+v, want final answer", assistant)
	}
	timelineJSON, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	for _, forbidden := range []string{"tool.call", "tool.result", "for_event_id", "tool_call_event_id", "operation_id"} {
		if strings.Contains(string(timelineJSON), forbidden) {
			t.Fatalf("timeline leaked %q: %s", forbidden, string(timelineJSON))
		}
	}
	if !strings.Contains(string(timelineJSON), "sk-timeline-settled") || strings.Contains(string(timelineJSON), "[REDACTED]") {
		t.Fatalf("timeline should preserve local user content without lossy redaction: %s", string(timelineJSON))
	}
}

func TestUITimelineJobTerminalDoesNotSettleTurnBeforeAssistantFinal(t *testing.T) {
	startedAt := time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC)
	sessionID := "timeline-job-terminal-session"
	turnID := "turn_job_terminal"
	job := JobProjection{
		JobID:           "job_download",
		SessionID:       sessionID,
		TurnID:          turnID,
		Tool:            "shell_exec",
		Status:          "running",
		Command:         "download --token sk-job-secret",
		Receipt:         "managed job accepted",
		StartedAt:       startedAt.Add(2 * time.Second),
		ToolCallEventID: "evt_job_tool_call",
	}
	completed := job
	completed.Status = "completed"
	completed.Stdout = "download complete api_key=sk-job-secret"
	completed.CompletedAt = startedAt.Add(8 * time.Second)
	k := &Kernel{
		ledger: newStaticLedger(
			StoredEvent{
				EventID:   "evt_job_submitted",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "turn.submitted",
				CreatedAt: startedAt,
				Data: EventData{InputItems: []InputItem{{
					Type: "text",
					Text: "download something",
				}}},
			},
			StoredEvent{
				EventID:   "evt_job_tool_call",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "tool.call",
				CreatedAt: startedAt.Add(time.Second),
				Data: EventData{ToolCall: &ToolCallProjection{
					ToolCallEventID: "evt_job_tool_call",
					Tool:            "shell_exec",
					Arguments:       `{"command":"download"}`,
				}},
			},
			StoredEvent{
				EventID:   "evt_job_started",
				SessionID: sessionID,
				TurnID:    turnID,
				JobID:     "job_download",
				Type:      "job.started",
				CreatedAt: startedAt.Add(2 * time.Second),
				Data:      EventData{Job: &job},
			},
			StoredEvent{
				EventID:   "evt_job_completed",
				SessionID: sessionID,
				TurnID:    turnID,
				JobID:     "job_download",
				Type:      "job.completed",
				CreatedAt: startedAt.Add(8 * time.Second),
				Data:      EventData{Job: &completed},
			},
		),
		clock: func() time.Time {
			return startedAt.Add(45 * time.Second)
		},
	}

	timeline, err := k.UITimeline(sessionID)
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	turn := requireSingleTimelineTurn(t, timeline, turnID)
	if turn.Phase != RuntimePhaseRunning {
		t.Fatalf("turn phase = %q, want running without assistant final", turn.Phase)
	}
	processing := requireTimelineChild(t, turn, "processing_group")
	if processing.Phase != RuntimePhaseRunning || !processing.DefaultOpen {
		t.Fatalf("processing group = %+v, want running and default open after job terminal", processing)
	}
	if processing.Text != "正在处理 45s" {
		t.Fatalf("processing text = %q, want live elapsed instead of fixed job duration", processing.Text)
	}
	operation := requireNestedTimelineChild(t, processing, "operation_detail")
	if operation.Phase != RuntimePhaseEnded || operation.TerminalOutcome != TerminalOutcomeSucceeded || !strings.Contains(operation.OutputPreview, "download complete") {
		t.Fatalf("operation detail = %+v, want completed job detail under running turn", operation)
	}
	timelineJSON, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	for _, forbidden := range []string{"command_preview", "visible_output"} {
		if strings.Contains(string(timelineJSON), forbidden) {
			t.Fatalf("main timeline leaked job detail-only field %q: %s", forbidden, string(timelineJSON))
		}
	}
	if !strings.Contains(string(timelineJSON), "sk-job-secret") || strings.Contains(string(timelineJSON), "[REDACTED]") {
		t.Fatalf("main timeline should preserve local job output preview without lossy redaction: %s", string(timelineJSON))
	}
	detail, err := k.UITimelineDetail(sessionID, operation.ItemID)
	if err != nil {
		t.Fatalf("UITimelineDetail returned error: %v", err)
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal job detail: %v", err)
	}
	var detailMap map[string]any
	if err := json.Unmarshal(detailJSON, &detailMap); err != nil {
		t.Fatalf("unmarshal job detail: %v", err)
	}
	item, ok := detailMap["item"].(map[string]any)
	if !ok {
		t.Fatalf("job detail item = %#v, want object", detailMap["item"])
	}
	assertStringFieldContains(t, item, "command_preview", "download")
	assertStringFieldContains(t, item, "visible_output", "download complete")
	if got := item["duration_ms"]; got != float64(6000) {
		t.Fatalf("job duration_ms = %#v, want 6000", got)
	}
	if timelineChild(turn, "assistant_message") != nil {
		t.Fatalf("turn children = %+v, want no assistant message before final", turn.Children)
	}
}

func TestUITimelineApprovalRequiredProjectsUserActionNode(t *testing.T) {
	startedAt := time.Date(2026, 6, 24, 11, 0, 0, 0, time.UTC)
	k := &Kernel{
		ledger: newStaticLedger(
			StoredEvent{
				EventID:   "evt_approval_submitted",
				SessionID: "timeline-approval-session",
				TurnID:    "turn_approval",
				Type:      "turn.submitted",
				CreatedAt: startedAt,
				Data: EventData{InputItems: []InputItem{{
					Type: "text",
					Text: "write a file",
				}}},
			},
			StoredEvent{
				EventID:   "evt_approval_tool_call",
				SessionID: "timeline-approval-session",
				TurnID:    "turn_approval",
				Type:      "tool.call",
				CreatedAt: startedAt.Add(time.Second),
				Data: EventData{ToolCall: &ToolCallProjection{
					ToolCallEventID: "evt_approval_tool_call",
					Tool:            "shell_exec",
					Arguments:       `{"command":"write"}`,
				}},
			},
			StoredEvent{
				EventID:   "evt_approval_tool_result",
				SessionID: "timeline-approval-session",
				TurnID:    "turn_approval",
				Type:      "tool.result",
				CreatedAt: startedAt.Add(2 * time.Second),
				Data: EventData{ToolResult: &ToolResultProjection{
					ToolCallEventID: "evt_approval_tool_call",
					Tool:            "shell_exec",
					ForEventID:      "evt_approval_tool_call",
					Status:          "approval_required",
					Content:         `{"status":"approval_required","executed":false,"error":{"code":"approval_required","message":"approval required"}}`,
				}},
			},
		),
		clock: func() time.Time {
			return startedAt.Add(30 * time.Second)
		},
	}

	timeline, err := k.UITimeline("timeline-approval-session")
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	turn := requireSingleTimelineTurn(t, timeline, "turn_approval")
	action := requireTimelineChild(t, turn, "user_action_request")
	if action.Phase != RuntimePhaseWaiting || action.WaitReason != WaitReasonApprovalRequired || action.Tool != "shell_exec" {
		t.Fatalf("user action = %+v, want shell approval request", action)
	}
	if timelineChild(turn, "assistant_message") != nil {
		t.Fatalf("turn children = %+v, want no assistant-authored approval prompt", turn.Children)
	}
	if timelineChild(turn, "tool") != nil {
		t.Fatalf("turn children = %+v, want no generic tool row for approval", turn.Children)
	}
}

func TestUITimelineDetailProjectionAddsSanitizedOperationDiagnostics(t *testing.T) {
	startedAt := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	sessionID := "timeline-detail-session"
	turnID := "turn_detail"
	k := &Kernel{
		ledger: newStaticLedger(
			StoredEvent{
				EventID:   "evt_detail_submitted",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "turn.submitted",
				CreatedAt: startedAt,
				Data: EventData{InputItems: []InputItem{{
					Type: "text",
					Text: "show detail",
				}}},
			},
			StoredEvent{
				EventID:   "evt_detail_tool_call",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "tool.call",
				CreatedAt: startedAt.Add(time.Second),
				Data: EventData{ToolCall: &ToolCallProjection{
					ToolCallEventID:    "evt_detail_tool_call",
					ProviderToolCallID: "call_detail_provider",
					Tool:               "shell_exec",
					Arguments:          `{"command":"echo api_key=sk-detail-secret"}`,
				}},
			},
			StoredEvent{
				EventID:   "evt_detail_tool_result",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "tool.result",
				CreatedAt: startedAt.Add(2 * time.Second),
				Data: EventData{ToolResult: &ToolResultProjection{
					ToolCallEventID:    "evt_detail_tool_call",
					ProviderToolCallID: "call_detail_provider",
					Tool:               "shell_exec",
					ForEventID:         "evt_detail_tool_call",
					Status:             "failed",
					Content:            `{"status":"failed","executed":true,"exit_code":2,"elapsed_ms":1400,"stderr":"missing argument api_key=sk-detail-secret","stderr_truncated":true,"stderr_original_bytes":1200,"stderr_omitted_bytes":880,"output_truncation":"head_tail"}`,
				}},
			},
			StoredEvent{
				EventID:   "evt_detail_final",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "model.final",
				CreatedAt: startedAt.Add(5 * time.Second),
				Data: EventData{Final: &FinalMessage{
					Text: "done",
				}},
			},
		),
		runtimeToken: testRuntimeToken,
		clock: func() time.Time {
			return startedAt.Add(time.Minute)
		},
	}

	timeline, err := k.UITimeline(sessionID)
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	turn := requireSingleTimelineTurn(t, timeline, turnID)
	processing := requireTimelineChild(t, turn, "processing_group")
	operation := requireNestedTimelineChild(t, processing, "operation_detail")
	timelineJSON, err := json.Marshal(timeline)
	if err != nil {
		t.Fatalf("marshal timeline: %v", err)
	}
	for _, forbidden := range []string{"command_preview", "visible_output", "output_truncation", "provider_tool_call_id", "operation_id"} {
		if strings.Contains(string(timelineJSON), forbidden) {
			t.Fatalf("main timeline leaked detail-only field %q: %s", forbidden, string(timelineJSON))
		}
	}
	if !strings.Contains(string(timelineJSON), "sk-detail-secret") || strings.Contains(string(timelineJSON), "[REDACTED]") {
		t.Fatalf("main timeline should preserve local output preview without lossy redaction: %s", string(timelineJSON))
	}

	processingDetail, err := k.UITimelineDetail(sessionID, processing.DetailRef)
	if err != nil {
		t.Fatalf("UITimelineDetail processing returned error: %v", err)
	}
	if processingDetail.Item.Kind != "processing_group" || processingDetail.Item.ToolCount != 1 {
		t.Fatalf("processing detail = %+v, want selected processing group", processingDetail)
	}

	operationDetail, err := k.UITimelineDetail(sessionID, operation.ItemID)
	if err != nil {
		t.Fatalf("UITimelineDetail operation returned error: %v", err)
	}
	if operationDetail.Item.Kind != "operation_detail" || operationDetail.Item.Phase != RuntimePhaseEnded || operationDetail.Item.TerminalOutcome != TerminalOutcomeFailed || operationDetail.Item.OutputSource != "stderr" {
		t.Fatalf("operation detail = %+v, want failed stderr operation", operationDetail)
	}
	detailJSON, err := json.Marshal(operationDetail)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	var detailMap map[string]any
	if err := json.Unmarshal(detailJSON, &detailMap); err != nil {
		t.Fatalf("unmarshal detail json: %v", err)
	}
	item, ok := detailMap["item"].(map[string]any)
	if !ok {
		t.Fatalf("detail json item = %#v, want object", detailMap["item"])
	}
	assertStringFieldContains(t, item, "command_preview", "echo")
	assertStringFieldContains(t, item, "visible_output", "missing argument")
	if got := item["duration_ms"]; got != float64(1400) {
		t.Fatalf("duration_ms = %#v, want 1400", got)
	}
	if got := item["output_truncation"]; got != "head_tail" {
		t.Fatalf("output_truncation = %#v, want head_tail", got)
	}
	if got := item["stderr_original_bytes"]; got != float64(1200) {
		t.Fatalf("stderr_original_bytes = %#v, want 1200", got)
	}
	if got := item["stderr_omitted_bytes"]; got != float64(880) {
		t.Fatalf("stderr_omitted_bytes = %#v, want 880", got)
	}
	for _, forbidden := range []string{"tool.call", "tool.result", "for_event_id", "tool_call_event_id", "provider_tool_call_id", "operation_id"} {
		if strings.Contains(string(detailJSON), forbidden) {
			t.Fatalf("detail leaked %q: %s", forbidden, string(detailJSON))
		}
	}
	if !strings.Contains(string(detailJSON), "sk-detail-secret") || strings.Contains(string(detailJSON), "[REDACTED]") {
		t.Fatalf("detail should preserve local command/output content without lossy redaction: %s", string(detailJSON))
	}

	server := httptest.NewServer(Handler(k))
	defer server.Close()
	resp, err := getWithAuth(server.URL + "/sessions/" + sessionID + "/timeline/details/" + operation.ItemID)
	if err != nil {
		t.Fatalf("GET timeline detail failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("timeline detail status = %d, want 200", resp.StatusCode)
	}
	var httpDetail UITimelineDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&httpDetail); err != nil {
		t.Fatalf("decode timeline detail: %v", err)
	}
	if httpDetail.Item.Kind != "operation_detail" || httpDetail.Item.Phase != RuntimePhaseEnded || httpDetail.Item.TerminalOutcome != TerminalOutcomeFailed {
		t.Fatalf("http detail = %+v, want operation detail", httpDetail)
	}
}

func TestUITimelineResourceReadResultUsesTextPreview(t *testing.T) {
	startedAt := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	sessionID := "timeline-resource-read-session"
	turnID := "turn_resource_read"
	k := &Kernel{
		ledger: newStaticLedger(
			StoredEvent{
				EventID:   "evt_resource_submitted",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "turn.submitted",
				CreatedAt: startedAt,
				Data: EventData{InputItems: []InputItem{{
					Type: "text",
					Text: "read resource",
				}}},
			},
			StoredEvent{
				EventID:   "evt_resource_tool_call",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "tool.call",
				CreatedAt: startedAt.Add(time.Second),
				Data: EventData{ToolCall: &ToolCallProjection{
					ToolCallEventID: "evt_resource_tool_call",
					Tool:            "resource_read",
					Arguments:       `{"resource_ref":"res_alpha"}`,
				}},
			},
			StoredEvent{
				EventID:   "evt_resource_tool_result",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "tool.result",
				CreatedAt: startedAt.Add(2 * time.Second),
				Data: EventData{ToolResult: &ToolResultProjection{
					ToolCallEventID: "evt_resource_tool_call",
					Tool:            "resource_read",
					ForEventID:      "evt_resource_tool_call",
					Status:          "completed",
					Content:         `{"status":"completed","executed":true,"resource_ref":"res_alpha","mime_type":"text/plain","text":"resource body api_key=sk-resource-secret","truncated":true}`,
				}},
			},
			StoredEvent{
				EventID:   "evt_resource_final",
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "model.final",
				CreatedAt: startedAt.Add(3 * time.Second),
				Data: EventData{Final: &FinalMessage{
					Text: "read complete",
				}},
			},
		),
		clock: func() time.Time {
			return startedAt.Add(time.Minute)
		},
	}

	timeline, err := k.UITimeline(sessionID)
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	turn := requireSingleTimelineTurn(t, timeline, turnID)
	processing := requireTimelineChild(t, turn, "processing_group")
	operation := requireNestedTimelineChild(t, processing, "operation_detail")
	if operation.Tool != "resource_read" || operation.OutputSource != "text" {
		t.Fatalf("operation detail = %+v, want resource text preview", operation)
	}
	if !strings.Contains(operation.OutputPreview, "resource body api_key=sk-resource-secret") || strings.Contains(operation.OutputPreview, "[REDACTED]") {
		t.Fatalf("resource preview = %q, want local text content with budget metadata only", operation.OutputPreview)
	}
	if !operation.OutputTruncated || !operation.FullOutputAvailable {
		t.Fatalf("operation detail = %+v, want truncation and full-output signal", operation)
	}
	detail, err := k.UITimelineDetail(sessionID, operation.ItemID)
	if err != nil {
		t.Fatalf("UITimelineDetail returned error: %v", err)
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal resource detail: %v", err)
	}
	var detailMap map[string]any
	if err := json.Unmarshal(detailJSON, &detailMap); err != nil {
		t.Fatalf("unmarshal resource detail: %v", err)
	}
	item, ok := detailMap["item"].(map[string]any)
	if !ok {
		t.Fatalf("resource detail json item = %#v, want object", detailMap["item"])
	}
	assertStringFieldContains(t, item, "command_preview", "res_alpha")
	assertStringFieldContains(t, item, "visible_output", "resource body")
	if got := item["original_bytes"]; got != nil {
		t.Fatalf("original_bytes = %#v, want omitted when source did not report bytes", got)
	}
}

func assertStringFieldContains(t *testing.T, fields map[string]any, key string, want string) {
	t.Helper()
	got, ok := fields[key].(string)
	if !ok || !strings.Contains(got, want) {
		t.Fatalf("%s = %#v, want string containing %q", key, fields[key], want)
	}
}

func requireSingleTimelineTurn(t *testing.T, timeline UITimelineResponse, turnID string) UITimelineItem {
	t.Helper()
	if timeline.Readiness != ReadinessReady || len(timeline.Items) != 1 {
		t.Fatalf("timeline = %+v, want one turn item", timeline)
	}
	turn := timeline.Items[0]
	if turn.Kind != "turn" || turn.TurnID != turnID {
		t.Fatalf("turn item = %+v, want turn %q", turn, turnID)
	}
	return turn
}

func requireTimelineChild(t *testing.T, item UITimelineItem, kind string) UITimelineItem {
	t.Helper()
	child := timelineChild(item, kind)
	if child == nil {
		t.Fatalf("item children = %+v, want %s child", item.Children, kind)
	}
	return *child
}

func requireNestedTimelineChild(t *testing.T, item UITimelineItem, kind string) UITimelineItem {
	t.Helper()
	for _, child := range item.Children {
		if child.Kind == kind {
			return child
		}
		if nested := timelineChild(child, kind); nested != nil {
			return *nested
		}
	}
	t.Fatalf("item children = %+v, want nested %s child", item.Children, kind)
	return UITimelineItem{}
}

func timelineChild(item UITimelineItem, kind string) *UITimelineItem {
	for i := range item.Children {
		if item.Children[i].Kind == kind {
			return &item.Children[i]
		}
	}
	return nil
}

func timelineAnyItem(items []UITimelineItem, match func(UITimelineItem) bool) bool {
	for _, item := range items {
		if match(item) {
			return true
		}
		if timelineAnyItem(item.Children, match) {
			return true
		}
	}
	return false
}
