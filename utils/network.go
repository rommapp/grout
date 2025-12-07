package utils

import (
	"net"
	"time"
)

func IsConnectedToInternet() bool {
	timeout := 5 * time.Second
	_, err := net.DialTimeout("tcp", "8.8.8.8:53", timeout)
	return err == nil
}
