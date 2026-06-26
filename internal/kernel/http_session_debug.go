package kernel

import (
	"errors"
	"net/http"
	"strings"
)

func isSessionDebugEnablePath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 4 && parts[0] == "sessions" && strings.TrimSpace(parts[1]) != "" && parts[2] == "debug" && parts[3] == "enable"
}

func isSessionDebugExportPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "sessions" && strings.TrimSpace(parts[1]) != "" && parts[2] == "debug"
}

func sessionDebugID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[0] != "sessions" || parts[2] != "debug" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func handleEnableSessionDebug(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req struct{}
	if !decodeRequest(w, r, &req) {
		return
	}
	sessionID := sessionDebugID(r.URL.Path)
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
	sessionID := sessionDebugID(r.URL.Path)
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
