//go:build !windows

package kernel

func dpapiUnprotect(_ []byte) ([]byte, error) {
	return nil, ErrLocalSecretUnsupported
}
