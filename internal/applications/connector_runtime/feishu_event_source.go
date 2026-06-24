package connectorruntime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultFeishuMessageEventKey = "im.message.receive_v1"
	defaultFeishuEventIdentity   = "bot"
	defaultFeishuReadyTimeout    = 15 * time.Second
)

type FeishuEventSourceConfig struct {
	Executable      string
	Profile         string
	EventKey        string
	Identity        string
	MaxEvents       int
	Timeout         string
	IgnoreSenderIDs []string
	FailureStore    SourceFailureStore
}

type FeishuEventSourceRetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

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

func ExternalEventFromFeishuMessageReceiveJSON(raw []byte) (ExternalEvent, error) {
	var event FeishuMessageReceiveEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return ExternalEvent{}, fmt.Errorf("decode Feishu message event: %w", err)
	}
	return ExternalEventFromFeishuMessageReceive(event)
}

func ExternalEventFromFeishuMessageReceive(event FeishuMessageReceiveEvent) (ExternalEvent, error) {
	eventID := strings.TrimSpace(event.EventID)
	messageID := firstNonEmpty(event.MessageID, event.ID)
	body := strings.TrimSpace(event.Content)
	eventType := strings.TrimSpace(event.Type)
	switch {
	case eventType != DefaultFeishuMessageEventKey:
		return ExternalEvent{}, fmt.Errorf("Feishu message event type = %q, want %q", eventType, DefaultFeishuMessageEventKey)
	case eventID == "":
		return ExternalEvent{}, errors.New("Feishu message event missing event_id")
	case strings.TrimSpace(event.ChatID) == "":
		return ExternalEvent{}, errors.New("Feishu message event missing chat_id")
	case strings.TrimSpace(event.SenderID) == "":
		return ExternalEvent{}, errors.New("Feishu message event missing sender_id")
	case messageID == "":
		return ExternalEvent{}, errors.New("Feishu message event missing message_id")
	case body == "":
		return ExternalEvent{}, errors.New("Feishu message event missing content")
	}
	receivedAt, ok := parseFeishuEventTime(firstNonEmpty(event.Timestamp, event.CreateTime))
	if !ok {
		return ExternalEvent{}, errors.New("Feishu message event missing valid timestamp")
	}
	metadata := map[string]string{}
	metadata["external_event_type"] = eventType
	if value := strings.TrimSpace(event.MessageType); value != "" {
		metadata["message_type"] = value
	}
	if value := strings.TrimSpace(event.ChatType); value != "" {
		metadata["chat_type"] = value
	}
	return ExternalEvent{
		Connector:       "feishu",
		ExternalEventID: eventID,
		EventType:       "message.created",
		ThreadRef: ExternalThreadRef{
			Connector:  "feishu",
			Kind:       "chat",
			ExternalID: strings.TrimSpace(event.ChatID),
		},
		SenderRef: ExternalRef{
			Connector:  "feishu",
			Kind:       "user",
			ExternalID: strings.TrimSpace(event.SenderID),
		},
		MessageRef: ExternalRef{
			Connector:  "feishu",
			Kind:       "message",
			ExternalID: messageID,
		},
		Body:             body,
		ReceivedAt:       receivedAt,
		SourceValidation: SourceValidationUnchecked,
		Metadata:         metadata,
	}, nil
}

func ConsumeFeishuEventSource(ctx context.Context, config FeishuEventSourceConfig, diagnostics io.Writer, handle func(ExternalEvent) error) error {
	if handle == nil {
		return errors.New("Feishu event source handler is required")
	}
	executable, args, err := config.Command()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = connectorCommandEnvironment(os.Environ())
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	ready := make(chan struct{})
	stderrDone := make(chan struct{})
	go drainFeishuEventStderr(stderrPipe, diagnostics, ready, stderrDone)
	if err := waitForFeishuReady(ctx, ready); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		<-stderrDone
		return err
	}

	scanErr := processFeishuEventStdout(ctx, stdoutPipe, diagnostics, ignoreSenderIDSet(config.IgnoreSenderIDs), config.FailureStore, handle)
	if scanErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		<-stderrDone
		return scanErr
	}
	waitErr := cmd.Wait()
	<-stderrDone
	if waitErr != nil {
		return waitErr
	}
	return nil
}

func ConsumeFeishuEventSourceWithRetry(ctx context.Context, config FeishuEventSourceConfig, retry FeishuEventSourceRetryPolicy, diagnostics io.Writer, handle func(ExternalEvent) error) error {
	if handle == nil {
		return errors.New("Feishu event source handler is required")
	}
	if _, _, err := config.Command(); err != nil {
		return err
	}
	return consumeFeishuEventSourceWithRetry(ctx, config, normalizeFeishuEventSourceRetryPolicy(retry), diagnostics, func() error {
		return ConsumeFeishuEventSource(ctx, config, diagnostics, handle)
	})
}

func consumeFeishuEventSourceWithRetry(ctx context.Context, config FeishuEventSourceConfig, retry FeishuEventSourceRetryPolicy, diagnostics io.Writer, consume func() error) error {
	if consume == nil {
		return errors.New("Feishu event source consume function is required")
	}
	retry = normalizeFeishuEventSourceRetryPolicy(retry)
	var lastErr error
	for attempt := 1; attempt <= retry.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := consume()
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		lastErr = err
		if recordErr := recordFeishuSourceRuntimeFailure(ctx, config.FailureStore, config, attempt, err); recordErr != nil {
			return recordErr
		}
		if attempt == retry.MaxAttempts {
			break
		}
		if diagnostics != nil {
			fmt.Fprintf(diagnostics, "Feishu event source attempt %d failed: %s; retrying\n", attempt, SafeCLIProbeExcerpt([]byte(err.Error())))
		}
		if err := sleepFeishuEventSourceBackoff(ctx, retry.Backoff); err != nil {
			return err
		}
	}
	return fmt.Errorf("Feishu event source failed after %d attempt(s): %w", retry.MaxAttempts, lastErr)
}

func normalizeFeishuEventSourceRetryPolicy(retry FeishuEventSourceRetryPolicy) FeishuEventSourceRetryPolicy {
	if retry.MaxAttempts <= 0 {
		retry.MaxAttempts = 1
	}
	if retry.Backoff < 0 {
		retry.Backoff = 0
	}
	return retry
}

func sleepFeishuEventSourceBackoff(ctx context.Context, backoff time.Duration) error {
	if backoff <= 0 {
		return nil
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func processFeishuEventStdout(ctx context.Context, reader io.Reader, diagnostics io.Writer, ignoredSenderIDs map[string]struct{}, failureStore SourceFailureStore, handle func(ExternalEvent) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		event, err := ExternalEventFromFeishuMessageReceiveJSON([]byte(line))
		if err != nil {
			fmt.Fprintf(diagnostics, "Feishu source event rejected: %v\n", err)
			if err := recordFeishuSourceFailure(ctx, failureStore, line, err); err != nil {
				return err
			}
			continue
		}
		if _, ignored := ignoredSenderIDs[event.SenderRef.ExternalID]; ignored {
			continue
		}
		if err := handle(event); err != nil {
			fmt.Fprintf(diagnostics, "Feishu source event processing failed: %v\n", err)
		}
	}
	return scanner.Err()
}

func recordFeishuSourceRuntimeFailure(ctx context.Context, store SourceFailureStore, config FeishuEventSourceConfig, attempt int, cause error) error {
	if store == nil {
		return nil
	}
	eventKey := strings.TrimSpace(config.EventKey)
	if eventKey == "" {
		eventKey = DefaultFeishuMessageEventKey
	}
	detail := fmt.Sprintf("attempt %d: %s", attempt, SafeCLIProbeExcerpt([]byte(cause.Error())))
	record := SourceFailureRecord{
		Connector:        "feishu",
		EventSource:      eventKey,
		Reason:           "source_runtime_error",
		Detail:           detail,
		SourceValidation: SourceValidationUnchecked,
	}
	if err := store.RecordSourceFailure(ctx, record); err != nil {
		return fmt.Errorf("record Feishu source runtime failure: %w", err)
	}
	return nil
}

func recordFeishuSourceFailure(ctx context.Context, store SourceFailureStore, rawLine string, cause error) error {
	if store == nil {
		return nil
	}
	record := SourceFailureRecord{
		Connector:        "feishu",
		EventSource:      DefaultFeishuMessageEventKey,
		Reason:           "malformed_source_event",
		Detail:           cause.Error(),
		RawExcerpt:       boundedSourceExcerpt(rawLine),
		SourceValidation: SourceValidationRejected,
	}
	if err := store.RecordSourceFailure(ctx, record); err != nil {
		return fmt.Errorf("record Feishu source failure: %w", err)
	}
	return nil
}

func boundedSourceExcerpt(value string) string {
	const limit = 2048
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n[truncated]"
}

func ignoreSenderIDSet(values []string) map[string]struct{} {
	ignored := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			ignored[value] = struct{}{}
		}
	}
	return ignored
}

func (c FeishuEventSourceConfig) Command() (string, []string, error) {
	profile := strings.TrimSpace(c.Profile)
	if profile == "" {
		return "", nil, errors.New("Feishu event source requires explicit profile")
	}
	eventKey := strings.TrimSpace(c.EventKey)
	if eventKey == "" {
		eventKey = DefaultFeishuMessageEventKey
	}
	if eventKey != DefaultFeishuMessageEventKey {
		return "", nil, fmt.Errorf("unsupported Feishu event key %q", eventKey)
	}
	identity := strings.TrimSpace(c.Identity)
	if identity == "" {
		identity = defaultFeishuEventIdentity
	}
	if identity != "bot" && identity != "user" && identity != "auto" {
		return "", nil, fmt.Errorf("unsupported Feishu event identity %q", identity)
	}
	if c.MaxEvents < 0 {
		return "", nil, errors.New("Feishu event source max events must be non-negative")
	}
	executable := SelectFeishuCLIExecutable(c.Executable, InstalledOfficialLarkCLIExecutable())
	resolved, err := resolveCommandExecutable(executable)
	if err != nil {
		return "", nil, err
	}
	if unsafeResolvedCommandExecutable(resolved) {
		return "", nil, fmt.Errorf("%w: %q is not a direct binary", errUnsafeCommandExecutable, resolved)
	}
	args := []string{
		"--profile", profile,
		"event", "consume", eventKey,
		"--as", identity,
	}
	if c.MaxEvents > 0 {
		args = append(args, "--max-events", strconv.Itoa(c.MaxEvents))
	}
	if timeout := strings.TrimSpace(c.Timeout); timeout != "" {
		args = append(args, "--timeout", timeout)
	}
	return resolved, args, nil
}

func drainFeishuEventStderr(reader io.Reader, diagnostics io.Writer, ready chan<- struct{}, done chan<- struct{}) {
	defer close(done)
	scanner := bufio.NewScanner(reader)
	readyClosed := false
	for scanner.Scan() {
		line := scanner.Text()
		if diagnostics != nil {
			fmt.Fprintln(diagnostics, SafeCLIProbeExcerpt([]byte(line)))
		}
		if !readyClosed && strings.Contains(line, "[event] ready") {
			close(ready)
			readyClosed = true
		}
	}
}

func waitForFeishuReady(ctx context.Context, ready <-chan struct{}) error {
	timer := time.NewTimer(defaultFeishuReadyTimeout)
	defer timer.Stop()
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errors.New("Feishu event source did not become ready")
	}
}

func SelectFeishuCLIExecutable(explicit string, installed string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	if strings.TrimSpace(installed) != "" {
		return strings.TrimSpace(installed)
	}
	return "lark-cli"
}

func InstalledOfficialLarkCLIExecutable() string {
	candidate := OfficialLarkCLIExecutable(os.Getenv("APPDATA"), runtime.GOOS)
	if candidate == "" {
		return ""
	}
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return ""
	}
	return candidate
}

func OfficialLarkCLIExecutable(appData string, goos string) string {
	if goos != "windows" || strings.TrimSpace(appData) == "" {
		return ""
	}
	return filepath.Join(appData, "npm", "node_modules", "@larksuite", "cli", "bin", "lark-cli.exe")
}

func SafeCLIProbeExcerpt(output []byte) string {
	const limit = 1024
	truncated := false
	if len(output) > limit {
		output = output[:limit]
		truncated = true
	}
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if isCredentialShapedExternalValue(line) {
			lines[i] = "[redacted credential-shaped CLI output]"
		}
	}
	text := strings.Join(lines, "\n")
	if truncated {
		text += "\n[truncated]"
	}
	return text
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
