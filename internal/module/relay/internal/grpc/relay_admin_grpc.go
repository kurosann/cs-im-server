package grpc

import (
	"context"
	"time"

	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	relaydto "cs-im-server/internal/module/relay/dto"
	"cs-im-server/internal/module/relay/internal/service"
	"cs-im-server/pkg/grpcapi"
	relayv1 "github.com/kurosann/cs-im-internal-sdk/relay/v1"
)

type RelayWebhookGRPCAPI struct {
	relayv1.UnimplementedRelayWebhookServiceServer
	Accounts *iampkg.AccountService
	Service  *service.RelayService
}

type RelayAIGRPCAPI struct {
	relayv1.UnimplementedRelayAIServiceServer
	Accounts *iampkg.AccountService
	Service  *service.RelayService
}

func NewRelayWebhookGRPCAPI(accounts *iampkg.AccountService, adminService *service.RelayService) *RelayWebhookGRPCAPI {
	return &RelayWebhookGRPCAPI{Accounts: accounts, Service: adminService}
}

func NewRelayAIGRPCAPI(accounts *iampkg.AccountService, adminService *service.RelayService) *RelayAIGRPCAPI {
	return &RelayAIGRPCAPI{Accounts: accounts, Service: adminService}
}

func (a *RelayWebhookGRPCAPI) authorize(ctx context.Context) error {
	_, err := a.Accounts.Authenticate(ctx, grpcapi.AccountToken(ctx))
	return grpcapi.Error(err)
}

func webhookToProto(item service.Webhook) *relayv1.RelayWebhook {
	events := item.Events
	if events == nil {
		events = []string{}
	}
	return &relayv1.RelayWebhook{
		WebhookId: item.WebhookKey,
		Name:      item.Name,
		Url:       item.URL,
		Events:    events,
		Secret:    item.Secret,
		Status:    item.Status,
		CreatedAt: item.BaseModel.CreateAt.Format(time.RFC3339Nano),
		UpdatedAt: item.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	}
}

func webhookFromRequest(req *relayv1.RelayWebhookCreateRequest) relaydto.CreateWebhookRequest {
	return relaydto.CreateWebhookRequest{
		Name:     req.GetName(),
		URL:      req.GetUrl(),
		Events:   req.GetEvents(),
		IsActive: req.GetIsActive(),
	}
}

func webhookFromUpdateRequest(req *relayv1.RelayWebhookUpdateRequest) relaydto.UpdateWebhookRequest {
	return relaydto.UpdateWebhookRequest{
		Name:     req.GetName(),
		URL:      req.GetUrl(),
		Events:   req.GetEvents(),
		IsActive: req.GetIsActive(),
	}
}

func (a *RelayWebhookGRPCAPI) CreateWebhook(ctx context.Context, req *relayv1.RelayWebhookCreateRequest) (*relayv1.RelayWebhookResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	webhook, err := a.Service.CreateWebhook(ctx, webhookFromRequest(req))
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayWebhookResponse{Webhook: webhookToProto(webhook)}, nil
}

func (a *RelayWebhookGRPCAPI) ListWebhooks(ctx context.Context, _ *relayv1.RelayWebhookListRequest) (*relayv1.RelayWebhookListResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	webhooks := a.Service.ListWebhooks(ctx)
	items := make([]*relayv1.RelayWebhook, 0, len(webhooks))
	for _, item := range webhooks {
		items = append(items, webhookToProto(item))
	}
	return &relayv1.RelayWebhookListResponse{Items: items}, nil
}

func (a *RelayWebhookGRPCAPI) GetWebhook(ctx context.Context, req *relayv1.RelayWebhookGetRequest) (*relayv1.RelayWebhookResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	webhook, err := a.Service.GetWebhook(ctx, req.GetWebhookId())
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayWebhookResponse{Webhook: webhookToProto(webhook)}, nil
}

func (a *RelayWebhookGRPCAPI) UpdateWebhook(ctx context.Context, req *relayv1.RelayWebhookUpdateRequest) (*relayv1.RelayWebhookResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	webhook, err := a.Service.UpdateWebhook(ctx, req.GetWebhookId(), webhookFromUpdateRequest(req))
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayWebhookResponse{Webhook: webhookToProto(webhook)}, nil
}

func (a *RelayWebhookGRPCAPI) TestWebhook(ctx context.Context, req *relayv1.RelayWebhookTestRequest) (*relayv1.RelayWebhookResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	webhook, err := a.Service.TestWebhook(ctx, req.GetWebhookId())
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayWebhookResponse{Webhook: webhookToProto(webhook)}, nil
}

func (a *RelayWebhookGRPCAPI) SetWebhookStatus(ctx context.Context, req *relayv1.RelayWebhookSetStatusRequest) (*relayv1.RelayWebhookResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	webhook, err := a.Service.SetWebhookStatus(ctx, req.GetWebhookId(), req.GetEnabled())
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayWebhookResponse{Webhook: webhookToProto(webhook)}, nil
}

func (a *RelayWebhookGRPCAPI) DeleteWebhook(ctx context.Context, req *relayv1.RelayWebhookDeleteRequest) (*relayv1.RelayWebhookDeleteResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	if err := a.Service.DeleteWebhook(ctx, req.GetWebhookId()); err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayWebhookDeleteResponse{}, nil
}

func (a *RelayAIGRPCAPI) authorize(ctx context.Context) error {
	_, err := a.Accounts.Authenticate(ctx, grpcapi.AccountToken(ctx))
	return grpcapi.Error(err)
}

func aiConfigToProto(item service.AIConfig) *relayv1.RelayAIConfig {
	return &relayv1.RelayAIConfig{
		Prompt:              item.Prompt,
		HistoryMessageCount: int32(item.HistoryMessageCount),
		AutoReplyEnabled:    item.AutoReplyEnabled,
		ReportingEnabled:    item.ReportingEnabled,
	}
}

func aiConfigFromRequest(req *relayv1.RelayUpdateAIConfigRequest) service.AIConfig {
	return service.AIConfig{
		Prompt:              req.GetPrompt(),
		HistoryMessageCount: int(req.GetHistoryMessageCount()),
		AutoReplyEnabled:    req.GetAutoReplyEnabled(),
		ReportingEnabled:    req.GetReportingEnabled(),
	}
}

func aiExecutionToProto(item service.AIExecution) *relayv1.RelayAIExecution {
	return &relayv1.RelayAIExecution{
		ExecutionId:    item.ExecutionKey,
		ConversationId: item.ConversationKey,
		MessageId:      item.MessageKey,
		Action:         item.Action,
		Result:         item.Result,
		CreatedAt:      item.BaseModel.CreateAt.Format(time.RFC3339Nano),
	}
}

func (a *RelayAIGRPCAPI) GetAIConfig(ctx context.Context, _ *relayv1.RelayGetAIConfigRequest) (*relayv1.RelayAIConfigResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	item := a.Service.GetAIConfig(ctx)
	return &relayv1.RelayAIConfigResponse{Config: aiConfigToProto(item)}, nil
}

func (a *RelayAIGRPCAPI) UpdateAIConfig(ctx context.Context, req *relayv1.RelayUpdateAIConfigRequest) (*relayv1.RelayAIConfigResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	item := a.Service.UpdateAIConfig(ctx, aiConfigFromRequest(req))
	return &relayv1.RelayAIConfigResponse{Config: aiConfigToProto(item)}, nil
}

func (a *RelayAIGRPCAPI) ListAIExecutions(ctx context.Context, _ *relayv1.RelayAIExecutionListRequest) (*relayv1.RelayAIExecutionListResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	items := a.Service.ListAIExecutions(ctx)
	result := make([]*relayv1.RelayAIExecution, 0, len(items))
	for _, item := range items {
		result = append(result, aiExecutionToProto(item))
	}
	return &relayv1.RelayAIExecutionListResponse{Items: result}, nil
}
