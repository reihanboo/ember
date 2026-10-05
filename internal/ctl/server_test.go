package ctl

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestServerPublishesPortServesRequestsAndRemovesPortFile(t *testing.T) {
	controlFile := filepath.Join(t.TempDir(), ".ember", "ctl")
	if err := os.MkdirAll(filepath.Dir(controlFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(controlFile, []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	requests := make(chan Request, 1)
	server, err := StartServer(controlFile, func(request Request) []string {
		requests <- request
		return []string{"accepted"}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})

	portContents, err := os.ReadFile(controlFile)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(portContents)))
	if err != nil || port < 1 || port > 65535 {
		t.Fatalf("control port file contains %q, want a valid TCP port (parse error: %v)", portContents, err)
	}
	listenerAddress := server.listener.Addr().(*net.TCPAddr)
	if !listenerAddress.IP.IsLoopback() || listenerAddress.Port != port {
		t.Errorf("control listener address = %v, want loopback port %d", listenerAddress, port)
	}

	connection, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	request, err := EncodeRequest(RequestStatus)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(connection, bytes.NewReader(request)); err != nil {
		_ = connection.Close()
		t.Fatal(err)
	}
	responseBytes, err := io.ReadAll(connection)
	_ = connection.Close()
	if err != nil {
		t.Fatal(err)
	}
	response, err := DecodeResponse(responseBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(response) != 1 || response[0] != "accepted" {
		t.Errorf("control response = %#v, want [\"accepted\"]", response)
	}
	select {
	case got := <-requests:
		if got != RequestStatus {
			t.Errorf("handler request = %q, want %q", got, RequestStatus)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not receive request")
	}

	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(controlFile); !os.IsNotExist(err) {
		t.Errorf("control port file stat error = %v, want file removed", err)
	}
}

func TestStartServerRefusesLiveInstance(t *testing.T) {
	controlFile := filepath.Join(t.TempDir(), ".ember", "ctl")
	first, err := StartServer(controlFile, func(Request) []string {
		return []string{"running"}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := first.Close(); err != nil {
			t.Error(err)
		}
	})
	originalPortFile, err := os.ReadFile(controlFile)
	if err != nil {
		t.Fatal(err)
	}

	second, err := StartServer(controlFile, func(Request) []string {
		return []string{"unexpected"}
	})
	if err == nil {
		if second != nil {
			_ = second.Close()
		}
		t.Fatal("StartServer() succeeded while a server was responding")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("StartServer() error = %q, want already-running message", err)
	}
	currentPortFile, err := os.ReadFile(controlFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(currentPortFile, originalPortFile) {
		t.Errorf("port file after rejected start = %q, want unchanged %q", currentPortFile, originalPortFile)
	}
}

func TestStartServerOverwritesStalePortFile(t *testing.T) {
	oldListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stalePort := oldListener.Addr().(*net.TCPAddr).Port
	if err := oldListener.Close(); err != nil {
		t.Fatal(err)
	}

	controlFile := filepath.Join(t.TempDir(), ".ember", "ctl")
	if err := os.MkdirAll(filepath.Dir(controlFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(controlFile, []byte(strconv.Itoa(stalePort)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server, err := StartServer(controlFile, func(Request) []string {
		return []string{"running"}
	})
	if err != nil {
		t.Fatalf("StartServer() rejected stale port file: %v", err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	portContents, err := os.ReadFile(controlFile)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(portContents)))
	if err != nil || port < 1 || port > 65535 {
		t.Fatalf("overwritten control port file contains %q, want valid port (parse error: %v)", portContents, err)
	}
	if got := server.listener.Addr().(*net.TCPAddr).Port; got != port {
		t.Errorf("listener port = %d, port file = %d", got, port)
	}
}

func TestIsLoopbackPeer(t *testing.T) {
	for _, test := range []struct {
		name    string
		address net.Addr
		want    bool
	}{
		{name: "IPv4 loopback", address: &net.TCPAddr{IP: net.ParseIP("127.0.0.1")}, want: true},
		{name: "IPv6 loopback", address: &net.TCPAddr{IP: net.ParseIP("::1")}, want: true},
		{name: "non-loopback", address: &net.TCPAddr{IP: net.ParseIP("192.0.2.1")}},
		{name: "non-TCP", address: &net.UnixAddr{Name: "control.sock", Net: "unix"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isLoopbackPeer(test.address); got != test.want {
				t.Errorf("isLoopbackPeer(%v) = %t, want %t", test.address, got, test.want)
			}
		})
	}
}
