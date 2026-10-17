package service

import (
	"context"
	"time"
)

type RealtimeEvent struct {
	Type string    `json:"type"`
	ID   string    `json:"id,omitempty"`
	Data any       `json:"data,omitempty"`
	At   time.Time `json:"at"`
}

func (s *Service) Subscribe(ctx context.Context) (<-chan RealtimeEvent, func(), error) {
	events := make(chan RealtimeEvent, 64)

	s.mu.Lock()
	s.subscriberSeq++
	id := s.subscriberSeq
	s.subscribers[id] = events
	s.mu.Unlock()

	closeFn := func() {
		s.mu.Lock()
		delete(s.subscribers, id)
		s.mu.Unlock()
	}
	publish := RealtimeEvent{Type: "connected", At: time.Now().UTC()}
	select {
	case events <- publish:
	case <-ctx.Done():
		closeFn()
		return nil, nil, ctx.Err()
	}
	return events, closeFn, nil
}

func (s *Service) publishLocked(eventType string, id string, data any) {
	subscribers := make([]chan RealtimeEvent, 0, len(s.subscribers))
	for _, events := range s.subscribers {
		subscribers = append(subscribers, events)
	}

	if len(subscribers) == 0 {
		return
	}
	event := RealtimeEvent{Type: eventType, ID: id, Data: data, At: time.Now().UTC()}
	for _, events := range subscribers {
		select {
		case events <- event:
		default:
		}
	}
}
