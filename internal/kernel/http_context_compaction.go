package kernel

import (
	"net/http"
	"strings"
)

func isSessionContextCompactPath(path string) (string, bool) {
	const prefix = "/sessions/"
	const suffix = "/context/compact"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	sessionID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	sessionID = strings.Trim(sessionID, "/")
	return sessionID, sessionID != ""
}

func handleCompactSessionContext(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID, ok := isSessionContextCompactPath(r.URL.Path)
	if !ok {
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
