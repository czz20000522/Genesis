package connectorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type CommandTemplateDriver struct {
	Executable string
	Profile    string
	Actions    map[string]CommandTemplateAction
	Runner     CommandRunner
}

type CommandTemplateAction struct {
	Argv                       []string
	ExternalActionRefJSONPaths []string
}

func (d CommandTemplateDriver) Execute(ctx context.Context, action ConnectorAction) (ConnectorActionResult, error) {
	executable, args, templateAction, reason, err := d.render(action)
	if err != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: reason}, err
	}
	runner := d.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	output, err := runner.Run(ctx, executable, args...)
	boundedOutput, outputErr := boundConnectorCommandOutput(output)
	if outputErr != nil {
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "external_command_output_exceeded"}, outputErr
	}
	if err != nil {
		if errors.Is(err, errUnsafeCommandExecutable) {
			return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "invalid_command_template"}, err
		}
		if errors.Is(err, errConnectorCommandOutputExceeded) {
			return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "external_command_output_exceeded"}, err
		}
		return ConnectorActionResult{Status: DeliveryStatusFailed, Reason: "external_command_failed"}, err
	}
	return ConnectorActionResult{
		ExternalActionRef: firstStringAtJSONPath(boundedOutput, templateAction.ExternalActionRefJSONPaths),
		Status:            DeliveryStatusSent,
	}, nil
}

func (d CommandTemplateDriver) render(action ConnectorAction) (string, []string, CommandTemplateAction, string, error) {
	executable := strings.TrimSpace(d.Executable)
	if executable == "" || len(d.Actions) == 0 {
		return "", nil, CommandTemplateAction{}, "missing_connector_driver_config", fmt.Errorf("command template driver requires executable and action templates")
	}
	if invalidCommandTemplateExecutable(executable) {
		return "", nil, CommandTemplateAction{}, "invalid_command_template", fmt.Errorf("command template executable must be a direct executable, not a shell command")
	}
	if len(action.TargetRef.Metadata) != 0 {
		return "", nil, CommandTemplateAction{}, "invalid_action_payload", fmt.Errorf("connector action target metadata is not executable payload")
	}
	templateAction, ok := d.Actions[action.ActionKind]
	if !ok {
		return "", nil, CommandTemplateAction{}, "unsupported_action_kind", fmt.Errorf("unsupported connector action kind %q", action.ActionKind)
	}
	allowedPayloadKeys, err := validateTemplateAction(templateAction)
	if err != nil {
		return "", nil, CommandTemplateAction{}, "invalid_command_template", err
	}
	if !templateActionUsesVariable(templateAction, "profile") {
		return "", nil, CommandTemplateAction{}, "invalid_command_template", fmt.Errorf("command template requires explicit profile binding")
	}
	if strings.TrimSpace(d.Profile) == "" {
		return "", nil, CommandTemplateAction{}, "missing_explicit_profile", fmt.Errorf("command template requires explicit profile")
	}
	if hasUnexpectedPayloadKeySet(action.Payload, allowedPayloadKeys) {
		return "", nil, CommandTemplateAction{}, "invalid_action_payload", fmt.Errorf("connector action contains payload keys not referenced by the command template")
	}
	if err := validateActionValuesForTemplate(templateAction, action); err != nil {
		return "", nil, CommandTemplateAction{}, "invalid_action_payload", err
	}
	if len(templateAction.Argv) == 1 && strings.ContainsAny(templateAction.Argv[0], " \t\r\n") {
		return "", nil, CommandTemplateAction{}, "invalid_command_template", fmt.Errorf("command template must be argv tokens, not a shell string")
	}
	args := make([]string, 0, len(templateAction.Argv))
	for _, token := range templateAction.Argv {
		rendered, err := d.renderToken(token, action)
		if err != nil {
			return "", nil, CommandTemplateAction{}, "invalid_command_template", err
		}
		args = append(args, rendered)
	}
	return executable, args, templateAction, "", nil
}

func validateTemplateAction(templateAction CommandTemplateAction) (map[string]struct{}, error) {
	if len(templateAction.Argv) == 0 {
		return nil, fmt.Errorf("command template action requires argv")
	}
	allowedPayloadKeys := map[string]struct{}{}
	for _, token := range templateAction.Argv {
		variables, err := templateVariables(token)
		if err != nil {
			return nil, err
		}
		for _, variable := range variables {
			if isCredentialShapedTemplateField(variable) {
				return nil, fmt.Errorf("command template variable %q may expose connector credentials", variable)
			}
			if !isAllowedCommandTemplateVariable(variable) {
				return nil, fmt.Errorf("unknown command template variable %q", variable)
			}
			if strings.HasPrefix(variable, "payload.") {
				key := strings.TrimPrefix(variable, "payload.")
				if key == "" {
					return nil, fmt.Errorf("payload template variable is missing a key")
				}
				allowedPayloadKeys[key] = struct{}{}
			}
		}
	}
	for _, path := range templateAction.ExternalActionRefJSONPaths {
		if path == "" || isCredentialShapedTemplateField(path) {
			return nil, fmt.Errorf("external action ref json path %q is not allowed", path)
		}
	}
	return allowedPayloadKeys, nil
}

func validateActionValuesForTemplate(templateAction CommandTemplateAction, action ConnectorAction) error {
	for _, token := range templateAction.Argv {
		variables, err := templateVariables(token)
		if err != nil {
			return err
		}
		for _, variable := range variables {
			switch {
			case variable == "target.external_id":
				if strings.TrimSpace(action.TargetRef.ExternalID) == "" {
					return fmt.Errorf("connector action target external id is required")
				}
			case variable == "idempotency_key":
				if strings.TrimSpace(action.IdempotencyKey) == "" {
					return fmt.Errorf("connector action idempotency key is required")
				}
			case strings.HasPrefix(variable, "payload."):
				key := strings.TrimPrefix(variable, "payload.")
				value, ok := action.Payload[key]
				if !ok || strings.TrimSpace(value) == "" {
					return fmt.Errorf("connector action payload %q is required", key)
				}
			}
		}
	}
	return nil
}

func isAllowedCommandTemplateVariable(variable string) bool {
	return variable == "profile" ||
		variable == "target.external_id" ||
		variable == "idempotency_key" ||
		strings.HasPrefix(variable, "payload.")
}

func templateActionUsesVariable(templateAction CommandTemplateAction, variable string) bool {
	for _, token := range templateAction.Argv {
		variables, err := templateVariables(token)
		if err != nil {
			return false
		}
		for _, found := range variables {
			if found == variable {
				return true
			}
		}
	}
	return false
}

func invalidCommandTemplateExecutable(executable string) bool {
	ext := strings.ToLower(filepath.Ext(executable))
	if strings.ContainsAny(executable, " \t\r\n") && (ext == "" || strings.ContainsAny(ext, " \t\r\n")) {
		return true
	}
	name := strings.ToLower(filepath.Base(executable))
	switch name {
	case "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe", "sh", "sh.exe", "bash", "bash.exe":
		return true
	}
	switch ext {
	case ".bat", ".cmd", ".ps1", ".psm1", ".sh", ".bash", ".zsh", ".fish":
		return true
	default:
		return false
	}
}

func (d CommandTemplateDriver) renderToken(token string, action ConnectorAction) (string, error) {
	var builder strings.Builder
	rest := token
	for {
		start := strings.Index(rest, "${")
		if start == -1 {
			builder.WriteString(rest)
			return builder.String(), nil
		}
		builder.WriteString(rest[:start])
		end := strings.Index(rest[start+2:], "}")
		if end == -1 {
			return "", fmt.Errorf("unterminated command template variable")
		}
		variable := rest[start+2 : start+2+end]
		value, err := d.templateVariableValue(variable, action)
		if err != nil {
			return "", err
		}
		builder.WriteString(value)
		rest = rest[start+2+end+1:]
	}
}

func (d CommandTemplateDriver) templateVariableValue(variable string, action ConnectorAction) (string, error) {
	if isCredentialShapedTemplateField(variable) {
		return "", fmt.Errorf("command template variable %q may expose connector credentials", variable)
	}
	switch {
	case variable == "profile":
		profile := strings.TrimSpace(d.Profile)
		if profile == "" {
			return "", fmt.Errorf("command template requires explicit profile")
		}
		return profile, nil
	case variable == "target.external_id":
		target := strings.TrimSpace(action.TargetRef.ExternalID)
		if target == "" {
			return "", fmt.Errorf("command template requires target external id")
		}
		return target, nil
	case variable == "idempotency_key":
		if strings.TrimSpace(action.IdempotencyKey) == "" {
			return "", fmt.Errorf("command template requires idempotency key")
		}
		return action.IdempotencyKey, nil
	case strings.HasPrefix(variable, "payload."):
		key := strings.TrimPrefix(variable, "payload.")
		value, ok := action.Payload[key]
		if !ok || strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("command template requires payload %q", key)
		}
		return value, nil
	default:
		return "", fmt.Errorf("unknown command template variable %q", variable)
	}
}

func templateVariables(token string) ([]string, error) {
	var variables []string
	rest := token
	for {
		start := strings.Index(rest, "${")
		if start == -1 {
			return variables, nil
		}
		end := strings.Index(rest[start+2:], "}")
		if end == -1 {
			return nil, fmt.Errorf("unterminated command template variable")
		}
		variable := rest[start+2 : start+2+end]
		if strings.TrimSpace(variable) != variable || variable == "" {
			return nil, fmt.Errorf("invalid command template variable %q", variable)
		}
		variables = append(variables, variable)
		rest = rest[start+2+end+1:]
	}
}

func hasUnexpectedPayloadKeySet(payload map[string]string, allowed map[string]struct{}) bool {
	for key := range payload {
		if _, ok := allowed[key]; !ok {
			return true
		}
	}
	return false
}

func isCredentialShapedTemplateField(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"credential", "secret", "api_key", "apikey", "authorization", "password", "token"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func firstStringAtJSONPath(output []byte, paths []string) string {
	if len(output) == 0 || len(paths) == 0 {
		return ""
	}
	if len(output) > 4096 {
		output = output[:4096]
	}
	var payload any
	if err := json.Unmarshal(output, &payload); err != nil {
		return ""
	}
	for _, path := range paths {
		value, ok := valueAtJSONPath(payload, path)
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if safeExternalActionRef(text) {
			return text
		}
	}
	return ""
}

func safeExternalActionRef(ref string) bool {
	if ref == "" || len(ref) > 256 || isCredentialShapedExternalValue(ref) {
		return false
	}
	for _, r := range ref {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.' || r == ':':
		default:
			return false
		}
	}
	return true
}

func isCredentialShapedExternalValue(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"authorization", "bearer ", "credential", "secret", "api_key", "apikey", "password", "token", "sk-", "xoxb-", "xoxp-"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func valueAtJSONPath(payload any, path string) (any, bool) {
	current := payload
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			return nil, false
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
