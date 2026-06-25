package resource

import "testing"

func TestRegistryReadReturnsBudgetedRawResourceText(t *testing.T) {
	rawText := "resource body sk-owner-secret"
	registry, err := NewRegistry([]Descriptor{{
		Ref:      "res_secret",
		MimeType: "text/plain",
		Text:     rawText,
	}})
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}
	if stored := registry.items["res_secret"].text; stored != rawText {
		t.Fatalf("stored resource text = %q, want raw owner text", stored)
	}

	result, err := registry.Read(ReadRequest{ResourceRef: "res_secret", LimitBytes: 4096})
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if result.Text != rawText {
		t.Fatalf("read text = %q, want raw resource text", result.Text)
	}
}
