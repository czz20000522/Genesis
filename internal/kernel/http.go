package kernel

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
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
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleSubmitTurn(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/tools/shell.exec":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleExecShell(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/memory/candidates":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleCreateMemoryCandidate(w, r, k)
		case r.Method == http.MethodPost && isMemoryApprovePath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) || !requireEmptyBody(w, r) {
				return
			}
			handleApproveMemoryCandidate(w, r, k)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/sessions/"):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetSession(w, r, k)
		default:
			writeError(w, http.StatusNotFound, "not_found", "route not found")
		}
	})
}

func authorizeRuntimeRequest(w http.ResponseWriter, r *http.Request, k *Kernel) bool {
	if k.runtimeToken == "" {
		writeError(w, http.StatusServiceUnavailable, "runtime_token_missing", "runtime token is not configured")
		return false
	}
	expected := "Bearer " + k.runtimeToken
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "runtime token is required")
		return false
	}
	return true
}

func requireJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "content-type must be application/json")
		return false
	}
	return true
}

func requireEmptyBody(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid body: %s", err.Error()))
		return false
	}
	if len(strings.TrimSpace(string(data))) != 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be empty")
		return false
	}
	return true
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

func handleCreateMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryCandidateRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidate, err := k.CreateMemoryCandidate(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func handleApproveMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	candidateID := memoryApproveCandidateID(r.URL.Path)
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	candidate, err := k.ApproveMemoryCandidate(candidateID)
	if errors.Is(err, ErrMemoryCandidateNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func isMemoryApprovePath(path string) bool {
	return strings.HasPrefix(path, "/memory/candidates/") && strings.HasSuffix(path, "/approve")
}

func memoryApproveCandidateID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[0] != "memory" || parts[1] != "candidates" || parts[3] != "approve" {
		return ""
	}
	return strings.TrimSpace(parts[2])
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
