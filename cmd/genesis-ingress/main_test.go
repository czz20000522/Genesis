package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"genesis/internal/applications/connector_runtime"
	"genesis/internal/testsupport"
)

func TestFeishuOnceSubmitsInboundMessageWithoutOutboundCLIFlags(t *testing.T) {
	var got connectorruntime.TurnSubmitRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/turn" {
			t.Fatalf("path = %q, want /turn", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode turn request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(connectorruntime.TurnSubmitResponse{
			SessionID: got.SessionID,
			TurnID:    "turn-1",
			Final:     connectorruntime.FinalAnswer{Text: "local final"},
		})
	}))
	t.Cleanup(server.Close)

	err := run(context.Background(), []string{
		"feishu-once",
		"--kernel-url", server.URL,
		"--runtime-token", "token",
		"--state", filepath.Join(testsupport.ProjectTempDir(t, "feishu-once"), "state.json"),
		"--message-id", "msg-1",
		"--thread-id", "chat-1",
		"--user-id", "user-1",
		"--chat-id", "oc_123",
		"--text", "hello",
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if len(got.InputItems) != 1 {
		t.Fatalf("input items = %+v", got.InputItems)
	}
	input := got.InputItems[0].Text
	for _, want := range []string{"connector: feishu", "event_type: message.created", "thread_kind: chat", "text:\nhello"} {
		if !strings.Contains(input, want) {
			t.Fatalf("turn input missing %q in:\n%s", want, input)
		}
	}
	for _, forbidden := range []string{"oc_123", "msg-1", "user-1", "credential", "provider_context"} {
		if strings.Contains(input, forbidden) {
			t.Fatalf("turn input contains forbidden value %q in:\n%s", forbidden, input)
		}
	}
}

func TestFeishuListenConsumesNDJSONEventsAndDedupes(t *testing.T) {
	var submitCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/turn" {
			t.Fatalf("path = %q, want /turn", r.URL.Path)
		}
		submitCount++
		var got connectorruntime.TurnSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode turn request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(connectorruntime.TurnSubmitResponse{
			SessionID: got.SessionID,
			TurnID:    "turn-1",
			Final:     connectorruntime.FinalAnswer{Text: "listener final"},
		})
	}))
	t.Cleanup(server.Close)

	event := testFeishuExternalEvent("msg-dup")
	var input bytes.Buffer
	if err := json.NewEncoder(&input).Encode(event); err != nil {
		t.Fatalf("encode first event: %v", err)
	}
	if err := json.NewEncoder(&input).Encode(event); err != nil {
		t.Fatalf("encode duplicate event: %v", err)
	}

	var stdout bytes.Buffer
	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--runtime-token", "token",
		"--state", filepath.Join(testsupport.ProjectTempDir(t, "feishu-listen"), "state.json"),
		"--profile", "codex",
		"--stdin-jsonl",
	}, &input, &stdout, io.Discard)
	if err != nil {
		t.Fatalf("runWithIO returned error: %v", err)
	}
	if submitCount != 1 {
		t.Fatalf("submit count = %d, want one kernel turn after dedupe", submitCount)
	}
	output := stdout.String()
	if !strings.Contains(output, `"duplicate":false`) || !strings.Contains(output, `"duplicate":true`) {
		t.Fatalf("listener output should contain first result and duplicate result, got:\n%s", output)
	}
}

func TestFeishuListenDeliverFinalRequiresExplicitProfileBeforeKernelCall(t *testing.T) {
	var submitCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		submitCount++
		t.Fatalf("kernel should not be called when final delivery profile is missing; got %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	var input bytes.Buffer
	if err := json.NewEncoder(&input).Encode(testFeishuExternalEvent("msg-1")); err != nil {
		t.Fatalf("encode event: %v", err)
	}

	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--runtime-token", "token",
		"--state", filepath.Join(testsupport.ProjectTempDir(t, "feishu-listen-missing-profile"), "state.json"),
		"--stdin-jsonl",
		"--deliver-final",
	}, &input, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should reject missing explicit profile for final delivery")
	}
	if submitCount != 0 {
		t.Fatalf("submit count = %d, want 0", submitCount)
	}
}

func TestFeishuListenMissingSourceCommandRecordsBlockedSourceRun(t *testing.T) {
	var submitCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		submitCount++
		t.Fatalf("kernel should not be called when source command is missing; got %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)
	dir := testsupport.ProjectTempDir(t, "feishu-listen-missing-source-command")
	sourceSupervisorPath := filepath.Join(dir, "source-supervisor.json")

	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--source-id", "source_feishu_chat",
		"--source-state", filepath.Join(dir, "source-failures.json"),
		"--source-supervisor-state", sourceSupervisorPath,
	}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should reject missing source command")
	}
	if submitCount != 0 {
		t.Fatalf("submit count = %d, want 0", submitCount)
	}
	store, err := connectorruntime.NewFileSourceSupervisorStore(sourceSupervisorPath)
	if err != nil {
		t.Fatalf("NewFileSourceSupervisorStore returned error: %v", err)
	}
	runs, err := store.ListSourceRuns(context.Background())
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != connectorruntime.SourceRunStatusBlocked || !strings.Contains(runs[0].BlockedReason, "source command executable") {
		t.Fatalf("source runs = %+v, want blocked missing source command readiness record", runs)
	}
}

func TestFeishuListenInvalidSourceCommandRecordsBlockedSourceRun(t *testing.T) {
	var submitCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		submitCount++
		t.Fatalf("kernel should not be called when source command is invalid; got %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)
	dir := testsupport.ProjectTempDir(t, "feishu-listen-source-blocked")
	sourceSupervisorPath := filepath.Join(dir, "source-supervisor.json")

	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--source-id", "source_feishu_chat",
		"--source-command", "adapter --bad",
		"--source-state", filepath.Join(dir, "source-failures.json"),
		"--source-supervisor-state", sourceSupervisorPath,
	}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should reject invalid source command executable")
	}
	if submitCount != 0 {
		t.Fatalf("submit count = %d, want 0", submitCount)
	}
	store, err := connectorruntime.NewFileSourceSupervisorStore(sourceSupervisorPath)
	if err != nil {
		t.Fatalf("NewFileSourceSupervisorStore returned error: %v", err)
	}
	runs, err := store.ListSourceRuns(context.Background())
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != connectorruntime.SourceRunStatusBlocked || !strings.Contains(runs[0].BlockedReason, "direct executable") {
		t.Fatalf("source runs = %+v, want blocked invalid source command readiness record", runs)
	}
}

func TestFeishuProbeReportsInstalledAdapterReadiness(t *testing.T) {
	var stdout bytes.Buffer
	if err := runWithIO(context.Background(), []string{
		"feishu-probe",
		"--profile", "genesis",
		"--lark-cli", os.Args[0],
		"--source-command", os.Args[0],
		"--source-command-arg", "-test.run=TestHelper",
	}, strings.NewReader(""), &stdout, io.Discard); err != nil {
		t.Fatalf("runWithIO returned error: %v\n%s", err, stdout.String())
	}
	var got connectorruntime.FeishuAdapterProbeReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode probe report: %v\n%s", err, stdout.String())
	}
	if !got.Ready || got.EventSource.Status != connectorruntime.ProbeStatusOK || got.FinalDelivery.Status != connectorruntime.ProbeStatusOK {
		t.Fatalf("probe report = %+v", got)
	}
	if strings.Contains(strings.Join(got.EventSource.Args, " "), "event consume") {
		t.Fatalf("event source args must describe source adapter args, not lark-cli event syntax: %#v", got.EventSource.Args)
	}
	if !strings.Contains(strings.Join(got.FinalDelivery.Args, " "), "+messages-send") {
		t.Fatalf("final delivery args = %#v", got.FinalDelivery.Args)
	}
}

func TestFeishuProbeRejectsMissingProfileWithoutKernelCall(t *testing.T) {
	var stdout bytes.Buffer
	err := runWithIO(context.Background(), []string{
		"feishu-probe",
		"--lark-cli", os.Args[0],
	}, strings.NewReader(""), &stdout, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should reject missing profile")
	}
	var got connectorruntime.FeishuAdapterProbeReport
	if decodeErr := json.Unmarshal(stdout.Bytes(), &got); decodeErr != nil {
		t.Fatalf("decode probe report: %v\n%s", decodeErr, stdout.String())
	}
	if got.Ready || got.EventSource.Status != connectorruntime.ProbeStatusFailed || got.FinalDelivery.Status != connectorruntime.ProbeStatusFailed {
		t.Fatalf("probe report = %+v", got)
	}
}

func testFeishuExternalEvent(eventID string) connectorruntime.ExternalEvent {
	return connectorruntime.ExternalEvent{
		Connector:       "feishu",
		ExternalEventID: eventID,
		EventType:       "message.created",
		ThreadRef: connectorruntime.ExternalThreadRef{
			Connector:  "feishu",
			Kind:       "chat",
			ExternalID: "oc_123",
		},
		SenderRef: connectorruntime.ExternalRef{
			Connector:  "feishu",
			Kind:       "user",
			ExternalID: "ou_123",
			Display:    "Codex",
		},
		MessageRef: connectorruntime.ExternalRef{
			Connector:  "feishu",
			Kind:       "message",
			ExternalID: eventID,
		},
		Body:             "hello from feishu stream",
		SourceValidation: connectorruntime.SourceValidationVerified,
	}
}
