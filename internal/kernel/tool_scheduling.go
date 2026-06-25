package kernel

import "genesis/internal/kernel/toolruntime"

const (
	ToolEffectClassPureRead           = toolruntime.EffectClassPureRead
	ToolEffectClassStateRead          = toolruntime.EffectClassStateRead
	ToolEffectClassWorkspaceWrite     = toolruntime.EffectClassWorkspaceWrite
	ToolEffectClassKernelStateWrite   = toolruntime.EffectClassKernelStateWrite
	ToolEffectClassProcessStart       = toolruntime.EffectClassProcessStart
	ToolEffectClassProcessIO          = toolruntime.EffectClassProcessIO
	ToolEffectClassExternalSideEffect = toolruntime.EffectClassExternalSideEffect

	ToolParallelPolicyCompatibleLocks          = toolruntime.ParallelPolicyCompatibleLocks
	ToolParallelPolicySerialFence              = toolruntime.ParallelPolicySerialFence
	ToolParallelPolicyPerHandleSerial          = toolruntime.ParallelPolicyPerHandleSerial
	ToolParallelPolicyBackgroundAfterAdmission = toolruntime.ParallelPolicyBackgroundAfterAdmission
)

type ToolAccessPlan = toolruntime.AccessPlan
type ToolExecutionBatch = toolruntime.ExecutionBatch

func shellExecToolSchedulingSpec() ToolSchedulingSpec {
	return toolruntime.ShellExecSchedulingSpec()
}

func jobControlToolSchedulingSpec() ToolSchedulingSpec {
	return toolruntime.JobControlSchedulingSpec()
}

func resourceReadToolSchedulingSpec() ToolSchedulingSpec {
	return toolruntime.ResourceReadSchedulingSpec()
}

func shellExecToolAccessPlan(toolName string, cwd string, timeoutSec int) ToolAccessPlan {
	return toolruntime.ShellExecAccessPlan(toolName, cwd, timeoutSec, maxForegroundShellTimeoutSec)
}

func resourceReadToolAccessPlan(toolName string, resourceRef string) ToolAccessPlan {
	return toolruntime.ResourceReadAccessPlan(toolName, resourceRef)
}

func jobControlToolAccessPlan(toolName string, jobID string) ToolAccessPlan {
	return toolruntime.JobControlAccessPlan(toolName, jobID)
}

func planToolExecutionBatches(calls []preparedModelToolCall) []ToolExecutionBatch {
	planned := make([]toolruntime.PlannedCall, 0, len(calls))
	for _, call := range calls {
		planned = append(planned, toolruntime.PlannedCall{
			Name:       call.name,
			AccessPlan: call.accessPlan,
		})
	}
	return toolruntime.PlanExecutionBatches(planned)
}
