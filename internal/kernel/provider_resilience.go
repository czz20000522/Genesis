package kernel

import (
	"time"

	"genesis/internal/kernel/modelgateway"
)

const (
	maxProviderTransientAttempts     = modelgateway.MaxProviderTransientAttempts
	maxProviderVisibleFinalRepairs   = modelgateway.MaxProviderVisibleFinalRepairs
	providerVisibleFinalRepairPrompt = modelgateway.VisibleFinalRepairPrompt
)

var ErrProviderVisibleFinalRequired = modelgateway.ErrVisibleFinalRequired

type ProviderClassifiedError = modelgateway.ClassifiedError

func newProviderStatusError(statusCode int, body string, retryAfter time.Duration) error {
	return modelgateway.NewStatusError(statusCode, body, retryAfter)
}

func newProviderTransportError(err error) error {
	return modelgateway.NewTransportError(err)
}

func newProviderVisibleFinalRequiredError() error {
	return modelgateway.NewVisibleFinalRequiredError()
}

func providerFailureFromError(err error) ProviderAttemptProjection {
	return modelgateway.FailureClassifier{
		ProviderUnavailable: ErrProviderUnavailable,
		Redact:              redactEvidenceText,
	}.FromError(err)
}

func providerRetryDelay(err error) time.Duration {
	return modelgateway.RetryDelay(err)
}

func modelResponseNeedsVisibleFinalRepair(resp ModelResponse) bool {
	return modelgateway.NeedsVisibleFinalRepair(resp.Text, len(resp.ToolCalls))
}
