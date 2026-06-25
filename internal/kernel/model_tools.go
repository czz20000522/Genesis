package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"genesis/internal/kernel/resource"
)

var ErrModelToolCallRejected = errors.New("model tool call rejected")
var ErrToolInfrastructureFailed = errors.New("tool infrastructure failed")

type shellExecToolArguments struct {
	CWD        string `json:"cwd"`
	Command    string `json:"command"`
	TimeoutSec *int   `json:"timeout_sec,omitempty"`
}

type resourceReadToolArguments struct {
	ResourceRef string `json:"resource_ref"`
	OffsetBytes *int   `json:"offset_bytes,omitempty"`
	LimitBytes  *int   `json:"limit_bytes,omitempty"`
}

type jobStatusToolArguments struct {
	JobID string `json:"job_id"`
}

type jobCancelToolArguments struct {
	JobID  string `json:"job_id"`
	Reason string `json:"reason,omitempty"`
}

type preparedModelToolCall struct {
	eventID                string
	providerCallID         string
	name                   string
	spec                   ToolSpec
	hasSpec                bool
	accessPlan             ToolAccessPlan
	requestInvalid         *ToolRequestInvalidProjection
	repeatSuccessSignature string
	execute                func(context.Context, string, string) (ModelToolResult, error)
	onDenied               func(context.Context, string, string) (ModelToolResult, error)
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
	prepared.spec = definition.Spec
	prepared.hasSpec = true
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
		eventID:                eventID,
		providerCallID:         providerCallID,
		name:                   name,
		accessPlan:             shellExecToolAccessPlan(name, args.CWD, timeoutSec),
		repeatSuccessSignature: shellExecRepeatSuccessSignature(args.CWD, args.Command, timeoutSec),
		onDenied: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.shellInvokeModelToolResult(ctx, sessionID, turnID, eventID, providerCallID, name, ShellExecRequest{
				SessionID:      sessionID,
				CWD:            args.CWD,
				Command:        args.Command,
				TimeoutSec:     timeoutSec,
				IdempotencyKey: eventID,
			})
		},
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.shellInvokeModelToolResult(ctx, sessionID, turnID, eventID, providerCallID, name, ShellExecRequest{
				SessionID:      sessionID,
				CWD:            args.CWD,
				Command:        args.Command,
				TimeoutSec:     timeoutSec,
				IdempotencyKey: eventID,
			})
		},
	}, nil
}

func (k *Kernel) shellInvokeModelToolResult(ctx context.Context, sessionID string, turnID string, eventID string, providerCallID string, name string, req ShellExecRequest) (ModelToolResult, error) {
	result, err := k.toolGateway().InvokeShell(ctx, req, turnID, eventID, false)
	if err != nil {
		return ModelToolResult{}, fmt.Errorf("%w: %w", ErrToolInfrastructureFailed, err)
	}
	toolResult := ModelToolResult{
		ToolCallID:      strings.TrimSpace(providerCallID),
		ToolCallEventID: strings.TrimSpace(eventID),
		Name:            strings.TrimSpace(name),
	}
	switch {
	case result.Job != nil && result.Operation != nil && result.Operation.Interrupted && result.Operation.InterruptReason == foregroundAttachedManagedJobReason:
		content, err := json.Marshal(ModelManagedJobResult{
			Status:        "managed_job_started",
			Executed:      true,
			JobID:         result.Job.JobID,
			VisibleOutput: result.Job.Receipt,
		})
		if err != nil {
			return ModelToolResult{}, err
		}
		toolResult.Content = string(content)
	case result.Operation != nil:
		content, err := json.Marshal(modelOperationResult(*result.Operation))
		if err != nil {
			return ModelToolResult{}, err
		}
		toolResult.Content = string(content)
	case result.Job != nil:
		content, err := json.Marshal(ModelManagedJobResult{
			Status:        "managed_job_started",
			Executed:      true,
			JobID:         result.Job.JobID,
			VisibleOutput: result.Job.Receipt,
		})
		if err != nil {
			return ModelToolResult{}, err
		}
		toolResult.Content = string(content)
		toolResult.PendingJobStart = result.PendingJobStart
	default:
		return ModelToolResult{}, fmt.Errorf("%w: shell_exec produced no operation or job", ErrToolInfrastructureFailed)
	}
	return toolResult, nil
}

func (k *Kernel) prepareResourceReadToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args resourceReadToolArguments
	if err := decodeStrictModelToolArguments("resource_read", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	req, code, err := resource.NormalizeReadRequest(args.ResourceRef, args.OffsetBytes, args.LimitBytes)
	if err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, code, fmt.Sprintf("invalid resource_read request: %v", err)), nil
	}
	if !k.resourceRegistry.Has(req.ResourceRef) {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "unknown_resource_ref", fmt.Sprintf("unknown resource ref %q", req.ResourceRef)), nil
	}
	metadata, err := k.resourceRegistry.Metadata(req.ResourceRef)
	if err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "unknown_resource_ref", fmt.Sprintf("unknown resource ref %q", req.ResourceRef)), nil
	}
	if !metadata.TextReadable {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "unsupported_mime_type", fmt.Sprintf("resource %q has unsupported mime type %q", req.ResourceRef, metadata.MimeType)), nil
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		accessPlan:     resourceReadToolAccessPlan(name, req.ResourceRef),
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.resourceReadModelToolResult(eventID, providerCallID, name, req)
		},
	}, nil
}

func (k *Kernel) resourceReadModelToolResult(eventID string, providerCallID string, name string, req resource.ReadRequest) (ModelToolResult, error) {
	result, err := k.resourceRegistry.Read(req)
	if err != nil {
		return ModelToolResult{}, fmt.Errorf("%w: resource_read failed: %v", ErrToolInfrastructureFailed, err)
	}
	content, err := json.Marshal(result)
	if err != nil {
		return ModelToolResult{}, err
	}
	return ModelToolResult{
		ToolCallID:      strings.TrimSpace(providerCallID),
		ToolCallEventID: strings.TrimSpace(eventID),
		Name:            strings.TrimSpace(name),
		Content:         string(content),
	}, nil
}

func (k *Kernel) prepareJobStatusToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args jobStatusToolArguments
	if err := decodeStrictModelToolArguments("job_status", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	jobID, err := validateJobControlRequest("job_status", args.JobID)
	if err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_job_control_request", err.Error()), nil
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		accessPlan:     jobControlToolAccessPlan(name, jobID),
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.jobStatusModelToolResult(sessionID, eventID, providerCallID, name, jobID)
		},
	}, nil
}

func (k *Kernel) prepareJobCancelToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args jobCancelToolArguments
	if err := decodeStrictModelToolArguments("job_cancel", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	jobID, err := validateJobControlRequest("job_cancel", args.JobID)
	if err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_job_control_request", err.Error()), nil
	}
	reason := strings.TrimSpace(args.Reason)
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		accessPlan:     jobControlToolAccessPlan(name, jobID),
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.cancelJobModelToolResult(sessionID, turnID, eventID, providerCallID, name, jobID, reason)
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
	if prepared.hasSpec {
		authorization := authorizeKernelTool(g.kernel.toolPolicy, prepared.spec)
		if !authorization.Allowed {
			if prepared.onDenied != nil {
				return prepared.onDenied(ctx, sessionID, turnID)
			}
			return permissionDeniedModelToolResult(prepared.eventID, prepared.providerCallID, prepared.name)
		}
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
		status, code, message := blockedToolResultError(operation.BlockedReason)
		return ToolRequestInvalidProjection{
			Status:   status,
			Tool:     operation.Tool,
			Executed: false,
			Error: ToolRequestError{
				Code:    code,
				Message: message,
			},
		}
	}
	return ModelOperationResult{
		Status:              operation.Status,
		Executed:            true,
		ExitCode:            operation.ExitCode,
		TimedOut:            operation.TimedOut,
		TimeoutReason:       operation.TimeoutReason,
		Interrupted:         operation.Interrupted,
		InterruptReason:     operation.InterruptReason,
		ElapsedMs:           operation.ElapsedMs,
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

func blockedToolResultError(reason string) (string, string, string) {
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "approval_required":
		return "approval_required", "approval_required", "tool execution requires approval before it can run"
	case strings.HasPrefix(reason, "sandbox_profile_unavailable"):
		return "sandbox_profile_unavailable", "sandbox_profile_unavailable", "tool execution requires a sandbox profile that is not available"
	case reason == "unknown_sandbox_profile":
		return "sandbox_profile_unavailable", "sandbox_profile_unavailable", "tool execution was blocked because the configured sandbox profile is unknown"
	case reason == "unknown_approval_policy":
		return "approval_policy_invalid", "approval_policy_invalid", "tool execution was blocked because the configured approval policy is unknown"
	default:
		return "permission_denied", "permission_denied", "tool execution was blocked by kernel policy"
	}
}

func modelJobControlResult(job JobProjection, cancelRequested bool, visibleOutput string) ModelJobControlResult {
	job = cloneJobProjection(job)
	return ModelJobControlResult{
		Status:          strings.TrimSpace(job.Status),
		Executed:        true,
		JobID:           strings.TrimSpace(job.JobID),
		Tool:            strings.TrimSpace(job.Tool),
		CancelRequested: cancelRequested,
		VisibleOutput:   strings.TrimSpace(visibleOutput),
		ExitCode:        job.ExitCode,
		Stdout:          job.Stdout,
		Stderr:          job.Stderr,
		StdoutTruncated: job.StdoutTruncated,
		StderrTruncated: job.StderrTruncated,
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
		accessPlan: ToolAccessPlan{
			ToolName: tool,
			Trusted:  false,
			Reason:   "tool_request_invalid",
		},
		requestInvalid: &ToolRequestInvalidProjection{
			Status:   "tool_request_invalid",
			Tool:     tool,
			Executed: false,
			Error: ToolRequestError{
				Code:    code,
				Message: strings.TrimSpace(message),
			},
		},
	}
}

func invalidModelToolResult(eventID string, providerCallID string, name string, code string, message string) (ModelToolResult, error) {
	prepared := invalidPreparedModelToolCall(eventID, providerCallID, name, code, message)
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

func permissionDeniedModelToolResult(eventID string, providerCallID string, name string) (ModelToolResult, error) {
	tool := strings.TrimSpace(name)
	if tool == "" {
		tool = "unknown"
	}
	content, err := json.Marshal(ToolRequestInvalidProjection{
		Status:   "permission_denied",
		Tool:     tool,
		Executed: false,
		Error: ToolRequestError{
			Code:    "permission_denied",
			Message: "tool execution was blocked by kernel policy",
		},
	})
	if err != nil {
		return ModelToolResult{}, err
	}
	return ModelToolResult{
		ToolCallID:      strings.TrimSpace(providerCallID),
		ToolCallEventID: strings.TrimSpace(eventID),
		Name:            tool,
		Content:         string(content),
	}, nil
}

func validateJobControlRequest(tool string, jobID string) (string, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return "", fmt.Errorf("invalid %s request: job_id is required", tool)
	}
	if err := validateIdempotencyKey(jobID); err != nil {
		return "", fmt.Errorf("invalid %s request: invalid job_id: %v", tool, err)
	}
	return jobID, nil
}

func toolRequestInvalidMessage(err error) string {
	message := err.Error()
	prefix := ErrModelToolCallRejected.Error() + ": "
	return strings.TrimPrefix(message, prefix)
}
