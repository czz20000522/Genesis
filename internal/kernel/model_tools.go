package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const maxModelToolRounds = 4

var ErrModelToolCallRejected = errors.New("model tool call rejected")
var ErrToolInfrastructureFailed = errors.New("tool infrastructure failed")

type shellExecToolArguments struct {
	CWD        string `json:"cwd"`
	Command    string `json:"command"`
	TimeoutSec *int   `json:"timeout_sec,omitempty"`
}

type preparedModelToolCall struct {
	eventID        string
	providerCallID string
	name           string
	requestInvalid *ToolRequestInvalidProjection
	execute        func(context.Context, string, string) (ModelToolResult, error)
}

type ToolGateway struct {
	kernel   *Kernel
	registry *ToolRegistry
}

func (g ToolGateway) ToolManifest() []ToolSpec {
	return g.registry.Manifest()
}

func (g ToolGateway) CapabilityProjections() []ToolCapabilityProjection {
	return g.registry.CapabilityProjections()
}

func (g ToolGateway) PrepareBatch(calls []ModelToolCall) ([]preparedModelToolCall, error) {
	prepared := make([]preparedModelToolCall, 0, len(calls))
	hasInvalidRequest := false
	seenEventIDs := map[string]bool{}
	for _, call := range calls {
		eventID := strings.TrimSpace(call.ToolCallEventID)
		if eventID != "" {
			if seenEventIDs[eventID] {
				return nil, fmt.Errorf("%w: duplicate tool_call_event_id %q", ErrModelToolCallRejected, eventID)
			}
			seenEventIDs[eventID] = true
		}
		item, err := g.prepareCall(call)
		if err != nil {
			return nil, err
		}
		if item.requestInvalid != nil {
			hasInvalidRequest = true
		}
		prepared = append(prepared, item)
	}
	if hasInvalidRequest {
		for i := range prepared {
			if prepared[i].requestInvalid != nil {
				continue
			}
			providerCallID := prepared[i].providerCallID
			prepared[i] = invalidPreparedModelToolCall(
				prepared[i].eventID,
				providerCallID,
				prepared[i].name,
				"tool_batch_not_executed",
				"tool batch was not executed because at least one tool request was invalid",
			)
		}
	}
	return prepared, nil
}

func (g ToolGateway) prepareCall(call ModelToolCall) (preparedModelToolCall, error) {
	name := strings.TrimSpace(call.Name)
	eventID := strings.TrimSpace(call.ToolCallEventID)
	if eventID == "" {
		return preparedModelToolCall{}, fmt.Errorf("%w: tool_call_event_id is required", ErrModelToolCallRejected)
	}
	if err := validateIdempotencyKey(eventID); err != nil {
		return preparedModelToolCall{}, fmt.Errorf("%w: invalid tool_call_event_id: %v", ErrModelToolCallRejected, err)
	}
	providerCallID := strings.TrimSpace(call.ToolCallID)
	definition, ok := g.registry.Resolve(name)
	if !ok {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "unsupported_tool", fmt.Sprintf("unsupported tool %q", call.Name)), nil
	}
	prepared, err := definition.Prepare(g.kernel, eventID, providerCallID, name, call.Arguments)
	if err != nil {
		return preparedModelToolCall{}, err
	}
	return prepared, nil
}

func (k *Kernel) prepareShellExecToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args shellExecToolArguments
	if err := decodeStrictModelToolArguments("shell_exec", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	args.CWD = strings.TrimSpace(args.CWD)
	if args.CWD == "" {
		args.CWD = strings.TrimSpace(k.toolPolicy.WorkspaceRoot)
	}
	timeoutSec := defaultShellTimeoutSec
	if args.TimeoutSec != nil {
		timeoutSec = *args.TimeoutSec
		if timeoutSec <= 0 {
			return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_shell_exec_request", "invalid shell_exec request: timeout_sec must be greater than zero"), nil
		}
	}
	if err := validateShellRequest(ShellExecRequest{
		SessionID:      "model-tool-validation",
		CWD:            args.CWD,
		Command:        args.Command,
		TimeoutSec:     timeoutSec,
		IdempotencyKey: eventID,
	}); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_shell_exec_request", fmt.Sprintf("invalid shell_exec request: %v", err)), nil
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			if timeoutSec > maxForegroundShellTimeoutSec {
				return k.startManagedShellJobReceipt(sessionID, turnID, eventID, providerCallID, name, ShellExecRequest{
					SessionID:      sessionID,
					CWD:            args.CWD,
					Command:        args.Command,
					TimeoutSec:     timeoutSec,
					IdempotencyKey: eventID,
				})
			}
			operation, err := k.toolGateway().ExecShell(ctx, ShellExecRequest{
				SessionID:      sessionID,
				CWD:            args.CWD,
				Command:        args.Command,
				TimeoutSec:     timeoutSec,
				IdempotencyKey: eventID,
			}, turnID)
			if err != nil {
				return ModelToolResult{}, fmt.Errorf("%w: %w", ErrToolInfrastructureFailed, err)
			}
			content, err := json.Marshal(modelOperationResult(operation))
			if err != nil {
				return ModelToolResult{}, err
			}
			return ModelToolResult{
				ToolCallID:      providerCallID,
				ToolCallEventID: eventID,
				Name:            name,
				Content:         string(content),
			}, nil
		},
	}, nil
}

func decodeStrictModelToolArguments(tool string, arguments json.RawMessage, target interface{}) error {
	if len(arguments) == 0 {
		return fmt.Errorf("%w: %s arguments are required", ErrModelToolCallRejected, tool)
	}
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: invalid %s arguments: %v", ErrModelToolCallRejected, tool, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: invalid %s arguments: trailing data", ErrModelToolCallRejected, tool)
	}
	return nil
}

func (g ToolGateway) Execute(ctx context.Context, sessionID string, turnID string, prepared preparedModelToolCall) (ModelToolResult, error) {
	if prepared.requestInvalid != nil {
		content, err := json.Marshal(prepared.requestInvalid)
		if err != nil {
			return ModelToolResult{}, err
		}
		return ModelToolResult{
			ToolCallID:      prepared.providerCallID,
			ToolCallEventID: prepared.eventID,
			Name:            prepared.name,
			Content:         string(content),
		}, nil
	}
	if prepared.execute == nil {
		return ModelToolResult{}, fmt.Errorf("%w: prepared tool %q has no executable payload", ErrModelToolCallRejected, prepared.name)
	}
	result, err := prepared.execute(ctx, sessionID, turnID)
	if err != nil {
		return ModelToolResult{}, err
	}
	if strings.TrimSpace(result.ToolCallEventID) == "" {
		result.ToolCallEventID = prepared.eventID
	}
	if strings.TrimSpace(result.ToolCallID) == "" {
		result.ToolCallID = prepared.providerCallID
	}
	return result, nil
}

func modelOperationResult(operation OperationProjection) interface{} {
	if operation.Status == "blocked" {
		return ToolRequestInvalidProjection{
			Status:   "permission_denied",
			Tool:     operation.Tool,
			Executed: false,
			Error: ToolRequestError{
				Code:    "permission_denied",
				Message: "tool execution was blocked by kernel policy",
			},
		}
	}
	return ModelOperationResult{
		Status:              operation.Status,
		Executed:            true,
		ExitCode:            operation.ExitCode,
		Stdout:              operation.Stdout,
		Stderr:              operation.Stderr,
		StdoutTruncated:     operation.StdoutTruncated,
		StderrTruncated:     operation.StderrTruncated,
		StdoutOriginalBytes: operation.StdoutOriginalBytes,
		StderrOriginalBytes: operation.StderrOriginalBytes,
		StdoutOmittedBytes:  operation.StdoutOmittedBytes,
		StderrOmittedBytes:  operation.StderrOmittedBytes,
		OutputTruncation:    operation.OutputTruncation,
	}
}

func invalidPreparedModelToolCall(eventID string, providerCallID string, name string, code string, message string) preparedModelToolCall {
	tool := strings.TrimSpace(name)
	if tool == "" {
		tool = "unknown"
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           tool,
		requestInvalid: &ToolRequestInvalidProjection{
			Status:   "tool_request_invalid",
			Tool:     tool,
			Executed: false,
			Error: ToolRequestError{
				Code:    code,
				Message: redactEvidenceText(strings.TrimSpace(message)),
			},
		},
	}
}

func toolRequestInvalidMessage(err error) string {
	message := err.Error()
	prefix := ErrModelToolCallRejected.Error() + ": "
	return strings.TrimPrefix(message, prefix)
}
