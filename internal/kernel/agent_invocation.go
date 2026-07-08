package kernel

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrAgentInvocationNotFound = errors.New("agent invocation not found")

func (k *Kernel) AdmitAgentInvocation(req AgentInvocationAdmissionRequest) (AgentInvocationProjection, error) {
	if err := validateAgentInvocationAdmissionRequest(req); err != nil {
		return AgentInvocationProjection{}, err
	}
	k.workMu.Lock()
	defer k.workMu.Unlock()

	sessionID := strings.TrimSpace(req.SessionID)
	if key := strings.TrimSpace(req.IdempotencyKey); key != "" {
		existing, ok, err := k.agentInvocationByIdempotencyKey(sessionID, key)
		if err != nil {
			return AgentInvocationProjection{}, err
		}
		if ok {
			return existing, nil
		}
	}
	invocations, err := k.agentInvocations()
	if err != nil {
		return AgentInvocationProjection{}, err
	}
	parentInvocationID := strings.TrimSpace(req.ParentInvocationID)
	var parent *AgentInvocationProjection
	if parentInvocationID != "" {
		parentProjection, ok := invocations[parentInvocationID]
		if !ok {
			return AgentInvocationProjection{}, errors.New("parent_invocation_not_found")
		}
		if parentProjection.SessionID != sessionID {
			return AgentInvocationProjection{}, errors.New("parent_invocation_session_mismatch")
		}
		parent = &parentProjection
	}
	grant, err := k.admitCapabilityGrant(req.CapabilityGrant, parent)
	if err != nil {
		return AgentInvocationProjection{}, err
	}
	now := k.clock()
	invocation := AgentInvocationProjection{
		InvocationID:        newID("invocation", now),
		SessionID:           sessionID,
		ParentInvocationID:  parentInvocationID,
		Principal:           strings.TrimSpace(req.Principal),
		AgentProfileRef:     strings.TrimSpace(req.AgentProfileRef),
		CapabilityGrant:     grant,
		ContextScope:        strings.TrimSpace(req.ContextScope),
		ParentResultChannel: strings.TrimSpace(req.ParentResultChannel),
		Status:              AgentInvocationStatusAdmitted,
		IdempotencyKey:      strings.TrimSpace(req.IdempotencyKey),
		AdmittedAt:          now,
	}
	if err := k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: invocation.SessionID,
		Type:      "agent_invocation.admitted",
		CreatedAt: now,
		Data: EventData{
			AgentInvocation: &invocation,
		},
	}); err != nil {
		return AgentInvocationProjection{}, err
	}
	return invocation, nil
}

func (k *Kernel) AgentInvocation(invocationID string) (AgentInvocationProjection, error) {
	invocationID = strings.TrimSpace(invocationID)
	if invocationID == "" {
		return AgentInvocationProjection{}, errors.New("invocation_id is required")
	}
	invocations, err := k.agentInvocations()
	if err != nil {
		return AgentInvocationProjection{}, err
	}
	invocation, ok := invocations[invocationID]
	if !ok {
		return AgentInvocationProjection{}, ErrAgentInvocationNotFound
	}
	return invocation, nil
}

func (k *Kernel) AgentInvocations(sessionID string) ([]AgentInvocationProjection, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session_id is required")
	}
	invocations, err := k.agentInvocations()
	if err != nil {
		return nil, err
	}
	items := make([]AgentInvocationProjection, 0, len(invocations))
	for _, invocation := range invocations {
		if invocation.SessionID == sessionID {
			items = append(items, invocation)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].AdmittedAt.Equal(items[j].AdmittedAt) {
			return items[i].InvocationID < items[j].InvocationID
		}
		return items[i].AdmittedAt.Before(items[j].AdmittedAt)
	})
	return items, nil
}

func validateAgentInvocationAdmissionRequest(req AgentInvocationAdmissionRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return errors.New("session_id is required")
	}
	if strings.TrimSpace(req.Principal) == "" {
		return errors.New("principal is required")
	}
	if err := validateKernelControlToken("session_id", req.SessionID); err != nil {
		return err
	}
	if err := validateKernelAuthority("principal", req.Principal); err != nil {
		return err
	}
	if err := validateKernelControlToken("parent_invocation_id", req.ParentInvocationID); err != nil {
		return err
	}
	if err := validateKernelRefIfPresent("agent_profile_ref", req.AgentProfileRef); err != nil {
		return err
	}
	if err := validateKernelControlToken("context_scope", req.ContextScope); err != nil {
		return err
	}
	if err := validateKernelRefIfPresent("parent_result_channel", req.ParentResultChannel); err != nil {
		return err
	}
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return err
	}
	return nil
}

func validateKernelRefIfPresent(field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return validateKernelRef(field, value)
}

func (k *Kernel) admitCapabilityGrant(grant CapabilityGrant, parent *AgentInvocationProjection) (CapabilityGrant, error) {
	normalized := normalizeCapabilityGrant(grant)
	if err := k.validateGrantToolsAllowed(normalized); err != nil {
		return CapabilityGrant{}, err
	}
	if parent != nil {
		if err := validateGrantSubset(normalized, parent.CapabilityGrant); err != nil {
			return CapabilityGrant{}, err
		}
	}
	return normalized, nil
}

func normalizeCapabilityGrant(grant CapabilityGrant) CapabilityGrant {
	seen := map[string]struct{}{}
	tools := make([]string, 0, len(grant.ToolNames))
	for _, tool := range grant.ToolNames {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}
		if _, ok := seen[tool]; ok {
			continue
		}
		seen[tool] = struct{}{}
		tools = append(tools, tool)
	}
	sort.Strings(tools)
	return CapabilityGrant{ToolNames: tools}
}

func (k *Kernel) validateGrantToolsAllowed(grant CapabilityGrant) error {
	for _, toolName := range grant.ToolNames {
		tool, ok := k.toolRegistry.Resolve(toolName)
		if !ok {
			return fmt.Errorf("capability_grant_unknown_tool: %s", toolName)
		}
		decision := authorizeKernelTool(k.toolPolicy, tool.Spec)
		if !decision.Allowed {
			return fmt.Errorf("capability_grant_tool_not_allowed: %s", toolName)
		}
	}
	return nil
}

func validateGrantSubset(child CapabilityGrant, parent CapabilityGrant) error {
	parentTools := map[string]struct{}{}
	for _, toolName := range parent.ToolNames {
		parentTools[toolName] = struct{}{}
	}
	for _, toolName := range child.ToolNames {
		if _, ok := parentTools[toolName]; !ok {
			return fmt.Errorf("capability_grant_exceeds_parent: %s", toolName)
		}
	}
	return nil
}

func (k *Kernel) agentInvocationByIdempotencyKey(sessionID string, key string) (AgentInvocationProjection, bool, error) {
	invocations, err := k.agentInvocations()
	if err != nil {
		return AgentInvocationProjection{}, false, err
	}
	for _, invocation := range invocations {
		if invocation.SessionID == sessionID && invocation.IdempotencyKey == key {
			return invocation, true, nil
		}
	}
	return AgentInvocationProjection{}, false, nil
}

func (k *Kernel) agentInvocations() (map[string]AgentInvocationProjection, error) {
	events, err := k.loadEvents()
	if err != nil {
		return nil, err
	}
	invocations := map[string]AgentInvocationProjection{}
	for _, event := range events {
		if event.Type != "agent_invocation.admitted" || event.Data.AgentInvocation == nil {
			continue
		}
		invocation := *event.Data.AgentInvocation
		if invocation.InvocationID == "" {
			return nil, errors.New("agent_invocation.admitted event missing invocation id")
		}
		if invocation.SessionID == "" {
			invocation.SessionID = event.SessionID
		}
		if invocation.AdmittedAt.IsZero() {
			invocation.AdmittedAt = event.CreatedAt
		}
		invocation.CapabilityGrant = normalizeCapabilityGrant(invocation.CapabilityGrant)
		current, exists := invocations[invocation.InvocationID]
		if exists && !sameAgentInvocation(current, invocation) {
			return nil, fmt.Errorf("competing agent invocation fact for %s", invocation.InvocationID)
		}
		invocations[invocation.InvocationID] = invocation
	}
	return invocations, nil
}

func sameAgentInvocation(left AgentInvocationProjection, right AgentInvocationProjection) bool {
	return left.InvocationID == right.InvocationID &&
		left.SessionID == right.SessionID &&
		left.ParentInvocationID == right.ParentInvocationID &&
		left.Principal == right.Principal &&
		left.AgentProfileRef == right.AgentProfileRef &&
		left.ContextScope == right.ContextScope &&
		left.ParentResultChannel == right.ParentResultChannel &&
		left.Status == right.Status &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.AdmittedAt.Equal(right.AdmittedAt) &&
		sameStringSet(left.CapabilityGrant.ToolNames, right.CapabilityGrant.ToolNames)
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
