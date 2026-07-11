package kernel

import "strings"

type ProviderAdapterBinding struct {
	AdapterID         string
	ProfileID         string
	TransportProtocol string
}

func providerAdapterBindingFromProfile(profile genesisGatewayProfile, transportProtocol string) ProviderAdapterBinding {
	return ProviderAdapterBinding{
		AdapterID:         strings.TrimSpace(profile.ProviderAdapterID),
		ProfileID:         strings.TrimSpace(profile.ProviderAdapterProfileID),
		TransportProtocol: strings.TrimSpace(transportProtocol),
	}
}

func validateProviderAdapterBinding(binding ProviderAdapterBinding) error {
	if strings.TrimSpace(binding.AdapterID) == "" && strings.TrimSpace(binding.ProfileID) == "" {
		return nil
	}
	if strings.TrimSpace(binding.AdapterID) == "" || strings.TrimSpace(binding.ProfileID) == "" {
		return ErrGenesisModelProviderAdapterInvalid
	}
	return nil
}
