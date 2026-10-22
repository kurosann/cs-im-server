package service

import (
	"context"
	"testing"
	"time"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
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

func TestCreateNotifiesCustomerMessagesOnly(t *testing.T) {
	messages := setupMessageService(t)
	conversations := NewConversationService(nil, nil, nil, NewRealtimeService())
	conversations.SetForTest(Conversation{ConversationKey: "conv_1", CustomerKey: "cus_1", Status: "open", AppKey: "app_1"})
	messages.conversations = conversations
	notified := make(chan string, 2)
	messages.SetCustomerObserver(func(message Message) {
		notified <- message.SenderRole
	})

	if _, err := messages.CreateAIReply(context.Background(), "conv_1", "emp_1", "自动回复"); err != nil {
		t.Fatalf("CreateAIReply() error = %v", err)
	}
	select {
	case role := <-notified:
		t.Fatalf("ai reply notified observer: %s", role)
	case <-time.After(30 * time.Millisecond):
	}

	created, err := messages.Create(context.Background(), gatewaydto.CreateMessageRequest{
		ConversationKey: "conv_1",
		CustomerID:      "cus_1",
		Sender:          gatewaydto.MessageSender{Role: "customer", ID: "cus_1"},
		Type:            "text",
		Text:            "你好",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	select {
	case role := <-notified:
		if role != "customer" {
			t.Fatalf("observer role = %s", role)
		}
	case <-time.After(time.Second):
		t.Fatalf("customer message %s was not observed", created.MessageKey)
	}
	if !messages.RelayEventAllowed(RealtimeEvent{Type: "message.created", Data: gatewaydto.Message{Sender: gatewaydto.MessageSender{Role: "ai"}}}) {
		t.Fatal("relay stream did not allow ai replies")
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
