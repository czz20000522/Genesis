package kernel

import "regexp"

var credentialShapedTextRules = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_-]{6,}\b`),
		replacement: `[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`),
		replacement: `[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)(authorization[\t\r\n ]*:[\t\r\n ]*bearer[\t\r\n ]+)[^\t\r\n "']+`),
		replacement: `${1}[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\b(bearer[\t\r\n ]+)[A-Za-z0-9._~+/=-]{8,}`),
		replacement: `${1}[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:API[_-]?KEY|TOKEN|SECRET|PASSWORD)[A-Z0-9_]*[\t\r\n ]*=[\t\r\n ]*)[^\t\r\n "',;]+`),
		replacement: `${1}[REDACTED]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)("(?:api[_-]?key|token|secret|password|access[_-]?token|refresh[_-]?token|authorization)"[\t\r\n ]*:[\t\r\n ]*")[^"]+(")`),
		replacement: `${1}[REDACTED]${2}`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\b((?:api[_-]?key|token|secret|password|access[_-]?token|refresh[_-]?token)[\t\r\n ]*[:=][\t\r\n ]*)[^\t\r\n "',;]+`),
		replacement: `${1}[REDACTED]`,
	},
}

func localOperationProjection(operation OperationProjection) OperationProjection {
	return operation
}

func containsCredentialShapedText(text string) bool {
	for _, rule := range credentialShapedTextRules {
		if rule.pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func externalBoundaryDiagnosticText(text string) string {
	for _, rule := range credentialShapedTextRules {
		text = rule.pattern.ReplaceAllString(text, rule.replacement)
	}
	return text
}
