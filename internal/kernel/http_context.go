package kernel

import "net/http"

func handleAdmitContextResource(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req ContextHydrationAdmissionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	projection, err := k.AdmitContextResource(req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projection)
}
