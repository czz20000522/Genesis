package kernel

import "net/http"

func handleDiscoveryQuery(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req DiscoveryQueryRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := k.DiscoverContext(req)
	if writeKernelUnavailable(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
