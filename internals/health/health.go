// Package health provides lightweight server reachability checks.
package health

import (
	"net"
	"strconv"
	"time"

	"github.com/Omotolani98/mgr/internals/inventory"
)

type Status struct {
	Name      string        `json:"name"`
	Address   string        `json:"address"`
	Reachable bool          `json:"reachable"`
	Latency   time.Duration `json:"latency"`
	CheckedAt time.Time     `json:"checked_at"`
	Error     string        `json:"error,omitempty"`
}

func Check(server inventory.Server, timeout time.Duration) Status {
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	address := net.JoinHostPort(server.Host, strconv.Itoa(server.Port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	checkedAt := time.Now().UTC()
	status := Status{
		Name:      server.Name,
		Address:   address,
		Latency:   time.Since(start),
		CheckedAt: checkedAt,
	}
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Reachable = true
	_ = conn.Close()
	return status
}
