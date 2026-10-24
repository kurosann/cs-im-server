package service

import (
	"context"
	"encoding/json"
	"time"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"cs-im-server/pkg/consts"
	"vane-common/pkg/safe"
	"vane-common/publish"
	"vane-common/trpc"
)

const realtimeBusTopic = consts.ServiceName + ":realtime:v1"

type RealtimeBus interface {
	Publish(ctx context.Context, event RealtimeEvent) error
	Subscribe(ctx context.Context) (<-chan RealtimeEvent, func(), error)
}

type PublishServerRealtimeBus struct {
	topic string
}

func NewPublishServerRealtimeBus() *PublishServerRealtimeBus {
	publish.SetSubscriberStore(&publishServerSubscriberStore{})
	return &PublishServerRealtimeBus{topic: realtimeBusTopic}
}

func (b *PublishServerRealtimeBus) Publish(ctx context.Context, event RealtimeEvent) error {
	payload, err := encodeRealtimeEvent(event)
	if err != nil {
		return err
	}
	return publish.Publish(ctx, b.topic, payload)
}

func (b *PublishServerRealtimeBus) Subscribe(ctx context.Context) (<-chan RealtimeEvent, func(), error) {
	payloads, cancel, err := publish.Subscribe(ctx, b.topic)
	if err != nil {
		return nil, nil, err
	}

	events := make(chan RealtimeEvent, 64)
	safe.Go(func() {
		defer close(events)
		for payload := range payloads {
			event, err := decodeRealtimeEvent(payload)
			if err != nil {
				continue
			}
			select {
			case events <- event:
			default:
			}
		}
	})

	return events, func() {
		_ = cancel()
	}, nil
}

type publishServerSubscriberStore struct{}

func (s *publishServerSubscriberStore) RecordSubscribe(context.Context, string) error { return nil }
func (s *publishServerSubscriberStore) RemoveSubscribe(context.Context, string) error { return nil }

func (s *publishServerSubscriberStore) QueryAllSubcribe(context.Context) (map[string][]string, error) {
	endpoints := trpc.GetSvcmap().GetSvcmap()[consts.ServiceName]
	if len(endpoints) == 0 {
		return map[string][]string{}, nil
	}
	return map[string][]string{realtimeBusTopic: endpoints}, nil
}

type realtimeBusEnvelope struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
	At   time.Time       `json:"at"`
}

func encodeRealtimeEvent(event RealtimeEvent) ([]byte, error) {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(realtimeBusEnvelope{
		Type: event.Type,
		ID:   event.ID,
		Data: data,
		At:   event.At,
	})
}

func decodeRealtimeEvent(payload []byte) (RealtimeEvent, error) {
	envelope := realtimeBusEnvelope{}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return RealtimeEvent{}, err
	}

	event := RealtimeEvent{Type: envelope.Type, ID: envelope.ID, At: envelope.At}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return event, nil
	}

	data, err := decodeRealtimeEventData(envelope.Type, envelope.Data)
	if err != nil {
		return RealtimeEvent{}, err
	}
	event.Data = data
	return event, nil
}

func decodeRealtimeEventData(eventType string, payload []byte) (any, error) {
	switch eventType {
	case "message.status_changed":
		var item gatewaydto.Message
		return item, json.Unmarshal(payload, &item)
	case "message.created":
		var item gatewaydto.Message
		return item, json.Unmarshal(payload, &item)
	case "message.created_batch":
		var items []gatewaydto.Message
		return items, json.Unmarshal(payload, &items)
	case
		"conversation.created",
		"conversation.assigned",
		"conversation.closed",
		"conversation.forwarded":
		var item gatewaydto.Conversation
		return item, json.Unmarshal(payload, &item)
	case
		"customer.created",
		"customer.identity_added",
		"customer.merged":
		var item gatewaydto.Customer
		return item, json.Unmarshal(payload, &item)
	default:
		var item any
		return item, json.Unmarshal(payload, &item)
	}
}
