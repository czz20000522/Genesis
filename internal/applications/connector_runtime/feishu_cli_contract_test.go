package connectorruntime

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFeishuCLISendMessageShortcutDryRunContract(t *testing.T) {
	if os.Getenv("GENESIS_FEISHU_CLI_CONTRACT") != "1" {
		t.Skip("set GENESIS_FEISHU_CLI_CONTRACT=1 to verify the local lark-cli shortcut contract")
	}
	profile := strings.TrimSpace(os.Getenv("GENESIS_FEISHU_CLI_PROFILE"))
	if profile == "" {
		t.Fatal("GENESIS_FEISHU_CLI_PROFILE is required")
	}
	chatID := strings.TrimSpace(os.Getenv("GENESIS_FEISHU_CLI_CHAT_ID"))
	if chatID == "" {
		t.Fatal("GENESIS_FEISHU_CLI_CHAT_ID is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	driver := testFeishuCommandTemplateDriver(profile, OSCommandRunner{})
	action := testConnectorSendAction()
	action.TargetRef.ExternalID = chatID
	action.Payload["body"] = "Genesis connector dry-run contract"
	_, args, _, reason, renderErr := driver.render(action)
	if renderErr != nil {
		t.Fatalf("render failed with reason %s: %v", reason, renderErr)
	}
	args = append(args, "--dry-run")
	resolved, resolveErr := resolveCommandExecutable("lark-cli")
	if resolveErr != nil {
		t.Fatalf("lark-cli lookup failed: %v", resolveErr)
	}
	if unsafeResolvedCommandExecutable(resolved) {
		t.Skipf("lark-cli resolves to script wrapper %q; command_template requires a direct binary or connector_command adapter", resolved)
	}
	output, err := OSCommandRunner{}.Run(ctx, "lark-cli", args...)
	if err != nil {
		t.Fatalf("lark-cli dry-run failed: %v\n%s", err, safeCLIProbeExcerpt(output))
	}
	text := string(output)
	for _, want := range []string{"/open-apis/im/v1/messages", `"receive_id_type": "chat_id"`, `"msg_type": "text"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, safeCLIProbeExcerpt(output))
		}
	}
}

func TestSafeCLIProbeExcerptRedactsCredentialShapedOutput(t *testing.T) {
	got := safeCLIProbeExcerpt([]byte("Authorization: Bearer sk-secret\nplain line"))
	if strings.Contains(got, "Authorization") || strings.Contains(got, "sk-secret") {
		t.Fatalf("excerpt leaked credential-shaped output: %q", got)
	}
	if !strings.Contains(got, "plain line") {
		t.Fatalf("excerpt dropped non-secret diagnostic line: %q", got)
	}
}

func safeCLIProbeExcerpt(output []byte) string {
	const limit = 1024
	truncated := false
	if len(output) > limit {
		output = output[:limit]
		truncated = true
	}
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if isCredentialShapedExternalValue(line) {
			lines[i] = "[redacted credential-shaped CLI output]"
		}
	}
	text := strings.Join(lines, "\n")
	if truncated {
		text += "\n[truncated]"
	}
	return text
}
