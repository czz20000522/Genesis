package kernel

import (
	"net/http"
)

func handleCompactSessionContext(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := routePathValue(r, "session_id")
	if sessionID == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	var req ContextCompactionControlRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := k.CompactSessionContext(r.Context(), sessionID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	status := http.StatusOK
	if resp.AdmissionResult == contextCompactionAdmissionRefused {
		status = http.StatusConflict
	}
	writeJSON(w, status, resp)
}
