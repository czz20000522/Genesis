package kernel

import (
	"errors"
	"net/http"
	"strings"
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
	workID := workReadID(r.URL.Path)
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
	workID := workCancelID(r.URL.Path)
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

func isWorkGetPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 2 && parts[0] == "work" && strings.TrimSpace(parts[1]) != ""
}

func workReadID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] != "work" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func isWorkCancelPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "work" && strings.TrimSpace(parts[1]) != "" && parts[2] == "cancel"
}

func workCancelID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "work" || parts[2] != "cancel" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
