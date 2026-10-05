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
		event, reply, statusReply, err := eventForRequest(request)
		if err != nil {
			return []string{"error: " + err.Error()}
		}
		supervisor.Send(event)
		if statusReply != nil {
			return SnapshotLines(<-statusReply)
		}
		if reply != nil {
			if err := <-reply; err != nil {
				return []string{"error: " + err.Error()}
			}
		}
		return []string{"ok"}
	}
}

func eventForRequest(request Request) (sup.Event, <-chan error, <-chan sup.Snapshot, error) {
	switch request {
	case RequestReload:
		return sup.ReloadRequested{}, nil, nil, nil
	case RequestBuild:
		return sup.BuildOnlyRequested{}, nil, nil, nil
	case RequestStop:
		reply := make(chan error, 1)
		return sup.StopRequested{Reply: reply}, reply, nil, nil
	case RequestStart:
		reply := make(chan error, 1)
		return sup.StartRequested{Reply: reply}, reply, nil, nil
	case RequestStatus:
		reply := make(chan sup.Snapshot, 1)
		return sup.StatusRequested{Reply: reply}, nil, reply, nil
	case RequestQuit:
		return sup.ControlEvent{Command: string(request)}, nil, nil, nil
	default:
		return nil, nil, nil, fmt.Errorf("unsupported control request %q", request)
	}
}
