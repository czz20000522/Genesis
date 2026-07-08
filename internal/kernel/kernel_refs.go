package kernel

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	kernelRefPattern          = regexp.MustCompile(`^(turn|review|approval|work|operation|memory|event|agent_profile|parent_result):[A-Za-z0-9][A-Za-z0-9._:/-]{0,190}$`)
	kernelAuthorityPattern    = regexp.MustCompile(`^(runtime|operator|user|daemon|system|application):[A-Za-z0-9][A-Za-z0-9._:/-]{0,190}$`)
	kernelControlTokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

func validateKernelRef(field string, value string) error {
	value = strings.TrimSpace(value)
	if !kernelRefPattern.MatchString(value) {
		return fmt.Errorf("%s must be a kernel ref", field)
	}
	return nil
}

func validateKernelAuthority(field string, value string) error {
	value = strings.TrimSpace(value)
	if !kernelAuthorityPattern.MatchString(value) {
		return fmt.Errorf("%s must be a kernel authority ref", field)
	}
	return nil
}

func validateKernelControlToken(field string, value string) error {
	if value == "" {
		return nil
	}
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("%s must not contain leading or trailing whitespace", field)
	}
	if !kernelControlTokenPattern.MatchString(value) {
		return fmt.Errorf("%s may contain only letters, numbers, '.', '_', '-', or ':'", field)
	}
	return nil
}

func validateKernelTextNotSecret(field string, value string) error {
	if containsCredentialShapedText(value) {
		return fmt.Errorf("%s must not contain secret-shaped content", field)
	}
	return nil
}
