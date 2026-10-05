package ctl

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const clientRequestTimeout = 5 * time.Second

var ErrNoInstance = errors.New("no running instance found")

func SendRequest(ctx context.Context, controlFile string, request Request) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	contents, err := os.ReadFile(controlFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: control port file %q does not exist", ErrNoInstance, controlFile)
	}
	if err != nil {
		return nil, fmt.Errorf("read control port file %q: %w", controlFile, err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("%w: control port file %q contains an invalid port", ErrNoInstance, controlFile)
	}
	encodedRequest, err := EncodeRequest(request)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: serverProbeTimeout}
	connection, err := dialer.DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("connect to control server: %w", err)
		}
		return nil, fmt.Errorf("%w: connect to control server: %w", ErrNoInstance, err)
	}
	defer connection.Close()
	stopCancellation := context.AfterFunc(ctx, func() {
		_ = connection.Close()
	})
	defer stopCancellation()
	deadline := time.Now().Add(clientRequestTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("set control request deadline: %w", err)
	}
	if _, err := io.Copy(connection, bytes.NewReader(encodedRequest)); err != nil {
		return nil, fmt.Errorf("send control request: %w", err)
	}
	response, err := readControlResponse(connection)
	if err != nil {
		return nil, err
	}
	if len(response) > 0 && strings.HasPrefix(response[0], "error: ") {
		return nil, errors.New(strings.TrimPrefix(response[0], "error: "))
	}
	return response, nil
}

func readControlResponse(reader io.Reader) ([]string, error) {
	const maxResponseSize = 1 << 20
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), maxResponseSize)
	var response bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if response.Len()+len(line)+1 > maxResponseSize {
			return nil, errors.New("control response exceeds the maximum size")
		}
		response.WriteString(line)
		response.WriteByte('\n')
		if line == responseTerminator {
			lines, err := DecodeResponse(response.Bytes())
			if err != nil {
				return nil, fmt.Errorf("decode control response: %w", err)
			}
			return lines, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read control response: %w", err)
	}
	return nil, errors.New("control server closed before the response terminator")
}
