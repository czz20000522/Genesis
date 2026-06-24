package kernel

import "strings"

const (
	ToolEffectClassPureRead           = "pure_read"
	ToolEffectClassStateRead          = "state_read"
	ToolEffectClassWorkspaceWrite     = "workspace_write"
	ToolEffectClassKernelStateWrite   = "kernel_state_write"
	ToolEffectClassProcessStart       = "process_start"
	ToolEffectClassProcessIO          = "process_io"
	ToolEffectClassExternalSideEffect = "external_side_effect"

	ToolParallelPolicyCompatibleLocks          = "compatible_locks"
	ToolParallelPolicySerialFence              = "serial_fence"
	ToolParallelPolicyPerHandleSerial          = "per_handle_serial"
	ToolParallelPolicyBackgroundAfterAdmission = "background_after_admission"
)

type ToolAccessPlan struct {
	ToolName          string
	EffectClass       string
	ParallelPolicy    string
	ResourceFootprint ToolResourceFootprint
	Trusted           bool
	Reason            string
}

type ToolExecutionBatch struct {
	CallIndexes []int
	Parallel    bool
	Reason      string
}

func shellExecToolSchedulingSpec() ToolSchedulingSpec {
	return ToolSchedulingSpec{
		EffectClass:       ToolEffectClassWorkspaceWrite,
		ParallelPolicy:    ToolParallelPolicySerialFence,
		ResourceFootprint: ToolResourceFootprint{WriteScopes: []string{"workspace"}},
	}
}

func jobControlToolSchedulingSpec() ToolSchedulingSpec {
	return ToolSchedulingSpec{
		EffectClass:    ToolEffectClassProcessIO,
		ParallelPolicy: ToolParallelPolicyPerHandleSerial,
	}
}

func resourceReadToolSchedulingSpec() ToolSchedulingSpec {
	return ToolSchedulingSpec{
		EffectClass:       ToolEffectClassPureRead,
		ParallelPolicy:    ToolParallelPolicyCompatibleLocks,
		ResourceFootprint: ToolResourceFootprint{ReadScopes: []string{"resource"}},
	}
}

func shellExecToolAccessPlan(toolName string, cwd string, timeoutSec int) ToolAccessPlan {
	spec := shellExecToolSchedulingSpec()
	if timeoutSec > maxForegroundShellTimeoutSec {
		spec.EffectClass = ToolEffectClassProcessStart
		spec.ParallelPolicy = ToolParallelPolicyBackgroundAfterAdmission
	}
	scope := strings.TrimSpace(cwd)
	if scope == "" {
		scope = "workspace"
	}
	spec.ResourceFootprint.WriteScopes = []string{scope}
	return ToolAccessPlan{
		ToolName:          strings.TrimSpace(toolName),
		EffectClass:       spec.EffectClass,
		ParallelPolicy:    spec.ParallelPolicy,
		ResourceFootprint: spec.ResourceFootprint,
		Trusted:           true,
	}
}

func resourceReadToolAccessPlan(toolName string, resourceRef string) ToolAccessPlan {
	spec := resourceReadToolSchedulingSpec()
	spec.ResourceFootprint.ReadScopes = []string{"resource:" + strings.TrimSpace(resourceRef)}
	return ToolAccessPlan{
		ToolName:          strings.TrimSpace(toolName),
		EffectClass:       spec.EffectClass,
		ParallelPolicy:    spec.ParallelPolicy,
		ResourceFootprint: spec.ResourceFootprint,
		Trusted:           true,
	}
}

func jobControlToolAccessPlan(toolName string, jobID string) ToolAccessPlan {
	spec := jobControlToolSchedulingSpec()
	spec.ResourceFootprint.Handles = []string{"job:" + strings.TrimSpace(jobID)}
	return ToolAccessPlan{
		ToolName:          strings.TrimSpace(toolName),
		EffectClass:       spec.EffectClass,
		ParallelPolicy:    spec.ParallelPolicy,
		ResourceFootprint: spec.ResourceFootprint,
		Trusted:           true,
	}
}

func planToolExecutionBatches(calls []preparedModelToolCall) []ToolExecutionBatch {
	batches := make([]ToolExecutionBatch, 0, len(calls))
	current := ToolExecutionBatch{}
	currentKind := ""
	currentHandles := map[string]struct{}{}

	flush := func() {
		if len(current.CallIndexes) == 0 {
			return
		}
		current.Parallel = len(current.CallIndexes) > 1
		batches = append(batches, current)
		current = ToolExecutionBatch{}
		currentKind = ""
		currentHandles = map[string]struct{}{}
	}

	for i, call := range calls {
		class := call.accessPlan.parallelClass()
		if class == "" {
			flush()
			batches = append(batches, ToolExecutionBatch{
				CallIndexes: []int{i},
				Reason:      call.accessPlan.serialReason(call.name),
			})
			continue
		}
		handles := normalizedToolSchedulingHandles(call.accessPlan.ResourceFootprint.Handles)
		if currentKind == "" {
			currentKind = class
		}
		if currentKind != class || class == ToolEffectClassProcessIO && hasAnyToolSchedulingHandle(currentHandles, handles) {
			flush()
			currentKind = class
		}
		current.CallIndexes = append(current.CallIndexes, i)
		current.Reason = class
		if class == ToolEffectClassProcessIO {
			for _, handle := range handles {
				currentHandles[handle] = struct{}{}
			}
		}
	}
	flush()
	return batches
}

func (p ToolAccessPlan) parallelClass() string {
	if !p.Trusted {
		return ""
	}
	switch strings.TrimSpace(p.EffectClass) {
	case ToolEffectClassPureRead:
		if strings.TrimSpace(p.ParallelPolicy) == ToolParallelPolicyCompatibleLocks {
			return ToolEffectClassPureRead
		}
	case ToolEffectClassProcessIO:
		if strings.TrimSpace(p.ParallelPolicy) == ToolParallelPolicyPerHandleSerial && len(normalizedToolSchedulingHandles(p.ResourceFootprint.Handles)) != 0 {
			return ToolEffectClassProcessIO
		}
	}
	return ""
}

func (p ToolAccessPlan) serialReason(toolName string) string {
	if strings.TrimSpace(p.Reason) != "" {
		return strings.TrimSpace(p.Reason)
	}
	if !p.Trusted {
		return "missing_or_untrusted_tool_access_plan"
	}
	switch strings.TrimSpace(p.EffectClass) {
	case "":
		return "missing_tool_effect_class"
	case ToolEffectClassStateRead:
		return "state_read_waits_for_prior_committed_facts"
	case ToolEffectClassWorkspaceWrite, ToolEffectClassKernelStateWrite:
		return "write_effect_serial_fence"
	case ToolEffectClassProcessStart:
		return "process_start_serial_admission"
	case ToolEffectClassProcessIO:
		return "process_io_missing_handle_or_policy"
	case ToolEffectClassExternalSideEffect:
		return "external_side_effect_routes_through_owner"
	default:
		return "unknown_tool_effect_class:" + strings.TrimSpace(toolName)
	}
}

func normalizedToolSchedulingHandles(handles []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(handles))
	for _, handle := range handles {
		handle = strings.TrimSpace(handle)
		if handle == "" {
			continue
		}
		if _, ok := seen[handle]; ok {
			continue
		}
		seen[handle] = struct{}{}
		normalized = append(normalized, handle)
	}
	return normalized
}

func hasAnyToolSchedulingHandle(existing map[string]struct{}, handles []string) bool {
	for _, handle := range handles {
		if _, ok := existing[handle]; ok {
			return true
		}
	}
	return false
}
