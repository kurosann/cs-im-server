package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"
)

const knowledgeStatusPublished = "已发布"

var knowledgeTagPattern = regexp.MustCompile(`<[^>]*>`)

type KnowledgeGroupView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type KnowledgeBaseView struct {
	ID          string `json:"id"`
	GroupID     string `json:"groupId"`
	Name        string `json:"name"`
	Product     string `json:"product"`
	Description string `json:"description"`
}

type KnowledgeDocView struct {
	ID          string   `json:"id"`
	BaseID      string   `json:"baseId"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Language    string   `json:"language"`
	Status      string   `json:"status"`
	Answer      string   `json:"answer"`
	Hits        int      `json:"hits"`
	Channels    []string `json:"channels"`
	EffectiveAt string   `json:"effectiveAt,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
	Source      string   `json:"source,omitempty"`
}

type KnowledgeLibrary struct {
	Groups []KnowledgeGroupView `json:"groups"`
	Bases  []KnowledgeBaseView  `json:"bases"`
	Docs   []KnowledgeDocView   `json:"docs"`
}

type CreateKnowledgeGroupRequest struct {
	Name string `json:"name"`
}

type CreateKnowledgeBaseRequest struct {
	GroupID     string `json:"groupId"`
	Name        string `json:"name"`
	Product     string `json:"product"`
	Description string `json:"description"`
}

type SaveKnowledgeDocRequest struct {
	BaseID      string   `json:"baseId"`
	Title       string   `json:"title"`
	Answer      string   `json:"answer"`
	Type        string   `json:"type"`
	Language    string   `json:"language"`
	Status      string   `json:"status"`
	Channels    []string `json:"channels"`
	EffectiveAt string   `json:"effectiveAt"`
	Source      string   `json:"source"`
}

func (s *RelayService) ListKnowledge(ctx context.Context, tenantID string) (KnowledgeLibrary, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return KnowledgeLibrary{}, unauthorized("请先登录")
	}
	if err := s.ensureKnowledgeSeed(ctx, tenantID); err != nil {
		return KnowledgeLibrary{}, err
	}
	groups, err := s.knowledgeGroups.List(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Order("id", dataaccess.OrderTypeAsc))
	if err != nil {
		return KnowledgeLibrary{}, err
	}
	bases, err := s.knowledgeBases.List(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Order("id", dataaccess.OrderTypeAsc))
	if err != nil {
		return KnowledgeLibrary{}, err
	}
	docs, err := s.knowledgeDocs.List(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Order("id", dataaccess.OrderTypeAsc))
	if err != nil {
		return KnowledgeLibrary{}, err
	}
	library := KnowledgeLibrary{
		Groups: make([]KnowledgeGroupView, 0, len(groups)),
		Bases:  make([]KnowledgeBaseView, 0, len(bases)),
		Docs:   make([]KnowledgeDocView, 0, len(docs)),
	}
	for _, group := range groups {
		library.Groups = append(library.Groups, KnowledgeGroupView{ID: group.GroupKey, Name: group.Name})
	}
	for _, base := range bases {
		library.Bases = append(library.Bases, KnowledgeBaseView{
			ID: base.BaseKey, GroupID: base.GroupKey, Name: base.Name, Product: base.Product, Description: base.Description,
		})
	}
	for _, doc := range docs {
		library.Docs = append(library.Docs, knowledgeDocView(doc))
	}
	return library, nil
}

func (s *RelayService) CreateKnowledgeGroup(ctx context.Context, tenantID string, req CreateKnowledgeGroupRequest) (KnowledgeGroupView, error) {
	tenantID = strings.TrimSpace(tenantID)
	name := strings.TrimSpace(req.Name)
	if tenantID == "" {
		return KnowledgeGroupView{}, unauthorized("请先登录")
	}
	if name == "" {
		return KnowledgeGroupView{}, badRequest("分组名称不能为空")
	}
	if utf8.RuneCountInString(name) > 100 {
		return KnowledgeGroupView{}, badRequest("分组名称不能超过 100 个字")
	}
	if err := s.ensureKnowledgeNameAvailable(ctx, tenantID, name); err != nil {
		return KnowledgeGroupView{}, err
	}
	item := &KnowledgeGroup{GroupKey: "g_" + idutil.GenShortId(), TenantID: tenantID, Name: name}
	if err := s.knowledgeGroups.Create(ctx, item); err != nil {
		return KnowledgeGroupView{}, err
	}
	return KnowledgeGroupView{ID: item.GroupKey, Name: item.Name}, nil
}

func (s *RelayService) CreateKnowledgeBase(ctx context.Context, tenantID string, req CreateKnowledgeBaseRequest) (KnowledgeBaseView, error) {
	tenantID = strings.TrimSpace(tenantID)
	name := strings.TrimSpace(req.Name)
	if tenantID == "" {
		return KnowledgeBaseView{}, unauthorized("请先登录")
	}
	if name == "" {
		return KnowledgeBaseView{}, badRequest("知识库名称不能为空")
	}
	if _, err := s.findKnowledgeGroup(ctx, tenantID, req.GroupID); err != nil {
		return KnowledgeBaseView{}, err
	}
	item := &KnowledgeBase{
		BaseKey: "kb_" + idutil.GenShortId(), TenantID: tenantID, GroupKey: strings.TrimSpace(req.GroupID),
		Name: name, Product: strings.TrimSpace(req.Product), Description: strings.TrimSpace(req.Description),
	}
	if item.Product == "" {
		item.Product = "自定义产品"
	}
	if item.Description == "" {
		item.Description = "用于维护团队共享知识。"
	}
	if err := s.knowledgeBases.Create(ctx, item); err != nil {
		return KnowledgeBaseView{}, err
	}
	return KnowledgeBaseView{ID: item.BaseKey, GroupID: item.GroupKey, Name: item.Name, Product: item.Product, Description: item.Description}, nil
}

func (s *RelayService) CreateKnowledgeDoc(ctx context.Context, tenantID string, req SaveKnowledgeDocRequest) (KnowledgeDocView, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return KnowledgeDocView{}, unauthorized("请先登录")
	}
	if _, err := s.findKnowledgeBase(ctx, tenantID, req.BaseID); err != nil {
		return KnowledgeDocView{}, err
	}
	normalized, err := normalizeKnowledgeDoc(req)
	if err != nil {
		return KnowledgeDocView{}, err
	}
	item := &KnowledgeDoc{
		DocKey: "d_" + idutil.GenShortId(), TenantID: tenantID, BaseKey: strings.TrimSpace(req.BaseID),
		Title: normalized.Title, DocType: normalized.Type, Language: normalized.Language, Status: normalized.Status,
		Answer: normalized.Answer, Channels: normalized.Channels, EffectiveAt: normalized.EffectiveAt,
		UpdatedOn: time.Now().Format("2006-01-02"), Source: normalized.Source,
	}
	if err := s.knowledgeDocs.Create(ctx, item); err != nil {
		return KnowledgeDocView{}, err
	}
	return knowledgeDocView(item), nil
}

func (s *RelayService) UpdateKnowledgeDoc(ctx context.Context, tenantID, docID string, req SaveKnowledgeDocRequest) (KnowledgeDocView, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return KnowledgeDocView{}, unauthorized("请先登录")
	}
	item, err := s.findKnowledgeDoc(ctx, tenantID, docID)
	if err != nil {
		return KnowledgeDocView{}, err
	}
	normalized, err := normalizeKnowledgeDoc(req)
	if err != nil {
		return KnowledgeDocView{}, err
	}
	item.Title = normalized.Title
	item.DocType = normalized.Type
	item.Language = normalized.Language
	item.Status = normalized.Status
	item.Answer = normalized.Answer
	item.Channels = normalized.Channels
	item.EffectiveAt = normalized.EffectiveAt
	item.Source = normalized.Source
	item.UpdatedOn = time.Now().Format("2006-01-02")
	if err := s.knowledgeDocs.Update(ctx, item); err != nil {
		return KnowledgeDocView{}, err
	}
	return knowledgeDocView(item), nil
}

func (s *RelayService) KnowledgeFacts(ctx context.Context, tenantID string, docIDs []string, platform string, filterChannel bool) ([]KnowledgeFact, error) {
	tenantID = strings.TrimSpace(tenantID)
	ids := normalizeStringList(docIDs)
	if tenantID == "" || len(ids) == 0 {
		return []KnowledgeFact{}, nil
	}
	if err := s.ensureKnowledgeSeed(ctx, tenantID); err != nil {
		return nil, err
	}
	docs, err := s.knowledgeDocs.List(ctx, dataaccess.Eq("tenant_id", tenantID))
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	byID := make(map[string]*KnowledgeDoc, len(docs))
	for _, doc := range docs {
		byID[doc.DocKey] = doc
	}
	facts := make([]KnowledgeFact, 0, len(ids))
	for _, id := range ids {
		if _, ok := wanted[id]; !ok {
			continue
		}
		doc := byID[id]
		if doc == nil || doc.Status != knowledgeStatusPublished {
			continue
		}
		if filterChannel && !knowledgeChannelMatches(doc.Channels, platform) {
			continue
		}
		facts = append(facts, KnowledgeFact{
			Title:  doc.Title,
			Type:   doc.DocType,
			Answer: knowledgePlainText(doc.Answer),
		})
	}
	return facts, nil
}

func (s *RelayService) ensureKnowledgeSeed(ctx context.Context, tenantID string) error {
	count, err := s.knowledgeGroups.Count(ctx, dataaccess.Eq("tenant_id", tenantID))
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.knowledgeGroups.Transaction(ctx, func(txCtx context.Context) error {
		again, err := s.knowledgeGroups.Count(txCtx, dataaccess.Eq("tenant_id", tenantID))
		if err != nil {
			return err
		}
		if again > 0 {
			return nil
		}
		groups, bases, docs := seedKnowledge(tenantID)
		for _, group := range groups {
			if err := s.knowledgeGroups.Create(txCtx, group); err != nil {
				return err
			}
		}
		for _, base := range bases {
			if err := s.knowledgeBases.Create(txCtx, base); err != nil {
				return err
			}
		}
		for _, doc := range docs {
			if err := s.knowledgeDocs.Create(txCtx, doc); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *RelayService) ensureKnowledgeNameAvailable(ctx context.Context, tenantID, name string) error {
	_, err := s.knowledgeGroups.First(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Eq("name", name))
	if err == nil {
		return conflict("分组名称已存在")
	}
	if errors.Is(err, dataaccess.ErrDataNotFound) {
		return nil
	}
	return err
}

func (s *RelayService) findKnowledgeGroup(ctx context.Context, tenantID, groupID string) (*KnowledgeGroup, error) {
	item, err := s.knowledgeGroups.First(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Eq("group_key", strings.TrimSpace(groupID)))
	if err != nil {
		if errors.Is(err, dataaccess.ErrDataNotFound) {
			return nil, notFound("知识分组不存在")
		}
		return nil, err
	}
	return item, nil
}

func (s *RelayService) findKnowledgeBase(ctx context.Context, tenantID, baseID string) (*KnowledgeBase, error) {
	item, err := s.knowledgeBases.First(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Eq("base_key", strings.TrimSpace(baseID)))
	if err != nil {
		if errors.Is(err, dataaccess.ErrDataNotFound) {
			return nil, notFound("知识库不存在")
		}
		return nil, err
	}
	return item, nil
}

func (s *RelayService) findKnowledgeDoc(ctx context.Context, tenantID, docID string) (*KnowledgeDoc, error) {
	item, err := s.knowledgeDocs.First(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Eq("doc_key", strings.TrimSpace(docID)))
	if err != nil {
		if errors.Is(err, dataaccess.ErrDataNotFound) {
			return nil, notFound("知识文档不存在")
		}
		return nil, err
	}
	return item, nil
}

func normalizeKnowledgeDoc(req SaveKnowledgeDocRequest) (SaveKnowledgeDocRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Answer = strings.TrimSpace(req.Answer)
	req.Type = strings.TrimSpace(req.Type)
	req.Language = strings.TrimSpace(req.Language)
	req.Status = strings.TrimSpace(req.Status)
	req.Source = strings.TrimSpace(req.Source)
	req.EffectiveAt = strings.TrimSpace(req.EffectiveAt)
	if req.Title == "" || req.Answer == "" {
		return SaveKnowledgeDocRequest{}, badRequest("标题和答案不能为空")
	}
	if req.Type == "" {
		req.Type = "FAQ"
	}
	switch req.Type {
	case "FAQ", "政策", "规则", "模板":
	default:
		return SaveKnowledgeDocRequest{}, badRequest("不支持的文档类型")
	}
	if req.Language == "" {
		req.Language = "中文"
	}
	switch req.Language {
	case "中文", "English", "Español":
	default:
		return SaveKnowledgeDocRequest{}, badRequest("不支持的文档语言")
	}
	if req.Status == "" {
		req.Status = "草稿"
	}
	switch req.Status {
	case "已发布", "待审核", "草稿":
	default:
		return SaveKnowledgeDocRequest{}, badRequest("不支持的文档状态")
	}
	if req.Source == "" {
		req.Source = "手动问答对"
	}
	req.Channels = normalizeStringList(req.Channels)
	return req, nil
}

func knowledgeDocView(doc *KnowledgeDoc) KnowledgeDocView {
	updated := doc.UpdatedOn
	if updated == "" && !doc.UpdateAt.IsZero() {
		updated = doc.UpdateAt.Format("2006-01-02")
	}
	return KnowledgeDocView{
		ID: doc.DocKey, BaseID: doc.BaseKey, Title: doc.Title, Type: doc.DocType, Language: doc.Language,
		Status: doc.Status, Answer: doc.Answer, Hits: doc.Hits, Channels: stringList(doc.Channels),
		EffectiveAt: doc.EffectiveAt, UpdatedAt: updated, Source: doc.Source,
	}
}

func knowledgeChannelMatches(channels []string, platform string) bool {
	platform = normalizePlatform(platform)
	if len(channels) == 0 || platform == "" {
		return true
	}
	for _, channel := range channels {
		if normalizeKnowledgeChannel(channel) == platform {
			return true
		}
	}
	return false
}

func normalizeKnowledgeChannel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "whatsapp":
		return "whatsapp"
	case "telegram":
		return "telegram"
	case "web", "web chat", "webchat":
		return "web"
	case "messenger":
		return "messenger"
	default:
		return normalizePlatform(value)
	}
}

func knowledgePlainText(value string) string {
	value = knowledgeTagPattern.ReplaceAllString(value, " ")
	value = strings.ReplaceAll(value, "&nbsp;", " ")
	return strings.Join(strings.Fields(value), " ")
}
