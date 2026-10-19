package service

import (
	"context"
	"errors"
	"time"

	"cs-im-server/internal/module/iam/internal/model"

	"vane-common/pkg/dataaccess"
)

func (s *AccountService) IssueChannelCredential(ctx context.Context, principal Principal, req IssueChannelCredentialRequest) (IssueChannelCredentialResult, error) {
	if !principal.HasKind(PrincipalKindHuman) {
		return IssueChannelCredentialResult{}, forbidden("only staff can manage channel credentials")
	}
	if !principal.HasScope(ScopeIAMTokenManage) {
		return IssueChannelCredentialResult{}, forbidden("principal cannot manage channel credentials")
	}
	if req.AppID == "" {
		return IssueChannelCredentialResult{}, badRequest("appId is required")
	}
	if req.Name == "" {
		return IssueChannelCredentialResult{}, badRequest("name is required")
	}

	existing, err := s.credentials.First(ctx, dataaccess.Eq("app_key", req.AppID), dataaccess.Eq("tenant_id", principal.TenantID))
	if err == nil {
		_ = existing
		return IssueChannelCredentialResult{}, badRequest("channel credential already exists")
	}
	if !errors.Is(err, dataaccess.ErrDataNotFound) {
		return IssueChannelCredentialResult{}, err
	}

	secret, err := generateToken("csim")
	if err != nil {
		return IssueChannelCredentialResult{}, err
	}
	item := &model.ChannelCredential{
		AppKey:     req.AppID,
		TenantID:   principal.TenantID,
		Name:       req.Name,
		SecretHash: hashToken(secret),
		Status:     "active",
		ExpiresAt:  req.ExpiresAt,
	}
	if err := s.credentials.Create(ctx, item); err != nil {
		return IssueChannelCredentialResult{}, err
	}
	return IssueChannelCredentialResult{Credential: channelCredentialInfo(item), Secret: secret}, nil
}

func (s *AccountService) ListChannelCredentials(ctx context.Context, principal Principal) ([]ChannelCredentialInfo, error) {
	if !principal.HasKind(PrincipalKindHuman) || !principal.HasScope(ScopeIAMTokenManage) {
		return nil, forbidden("principal cannot manage channel credentials")
	}
	items, err := s.credentials.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, err
	}
	result := make([]ChannelCredentialInfo, 0, len(items))
	for _, item := range items {
		result = append(result, channelCredentialInfo(item))
	}
	return result, nil
}

func (s *AccountService) SetChannelCredentialStatus(ctx context.Context, principal Principal, appID, status string) (ChannelCredentialInfo, error) {
	if status != "active" && status != "disabled" && status != "revoked" {
		return ChannelCredentialInfo{}, badRequest("unsupported channel credential status")
	}
	item, err := s.accessibleCredential(ctx, principal, appID)
	if err != nil {
		return ChannelCredentialInfo{}, err
	}
	item.Status = status
	if status == "revoked" {
		now := time.Now()
		item.RevokedAt = &now
	}
	if err := s.credentials.Update(ctx, item); err != nil {
		return ChannelCredentialInfo{}, err
	}
	return channelCredentialInfo(item), nil
}

func (s *AccountService) RotateChannelCredential(ctx context.Context, principal Principal, appID string) (IssueChannelCredentialResult, error) {
	item, err := s.accessibleCredential(ctx, principal, appID)
	if err != nil {
		return IssueChannelCredentialResult{}, err
	}
	secret, err := generateToken("csim")
	if err := s.credentials.Update(ctx, item); err != nil {
		return IssueChannelCredentialResult{}, err
	}
	item.SecretHash = hashToken(secret)
	item.Status = "active"
	item.RevokedAt = nil
	if err := s.credentials.Update(ctx, item); err != nil {
		return IssueChannelCredentialResult{}, err
	}
	return IssueChannelCredentialResult{Credential: channelCredentialInfo(item), Secret: secret}, nil
}

func (s *AccountService) accessibleCredential(ctx context.Context, principal Principal, appID string) (*model.ChannelCredential, error) {
	if appID == "" {
		return nil, badRequest("appId is required")
	}
	if !principal.HasKind(PrincipalKindHuman) || !principal.HasScope(ScopeIAMTokenManage) {
		return nil, forbidden("principal cannot manage channel credentials")
	}
	item, err := s.credentials.First(ctx, dataaccess.Eq("app_key", appID), dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, notFound("channel credential not found")
	}
	return item, nil
}

func channelCredentialInfo(item *model.ChannelCredential) ChannelCredentialInfo {
	return ChannelCredentialInfo{
		AppID:      item.AppKey,
		Name:       item.Name,
		TenantID:   item.TenantID,
		Status:     item.Status,
		ExpiresAt:  item.ExpiresAt,
		RevokedAt:  item.RevokedAt,
		LastUsedAt: item.LastUsedAt,
	}
}
