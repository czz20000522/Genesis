package kernel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func controlledDefaultCommandNames() []string {
	return []string{"cat", "echo", "get-content", "printf", "pwd", "set-content", "type", "write-output"}
}

func isControlledDefaultCommandName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, allowed := range controlledDefaultCommandNames() {
		if name == allowed {
			return true
		}
	}
	return false
}

type shellExecutionPlan struct {
	cwd        string
	controlled *controlledShellCommand
}

type controlledShellCommand struct {
	kind   string
	path   string
	value  string
	stdout string
}

func prepareShellExecution(policy ToolPolicy, req ShellExecRequest) (shellExecutionPlan, string) {
	plan := shellExecutionPlan{cwd: strings.TrimSpace(req.CWD)}
	switch policy.PermissionMode {
	case PermissionModeDefault:
		return prepareDefaultShellExecution(policy, req)
	}
	return plan, ""
}

func prepareDefaultShellExecution(policy ToolPolicy, req ShellExecRequest) (shellExecutionPlan, string) {
	plan := shellExecutionPlan{cwd: strings.TrimSpace(req.CWD)}
	if strings.TrimSpace(policy.WorkspaceRoot) == "" {
		return plan, "workspace_root_required"
	}
	if !pathWithin(req.CWD, policy.WorkspaceRoot) {
		return plan, "cwd_outside_workspace"
	}
	cwd, err := canonicalPathForContainment(req.CWD)
	if err != nil {
		return plan, "cwd_outside_workspace"
	}
	plan.cwd = cwd
	fields, err := splitCommandFields(req.Command)
	if err != nil || len(fields) == 0 {
		return plan, "unsupported_default_command"
	}
	action, reason := controlledDefaultCommand(fields, plan.cwd, policy.WorkspaceRoot)
	if reason != "" {
		return plan, reason
	}
	plan.controlled = &action
	return plan, ""
}

func hasParentTraversal(token string) bool {
	normalized := strings.ReplaceAll(token, "\\", "/")
	return normalized == ".." ||
		strings.HasPrefix(normalized, "../") ||
		strings.Contains(normalized, "/../") ||
		strings.HasSuffix(normalized, "/..") ||
		strings.Contains(normalized, "=../") ||
		strings.HasSuffix(normalized, "=..")
}

func pathWithin(path string, root string) bool {
	if pathHasLinkOrReparsePoint(path) || pathHasLinkOrReparsePoint(root) {
		return false
	}
	candidate, err := canonicalPathForContainment(path)
	if err != nil {
		return false
	}
	canonicalRoot, err := canonicalExistingPath(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(canonicalRoot, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func pathHasLinkOrReparsePoint(path string) bool {
	current, err := filepath.Abs(path)
	if err != nil {
		return true
	}
	current = filepath.Clean(current)
	for {
		info, err := os.Lstat(current)
		if err == nil {
			mode := info.Mode()
			if mode&os.ModeSymlink != 0 || (runtime.GOOS == "windows" && mode&os.ModeIrregular != 0) {
				return true
			}
		} else if !os.IsNotExist(err) {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

func canonicalExistingPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func canonicalPathForContainment(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		return filepath.Clean(resolved), nil
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absPath))
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(parent, filepath.Base(absPath))), nil
}

func controlledDefaultCommand(fields []string, cwd string, workspaceRoot string) (controlledShellCommand, string) {
	name := strings.ToLower(fields[0])
	if !isControlledDefaultCommandName(name) {
		return controlledShellCommand{}, "unsupported_default_command"
	}
	switch name {
	case "echo", "write-output":
		if hasUnsupportedDefaultToken(fields[1:], false) {
			return controlledShellCommand{}, "unsupported_default_command"
		}
		return controlledShellCommand{kind: "stdout", stdout: strings.Join(fields[1:], " ") + "\n"}, ""
	case "printf":
		return controlledPrintfCommand(fields, cwd, workspaceRoot)
	case "set-content":
		return controlledSetContentCommand(fields, cwd, workspaceRoot)
	case "cat", "type", "get-content":
		return controlledReadCommand(fields, cwd, workspaceRoot)
	case "pwd":
		if len(fields) != 1 {
			return controlledShellCommand{}, "unsupported_default_command"
		}
		return controlledShellCommand{kind: "stdout", stdout: cwd + "\n"}, ""
	default:
		return controlledShellCommand{}, "unsupported_default_command"
	}
}

func controlledPrintfCommand(fields []string, cwd string, workspaceRoot string) (controlledShellCommand, string) {
	redirectAt := -1
	for i, field := range fields {
		if field == ">" {
			redirectAt = i
			break
		}
	}
	if redirectAt == -1 {
		if hasUnsupportedDefaultToken(fields[1:], false) {
			return controlledShellCommand{}, "unsupported_default_command"
		}
		return controlledShellCommand{kind: "stdout", stdout: strings.Join(fields[1:], " ")}, ""
	}
	if redirectAt == 1 || redirectAt != len(fields)-2 || hasUnsupportedDefaultToken(fields[1:redirectAt], false) {
		return controlledShellCommand{}, "unsupported_default_command"
	}
	path, reason := resolveWorkspacePath(cwd, workspaceRoot, fields[len(fields)-1])
	if reason != "" {
		return controlledShellCommand{}, reason
	}
	return controlledShellCommand{kind: "write", path: path, value: strings.Join(fields[1:redirectAt], " ")}, ""
}

func controlledSetContentCommand(fields []string, cwd string, workspaceRoot string) (controlledShellCommand, string) {
	pathArg, value, noNewline, ok := parseSetContentFields(fields[1:])
	if !ok || hasUnsupportedDefaultToken([]string{value}, false) {
		return controlledShellCommand{}, "unsupported_default_command"
	}
	path, reason := resolveWorkspacePath(cwd, workspaceRoot, pathArg)
	if reason != "" {
		return controlledShellCommand{}, reason
	}
	if !noNewline {
		value += "\n"
	}
	return controlledShellCommand{kind: "write", path: path, value: value}, ""
}

func controlledReadCommand(fields []string, cwd string, workspaceRoot string) (controlledShellCommand, string) {
	pathArg, ok := parsePathOnlyFields(fields[1:])
	if !ok {
		return controlledShellCommand{}, "unsupported_default_command"
	}
	path, reason := resolveWorkspacePath(cwd, workspaceRoot, pathArg)
	if reason != "" {
		return controlledShellCommand{}, reason
	}
	return controlledShellCommand{kind: "read", path: path}, ""
}

func parseSetContentFields(fields []string) (string, string, bool, bool) {
	var pathArg string
	var value string
	noNewline := false
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		lower := strings.ToLower(field)
		switch {
		case lower == "-literalpath" || lower == "-path":
			i++
			if i >= len(fields) {
				return "", "", false, false
			}
			pathArg = fields[i]
		case strings.HasPrefix(lower, "-literalpath=") || strings.HasPrefix(lower, "-path="):
			_, pathArg, _ = strings.Cut(field, "=")
		case lower == "-value":
			i++
			if i >= len(fields) {
				return "", "", false, false
			}
			value = fields[i]
		case strings.HasPrefix(lower, "-value="):
			_, value, _ = strings.Cut(field, "=")
		case lower == "-nonewline":
			noNewline = true
		default:
			return "", "", false, false
		}
	}
	return pathArg, value, noNewline, pathArg != "" && value != ""
}

func parsePathOnlyFields(fields []string) (string, bool) {
	if len(fields) == 1 {
		lower := strings.ToLower(fields[0])
		if strings.HasPrefix(lower, "-literalpath=") || strings.HasPrefix(lower, "-path=") {
			_, value, ok := strings.Cut(fields[0], "=")
			return value, ok
		}
		return fields[0], true
	}
	if len(fields) == 2 {
		lower := strings.ToLower(fields[0])
		if lower == "-literalpath" || lower == "-path" {
			return fields[1], true
		}
	}
	return "", false
}

func hasUnsupportedDefaultToken(fields []string, allowRedirect bool) bool {
	for _, field := range fields {
		if allowRedirect && field == ">" {
			continue
		}
		if strings.ContainsAny(field, "\r\n;|&`$<>") {
			return true
		}
	}
	return false
}

func resolveWorkspacePath(cwd string, workspaceRoot string, pathArg string) (string, string) {
	pathArg = strings.TrimSpace(pathArg)
	if pathArg == "" {
		return "", "unsupported_default_command"
	}
	if hasParentTraversal(pathArg) {
		return "", "command_path_outside_workspace"
	}
	target := pathArg
	if !filepath.IsAbs(target) {
		target = filepath.Join(cwd, target)
	}
	if !pathWithin(target, workspaceRoot) {
		return "", "command_path_outside_workspace"
	}
	resolved, err := canonicalPathForContainment(target)
	if err != nil {
		return "", "command_path_outside_workspace"
	}
	if targetHasUnsafeHardlinkAlias(resolved) {
		return "", "command_path_unsafe_link"
	}
	return resolved, ""
}

func targetHasUnsafeHardlinkAlias(path string) bool {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	if !info.Mode().IsRegular() {
		return false
	}
	return regularFileHasMultipleLinks(path, info)
}

func executeControlledShellCommand(command controlledShellCommand) (capturedOutput, capturedOutput, int) {
	switch command.kind {
	case "stdout":
		return captureBytes([]byte(command.stdout), maxShellOutputBytes), capturedOutput{}, 0
	case "write":
		if err := os.WriteFile(command.path, []byte(command.value), 0o644); err != nil {
			return capturedOutput{}, captureBytes([]byte("write failed"), maxShellOutputBytes), -1
		}
		return capturedOutput{}, capturedOutput{}, 0
	case "read":
		data, err := os.ReadFile(command.path)
		if err != nil {
			return capturedOutput{}, captureBytes([]byte("read failed"), maxShellOutputBytes), -1
		}
		return captureBytes(data, maxShellOutputBytes), capturedOutput{}, 0
	default:
		return capturedOutput{}, captureBytes([]byte(fmt.Sprintf("unsupported controlled shell command kind %q", command.kind)), maxShellOutputBytes), -1
	}
}

func splitCommandFields(command string) ([]string, error) {
	var fields []string
	var current strings.Builder
	var quote rune
	for _, char := range command {
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
				continue
			}
			current.WriteRune(char)
		case char == '\'' || char == '"':
			quote = char
		case char == ' ' || char == '\t':
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields, nil
}
