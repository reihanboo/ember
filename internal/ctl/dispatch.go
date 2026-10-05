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
		event, reply, err := eventForRequest(request)
		if err != nil {
			return []string{"error: " + err.Error()}
		}
		supervisor.Send(event)
		if reply != nil {
			if err := <-reply; err != nil {
				return []string{"error: " + err.Error()}
			}
		}
		return []string{"ok"}
	}
}

func eventForRequest(request Request) (sup.Event, <-chan error, error) {
	switch request {
	case RequestReload:
		return sup.ReloadRequested{}, nil, nil
	case RequestBuild:
		return sup.BuildOnlyRequested{}, nil, nil
	case RequestStop:
		reply := make(chan error, 1)
		return sup.StopRequested{Reply: reply}, reply, nil
	case RequestStart:
		reply := make(chan error, 1)
		return sup.StartRequested{Reply: reply}, reply, nil
	case RequestStatus, RequestQuit:
		return sup.ControlEvent{Command: string(request)}, nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported control request %q", request)
	}
}
