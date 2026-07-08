package kernel

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	sessionDebugTextBytes       = 16 * 1024
	sessionDebugToolResultBytes = 16 * 1024
	sessionDebugMaxSteps        = 50
)

type SessionDebugProjection struct {
	SessionID string    `json:"session_id"`
	Enabled   bool      `json:"enabled"`
	EnabledAt time.Time `json:"enabled_at"`
}

type SessionDebugControlResponse struct {
	SessionID string `json:"session_id"`
	Readiness string `json:"readiness"`
	Enabled   bool   `json:"enabled"`
}

type SessionDebugExportResponse struct {
	SessionID       string                     `json:"session_id"`
	Readiness       string                     `json:"readiness"`
	ReadinessReason string                     `json:"readiness_reason,omitempty"`
	CaptureBounds   SessionDebugCaptureBounds  `json:"capture_bounds"`
	Steps           []SessionDebugProviderStep `json:"steps,omitempty"`
}

type SessionDebugCaptureBounds struct {
	MaxInputItemBytes  int  `json:"max_input_item_bytes"`
	MaxToolResultBytes int  `json:"max_tool_result_bytes"`
	MaxSteps           int  `json:"max_steps"`
	RetainedSteps      int  `json:"retained_steps,omitempty"`
	Truncated          bool `json:"truncated,omitempty"`
}

type SessionDebugProviderStep struct {
	SessionID                 string                          `json:"session_id"`
	TurnID                    string                          `json:"turn_id"`
	ProviderStep              int                             `json:"provider_step"`
	Attempt                   int                             `json:"attempt,omitempty"`
	CapturedAt                time.Time                       `json:"captured_at"`
	Provider                  ProviderStatus                  `json:"provider"`
	Model                     string                          `json:"model,omitempty"`
	ModelInputKinds           []string                        `json:"model_input_kinds,omitempty"`
	InputItems                []SessionDebugInputItem         `json:"input_items,omitempty"`
	ToolManifest              []ToolManifestInspection        `json:"tool_manifest,omitempty"`
	SkillSummaries            []SessionDebugInputItem         `json:"skill_summaries,omitempty"`
	SkillWarnings             []SkillCatalogWarningProjection `json:"skill_warnings,omitempty"`
	SkillRoots                []SkillCatalogRootProjection    `json:"skill_roots,omitempty"`
	SourceContext             []SessionDebugInputItem         `json:"source_context,omitempty"`
	HydratedContext           []SessionDebugInputItem         `json:"hydrated_context,omitempty"`
	ToolRounds                []SessionDebugToolRound         `json:"tool_rounds,omitempty"`
	ToolCalls                 []SessionDebugToolCallSummary   `json:"tool_calls,omitempty"`
	KernelObservationEventIDs []string                        `json:"kernel_observation_event_ids,omitempty"`
	Final                     *FinalMessage                   `json:"final,omitempty"`
	Error                     *ProviderAttemptProjection      `json:"error,omitempty"`
	Usage                     *TokenUsage                     `json:"usage,omitempty"`
	CaptureBounds             SessionDebugCaptureBounds       `json:"capture_bounds"`
}

type SessionDebugInputItem struct {
	Kind          string `json:"kind"`
	Text          string `json:"text,omitempty"`
	OriginalBytes int    `json:"original_bytes,omitempty"`
	VisibleBytes  int    `json:"visible_bytes,omitempty"`
	Truncated     bool   `json:"truncated,omitempty"`
}

type SessionDebugToolRound struct {
	Calls   []SessionDebugToolCallSummary   `json:"calls,omitempty"`
	Results []SessionDebugToolResultSummary `json:"results,omitempty"`
}

type SessionDebugToolCallSummary struct {
	ToolCallID     string   `json:"tool_call_id,omitempty"`
	Name           string   `json:"name"`
	ArgumentFields []string `json:"argument_fields,omitempty"`
	ArgumentsBytes int      `json:"arguments_bytes,omitempty"`
}

type SessionDebugToolResultSummary struct {
	ToolCallID     string `json:"tool_call_id,omitempty"`
	Name           string `json:"name"`
	ContentPreview string `json:"content_preview,omitempty"`
	OriginalBytes  int    `json:"original_bytes,omitempty"`
	VisibleBytes   int    `json:"visible_bytes,omitempty"`
	Truncated      bool   `json:"truncated,omitempty"`
}

func (k *Kernel) EnableSessionDebug(sessionID string) (SessionDebugControlResponse, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return SessionDebugControlResponse{}, errors.New("session id is required")
	}
	now := k.clock()
	projection := SessionDebugProjection{SessionID: sessionID, Enabled: true, EnabledAt: now}
	if err := k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: sessionID,
		Type:      "session.debug.enabled",
		CreatedAt: now,
		Data:      EventData{SessionDebug: &projection},
	}); err != nil {
		return SessionDebugControlResponse{}, err
	}
	return SessionDebugControlResponse{SessionID: sessionID, Readiness: ReadinessReady, Enabled: true}, nil
}

func (k *Kernel) SessionDebugExport(sessionID string) (SessionDebugExportResponse, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return SessionDebugExportResponse{}, errors.New("session id is required")
	}
	bounds := SessionDebugCaptureBounds{MaxInputItemBytes: sessionDebugTextBytes, MaxToolResultBytes: sessionDebugToolResultBytes, MaxSteps: sessionDebugMaxSteps}
	if !k.sessionDebugEnabled(sessionID) {
		return SessionDebugExportResponse{
			SessionID:       sessionID,
			Readiness:       ReadinessNotReady,
			ReadinessReason: "session_debug_disabled",
			CaptureBounds:   bounds,
		}, nil
	}
	path := k.sessionDebugArtifactPath(sessionID)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return SessionDebugExportResponse{SessionID: sessionID, Readiness: ReadinessNotReady, ReadinessReason: "session_debug_artifact_missing", CaptureBounds: bounds}, nil
	}
	if err != nil {
		return SessionDebugExportResponse{}, err
	}
	defer file.Close()
	steps := []SessionDebugProviderStep{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var step SessionDebugProviderStep
		if err := json.Unmarshal(scanner.Bytes(), &step); err != nil {
			return SessionDebugExportResponse{}, err
		}
		steps = append(steps, step)
	}
	if err := scanner.Err(); err != nil {
		return SessionDebugExportResponse{}, err
	}
	bounds.RetainedSteps = len(steps)
	if len(steps) >= sessionDebugMaxSteps {
		bounds.Truncated = true
	}
	return SessionDebugExportResponse{
		SessionID:     sessionID,
		Readiness:     ReadinessReady,
		CaptureBounds: bounds,
		Steps:         steps,
	}, nil
}

func (k *Kernel) sessionDebugEnabled(sessionID string) bool {
	events, err := k.loadEvents()
	if err != nil {
		return false
	}
	for _, event := range events {
		if event.SessionID != sessionID || event.Type != "session.debug.enabled" {
			continue
		}
		if event.Data.SessionDebug == nil || event.Data.SessionDebug.Enabled {
			return true
		}
	}
	return false
}

func (k *Kernel) captureSessionDebugProviderStep(sessionID string, turnID string, providerStep int, attempt int, request ModelRequest, response ModelResponse, providerErr error, observationEventIDs []string) {
	if !k.sessionDebugEnabled(sessionID) {
		return
	}
	step := SessionDebugProviderStep{
		SessionID:                 sessionID,
		TurnID:                    turnID,
		ProviderStep:              providerStep,
		Attempt:                   attempt,
		CapturedAt:                k.clock(),
		Provider:                  safeProviderStatusForInspection(k.provider.Ready()),
		Model:                     safeInspectionToken(response.Model, "model_unavailable"),
		ModelInputKinds:           modelInputKindsFromModelItems(request.InputItems),
		InputItems:                sessionDebugInputItems(request.InputItems, sessionDebugTextBytes),
		ToolManifest:              toolManifestInspection(request.ToolManifest),
		SkillSummaries:            sessionDebugItemsByKind(request.InputItems, ModelInputKindSkillIndexContext),
		SkillWarnings:             skillIndexWarnings(k.skillCatalogProjection().Items, k.contextPolicy.SkillIndexChars),
		SkillRoots:                append([]SkillCatalogRootProjection(nil), k.skillCatalogProjection().Roots...),
		SourceContext:             sessionDebugItemsByKind(request.InputItems, ModelInputKindSourceSnapshotContext),
		HydratedContext:           sessionDebugItemsByKind(request.InputItems, ModelInputKindHydratedContext),
		ToolRounds:                sessionDebugToolRounds(request.ToolRounds),
		ToolCalls:                 sessionDebugToolCalls(response.ToolCalls),
		KernelObservationEventIDs: append([]string(nil), observationEventIDs...),
		Usage:                     response.Usage,
		CaptureBounds:             SessionDebugCaptureBounds{MaxInputItemBytes: sessionDebugTextBytes, MaxToolResultBytes: sessionDebugToolResultBytes, MaxSteps: sessionDebugMaxSteps},
	}
	if providerErr != nil {
		failure := providerFailureFromError(providerErr)
		step.Error = &failure
	} else if len(response.ToolCalls) == 0 {
		final := FinalMessage{Text: debugBoundedText(response.Text, sessionDebugTextBytes).Text, Model: response.Model, Usage: response.Usage}
		step.Final = &final
	}
	if err := k.writeSessionDebugStep(sessionID, step); err != nil {
		return
	}
}

func (k *Kernel) writeSessionDebugStep(sessionID string, step SessionDebugProviderStep) error {
	path := k.sessionDebugArtifactPath(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	steps := []SessionDebugProviderStep{}
	file, err := os.Open(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err != nil {
		file = nil
	}
	if file != nil {
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			var existing SessionDebugProviderStep
			if err := json.Unmarshal(scanner.Bytes(), &existing); err != nil {
				_ = file.Close()
				return err
			}
			steps = append(steps, existing)
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	steps = append(steps, step)
	if len(steps) > sessionDebugMaxSteps {
		steps = steps[len(steps)-sessionDebugMaxSteps:]
		for i := range steps {
			steps[i].CaptureBounds.Truncated = true
		}
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	encoder := json.NewEncoder(out)
	for _, retained := range steps {
		if err := encoder.Encode(retained); err != nil {
			return err
		}
	}
	return nil
}

func (k *Kernel) sessionDebugArtifactPath(sessionID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sessionID)))
	name := hex.EncodeToString(sum[:]) + ".jsonl"
	return filepath.Join(k.materialStorePath, "session-debug", name)
}

func sessionDebugInputItems(items []ModelInputItem, limit int) []SessionDebugInputItem {
	out := make([]SessionDebugInputItem, 0, len(items))
	for _, item := range items {
		bounded := debugBoundedText(item.Text, limit)
		out = append(out, SessionDebugInputItem{
			Kind:          item.Kind,
			Text:          bounded.Text,
			OriginalBytes: bounded.OriginalBytes,
			VisibleBytes:  bounded.VisibleBytes,
			Truncated:     bounded.Truncated,
		})
	}
	return out
}

func sessionDebugItemsByKind(items []ModelInputItem, kind string) []SessionDebugInputItem {
	filtered := make([]ModelInputItem, 0, len(items))
	for _, item := range items {
		if item.Kind == kind {
			filtered = append(filtered, item)
		}
	}
	return sessionDebugInputItems(filtered, sessionDebugTextBytes)
}

func sessionDebugToolRounds(rounds []ModelToolRound) []SessionDebugToolRound {
	out := make([]SessionDebugToolRound, 0, len(rounds))
	for _, round := range rounds {
		out = append(out, SessionDebugToolRound{
			Calls:   sessionDebugToolCalls(round.Calls),
			Results: sessionDebugToolResults(round.Results),
		})
	}
	return out
}

func sessionDebugToolCalls(calls []ModelToolCall) []SessionDebugToolCallSummary {
	out := make([]SessionDebugToolCallSummary, 0, len(calls))
	for _, call := range calls {
		out = append(out, SessionDebugToolCallSummary{
			ToolCallID:     redactProviderToolCallID(call.ToolCallID),
			Name:           call.Name,
			ArgumentFields: argumentFieldNames(call.Arguments),
			ArgumentsBytes: len(call.Arguments),
		})
	}
	return out
}

func sessionDebugToolResults(results []ModelToolResult) []SessionDebugToolResultSummary {
	out := make([]SessionDebugToolResultSummary, 0, len(results))
	for _, result := range results {
		bounded := debugBoundedText(result.Content, sessionDebugToolResultBytes)
		out = append(out, SessionDebugToolResultSummary{
			ToolCallID:     redactProviderToolCallID(result.ToolCallID),
			Name:           result.Name,
			ContentPreview: bounded.Text,
			OriginalBytes:  bounded.OriginalBytes,
			VisibleBytes:   bounded.VisibleBytes,
			Truncated:      bounded.Truncated,
		})
	}
	return out
}

func argumentFieldNames(raw json.RawMessage) []string {
	var payload map[string]interface{}
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	fields := make([]string, 0, len(payload))
	for field := range payload {
		fields = append(fields, safeInspectionToken(field, "argument_field_unavailable"))
	}
	sort.Strings(fields)
	return fields
}

func modelInputKindsFromModelItems(items []ModelInputItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Kind) != "" {
			out = append(out, item.Kind)
		}
	}
	return out
}

type debugBoundedTextResult struct {
	Text          string
	OriginalBytes int
	VisibleBytes  int
	Truncated     bool
}

func debugBoundedText(text string, limit int) debugBoundedTextResult {
	redacted := externalBoundaryDiagnosticText(text)
	originalBytes := len([]byte(redacted))
	if limit <= 0 || originalBytes <= limit {
		return debugBoundedTextResult{Text: redacted, OriginalBytes: originalBytes, VisibleBytes: originalBytes}
	}
	visible := utf8SafePrefix(redacted, limit)
	return debugBoundedTextResult{Text: visible, OriginalBytes: originalBytes, VisibleBytes: len([]byte(visible)), Truncated: true}
}

func utf8SafePrefix(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	data := []byte(text)
	if len(data) <= limit {
		return text
	}
	end := limit
	for end > 0 && !utf8.Valid(data[:end]) {
		end--
	}
	return string(data[:end])
}
