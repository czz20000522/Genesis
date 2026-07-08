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

func TestHTTPSessionSearchValidatesQueryAndPreservesSessionList(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	appendSearchSessionEvents(t, k)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	missingQuery, err := getWithAuth(server.URL + "/sessions/search")
	if err != nil {
		t.Fatalf("GET /sessions/search failed: %v", err)
	}
	assertErrorCode(t, missingQuery, http.StatusBadRequest, "invalid_request")

	emptyQuery, err := getWithAuth(server.URL + "/sessions/search?q=%20%20")
	if err != nil {
		t.Fatalf("GET /sessions/search?q failed: %v", err)
	}
	assertErrorCode(t, emptyQuery, http.StatusBadRequest, "invalid_request")

	invalidLimit, err := getWithAuth(server.URL + "/sessions/search?q=basalt&limit=bogus")
	if err != nil {
		t.Fatalf("GET /sessions/search invalid limit failed: %v", err)
	}
	assertErrorCode(t, invalidLimit, http.StatusBadRequest, "invalid_request")

	limitTooLarge, err := getWithAuth(server.URL + "/sessions/search?q=basalt&limit=101")
	if err != nil {
		t.Fatalf("GET /sessions/search large limit failed: %v", err)
	}
	assertErrorCode(t, limitTooLarge, http.StatusBadRequest, "invalid_request")

	noMatch, err := getWithAuth(server.URL + "/sessions/search?q=zircon")
	if err != nil {
		t.Fatalf("GET /sessions/search no match failed: %v", err)
	}
	defer noMatch.Body.Close()
	if noMatch.StatusCode != http.StatusOK {
		t.Fatalf("no-match search status = %d, want 200", noMatch.StatusCode)
	}
	var noMatchPayload SessionSearchResponse
	if err := json.NewDecoder(noMatch.Body).Decode(&noMatchPayload); err != nil {
		t.Fatalf("decode no-match search response: %v", err)
	}
	if noMatchPayload.Query != "zircon" || len(noMatchPayload.Items) != 0 {
		t.Fatalf("no-match payload = %+v, want normalized query and empty items", noMatchPayload)
	}

	listResp, err := getWithAuth(server.URL + "/sessions")
	if err != nil {
		t.Fatalf("GET /sessions failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("session list status = %d, want 200", listResp.StatusCode)
	}
}

func TestHTTPSessionSearchMatchesProjectionFieldsAndOmitsControlIDs(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	appendSearchSessionEvents(t, k)
	server := httptest.NewServer(Handler(k))
	defer server.Close()

	resp, err := getWithAuth(server.URL + "/sessions/search?q=basalt")
	if err != nil {
		t.Fatalf("GET /sessions/search failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search status = %d, want 200", resp.StatusCode)
	}
	var payload SessionSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if payload.Query != "basalt" {
		t.Fatalf("query = %q, want normalized query", payload.Query)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("items = %+v, want one basalt match", payload.Items)
	}
	item := payload.Items[0]
	if item.SessionID != "session-basalt" || item.Title == "" || item.Snippet == "" {
		t.Fatalf("item = %+v, want basalt session metadata and snippet", item)
	}
	if !hasSearchMatchField(item.MatchFields, "title") && !hasSearchMatchField(item.MatchFields, "user_text") && !hasSearchMatchField(item.MatchFields, "assistant_text") {
		t.Fatalf("match fields = %+v, want semantic search field", item.MatchFields)
	}
	for _, tc := range []struct {
		name      string
		query     string
		field     string
		sessionID string
	}{
		{name: "session id", query: "session-compaction", field: "session_id", sessionID: "session-compaction"},
		{name: "title", query: "roadmap", field: "title", sessionID: "session-basalt"},
		{name: "first user text", query: "notes", field: "user_text", sessionID: "session-basalt"},
		{name: "final assistant text", query: "stable", field: "assistant_text", sessionID: "session-compaction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match := searchSessionsForTest(t, server.URL, tc.query)
			if len(match.Items) == 0 {
				t.Fatalf("items = %+v, want at least one match", match.Items)
			}
			item := match.Items[0]
			if item.SessionID != tc.sessionID {
				t.Fatalf("session id = %q, want %q", item.SessionID, tc.sessionID)
			}
			if !hasSearchMatchField(item.MatchFields, tc.field) {
				t.Fatalf("match fields = %+v, want %q", item.MatchFields, tc.field)
			}
		})
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal search response: %v", err)
	}
	for _, forbidden := range []string{"evt_search_basalt_user", "turn-search-basalt", "operation-search-basalt", "job-search-basalt"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("search response leaked control id %q: %s", forbidden, string(encoded))
		}
	}
}

func TestSessionSearchStableAfterRestart(t *testing.T) {
	ledgerPath := filepath.Join(testTempDir(t), "events.sqlite")
	firstKernel := newTestKernel(t, ledgerPath)
	appendSearchSessionEvents(t, firstKernel)
	firstKernel.Close()

	restarted := newTestKernel(t, ledgerPath)
	result, err := restarted.SearchSessions(SessionSearchRequest{Query: "compaction", Limit: 5})
	if err != nil {
		t.Fatalf("SearchSessions returned error: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].SessionID != "session-compaction" {
		t.Fatalf("items = %+v, want compaction session after restart", result.Items)
	}
}

func searchSessionsForTest(t *testing.T, serverURL string, query string) SessionSearchResponse {
	t.Helper()
	resp, err := getWithAuth(serverURL + "/sessions/search?q=" + query)
	if err != nil {
		t.Fatalf("GET /sessions/search %q failed: %v", query, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search %q status = %d, want 200", query, resp.StatusCode)
	}
	var payload SessionSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode search %q response: %v", query, err)
	}
	return payload
}

func appendSearchSessionEvents(t *testing.T, k *Kernel) {
	t.Helper()
	base := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	for _, event := range []StoredEvent{
		{
			EventID:     "evt_search_basalt_user",
			SessionID:   "session-basalt",
			TurnID:      "turn-search-basalt",
			OperationID: "operation-search-basalt",
			JobID:       "job-search-basalt",
			Type:        "turn.submitted",
			CreatedAt:   base,
			Data: EventData{InputItems: []InputItem{{
				Type: "text",
				Text: "Basalt roadmap notes",
			}}},
		},
		{
			EventID:   "evt_search_basalt_final",
			SessionID: "session-basalt",
			TurnID:    "turn-search-basalt",
			Type:      "model.final",
			CreatedAt: base.Add(time.Minute),
			Data:      EventData{Final: &FinalMessage{Text: "The basalt roadmap is ready."}},
		},
		{
			EventID:   "evt_search_compaction_user",
			SessionID: "session-compaction",
			TurnID:    "turn-search-compaction",
			Type:      "turn.submitted",
			CreatedAt: base.Add(2 * time.Minute),
			Data: EventData{InputItems: []InputItem{{
				Type: "text",
				Text: "Context compaction regression",
			}}},
		},
		{
			EventID:   "evt_search_compaction_final",
			SessionID: "session-compaction",
			TurnID:    "turn-search-compaction",
			Type:      "model.final",
			CreatedAt: base.Add(3 * time.Minute),
			Data:      EventData{Final: &FinalMessage{Text: "Compaction remains stable."}},
		},
	} {
		if err := k.appendEvent(event); err != nil {
			t.Fatalf("append event %s: %v", event.EventID, err)
		}
	}
}

func hasSearchMatchField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}
