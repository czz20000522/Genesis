package kernel

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestArchitectureBoundarySemanticFieldsDoNotUseSecretRejector(t *testing.T) {
	root := kernelPackageDir(t)
	for _, check := range []struct {
		file      string
		forbidden []string
	}{
		{
			file: "work.go",
			forbidden: []string{
				`validateWorkTextNotSecret("title"`,
				`validateWorkTextNotSecret("cancel_reason"`,
				`validateKernelTextNotSecret("title"`,
				`validateKernelTextNotSecret("cancel_reason"`,
			},
		},
		{
			file: "memory.go",
			forbidden: []string{
				`validateKernelTextNotSecret("approval_reason"`,
				`validateKernelTextNotSecret("rejection_reason"`,
				`validateKernelTextNotSecret("supersession_reason"`,
				`validateKernelTextNotSecret("replacement_text"`,
				`validateKernelTextNotSecret("text"`,
			},
		},
	} {
		payload, err := os.ReadFile(filepath.Join(root, check.file))
		if err != nil {
			t.Fatalf("read %s: %v", check.file, err)
		}
		content := string(payload)
		for _, forbidden := range check.forbidden {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s contains %q; semantic text must not be rejected by secret-shaped heuristics", check.file, forbidden)
			}
		}
	}
}

func TestArchitectureBoundaryModelVisibleToolsStayGeneric(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	descriptors := k.modelToolDescriptors()
	if len(descriptors) == 0 {
		t.Fatal("model tool descriptors are empty")
	}
	for _, descriptor := range descriptors {
		if toolCapabilityKind(descriptor.Name) == "unknown" {
			t.Fatalf("tool %q has no explicit capability kind", descriptor.Name)
		}
		visible := strings.ToLower(descriptor.Name + " " + descriptor.Description)
		for _, forbidden := range []string{
			"feishu",
			"lark",
			"wechat",
			"email",
			"calendar",
			"docx",
		} {
			if strings.Contains(visible, forbidden) {
				t.Fatalf("model tool descriptor %q is application-specific: contains %q", descriptor.Name, forbidden)
			}
		}
	}
}

func TestArchitectureBoundaryToolRegistryBindsSurface(t *testing.T) {
	definitions := kernelToolDefinitions()
	if len(definitions) == 0 {
		t.Fatal("tool registry is empty")
	}
	seen := map[string]bool{}
	for _, definition := range definitions {
		name := strings.TrimSpace(definition.Descriptor.Name)
		if name == "" {
			t.Fatalf("tool registry entry has empty name: %+v", definition)
		}
		if seen[name] {
			t.Fatalf("tool registry has duplicate tool name %q", name)
		}
		seen[name] = true
		switch definition.Kind {
		case ToolKindRead, ToolKindEffect:
		default:
			t.Fatalf("tool %q has invalid kind %q", name, definition.Kind)
		}
		if definition.Prepare == nil {
			t.Fatalf("tool %q has no prepare/handler binding", name)
		}
		if definition.Descriptor.Parameters == nil {
			t.Fatalf("tool %q has no model-visible parameter schema", name)
		}
		if got := toolCapabilityKind(name); got != definition.Kind {
			t.Fatalf("toolCapabilityKind(%q) = %q, want registry kind %q", name, got, definition.Kind)
		}
	}
}

func TestArchitectureBoundaryCapabilitiesProjectFromToolRegistry(t *testing.T) {
	k := newTestKernel(t, filepath.Join(t.TempDir(), "events.jsonl"))
	definitions := kernelToolDefinitions()
	descriptors := k.modelToolDescriptors()
	if len(descriptors) != len(definitions) {
		t.Fatalf("descriptor count = %d, want registry count %d", len(descriptors), len(definitions))
	}
	for i, definition := range definitions {
		if descriptors[i].Name != definition.Descriptor.Name {
			t.Fatalf("descriptor[%d] = %q, want registry tool %q", i, descriptors[i].Name, definition.Descriptor.Name)
		}
	}

	projections := k.toolCapabilityProjections()
	if len(projections) != len(definitions) {
		t.Fatalf("capability count = %d, want registry count %d", len(projections), len(definitions))
	}
	for i, definition := range definitions {
		if projections[i].Name != definition.Descriptor.Name || projections[i].Kind != definition.Kind {
			t.Fatalf("capability[%d] = %+v, want registry tool %q kind %q", i, projections[i], definition.Descriptor.Name, definition.Kind)
		}
	}
}

func TestArchitectureBoundaryAuthorityGateUsesToolKind(t *testing.T) {
	shell, ok := lookupKernelTool("shell.exec")
	if !ok {
		t.Fatal("shell.exec is not registered")
	}
	skillRead, ok := lookupKernelTool("skill.read")
	if !ok {
		t.Fatal("skill.read is not registered")
	}

	blocked := authorizeKernelTool(ToolPolicy{PermissionMode: PermissionModePlan}, shell)
	if blocked.Allowed || blocked.Reason != "blocked_by_permission_mode=plan" {
		t.Fatalf("plan shell decision = %+v, want blocked by permission mode", blocked)
	}
	allowedRead := authorizeKernelTool(ToolPolicy{PermissionMode: PermissionModePlan}, skillRead)
	if !allowedRead.Allowed || allowedRead.Reason != "" {
		t.Fatalf("plan skill.read decision = %+v, want read tool allowed", allowedRead)
	}
	for _, mode := range []string{PermissionModeDefault, PermissionModeYolo} {
		decision := authorizeKernelTool(ToolPolicy{PermissionMode: mode}, shell)
		if !decision.Allowed || decision.Reason != "" {
			t.Fatalf("%s shell decision = %+v, want allowed by generic gate", mode, decision)
		}
	}
	unknownMode := authorizeKernelTool(ToolPolicy{PermissionMode: "surprise"}, shell)
	if unknownMode.Allowed || unknownMode.Reason != "unknown_permission_mode" {
		t.Fatalf("unknown permission mode decision = %+v, want fail-closed unknown_permission_mode", unknownMode)
	}
	unknown := authorizeKernelTool(ToolPolicy{PermissionMode: PermissionModeYolo}, kernelToolDefinition{
		Descriptor: ModelToolDescriptor{Name: "future.tool"},
		Kind:       "unknown",
	})
	if unknown.Allowed || unknown.Reason != "unknown_tool_kind" {
		t.Fatalf("unknown kind decision = %+v, want fail-closed unknown_tool_kind", unknown)
	}
}

func kernelPackageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}
