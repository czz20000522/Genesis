package connectorruntime

import (
	"strings"
)

const (
	ProbeStatusOK     = "ok"
	ProbeStatusFailed = "failed"
)

type FeishuAdapterProbeConfig struct {
	SourceCommand              string
	SourceCommandArgs          []string
	SourceBlockedReason        string
	FinalDeliveryCommand       string
	FinalDeliveryCommandArgs   []string
	FinalDeliveryBlockedReason string
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
	command := strings.TrimSpace(config.SourceCommand)
	if command == "" {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: "source_command_missing"}
	}
	if strings.TrimSpace(config.SourceBlockedReason) != "" {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: safeProbeReason(config.SourceBlockedReason)}
	}
	executable, err := (SourceCommandAdapter{Executable: command}).resolveExecutable()
	if err != nil {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: safeProbeReason(err.Error())}
	}
	return ProbeSurfaceReport{
		Status:     ProbeStatusOK,
		Executable: executable,
		Args:       append([]string(nil), config.SourceCommandArgs...),
	}
}

func probeFeishuFinalDelivery(config FeishuAdapterProbeConfig) ProbeSurfaceReport {
	if strings.TrimSpace(config.FinalDeliveryBlockedReason) != "" {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: safeProbeReason(config.FinalDeliveryBlockedReason)}
	}
	adapter := ConnectorCommandAdapter{Executable: config.FinalDeliveryCommand}
	resolved, err := adapter.resolveExecutable()
	if err != nil {
		return ProbeSurfaceReport{Status: ProbeStatusFailed, Reason: safeProbeReason(err.Error())}
	}
	return ProbeSurfaceReport{
		Status:     ProbeStatusOK,
		Executable: resolved,
		Args:       append([]string(nil), config.FinalDeliveryCommandArgs...),
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
