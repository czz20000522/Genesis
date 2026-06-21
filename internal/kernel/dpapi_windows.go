//go:build windows

package kernel

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func dpapiUnprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrLocalSecretUnreadable
	}
	inBlob := dataBlob{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}
	var outBlob dataBlob
	r1, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r1 == 0 {
		if err != syscall.Errno(0) {
			return nil, err
		}
		return nil, fmt.Errorf("CryptUnprotectData failed")
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	plain := unsafe.Slice(outBlob.pbData, int(outBlob.cbData))
	result := make([]byte, len(plain))
	copy(result, plain)
	return result, nil
}
