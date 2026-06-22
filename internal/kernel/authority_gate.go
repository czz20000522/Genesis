package kernel

type toolAuthorizationDecision struct {
	Allowed bool
	Reason  string
}

func authorizeKernelTool(policy ToolPolicy, definition kernelToolDefinition) toolAuthorizationDecision {
	switch definition.Kind {
	case ToolKindRead:
		return toolAuthorizationDecision{Allowed: true}
	case ToolKindEffect:
		switch policy.PermissionMode {
		case PermissionModePlan:
			return toolAuthorizationDecision{Reason: "blocked_by_permission_mode=plan"}
		case PermissionModeDefault, PermissionModeYolo:
			return toolAuthorizationDecision{Allowed: true}
		default:
			return toolAuthorizationDecision{Reason: "unknown_permission_mode"}
		}
	default:
		return toolAuthorizationDecision{Reason: "unknown_tool_kind"}
	}
}
