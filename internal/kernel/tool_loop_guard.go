package kernel

import (
	"encoding/json"
	"strconv"
	"strings"
)

const (
	toolLoopRepeatedFailureThreshold = 3
	toolLoopRepeatedSuccessThreshold = 2
)

type toolLoopGuard struct {
	failureSignature     string
	failureCount         int
	repeatedSuccessCount map[string]int
}

type toolLoopGuardProjection struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	RepeatCount   int    `json:"repeat_count"`
	SignatureKind string `json:"signature_kind"`
}

type toolLoopGuardedToolResult struct {
	Status    string                  `json:"status"`
	Tool      string                  `json:"tool"`
	Executed  bool                    `json:"executed"`
	Error     ToolRequestError        `json:"error"`
	LoopGuard toolLoopGuardProjection `json:"loop_guard"`
}

func newToolLoopGuard() *toolLoopGuard {
	return &toolLoopGuard{repeatedSuccessCount: map[string]int{}}
}

func (g *toolLoopGuard) beforeExecute(call preparedModelToolCall) (ModelToolResult, bool, error) {
	if g == nil {
		return ModelToolResult{}, false, nil
	}
	signature := strings.TrimSpace(call.repeatSuccessSignature)
	if signature == "" {
		return ModelToolResult{}, false, nil
	}
	count := g.repeatedSuccessCount[signature]
	if count < toolLoopRepeatedSuccessThreshold {
		return ModelToolResult{}, false, nil
	}
	content, err := json.Marshal(toolLoopGuardedToolResult{
		Status:   "tool_loop_guarded",
		Tool:     strings.TrimSpace(call.name),
		Executed: false,
		Error: ToolRequestError{
			Code:    "repeated_write_success",
			Message: "tool loop guard blocked a repeated successful write-like effect before execution",
		},
		LoopGuard: toolLoopGuardProjection{
			Code:          "repeated_write_success",
			Message:       "this write-like tool call already succeeded with the same effect signature in this turn; change approach, verify, or produce a final answer",
			RepeatCount:   count,
			SignatureKind: "write_like_success",
		},
	})
	if err != nil {
		return ModelToolResult{}, false, err
	}
	return ModelToolResult{
		ToolCallID:      strings.TrimSpace(call.providerCallID),
		ToolCallEventID: strings.TrimSpace(call.eventID),
		Name:            strings.TrimSpace(call.name),
		Content:         string(content),
	}, true, nil
}

func (g *toolLoopGuard) afterExecute(call preparedModelToolCall, result ModelToolResult) (ModelToolResult, error) {
	if g == nil {
		return result, nil
	}
	result, err := g.observeFailure(result)
	if err != nil {
		return ModelToolResult{}, err
	}
	g.observeSuccess(call, result)
	return result, nil
}

func (g *toolLoopGuard) observeFailure(result ModelToolResult) (ModelToolResult, error) {
	signature, ok := toolLoopFailureSignature(result)
	if !ok {
		g.failureSignature = ""
		g.failureCount = 0
		return result, nil
	}
	if signature != g.failureSignature {
		g.failureSignature = signature
		g.failureCount = 1
		return result, nil
	}
	g.failureCount++
	if g.failureCount < toolLoopRepeatedFailureThreshold {
		return result, nil
	}
	return resultWithFailureLoopGuard(result, g.failureCount)
}

func (g *toolLoopGuard) observeSuccess(call preparedModelToolCall, result ModelToolResult) {
	if !toolLoopResultSucceeded(result) {
		return
	}
	signature := strings.TrimSpace(call.repeatSuccessSignature)
	if signature == "" {
		g.repeatedSuccessCount = map[string]int{}
		return
	}
	if !toolLoopSuccessCountsOnlyTrackSignature(g.repeatedSuccessCount, signature) {
		g.repeatedSuccessCount = map[string]int{}
	}
	g.repeatedSuccessCount[signature]++
}

func toolLoopSuccessCountsOnlyTrackSignature(counts map[string]int, signature string) bool {
	for tracked := range counts {
		if tracked != signature {
			return false
		}
	}
	return true
}

func shellExecRepeatSuccessSignature(cwd string, command string, timeoutSec int) string {
	if timeoutSec > maxForegroundShellTimeoutSec {
		return ""
	}
	cwd = strings.TrimSpace(cwd)
	command = normalizeToolLoopSignatureText(command)
	if cwd == "" || command == "" {
		return ""
	}
	return "shell_exec" + "\x00" + cwd + "\x00" + command
}

func normalizeToolLoopSignatureText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func toolLoopFailureSignature(result ModelToolResult) (string, bool) {
	payload, ok := decodeToolLoopResultPayload(result.Content)
	if !ok {
		return "", false
	}
	status := stringMapValue(payload, "status")
	if status == "" || toolLoopStatusSucceeded(status) || toolLoopStatusBlocked(status) {
		return "", false
	}
	code := ""
	if errPayload, ok := payload["error"].(map[string]interface{}); ok {
		code = stringMapValue(errPayload, "code")
	}
	if code == "" {
		code = stringMapValue(payload, "timeout_reason")
	}
	if code == "" {
		if exitCode, ok := numericMapValue(payload, "exit_code"); ok {
			code = "exit_code:" + strconv.Itoa(exitCode)
		}
	}
	if code == "" {
		code = "unclassified_failure"
	}
	return strings.TrimSpace(result.Name) + "\x00" + status + "\x00" + code, true
}

func resultWithFailureLoopGuard(result ModelToolResult, repeatCount int) (ModelToolResult, error) {
	payload, ok := decodeToolLoopResultPayload(result.Content)
	if !ok {
		return result, nil
	}
	payload["loop_guard"] = map[string]interface{}{
		"code":           "repeated_tool_failure",
		"message":        "this tool call or batch has failed repeatedly with the same error in this turn; change approach, reduce arguments, use a different tool, or produce a final answer explaining the blocker",
		"repeat_count":   repeatCount,
		"signature_kind": "tool_failure",
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return ModelToolResult{}, err
	}
	result.Content = string(content)
	return result, nil
}

func toolLoopResultSucceeded(result ModelToolResult) bool {
	payload, ok := decodeToolLoopResultPayload(result.Content)
	if !ok {
		return false
	}
	return toolLoopStatusSucceeded(stringMapValue(payload, "status"))
}

func toolLoopStatusSucceeded(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "managed_job_started":
		return true
	default:
		return false
	}
}

func toolLoopStatusBlocked(status string) bool {
	switch strings.TrimSpace(status) {
	case "permission_denied", "approval_required", "sandbox_profile_unavailable", "approval_policy_invalid":
		return true
	default:
		return false
	}
}

func decodeToolLoopResultPayload(content string) (map[string]interface{}, bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func stringMapValue(payload map[string]interface{}, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func numericMapValue(payload map[string]interface{}, key string) (int, bool) {
	switch value := payload[key].(type) {
	case float64:
		return int(value), true
	case int:
		return value, true
	default:
		return 0, false
	}
}
