package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	commonmodule "vane-common/module"

	gatewaymodel "cs-im-server/internal/module/gateway/internal/model"
	"cs-im-server/internal/module/gateway/internal/service"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"
)

type fakeRequestContext struct {
	commonmodule.HttpReqContext
	params map[string]string
	body   *bytes.Buffer
	code   int
}

func (c fakeRequestContext) Context() context.Context { return context.Background() }

func (c fakeRequestContext) Request() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/", nil)
}

func (c fakeRequestContext) Param(key string) string { return c.params[key] }

func (c *fakeRequestContext) JSON(code int, payload any) error {
	c.code = code
	return json.NewEncoder(c.body).Encode(payload)
}

func setupConversationService(t *testing.T) *service.ConversationService {
	t.Helper()

	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	if err := db.AutoMigrate(&gatewaymodel.HistoryRequest{}, &gatewaymodel.Message{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&gatewaymodel.Conversation{},
		&gatewaymodel.HistoryRequest{},
		&gatewaymodel.Message{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return service.NewConversationService(nil, nil, nil, nil)
}

func TestConversationAPIGet(t *testing.T) {
	conversations := setupConversationService(t)
	expected := gatewaymodel.Conversation{ConversationKey: "conv_test", AppKey: "app_test", CustomerKey: "cus_test"}
	conversations.SetForTest(expected)

	ctx := fakeRequestContext{
		params: map[string]string{"id": "conv_test"},
		body:   &bytes.Buffer{},
	}
	NewConversationAPI(conversations).Get(&ctx)

	if ctx.code != http.StatusOK {
		t.Fatalf("Get() status = %d, want %d", ctx.code, http.StatusOK)
	}
	var got struct {
		Code string                    `json:"code"`
		Data gatewaymodel.Conversation `json:"data"`
	}
	if err := json.NewDecoder(ctx.body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != "OK" || got.Data.ConversationKey != expected.ConversationKey {
		t.Fatalf("Get() = %+v, want conversation %s", got, expected.ConversationKey)
	}
}

func TestConversationAPIGetNotFound(t *testing.T) {
	ctx := fakeRequestContext{
		params: map[string]string{"id": "conv_missing"},
		body:   &bytes.Buffer{},
	}
	conversations := setupConversationService(t)
	conversations.SetForTest(gatewaymodel.Conversation{})
	NewConversationAPI(conversations).Get(&ctx)

	if ctx.code != http.StatusNotFound {
		t.Fatalf("Get() status = %d, want %d", ctx.code, http.StatusNotFound)
	}
}
