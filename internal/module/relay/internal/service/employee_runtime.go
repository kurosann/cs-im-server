package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/logger"
)

const inboundDebounce = 2 * time.Second

type ConversationRef struct {
	AppID      string
	CustomerID string
	Subject    string
}

type TranscriptMessage struct {
	ID   string
	Role string
	Type string
	Text string
	Seq  int64
}

type InboundGateway interface {
	LookupConversation(ctx context.Context, conversationID string) (ConversationRef, error)
	RecentMessages(ctx context.Context, conversationID string, limit int) ([]TranscriptMessage, error)
	SendAIReply(ctx context.Context, conversationID, employeeID, text string) error
}

type InboundMessage struct {
	ID             string
	ConversationID string
	Role           string
	Type           string
	Text           string
}

type inboundBatch struct {
	items []InboundMessage
	timer *time.Timer
}

func (s *RelayService) SetInboundGateway(gateway InboundGateway) {
	s.inboundMu.Lock()
	s.inbound = gateway
	s.inboundMu.Unlock()
}

func (s *RelayService) SetCompletionClient(client CompletionClient) {
	s.completer = client
}

func (s *RelayService) EnqueueInbound(message InboundMessage) {
	message.Role = strings.TrimSpace(message.Role)
	message.ConversationID = strings.TrimSpace(message.ConversationID)
	message.ID = strings.TrimSpace(message.ID)
	if message.Role != "customer" || message.ConversationID == "" || message.ID == "" {
		return
	}
	if messageMarker(message.Type, message.Text) == "" {
		return
	}
	s.inboundMu.Lock()
	defer s.inboundMu.Unlock()
	if s.inboundPending == nil {
		s.inboundPending = map[string]*inboundBatch{}
	}
	batch := s.inboundPending[message.ConversationID]
	if batch == nil {
		batch = &inboundBatch{}
		s.inboundPending[message.ConversationID] = batch
	}
	replaced := false
	for index, item := range batch.items {
		if item.ID == message.ID {
			batch.items[index] = message
			replaced = true
			break
		}
	}
	if !replaced {
		batch.items = append(batch.items, message)
	}
	if batch.timer != nil {
		batch.timer.Stop()
	}
	conversationID := message.ConversationID
	batch.timer = time.AfterFunc(inboundDebounce, func() {
		s.flushInbound(conversationID)
	})
}

func (s *RelayService) flushInbound(conversationID string) {
	s.inboundMu.Lock()
	batch := s.inboundPending[conversationID]
	delete(s.inboundPending, conversationID)
	s.inboundMu.Unlock()
	if batch == nil || len(batch.items) == 0 {
		return
	}
	lane := s.inboundLane(conversationID)
	lane.Lock()
	defer lane.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := s.HandleInboundBatch(ctx, batch.items); err != nil {
		logger.Errorf(ctx, "ai inbound %s: %v", conversationID, err)
	}
}

func (s *RelayService) inboundLane(conversationID string) *sync.Mutex {
	s.inboundMu.Lock()
	defer s.inboundMu.Unlock()
	if s.inboundLanes == nil {
		s.inboundLanes = map[string]*sync.Mutex{}
	}
	lane := s.inboundLanes[conversationID]
	if lane == nil {
		lane = &sync.Mutex{}
		s.inboundLanes[conversationID] = lane
	}
	return lane
}

func (s *RelayService) HandleInboundBatch(ctx context.Context, messages []InboundMessage) error {
	messages = customerInbound(messages)
	if len(messages) == 0 {
		return nil
	}
	conversationID := messages[0].ConversationID
	gateway := s.inboundGateway()
	if gateway == nil {
		return nil
	}
	ref, err := gateway.LookupConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	app, err := s.apps.First(ctx, dataaccess.Eq("app_key", strings.TrimSpace(ref.AppID)))
	if err != nil {
		if isMissingTable(err) || errorsIsNotFound(err) {
			return nil
		}
		return err
	}
	if app == nil {
		return nil
	}
	platform := normalizePlatform(app.Kind)
	employee, _, err := s.resolveEmployee(ctx, app.TenantID, platform)
	if err != nil || employee == nil {
		return err
	}
	return s.decideAndRecord(ctx, gateway, employee, app, ref, platform, messages, true)
}

func (s *RelayService) inboundGateway() InboundGateway {
	s.inboundMu.Lock()
	defer s.inboundMu.Unlock()
	return s.inbound
}

func customerInbound(messages []InboundMessage) []InboundMessage {
	out := make([]InboundMessage, 0, len(messages))
	seen := map[string]struct{}{}
	var conversationID string
	for _, message := range messages {
		if strings.TrimSpace(message.Role) != "customer" {
			continue
		}
		message.ConversationID = strings.TrimSpace(message.ConversationID)
		message.ID = strings.TrimSpace(message.ID)
		if message.ConversationID == "" || message.ID == "" || messageMarker(message.Type, message.Text) == "" {
			continue
		}
		if conversationID == "" {
			conversationID = message.ConversationID
		}
		if message.ConversationID != conversationID {
			continue
		}
		if _, ok := seen[message.ID]; ok {
			continue
		}
		seen[message.ID] = struct{}{}
		out = append(out, message)
	}
	return out
}

func errorsIsNotFound(err error) bool {
	return err != nil && (strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(err.Error(), "不存在"))
}
