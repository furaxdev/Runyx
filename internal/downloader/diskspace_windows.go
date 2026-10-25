//go:build windows

package downloader

import (
	"syscall"
	"unsafe"
)

func freeDiskSpace(dir string) (int64, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetDiskFreeSpaceExW")

	path, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var freeBytesAvailable int64
	ret, _, callErr := proc.Call(
		uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		0,
		0,
	)
	if ret == 0 {
		return 0, callErr
	}
	return freeBytesAvailable, nil
}
