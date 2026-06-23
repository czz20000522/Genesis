package kernel

type TokenUsage struct {
	InputTokens     int `json:"input_tokens,omitempty"`
	OutputTokens    int `json:"output_tokens,omitempty"`
	TotalTokens     int `json:"total_tokens,omitempty"`
	CacheHitTokens  int `json:"cache_hit_tokens,omitempty"`
	CacheMissTokens int `json:"cache_miss_tokens,omitempty"`
}

type ModelContextAccountingProjection struct {
	RoundIndex                int         `json:"round_index,omitempty"`
	Model                     string      `json:"model,omitempty"`
	ModelInputKinds           []string    `json:"model_input_kinds,omitempty"`
	HistoryTurnIDs            []string    `json:"history_turn_ids,omitempty"`
	CompactedThroughTurnID    string      `json:"compacted_through_turn_id,omitempty"`
	Usage                     *TokenUsage `json:"usage,omitempty"`
	ProcessedInputTokens      int         `json:"processed_input_tokens,omitempty"`
	ProcessedInputTokenSource string      `json:"processed_input_token_source,omitempty"`
	ToolRoundCount            int         `json:"tool_round_count,omitempty"`
	ToolCallCount             int         `json:"tool_call_count,omitempty"`
	ToolResultCount           int         `json:"tool_result_count,omitempty"`
}
