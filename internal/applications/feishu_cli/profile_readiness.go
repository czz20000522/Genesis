package feishucli

import (
	"encoding/json"
	"strings"

	connectorruntime "genesis/internal/applications/connector_runtime"
)

type authStatusResponse struct {
	Error struct {
		Subtype string `json:"subtype"`
		Type    string `json:"type"`
	} `json:"error"`
	Identities map[string]struct {
		Available bool   `json:"available"`
		Status    string `json:"status"`
	} `json:"identities"`
}

func ProfileReadinessFromAuthStatus(output []byte, commandErr error) connectorruntime.ProfileReadinessCommandResult {
	ready := false
	result := connectorruntime.ProfileReadinessCommandResult{Ready: &ready, Reason: connectorruntime.SourceReadinessReasonOperatorActionRequired}
	if commandErr != nil {
		return result
	}
	var status authStatusResponse
	if json.Unmarshal(output, &status) != nil {
		return result
	}
	if strings.EqualFold(strings.TrimSpace(status.Error.Type), "config") && strings.EqualFold(strings.TrimSpace(status.Error.Subtype), "not_configured") {
		result.Reason = connectorruntime.SourceReadinessReasonMissingProfile
		return result
	}
	bot, ok := status.Identities["bot"]
	if !ok || !bot.Available || !strings.EqualFold(strings.TrimSpace(bot.Status), "ready") {
		return result
	}
	ready = true
	return connectorruntime.ProfileReadinessCommandResult{Ready: &ready}
}
