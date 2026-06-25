package toolruntime

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRequestInvalidProjectionJSONShape(t *testing.T) {
	payload, err := json.Marshal(RequestInvalidProjection{
		Status:   "tool_request_invalid",
		Tool:     "resource_read",
		Executed: false,
		Error: RequestError{
			Code:    "unknown_resource_ref",
			Message: "resource ref was not admitted",
		},
	})
	if err != nil {
		t.Fatalf("marshal RequestInvalidProjection: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"status":"tool_request_invalid"`,
		`"tool":"resource_read"`,
		`"executed":false`,
		`"code":"unknown_resource_ref"`,
		`"message":"resource ref was not admitted"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("request invalid payload = %s, want %s", text, want)
		}
	}
}

func TestOperationResultCarriesTerminalEquivalentFields(t *testing.T) {
	exitCode := 2
	payload, err := json.Marshal(OperationResult{
		Status:              "failed",
		Executed:            true,
		ExitCode:            &exitCode,
		ElapsedMs:           17,
		Stderr:              "syntax error",
		StderrTruncated:     true,
		StderrOriginalBytes: 80,
		StderrOmittedBytes:  32,
		OutputTruncation:    "head_tail",
	})
	if err != nil {
		t.Fatalf("marshal OperationResult: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"status":"failed"`,
		`"executed":true`,
		`"exit_code":2`,
		`"stderr":"syntax error"`,
		`"stderr_truncated":true`,
		`"output_truncation":"head_tail"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("operation result payload = %s, want %s", text, want)
		}
	}
}
