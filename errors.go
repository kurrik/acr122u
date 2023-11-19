package acr122u

import (
	"errors"
	"fmt"

	"github.com/ebfe/scard"
)

var (
	// ErrOperationFailed is returned when the response code is 0x63 0x00
	ErrOperationFailed = errors.New("operation failed")

	// ErrShutdown is returned when the library detects an interrupt signal
	ErrShutdown = errors.New("shutting down")
)

func wrapError(message string, err error) error {
	switch v := err.(type) {
	case scard.Error:
		return fmt.Errorf("%v [%w (%X)]", message, err, uint32(v))
	default:
		return fmt.Errorf("%v [%w]", message, err)
	}
}
