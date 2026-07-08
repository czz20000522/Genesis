package kernel

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func handleGetSession(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := routePathValue(r, "session_id")
	if sessionID == "" {
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

func handleListSessions(w http.ResponseWriter, _ *http.Request, k *Kernel) {
	projection, err := k.ListSessions()
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projection)
}

func handleSearchSessions(w http.ResponseWriter, r *http.Request, k *Kernel) {
	limit := 0
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid session search request")
			return
		}
		limit = parsed
	}
	projection, err := k.SearchSessions(SessionSearchRequest{
		Query: r.URL.Query().Get("q"),
		Limit: limit,
	})
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrSessionSearchInvalidRequest) {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid session search request")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projection)
}

func handleGetSessionTimeline(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := routePathValue(r, "session_id")
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

func handleGetSessionTimelineDetail(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := routePathValue(r, "session_id")
	detailRef := routePathValue(r, "detail_ref")
	if sessionID == "" || detailRef == "" {
		writeError(w, http.StatusNotFound, "not_found", "session timeline detail route not found")
		return
	}
	projection, err := k.UITimelineDetail(sessionID, detailRef)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrSessionNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "session not found")
		return
	}
	if errors.Is(err, ErrTimelineDetailNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "timeline detail not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projection)
}

func handleGetTurnContext(w http.ResponseWriter, r *http.Request, k *Kernel) {
	turnID := routePathValue(r, "turn_id")
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
	turnID := routePathValue(r, "turn_id")
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
	turnID := routePathValue(r, "turn_id")
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
