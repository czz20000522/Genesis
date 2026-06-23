package kernel

import (
	"errors"
	"net/http"
	"strings"
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
	candidateID := memoryCandidateReadID(r.URL.Path)
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

func handleRecallMemories(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryRecallRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	recalls, err := k.RecallMemories(req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrIngressSecurityBlocked) {
		writeError(w, http.StatusForbidden, "memory_recall_blocked_by_ingress_security", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recalls)
}

func handleApproveMemoryCandidate(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MemoryApprovalRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	candidateID := memoryApproveCandidateID(r.URL.Path)
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
	candidateID := memoryRejectCandidateID(r.URL.Path)
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
	candidateID := memorySupersedeCandidateID(r.URL.Path)
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

func isMemoryCandidateGetPath(path string) bool {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return len(parts) == 3 && parts[0] == "memory" && parts[1] == "candidates" && strings.TrimSpace(parts[2]) != ""
}

func memoryCandidateReadID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "memory" || parts[1] != "candidates" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

func isMemoryApprovePath(path string) bool {
	return strings.HasPrefix(path, "/memory/candidates/") && strings.HasSuffix(path, "/approve")
}

func isMemoryRejectPath(path string) bool {
	return strings.HasPrefix(path, "/memory/candidates/") && strings.HasSuffix(path, "/reject")
}

func isMemorySupersedePath(path string) bool {
	return strings.HasPrefix(path, "/memory/candidates/") && strings.HasSuffix(path, "/supersede")
}

func memoryApproveCandidateID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[0] != "memory" || parts[1] != "candidates" || parts[3] != "approve" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

func memoryRejectCandidateID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[0] != "memory" || parts[1] != "candidates" || parts[3] != "reject" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

func memorySupersedeCandidateID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[0] != "memory" || parts[1] != "candidates" || parts[3] != "supersede" {
		return ""
	}
	return strings.TrimSpace(parts[2])
}
