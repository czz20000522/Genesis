//go:build windows

package kernel

import (
	"os"
	"syscall"
)

func regularFileHasMultipleLinks(path string, _ os.FileInfo) bool {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return true
	}
	defer syscall.CloseHandle(handle)

	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &data); err != nil {
		return true
	}
	return data.NumberOfLinks > 1
}
