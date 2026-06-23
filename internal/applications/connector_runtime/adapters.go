package connectorruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var errUnsafeCommandExecutable = errors.New("unsafe connector command executable")

type ConsoleAdapter struct {
	Writer io.Writer
}

func (a ConsoleAdapter) Execute(_ context.Context, action ConnectorAction) (ConnectorActionResult, error) {
	writer := a.Writer
	if writer == nil {
		writer = os.Stdout
	}
	body := strings.TrimSpace(action.Payload["body"])
	if _, err := fmt.Fprintln(writer, body); err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: err.Error()}, err
	}
	return ConnectorActionResult{
		ExternalActionRef: action.OutboxID,
		Status:            DeliveryStatusSent,
	}, nil
}

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type OSCommandRunner struct {
	Env []string
}

func (r OSCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	executable, err := resolveCommandExecutable(name)
	if err != nil {
		return nil, err
	}
	if unsafeResolvedCommandExecutable(executable) {
		return nil, fmt.Errorf("%w: %q is not a direct binary", errUnsafeCommandExecutable, executable)
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	if r.Env != nil {
		cmd.Env = append([]string(nil), r.Env...)
	} else {
		cmd.Env = connectorCommandEnvironment(os.Environ())
	}
	return cmd.CombinedOutput()
}

func resolveCommandExecutable(name string) (string, error) {
	return exec.LookPath(name)
}

func connectorCommandEnvironment(hostEnv []string) []string {
	allowed := map[string]struct{}{
		"APPDATA":      {},
		"HOME":         {},
		"LOCALAPPDATA": {},
		"PATH":         {},
		"PATHEXT":      {},
		"PROGRAMDATA":  {},
		"SYSTEMROOT":   {},
		"TEMP":         {},
		"TMP":          {},
		"USERPROFILE":  {},
		"WINDIR":       {},
	}
	env := make([]string, 0, len(allowed))
	seen := map[string]struct{}{}
	for _, entry := range hostEnv {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(key)
		if _, ok := allowed[upper]; !ok {
			continue
		}
		if _, ok := seen[upper]; ok {
			continue
		}
		seen[upper] = struct{}{}
		env = append(env, entry)
	}
	return env
}

func unsafeResolvedCommandExecutable(executable string) bool {
	if invalidCommandTemplateExecutable(executable) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(executable))
	if runtime.GOOS == "windows" {
		return ext != ".exe"
	}
	switch ext {
	case ".bat", ".cmd", ".ps1", ".psm1", ".sh", ".bash", ".zsh", ".fish":
		return true
	default:
		return false
	}
}
