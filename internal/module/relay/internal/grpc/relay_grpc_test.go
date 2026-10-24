package grpc

import (
	"context"
	"testing"

	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	relayservice "cs-im-server/internal/module/relay/internal/service"
	relayv1 "github.com/kurosann/cs-im-internal-sdk/relay/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"
)

type realtimePublisherStub struct{}

func (realtimePublisherStub) Publish(string, string, any) {}

func setupRelayGRPC(t *testing.T) (*RelayChannelGRPCAPI, string) {
	t.Helper()
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	if err := db.AutoMigrate(&iampkg.AccountToken{}, &iampkg.ChannelCredential{}, &relayservice.Channel{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	accounts := iampkg.NewAccountService()
	manager := iampkg.Principal{
		AccountKey: "staff",
		Name:       "Relay Manager",
		TenantID:   "tenant_1",
		Kind:       iampkg.PrincipalKindHuman,
		Scopes:     []string{iampkg.ScopeAll},
	}
	issued, err := accounts.Issue(context.Background(), manager, iampkg.IssueAccountTokenRequest{
		AccountKey: "relay-manager",
		Name:       "Relay Manager Token",
		Kind:       iampkg.PrincipalKindRelay,
	})
	if err != nil {
		t.Fatalf("issue relay token: %v", err)
	}
	return NewRelayChannelGRPCAPI(
		accounts,
		relayservice.NewRelayService(relayservice.RuntimeDeps{Realtime: realtimePublisherStub{}}),
	), issued.Secret
}

func relayContext(token string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.MD{
		"authorization": []string{"Bearer " + token},
	})
}

func TestRelayChannelGRPCManagement(t *testing.T) {
	api, token := setupRelayGRPC(t)
	ctx := relayContext(token)

	config, err := structpb.NewStruct(map[string]any{"queue": "cs-im"})
	if err != nil {
		t.Fatalf("config struct: %v", err)
	}
	created, err := api.CreateChannel(ctx, &relayv1.CreateChannelRequest{
		Name: "WA relay", Type: "whatsapp", Endpoint: "https://relay.example", Config: config.GetFields(),
	})
	if err != nil {
		t.Fatalf("CreateChannel() error = %v", err)
	}
	channelID := created.Channel.ChannelId
	if channelID == "" || created.Channel.Config["queue"].GetStringValue() != "cs-im" {
		t.Fatalf("CreateChannel() = %+v", created)
	}

	list, err := api.ListChannels(ctx, &relayv1.ListChannelsRequest{})
	if err != nil || len(list.Items) != 1 {
		t.Fatalf("ListChannels() = %+v, error = %v", list, err)
	}
	status, err := api.SetChannelStatus(ctx, &relayv1.SetChannelStatusRequest{ChannelId: channelID, Enabled: true})
	if err != nil || status.Channel.Status != "active" {
		t.Fatalf("SetChannelStatus() = %+v, error = %v", status, err)
	}
	tested, err := api.TestChannel(ctx, &relayv1.TestChannelRequest{ChannelId: channelID})
	if err != nil || tested.Channel.LastTestedAt == "" {
		t.Fatalf("TestChannel() = %+v, error = %v", tested, err)
	}
}

func TestRelayChannelGRPCRequiresToken(t *testing.T) {
	api, _ := setupRelayGRPC(t)
	_, err := api.ListChannels(context.Background(), &relayv1.ListChannelsRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("ListChannels() error = %v, want Unauthenticated", err)
	}
}
