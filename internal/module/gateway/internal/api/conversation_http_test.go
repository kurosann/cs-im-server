package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	commonmodule "vane-common/module"

	"cs-im-server/internal/module/gateway/internal/service"
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

func TestConversationAPIGet(t *testing.T) {
	conversations := service.NewConversationService(nil, nil, nil, nil)
	expected := service.Conversation{ConversationKey: "conv_test", AppKey: "app_test", CustomerKey: "cus_test"}
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
		Code string               `json:"code"`
		Data service.Conversation `json:"data"`
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
	conversations := service.NewConversationService(nil, nil, nil, nil)
	conversations.SetForTest(service.Conversation{})
	NewConversationAPI(conversations).Get(&ctx)

	if ctx.code != http.StatusNotFound {
		t.Fatalf("Get() status = %d, want %d", ctx.code, http.StatusNotFound)
	}
}
