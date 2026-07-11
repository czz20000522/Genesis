package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"genesis/internal/applications/connector_runtime"
)

func TestFeishuSourceAdapterStdinJSONLEmitsTypedSourceFrames(t *testing.T) {
	input := strings.NewReader(`{"event_id":"evt_1","chat_id":"oc_1","chat_type":"group","message_id":"om_1","sender_id":"ou_1","message_type":"text","content":"hello","timestamp":"1782269315000","type":"im.message.receive_v1"}` + "\n")
	var frames bytes.Buffer
	encoder := json.NewEncoder(&frames)
	if err := consumeRawFeishuEventLines(context.Background(), stdinSource{reader: input}, encoder, "source_feishu_chat", true); err != nil {
		t.Fatalf("consumeRawFeishuEventLines returned error: %v", err)
	}
	decoded := decodeSourceFrames(t, frames.String())
	if len(decoded) != 3 {
		t.Fatalf("frames = %+v, want ready event stopped", decoded)
	}
	if decoded[0].Kind != connectorruntime.SourceFrameKindReady {
		t.Fatalf("first frame = %+v, want source.ready", decoded[0])
	}
	if decoded[1].Kind != connectorruntime.SourceFrameKindEvent || decoded[1].Event == nil || decoded[1].Event.ExternalEventID != "evt_1" {
		t.Fatalf("event frame = %+v, want typed Feishu event", decoded[1])
	}
	if decoded[1].Event.SourceValidation != connectorruntime.SourceValidationUnchecked {
		t.Fatalf("source validation = %q, want unchecked", decoded[1].Event.SourceValidation)
	}
	if decoded[1].Cursor == nil || decoded[1].Cursor.CursorValue != "evt_1" || decoded[1].AfterEventID != "evt_1" {
		t.Fatalf("cursor frame fields = %+v, want candidate cursor tied to accepted event", decoded[1])
	}
}

func TestFeishuSourceAdapterMalformedPayloadEmitsFailedFrame(t *testing.T) {
	var frames bytes.Buffer
	encoder := json.NewEncoder(&frames)
	input := strings.NewReader(`{"event_id":"evt_bad","chat_id":"oc_1","message_id":"om_1","content":"missing sender","timestamp":"1782269315000","type":"im.message.receive_v1"}` + "\n")
	if err := consumeRawFeishuEventLines(context.Background(), stdinSource{reader: input}, encoder, "source_feishu_chat", true); err != nil {
		t.Fatalf("consumeRawFeishuEventLines returned error: %v", err)
	}
	decoded := decodeSourceFrames(t, frames.String())
	if len(decoded) != 3 {
		t.Fatalf("frames = %+v, want ready failed stopped", decoded)
	}
	if decoded[1].Kind != connectorruntime.SourceFrameKindFailed || decoded[1].Reason != "source_payload_malformed" {
		t.Fatalf("failed frame = %+v, want malformed source payload failure", decoded[1])
	}
	if decoded[1].PayloadHash == "" || decoded[1].PayloadSizeBytes == 0 {
		t.Fatalf("failed frame = %+v, want bounded payload metadata", decoded[1])
	}
}

func TestFeishuEventCommandDoesNotInventUnsupportedCursorArgument(t *testing.T) {
	executable, args, err := feishuEventCommand("lark-cli", "genesis", defaultFeishuMessageEventKey, "bot", 1, "30s")
	if err != nil {
		t.Fatalf("feishuEventCommand returned error: %v", err)
	}
	if executable != "lark-cli" {
		t.Fatalf("executable = %q, want lark-cli", executable)
	}
	want := []string{"--profile", "genesis", "event", "consume", defaultFeishuMessageEventKey, "--as", "bot", "--max-events", "1", "--timeout", "30s"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestFeishuEventCommandRequiresExplicitProfile(t *testing.T) {
	_, _, err := feishuEventCommand("lark-cli", "", defaultFeishuMessageEventKey, "bot", 1, "30s")
	if err == nil {
		t.Fatal("feishuEventCommand should reject missing explicit profile")
	}
	if !strings.Contains(err.Error(), "explicit --profile") {
		t.Fatalf("error = %v, want explicit profile readiness failure", err)
	}
}

func decodeSourceFrames(t *testing.T, text string) []connectorruntime.SourceCommandFrame {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(text))
	var frames []connectorruntime.SourceCommandFrame
	for {
		var frame connectorruntime.SourceCommandFrame
		if err := decoder.Decode(&frame); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("decode frame: %v\n%s", err, text)
		}
		frames = append(frames, frame)
	}
	return frames
}
