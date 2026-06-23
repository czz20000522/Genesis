package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	ToolSideEffectRead  = "read"
	ToolSideEffectWrite = "write"

	ToolExecutionKindSandboxedProcess = "sandboxed_process"
	ToolExecutionKindKernelControl    = "kernel_control"
)

type registeredTool struct {
	Spec    ToolSpec
	Prepare func(toolInvocationContext, string, string, string, json.RawMessage) (preparedModelToolCall, error)
}

type toolInvocationContext interface {
	prepareShellExecToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
	prepareJobStatusToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
	prepareJobCancelToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
}

type ToolRegistry struct {
	tools map[string]registeredTool
	order []string
}

func NewToolRegistry(tools []registeredTool) (*ToolRegistry, error) {
	registry := &ToolRegistry{
		tools: map[string]registeredTool{},
		order: []string{},
	}
	for _, tool := range tools {
		if err := validateRegisteredTool(tool); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(tool.Spec.Name)
		if _, exists := registry.tools[name]; exists {
			return nil, fmt.Errorf("duplicate tool %q", name)
		}
		registry.tools[name] = tool
		registry.order = append(registry.order, name)
	}
	return registry, nil
}

func defaultToolRegistry() (*ToolRegistry, error) {
	return NewToolRegistry(defaultKernelTools())
}

func defaultKernelTools() []registeredTool {
	return []registeredTool{
		{
			Spec: ToolSpec{
				Name:        "shell_exec",
				Description: "Execute a small governed shell command. Permission mode and workspace root are controlled by the Genesis kernel.",
				InputSchema: map[string]interface{}{
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
						"timeout_sec": map[string]interface{}{
							"type":        "integer",
							"minimum":     1,
							"description": "Foreground timeout in seconds. Omit for 30 seconds. Values above 180 are accepted as managed-job intent.",
						},
					},
					"required":             []string{"command"},
					"additionalProperties": false,
				},
				SideEffectLevel: ToolSideEffectWrite,
				ExecutionKind:   ToolExecutionKindSandboxedProcess,
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareShellExecToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "job_status",
				Description: "Inspect a Genesis-managed job by kernel-issued job id without creating a new operation.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"job_id": map[string]interface{}{
							"type":        "string",
							"description": "Kernel-issued job id returned by a prior managed job receipt.",
						},
					},
					"required":             []string{"job_id"},
					"additionalProperties": false,
				},
				SideEffectLevel: ToolSideEffectRead,
				ExecutionKind:   ToolExecutionKindKernelControl,
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareJobStatusToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "job_cancel",
				Description: "Request cancellation for a Genesis-managed job by kernel-issued job id.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"job_id": map[string]interface{}{
							"type":        "string",
							"description": "Kernel-issued job id returned by a prior managed job receipt.",
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "Optional user-visible cancellation reason.",
						},
					},
					"required":             []string{"job_id"},
					"additionalProperties": false,
				},
				SideEffectLevel: ToolSideEffectWrite,
				ExecutionKind:   ToolExecutionKindKernelControl,
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareJobCancelToolCall(eventID, providerCallID, name, arguments)
			},
		},
	}
}

func validateRegisteredTool(tool registeredTool) error {
	spec := tool.Spec
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return errors.New("tool name is required")
	}
	if strings.Contains(name, ".") {
		return fmt.Errorf("tool %q uses a dotted id", name)
	}
	if strings.TrimSpace(spec.Description) == "" {
		return fmt.Errorf("tool %q description is required", name)
	}
	if spec.InputSchema == nil {
		return fmt.Errorf("tool %q input_schema is required", name)
	}
	switch spec.SideEffectLevel {
	case ToolSideEffectRead, ToolSideEffectWrite:
	default:
		return fmt.Errorf("tool %q has invalid side_effect_level %q", name, spec.SideEffectLevel)
	}
	if strings.TrimSpace(spec.ExecutionKind) == "" {
		return fmt.Errorf("tool %q execution_kind is required", name)
	}
	if tool.Prepare == nil {
		return fmt.Errorf("tool %q has no executor binding", name)
	}
	return nil
}

func (r *ToolRegistry) Resolve(name string) (registeredTool, bool) {
	if r == nil {
		return registeredTool{}, false
	}
	tool, ok := r.tools[strings.TrimSpace(name)]
	return tool, ok
}

func (r *ToolRegistry) Manifest() []ToolSpec {
	if r == nil {
		return nil
	}
	manifest := make([]ToolSpec, 0, len(r.order))
	for _, name := range r.order {
		manifest = append(manifest, r.tools[name].Spec)
	}
	return manifest
}

func (r *ToolRegistry) CapabilityProjections() []ToolCapabilityProjection {
	if r == nil {
		return nil
	}
	projections := make([]ToolCapabilityProjection, 0, len(r.order))
	for _, name := range r.order {
		spec := r.tools[name].Spec
		projections = append(projections, ToolCapabilityProjection{
			Name:            spec.Name,
			SideEffectLevel: spec.SideEffectLevel,
			ExecutionKind:   spec.ExecutionKind,
			Status:          "ok",
		})
	}
	return projections
}

func (k *Kernel) toolGateway() ToolGateway {
	return ToolGateway{
		kernel:   k,
		registry: k.toolRegistry,
	}
}

func toolCapabilitySideEffectLevel(registry *ToolRegistry, name string) string {
	definition, ok := registry.Resolve(name)
	if !ok {
		return "unknown"
	}
	return definition.Spec.SideEffectLevel
}
