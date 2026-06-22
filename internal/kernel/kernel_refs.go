package kernel

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	kernelRefPattern       = regexp.MustCompile(`^(turn|review|approval|work|operation|memory|event):[A-Za-z0-9][A-Za-z0-9._:/=-]{0,190}$`)
	kernelAuthorityPattern = regexp.MustCompile(`^(runtime|operator|user|daemon|system):[A-Za-z0-9][A-Za-z0-9._:/=-]{0,190}$`)
)

func validateKernelRef(field string, value string) error {
	value = strings.TrimSpace(value)
	if !kernelRefPattern.MatchString(value) {
		return fmt.Errorf("%s must be a kernel ref", field)
	}
	return validateKernelTextNotSecret(field, value)
}

func validateKernelAuthority(field string, value string) error {
	value = strings.TrimSpace(value)
	if !kernelAuthorityPattern.MatchString(value) {
		return fmt.Errorf("%s must be a kernel authority ref", field)
	}
	return validateKernelTextNotSecret(field, value)
}

func validateKernelTextNotSecret(field string, value string) error {
	if redactEvidenceText(value) != value {
		return fmt.Errorf("%s must not contain secret-shaped content", field)
	}
	return nil
}
