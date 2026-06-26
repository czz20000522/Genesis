package connectorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	ProfileReadinessOK                  = "ok"
	defaultProfileReadinessProbeTimeout = 5 * time.Second
)

type ProfileReadinessCommandProbe struct {
	Executable string
	Args       []string
	Runner     CommandRunner
	Timeout    time.Duration
}

type ProfileReadinessCommandResult struct {
	Readiness string `json:"readiness,omitempty"`
	Ready     *bool  `json:"ready,omitempty"`
	Status    string `json:"status,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func ResolveProfileReadiness(ctx context.Context, profile string, staticReadiness string, probe ProfileReadinessCommandProbe) (string, error) {
	blockReason, err := StaticProfileReadinessBlockReason(profile, staticReadiness)
	if err != nil || blockReason != "" {
		return blockReason, err
	}
	if strings.TrimSpace(probe.Executable) == "" {
		return "", nil
	}
	return probe.Probe(ctx, profile)
}

func StaticProfileReadinessBlockReason(profile string, readiness string) (string, error) {
	readiness = strings.TrimSpace(readiness)
	if readiness == "" || readiness == ProfileReadinessOK {
		if strings.TrimSpace(profile) == "" {
			return SourceReadinessReasonMissingProfile, nil
		}
		return "", nil
	}
	if !ValidSourceReadinessReasonCode(readiness) {
		return "", fmt.Errorf("profile readiness must be ok or a known source readiness reason")
	}
	return readiness, nil
}

func (p ProfileReadinessCommandProbe) Probe(ctx context.Context, profile string) (string, error) {
	if strings.TrimSpace(profile) == "" {
		return SourceReadinessReasonMissingProfile, nil
	}
	executable := strings.TrimSpace(p.Executable)
	if executable == "" {
		return "", nil
	}
	runner := p.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	args := append([]string(nil), p.Args...)
	if profileProbeArgsContainProfile(args) {
		return SourceReadinessReasonOperatorActionRequired, errors.New("profile readiness command arguments must not provide profile")
	}
	args = append(args, "--profile", strings.TrimSpace(profile))
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultProfileReadinessProbeTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, err := runner.Run(runCtx, executable, args...)
	if IsConnectorCommandOutputExceeded(err) {
		return SourceReadinessReasonOperatorActionRequired, nil
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return SourceReadinessReasonOperatorActionRequired, nil
	}
	if err != nil {
		return SourceReadinessReasonOperatorActionRequired, nil
	}
	blockReason, err := DecodeProfileReadinessCommandResult(output)
	if err != nil {
		return SourceReadinessReasonOperatorActionRequired, err
	}
	return blockReason, nil
}

func profileProbeArgsContainProfile(args []string) bool {
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "--profile" || strings.HasPrefix(arg, "--profile=") {
			return true
		}
	}
	return false
}

func DecodeProfileReadinessCommandResult(output []byte) (string, error) {
	if strings.TrimSpace(string(output)) == "" {
		return "", errors.New("profile readiness command returned empty result")
	}
	var result ProfileReadinessCommandResult
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return "", fmt.Errorf("decode profile readiness command result: %w", err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return "", errors.New("profile readiness command returned multiple JSON values")
	}
	readiness := strings.TrimSpace(result.Readiness)
	if readiness != "" {
		return readinessBlockReasonFromCommand(readiness)
	}
	reason := strings.TrimSpace(result.Reason)
	status := strings.TrimSpace(result.Status)
	if result.Ready != nil && *result.Ready {
		return "", nil
	}
	if result.Ready != nil && !*result.Ready {
		if reason != "" {
			return readinessBlockReasonFromCommand(reason)
		}
		if status != "" {
			return readinessBlockReasonFromStatus(status)
		}
		return SourceReadinessReasonOperatorActionRequired, nil
	}
	if reason != "" {
		return readinessBlockReasonFromCommand(reason)
	}
	return readinessBlockReasonFromStatus(status)
}

func readinessBlockReasonFromStatus(status string) (string, error) {
	switch strings.TrimSpace(status) {
	case "", ProfileReadinessOK:
		return "", nil
	case ProbeStatusFailed:
		return SourceReadinessReasonOperatorActionRequired, nil
	default:
		return readinessBlockReasonFromCommand(status)
	}
}

func readinessBlockReasonFromCommand(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == ProfileReadinessOK || value == ProbeStatusOK {
		return "", nil
	}
	if !ValidSourceReadinessReasonCode(value) {
		return "", fmt.Errorf("profile readiness command returned unsupported reason %q", value)
	}
	return value, nil
}
