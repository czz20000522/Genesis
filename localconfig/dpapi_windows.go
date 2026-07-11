//go:build windows

package localconfig

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func dpapiProtect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrCredentialUnreadable
	}
	inBlob := dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
	var outBlob dataBlob
	r1, _, err := procCryptProtectData.Call(uintptr(unsafe.Pointer(&inBlob)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&outBlob)))
	if r1 == 0 {
		if err != syscall.Errno(0) {
			return nil, err
		}
		return nil, fmt.Errorf("CryptProtectData failed")
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	protected := unsafe.Slice(outBlob.pbData, int(outBlob.cbData))
	return append([]byte(nil), protected...), nil
}

func dpapiUnprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrCredentialUnreadable
	}
	inBlob := dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
	var outBlob dataBlob
	r1, _, err := procCryptUnprotectData.Call(uintptr(unsafe.Pointer(&inBlob)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&outBlob)))
	if r1 == 0 {
		if err != syscall.Errno(0) {
			return nil, err
		}
		return nil, fmt.Errorf("CryptUnprotectData failed")
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	plain := unsafe.Slice(outBlob.pbData, int(outBlob.cbData))
	return append([]byte(nil), plain...), nil
}
