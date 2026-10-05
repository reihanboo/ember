package ctl

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Handler func(Request) []string

const serverProbeTimeout = time.Second

type Server struct {
	listener     net.Listener
	controlFile  string
	handler      Handler
	acceptDone   chan struct{}
	connections  map[net.Conn]struct{}
	connectionM  sync.Mutex
	connectionWG sync.WaitGroup
	closeOnce    sync.Once
	closeErr     error
}

func StartServer(controlFile string, handler Handler) (*Server, error) {
	if handler == nil {
		return nil, errors.New("control handler is nil")
	}
	if running, port, err := existingServerResponds(controlFile); err != nil {
		return nil, err
	} else if running {
		return nil, fmt.Errorf("control server is already running for this project on port %d", port)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for control requests: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := writePortFile(controlFile, port); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("write control port file: %w", err)
	}
	server := &Server{
		listener:    listener,
		controlFile: controlFile,
		handler:     handler,
		acceptDone:  make(chan struct{}),
		connections: make(map[net.Conn]struct{}),
	}
	go server.acceptConnections()
	return server, nil
}

func (server *Server) Close() error {
	server.closeOnce.Do(func() {
		if err := server.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			server.closeErr = errors.Join(server.closeErr, fmt.Errorf("close control listener: %w", err))
		}
		<-server.acceptDone
		server.connectionM.Lock()
		for connection := range server.connections {
			_ = connection.Close()
		}
		server.connectionM.Unlock()
		server.connectionWG.Wait()
		if err := os.Remove(server.controlFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			server.closeErr = errors.Join(server.closeErr, fmt.Errorf("remove control port file: %w", err))
		}
	})
	return server.closeErr
}

func (server *Server) acceptConnections() {
	defer close(server.acceptDone)
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			return
		}
		if !isLoopbackPeer(connection.RemoteAddr()) {
			_ = connection.Close()
			continue
		}
		server.connectionM.Lock()
		server.connections[connection] = struct{}{}
		server.connectionWG.Add(1)
		server.connectionM.Unlock()
		go server.serveConnection(connection)
	}
}

func (server *Server) serveConnection(connection net.Conn) {
	defer server.connectionWG.Done()
	defer func() {
		server.connectionM.Lock()
		delete(server.connections, connection)
		server.connectionM.Unlock()
		_ = connection.Close()
	}()

	scanner := bufio.NewScanner(connection)
	if !scanner.Scan() {
		return
	}
	request, err := DecodeRequest(scanner.Bytes())
	if err != nil {
		return
	}
	response, err := EncodeResponse(server.handler(request))
	if err != nil {
		return
	}
	_, _ = io.Copy(connection, bytes.NewReader(response))
}

func isLoopbackPeer(address net.Addr) bool {
	tcpAddress, ok := address.(*net.TCPAddr)
	return ok && tcpAddress.IP != nil && tcpAddress.IP.IsLoopback()
}

func existingServerResponds(controlFile string) (bool, int, error) {
	contents, err := os.ReadFile(controlFile)
	if errors.Is(err, os.ErrNotExist) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("read existing control port file: %w", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil || port < 1 || port > 65535 {
		return false, 0, nil
	}
	connection, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), serverProbeTimeout)
	if err != nil {
		return false, 0, nil
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(serverProbeTimeout)); err != nil {
		return false, 0, nil
	}
	request, err := EncodeRequest(RequestStatus)
	if err != nil {
		return false, 0, nil
	}
	if _, err := io.Copy(connection, bytes.NewReader(request)); err != nil {
		return false, 0, nil
	}
	scanner := bufio.NewScanner(connection)
	scanner.Buffer(make([]byte, 1024), 1<<20)
	var response bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if response.Len()+len(line)+1 > 1<<20 {
			return false, 0, nil
		}
		response.WriteString(line)
		response.WriteByte('\n')
		if line == responseTerminator {
			_, err := DecodeResponse(response.Bytes())
			return err == nil, port, nil
		}
	}
	return false, 0, nil
}

func writePortFile(path string, port int) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create control directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".ctl-*")
	if err != nil {
		return fmt.Errorf("create temporary control port file: %w", err)
	}
	temporaryPath := file.Name()
	defer os.Remove(temporaryPath)
	if _, err := file.WriteString(strconv.Itoa(port) + "\n"); err != nil {
		_ = file.Close()
		return fmt.Errorf("write temporary control port file: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync temporary control port file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary control port file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish control port file: %w", err)
	}
	return nil
}
