package ctl

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/reihanboo/ember/internal/sup"
)

type fakeEventSender struct {
	events   chan sup.Event
	replyErr error
}

func (sender fakeEventSender) Send(event sup.Event) {
	sender.events <- event
	switch requested := event.(type) {
	case sup.StopRequested:
		requested.Reply <- sender.replyErr
	case sup.StartRequested:
		requested.Reply <- sender.replyErr
	}
}

func TestDispatcherSendsSupervisorEventsAndRepliesOK(t *testing.T) {
	sender := fakeEventSender{events: make(chan sup.Event, 6)}
	dispatch := NewDispatcher(sender)
	tests := []struct {
		request Request
		want    sup.Event
	}{
		{request: RequestReload, want: sup.ReloadRequested{}},
		{request: RequestBuild, want: sup.BuildOnlyRequested{}},
		{request: RequestStop, want: sup.StopRequested{}},
		{request: RequestStart, want: sup.StartRequested{}},
		{request: RequestStatus, want: sup.ControlEvent{Command: "status"}},
		{request: RequestQuit, want: sup.ControlEvent{Command: "quit"}},
	}
	for _, test := range tests {
		if response := dispatch(test.request); !reflect.DeepEqual(response, []string{"ok"}) {
			t.Errorf("dispatch(%q) = %#v, want [\"ok\"]", test.request, response)
		}
		got := <-sender.events
		switch test.request {
		case RequestStop:
			requested, ok := got.(sup.StopRequested)
			if !ok || requested.Reply == nil {
				t.Errorf("event for %q = %#v, want StopRequested with reply channel", test.request, got)
			}
		case RequestStart:
			requested, ok := got.(sup.StartRequested)
			if !ok || requested.Reply == nil {
				t.Errorf("event for %q = %#v, want StartRequested with reply channel", test.request, got)
			}
		default:
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("event for %q = %#v, want %#v", test.request, got, test.want)
			}
		}
	}
}

func TestDispatcherRepliesWithErrors(t *testing.T) {
	tests := []struct {
		name     string
		dispatch Handler
		request  Request
	}{
		{name: "missing supervisor", dispatch: NewDispatcher(nil), request: RequestReload},
		{name: "unsupported request", dispatch: NewDispatcher(fakeEventSender{events: make(chan sup.Event, 1)}), request: Request("unknown")},
		{name: "supervisor operation failed", dispatch: NewDispatcher(fakeEventSender{events: make(chan sup.Event, 1), replyErr: errors.New("no successful build")}), request: RequestStart},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := test.dispatch(test.request)
			if len(response) != 1 || !strings.HasPrefix(response[0], "error: ") {
				t.Errorf("dispatch(%q) = %#v, want error response", test.request, response)
			}
		})
	}
}
