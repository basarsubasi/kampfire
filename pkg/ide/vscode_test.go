package ide

import (
	"fmt"
	"net"
	"testing"
)

func TestGetFreePort(t *testing.T) {
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("getFreePort() failed: %v", err)
	}

	if port <= 0 || port > 65535 {
		t.Fatalf("getFreePort() returned invalid port: %d", port)
	}

	// Verify that the port can be bound on 0.0.0.0
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		t.Fatalf("failed to resolve address on 0.0.0.0:%d: %v", port, err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen on 0.0.0.0:%d: %v", port, err)
	}
	defer l.Close()
}
