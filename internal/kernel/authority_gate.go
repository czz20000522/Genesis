package kernel

import "strings"

const (
	AuthorityPolicyReadOnly       = "read_only"
	AuthorityPolicyWorkspaceWrite = "workspace_write"
	AuthorityPolicyFullAccess     = "full_access"
	AuthorityPolicyUnknown        = "unknown"

	SandboxProfileReadOnly            = "read_only"
	SandboxProfileControlledWorkspace = "controlled_workspace"
	SandboxProfileHost                = "host"
	SandboxProfileNone                = "none"

	ApprovalPolicyNever = "never"
)

type ResolvedToolPolicy struct {
	PermissionMode  string
	AuthorityPolicy string
	SandboxProfile  string
	ApprovalPolicy  string
	WorkspaceRoot   string
	Known           bool
}

type toolAuthorizationDecision struct {
	Allowed bool
	Reason  string
}

func authorizeKernelTool(policy ToolPolicy, spec ToolSpec) toolAuthorizationDecision {
	resolved := resolveToolPolicy(policy)
	if !resolved.Known {
		return toolAuthorizationDecision{Reason: "unknown_permission_mode"}
	}
	switch spec.SideEffectLevel {
	case ToolSideEffectRead:
		return toolAuthorizationDecision{Allowed: true}
	case ToolSideEffectWrite:
		switch resolved.AuthorityPolicy {
		case AuthorityPolicyReadOnly:
			return toolAuthorizationDecision{Reason: "blocked_by_permission_mode=plan"}
		case AuthorityPolicyWorkspaceWrite, AuthorityPolicyFullAccess:
			return toolAuthorizationDecision{Allowed: true}
		default:
			return toolAuthorizationDecision{Reason: "unknown_permission_mode"}
		}
	default:
		return toolAuthorizationDecision{Reason: "unknown_tool_kind"}
	}
}

func resolveToolPolicy(policy ToolPolicy) ResolvedToolPolicy {
	mode := normalizedPermissionMode(policy.PermissionMode)
	resolved := ResolvedToolPolicy{
		PermissionMode: strings.TrimSpace(mode),
		WorkspaceRoot:  strings.TrimSpace(policy.WorkspaceRoot),
		ApprovalPolicy: ApprovalPolicyNever,
	}
	switch mode {
	case PermissionModePlan:
		resolved.AuthorityPolicy = AuthorityPolicyReadOnly
		resolved.SandboxProfile = SandboxProfileReadOnly
		resolved.Known = true
	case PermissionModeDefault:
		resolved.AuthorityPolicy = AuthorityPolicyWorkspaceWrite
		resolved.SandboxProfile = SandboxProfileControlledWorkspace
		resolved.Known = true
	case PermissionModeYolo:
		resolved.AuthorityPolicy = AuthorityPolicyFullAccess
		resolved.SandboxProfile = SandboxProfileHost
		resolved.Known = true
	default:
		resolved.AuthorityPolicy = AuthorityPolicyUnknown
		resolved.SandboxProfile = SandboxProfileNone
	}
	return resolved
}
