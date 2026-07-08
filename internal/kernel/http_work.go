package kernel

import (
	"errors"
	"net/http"
)

func handleSubmitWork(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req WorkSubmitRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	work, err := k.SubmitWork(req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, work)
}

func handleGetWork(w http.ResponseWriter, r *http.Request, k *Kernel) {
	workID := routePathValue(r, "work_id")
	if workID == "" {
		writeError(w, http.StatusNotFound, "not_found", "work route not found")
		return
	}
	work, err := k.Work(workID)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrWorkNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "work not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, work)
}

func handleCancelWork(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req WorkCancelRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	workID := routePathValue(r, "work_id")
	if workID == "" {
		writeError(w, http.StatusNotFound, "not_found", "work route not found")
		return
	}
	work, err := k.CancelWork(workID, req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrWorkNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "work not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, work)
}
