package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTTPReadyTurnAndSession(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	readyResp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer readyResp.Body.Close()
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", readyResp.StatusCode)
	}
	var ready ReadyResponse
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("decode ready response: %v", err)
	}
	if ready.Readiness != ReadinessReady || ready.Provider.Name != "fake" || ready.Provider.Readiness != ReadinessReady {
		t.Fatalf("ready = %+v, want ok fake provider", ready)
	}
	if ready.RuntimeAuth.Readiness != ReadinessReady {
		t.Fatalf("runtime auth ready = %+v, want ok", ready.RuntimeAuth)
	}
	if ready.Ledger.Readiness != ReadinessReady {
		t.Fatalf("ledger ready = %+v, want ok", ready.Ledger)
	}

	body := []byte(`{"session_id":"http-session","input_items":[{"type":"text","text":"hello over http"}]}`)
	turnResp, err := postJSONWithAuth(server.URL+"/turn", body)
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
	if turn.Final.Text != "fake: hello over http" {
		t.Fatalf("turn final = %q, want fake: hello over http", turn.Final.Text)
	}

	sessionResp, err := getWithAuth(server.URL + "/sessions/http-session")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer sessionResp.Body.Close()
	if sessionResp.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want 200", sessionResp.StatusCode)
	}
	var projection SessionProjection
	if err := json.NewDecoder(sessionResp.Body).Decode(&projection); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if len(projection.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(projection.Turns))
	}
}

func TestHTTPReadyDoesNotExposeInspectionDetails(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	unsafeReason := filepath.Join(testTempDir(t), "models.json") + " secret://provider Authorization: Bearer tokentest123456"
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     unsafeReadinessProvider{name: "sk-secret123", reason: unsafeReason},
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read ready body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d, want 200; body=%s", resp.StatusCode, string(body))
	}
	var ready ReadyResponse
	if err := json.Unmarshal(body, &ready); err != nil {
		t.Fatalf("decode ready response: %v; body=%s", err, string(body))
	}
	if ready.Provider.Name != "provider" || ready.Provider.ReadinessReason != "provider_status_unavailable" {
		t.Fatalf("ready provider = %+v, want sanitized provider status", ready.Provider)
	}
	forbiddenValues := append(pathLeakVariants(ledgerPath), pathLeakVariants(unsafeReason)...)
	forbiddenValues = append(forbiddenValues, "ledger_path", "secret://provider", "tokentest123456", "Authorization", "sk-secret123")
	for _, forbidden := range forbiddenValues {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("ready body = %s, must not contain %q", string(body), forbidden)
		}
	}
}

func TestHTTPTurnSubmitIdempotencyKeyReturnsExistingTurnAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	firstProvider := &countingTextProvider{text: "first answer"}
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     firstProvider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))

	firstResp, err := postJSONWithAuth(server.URL+"/turn", []byte(`{"session_id":"http-turn-idempotency","idempotency_key":"turn-submit-1","input_items":[{"type":"text","text":"first prompt"}]}`))
	if err != nil {
		t.Fatalf("first POST /turn failed: %v", err)
	}
	defer firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusOK {
		t.Fatalf("first turn status = %d, want 200", firstResp.StatusCode)
	}
	var first TurnResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first turn: %v", err)
	}
	if first.Final.Text != "first answer" {
		t.Fatalf("first final = %q, want first answer", first.Final.Text)
	}
	if firstProvider.Calls() != 1 {
		t.Fatalf("first provider calls = %d, want 1", firstProvider.Calls())
	}
	server.Close()

	retryProvider := &countingTextProvider{text: "retry answer should not be used"}
	restarted, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     retryProvider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New restarted returned error: %v", err)
	}
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	retryResp, err := postJSONWithAuth(restartedServer.URL+"/turn", []byte(`{"session_id":"http-turn-idempotency","idempotency_key":"turn-submit-1","input_items":[{"type":"text","text":"retry prompt must not run"}]}`))
	if err != nil {
		t.Fatalf("retry POST /turn failed: %v", err)
	}
	defer retryResp.Body.Close()
	if retryResp.StatusCode != http.StatusOK {
		t.Fatalf("retry turn status = %d, want 200", retryResp.StatusCode)
	}
	var retry TurnResponse
	if err := json.NewDecoder(retryResp.Body).Decode(&retry); err != nil {
		t.Fatalf("decode retry turn: %v", err)
	}
	if retry.TurnID != first.TurnID || retry.Final.Text != "first answer" {
		t.Fatalf("retry = %+v, want original turn id %s and first answer", retry, first.TurnID)
	}
	if retryProvider.Calls() != 0 {
		t.Fatalf("retry provider calls = %d, want 0", retryProvider.Calls())
	}
	projection, err := restarted.Session("http-turn-idempotency")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || len(projection.Events) != 2 {
		t.Fatalf("projection turns/events = %d/%d, want one turn and two events", len(projection.Turns), len(projection.Events))
	}
}

func TestHTTPTurnSubmitIdempotencyKeyReturnsConflictWhileOriginalTurnRuns(t *testing.T) {
	provider := newBlockingProvider()
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"http-turn-idempotent-running","idempotency_key":"turn-running-1","input_items":[{"type":"text","text":"first prompt"}]}`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstReq, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/turn", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build first request: %v", err)
	}
	firstReq.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	firstReq.Header.Set("Content-Type", "application/json")
	firstDone := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(firstReq)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		firstDone <- err
	}()
	provider.waitStarted(t)

	retryResp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("retry POST /turn failed: %v", err)
	}
	defer retryResp.Body.Close()
	assertErrorCode(t, retryResp, http.StatusConflict, "session_active")

	projection, err := k.Session("http-turn-idempotent-running")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if got := countSessionEventType(projection.Events, "turn.submitted"); got != 1 {
		t.Fatalf("turn.submitted count = %d, want original running turn only", got)
	}

	cancel()
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("first turn request did not exit after cancellation")
	}
}

func TestHTTPTurnStreamReportsSessionActiveConflict(t *testing.T) {
	provider := newBlockingProvider()
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     provider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"http-turn-stream-active","idempotency_key":"turn-stream-active-1","input_items":[{"type":"text","text":"first prompt"}]}`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstReq, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/turn", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build first request: %v", err)
	}
	firstReq.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	firstReq.Header.Set("Content-Type", "application/json")
	firstDone := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(firstReq)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		firstDone <- err
	}()
	provider.waitStarted(t)

	streamReq, err := http.NewRequest(http.MethodPost, server.URL+"/turn/stream", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build stream request: %v", err)
	}
	streamReq.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	streamReq.Header.Set("Content-Type", "application/json")
	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("POST /turn/stream failed: %v", err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d, want 200 with typed NDJSON error", streamResp.StatusCode)
	}
	payload, err := io.ReadAll(streamResp.Body)
	if err != nil {
		t.Fatalf("read stream response: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(payload)), "\n")
	if len(lines) != 1 {
		t.Fatalf("stream payload = %q, want one turn_failed event", string(payload))
	}
	var event TurnStreamEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("decode stream event: %v; payload=%s", err, string(payload))
	}
	if event.Type != "turn_failed" || event.Error == nil || event.Error.Code != "session_active" {
		t.Fatalf("stream event = %+v, want turn_failed session_active", event)
	}

	cancel()
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("first turn request did not exit after cancellation")
	}
}

func TestHTTPTurnSubmitIdempotencyKeyReturnsExistingFailureAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     NewBlockedProvider("blocked-test", "no_provider"),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))

	firstResp, err := postJSONWithAuth(server.URL+"/turn", []byte(`{"session_id":"http-turn-idempotent-failure","idempotency_key":"turn-fail-1","input_items":[{"type":"text","text":"first prompt"}]}`))
	if err != nil {
		t.Fatalf("first POST /turn failed: %v", err)
	}
	defer firstResp.Body.Close()
	assertErrorCode(t, firstResp, http.StatusServiceUnavailable, "provider_unavailable")
	server.Close()

	retryProvider := &countingTextProvider{text: "should not recover by retry"}
	restarted, err := New(Config{
		LedgerPath:   ledgerPath,
		Provider:     retryProvider,
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New restarted returned error: %v", err)
	}
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	retryResp, err := postJSONWithAuth(restartedServer.URL+"/turn", []byte(`{"session_id":"http-turn-idempotent-failure","idempotency_key":"turn-fail-1","input_items":[{"type":"text","text":"retry prompt must not run"}]}`))
	if err != nil {
		t.Fatalf("retry POST /turn failed: %v", err)
	}
	defer retryResp.Body.Close()
	if retryResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("retry status = %d, want 503", retryResp.StatusCode)
	}
	var retry map[string]interface{}
	if err := json.NewDecoder(retryResp.Body).Decode(&retry); err != nil {
		t.Fatalf("decode retry turn response: %v", err)
	}
	retryTurnID, _ := retry["turn_id"].(string)
	if retryTurnID == "" {
		t.Fatalf("retry turn_id = %#v, want original failed turn evidence", retry["turn_id"])
	}
	retryError, ok := retry["error"].(map[string]interface{})
	if !ok || retryError["code"] != "provider_unavailable" {
		t.Fatalf("retry error = %#v, want provider_unavailable turn error", retry["error"])
	}
	retryEvents, ok := retry["events"].([]interface{})
	if !ok || len(retryEvents) != 3 {
		t.Fatalf("retry events = %#v, want original submitted, provider attempt, and failed events", retry["events"])
	}
	attemptEvent, ok := retryEvents[1].(map[string]interface{})
	if !ok || attemptEvent["type"] != "model.provider_attempt" {
		t.Fatalf("retry attempt event = %#v, want model.provider_attempt", retryEvents[1])
	}
	lastEvent, ok := retryEvents[2].(map[string]interface{})
	if !ok || lastEvent["type"] != "turn.failed" {
		t.Fatalf("retry last event = %#v, want turn.failed", retryEvents[2])
	}
	if retryProvider.Calls() != 0 {
		t.Fatalf("retry provider calls = %d, want 0", retryProvider.Calls())
	}
	projection, err := restarted.Session("http-turn-idempotent-failure")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || projection.Turns[0].Phase != RuntimePhaseEnded || projection.Turns[0].TerminalOutcome != TerminalOutcomeFailed || len(projection.Events) != 3 {
		t.Fatalf("projection = %+v, want original failed turn only", projection)
	}
}

func TestHTTPTurnSubmitIdempotencyKeyRequiresValidExplicitSession(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	missingSession, err := postJSONWithAuth(server.URL+"/turn", []byte(`{"idempotency_key":"turn-no-session","input_items":[{"type":"text","text":"hello"}]}`))
	if err != nil {
		t.Fatalf("POST /turn without session failed: %v", err)
	}
	defer missingSession.Body.Close()
	assertErrorCode(t, missingSession, http.StatusBadRequest, "invalid_request")

	badKey, err := postJSONWithAuth(server.URL+"/turn", []byte(`{"session_id":"bad-turn-key","idempotency_key":"bad key","input_items":[{"type":"text","text":"hello"}]}`))
	if err != nil {
		t.Fatalf("POST /turn with bad key failed: %v", err)
	}
	defer badKey.Body.Close()
	assertErrorCode(t, badKey, http.StatusBadRequest, "invalid_request")

	if _, err := k.Session("bad-turn-key"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Session after bad turn key error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPFinalUsageSummarySurvivesSessionReplay(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"served-model","choices":[{"message":{"role":"assistant","content":"usage answer"}}],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`))
	}))
	defer providerServer.Close()

	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k, err := New(Config{
		LedgerPath: ledgerPath,
		Provider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
			BaseURL: providerServer.URL,
			APIKey:  "test-key",
			Model:   "test-model",
		}),
		RuntimeToken: testRuntimeToken,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	server := httptest.NewServer(Handler(k))

	body := []byte(`{"session_id":"http-usage-session","input_items":[{"type":"text","text":"hello usage"}]}`)
	turnResp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	if turnResp.StatusCode != http.StatusOK {
		t.Fatalf("turn status = %d, want 200", turnResp.StatusCode)
	}
	var turn map[string]interface{}
	if err := json.NewDecoder(turnResp.Body).Decode(&turn); err != nil {
		t.Fatalf("decode turn response: %v", err)
	}
	assertJSONUsage(t, turn["final"], 11, 7, 18)
	server.Close()

	restarted := newTestKernelWithRuntimeToken(t, ledgerPath, testRuntimeToken)
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	sessionResp, err := getWithAuth(restartedServer.URL + "/sessions/http-usage-session")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer sessionResp.Body.Close()
	if sessionResp.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want 200", sessionResp.StatusCode)
	}
	var session map[string]interface{}
	if err := json.NewDecoder(sessionResp.Body).Decode(&session); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	turns, ok := session["turns"].([]interface{})
	if !ok || len(turns) != 1 {
		t.Fatalf("turns = %#v, want one turn", session["turns"])
	}
	turnProjection, ok := turns[0].(map[string]interface{})
	if !ok {
		t.Fatalf("turn projection = %#v", turns[0])
	}
	assertJSONUsage(t, turnProjection["final"], 11, 7, 18)
}

func TestHTTPTurnEventsAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))

	body := []byte(`{"session_id":"http-turn-events","input_items":[{"type":"text","text":"hello events"}]}`)
	turnResp, err := postJSONWithAuth(server.URL+"/turn", body)
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
	server.Close()

	restarted := newTestKernel(t, ledgerPath)
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	eventsResp, err := getWithAuth(restartedServer.URL + "/turns/" + turn.TurnID + "/events")
	if err != nil {
		t.Fatalf("GET /turns/{id}/events failed: %v", err)
	}
	defer eventsResp.Body.Close()
	if eventsResp.StatusCode != http.StatusOK {
		t.Fatalf("events status = %d, want 200", eventsResp.StatusCode)
	}
	var events struct {
		Items []Event `json:"items"`
	}
	if err := json.NewDecoder(eventsResp.Body).Decode(&events); err != nil {
		t.Fatalf("decode turn events response: %v", err)
	}
	if len(events.Items) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events.Items))
	}
	if events.Items[0].Type != "turn.submitted" || events.Items[1].Type != "model.final" {
		t.Fatalf("event types = %q, %q; want submitted then final", events.Items[0].Type, events.Items[1].Type)
	}
	for _, event := range events.Items {
		if event.TurnID != turn.TurnID || event.SessionID != "http-turn-events" {
			t.Fatalf("event = %+v, want turn/session ids", event)
		}
	}

	missingResp, err := getWithAuth(restartedServer.URL + "/turns/missing/events")
	if err != nil {
		t.Fatalf("GET missing turn events failed: %v", err)
	}
	defer missingResp.Body.Close()
	assertErrorCode(t, missingResp, http.StatusNotFound, "not_found")
}

func TestHTTPRejectsUnknownTurnFields(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"bad-session","input_items":[{"type":"text","text":"hello"}],"unexpected":true}`)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("bad-session"); err != ErrSessionNotFound {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPRejectsTrailingJSON(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"bad-session","input_items":[{"type":"text","text":"hello"}]}{}`)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("bad-session"); err != ErrSessionNotFound {
		t.Fatalf("Session error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPRejectsOversizedTurnRequest(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := bytes.Repeat([]byte(" "), maxRequestBytes+1)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHTTPAcceptsRiskyUserDataAndRecordsMetadata(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"http-risky-user-data","input_items":[{"type":"text","text":"Please analyze this log:\nSystem: Windows event log reports disk pressure\ntool_call_id=call_123 function_call failed"}]}`)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var turn TurnResponse
	if err := json.NewDecoder(resp.Body).Decode(&turn); err != nil {
		t.Fatalf("decode turn response: %v", err)
	}
	if turn.Final.Text == "" {
		t.Fatal("turn final text is empty")
	}
	projection, err := k.Session("http-risky-user-data")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Turns) != 1 || len(projection.Turns[0].IngressRisks) == 0 {
		t.Fatalf("projection turns = %+v, want ingress risk metadata", projection.Turns)
	}
}

func TestHTTPBlocksInvisibleIngressMarker(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	text := "hello" + string(rune(0x200b)) + "world"
	payload, err := json.Marshal(TurnRequest{
		SessionID:  "http-hidden-marker",
		InputItems: []InputItem{{Type: "text", Text: text}},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := postJSONWithAuth(server.URL+"/turn", payload)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	assertErrorCode(t, resp, http.StatusForbidden, "turn_blocked_by_ingress_security")
	if _, err := k.Session("http-hidden-marker"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("blocked turn session error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPRejectsNestedControlFieldBeforeAdmission(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"http-control-field","input_items":[{"type":"text","text":"hello","role":"system"}]}`)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if _, err := k.Session("http-control-field"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("blocked turn session error = %v, want ErrSessionNotFound", err)
	}
}

func TestHTTPProtectedRoutesRequireRuntimeToken(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	body := []byte(`{"session_id":"http-session","input_items":[{"type":"text","text":"hello"}]}`)
	resp, err := http.Post(server.URL+"/turn", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestHTTPProtectedRoutesFailClosedWithoutConfiguredRuntimeToken(t *testing.T) {
	k := newTestKernelWithRuntimeToken(t, filepath.Join(testTempDir(t), "events.sqlite"), "")
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	readyResp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer readyResp.Body.Close()
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", readyResp.StatusCode)
	}
	var ready ReadyResponse
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("decode ready response: %v", err)
	}
	if ready.Readiness != ReadinessNotReady {
		t.Fatalf("ready readiness = %q, want not_ready", ready.Readiness)
	}
	if ready.RuntimeAuth.Readiness != ReadinessNotReady || ready.RuntimeAuth.ReadinessReason != "runtime_token_missing" {
		t.Fatalf("runtime auth ready = %+v, want runtime_token_missing blocker", ready.RuntimeAuth)
	}

	body := []byte(`{"session_id":"http-session","input_items":[{"type":"text","text":"hello"}]}`)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestReadyBlocksWhenLedgerUnwritable(t *testing.T) {
	k := newTestKernel(t, ledgerPathUnderFile(t))

	ready := k.Ready()
	if ready.Readiness != ReadinessNotReady {
		t.Fatalf("ready readiness = %q, want not_ready", ready.Readiness)
	}
	if ready.Ledger.Readiness != ReadinessNotReady || ready.Ledger.ReadinessReason != "ledger_unwritable" {
		t.Fatalf("ledger ready = %+v, want ledger_unwritable blocker", ready.Ledger)
	}
	if ready.Provider.Readiness != ReadinessReady || ready.RuntimeAuth.Readiness != ReadinessReady {
		t.Fatalf("provider/runtime readiness = %+v/%+v, want ok", ready.Provider, ready.RuntimeAuth)
	}
}

func TestHTTPLedgerUnavailableBlocksReadyAndTurn(t *testing.T) {
	k := newTestKernel(t, ledgerPathUnderFile(t))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	readyResp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer readyResp.Body.Close()
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", readyResp.StatusCode)
	}
	var ready ReadyResponse
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("decode ready response: %v", err)
	}
	if ready.Readiness != ReadinessNotReady || ready.Ledger.Readiness != ReadinessNotReady || ready.Ledger.ReadinessReason != "ledger_unwritable" {
		t.Fatalf("ready = %+v, want ledger_unwritable blocker", ready)
	}

	body := []byte(`{"session_id":"ledger-bad","input_items":[{"type":"text","text":"hello"}]}`)
	resp, err := postJSONWithAuth(server.URL+"/turn", body)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	var envelope errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if envelope.Error.Code != "ledger_unwritable" {
		t.Fatalf("error code = %q, want ledger_unwritable", envelope.Error.Code)
	}
}

func TestHTTPCorruptLedgerBlocksReadyReplayAndAppend(t *testing.T) {
	k := newTestKernel(t, corruptLedgerPath(t))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	readyResp, err := http.Get(server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer readyResp.Body.Close()
	var ready ReadyResponse
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("decode ready response: %v", err)
	}
	if ready.Readiness != ReadinessNotReady || ready.Ledger.Readiness != ReadinessNotReady || ready.Ledger.ReadinessReason != "ledger_corrupt" {
		t.Fatalf("ready = %+v, want ledger_corrupt blocker", ready)
	}

	turnBody := []byte(`{"session_id":"corrupt-ledger","input_items":[{"type":"text","text":"hello"}]}`)
	turnResp, err := postJSONWithAuth(server.URL+"/turn", turnBody)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer turnResp.Body.Close()
	assertErrorCode(t, turnResp, http.StatusServiceUnavailable, "ledger_corrupt")

	sessionResp, err := getWithAuth(server.URL + "/sessions/corrupt-ledger")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer sessionResp.Body.Close()
	assertErrorCode(t, sessionResp, http.StatusServiceUnavailable, "ledger_corrupt")

	memoryBody := []byte(`{"session_id":"corrupt-ledger","text":"remember this","source_ref":"turn:corrupt-ledger"}`)
	memoryResp, err := postJSONWithAuth(server.URL+"/memory/candidates", memoryBody)
	if err != nil {
		t.Fatalf("POST /memory/candidates failed: %v", err)
	}
	defer memoryResp.Body.Close()
	assertErrorCode(t, memoryResp, http.StatusServiceUnavailable, "ledger_corrupt")
}

func TestHTTPRejectsNonJSONContentType(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL+"/turn", strings.NewReader(`{"input_items":[{"type":"text","text":"hello"}]}`))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testRuntimeToken)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /turn failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", resp.StatusCode)
	}
}
