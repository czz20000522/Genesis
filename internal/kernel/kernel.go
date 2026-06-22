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
	ledger          Ledger
	provider        Provider
	runtimeToken    string
	toolPolicy      ToolPolicy
	toolRegistry    *ToolRegistry
	skillCatalog    []SkillDescriptor
	skillExclusions []SkillCatalogExclusionProjection
	clock           func() time.Time
	turnMu          sync.Mutex
	operationMu     sync.Mutex
	memoryReviewMu  sync.Mutex
	workMu          sync.Mutex
}

func New(config Config) (*Kernel, error) {
	if strings.TrimSpace(config.LedgerPath) == "" {
		return nil, errors.New("ledger path is required")
	}
	provider := config.Provider
	if provider == nil {
		provider = FakeProvider{}
	}
	clock := config.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	toolRegistry, err := defaultToolRegistry()
	if err != nil {
		return nil, err
	}
	skillCatalog := loadSkillCatalogWithDiagnostics(config.SkillRoots)
	return &Kernel{
		ledger:          NewJSONLLedger(config.LedgerPath),
		provider:        provider,
		runtimeToken:    strings.TrimSpace(config.RuntimeToken),
		toolPolicy:      normalizedToolPolicy(config.ToolPolicy),
		toolRegistry:    toolRegistry,
		skillCatalog:    skillCatalog.Items,
		skillExclusions: skillCatalog.Exclusions,
		clock:           clock,
	}, nil
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
		Provider:    providerStatus,
		RuntimeAuth: runtimeAuth,
		Ledger:      ledgerStatus,
		LedgerPath:  k.ledger.Path(),
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
	var inputItems []ModelInputItem
	if idempotencyKey != "" {
		var existing TurnResponse
		var ok bool
		k.turnMu.Lock()
		existing, ok, err = k.turnByIdempotencyKey(sessionID, idempotencyKey)
		if err == nil && !ok {
			turnID = newID("turn", now)
			_, inputItems, err = k.submitNewTurn(req, sessionID, turnID, idempotencyKey, ingressRisks, now)
		}
		k.turnMu.Unlock()
		if err != nil || ok {
			return existing, err
		}
	} else {
		turnID = newID("turn", now)
		_, inputItems, err = k.submitNewTurn(req, sessionID, turnID, "", ingressRisks, now)
		if err != nil {
			return TurnResponse{}, err
		}
	}

	toolRounds := []ModelToolRound{}
	toolGateway := k.toolGateway()
	for roundIndex := 0; roundIndex <= maxModelToolRounds; roundIndex++ {
		modelResp, err := k.provider.Complete(ctx, ModelRequest{
			SessionID:    sessionID,
			TurnID:       turnID,
			InputItems:   inputItems,
			ToolManifest: toolGateway.ToolManifest(),
			ToolRounds:   toolRounds,
		})
		if err != nil {
			failure := turnFailureFromProviderError(err)
			if appendErr := k.appendTurnFailure(sessionID, turnID, failure); appendErr != nil {
				return TurnResponse{}, appendErr
			}
			return TurnResponse{}, providerCompleteError(err)
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
			failure := TurnError{
				Code:    "tool_loop_limit_exceeded",
				Message: "model tool loop exceeded the maximum number of rounds",
			}
			if appendErr := k.appendTurnFailure(sessionID, turnID, failure); appendErr != nil {
				return TurnResponse{}, appendErr
			}
			return TurnResponse{}, errors.New("model tool loop exceeded the maximum number of rounds")
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
		for _, call := range preparedCalls {
			result, err := toolGateway.Execute(ctx, sessionID, turnID, call)
			if err != nil {
				code := "tool_call_rejected"
				if errors.Is(err, ErrToolInfrastructureFailed) {
					code = "tool_infrastructure_failed"
				}
				failure := TurnError{
					Code:    code,
					Message: err.Error(),
				}
				if appendErr := k.appendTurnFailure(sessionID, turnID, failure); appendErr != nil {
					return TurnResponse{}, appendErr
				}
				return TurnResponse{}, err
			}
			forEventID := toolCallEventIDs[result.ToolCallEventID]
			if forEventID == "" {
				return TurnResponse{}, fmt.Errorf("missing tool.call event for tool_call_event_id %q", result.ToolCallEventID)
			}
			if err := k.appendToolResultEvent(sessionID, turnID, result, forEventID); err != nil {
				return TurnResponse{}, err
			}
		}
		toolRounds, err = k.modelToolRoundsFromTurnEvents(turnID)
		if err != nil {
			return TurnResponse{}, err
		}
	}
	return TurnResponse{}, errors.New("unreachable model tool loop state")
}

func (k *Kernel) submitNewTurn(req TurnRequest, sessionID string, turnID string, idempotencyKey string, ingressRisks []IngressRisk, now time.Time) ([]MemoryRecall, []ModelInputItem, error) {
	recalledMemories, err := k.recallMemories(req.InputItems)
	if err != nil {
		return nil, nil, err
	}
	modelInputs := modelInputItems(req.InputItems, recalledMemories, k.skillCatalog)
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
			SkillCatalog:     k.skillCatalogProjection().Items,
			RuntimeContext:   k.contextRuntimeSnapshot(),
			RecalledMemories: recalledMemories,
		},
	}
	if err := k.appendEvent(submitted); err != nil {
		return nil, nil, err
	}
	return recalledMemories, modelInputs, nil
}

func (k *Kernel) contextRuntimeSnapshot() *ContextRuntimeSnapshot {
	return &ContextRuntimeSnapshot{
		Provider: safeProviderStatusForInspection(k.provider.Ready()),
		Permission: PermissionInspection{
			PermissionMode: k.toolPolicy.PermissionMode,
			Sandbox:        permissionSandboxSummary(k.toolPolicy),
		},
	}
}

func permissionSandboxSummary(policy ToolPolicy) string {
	switch policy.PermissionMode {
	case PermissionModeYolo:
		return "host"
	case PermissionModeDefault:
		return "workspace"
	default:
		return "none"
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
	projection := SessionProjection{
		SessionID:        sessionID,
		Turns:            []TurnProjection{},
		Operations:       []OperationProjection{},
		Works:            []WorkProjection{},
		MemoryCandidates: []MemoryCandidateProjection{},
		Events:           []EventProjection{},
	}
	turnByID := map[string]int{}
	workByID := map[string]int{}
	candidateByID := map[string]int{}
	for _, event := range events {
		if event.SessionID != sessionID {
			continue
		}
		projection.Events = append(projection.Events, EventProjection{
			EventID:     event.EventID,
			TurnID:      event.TurnID,
			OperationID: event.OperationID,
			WorkID:      event.WorkID,
			CandidateID: event.CandidateID,
			Type:        event.Type,
			CreatedAt:   event.CreatedAt,
			Data:        event.Data,
		})
		switch event.Type {
		case "turn.submitted":
			turnByID[event.TurnID] = len(projection.Turns)
			projection.Turns = append(projection.Turns, TurnProjection{
				TurnID:           event.TurnID,
				IdempotencyKey:   event.Data.IdempotencyKey,
				Status:           "running",
				InputItems:       event.Data.InputItems,
				IngressRisks:     event.Data.IngressRisks,
				ModelInputKinds:  event.Data.ModelInputKinds,
				RecalledMemories: event.Data.RecalledMemories,
				StartedAt:        event.CreatedAt,
			})
		case "model.final":
			idx, ok := turnByID[event.TurnID]
			if !ok {
				continue
			}
			projection.Turns[idx].Status = "completed"
			if event.Data.Final != nil {
				projection.Turns[idx].FinalMessage = *event.Data.Final
			}
			projection.Turns[idx].CompletedAt = event.CreatedAt
		case "turn.failed":
			idx, ok := turnByID[event.TurnID]
			if !ok {
				continue
			}
			projection.Turns[idx].Status = "failed"
			if event.Data.TurnError != nil {
				projection.Turns[idx].Error = event.Data.TurnError
			}
			projection.Turns[idx].CompletedAt = event.CreatedAt
		case "operation.running", "operation.completed", "operation.failed", "operation.blocked", "operation.tool_infrastructure_failed":
			if event.Data.Operation != nil {
				operation := *event.Data.Operation
				replaced := false
				for i := range projection.Operations {
					if projection.Operations[i].OperationID == operation.OperationID {
						projection.Operations[i] = operation
						replaced = true
						break
					}
				}
				if !replaced {
					projection.Operations = append(projection.Operations, operation)
				}
			}
		case "work.submitted", "work.canceled":
			if event.Data.Work == nil {
				continue
			}
			work := *event.Data.Work
			if work.WorkID == "" {
				work.WorkID = event.WorkID
			}
			if work.WorkID == "" {
				return projection, fmt.Errorf("%s event missing work id", event.Type)
			}
			idx, ok := workByID[work.WorkID]
			if ok {
				merged, err := mergeWorkProjection(projection.Works[idx], work, true)
				if err != nil {
					return projection, err
				}
				projection.Works[idx] = merged
				continue
			}
			workByID[work.WorkID] = len(projection.Works)
			projection.Works = append(projection.Works, work)
		case "memory.candidate.created", "memory.candidate.approved", "memory.candidate.rejected", "memory.candidate.superseded":
			if event.Data.MemoryCandidate == nil {
				continue
			}
			candidate := *event.Data.MemoryCandidate
			if candidate.CandidateID == "" {
				candidate.CandidateID = event.CandidateID
			}
			if candidate.CandidateID == "" {
				return projection, fmt.Errorf("%s event missing candidate id", event.Type)
			}
			idx, ok := candidateByID[candidate.CandidateID]
			if ok {
				merged, err := mergeMemoryCandidateProjection(projection.MemoryCandidates[idx], candidate, true)
				if err != nil {
					return projection, err
				}
				projection.MemoryCandidates[idx] = merged
			} else {
				candidateByID[candidate.CandidateID] = len(projection.MemoryCandidates)
				projection.MemoryCandidates = append(projection.MemoryCandidates, candidate)
			}
			if event.Type == "memory.candidate.superseded" {
				if event.Data.ReplacementMemoryCandidate == nil {
					return projection, errors.New("superseded memory candidate missing replacement candidate")
				}
				replacement := *event.Data.ReplacementMemoryCandidate
				if replacement.CandidateID == "" {
					return projection, fmt.Errorf("%s event missing replacement candidate id", event.Type)
				}
				idx, ok := candidateByID[replacement.CandidateID]
				if ok {
					merged, err := mergeMemoryCandidateProjection(projection.MemoryCandidates[idx], replacement, true)
					if err != nil {
						return projection, err
					}
					projection.MemoryCandidates[idx] = merged
					continue
				}
				candidateByID[replacement.CandidateID] = len(projection.MemoryCandidates)
				projection.MemoryCandidates = append(projection.MemoryCandidates, replacement)
			}
		}
	}
	if len(projection.Events) == 0 {
		return SessionProjection{}, ErrSessionNotFound
	}
	return projection, nil
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
		turnEvents = append(turnEvents, toEvent(event))
		switch event.Type {
		case "model.final":
			if event.Data.Final != nil {
				copied := *event.Data.Final
				final = &copied
			}
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
			items = append(items, toEvent(event))
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
		id := providerToolCallID(call)
		if id == "" {
			continue
		}
		if seen[id] {
			return fmt.Errorf("%w: duplicate provider tool_call_id %q", ErrModelToolCallRejected, id)
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

func (k *Kernel) modelToolRoundsFromTurnEvents(turnID string) ([]ModelToolRound, error) {
	events, err := k.loadEvents()
	if err != nil {
		return nil, err
	}
	return modelToolRoundsFromStoredEvents(events, turnID), nil
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
				ToolCallID:      event.Data.ToolCall.ProviderToolCallID,
				ToolCallEventID: event.Data.ToolCall.ToolCallEventID,
				Name:            event.Data.ToolCall.Tool,
				Arguments:       json.RawMessage(event.Data.ToolCall.Arguments),
			})
		case "tool.result":
			if event.Data.ToolResult == nil {
				continue
			}
			current.Results = append(current.Results, ModelToolResult{
				ToolCallID:      event.Data.ToolResult.ProviderToolCallID,
				ToolCallEventID: event.Data.ToolResult.ToolCallEventID,
				Name:            event.Data.ToolResult.Tool,
				Content:         event.Data.ToolResult.Content,
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
	code := "provider_error"
	if errors.Is(err, ErrProviderUnavailable) {
		code = "provider_unavailable"
	}
	return TurnError{
		Code:    code,
		Message: redactEvidenceText(err.Error()),
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
