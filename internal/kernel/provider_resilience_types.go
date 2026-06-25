package kernel

type ProviderAttemptProjection struct {
	RoundIndex  int    `json:"round_index"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	Status      string `json:"status"`
	ReasonCode  string `json:"reason_code,omitempty"`
	Message     string `json:"message,omitempty"`
	Retryable   bool   `json:"retryable,omitempty"`
	RepairKind  string `json:"repair_kind,omitempty"`
}
