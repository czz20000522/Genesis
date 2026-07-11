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
	CWD        string `json:"cwd,omitempty"`
	Command    string `json:"command"`
	TimeoutSec *int   `json:"timeout_sec,omitempty" jsonschema:"minimum=1"`
}

type resourceReadToolArguments struct {
	ResourceRef string `json:"resource_ref"`
	OffsetBytes *int   `json:"offset_bytes,omitempty" jsonschema:"minimum=0"`
	LimitBytes  *int   `json:"limit_bytes,omitempty" jsonschema:"minimum=1"`
}

type sourceTreeToolArguments struct {
	SourceSnapshotRef string `json:"source_snapshot_ref"`
	MaxEntries        *int   `json:"max_entries,omitempty" jsonschema:"minimum=1"`
}

type sourceReadToolArguments struct {
	SourceFileRef string `json:"source_file_ref"`
	OffsetBytes   *int   `json:"offset_bytes,omitempty" jsonschema:"minimum=0"`
	LimitBytes    *int   `json:"limit_bytes,omitempty" jsonschema:"minimum=1"`
}

type workspaceEditToolArguments struct {
	Path      string                    `json:"path"`
	OldString string                    `json:"old_string,omitempty"`
	NewString string                    `json:"new_string,omitempty"`
	Edits     []workspaceEditToolChange `json:"edits,omitempty" jsonschema:"minItems=1"`
}

type workspaceEditToolChange struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type contextDiscoverToolArguments struct {
	Intent                string   `json:"intent"`
	CurrentContextSummary string   `json:"current_context_summary,omitempty"`
	RequestedKinds        []string `json:"requested_kinds,omitempty"`
	Limit                 int      `json:"limit,omitempty" jsonschema:"minimum=1"`
}

type jobStatusToolArguments struct {
	JobID string `json:"job_id"`
}

type jobWaitToolArguments struct {
	JobID      string `json:"job_id"`
	TimeoutSec *int   `json:"timeout_sec,omitempty" jsonschema:"minimum=1"`
}

type jobCancelToolArguments struct {
	JobID  string `json:"job_id"`
	Reason string `json:"reason,omitempty"`
}

type delegateWorkerToolArguments struct {
	RoleID string `json:"role_id"`
	Task   string `json:"task"`
}

type taskGraphEditToolArguments struct {
	Operation   string `json:"operation"`
	GraphID     string `json:"graph_id,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	FromNodeID  string `json:"from_node_id,omitempty"`
	ToNodeID    string `json:"to_node_id,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
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
	kernel       *Kernel
	registry     *ToolRegistry
	policy       ToolPolicy
	allowedTools map[string]struct{}
}

type workspaceToolInvocationContext struct {
	*Kernel
	policy ToolPolicy
}

func (c workspaceToolInvocationContext) prepareShellExecToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	return c.Kernel.prepareShellExecToolCallWithPolicy(c.policy, eventID, providerCallID, name, arguments)
}

func (c workspaceToolInvocationContext) prepareWorkspaceEditToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	return c.Kernel.prepareWorkspaceEditToolCallWithRoot(c.policy.WorkspaceRoot, eventID, providerCallID, name, arguments)
}

func (c workspaceToolInvocationContext) prepareDelegateWorkerToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	return c.Kernel.prepareDelegateWorkerToolCall(eventID, providerCallID, name, arguments)
}
func (c workspaceToolInvocationContext) prepareTaskGraphEditToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	return c.Kernel.prepareTaskGraphEditToolCall(eventID, providerCallID, name, arguments)
}

func (g ToolGateway) invocationContext() toolInvocationContext {
	return workspaceToolInvocationContext{Kernel: g.kernel, policy: g.policy}
}

func (g ToolGateway) ToolManifest() []ToolSpec {
	manifest := g.registry.Manifest()
	if g.allowedTools == nil {
		return manifest
	}
	filtered := make([]ToolSpec, 0, len(manifest))
	for _, spec := range manifest {
		if g.toolAllowed(spec.Name) {
			filtered = append(filtered, spec)
		}
	}
	return filtered
}

func (g ToolGateway) CapabilityProjections() []ToolCapabilityProjection {
	projections := g.registry.CapabilityProjections()
	if g.allowedTools == nil {
		return projections
	}
	filtered := make([]ToolCapabilityProjection, 0, len(projections))
	for _, projection := range projections {
		if g.toolAllowed(projection.Name) {
			filtered = append(filtered, projection)
		}
	}
	return filtered
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
	if !g.toolAllowed(name) {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "capability_grant_tool_not_allowed", fmt.Sprintf("tool %q is not allowed by the admitted invocation capability grant", name)), nil
	}
	prepared, err := definition.Prepare(g.invocationContext(), eventID, providerCallID, name, call.Arguments)
	if err != nil {
		return preparedModelToolCall{}, err
	}
	prepared.spec = definition.Spec
	prepared.hasSpec = true
	return prepared, nil
}

func (g ToolGateway) toolAllowed(name string) bool {
	if g.allowedTools == nil {
		return true
	}
	if strings.TrimSpace(name) == "task_graph_edit" {
		return false
	}
	_, ok := g.allowedTools[strings.TrimSpace(name)]
	return ok
}

func (k *Kernel) prepareShellExecToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	return k.prepareShellExecToolCallWithPolicy(k.toolPolicy, eventID, providerCallID, name, arguments)
}

func (k *Kernel) prepareShellExecToolCallWithPolicy(policy ToolPolicy, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args shellExecToolArguments
	if err := decodeStrictModelToolArguments("shell_exec", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	args.CWD = strings.TrimSpace(args.CWD)
	if args.CWD == "" {
		args.CWD = strings.TrimSpace(policy.WorkspaceRoot)
	}
	timeoutSec := k.shellTimeoutPolicy.DefaultForegroundTimeoutSec
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
		accessPlan:             k.shellExecAccessPlan(name, args.CWD, timeoutSec),
		repeatSuccessSignature: k.shellExecRepeatSuccessSignature(args.CWD, args.Command, timeoutSec),
		onDenied: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.shellInvokeModelToolResultWithPolicy(policy, ctx, sessionID, turnID, eventID, providerCallID, name, ShellExecRequest{
				SessionID:      sessionID,
				CWD:            args.CWD,
				Command:        args.Command,
				TimeoutSec:     timeoutSec,
				IdempotencyKey: eventID,
			})
		},
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.shellInvokeModelToolResultWithPolicy(policy, ctx, sessionID, turnID, eventID, providerCallID, name, ShellExecRequest{
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
	return k.shellInvokeModelToolResultWithPolicy(k.toolPolicy, ctx, sessionID, turnID, eventID, providerCallID, name, req)
}

func (k *Kernel) shellInvokeModelToolResultWithPolicy(policy ToolPolicy, ctx context.Context, sessionID string, turnID string, eventID string, providerCallID string, name string, req ShellExecRequest) (ModelToolResult, error) {
	result, err := k.toolGatewayWithPolicy(policy).InvokeShell(ctx, req, turnID, eventID, false)
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
	req, _, code, err := k.resourceRegistry.AdmitReadText(args.ResourceRef, args.OffsetBytes, args.LimitBytes)
	if err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, code, fmt.Sprintf("invalid resource_read request: %v", err)), nil
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
	offsetBytes := req.OffsetBytes
	limitBytes := req.LimitBytes
	readReq, _, code, err := k.resourceRegistry.AdmitReadText(req.ResourceRef, &offsetBytes, &limitBytes)
	if err != nil {
		return invalidModelToolResult(eventID, providerCallID, name, code, fmt.Sprintf("invalid resource_read request: %v", err))
	}
	result, err := k.resourceRegistry.Read(readReq)
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

func (k *Kernel) prepareSourceTreeToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args sourceTreeToolArguments
	if err := decodeStrictModelToolArguments("source_tree", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", sourceTreeToolRequestInvalidMessage(err)), nil
	}
	req, _, code, err := k.resourceRegistry.AdmitSourceTree(args.SourceSnapshotRef, args.MaxEntries)
	if err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, code, fmt.Sprintf("invalid source_tree request: %v", err)), nil
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		accessPlan:     sourceReadToolAccessPlan(name, req.SourceSnapshotRef),
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.sourceTreeModelToolResult(eventID, providerCallID, name, req)
		},
	}, nil
}

func (k *Kernel) sourceTreeModelToolResult(eventID string, providerCallID string, name string, req resource.SourceTreeRequest) (ModelToolResult, error) {
	maxEntries := req.MaxEntries
	treeReq, _, code, err := k.resourceRegistry.AdmitSourceTree(req.SourceSnapshotRef, &maxEntries)
	if err != nil {
		return invalidModelToolResult(eventID, providerCallID, name, code, fmt.Sprintf("invalid source_tree request: %v", err))
	}
	result, err := k.resourceRegistry.SourceTree(treeReq)
	if err != nil {
		return ModelToolResult{}, fmt.Errorf("%w: source_tree failed: %v", ErrToolInfrastructureFailed, err)
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

func (k *Kernel) prepareSourceReadToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args sourceReadToolArguments
	if err := decodeStrictModelToolArguments("source_read", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	req, _, code, err := k.resourceRegistry.AdmitSourceRead(args.SourceFileRef, args.OffsetBytes, args.LimitBytes)
	if err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, code, fmt.Sprintf("invalid source_read request: %v", err)), nil
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		accessPlan:     sourceReadToolAccessPlan(name, req.SourceFileRef),
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.sourceReadModelToolResult(eventID, providerCallID, name, req)
		},
	}, nil
}

func (k *Kernel) sourceReadModelToolResult(eventID string, providerCallID string, name string, req resource.SourceReadRequest) (ModelToolResult, error) {
	offsetBytes := req.OffsetBytes
	limitBytes := req.LimitBytes
	readReq, _, code, err := k.resourceRegistry.AdmitSourceRead(req.SourceFileRef, &offsetBytes, &limitBytes)
	if err != nil {
		return invalidModelToolResult(eventID, providerCallID, name, code, fmt.Sprintf("invalid source_read request: %v", err))
	}
	result, err := k.resourceRegistry.SourceRead(readReq)
	if err != nil {
		return ModelToolResult{}, fmt.Errorf("%w: source_read failed: %v", ErrToolInfrastructureFailed, err)
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

func (k *Kernel) prepareWorkspaceEditToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	return k.prepareWorkspaceEditToolCallWithRoot(k.toolPolicy.WorkspaceRoot, eventID, providerCallID, name, arguments)
}

func (k *Kernel) prepareWorkspaceEditToolCallWithRoot(workspaceRoot string, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args workspaceEditToolArguments
	if err := decodeStrictModelToolArguments("workspace_edit", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	req, code, err := k.admitWorkspaceEditRequestWithRoot(workspaceRoot, args)
	if err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, code, fmt.Sprintf("invalid workspace_edit request: %v", err)), nil
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		accessPlan:     workspaceEditToolAccessPlan(name, req.RelativePath),
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.workspaceEditModelToolResult(eventID, providerCallID, name, req)
		},
	}, nil
}

func (k *Kernel) prepareContextDiscoverToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args contextDiscoverToolArguments
	if err := decodeStrictModelToolArguments("context_discover", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	req := DiscoveryQueryRequest{
		Intent:                args.Intent,
		CurrentContextSummary: args.CurrentContextSummary,
		RequestedKinds:        append([]string(nil), args.RequestedKinds...),
		Limit:                 args.Limit,
	}
	if _, _, _, err := normalizeDiscoveryQuery(req); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_context_discovery_request", fmt.Sprintf("invalid context_discover request: %v", err)), nil
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		accessPlan:     contextDiscoveryToolAccessPlan(name),
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.contextDiscoverModelToolResult(eventID, providerCallID, name, req)
		},
	}, nil
}

func (k *Kernel) contextDiscoverModelToolResult(eventID string, providerCallID string, name string, req DiscoveryQueryRequest) (ModelToolResult, error) {
	result, err := k.DiscoverContext(req)
	if err != nil {
		return invalidModelToolResult(eventID, providerCallID, name, "invalid_context_discovery_request", fmt.Sprintf("invalid context_discover request: %v", err))
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

func (k *Kernel) prepareJobWaitToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args jobWaitToolArguments
	if err := decodeStrictModelToolArguments("job_wait", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	jobID, err := validateJobControlRequest("job_wait", args.JobID)
	if err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_job_control_request", err.Error()), nil
	}
	timeoutSec := defaultJobWaitTimeoutSec
	if args.TimeoutSec != nil {
		timeoutSec = *args.TimeoutSec
	}
	if timeoutSec <= 0 || timeoutSec > maxJobWaitTimeoutSec {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_job_wait_request", fmt.Sprintf("invalid job_wait request: timeout_sec must be between 1 and %d", maxJobWaitTimeoutSec)), nil
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		accessPlan:     jobControlToolAccessPlan(name, jobID),
		execute: func(ctx context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.jobWaitModelToolResult(ctx, sessionID, eventID, providerCallID, name, jobID, timeoutSec)
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

func (k *Kernel) prepareDelegateWorkerToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args delegateWorkerToolArguments
	if err := decodeStrictModelToolArguments("delegate_worker", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	args.RoleID = strings.TrimSpace(args.RoleID)
	args.Task = strings.TrimSpace(args.Task)
	if args.RoleID == "" || args.Task == "" {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_delegate_worker_request", "invalid delegate_worker request: role_id and task are required"), nil
	}
	return preparedModelToolCall{
		eventID:        eventID,
		providerCallID: providerCallID,
		name:           name,
		accessPlan:     delegateWorkerToolAccessPlan(name),
		execute: func(_ context.Context, sessionID string, turnID string) (ModelToolResult, error) {
			return k.delegateWorkerModelToolResult(sessionID, turnID, eventID, providerCallID, name, args)
		},
	}, nil
}

func (k *Kernel) prepareTaskGraphEditToolCall(eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
	var args taskGraphEditToolArguments
	if err := decodeStrictModelToolArguments("task_graph_edit", arguments, &args); err != nil {
		return invalidPreparedModelToolCall(eventID, providerCallID, name, "invalid_tool_arguments", toolRequestInvalidMessage(err)), nil
	}
	args.Operation, args.GraphID, args.NodeID = strings.TrimSpace(args.Operation), strings.TrimSpace(args.GraphID), strings.TrimSpace(args.NodeID)
	return preparedModelToolCall{eventID: eventID, providerCallID: providerCallID, name: name, accessPlan: delegateWorkerToolAccessPlan(name), execute: func(_ context.Context, sessionID string, _ string) (ModelToolResult, error) {
		var result interface{}
		var err error
		switch args.Operation {
		case "create_graph":
			result, err = k.CreateTaskGraph(TaskGraphCreateRequest{SessionID: sessionID})
		case "add_task":
			result, err = k.AddTaskGraphNode(TaskGraphNodeRequest{GraphID: args.GraphID, Title: args.Title, Description: args.Description})
		case "add_dependency":
			err = k.AddTaskGraphEdge(TaskGraphEdgeRequest{GraphID: args.GraphID, FromNodeID: args.FromNodeID, ToNodeID: args.ToNodeID})
			result = map[string]string{"status": "accepted"}
		case "remove_dependency":
			err = k.RemoveTaskGraphEdge(TaskGraphEdgeRemoveRequest{GraphID: args.GraphID, FromNodeID: args.FromNodeID, ToNodeID: args.ToNodeID})
			result = map[string]string{"status": "accepted"}
		case "update_task":
			err = k.UpdateTaskGraphNode(TaskGraphNodeUpdateRequest{GraphID: args.GraphID, NodeID: args.NodeID, Title: args.Title, Description: args.Description})
			result = map[string]string{"status": "accepted"}
		default:
			return invalidModelToolResult(eventID, providerCallID, name, "invalid_task_graph_operation", "invalid task_graph_edit operation")
		}
		if err != nil {
			return invalidModelToolResult(eventID, providerCallID, name, "task_graph_edit_failed", err.Error())
		}
		content, err := json.Marshal(result)
		if err != nil {
			return ModelToolResult{}, err
		}
		return ModelToolResult{ToolCallID: providerCallID, ToolCallEventID: eventID, Name: name, Content: string(content)}, nil
	}}, nil
}

func (k *Kernel) delegateWorkerModelToolResult(sessionID string, turnID string, eventID string, providerCallID string, name string, args delegateWorkerToolArguments) (ModelToolResult, error) {
	invocation, err := k.AdmitWorkerInvocationFromRole(WorkerInvocationAdmissionRequest{
		ConfigRoot:     k.parentWorkerConfigRoot,
		ParentID:       k.parentWorkerParentID,
		RoleID:         args.RoleID,
		SessionID:      sessionID,
		ParentTurnID:   turnID,
		Principal:      "application:kernel",
		IdempotencyKey: eventID,
	})
	if err != nil {
		return invalidModelToolResult(eventID, providerCallID, name, "worker_delegation_failed", fmt.Sprintf("delegate_worker failed: %v", err))
	}
	content, err := json.Marshal(struct {
		Status       string `json:"status"`
		Executed     bool   `json:"executed"`
		InvocationID string `json:"invocation_id"`
		RoleID       string `json:"role_id"`
	}{
		Status:       "queued",
		Executed:     true,
		InvocationID: invocation.InvocationID,
		RoleID:       args.RoleID,
	})
	if err != nil {
		return ModelToolResult{}, err
	}
	return ModelToolResult{
		ToolCallID:                  strings.TrimSpace(providerCallID),
		ToolCallEventID:             strings.TrimSpace(eventID),
		Name:                        strings.TrimSpace(name),
		Content:                     string(content),
		PendingAgentInvocationStart: &AgentInvocationRunRequest{InvocationID: invocation.InvocationID, Principal: "application:kernel", InputItems: []InputItem{{Type: "text", Text: args.Task}}, IdempotencyKey: eventID},
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
		authorization := authorizeKernelTool(g.policy, prepared.spec)
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
	return modelJobControlResultWithTimeout(job, cancelRequested, false, visibleOutput)
}

func modelJobControlResultWithTimeout(job JobProjection, cancelRequested bool, timedOut bool, visibleOutput string) ModelJobControlResult {
	job = cloneJobProjection(job)
	return ModelJobControlResult{
		Status:          strings.TrimSpace(job.Status),
		Executed:        true,
		JobID:           strings.TrimSpace(job.JobID),
		Tool:            strings.TrimSpace(job.Tool),
		CancelRequested: cancelRequested,
		TimedOut:        timedOut,
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

func sourceTreeToolRequestInvalidMessage(err error) string {
	message := toolRequestInvalidMessage(err)
	if strings.Contains(message, `unknown field "offset_entries"`) {
		return message + "; offset_entries is not supported by source_tree. Use max_entries to request more entries, and stay within the max_entries_limit returned by the previous source_tree result."
	}
	return message
}
