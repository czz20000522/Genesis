//go:build !windows

package kernel

func dpapiProtect(_ []byte) ([]byte, error) {
	return nil, ErrLocalSecretUnsupported
}

func dpapiUnprotect(_ []byte) ([]byte, error) {
	return nil, ErrLocalSecretUnsupported
}
