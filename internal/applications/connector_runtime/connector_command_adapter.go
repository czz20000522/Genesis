package connectorruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	defaultConnectorCommandTimeout = 30 * time.Second
	maxConnectorCommandOutputBytes = 64 * 1024
	maxConnectorCommandReasonBytes = 256
)

var connectorCommandEnvNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type ConnectorCommandAdapter struct {
	Executable string
	Args       []string
	Env        []string
	WorkingDir string
	Timeout    time.Duration
}

func (a ConnectorCommandAdapter) Execute(ctx context.Context, action ConnectorAction) (ConnectorActionResult, error) {
	if err := validateConnectorCommandAction(action); err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "invalid_connector_action"}, err
	}
	executable, err := a.resolveExecutable()
	if err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "connector_command_missing"}, err
	}
	env, err := a.environment()
	if err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "connector_command_env_rejected"}, err
	}
	payload, err := json.Marshal(action)
	if err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "invalid_connector_action"}, err
	}

	timeout := a.Timeout
	if timeout <= 0 {
		timeout = defaultConnectorCommandTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, executable, a.Args...)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Dir = strings.TrimSpace(a.WorkingDir)
	cmd.Env = env
	var stdout connectorCommandCappedBuffer
	var stderr connectorCommandCappedBuffer
	stdout.limit = maxConnectorCommandOutputBytes
	stderr.limit = maxConnectorCommandOutputBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "connector_command_timeout"}, errors.New("connector command timed out")
		}
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "connector_command_failed"}, errors.New("connector command failed")
	}
	if stdout.truncated {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "connector_command_invalid_result"}, errors.New("connector command stdout exceeded limit")
	}

	result, err := decodeConnectorCommandResult(stdout.String())
	if err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "connector_command_invalid_result"}, err
	}
	if err := validateConnectorCommandResult(result); err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "connector_command_invalid_result"}, err
	}
	return result, nil
}

func (a ConnectorCommandAdapter) resolveExecutable() (string, error) {
	executable := strings.TrimSpace(a.Executable)
	if executable == "" || invalidCommandTemplateExecutable(executable) {
		return "", fmt.Errorf("connector command executable must be a direct executable")
	}
	resolved, err := resolveCommandExecutable(executable)
	if err != nil {
		return "", err
	}
	if unsafeResolvedCommandExecutable(resolved) {
		return "", fmt.Errorf("%w: %q is not a direct binary", errUnsafeCommandExecutable, resolved)
	}
	return resolved, nil
}

func (a ConnectorCommandAdapter) environment() ([]string, error) {
	env := a.Env
	if env == nil {
		env = connectorCommandEnvironment(os.Environ())
	}
	if err := validateConnectorCommandEnv(env); err != nil {
		return nil, err
	}
	return append([]string(nil), env...), nil
}

func validateConnectorCommandAction(action ConnectorAction) error {
	switch {
	case strings.TrimSpace(action.Connector) == "":
		return errors.New("connector action missing connector")
	case strings.TrimSpace(action.ActionKind) == "":
		return errors.New("connector action missing action kind")
	case strings.TrimSpace(action.TargetRef.Connector) == "":
		return errors.New("connector action missing target connector")
	case strings.TrimSpace(action.TargetRef.Connector) != strings.TrimSpace(action.Connector):
		return errors.New("connector action target connector mismatch")
	case strings.TrimSpace(action.TargetRef.Kind) == "":
		return errors.New("connector action missing target kind")
	case strings.TrimSpace(action.TargetRef.ExternalID) == "":
		return errors.New("connector action missing target external id")
	case strings.TrimSpace(action.IdempotencyKey) == "":
		return errors.New("connector action missing idempotency key")
	case len(action.TargetRef.Metadata) != 0:
		return errors.New("connector action target metadata is not executable payload")
	default:
		return nil
	}
}

func validateConnectorCommandEnv(env []string) error {
	for _, raw := range env {
		entry := strings.TrimSpace(raw)
		name, value, ok := strings.Cut(entry, "=")
		if entry == "" || !ok || !connectorCommandEnvNamePattern.MatchString(name) {
			return errors.New("invalid connector command environment entry")
		}
		if isCredentialShapedExternalValue(name) || isCredentialShapedExternalValue(value) {
			return errors.New("credential-shaped connector command environment entry")
		}
	}
	return nil
}

func decodeConnectorCommandResult(text string) (ConnectorActionResult, error) {
	if strings.TrimSpace(text) == "" {
		return ConnectorActionResult{}, errors.New("connector command returned empty result")
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	var result ConnectorActionResult
	if err := decoder.Decode(&result); err != nil {
		return ConnectorActionResult{}, fmt.Errorf("decode connector command result: %w", err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ConnectorActionResult{}, errors.New("connector command returned multiple JSON values")
	}
	return result, nil
}

func validateConnectorCommandResult(result ConnectorActionResult) error {
	switch result.Status {
	case DeliveryStatusSent, DeliveryStatusFailed, DeliveryStatusRetrying, DeliveryStatusDeadLettered, DeliveryStatusPartialSuccess, DeliveryStatusAmbiguous:
	default:
		return fmt.Errorf("connector command returned unsupported status %q", result.Status)
	}
	if result.ExternalActionRef != "" && !safeExternalActionRef(result.ExternalActionRef) {
		return errors.New("connector command returned unsafe external action ref")
	}
	if !safeConnectorCommandReason(result.Reason) {
		return errors.New("connector command returned unsafe reason")
	}
	return nil
}

func safeConnectorCommandReason(reason string) bool {
	if reason == "" {
		return true
	}
	if len(reason) > maxConnectorCommandReasonBytes || isCredentialShapedExternalValue(reason) {
		return false
	}
	for _, r := range reason {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

type connectorCommandCappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *connectorCommandCappedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	available := b.limit - b.buf.Len()
	if available <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > available {
		_, _ = b.buf.Write(p[:available])
		b.truncated = true
		return len(p), nil
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *connectorCommandCappedBuffer) String() string {
	return b.buf.String()
}
