package kernel

import "net/http"

type providerVerifyRequest struct {
	ModelRole string `json:"model_role"`
	ProfileID string `json:"profile_id"`
}

func handleVerifyProvider(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var request providerVerifyRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	writeJSON(w, http.StatusOK, k.VerifyConfiguredProvider(ProviderVerificationRequest{
		ModelRole: request.ModelRole,
		ProfileID: request.ProfileID,
	}))
}
