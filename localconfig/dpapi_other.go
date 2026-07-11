//go:build !windows

package localconfig

func dpapiProtect(_ []byte) ([]byte, error) {
	return nil, ErrCredentialUnsupported
}

func dpapiUnprotect(_ []byte) ([]byte, error) {
	return nil, ErrCredentialUnsupported
}
