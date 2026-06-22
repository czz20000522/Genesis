package kernel

import (
	"go/ast"
	"go/parser"
	"go/token"
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
		if strings.Contains(name, ".") {
			t.Fatalf("tool %q uses a dotted id; canonical tool ids must be provider-safe", name)
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
	shell, ok := lookupKernelTool("shell_exec")
	if !ok {
		t.Fatal("shell_exec is not registered")
	}

	blocked := authorizeKernelTool(ToolPolicy{PermissionMode: PermissionModePlan}, shell)
	if blocked.Allowed || blocked.Reason != "blocked_by_permission_mode=plan" {
		t.Fatalf("plan shell decision = %+v, want blocked by permission mode", blocked)
	}
	allowedRead := authorizeKernelTool(ToolPolicy{PermissionMode: PermissionModePlan}, kernelToolDefinition{
		Descriptor: ModelToolDescriptor{Name: "resource_read"},
		Kind:       ToolKindRead,
	})
	if !allowedRead.Allowed || allowedRead.Reason != "" {
		t.Fatalf("plan read-tool decision = %+v, want read tool allowed", allowedRead)
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

func TestArchitectureBoundaryControlledShellAllowlistStaysSmall(t *testing.T) {
	got := controlledDefaultCommandNames()
	want := []string{"cat", "echo", "get-content", "printf", "pwd", "set-content", "type", "write-output"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("controlled default shell commands = %v, want %v", got, want)
	}
}

func TestArchitectureBoundaryShellGoOnlyOwnsOrchestration(t *testing.T) {
	root := kernelPackageDir(t)
	fileSet := token.NewFileSet()
	tree, err := parser.ParseFile(fileSet, filepath.Join(root, "shell.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse shell.go: %v", err)
	}

	forbiddenImports := map[string]string{
		"os":            "filesystem effects belong in controlled_shell.go",
		"os/exec":       "host process execution belongs in process_runtime.go",
		"path/filepath": "workspace path containment belongs in controlled_shell.go",
		"runtime":       "platform process selection belongs in process_runtime.go",
		"syscall":       "link/process platform probes belong in adapter files",
	}
	for _, imported := range tree.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if reason, forbidden := forbiddenImports[path]; forbidden {
			t.Fatalf("shell.go imports %q; %s", path, reason)
		}
	}

	forbiddenDeclarations := map[string]string{
		"prepareDefaultShellExecution":   "default-mode adapter parsing belongs in controlled_shell.go",
		"controlledDefaultCommand":       "default command semantics belong in controlled_shell.go",
		"executeControlledShellCommand":  "controlled filesystem effects belong in controlled_shell.go",
		"splitCommandFields":             "command tokenization belongs in controlled_shell.go",
		"platformShellCommand":           "host shell process selection belongs in process_runtime.go",
		"runShellProcess":                "host shell process execution belongs in process_runtime.go",
		"regularFileHasMultipleLinks":    "platform link probes belong in platform adapter files",
		"targetHasUnsafeHardlinkAlias":   "workspace containment belongs in controlled_shell.go",
		"pathWithin":                     "workspace containment belongs in controlled_shell.go",
		"canonicalPathForContainment":    "workspace containment belongs in controlled_shell.go",
		"canonicalExistingPath":          "workspace containment belongs in controlled_shell.go",
		"pathHasLinkOrReparsePoint":      "workspace containment belongs in controlled_shell.go",
		"resolveWorkspacePath":           "workspace containment belongs in controlled_shell.go",
		"hasUnsupportedDefaultToken":     "default-mode token policy belongs in controlled_shell.go",
		"parseSetContentFields":          "default command semantics belong in controlled_shell.go",
		"parsePathOnlyFields":            "default command semantics belong in controlled_shell.go",
		"controlledPrintfCommand":        "default command semantics belong in controlled_shell.go",
		"controlledSetContentCommand":    "default command semantics belong in controlled_shell.go",
		"controlledReadCommand":          "default command semantics belong in controlled_shell.go",
		"isControlledDefaultCommandName": "default command allowlist belongs in controlled_shell.go",
		"controlledDefaultCommandNames":  "default command allowlist belongs in controlled_shell.go",
	}
	for _, declaration := range tree.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if reason, forbidden := forbiddenDeclarations[function.Name.Name]; forbidden {
			t.Fatalf("shell.go declares %s; %s", function.Name.Name, reason)
		}
	}
}

func TestArchitectureBoundaryShellRuntimeHasNoApplicationAliases(t *testing.T) {
	for _, command := range controlledDefaultCommandNames() {
		assertNoApplicationSpecificTerm(t, "controlled default command", command)
	}

	root := kernelPackageDir(t)
	for _, file := range []string{"shell.go", "controlled_shell.go", "process_runtime.go"} {
		payload, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		assertNoApplicationSpecificTerm(t, file, string(payload))
	}
}

func TestArchitectureBoundaryKernelIssuesRequireReferenceAlignment(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	activeIssues := readRepoText(t, repoRoot, "docs", "operations", "kernel-issues.md")
	for _, issue := range markdownIssueSections(activeIssues) {
		if !strings.HasPrefix(issue.id, "KERNEL-") {
			continue
		}
		if !strings.Contains(issue.body, "\n- Reference alignment:") {
			t.Fatalf("active kernel issue %s has no Reference alignment field", issue.id)
		}
	}

	retirementLog := readRepoText(t, repoRoot, "docs", "operations", "kernel-retirement-log.md")
	for _, issue := range markdownIssueSections(retirementLog) {
		if !strings.HasPrefix(issue.id, "KERNEL-BOUNDARY-") {
			continue
		}
		if !strings.Contains(issue.body, "\n- Reference alignment:") {
			t.Fatalf("retirement log boundary issue %s has no Reference alignment field", issue.id)
		}
	}
}

type markdownIssueSection struct {
	id   string
	body string
}

func readRepoText(t *testing.T, repoRoot string, pathParts ...string) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(append([]string{repoRoot}, pathParts...)...))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(pathParts...), err)
	}
	return string(payload)
}

func markdownIssueSections(payload string) []markdownIssueSection {
	var sections []markdownIssueSection
	for _, part := range strings.Split(payload, "\n### ")[1:] {
		heading, body, _ := strings.Cut(part, "\n")
		id := strings.Fields(heading)
		if len(id) == 0 {
			continue
		}
		sections = append(sections, markdownIssueSection{id: id[0], body: "\n" + body})
	}
	return sections
}

func assertNoApplicationSpecificTerm(t *testing.T, subject string, content string) {
	t.Helper()
	visible := strings.ToLower(content)
	for _, forbidden := range []string{
		"feishu",
		"lark",
		"wechat",
		"email",
		"calendar",
		"docx",
		"smtp",
		"imap",
	} {
		if strings.Contains(visible, forbidden) {
			t.Fatalf("%s contains application-specific term %q", subject, forbidden)
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
