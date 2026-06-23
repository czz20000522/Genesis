package kernel

import "strings"

const (
	AuthorityPolicyReadOnly       = "read_only"
	AuthorityPolicyWorkspaceWrite = "workspace_write"
	AuthorityPolicyFullAccess     = "full_access"
	AuthorityPolicyUnknown        = "unknown"

	SandboxProfileReadOnly            = "read_only"
	SandboxProfileControlledWorkspace = "controlled_workspace"
	SandboxProfileOSWorkspace         = "os_workspace"
	SandboxProfileHost                = "host"
	SandboxProfileNone                = "none"

	ApprovalPolicyNever     = "never"
	ApprovalPolicyOnRequest = "on_request"
)

type ResolvedToolPolicy struct {
	PermissionMode  string
	AuthorityPolicy string
	SandboxProfile  string
	ApprovalPolicy  string
	WorkspaceRoot   string
	Known           bool
	InvalidReason   string
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
	if resolved.InvalidReason != "" {
		return toolAuthorizationDecision{Reason: resolved.InvalidReason}
	}
	switch spec.SideEffectLevel {
	case ToolSideEffectRead:
		return toolAuthorizationDecision{Allowed: true}
	case ToolSideEffectWrite:
		switch resolved.AuthorityPolicy {
		case AuthorityPolicyReadOnly:
			return toolAuthorizationDecision{Reason: "blocked_by_permission_mode=plan"}
		case AuthorityPolicyWorkspaceWrite, AuthorityPolicyFullAccess:
			if resolved.ApprovalPolicy == ApprovalPolicyOnRequest {
				return toolAuthorizationDecision{Reason: "approval_required"}
			}
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
	applyToolPolicyOverrides(&resolved, policy)
	return resolved
}

func applyToolPolicyOverrides(resolved *ResolvedToolPolicy, policy ToolPolicy) {
	if resolved == nil || !resolved.Known {
		return
	}
	if sandboxProfile := normalizedPolicyValue(policy.SandboxProfile); sandboxProfile != "" {
		applySandboxProfileOverride(resolved, sandboxProfile)
	}
	if approvalPolicy := normalizedPolicyValue(policy.ApprovalPolicy); approvalPolicy != "" {
		applyApprovalPolicyOverride(resolved, approvalPolicy)
	}
}

func applySandboxProfileOverride(resolved *ResolvedToolPolicy, sandboxProfile string) {
	switch sandboxProfile {
	case SandboxProfileReadOnly, SandboxProfileControlledWorkspace, SandboxProfileOSWorkspace, SandboxProfileHost:
		resolved.SandboxProfile = sandboxProfile
	default:
		resolved.SandboxProfile = SandboxProfileNone
		resolved.InvalidReason = "unknown_sandbox_profile"
		return
	}
	if !sandboxProfileAllowedForAuthority(resolved.AuthorityPolicy, sandboxProfile) {
		resolved.InvalidReason = "sandbox_profile_not_allowed_for_permission_mode"
		return
	}
	if sandboxProfile == SandboxProfileOSWorkspace {
		resolved.InvalidReason = "sandbox_profile_unavailable=os_workspace"
	}
}

func applyApprovalPolicyOverride(resolved *ResolvedToolPolicy, approvalPolicy string) {
	switch approvalPolicy {
	case ApprovalPolicyNever, ApprovalPolicyOnRequest:
		resolved.ApprovalPolicy = approvalPolicy
	default:
		resolved.ApprovalPolicy = approvalPolicy
		resolved.InvalidReason = "unknown_approval_policy"
	}
}

func sandboxProfileAllowedForAuthority(authorityPolicy string, sandboxProfile string) bool {
	switch authorityPolicy {
	case AuthorityPolicyReadOnly:
		return sandboxProfile == SandboxProfileReadOnly
	case AuthorityPolicyWorkspaceWrite:
		return sandboxProfile == SandboxProfileControlledWorkspace ||
			sandboxProfile == SandboxProfileOSWorkspace
	case AuthorityPolicyFullAccess:
		return sandboxProfile == SandboxProfileControlledWorkspace ||
			sandboxProfile == SandboxProfileOSWorkspace ||
			sandboxProfile == SandboxProfileHost
	default:
		return false
	}
}

func normalizedPolicyValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
