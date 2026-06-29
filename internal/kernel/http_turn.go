package kernel

import (
	"errors"
	"net/http"
	"strings"
)

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
	if errors.Is(err, ErrSessionActive) {
		writeError(w, http.StatusConflict, "session_active", err.Error())
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
	case "turn_interrupted":
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

func handleInterruptSession(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID, ok := sessionInterruptPathSessionID(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	var req TurnInterruptRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	interruption, err := k.InterruptSession(sessionID, req)
	if errors.Is(err, ErrNoActiveTurn) {
		writeError(w, http.StatusConflict, "no_active_turn", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, interruption)
}

func isSessionInterruptPath(path string) bool {
	_, ok := sessionInterruptPathSessionID(path)
	return ok
}

func sessionInterruptPathSessionID(path string) (string, bool) {
	const prefix = "/sessions/"
	const suffix = "/interrupt"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	sessionID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	sessionID = strings.Trim(sessionID, "/")
	if sessionID == "" || strings.Contains(sessionID, "/") {
		return "", false
	}
	return sessionID, true
}
