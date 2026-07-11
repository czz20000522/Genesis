package kernel

import "strings"

func (k *Kernel) VerifyConfiguredProvider(req ProviderVerificationRequest) ProviderLiveVerifyResult {
	role := strings.TrimSpace(req.ModelRole)
	if role == "" {
		role = DefaultModelRole
	}
	profileID := strings.TrimSpace(req.ProfileID)
	if k == nil || k.providerVerifier == nil {
		return ProviderLiveVerifyResult{
			Readiness:       ReadinessNotReady,
			ReadinessReason: "provider_verification_unavailable",
			Provider:        ProviderStatus{Name: "provider", Readiness: ReadinessNotReady, ReadinessReason: "provider_verification_unavailable"},
			ModelRole:       role,
			ProfileID:       profileID,
		}
	}
	return k.providerVerifier(ProviderVerificationRequest{ModelRole: role, ProfileID: profileID})
}
