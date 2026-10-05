package ctl

import (
	"fmt"

	"github.com/reihanboo/ember/internal/sup"
)

type EventSender interface {
	Send(sup.Event)
}

func NewDispatcher(supervisor EventSender) Handler {
	return func(request Request) []string {
		if supervisor == nil {
			return []string{"error: control supervisor is unavailable"}
		}
		event, err := eventForRequest(request)
		if err != nil {
			return []string{"error: " + err.Error()}
		}
		supervisor.Send(event)
		return []string{"ok"}
	}
}

func eventForRequest(request Request) (sup.Event, error) {
	switch request {
	case RequestReload:
		return sup.ReloadRequested{}, nil
	case RequestBuild:
		return sup.BuildOnlyRequested{}, nil
	case RequestStop, RequestStart, RequestStatus, RequestQuit:
		return sup.ControlEvent{Command: string(request)}, nil
	default:
		return nil, fmt.Errorf("unsupported control request %q", request)
	}
}
