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
		{
			Name:        "skill.read",
			Description: "Read the bounded instructions for a configured user-space skill by skill name. This does not grant authority or bypass kernel tool permissions.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Configured skill name from the available external skills catalog.",
					},
				},
				"required":             []string{"name"},
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

type skillReadToolArguments struct {
	Name string `json:"name"`
}

type preparedModelToolCall struct {
	callID    string
	name      string
	shellExec *shellExecToolArguments
	skillRead *SkillReadProjection
}

func (k *Kernel) prepareModelToolCalls(calls []ModelToolCall) ([]preparedModelToolCall, error) {
	prepared := make([]preparedModelToolCall, 0, len(calls))
	for _, call := range calls {
		item, err := k.prepareModelToolCall(call)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, item)
	}
	return prepared, nil
}

func (k *Kernel) prepareModelToolCall(call ModelToolCall) (preparedModelToolCall, error) {
	name := strings.TrimSpace(call.Name)
	callID := strings.TrimSpace(call.ToolCallID)
	if callID == "" {
		return preparedModelToolCall{}, fmt.Errorf("%w: tool_call_id is required", ErrModelToolCallRejected)
	}
	if err := validateIdempotencyKey(callID); err != nil {
		return preparedModelToolCall{}, fmt.Errorf("%w: invalid tool_call_id: %v", ErrModelToolCallRejected, err)
	}
	switch name {
	case "shell.exec":
		return k.prepareShellExecToolCall(callID, name, call.Arguments)
	case "skill.read":
		return k.prepareSkillReadToolCall(callID, name, call.Arguments)
	default:
		return preparedModelToolCall{}, fmt.Errorf("%w: unsupported tool %q", ErrModelToolCallRejected, call.Name)
	}
}

func (k *Kernel) prepareShellExecToolCall(callID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args shellExecToolArguments
	if err := decodeStrictModelToolArguments("shell.exec", arguments, &args); err != nil {
		return preparedModelToolCall{}, err
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
		callID:    callID,
		name:      name,
		shellExec: &args,
	}, nil
}

func (k *Kernel) prepareSkillReadToolCall(callID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args skillReadToolArguments
	if err := decodeStrictModelToolArguments("skill.read", arguments, &args); err != nil {
		return preparedModelToolCall{}, err
	}
	args.Name = strings.TrimSpace(args.Name)
	if args.Name == "" {
		return preparedModelToolCall{}, fmt.Errorf("%w: skill.read name is required", ErrModelToolCallRejected)
	}
	if hasInvisibleControlMarker(args.Name) {
		return preparedModelToolCall{}, fmt.Errorf("%w: skill.read name contains invisible control markers", ErrModelToolCallRejected)
	}
	if err := validateKernelTextNotSecret("skill.read name", args.Name); err != nil {
		return preparedModelToolCall{}, fmt.Errorf("%w: %v", ErrModelToolCallRejected, err)
	}
	if _, ok := k.skillDescriptorByName(args.Name); !ok {
		return preparedModelToolCall{}, fmt.Errorf("%w: unknown skill %q", ErrModelToolCallRejected, args.Name)
	}
	projection, err := k.readSkillInstruction(args.Name)
	if err != nil {
		return preparedModelToolCall{}, fmt.Errorf("%w: skill.read failed: %v", ErrModelToolCallRejected, err)
	}
	return preparedModelToolCall{
		callID:    callID,
		name:      name,
		skillRead: &projection,
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

func (k *Kernel) executePreparedModelToolCall(ctx context.Context, sessionID string, turnID string, prepared preparedModelToolCall) (ModelToolResult, error) {
	if prepared.skillRead != nil {
		content, err := json.Marshal(prepared.skillRead)
		if err != nil {
			return ModelToolResult{}, err
		}
		return ModelToolResult{
			ToolCallID: prepared.callID,
			Name:       prepared.name,
			Content:    string(content),
		}, nil
	}
	if prepared.shellExec == nil {
		return ModelToolResult{}, fmt.Errorf("%w: prepared tool %q has no executable payload", ErrModelToolCallRejected, prepared.name)
	}
	operation, err := k.execShell(ctx, ShellExecRequest{
		SessionID:      sessionID,
		CWD:            prepared.shellExec.CWD,
		Command:        prepared.shellExec.Command,
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
