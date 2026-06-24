package connectorruntime

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
