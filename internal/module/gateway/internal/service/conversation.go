package service

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"cs-im-server/internal/module/gateway/internal/model"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	relaypkg "cs-im-server/internal/module/relay/pkg"
)

type ConversationService struct {
	conversationAccessor dataaccess.DataAccessor[*model.Conversation]
	deliveryAccessor     dataaccess.DataAccessor[*relaypkg.Delivery]
	apps                 *AppService
	customers            *CustomerService
	messages             *MessageActivity
	realtime             *RealtimeService
	ensureLocks          map[string]*sync.Mutex
	ensureLocksMu        sync.Mutex
}

type ConversationFilter struct {
	Keyword  string
	AppID    string
	Status   string
	Page     int
	PageSize int
}

type ConversationSummary struct {
	ConversationID string `json:"conversationId"`
	AppID          string `json:"appId"`
	CustomerID     string `json:"customerId"`
	CustomerName   string `json:"customerName"`
	Channel        string `json:"channel"`
	Subject        string `json:"subject"`
	Status         string `json:"status"`
	Assignee       string `json:"assignee"`
	LastMessage    string `json:"lastMessage"`
	LastMessageAt  string `json:"lastMessageAt,omitempty"`
	UnreadCount    int    `json:"unreadCount"`
	ReplyStatus    string `json:"replyStatus"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type ConversationPage struct {
	Items []ConversationSummary `json:"items"`
	Total int64                 `json:"total"`
}

type DeliveryStats struct {
	Total       int        `json:"total"`
	Queued      int        `json:"queued"`
	Delivered   int        `json:"delivered"`
	Failed      int        `json:"failed"`
	LastEventAt *time.Time `json:"lastEventAt,omitempty"`
}

func NewConversationService(apps *AppService, customers *CustomerService, realtime *RealtimeService, messages *MessageActivity) *ConversationService {
	return &ConversationService{
		apps: apps, customers: customers, realtime: realtime, messages: messages,
	}
}

func (s *ConversationService) SetForTest(conversation model.Conversation) {
	if err := s.conversationAccessor.Save(context.Background(), &conversation); err != nil {
		panic(err)
	}
}

func (s *ConversationService) Ensure(ctx context.Context, appID string, req gatewaydto.EnsureConversationRequest) (gatewaydto.Conversation, bool, error) {
	if appID == "" {
		return gatewaydto.Conversation{}, false, badRequest("appId is required")
	}
	if req.CustomerExternalID == "" {
		return gatewaydto.Conversation{}, false, badRequest("customerExternalId is required")
	}
	req.CustomerExternalID = canonicalCustomerExternalID(req.Channel, req.CustomerExternalID)
	ensureMu := s.ensureLockFor(appID, req.CustomerExternalID)
	ensureMu.Lock()
	defer ensureMu.Unlock()

	customer, err := s.customers.EnsureExternal(ctx, appID, gatewaydto.EnsureExternalCustomerRequest{
		ExternalID:  req.CustomerExternalID,
		DisplayName: req.CustomerName,
		Channel:     req.Channel,
	})
	if err != nil {
		return gatewaydto.Conversation{}, false, err
	}
	if err := s.closeDuplicateOpen(ctx, appID, customer.CustomerKey); err != nil {
		return gatewaydto.Conversation{}, false, err
	}

	existing, err := s.GetOpen(ctx, appID, customer.CustomerKey)
	if err != nil {
		return gatewaydto.Conversation{}, false, err
	}
	if existing != nil {
		return conversationToDTO(existing), false, nil
	}

	conversation, err := s.Create(ctx, gatewaydto.CreateConversationRequest{
		AppID:      appID,
		CustomerID: customer.CustomerKey,
		Channel:    req.Channel,
		Subject:    req.Subject,
		Metadata:   req.Metadata,
	})
	if err != nil {
		return gatewaydto.Conversation{}, false, err
	}
	return conversation, true, nil
}

func (s *ConversationService) ensureLockFor(appID, externalID string) *sync.Mutex {
	key := appID + ":" + externalID
	s.ensureLocksMu.Lock()
	defer s.ensureLocksMu.Unlock()
	if s.ensureLocks == nil {
		s.ensureLocks = make(map[string]*sync.Mutex)
	}
	lock, ok := s.ensureLocks[key]
	if !ok {
		lock = &sync.Mutex{}
		s.ensureLocks[key] = lock
	}
	return lock
}

func (s *ConversationService) closeDuplicateOpen(ctx context.Context, appID, customerID string) error {
	conditions := []dataaccess.QueryCondition{
		dataaccess.Eq("app_key", appID),
		dataaccess.Eq("customer_key", customerID),
		dataaccess.Eq("status", "open"),
	}
	var found []*model.Conversation
	if err := s.conversationAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return err
	}
	for index, conversation := range found {
		if index == 0 {
			continue
		}
		now := time.Now().UTC()
		conversation.Status = "closed"
		conversation.UpdateAt = now
		conversation.ClosedAt = &now
		conversation.CloseReason = "重复会话自动合并"
		if err := s.conversationAccessor.Save(ctx, conversation); err != nil {
			return err
		}
		s.realtime.Publish("conversation.closed", conversation.ConversationKey, conversationToDTO(conversation))
	}
	return nil
}

func canonicalCustomerExternalID(channel, externalID string) string {
	externalID = strings.TrimSpace(externalID)
	channel = strings.TrimSpace(strings.ToLower(channel))
	if channel != "wa" && channel != "whatsapp" {
		return externalID
	}
	localPart := strings.Split(externalID, "@")[0]
	localPart = strings.Split(localPart, ":")[0]
	if len(localPart) >= 7 && len(localPart) <= 20 {
		digits := localPart
		for _, char := range localPart {
			if char < '0' || char > '9' {
				return externalID
			}
		}
		return digits
	}
	return externalID
}

func (s *ConversationService) GetCustomerExternalID(ctx context.Context, customerID string) (string, error) {
	if s.customers == nil {
		return "", notFound("customer %s not found", customerID)
	}
	return s.customers.ExternalIDForCustomer(ctx, customerID)
}

func (s *ConversationService) Create(_ context.Context, req gatewaydto.CreateConversationRequest) (gatewaydto.Conversation, error) {
	if req.AppID == "" {
		return gatewaydto.Conversation{}, badRequest("appId is required")
	}
	if req.CustomerID == "" {
		return gatewaydto.Conversation{}, badRequest("customerId is required")
	}
	app, err := s.apps.Get(context.Background(), req.AppID)
	if err != nil {
		return gatewaydto.Conversation{}, err
	}
	if app.Status != "active" {
		return gatewaydto.Conversation{}, badRequest("app %s is disabled", req.AppID)
	}
	if _, err := s.customers.Get(context.Background(), req.CustomerID); err != nil {
		return gatewaydto.Conversation{}, err
	}
	conversation := &model.Conversation{ConversationKey: "conv_" + idutil.GenShortId(), AppKey: req.AppID, CustomerKey: req.CustomerID, Channel: req.Channel, Subject: req.Subject, Status: "open"}
	if req.Metadata != nil {
		conversation.Metadata = req.Metadata
	}
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return gatewaydto.Conversation{}, err
	}
	dto := conversationToDTO(conversation)
	s.realtime.Publish("conversation.created", conversation.ConversationKey, dto)
	return dto, nil
}

func (s *ConversationService) List(_ context.Context, appID, customerID, status string) []model.Conversation {
	conditions := []dataaccess.QueryCondition{}
	if appID != "" {
		conditions = append(conditions, dataaccess.Eq("app_key", appID))
	}
	if customerID != "" {
		conditions = append(conditions, dataaccess.Eq("customer_key", customerID))
	}
	if status != "" {
		conditions = append(conditions, dataaccess.Eq("status", status))
	}
	conversations, err := s.listConversations(context.Background(), conditions...)
	if err != nil {
		return []model.Conversation{}
	}
	return conversations
}

func (s *ConversationService) ListForAdmin(ctx context.Context, principal iamprincipal.Principal, filter ConversationFilter) (ConversationPage, error) {
	allowedApps := make(map[string]struct{})
	appNames := make(map[string]string)
	for _, app := range s.apps.List(ctx, principal) {
		allowedApps[app.AppKey] = struct{}{}
		appNames[app.AppKey] = app.DisplayName
	}
	if filter.AppID != "" {
		if _, ok := allowedApps[filter.AppID]; !ok {
			return ConversationPage{Items: []ConversationSummary{}}, nil
		}
	}

	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	appIDs := make([]string, 0, len(allowedApps))
	for appID := range allowedApps {
		appIDs = append(appIDs, appID)
	}
	conditions := []dataaccess.QueryCondition{dataaccess.In("app_key", appIDs)}
	if filter.AppID != "" {
		conditions = append(conditions, dataaccess.Eq("app_key", filter.AppID))
	}
	if filter.Status != "" && filter.Status != "all" {
		conditions = append(conditions, dataaccess.Eq("status", filter.Status))
	}
	items, err := s.listConversations(ctx, conditions...)
	if err != nil {
		return ConversationPage{}, err
	}
	for index := range items {
		conversation := &items[index]
		if _, ok := allowedApps[conversation.AppKey]; !ok {
			continue
		}
		if filter.AppID != "" && conversation.AppKey != filter.AppID {
			continue
		}
		if filter.Status != "" && filter.Status != "all" && conversation.Status != filter.Status {
			continue
		}
		if keyword != "" {
			searchText := strings.ToLower(strings.Join([]string{
				conversation.ConversationKey,
				conversation.CustomerKey,
				conversation.Subject,
				appNames[conversation.AppKey],
			}, " "))
			if !strings.Contains(searchText, keyword) {
				continue
			}
		}
	}
	lastActivity := make(map[string]time.Time)
	if s.messages != nil {
		for _, conversation := range items {
			if activity, err := s.messages.LastReceivedActivity(ctx, conversation.ConversationKey); err == nil {
				lastActivity[conversation.ConversationKey] = activity
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		left := lastActivity[items[i].ConversationKey]
		if left.IsZero() {
			left = items[i].UpdateAt
		}
		right := lastActivity[items[j].ConversationKey]
		if right.IsZero() {
			right = items[j].UpdateAt
		}
		return left.After(right)
	})
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return ConversationPage{Items: []ConversationSummary{}}, nil
	}
	end := min(start+pageSize, len(items))

	result := ConversationPage{Items: make([]ConversationSummary, 0, end-start), Total: int64(len(items))}
	for _, conversation := range items[start:end] {
		summary := ConversationSummary{
			ConversationID: conversation.ConversationKey,
			AppID:          conversation.AppKey,
			CustomerID:     conversation.CustomerKey,
			Channel:        appNames[conversation.AppKey],
			Subject:        conversation.Subject,
			Status:         conversation.Status,
			Assignee:       conversation.Assignee,
			CreatedAt:      conversation.CreateAt.Format(time.RFC3339Nano),
			UpdatedAt:      conversation.UpdateAt.Format(time.RFC3339Nano),
		}
		if customer, err := s.customers.Get(context.Background(), conversation.CustomerKey); err == nil {
			summary.CustomerName = customer.Name
		}
		if activity, ok := lastActivity[conversation.ConversationKey]; ok {
			summary.LastMessageAt = activity.Format(time.RFC3339Nano)
		}
		if s.messages != nil {
			if message, err := s.messages.Latest(context.Background(), conversation.ConversationKey); err == nil {
				summary.LastMessage = message.Text
			}
			if stats, err := s.messages.ConversationStats(context.Background(), conversation.ConversationKey); err == nil {
				summary.UnreadCount = stats.UnreadCount
				summary.ReplyStatus = stats.ReplyStatus
			}
		}
		result.Items = append(result.Items, summary)
	}
	return result, nil
}

func (s *ConversationService) Get(_ context.Context, id string) (gatewaydto.Conversation, error) {
	conversation, err := s.getByKey(context.Background(), id)
	if err != nil {
		return gatewaydto.Conversation{}, notFound("conversation %s not found", id)
	}
	dto := conversationToDTO(conversation)
	if externalID, externalIDErr := s.GetCustomerExternalID(context.Background(), conversation.CustomerKey); externalIDErr == nil {
		dto.CustomerExternalID = externalID
	}
	return dto, nil
}

func (s *ConversationService) AdminEventAllowed(principal iamprincipal.Principal, event RealtimeEvent) bool {
	if event.Type != "conversation.created" {
		return false
	}
	conversation, ok := event.Data.(gatewaydto.Conversation)
	if !ok || principal.TenantID == "" {
		return false
	}
	app, err := s.apps.Get(context.Background(), conversation.AppKey)
	return err == nil && app.TenantID == principal.TenantID
}

func (s *ConversationService) MarkRead(ctx context.Context, id string) (int, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return 0, err
	}
	if s.messages == nil {
		return 0, nil
	}
	return s.messages.MarkConversationRead(ctx, id)
}

func (s *ConversationService) Assign(_ context.Context, id string, req gatewaydto.AssignConversationRequest) (model.Conversation, error) {
	if req.Assignee == "" {
		return model.Conversation{}, badRequest("assignee is required")
	}
	now := time.Now().UTC()
	conversation, err := s.getByKey(context.Background(), id)
	if err != nil {
		return model.Conversation{}, notFound("conversation %s not found", id)
	}
	if conversation.Status != "open" {
		return model.Conversation{}, badRequest("conversation %s is not open", id)
	}
	conversation.Assignee = req.Assignee
	conversation.UpdateAt = now
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return model.Conversation{}, err
	}
	result := *conversation
	s.realtime.Publish("conversation.assigned", id, conversationToDTO(&result))
	return result, nil
}

func (s *ConversationService) Close(_ context.Context, id string, req gatewaydto.CloseConversationRequest) (model.Conversation, error) {
	now := time.Now().UTC()
	conversation, err := s.getByKey(context.Background(), id)
	if err != nil {
		return model.Conversation{}, notFound("conversation %s not found", id)
	}
	if conversation.Status == "closed" {
		return *conversation, nil
	}
	conversation.Status = "closed"
	conversation.UpdateAt = now
	conversation.ClosedAt = &now
	if req.Reason != "" {
		conversation.CloseReason = req.Reason
	}
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return model.Conversation{}, err
	}
	result := *conversation
	s.realtime.Publish("conversation.closed", id, conversationToDTO(&result))
	return result, nil
}

func (s *ConversationService) Reopen(_ context.Context, id string) (model.Conversation, error) {
	now := time.Now().UTC()
	conversation, err := s.getByKey(context.Background(), id)
	if err != nil {
		return model.Conversation{}, notFound("conversation %s not found", id)
	}
	if conversation.Status == "open" {
		return *conversation, nil
	}
	conversation.Status = "open"
	conversation.UpdateAt = now
	conversation.ClosedAt = nil
	conversation.CloseReason = ""
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return model.Conversation{}, err
	}
	result := *conversation
	s.realtime.Publish("conversation.reopened", id, conversationToDTO(&result))
	return result, nil
}

func (s *ConversationService) Forward(_ context.Context, id string, req gatewaydto.ForwardConversationRequest) (relaypkg.Delivery, error) {
	if req.ChannelID == "" {
		return relaypkg.Delivery{}, badRequest("channelId is required")
	}
	channel, err := relaypkg.Service().GetChannel(context.Background(), req.ChannelID)
	if err != nil {
		return relaypkg.Delivery{}, err
	}
	if channel.Status != "active" {
		return relaypkg.Delivery{}, badRequest("channel %s is disabled", req.ChannelID)
	}
	now := time.Now().UTC()
	conversation, err := s.getByKey(context.Background(), id)
	if err != nil {
		return relaypkg.Delivery{}, notFound("conversation %s not found", id)
	}
	messageID := req.MessageID
	if messageID == "" {
		latest, err := s.messages.Latest(context.Background(), id)
		if err != nil {
			return relaypkg.Delivery{}, err
		}
		messageID = latest.MessageKey
	} else if _, err := s.messages.Get(context.Background(), messageID); err != nil {
		return relaypkg.Delivery{}, err
	}
	delivery := &relaypkg.Delivery{DeliveryKey: "dlv_" + idutil.GenShortId(), ConversationKey: id, MessageKey: messageID, ChannelKey: channel.ChannelKey, Status: "queued"}
	if err := s.deliveryAccessor.Save(context.Background(), delivery); err != nil {
		return relaypkg.Delivery{}, err
	}
	conversation.ForwardedTo = channel.ChannelKey
	conversation.UpdateAt = now
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return relaypkg.Delivery{}, err
	}
	result := *delivery
	s.realtime.Publish("conversation.forwarded", id, conversationToDTO(conversation))
	return result, nil
}

func (s *ConversationService) Deliveries(_ context.Context, conversationID string) []relaypkg.Delivery {
	deliveries, err := s.listDeliveries(context.Background(), dataaccess.Eq("conversation_key", conversationID))
	if err != nil {
		return []relaypkg.Delivery{}
	}
	return deliveries
}

func (s *ConversationService) DeliveryStatsByChannel() map[string]DeliveryStats {
	deliveries, err := s.listDeliveries(context.Background())
	if err != nil {
		return map[string]DeliveryStats{}
	}
	stats := make(map[string]DeliveryStats)
	for _, delivery := range deliveries {
		item := stats[delivery.ChannelKey]
		item.Total++
		switch delivery.Status {
		case "queued":
			item.Queued++
		case "delivered":
			item.Delivered++
		case "failed":
			item.Failed++
		}
		lastEventAt := delivery.UpdateAt
		if item.LastEventAt == nil || lastEventAt.After(*item.LastEventAt) {
			item.LastEventAt = &lastEventAt
		}
		stats[delivery.ChannelKey] = item
	}
	return stats
}

func (s *ConversationService) ConversationCountByApp() map[string]int {
	conversations, err := s.listConversations(context.Background())
	if err != nil {
		return map[string]int{}
	}
	counts := make(map[string]int)
	for _, conversation := range conversations {
		counts[conversation.AppKey]++
	}
	return counts
}

func (s *ConversationService) RetargetCustomer(sourceID, targetID string) error {
	conversations, err := s.listConversations(context.Background(), dataaccess.Eq("customer_key", sourceID))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, conversation := range conversations {
		conversation.CustomerKey = targetID
		conversation.UpdateAt = now
		if err := s.conversationAccessor.Save(context.Background(), &conversation); err != nil {
			return err
		}
	}
	return nil
}

func (s *ConversationService) listConversations(ctx context.Context, conditions ...dataaccess.QueryCondition) ([]model.Conversation, error) {
	conditions = append(conditions, dataaccess.Order("create_at", dataaccess.OrderTypeAsc))
	var found []*model.Conversation
	if err := s.conversationAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	conversations := make([]model.Conversation, 0, len(found))
	for _, item := range found {
		conversations = append(conversations, *item)
	}
	return conversations, nil
}

func (s *ConversationService) listDeliveries(ctx context.Context, conditions ...dataaccess.QueryCondition) ([]relaypkg.Delivery, error) {
	conditions = append(conditions, dataaccess.Order("create_at", dataaccess.OrderTypeAsc))
	var found []*relaypkg.Delivery
	if err := s.deliveryAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	deliveries := make([]relaypkg.Delivery, 0, len(found))
	for _, item := range found {
		deliveries = append(deliveries, *item)
	}
	return deliveries, nil
}

func (s *ConversationService) getByKey(ctx context.Context, id string) (*model.Conversation, error) {
	conditions := []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", id),
		dataaccess.Limit(1),
	}
	var found []*model.Conversation
	if err := s.conversationAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, dataaccess.ErrDataNotFound
	}
	return found[0], nil
}

func (s *ConversationService) GetOpen(ctx context.Context, appID, customerID string) (*model.Conversation, error) {
	conditions := []dataaccess.QueryCondition{
		dataaccess.Eq("app_key", appID),
		dataaccess.Eq("customer_key", customerID),
		dataaccess.Eq("status", "open"),
		dataaccess.Limit(1),
	}
	var found []*model.Conversation
	if err := s.conversationAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}
