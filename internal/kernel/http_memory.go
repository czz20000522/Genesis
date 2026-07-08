package kernel

import (
	"errors"
	"net/http"
)

func handleCreateMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryCandidateRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidate, err := k.CreateMemoryCandidate(req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func handleListMemoryCandidates(w http.ResponseWriter, r *http.Request, k *Kernel) {
	status := r.URL.Query().Get("status")
	candidates, err := k.MemoryCandidates(status)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, MemoryCandidateListResponse{Items: candidates})
}

func handleGetMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	candidateID := routePathValue(r, "candidate_id")
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	candidate, err := k.MemoryCandidate(candidateID)
	if writeKernelUnavailable(w, err) {
		return
	}
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

func handleApproveMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryApprovalRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidateID := routePathValue(r, "candidate_id")
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	candidate, err := k.ApproveMemoryCandidate(candidateID, req)
	if writeKernelUnavailable(w, err) {
		return
	}
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

func handleRejectMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryRejectionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidateID := routePathValue(r, "candidate_id")
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	candidate, err := k.RejectMemoryCandidate(candidateID, req)
	if writeKernelUnavailable(w, err) {
		return
	}
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

func handleSupersedeMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemorySupersessionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidateID := routePathValue(r, "candidate_id")
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	supersession, err := k.SupersedeMemoryCandidate(candidateID, req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrMemoryCandidateNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, supersession)
}

func handleForgetMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryForgetRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidateID := routePathValue(r, "candidate_id")
	if candidateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "memory candidate route not found")
		return
	}
	candidate, err := k.ForgetMemoryCandidate(candidateID, req)
	if writeKernelUnavailable(w, err) {
		return
	}
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
