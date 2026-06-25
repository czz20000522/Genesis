package modelgateway

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStatusErrorsClassifyRetryableProviderFailures(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable} {
		err := NewStatusError(status, "temporary", 0)
		var classified *ClassifiedError
		if !errors.As(err, &classified) {
			t.Fatalf("status %d produced %T, want ClassifiedError", status, err)
		}
		if !classified.Retryable || classified.Code != "provider_transient_failure" {
			t.Fatalf("status %d classified as %+v, want retryable transient", status, classified)
		}
	}

	auth := NewStatusError(http.StatusUnauthorized, "bad key", 0)
	var classified *ClassifiedError
	if !errors.As(auth, &classified) {
		t.Fatalf("auth error type = %T, want ClassifiedError", auth)
	}
	if classified.Retryable || classified.Code != "provider_auth_failed" {
		t.Fatalf("auth classified as %+v, want fail-fast auth", classified)
	}
}

func TestFailureClassifierRedactsAndSeparatesUnavailableProvider(t *testing.T) {
	unavailable := errors.New("provider unavailable")
	classifier := FailureClassifier{
		ProviderUnavailable: unavailable,
		Redact: func(text string) string {
			return strings.ReplaceAll(text, "sk-secret", "[REDACTED]")
		},
	}

	attempt := classifier.FromError(NewStatusError(http.StatusInternalServerError, "sk-secret", 0))
	if attempt.Status != "failed" || attempt.ReasonCode != "provider_transient_failure" || !attempt.Retryable {
		t.Fatalf("attempt = %+v, want failed retryable transient", attempt)
	}
	if strings.Contains(attempt.Message, "sk-secret") {
		t.Fatalf("attempt message was not redacted: %+v", attempt)
	}

	attempt = classifier.FromError(unavailable)
	if attempt.ReasonCode != "provider_unavailable" || attempt.Retryable {
		t.Fatalf("unavailable attempt = %+v, want provider_unavailable non-retryable", attempt)
	}
}

func TestRetryDelayUsesCappedRetryAfter(t *testing.T) {
	if got := RetryDelay(NewStatusError(http.StatusTooManyRequests, "slow down", time.Hour)); got != 2*time.Second {
		t.Fatalf("RetryDelay capped = %v, want 2s", got)
	}
	if got := RetryDelay(NewStatusError(http.StatusTooManyRequests, "slow down", 750*time.Millisecond)); got != 750*time.Millisecond {
		t.Fatalf("RetryDelay = %v, want Retry-After duration", got)
	}
	if got := RetryDelay(NewStatusError(http.StatusInternalServerError, "temporary", 0)); got != 0 {
		t.Fatalf("RetryDelay without Retry-After = %v, want 0", got)
	}
}

func TestVisibleFinalRepairNeedDependsOnVisibleTextAndToolCalls(t *testing.T) {
	if !NeedsVisibleFinalRepair("", 0) {
		t.Fatal("empty text without tool calls should require visible final repair")
	}
	if NeedsVisibleFinalRepair("", 1) {
		t.Fatal("tool calls should not require visible final repair")
	}
	if NeedsVisibleFinalRepair("visible", 0) {
		t.Fatal("visible text should not require visible final repair")
	}
}
