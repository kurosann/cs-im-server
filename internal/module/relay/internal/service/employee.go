package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/idutil"
)

const (
	employeeStatusRunning     = "running"
	employeeStatusPaused      = "paused"
	employeeModeBasic         = "basic"
	employeeModeProfessional  = "professional"
	employeeHistoryDefault    = 10
	employeeHistoryMin        = 1
	employeeHistoryMax        = 50
	employeeNameMaxRunes      = 50
	employeeDeletedNamePrefix = "__deleted__:"
	channelBindingAccountKey  = "*"
)

var channelDisplayNames = map[string]string{
	"whatsapp":  "WhatsApp",
	"telegram":  "Telegram",
	"web":       "Web Chat",
	"messenger": "Messenger",
}

type ChannelBinding struct {
	Platform string `json:"platform"`
	Label    string `json:"label,omitempty"`
}

type EmployeeView struct {
	EmployeeID     string           `json:"employeeId"`
	Name           string           `json:"name"`
	Persona        string           `json:"persona"`
	Status         string           `json:"status"`
	Mode           string           `json:"mode"`
	Prompt         string           `json:"prompt"`
	History        int              `json:"history"`
	AutoReply      bool             `json:"autoReply"`
	IsDefault      bool             `json:"isDefault"`
	SelectedDocIDs []string         `json:"selectedDocIds"`
	Tools          []string         `json:"tools"`
	Channels       []ChannelBinding `json:"channels"`
}

type CreateEmployeeRequest struct {
	Name string `json:"name"`
}

type UpdateEmployeeRequest struct {
	Name           string           `json:"name"`
	Persona        string           `json:"persona"`
	Status         string           `json:"status"`
	Mode           string           `json:"mode"`
	Prompt         string           `json:"prompt"`
	History        int              `json:"history"`
	AutoReply      bool             `json:"autoReply"`
	IsDefault      bool             `json:"isDefault"`
	SelectedDocIDs []string         `json:"selectedDocIds"`
	Tools          []string         `json:"tools"`
	Channels       []ChannelBinding `json:"channels"`
}

func (s *RelayService) ListEmployees(ctx context.Context, accountKey, tenantID string) ([]EmployeeView, error) {
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" {
		return nil, unauthorized("请先登录")
	}
	items, err := s.employees.List(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.IsNull("deleted_at"),
		dataaccess.Order("create_at", dataaccess.OrderTypeAsc),
		dataaccess.Order("id", dataaccess.OrderTypeAsc),
	)
	if err != nil {
		return nil, err
	}
	labels, err := s.channelLabels(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]EmployeeView, 0, len(items))
	for _, item := range items {
		bindings, err := s.bindingsFor(ctx, accountKey, item.EmployeeKey, labels)
		if err != nil {
			return nil, err
		}
		views = append(views, employeeView(item, bindings))
	}
	return views, nil
}

func (s *RelayService) ListEmployeeChannels(ctx context.Context, tenantID string) ([]ChannelBinding, error) {
	return s.channelOptions(ctx, strings.TrimSpace(tenantID))
}

func (s *RelayService) CreateEmployee(ctx context.Context, accountKey, tenantID string, req CreateEmployeeRequest) (EmployeeView, error) {
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" {
		return EmployeeView{}, unauthorized("请先登录")
	}
	name, nameKey, err := normalizeEmployeeName(req.Name)
	if err != nil {
		return EmployeeView{}, err
	}
	if err := s.ensureEmployeeNameAvailable(ctx, accountKey, nameKey, ""); err != nil {
		return EmployeeView{}, err
	}
	item := &AIEmployee{
		EmployeeKey:         "emp_" + idutil.GenShortId(),
		AccountKey:          accountKey,
		TenantID:            strings.TrimSpace(tenantID),
		Name:                name,
		NameKey:             nameKey,
		Mode:                employeeModeBasic,
		HistoryMessageCount: employeeHistoryDefault,
		AutoReplyEnabled:    true,
		Status:              employeeStatusPaused,
		SelectedDocIDs:      []string{},
		Tools:               []string{},
	}
	if err := s.employees.Create(ctx, item); err != nil {
		if isDuplicateKey(err) {
			return EmployeeView{}, conflict("名称已存在")
		}
		return EmployeeView{}, err
	}
	return employeeView(item, []ChannelBinding{}), nil
}

func (s *RelayService) UpdateEmployee(ctx context.Context, accountKey, tenantID, employeeID string, req UpdateEmployeeRequest) (EmployeeView, error) {
	accountKey = strings.TrimSpace(accountKey)
	employeeID = strings.TrimSpace(employeeID)
	if accountKey == "" {
		return EmployeeView{}, unauthorized("请先登录")
	}
	name, nameKey, err := normalizeEmployeeName(req.Name)
	if err != nil {
		return EmployeeView{}, err
	}
	mode, err := normalizeEmployeeMode(req.Mode)
	if err != nil {
		return EmployeeView{}, err
	}
	status, err := normalizeEmployeeStatus(req.Status)
	if err != nil {
		return EmployeeView{}, err
	}
	if req.History < employeeHistoryMin || req.History > employeeHistoryMax {
		return EmployeeView{}, badRequest("上下文条数需要在 1 到 50 之间")
	}
	persona := strings.TrimSpace(req.Persona)
	if utf8.RuneCountInString(persona) > 255 {
		return EmployeeView{}, badRequest("人设不能超过 255 个字")
	}
	docs := normalizeStringList(req.SelectedDocIDs)
	tools := normalizeStringList(req.Tools)
	options, err := s.channelOptions(ctx, tenantID)
	if err != nil {
		return EmployeeView{}, err
	}
	desired, err := normalizeBindings(req.Channels, options)
	if err != nil {
		return EmployeeView{}, err
	}

	var saved *AIEmployee
	err = s.employees.Transaction(ctx, func(txCtx context.Context) error {
		item, err := s.findEmployee(txCtx, accountKey, employeeID)
		if err != nil {
			return err
		}
		if err := s.ensureEmployeeNameAvailable(txCtx, accountKey, nameKey, employeeID); err != nil {
			return err
		}
		if strings.TrimSpace(item.TenantID) == "" {
			item.TenantID = strings.TrimSpace(tenantID)
		}
		if tenantID == "" {
			tenantID = item.TenantID
		}
		if err := s.ensureBindingsAvailable(txCtx, accountKey, tenantID, employeeID, desired); err != nil {
			return err
		}
		if req.IsDefault {
			if err := s.clearDefaultEmployees(txCtx, tenantID, employeeID); err != nil {
				return err
			}
		}
		if err := s.replaceBindings(txCtx, accountKey, tenantID, employeeID, desired); err != nil {
			return err
		}
		item.Name = name
		item.NameKey = nameKey
		item.Persona = persona
		item.Mode = mode
		item.Prompt = req.Prompt
		item.HistoryMessageCount = req.History
		item.AutoReplyEnabled = req.AutoReply
		item.IsDefault = req.IsDefault
		item.Status = status
		item.SelectedDocIDs = docs
		item.Tools = tools
		if err := s.employees.Update(txCtx, item); err != nil {
			if isDuplicateKey(err) {
				return conflict("名称已存在")
			}
			return err
		}
		saved = item
		return nil
	})
	if err != nil {
		return EmployeeView{}, err
	}
	labels := labelIndex(options)
	return employeeView(saved, bindingsWithLabels(desired, labels)), nil
}

func (s *RelayService) DeleteEmployee(ctx context.Context, accountKey, employeeID string) error {
	accountKey = strings.TrimSpace(accountKey)
	employeeID = strings.TrimSpace(employeeID)
	if accountKey == "" {
		return unauthorized("请先登录")
	}
	return s.employees.Transaction(ctx, func(txCtx context.Context) error {
		item, err := s.findEmployee(txCtx, accountKey, employeeID)
		if err != nil {
			return err
		}
		if err := s.replaceBindings(txCtx, accountKey, item.TenantID, employeeID, nil); err != nil {
			return err
		}
		now := time.Now()
		item.DeletedAt = &now
		item.NameKey = employeeDeletedNamePrefix + item.EmployeeKey
		item.SelectedDocIDs = []string{}
		item.Tools = []string{}
		return s.employees.Update(txCtx, item)
	})
}

func (s *RelayService) findEmployee(ctx context.Context, accountKey, employeeID string) (*AIEmployee, error) {
	item, err := s.employees.First(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.Eq("employee_key", employeeID),
		dataaccess.IsNull("deleted_at"),
	)
	if err != nil {
		if errors.Is(err, dataaccess.ErrDataNotFound) {
			return nil, notFound("AI 客服不存在")
		}
		return nil, err
	}
	return item, nil
}

func (s *RelayService) ensureEmployeeNameAvailable(ctx context.Context, accountKey, nameKey, exceptID string) error {
	item, err := s.employees.First(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.Eq("name_key", nameKey),
		dataaccess.IsNull("deleted_at"),
	)
	if errors.Is(err, dataaccess.ErrDataNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if item.EmployeeKey != exceptID {
		return conflict("名称已存在")
	}
	return nil
}

func (s *RelayService) ensureBindingsAvailable(ctx context.Context, accountKey, tenantID, employeeID string, desired []ChannelBinding) error {
	for _, binding := range desired {
		rows, err := s.bindings.List(ctx, dataaccess.Eq("platform", binding.Platform))
		if err != nil {
			return err
		}
		for _, current := range rows {
			if current.EmployeeKey == employeeID {
				continue
			}
			sameAccount := current.AccountKey == accountKey
			sameTenant := tenantID != "" && current.TenantID == tenantID
			if sameAccount || sameTenant {
				return conflict("渠道 %s 已绑定到其他 AI 客服", binding.displayName())
			}
		}
	}
	return nil
}

func (s *RelayService) replaceBindings(ctx context.Context, accountKey, tenantID, employeeID string, desired []ChannelBinding) error {
	current, err := s.bindings.List(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.Eq("employee_key", employeeID),
	)
	if err != nil {
		return err
	}
	for _, binding := range current {
		if err := s.bindings.Delete(ctx, binding); err != nil {
			return err
		}
	}
	for _, binding := range desired {
		row := &AIEmployeeBinding{
			AccountKey:      accountKey,
			TenantID:        tenantID,
			EmployeeKey:     employeeID,
			Platform:        binding.Platform,
			BoundAccountKey: channelBindingAccountKey,
		}
		if err := s.bindings.Create(ctx, row); err != nil {
			if isDuplicateKey(err) {
				return conflict("渠道 %s 已绑定到其他 AI 客服", binding.displayName())
			}
			return err
		}
	}
	return nil
}

func (s *RelayService) bindingsFor(ctx context.Context, accountKey, employeeID string, labels map[string]string) ([]ChannelBinding, error) {
	rows, err := s.bindings.List(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.Eq("employee_key", employeeID),
		dataaccess.Order("platform", dataaccess.OrderTypeAsc),
		dataaccess.Order("account_key_binding", dataaccess.OrderTypeAsc),
	)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(rows))
	out := make([]ChannelBinding, 0, len(rows))
	for _, row := range rows {
		platform := normalizePlatform(row.Platform)
		if platform == "" {
			continue
		}
		if _, ok := seen[platform]; ok {
			continue
		}
		seen[platform] = struct{}{}
		out = append(out, ChannelBinding{Platform: platform, Label: channelLabel(platform, labels[platform])})
	}
	return out, nil
}

func (s *RelayService) channelLabels(ctx context.Context, tenantID string) (map[string]string, error) {
	options, err := s.channelOptions(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return labelIndex(options), nil
}

func (s *RelayService) channelOptions(ctx context.Context, tenantID string) ([]ChannelBinding, error) {
	channels, err := s.channelAccessor.List(ctx)
	if err != nil {
		return nil, err
	}
	var apps []*businessAppRow
	if tenantID != "" {
		apps, err = s.apps.List(ctx, dataaccess.Eq("tenant_id", tenantID))
		if err != nil {
			return nil, err
		}
	}
	seen := make(map[string]ChannelBinding, len(channels)+len(apps))
	for _, app := range apps {
		if app == nil {
			continue
		}
		rememberChannel(seen, app.Kind)
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		rememberChannel(seen, channel.Type)
	}
	options := make([]ChannelBinding, 0, len(seen))
	for _, option := range seen {
		options = append(options, option)
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].Platform < options[j].Platform
	})
	return options, nil
}

func employeeView(item *AIEmployee, bindings []ChannelBinding) EmployeeView {
	if bindings == nil {
		bindings = []ChannelBinding{}
	}
	return EmployeeView{
		EmployeeID:     item.EmployeeKey,
		Name:           item.Name,
		Persona:        item.Persona,
		Status:         item.Status,
		Mode:           item.Mode,
		Prompt:         item.Prompt,
		History:        item.HistoryMessageCount,
		AutoReply:      item.AutoReplyEnabled,
		IsDefault:      item.IsDefault,
		SelectedDocIDs: stringList(item.SelectedDocIDs),
		Tools:          stringList(item.Tools),
		Channels:       bindings,
	}
}

func normalizeEmployeeName(value string) (string, string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", "", badRequest("名称不能为空")
	}
	if utf8.RuneCountInString(name) > employeeNameMaxRunes {
		return "", "", badRequest("名称不能超过 50 个字")
	}
	return name, strings.ToLower(name), nil
}

func normalizeEmployeeMode(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case employeeModeBasic, employeeModeProfessional:
		return strings.TrimSpace(value), nil
	default:
		return "", badRequest("不支持的配置模式")
	}
}

func normalizeEmployeeStatus(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case employeeStatusRunning, employeeStatusPaused:
		return strings.TrimSpace(value), nil
	default:
		return "", badRequest("不支持的运行状态")
	}
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeBindings(values, options []ChannelBinding) ([]ChannelBinding, error) {
	allowed := make(map[string]ChannelBinding, len(options))
	for _, option := range options {
		allowed[option.Platform] = option
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]ChannelBinding, 0, len(values))
	for _, value := range values {
		platform := normalizePlatform(value.Platform)
		if platform == "" {
			return nil, badRequest("渠道绑定不完整")
		}
		option, ok := allowed[platform]
		if !ok {
			return nil, badRequest("渠道不存在")
		}
		if _, ok := seen[platform]; ok {
			continue
		}
		seen[platform] = struct{}{}
		out = append(out, option)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Platform < out[j].Platform
	})
	return out, nil
}

func normalizePlatform(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func rememberChannel(seen map[string]ChannelBinding, raw string) {
	platform := normalizePlatform(raw)
	if platform == "" {
		return
	}
	if _, ok := seen[platform]; ok {
		return
	}
	seen[platform] = ChannelBinding{Platform: platform, Label: channelLabel(platform, raw)}
}

func channelLabel(platform, raw string) string {
	if label, ok := channelDisplayNames[platform]; ok {
		return label
	}
	if label := strings.TrimSpace(raw); label != "" {
		return label
	}
	return platform
}

func (b ChannelBinding) displayName() string {
	if label := strings.TrimSpace(b.Label); label != "" {
		return label
	}
	return channelLabel(b.Platform, b.Platform)
}

func labelIndex(options []ChannelBinding) map[string]string {
	labels := make(map[string]string, len(options))
	for _, option := range options {
		labels[option.Platform] = option.Label
	}
	return labels
}

func bindingsWithLabels(bindings []ChannelBinding, labels map[string]string) []ChannelBinding {
	out := make([]ChannelBinding, 0, len(bindings))
	for _, binding := range bindings {
		binding.Platform = normalizePlatform(binding.Platform)
		binding.Label = channelLabel(binding.Platform, firstNonEmpty(labels[binding.Platform], binding.Label))
		out = append(out, binding)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return cloneStrings(values)
}

// businessAppRow reads the gateway apps table without importing that module.
type businessAppRow struct {
	base.BaseModel
	AppKey      string `gorm:"column:app_key"`
	TenantID    string `gorm:"column:tenant_id"`
	DisplayName string `gorm:"column:display_name"`
	Kind        string `gorm:"column:kind"`
}

func (*businessAppRow) TableName() string { return "apps" }

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate")
}
