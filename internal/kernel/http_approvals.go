package kernel

import (
	"errors"
	"net/http"
	"strings"
)

func handleListApprovals(w http.ResponseWriter, r *http.Request, k *Kernel) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if !validApprovalStatusFilter(status) {
		writeError(w, http.StatusBadRequest, "invalid_request", "status must be pending, approved, denied, or expired")
		return
	}
	items, err := k.Approvals(status)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ApprovalListResponse{Items: items})
}

func validApprovalStatusFilter(status string) bool {
	switch strings.TrimSpace(status) {
	case "", ApprovalStatusPending, ApprovalStatusApproved, ApprovalStatusDenied, ApprovalStatusExpired:
		return true
	default:
		return false
	}
}

func approvalDecisionID(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "approvals" || parts[2] != "decision" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func handleDecideApproval(w http.ResponseWriter, r *http.Request, k *Kernel) {
	approvalID := approvalDecisionID(r.URL.Path)
	if approvalID == "" {
		writeError(w, http.StatusNotFound, "not_found", "approval decision route not found")
		return
	}
	var req ApprovalDecisionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	if bodyID := strings.TrimSpace(req.ApprovalID); bodyID != "" && bodyID != approvalID {
		writeError(w, http.StatusBadRequest, "invalid_request", "approval_id must match the route")
		return
	}
	req.ApprovalID = approvalID
	approval, err := k.DecideApproval(r.Context(), req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrApprovalRejected) {
		writeError(w, http.StatusConflict, "approval_rejected", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, approval)
}
