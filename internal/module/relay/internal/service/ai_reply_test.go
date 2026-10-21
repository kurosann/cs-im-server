package service

import (
	"context"
	"strings"
	"testing"
)

type scriptedClient struct {
	replies []string
	users   []string
	err     error
}

func (c *scriptedClient) Complete(_ context.Context, _, user string) (Completion, error) {
	c.users = append(c.users, user)
	if c.err != nil {
		err := c.err
		c.err = nil
		return Completion{}, err
	}
	if len(c.replies) == 0 {
		return Completion{}, errAINotConfigured
	}
	reply := c.replies[0]
	c.replies = c.replies[1:]
	return Completion{Content: reply, PromptTokens: 3, CompletionTokens: 2}, nil
}

func TestAssembleDecisionIncludesPromptKnowledgeAndHistory(t *testing.T) {
	assembled := assembleDecision(DecisionInput{
		Platform:  "WhatsApp",
		Persona:   "耐心",
		Prompt:    "只回答退款问题",
		Knowledge: []KnowledgeFact{{Title: "退款多久到账？", Type: "FAQ", Answer: "1-2 个工作日"}},
		AutoReply: true,
		History: []DecisionMessage{
			{Role: "customer", Text: "你好"},
			{Role: "business", Text: "您好，请说订单号"},
		},
		NewMessages: []DecisionMessage{{Role: "customer", Text: "退款多久到账"}},
	})
	if !strings.Contains(assembled.System, "knowledge") || !strings.Contains(assembled.System, "知识库") {
		t.Fatalf("system prompt missing knowledge rule: %.120s", assembled.System)
	}
	for _, piece := range []string{"只回答退款问题", "回复性格：耐心", "退款多久到账？", "您好，请说订单号", "退款多久到账", `"platform":"whatsapp"`} {
		if !strings.Contains(assembled.User, piece) {
			t.Fatalf("user payload missing %q: %s", piece, assembled.User)
		}
	}
	if assembled.Meta.HistoryCount != 2 || !assembled.Meta.HistoryAvailable || len(assembled.Meta.KnowledgeTitles) != 1 {
		t.Fatalf("meta = %+v", assembled.Meta)
	}
}

func TestDecideRetriesRepeatedReply(t *testing.T) {
	client := &scriptedClient{replies: []string{
		`{"action":"reply","replyText":"hello there","reasonCode":"prompt_supported","reason":"重复回复"}`,
		`{"action":"reply","replyText":"It arrives in 1-2 days","reasonCode":"prompt_supported","reason":"知识库已说明到账时间"}`,
	}}
	result := decideAIReply(context.Background(), client, DecisionInput{
		Platform:    "whatsapp",
		Prompt:      "Refunds arrive in 1-2 business days.",
		History:     []DecisionMessage{{Role: "business", Text: "hello there"}},
		NewMessages: []DecisionMessage{{Role: "customer", Text: "When does the refund arrive?"}},
		AutoReply:   true,
	})
	if result.Failed || result.ReplyText != "It arrives in 1-2 days" || len(client.users) != 2 {
		t.Fatalf("result = %+v calls = %d", result, len(client.users))
	}
	if !strings.Contains(client.users[1], "serverCorrection") {
		t.Fatalf("correction payload = %s", client.users[1])
	}
}

func TestEnqueueInboundIgnoresOwnReply(t *testing.T) {
	service := setupEmployees(t)
	service.EnqueueInbound(InboundMessage{ID: "msg_ai", ConversationID: "conv_1", Role: "ai", Type: "text", Text: "自动回复"})
	service.EnqueueInbound(InboundMessage{ID: "msg_agent", ConversationID: "conv_1", Role: "agent", Type: "text", Text: "人工回复"})
	service.inboundMu.Lock()
	defer service.inboundMu.Unlock()
	if len(service.inboundPending) != 0 {
		t.Fatalf("pending = %+v", service.inboundPending)
	}
}

type fakeInbound struct {
	ref  ConversationRef
	sent []string
}

func (f *fakeInbound) LookupConversation(context.Context, string) (ConversationRef, error) {
	return f.ref, nil
}

func (f *fakeInbound) RecentMessages(context.Context, string, int) ([]TranscriptMessage, error) {
	return []TranscriptMessage{{ID: "old", Role: "ai", Type: "text", Text: "之前回复过", Seq: 1}}, nil
}

func (f *fakeInbound) SendAIReply(_ context.Context, _, _, text string) error {
	f.sent = append(f.sent, text)
	return nil
}

func TestInboundUsesBindingAndSkipsPausedDefault(t *testing.T) {
	ctx := context.Background()
	service := setupEmployees(t)
	if err := service.apps.Create(ctx, &businessAppRow{AppKey: "app_wa", TenantID: "tenant-a", DisplayName: "售前", Kind: "WhatsApp"}); err != nil {
		t.Fatalf("app: %v", err)
	}
	if err := service.apps.Create(ctx, &businessAppRow{AppKey: "app_tg", TenantID: "tenant-a", DisplayName: "电报", Kind: "telegram"}); err != nil {
		t.Fatalf("telegram app: %v", err)
	}
	defaultEmp, err := service.CreateEmployee(ctx, "account-a", "tenant-a", CreateEmployeeRequest{Name: "默认客服"})
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if _, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", defaultEmp.EmployeeID, UpdateEmployeeRequest{
		Name: "默认客服", Status: "running", Mode: "basic", History: 10, AutoReply: true, IsDefault: true, Prompt: "默认回复",
	}); err != nil {
		t.Fatalf("save default: %v", err)
	}
	bound, err := service.CreateEmployee(ctx, "account-b", "tenant-a", CreateEmployeeRequest{Name: "WhatsApp客服"})
	if err != nil {
		t.Fatalf("bound: %v", err)
	}
	if _, err := service.UpdateEmployee(ctx, "account-b", "tenant-a", bound.EmployeeID, UpdateEmployeeRequest{
		Name: "WhatsApp客服", Status: "running", Mode: "basic", History: 10, AutoReply: true, Prompt: "只回答 WhatsApp",
		Channels: []ChannelBinding{{Platform: "whatsapp"}},
	}); err != nil {
		t.Fatalf("bind whatsapp: %v", err)
	}
	paused, err := service.CreateEmployee(ctx, "account-a", "tenant-a", CreateEmployeeRequest{Name: "暂停电报"})
	if err != nil {
		t.Fatalf("paused: %v", err)
	}
	if _, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", paused.EmployeeID, UpdateEmployeeRequest{
		Name: "暂停电报", Status: "paused", Mode: "basic", History: 10, Prompt: "暂停",
		Channels: []ChannelBinding{{Platform: "telegram"}},
	}); err != nil {
		t.Fatalf("bind telegram: %v", err)
	}
	other, err := service.CreateEmployee(ctx, "account-c", "tenant-a", CreateEmployeeRequest{Name: "冲突客服"})
	if err != nil {
		t.Fatalf("other: %v", err)
	}
	if _, err := service.UpdateEmployee(ctx, "account-c", "tenant-a", other.EmployeeID, UpdateEmployeeRequest{
		Name: "冲突客服", Status: "paused", Mode: "basic", History: 10,
		Channels: []ChannelBinding{{Platform: "whatsapp"}},
	}); !isStatus(err, 409) {
		t.Fatalf("tenant binding conflict = %v", err)
	}

	matched, blocked, err := service.resolveEmployee(ctx, "tenant-a", "whatsapp")
	if err != nil || blocked || matched == nil || matched.EmployeeKey != bound.EmployeeID {
		t.Fatalf("whatsapp route = %+v blocked=%v err=%v", matched, blocked, err)
	}
	fallback, blocked, err := service.resolveEmployee(ctx, "tenant-a", "web")
	if err != nil || blocked || fallback == nil || fallback.EmployeeKey != defaultEmp.EmployeeID {
		t.Fatalf("default route = %+v blocked=%v err=%v", fallback, blocked, err)
	}
	pausedMatch, blocked, err := service.resolveEmployee(ctx, "tenant-a", "telegram")
	if err != nil || !blocked || pausedMatch != nil {
		t.Fatalf("paused route = %+v blocked=%v err=%v", pausedMatch, blocked, err)
	}

	client := &scriptedClient{replies: []string{
		`{"action":"reply","replyText":"预计 1-2 个工作日到账","reasonCode":"prompt_supported","reason":"知识库说明了退款到账时间"}`,
	}}
	gateway := &fakeInbound{ref: ConversationRef{AppID: "app_wa", CustomerID: "cus_1", Subject: "退款"}}
	service.SetCompletionClient(client)
	service.SetInboundGateway(gateway)
	if err := service.HandleInboundBatch(ctx, []InboundMessage{
		{ID: "msg_ai_loop", ConversationID: "conv_1", Role: "ai", Type: "text", Text: "不要再次处理"},
		{ID: "msg_customer", ConversationID: "conv_1", Role: "customer", Type: "text", Text: "退款多久到账？"},
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(gateway.sent) != 1 || gateway.sent[0] != "预计 1-2 个工作日到账" {
		t.Fatalf("sent = %+v", gateway.sent)
	}
	if len(client.users) != 1 || !strings.Contains(client.users[0], "只回答 WhatsApp") || !strings.Contains(client.users[0], "之前回复过") {
		t.Fatalf("prompt = %s", strings.Join(client.users, "\n"))
	}
	logs, err := service.ListEmployeeExecutions(ctx, "account-b", bound.EmployeeID, 10)
	if err != nil || len(logs) != 1 || logs[0].Status != "replied" || logs[0].Platform != "whatsapp" {
		t.Fatalf("logs = %+v %v", logs, err)
	}
}

func TestKnowledgeSeedFeedsPublishedDocs(t *testing.T) {
	ctx := context.Background()
	service := setupEmployees(t)
	library, err := service.ListKnowledge(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(library.Docs) != 8 || library.Docs[2].ID != "d3" {
		t.Fatalf("seed = %+v", library.Docs)
	}
	facts, err := service.KnowledgeFacts(ctx, "tenant-a", []string{"d3", "d5", "missing"}, "telegram", true)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}
	if len(facts) != 1 || facts[0].Title != "退款多久到账？" {
		t.Fatalf("facts = %+v", facts)
	}
	webOnly, err := service.KnowledgeFacts(ctx, "tenant-a", []string{"d1"}, "telegram", true)
	if err != nil || len(webOnly) != 0 {
		t.Fatalf("channel filter = %+v %v", webOnly, err)
	}
}
