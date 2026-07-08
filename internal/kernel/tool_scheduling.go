package kernel

import (
	"strings"

	"genesis/internal/kernel/toolruntime"
)

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

func workspaceEditToolSchedulingSpec() ToolSchedulingSpec {
	return ToolSchedulingSpec{
		EffectClass:       ToolEffectClassWorkspaceWrite,
		ParallelPolicy:    ToolParallelPolicySerialFence,
		ResourceFootprint: ToolResourceFootprint{WriteScopes: []string{"workspace"}},
	}
}

func jobControlToolSchedulingSpec() ToolSchedulingSpec {
	return toolruntime.JobControlSchedulingSpec()
}

func resourceReadToolSchedulingSpec() ToolSchedulingSpec {
	return toolruntime.ResourceReadSchedulingSpec()
}

func contextDiscoveryToolSchedulingSpec() ToolSchedulingSpec {
	return toolruntime.SchedulingSpec{
		EffectClass:    ToolEffectClassStateRead,
		ParallelPolicy: ToolParallelPolicySerialFence,
		ResourceFootprint: ToolResourceFootprint{
			StateScopes: []string{"discovery"},
		},
	}
}

func shellExecToolAccessPlan(toolName string, cwd string, timeoutSec int) ToolAccessPlan {
	return toolruntime.ShellExecAccessPlan(toolName, cwd, timeoutSec, maxForegroundShellTimeoutSec)
}

func (k *Kernel) shellExecAccessPlan(toolName string, cwd string, timeoutSec int) ToolAccessPlan {
	return toolruntime.ShellExecAccessPlan(toolName, cwd, timeoutSec, k.shellTimeoutPolicy.ManagedJobThresholdSec)
}

func workspaceEditToolAccessPlan(toolName string, path string) ToolAccessPlan {
	spec := workspaceEditToolSchedulingSpec()
	scope := strings.TrimSpace(path)
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
	return toolruntime.ResourceReadAccessPlan(toolName, resourceRef)
}

func sourceReadToolAccessPlan(toolName string, sourceRef string) ToolAccessPlan {
	plan := toolruntime.ResourceReadAccessPlan(toolName, "source:"+sourceRef)
	plan.ResourceFootprint.ReadScopes = []string{"source:" + sourceRef}
	return plan
}

func contextDiscoveryToolAccessPlan(toolName string) ToolAccessPlan {
	spec := contextDiscoveryToolSchedulingSpec()
	return ToolAccessPlan{
		ToolName:          toolName,
		EffectClass:       spec.EffectClass,
		ParallelPolicy:    spec.ParallelPolicy,
		ResourceFootprint: spec.ResourceFootprint,
		Trusted:           true,
	}
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
