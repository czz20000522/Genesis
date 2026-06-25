package connectorruntime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProfileReadinessCommandProbeClassifiesKnownReasons(t *testing.T) {
	runner := &profileProbeRunner{output: []byte(`{"readiness":"refresh_required"}`)}

	reason, err := ResolveProfileReadiness(context.Background(), "genesis", "ok", ProfileReadinessCommandProbe{
		Executable: "profile-probe",
		Args:       []string{"--mode", "readiness"},
		Runner:     runner,
	})
	if err != nil {
		t.Fatalf("ResolveProfileReadiness returned error: %v", err)
	}
	if reason != SourceReadinessReasonRefreshRequired {
		t.Fatalf("reason = %q, want refresh_required", reason)
	}
	if runner.name != "profile-probe" || strings.Join(runner.args, "\x00") != "--mode\x00readiness\x00--profile\x00genesis" {
		t.Fatalf("command = %q %#v, want configured args plus generated profile", runner.name, runner.args)
	}
}

func TestProfileReadinessCommandProbeRejectsUnsupportedReason(t *testing.T) {
	runner := &profileProbeRunner{output: []byte(`{"readiness":"trusted"}`)}

	reason, err := ResolveProfileReadiness(context.Background(), "genesis", "ok", ProfileReadinessCommandProbe{
		Executable: "profile-probe",
		Runner:     runner,
	})
	if err == nil {
		t.Fatal("ResolveProfileReadiness should reject unsupported readiness")
	}
	if reason != SourceReadinessReasonOperatorActionRequired {
		t.Fatalf("reason = %q, want operator_action_required fail-closed reason", reason)
	}
}

func TestProfileReadinessStaticBlockSkipsCommand(t *testing.T) {
	runner := &profileProbeRunner{output: []byte(`{"readiness":"ok"}`)}

	reason, err := ResolveProfileReadiness(context.Background(), "genesis", SourceReadinessReasonPermissionDenied, ProfileReadinessCommandProbe{
		Executable: "profile-probe",
		Runner:     runner,
	})
	if err != nil {
		t.Fatalf("ResolveProfileReadiness returned error: %v", err)
	}
	if reason != SourceReadinessReasonPermissionDenied {
		t.Fatalf("reason = %q, want static permission_denied", reason)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want static fail-closed without command", runner.calls)
	}
}

func TestProfileReadinessProbeRejectsCallerProvidedProfileArgument(t *testing.T) {
	runner := &profileProbeRunner{output: []byte(`{"readiness":"ok"}`)}

	reason, err := ResolveProfileReadiness(context.Background(), "genesis", "ok", ProfileReadinessCommandProbe{
		Executable: "profile-probe",
		Args:       []string{"--profile", "codex"},
		Runner:     runner,
	})
	if err == nil {
		t.Fatal("ResolveProfileReadiness should reject caller-provided profile arguments")
	}
	if reason != SourceReadinessReasonOperatorActionRequired {
		t.Fatalf("reason = %q, want operator_action_required", reason)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want rejection before command execution", runner.calls)
	}
}

func TestProfileReadinessCommandFailureFailsClosed(t *testing.T) {
	runner := &profileProbeRunner{err: errors.New("profile backend unavailable")}

	reason, err := ResolveProfileReadiness(context.Background(), "genesis", "ok", ProfileReadinessCommandProbe{
		Executable: "profile-probe",
		Runner:     runner,
	})
	if err != nil {
		t.Fatalf("ResolveProfileReadiness returned error: %v", err)
	}
	if reason != SourceReadinessReasonOperatorActionRequired {
		t.Fatalf("reason = %q, want operator_action_required", reason)
	}
}

type profileProbeRunner struct {
	name   string
	args   []string
	output []byte
	err    error
	calls  int
}

func (r *profileProbeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls++
	r.name = name
	r.args = append([]string(nil), args...)
	return append([]byte(nil), r.output...), r.err
}
