package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Kernel struct {
	ledger           Ledger
	provider         Provider
	jobExecutor      ManagedJobExecutor
	runtimeToken     string
	toolPolicy       ToolPolicy
	contextPolicy    ContextPolicy
	toolRegistry     *ToolRegistry
	resourceRegistry *resourceRegistry
	skillCatalog     []SkillDescriptor
	skillExclusions  []SkillCatalogExclusionProjection
	clock            func() time.Time
	turnMu           sync.Mutex
	activeTurnMu     sync.Mutex
	activeTurns      map[string]*activeTurn
	operationMu      sync.Mutex
	jobMu            sync.Mutex
	memoryReviewMu   sync.Mutex
	workMu           sync.Mutex
}

type activeTurn struct {
	sessionID string
	turnID    string
	cancel    context.CancelFunc
	reason    string
}

func New(config Config) (*Kernel, error) {
	if strings.TrimSpace(config.LedgerPath) == "" {
		return nil, errors.New("ledger path is required")
	}
	provider := config.Provider
	if provider == nil {
		provider = FakeProvider{}
	}
	jobExecutor := config.JobExecutor
	if jobExecutor == nil {
		jobExecutor = newLocalManagedJobExecutor()
	}
	clock := config.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	toolRegistry, err := defaultToolRegistry()
	if err != nil {
		return nil, err
	}
	resourceRegistry, err := newResourceRegistry(config.Resources)
	if err != nil {
		return nil, err
	}
	skillCatalog := loadSkillCatalogWithDiagnostics(config.SkillRoots)
	return &Kernel{
		ledger:           NewJSONLLedger(config.LedgerPath),
		provider:         provider,
		jobExecutor:      jobExecutor,
		runtimeToken:     strings.TrimSpace(config.RuntimeToken),
		toolPolicy:       normalizedToolPolicy(config.ToolPolicy),
		contextPolicy:    normalizedContextPolicy(config.ContextPolicy),
		toolRegistry:     toolRegistry,
		resourceRegistry: resourceRegistry,
		skillCatalog:     skillCatalog.Items,
		skillExclusions:  skillCatalog.Exclusions,
		clock:            clock,
		activeTurns:      map[string]*activeTurn{},
	}, nil
}

func (k *Kernel) Close() {
	if closer, ok := k.jobExecutor.(interface{ Close() }); ok {
		closer.Close()
	}
}

func (k *Kernel) Ready() ReadyResponse {
	providerStatus := k.provider.Ready()
	runtimeAuth := ReadyCheck{Status: "ok"}
	if k.runtimeToken == "" {
		runtimeAuth = ReadyCheck{Status: "blocked", Reason: "runtime_token_missing"}
	}
	ledgerStatus := k.ledger.Ready()
	status := "ok"
	if providerStatus.Status != "ok" || runtimeAuth.Status != "ok" || ledgerStatus.Status != "ok" {
		status = "blocked"
	}
	return ReadyResponse{
		Status:      status,
		Provider:    safeProviderStatusForInspection(providerStatus),
		RuntimeAuth: runtimeAuth,
		Ledger:      ledgerStatus,
	}
}

func (k *Kernel) Capabilities() CapabilitiesResponse {
	ready := k.Ready()
	return CapabilitiesResponse{
		Status:       ready.Status,
		Provider:     safeProviderStatusForInspection(ready.Provider),
		RuntimeAuth:  ready.RuntimeAuth,
		Ledger:       ready.Ledger,
		Tools:        k.toolCapabilityProjections(),
		SkillCatalog: k.skillCatalogProjection(),
	}
}

func (k *Kernel) SubmitTurn(ctx context.Context, req TurnRequest) (TurnResponse, error) {
	if err := validateTurnRequest(req); err != nil {
		return TurnResponse{}, err
	}
	ingressRisks, err := scanTurnIngressSecurity(req.InputItems)
	if err != nil {
		return TurnResponse{}, err
	}
	now := k.clock()
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = newID("sess", now)
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	var turnID string
	if idempotencyKey != "" {
		var existing TurnResponse
		var ok bool
		k.turnMu.Lock()
		existing, ok, err = k.turnByIdempotencyKey(sessionID, idempotencyKey)
		if err == nil && !ok {
			turnID = newID("turn", now)
			_, _, err = k.submitNewTurn(req, sessionID, turnID, idempotencyKey, ingressRisks, now)
		}
		k.turnMu.Unlock()
		if err != nil || ok {
			return existing, err
		}
	} else {
		turnID = newID("turn", now)
		_, _, err = k.submitNewTurn(req, sessionID, turnID, "", ingressRisks, now)
		if err != nil {
			return TurnResponse{}, err
		}
	}

	runCtx, finishActiveTurn := k.beginActiveTurn(ctx, sessionID, turnID)
	defer finishActiveTurn()

	toolGateway := k.toolGateway()
	loopGuard := newToolLoopGuard()
	for roundIndex := 0; roundIndex <= maxModelToolRounds; roundIndex++ {
		providerContext, err := k.ProviderContextProjection(turnID)
		if err != nil {
			return TurnResponse{}, err
		}
		modelResp, err := k.completeProviderStep(runCtx, sessionID, turnID, roundIndex, providerContext)
		if err != nil {
			if isTurnContextInterrupted(runCtx, err) {
				return k.completeInterruptedTurn(sessionID, turnID)
			}
			failure := turnFailureFromProviderError(err)
			if appendErr := k.appendTurnFailure(sessionID, turnID, failure); appendErr != nil {
				return TurnResponse{}, appendErr
			}
			return TurnResponse{}, providerCompleteError(err)
		}
		if err := k.appendKernelObservationDelivered(sessionID, turnID, providerContext.KernelObservationEventIDs); err != nil {
			return TurnResponse{}, err
		}
		if err := k.appendModelContextAccounting(sessionID, turnID, roundIndex, providerContext, modelResp); err != nil {
			return TurnResponse{}, err
		}
		if len(modelResp.ToolCalls) == 0 {
			completedAt := k.clock()
			final := FinalMessage{Text: modelResp.Text, Model: modelResp.Model, Usage: modelResp.Usage}
			completed := StoredEvent{
				EventID:   newID("evt", completedAt),
				SessionID: sessionID,
				TurnID:    turnID,
				Type:      "model.final",
				CreatedAt: completedAt,
				Data: EventData{
					Final: &final,
				},
			}
			if err := k.appendEvent(completed); err != nil {
				return TurnResponse{}, err
			}
			k.maybeSubmitAutoContextCompaction(ctx, sessionID, turnID, final)
			events, err := k.TurnEvents(turnID)
			if err != nil {
				return TurnResponse{}, err
			}
			return TurnResponse{
				SessionID: sessionID,
				TurnID:    turnID,
				Events:    events,
				Final:     final,
			}, nil
		}
		if roundIndex == maxModelToolRounds {
			pause, err := k.appendToolLoopBudgetPause(sessionID, turnID, providerContext)
			if err != nil {
				return TurnResponse{}, err
			}
			events, err := k.TurnEvents(turnID)
			if err != nil {
				return TurnResponse{}, err
			}
			return TurnResponse{
				SessionID: sessionID,
				TurnID:    turnID,
				Events:    events,
				Pause:     &pause,
			}, nil
		}
		normalizedCalls, toolCallEventIDs, err := k.appendToolCallEvents(sessionID, turnID, modelResp.ToolCalls)
		if err != nil {
			if errors.Is(err, ErrModelToolCallRejected) {
				failure := TurnError{
					Code:    "tool_call_rejected",
					Message: err.Error(),
				}
				if appendErr := k.appendTurnFailure(sessionID, turnID, failure); appendErr != nil {
					return TurnResponse{}, appendErr
				}
			}
			return TurnResponse{}, err
		}
		preparedCalls, err := toolGateway.PrepareBatch(normalizedCalls)
		if err != nil {
			failure := TurnError{
				Code:    "tool_call_rejected",
				Message: err.Error(),
			}
			if appendErr := k.appendTurnFailure(sessionID, turnID, failure); appendErr != nil {
				return TurnResponse{}, appendErr
			}
			return TurnResponse{}, err
		}
		outcome, err := k.executeToolBatchesGuarded(runCtx, toolGateway, sessionID, turnID, preparedCalls, toolCallEventIDs, loopGuard)
		if err != nil {
			return TurnResponse{}, err
		}
		if outcome.Completed {
			return outcome.Response, nil
		}
	}
	return TurnResponse{}, errors.New("unreachable model tool loop state")
}

func (k *Kernel) submitNewTurn(req TurnRequest, sessionID string, turnID string, idempotencyKey string, ingressRisks []IngressRisk, now time.Time) ([]MemoryRecall, []ModelInputItem, error) {
	events, err := k.loadEvents()
	if err != nil {
		return nil, nil, err
	}
	recalledMemories, err := k.recallMemories(req.InputItems)
	if err != nil {
		return nil, nil, err
	}
	historyContext := sameSessionConversationHistoryContext(events, sessionID, "")
	skillIndex := k.skillCatalogProjection().Items
	modelInputs := modelInputItemsWithHistory(req.InputItems, recalledMemories, skillIndex, k.contextPolicy.SkillIndexChars, historyContext)
	submitted := StoredEvent{
		EventID:   newID("evt", now),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      "turn.submitted",
		CreatedAt: now,
		Data: EventData{
			IdempotencyKey:   idempotencyKey,
			InputItems:       req.InputItems,
			IngressRisks:     ingressRisks,
			ModelInputKinds:  modelInputKinds(modelInputs),
			ToolManifest:     k.toolGateway().ToolManifest(),
			SkillCatalog:     skillIndex,
			RuntimeContext:   k.contextRuntimeSnapshot(),
			RecalledMemories: recalledMemories,
		},
	}
	if err := k.appendEvent(submitted); err != nil {
		return nil, nil, err
	}
	return recalledMemories, modelInputs, nil
}

func (k *Kernel) completeProviderStep(ctx context.Context, sessionID string, turnID string, roundIndex int, providerContext ProviderContextProjection) (ModelResponse, error) {
	baseRequest := providerContext.ModelRequest()
	transientAttempt := 1
	visibleRepairCount := 0
	for {
		request := cloneModelRequest(baseRequest)
		if visibleRepairCount > 0 {
			request.InputItems = append(request.InputItems, ModelInputItem{
				Kind: ModelInputKindProviderRepairContext,
				Text: providerVisibleFinalRepairPrompt,
			})
		}
		modelResp, err := k.provider.Complete(ctx, request)
		if err == nil && modelResponseNeedsVisibleFinalRepair(modelResp) {
			err = newProviderVisibleFinalRequiredError()
		}
		if err == nil {
			return modelResp, nil
		}
		if isTurnContextInterrupted(ctx, err) {
			return ModelResponse{}, err
		}
		if errors.Is(err, ErrProviderVisibleFinalRequired) && visibleRepairCount < maxProviderVisibleFinalRepairs {
			visibleRepairCount++
			if appendErr := k.appendProviderAttempt(sessionID, turnID, "model.provider_repair", ProviderAttemptProjection{
				RoundIndex:  roundIndex,
				Attempt:     visibleRepairCount,
				MaxAttempts: maxProviderVisibleFinalRepairs,
				Status:      "repairing",
				ReasonCode:  "provider_visible_final_required",
				Message:     "provider returned no visible assistant content",
				RepairKind:  "visible_final",
			}); appendErr != nil {
				return ModelResponse{}, appendErr
			}
			continue
		}
		failure := providerFailureFromError(err)
		if failure.Retryable && transientAttempt < maxProviderTransientAttempts {
			if appendErr := k.appendProviderAttempt(sessionID, turnID, "model.provider_attempt", ProviderAttemptProjection{
				RoundIndex:  roundIndex,
				Attempt:     transientAttempt,
				MaxAttempts: maxProviderTransientAttempts,
				Status:      "retrying",
				ReasonCode:  failure.ReasonCode,
				Message:     failure.Message,
				Retryable:   true,
			}); appendErr != nil {
				return ModelResponse{}, appendErr
			}
			if delay := providerRetryDelay(err); delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ModelResponse{}, ctx.Err()
				case <-timer.C:
				}
			}
			transientAttempt++
			continue
		}
		failure.RoundIndex = roundIndex
		failure.Attempt = transientAttempt
		failure.MaxAttempts = maxProviderTransientAttempts
		if errors.Is(err, ErrProviderVisibleFinalRequired) {
			failure.Attempt = visibleRepairCount + 1
			failure.MaxAttempts = maxProviderVisibleFinalRepairs + 1
		}
		if appendErr := k.appendProviderAttempt(sessionID, turnID, "model.provider_attempt", failure); appendErr != nil {
			return ModelResponse{}, appendErr
		}
		return ModelResponse{}, err
	}
}

func cloneModelRequest(req ModelRequest) ModelRequest {
	return ModelRequest{
		SessionID:    req.SessionID,
		TurnID:       req.TurnID,
		InputItems:   cloneModelInputItems(req.InputItems),
		ToolManifest: cloneToolSpecs(req.ToolManifest),
		ToolRounds:   cloneModelToolRounds(req.ToolRounds),
	}
}

func (k *Kernel) contextRuntimeSnapshot() *ContextRuntimeSnapshot {
	policy := resolveToolPolicy(k.toolPolicy)
	return &ContextRuntimeSnapshot{
		Provider: safeProviderStatusForInspection(k.provider.Ready()),
		Permission: PermissionInspection{
			PermissionMode:  policy.PermissionMode,
			AuthorityPolicy: policy.AuthorityPolicy,
			SandboxProfile:  policy.SandboxProfile,
			ApprovalPolicy:  policy.ApprovalPolicy,
		},
	}
}

func (k *Kernel) Session(sessionID string) (SessionProjection, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return SessionProjection{}, errors.New("session id is required")
	}
	events, err := k.loadEvents()
	if err != nil {
		return SessionProjection{}, err
	}
	projection, err := projectSessionProjection(sessionID, events)
	if err != nil {
		return SessionProjection{}, err
	}
	return redactSessionProjection(projection), nil
}

var ErrSessionNotFound = errors.New("session not found")
var ErrTurnNotFound = errors.New("turn not found")
var ErrLedgerUnavailable = errors.New("ledger unavailable")

type replayedTurnFailure struct {
	failure TurnError
}

func (e replayedTurnFailure) Error() string {
	if e.failure.Message != "" {
		return e.failure.Message
	}
	if e.failure.Code != "" {
		return e.failure.Code
	}
	return "turn failed"
}

func (e replayedTurnFailure) Unwrap() error {
	switch e.failure.Code {
	case "provider_unavailable":
		return ErrProviderUnavailable
	case "tool_call_rejected":
		return ErrModelToolCallRejected
	case "turn_interrupted":
		return ErrTurnInterrupted
	default:
		return nil
	}
}

func (k *Kernel) turnByIdempotencyKey(sessionID string, key string) (TurnResponse, bool, error) {
	events, err := k.loadEvents()
	if err != nil {
		return TurnResponse{}, false, err
	}
	var turnID string
	var turnEvents []Event
	var final *FinalMessage
	var pause *TurnPauseProjection
	var failure *TurnError
	for _, event := range events {
		if event.SessionID != sessionID {
			continue
		}
		if event.Type == "turn.submitted" && event.Data.IdempotencyKey == key {
			if turnID != "" && turnID != event.TurnID {
				return TurnResponse{}, false, errors.New("competing turn idempotency evidence")
			}
			turnID = event.TurnID
		}
		if turnID == "" || event.TurnID != turnID {
			continue
		}
		turnEvents = append(turnEvents, toInspectionEvent(event))
		switch event.Type {
		case "model.final":
			if event.Data.Final != nil {
				copied := *event.Data.Final
				final = &copied
			}
		case "turn.paused":
			if event.Data.TurnPause != nil {
				copied := *event.Data.TurnPause
				pause = &copied
			}
		case "assistant.interrupted":
			failure = &TurnError{Code: "turn_interrupted", Message: "turn was interrupted"}
		case "turn.failed":
			if event.Data.TurnError != nil {
				copied := *event.Data.TurnError
				failure = &copied
			}
		}
	}
	if turnID == "" {
		return TurnResponse{}, false, nil
	}
	if final != nil {
		return TurnResponse{
			SessionID: sessionID,
			TurnID:    turnID,
			Events:    turnEvents,
			Final:     *final,
		}, true, nil
	}
	if pause != nil {
		return TurnResponse{
			SessionID: sessionID,
			TurnID:    turnID,
			Events:    turnEvents,
			Pause:     pause,
		}, true, nil
	}
	if failure != nil {
		return TurnResponse{
			SessionID: sessionID,
			TurnID:    turnID,
			Events:    turnEvents,
			Error:     failure,
		}, true, replayedTurnFailure{failure: *failure}
	}
	return TurnResponse{}, true, errors.New("turn idempotency key is already running")
}

func (k *Kernel) TurnEvents(turnID string) ([]Event, error) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return nil, errors.New("turn id is required")
	}
	events, err := k.loadEvents()
	if err != nil {
		return nil, err
	}
	items := []Event{}
	for _, event := range events {
		if event.TurnID == turnID {
			items = append(items, toInspectionEvent(event))
		}
	}
	if len(items) == 0 {
		return nil, ErrTurnNotFound
	}
	return items, nil
}

func (k *Kernel) appendEvent(event StoredEvent) error {
	if err := k.ensureLedgerReady(); err != nil {
		return err
	}
	if err := k.ledger.Append(event); err != nil {
		return wrapLedgerUnavailable(err)
	}
	return nil
}

func (k *Kernel) appendTurnFailure(sessionID string, turnID string, failure TurnError) error {
	failedAt := k.clock()
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", failedAt),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      "turn.failed",
		CreatedAt: failedAt,
		Data: EventData{
			TurnError: &failure,
		},
	})
}

func (k *Kernel) appendToolLoopBudgetPause(sessionID string, turnID string, providerContext ProviderContextProjection) (TurnPauseProjection, error) {
	pausedAt := k.clock()
	completedRounds, _, _ := modelToolRoundCounts(providerContext.ToolRounds)
	pause := TurnPauseProjection{
		SessionID:           strings.TrimSpace(sessionID),
		TurnID:              strings.TrimSpace(turnID),
		Status:              "paused",
		Reason:              "tool_loop_round_budget_exhausted",
		RoundBudget:         maxModelToolRounds,
		CompletedToolRounds: completedRounds,
		PausedAt:            pausedAt,
	}
	err := k.appendEvent(StoredEvent{
		EventID:   newID("evt", pausedAt),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      "turn.paused",
		CreatedAt: pausedAt,
		Data: EventData{
			TurnPause: &pause,
		},
	})
	return pause, err
}

func (k *Kernel) appendProviderAttempt(sessionID string, turnID string, eventType string, attempt ProviderAttemptProjection) error {
	now := k.clock()
	if strings.TrimSpace(eventType) == "" {
		eventType = "model.provider_attempt"
	}
	attempt.Message = redactEvidenceText(attempt.Message)
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      eventType,
		CreatedAt: now,
		Data: EventData{
			ProviderAttempt: &attempt,
		},
	})
}

func (k *Kernel) appendToolCallEvents(sessionID string, turnID string, calls []ModelToolCall) ([]ModelToolCall, map[string]string, error) {
	if err := validateProviderToolCallBatch(calls); err != nil {
		return nil, nil, err
	}
	normalized := make([]ModelToolCall, 0, len(calls))
	eventIDs := make(map[string]string, len(calls))
	for _, call := range calls {
		createdAt := k.clock()
		eventID := newID("evt", createdAt)
		providerCallID := providerToolCallID(call)
		normalizedCall := ModelToolCall{
			ToolCallID:      providerCallID,
			ToolCallEventID: eventID,
			Name:            call.Name,
			Arguments:       append(json.RawMessage(nil), call.Arguments...),
		}
		if err := k.appendEvent(StoredEvent{
			EventID:   eventID,
			SessionID: sessionID,
			TurnID:    turnID,
			Type:      "tool.call",
			CreatedAt: createdAt,
			Data: EventData{
				ToolCall: &ToolCallProjection{
					ToolCallEventID:    eventID,
					ProviderToolCallID: providerCallID,
					Tool:               strings.TrimSpace(call.Name),
					Arguments:          string(call.Arguments),
				},
			},
		}); err != nil {
			return nil, nil, err
		}
		normalized = append(normalized, normalizedCall)
		eventIDs[eventID] = eventID
	}
	return normalized, eventIDs, nil
}

func (k *Kernel) appendToolResultEvent(sessionID string, turnID string, result ModelToolResult, forEventID string) error {
	createdAt := k.clock()
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", createdAt),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      "tool.result",
		CreatedAt: createdAt,
		Data: EventData{
			ToolResult: &ToolResultProjection{
				ToolCallEventID:    strings.TrimSpace(result.ToolCallEventID),
				ProviderToolCallID: strings.TrimSpace(result.ToolCallID),
				Tool:               strings.TrimSpace(result.Name),
				ForEventID:         strings.TrimSpace(forEventID),
				Status:             toolResultStatus(result.Content),
				Content:            result.Content,
			},
		},
	})
}

func validateProviderToolCallBatch(calls []ModelToolCall) error {
	seen := map[string]bool{}
	for _, call := range calls {
		if strings.TrimSpace(call.ToolCallEventID) != "" {
			return fmt.Errorf("%w: provider supplied kernel-owned tool_call_event_id", ErrModelToolCallRejected)
		}
		id := providerToolCallID(call)
		if id == "" {
			continue
		}
		if seen[id] {
			return fmt.Errorf("%w: duplicate provider tool_call_id", ErrModelToolCallRejected)
		}
		seen[id] = true
	}
	return nil
}

func providerToolCallID(call ModelToolCall) string {
	return strings.TrimSpace(call.ToolCallID)
}

func toolResultStatus(content string) string {
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err == nil && strings.TrimSpace(payload.Status) != "" {
		return strings.TrimSpace(payload.Status)
	}
	return "tool_result"
}

func (k *Kernel) ProviderContextProjection(turnID string) (ProviderContextProjection, error) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return ProviderContextProjection{}, errors.New("turn id is required")
	}
	events, err := k.loadEvents()
	if err != nil {
		return ProviderContextProjection{}, err
	}
	projection, ok := providerContextProjectionFromStoredEvents(events, turnID, k.contextPolicy)
	if !ok {
		return ProviderContextProjection{}, ErrTurnNotFound
	}
	return projection, nil
}

func (k *Kernel) appendModelContextAccounting(sessionID string, turnID string, roundIndex int, providerContext ProviderContextProjection, response ModelResponse) error {
	if response.Usage == nil {
		return nil
	}
	now := k.clock()
	accounting := ModelContextAccountingProjection{
		RoundIndex:             roundIndex,
		Model:                  strings.TrimSpace(response.Model),
		ModelInputKinds:        modelInputKinds(providerContext.InputItems),
		HistoryTurnIDs:         append([]string(nil), providerContext.HistoryTurnIDs...),
		CompactedThroughTurnID: providerContext.CompactedThroughTurnID,
		Usage:                  cloneTokenUsage(response.Usage),
	}
	accounting.ToolRoundCount, accounting.ToolCallCount, accounting.ToolResultCount = modelToolRoundCounts(providerContext.ToolRounds)
	if response.Usage.CacheMissTokens > 0 {
		accounting.ProcessedInputTokens = response.Usage.CacheMissTokens
		accounting.ProcessedInputTokenSource = "prompt_cache_miss_tokens"
	}
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: strings.TrimSpace(sessionID),
		TurnID:    strings.TrimSpace(turnID),
		Type:      "model.context.accounted",
		CreatedAt: now,
		Data: EventData{
			ModelContextAccounting: &accounting,
		},
	})
}

func modelToolRoundCounts(rounds []ModelToolRound) (int, int, int) {
	roundCount := 0
	callCount := 0
	resultCount := 0
	for _, round := range rounds {
		if len(round.Calls) == 0 && len(round.Results) == 0 {
			continue
		}
		roundCount++
		callCount += len(round.Calls)
		resultCount += len(round.Results)
	}
	return roundCount, callCount, resultCount
}

func cloneTokenUsage(usage *TokenUsage) *TokenUsage {
	if usage == nil {
		return nil
	}
	cloned := *usage
	return &cloned
}

func providerContextProjectionFromStoredEvents(events []StoredEvent, turnID string, policy ContextPolicy) (ProviderContextProjection, bool) {
	projection := ProviderContextProjection{TurnID: turnID}
	found := false
	var submitted EventData
	for _, event := range events {
		if event.TurnID != turnID || event.Type != "turn.submitted" {
			continue
		}
		found = true
		projection.SessionID = event.SessionID
		projection.ToolManifest = cloneToolSpecs(event.Data.ToolManifest)
		submitted = event.Data
		break
	}
	if !found {
		return ProviderContextProjection{}, false
	}
	history := sameSessionConversationHistoryProjection(events, projection.SessionID, turnID)
	observations := pendingKernelObservations(events, projection.SessionID)
	projection.InputItems = modelInputItemsFromSubmittedEvent(submitted, history.Text, policy.SkillIndexChars, kernelObservationContext(observations))
	projection.KernelObservationEventIDs = kernelObservationEventIDs(observations)
	projection.ToolRounds = modelToolRoundsFromStoredEvents(events, turnID)
	projection.HistoryTurnIDs = history.TurnIDs()
	projection.CompactedThroughTurnID = history.CompactedThroughTurnID
	return projection, true
}

func modelInputItemsFromSubmittedEvent(data EventData, historyContext string, skillIndexBudget int, observationContext string) []ModelInputItem {
	items := []ModelInputItem{}
	if strings.TrimSpace(historyContext) != "" {
		items = append(items, ModelInputItem{Kind: ModelInputKindConversationHistoryContext, Text: historyContext})
	}
	if context := skillIndexContext(data.SkillCatalog, skillIndexBudget); context != "" {
		items = append(items, ModelInputItem{Kind: ModelInputKindSkillIndexContext, Text: context})
	}
	if context := approvedMemoryContext(data.RecalledMemories); context != "" {
		items = append(items, ModelInputItem{Kind: ModelInputKindApprovedMemoryContext, Text: context})
	}
	if context := strings.TrimSpace(observationContext); context != "" {
		items = append(items, ModelInputItem{Kind: ModelInputKindKernelObservationContext, Text: context})
	}
	for _, item := range data.InputItems {
		if item.Type == "text" && item.Text != "" {
			items = append(items, ModelInputItem{Kind: ModelInputKindUserText, Text: item.Text})
		}
	}
	return items
}

func sameSessionConversationHistoryContext(events []StoredEvent, sessionID string, beforeTurnID string) string {
	return sameSessionConversationHistoryProjection(events, sessionID, beforeTurnID).Text
}

type sameSessionHistoryProjection struct {
	Text                   string
	CompactedThroughTurnID string
	Turns                  []conversationHistoryTurn
}

func (p sameSessionHistoryProjection) TurnIDs() []string {
	ids := make([]string, 0, len(p.Turns))
	for _, turn := range p.Turns {
		if strings.TrimSpace(turn.TurnID) != "" {
			ids = append(ids, turn.TurnID)
		}
	}
	return ids
}

func sameSessionConversationHistoryProjection(events []StoredEvent, sessionID string, beforeTurnID string) sameSessionHistoryProjection {
	compaction := latestSessionContextCompaction(events, sessionID, beforeTurnID)
	turns := sameSessionCompletedConversationTurns(events, sessionID, beforeTurnID)
	turns = turnsAfterCompactedTurn(turns, compaction.CompactedThroughTurnID)
	return sameSessionHistoryProjection{
		Text:                   conversationHistoryContextWithSummary(compaction.Summary, turns),
		CompactedThroughTurnID: compaction.CompactedThroughTurnID,
		Turns:                  turns,
	}
}

func sameSessionCompletedConversationTurns(events []StoredEvent, sessionID string, beforeTurnID string) []conversationHistoryTurn {
	sessionID = strings.TrimSpace(sessionID)
	beforeTurnID = strings.TrimSpace(beforeTurnID)
	if sessionID == "" {
		return nil
	}
	submittedInputs := map[string][]InputItem{}
	toolCallsByTurn := map[string][]ToolCallProjection{}
	toolResultsByTurn := map[string][]ToolResultProjection{}
	var turns []conversationHistoryTurn
	for _, event := range events {
		if event.SessionID != sessionID {
			continue
		}
		if beforeTurnID != "" && event.TurnID == beforeTurnID && event.Type == "turn.submitted" {
			break
		}
		switch event.Type {
		case "turn.submitted":
			submittedInputs[event.TurnID] = cloneInputItems(event.Data.InputItems)
		case "tool.call":
			if event.Data.ToolCall != nil {
				toolCallsByTurn[event.TurnID] = append(toolCallsByTurn[event.TurnID], *event.Data.ToolCall)
			}
		case "tool.result":
			if event.Data.ToolResult != nil {
				toolResultsByTurn[event.TurnID] = append(toolResultsByTurn[event.TurnID], *event.Data.ToolResult)
			}
		case "model.final":
			if event.Data.Final == nil {
				continue
			}
			userText := inputText(submittedInputs[event.TurnID])
			assistantText := strings.TrimSpace(event.Data.Final.Text)
			toolExchanges := pairedConversationToolExchanges(toolCallsByTurn[event.TurnID], toolResultsByTurn[event.TurnID])
			if strings.TrimSpace(userText) != "" || len(toolExchanges) > 0 || assistantText != "" {
				turns = append(turns, conversationHistoryTurn{
					TurnID:        event.TurnID,
					UserText:      userText,
					ToolExchanges: toolExchanges,
					AssistantText: assistantText,
				})
			}
			delete(submittedInputs, event.TurnID)
			delete(toolCallsByTurn, event.TurnID)
			delete(toolResultsByTurn, event.TurnID)
		case "turn.paused":
			if event.Data.TurnPause == nil {
				continue
			}
			userText := inputText(submittedInputs[event.TurnID])
			toolExchanges := pairedConversationToolExchanges(toolCallsByTurn[event.TurnID], toolResultsByTurn[event.TurnID])
			assistantText := toolLoopPausedHistoryText(*event.Data.TurnPause)
			if strings.TrimSpace(userText) != "" || len(toolExchanges) > 0 {
				turns = append(turns, conversationHistoryTurn{
					TurnID:        event.TurnID,
					UserText:      userText,
					ToolExchanges: toolExchanges,
					AssistantText: assistantText,
				})
			}
			delete(submittedInputs, event.TurnID)
			delete(toolCallsByTurn, event.TurnID)
			delete(toolResultsByTurn, event.TurnID)
		case "turn.failed":
			delete(submittedInputs, event.TurnID)
			delete(toolCallsByTurn, event.TurnID)
			delete(toolResultsByTurn, event.TurnID)
		}
	}
	return turns
}

func toolLoopPausedHistoryText(pause TurnPauseProjection) string {
	reason := strings.TrimSpace(pause.Reason)
	if reason == "" {
		reason = "tool_loop_round_budget_exhausted"
	}
	return fmt.Sprintf("tool loop paused: %s after %d committed tool rounds; continue from the committed tool results", reason, pause.CompletedToolRounds)
}

func pairedConversationToolExchanges(calls []ToolCallProjection, results []ToolResultProjection) []conversationToolExchange {
	if len(calls) == 0 || len(results) == 0 {
		return nil
	}
	resultByEventID := make(map[string]ToolResultProjection, len(results))
	for _, result := range results {
		resultByEventID[strings.TrimSpace(result.ForEventID)] = result
	}
	exchanges := make([]conversationToolExchange, 0, len(calls))
	for _, call := range calls {
		result, ok := resultByEventID[strings.TrimSpace(call.ToolCallEventID)]
		if !ok {
			continue
		}
		exchanges = append(exchanges, conversationToolExchange{
			Tool:          strings.TrimSpace(call.Tool),
			Arguments:     strings.TrimSpace(call.Arguments),
			ResultStatus:  strings.TrimSpace(result.Status),
			ResultContent: strings.TrimSpace(result.Content),
		})
	}
	if len(exchanges) == 0 {
		return nil
	}
	return exchanges
}

func modelToolRoundsFromStoredEvents(events []StoredEvent, turnID string) []ModelToolRound {
	rounds := []ModelToolRound{}
	current := ModelToolRound{}
	for _, event := range events {
		if event.TurnID != turnID {
			continue
		}
		switch event.Type {
		case "tool.call":
			if event.Data.ToolCall == nil {
				continue
			}
			current.Calls = append(current.Calls, ModelToolCall{
				ToolCallID: event.Data.ToolCall.ProviderToolCallID,
				Name:       event.Data.ToolCall.Tool,
				Arguments:  json.RawMessage(event.Data.ToolCall.Arguments),
			})
		case "tool.result":
			if event.Data.ToolResult == nil {
				continue
			}
			current.Results = append(current.Results, ModelToolResult{
				ToolCallID: event.Data.ToolResult.ProviderToolCallID,
				Name:       event.Data.ToolResult.Tool,
				Content:    event.Data.ToolResult.Content,
			})
			if len(current.Calls) > 0 && len(current.Results) == len(current.Calls) {
				rounds = append(rounds, current)
				current = ModelToolRound{}
			}
		}
	}
	return rounds
}

func (k *Kernel) loadEvents() ([]StoredEvent, error) {
	events, err := k.ledger.Load()
	if err != nil {
		return nil, wrapLedgerUnavailable(err)
	}
	return events, nil
}

func (k *Kernel) ensureLedgerReady() error {
	check := k.ledger.Ready()
	if check.Status == "ok" {
		return nil
	}
	switch check.Reason {
	case "ledger_corrupt":
		return wrapLedgerUnavailable(ErrLedgerCorrupt)
	case "ledger_unreadable":
		return wrapLedgerUnavailable(ErrLedgerUnreadable)
	default:
		return wrapLedgerUnavailable(ErrLedgerUnwritable)
	}
}

func wrapLedgerUnavailable(err error) error {
	if errors.Is(err, ErrLedgerUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrLedgerUnavailable, err)
}

func ledgerErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrLedgerCorrupt):
		return "ledger_corrupt"
	case errors.Is(err, ErrLedgerUnreadable):
		return "ledger_unreadable"
	case errors.Is(err, ErrLedgerUnwritable):
		return "ledger_unwritable"
	default:
		return "ledger_unavailable"
	}
}

func validateTurnRequest(req TurnRequest) error {
	if strings.TrimSpace(req.SessionID) != "" {
		if err := validateKernelControlToken("session_id", req.SessionID); err != nil {
			return err
		}
	}
	if err := validateIdempotencyKey(req.IdempotencyKey); err != nil {
		return err
	}
	if strings.TrimSpace(req.IdempotencyKey) != "" {
		if strings.TrimSpace(req.SessionID) == "" {
			return errors.New("session_id is required when idempotency_key is set")
		}
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

func turnFailureFromProviderError(err error) TurnError {
	failure := providerFailureFromError(err)
	code := failure.ReasonCode
	if code == "" {
		code = "provider_error"
	}
	message := failure.Message
	if message == "" {
		message = redactEvidenceText(err.Error())
	}
	return TurnError{
		Code:    code,
		Message: message,
	}
}

func providerCompleteError(err error) error {
	message := redactEvidenceText(err.Error())
	if errors.Is(err, ErrProviderUnavailable) {
		return fmt.Errorf("provider complete: %w: %s", ErrProviderUnavailable, message)
	}
	return fmt.Errorf("provider complete: %s", message)
}

func toEvent(event StoredEvent) Event {
	return Event{
		EventID:     event.EventID,
		SessionID:   event.SessionID,
		TurnID:      event.TurnID,
		OperationID: event.OperationID,
		WorkID:      event.WorkID,
		CandidateID: event.CandidateID,
		Type:        event.Type,
		CreatedAt:   event.CreatedAt,
		Data:        event.Data,
	}
}
