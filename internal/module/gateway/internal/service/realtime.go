package service

import (
	"context"
	"sync"
	"time"

	"cs-im-server/pkg/sse"
)

type RealtimeEvent = sse.Event

type RealtimeService struct {
	mu            sync.RWMutex
	subscribers   map[int]chan RealtimeEvent
	subscriberSeq int
	bus           RealtimeBus
}

func NewRealtimeService(buses ...RealtimeBus) *RealtimeService {
	realtime := &RealtimeService{subscribers: make(map[int]chan RealtimeEvent)}
	if len(buses) > 0 && buses[0] != nil {
		realtime.bus = buses[0]
	}
	return realtime
}

func (s *RealtimeService) Subscribe(ctx context.Context) (<-chan RealtimeEvent, func(), error) {
	if s.bus != nil {
		return s.bus.Subscribe(ctx)
	}

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

func (s *RealtimeService) Publish(eventType, id string, data any) {
	event := RealtimeEvent{Type: eventType, ID: id, Data: data, At: time.Now().UTC()}
	if s.bus != nil {
		if err := s.bus.Publish(context.Background(), event); err != nil {
			s.publishLocal(event)
		}
		return
	}
	s.publishLocal(event)
}

func (s *RealtimeService) publishLocal(event RealtimeEvent) {
	s.mu.RLock()
	subscribers := make([]chan RealtimeEvent, 0, len(s.subscribers))
	for _, events := range s.subscribers {
		subscribers = append(subscribers, events)
	}
	s.mu.RUnlock()
	if len(subscribers) == 0 {
		return
	}
	for _, events := range subscribers {
		select {
		case events <- event:
		default:
		}
	}
}
