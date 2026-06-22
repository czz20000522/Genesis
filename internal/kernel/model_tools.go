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

func (k *Kernel) modelToolDescriptors() []ModelToolDescriptor {
	return []ModelToolDescriptor{
		{
			Name:        "shell.exec",
			Description: "Execute a small governed shell command. Permission mode and workspace root are controlled by the Genesis kernel.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Command to execute through the governed shell tool.",
					},
					"cwd": map[string]interface{}{
						"type":        "string",
						"description": "Optional working directory. When omitted, the kernel uses the configured workspace root when available.",
					},
				},
				"required":             []string{"command"},
				"additionalProperties": false,
			},
		},
	}
}

func modelToolCallRecords(calls []ModelToolCall) []ModelToolCallRecord {
	records := make([]ModelToolCallRecord, 0, len(calls))
	for _, call := range calls {
		records = append(records, ModelToolCallRecord{
			ToolCallID: call.ToolCallID,
			Tool:       call.Name,
		})
	}
	return records
}

type shellExecToolArguments struct {
	CWD     string `json:"cwd"`
	Command string `json:"command"`
}

type preparedModelToolCall struct {
	callID string
	name   string
	args   shellExecToolArguments
}

func (k *Kernel) validateModelToolCalls(calls []ModelToolCall) error {
	for _, call := range calls {
		if _, err := k.prepareModelToolCall(call); err != nil {
			return err
		}
	}
	return nil
}

func (k *Kernel) prepareModelToolCall(call ModelToolCall) (preparedModelToolCall, error) {
	if strings.TrimSpace(call.Name) != "shell.exec" {
		return preparedModelToolCall{}, fmt.Errorf("%w: unsupported tool %q", ErrModelToolCallRejected, call.Name)
	}
	callID := strings.TrimSpace(call.ToolCallID)
	if callID == "" {
		return preparedModelToolCall{}, fmt.Errorf("%w: tool_call_id is required", ErrModelToolCallRejected)
	}
	if err := validateIdempotencyKey(callID); err != nil {
		return preparedModelToolCall{}, fmt.Errorf("%w: invalid tool_call_id: %v", ErrModelToolCallRejected, err)
	}
	var args shellExecToolArguments
	if len(call.Arguments) == 0 {
		return preparedModelToolCall{}, fmt.Errorf("%w: shell.exec arguments are required", ErrModelToolCallRejected)
	}
	decoder := json.NewDecoder(bytes.NewReader(call.Arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return preparedModelToolCall{}, fmt.Errorf("%w: invalid shell.exec arguments: %v", ErrModelToolCallRejected, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return preparedModelToolCall{}, fmt.Errorf("%w: invalid shell.exec arguments: trailing data", ErrModelToolCallRejected)
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
		return preparedModelToolCall{}, fmt.Errorf("%w: invalid shell.exec request: %v", ErrModelToolCallRejected, err)
	}
	return preparedModelToolCall{
		callID: callID,
		name:   strings.TrimSpace(call.Name),
		args:   args,
	}, nil
}

func (k *Kernel) executeModelToolCall(ctx context.Context, sessionID string, turnID string, call ModelToolCall) (ModelToolResult, error) {
	prepared, err := k.prepareModelToolCall(call)
	if err != nil {
		return ModelToolResult{}, err
	}
	operation, err := k.execShell(ctx, ShellExecRequest{
		SessionID:      sessionID,
		CWD:            prepared.args.CWD,
		Command:        prepared.args.Command,
		IdempotencyKey: prepared.callID,
	}, turnID)
	if err != nil {
		return ModelToolResult{}, err
	}
	content, err := json.Marshal(operation)
	if err != nil {
		return ModelToolResult{}, err
	}
	return ModelToolResult{
		ToolCallID: prepared.callID,
		Name:       prepared.name,
		Content:    string(content),
	}, nil
}
