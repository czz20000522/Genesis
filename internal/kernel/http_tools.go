package kernel

import (
	"errors"
	"net/http"
)

type shellExecHTTPRequest struct {
	SessionID      string `json:"session_id"`
	CWD            string `json:"cwd"`
	Command        string `json:"command"`
	TimeoutSec     *int   `json:"timeout_sec,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func (req shellExecHTTPRequest) shellExecRequest() (ShellExecRequest, error) {
	timeoutSec := 0
	if req.TimeoutSec != nil {
		timeoutSec = *req.TimeoutSec
		if timeoutSec <= 0 {
			return ShellExecRequest{}, errors.New("timeout_sec must be greater than zero")
		}
	}
	return ShellExecRequest{
		SessionID:      req.SessionID,
		CWD:            req.CWD,
		Command:        req.Command,
		TimeoutSec:     timeoutSec,
		IdempotencyKey: req.IdempotencyKey,
	}, nil
}

func handleExecShell(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var httpReq shellExecHTTPRequest
	if !decodeRequest(w, r, &httpReq) {
		return
	}
	req, err := httpReq.shellExecRequest()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := k.toolGateway().InvokeShell(r.Context(), req, "", "", true)
	if writeKernelUnavailable(w, err) {
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
	if result.Operation != nil {
		writeJSON(w, http.StatusOK, *result.Operation)
		return
	}
	if result.Job != nil {
		status := http.StatusOK
		if result.CreatedJob {
			status = http.StatusAccepted
		}
		writeJSON(w, status, redactJobProjection(*result.Job))
		return
	}
	writeError(w, http.StatusServiceUnavailable, "tool_infrastructure_failed", "shell_exec produced no operation or job")
}
