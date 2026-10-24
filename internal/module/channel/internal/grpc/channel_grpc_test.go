package grpc

import (
	"context"
	"testing"
	"time"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	gatewayv1 "github.com/kurosann/cs-im-sdk/gateway/v1"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type testingServerStream struct {
	gatewayv1.ChannelGatewayService_StreamMessagesServer
	context context.Context
	events  chan gatewayv1.MessageEventResponse
}

func (stream *testingServerStream) Context() context.Context { return stream.context }
func (stream *testingServerStream) Send(event *gatewayv1.MessageEventResponse) error {
	stream.events <- *event
	return nil
}

type channelGatewayFixture struct {
	appID          string
	conversationID string
	api            *ChannelGatewayGRPCAPI
	realtime       gatewaypkg.RealtimeTestService
	tokenCtx       context.Context
}

func setupChannelGateway(t *testing.T) channelGatewayFixture {
	t.Helper()
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	if err := db.AutoMigrate(
		&iampkg.AccountToken{}, &iampkg.ChannelCredential{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	if err := gatewaypkg.AutoMigrateTestModels(db.DB); err != nil {
		t.Fatalf("migrate gateway models: %v", err)
	}

	services := gatewaypkg.NewTestGatewayServices()
	apps := services.Apps
	conversations := services.Conversations
	messages := services.Messages
	realtime := services.Realtime
	principal := iampkg.Principal{AccountKey: "account_1", TenantID: "tenant_1"}
	app, err := apps.Register(context.Background(), principal, gatewaydto.RegisterAppRequest{
		ServiceKey: "service-1", DisplayName: "Service",
	})
	if err != nil {
		t.Fatalf("register app: %v", err)
	}
	accounts := iampkg.NewAccountService()
	credential, err := accounts.IssueChannelCredential(
		context.Background(),
		iampkg.Principal{
			AccountKey: "staff_1", TenantID: "tenant_1",
			Kind: iampkg.PrincipalKindHuman, Scopes: []string{iampkg.ScopeIAMTokenManage},
		},
		iampkg.IssueChannelCredentialRequest{AppID: app.AppKey, Name: "Test Channel"},
	)
	if err != nil {
		t.Fatalf("issue channel credential: %v", err)
	}
	api := NewChannelGatewayGRPCAPI(
		accounts,
		apps, conversations, messages, realtime,
	)
	tokenContext := metadata.NewIncomingContext(context.Background(), metadata.MD{
		"authorization": []string{"Bearer " + credential.Secret},
	})
	conversation, err := api.EnsureConversation(tokenContext, &gatewayv1.EnsureConversationRequest{
		AppId: app.AppKey, CustomerExternalId: "customer-1", Channel: "web",
	})
	if err != nil {
		t.Fatalf("ensure conversation: %v", err)
	}
	return channelGatewayFixture{
		appID: app.AppKey, conversationID: conversation.ConversationId,
		api: api, realtime: realtime, tokenCtx: tokenContext,
	}
}

func TestChannelGatewayCustomerBoundary(t *testing.T) {
	fixture := setupChannelGateway(t)
	ctx := fixture.tokenCtx

	_, err := fixture.api.EnsureConversation(ctx, &gatewayv1.EnsureConversationRequest{
		AppId: "app_missing", CustomerExternalId: "customer-1", Channel: "web",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown app error = %v, want NotFound", err)
	}

	conversation, err := fixture.api.EnsureConversation(ctx, &gatewayv1.EnsureConversationRequest{
		AppId: fixture.appID, CustomerExternalId: "customer-1", Channel: "web",
	})
	if err != nil {
		t.Fatalf("EnsureConversation() error = %v", err)
	}
	if conversation.Created != false || conversation.ConversationId != fixture.conversationID {
		t.Fatalf("EnsureConversation() = %+v", conversation)
	}

	fetched, err := fixture.api.GetConversation(ctx, &gatewayv1.GetConversationRequest{
		AppId: fixture.appID, ConversationId: fixture.conversationID,
	})
	if err != nil {
		t.Fatalf("GetConversation() error = %v", err)
	}
	if fetched.ConversationId != fixture.conversationID || fetched.CustomerId != conversation.CustomerId {
		t.Fatalf("GetConversation() = %+v", fetched)
	}

	sentAt := time.Now().UTC().Format(time.RFC3339Nano)
	agentRequest := &gatewayv1.SendMessageRequest{
		AppId: fixture.appID, ConversationId: conversation.ConversationId,
		CustomerId: conversation.CustomerId,
		Sender:     &gatewayv1.MessageSender{Role: "agent", Id: fixture.appID},
		Type:       "text", Text: "agent message",
		Source: &gatewayv1.MessageSource{
			AppId: fixture.appID, ExternalId: "agent-1", SentAt: sentAt,
		},
	}
	agent, err := fixture.api.SendMessage(ctx, agentRequest)
	if err != nil {
		t.Fatalf("SendMessage(agent) error = %v", err)
	}
	if agent.Message.Sender.Role != "agent" || agent.Message.Source.ExternalId != "agent-1" {
		t.Fatalf("SendMessage(agent) = %+v", agent)
	}

	customer, err := fixture.api.SendMessage(ctx, &gatewayv1.SendMessageRequest{
		AppId: fixture.appID, ConversationId: conversation.ConversationId,
		CustomerId: conversation.CustomerId,
		Sender:     &gatewayv1.MessageSender{Role: "customer", Id: conversation.CustomerId},
		Type:       "text", Text: "customer message",
		Source: &gatewayv1.MessageSource{
			AppId: fixture.appID, ExternalId: "customer-1", SentAt: sentAt,
		},
	})
	if err != nil {
		t.Fatalf("SendMessage(customer) error = %v", err)
	}
	if customer.Message.Sender.Role != "customer" || customer.Message.Source.ExternalId != "customer-1" {
		t.Fatalf("SendMessage(customer) = %+v", customer)
	}

	page, err := fixture.api.ListMessages(ctx, &gatewayv1.ListMessagesRequest{
		AppId: fixture.appID, ConversationId: conversation.ConversationId, Order: "asc",
	})
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].MessageId != agent.Message.MessageId || page.Items[1].MessageId != customer.Message.MessageId {
		t.Fatalf("ListMessages() = %+v", page)
	}
}

func TestChannelGatewayMediaMessage(t *testing.T) {
	fixture := setupChannelGateway(t)
	ctx := fixture.tokenCtx
	conversation, err := fixture.api.EnsureConversation(ctx, &gatewayv1.EnsureConversationRequest{
		AppId: fixture.appID, CustomerExternalId: "customer-1", Channel: "web",
	})
	if err != nil {
		t.Fatalf("EnsureConversation() error = %v", err)
	}
	sentAt := time.Now().UTC().Format(time.RFC3339Nano)
	content := &gatewayv1.MessageContent{
		Url: "https://cdn.example.com/photo.png", MimeType: "image/png",
		FileName: "photo.png", Size: 1024, Width: 640, Height: 480,
	}

	_, err = fixture.api.SendMessage(ctx, &gatewayv1.SendMessageRequest{
		AppId: fixture.appID, ConversationId: fixture.conversationID,
		CustomerId: conversation.CustomerId,
		Sender:     &gatewayv1.MessageSender{Role: "customer", Id: "cus_1"},
		Type:       "image", Content: content,
		Source: &gatewayv1.MessageSource{
			AppId: fixture.appID, ExternalId: "media-1", SentAt: sentAt,
		},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("SendMessage(mismatched customer) error = %v, want InvalidArgument", err)
	}

	message, err := fixture.api.SendMessage(ctx, &gatewayv1.SendMessageRequest{
		AppId: fixture.appID, ConversationId: fixture.conversationID,
		CustomerId: conversation.CustomerId,
		Sender:     &gatewayv1.MessageSender{Role: "customer", Id: conversation.CustomerId},
		Type:       "image", Content: content,
		Source: &gatewayv1.MessageSource{
			AppId: fixture.appID, ExternalId: "media-1", SentAt: sentAt,
		},
	})
	if err != nil {
		t.Fatalf("SendMessage(image) error = %v", err)
	}
	if message.Message.Content == nil || message.Message.Content.Url != content.Url || message.Message.Content.FileName != content.FileName {
		t.Fatalf("SendMessage(image) content = %+v", message.Message.Content)
	}

	page, err := fixture.api.ListMessages(ctx, &gatewayv1.ListMessagesRequest{
		AppId: fixture.appID, ConversationId: fixture.conversationID, Order: "asc",
	})
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Content == nil || page.Items[0].Content.Url != content.Url {
		t.Fatalf("ListMessages() media content = %+v", page.Items[0].Content)
	}
}
