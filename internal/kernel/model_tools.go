package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func (k *Kernel) executeModelToolCall(ctx context.Context, sessionID string, turnID string, call ModelToolCall) (ModelToolResult, error) {
	if strings.TrimSpace(call.Name) != "shell.exec" {
		return ModelToolResult{}, fmt.Errorf("%w: unsupported tool %q", ErrModelToolCallRejected, call.Name)
	}
	callID := strings.TrimSpace(call.ToolCallID)
	if callID == "" {
		return ModelToolResult{}, fmt.Errorf("%w: tool_call_id is required", ErrModelToolCallRejected)
	}
	if err := validateIdempotencyKey(callID); err != nil {
		return ModelToolResult{}, fmt.Errorf("%w: invalid tool_call_id: %v", ErrModelToolCallRejected, err)
	}
	var args shellExecToolArguments
	if len(call.Arguments) == 0 {
		return ModelToolResult{}, fmt.Errorf("%w: shell.exec arguments are required", ErrModelToolCallRejected)
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return ModelToolResult{}, fmt.Errorf("%w: invalid shell.exec arguments: %v", ErrModelToolCallRejected, err)
	}
	args.CWD = strings.TrimSpace(args.CWD)
	if args.CWD == "" {
		args.CWD = strings.TrimSpace(k.toolPolicy.WorkspaceRoot)
	}
	operation, err := k.execShell(ctx, ShellExecRequest{
		SessionID:      sessionID,
		CWD:            args.CWD,
		Command:        args.Command,
		IdempotencyKey: callID,
	}, turnID)
	if err != nil {
		return ModelToolResult{}, err
	}
	content, err := json.Marshal(operation)
	if err != nil {
		return ModelToolResult{}, err
	}
	return ModelToolResult{
		ToolCallID: callID,
		Name:       call.Name,
		Content:    string(content),
	}, nil
}
