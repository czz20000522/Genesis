package kernel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	invopopjsonschema "github.com/invopop/jsonschema"
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
	prepareResourceReadToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
	prepareContextDiscoverToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
	prepareSourceTreeToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
	prepareSourceReadToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
	prepareWorkspaceEditToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
	prepareJobStatusToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
	prepareJobWaitToolCall(string, string, string, json.RawMessage) (preparedModelToolCall, error)
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

func defaultToolRegistry(shellPolicy ShellTimeoutPolicy) (*ToolRegistry, error) {
	return NewToolRegistry(defaultKernelTools(shellPolicy))
}

func defaultKernelTools(policies ...ShellTimeoutPolicy) []registeredTool {
	shellPolicy := normalizedShellTimeoutPolicy(ShellTimeoutPolicy{})
	if len(policies) > 0 {
		shellPolicy = normalizedShellTimeoutPolicy(policies[0])
	}
	return []registeredTool{
		{
			Spec: ToolSpec{
				Name:        "shell_exec",
				Description: "Execute a small governed shell command. Permission mode and workspace root are controlled by the Genesis kernel.",
				InputSchema: toolInputSchema(shellExecToolArguments{}, map[string]string{
					"command":     "Command to execute through the governed shell tool.",
					"cwd":         "Optional working directory. When omitted, the kernel uses the configured workspace root when available.",
					"timeout_sec": shellTimeoutDescription(shellPolicy),
				}),
				SideEffectLevel: ToolSideEffectWrite,
				ExecutionKind:   ToolExecutionKindSandboxedProcess,
				Scheduling:      shellExecToolSchedulingSpec(),
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareShellExecToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "resource_read",
				Description: "Read bounded text from a kernel-owned immutable resource reference.",
				InputSchema: toolInputSchema(resourceReadToolArguments{}, map[string]string{
					"resource_ref": "Kernel-issued resource reference to read.",
					"offset_bytes": "Optional byte offset. Omit to start at zero.",
					"limit_bytes":  "Optional byte limit. Omit for the kernel default.",
				}),
				SideEffectLevel: ToolSideEffectRead,
				ExecutionKind:   ToolExecutionKindKernelControl,
				Scheduling:      resourceReadToolSchedulingSpec(),
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareResourceReadToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "context_discover",
				Description: "Discover bounded user-level accumulation and capability descriptors relevant to the current intent. Results are hints only and do not grant tool, resource, connector, or provider-context authority.",
				InputSchema: toolInputSchema(contextDiscoverToolArguments{}, map[string]string{
					"intent":                  "Current semantic intent to search for relevant accumulation or capability descriptors.",
					"current_context_summary": "Optional short summary of the current task context.",
					"requested_kinds":         "Optional memory kinds to search, such as preference, heuristic, method, lesson, project_overlay, capability_hint, or memory_fact.",
					"limit":                   "Optional maximum number of discovery candidates to return.",
				}, toolSchemaNumericLimit{Field: "limit", Keyword: "maximum", Value: maxDiscoveryLimit}),
				SideEffectLevel: ToolSideEffectRead,
				ExecutionKind:   ToolExecutionKindKernelControl,
				Scheduling:      contextDiscoveryToolSchedulingSpec(),
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareContextDiscoverToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "source_tree",
				Description: "List a bounded tree for a kernel-issued source snapshot reference.",
				InputSchema: toolInputSchema(sourceTreeToolArguments{}, map[string]string{
					"source_snapshot_ref": "Kernel-issued source snapshot reference to list.",
					"max_entries":         "Optional maximum number of tree entries to return.",
				}),
				SideEffectLevel: ToolSideEffectRead,
				ExecutionKind:   ToolExecutionKindKernelControl,
				Scheduling:      resourceReadToolSchedulingSpec(),
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareSourceTreeToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "source_read",
				Description: "Read bounded text from a source file reference inside an admitted source snapshot.",
				InputSchema: toolInputSchema(sourceReadToolArguments{}, map[string]string{
					"source_file_ref": "Kernel-issued source file reference to read.",
					"offset_bytes":    "Optional byte offset. Omit to start at zero.",
					"limit_bytes":     "Optional byte limit. Omit for the kernel default.",
				}),
				SideEffectLevel: ToolSideEffectRead,
				ExecutionKind:   ToolExecutionKindKernelControl,
				Scheduling:      resourceReadToolSchedulingSpec(),
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareSourceReadToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "workspace_edit",
				Description: "Replace one exact, unique string in one existing file under the configured workspace root.",
				InputSchema: toolInputSchema(workspaceEditToolArguments{}, map[string]string{
					"path":       "Relative path to an existing workspace file.",
					"old_string": "Exact text to replace. It must occur exactly once.",
					"new_string": "Replacement text. Use an empty string to delete the old string.",
				}),
				SideEffectLevel: ToolSideEffectWrite,
				ExecutionKind:   ToolExecutionKindKernelControl,
				Scheduling:      workspaceEditToolSchedulingSpec(),
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareWorkspaceEditToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "job_status",
				Description: "Inspect a Genesis-managed job by kernel-issued job id without creating a new operation.",
				InputSchema: toolInputSchema(jobStatusToolArguments{}, map[string]string{
					"job_id": "Kernel-issued job id returned by a prior managed job receipt.",
				}),
				SideEffectLevel: ToolSideEffectRead,
				ExecutionKind:   ToolExecutionKindKernelControl,
				Scheduling:      jobControlToolSchedulingSpec(),
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareJobStatusToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "job_wait",
				Description: "Wait briefly for a Genesis-managed job to reach a terminal state without blocking indefinitely.",
				InputSchema: toolInputSchema(jobWaitToolArguments{}, map[string]string{
					"job_id":      "Kernel-issued job id returned by a prior managed job receipt.",
					"timeout_sec": "Maximum seconds to wait. Omit for a short bounded wait.",
				}, toolSchemaNumericLimit{Field: "timeout_sec", Keyword: "maximum", Value: maxJobWaitTimeoutSec}),
				SideEffectLevel: ToolSideEffectRead,
				ExecutionKind:   ToolExecutionKindKernelControl,
				Scheduling:      jobControlToolSchedulingSpec(),
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareJobWaitToolCall(eventID, providerCallID, name, arguments)
			},
		},
		{
			Spec: ToolSpec{
				Name:        "job_cancel",
				Description: "Request cancellation for a Genesis-managed job by kernel-issued job id.",
				InputSchema: toolInputSchema(jobCancelToolArguments{}, map[string]string{
					"job_id": "Kernel-issued job id returned by a prior managed job receipt.",
					"reason": "Optional user-visible cancellation reason.",
				}),
				SideEffectLevel: ToolSideEffectWrite,
				ExecutionKind:   ToolExecutionKindKernelControl,
				Scheduling:      jobControlToolSchedulingSpec(),
			},
			Prepare: func(ctx toolInvocationContext, eventID string, providerCallID string, name string, arguments json.RawMessage) (preparedModelToolCall, error) {
				return ctx.prepareJobCancelToolCall(eventID, providerCallID, name, arguments)
			},
		},
	}
}

type toolSchemaNumericLimit struct {
	Field   string
	Keyword string
	Value   int
}

func toolInputSchema(request any, descriptions map[string]string, limits ...toolSchemaNumericLimit) map[string]interface{} {
	reflector := invopopjsonschema.Reflector{
		Anonymous:      true,
		DoNotReference: true,
		ExpandedStruct: true,
	}
	schema := schemaMap(reflector.Reflect(request))
	delete(schema, "$schema")
	delete(schema, "$id")
	delete(schema, "$defs")
	properties, _ := schema["properties"].(map[string]interface{})
	for name, description := range descriptions {
		property, _ := properties[name].(map[string]interface{})
		if property == nil {
			continue
		}
		property["description"] = description
	}
	for _, limit := range limits {
		property, _ := properties[limit.Field].(map[string]interface{})
		if property == nil {
			continue
		}
		property[limit.Keyword] = limit.Value
	}
	return schema
}

func schemaMap(schema *invopopjsonschema.Schema) map[string]interface{} {
	data, err := json.Marshal(schema)
	if err != nil {
		return invalidReflectedToolSchema()
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var raw map[string]interface{}
	if err := decoder.Decode(&raw); err != nil {
		return invalidReflectedToolSchema()
	}
	normalized, _ := normalizeToolSchemaValue(raw).(map[string]interface{})
	return normalized
}

func invalidReflectedToolSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func normalizeToolSchemaValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			typed[key] = normalizeToolSchemaValue(item)
		}
		return typed
	case []interface{}:
		stringsOnly := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				stringsOnly = nil
				break
			}
			stringsOnly = append(stringsOnly, text)
		}
		if stringsOnly != nil {
			return stringsOnly
		}
		for i, item := range typed {
			typed[i] = normalizeToolSchemaValue(item)
		}
		return typed
	case json.Number:
		if number, err := typed.Int64(); err == nil {
			return int(number)
		}
		return typed.String()
	default:
		return value
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

func (k *Kernel) ToolGatewayForInvocation(invocationID string) (ToolGateway, error) {
	invocation, err := k.AgentInvocation(invocationID)
	if err != nil {
		return ToolGateway{}, err
	}
	return ToolGateway{
		kernel:       k,
		registry:     k.toolRegistry,
		allowedTools: capabilityGrantToolSet(invocation.CapabilityGrant),
	}, nil
}

func capabilityGrantToolSet(grant CapabilityGrant) map[string]struct{} {
	tools := map[string]struct{}{}
	for _, toolName := range grant.ToolNames {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			continue
		}
		tools[toolName] = struct{}{}
	}
	return tools
}

func toolCapabilitySideEffectLevel(registry *ToolRegistry, name string) string {
	definition, ok := registry.Resolve(name)
	if !ok {
		return "unknown"
	}
	return definition.Spec.SideEffectLevel
}
