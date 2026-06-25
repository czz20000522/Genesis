package modelgateway

import "testing"

func TestCloneTokenUsageReturnsIndependentCopy(t *testing.T) {
	original := &TokenUsage{
		InputTokens:     10,
		OutputTokens:    4,
		TotalTokens:     14,
		CacheHitTokens:  6,
		CacheMissTokens: 4,
	}

	cloned := CloneTokenUsage(original)

	if cloned == nil || cloned == original {
		t.Fatalf("CloneTokenUsage returned %+v, want independent copy", cloned)
	}
	cloned.InputTokens = 99
	if original.InputTokens != 10 {
		t.Fatalf("mutating clone changed original: %+v", original)
	}
}

func TestCloneTokenUsagePreservesNil(t *testing.T) {
	if cloned := CloneTokenUsage(nil); cloned != nil {
		t.Fatalf("CloneTokenUsage(nil) = %+v, want nil", cloned)
	}
}
