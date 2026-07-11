package feishucli

import (
	"errors"
	"testing"

	connectorruntime "genesis/internal/applications/connector_runtime"
)

func TestProfileReadinessFromAuthStatusClassifiesOnlyProvenStates(t *testing.T) {
	tests := []struct {
		name   string
		output string
		err    error
		ready  bool
		reason string
	}{
		{
			name:   "ready bot identity",
			output: `{"appId":"cli","identities":{"bot":{"status":"ready","available":true}}}`,
			ready:  true,
		},
		{
			name:   "structured missing profile",
			output: `{"ok":false,"error":{"type":"config","subtype":"not_configured"}}`,
			reason: connectorruntime.SourceReadinessReasonMissingProfile,
		},
		{
			name:   "unknown bot state fails closed",
			output: `{"identities":{"bot":{"status":"expired","available":false}}}`,
			reason: connectorruntime.SourceReadinessReasonOperatorActionRequired,
		},
		{
			name:   "command failure fails closed",
			err:    errors.New("lark cli unavailable"),
			reason: connectorruntime.SourceReadinessReasonOperatorActionRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ProfileReadinessFromAuthStatus([]byte(test.output), test.err)
			if result.Ready == nil || *result.Ready != test.ready {
				t.Fatalf("ready = %+v, want %v", result.Ready, test.ready)
			}
			if result.Reason != test.reason {
				t.Fatalf("reason = %q, want %q", result.Reason, test.reason)
			}
		})
	}
}
