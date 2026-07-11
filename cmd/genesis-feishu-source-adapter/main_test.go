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
	feishucli "genesis/internal/applications/feishu_cli"
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
	if executable != feishucli.SelectExecutable("lark-cli", feishucli.InstalledOfficialExecutable()) {
		t.Fatalf("executable = %q, want resolved direct lark-cli binary", executable)
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

func TestRunProfileProbeWritesTypedReadinessWithoutStartingSource(t *testing.T) {
	var stdout bytes.Buffer
	runner := &profileProbeRunner{output: []byte(`{"identities":{"bot":{"status":"ready","available":true}}}`)}
	if err := runProfileProbe(context.Background(), "genesis", "lark-cli", &stdout, runner); err != nil {
		t.Fatalf("runProfileProbe returned error: %v", err)
	}
	var result connectorruntime.ProfileReadinessCommandResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode profile probe result: %v", err)
	}
	if result.Ready == nil || !*result.Ready || result.Reason != "" {
		t.Fatalf("profile probe result = %+v, want ready", result)
	}
	if runner.executable != feishucli.SelectExecutable("lark-cli", feishucli.InstalledOfficialExecutable()) || strings.Join(runner.args, "\x00") != "auth\x00status\x00--profile\x00genesis" {
		t.Fatalf("profile probe command = %q %#v", runner.executable, runner.args)
	}
}

type profileProbeRunner struct {
	executable string
	args       []string
	output     []byte
}

func (r *profileProbeRunner) Run(_ context.Context, executable string, args ...string) ([]byte, error) {
	r.executable = executable
	r.args = append([]string(nil), args...)
	return append([]byte(nil), r.output...), nil
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
