package kernel

import (
	"errors"
	"net/http"
)

func handleEnableSessionDebug(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req struct{}
	if !decodeRequest(w, r, &req) {
		return
	}
	sessionID := routePathValue(r, "session_id")
	if sessionID == "" {
		writeError(w, http.StatusNotFound, "not_found", "session debug route not found")
		return
	}
	resp, err := k.EnableSessionDebug(sessionID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleGetSessionDebug(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := routePathValue(r, "session_id")
	if sessionID == "" {
		writeError(w, http.StatusNotFound, "not_found", "session debug route not found")
		return
	}
	resp, err := k.SessionDebugExport(sessionID)
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
	writeJSON(w, http.StatusOK, resp)
}
