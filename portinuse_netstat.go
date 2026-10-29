//go:build linux || windows

package main

import "github.com/cakturk/go-netstat/netstat"

// This method checks the system to see if something is binding to the port
func isPortInUse(port int) (bool, error) {
	tcpSocks, err := netstat.TCPSocks(netstat.NoopFilter)
	if err != nil {
		return false, err
	}
	for _, s := range tcpSocks {
		if s.LocalAddr.Port == uint16(port) && s.State == netstat.Listen {
			return true, nil
		}
	}
	return false, nil
}
