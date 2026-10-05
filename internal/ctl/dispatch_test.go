package ctl

import (
	"reflect"
	"strings"
	"testing"

	"github.com/reihanboo/ember/internal/sup"
)

type fakeEventSender struct {
	events chan sup.Event
}

func (sender fakeEventSender) Send(event sup.Event) {
	sender.events <- event
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
		{request: RequestStop, want: sup.ControlEvent{Command: "stop"}},
		{request: RequestStart, want: sup.ControlEvent{Command: "start"}},
		{request: RequestStatus, want: sup.ControlEvent{Command: "status"}},
		{request: RequestQuit, want: sup.ControlEvent{Command: "quit"}},
	}
	for _, test := range tests {
		if response := dispatch(test.request); !reflect.DeepEqual(response, []string{"ok"}) {
			t.Errorf("dispatch(%q) = %#v, want [\"ok\"]", test.request, response)
		}
		if got := <-sender.events; !reflect.DeepEqual(got, test.want) {
			t.Errorf("event for %q = %#v, want %#v", test.request, got, test.want)
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
