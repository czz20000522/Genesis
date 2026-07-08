package kernel

import (
	"errors"
	"net/http"
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
	invocationID := routePathValue(r, "invocation_id")
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
	sessionID := routePathValue(r, "session_id")
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
