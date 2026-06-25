package kernel

type ContextCompactionProjection struct {
	Trigger                         string                           `json:"trigger"`
	Status                          string                           `json:"status,omitempty"`
	Summary                         string                           `json:"summary,omitempty"`
	CompactedThroughTurnID          string                           `json:"compacted_through_turn_id,omitempty"`
	CompactedTurnCount              int                              `json:"compacted_turn_count,omitempty"`
	SourceInputTokens               int                              `json:"source_input_tokens,omitempty"`
	SourceUsage                     *TokenUsage                      `json:"source_usage,omitempty"`
	CacheStability                  *ContextCacheStabilityProjection `json:"cache_stability,omitempty"`
	FailureReason                   string                           `json:"failure_reason,omitempty"`
	PreviousFailureReason           string                           `json:"previous_failure_reason,omitempty"`
	DeferredReason                  string                           `json:"deferred_reason,omitempty"`
	RetryAfterCompletedTurns        int                              `json:"retry_after_completed_turns,omitempty"`
	BackoffRemainingTurns           int                              `json:"backoff_remaining_turns,omitempty"`
	ConsecutiveCompletedCompactions int                              `json:"consecutive_completed_compactions,omitempty"`
	Model                           string                           `json:"model,omitempty"`
	Usage                           *TokenUsage                      `json:"usage,omitempty"`
}

type ContextCacheStabilityProjection struct {
	Samples               int    `json:"samples,omitempty"`
	CacheHitTokens        int    `json:"cache_hit_tokens,omitempty"`
	CacheMissTokens       int    `json:"cache_miss_tokens,omitempty"`
	HitRatePermille       int    `json:"hit_rate_per_mille,omitempty"`
	FirstHitRatePermille  int    `json:"first_hit_rate_per_mille,omitempty"`
	LatestHitRatePermille int    `json:"latest_hit_rate_per_mille,omitempty"`
	LatestCacheHitTokens  int    `json:"latest_cache_hit_tokens,omitempty"`
	LatestCacheMissTokens int    `json:"latest_cache_miss_tokens,omitempty"`
	Trend                 string `json:"trend,omitempty"`
}
