package kernel

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
)

const maxRequestBytes = 1024 * 1024

func Handler(k *Kernel) http.Handler {
	mux := http.NewServeMux()
	route := func(method string, pattern string, requireJSON bool, handler func(http.ResponseWriter, *http.Request, *Kernel)) {
		mux.HandleFunc(method+" "+pattern, func(w http.ResponseWriter, r *http.Request) {
			if !authorizeRuntimeRequest(w, r, k) || (requireJSON && !requireJSONContentType(w, r)) {
				return
			}
			handler(w, r, k)
		})
	}

	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, k.Ready())
	})
	route(http.MethodGet, "/capabilities", false, func(w http.ResponseWriter, r *http.Request, k *Kernel) {
		writeJSON(w, http.StatusOK, k.Capabilities())
	})
	route(http.MethodPost, "/providers/verify", true, handleVerifyProvider)
	route(http.MethodPost, "/providers/{route_id}/models/discover", true, handleDiscoverProviderRouteModels)
	route(http.MethodPost, "/discovery/query", true, handleDiscoveryQuery)
	route(http.MethodPost, "/turn", true, handleSubmitTurn)
	route(http.MethodPost, "/turn/stream", true, handleSubmitTurnStream)
	route(http.MethodPost, "/sessions/{session_id}/interrupt", true, handleInterruptSession)
	route(http.MethodPost, "/sessions/{session_id}/workspace", true, handleBindSessionWorkspace)
	route(http.MethodPost, "/sessions/{session_id}/model", true, handleBindSessionModel)
	route(http.MethodPost, "/tools/shell_exec", true, handleExecShell)
	route(http.MethodPost, "/context/admit_resource", true, handleAdmitContextResource)
	route(http.MethodPost, "/materials/intake", true, handleIntakeMaterial)
	route(http.MethodPost, "/materials/upload", false, handleUploadMaterial)
	route(http.MethodGet, "/approvals", false, handleListApprovals)
	route(http.MethodPost, "/approvals/{approval_id}/decision", true, handleDecideApproval)
	route(http.MethodPost, "/work", true, handleSubmitWork)
	route(http.MethodGet, "/work/{work_id}", false, handleGetWork)
	route(http.MethodPost, "/work/{work_id}/cancel", true, handleCancelWork)
	route(http.MethodPost, "/agent-invocations", true, handleAdmitAgentInvocation)
	route(http.MethodGet, "/agent-invocations/{invocation_id}/child-conversation", false, handleGetAgentInvocationChildConversation)
	route(http.MethodGet, "/agent-invocations/{invocation_id}", false, handleGetAgentInvocation)
	route(http.MethodPost, "/memory/candidates", true, handleCreateMemoryCandidate)
	route(http.MethodGet, "/memory/candidates", false, handleListMemoryCandidates)
	route(http.MethodGet, "/memory/candidates/{candidate_id}", false, handleGetMemoryCandidate)
	route(http.MethodPost, "/memory/candidates/{candidate_id}/approve", true, handleApproveMemoryCandidate)
	route(http.MethodPost, "/memory/candidates/{candidate_id}/reject", true, handleRejectMemoryCandidate)
	route(http.MethodPost, "/memory/candidates/{candidate_id}/supersede", true, handleSupersedeMemoryCandidate)
	route(http.MethodPost, "/memory/candidates/{candidate_id}/forget", true, handleForgetMemoryCandidate)
	route(http.MethodGet, "/sessions/{session_id}/timeline", false, handleGetSessionTimeline)
	route(http.MethodGet, "/sessions/{session_id}/timeline/details/{detail_ref}", false, handleGetSessionTimelineDetail)
	route(http.MethodPost, "/sessions/{session_id}/debug/enable", true, handleEnableSessionDebug)
	route(http.MethodGet, "/sessions/{session_id}/debug", false, handleGetSessionDebug)
	route(http.MethodPost, "/sessions/{session_id}/context/compact", true, handleCompactSessionContext)
	route(http.MethodGet, "/sessions", false, handleListSessions)
	route(http.MethodGet, "/sessions/search", false, handleSearchSessions)
	route(http.MethodGet, "/sessions/{session_id}/agent-invocations", false, handleListSessionAgentInvocations)
	route(http.MethodGet, "/sessions/{session_id}/task-graphs", false, handleListSessionTaskGraphs)
	route(http.MethodGet, "/task-graphs/{graph_id}", false, handleGetTaskGraph)
	route(http.MethodGet, "/sessions/{session_id}", false, handleGetSession)
	route(http.MethodGet, "/turns/{turn_id}/context", false, handleGetTurnContext)
	route(http.MethodGet, "/turns/{turn_id}/audit", false, handleGetTurnAudit)
	route(http.MethodGet, "/turns/{turn_id}/events", false, handleGetTurnEvents)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead || uncleanRequestPath(r.URL.Path) {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}
		mux.ServeHTTP(&methodNotAllowedWriter{ResponseWriter: w}, r)
	})
}

func uncleanRequestPath(requestPath string) bool {
	if requestPath == "" || requestPath[0] != '/' {
		return true
	}
	return path.Clean(requestPath) != requestPath
}

func routePathValue(r *http.Request, name string) string {
	return strings.TrimSpace(r.PathValue(name))
}

type methodNotAllowedWriter struct {
	http.ResponseWriter
	wrote             bool
	suppressErrorBody bool
}

func (w *methodNotAllowedWriter) WriteHeader(status int) {
	if status == http.StatusMethodNotAllowed {
		w.ResponseWriter.Header().Del("Allow")
		writeError(w.ResponseWriter, http.StatusNotFound, "not_found", "route not found")
		w.wrote = true
		w.suppressErrorBody = true
		return
	}
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *methodNotAllowedWriter) Write(payload []byte) (int, error) {
	if w.suppressErrorBody {
		return len(payload), nil
	}
	if w.wrote {
		return w.ResponseWriter.Write(payload)
	}
	return w.ResponseWriter.Write(payload)
}

func (w *methodNotAllowedWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
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
		code := ledgerErrorCode(err)
		writeError(w, http.StatusServiceUnavailable, code, code)
		return true
	}
	return false
}
