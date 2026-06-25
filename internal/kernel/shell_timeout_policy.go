package kernel

import "fmt"

const (
	defaultShellTimeoutSec       = 30
	maxForegroundShellTimeoutSec = 180
)

func normalizedShellTimeoutPolicy(policy ShellTimeoutPolicy) ShellTimeoutPolicy {
	defaultTimeout := policy.DefaultForegroundTimeoutSec
	if defaultTimeout <= 0 {
		defaultTimeout = defaultShellTimeoutSec
	}
	foregroundCap := policy.ForegroundTimeoutCapSec
	if foregroundCap <= 0 {
		foregroundCap = maxForegroundShellTimeoutSec
	}
	if defaultTimeout > foregroundCap {
		defaultTimeout = foregroundCap
	}
	managedThreshold := policy.ManagedJobThresholdSec
	if managedThreshold <= 0 || managedThreshold != foregroundCap {
		managedThreshold = foregroundCap
	}
	return ShellTimeoutPolicy{
		DefaultForegroundTimeoutSec: defaultTimeout,
		ForegroundTimeoutCapSec:     foregroundCap,
		ManagedJobThresholdSec:      managedThreshold,
	}
}

func (k *Kernel) normalizedShellTimeoutSec(timeoutSec int) int {
	if timeoutSec == 0 {
		return k.shellTimeoutPolicy.DefaultForegroundTimeoutSec
	}
	return timeoutSec
}

func (k *Kernel) shellTimeoutExceedsForeground(timeoutSec int) bool {
	return timeoutSec > k.shellTimeoutPolicy.ManagedJobThresholdSec
}

func shellTimeoutDescription(policy ShellTimeoutPolicy) string {
	policy = normalizedShellTimeoutPolicy(policy)
	return fmt.Sprintf(
		"Foreground timeout in seconds. Omit for %d seconds. Values above %d are accepted as managed-job intent.",
		policy.DefaultForegroundTimeoutSec,
		policy.ManagedJobThresholdSec,
	)
}

func normalizedShellTimeoutSec(timeoutSec int) int {
	if timeoutSec == 0 {
		return defaultShellTimeoutSec
	}
	return timeoutSec
}
