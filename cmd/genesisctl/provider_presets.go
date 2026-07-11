package main

import (
	"strings"
	"time"
)

type providerSetupPreset struct {
	ProviderID          string
	ModelID             string
	ProfileID           string
	GatewayRoute        string
	AdapterID           string
	AdapterProfileID    string
	BaseURL             string
	CredentialRef       string
	APIKeyEnv           string
	ContextWindowTokens int
	RequestTimeout      time.Duration
}

func providerSetupPresetByRef(ref string) (providerSetupPreset, bool) {
	ref = strings.TrimSpace(strings.ToLower(ref))
	for _, preset := range providerSetupPresets() {
		if ref == strings.ToLower(preset.ProviderID+"/"+preset.ModelID) {
			return preset, true
		}
	}
	return providerSetupPreset{}, false
}

func providerSetupPresets() []providerSetupPreset {
	return []providerSetupPreset{
		{
			ProviderID:          "deepseek",
			ModelID:             "deepseek-v4-flash",
			ProfileID:           "deepseek-flash",
			GatewayRoute:        "deepseek",
			AdapterID:           "deepseek",
			AdapterProfileID:    "deepseek-v4-flash",
			BaseURL:             "https://api.deepseek.com",
			CredentialRef:       "secret://models/deepseek/local",
			APIKeyEnv:           "DEEPSEEK_API_KEY",
			ContextWindowTokens: 1000000,
			RequestTimeout:      60 * time.Second,
		},
		{
			ProviderID:          "deepseek",
			ModelID:             "deepseek-v4-pro",
			ProfileID:           "deepseek-pro",
			GatewayRoute:        "deepseek",
			AdapterID:           "deepseek",
			AdapterProfileID:    "deepseek-v4-pro",
			BaseURL:             "https://api.deepseek.com",
			CredentialRef:       "secret://models/deepseek/local",
			APIKeyEnv:           "DEEPSEEK_API_KEY",
			ContextWindowTokens: 1000000,
			RequestTimeout:      60 * time.Second,
		},
		{
			ProviderID:       "scnet",
			ModelID:          "DeepSeek-R1-Distill-Qwen-7B",
			ProfileID:        "scnet-deepseek-r1-distill-qwen-7b",
			GatewayRoute:     "scnet",
			AdapterID:        "scnet",
			AdapterProfileID: "DeepSeek-R1-Distill-Qwen-7B",
			BaseURL:          "https://api.scnet.cn/api/llm/v1",
			CredentialRef:    "secret://models/scnet/local",
			APIKeyEnv:        "SCNET_API_KEY",
			RequestTimeout:   60 * time.Second,
		},
	}
}
