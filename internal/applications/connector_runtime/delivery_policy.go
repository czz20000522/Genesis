package connectorruntime

import "time"

const (
	defaultMaxDeliveryAttempts = 3
	defaultRetryBaseDelay      = 30 * time.Second
	defaultRetryMaxDelay       = 5 * time.Minute
	defaultDeliveryLeaseOwner  = "connector-runtime"
	defaultDeliveryLeaseTTL    = 2 * time.Minute
)

func normalizeDeliveryResult(item ConnectorOutboxItem, result ConnectorActionResult, now time.Time) ConnectorActionResult {
	attempt := item.AttemptCount + 1
	switch result.Status {
	case DeliveryStatusRetrying:
		if attempt >= defaultMaxDeliveryAttempts {
			result.Status = DeliveryStatusDeadLettered
			result.NextAttemptAt = time.Time{}
			if result.Reason == "" {
				result.Reason = "retry_exhausted"
			}
			return result
		}
		if result.NextAttemptAt.IsZero() {
			result.NextAttemptAt = defaultNextAttemptAt(now, attempt)
		}
	case DeliveryStatusFailed:
		result.Status = DeliveryStatusDeadLettered
		if result.Reason == "" {
			result.Reason = "delivery_failed"
		}
	case DeliveryStatusPartialSuccess, DeliveryStatusAmbiguous:
		if result.Reason == "" {
			result.Reason = "recovery_required"
		}
	}
	return result
}

func defaultNextAttemptAt(now time.Time, attempt int) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	if attempt < 1 {
		attempt = 1
	}
	delay := defaultRetryBaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= defaultRetryMaxDelay {
			delay = defaultRetryMaxDelay
			break
		}
	}
	return now.Add(delay)
}
