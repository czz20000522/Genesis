package kernel

import "testing"

func TestProviderPrefixFingerprintChangesOnlyWithStablePrefixInputs(t *testing.T) {
	conversation := []ModelConversationMessage{
		{Role: "system", Text: "Genesis stable prefix\n- repo-read"},
		{Role: "user", Text: "current user text must not affect prefix"},
	}
	tools := []ToolSpec{{Name: "shell_exec", Description: "run", InputSchema: map[string]interface{}{"type": "object"}}}

	first := providerPrefixFingerprint("provider\nadapter\nprofile\nprotocol\nmodel", conversation, tools)
	second := providerPrefixFingerprint("provider\nadapter\nprofile\nprotocol\nmodel", []ModelConversationMessage{
		{Role: "system", Text: "Genesis stable prefix\n- repo-read"},
		{Role: "user", Text: "a different current user message"},
	}, tools)
	if first == "" || first != second {
		t.Fatalf("fingerprints = %q %q, want stable prefix fingerprint", first, second)
	}
	changed := providerPrefixFingerprint("provider\nadapter\nprofile\nprotocol\nmodel", conversation, []ToolSpec{{Name: "workspace_edit", Description: "edit", InputSchema: map[string]interface{}{"type": "object"}}})
	if changed == first {
		t.Fatalf("changed tool manifest fingerprint = %q, want different from %q", changed, first)
	}
}

func TestProviderPrefixFingerprintChangesWithAdapterIdentity(t *testing.T) {
	conversation := []ModelConversationMessage{{Role: "system", Text: "Genesis stable prefix"}}
	tools := []ToolSpec{{Name: "shell_exec", InputSchema: map[string]interface{}{"type": "object"}}}
	first := providerPrefixFingerprint("openai-compatible\ndeepseek\ndeepseek-v4-flash\nopenai-chat-completions\ndeepseek-v4-flash", conversation, tools)
	changed := providerPrefixFingerprint("provider_command\nllama.cpp\nqwen\nprovider_command\nqwen.gguf", conversation, tools)
	if first == changed {
		t.Fatal("provider adapter identity change must invalidate the stable prefix fingerprint")
	}
}

func TestCloneModelRequestPreservesPrefixFingerprint(t *testing.T) {
	cloned := cloneModelRequest(ModelRequest{PrefixFingerprint: "prefix-fingerprint"})
	if cloned.PrefixFingerprint != "prefix-fingerprint" {
		t.Fatalf("prefix fingerprint = %q, want preserved value", cloned.PrefixFingerprint)
	}
}

func TestPrefixChangeReasonsNameOnlyChangedStableComponents(t *testing.T) {
	tools := []ToolSpec{{Name: "shell_exec", InputSchema: map[string]interface{}{"type": "object"}}}
	base := providerPrefixFingerprintComponents("provider\nadapter-a\nprofile\nprotocol\nmodel", "instruction", "skill index", tools)
	for _, tc := range []struct {
		name string
		next PrefixFingerprintComponents
		want string
	}{
		{name: "system instruction", next: providerPrefixFingerprintComponents("provider\nadapter-a\nprofile\nprotocol\nmodel", "changed instruction", "skill index", tools), want: "system_instruction"},
		{name: "skill index", next: providerPrefixFingerprintComponents("provider\nadapter-a\nprofile\nprotocol\nmodel", "instruction", "changed skill index", tools), want: "skill_index"},
		{name: "tool manifest", next: providerPrefixFingerprintComponents("provider\nadapter-a\nprofile\nprotocol\nmodel", "instruction", "skill index", []ToolSpec{{Name: "source_read", InputSchema: map[string]interface{}{"type": "object"}}}), want: "tool_manifest"},
		{name: "adapter binding", next: providerPrefixFingerprintComponents("provider\nadapter-b\nprofile\nprotocol\nmodel", "instruction", "skill index", tools), want: "adapter_binding"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reasons := prefixChangeReasons(base, tc.next)
			if len(reasons) != 1 || reasons[0] != tc.want {
				t.Fatalf("reasons = %#v, want [%s]", reasons, tc.want)
			}
		})
	}
	if reasons := prefixChangeReasons(PrefixFingerprintComponents{}, base); len(reasons) != 1 || reasons[0] != "initial" {
		t.Fatalf("initial reasons = %#v, want [initial]", reasons)
	}
}
