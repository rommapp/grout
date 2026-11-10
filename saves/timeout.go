package saves

import (
	"context"
	"errors"
	"net"
	"os"
	"time"

	"grout/romm"
)

// completeSessionTimeout bounds telling RomM a sync is over. That call is a
// courtesy: if it is lost the server expires the session on its own, so it is
// not worth the player waiting out the full request timeout for it.
var completeSessionTimeout = 5 * time.Second

// isTimeout reports whether an error is RomM not answering in time.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, romm.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
