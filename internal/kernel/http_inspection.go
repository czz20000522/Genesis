package kernel

import (
	"errors"
	"net/http"
	"strings"
)

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

func isTurnAuditPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "turns" && strings.TrimSpace(parts[1]) != "" && parts[2] == "audit"
}

func turnAuditID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "turns" || parts[2] != "audit" {
		return ""
	}
	return strings.TrimSpace(parts[1])
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

func handleGetTurnAudit(w http.ResponseWriter, r *http.Request, k *Kernel) {
	turnID := turnAuditID(r.URL.Path)
	if turnID == "" {
		writeError(w, http.StatusNotFound, "not_found", "turn audit route not found")
		return
	}
	projection, err := k.AuditReplay(turnID)
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
