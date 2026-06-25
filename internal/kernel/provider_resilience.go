package kernel

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	maxProviderTransientAttempts     = 3
	maxProviderVisibleFinalRepairs   = 2
	providerVisibleFinalRepairPrompt = "The previous provider response did not include a visible final answer. Return a concise visible final answer to the user now. Do not include hidden reasoning."
)

var ErrProviderVisibleFinalRequired = errors.New("provider visible final required")

type ProviderClassifiedError struct {
	Code       string
	Message    string
	Retryable  bool
	StatusCode int
	RetryAfter time.Duration
	Err        error
}

func (e *ProviderClassifiedError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *ProviderClassifiedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newProviderStatusError(statusCode int, body string, retryAfter time.Duration) error {
	code := "provider_http_error"
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		code = "provider_auth_failed"
	case statusCode == http.StatusPaymentRequired:
		code = "provider_quota_or_billing_failed"
	case statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests || (statusCode >= 500 && statusCode <= 599):
		code = "provider_transient_failure"
	case statusCode >= 400 && statusCode <= 499:
		code = "provider_request_rejected"
	}
	message := fmt.Sprintf("provider returned status %d", statusCode)
	if trimmed := strings.TrimSpace(body); trimmed != "" {
		message += ": " + trimmed
	}
	return &ProviderClassifiedError{
		Code:       code,
		Message:    message,
		Retryable:  isRetryableProviderStatus(statusCode),
		StatusCode: statusCode,
		RetryAfter: retryAfter,
	}
}

func newProviderTransportError(err error) error {
	return &ProviderClassifiedError{
		Code:      "provider_transient_failure",
		Message:   fmt.Sprintf("provider request failed: %v", err),
		Retryable: true,
		Err:       err,
	}
}

func newProviderVisibleFinalRequiredError() error {
	return &ProviderClassifiedError{
		Code:      "provider_visible_final_required",
		Message:   "provider returned no visible assistant content",
		Retryable: false,
		Err:       ErrProviderVisibleFinalRequired,
	}
}

func isRetryableProviderStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests || (statusCode >= 500 && statusCode <= 599)
}

func providerFailureFromError(err error) ProviderAttemptProjection {
	var classified *ProviderClassifiedError
	if errors.As(err, &classified) {
		return ProviderAttemptProjection{
			Status:     "failed",
			ReasonCode: classified.Code,
			Message:    redactEvidenceText(classified.Error()),
			Retryable:  classified.Retryable,
		}
	}
	if errors.Is(err, ErrProviderUnavailable) {
		return ProviderAttemptProjection{
			Status:     "failed",
			ReasonCode: "provider_unavailable",
			Message:    redactEvidenceText(err.Error()),
		}
	}
	return ProviderAttemptProjection{
		Status:     "failed",
		ReasonCode: "provider_error",
		Message:    redactEvidenceText(err.Error()),
	}
}

func providerRetryDelay(err error) time.Duration {
	var classified *ProviderClassifiedError
	if errors.As(err, &classified) && classified.RetryAfter > 0 {
		if classified.RetryAfter > 2*time.Second {
			return 2 * time.Second
		}
		return classified.RetryAfter
	}
	return 0
}

func modelResponseNeedsVisibleFinalRepair(resp ModelResponse) bool {
	return len(resp.ToolCalls) == 0 && strings.TrimSpace(resp.Text) == ""
}
