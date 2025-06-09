package main

import (
	"testing"
)

func TestPing(t *testing.T) {
	if !Ping("127.0.0.1") {
		t.Error("Ping failed for localhost")
	}
}

func TestPortOpen(t *testing.T) {
	if !PortOpen("127.0.0.1", 22) {
		t.Log("Port 22 not open — might be expected on your system")
	}
}
