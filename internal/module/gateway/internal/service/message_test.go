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
	if err := db.AutoMigrate(&gatewaymodel.Conversation{}, &gatewaymodel.Message{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	messages := NewMessageService(nil, nil, NewRealtimeService())
	messages.accessor = dataaccess.DataAccessor[*gatewaymodel.Message]{}
	return messages
}

func addTestMessage(t *testing.T, messages *MessageService, message *gatewaymodel.Message) {
	t.Helper()

	if err := messages.accessor.Save(context.Background(), message); err != nil {
		t.Fatalf("save message: %v", err)
	}
}

func TestChannelEventIncludesBusinessMessages(t *testing.T) {
	messages := setupMessageService(t)
	conversations := NewConversationService(nil, nil, NewRealtimeService(), nil)
	conversations.SetForTest(gatewaymodel.Conversation{ConversationKey: "conv_1", CustomerKey: "cus_1", Status: "open", AppKey: "app_1"})
	messages.conversations = conversations

	allowed := messages.ChannelEventAllowed("app_1", RealtimeEvent{
		Type: "message.created",
		Data: gatewaydto.Message{
			ConversationKey: "conv_1",
			CustomerKey:     "cus_1",
			Sender:          gatewaydto.MessageSender{Role: "customer", ID: "cus_1"},
			Type:            "text",
			Text:            "你好",
		},
	})
	if !allowed {
		t.Fatal("business channel event should allow its own customer message")
	}
}

func TestRelayAgentMessageIsStreamedToBusinessChannel(t *testing.T) {
	messages := setupMessageService(t)
	conversations := NewConversationService(nil, nil, NewRealtimeService(), nil)
	conversations.SetForTest(gatewaymodel.Conversation{ConversationKey: "conv_1", CustomerKey: "cus_1", Status: "open", AppKey: "app_1"})
	messages.conversations = conversations

	created, err := messages.CreateRelayMessage(context.Background(), "relay_1", gatewaydto.CreateMessageRequest{
		ConversationKey: "conv_1",
		Type:            gatewaydto.MessageTypeText,
		Text:            "客服回复",
	})
	if err != nil {
		t.Fatalf("CreateRelayMessage() error = %v", err)
	}
	if created.Sender.Role != gatewaydto.MessageSenderRoleAgent || created.Sender.ID != "relay_1" {
		t.Fatalf("created sender = %+v, want relay agent", created.Sender)
	}
	if created.Status != gatewaydto.MessageStatusSending || created.Source != nil {
		t.Fatalf("created status/source = %q/#%v, want pending channel outbound", created.Status, created.Source)
	}
	if !messages.RelayEventAllowed(RealtimeEvent{Type: "message.created", Data: created}) {
		t.Fatal("agent outbound was not allowed on relay stream")
	}
	if messages.ChannelEventAllowed("app_2", RealtimeEvent{Type: "message.created", Data: created}) {
		t.Fatal("foreign business channel received outbound event")
	}
}

func TestCreateNotifiesCustomerMessagesOnly(t *testing.T) {
	messages := setupMessageService(t)
	conversations := NewConversationService(nil, nil, NewRealtimeService(), nil)
	conversations.SetForTest(gatewaymodel.Conversation{ConversationKey: "conv_1", CustomerKey: "cus_1", Status: "open", AppKey: "app_1"})
	messages.conversations = conversations
	notified := make(chan string, 2)
	messages.SetCustomerObserver(func(message gatewaymodel.Message) {
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
	}, true)
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
	addTestMessage(t, messages, &gatewaymodel.Message{
		MessageKey:      "msg_customer_unread",
		Seq:             1,
		ConversationKey: "conv_1",
		CustomerKey:     "cus_1",
		SenderRole:      "customer",
		Status:          "received",
	})
	addTestMessage(t, messages, &gatewaymodel.Message{
		MessageKey:      "msg_agent",
		Seq:             2,
		ConversationKey: "conv_1",
		CustomerKey:     "cus_1",
		SenderRole:      "agent",
		Status:          "sent",
	})
	addTestMessage(t, messages, &gatewaymodel.Message{
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
	if message, err := messages.Get(context.Background(), "msg_customer_unread"); err != nil || message.Status != "read" {
		t.Fatalf("customer message status from database = %q, error = %v", message.Status, err)
	}
	if message, err := messages.Get(context.Background(), "msg_agent"); err != nil || message.Status != "sent" {
		t.Fatalf("agent message status from database = %q, error = %v", message.Status, err)
	}
	if message, err := messages.Get(context.Background(), "msg_other_conversation"); err != nil || message.Status != "received" {
		t.Fatalf("other conversation status from database = %q, error = %v", message.Status, err)
	}
}

func TestMessagesReadFromDatabaseWithoutServiceCache(t *testing.T) {
	messages := setupMessageService(t)
	conversations := NewConversationService(nil, nil, NewRealtimeService(), nil)
	conversations.SetForTest(gatewaymodel.Conversation{ConversationKey: "conv_1", CustomerKey: "cus_1", Status: "open", AppKey: "app_1"})
	messages.conversations = conversations

	first, err := messages.Create(context.Background(), gatewaydto.CreateMessageRequest{
		ConversationKey: "conv_1",
		CustomerID:      "cus_1",
		Sender:          gatewaydto.MessageSender{Role: "customer", ID: "cus_1"},
		Type:            "text",
		Text:            "first",
	}, true)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := messages.Create(context.Background(), gatewaydto.CreateMessageRequest{
		ConversationKey: "conv_1",
		CustomerID:      "cus_1",
		Sender:          gatewaydto.MessageSender{Role: "agent", ID: "emp_1"},
		Type:            "text",
		Text:            "second",
	}, true)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	restarted := NewMessageService(nil, conversations, NewRealtimeService())
	restarted.accessor = dataaccess.DataAccessor[*gatewaymodel.Message]{}
	found, err := restarted.Get(context.Background(), first.MessageKey)
	if err != nil || found.Text != first.Text {
		t.Fatalf("restarted Get() = %+v, error = %v", found, err)
	}
	latest, err := restarted.Latest(context.Background(), "conv_1")
	if err != nil || latest.MessageKey != second.MessageKey || latest.Seq != second.Seq {
		t.Fatalf("restarted Latest() = %+v, want %q with seq %d", latest, second.MessageKey, second.Seq)
	}
}
