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
		case r.Method == http.MethodGet && r.URL.Path == "/capabilities":
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			writeJSON(w, http.StatusOK, k.Capabilities())
		case r.Method == http.MethodPost && r.URL.Path == "/turn":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleSubmitTurn(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/tools/shell_exec":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleExecShell(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/work":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleSubmitWork(w, r, k)
		case r.Method == http.MethodGet && isWorkGetPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetWork(w, r, k)
		case r.Method == http.MethodPost && isWorkCancelPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleCancelWork(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/memory/candidates":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleCreateMemoryCandidate(w, r, k)
		case r.Method == http.MethodGet && r.URL.Path == "/memory/candidates":
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleListMemoryCandidates(w, r, k)
		case r.Method == http.MethodGet && isMemoryCandidateGetPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetMemoryCandidate(w, r, k)
		case r.Method == http.MethodPost && r.URL.Path == "/memory/recall":
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleRecallMemories(w, r, k)
		case r.Method == http.MethodPost && isMemoryApprovePath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleApproveMemoryCandidate(w, r, k)
		case r.Method == http.MethodPost && isMemoryRejectPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleRejectMemoryCandidate(w, r, k)
		case r.Method == http.MethodPost && isMemorySupersedePath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) || !requireJSONContentType(w, r) {
				return
			}
			handleSupersedeMemoryCandidate(w, r, k)
		case r.Method == http.MethodGet && isSessionTimelinePath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetSessionTimeline(w, r, k)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/sessions/"):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetSession(w, r, k)
		case r.Method == http.MethodGet && isTurnContextPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetTurnContext(w, r, k)
		case r.Method == http.MethodGet && isTurnAuditPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetTurnAudit(w, r, k)
		case r.Method == http.MethodGet && isTurnEventsPath(r.URL.Path):
			if !authorizeRuntimeRequest(w, r, k) {
				return
			}
			handleGetTurnEvents(w, r, k)
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

func writeKernelUnavailable(w http.ResponseWriter, err error) bool {
	if errors.Is(err, ErrLedgerUnavailable) {
		writeError(w, http.StatusServiceUnavailable, ledgerErrorCode(err), err.Error())
		return true
	}
	return false
}
