package kernel

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrIngressSecurityBlocked = errors.New("ingress security blocked turn")

type IngressSecurityError struct {
	Category string
	RuleID   string
}

func (e IngressSecurityError) Error() string {
	return fmt.Sprintf("%s: %s:%s", ErrIngressSecurityBlocked.Error(), e.Category, e.RuleID)
}

func (e IngressSecurityError) Unwrap() error {
	return ErrIngressSecurityBlocked
}

type ingressSecurityRule struct {
	id       string
	category string
	pattern  *regexp.Regexp
}

type IngressRisk struct {
	Category string `json:"category"`
	RuleID   string `json:"rule_id"`
}

var ingressSecurityRules = []ingressSecurityRule{
	{
		id:       "ignore_prior_instructions",
		category: "prompt_injection",
		pattern:  regexp.MustCompile(`(?i)\b(ignore|disregard|forget)\s+(all\s+)?(previous|prior|above|earlier)\s+(instructions|rules|system|developer)\b`),
	},
	{
		id:       "override_authority",
		category: "prompt_injection",
		pattern:  regexp.MustCompile(`(?i)\b(override|bypass|disable)\s+(the\s+)?(system|developer|safety|policy|guardrails?)\b`),
	},
	{
		id:       "role_block_marker",
		category: "authority_forgery",
		pattern:  regexp.MustCompile(`(?im)(^|\n)\s*(#+\s*)?(system|developer|tool|assistant)\s*:\s*`),
	},
	{
		id:       "xml_role_marker",
		category: "authority_forgery",
		pattern:  regexp.MustCompile(`(?i)</?\s*(system|developer|tool|tool_call|function_call)\s*>`),
	},
	{
		id:       "json_role_marker",
		category: "authority_forgery",
		pattern:  regexp.MustCompile(`(?i)"role"\s*:\s*"(system|developer|tool)"`),
	},
	{
		id:       "tool_call_marker",
		category: "authority_forgery",
		pattern:  regexp.MustCompile(`(?i)\b(tool_call_id|function_call|tool_calls)\b`),
	},
	{
		id:       "system_prompt_exfiltration",
		category: "prompt_exfiltration",
		pattern:  regexp.MustCompile(`(?i)\b(reveal|print|dump|show)\s+(the\s+)?(system|developer)\s+(prompt|message|instructions)\b`),
	},
}

func scanTurnIngressSecurity(items []InputItem) ([]IngressRisk, error) {
	var risks []IngressRisk
	for i, item := range items {
		text := item.Text
		if hasInvisibleControlMarker(text) {
			return nil, IngressSecurityError{Category: "hidden_text", RuleID: fmt.Sprintf("invisible_control:item_%d", i)}
		}
		normalized := strings.ReplaceAll(text, "\r\n", "\n")
		for _, rule := range ingressSecurityRules {
			if rule.pattern.MatchString(normalized) {
				risks = append(risks, IngressRisk{Category: rule.category, RuleID: rule.id})
			}
		}
	}
	return dedupeIngressRisks(risks), nil
}

func hasInvisibleControlMarker(text string) bool {
	for _, char := range text {
		switch char {
		case 0x200b, 0x200c, 0x200d, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2060, 0xfeff:
			return true
		default:
			if char < 0x20 && char != '\n' && char != '\r' && char != '\t' {
				return true
			}
		}
	}
	return false
}

func dedupeIngressRisks(risks []IngressRisk) []IngressRisk {
	if len(risks) < 2 {
		return risks
	}
	seen := map[IngressRisk]bool{}
	deduped := make([]IngressRisk, 0, len(risks))
	for _, risk := range risks {
		if seen[risk] {
			continue
		}
		seen[risk] = true
		deduped = append(deduped, risk)
	}
	return deduped
}
