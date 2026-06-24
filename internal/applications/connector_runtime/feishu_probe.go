package connectorruntime

import (
	"strings"
)

const (
	ProbeStatusOK     = "ok"
	ProbeStatusFailed = "failed"
)

type FeishuAdapterProbeConfig struct {
	Executable string
	Profile    string
	EventKey   string
	Identity   string
}

type FeishuAdapterProbeReport struct {
	Connector     string             `json:"connector"`
	Ready         bool               `json:"ready"`
	EventSource   ProbeSurfaceReport `json:"event_source"`
	FinalDelivery ProbeSurfaceReport `json:"final_delivery"`
}

type ProbeSurfaceReport struct {
	Status     string   `json:"status"`
	Reason     string   `json:"reason,omitempty"`
	Executable string   `json:"executable,omitempty"`
	Args       []string `json:"args,omitempty"`
}

func ProbeFeishuAdapter(config FeishuAdapterProbeConfig) FeishuAdapterProbeReport {
	eventSource := probeFeishuEventSource(config)
	finalDelivery := probeFeishuFinalDelivery(config)
	return FeishuAdapterProbeReport{
		Connector:     "feishu",
		Ready:         eventSource.Status == ProbeStatusOK && finalDelivery.Status == ProbeStatusOK,
		EventSource:   eventSource,
		FinalDelivery: finalDelivery,
	}
}

func probeFeishuEventSource(config FeishuAdapterProbeConfig) ProbeSurfaceReport {
	executable, args, err := (FeishuEventSourceConfig{
		Executable: config.Executable,
		Profile:    config.Profile,
		EventKey:   config.EventKey,
		Identity:   config.Identity,
		MaxEvents:  1,
		Timeout:    "1s",
	}).Command()
	if err != nil {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: safeProbeReason(err.Error())}
	}
	return ProbeSurfaceReport{
		Status:     ProbeStatusOK,
		Executable: executable,
		Args:       append([]string(nil), args...),
	}
}

func probeFeishuFinalDelivery(config FeishuAdapterProbeConfig) ProbeSurfaceReport {
	driver := NewFeishuSendMessageCommandTemplateDriver(config.Profile, config.Executable, nil)
	executable, args, _, reason, err := driver.render(ConnectorAction{
		OutboxID:       "outbox_probe",
		Connector:      "feishu",
		ActionKind:     "send_message",
		TargetRef:      ExternalThreadRef{Connector: "feishu", Kind: "chat", ExternalID: "oc_probe"},
		Payload:        map[string]string{"body": "probe"},
		IdempotencyKey: "probe_idempotency_key",
		Attempt:        1,
	})
	if err != nil {
		if strings.TrimSpace(reason) == "" {
			reason = err.Error()
		}
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: safeProbeReason(reason)}
	}
	resolved, err := resolveCommandExecutable(executable)
	if err != nil {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: safeProbeReason(err.Error())}
	}
	if unsafeResolvedCommandExecutable(resolved) {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: "unsafe_command_executable"}
	}
	return ProbeSurfaceReport{
		Status:     ProbeStatusOK,
		Executable: resolved,
		Args:       append([]string(nil), args...),
	}
}

func safeProbeReason(reason string) string {
	reason = strings.TrimSpace(SafeCLIProbeExcerpt([]byte(reason)))
	if reason == "" {
		return "connector_probe_failed"
	}
	if isCredentialShapedExternalValue(reason) {
		return "connector_probe_failed"
	}
	return reason
}
