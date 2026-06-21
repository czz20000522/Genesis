package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxRequestBytes = 1024 * 1024

func Handler(k *Kernel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ready":
			writeJSON(w, http.StatusOK, k.Ready())
		case r.Method == http.MethodPost && r.URL.Path == "/turn":
			handleSubmitTurn(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/tools/shell.exec":
			handleExecShell(w, r, k)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/sessions/"):
			handleGetSession(w, r, k)
		default:
			writeError(w, http.StatusNotFound, "not_found", "route not found")
		}
	})
}

func handleSubmitTurn(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req TurnRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := k.SubmitTurn(r.Context(), req)
	if errors.Is(err, ErrProviderUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "provider_unavailable", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleExecShell(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req ShellExecRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	operation, err := k.ExecShell(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, operation)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid JSON: %s", err.Error()))
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body contains trailing data")
		return false
	}
	return true
}

func handleGetSession(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/sessions/")
	sessionID = strings.Trim(sessionID, "/")
	if sessionID == "" || strings.Contains(sessionID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "session route not found")
		return
	}
	projection, err := k.Session(sessionID)
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

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorEnvelope{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
	})
}
