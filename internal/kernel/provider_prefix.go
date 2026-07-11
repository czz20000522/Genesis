package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

func providerPrefixIdentity(provider Provider) string {
	if provider == nil {
		return ""
	}
	if identified, ok := provider.(ProviderPrefixIdentityProvider); ok {
		if identity := strings.TrimSpace(identified.PrefixIdentity()); identity != "" {
			return identity
		}
	}
	return strings.TrimSpace(provider.Name())
}

func providerPrefixIdentityFromBinding(providerName string, binding ProviderAdapterBinding, model string) string {
	return strings.Join([]string{
		strings.TrimSpace(providerName),
		strings.TrimSpace(binding.AdapterID),
		strings.TrimSpace(binding.ProfileID),
		strings.TrimSpace(binding.TransportProtocol),
		strings.TrimSpace(model),
	}, "\n")
}

func providerPrefixFingerprint(identity string, conversation []ModelConversationMessage, tools []ToolSpec) string {
	system := ""
	for _, message := range conversation {
		if strings.TrimSpace(message.Role) == "system" {
			system = message.Text
			break
		}
	}
	return providerPrefixFingerprintComponents(identity, system, "", tools).Fingerprint
}

func providerPrefixFingerprintComponents(identity string, systemInstruction string, skillIndex string, tools []ToolSpec) PrefixFingerprintComponents {
	normalizedTools := append([]ToolSpec(nil), tools...)
	sort.Slice(normalizedTools, func(i int, j int) bool {
		return normalizedTools[i].Name < normalizedTools[j].Name
	})
	components := PrefixFingerprintComponents{
		SystemInstruction: prefixDigest(systemInstruction),
		SkillIndex:        prefixDigest(skillIndex),
		ToolManifest:      prefixDigest(normalizedTools),
		AdapterBinding:    prefixDigest(strings.TrimSpace(identity)),
	}
	components.Fingerprint = prefixDigest(struct {
		SystemInstruction string `json:"system_instruction"`
		SkillIndex        string `json:"skill_index"`
		ToolManifest      string `json:"tool_manifest"`
		AdapterBinding    string `json:"adapter_binding"`
	}{
		SystemInstruction: components.SystemInstruction,
		SkillIndex:        components.SkillIndex,
		ToolManifest:      components.ToolManifest,
		AdapterBinding:    components.AdapterBinding,
	})
	return components
}

func prefixDigest(value interface{}) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func prefixChangeReasons(previous PrefixFingerprintComponents, current PrefixFingerprintComponents) []string {
	if strings.TrimSpace(current.Fingerprint) == "" {
		return nil
	}
	if strings.TrimSpace(previous.Fingerprint) == "" {
		return []string{"initial"}
	}
	if previous.Fingerprint == current.Fingerprint {
		return nil
	}
	reasons := []string{}
	if previous.SystemInstruction != current.SystemInstruction {
		reasons = append(reasons, "system_instruction")
	}
	if previous.SkillIndex != current.SkillIndex {
		reasons = append(reasons, "skill_index")
	}
	if previous.ToolManifest != current.ToolManifest {
		reasons = append(reasons, "tool_manifest")
	}
	if previous.AdapterBinding != current.AdapterBinding {
		reasons = append(reasons, "adapter_binding")
	}
	if len(reasons) == 0 {
		return []string{"prefix_shape"}
	}
	return reasons
}
