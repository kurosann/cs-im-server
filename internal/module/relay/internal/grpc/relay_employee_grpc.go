package grpc

import (
	"context"

	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/internal/module/relay/internal/service"
	"cs-im-server/pkg/grpcapi"
	relayv1 "github.com/kurosann/cs-im-internal-sdk/relay/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func optionalText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type RelayEmployeeGRPCAPI struct {
	relayv1.UnimplementedRelayEmployeeServiceServer
	Accounts *iampkg.AccountService
	Service  *service.RelayService
}

func NewRelayEmployeeGRPCAPI(accounts *iampkg.AccountService, employeeService *service.RelayService) *RelayEmployeeGRPCAPI {
	return &RelayEmployeeGRPCAPI{Accounts: accounts, Service: employeeService}
}

func (a *RelayEmployeeGRPCAPI) principal(ctx context.Context) (iampkg.Principal, error) {
	item, err := a.Accounts.Authenticate(ctx, grpcapi.AccountToken(ctx))
	if err != nil {
		return iampkg.Principal{}, grpcapi.Error(err)
	}
	if item.AccountKey == "" {
		return iampkg.Principal{}, status.Error(codes.Unauthenticated, "请先登录")
	}
	return item, nil
}

func channelBindingsToProto(items []service.ChannelBinding) []*relayv1.RelayChannelBinding {
	result := make([]*relayv1.RelayChannelBinding, 0, len(items))
	for _, item := range items {
		result = append(result, &relayv1.RelayChannelBinding{Platform: item.Platform, Label: item.Label})
	}
	return result
}

func channelBindingsFromProto(items []*relayv1.RelayChannelBinding) []service.ChannelBinding {
	result := make([]service.ChannelBinding, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, service.ChannelBinding{Platform: item.GetPlatform(), Label: item.GetLabel()})
	}
	return result
}

func employeeToProto(item service.EmployeeView) *relayv1.RelayEmployee {
	return &relayv1.RelayEmployee{
		EmployeeId:     item.EmployeeID,
		Name:           item.Name,
		Persona:        item.Persona,
		Status:         item.Status,
		Mode:           item.Mode,
		Prompt:         item.Prompt,
		History:        int32(item.History),
		AutoReply:      item.AutoReply,
		SelectedDocIds: item.SelectedDocIDs,
		Tools:          item.Tools,
		Channels:       channelBindingsToProto(item.Channels),
	}
}

func executionToProto(item service.EmployeeExecutionView) *relayv1.RelayEmployeeExecution {
	execution := &relayv1.RelayEmployeeExecution{
		Id:                 item.ID,
		EmployeeId:         item.EmployeeID,
		EmployeeName:       item.EmployeeName,
		Platform:           item.Platform,
		AccountKey:         item.AccountKey,
		AccountDisplayName: optionalText(item.AccountDisplayName),
		ProfileId:          item.ProfileID,
		PeerId:             item.PeerID,
		ConversationId:     item.ConversationID,
		PeerDisplayName:    optionalText(item.PeerDisplayName),
		MessageIds:         item.MessageIDs,
		MergedText:         item.MergedText,
		HistoryCount:       int32(item.HistoryCount),
		HistoryAvailable:   item.HistoryAvailable,
		HistoryTruncated:   item.HistoryTruncated,
		Status:             item.Status,
		Action:             optionalText(item.Action),
		ReasonCode:         optionalText(item.ReasonCode),
		Reason:             optionalText(item.Reason),
		ReplyText:          optionalText(item.ReplyText),
		PromptTokens:       int32(item.PromptTokens),
		CompletionTokens:   int32(item.CompletionTokens),
		TotalTokens:        int32(item.TotalTokens),
		Result:             optionalText(item.Result),
		Error:              optionalText(item.Error),
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
	if item.HistoryDiagnostics != nil {
		execution.HistoryDiagnosticsStatus = item.HistoryDiagnostics.Status
		execution.HistoryDiagnosticsRequestedCount = int64(item.HistoryDiagnostics.RequestedCount)
		execution.HistoryDiagnosticsReadCount = int64(item.HistoryDiagnostics.ReadCount)
		execution.HistoryDiagnosticsPages = int64(item.HistoryDiagnostics.Pages)
		execution.HistoryDiagnosticsScannedCount = int64(item.HistoryDiagnostics.ScannedCount)
		execution.HistoryDiagnosticsError = optionalText(item.HistoryDiagnostics.Error)
	}
	if item.CompletedAt != nil {
		execution.CompletedAt = *item.CompletedAt
	}
	return execution
}

func (a *RelayEmployeeGRPCAPI) ListEmployees(ctx context.Context, _ *relayv1.RelayEmployeeListRequest) (*relayv1.RelayEmployeeListResponse, error) {
	principal, err := a.principal(ctx)
	if err != nil {
		return nil, err
	}
	employees, err := a.Service.ListEmployees(ctx, principal.AccountKey, principal.TenantID)
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	result := make([]*relayv1.RelayEmployee, 0, len(employees))
	for _, item := range employees {
		result = append(result, employeeToProto(item))
	}
	return &relayv1.RelayEmployeeListResponse{Items: result}, nil
}

func (a *RelayEmployeeGRPCAPI) CreateEmployee(ctx context.Context, req *relayv1.RelayEmployeeCreateRequest) (*relayv1.RelayEmployeeResponse, error) {
	principal, err := a.principal(ctx)
	if err != nil {
		return nil, err
	}
	employee, err := a.Service.CreateEmployee(ctx, principal.AccountKey, service.CreateEmployeeRequest{Name: req.GetName()})
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayEmployeeResponse{Employee: employeeToProto(employee)}, nil
}

func (a *RelayEmployeeGRPCAPI) UpdateEmployee(ctx context.Context, req *relayv1.RelayEmployeeUpdateRequest) (*relayv1.RelayEmployeeResponse, error) {
	principal, err := a.principal(ctx)
	if err != nil {
		return nil, err
	}
	employee, err := a.Service.UpdateEmployee(ctx, principal.AccountKey, principal.TenantID, req.GetEmployeeId(), service.UpdateEmployeeRequest{
		Name:           req.GetName(),
		Persona:        req.GetPersona(),
		Status:         req.GetStatus(),
		Mode:           req.GetMode(),
		Prompt:         req.GetPrompt(),
		History:        int(req.GetHistory()),
		AutoReply:      req.GetAutoReply(),
		SelectedDocIDs: req.GetSelectedDocIds(),
		Tools:          req.GetTools(),
		Channels:       channelBindingsFromProto(req.GetChannels()),
	})
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayEmployeeResponse{Employee: employeeToProto(employee)}, nil
}

func (a *RelayEmployeeGRPCAPI) DeleteEmployee(ctx context.Context, req *relayv1.RelayEmployeeDeleteRequest) (*relayv1.RelayEmployeeDeleteResponse, error) {
	principal, err := a.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.Service.DeleteEmployee(ctx, principal.AccountKey, req.GetEmployeeId()); err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayEmployeeDeleteResponse{}, nil
}

func (a *RelayEmployeeGRPCAPI) ListEmployeeExecutions(ctx context.Context, req *relayv1.RelayEmployeeExecutionListRequest) (*relayv1.RelayEmployeeExecutionListResponse, error) {
	principal, err := a.principal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := a.Service.ListEmployeeExecutions(ctx, principal.AccountKey, req.GetEmployeeId(), int(req.GetLimit()))
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	result := make([]*relayv1.RelayEmployeeExecution, 0, len(items))
	for _, item := range items {
		result = append(result, executionToProto(item))
	}
	return &relayv1.RelayEmployeeExecutionListResponse{Items: result}, nil
}

func (a *RelayEmployeeGRPCAPI) ListEmployeeChannels(ctx context.Context, _ *relayv1.RelayEmployeeChannelListRequest) (*relayv1.RelayEmployeeChannelListResponse, error) {
	principal, err := a.principal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := a.Service.ListEmployeeChannels(ctx, principal.TenantID)
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayEmployeeChannelListResponse{Items: channelBindingsToProto(items)}, nil
}
