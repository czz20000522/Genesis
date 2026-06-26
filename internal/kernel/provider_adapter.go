package kernel

import "strings"

const (
	providerHiddenReasoningPolicyDiscard = "discard"
)

type ProviderAdapterBinding struct {
	AdapterID             string
	ProfileID             string
	TransportProtocol     string
	HiddenReasoningPolicy string
}

func providerAdapterBindingFromProfile(profile genesisGatewayProfile, transportProtocol string) ProviderAdapterBinding {
	return ProviderAdapterBinding{
		AdapterID:             strings.TrimSpace(profile.ProviderAdapterID),
		ProfileID:             strings.TrimSpace(profile.ProviderAdapterProfileID),
		TransportProtocol:     strings.TrimSpace(transportProtocol),
		HiddenReasoningPolicy: strings.TrimSpace(profile.HiddenReasoningPolicy),
	}
}

func validateProviderAdapterBinding(binding ProviderAdapterBinding) error {
	switch strings.TrimSpace(binding.HiddenReasoningPolicy) {
	case "":
		return nil
	case providerHiddenReasoningPolicyDiscard:
		if strings.TrimSpace(binding.AdapterID) == "" || strings.TrimSpace(binding.ProfileID) == "" {
			return ErrGenesisModelProviderAdapterInvalid
		}
		return nil
	default:
		return ErrGenesisModelProviderAdapterInvalid
	}
}

func (binding ProviderAdapterBinding) allowsHiddenReasoningDiscard() bool {
	return strings.EqualFold(strings.TrimSpace(binding.HiddenReasoningPolicy), providerHiddenReasoningPolicyDiscard)
}
