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

func kernelPackageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}
