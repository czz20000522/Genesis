package resource

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func TestReferenceDescriptorProjectsTextResourceReadText(t *testing.T) {
	descriptor := ReferenceDescriptor{
		Ref:                 "res_alpha",
		RefKind:             ReferenceKindTextResource,
		Owner:               ReferenceOwnerKernelResource,
		DisplayLabel:        "res_alpha",
		AvailableOperations: []string{ReferenceOperationReadText},
		Scope:               "session",
		Provenance:          "kernel_resource_registry",
		PublicMetadata: map[string]string{
			"mime_type": "text/plain",
		},
	}

	if descriptor.Ref != "res_alpha" || descriptor.RefKind != "text_resource" || descriptor.Owner != "kernel.resource" {
		t.Fatalf("descriptor identity = %+v, want text resource owned by kernel.resource", descriptor)
	}
	if len(descriptor.AvailableOperations) != 1 || descriptor.AvailableOperations[0] != "read_text" {
		t.Fatalf("descriptor operations = %v, want read_text", descriptor.AvailableOperations)
	}
}

func TestReferenceDescriptorOmitsSupportedOperations(t *testing.T) {
	descriptor := ReferenceDescriptor{
		Ref:                 "res_alpha",
		RefKind:             ReferenceKindTextResource,
		Owner:               ReferenceOwnerKernelResource,
		AvailableOperations: []string{ReferenceOperationReadText},
	}

	payload, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("marshal descriptor: %v", err)
	}
	if strings.Contains(string(payload), "supported_operations") {
		t.Fatalf("descriptor JSON exposed supported_operations: %s", string(payload))
	}
	if !strings.Contains(string(payload), "available_operations") {
		t.Fatalf("descriptor JSON = %s, want available_operations projection", string(payload))
	}
}

func TestReferenceDescriptorDoesNotExposeInternalRefs(t *testing.T) {
	descriptor := ReferenceDescriptor{
		Ref:                 "res_alpha",
		RefKind:             ReferenceKindTextResource,
		Owner:               ReferenceOwnerKernelResource,
		AvailableOperations: []string{ReferenceOperationReadText},
		PublicMetadata: map[string]string{
			"mime_type": "text/plain",
		},
	}
	payload, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("marshal descriptor: %v", err)
	}
	for _, forbidden := range []string{"storage_ref", "host_path", "raw_payload", "SKILL.md", "C:\\", "/tmp/"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("descriptor leaked internal ref marker %q: %s", forbidden, string(payload))
		}
	}
}
