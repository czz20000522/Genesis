package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"genesis/internal/kernel/modelgateway"
	"genesis/internal/kernel/resource"
)

type Kernel struct {
	ledger                Ledger
	provider              Provider
	jobExecutor           ManagedJobExecutor
	runtimeToken          string
	toolPolicy            ToolPolicy
	contextPolicy         ContextPolicy
	budgetPolicy          BudgetPolicy
	shellTimeoutPolicy    ShellTimeoutPolicy
	toolRegistry          *ToolRegistry
	resourceRegistry      *resource.Registry
	materialStorePath     string
	skillCatalog          []SkillDescriptor
	skillRoots            []SkillCatalogRootProjection
	skillExclusions       []SkillCatalogExclusionProjection
	capabilityDescriptors []CapabilityDescriptor
	clock                 func() time.Time
	turnMu                sync.Mutex
	activeTurnMu          sync.Mutex
	activeTurns           map[string]*activeTurn
	operationMu           sync.Mutex
	jobMu                 sync.Mutex
	approvalMu            sync.Mutex
	memoryReviewMu        sync.Mutex
	workMu                sync.Mutex
}

type activeTurn struct {
	sessionID string
	turnID    string
	kind      string
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
	shellTimeoutPolicy := normalizedShellTimeoutPolicy(config.ShellTimeoutPolicy)
	toolRegistry, err := defaultToolRegistry(shellTimeoutPolicy)
	if err != nil {
		return nil, err
	}
	resourceRegistry, err := resource.NewRegistryWithSourceSnapshotPolicy(config.Resources, config.SourceSnapshotPolicy)
	if err != nil {
		return nil, err
	}
	materialStorePath := strings.TrimSpace(config.MaterialStorePath)
	if materialStorePath == "" {
		materialStorePath = filepath.Join(filepath.Dir(config.LedgerPath), "material-store")
	}
	skillCatalog := loadSkillCatalogWithDiagnostics(config.SkillRoots)
	capabilityDescriptors, err := normalizeCapabilityDescriptors(config.CapabilityDescriptors)
	if err != nil {
		return nil, err
	}
	k := &Kernel{
		ledger:                NewSQLiteLedger(config.LedgerPath),
		provider:              provider,
		jobExecutor:           jobExecutor,
		runtimeToken:          strings.TrimSpace(config.RuntimeToken),
		toolPolicy:            normalizedToolPolicy(config.ToolPolicy),
		contextPolicy:         normalizedContextPolicy(config.ContextPolicy),
		budgetPolicy:          normalizedBudgetPolicy(config.BudgetPolicy),
		shellTimeoutPolicy:    shellTimeoutPolicy,
		toolRegistry:          toolRegistry,
		resourceRegistry:      resourceRegistry,
		materialStorePath:     materialStorePath,
		skillCatalog:          skillCatalog.Items,
		skillRoots:            skillCatalog.Roots,
		skillExclusions:       skillCatalog.Exclusions,
		capabilityDescriptors: capabilityDescriptors,
		clock:                 clock,
		activeTurns:           map[string]*activeTurn{},
	}
	_ = k.recoverLostLocalManagedJobs()
	return k, nil
}

func (k *Kernel) Close() {
	if closer, ok := k.jobExecutor.(interface{ Close() }); ok {
		closer.Close()
	}
	if closer, ok := k.ledger.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func (k *Kernel) Ready() ReadyResponse {
	providerStatus := k.provider.Ready()
	runtimeAuth := ReadyCheck{Readiness: ReadinessReady}
	if k.runtimeToken == "" {
		runtimeAuth = ReadyCheck{Readiness: ReadinessNotReady, ReadinessReason: "runtime_token_missing"}
	}
	ledgerStatus := k.ledger.Ready()
	readiness := ReadinessReady
	readinessReason := ""
	if providerStatus.Readiness != ReadinessReady {
		readiness = ReadinessNotReady
		readinessReason = "provider_not_ready"
	}
	if readiness == ReadinessReady && runtimeAuth.Readiness != ReadinessReady {
		readiness = ReadinessNotReady
		readinessReason = firstNonEmpty(runtimeAuth.ReadinessReason, "runtime_auth_unavailable")
	}
	if readiness == ReadinessReady && ledgerStatus.Readiness != ReadinessReady {
		readiness = ReadinessNotReady
		readinessReason = firstNonEmpty(ledgerStatus.ReadinessReason, "ledger_unavailable")
	}
	return ReadyResponse{
		Readiness:       readiness,
		ReadinessReason: readinessReason,
		Provider:        safeProviderStatusForInspection(providerStatus),
		RuntimeAuth:     runtimeAuth,
		Ledger:          ledgerStatus,
	}
}

func (k *Kernel) Capabilities() CapabilitiesResponse {
	ready := k.Ready()
	return CapabilitiesResponse{
		Readiness:                 ready.Readiness,
		ReadinessReason:           ready.ReadinessReason,
		Provider:                  safeProviderStatusForInspection(ready.Provider),
		RuntimeAuth:               ready.RuntimeAuth,
		Ledger:                    ready.Ledger,
		BudgetLease:               k.budgetLeaseProjection(),
		ShellTimeoutPolicy:        k.shellTimeoutPolicy,
		SourceSnapshotPersistence: k.sourceSnapshotPersistence(),
		Limits:                    k.runtimeLimitProjections(),
		Tools:                     k.toolCapabilityProjections(),
		SkillCatalog:              k.skillCatalogProjection(),
	}
}

func (k *Kernel) sourceSnapshotPersistence() ReadyCheck {
	return ReadyCheck{Readiness: ReadinessNotReady, ReadinessReason: "process_lifetime_only"}
}

func (k *Kernel) SubmitTurn(ctx context.Context, req TurnRequest) (TurnResponse, error) {
	return k.submitTurn(ctx, req, nil)
}

func (k *Kernel) SubmitTurnStream(ctx context.Context, req TurnRequest, emit func(TurnStreamEvent) error) (TurnResponse, error) {
	if emit == nil {
		return k.SubmitTurn(ctx, req)
	}
	return k.submitTurn(ctx, req, emit)
}

func (k *Kernel) submitTurn(ctx context.Context, req TurnRequest, emit func(TurnStreamEvent) error) (TurnResponse, error) {
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
	var runCtx context.Context
	var finishActiveTurn func()
	if idempotencyKey != "" {
		var existing TurnResponse
		var ok bool
		k.turnMu.Lock()
		existing, ok, err = k.turnByIdempotencyKey(sessionID, idempotencyKey)
		if err == nil && !ok {
			turnID = newID("turn", now)
			var admitted bool
			runCtx, finishActiveTurn, admitted = k.tryBeginActiveTurn(ctx, sessionID, turnID)
			if !admitted {
				err = ErrSessionActive
			} else {
				_, err = k.submitNewTurn(req, sessionID, turnID, idempotencyKey, ingressRisks, now)
				if err != nil {
					finishActiveTurn()
				}
			}
		}
		k.turnMu.Unlock()
		if err != nil || ok {
			return existing, err
		}
	} else {
		turnID = newID("turn", now)
		var admitted bool
		runCtx, finishActiveTurn, admitted = k.tryBeginActiveTurn(ctx, sessionID, turnID)
		if !admitted {
			return TurnResponse{}, ErrSessionActive
		}
		_, err = k.submitNewTurn(req, sessionID, turnID, "", ingressRisks, now)
		if err != nil {
			finishActiveTurn()
			return TurnResponse{}, err
		}
	}
	defer finishActiveTurn()

	toolGateway := k.toolGateway()
	loopGuard := newToolLoopGuard()
	budgetLease := k.newTurnBudgetLease()
	for roundIndex := 0; ; roundIndex++ {
		providerContext, err := k.ProviderContextProjection(turnID)
		if err != nil {
			return TurnResponse{}, err
		}
		modelResp, err := k.completeProviderStep(runCtx, sessionID, turnID, roundIndex, providerContext, emit)
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
		if !budgetLease.allowModelToolRound(roundIndex) {
			pause, err := k.appendToolLoopBudgetPause(sessionID, turnID, providerContext, budgetLease)
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

func (k *Kernel) submitNewTurn(req TurnRequest, sessionID string, turnID string, idempotencyKey string, ingressRisks []IngressRisk, now time.Time) ([]ModelInputItem, error) {
	events, err := k.loadEvents()
	if err != nil {
		return nil, err
	}
	historyContext := sameSessionConversationHistoryContext(events, sessionID, "")
	skillIndex := k.skillCatalogProjection().Items
	sourceSnapshots := k.resourceRegistry.ListSourceSnapshotDescriptors(sessionID)
	hydratedContexts := pendingContextHydrationsForNewTurn(events, sessionID, turnID)
	providerHydratedContexts := k.providerHydratedContextFragments(hydratedContexts)
	modelInputs := modelInputItemsWithHistoryAndHydration(req.InputItems, skillIndex, providerHydratedContexts, sourceSnapshots, k.contextPolicy.SkillIndexChars, historyContext, "")
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
			SourceSnapshots:  sourceSnapshots,
			RuntimeContext:   k.contextRuntimeSnapshot(),
			HydratedContexts: hydratedContexts,
		},
	}
	if err := k.appendEvent(submitted); err != nil {
		return nil, err
	}
	return modelInputs, nil
}

func (k *Kernel) completeProviderStep(ctx context.Context, sessionID string, turnID string, roundIndex int, providerContext ProviderContextProjection, emit func(TurnStreamEvent) error) (ModelResponse, error) {
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
		modelResp, streamedDelta, err := k.completeModel(ctx, request, emit)
		if err == nil && modelResponseNeedsVisibleFinalRepair(modelResp) {
			err = newProviderVisibleFinalRequiredError()
		}
		k.captureSessionDebugProviderStep(sessionID, turnID, roundIndex, transientAttempt+visibleRepairCount, request, modelResp, err, providerContext.KernelObservationEventIDs)
		if err == nil {
			return modelResp, nil
		}
		if isTurnContextInterrupted(ctx, err) {
			return ModelResponse{}, err
		}
		if streamedDelta {
			failure := providerFailureFromError(err)
			failure.RoundIndex = roundIndex
			failure.Attempt = transientAttempt
			failure.MaxAttempts = maxProviderTransientAttempts
			if appendErr := k.appendProviderAttempt(sessionID, turnID, "model.provider_attempt", failure); appendErr != nil {
				return ModelResponse{}, appendErr
			}
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

func (k *Kernel) completeModel(ctx context.Context, request ModelRequest, emit func(TurnStreamEvent) error) (ModelResponse, bool, error) {
	streamer, ok := k.provider.(StreamingProvider)
	if emit == nil || !ok {
		resp, err := k.provider.Complete(ctx, request)
		return resp, false, err
	}
	streamedDelta := false
	resp, err := streamer.StreamComplete(ctx, request, func(delta ModelStreamDelta) error {
		if strings.TrimSpace(delta.Text) == "" {
			return nil
		}
		streamedDelta = true
		return emit(TurnStreamEvent{Type: "assistant_delta", Delta: delta.Text})
	})
	return resp, streamedDelta, err
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
		Provider:           safeProviderStatusForInspection(k.provider.Ready()),
		BudgetLease:        k.budgetLeaseProjection(),
		ShellTimeoutPolicy: k.shellTimeoutPolicy,
		Limits:             k.runtimeLimitProjections(),
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
	return localSessionProjection(projection), nil
}

func (k *Kernel) ListSessions() (SessionListResponse, error) {
	if index, ok := k.ledger.(interface {
		ListSessions() ([]SessionListItem, error)
	}); ok {
		items, err := index.ListSessions()
		if err != nil {
			return SessionListResponse{}, wrapLedgerUnavailable(err)
		}
		return SessionListResponse{Items: items}, nil
	}
	events, err := k.loadEvents()
	if err != nil {
		return SessionListResponse{}, err
	}
	updatedAtBySession := map[string]time.Time{}
	for _, event := range events {
		sessionID := strings.TrimSpace(event.SessionID)
		if sessionID == "" {
			continue
		}
		if current, ok := updatedAtBySession[sessionID]; !ok || event.CreatedAt.After(current) {
			updatedAtBySession[sessionID] = event.CreatedAt
		}
	}
	items := make([]SessionListItem, 0, len(updatedAtBySession))
	for sessionID, updatedAt := range updatedAtBySession {
		items = append(items, SessionListItem{
			SessionID: sessionID,
			UpdatedAt: updatedAt,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].SessionID < items[j].SessionID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return SessionListResponse{Items: items}, nil
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

func (k *Kernel) appendToolLoopBudgetPause(sessionID string, turnID string, providerContext ProviderContextProjection, lease budgetLease) (TurnPauseProjection, error) {
	pausedAt := k.clock()
	completedRounds, _, _ := modelToolRoundCounts(providerContext.ToolRounds)
	pause := TurnPauseProjection{
		SessionID:           strings.TrimSpace(sessionID),
		TurnID:              strings.TrimSpace(turnID),
		Phase:               RuntimePhaseWaiting,
		WaitReason:          WaitReasonBudgetPause,
		Reason:              "tool_loop_round_budget_exhausted",
		RoundBudget:         lease.modelToolRoundBudget,
		BudgetLease:         lease.projection(),
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
	projection, ok := k.providerContextProjectionFromStoredEvents(events, turnID, k.contextPolicy)
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
	return modelgateway.CloneTokenUsage(usage)
}

func (k *Kernel) providerContextProjectionFromStoredEvents(events []StoredEvent, turnID string, policy ContextPolicy) (ProviderContextProjection, bool) {
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
	projection.InputItems = k.modelInputItemsFromSubmittedEvent(submitted, history.Text, policy.SkillIndexChars, kernelObservationContext(observations))
	projection.KernelObservationEventIDs = kernelObservationEventIDs(observations)
	projection.ToolRounds = modelToolRoundsFromStoredEvents(events, turnID)
	projection.HistoryTurnIDs = history.TurnIDs()
	projection.CompactedThroughTurnID = history.CompactedThroughTurnID
	return projection, true
}

func (k *Kernel) modelInputItemsFromSubmittedEvent(data EventData, historyContext string, skillIndexBudget int, observationContext string) []ModelInputItem {
	return modelInputItemsWithHistoryAndHydration(data.InputItems, data.SkillCatalog, k.providerHydratedContextFragments(data.HydratedContexts), data.SourceSnapshots, skillIndexBudget, historyContext, observationContext)
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
			userText := inputItemsText(submittedInputs[event.TurnID])
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
			userText := inputItemsText(submittedInputs[event.TurnID])
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
	if check.Readiness == ReadinessReady {
		return nil
	}
	switch check.ReadinessReason {
	case "ledger_corrupt":
		return wrapLedgerUnavailable(ErrLedgerCorrupt)
	case "ledger_unreadable":
		return wrapLedgerUnavailable(ErrLedgerUnreadable)
	case "ledger_locked":
		return wrapLedgerUnavailable(ErrLedgerLocked)
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
	case errors.Is(err, ErrLedgerLocked):
		return "ledger_locked"
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
		message = externalBoundaryDiagnosticText(err.Error())
	}
	return TurnError{
		Code:    code,
		Message: message,
	}
}

func providerCompleteError(err error) error {
	message := externalBoundaryDiagnosticText(err.Error())
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
