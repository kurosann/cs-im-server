package service

import (
	"context"
	iamdto "cs-im-server/internal/module/iam/dto"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"errors"
	"time"

	"cs-im-server/internal/module/iam/internal/model"

	"vane-common/pkg/dataaccess"
)

func (s *AccountService) IssueChannelCredential(ctx context.Context, principal iamprincipal.Principal, req iamdto.IssueChannelCredentialRequest) (iamdto.IssueChannelCredentialResult, error) {
	if !principal.HasKind(iamprincipal.KindHuman) {
		return iamdto.IssueChannelCredentialResult{}, forbidden("only staff can manage channel credentials")
	}
	if !principal.HasScope(ScopeIAMTokenManage) {
		return iamdto.IssueChannelCredentialResult{}, forbidden("principal cannot manage channel credentials")
	}
	if req.AppID == "" {
		return iamdto.IssueChannelCredentialResult{}, badRequest("appId is required")
	}
	if req.Name == "" {
		return iamdto.IssueChannelCredentialResult{}, badRequest("name is required")
	}

	existing, err := s.credentials.First(ctx, dataaccess.Eq("app_key", req.AppID), dataaccess.Eq("tenant_id", principal.TenantID))
	if err == nil {
		_ = existing
		return iamdto.IssueChannelCredentialResult{}, badRequest("channel credential already exists")
	}
	if !errors.Is(err, dataaccess.ErrDataNotFound) {
		return iamdto.IssueChannelCredentialResult{}, err
	}

	secret, err := generateToken("csim")
	if err != nil {
		return iamdto.IssueChannelCredentialResult{}, err
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
		return iamdto.IssueChannelCredentialResult{}, err
	}
	s.recordAudit(ctx, principal, "iam.channel", "create", item.AppKey, "success")
	return iamdto.IssueChannelCredentialResult{Credential: channelCredentialInfo(item), Secret: secret}, nil
}

func (s *AccountService) ListChannelCredentials(ctx context.Context, principal iamprincipal.Principal) ([]iamdto.ChannelCredentialInfo, error) {
	if !principal.HasKind(iamprincipal.KindHuman) || !principal.HasScope(ScopeIAMTokenManage) {
		return nil, forbidden("principal cannot manage channel credentials")
	}
	items, err := s.credentials.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, err
	}
	result := make([]iamdto.ChannelCredentialInfo, 0, len(items))
	for _, item := range items {
		result = append(result, channelCredentialInfo(item))
	}
	return result, nil
}

func (s *AccountService) SetChannelCredentialStatus(ctx context.Context, principal iamprincipal.Principal, appID, status string) (iamdto.ChannelCredentialInfo, error) {
	if status != "active" && status != "disabled" && status != "revoked" {
		return iamdto.ChannelCredentialInfo{}, badRequest("unsupported channel credential status")
	}
	item, err := s.accessibleCredential(ctx, principal, appID)
	if err != nil {
		return iamdto.ChannelCredentialInfo{}, err
	}
	item.Status = status
	if status == "revoked" {
		now := time.Now()
		item.RevokedAt = &now
	}
	if err := s.credentials.Update(ctx, item); err != nil {
		return iamdto.ChannelCredentialInfo{}, err
	}
	s.recordAudit(ctx, principal, "iam.channel", "status_change", item.AppKey, "success")
	return channelCredentialInfo(item), nil
}

func (s *AccountService) RotateChannelCredential(ctx context.Context, principal iamprincipal.Principal, appID string) (iamdto.IssueChannelCredentialResult, error) {
	item, err := s.accessibleCredential(ctx, principal, appID)
	if err != nil {
		return iamdto.IssueChannelCredentialResult{}, err
	}
	secret, err := generateToken("csim")
	if err := s.credentials.Update(ctx, item); err != nil {
		return iamdto.IssueChannelCredentialResult{}, err
	}
	item.SecretHash = hashToken(secret)
	item.Status = "active"
	item.RevokedAt = nil
	if err := s.credentials.Update(ctx, item); err != nil {
		return iamdto.IssueChannelCredentialResult{}, err
	}
	s.recordAudit(ctx, principal, "iam.channel", "rotate", item.AppKey, "success")
	return iamdto.IssueChannelCredentialResult{Credential: channelCredentialInfo(item), Secret: secret}, nil
}

func (s *AccountService) DeleteChannelCredential(ctx context.Context, principal iamprincipal.Principal, appID string) error {
	item, err := s.accessibleCredential(ctx, principal, appID)
	if err != nil {
		return err
	}
	if err := s.credentials.Delete(ctx, item); err != nil {
		return err
	}
	s.recordAudit(ctx, principal, "iam.channel", "delete", item.AppKey, "success")
	return nil
}

func (s *AccountService) accessibleCredential(ctx context.Context, principal iamprincipal.Principal, appID string) (*model.ChannelCredential, error) {
	if appID == "" {
		return nil, badRequest("appId is required")
	}
	if !principal.HasKind(iamprincipal.KindHuman) || !principal.HasScope(ScopeIAMTokenManage) {
		return nil, forbidden("principal cannot manage channel credentials")
	}
	item, err := s.credentials.First(ctx, dataaccess.Eq("app_key", appID), dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, notFound("channel credential not found")
	}
	return item, nil
}

func channelCredentialInfo(item *model.ChannelCredential) iamdto.ChannelCredentialInfo {
	return iamdto.ChannelCredentialInfo{
		AppID:      item.AppKey,
		Name:       item.Name,
		TenantID:   item.TenantID,
		Status:     item.Status,
		ExpiresAt:  item.ExpiresAt,
		RevokedAt:  item.RevokedAt,
		LastUsedAt: item.LastUsedAt,
	}
}
