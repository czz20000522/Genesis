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

type toolSchemaFieldShape struct {
	typ     string
	minimum interface{}
	maximum interface{}
	itemTyp string
}

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
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	manifest := k.toolGateway().ToolManifest()
	if len(manifest) == 0 {
		t.Fatal("model tool manifest is empty")
	}
	for _, tool := range manifest {
		if toolCapabilitySideEffectLevel(k.toolRegistry, tool.Name) == "unknown" {
			t.Fatalf("tool %q has no explicit side-effect level", tool.Name)
		}
		visible := strings.ToLower(tool.Name + " " + tool.Description)
		for _, forbidden := range []string{
			"feishu",
			"lark",
			"wechat",
			"email",
			"calendar",
			"docx",
		} {
			if strings.Contains(visible, forbidden) {
				t.Fatalf("model tool descriptor %q is application-specific: contains %q", tool.Name, forbidden)
			}
		}
	}
}

func TestArchitectureBoundaryNoSkillSpecificHydrationTools(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	manifest := k.toolGateway().ToolManifest()
	if len(manifest) == 0 {
		t.Fatal("model tool manifest is empty")
	}
	for _, tool := range manifest {
		name := strings.ToLower(strings.TrimSpace(tool.Name))
		description := strings.ToLower(strings.TrimSpace(tool.Description))
		for _, forbidden := range []string{"skill.read", "read_skill", "skill_read"} {
			if strings.Contains(name, forbidden) || strings.Contains(description, forbidden) {
				t.Fatalf("model tool %q exposes forbidden skill-specific hydration surface %q", tool.Name, forbidden)
			}
		}
		if strings.Contains(name, "skill") || strings.Contains(description, "skill body") || strings.Contains(description, "skill package") {
			t.Fatalf("model tool %q is skill-specific; hydration must go through generic resource/context admission", tool.Name)
		}
	}
}

func TestToolManifestDoesNotExposeUniversalRefRead(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	manifest := k.toolGateway().ToolManifest()
	if len(manifest) == 0 {
		t.Fatal("model tool manifest is empty")
	}
	for _, tool := range manifest {
		visible := strings.ToLower(strings.TrimSpace(tool.Name) + " " + strings.TrimSpace(tool.Description))
		for _, forbidden := range []string{"ref_read", "ref.list", "ref_list", "ref_search", "ref_span", "universal ref"} {
			if strings.Contains(visible, forbidden) {
				t.Fatalf("model tool %q exposes universal reference surface %q", tool.Name, forbidden)
			}
		}
	}
}

func TestResourceReadSchemaRemainsTyped(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	var resourceRead ToolSpec
	for _, tool := range k.toolGateway().ToolManifest() {
		if tool.Name == "resource_read" {
			resourceRead = tool
			break
		}
	}
	if resourceRead.Name == "" {
		t.Fatal("resource_read tool is not registered")
	}
	properties, ok := resourceRead.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("resource_read input schema = %+v, want object properties", resourceRead.InputSchema)
	}
	for _, required := range []string{"resource_ref", "offset_bytes", "limit_bytes"} {
		if _, ok := properties[required]; !ok {
			t.Fatalf("resource_read properties = %+v, missing %q", properties, required)
		}
	}
	for _, forbidden := range []string{"ref", "job_id", "event_id", "tool_call_event_id", "operation_id", "checkpoint_ref", "storage_ref", "host_path", "raw_payload_ref"} {
		if _, ok := properties[forbidden]; ok {
			t.Fatalf("resource_read schema exposed forbidden argument %q: %+v", forbidden, properties)
		}
	}
	requiredValues, ok := resourceRead.InputSchema["required"].([]string)
	if !ok || len(requiredValues) != 1 || requiredValues[0] != "resource_ref" {
		t.Fatalf("resource_read required = %+v, want only resource_ref", resourceRead.InputSchema["required"])
	}
	if additional, ok := resourceRead.InputSchema["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("resource_read additionalProperties = %+v, want false", resourceRead.InputSchema["additionalProperties"])
	}
}

func TestArchitectureBoundaryModelVisibleToolSchemaShapeIsStable(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	want := map[string]struct {
		required []string
		fields   map[string]toolSchemaFieldShape
	}{
		"shell_exec": {
			required: []string{"command"},
			fields: map[string]toolSchemaFieldShape{
				"command":     {typ: "string"},
				"cwd":         {typ: "string"},
				"timeout_sec": {typ: "integer", minimum: 1},
			},
		},
		"resource_read": {
			required: []string{"resource_ref"},
			fields: map[string]toolSchemaFieldShape{
				"resource_ref": {typ: "string"},
				"offset_bytes": {typ: "integer", minimum: 0},
				"limit_bytes":  {typ: "integer", minimum: 1},
			},
		},
		"context_discover": {
			required: []string{"intent"},
			fields: map[string]toolSchemaFieldShape{
				"intent":                  {typ: "string"},
				"current_context_summary": {typ: "string"},
				"requested_kinds":         {typ: "array", itemTyp: "string"},
				"limit":                   {typ: "integer", minimum: 1, maximum: maxDiscoveryLimit},
			},
		},
		"source_tree": {
			required: []string{"source_snapshot_ref"},
			fields: map[string]toolSchemaFieldShape{
				"source_snapshot_ref": {typ: "string"},
				"max_entries":         {typ: "integer", minimum: 1},
			},
		},
		"source_read": {
			required: []string{"source_file_ref"},
			fields: map[string]toolSchemaFieldShape{
				"source_file_ref": {typ: "string"},
				"offset_bytes":    {typ: "integer", minimum: 0},
				"limit_bytes":     {typ: "integer", minimum: 1},
			},
		},
		"workspace_edit": {
			required: []string{"path"},
			fields: map[string]toolSchemaFieldShape{
				"path":       {typ: "string"},
				"old_string": {typ: "string"},
				"new_string": {typ: "string"},
				"edits":      {typ: "array", itemTyp: "object"},
			},
		},
		"job_status": {
			required: []string{"job_id"},
			fields: map[string]toolSchemaFieldShape{
				"job_id": {typ: "string"},
			},
		},
		"job_wait": {
			required: []string{"job_id"},
			fields: map[string]toolSchemaFieldShape{
				"job_id":      {typ: "string"},
				"timeout_sec": {typ: "integer", minimum: 1, maximum: maxJobWaitTimeoutSec},
			},
		},
		"job_cancel": {
			required: []string{"job_id"},
			fields: map[string]toolSchemaFieldShape{
				"job_id": {typ: "string"},
				"reason": {typ: "string"},
			},
		},
	}

	for _, tool := range k.toolGateway().ToolManifest() {
		expected, ok := want[tool.Name]
		if !ok {
			t.Fatalf("unexpected model-visible tool %q", tool.Name)
		}
		assertToolInputSchemaShape(t, tool, expected.required, expected.fields)
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing model-visible tools: %+v", want)
	}
}

func TestArchitectureBoundaryToolRegistryBindsSurface(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	manifest := k.toolGateway().ToolManifest()
	if len(manifest) == 0 {
		t.Fatal("tool registry is empty")
	}
	seen := map[string]bool{}
	for _, spec := range manifest {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			t.Fatalf("tool registry entry has empty name: %+v", spec)
		}
		if strings.Contains(name, ".") {
			t.Fatalf("tool %q uses a dotted id; canonical tool ids must be provider-safe", name)
		}
		if seen[name] {
			t.Fatalf("tool registry has duplicate tool name %q", name)
		}
		seen[name] = true
		switch spec.SideEffectLevel {
		case ToolSideEffectRead, ToolSideEffectWrite:
		default:
			t.Fatalf("tool %q has invalid side_effect_level %q", name, spec.SideEffectLevel)
		}
		if strings.TrimSpace(spec.ExecutionKind) == "" {
			t.Fatalf("tool %q has no execution_kind", name)
		}
		if spec.InputSchema == nil {
			t.Fatalf("tool %q has no model-visible parameter schema", name)
		}
		definition, ok := k.toolRegistry.Resolve(name)
		if !ok {
			t.Fatalf("registry could not resolve manifest tool %q", name)
		}
		if definition.Prepare == nil {
			t.Fatalf("tool %q has no prepare/handler binding", name)
		}
		if got := toolCapabilitySideEffectLevel(k.toolRegistry, name); got != spec.SideEffectLevel {
			t.Fatalf("toolCapabilitySideEffectLevel(%q) = %q, want registry level %q", name, got, spec.SideEffectLevel)
		}
	}
}

func assertToolInputSchemaShape(t *testing.T, tool ToolSpec, required []string, fields map[string]toolSchemaFieldShape) {
	t.Helper()
	if tool.InputSchema["type"] != "object" {
		t.Fatalf("%s schema type = %+v, want object", tool.Name, tool.InputSchema["type"])
	}
	if additional, ok := tool.InputSchema["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("%s additionalProperties = %+v, want false", tool.Name, tool.InputSchema["additionalProperties"])
	}
	gotRequired, ok := tool.InputSchema["required"].([]string)
	if !ok || strings.Join(gotRequired, ",") != strings.Join(required, ",") {
		t.Fatalf("%s required = %+v, want %+v", tool.Name, tool.InputSchema["required"], required)
	}
	properties, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s properties = %+v, want object", tool.Name, tool.InputSchema["properties"])
	}
	if len(properties) != len(fields) {
		t.Fatalf("%s property count = %d, want %d: %+v", tool.Name, len(properties), len(fields), properties)
	}
	for name, expected := range fields {
		property, ok := properties[name].(map[string]interface{})
		if !ok {
			t.Fatalf("%s properties missing %q: %+v", tool.Name, name, properties)
		}
		if property["type"] != expected.typ {
			t.Fatalf("%s.%s type = %+v, want %q", tool.Name, name, property["type"], expected.typ)
		}
		if expected.minimum != nil && property["minimum"] != expected.minimum {
			t.Fatalf("%s.%s minimum = %+v, want %+v", tool.Name, name, property["minimum"], expected.minimum)
		}
		if expected.maximum != nil && property["maximum"] != expected.maximum {
			t.Fatalf("%s.%s maximum = %+v, want %+v", tool.Name, name, property["maximum"], expected.maximum)
		}
		if expected.itemTyp != "" {
			items, ok := property["items"].(map[string]interface{})
			if !ok || items["type"] != expected.itemTyp {
				t.Fatalf("%s.%s items = %+v, want type %q", tool.Name, name, property["items"], expected.itemTyp)
			}
		}
	}
}

func TestArchitectureBoundaryCapabilitiesProjectFromToolRegistry(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	manifest := k.toolGateway().ToolManifest()
	if len(manifest) == 0 {
		t.Fatal("tool manifest is empty")
	}

	projections := k.toolCapabilityProjections()
	if len(projections) != len(manifest) {
		t.Fatalf("capability count = %d, want registry count %d", len(projections), len(manifest))
	}
	for i, spec := range manifest {
		if projections[i].Name != spec.Name || projections[i].SideEffectLevel != spec.SideEffectLevel || projections[i].ExecutionKind != spec.ExecutionKind {
			t.Fatalf("capability[%d] = %+v, want registry tool %q level %q execution %q", i, projections[i], spec.Name, spec.SideEffectLevel, spec.ExecutionKind)
		}
	}
}

func TestArchitectureBoundaryAuthorityGateUsesToolSideEffectLevel(t *testing.T) {
	k := newTestKernel(t, filepath.Join(testTempDir(t), "events.sqlite"))
	shell, ok := k.toolRegistry.Resolve("shell_exec")
	if !ok {
		t.Fatal("shell_exec is not registered")
	}

	blocked := authorizeKernelTool(ToolPolicy{PermissionMode: PermissionModePlan}, shell.Spec)
	if blocked.Allowed || blocked.Reason != "blocked_by_permission_mode=plan" {
		t.Fatalf("plan shell decision = %+v, want blocked by permission mode", blocked)
	}
	allowedRead := authorizeKernelTool(ToolPolicy{PermissionMode: PermissionModePlan}, ToolSpec{
		Name:            "resource_read",
		SideEffectLevel: ToolSideEffectRead,
	})
	if !allowedRead.Allowed || allowedRead.Reason != "" {
		t.Fatalf("plan read-tool decision = %+v, want read tool allowed", allowedRead)
	}
	for _, mode := range []string{PermissionModeDefault, PermissionModeYolo} {
		decision := authorizeKernelTool(ToolPolicy{PermissionMode: mode}, shell.Spec)
		if !decision.Allowed || decision.Reason != "" {
			t.Fatalf("%s shell decision = %+v, want allowed by generic gate", mode, decision)
		}
	}
	unknownMode := authorizeKernelTool(ToolPolicy{PermissionMode: "surprise"}, shell.Spec)
	if unknownMode.Allowed || unknownMode.Reason != "unknown_permission_mode" {
		t.Fatalf("unknown permission mode decision = %+v, want fail-closed unknown_permission_mode", unknownMode)
	}
	unknown := authorizeKernelTool(ToolPolicy{PermissionMode: PermissionModeYolo}, ToolSpec{
		Name:            "future_tool",
		SideEffectLevel: "unknown",
	})
	if unknown.Allowed || unknown.Reason != "unknown_tool_kind" {
		t.Fatalf("unknown kind decision = %+v, want fail-closed unknown_tool_kind", unknown)
	}
}

func TestArchitectureBoundaryPermissionModesResolveToPolicyProfiles(t *testing.T) {
	for _, tt := range []struct {
		mode            string
		authorityPolicy string
		sandboxProfile  string
		approvalPolicy  string
	}{
		{
			mode:            PermissionModePlan,
			authorityPolicy: AuthorityPolicyReadOnly,
			sandboxProfile:  SandboxProfileReadOnly,
			approvalPolicy:  ApprovalPolicyNever,
		},
		{
			mode:            PermissionModeDefault,
			authorityPolicy: AuthorityPolicyWorkspaceWrite,
			sandboxProfile:  SandboxProfileControlledWorkspace,
			approvalPolicy:  ApprovalPolicyNever,
		},
		{
			mode:            PermissionModeYolo,
			authorityPolicy: AuthorityPolicyFullAccess,
			sandboxProfile:  SandboxProfileHost,
			approvalPolicy:  ApprovalPolicyNever,
		},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			resolved := resolveToolPolicy(ToolPolicy{PermissionMode: tt.mode, WorkspaceRoot: "workspace"})
			if !resolved.Known {
				t.Fatalf("resolved policy = %+v, want known policy", resolved)
			}
			if resolved.AuthorityPolicy != tt.authorityPolicy || resolved.SandboxProfile != tt.sandboxProfile || resolved.ApprovalPolicy != tt.approvalPolicy {
				t.Fatalf("resolved policy = %+v, want authority=%q sandbox=%q approval=%q", resolved, tt.authorityPolicy, tt.sandboxProfile, tt.approvalPolicy)
			}
		})
	}

	unknown := resolveToolPolicy(ToolPolicy{PermissionMode: "surprise"})
	if unknown.Known || unknown.AuthorityPolicy != AuthorityPolicyUnknown || unknown.SandboxProfile != SandboxProfileNone || unknown.ApprovalPolicy != ApprovalPolicyNever {
		t.Fatalf("unknown resolved policy = %+v, want fail-closed unknown profile", unknown)
	}
}

func TestArchitectureBoundarySandboxProfileCannotBroadenPermissionMode(t *testing.T) {
	for _, tt := range []struct {
		name           string
		permissionMode string
		sandboxProfile string
	}{
		{
			name:           "default_to_host",
			permissionMode: PermissionModeDefault,
			sandboxProfile: SandboxProfileHost,
		},
		{
			name:           "default_to_read_only",
			permissionMode: PermissionModeDefault,
			sandboxProfile: SandboxProfileReadOnly,
		},
		{
			name:           "yolo_to_read_only",
			permissionMode: PermissionModeYolo,
			sandboxProfile: SandboxProfileReadOnly,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			broadened := authorizeKernelTool(ToolPolicy{
				PermissionMode: tt.permissionMode,
				SandboxProfile: tt.sandboxProfile,
				ApprovalPolicy: ApprovalPolicyNever,
				WorkspaceRoot:  "workspace",
			}, ToolSpec{
				Name:            "shell_exec",
				SideEffectLevel: ToolSideEffectWrite,
			})
			if broadened.Allowed || broadened.Reason != "sandbox_profile_not_allowed_for_permission_mode" {
				t.Fatalf("sandbox decision = %+v, want fail-closed profile mismatch", broadened)
			}
		})
	}

	unavailable := authorizeKernelTool(ToolPolicy{
		PermissionMode: PermissionModeYolo,
		SandboxProfile: SandboxProfileOSWorkspace,
		ApprovalPolicy: ApprovalPolicyNever,
		WorkspaceRoot:  "workspace",
	}, ToolSpec{
		Name:            "shell_exec",
		SideEffectLevel: ToolSideEffectWrite,
	})
	if unavailable.Allowed || unavailable.Reason != "sandbox_profile_unavailable=os_workspace" {
		t.Fatalf("unavailable os workspace decision = %+v, want fail-closed unavailable sandbox", unavailable)
	}
}

func TestArchitectureBoundaryApprovalOnRequestBlocksWriteToolsAtAdmission(t *testing.T) {
	read := authorizeKernelTool(ToolPolicy{
		PermissionMode: PermissionModeYolo,
		ApprovalPolicy: ApprovalPolicyOnRequest,
	}, ToolSpec{
		Name:            "job_status",
		SideEffectLevel: ToolSideEffectRead,
	})
	if !read.Allowed || read.Reason != "" {
		t.Fatalf("read tool with approval policy = %+v, want allowed", read)
	}

	write := authorizeKernelTool(ToolPolicy{
		PermissionMode: PermissionModeYolo,
		ApprovalPolicy: ApprovalPolicyOnRequest,
	}, ToolSpec{
		Name:            "shell_exec",
		SideEffectLevel: ToolSideEffectWrite,
	})
	if write.Allowed || write.Reason != "approval_required" {
		t.Fatalf("write tool with approval policy = %+v, want approval_required", write)
	}

	readOnlyWrite := authorizeKernelTool(ToolPolicy{
		PermissionMode: PermissionModePlan,
		ApprovalPolicy: ApprovalPolicyOnRequest,
	}, ToolSpec{
		Name:            "shell_exec",
		SideEffectLevel: ToolSideEffectWrite,
	})
	if readOnlyWrite.Allowed || readOnlyWrite.Reason != "blocked_by_permission_mode=plan" {
		t.Fatalf("plan write with approval policy = %+v, want hard read-only denial before approval", readOnlyWrite)
	}
}

func TestArchitectureBoundaryShellExecutionUsesResolvedSandboxProfile(t *testing.T) {
	workspace := testTempDir(t)
	req := ShellExecRequest{
		SessionID: "sandbox-profile-selection",
		CWD:       workspace,
		Command:   writeFileCommand("profile.txt", "ok"),
	}

	controlled, reason := prepareShellExecution(ResolvedToolPolicy{
		PermissionMode:  PermissionModeYolo,
		AuthorityPolicy: AuthorityPolicyWorkspaceWrite,
		SandboxProfile:  SandboxProfileControlledWorkspace,
		ApprovalPolicy:  ApprovalPolicyNever,
		WorkspaceRoot:   workspace,
		Known:           true,
	}, req)
	if reason != "" || controlled.controlled == nil {
		t.Fatalf("controlled profile execution = %+v reason=%q, want controlled workspace plan", controlled, reason)
	}

	host, reason := prepareShellExecution(ResolvedToolPolicy{
		PermissionMode:  PermissionModeDefault,
		AuthorityPolicy: AuthorityPolicyFullAccess,
		SandboxProfile:  SandboxProfileHost,
		ApprovalPolicy:  ApprovalPolicyNever,
		WorkspaceRoot:   workspace,
		Known:           true,
	}, req)
	if reason != "" || host.controlled != nil {
		t.Fatalf("host profile execution = %+v reason=%q, want host shell plan", host, reason)
	}
}

func TestArchitectureBoundaryToolRegistryRejectsIncompleteSpecs(t *testing.T) {
	validPrepare := defaultKernelTools()[0].Prepare
	_, err := NewToolRegistry([]registeredTool{{
		Spec: ToolSpec{
			Name:            "bad.tool",
			Description:     "bad dotted name",
			InputSchema:     map[string]interface{}{"type": "object"},
			SideEffectLevel: ToolSideEffectRead,
			ExecutionKind:   ToolExecutionKindSandboxedProcess,
		},
		Prepare: validPrepare,
	}})
	if err == nil {
		t.Fatal("NewToolRegistry accepted a dotted tool id")
	}

	_, err = NewToolRegistry([]registeredTool{{
		Spec: ToolSpec{
			Name:            "missing_execution_kind",
			Description:     "missing execution kind",
			InputSchema:     map[string]interface{}{"type": "object"},
			SideEffectLevel: ToolSideEffectRead,
		},
		Prepare: validPrepare,
	}})
	if err == nil {
		t.Fatal("NewToolRegistry accepted a tool without execution_kind")
	}
}

func TestArchitectureBoundaryMaterialToolsStayMinimal(t *testing.T) {
	registry, err := defaultToolRegistry(ShellTimeoutPolicy{})
	if err != nil {
		t.Fatalf("defaultToolRegistry returned error: %v", err)
	}
	forbidden := map[string]string{
		"source_search":    "large code exploration should use shell/rg or a user-space code intelligence adapter first",
		"source_span":      "citation/span projection needs a separate owner decision",
		"artifact_list":    "artifact browsing is not part of the default kernel tool surface",
		"artifact_preview": "artifact preview is not part of the default kernel tool surface",
	}
	for _, tool := range registry.Manifest() {
		if reason, ok := forbidden[tool.Name]; ok {
			t.Fatalf("default tool registry exposes %q; %s", tool.Name, reason)
		}
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
		if !hasReferenceAlignmentOrRejectedDrift(issue.body) {
			t.Fatalf("active kernel issue %s has no Reference alignment or Rejected drift risk field", issue.id)
		}
	}

	retirementLog := readRepoText(t, repoRoot, "docs", "operations", "kernel-retirement-log.md")
	for _, issue := range markdownIssueSections(retirementLog) {
		if !strings.HasPrefix(issue.id, "KERNEL-") || !retiredIssueRequiresReferenceAlignment(issue) {
			continue
		}
		if !hasReferenceAlignmentOrRejectedDrift(issue.body) {
			t.Fatalf("retirement log architecture issue %s has no Reference alignment or Rejected drift risk field", issue.id)
		}
	}
}

func TestArchitectureBoundaryKernelImplementationPlansNameReferenceBehaviorRedTests(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	files, err := filepath.Glob(filepath.Join(repoRoot, "docs", "implementation-plans", "kernel-*.md"))
	if err != nil {
		t.Fatalf("glob kernel implementation plans: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no kernel implementation plans found")
	}
	for _, file := range files {
		contentBytes, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		content := string(contentBytes)
		if !strings.Contains(content, "## Reference Scan") {
			continue
		}
		if !strings.Contains(content, "## Reference Behavior Red Tests") {
			t.Fatalf("%s has a Reference Scan but no Reference Behavior Red Tests section", filepath.ToSlash(file))
		}
	}
}

func TestArchitectureBoundaryToolRegistryDoesNotBindWholeKernel(t *testing.T) {
	root := kernelPackageDir(t)
	content := readRepoText(t, root, "tool_registry.go")
	for _, forbidden := range []string{
		"Prepare func(*Kernel",
		"Prepare: (*Kernel)",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("tool_registry.go contains broad kernel tool binding %q; use toolInvocationContext instead", forbidden)
		}
	}
	if !strings.Contains(content, "type toolInvocationContext interface") {
		t.Fatal("tool_registry.go does not declare toolInvocationContext")
	}
}

func TestArchitectureBoundaryCoreLoopHasNoProviderNativeWireTerms(t *testing.T) {
	root := kernelPackageDir(t)
	for _, file := range []string{
		"kernel.go",
		"model_tools.go",
		"provider.go",
		"provider_command.go",
		"tool_registry.go",
		"config_types.go",
		"context_compaction_types.go",
		"event_types.go",
		"inspection_types.go",
		"memory_types.go",
		"provider_accounting_types.go",
		"provider_resilience_types.go",
		"skill_catalog_types.go",
		"tool_types.go",
		"turn_types.go",
		"work_types.go",
	} {
		content := readRepoText(t, root, file)
		for _, forbidden := range []string{
			"/chat/completions",
			"chatCompletion",
			"chatTool",
			"tool_choice",
			"prompt_tokens",
			"completion_tokens",
			"reasoning_content",
			"Bearer ",
			"Authorization",
		} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s contains provider-native wire term %q; keep vendor protocol handling inside provider adapters", file, forbidden)
			}
		}
	}
}

func TestArchitectureBoundaryProviderWireTermsStayInsideAdapterFiles(t *testing.T) {
	root := kernelPackageDir(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read kernel package dir: %v", err)
	}
	allowedProviderAdapterFiles := map[string]bool{
		"openai_compatible.go": true,
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if allowedProviderAdapterFiles[name] {
			continue
		}
		content := readRepoText(t, root, name)
		for _, forbidden := range []string{
			"/chat/completions",
			"chatCompletion",
			"chatTool",
			"tool_choice",
			"prompt_tokens",
			"completion_tokens",
			"reasoning_content",
			"DeepSeek",
			"SCNet",
			"scnet",
			"OpenRouter",
			"openai-responses",
		} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s contains provider wire term %q; provider compatibility belongs in adapter/translator files", name, forbidden)
			}
		}
	}
}

func TestArchitectureBoundaryKernelSessionDelegatesOwnerReplay(t *testing.T) {
	root := kernelPackageDir(t)
	body := functionBodySource(t, filepath.Join(root, "kernel.go"), "Session")
	for _, forbidden := range []string{
		`"turn.submitted"`,
		`"model.final"`,
		`"turn.failed"`,
		`"operation.`,
		`"job.`,
		`"work.`,
		`"memory.`,
		"mergeWorkProjection",
		"mergeMemoryCandidateProjection",
		"OperationProjection{",
		"JobProjection{",
		"WorkProjection{",
		"MemoryCandidateProjection{",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("Kernel.Session directly contains owner replay marker %q; delegate replay to owner projection helpers", forbidden)
		}
	}
}

func TestArchitectureBoundaryOwnerDTOsLiveInNamedFiles(t *testing.T) {
	root := kernelPackageDir(t)
	want := map[string]string{
		"Config":                              "config_types.go",
		"ToolPolicy":                          "config_types.go",
		"ContextPolicy":                       "config_types.go",
		"BudgetPolicy":                        "config_types.go",
		"BudgetLeaseProjection":               "config_types.go",
		"ReadyResponse":                       "config_types.go",
		"CapabilitiesResponse":                "config_types.go",
		"ProviderStatus":                      "config_types.go",
		"ReadyCheck":                          "config_types.go",
		"ResourceDescriptor":                  "resource_types.go",
		"ResourceMetadata":                    "resource_types.go",
		"ModelResourceReadResult":             "resource_types.go",
		"ContextHydrationAdmissionRequest":    "resource_types.go",
		"ContextHydrationProjection":          "resource_types.go",
		"TurnRequest":                         "turn_types.go",
		"InputItem":                           "turn_types.go",
		"TurnResponse":                        "turn_types.go",
		"TurnEventsResponse":                  "turn_types.go",
		"FinalMessage":                        "turn_types.go",
		"TurnProjection":                      "turn_types.go",
		"TurnError":                           "turn_types.go",
		"ToolSpec":                            "tool_types.go",
		"ModelToolCall":                       "tool_types.go",
		"ModelToolRound":                      "tool_types.go",
		"ModelToolResult":                     "tool_types.go",
		"ToolRequestInvalidProjection":        "tool_types.go",
		"ToolRequestError":                    "tool_types.go",
		"ToolCapabilityProjection":            "tool_types.go",
		"ShellExecRequest":                    "tool_types.go",
		"OperationProjection":                 "tool_types.go",
		"JobProjection":                       "tool_types.go",
		"KernelObservationDeliveryProjection": "tool_types.go",
		"ModelOperationResult":                "tool_types.go",
		"ModelManagedJobResult":               "tool_types.go",
		"ModelJobControlResult":               "tool_types.go",
		"ToolCallProjection":                  "tool_types.go",
		"ToolResultProjection":                "tool_types.go",
		"ApprovalListResponse":                "approval_types.go",
		"ApprovalDecisionRequest":             "approval_types.go",
		"ApprovalProjection":                  "approval_types.go",
		"ApprovalPolicySnapshot":              "approval_types.go",
		"ApprovalEffectSummary":               "approval_types.go",
		"SandboxReadinessProjection":          "approval_types.go",
		"WorkSubmitRequest":                   "work_types.go",
		"WorkCancelRequest":                   "work_types.go",
		"WorkProjection":                      "work_types.go",
		"MemoryCandidateRequest":              "memory_types.go",
		"MemoryCandidateListResponse":         "memory_types.go",
		"MemoryApprovalRequest":               "memory_types.go",
		"MemoryRejectionRequest":              "memory_types.go",
		"MemorySupersessionRequest":           "memory_types.go",
		"MemoryForgetRequest":                 "memory_types.go",
		"MemorySupersessionProjection":        "memory_types.go",
		"MemoryCandidateProjection":           "memory_types.go",
		"DiscoveryQueryRequest":               "discovery_types.go",
		"DiscoveryQueryResponse":              "discovery_types.go",
		"DiscoveryCandidateProjection":        "discovery_types.go",
		"CapabilityDescriptor":                "discovery_types.go",
		"Event":                               "event_types.go",
		"StoredEvent":                         "event_types.go",
		"EventData":                           "event_types.go",
		"EventProjection":                     "event_types.go",
		"AuditReplayResponse":                 "inspection_types.go",
		"AuditReplayItem":                     "inspection_types.go",
		"UITimelineResponse":                  "inspection_types.go",
		"UITimelineDetailResponse":            "inspection_types.go",
		"UITimelineItem":                      "inspection_types.go",
		"ContextInspectionResponse":           "inspection_types.go",
		"ToolManifestInspection":              "inspection_types.go",
		"ContextRuntimeSnapshot":              "inspection_types.go",
		"PermissionInspection":                "inspection_types.go",
		"SessionProjection":                   "inspection_types.go",
		"TokenUsage":                          "provider_accounting_types.go",
		"ModelContextAccountingProjection":    "provider_accounting_types.go",
		"ProviderAttemptProjection":           "provider_resilience_types.go",
		"ContextCompactionProjection":         "context_compaction_types.go",
		"ContextCacheStabilityProjection":     "context_compaction_types.go",
		"SkillDescriptor":                     "skill_catalog_types.go",
		"SkillCatalogProjection":              "skill_catalog_types.go",
		"SkillCatalogItemProjection":          "skill_catalog_types.go",
		"SkillCatalogExclusionProjection":     "skill_catalog_types.go",
	}
	got := kernelTypeDeclarationFiles(t, root)
	for typeName, wantFile := range want {
		if gotFile := got[typeName]; gotFile != wantFile {
			t.Fatalf("%s declared in %q, want %q", typeName, gotFile, wantFile)
		}
	}
}

func TestArchitectureBoundaryResourceOwnerHasSubpackageTypes(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	resourceDir := filepath.Join(repoRoot, "internal", "kernel", "resource")
	got := kernelTypeDeclarationFiles(t, resourceDir)
	for typeName, wantFile := range map[string]string{
		"Descriptor":         "types.go",
		"Metadata":           "types.go",
		"ModelReadResult":    "types.go",
		"Registry":           "registry.go",
		"ReadRequest":        "registry.go",
		"registeredResource": "registry.go",
	} {
		if gotFile := got[typeName]; gotFile != wantFile {
			t.Fatalf("resource owner type %s declared in %q, want %q", typeName, gotFile, wantFile)
		}
	}
}

func TestArchitectureBoundaryModelGatewayOwnerHasSubpackageResilienceSurface(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	modelGatewayDir := filepath.Join(repoRoot, "internal", "kernel", "modelgateway")
	gotTypes := kernelTypeDeclarationFiles(t, modelGatewayDir)
	for typeName, wantFile := range map[string]string{
		"AttemptProjection": "resilience.go",
		"ClassifiedError":   "resilience.go",
		"FailureClassifier": "resilience.go",
	} {
		if gotFile := gotTypes[typeName]; gotFile != wantFile {
			t.Fatalf("modelgateway owner type %s declared in %q, want %q", typeName, gotFile, wantFile)
		}
	}
	gotFunctions := kernelFunctionDeclarationFiles(t, modelGatewayDir)
	for functionName, wantFile := range map[string]string{
		"NewStatusError":               "resilience.go",
		"NewTransportError":            "resilience.go",
		"NewVisibleFinalRequiredError": "resilience.go",
		"RetryDelay":                   "resilience.go",
		"NeedsVisibleFinalRepair":      "resilience.go",
	} {
		if gotFile := gotFunctions[functionName]; gotFile != wantFile {
			t.Fatalf("modelgateway owner function %s declared in %q, want %q", functionName, gotFile, wantFile)
		}
	}
}

func TestArchitectureBoundaryModelGatewayOwnerHasSubpackageAccountingSurface(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	modelGatewayDir := filepath.Join(repoRoot, "internal", "kernel", "modelgateway")
	gotTypes := kernelTypeDeclarationFiles(t, modelGatewayDir)
	for typeName, wantFile := range map[string]string{
		"ContextAccountingProjection": "accounting.go",
		"TokenUsage":                  "accounting.go",
	} {
		if gotFile := gotTypes[typeName]; gotFile != wantFile {
			t.Fatalf("modelgateway owner accounting type %s declared in %q, want %q", typeName, gotFile, wantFile)
		}
	}
	gotFunctions := kernelFunctionDeclarationFiles(t, modelGatewayDir)
	if gotFile := gotFunctions["CloneTokenUsage"]; gotFile != "accounting.go" {
		t.Fatalf("modelgateway owner function CloneTokenUsage declared in %q, want accounting.go", gotFile)
	}
}

func TestArchitectureBoundaryToolRuntimeOwnerHasSubpackageSchedulingSurface(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	toolRuntimeDir := filepath.Join(repoRoot, "internal", "kernel", "toolruntime")
	gotTypes := kernelTypeDeclarationFiles(t, toolRuntimeDir)
	for typeName, wantFile := range map[string]string{
		"AccessPlan":        "scheduling.go",
		"ExecutionBatch":    "scheduling.go",
		"PlannedCall":       "scheduling.go",
		"ResourceFootprint": "scheduling.go",
		"SchedulingSpec":    "scheduling.go",
	} {
		if gotFile := gotTypes[typeName]; gotFile != wantFile {
			t.Fatalf("toolruntime owner type %s declared in %q, want %q", typeName, gotFile, wantFile)
		}
	}
	gotFunctions := kernelFunctionDeclarationFiles(t, toolRuntimeDir)
	for functionName, wantFile := range map[string]string{
		"JobControlSchedulingSpec":   "scheduling.go",
		"PlanExecutionBatches":       "scheduling.go",
		"ResourceReadSchedulingSpec": "scheduling.go",
		"ShellExecSchedulingSpec":    "scheduling.go",
	} {
		if gotFile := gotFunctions[functionName]; gotFile != wantFile {
			t.Fatalf("toolruntime owner function %s declared in %q, want %q", functionName, gotFile, wantFile)
		}
	}
}

func TestArchitectureBoundaryToolRuntimeOwnerHasSubpackageResultSurface(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	toolRuntimeDir := filepath.Join(repoRoot, "internal", "kernel", "toolruntime")
	gotTypes := kernelTypeDeclarationFiles(t, toolRuntimeDir)
	for typeName, wantFile := range map[string]string{
		"RequestInvalidProjection": "results.go",
		"RequestError":             "results.go",
		"CapabilityProjection":     "results.go",
		"OperationResult":          "results.go",
		"ManagedJobResult":         "results.go",
		"JobControlResult":         "results.go",
	} {
		if gotFile := gotTypes[typeName]; gotFile != wantFile {
			t.Fatalf("toolruntime owner result type %s declared in %q, want %q", typeName, gotFile, wantFile)
		}
	}
}

func TestArchitectureBoundaryAuthorityOwnerHasSubpackageApprovalSurface(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	authorityDir := filepath.Join(repoRoot, "internal", "kernel", "authority")
	gotTypes := kernelTypeDeclarationFiles(t, authorityDir)
	for typeName, wantFile := range map[string]string{
		"ApprovalListResponse":       "types.go",
		"ApprovalDecisionRequest":    "types.go",
		"ApprovalProjection":         "types.go",
		"ApprovalPolicySnapshot":     "types.go",
		"ApprovalEffectSummary":      "types.go",
		"SandboxReadinessProjection": "types.go",
	} {
		if gotFile := gotTypes[typeName]; gotFile != wantFile {
			t.Fatalf("authority owner type %s declared in %q, want %q", typeName, gotFile, wantFile)
		}
	}
}

func TestArchitectureBoundaryWorkRegistryOwnerHasSubpackageTypeSurface(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	workRegistryDir := filepath.Join(repoRoot, "internal", "kernel", "workregistry")
	gotTypes := kernelTypeDeclarationFiles(t, workRegistryDir)
	for typeName, wantFile := range map[string]string{
		"SubmitRequest":  "types.go",
		"CancelRequest":  "types.go",
		"WorkProjection": "types.go",
	} {
		if gotFile := gotTypes[typeName]; gotFile != wantFile {
			t.Fatalf("workregistry owner type %s declared in %q, want %q", typeName, gotFile, wantFile)
		}
	}
}

func TestArchitectureBoundaryJobRuntimeOwnerHasSubpackageTypeSurface(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(kernelPackageDir(t), "..", ".."))
	jobRuntimeDir := filepath.Join(repoRoot, "internal", "kernel", "jobruntime")
	gotTypes := kernelTypeDeclarationFiles(t, jobRuntimeDir)
	for typeName, wantFile := range map[string]string{
		"JobProjection":       "types.go",
		"ObservationDelivery": "types.go",
	} {
		if gotFile := gotTypes[typeName]; gotFile != wantFile {
			t.Fatalf("jobruntime owner type %s declared in %q, want %q", typeName, gotFile, wantFile)
		}
	}
}

func TestArchitectureBoundaryHTTPTransportDoesNotReplayOwnerFacts(t *testing.T) {
	root := kernelPackageDir(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read kernel package dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "http") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content := readRepoText(t, root, name)
		for _, forbidden := range []string{
			"loadEvents(",
			"appendEvent(",
			"appendOperationEvent(",
			"appendJobEvent(",
			"appendTerminalJobEvent(",
			"appendMemoryCandidateEvent(",
			"appendWorkEvent(",
			"mergeWorkProjection(",
			"mergeMemoryCandidateProjection(",
		} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s contains owner state/replay helper %q; HTTP transport must auth/decode/delegate/encode only", name, forbidden)
			}
		}
	}
}

func TestArchitectureBoundaryHTTPHandlersLiveInSurfaceFiles(t *testing.T) {
	root := kernelPackageDir(t)
	want := map[string]string{
		"Handler":                        "http.go",
		"authorizeRuntimeRequest":        "http.go",
		"requireJSONContentType":         "http.go",
		"decodeRequest":                  "http.go",
		"writeJSON":                      "http.go",
		"writeError":                     "http.go",
		"writeKernelUnavailable":         "http.go",
		"routePathValue":                 "http.go",
		"handleSubmitTurn":               "http_turn.go",
		"turnErrorHTTPStatus":            "http_turn.go",
		"handleExecShell":                "http_tools.go",
		"handleListApprovals":            "http_approvals.go",
		"handleDecideApproval":           "http_approvals.go",
		"validApprovalStatusFilter":      "http_approvals.go",
		"handleSubmitWork":               "http_work.go",
		"handleGetWork":                  "http_work.go",
		"handleCancelWork":               "http_work.go",
		"handleCreateMemoryCandidate":    "http_memory.go",
		"handleListMemoryCandidates":     "http_memory.go",
		"handleGetMemoryCandidate":       "http_memory.go",
		"handleApproveMemoryCandidate":   "http_memory.go",
		"handleRejectMemoryCandidate":    "http_memory.go",
		"handleSupersedeMemoryCandidate": "http_memory.go",
		"handleForgetMemoryCandidate":    "http_memory.go",
		"handleListSessions":             "http_inspection.go",
		"handleGetSession":               "http_inspection.go",
		"handleGetSessionTimeline":       "http_inspection.go",
		"handleGetSessionTimelineDetail": "http_inspection.go",
		"handleGetTurnContext":           "http_inspection.go",
		"handleGetTurnAudit":             "http_inspection.go",
		"handleGetTurnEvents":            "http_inspection.go",
	}
	got := kernelFunctionDeclarationFiles(t, root)
	for functionName, wantFile := range want {
		if gotFile := got[functionName]; gotFile != wantFile {
			t.Fatalf("%s declared in %q, want %q", functionName, gotFile, wantFile)
		}
	}
}

func readRepoText(t *testing.T, repoRoot string, pathParts ...string) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(append([]string{repoRoot}, pathParts...)...))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(pathParts...), err)
	}
	return string(payload)
}

func functionBodySource(t *testing.T, path string, functionName string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, payload, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != functionName || function.Body == nil {
			continue
		}
		start := fileSet.Position(function.Body.Pos()).Offset
		end := fileSet.Position(function.Body.End()).Offset
		return string(payload[start:end])
	}
	t.Fatalf("function %s not found in %s", functionName, path)
	return ""
}

func kernelTypeDeclarationFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read kernel package dir: %v", err)
	}
	result := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(root, name)
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, declaration := range parsed.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, spec := range general.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				result[typeSpec.Name.Name] = name
			}
		}
	}
	return result
}

func kernelFunctionDeclarationFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read kernel package dir: %v", err)
	}
	result := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(root, name)
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			result[function.Name.Name] = name
		}
	}
	return result
}

type markdownIssueSection struct {
	id   string
	body string
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

func retiredIssueRequiresReferenceAlignment(issue markdownIssueSection) bool {
	return strings.HasPrefix(issue.id, "KERNEL-BOUNDARY-") ||
		strings.Contains(issue.body, "\n- Type: architecture issue.")
}

func hasReferenceAlignmentOrRejectedDrift(body string) bool {
	return strings.Contains(body, "\n- Reference alignment:") ||
		strings.Contains(body, "\n- Rejected drift risk:")
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
