package connectorruntime

// NewFeishuSendMessageCommandTemplateDriver is retained only for explicit local
// smoke checks of the lark-cli shortcut. Production Feishu delivery must use
// ConnectorCommandAdapter with an external Feishu connector adapter process.
func NewFeishuSendMessageCommandTemplateDriver(profile string, executable string, runner CommandRunner) CommandTemplateDriver {
	return CommandTemplateDriver{
		Executable: SelectFeishuCLIExecutable(executable, InstalledOfficialLarkCLIExecutable()),
		Profile:    profile,
		Runner:     runner,
		Actions: map[string]CommandTemplateAction{
			"send_message": {
				Argv: []string{
					"--profile", "${profile}",
					"im", "+messages-send",
					"--as", "bot",
					"--chat-id", "${target.external_id}",
					"--text", "${payload.body}",
					"--idempotency-key", "${idempotency_key}",
				},
				ExternalActionRefJSONPaths: []string{"data.message_id", "message_id"},
			},
		},
	}
}
