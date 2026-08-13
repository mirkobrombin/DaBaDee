//go:build linux

package store

import (
	"os"
	"syscall"
	"unsafe"
)

const fileCloneRange = 0x4020940d

type cloneRange struct {
	SourceFD     int64
	SourceOffset uint64
	SourceLength uint64
	DestOffset   uint64
}

func reflinkRange(source, destination *os.File, sourceOffset, destinationOffset, length int64) error {
	rangeInfo := cloneRange{
		SourceFD:     int64(source.Fd()),
		SourceOffset: uint64(sourceOffset),
		SourceLength: uint64(length),
		DestOffset:   uint64(destinationOffset),
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, destination.Fd(), fileCloneRange, uintptr(unsafe.Pointer(&rangeInfo)))
	if errno != 0 {
		return errno
	}
	return nil
}
