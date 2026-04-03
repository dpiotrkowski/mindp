package mindp

import (
	"errors"
	"fmt"
)

var ErrNoHLSManifest = errors.New("mindp: no HLS manifest detected")

type CDPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *CDPError) Error() string {
	return fmt.Sprintf("mindp: cdp error %d: %s", e.Code, e.Message)
}
