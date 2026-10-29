//go:build freebsd

package main

import (
	"bytes"
	"os/exec"
	"strconv"
)

// go-netstat has no FreeBSD backend, so ask sockstat(1) for TCP listeners on the port.
// Never connect to the port: VARA under Wine cannot rebind after a probe connection.
func isPortInUse(port int) (bool, error) {
	out, err := exec.Command("/usr/bin/sockstat", "-46lq", "-P", "tcp", "-p", strconv.Itoa(port)).Output()
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}
