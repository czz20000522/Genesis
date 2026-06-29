package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"genesis/internal/applications/connector_runtime"
	feishucli "genesis/internal/applications/feishu_cli"
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
		"--profile", "genesis",
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

func TestFeishuListenDeliverFinalProfileReadinessBlocksBeforeKernelCall(t *testing.T) {
	var submitCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		submitCount++
		t.Fatalf("kernel should not be called when final delivery profile requires refresh; got %s %s", r.Method, r.URL.Path)
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
		"--state", filepath.Join(testsupport.ProjectTempDir(t, "feishu-listen-refresh-required"), "state.json"),
		"--stdin-jsonl",
		"--deliver-final",
		"--profile", "genesis",
		"--profile-readiness", connectorruntime.SourceReadinessReasonRefreshRequired,
	}, &input, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should reject refresh-required profile readiness for final delivery")
	}
	if submitCount != 0 {
		t.Fatalf("submit count = %d, want 0", submitCount)
	}
}

func TestFeishuListenDeliverFinalUsesConnectorCommandAdapter(t *testing.T) {
	var submitCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/turn" {
			t.Fatalf("path = %q, want /turn", r.URL.Path)
		}
		submitCount++
		_ = json.NewEncoder(w).Encode(connectorruntime.TurnSubmitResponse{
			SessionID: "session-1",
			TurnID:    "turn-1",
			Final:     connectorruntime.FinalAnswer{Text: "listener final"},
		})
	}))
	t.Cleanup(server.Close)
	dir := testsupport.ProjectTempDir(t, "feishu-listen-deliver-final-connector-command")
	capturePath := filepath.Join(dir, "captured-action.json")

	var input bytes.Buffer
	if err := json.NewEncoder(&input).Encode(testFeishuExternalEvent("msg-1")); err != nil {
		t.Fatalf("encode event: %v", err)
	}
	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--runtime-token", "token",
		"--state", filepath.Join(dir, "state.json"),
		"--outbox-state", filepath.Join(dir, "outbox.json"),
		"--stdin-jsonl",
		"--deliver-final",
		"--profile", "genesis",
		"--delivery-command", os.Args[0],
		"--delivery-command-arg", "-test.run=TestFeishuDeliveryCommandHelper",
		"--delivery-command-arg", "--",
		"--delivery-command-arg", "--capture=" + capturePath,
	}, &input, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("runWithIO returned error: %v", err)
	}
	if submitCount != 1 {
		t.Fatalf("submit count = %d, want 1", submitCount)
	}
	var captured connectorruntime.ConnectorAction
	content, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("read captured connector action: %v", err)
	}
	if err := json.Unmarshal(content, &captured); err != nil {
		t.Fatalf("decode captured connector action: %v", err)
	}
	if captured.Connector != "feishu" || captured.ActionKind != "send_message" || captured.Payload["body"] != "listener final" {
		t.Fatalf("captured action = %+v, want typed Feishu send_message action", captured)
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
	sourceLifecyclePath := filepath.Join(dir, "source-lifecycle.json")

	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--source-id", "source_feishu_chat",
		"--source-state", filepath.Join(dir, "source-failures.json"),
		"--source-lifecycle-state", sourceLifecyclePath,
	}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should reject missing source command")
	}
	if submitCount != 0 {
		t.Fatalf("submit count = %d, want 0", submitCount)
	}
	store, err := connectorruntime.NewFileSourceLifecycleStore(sourceLifecyclePath)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	runs, err := store.ListSourceRuns(context.Background())
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != connectorruntime.SourceRunStatusBlocked || runs[0].BlockedReasonCode != connectorruntime.SourceReadinessReasonSourceCommandInvalid || !strings.Contains(runs[0].BlockedReason, "source command executable") {
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
	sourceLifecyclePath := filepath.Join(dir, "source-lifecycle.json")

	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--source-id", "source_feishu_chat",
		"--profile", "genesis",
		"--source-command", "adapter --bad",
		"--source-state", filepath.Join(dir, "source-failures.json"),
		"--source-lifecycle-state", sourceLifecyclePath,
	}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should reject invalid source command executable")
	}
	if submitCount != 0 {
		t.Fatalf("submit count = %d, want 0", submitCount)
	}
	store, err := connectorruntime.NewFileSourceLifecycleStore(sourceLifecyclePath)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	runs, err := store.ListSourceRuns(context.Background())
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != connectorruntime.SourceRunStatusBlocked || runs[0].BlockedReasonCode != connectorruntime.SourceReadinessReasonSourceCommandInvalid || !strings.Contains(runs[0].BlockedReason, "direct executable") {
		t.Fatalf("source runs = %+v, want blocked invalid source command readiness record", runs)
	}
}

func TestFeishuListenSourceCommandRequiresExplicitProfileBeforeProcessStart(t *testing.T) {
	var submitCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		submitCount++
		t.Fatalf("kernel should not be called when source profile is missing; got %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)
	dir := testsupport.ProjectTempDir(t, "feishu-listen-source-missing-profile")
	sourceLifecyclePath := filepath.Join(dir, "source-lifecycle.json")
	startedPath := filepath.Join(dir, "source-started.txt")

	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--source-id", "source_feishu_chat",
		"--source-command", os.Args[0],
		"--source-command-arg", "-test.run=TestFeishuListenSourceCommandHelper",
		"--source-command-arg", "--",
		"--source-command-arg", "record-start",
		"--source-command-arg", startedPath,
		"--source-state", filepath.Join(dir, "source-failures.json"),
		"--source-lifecycle-state", sourceLifecyclePath,
	}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should block missing profile readiness before source process start")
	}
	if submitCount != 0 {
		t.Fatalf("submit count = %d, want 0", submitCount)
	}
	if _, statErr := os.Stat(startedPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("source command side effect exists or stat failed: %v", statErr)
	}
	store, err := connectorruntime.NewFileSourceLifecycleStore(sourceLifecyclePath)
	if err != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
	}
	runs, err := store.ListSourceRuns(context.Background())
	if err != nil {
		t.Fatalf("ListSourceRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != connectorruntime.SourceRunStatusBlocked || runs[0].BlockedReasonCode != connectorruntime.SourceReadinessReasonMissingProfile {
		t.Fatalf("source runs = %+v, want missing_profile blocked readiness record", runs)
	}
}

func TestFeishuListenProfileReadinessBlocksSourceBeforeProcessStart(t *testing.T) {
	for _, reason := range []string{
		connectorruntime.SourceReadinessReasonProfileExpired,
		connectorruntime.SourceReadinessReasonPermissionDenied,
		connectorruntime.SourceReadinessReasonRefreshRequired,
	} {
		t.Run(reason, func(t *testing.T) {
			var submitCount int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				submitCount++
				t.Fatalf("kernel should not be called when source profile readiness is blocked; got %s %s", r.Method, r.URL.Path)
			}))
			t.Cleanup(server.Close)
			dir := testsupport.ProjectTempDir(t, "feishu-listen-source-"+reason)
			sourceLifecyclePath := filepath.Join(dir, "source-lifecycle.json")
			startedPath := filepath.Join(dir, "source-started.txt")

			err := runWithIO(context.Background(), []string{
				"feishu-listen",
				"--kernel-url", server.URL,
				"--source-id", "source_feishu_chat",
				"--profile", "genesis",
				"--profile-readiness", reason,
				"--source-command", os.Args[0],
				"--source-command-arg", "-test.run=TestFeishuListenSourceCommandHelper",
				"--source-command-arg", "--",
				"--source-command-arg", "record-start",
				"--source-command-arg", startedPath,
				"--source-state", filepath.Join(dir, "source-failures.json"),
				"--source-lifecycle-state", sourceLifecyclePath,
			}, strings.NewReader(""), io.Discard, io.Discard)
			if err == nil {
				t.Fatalf("runWithIO should block %s profile readiness before source process start", reason)
			}
			if submitCount != 0 {
				t.Fatalf("submit count = %d, want 0", submitCount)
			}
			if _, statErr := os.Stat(startedPath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("source command side effect exists or stat failed: %v", statErr)
			}
			store, err := connectorruntime.NewFileSourceLifecycleStore(sourceLifecyclePath)
			if err != nil {
				t.Fatalf("NewFileSourceLifecycleStore returned error: %v", err)
			}
			runs, err := store.ListSourceRuns(context.Background())
			if err != nil {
				t.Fatalf("ListSourceRuns returned error: %v", err)
			}
			if len(runs) != 1 || runs[0].Status != connectorruntime.SourceRunStatusBlocked || runs[0].BlockedReasonCode != reason {
				t.Fatalf("source runs = %+v, want %s blocked readiness record", runs, reason)
			}
		})
	}
}

func TestFeishuListenProfileProbeBlocksSourceBeforeProcessStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("kernel should not be called when profile probe blocks source; got %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)
	dir := testsupport.ProjectTempDir(t, "feishu-listen-source-profile-probe")
	sourceLifecyclePath := filepath.Join(dir, "source-lifecycle.json")
	startedPath := filepath.Join(dir, "source-started.txt")

	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--runtime-token", "token",
		"--state", filepath.Join(dir, "state.json"),
		"--source-id", "source_feishu_chat",
		"--profile", "genesis",
		"--profile-probe-command", os.Args[0],
		"--profile-probe-command-arg", "-test.run=TestFeishuProfileProbeHelper",
		"--profile-probe-command-arg", "--",
		"--profile-probe-command-arg", connectorruntime.SourceReadinessReasonRefreshRequired,
		"--source-command", os.Args[0],
		"--source-command-arg", "-test.run=TestFeishuListenSourceCommandHelper",
		"--source-command-arg", "--",
		"--source-command-arg", "record-start",
		"--source-command-arg", startedPath,
		"--source-state", filepath.Join(dir, "source-failures.json"),
		"--source-lifecycle-state", sourceLifecyclePath,
	}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should block refresh_required profile probe before source process start")
	}
	if _, statErr := os.Stat(startedPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("source command side effect exists or stat failed: %v", statErr)
	}
	store, storeErr := connectorruntime.NewFileSourceLifecycleStore(sourceLifecyclePath)
	if storeErr != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", storeErr)
	}
	runs, listErr := store.ListSourceRuns(context.Background())
	if listErr != nil {
		t.Fatalf("ListSourceRuns returned error: %v", listErr)
	}
	if len(runs) != 1 || runs[0].Status != connectorruntime.SourceRunStatusBlocked || runs[0].BlockedReasonCode != connectorruntime.SourceReadinessReasonRefreshRequired {
		t.Fatalf("source runs = %+v, want refresh_required blocked readiness record", runs)
	}
}

func TestFeishuListenProfileProbeTimeoutBlocksSourceBeforeProcessStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("kernel should not be called when profile probe times out; got %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)
	dir := testsupport.ProjectTempDir(t, "feishu-listen-source-profile-timeout")
	sourceLifecyclePath := filepath.Join(dir, "source-lifecycle.json")
	startedPath := filepath.Join(dir, "source-started.txt")
	startedAt := time.Now()

	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--runtime-token", "token",
		"--state", filepath.Join(dir, "state.json"),
		"--source-id", "source_feishu_chat",
		"--profile", "genesis",
		"--profile-probe-command", os.Args[0],
		"--profile-probe-command-arg", "-test.run=TestFeishuProfileProbeHelper",
		"--profile-probe-command-arg", "--",
		"--profile-probe-command-arg", "hang",
		"--profile-probe-timeout", "10ms",
		"--source-command", os.Args[0],
		"--source-command-arg", "-test.run=TestFeishuListenSourceCommandHelper",
		"--source-command-arg", "--",
		"--source-command-arg", "record-start",
		"--source-command-arg", startedPath,
		"--source-state", filepath.Join(dir, "source-failures.json"),
		"--source-lifecycle-state", sourceLifecyclePath,
	}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should block timed-out profile probe before source process start")
	}
	if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
		t.Fatalf("profile probe elapsed %s, want bounded timeout", elapsed)
	}
	if _, statErr := os.Stat(startedPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("source command side effect exists or stat failed: %v", statErr)
	}
	store, storeErr := connectorruntime.NewFileSourceLifecycleStore(sourceLifecyclePath)
	if storeErr != nil {
		t.Fatalf("NewFileSourceLifecycleStore returned error: %v", storeErr)
	}
	runs, listErr := store.ListSourceRuns(context.Background())
	if listErr != nil {
		t.Fatalf("ListSourceRuns returned error: %v", listErr)
	}
	if len(runs) != 1 || runs[0].Status != connectorruntime.SourceRunStatusBlocked || runs[0].BlockedReasonCode != connectorruntime.SourceReadinessReasonOperatorActionRequired {
		t.Fatalf("source runs = %+v, want operator_action_required blocked readiness record", runs)
	}
}

func TestFeishuListenRetriesRecoverableSourceCommandFailure(t *testing.T) {
	var submitCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/turn" {
			t.Fatalf("path = %q, want /turn", r.URL.Path)
		}
		submitCount++
		_ = json.NewEncoder(w).Encode(connectorruntime.TurnSubmitResponse{
			SessionID: "session-1",
			TurnID:    "turn-1",
			Final:     connectorruntime.FinalAnswer{Text: "listener final"},
		})
	}))
	t.Cleanup(server.Close)
	dir := testsupport.ProjectTempDir(t, "feishu-listen-source-retry")
	attemptFile := filepath.Join(dir, "attempts.txt")

	var stdout bytes.Buffer
	err := runWithIO(context.Background(), []string{
		"feishu-listen",
		"--kernel-url", server.URL,
		"--runtime-token", "token",
		"--state", filepath.Join(dir, "state.json"),
		"--source-command", os.Args[0],
		"--profile", "genesis",
		"--source-command-arg", "-test.run=TestFeishuListenSourceCommandHelper",
		"--source-command-arg", "--",
		"--source-command-arg", "fail-once-then-event",
		"--source-command-arg", attemptFile,
		"--source-id", "source_feishu_chat",
		"--source-state", filepath.Join(dir, "source-failures.json"),
		"--source-lifecycle-state", filepath.Join(dir, "source-lifecycle.json"),
		"--source-attempts", "2",
		"--source-backoff", "0s",
	}, strings.NewReader(""), &stdout, io.Discard)
	if err != nil {
		t.Fatalf("runWithIO returned error: %v\n%s", err, stdout.String())
	}
	if submitCount != 1 {
		t.Fatalf("submit count = %d, want one kernel turn after source retry", submitCount)
	}
	if !strings.Contains(stdout.String(), "listener final") {
		t.Fatalf("listener output = %s, want kernel final", stdout.String())
	}
}

func TestFeishuProbeReportsInstalledAdapterReadiness(t *testing.T) {
	var stdout bytes.Buffer
	if err := runWithIO(context.Background(), []string{
		"feishu-probe",
		"--profile", "genesis",
		"--delivery-command", os.Args[0],
		"--delivery-command-arg", "-test.run=TestFeishuDeliveryCommandHelper",
		"--source-command", os.Args[0],
		"--source-command-arg", "-test.run=TestHelper",
	}, strings.NewReader(""), &stdout, io.Discard); err != nil {
		t.Fatalf("runWithIO returned error: %v\n%s", err, stdout.String())
	}
	var got feishucli.AdapterProbeReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode probe report: %v\n%s", err, stdout.String())
	}
	if !got.Ready || got.EventSource.Status != connectorruntime.ProbeStatusOK || got.FinalDelivery.Status != connectorruntime.ProbeStatusOK {
		t.Fatalf("probe report = %+v", got)
	}
	if strings.Contains(strings.Join(got.EventSource.Args, " "), "event consume") {
		t.Fatalf("event source args must describe source adapter args, not lark-cli event syntax: %#v", got.EventSource.Args)
	}
	if !strings.Contains(strings.Join(got.EventSource.Args, " "), "--profile genesis") {
		t.Fatalf("event source args = %#v, want source adapter profile binding", got.EventSource.Args)
	}
	finalArgs := strings.Join(got.FinalDelivery.Args, " ")
	if strings.Contains(finalArgs, "+messages-send") {
		t.Fatalf("final delivery args must describe connector adapter args, not lark-cli message syntax: %#v", got.FinalDelivery.Args)
	}
	if !strings.Contains(finalArgs, "--profile genesis") {
		t.Fatalf("final delivery args = %#v, want connector adapter profile binding", got.FinalDelivery.Args)
	}
}

func TestFeishuProbeDoesNotStartSourceOrDeliveryAdapters(t *testing.T) {
	dir := testsupport.ProjectTempDir(t, "feishu-probe-no-adapter-effects")
	startedPath := filepath.Join(dir, "source-started.txt")
	capturePath := filepath.Join(dir, "delivery-action.json")

	var stdout bytes.Buffer
	if err := runWithIO(context.Background(), []string{
		"feishu-probe",
		"--profile", "genesis",
		"--delivery-command", os.Args[0],
		"--delivery-command-arg", "-test.run=TestFeishuDeliveryCommandHelper",
		"--delivery-command-arg", "--capture=" + capturePath,
		"--source-command", os.Args[0],
		"--source-command-arg", "-test.run=TestFeishuListenSourceCommandHelper",
		"--source-command-arg", "--",
		"--source-command-arg", "record-start",
		"--source-command-arg", startedPath,
	}, strings.NewReader(""), &stdout, io.Discard); err != nil {
		t.Fatalf("runWithIO returned error: %v\n%s", err, stdout.String())
	}
	if _, err := os.Stat(startedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source probe executed source adapter side effect or stat failed: %v", err)
	}
	if _, err := os.Stat(capturePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source probe executed delivery adapter side effect or stat failed: %v", err)
	}
}

func TestFeishuProbeReportsProfileReadinessFailure(t *testing.T) {
	var stdout bytes.Buffer
	err := runWithIO(context.Background(), []string{
		"feishu-probe",
		"--profile", "genesis",
		"--profile-readiness", connectorruntime.SourceReadinessReasonPermissionDenied,
		"--source-command", os.Args[0],
		"--delivery-command", os.Args[0],
	}, strings.NewReader(""), &stdout, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should reject permission-denied profile readiness")
	}
	var got feishucli.AdapterProbeReport
	if decodeErr := json.Unmarshal(stdout.Bytes(), &got); decodeErr != nil {
		t.Fatalf("decode probe report: %v\n%s", decodeErr, stdout.String())
	}
	if got.Ready || got.EventSource.Reason != connectorruntime.SourceReadinessReasonPermissionDenied || got.FinalDelivery.Reason != connectorruntime.SourceReadinessReasonPermissionDenied {
		t.Fatalf("probe report = %+v, want permission_denied on source and final delivery", got)
	}
}

func TestFeishuProbeUsesProfileReadinessCommandBeforeAdapters(t *testing.T) {
	var stdout bytes.Buffer
	err := runWithIO(context.Background(), []string{
		"feishu-probe",
		"--profile", "genesis",
		"--profile-probe-command", os.Args[0],
		"--profile-probe-command-arg", "-test.run=TestFeishuProfileProbeHelper",
		"--profile-probe-command-arg", "--",
		"--profile-probe-command-arg", connectorruntime.SourceReadinessReasonProfileExpired,
		"--source-command", os.Args[0],
		"--source-command-arg", "-test.run=TestFeishuListenSourceCommandHelper",
		"--source-command-arg", "--",
		"--source-command-arg", "record-start",
		"--source-command-arg", filepath.Join(testsupport.ProjectTempDir(t, "feishu-profile-probe"), "source-started.txt"),
		"--delivery-command", os.Args[0],
		"--delivery-command-arg", "-test.run=TestFeishuDeliveryCommandHelper",
	}, strings.NewReader(""), &stdout, io.Discard)
	if err == nil {
		t.Fatal("runWithIO should reject profile_expired from profile probe command")
	}
	var got feishucli.AdapterProbeReport
	if decodeErr := json.Unmarshal(stdout.Bytes(), &got); decodeErr != nil {
		t.Fatalf("decode probe report: %v\n%s", decodeErr, stdout.String())
	}
	if got.Ready || got.EventSource.Reason != connectorruntime.SourceReadinessReasonProfileExpired || got.FinalDelivery.Reason != connectorruntime.SourceReadinessReasonProfileExpired {
		t.Fatalf("probe report = %+v, want profile_expired on source and final delivery", got)
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
	var got feishucli.AdapterProbeReport
	if decodeErr := json.Unmarshal(stdout.Bytes(), &got); decodeErr != nil {
		t.Fatalf("decode probe report: %v\n%s", decodeErr, stdout.String())
	}
	if got.Ready || got.EventSource.Status != connectorruntime.ProbeStatusFailed || got.FinalDelivery.Status != connectorruntime.ProbeStatusFailed {
		t.Fatalf("probe report = %+v", got)
	}
}

func TestFeishuProfileReadinessBlockReasonClassifiesKnownStates(t *testing.T) {
	tests := []struct {
		name      string
		profile   string
		readiness string
		want      string
		wantErr   bool
	}{
		{name: "ok", profile: "genesis", readiness: "ok", want: ""},
		{name: "missing", profile: "", readiness: "ok", want: connectorruntime.SourceReadinessReasonMissingProfile},
		{name: "expired", profile: "genesis", readiness: connectorruntime.SourceReadinessReasonProfileExpired, want: connectorruntime.SourceReadinessReasonProfileExpired},
		{name: "denied", profile: "genesis", readiness: connectorruntime.SourceReadinessReasonPermissionDenied, want: connectorruntime.SourceReadinessReasonPermissionDenied},
		{name: "refresh", profile: "genesis", readiness: connectorruntime.SourceReadinessReasonRefreshRequired, want: connectorruntime.SourceReadinessReasonRefreshRequired},
		{name: "invalid", profile: "genesis", readiness: "trusted", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := feishuProfileReadinessBlockReason(context.Background(), tt.profile, tt.readiness, "", nil, 0)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("feishuProfileReadinessBlockReason returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("feishuProfileReadinessBlockReason returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFeishuListenSourceCommandHelper(t *testing.T) {
	mode, attemptFile := sourceCommandHelperArgs()
	if mode == "" {
		return
	}
	switch mode {
	case "fail-once-then-event":
		if attemptFile == "" {
			t.Fatal("attempt file argument is required")
		}
		attempt := 0
		if raw, err := os.ReadFile(attemptFile); err == nil && strings.TrimSpace(string(raw)) != "" {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
			if parseErr != nil {
				t.Fatalf("parse attempt file: %v", parseErr)
			}
			attempt = parsed
		}
		attempt++
		if err := os.WriteFile(attemptFile, []byte(strconv.Itoa(attempt)), 0o600); err != nil {
			t.Fatalf("write attempt file: %v", err)
		}
		if attempt == 1 {
			fmt.Fprintln(os.Stderr, "transient source runtime failure")
			os.Exit(42)
		}
		encoder := json.NewEncoder(os.Stdout)
		frames := []connectorruntime.SourceCommandFrame{
			{Kind: connectorruntime.SourceFrameKindReady, SourceID: "source_feishu_chat", Connector: "feishu", AdapterRef: "feishu-source-adapter"},
			{
				Kind:     connectorruntime.SourceFrameKindEvent,
				SourceID: "source_feishu_chat",
				Event: &connectorruntime.ExternalEvent{
					Connector:        "feishu",
					ExternalEventID:  "evt_retry_success",
					EventType:        "message.created",
					ThreadRef:        connectorruntime.ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_1"},
					SenderRef:        connectorruntime.ExternalRef{Connector: "feishu", Kind: "user", ExternalID: "ou_1"},
					MessageRef:       connectorruntime.ExternalRef{Connector: "feishu", Kind: "message", ExternalID: "om_1"},
					Body:             "hello after retry",
					SourceValidation: connectorruntime.SourceValidationUnchecked,
				},
			},
			{Kind: connectorruntime.SourceFrameKindStopped, SourceID: "source_feishu_chat", Connector: "feishu", AdapterRef: "feishu-source-adapter"},
		}
		for _, frame := range frames {
			if err := encoder.Encode(frame); err != nil {
				t.Fatalf("encode frame: %v", err)
			}
		}
	case "record-start":
		if attemptFile == "" {
			t.Fatal("started file argument is required")
		}
		if err := os.WriteFile(attemptFile, []byte("started"), 0o600); err != nil {
			t.Fatalf("write started file: %v", err)
		}
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
	os.Exit(0)
}

func TestFeishuProfileProbeHelper(t *testing.T) {
	readiness := profileProbeHelperReadiness()
	if readiness == "" {
		return
	}
	if readiness == "hang" {
		time.Sleep(2 * time.Second)
		return
	}
	if err := json.NewEncoder(os.Stdout).Encode(connectorruntime.ProfileReadinessCommandResult{
		Readiness: readiness,
	}); err != nil {
		t.Fatalf("encode profile readiness result: %v", err)
	}
	os.Exit(0)
}

func TestFeishuDeliveryCommandHelper(t *testing.T) {
	capturePath := feishuDeliveryHelperCapturePath()
	if capturePath == "" {
		return
	}
	var action connectorruntime.ConnectorAction
	if err := json.NewDecoder(os.Stdin).Decode(&action); err != nil {
		t.Fatalf("decode connector action: %v", err)
	}
	content, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("marshal connector action: %v", err)
	}
	if err := os.WriteFile(capturePath, content, 0o644); err != nil {
		t.Fatalf("write connector action: %v", err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(connectorruntime.ConnectorActionResult{
		Status:            connectorruntime.DeliveryStatusSent,
		ExternalActionRef: "om_delivery_helper",
	}); err != nil {
		t.Fatalf("encode connector action result: %v", err)
	}
	os.Exit(0)
}

func feishuDeliveryHelperCapturePath() string {
	for _, arg := range os.Args {
		if path, ok := strings.CutPrefix(arg, "--capture="); ok {
			return path
		}
	}
	return ""
}

func profileProbeHelperReadiness() string {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func sourceCommandHelperArgs() (string, string) {
	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}
		mode := ""
		attemptFile := ""
		if i+1 < len(os.Args) {
			mode = os.Args[i+1]
		}
		if i+2 < len(os.Args) {
			attemptFile = os.Args[i+2]
		}
		return mode, attemptFile
	}
	return "", ""
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
		SourceValidation: connectorruntime.SourceValidationUnchecked,
	}
}
