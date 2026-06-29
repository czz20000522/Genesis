package kernel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTTPWorkSubmitCancelReadAndSessionProjectionAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))

	createResp, err := postJSONWithAuth(server.URL+"/work", []byte(`{"session_id":"http-work-source","title":"Draft migration plan","source_ref":"turn:http-work-source"}`))
	if err != nil {
		t.Fatalf("POST /work failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create work status = %d, want 200", createResp.StatusCode)
	}
	var created map[string]interface{}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created work: %v", err)
	}
	workID, _ := created["work_id"].(string)
	if workID == "" || created["status"] != "open" || created["source_ref"] != "turn:http-work-source" {
		t.Fatalf("created work = %#v, want open work with source ref", created)
	}
	server.Close()

	restarted := newTestKernel(t, ledgerPath)
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	readResp, err := getWithAuth(restartedServer.URL + "/work/" + workID)
	if err != nil {
		t.Fatalf("GET /work/{id} failed: %v", err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("read work status = %d, want 200", readResp.StatusCode)
	}
	var readBack map[string]interface{}
	if err := json.NewDecoder(readResp.Body).Decode(&readBack); err != nil {
		t.Fatalf("decode read work: %v", err)
	}
	if readBack["status"] != "open" || readBack["title"] != "Draft migration plan" {
		t.Fatalf("read work = %#v, want restart-safe open work", readBack)
	}

	cancelResp, err := postJSONWithAuth(restartedServer.URL+"/work/"+workID+"/cancel", []byte(`{"cancel_authority":"runtime:test","cancel_reason":"operator stopped it","cancel_evidence_ref":"review:work-cancel"}`))
	if err != nil {
		t.Fatalf("POST /work/{id}/cancel failed: %v", err)
	}
	defer cancelResp.Body.Close()
	if cancelResp.StatusCode != http.StatusOK {
		t.Fatalf("cancel work status = %d, want 200", cancelResp.StatusCode)
	}
	var canceled map[string]interface{}
	if err := json.NewDecoder(cancelResp.Body).Decode(&canceled); err != nil {
		t.Fatalf("decode canceled work: %v", err)
	}
	if canceled["status"] != "canceled" || canceled["cancel_evidence_ref"] != "review:work-cancel" {
		t.Fatalf("canceled work = %#v, want canceled evidence", canceled)
	}

	secondRestart := newTestKernel(t, ledgerPath)
	secondServer := httptest.NewServer(Handler(secondRestart))
	defer secondServer.Close()

	sessionResp, err := getWithAuth(secondServer.URL + "/sessions/http-work-source")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer sessionResp.Body.Close()
	if sessionResp.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want 200", sessionResp.StatusCode)
	}
	var session map[string]interface{}
	if err := json.NewDecoder(sessionResp.Body).Decode(&session); err != nil {
		t.Fatalf("decode session projection: %v", err)
	}
	works, ok := session["works"].([]interface{})
	if !ok || len(works) != 1 {
		t.Fatalf("session works = %#v, want one work projection", session["works"])
	}
	sessionWork, ok := works[0].(map[string]interface{})
	if !ok || sessionWork["work_id"] != workID || sessionWork["status"] != "canceled" || sessionWork["cancel_evidence_ref"] != "review:work-cancel" {
		t.Fatalf("session work = %#v, want canceled work projection", works[0])
	}
}

func TestHTTPCancelWorkIsIdempotentWithoutOverwritingEvidence(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	createResp, err := postJSONWithAuth(server.URL+"/work", []byte(`{"session_id":"http-work-duplicate-cancel","title":"Keep original cancel evidence","source_ref":"turn:http-work-duplicate-cancel"}`))
	if err != nil {
		t.Fatalf("POST /work failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create work status = %d, want 200", createResp.StatusCode)
	}
	var created map[string]interface{}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created work: %v", err)
	}
	workID, _ := created["work_id"].(string)
	if workID == "" {
		t.Fatalf("created work = %#v, want work_id", created)
	}

	firstCancel, err := postJSONWithAuth(server.URL+"/work/"+workID+"/cancel", []byte(`{"cancel_authority":"runtime:test","cancel_reason":"first reason","cancel_evidence_ref":"review:first-cancel"}`))
	if err != nil {
		t.Fatalf("first POST cancel failed: %v", err)
	}
	firstCancel.Body.Close()
	if firstCancel.StatusCode != http.StatusOK {
		t.Fatalf("first cancel status = %d, want 200", firstCancel.StatusCode)
	}
	secondCancel, err := postJSONWithAuth(server.URL+"/work/"+workID+"/cancel", []byte(`{"cancel_authority":"runtime:test","cancel_reason":"second reason","cancel_evidence_ref":"review:second-cancel"}`))
	if err != nil {
		t.Fatalf("second POST cancel failed: %v", err)
	}
	defer secondCancel.Body.Close()
	if secondCancel.StatusCode != http.StatusOK {
		t.Fatalf("second cancel status = %d, want 200", secondCancel.StatusCode)
	}
	var second map[string]interface{}
	if err := json.NewDecoder(secondCancel.Body).Decode(&second); err != nil {
		t.Fatalf("decode second cancel: %v", err)
	}
	if second["cancel_evidence_ref"] != "review:first-cancel" {
		t.Fatalf("second cancel evidence = %#v, want original evidence", second["cancel_evidence_ref"])
	}

	projection, err := k.Session("http-work-duplicate-cancel")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	cancelEvents := 0
	for _, event := range projection.Events {
		if event.Type == "work.canceled" {
			cancelEvents++
		}
	}
	if cancelEvents != 1 {
		t.Fatalf("cancel event count = %d, want 1", cancelEvents)
	}
}

func TestHTTPWorkSubmitIdempotencyKeyReturnsExistingWorkAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	k := newTestKernel(t, ledgerPath)
	server := httptest.NewServer(Handler(k))

	createResp, err := postJSONWithAuth(server.URL+"/work", []byte(`{"session_id":"http-work-submit-idempotency","title":"first title","source_ref":"turn:http-work-submit-idempotency","idempotency_key":"work-submit-1"}`))
	if err != nil {
		t.Fatalf("first POST /work failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("first create status = %d, want 200", createResp.StatusCode)
	}
	var first WorkProjection
	if err := json.NewDecoder(createResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first work: %v", err)
	}
	if first.WorkID == "" || first.IdempotencyKey != "work-submit-1" {
		t.Fatalf("first work = %#v, want work id and idempotency key", first)
	}
	server.Close()

	restarted := newTestKernel(t, ledgerPath)
	restartedServer := httptest.NewServer(Handler(restarted))
	defer restartedServer.Close()

	retryResp, err := postJSONWithAuth(restartedServer.URL+"/work", []byte(`{"session_id":"http-work-submit-idempotency","title":"retry title must not replace","source_ref":"turn:http-work-submit-idempotency-retry","idempotency_key":"work-submit-1"}`))
	if err != nil {
		t.Fatalf("retry POST /work failed: %v", err)
	}
	defer retryResp.Body.Close()
	if retryResp.StatusCode != http.StatusOK {
		t.Fatalf("retry create status = %d, want 200", retryResp.StatusCode)
	}
	var retry WorkProjection
	if err := json.NewDecoder(retryResp.Body).Decode(&retry); err != nil {
		t.Fatalf("decode retry work: %v", err)
	}
	if retry.WorkID != first.WorkID || retry.Title != first.Title || retry.SourceRef != first.SourceRef {
		t.Fatalf("retry work = %#v, want original %#v", retry, first)
	}

	projection, err := restarted.Session("http-work-submit-idempotency")
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	if len(projection.Works) != 1 {
		t.Fatalf("projected works = %+v, want one work", projection.Works)
	}
	submitEvents := 0
	for _, event := range projection.Events {
		if event.Type == "work.submitted" {
			submitEvents++
		}
	}
	if submitEvents != 1 {
		t.Fatalf("submit event count = %d, want 1", submitEvents)
	}
}

func TestWorkReplayRejectsCompetingCancelEvidence(t *testing.T) {
	createdAt := time.Date(2026, 6, 22, 2, 0, 0, 0, time.UTC)
	firstCanceledAt := createdAt.Add(time.Minute)
	secondCanceledAt := createdAt.Add(2 * time.Minute)
	submitted := WorkProjection{
		WorkID:    "work-competing-cancel",
		SessionID: "work-competing-cancel-session",
		Title:     "competing cancel",
		SourceRef: "turn:work-competing-cancel-session",
		Status:    WorkStatusOpen,
		CreatedAt: createdAt,
	}
	firstCancel := submitted
	firstCancel.Status = WorkStatusCanceled
	firstCancel.CancelAuthority = "runtime:first"
	firstCancel.CancelReason = "first reason"
	firstCancel.CancelEvidenceRef = "review:first"
	firstCancel.CanceledAt = &firstCanceledAt
	secondCancel := submitted
	secondCancel.Status = WorkStatusCanceled
	secondCancel.CancelAuthority = "runtime:second"
	secondCancel.CancelReason = "second reason"
	secondCancel.CancelEvidenceRef = "review:second"
	secondCancel.CanceledAt = &secondCanceledAt

	k := &Kernel{
		ledger: newStaticLedger(
			StoredEvent{
				EventID:   "evt-work-submit",
				SessionID: submitted.SessionID,
				WorkID:    submitted.WorkID,
				Type:      "work.submitted",
				CreatedAt: createdAt,
				Data:      EventData{Work: &submitted},
			},
			StoredEvent{
				EventID:   "evt-work-cancel-first",
				SessionID: submitted.SessionID,
				WorkID:    submitted.WorkID,
				Type:      "work.canceled",
				CreatedAt: firstCanceledAt,
				Data:      EventData{Work: &firstCancel},
			},
			StoredEvent{
				EventID:   "evt-work-cancel-second",
				SessionID: submitted.SessionID,
				WorkID:    submitted.WorkID,
				Type:      "work.canceled",
				CreatedAt: secondCanceledAt,
				Data:      EventData{Work: &secondCancel},
			},
		),
		provider:     FakeProvider{},
		runtimeToken: testRuntimeToken,
		toolPolicy:   normalizedToolPolicy(ToolPolicy{}),
		clock:        time.Now,
	}

	if _, err := k.Work(submitted.WorkID); err == nil || !strings.Contains(err.Error(), "competing work cancel evidence") {
		t.Fatalf("Work error = %v, want competing cancel evidence error", err)
	}
	if _, err := k.Session(submitted.SessionID); err == nil || !strings.Contains(err.Error(), "competing work cancel evidence") {
		t.Fatalf("Session error = %v, want competing cancel evidence error", err)
	}
}

func TestConcurrentWorkCancelWritesOnlyOneTerminalDecision(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	work, err := k.SubmitWork(WorkSubmitRequest{
		SessionID: "work-cancel-race",
		Title:     "race cancel",
		SourceRef: "turn:work-cancel-race",
	})
	if err != nil {
		t.Fatalf("SubmitWork returned error: %v", err)
	}

	type result struct {
		work WorkProjection
		err  error
	}
	results := make(chan result, 2)
	go func() {
		canceled, err := k.CancelWork(work.WorkID, WorkCancelRequest{
			CancelAuthority:   "runtime:first",
			CancelReason:      "first reason",
			CancelEvidenceRef: "review:first-cancel",
		})
		results <- result{work: canceled, err: err}
	}()
	go func() {
		canceled, err := k.CancelWork(work.WorkID, WorkCancelRequest{
			CancelAuthority:   "runtime:second",
			CancelReason:      "second reason",
			CancelEvidenceRef: "review:second-cancel",
		})
		results <- result{work: canceled, err: err}
	}()

	first := <-results
	second := <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("CancelWork errors = %v, %v; want both callers to observe the terminal work", first.err, second.err)
	}
	if first.work.CancelEvidenceRef != second.work.CancelEvidenceRef {
		t.Fatalf("cancel evidence refs = %q and %q, want both callers to observe one terminal decision", first.work.CancelEvidenceRef, second.work.CancelEvidenceRef)
	}

	events, err := k.loadEvents()
	if err != nil {
		t.Fatalf("loadEvents returned error: %v", err)
	}
	cancelEvents := 0
	for _, event := range events {
		if event.Type == "work.canceled" && event.WorkID == work.WorkID {
			cancelEvents++
		}
	}
	if cancelEvents != 1 {
		t.Fatalf("cancel event count = %d, want 1", cancelEvents)
	}
}

func TestHTTPCancelWorkRejectsInvalidControlRefs(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	createResp, err := postJSONWithAuth(server.URL+"/work", []byte(`{"session_id":"bad-work-cancel-audit","title":"cancel audit","source_ref":"turn:bad-work-cancel-audit"}`))
	if err != nil {
		t.Fatalf("POST /work failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d, want 200", createResp.StatusCode)
	}
	var created map[string]interface{}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created work: %v", err)
	}
	workID, _ := created["work_id"].(string)
	if workID == "" {
		t.Fatalf("created work = %#v, want work_id", created)
	}

	for name, body := range map[string][]byte{
		"invalid authority":       []byte(`{"cancel_authority":"root","cancel_reason":"bad authority","cancel_evidence_ref":"review:bad-authority"}`),
		"invalid evidence ref":    []byte(`{"cancel_authority":"runtime:test","cancel_reason":"bad evidence","cancel_evidence_ref":"free text"}`),
		"secret evidence ref":     []byte(`{"cancel_authority":"runtime:test","cancel_reason":"bad secret evidence","cancel_evidence_ref":"review:api_key=sk-work-secret"}`),
		"secret cancel authority": []byte(`{"cancel_authority":"runtime:api_key=sk-work-secret","cancel_reason":"bad secret authority","cancel_evidence_ref":"review:secret-authority"}`),
	} {
		resp, err := postJSONWithAuth(server.URL+"/work/"+workID+"/cancel", body)
		if err != nil {
			t.Fatalf("%s: POST cancel failed: %v", name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}
}

func TestHTTPCreateWorkRejectsInvalidIdempotencyKey(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/work", []byte(`{"session_id":"bad-work-key","title":"bad key","source_ref":"turn:bad-work-key","idempotency_key":"bad key"}`))
	if err != nil {
		t.Fatalf("POST /work failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHTTPCreateWorkRequiresSourceRef(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := postJSONWithAuth(server.URL+"/work", []byte(`{"session_id":"bad-work","title":"missing source"}`))
	if err != nil {
		t.Fatalf("POST /work failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestHTTPCreateWorkRejectsInvalidControlRefs(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	for name, body := range map[string][]byte{
		"invalid source ref": []byte(`{"session_id":"bad-work-ref","title":"bad source","source_ref":"free text"}`),
		"secret session id":  []byte(`{"session_id":"api_key=sk-work-secret","title":"bad session secret","source_ref":"turn:bad-work-secret-session"}`),
		"secret source ref":  []byte(`{"session_id":"bad-work-secret-ref","title":"bad source secret","source_ref":"turn:api_key=sk-work-secret"}`),
	} {
		resp, err := postJSONWithAuth(server.URL+"/work", body)
		if err != nil {
			t.Fatalf("%s: POST /work failed: %v", name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}
}
