package kernel

type DiscoveryQueryRequest struct {
	Intent                string   `json:"intent"`
	CurrentContextSummary string   `json:"current_context_summary,omitempty"`
	RequestedKinds        []string `json:"requested_kinds,omitempty"`
	Limit                 int      `json:"limit,omitempty"`
}

type DiscoveryQueryResponse struct {
	Candidates []DiscoveryCandidateProjection `json:"candidates"`
}

type DiscoveryCandidateProjection struct {
	Ref           string `json:"ref"`
	Kind          string `json:"kind"`
	Summary       string `json:"summary"`
	Scope         string `json:"scope,omitempty"`
	AppliesWhen   string `json:"applies_when,omitempty"`
	Confidence    string `json:"confidence"`
	SourceSummary string `json:"source_summary,omitempty"`
}

type CapabilityDescriptor struct {
	CapabilityRef string   `json:"capability_ref"`
	Name          string   `json:"name"`
	Summary       string   `json:"summary"`
	Intents       []string `json:"intents,omitempty"`
	InputSummary  string   `json:"input_summary,omitempty"`
	HealthSummary string   `json:"health_summary,omitempty"`
}
