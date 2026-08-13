//go:build !linux

package store

import (
	"errors"
	"os"
)

var errReflinkUnsupported = errors.New("store: reflink is unsupported")

func reflinkRange(source, destination *os.File, sourceOffset, destinationOffset, length int64) error {
	return errReflinkUnsupported
}
