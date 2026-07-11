package kernel

import (
	"errors"
	"net/http"
)

type sessionWorkspaceHTTPRequest struct {
	Kind string `json:"kind"`
	Root string `json:"root"`
}

func handleBindSessionWorkspace(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := routePathValue(r, "session_id")
	if sessionID == "" {
		writeError(w, http.StatusNotFound, "not_found", "session route not found")
		return
	}
	var request sessionWorkspaceHTTPRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	if err := k.BindSessionWorkspace(sessionID, SessionWorkspaceBindingRequest{Kind: request.Kind, Root: request.Root}); err != nil {
		if writeKernelUnavailable(w, err) {
			return
		}
		if errors.Is(err, ErrSessionWorkspaceAlreadyBound) {
			writeError(w, http.StatusConflict, "session_workspace_already_bound", "session workspace is already bound")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid session workspace binding")
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
