package modelgateway

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	MaxProviderTransientAttempts   = 3
	MaxProviderVisibleFinalRepairs = 2
	VisibleFinalRepairPrompt       = "The previous provider response did not include a visible final answer. Return a concise visible final answer to the user now. Do not include hidden reasoning."
)

var ErrVisibleFinalRequired = errors.New("provider visible final required")

type AttemptProjection struct {
	RoundIndex  int    `json:"round_index"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	Status      string `json:"status"`
	ReasonCode  string `json:"reason_code,omitempty"`
	Message     string `json:"message,omitempty"`
	Retryable   bool   `json:"retryable,omitempty"`
	RepairKind  string `json:"repair_kind,omitempty"`
}

type ClassifiedError struct {
	Code       string
	Message    string
	Retryable  bool
	StatusCode int
	RetryAfter time.Duration
	Err        error
}

func (e *ClassifiedError) Error() string {
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

func (e *ClassifiedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type FailureClassifier struct {
	ProviderUnavailable error
	Redact              func(string) string
}

func (c FailureClassifier) FromError(err error) AttemptProjection {
	var classified *ClassifiedError
	if errors.As(err, &classified) {
		return AttemptProjection{
			Status:     "failed",
			ReasonCode: classified.Code,
			Message:    c.redact(classified.Error()),
			Retryable:  classified.Retryable,
		}
	}
	if c.ProviderUnavailable != nil && errors.Is(err, c.ProviderUnavailable) {
		return AttemptProjection{
			Status:     "failed",
			ReasonCode: "provider_unavailable",
			Message:    c.redact(err.Error()),
		}
	}
	return AttemptProjection{
		Status:     "failed",
		ReasonCode: "provider_error",
		Message:    c.redact(err.Error()),
	}
}

func (c FailureClassifier) redact(text string) string {
	if c.Redact == nil {
		return text
	}
	return c.Redact(text)
}

func NewStatusError(statusCode int, body string, retryAfter time.Duration) error {
	code := "provider_http_error"
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		code = "provider_auth_failed"
	case statusCode == http.StatusPaymentRequired:
		code = "provider_quota_or_billing_failed"
	case IsRetryableStatus(statusCode):
		code = "provider_transient_failure"
	case statusCode >= 400 && statusCode <= 499:
		code = "provider_request_rejected"
	}
	message := fmt.Sprintf("provider returned status %d", statusCode)
	if trimmed := strings.TrimSpace(body); trimmed != "" {
		message += ": " + trimmed
	}
	return &ClassifiedError{
		Code:       code,
		Message:    message,
		Retryable:  IsRetryableStatus(statusCode),
		StatusCode: statusCode,
		RetryAfter: retryAfter,
	}
}

func NewTransportError(err error) error {
	return &ClassifiedError{
		Code:      "provider_transient_failure",
		Message:   fmt.Sprintf("provider request failed: %v", err),
		Retryable: true,
		Err:       err,
	}
}

func NewVisibleFinalRequiredError() error {
	return &ClassifiedError{
		Code:      "provider_visible_final_required",
		Message:   "provider returned no visible assistant content",
		Retryable: false,
		Err:       ErrVisibleFinalRequired,
	}
}

func IsRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests || (statusCode >= 500 && statusCode <= 599)
}

func RetryDelay(err error) time.Duration {
	var classified *ClassifiedError
	if errors.As(err, &classified) && classified.RetryAfter > 0 {
		if classified.RetryAfter > 2*time.Second {
			return 2 * time.Second
		}
		return classified.RetryAfter
	}
	return 0
}

func NeedsVisibleFinalRepair(text string, toolCallCount int) bool {
	return toolCallCount == 0 && strings.TrimSpace(text) == ""
}
