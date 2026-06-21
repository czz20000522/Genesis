package kernel

import "regexp"

var evidenceRedactionRules = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)[^\s"']+`),
		replacement: `${1}[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9._~+/=-]{8,}`),
		replacement: `${1}[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:API[_-]?KEY|TOKEN|SECRET|PASSWORD)[A-Z0-9_]*\s*=\s*)[^\s"',;]+`),
		replacement: `${1}[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)("(?:api[_-]?key|token|secret|password|access[_-]?token|refresh[_-]?token|authorization)"\s*:\s*")[^"]+(")`),
		replacement: `${1}[REDACTED]${2}`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\b((?:api[_-]?key|token|secret|password|access[_-]?token|refresh[_-]?token)\s*[:=]\s*)[^\s"',;]+`),
		replacement: `${1}[REDACTED]`,
	},
}

func redactOperationEvidence(operation OperationProjection) OperationProjection {
	operation.Command = redactEvidenceText(operation.Command)
	operation.Stdout = redactEvidenceText(operation.Stdout)
	operation.Stderr = redactEvidenceText(operation.Stderr)
	return operation
}

func redactEvidenceText(text string) string {
	for _, rule := range evidenceRedactionRules {
		text = rule.pattern.ReplaceAllString(text, rule.replacement)
	}
	return text
}
