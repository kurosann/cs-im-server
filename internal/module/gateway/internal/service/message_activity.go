package service

import (
	"context"
	"time"

	"vane-common/pkg/dataaccess"

	"cs-im-server/internal/module/gateway/internal/model"
)

type MessageActivity struct {
	accessor dataaccess.DataAccessor[*model.Message]
	realtime *RealtimeService
}

func NewMessageActivity(realtime *RealtimeService) *MessageActivity {
	return &MessageActivity{realtime: realtime}
}

func (s *MessageActivity) Get(_ context.Context, id string) (model.Message, error) {
	items, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("message_key", id),
		dataaccess.Limit(1),
	}, dataaccess.OrderTypeDesc, 1)
	if err != nil {
		return model.Message{}, err
	}
	if len(items) == 0 {
		return model.Message{}, notFound("message %s not found", id)
	}
	return items[0], nil
}

func (s *MessageActivity) Latest(_ context.Context, conversationID string) (model.Message, error) {
	items, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", conversationID),
		dataaccess.Order("seq", dataaccess.OrderTypeDesc),
		dataaccess.Limit(1),
	}, dataaccess.OrderTypeDesc, 1)
	if err != nil {
		return model.Message{}, err
	}
	if len(items) == 0 {
		return model.Message{}, badRequest("conversation has no message to forward")
	}
	return items[0], nil
}

func (s *MessageActivity) ConversationStats(_ context.Context, conversationID string) (ConversationMessageStats, error) {
	messages, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", conversationID),
	}, dataaccess.OrderTypeAsc, 0)
	if err != nil {
		return ConversationMessageStats{}, err
	}
	stats := ConversationMessageStats{ReplyStatus: "unreplied"}
	for _, message := range messages {
		if message.SenderRole == "customer" && message.Status != "read" {
			stats.UnreadCount++
		}
	}
	if len(messages) != 0 {
		latest := messages[len(messages)-1]
		if latest.SenderRole == "agent" || latest.SenderRole == "ai" {
			stats.ReplyStatus = "replied"
		}
	}
	return stats, nil
}

func (s *MessageActivity) LastReceivedActivity(_ context.Context, conversationID string) (time.Time, error) {
	messages, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", conversationID),
	}, dataaccess.OrderTypeAsc, 0)
	if err != nil {
		return time.Time{}, err
	}
	var latest time.Time
	for _, message := range messages {
		if createdAt := message.CreatedAt(); createdAt.After(latest) {
			latest = createdAt
		}
	}
	if latest.IsZero() {
		return time.Time{}, badRequest("conversation has no activity")
	}
	return latest, nil
}

func (s *MessageActivity) MarkConversationRead(ctx context.Context, conversationID string) (int, error) {
	messages, err := s.listMessages(ctx, []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", conversationID),
	}, dataaccess.OrderTypeAsc, 0)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	markedCount := 0
	for _, message := range messages {
		if message.SenderRole != "customer" || message.Status == "read" {
			continue
		}
		message.Status = "read"
		message.UpdateAt = now
		if err := s.accessor.Save(ctx, &message); err != nil {
			return markedCount, err
		}
		markedCount++
		s.realtime.Publish("message.status_changed", message.MessageKey, messageToDTO(&message))
	}
	return markedCount, nil
}

func (s *MessageActivity) listMessages(ctx context.Context, conditions []dataaccess.QueryCondition, order dataaccess.OrderType, fetchLimit int) ([]model.Message, error) {
	conditions = append(conditions, dataaccess.Order("seq", order))
	if fetchLimit > 0 {
		conditions = append(conditions, dataaccess.Limit(fetchLimit))
	}
	var found []*model.Message
	if err := s.accessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	messages := make([]model.Message, 0, len(found))
	for _, item := range found {
		messages = append(messages, *item)
	}
	return messages, nil
}
