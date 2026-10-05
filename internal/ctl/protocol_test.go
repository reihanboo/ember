package ctl

import (
	"reflect"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	requests := []Request{
		RequestReload,
		RequestBuild,
		RequestStop,
		RequestStart,
		RequestStatus,
		RequestQuit,
	}
	for _, want := range requests {
		t.Run(string(want), func(t *testing.T) {
			encoded, err := EncodeRequest(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodeRequest(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("DecodeRequest() = %q, want %q", got, want)
			}
		})
	}
}

func TestDecodeRequestRejectsMalformedInput(t *testing.T) {
	for _, input := range []string{"", "unknown", " reload", "reload ", "reload\nstop\n", "reload\rstop", "reload\n\r"} {
		t.Run(input, func(t *testing.T) {
			if _, err := DecodeRequest([]byte(input)); err == nil {
				t.Errorf("DecodeRequest(%q) succeeded, want error", input)
			}
		})
	}
}

func TestEncodeRequestRejectsInvalidRequest(t *testing.T) {
	if _, err := EncodeRequest("unknown"); err == nil {
		t.Error("EncodeRequest() succeeded, want error")
	}
}

func TestResponseRoundTrip(t *testing.T) {
	want := []string{"state: running", "", ".", "..details", "last line"}
	encoded, err := EncodeResponse(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeResponse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DecodeResponse() = %#v, want %#v", got, want)
	}
}

func TestEncodeResponseRejectsMalformedInput(t *testing.T) {
	for name, lines := range map[string][]string{
		"no lines":        nil,
		"line feed":       {"first\nsecond"},
		"carriage return": {"first\rsecond"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := EncodeResponse(lines); err == nil {
				t.Errorf("EncodeResponse(%q) succeeded, want error", lines)
			}
		})
	}
}

func TestDecodeResponseRejectsMalformedInput(t *testing.T) {
	for name, input := range map[string]string{
		"empty":                 "",
		"missing terminator":    "response\n",
		"missing final newline": "response\n. ",
		"terminator only":       ".\n",
		"early terminator":      ".\nresponse\n.\n",
		"unescaped dot line":    ".detail\n.\n",
		"carriage return":       "response\r\n.\n",
		"trailing content":      "response\n.\nextra\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeResponse([]byte(input)); err == nil {
				t.Errorf("DecodeResponse(%q) succeeded, want error", input)
			}
		})
	}
}
