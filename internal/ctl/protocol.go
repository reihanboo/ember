package ctl

import (
	"errors"
	"strings"
)

type Request string

const (
	RequestReload Request = "reload"
	RequestBuild  Request = "build"
	RequestStop   Request = "stop"
	RequestStart  Request = "start"
	RequestStatus Request = "status"
	RequestQuit   Request = "quit"
)

const responseTerminator = "."

func EncodeRequest(request Request) ([]byte, error) {
	if !validRequest(request) {
		return nil, errors.New("invalid control request")
	}
	return []byte(string(request) + "\n"), nil
}

func DecodeRequest(line []byte) (Request, error) {
	value := string(line)
	if strings.HasSuffix(value, "\r\n") {
		value = strings.TrimSuffix(value, "\r\n")
	} else if strings.HasSuffix(value, "\n") {
		value = strings.TrimSuffix(value, "\n")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", errors.New("control request must be a single line")
	}
	request := Request(value)
	if !validRequest(request) {
		return "", errors.New("invalid control request")
	}
	return request, nil
}

func EncodeResponse(lines []string) ([]byte, error) {
	if len(lines) == 0 {
		return nil, errors.New("control response must contain at least one line")
	}
	var response strings.Builder
	for _, line := range lines {
		if strings.ContainsAny(line, "\r\n") {
			return nil, errors.New("control response lines must not contain line breaks")
		}
		if strings.HasPrefix(line, ".") {
			response.WriteByte('.')
		}
		response.WriteString(line)
		response.WriteByte('\n')
	}
	response.WriteString(responseTerminator)
	response.WriteByte('\n')
	return []byte(response.String()), nil
}

func DecodeResponse(data []byte) ([]string, error) {
	value := string(data)
	if !strings.HasSuffix(value, "\n") {
		return nil, errors.New("control response is missing its terminating line")
	}
	framedLines := strings.Split(strings.TrimSuffix(value, "\n"), "\n")
	if len(framedLines) < 2 || framedLines[len(framedLines)-1] != responseTerminator {
		return nil, errors.New("control response is missing its terminating line")
	}
	lines := make([]string, 0, len(framedLines)-1)
	for _, line := range framedLines[:len(framedLines)-1] {
		if strings.ContainsRune(line, '\r') {
			return nil, errors.New("control response lines must not contain carriage returns")
		}
		if line == responseTerminator {
			return nil, errors.New("control response contains an early terminator")
		}
		if strings.HasPrefix(line, ".") {
			if !strings.HasPrefix(line, "..") {
				return nil, errors.New("control response contains an invalid dot-prefixed line")
			}
			line = strings.TrimPrefix(line, ".")
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func validRequest(request Request) bool {
	switch request {
	case RequestReload, RequestBuild, RequestStop, RequestStart, RequestStatus, RequestQuit:
		return true
	default:
		return false
	}
}
