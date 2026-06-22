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
	CWD     string `json:"cwd"`
	Command string `json:"command"`
}

type preparedModelToolCall struct {
	callID         string
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
	seenCallIDs := map[string]bool{}
	for _, call := range calls {
		callID := strings.TrimSpace(call.ToolCallID)
		if callID != "" {
			if seenCallIDs[callID] {
				return nil, fmt.Errorf("%w: duplicate tool_call_id %q", ErrModelToolCallRejected, callID)
			}
			seenCallIDs[callID] = true
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
				prepared[i].callID,
				prepared[i].name,
				"tool_batch_not_executed",
				"tool batch was not executed because at least one tool request was invalid",
			)
			prepared[i].providerCallID = providerCallID
		}
	}
	return prepared, nil
}

func (g ToolGateway) prepareCall(call ModelToolCall) (preparedModelToolCall, error) {
	name := strings.TrimSpace(call.Name)
	callID := strings.TrimSpace(call.ToolCallID)
	if callID == "" {
		return preparedModelToolCall{}, fmt.Errorf("%w: tool_call_id is required", ErrModelToolCallRejected)
	}
	if err := validateIdempotencyKey(callID); err != nil {
		return preparedModelToolCall{}, fmt.Errorf("%w: invalid tool_call_id: %v", ErrModelToolCallRejected, err)
	}
	definition, ok := g.registry.Resolve(name)
	if !ok {
		prepared := invalidPreparedModelToolCall(callID, name, "unsupported_tool", fmt.Sprintf("unsupported tool %q", call.Name))
		prepared.providerCallID = strings.TrimSpace(call.ProviderToolCallID)
		return prepared, nil
	}
	prepared, err := definition.Prepare(g.kernel, callID, name, call.Arguments)
	if err != nil {
		return preparedModelToolCall{}, err
	}
	prepared.providerCallID = strings.TrimSpace(call.ProviderToolCallID)
	return prepared, nil
}

func (k *Kernel) prepareShellExecToolCall(callID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args shellExecToolArguments
	if err := decodeStrictModelToolArguments("shell_exec", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(callID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	args.CWD = strings.TrimSpace(args.CWD)
	if args.CWD == "" {
		args.CWD = strings.TrimSpace(k.toolPolicy.WorkspaceRoot)
	}
	if err := validateShellRequest(ShellExecRequest{
		SessionID:      "model-tool-validation",
		CWD:            args.CWD,
		Command:        args.Command,
		IdempotencyKey: callID,
	}); err != nil {
		return invalidPreparedModelToolCall(callID, name, "invalid_shell_exec_request", fmt.Sprintf("invalid shell_exec request: %v", err)), nil
	}
	return preparedModelToolCall{
		callID: callID,
		name:   name,
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			operation, err := k.toolGateway().ExecShell(ctx, ShellExecRequest{
				SessionID:      sessionID,
				CWD:            args.CWD,
				Command:        args.Command,
				IdempotencyKey: callID,
			}, turnID)
			if err != nil {
				return ModelToolResult{}, fmt.Errorf("%w: %w", ErrToolInfrastructureFailed, err)
			}
			content, err := json.Marshal(modelOperationResult(operation))
			if err != nil {
				return ModelToolResult{}, err
			}
			return ModelToolResult{
				ToolCallID:         callID,
				ProviderToolCallID: "",
				Name:               name,
				Content:            string(content),
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
			ToolCallID:         prepared.callID,
			ProviderToolCallID: prepared.providerCallID,
			Name:               prepared.name,
			Content:            string(content),
		}, nil
	}
	if prepared.execute == nil {
		return ModelToolResult{}, fmt.Errorf("%w: prepared tool %q has no executable payload", ErrModelToolCallRejected, prepared.name)
	}
	result, err := prepared.execute(ctx, sessionID, turnID)
	if err != nil {
		return ModelToolResult{}, err
	}
	if strings.TrimSpace(result.ProviderToolCallID) == "" {
		result.ProviderToolCallID = prepared.providerCallID
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

func invalidPreparedModelToolCall(callID string, name string, code string, message string) preparedModelToolCall {
	tool := strings.TrimSpace(name)
	if tool == "" {
		tool = "unknown"
	}
	return preparedModelToolCall{
		callID:         callID,
		providerCallID: "",
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
