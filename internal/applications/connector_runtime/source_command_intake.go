package connectorruntime

import (
	"context"
	"errors"
	"time"
)

const maxSourceCommandBackoff = 30 * time.Second

var (
	ErrSourceCommandBlocked       = errors.New("source command blocked")
	ErrSourceCommandRuntimeFailed = errors.New("source command runtime failed")
	errSourceCommandHandlerFailed = errors.New("source command handler failed")
)

type SourceCommandRetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

type SourceCommandIntake struct {
	Adapter SourceCommandAdapter
	Retry   SourceCommandRetryPolicy
	Sleep   func(context.Context, time.Duration) error
}

func (s SourceCommandIntake) Run(ctx context.Context, handle func(ExternalEvent) error) error {
	policy := normalizeSourceCommandRetryPolicy(s.Retry)
	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		err := s.Adapter.Consume(ctx, handle)
		if err == nil {
			return nil
		}
		lastErr = err
		if !errors.Is(err, ErrSourceCommandRuntimeFailed) || attempt == policy.MaxAttempts {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if delay := sourceCommandBackoffDelay(policy, attempt); delay > 0 {
			sleep := s.Sleep
			if sleep == nil {
				sleep = sleepSourceCommandBackoff
			}
			if err := sleep(ctx, delay); err != nil {
				return err
			}
		}
	}
	return lastErr
}

func normalizeSourceCommandRetryPolicy(policy SourceCommandRetryPolicy) SourceCommandRetryPolicy {
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = 1
	}
	if policy.Backoff < 0 {
		policy.Backoff = 0
	}
	return policy
}

func sourceCommandBackoffDelay(policy SourceCommandRetryPolicy, failedAttempt int) time.Duration {
	if policy.Backoff <= 0 || failedAttempt < 1 {
		return 0
	}
	delay := policy.Backoff
	for i := 1; i < failedAttempt; i++ {
		if delay >= maxSourceCommandBackoff/2 {
			return maxSourceCommandBackoff
		}
		delay *= 2
	}
	if delay > maxSourceCommandBackoff {
		return maxSourceCommandBackoff
	}
	return delay
}

func sourceCommandBlockedError(err error) error {
	if err == nil || errors.Is(err, ErrSourceCommandBlocked) {
		return err
	}
	return errors.Join(ErrSourceCommandBlocked, err)
}

func sourceCommandRuntimeError(err error) error {
	if err == nil || errors.Is(err, ErrSourceCommandRuntimeFailed) {
		return err
	}
	return errors.Join(ErrSourceCommandRuntimeFailed, err)
}

func sourceCommandHandlerError(err error) error {
	if err == nil || errors.Is(err, errSourceCommandHandlerFailed) {
		return err
	}
	return errors.Join(errSourceCommandHandlerFailed, err)
}

func sleepSourceCommandBackoff(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
