package toolruntime

import "strings"

const (
	EffectClassPureRead           = "pure_read"
	EffectClassStateRead          = "state_read"
	EffectClassWorkspaceWrite     = "workspace_write"
	EffectClassKernelStateWrite   = "kernel_state_write"
	EffectClassProcessStart       = "process_start"
	EffectClassProcessIO          = "process_io"
	EffectClassExternalSideEffect = "external_side_effect"

	ParallelPolicyCompatibleLocks          = "compatible_locks"
	ParallelPolicySerialFence              = "serial_fence"
	ParallelPolicyPerHandleSerial          = "per_handle_serial"
	ParallelPolicyBackgroundAfterAdmission = "background_after_admission"
)

type SchedulingSpec struct {
	EffectClass       string            `json:"-"`
	ResourceFootprint ResourceFootprint `json:"-"`
	ParallelPolicy    string            `json:"-"`
}

type ResourceFootprint struct {
	ReadScopes      []string `json:"-"`
	WriteScopes     []string `json:"-"`
	StateScopes     []string `json:"-"`
	Handles         []string `json:"-"`
	ExternalTargets []string `json:"-"`
}

type AccessPlan struct {
	ToolName          string
	EffectClass       string
	ParallelPolicy    string
	ResourceFootprint ResourceFootprint
	Trusted           bool
	Reason            string
}

type PlannedCall struct {
	Name       string
	AccessPlan AccessPlan
}

type ExecutionBatch struct {
	CallIndexes []int
	Parallel    bool
	Reason      string
}

func ShellExecSchedulingSpec() SchedulingSpec {
	return SchedulingSpec{
		EffectClass:       EffectClassWorkspaceWrite,
		ParallelPolicy:    ParallelPolicySerialFence,
		ResourceFootprint: ResourceFootprint{WriteScopes: []string{"workspace"}},
	}
}

func JobControlSchedulingSpec() SchedulingSpec {
	return SchedulingSpec{
		EffectClass:    EffectClassProcessIO,
		ParallelPolicy: ParallelPolicyPerHandleSerial,
	}
}

func ResourceReadSchedulingSpec() SchedulingSpec {
	return SchedulingSpec{
		EffectClass:       EffectClassPureRead,
		ParallelPolicy:    ParallelPolicyCompatibleLocks,
		ResourceFootprint: ResourceFootprint{ReadScopes: []string{"resource"}},
	}
}

func ShellExecAccessPlan(toolName string, cwd string, timeoutSec int, maxForegroundTimeoutSec int) AccessPlan {
	spec := ShellExecSchedulingSpec()
	if timeoutSec > maxForegroundTimeoutSec {
		spec.EffectClass = EffectClassProcessStart
		spec.ParallelPolicy = ParallelPolicyBackgroundAfterAdmission
	}
	scope := strings.TrimSpace(cwd)
	if scope == "" {
		scope = "workspace"
	}
	spec.ResourceFootprint.WriteScopes = []string{scope}
	return AccessPlan{
		ToolName:          strings.TrimSpace(toolName),
		EffectClass:       spec.EffectClass,
		ParallelPolicy:    spec.ParallelPolicy,
		ResourceFootprint: spec.ResourceFootprint,
		Trusted:           true,
	}
}

func ResourceReadAccessPlan(toolName string, resourceRef string) AccessPlan {
	spec := ResourceReadSchedulingSpec()
	spec.ResourceFootprint.ReadScopes = []string{"resource:" + strings.TrimSpace(resourceRef)}
	return AccessPlan{
		ToolName:          strings.TrimSpace(toolName),
		EffectClass:       spec.EffectClass,
		ParallelPolicy:    spec.ParallelPolicy,
		ResourceFootprint: spec.ResourceFootprint,
		Trusted:           true,
	}
}

func JobControlAccessPlan(toolName string, jobID string) AccessPlan {
	spec := JobControlSchedulingSpec()
	spec.ResourceFootprint.Handles = []string{"job:" + strings.TrimSpace(jobID)}
	return AccessPlan{
		ToolName:          strings.TrimSpace(toolName),
		EffectClass:       spec.EffectClass,
		ParallelPolicy:    spec.ParallelPolicy,
		ResourceFootprint: spec.ResourceFootprint,
		Trusted:           true,
	}
}

func PlanExecutionBatches(calls []PlannedCall) []ExecutionBatch {
	batches := make([]ExecutionBatch, 0, len(calls))
	current := ExecutionBatch{}
	currentKind := ""
	currentHandles := map[string]struct{}{}

	flush := func() {
		if len(current.CallIndexes) == 0 {
			return
		}
		current.Parallel = len(current.CallIndexes) > 1 && current.Reason == EffectClassPureRead
		batches = append(batches, current)
		current = ExecutionBatch{}
		currentKind = ""
		currentHandles = map[string]struct{}{}
	}

	for i, call := range calls {
		class := call.AccessPlan.ParallelClass()
		if class == "" {
			flush()
			batches = append(batches, ExecutionBatch{
				CallIndexes: []int{i},
				Reason:      call.AccessPlan.SerialReason(call.Name),
			})
			continue
		}
		handles := normalizedHandles(call.AccessPlan.ResourceFootprint.Handles)
		if currentKind == "" {
			currentKind = class
		}
		if currentKind != class || class == EffectClassProcessIO && hasAnyHandle(currentHandles, handles) {
			flush()
			currentKind = class
		}
		current.CallIndexes = append(current.CallIndexes, i)
		current.Reason = class
		if class == EffectClassProcessIO {
			for _, handle := range handles {
				currentHandles[handle] = struct{}{}
			}
		}
	}
	flush()
	return batches
}

func (p AccessPlan) ParallelClass() string {
	if !p.Trusted {
		return ""
	}
	switch strings.TrimSpace(p.EffectClass) {
	case EffectClassPureRead:
		if strings.TrimSpace(p.ParallelPolicy) == ParallelPolicyCompatibleLocks {
			return EffectClassPureRead
		}
	case EffectClassProcessIO:
		if strings.TrimSpace(p.ParallelPolicy) == ParallelPolicyPerHandleSerial && len(normalizedHandles(p.ResourceFootprint.Handles)) != 0 {
			return EffectClassProcessIO
		}
	}
	return ""
}

func (p AccessPlan) SerialReason(toolName string) string {
	if strings.TrimSpace(p.Reason) != "" {
		return strings.TrimSpace(p.Reason)
	}
	if !p.Trusted {
		return "missing_or_untrusted_tool_access_plan"
	}
	switch strings.TrimSpace(p.EffectClass) {
	case "":
		return "missing_tool_effect_class"
	case EffectClassStateRead:
		return "state_read_waits_for_prior_committed_facts"
	case EffectClassWorkspaceWrite, EffectClassKernelStateWrite:
		return "write_effect_serial_fence"
	case EffectClassProcessStart:
		return "process_start_serial_admission"
	case EffectClassProcessIO:
		return "process_io_missing_handle_or_policy"
	case EffectClassExternalSideEffect:
		return "external_side_effect_routes_through_owner"
	default:
		return "unknown_tool_effect_class:" + strings.TrimSpace(toolName)
	}
}

func normalizedHandles(handles []string) []string {
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

func hasAnyHandle(existing map[string]struct{}, handles []string) bool {
	for _, handle := range handles {
		if _, ok := existing[handle]; ok {
			return true
		}
	}
	return false
}
