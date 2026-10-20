package service

import (
	"context"
	"testing"

	gatewaymodel "cs-im-server/internal/module/gateway/internal/model"
	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"
)

func setupMessageService(t *testing.T) *MessageService {
	t.Helper()

	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	if err := db.AutoMigrate(&gatewaymodel.Message{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	messages := NewMessageService(nil, nil, NewRealtimeService())
	messages.accessor = dataaccess.DataAccessor[*Message]{}
	return messages
}

func addTestMessage(t *testing.T, messages *MessageService, message *Message) {
	t.Helper()

	if err := messages.accessor.Save(context.Background(), message); err != nil {
		t.Fatalf("save message: %v", err)
	}
	messages.messages[message.MessageKey] = message
	if message.Seq > messages.conversationSeq[message.ConversationKey] {
		messages.conversationSeq[message.ConversationKey] = message.Seq
	}
}

func TestMessageServiceMarkConversationRead(t *testing.T) {
	messages := setupMessageService(t)
	addTestMessage(t, messages, &Message{
		MessageKey:      "msg_customer_unread",
		Seq:             1,
		ConversationKey: "conv_1",
		CustomerKey:     "cus_1",
		SenderRole:      "customer",
		Status:          "received",
	})
	addTestMessage(t, messages, &Message{
		MessageKey:      "msg_agent",
		Seq:             2,
		ConversationKey: "conv_1",
		CustomerKey:     "cus_1",
		SenderRole:      "agent",
		Status:          "sent",
	})
	addTestMessage(t, messages, &Message{
		MessageKey:      "msg_other_conversation",
		Seq:             1,
		ConversationKey: "conv_2",
		CustomerKey:     "cus_2",
		SenderRole:      "customer",
		Status:          "received",
	})

	markedCount, err := messages.MarkConversationRead(context.Background(), "conv_1")
	if err != nil {
		t.Fatalf("MarkConversationRead() error = %v", err)
	}
	if markedCount != 1 {
		t.Fatalf("MarkConversationRead() markedCount = %d, want 1", markedCount)
	}
	if messages.messages["msg_customer_unread"].Status != "read" {
		t.Fatal("MarkConversationRead() did not mark customer message as read")
	}
	if messages.messages["msg_agent"].Status != "sent" {
		t.Fatal("MarkConversationRead() marked agent message")
	}
	if messages.messages["msg_other_conversation"].Status != "received" {
		t.Fatal("MarkConversationRead() marked another conversation")
	}
}
