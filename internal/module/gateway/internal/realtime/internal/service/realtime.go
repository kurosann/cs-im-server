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
}

func NewRealtimeService() *RealtimeService {
	return &RealtimeService{subscribers: make(map[int]chan RealtimeEvent)}
}

func (s *RealtimeService) Subscribe(ctx context.Context) (<-chan RealtimeEvent, func(), error) {
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
	s.mu.RLock()
	subscribers := make([]chan RealtimeEvent, 0, len(s.subscribers))
	for _, events := range s.subscribers {
		subscribers = append(subscribers, events)
	}
	s.mu.RUnlock()
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
