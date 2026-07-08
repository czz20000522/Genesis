package kernel

import (
	"encoding/json"
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
	if writeProviderClassifiedError(w, err) {
		return
	}
	if errors.Is(err, ErrIngressSecurityBlocked) {
		writeError(w, http.StatusForbidden, "turn_blocked_by_ingress_security", err.Error())
		return
	}
	if errors.Is(err, ErrToolInfrastructureFailed) {
		writeError(w, http.StatusServiceUnavailable, "tool_infrastructure_failed", externalBoundaryDiagnosticText(err.Error()))
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

func handleSubmitTurnStream(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req TurnRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unavailable", "streaming unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	emit := func(event TurnStreamEvent) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(payload, '\n')); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	resp, err := k.SubmitTurnStream(r.Context(), req, emit)
	if err != nil {
		streamErr := turnStreamError(resp, err)
		_ = emit(TurnStreamEvent{Type: "turn_failed", Error: &streamErr})
		return
	}
	_ = emit(turnStreamTerminalEvent(resp))
}

func turnStreamTerminalEvent(resp TurnResponse) TurnStreamEvent {
	eventType := "turn_completed"
	if resp.Pause != nil {
		eventType = "turn_paused"
	}
	return TurnStreamEvent{Type: eventType, Response: &resp}
}

func turnStreamError(resp TurnResponse, err error) TurnError {
	if resp.Error != nil {
		return *resp.Error
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	switch {
	case errors.Is(err, ErrProviderUnavailable):
		return TurnError{Code: "provider_unavailable", Message: message}
	case errors.Is(err, ErrIngressSecurityBlocked):
		return TurnError{Code: "turn_blocked_by_ingress_security", Message: message}
	case errors.Is(err, ErrToolInfrastructureFailed):
		return TurnError{Code: "tool_infrastructure_failed", Message: externalBoundaryDiagnosticText(message)}
	case errors.Is(err, ErrSessionActive):
		return TurnError{Code: "session_active", Message: message}
	default:
		var classified *ProviderClassifiedError
		if errors.As(err, &classified) {
			code := strings.TrimSpace(classified.Code)
			if code == "" {
				code = "provider_error"
			}
			return TurnError{Code: code, Message: message}
		}
		return TurnError{Code: "turn_failed", Message: message}
	}
}

func turnErrorHTTPStatus(err TurnError) int {
	switch err.Code {
	case "provider_unavailable":
		return http.StatusServiceUnavailable
	case "turn_blocked_by_ingress_security":
		return http.StatusForbidden
	case "tool_infrastructure_failed":
		return http.StatusServiceUnavailable
	case "session_active":
		return http.StatusConflict
	case "turn_interrupted":
		return http.StatusConflict
	default:
		if strings.HasPrefix(err.Code, "provider_") {
			return providerErrorHTTPStatus(err.Code)
		}
		return http.StatusBadRequest
	}
}

func writeProviderClassifiedError(w http.ResponseWriter, err error) bool {
	var classified *ProviderClassifiedError
	if !errors.As(err, &classified) {
		return false
	}
	code := strings.TrimSpace(classified.Code)
	if code == "" {
		code = "provider_error"
	}
	writeError(w, providerErrorHTTPStatus(code), code, err.Error())
	return true
}

func providerErrorHTTPStatus(code string) int {
	switch code {
	case "provider_transient_failure", "provider_unavailable":
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
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
