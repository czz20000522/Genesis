package connectorruntime

import "strings"

const (
	ProbeStatusOK     = "ok"
	ProbeStatusFailed = "failed"
)

type ProbeSurfaceReport struct {
	Status     string   `json:"status"`
	Reason     string   `json:"reason,omitempty"`
	Executable string   `json:"executable,omitempty"`
	Args       []string `json:"args,omitempty"`
}

func ProbeSourceCommandSurface(command string, args []string, blockedReason string) ProbeSurfaceReport {
	command = strings.TrimSpace(command)
	if command == "" {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: "source_command_missing"}
	}
	if strings.TrimSpace(blockedReason) != "" {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: SafeProbeReason(blockedReason)}
	}
	executable, err := (SourceCommandAdapter{Executable: command}).resolveExecutable()
	if err != nil {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: SafeProbeReason(err.Error())}
	}
	return ProbeSurfaceReport{Status: ProbeStatusOK, Executable: executable, Args: append([]string(nil), args...)}
}

func ProbeConnectorCommandSurface(command string, args []string, blockedReason string) ProbeSurfaceReport {
	if strings.TrimSpace(blockedReason) != "" {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: SafeProbeReason(blockedReason)}
	}
	adapter := ConnectorCommandAdapter{Executable: command}
	resolved, err := adapter.resolveExecutable()
	if err != nil {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: SafeProbeReason(err.Error())}
	}
	return ProbeSurfaceReport{Status: ProbeStatusOK, Executable: resolved, Args: append([]string(nil), args...)}
}

func SafeProbeReason(reason string) string {
	reason = strings.TrimSpace(SafeCLIProbeExcerpt([]byte(reason)))
	if reason == "" {
		return "connector_probe_failed"
	}
	if isCredentialShapedExternalValue(reason) {
		return "connector_probe_failed"
	}
	return reason
}
