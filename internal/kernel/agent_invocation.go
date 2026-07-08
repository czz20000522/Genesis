package kernel

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrAgentInvocationNotFound        = errors.New("agent invocation not found")
	ErrAgentInvocationAlreadyRunning  = errors.New("agent invocation already running")
	ErrAgentInvocationAlreadyTerminal = errors.New("agent invocation already terminal")
)

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

func (k *Kernel) AdmitWorkerInvocationFromRole(req WorkerInvocationAdmissionRequest) (AgentInvocationProjection, error) {
	runtime, err := ResolveParentWorkerRuntimeFromGenesis(ParentWorkerRuntimeRequest{
		ConfigRoot: req.ConfigRoot,
		ParentID:   req.ParentID,
	})
	if err != nil {
		return AgentInvocationProjection{}, err
	}
	if !runtime.Parent.CanCreateWorkers {
		return AgentInvocationProjection{}, errors.New("parent_worker_creation_denied")
	}
	roleID := strings.TrimSpace(req.RoleID)
	if roleID == "" {
		roleID = strings.TrimSpace(runtime.Parent.DefaultWorkerRole)
	}
	worker, ok := workerRoleByID(runtime.WorkerRoles, roleID)
	if !ok {
		return AgentInvocationProjection{}, ErrGenesisWorkerRoleBindingMissing
	}
	if err := validateRequestedWorkerTools(req.RequestedToolNames, worker.ToolSet); err != nil {
		return AgentInvocationProjection{}, err
	}
	contextScope := strings.TrimSpace(req.ContextScope)
	if contextScope == "" {
		contextScope = strings.TrimSpace(worker.ContextPolicyRef)
	}
	return k.AdmitAgentInvocation(AgentInvocationAdmissionRequest{
		SessionID:           req.SessionID,
		ParentInvocationID:  req.ParentInvocationID,
		Principal:           req.Principal,
		AgentProfileRef:     "agent_profile:" + worker.RoleID,
		CapabilityGrant:     CapabilityGrant{ToolNames: worker.ToolSet},
		ContextScope:        contextScope,
		ParentResultChannel: req.ParentResultChannel,
		IdempotencyKey:      req.IdempotencyKey,
	})
}

func (k *Kernel) RunAgentInvocation(ctx context.Context, req AgentInvocationRunRequest) (AgentInvocationRunProjection, error) {
	if err := validateAgentInvocationRunRequest(req); err != nil {
		return AgentInvocationRunProjection{}, err
	}
	invocationID := strings.TrimSpace(req.InvocationID)
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	existing, ok, err := k.agentInvocationRunByKey(invocationID, idempotencyKey)
	if err != nil {
		return AgentInvocationRunProjection{}, err
	}
	if ok {
		return existing, nil
	}
	if terminal, ok, err := k.terminalAgentInvocationRun(invocationID); err != nil {
		return AgentInvocationRunProjection{}, err
	} else if ok && idempotencyKey != "" && terminal.IdempotencyKey != idempotencyKey {
		return AgentInvocationRunProjection{}, ErrAgentInvocationAlreadyTerminal
	}
	if err := k.beginActiveInvocationRun(invocationID); err != nil {
		return AgentInvocationRunProjection{}, err
	}
	defer k.finishActiveInvocationRun(invocationID)

	invocation, err := k.AgentInvocation(invocationID)
	if err != nil {
		return AgentInvocationRunProjection{}, err
	}
	now := k.clock()
	inputs := modelInputItems(req.InputItems)
	run := AgentInvocationRunProjection{
		InvocationID:    invocation.InvocationID,
		RunID:           newID("agent_run", now),
		SessionID:       invocation.SessionID,
		Principal:       strings.TrimSpace(req.Principal),
		Status:          AgentInvocationRunStatusRunning,
		ModelInputKinds: modelInputKinds(inputs),
		IdempotencyKey:  idempotencyKey,
		StartedAt:       now,
	}
	if err := k.appendAgentInvocationRunEvent("agent_invocation.run_started", run); err != nil {
		return AgentInvocationRunProjection{}, err
	}
	toolGateway, err := k.ToolGatewayForInvocation(invocation.InvocationID)
	if err != nil {
		failed := k.failedAgentInvocationRun(run, err)
		_ = k.appendAgentInvocationRunEvent("agent_invocation.run_failed", failed)
		return failed, err
	}
	final, err := k.runAgentInvocationLoop(ctx, run, inputs, toolGateway)
	if err != nil {
		failed := k.failedAgentInvocationRun(run, err)
		if appendErr := k.appendAgentInvocationRunEvent("agent_invocation.run_failed", failed); appendErr != nil {
			return AgentInvocationRunProjection{}, appendErr
		}
		return failed, err
	}
	completed := run
	completed.Status = AgentInvocationRunStatusCompleted
	completed.Model = final.Model
	completed.Usage = final.Usage
	completed.Final = final
	completed.CompletedAt = k.clock()
	if err := k.appendAgentInvocationRunEvent("agent_invocation.run_completed", completed); err != nil {
		return AgentInvocationRunProjection{}, err
	}
	return completed, nil
}

func (k *Kernel) AgentInvocationRun(runID string) (AgentInvocationRunProjection, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return AgentInvocationRunProjection{}, errors.New("run_id is required")
	}
	runs, err := k.agentInvocationRuns()
	if err != nil {
		return AgentInvocationRunProjection{}, err
	}
	run, ok := runs[runID]
	if !ok {
		return AgentInvocationRunProjection{}, errors.New("agent_invocation_run_not_found")
	}
	return run, nil
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

func validateAgentInvocationRunRequest(req AgentInvocationRunRequest) error {
	if strings.TrimSpace(req.InvocationID) == "" {
		return errors.New("invocation_id is required")
	}
	if strings.TrimSpace(req.Principal) == "" {
		return errors.New("principal is required")
	}
	if err := validateKernelControlToken("invocation_id", req.InvocationID); err != nil {
		return err
	}
	if err := validateKernelAuthority("principal", req.Principal); err != nil {
		return err
	}
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return err
	}
	if len(req.InputItems) == 0 {
		return errors.New("input_items is required")
	}
	for i, item := range req.InputItems {
		if item.Type != "text" {
			return fmt.Errorf("input_items[%d].type must be text", i)
		}
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("input_items[%d].text is required", i)
		}
	}
	return nil
}

func (k *Kernel) runAgentInvocationLoop(ctx context.Context, run AgentInvocationRunProjection, inputs []ModelInputItem, toolGateway ToolGateway) (FinalMessage, error) {
	toolRounds := []ModelToolRound{}
	loopGuard := newToolLoopGuard()
	budgetLease := k.newTurnBudgetLease()
	for roundIndex := 0; ; roundIndex++ {
		modelResp, _, err := k.completeModel(ctx, ModelRequest{
			SessionID:    run.SessionID,
			TurnID:       run.RunID,
			InputItems:   inputs,
			ToolManifest: toolGateway.ToolManifest(),
			ToolRounds:   cloneModelToolRounds(toolRounds),
		}, nil)
		if err != nil {
			return FinalMessage{}, providerCompleteError(err)
		}
		if len(modelResp.ToolCalls) == 0 {
			return FinalMessage{Text: modelResp.Text, Model: modelResp.Model, Usage: modelResp.Usage}, nil
		}
		if !budgetLease.allowModelToolRound(roundIndex) {
			return FinalMessage{}, errors.New("agent_invocation_tool_loop_budget_exhausted")
		}
		calls, results, err := k.executeAgentInvocationToolCalls(ctx, toolGateway, run, modelResp.ToolCalls, loopGuard)
		if err != nil {
			return FinalMessage{}, err
		}
		toolRounds = append(toolRounds, ModelToolRound{Calls: calls, Results: results})
	}
}

func (k *Kernel) executeAgentInvocationToolCalls(ctx context.Context, toolGateway ToolGateway, run AgentInvocationRunProjection, calls []ModelToolCall, guard *toolLoopGuard) ([]ModelToolCall, []ModelToolResult, error) {
	normalizedCalls, toolCallEventIDs, err := k.appendToolCallEvents(run.SessionID, run.RunID, calls)
	if err != nil {
		return nil, nil, err
	}
	preparedCalls, err := toolGateway.PrepareBatch(normalizedCalls)
	if err != nil {
		return nil, nil, err
	}
	results := make([]ModelToolResult, 0, len(preparedCalls))
	for _, prepared := range preparedCalls {
		if prepared.requestInvalid != nil {
			result, execErr := toolGateway.Execute(ctx, run.SessionID, run.RunID, prepared)
			if execErr != nil {
				return nil, nil, execErr
			}
			if appendErr := k.appendToolResultEvent(run.SessionID, run.RunID, result, toolCallEventIDs[result.ToolCallEventID]); appendErr != nil {
				return nil, nil, appendErr
			}
			return nil, nil, fmt.Errorf("tool_call_rejected: %s", prepared.requestInvalid.Error.Code)
		}
		if result, blocked, err := guardToolLoopBeforeExecution(guard, prepared); err != nil || blocked {
			if err != nil {
				return nil, nil, err
			}
			results = append(results, result)
			if appendErr := k.appendToolResultEvent(run.SessionID, run.RunID, result, toolCallEventIDs[result.ToolCallEventID]); appendErr != nil {
				return nil, nil, appendErr
			}
			continue
		}
		result, err := toolGateway.Execute(ctx, run.SessionID, run.RunID, prepared)
		if err != nil {
			return nil, nil, err
		}
		result, err = observeToolLoopGuardResult(guard, prepared, result)
		if err != nil {
			return nil, nil, err
		}
		if appendErr := k.appendToolResultEvent(run.SessionID, run.RunID, result, toolCallEventIDs[result.ToolCallEventID]); appendErr != nil {
			return nil, nil, appendErr
		}
		results = append(results, result)
	}
	return normalizedCalls, results, nil
}

func (k *Kernel) failedAgentInvocationRun(run AgentInvocationRunProjection, err error) AgentInvocationRunProjection {
	failed := run
	failed.Status = AgentInvocationRunStatusFailed
	failed.CompletedAt = k.clock()
	code := "agent_invocation_failed"
	message := externalBoundaryDiagnosticText(err.Error())
	if strings.Contains(err.Error(), "tool_call_rejected") {
		code = "tool_call_rejected"
	}
	if errors.Is(err, ErrProviderUnavailable) {
		code = "provider_unavailable"
	}
	var classified *ProviderClassifiedError
	if errors.As(err, &classified) && strings.TrimSpace(classified.Code) != "" {
		code = strings.TrimSpace(classified.Code)
	}
	failed.Error = &TurnError{Code: code, Message: message}
	return failed
}

func (k *Kernel) appendAgentInvocationRunEvent(eventType string, run AgentInvocationRunProjection) error {
	now := k.clock()
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: run.SessionID,
		TurnID:    run.RunID,
		Type:      eventType,
		CreatedAt: now,
		Data: EventData{
			AgentInvocationRun: &run,
		},
	})
}

func (k *Kernel) beginActiveInvocationRun(invocationID string) error {
	k.workMu.Lock()
	defer k.workMu.Unlock()
	if k.activeInvocationRuns == nil {
		k.activeInvocationRuns = map[string]struct{}{}
	}
	if _, exists := k.activeInvocationRuns[invocationID]; exists {
		return ErrAgentInvocationAlreadyRunning
	}
	k.activeInvocationRuns[invocationID] = struct{}{}
	return nil
}

func (k *Kernel) finishActiveInvocationRun(invocationID string) {
	k.workMu.Lock()
	defer k.workMu.Unlock()
	delete(k.activeInvocationRuns, invocationID)
}

func workerRoleByID(workers []WorkerRoleBindingProjection, roleID string) (WorkerRoleBindingProjection, bool) {
	roleID = strings.TrimSpace(roleID)
	for _, worker := range workers {
		if worker.RoleID == roleID {
			return worker, true
		}
	}
	return WorkerRoleBindingProjection{}, false
}

func validateRequestedWorkerTools(requested []string, roleTools []string) error {
	grant := normalizeCapabilityGrant(CapabilityGrant{ToolNames: requested})
	if len(grant.ToolNames) == 0 {
		return nil
	}
	roleSet := map[string]struct{}{}
	for _, tool := range roleTools {
		roleSet[tool] = struct{}{}
	}
	for _, tool := range grant.ToolNames {
		if _, ok := roleSet[tool]; !ok {
			return fmt.Errorf("capability_grant_exceeds_role: %s", tool)
		}
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

func (k *Kernel) agentInvocationRunByKey(invocationID string, key string) (AgentInvocationRunProjection, bool, error) {
	if key == "" {
		return AgentInvocationRunProjection{}, false, nil
	}
	runs, err := k.agentInvocationRuns()
	if err != nil {
		return AgentInvocationRunProjection{}, false, err
	}
	for _, run := range runs {
		if run.InvocationID == invocationID && run.IdempotencyKey == key && isTerminalAgentInvocationRun(run) {
			return run, true, nil
		}
	}
	return AgentInvocationRunProjection{}, false, nil
}

func (k *Kernel) terminalAgentInvocationRun(invocationID string) (AgentInvocationRunProjection, bool, error) {
	runs, err := k.agentInvocationRuns()
	if err != nil {
		return AgentInvocationRunProjection{}, false, err
	}
	for _, run := range runs {
		if run.InvocationID == invocationID && isTerminalAgentInvocationRun(run) {
			return run, true, nil
		}
	}
	return AgentInvocationRunProjection{}, false, nil
}

func (k *Kernel) agentInvocationRuns() (map[string]AgentInvocationRunProjection, error) {
	events, err := k.loadEvents()
	if err != nil {
		return nil, err
	}
	runs := map[string]AgentInvocationRunProjection{}
	for _, event := range events {
		if event.Data.AgentInvocationRun == nil {
			continue
		}
		run := *event.Data.AgentInvocationRun
		if run.RunID == "" {
			return nil, errors.New("agent invocation run event missing run id")
		}
		if run.SessionID == "" {
			run.SessionID = event.SessionID
		}
		if run.StartedAt.IsZero() {
			run.StartedAt = event.CreatedAt
		}
		runs[run.RunID] = run
	}
	return runs, nil
}

func isTerminalAgentInvocationRun(run AgentInvocationRunProjection) bool {
	return run.Status == AgentInvocationRunStatusCompleted || run.Status == AgentInvocationRunStatusFailed
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
