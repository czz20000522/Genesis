package kernel

import (
	"errors"
	"net/http"
)

func handleSubmitTurn(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req TurnRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := k.SubmitTurn(r.Context(), req)
	if err != nil && resp.TurnID != "" && resp.Error != nil {
		writeJSON(w, turnErrorHTTPStatus(*resp.Error), resp)
		return
	}
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrProviderUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "provider_unavailable", err.Error())
		return
	}
	if errors.Is(err, ErrIngressSecurityBlocked) {
		writeError(w, http.StatusForbidden, "turn_blocked_by_ingress_security", err.Error())
		return
	}
	if errors.Is(err, ErrToolInfrastructureFailed) {
		writeError(w, http.StatusServiceUnavailable, "tool_infrastructure_failed", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func turnErrorHTTPStatus(err TurnError) int {
	switch err.Code {
	case "provider_unavailable":
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}
