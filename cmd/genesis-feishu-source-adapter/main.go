package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"genesis/internal/applications/connector_runtime"
	feishucli "genesis/internal/applications/feishu_cli"
)

const (
	defaultFeishuMessageEventKey = "im.message.receive_v1"
	defaultFeishuEventIdentity   = "bot"
	defaultFeishuAdapterRef      = "feishu-source-adapter"
)

type FeishuMessageReceiveEvent struct {
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	Content     string `json:"content"`
	CreateTime  string `json:"create_time"`
	EventID     string `json:"event_id"`
	ID          string `json:"id"`
	MessageID   string `json:"message_id"`
	MessageType string `json:"message_type"`
	SenderID    string `json:"sender_id"`
	Timestamp   string `json:"timestamp"`
	Type        string `json:"type"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("genesis-feishu-source-adapter", flag.ContinueOnError)
	profile := fs.String("profile", "", "explicit lark-cli profile for this Feishu source adapter")
	larkCLI := fs.String("lark-cli", os.Getenv("GENESIS_FEISHU_CLI_EXECUTABLE"), "direct lark-cli executable")
	sourceID := fs.String("source-id", "", "stable connector source id supplied by the connector runtime")
	eventKey := fs.String("event-key", defaultFeishuMessageEventKey, "Feishu event key consumed by this adapter")
	afterEventID := fs.String("after-event-id", "", "connector-owned resume cursor; passed to lark-cli without exposing it to the kernel")
	identity := fs.String("as", defaultFeishuEventIdentity, "Feishu event source identity: bot, user, or auto")
	maxEvents := fs.Int("max-events", 0, "stop Feishu source after N events; 0 means unlimited")
	eventTimeout := fs.String("event-timeout", "", "stop Feishu source after duration such as 30s or 10m")
	stdinJSONL := fs.Bool("stdin-jsonl", false, "read raw Feishu event NDJSON from stdin instead of running lark-cli")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sourceID) == "" {
		return errors.New("Feishu source adapter requires --source-id")
	}
	encoder := json.NewEncoder(stdout)
	if *stdinJSONL {
		return consumeRawFeishuEventLines(ctx, stdinSource{reader: os.Stdin}, encoder, *sourceID, true)
	}
	executable, argv, err := feishuEventCommand(*larkCLI, *profile, *eventKey, *afterEventID, *identity, *maxEvents, *eventTimeout)
	if err != nil {
		emitSourceFailed(encoder, *sourceID, "source_not_ready", err.Error(), "", 0)
		return err
	}
	cmd := exec.CommandContext(ctx, executable, argv...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		emitSourceFailed(encoder, *sourceID, "source_not_ready", err.Error(), "", 0)
		return err
	}
	ready := make(chan struct{})
	stderrDone := make(chan struct{})
	go drainFeishuEventStderr(stderrPipe, stderr, encoder, *sourceID, ready, stderrDone)
	<-ready
	consumeErr := consumeRawFeishuEventLines(ctx, stdinSource{reader: stdoutPipe}, encoder, *sourceID, false)
	waitErr := cmd.Wait()
	<-stderrDone
	if consumeErr != nil {
		return consumeErr
	}
	if waitErr != nil {
		emitSourceFailed(encoder, *sourceID, "source_runtime_failed", waitErr.Error(), "", 0)
		return waitErr
	}
	return emitFrame(encoder, connectorruntime.SourceCommandFrame{
		Kind:     connectorruntime.SourceFrameKindStopped,
		SourceID: *sourceID,
	})
}

type stdinSource struct {
	reader io.Reader
}

func consumeRawFeishuEventLines(ctx context.Context, source stdinSource, encoder *json.Encoder, sourceID string, emitLifecycle bool) error {
	scanner := bufio.NewScanner(source.reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if emitLifecycle {
		if err := emitFrame(encoder, connectorruntime.SourceCommandFrame{
			Kind:       connectorruntime.SourceFrameKindReady,
			SourceID:   sourceID,
			Connector:  "feishu",
			AdapterRef: defaultFeishuAdapterRef,
		}); err != nil {
			return err
		}
	}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		event, err := externalEventFromFeishuMessageReceiveJSON([]byte(line))
		if err != nil {
			if err := emitSourceFailed(encoder, sourceID, "source_payload_malformed", err.Error(), sourcePayloadHash(line), len(line)); err != nil {
				return err
			}
			continue
		}
		frame := connectorruntime.SourceCommandFrame{
			Kind:     connectorruntime.SourceFrameKindEvent,
			SourceID: sourceID,
			Event:    &event,
			Cursor: &connectorruntime.SourceCommandCursorFrame{
				SourceID:     sourceID,
				CursorKind:   connectorruntime.SourceCursorKindExternalEventID,
				CursorValue:  event.ExternalEventID,
				WatermarkAt:  event.ReceivedAt,
				AfterEventID: event.ExternalEventID,
			},
			AfterEventID: event.ExternalEventID,
		}
		if err := emitFrame(encoder, frame); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if emitLifecycle {
		return emitFrame(encoder, connectorruntime.SourceCommandFrame{
			Kind:     connectorruntime.SourceFrameKindStopped,
			SourceID: sourceID,
		})
	}
	return nil
}

func feishuEventCommand(executable string, profile string, eventKey string, afterEventID string, identity string, maxEvents int, timeout string) (string, []string, error) {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return "", nil, errors.New("Feishu source adapter requires explicit --profile")
	}
	eventKey = strings.TrimSpace(eventKey)
	if eventKey == "" {
		eventKey = defaultFeishuMessageEventKey
	}
	if eventKey != defaultFeishuMessageEventKey {
		return "", nil, fmt.Errorf("unsupported Feishu event key %q", eventKey)
	}
	identity = strings.TrimSpace(identity)
	if identity == "" {
		identity = defaultFeishuEventIdentity
	}
	if identity != "bot" && identity != "user" && identity != "auto" {
		return "", nil, fmt.Errorf("unsupported Feishu event identity %q", identity)
	}
	if maxEvents < 0 {
		return "", nil, errors.New("Feishu source adapter max events must be non-negative")
	}
	executable = feishucli.SelectExecutable(executable, feishucli.InstalledOfficialExecutable())
	argv := []string{"--profile", profile, "event", "consume", eventKey, "--as", identity}
	if strings.TrimSpace(afterEventID) != "" {
		argv = append(argv, "--after-event-id", strings.TrimSpace(afterEventID))
	}
	if maxEvents > 0 {
		argv = append(argv, "--max-events", strconv.Itoa(maxEvents))
	}
	if strings.TrimSpace(timeout) != "" {
		argv = append(argv, "--timeout", strings.TrimSpace(timeout))
	}
	return executable, argv, nil
}

func externalEventFromFeishuMessageReceiveJSON(raw []byte) (connectorruntime.ExternalEvent, error) {
	var event FeishuMessageReceiveEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return connectorruntime.ExternalEvent{}, fmt.Errorf("decode Feishu message event: %w", err)
	}
	return externalEventFromFeishuMessageReceive(event)
}

func externalEventFromFeishuMessageReceive(event FeishuMessageReceiveEvent) (connectorruntime.ExternalEvent, error) {
	eventID := strings.TrimSpace(event.EventID)
	messageID := firstNonEmpty(event.MessageID, event.ID)
	body := strings.TrimSpace(event.Content)
	eventType := strings.TrimSpace(event.Type)
	switch {
	case eventType != defaultFeishuMessageEventKey:
		return connectorruntime.ExternalEvent{}, fmt.Errorf("Feishu message event type = %q, want %q", eventType, defaultFeishuMessageEventKey)
	case eventID == "":
		return connectorruntime.ExternalEvent{}, errors.New("Feishu message event missing event_id")
	case strings.TrimSpace(event.ChatID) == "":
		return connectorruntime.ExternalEvent{}, errors.New("Feishu message event missing chat_id")
	case strings.TrimSpace(event.SenderID) == "":
		return connectorruntime.ExternalEvent{}, errors.New("Feishu message event missing sender_id")
	case messageID == "":
		return connectorruntime.ExternalEvent{}, errors.New("Feishu message event missing message_id")
	case body == "":
		return connectorruntime.ExternalEvent{}, errors.New("Feishu message event missing content")
	}
	receivedAt, ok := parseFeishuEventTime(firstNonEmpty(event.Timestamp, event.CreateTime))
	if !ok {
		return connectorruntime.ExternalEvent{}, errors.New("Feishu message event missing valid timestamp")
	}
	metadata := map[string]string{"external_event_type": eventType}
	if value := strings.TrimSpace(event.MessageType); value != "" {
		metadata["message_type"] = value
	}
	if value := strings.TrimSpace(event.ChatType); value != "" {
		metadata["chat_type"] = value
	}
	return connectorruntime.ExternalEvent{
		Connector:       "feishu",
		ExternalEventID: eventID,
		EventType:       "message.created",
		ThreadRef: connectorruntime.ExternalThreadRef{
			Connector:  "feishu",
			Kind:       "chat",
			ExternalID: strings.TrimSpace(event.ChatID),
		},
		SenderRef: connectorruntime.ExternalRef{
			Connector:  "feishu",
			Kind:       "user",
			ExternalID: strings.TrimSpace(event.SenderID),
		},
		MessageRef: connectorruntime.ExternalRef{
			Connector:  "feishu",
			Kind:       "message",
			ExternalID: messageID,
		},
		Body:             body,
		ReceivedAt:       receivedAt,
		SourceValidation: connectorruntime.SourceValidationUnchecked,
		Metadata:         metadata,
	}, nil
}

func drainFeishuEventStderr(reader io.Reader, diagnostics io.Writer, encoder *json.Encoder, sourceID string, ready chan<- struct{}, done chan<- struct{}) {
	defer close(done)
	scanner := bufio.NewScanner(reader)
	readyClosed := false
	for scanner.Scan() {
		line := scanner.Text()
		if diagnostics != nil {
			fmt.Fprintln(diagnostics, connectorruntime.SafeCLIProbeExcerpt([]byte(line)))
		}
		if !readyClosed && strings.Contains(line, "[event] ready") {
			_ = emitFrame(encoder, connectorruntime.SourceCommandFrame{
				Kind:       connectorruntime.SourceFrameKindReady,
				SourceID:   sourceID,
				Connector:  "feishu",
				AdapterRef: defaultFeishuAdapterRef,
			})
			close(ready)
			readyClosed = true
		}
	}
	if !readyClosed {
		close(ready)
	}
}

func emitSourceFailed(encoder *json.Encoder, sourceID string, reason string, detail string, payloadHash string, payloadSizeBytes int) error {
	return emitFrame(encoder, connectorruntime.SourceCommandFrame{
		Kind:             connectorruntime.SourceFrameKindFailed,
		SourceID:         sourceID,
		Connector:        "feishu",
		AdapterRef:       defaultFeishuAdapterRef,
		EventSource:      defaultFeishuMessageEventKey,
		Reason:           reason,
		Detail:           detail,
		PayloadHash:      payloadHash,
		PayloadSizeBytes: payloadSizeBytes,
	})
}

func emitFrame(encoder *json.Encoder, frame connectorruntime.SourceCommandFrame) error {
	return encoder.Encode(frame)
}

func sourcePayloadHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseFeishuEventTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	millis, err := strconv.ParseInt(value, 10, 64)
	if err != nil || millis <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(millis).UTC(), true
}
