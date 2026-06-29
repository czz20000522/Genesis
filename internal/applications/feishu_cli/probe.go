package feishucli

import connectorruntime "genesis/internal/applications/connector_runtime"

type AdapterProbeConfig struct {
	SourceCommand              string
	SourceCommandArgs          []string
	SourceBlockedReason        string
	FinalDeliveryCommand       string
	FinalDeliveryCommandArgs   []string
	FinalDeliveryBlockedReason string
}

type AdapterProbeReport struct {
	Connector     string                              `json:"connector"`
	Ready         bool                                `json:"ready"`
	EventSource   connectorruntime.ProbeSurfaceReport `json:"event_source"`
	FinalDelivery connectorruntime.ProbeSurfaceReport `json:"final_delivery"`
}

func ProbeAdapter(config AdapterProbeConfig) AdapterProbeReport {
	eventSource := connectorruntime.ProbeSourceCommandSurface(config.SourceCommand, config.SourceCommandArgs, config.SourceBlockedReason)
	finalDelivery := connectorruntime.ProbeConnectorCommandSurface(config.FinalDeliveryCommand, config.FinalDeliveryCommandArgs, config.FinalDeliveryBlockedReason)
	return AdapterProbeReport{
		Connector:     "feishu",
		Ready:         eventSource.Status == connectorruntime.ProbeStatusOK && finalDelivery.Status == connectorruntime.ProbeStatusOK,
		EventSource:   eventSource,
		FinalDelivery: finalDelivery,
	}
}
