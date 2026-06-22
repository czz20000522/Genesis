package kernel

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

const maxRequestBytes = 1024 * 1024

func Handler(k *Kernel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ready":
			writeJSON(w, http.StatusOK, k.Ready())
		case r.Method == http.MethodGet && r.URL.Path == "/capabilities":
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			writeJSON(w, http.StatusOK, k.Capabilities())
		case r.Method == http.MethodPost && r.URL.Path == "/turn":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleSubmitTurn(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/tools/shell_exec":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleExecShell(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/work":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleSubmitWork(w, r, k)
		case r.Method == http.MethodGet && isWorkGetPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetWork(w, r, k)
		case r.Method == http.MethodPost && isWorkCancelPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleCancelWork(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/memory/candidates":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleCreateMemoryCandidate(w, r, k)
		case r.Method == http.MethodGet && r.URL.Path == "/memory/candidates":
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleListMemoryCandidates(w, r, k)
		case r.Method == http.MethodGet && isMemoryCandidateGetPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetMemoryCandidate(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/memory/recall":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleRecallMemories(w, r, k)
		case r.Method == http.MethodPost && isMemoryApprovePath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleApproveMemoryCandidate(w, r, k)
		case r.Method == http.MethodPost && isMemoryRejectPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleRejectMemoryCandidate(w, r, k)
		case r.Method == http.MethodPost && isMemorySupersedePath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleSupersedeMemoryCandidate(w, r, k)
		case r.Method == http.MethodGet && isSessionTimelinePath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetSessionTimeline(w, r, k)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/sessions/"):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetSession(w, r, k)
		case r.Method == http.MethodGet && isTurnContextPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetTurnContext(w, r, k)
		case r.Method == http.MethodGet && isTurnEventsPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetTurnEvents(w, r, k)
		default:
			writeError(w, http.StatusNotFound, "not_found", "route not found")
		}
	})
}

func authorizeRuntimeRequest(w http.ResponseWriter, r *http.Request, k *Kernel) bool {
	if k.runtimeToken == "" {
		writeError(w, http.StatusServiceUnavailable, "runtime_token_missing", "runtime token is not configured")
		return false
	}
	expected := "Bearer " + k.runtimeToken
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "runtime token is required")
		return false
	}
	return true
}

func requireJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "content-type must be application/json")
		return false
	}
	return true
}

func handleSubmitTurn(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req TurnRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := k.SubmitTurn(r.Context(), req)
	if err != nil && resp.TurnID != "" && resp.Error != nil {
		writeJSON(w, turnErrorHTTPStatus(*resp.Error), resp)
		return
	}
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrProviderUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "provider_unavailable", err.Error())
		return
	}
	if errors.Is(err, ErrIngressSecurityBlocked) {
		writeError(w, http.StatusForbidden, "turn_blocked_by_ingress_security", err.Error())
		return
	}
	if errors.Is(err, ErrToolInfrastructureFailed) {
		writeError(w, http.StatusServiceUnavailable, "tool_infrastructure_failed", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func turnErrorHTTPStatus(err TurnError) int {
	switch err.Code {
	case "provider_unavailable":
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

func handleExecShell(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req ShellExecRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	operation, err := k.ExecShell(r.Context(), req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrToolInfrastructureFailed) {
		writeError(w, http.StatusServiceUnavailable, "tool_infrastructure_failed", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, operation)
}

func handleSubmitWork(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req WorkSubmitRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	work, err := k.SubmitWork(req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, work)
}

func handleGetWork(w http.ResponseWriter, r *http.Request, k *Kernel) {
	workID := workReadID(r.URL.Path)
	if workID == "" {
		writeError(w, http.StatusNotFound, "not_found", "work route not found")
		return
	}
	work, err := k.Work(workID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrWorkNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "work not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, work)
}

func handleCancelWork(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req WorkCancelRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	workID := workCancelID(r.URL.Path)
	if workID == "" {
		writeError(w, http.StatusNotFound, "not_found", "work route not found")
		return
	}
	work, err := k.CancelWork(workID, req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrWorkNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "work not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, work)
}

func handleCreateMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryCandidateRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidate, err := k.CreateMemoryCandidate(req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func handleListMemoryCandidates(w http.ResponseWriter, r *http.Request, k *Kernel) {
	status := r.URL.Query().Get("status")
	candidates, err := k.MemoryCandidates(status)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, MemoryCandidateListResponse{Items: candidates})
}

func handleGetMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	candidateID := memoryCandidateReadID(r.URL.Path)
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	candidate, err := k.MemoryCandidate(candidateID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrMemoryCandidateNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func handleRecallMemories(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryRecallRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	recalls, err := k.RecallMemories(req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrIngressSecurityBlocked) {
		writeError(w, http.StatusForbidden, "memory_recall_blocked_by_ingress_security", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recalls)
}

func handleApproveMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryApprovalRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidateID := memoryApproveCandidateID(r.URL.Path)
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	candidate, err := k.ApproveMemoryCandidate(candidateID, req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrMemoryCandidateNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func handleRejectMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryRejectionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidateID := memoryRejectCandidateID(r.URL.Path)
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	candidate, err := k.RejectMemoryCandidate(candidateID, req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrMemoryCandidateNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func handleSupersedeMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemorySupersessionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidateID := memorySupersedeCandidateID(r.URL.Path)
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	supersession, err := k.SupersedeMemoryCandidate(candidateID, req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrMemoryCandidateNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, supersession)
}

func isWorkGetPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 2 && parts[0] == "work" && strings.TrimSpace(parts[1]) != ""
}

func workReadID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] != "work" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func isWorkCancelPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "work" && strings.TrimSpace(parts[1]) != "" && parts[2] == "cancel"
}

func workCancelID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "work" || parts[2] != "cancel" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func isMemoryCandidateGetPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "memory" && parts[1] == "candidates" && strings.TrimSpace(parts[2]) != ""
}

func memoryCandidateReadID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "memory" || parts[1] != "candidates" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

func isMemoryApprovePath(path string) bool {
	return strings.HasPrefix(path, "/memory/candidates/") && strings.HasSuffix(path, "/approve")
}

func isMemoryRejectPath(path string) bool {
	return strings.HasPrefix(path, "/memory/candidates/") && strings.HasSuffix(path, "/reject")
}

func isMemorySupersedePath(path string) bool {
	return strings.HasPrefix(path, "/memory/candidates/") && strings.HasSuffix(path, "/supersede")
}

func memoryApproveCandidateID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[0] != "memory" || parts[1] != "candidates" || parts[3] != "approve" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

func memoryRejectCandidateID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[0] != "memory" || parts[1] != "candidates" || parts[3] != "reject" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

func memorySupersedeCandidateID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[0] != "memory" || parts[1] != "candidates" || parts[3] != "supersede" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid JSON: %s", err.Error()))
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body contains trailing data")
		return false
	}
	return true
}

func handleGetSession(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/sessions/")
	sessionID = strings.Trim(sessionID, "/")
	if sessionID == "" || strings.Contains(sessionID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "session route not found")
		return
	}
	projection, err := k.Session(sessionID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrSessionNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projection)
}

func isSessionTimelinePath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "sessions" && strings.TrimSpace(parts[1]) != "" && parts[2] == "timeline"
}

func sessionTimelineID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "sessions" || parts[2] != "timeline" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func handleGetSessionTimeline(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := sessionTimelineID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusNotFound, "not_found", "session timeline route not found")
		return
	}
	projection, err := k.UITimeline(sessionID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrSessionNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projection)
}

func isTurnEventsPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "turns" && strings.TrimSpace(parts[1]) != "" && parts[2] == "events"
}

func turnEventsID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "turns" || parts[2] != "events" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func isTurnContextPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "turns" && strings.TrimSpace(parts[1]) != "" && parts[2] == "context"
}

func turnContextID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "turns" || parts[2] != "context" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func handleGetTurnContext(w http.ResponseWriter, r *http.Request, k *Kernel) {
	turnID := turnContextID(r.URL.Path)
	if turnID == "" {
		writeError(w, http.StatusNotFound, "not_found", "turn context route not found")
		return
	}
	projection, err := k.ContextInspection(turnID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrTurnNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "turn not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projection)
}

func handleGetTurnEvents(w http.ResponseWriter, r *http.Request, k *Kernel) {
	turnID := turnEventsID(r.URL.Path)
	if turnID == "" {
		writeError(w, http.StatusNotFound, "not_found", "turn event route not found")
		return
	}
	events, err := k.TurnEvents(turnID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrTurnNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "turn not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, TurnEventsResponse{Items: events})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorEnvelope{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
	})
}

func writeKernelUnavailable(w http.ResponseWriter, err error) bool {
	if errors.Is(err, ErrLedgerUnavailable) {
		writeError(w, http.StatusServiceUnavailable, ledgerErrorCode(err), err.Error())
		return true
	}
	return false
}
