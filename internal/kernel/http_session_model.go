package kernel

import (
	"errors"
	"net/http"
)

type sessionModelHTTPRequest struct {
	ProfileID string `json:"profile_id"`
}

func handleBindSessionModel(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := routePathValue(r, "session_id")
	if sessionID == "" {
		writeError(w, http.StatusNotFound, "not_found", "session route not found")
		return
	}
	var request sessionModelHTTPRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	if err := k.BindSessionModel(sessionID, SessionModelBindingRequest{ProfileID: request.ProfileID}); err != nil {
		if writeKernelUnavailable(w, err) {
			return
		}
		if errors.Is(err, ErrProviderUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "provider_unavailable", err.Error())
			return
		}
		if errors.Is(err, ErrSessionModelChangeBlockedActiveTurn) {
			writeError(w, http.StatusConflict, "session_model_change_blocked_active_turn", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid session model binding")
		return
	}
	projection, err := k.Session(sessionID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projection)
}
