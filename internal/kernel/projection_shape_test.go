package kernel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicProjectionArraysMarshalAsNonNullArrays(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.jsonl"))
	resp, err := k.SubmitTurn(context.Background(), TurnRequest{
		SessionID: "projection-array-session",
		InputItems: []InputItem{{
			Type: "text",
			Text: "check projection array shape",
		}},
	})
	if err != nil {
		t.Fatalf("SubmitTurn returned error: %v", err)
	}

	session, err := k.Session(resp.SessionID)
	if err != nil {
		t.Fatalf("Session returned error: %v", err)
	}
	assertJSONArrays(t, "session", session,
		"turns", "operations", "jobs", "works", "memory_candidates", "events",
	)

	timeline, err := k.UITimeline(resp.SessionID)
	if err != nil {
		t.Fatalf("UITimeline returned error: %v", err)
	}
	assertJSONArrays(t, "timeline", timeline, "items")
	assertTimelineChildrenAreArrays(t, timeline.Items)

	contextInspection, err := k.ContextInspection(resp.TurnID)
	if err != nil {
		t.Fatalf("ContextInspection returned error: %v", err)
	}
	assertJSONArrays(t, "context inspection", contextInspection,
		"input_items", "model_input_kinds", "tool_manifest", "skill_catalog", "recalled_memories",
	)

	audit, err := k.AuditReplay(resp.TurnID)
	if err != nil {
		t.Fatalf("AuditReplay returned error: %v", err)
	}
	assertJSONArrays(t, "audit replay", audit, "items")

	events, err := k.TurnEvents(resp.TurnID)
	if err != nil {
		t.Fatalf("TurnEvents returned error: %v", err)
	}
	assertJSONArrays(t, "turn events", TurnEventsResponse{Items: events}, "items")

	capabilities := k.Capabilities()
	assertJSONArrays(t, "capabilities", capabilities, "tools")
	assertJSONArrays(t, "skill catalog", capabilities.SkillCatalog, "items", "exclusions")

	candidates, err := k.MemoryCandidates("")
	if err != nil {
		t.Fatalf("MemoryCandidates returned error: %v", err)
	}
	assertJSONArrays(t, "memory candidates", MemoryCandidateListResponse{Items: candidates}, "items")

	recalls, err := k.RecallMemories(MemoryRecallRequest{InputItems: []InputItem{{
		Type: "text",
		Text: "nothing approved here",
	}}})
	if err != nil {
		t.Fatalf("RecallMemories returned error: %v", err)
	}
	assertJSONArrays(t, "memory recall", recalls, "items")
}

func assertJSONArrays(t *testing.T, name string, payload any, fields ...string) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode %s object: %v", name, err)
	}
	for _, field := range fields {
		value, ok := object[field]
		if !ok {
			t.Fatalf("%s missing array field %q in %s", name, field, string(raw))
		}
		trimmed := strings.TrimSpace(string(value))
		if trimmed == "null" {
			t.Fatalf("%s array field %q encoded as null in %s", name, field, string(raw))
		}
		if !strings.HasPrefix(trimmed, "[") {
			t.Fatalf("%s field %q encoded as %s, want JSON array in %s", name, field, trimmed, string(raw))
		}
	}
}

func assertTimelineChildrenAreArrays(t *testing.T, items []UITimelineItem) {
	t.Helper()
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("marshal timeline item %q: %v", item.ItemID, err)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			t.Fatalf("decode timeline item %q: %v", item.ItemID, err)
		}
		value, ok := object["children"]
		if !ok {
			t.Fatalf("timeline item %q missing children array in %s", item.ItemID, string(raw))
		}
		trimmed := strings.TrimSpace(string(value))
		if trimmed == "null" || !strings.HasPrefix(trimmed, "[") {
			t.Fatalf("timeline item %q children encoded as %s, want JSON array in %s", item.ItemID, trimmed, string(raw))
		}
		assertTimelineChildrenAreArrays(t, item.Children)
	}
}
