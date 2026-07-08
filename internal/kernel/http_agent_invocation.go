package kernel

import (
	"errors"
	"net/http"
	"strings"
)

func handleAdmitAgentInvocation(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req AgentInvocationAdmissionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	invocation, err := k.AdmitAgentInvocation(req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, invocation)
}

func handleGetAgentInvocation(w http.ResponseWriter, r *http.Request, k *Kernel) {
	invocationID := agentInvocationID(r.URL.Path)
	if invocationID == "" {
		writeError(w, http.StatusNotFound, "not_found", "agent invocation route not found")
		return
	}
	invocation, err := k.AgentInvocation(invocationID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrAgentInvocationNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "agent invocation not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, invocation)
}

func handleListSessionAgentInvocations(w http.ResponseWriter, r *http.Request, k *Kernel) {
	sessionID := sessionAgentInvocationsID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusNotFound, "not_found", "agent invocation route not found")
		return
	}
	invocations, err := k.AgentInvocations(sessionID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, invocations)
}

func isAgentInvocationGetPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 2 && parts[0] == "agent-invocations" && strings.TrimSpace(parts[1]) != ""
}

func agentInvocationID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] != "agent-invocations" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func isSessionAgentInvocationsPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "sessions" && strings.TrimSpace(parts[1]) != "" && parts[2] == "agent-invocations"
}

func sessionAgentInvocationsID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "sessions" || parts[2] != "agent-invocations" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
